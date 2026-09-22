package chatassist

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/repo"
)

type planStop struct {
	At        string  `json:"at"`
	Label     string  `json:"label"`
	PlaceID   *string `json:"place_id,omitempty"`
	PlaceName *string `json:"place_name,omitempty"`
}
type promotionInput struct {
	Source    string     `json:"source_message_id"`
	Title     string     `json:"title"`
	Starts    string     `json:"starts_on"`
	Ends      string     `json:"ends_on"`
	Headcount int64      `json:"headcount"`
	Budget    int64      `json:"budget_per_person_vnd"`
	Stops     []planStop `json:"stops,omitempty"`
}
type promotionResult struct {
	OutingID string `json:"outing_id"`
	Revision int64  `json:"timeline_revision"`
	Source   string `json:"source_message_id"`
}

func (h *Handler) promotion(w http.ResponseWriter, r *http.Request) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_source"))
		return
	}
	tx, _, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	out := promotionResult{Source: r.PathValue("id")}
	err = tx.QueryRow(r.Context(), `SELECT p.outing_id,o.timeline_revision FROM chat_plan_promotions p JOIN outings o ON o.id=p.outing_id WHERE p.context_id=$1 AND p.source_message_id=$2`, r.PathValue("context"), out.Source).Scan(&out.OutingID, &out.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "promotion_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, out)
}

func (h *Handler) promote(w http.ResponseWriter, r *http.Request) {
	var in promotionInput
	if err := readBody(w, r, &in); err != nil {
		failure(w, err)
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	start, e1 := time.Parse("2006-01-02", in.Starts)
	end, e2 := time.Parse("2006-01-02", in.Ends)
	if !chatv2.ValidID(in.Source) || in.Title == "" || utf8.RuneCountInString(in.Title) > 200 || e1 != nil || e2 != nil || end.Before(start) || in.Headcount < 1 || in.Headcount > 100 || in.Budget < 0 || in.Budget > 1<<53-1 || len(in.Stops) > 50 {
		failure(w, &denied{422, "invalid_plan"})
		return
	}
	raw, _ := json.Marshal(in)
	digest := sha256.Sum256(raw)
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if g.kind != "group" {
		refuse(w, 409, "group_plan_only")
		return
	}
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('chat_plan:'||$1||':'||$2,0))`, r.PathValue("context"), in.Source); err != nil {
		failure(w, err)
		return
	}
	out := promotionResult{Source: in.Source}
	var old []byte
	err = tx.QueryRow(r.Context(), `SELECT p.outing_id,o.timeline_revision,p.input_digest FROM chat_plan_promotions p JOIN outings o ON o.id=p.outing_id WHERE p.context_id=$1 AND p.source_message_id=$2`, r.PathValue("context"), in.Source).Scan(&out.OutingID, &out.Revision, &old)
	if err == nil {
		if !bytes.Equal(old, digest[:]) {
			reply(w, 409, map[string]any{"code": "plan_already_promoted", "outing_id": out.OutingID})
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, out)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		failure(w, err)
		return
	}
	if err = lockFeed(r.Context(), tx, r.PathValue("context")); err != nil {
		failure(w, err)
		return
	}
	var card []byte
	err = tx.QueryRow(r.Context(), `SELECT card FROM messages WHERE id=$1 AND context_id=$2 AND kind='ai_card' AND deleted_at IS NULL FOR UPDATE`, in.Source, r.PathValue("context")).Scan(&card)
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "plan_source_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	var source struct {
		Kind    string `json:"kind"`
		Payload struct {
			Stops []struct {
				At    string `json:"time_text"`
				Place struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"place"`
			} `json:"stops"`
		} `json:"payload"`
	}
	if json.Unmarshal(card, &source) != nil || source.Kind != "itinerary" || len(source.Payload.Stops) == 0 {
		refuse(w, 422, "plan_source_invalid")
		return
	}
	if in.Stops == nil {
		for _, s := range source.Payload.Stops {
			id, name := s.Place.ID, s.Place.Name
			stop := planStop{At: s.At, Label: name, PlaceName: &name}
			// A stop somebody typed on a group sheet has no catalogue place.
			// Binding an empty id here would send "" to GetPlace and refuse a
			// perfectly good plan with plan_place_unavailable.
			if id != "" {
				stop.PlaceID = &id
			}
			in.Stops = append(in.Stops, stop)
		}
	}
	if len(in.Stops) == 0 || len(in.Stops) > 50 {
		refuse(w, 422, "plan_stops_required")
		return
	}
	store := repo.Repository{Q: tx}
	stops := make([]repo.TimelineStop, 0, len(in.Stops))
	for _, s := range in.Stops {
		at, e := time.Parse("15:04", s.At)
		if e != nil || at.Format("15:04") != s.At {
			refuse(w, 422, "plan_time_review_required")
			return
		}
		label := strings.TrimSpace(s.Label)
		if label == "" || utf8.RuneCountInString(label) > 200 {
			refuse(w, 422, "invalid_stop_label")
			return
		}
		if s.PlaceID != nil {
			p, e := store.GetPlace(r.Context(), *s.PlaceID)
			if e != nil {
				failure(w, e)
				return
			}
			if p == nil {
				refuse(w, 422, "plan_place_unavailable")
				return
			}
			s.PlaceName = &p.Name
		}
		if s.PlaceName != nil && utf8.RuneCountInString(*s.PlaceName) > 200 {
			refuse(w, 422, "invalid_place_name")
			return
		}
		stops = append(stops, repo.TimelineStop{MinuteOfDay: int64(at.Hour()*60 + at.Minute()), Label: label, PlaceID: s.PlaceID, PlaceName: s.PlaceName})
	}
	outing, err := store.CreateOuting(r.Context(), repo.OutingInput{ContextID: r.PathValue("context"), CreatedByID: g.person, Title: in.Title, StartsOn: start, EndsOn: end, Headcount: in.Headcount, BudgetPerPersonVND: in.Budget, Now: time.Now().UTC()})
	if err != nil {
		failure(w, err)
		return
	}
	revision := outing.TimelineRevision
	outing, err = store.ReplaceOutingStops(r.Context(), outing.ID, stops, &revision)
	if err != nil {
		failure(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO chat_plan_promotions(context_id,source_message_id,outing_id,input_digest,created_by_id) VALUES($1,$2,$3,$4,$5)`, r.PathValue("context"), in.Source, outing.ID, digest[:], g.person)
	if err != nil {
		failure(w, err)
		return
	}
	// Bind the visible source sheet in the same transaction. Its normal change
	// event updates every reader, including the pinned sheet and quoted card.
	// The card carries the sheet's state as well as the outing binding. Leaving
	// `draft.status` at "open" here made a promoted card contradict itself on
	// the wire -- outing_id set, draft still open -- so a screen could offer to
	// keep editing a sheet that is already a kèo. `create_missing=false` makes
	// the last step a no-op for an AI card, which has no draft block.
	_, err = tx.Exec(r.Context(), `UPDATE messages SET card=jsonb_set(jsonb_set(jsonb_set(card,'{payload,outing_id}',to_jsonb($2::text)),'{payload,timeline_revision}',to_jsonb($3::bigint)),'{payload,draft,status}','"promoted"'::jsonb,false) WHERE id=$1`, in.Source, outing.ID, outing.TimelineRevision)
	if err != nil {
		failure(w, err)
		return
	}
	// A group sheet that becomes a kèo stops being a draft, in the same
	// transaction that created the kèo. Nothing happens for an AI card, which
	// has no row here.
	if _, err = tx.Exec(r.Context(),
		`UPDATE chat_shared_drafts SET status='promoted',revision=revision+1,updated_at=clock_timestamp()
		  WHERE message_id=$1 AND status='open'`, in.Source); err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	out.OutingID = outing.ID
	out.Revision = outing.TimelineRevision
	reply(w, 201, out)
}

// lockFeed follows the candidate writer order when the optional feed is
// installed. Standalone job tests may run without that additive migration.
func lockFeed(ctx context.Context, tx pgx.Tx, room string) error {
	var table *string
	if err := tx.QueryRow(ctx, `SELECT to_regclass('chat_legacy_change_heads')::text`).Scan(&table); err != nil {
		return err
	}
	if table == nil {
		return nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_legacy_change_heads(context_id,sequence) VALUES($1,0) ON CONFLICT DO NOTHING`, room); err != nil {
		return err
	}
	var sequence int64
	return tx.QueryRow(ctx, `SELECT sequence FROM chat_legacy_change_heads WHERE context_id=$1 FOR UPDATE`, room).Scan(&sequence)
}
