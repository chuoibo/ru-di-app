package achievementv1

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/domain/achievement"
	"mobile/services/core/internal/repo"
)

// Store is the sole writer for achievement state. Q is one request transaction.
type Store struct{ Q repo.Querier }

type EarnedBadge struct {
	ID        string    `json:"id"`
	EarnedAt  time.Time `json:"earned_at"`
	Displayed bool      `json:"displayed"`
}

type Run struct {
	ID         string     `json:"id"`
	RouteID    string     `json:"route_id"`
	EndingID   string     `json:"ending_id"`
	SelectedAt time.Time  `json:"selected_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ClosedAt   *time.Time `json:"-"`
}

type CreditCounts struct {
	Granted   int `json:"granted"`
	Used      int `json:"used"`
	Available int `json:"available"`
}

func (s Store) CreditSummary(ctx context.Context, personID string) (CreditCounts, error) {
	var c CreditCounts
	err := s.Q.QueryRow(ctx, `SELECT count(g.source),
	 count(*) FILTER(WHERE j.status='ready'),
	 count(*) FILTER(WHERE g.source IS NOT NULL AND j.id IS NULL)
	 FROM achievement_mp4_credits g LEFT JOIN profile_media_jobs j
	 ON j.person_id=g.person_id AND j.credit_source=g.source AND j.status IN ('reserved','queued','running','ready')
	 WHERE g.person_id=$1::uuid`, personID).Scan(&c.Granted, &c.Used, &c.Available)
	return c, err
}

// Facts counts self-reported check-ins and distinct server-recorded evidence.
// Arrival order does not affect the shared-outing calculation: each read joins
// every actor's check-ins in the outing.
func (s Store) Facts(ctx context.Context, personID string) (achievement.Facts, error) {
	var f achievement.Facts
	err := s.Q.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM outing_stop_checkins WHERE person_id=$1::uuid)
	 +(SELECT count(*) FROM memories WHERE author_id=$1::uuid AND kind='checkin'),
	 (SELECT count(DISTINCT (created_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) FROM memories WHERE author_id=$1::uuid AND kind='photo'),
	 (SELECT count(*) FROM posts WHERE author_id=$1::uuid AND char_length(btrim(body))>=40),
	 (SELECT count(DISTINCT (created_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) FROM posts WHERE author_id=$1::uuid AND char_length(btrim(body))>=40)`, personID).Scan(&f.Checkins, &f.PhotoDays, &f.StoryCount, &f.StoryDays)
	if err != nil {
		return f, err
	}
	err = s.Q.QueryRow(ctx, `WITH own AS (
	 SELECT DISTINCT os.outing_id,os.place_id,p.destination_id
	 FROM outing_stop_checkins c JOIN outing_stops os ON os.id=c.stop_id
	 LEFT JOIN places p ON p.id=os.place_id WHERE c.person_id=$1::uuid
	), per_trip AS (SELECT outing_id,count(DISTINCT place_id) AS places FROM own GROUP BY outing_id)
	 SELECT COALESCE((SELECT max(places) FROM per_trip),0),
	 (SELECT count(DISTINCT destination_id) FROM own),
	 (SELECT count(DISTINCT outing_id) FROM own WHERE destination_id IS NOT NULL)`, personID).
		Scan(&f.DistinctPlacesInOneOuting, &f.DistinctDestinations, &f.OutingsWithDestinations)
	if err != nil {
		return f, err
	}
	err = s.Q.QueryRow(ctx, `WITH own AS (
	 SELECT DISTINCT os.outing_id FROM outing_stop_checkins c JOIN outing_stops os ON os.id=c.stop_id WHERE c.person_id=$1::uuid
	), party AS (
	 SELECT own.outing_id,count(DISTINCT c.person_id) AS people
	 FROM own JOIN outing_stops os ON os.outing_id=own.outing_id
	 JOIN outing_stop_checkins c ON c.stop_id=os.id GROUP BY own.outing_id
	), companions AS (
	 SELECT c.person_id,count(DISTINCT own.outing_id) AS trips
	 FROM own JOIN outing_stops os ON os.outing_id=own.outing_id
	 JOIN outing_stop_checkins c ON c.stop_id=os.id
	 WHERE c.person_id<>$1::uuid GROUP BY c.person_id
	)
	 SELECT (SELECT count(*) FROM party WHERE people>=2),
	 COALESCE((SELECT max(people) FROM party),0),
	 COALESCE((SELECT max(trips) FROM companions),0)`, personID).
		Scan(&f.SharedOutings, &f.LargestSharedParty, &f.RepeatedCompanionOutings)
	if err != nil {
		return f, err
	}
	err = s.Q.QueryRow(ctx, `SELECT
	 EXISTS (SELECT 1 FROM memories photo WHERE photo.author_id=$1::uuid AND photo.kind='photo' AND photo.place_id IS NOT NULL
	   AND (EXISTS(SELECT 1 FROM memories c WHERE c.author_id=$1::uuid AND c.kind='checkin' AND c.place_id=photo.place_id)
	     OR EXISTS(SELECT 1 FROM outing_stop_checkins c JOIN outing_stops os ON os.id=c.stop_id WHERE c.person_id=$1::uuid AND os.place_id=photo.place_id))),
	 EXISTS (SELECT 1 FROM memories photo WHERE photo.author_id=$1::uuid AND photo.kind='photo'
	   AND EXISTS(SELECT 1 FROM outings o WHERE o.context_id=photo.context_id
	     AND EXISTS(SELECT 1 FROM outing_stops own_stop JOIN outing_stop_checkins own_c ON own_c.stop_id=own_stop.id
	       WHERE own_stop.outing_id=o.id AND own_c.person_id=$1::uuid)
	     AND (SELECT count(DISTINCT c.person_id) FROM outing_stops os JOIN outing_stop_checkins c ON c.stop_id=os.id WHERE os.outing_id=o.id)>=2))`, personID).
		Scan(&f.PhotoAtCheckedPlace, &f.PhotoInSharedGroup)
	return f, err
}

func (s Store) Earned(ctx context.Context, personID string) ([]EarnedBadge, error) {
	rows, err := s.Q.Query(ctx, `SELECT e.badge_id,e.earned_at,(d.badge_id IS NOT NULL)
	 FROM achievement_earned e LEFT JOIN achievement_display d ON d.person_id=e.person_id AND d.badge_id=e.badge_id
	 WHERE e.person_id=$1::uuid ORDER BY e.earned_at,e.badge_id`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EarnedBadge{}
	for rows.Next() {
		var badge EarnedBadge
		if err := rows.Scan(&badge.ID, &badge.EarnedAt, &badge.Displayed); err != nil {
			return nil, err
		}
		out = append(out, badge)
	}
	return out, rows.Err()
}

func (s Store) GrantBadge(ctx context.Context, personID, badgeID string) error {
	_, err := s.Q.Exec(ctx, `INSERT INTO achievement_earned(person_id,badge_id) VALUES($1::uuid,$2) ON CONFLICT DO NOTHING`, personID, badgeID)
	return err
}

func (s Store) GrantCredit(ctx context.Context, personID, source string) error {
	_, err := s.Q.Exec(ctx, `INSERT INTO achievement_mp4_credits(person_id,source) VALUES($1::uuid,$2) ON CONFLICT DO NOTHING`, personID, source)
	return err
}

func (s Store) Credits(ctx context.Context, personID string) (int, error) {
	var count int
	err := s.Q.QueryRow(ctx, `SELECT count(*) FROM achievement_mp4_credits WHERE person_id=$1::uuid`, personID).Scan(&count)
	return count, err
}

func (s Store) ActiveRun(ctx context.Context, personID string) (*Run, error) {
	var run Run
	err := s.Q.QueryRow(ctx, `SELECT id,route_id,ending_id,selected_at,finished_at FROM achievement_runs WHERE person_id=$1::uuid AND closed_at IS NULL`, personID).
		Scan(&run.ID, &run.RouteID, &run.EndingID, &run.SelectedAt, &run.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// RouteHistory remembers choices, not private outing or social contents.
func (s Store) RouteHistory(ctx context.Context, personID string) ([]string, error) {
	rows, err := s.Q.Query(ctx, `SELECT route_id FROM achievement_runs WHERE person_id=$1::uuid ORDER BY selected_at DESC,id DESC LIMIT 8`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		history = append(history, id)
	}
	return history, rows.Err()
}

func (s Store) LockPerson(ctx context.Context, personID string) error {
	var held string
	return s.Q.QueryRow(ctx, `SELECT id FROM people WHERE id=$1::uuid FOR UPDATE`, personID).Scan(&held)
}

func (s Store) SelectRun(ctx context.Context, personID, routeID, endingID string) (Run, error) {
	// Serialize two phones changing the selected route through the person row.
	if err := s.LockPerson(ctx, personID); err != nil {
		return Run{}, err
	}
	if _, err := s.Q.Exec(ctx, `UPDATE achievement_runs SET closed_at=clock_timestamp() WHERE person_id=$1::uuid AND closed_at IS NULL`, personID); err != nil {
		return Run{}, err
	}
	id, err := repo.NewUUID()
	if err != nil {
		return Run{}, err
	}
	var run Run
	err = s.Q.QueryRow(ctx, `INSERT INTO achievement_runs(id,person_id,route_id,ending_id) VALUES($1::uuid,$2::uuid,$3,$4) RETURNING id,route_id,ending_id,selected_at,finished_at`, id, personID, routeID, endingID).
		Scan(&run.ID, &run.RouteID, &run.EndingID, &run.SelectedAt, &run.FinishedAt)
	return run, err
}

func (s Store) RunByID(ctx context.Context, personID, runID string) (*Run, error) {
	var run Run
	err := s.Q.QueryRow(ctx, `SELECT id,route_id,ending_id,selected_at,finished_at,closed_at FROM achievement_runs WHERE id=$1::uuid AND person_id=$2::uuid`, runID, personID).
		Scan(&run.ID, &run.RouteID, &run.EndingID, &run.SelectedAt, &run.FinishedAt, &run.ClosedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s Store) FinishRun(ctx context.Context, personID, runID string) error {
	_, err := s.Q.Exec(ctx, `UPDATE achievement_runs SET finished_at=COALESCE(finished_at,clock_timestamp()),closed_at=COALESCE(closed_at,clock_timestamp()) WHERE id=$1::uuid AND person_id=$2::uuid`, runID, personID)
	return err
}

func (s Store) SetDisplay(ctx context.Context, personID string, ids []string) error {
	// The caller verifies ownership before replacing; this FK is a second gate.
	if _, err := s.Q.Exec(ctx, `DELETE FROM achievement_display WHERE person_id=$1::uuid`, personID); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := s.Q.Exec(ctx, `INSERT INTO achievement_display(person_id,badge_id,position) VALUES($1::uuid,$2,$3)`, personID, id, i); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) PublicDisplay(ctx context.Context, personID string) ([]EarnedBadge, error) {
	rows, err := s.Q.Query(ctx, `SELECT e.badge_id,e.earned_at FROM achievement_display d JOIN achievement_earned e ON e.person_id=d.person_id AND e.badge_id=d.badge_id WHERE d.person_id=$1::uuid ORDER BY d.position`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EarnedBadge{}
	for rows.Next() {
		var b EarnedBadge
		if err := rows.Scan(&b.ID, &b.EarnedAt); err != nil {
			return nil, err
		}
		b.Displayed = true
		out = append(out, b)
	}
	return out, rows.Err()
}

// BlockedBetween denies even an otherwise shared-context profile view.
func (s Store) BlockedBetween(ctx context.Context, a, b string) (bool, error) {
	var blocked bool
	err := s.Q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM friend_requests WHERE state='blocked' AND
		((requester_id=$1::uuid AND addressee_id=$2::uuid) OR (requester_id=$2::uuid AND addressee_id=$1::uuid)))`, a, b).Scan(&blocked)
	return blocked, err
}
