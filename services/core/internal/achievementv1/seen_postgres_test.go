//go:build postgres

package achievementv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/testdb"
)

// QA UI-160: «MỚI MỞ» follows the account. The owner's read says which earned
// badges were already presented; marking keeps the first presentation; a
// badge not earned is refused; another person's view never carries the mark;
// and the migration takes badges older than 48 hours as seen.
func TestSeenBadgesFollowTheAccountInPostgres(t *testing.T) {
	ctx := context.Background()
	tx := testdb.Tx(t)
	schema := fmt.Sprintf("achievement_seen_%d", time.Now().UnixNano())
	for _, q := range []string{
		"CREATE SCHEMA " + pgx.Identifier{schema}.Sanitize(),
		"SET LOCAL search_path TO " + pgx.Identifier{schema}.Sanitize(),
		`CREATE TABLE people(id uuid PRIMARY KEY,deleted_at timestamptz)`,
		`CREATE TABLE friend_requests(id uuid PRIMARY KEY,requester_id uuid NOT NULL,addressee_id uuid NOT NULL,state text NOT NULL)`,
		`CREATE TABLE memberships(id uuid PRIMARY KEY,context_id uuid NOT NULL,person_id uuid NOT NULL,state text NOT NULL)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := MigrateTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := MigrateTx(ctx, tx); err != nil {
		t.Fatalf("second migrate run: %v", err)
	}
	const a = "abababab-abab-4bab-8bab-abababababab"
	const b = "bcbcbcbc-bcbc-4cbc-8cbc-bcbcbcbcbcbc"
	const group = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for _, q := range []string{
		`INSERT INTO people(id) VALUES('` + a + `'),('` + b + `')`,
		`INSERT INTO memberships VALUES('cccccccc-cccc-4ccc-8ccc-cccccccccccc','` + group + `','` + a + `','active'),('dddddddd-dddd-4ddd-8ddd-dddddddddddd','` + group + `','` + b + `','active')`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	store := Store{Q: tx}
	for _, id := range []string{"first_checkin", "first_photo"} {
		if err := store.GrantBadge(ctx, a, id); err != nil {
			t.Fatal(err)
		}
	}
	seenOf := func() map[string]bool {
		earned, err := store.Earned(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, e := range earned {
			if e.Seen == nil {
				t.Fatalf("the owner's read left out seen for %s", e.ID)
			}
			out[e.ID] = *e.Seen
		}
		return out
	}
	if got := seenOf(); got["first_checkin"] || got["first_photo"] {
		t.Fatalf("a badge just earned reads as seen: %v", got)
	}

	h := New(nil, "dev")
	post := func(body string) (int, error) {
		r, err := http.NewRequest(http.MethodPost, "/me/achievement-seen", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		status, _, err := h.handle(ctx, r, store, a)
		return status, err
	}
	if status, err := post(`{"badge_ids":["first_photo"]}`); err != nil || status != 200 {
		t.Fatalf("mark seen: %d %v", status, err)
	}
	if got := seenOf(); !got["first_photo"] || got["first_checkin"] {
		t.Fatalf("after marking first_photo: %v", got)
	}
	var first time.Time
	if err := tx.QueryRow(ctx, `SELECT seen_at FROM achievement_earned WHERE person_id=$1 AND badge_id='first_photo'`, a).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if status, err := post(`{"badge_ids":["first_photo"]}`); err != nil || status != 200 {
		t.Fatalf("marking again: %d %v", status, err)
	}
	var again time.Time
	if err := tx.QueryRow(ctx, `SELECT seen_at FROM achievement_earned WHERE person_id=$1 AND badge_id='first_photo'`, a).Scan(&again); err != nil || !again.Equal(first) {
		t.Fatalf("the first presentation moved: %v -> %v (%v)", first, again, err)
	}
	for body, code := range map[string]string{
		`{"badge_ids":["storyteller"]}`:               "badge_not_earned",
		`{"badge_ids":["first_photo","first_photo"]}`: "badge_not_earned",
		`{"badge_ids":[]}`:                            "invalid_request",
		`{"badge_ids":["first_photo"],"extra":true}`:  "invalid_request",
	} {
		if _, err := post(body); err == nil || err.(*routeError).Code != code {
			t.Fatalf("%s: want %s, got %v", body, code, err)
		}
	}

	// Another person's view of the displayed badges carries no mark.
	if err := store.SetDisplay(ctx, a, []string{"first_photo"}); err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest(http.MethodGet, "/people/"+a+"/achievements", nil)
	status, body, err := h.handle(ctx, r, store, b)
	if err != nil || status != 200 {
		t.Fatalf("groupmate view: %d %v", status, err)
	}
	wire, _ := json.Marshal(body)
	if strings.Contains(string(wire), `"seen"`) {
		t.Fatalf("another person's view carries the seen mark: %s", wire)
	}

	// The migration's own rule, on a badge from before it: older than 48 hours
	// is seen, recent is not.
	for _, q := range []string{
		`ALTER TABLE achievement_earned DROP COLUMN seen_at`,
		`UPDATE achievement_earned SET earned_at = clock_timestamp() - interval '72 hours' WHERE badge_id='first_checkin'`,
		`UPDATE achievement_earned SET earned_at = clock_timestamp() - interval '2 hours' WHERE badge_id='first_photo'`,
		seenSQL,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if got := seenOf(); !got["first_checkin"] || got["first_photo"] {
		t.Fatalf("migration backfill: %v", got)
	}
}
