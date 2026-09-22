package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/chatv2"
)

// A shared draft is the group's own tờ hẹn: one sheet several people edit
// before anyone commits to it.
//
// Two decisions shape everything below.
//
// The sheet is *anchored to a message*. It is written as an ordinary
// `ai_card` carrying an itinerary card, so the durable change feed, the
// snapshot lane and `plan-promotions` all work on it without a line of new
// plumbing -- and everyone in the room sees an edit arrive through the lane
// they already listen on. This table is the source of truth; the card is its
// projection, repainted on every edit.
//
// Edits are *revision-guarded*. Two people typing at once is the normal case,
// not the exception, so the second writer is told its copy is stale (409) and
// hands the conflict to a person. Last-write-wins would silently drop whatever
// the other one just decided.
const (
	maxDraftStops    = 50
	maxDraftTitle    = 200
	maxDraftLabel    = 200
	maxDraftHeadroom = 1000
)

type draftStop struct {
	TimeText string `json:"time_text"`
	Label    string `json:"label"`
	PlaceID  string `json:"place_id,omitempty"`
}

type draftRow struct {
	ID        string
	Context   string
	Message   string
	SourceVot *string
	Revision  int64
	Title     string
	Starts    *string
	Ends      *string
	Headcount *int64
	Budget    *int64
	Stops     []draftStop
	Status    string
	CreatedBy string
}

type draftBody struct {
	Title     *string      `json:"title"`
	Starts    *string      `json:"starts_on"`
	Ends      *string      `json:"ends_on"`
	Headcount *int64       `json:"headcount"`
	Budget    *int64       `json:"budget_per_person_vnd"`
	Stops     *[]draftStop `json:"stops"`
	// FromVote seeds a new sheet from a decided poll, so the room can see which
	// sheet is receiving the choice it just made.
	FromVote *string `json:"from_vote_id,omitempty"`
	// Revision is the copy the editor was looking at. Required on a change.
	Revision *int64 `json:"revision,omitempty"`
}

type draftView struct {
	ID        string      `json:"id"`
	ContextID string      `json:"context_id"`
	MessageID string      `json:"message_id"`
	SourceVot *string     `json:"source_vote_id"`
	Revision  int64       `json:"revision"`
	Title     string      `json:"title"`
	Starts    *string     `json:"starts_on"`
	Ends      *string     `json:"ends_on"`
	Headcount *int64      `json:"headcount"`
	Budget    *int64      `json:"budget_per_person_vnd"`
	Stops     []draftStop `json:"stops"`
	Status    string      `json:"status"`
	CreatedBy string      `json:"created_by"`
}

func (d draftRow) view() draftView {
	stops := d.Stops
	if stops == nil {
		stops = []draftStop{}
	}
	return draftView{
		ID: d.ID, ContextID: d.Context, MessageID: d.Message, SourceVot: d.SourceVot,
		Revision: d.Revision, Title: d.Title, Starts: d.Starts, Ends: d.Ends,
		Headcount: d.Headcount, Budget: d.Budget, Stops: stops,
		Status: d.Status, CreatedBy: d.CreatedBy,
	}
}

const draftColumns = `id,context_id,message_id,source_vote_id,revision,title,
 to_char(starts_on,'YYYY-MM-DD'),to_char(ends_on,'YYYY-MM-DD'),
 headcount,budget_per_person_vnd,stops,status,created_by`

func scanDraft(row pgx.Row) (draftRow, error) {
	var d draftRow
	var stops []byte
	if err := row.Scan(&d.ID, &d.Context, &d.Message, &d.SourceVot, &d.Revision, &d.Title,
		&d.Starts, &d.Ends, &d.Headcount, &d.Budget, &stops, &d.Status, &d.CreatedBy); err != nil {
		return draftRow{}, err
	}
	if len(stops) > 0 {
		if err := json.Unmarshal(stops, &d.Stops); err != nil {
			return draftRow{}, err
		}
	}
	return d, nil
}

// validateStops keeps the sheet promotable: `plan-promotions` parses the same
// times, so a sheet that cannot be confirmed must not be storable either.
func validateStops(stops []draftStop) ([]draftStop, error) {
	if len(stops) == 0 {
		return nil, invalid("draft_stops_required")
	}
	if len(stops) > maxDraftStops {
		return nil, invalid("draft_too_many_stops")
	}
	out := make([]draftStop, 0, len(stops))
	for _, s := range stops {
		at, err := time.Parse("15:04", s.TimeText)
		if err != nil || at.Format("15:04") != s.TimeText {
			return nil, invalid("draft_time_invalid")
		}
		label := strings.TrimSpace(s.Label)
		if label == "" || utf8.RuneCountInString(label) > maxDraftLabel {
			return nil, invalid("draft_label_invalid")
		}
		if s.PlaceID != "" && !chatv2.ValidID(s.PlaceID) {
			return nil, invalid("draft_place_invalid")
		}
		out = append(out, draftStop{TimeText: s.TimeText, Label: label, PlaceID: s.PlaceID})
	}
	return out, nil
}

func validateDraftHead(title string, starts, ends *string, headcount, budget *int64) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > maxDraftTitle {
		return invalid("draft_title_invalid")
	}
	for _, day := range []*string{starts, ends} {
		if day == nil || *day == "" {
			continue
		}
		parsed, err := time.Parse("2006-01-02", *day)
		if err != nil || parsed.Format("2006-01-02") != *day {
			return invalid("draft_date_invalid")
		}
	}
	if starts != nil && ends != nil && *starts != "" && *ends != "" && *ends < *starts {
		return invalid("draft_date_order")
	}
	if headcount != nil && (*headcount <= 0 || *headcount > maxDraftHeadroom) {
		return invalid("draft_headcount_invalid")
	}
	if budget != nil && *budget < 0 {
		return invalid("draft_budget_invalid")
	}
	return nil
}

// card renders the sheet the way the thread shows it. The card kind stays
// "itinerary" on purpose: promotion already understands that shape, and the
// `draft` block is what tells a client this one can still be edited.
func (d draftRow) card() ([]byte, error) {
	stops := make([]map[string]any, 0, len(d.Stops))
	for _, s := range d.Stops {
		place := map[string]any{"id": s.PlaceID, "name": s.Label}
		if s.PlaceID == "" {
			place = map[string]any{"name": s.Label}
		}
		stops = append(stops, map[string]any{
			"time_text": s.TimeText,
			"note":      "",
			"place":     place,
		})
	}
	return json.Marshal(map[string]any{
		"kind": "itinerary",
		"payload": map[string]any{
			"title": d.Title,
			"stops": stops,
			"draft": map[string]any{
				"id":                    d.ID,
				"revision":              d.Revision,
				"status":                d.Status,
				"source_vote_id":        d.SourceVot,
				"starts_on":             d.Starts,
				"ends_on":               d.Ends,
				"headcount":             d.Headcount,
				"budget_per_person_vnd": d.Budget,
			},
		},
	})
}

func (h *Handler) draftCreate(w http.ResponseWriter, r *http.Request) {
	var in draftBody
	if err := readBody(w, r, &in); err != nil {
		failure(w, err)
		return
	}
	if in.Title == nil || in.Stops == nil {
		failure(w, invalid("draft_incomplete"))
		return
	}
	if err := validateDraftHead(*in.Title, in.Starts, in.Ends, in.Headcount, in.Budget); err != nil {
		failure(w, err)
		return
	}
	stops, err := validateStops(*in.Stops)
	if err != nil {
		failure(w, err)
		return
	}
	if in.FromVote != nil && !chatv2.ValidID(*in.FromVote) {
		failure(w, invalid("draft_vote_invalid"))
		return
	}
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
	room := r.PathValue("context")
	// Same lock the promotion path takes, and in the same order, so a sheet
	// being created cannot interleave with one being confirmed.
	if err = lockFeed(r.Context(), tx, room); err != nil {
		failure(w, err)
		return
	}
	if in.FromVote != nil {
		var closed bool
		err = tx.QueryRow(r.Context(),
			`SELECT closed_at IS NOT NULL FROM votes WHERE id=$1 AND context_id=$2`,
			*in.FromVote, room).Scan(&closed)
		if errors.Is(err, pgx.ErrNoRows) {
			refuse(w, 404, "draft_vote_not_found")
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		if !closed {
			// An open poll has not decided anything yet, so there is nothing
			// for a sheet to receive.
			refuse(w, 409, "draft_vote_open")
			return
		}
	}
	draft := draftRow{
		ID: newID(), Context: room, Message: newID(), SourceVot: in.FromVote,
		Revision: 1, Title: strings.TrimSpace(*in.Title), Starts: in.Starts, Ends: in.Ends,
		Headcount: in.Headcount, Budget: in.Budget, Stops: stops,
		Status: "open", CreatedBy: g.person,
	}
	payload, err := draft.card()
	if err != nil {
		failure(w, err)
		return
	}
	// The anchor is authored by the person, not by the assistant: a group sheet
	// is nobody's suggestion.
	if _, err = tx.Exec(r.Context(),
		`INSERT INTO messages (id,context_id,author_id,kind,card,created_at)
		 VALUES ($1::uuid,$2::uuid,$3::uuid,'ai_card',$4::jsonb,clock_timestamp())`,
		draft.Message, room, g.person, payload); err != nil {
		failure(w, err)
		return
	}
	stopsJSON, err := json.Marshal(stops)
	if err != nil {
		failure(w, err)
		return
	}
	_, err = tx.Exec(r.Context(),
		`INSERT INTO chat_shared_drafts
		  (id,context_id,message_id,source_vote_id,revision,title,starts_on,ends_on,
		   headcount,budget_per_person_vnd,stops,status,created_by)
		 VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,1,$5,$6::date,$7::date,$8,$9,$10::jsonb,'open',$11::uuid)`,
		draft.ID, room, draft.Message, draft.SourceVot, draft.Title,
		draft.Starts, draft.Ends, draft.Headcount, draft.Budget, stopsJSON, g.person)
	if err != nil {
		if isUniqueViolation(err) {
			// The partial unique index: one open sheet per decided poll.
			refuse(w, 409, "draft_vote_already_has_sheet")
			return
		}
		failure(w, err)
		return
	}
	if err = recordEdit(r.Context(), tx, draft.ID, 1, g.person, stopsJSON); err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, http.StatusCreated, draft.view())
}

func (h *Handler) draftGet(w http.ResponseWriter, r *http.Request) {
	tx, _, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_draft"))
		return
	}
	draft, err := scanDraft(tx.QueryRow(r.Context(),
		`SELECT `+draftColumns+` FROM chat_shared_drafts WHERE id=$1 AND context_id=$2`,
		r.PathValue("id"), r.PathValue("context")))
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "draft_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, http.StatusOK, draft.view())
}

func (h *Handler) draftPatch(w http.ResponseWriter, r *http.Request) {
	var in draftBody
	if err := readBody(w, r, &in); err != nil {
		failure(w, err)
		return
	}
	if in.Revision == nil {
		// Without the revision the writer is not telling us what it was
		// looking at, and we cannot tell an edit from an overwrite.
		failure(w, invalid("draft_revision_required"))
		return
	}
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_draft"))
		return
	}
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	room := r.PathValue("context")
	if err = lockFeed(r.Context(), tx, room); err != nil {
		failure(w, err)
		return
	}
	draft, err := scanDraft(tx.QueryRow(r.Context(),
		`SELECT `+draftColumns+` FROM chat_shared_drafts WHERE id=$1 AND context_id=$2 FOR UPDATE`,
		r.PathValue("id"), room))
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "draft_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	if draft.Status != "open" {
		refuse(w, 409, "draft_not_open")
		return
	}
	if *in.Revision != draft.Revision {
		// Somebody else changed the sheet while this editor was typing. The
		// person decides what to keep, not the database.
		refuse(w, 409, "draft_revision_stale")
		return
	}
	if in.Title != nil {
		draft.Title = strings.TrimSpace(*in.Title)
	}
	if in.Starts != nil {
		draft.Starts = in.Starts
	}
	if in.Ends != nil {
		draft.Ends = in.Ends
	}
	if in.Headcount != nil {
		draft.Headcount = in.Headcount
	}
	if in.Budget != nil {
		draft.Budget = in.Budget
	}
	if err = validateDraftHead(draft.Title, draft.Starts, draft.Ends, draft.Headcount, draft.Budget); err != nil {
		failure(w, err)
		return
	}
	if in.Stops != nil {
		stops, e := validateStops(*in.Stops)
		if e != nil {
			failure(w, e)
			return
		}
		draft.Stops = stops
	}
	draft.Revision++
	payload, err := draft.card()
	if err != nil {
		failure(w, err)
		return
	}
	stopsJSON, err := json.Marshal(draft.Stops)
	if err != nil {
		failure(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(),
		`UPDATE chat_shared_drafts SET revision=$2,title=$3,starts_on=$4::date,ends_on=$5::date,
		   headcount=$6,budget_per_person_vnd=$7,stops=$8::jsonb,updated_at=clock_timestamp()
		 WHERE id=$1::uuid`,
		draft.ID, draft.Revision, draft.Title, draft.Starts, draft.Ends,
		draft.Headcount, draft.Budget, stopsJSON); err != nil {
		failure(w, err)
		return
	}
	// Repainting the anchor is what puts the change on everyone else's screen:
	// the messages trigger writes one row into the durable feed.
	if _, err = tx.Exec(r.Context(),
		`UPDATE messages SET card=$2::jsonb WHERE id=$1::uuid AND deleted_at IS NULL`,
		draft.Message, payload); err != nil {
		failure(w, err)
		return
	}
	if err = recordEdit(r.Context(), tx, draft.ID, draft.Revision, g.person, stopsJSON); err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, http.StatusOK, draft.view())
}

func (h *Handler) draftDiscard(w http.ResponseWriter, r *http.Request) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_draft"))
		return
	}
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	room := r.PathValue("context")
	if err = lockFeed(r.Context(), tx, room); err != nil {
		failure(w, err)
		return
	}
	draft, err := scanDraft(tx.QueryRow(r.Context(),
		`SELECT `+draftColumns+` FROM chat_shared_drafts WHERE id=$1 AND context_id=$2 FOR UPDATE`,
		r.PathValue("id"), room))
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "draft_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	if draft.Status == "promoted" {
		// A sheet that already became a kèo is history, not a draft.
		refuse(w, 409, "draft_already_promoted")
		return
	}
	if draft.Status == "discarded" {
		reply(w, http.StatusOK, draft.view())
		return
	}
	draft.Status = "discarded"
	draft.Revision++
	payload, err := draft.card()
	if err != nil {
		failure(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(),
		`UPDATE chat_shared_drafts SET status='discarded',revision=$2,updated_at=clock_timestamp() WHERE id=$1::uuid`,
		draft.ID, draft.Revision); err != nil {
		failure(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(),
		`UPDATE messages SET card=$2::jsonb WHERE id=$1::uuid AND deleted_at IS NULL`,
		draft.Message, payload); err != nil {
		failure(w, err)
		return
	}
	if err = recordEdit(r.Context(), tx, draft.ID, draft.Revision, g.person, []byte(`{"discarded":true}`)); err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, http.StatusOK, draft.view())
}

func recordEdit(ctx context.Context, tx pgx.Tx, draft string, revision int64, editor string, patch []byte) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO chat_shared_draft_edits (draft_id,revision,editor_id,patch)
		 VALUES ($1::uuid,$2,$3::uuid,$4::jsonb)`,
		draft, revision, editor, patch)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}
