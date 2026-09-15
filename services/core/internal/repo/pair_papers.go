package repo

// The W8 slice of SqlAlchemyApiRepository behind the paper routes
// (routes/pair_papers.py): the sheet, its versions, views, responses, kept
// lines and the outing it became. The conventions are pair_notebooks.go's:
// one flush per method, changed columns only, every `session.get` a SELECT.
//
// Two triggers act inside these statements rather than at COMMIT:
// pair_paper_versions_immutable refuses an UPDATE or DELETE of a version
// whose sent_at is set, and pair_paper_response_is_participant refuses a
// response from somebody with no open membership of the paper's context. Both
// raise SQLSTATE 55000, returned as the *pgconn.PgError it is.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// ErrPythonValueError marks a stored value the Python method would have
// raised ValueError on while building its record.
var ErrPythonValueError = errors.New("repo: python would raise ValueError on this stored value")

// PairVersion is PairVersionRecord. Content and Nguon are `dict(value or {})`
// as JSON object text, keys in the order the Python dict holds them.
type PairVersion struct {
	Version    int64
	Content    json.RawMessage
	LyDo       *string
	Nguon      json.RawMessage
	AuthorType string
	SentAt     *time.Time
	SentBy     *string
}

// PairView is PairViewRecord.
type PairView struct {
	Version  int64
	PersonID string
	SeenAt   time.Time
}

// PairResponse is PairResponseRecord.
type PairResponse struct {
	Version   int64
	PersonID  string
	Kind      string
	CreatedAt time.Time
}

// PairKeep is PairKeepRecord.
type PairKeep struct {
	ID        string
	PersonID  string
	Line      string
	CreatedAt time.Time
}

// PairPaper is PairPaperRecord. Tuan is midnight UTC of the stored day.
type PairPaper struct {
	ID               string
	ContextID        string
	CycleID          *string
	IsTemporary      bool
	DraftOwnerID     string
	State            string
	CurrentVersion   int64
	Tuan             time.Time
	ExpiresAt        time.Time
	CreatedAt        time.Time
	DoneRecordedByID *string
	DoneRecordedAt   *time.Time
	OutingID         *string
	Versions         []PairVersion
	Views            []PairView
	Responses        []PairResponse
	Keeps            []PairKeep
}

// pairPaperColumns is `select(PairPaper)`: every column in table order,
// unlabelled.
const pairPaperColumns = `pair_papers.id, pair_papers.context_id, pair_papers.context_kind, pair_papers.cycle_id,
       pair_papers.is_temporary, pair_papers.draft_owner_id, pair_papers.state, pair_papers.current_version,
       pair_papers.tuan, pair_papers.done_recorded_by_id, pair_papers.done_recorded_at, pair_papers.created_at,
       pair_papers.expires_at`

// pairPaperLabelled is the same columns as `session.get(PairPaper, id)`
// labels them.
const pairPaperLabelled = `pair_papers.id AS pair_papers_id, pair_papers.context_id AS pair_papers_context_id,
       pair_papers.context_kind AS pair_papers_context_kind, pair_papers.cycle_id AS pair_papers_cycle_id,
       pair_papers.is_temporary AS pair_papers_is_temporary, pair_papers.draft_owner_id AS pair_papers_draft_owner_id,
       pair_papers.state AS pair_papers_state, pair_papers.current_version AS pair_papers_current_version,
       pair_papers.tuan AS pair_papers_tuan, pair_papers.done_recorded_by_id AS pair_papers_done_recorded_by_id,
       pair_papers.done_recorded_at AS pair_papers_done_recorded_at, pair_papers.created_at AS pair_papers_created_at,
       pair_papers.expires_at AS pair_papers_expires_at`

func scanPairPaper(row pgx.Row) (*PairPaper, error) {
	var p PairPaper
	err := row.Scan(&p.ID, &p.ContextID, new(string), &p.CycleID, &p.IsTemporary, &p.DraftOwnerID, &p.State,
		&p.CurrentVersion, &p.Tuan, &p.DoneRecordedByID, &p.DoneRecordedAt, &p.CreatedAt, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Tuan = calendarDay(p.Tuan)
	p.DoneRecordedAt, p.CreatedAt, p.ExpiresAt = utcOptional(p.DoneRecordedAt), p.CreatedAt.UTC(), p.ExpiresAt.UTC()
	return &p, nil
}

// pairPaperRecord is `_pair_paper_record(paper)` for a paper row already read.
//
// Statements, in Python's order: the versions by version, the views by
// (version, seen_at), the responses by (created_at, id), the kept lines by
// (created_at, id), then `session.get(PairPaperOuting, paper.id)`. A version
// whose content or nguon `dict(value or {})` refuses ends the record right
// there, after the versions statement, content before nguon.
func (r Repository) pairPaperRecord(ctx context.Context, p *PairPaper) (*PairPaper, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT pair_paper_versions.paper_id, pair_paper_versions.version, pair_paper_versions.content,
		        pair_paper_versions.ly_do, pair_paper_versions.nguon, pair_paper_versions.author_type,
		        pair_paper_versions.sent_at, pair_paper_versions.sent_by, pair_paper_versions.created_at
		   FROM pair_paper_versions
		  WHERE pair_paper_versions.paper_id = $1::UUID ORDER BY pair_paper_versions.version`, p.ID)
	if err != nil {
		return nil, err
	}
	p.Versions = []PairVersion{}
	for rows.Next() {
		var v PairVersion
		var content, nguon []byte
		if err := rows.Scan(new(string), &v.Version, &content, &v.LyDo, &nguon, &v.AuthorType, &v.SentAt, &v.SentBy,
			new(time.Time)); err != nil {
			rows.Close()
			return nil, err
		}
		if v.Content, err = pythonDictOrEmpty(content); err != nil {
			rows.Close()
			return nil, err
		}
		if v.Nguon, err = pythonDictOrEmpty(nguon); err != nil {
			rows.Close()
			return nil, err
		}
		v.SentAt = utcOptional(v.SentAt)
		p.Versions = append(p.Versions, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT pair_paper_views.paper_id, pair_paper_views.version, pair_paper_views.person_id, pair_paper_views.seen_at
		   FROM pair_paper_views
		  WHERE pair_paper_views.paper_id = $1::UUID ORDER BY pair_paper_views.version, pair_paper_views.seen_at`, p.ID)
	if err != nil {
		return nil, err
	}
	p.Views, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (PairView, error) {
		var v PairView
		err := row.Scan(new(string), &v.Version, &v.PersonID, &v.SeenAt)
		v.SeenAt = v.SeenAt.UTC()
		return v, err
	})
	if err != nil {
		return nil, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT pair_paper_responses.id, pair_paper_responses.paper_id, pair_paper_responses.version,
		        pair_paper_responses.person_id, pair_paper_responses.kind, pair_paper_responses.created_at
		   FROM pair_paper_responses
		  WHERE pair_paper_responses.paper_id = $1::UUID
		  ORDER BY pair_paper_responses.created_at, pair_paper_responses.id`, p.ID)
	if err != nil {
		return nil, err
	}
	p.Responses, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (PairResponse, error) {
		var v PairResponse
		err := row.Scan(new(string), new(string), &v.Version, &v.PersonID, &v.Kind, &v.CreatedAt)
		v.CreatedAt = v.CreatedAt.UTC()
		return v, err
	})
	if err != nil {
		return nil, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT pair_paper_keeps.id, pair_paper_keeps.paper_id, pair_paper_keeps.person_id, pair_paper_keeps.line,
		        pair_paper_keeps.created_at
		   FROM pair_paper_keeps
		  WHERE pair_paper_keeps.paper_id = $1::UUID ORDER BY pair_paper_keeps.created_at, pair_paper_keeps.id`, p.ID)
	if err != nil {
		return nil, err
	}
	p.Keeps, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (PairKeep, error) {
		var v PairKeep
		err := row.Scan(&v.ID, new(string), &v.PersonID, &v.Line, &v.CreatedAt)
		v.CreatedAt = v.CreatedAt.UTC()
		return v, err
	})
	if err != nil {
		return nil, err
	}

	if p.OutingID, err = r.GetPaperOuting(ctx, p.ID); err != nil {
		return nil, err
	}
	return p, nil
}

// PairPaperInput is create_pair_paper's keyword arguments. Content and Nguon
// are JSON objects; Tuan is a calendar day.
type PairPaperInput struct {
	ContextID    string
	CycleID      *string
	DraftOwnerID string
	Tuan         time.Time
	ExpiresAt    time.Time
	Content      json.RawMessage
	LyDo         *string
	Nguon        json.RawMessage
	AuthorType   string
	Now          time.Time
}

// CreatePairPaper is create_pair_paper.
//
// Statements, in Python's order:
//  1. the paper INSERT of every column (the psycopg dialect renders no bind
//     cast for the Boolean is_temporary): context_kind 'pair', is_temporary
//     true exactly when there is no cycle, state 'nhap', current_version 1,
//     the done columns as explicit NULLs, created_at the caller's clock;
//  2. version 1's INSERT (sent_at and sent_by explicit NULLs);
//  3. pairPaperRecord's reads. The paper fields of the record are what was
//     written; its versions and the rest are read back.
//
// A second open sheet in the context fails on uq_pair_papers_open_per_context
// at statement 1; a group context on fk_pair_papers_context.
func (r Repository) CreatePairPaper(ctx context.Context, in PairPaperInput) (PairPaper, error) {
	id, err := newUUID()
	if err != nil {
		return PairPaper{}, err
	}
	created, expires, tuan := pythonInstant(in.Now), pythonInstant(in.ExpiresAt), calendarDay(in.Tuan)
	var cycle any
	if in.CycleID != nil {
		cycle = *in.CycleID
	}
	if _, err := r.Q.Exec(ctx, renderInsert("pair_papers", []insertColumn{{"id", "::UUID"}, {"context_id", "::UUID"},
		{"context_kind", "::VARCHAR"}, {"cycle_id", "::UUID"}, {"is_temporary", ""}, {"draft_owner_id", "::UUID"},
		{"state", "::VARCHAR"}, {"current_version", "::INTEGER"}, {"tuan", "::DATE"}, {"done_recorded_by_id", "::UUID"},
		{"done_recorded_at", "::TIMESTAMP WITH TIME ZONE"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"},
		{"expires_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		id, in.ContextID, "pair", cycle, in.CycleID == nil, in.DraftOwnerID, "nhap", sqlInteger(1), tuan, nil, nil,
		created, expires); err != nil {
		return PairPaper{}, err
	}
	if err := r.insertVersion(ctx, PaperVersionInput{PaperID: id, Version: 1, Content: in.Content, LyDo: in.LyDo,
		Nguon: in.Nguon, AuthorType: in.AuthorType, Now: in.Now}); err != nil {
		return PairPaper{}, err
	}
	paper := &PairPaper{ID: id, ContextID: in.ContextID, CycleID: in.CycleID, IsTemporary: in.CycleID == nil,
		DraftOwnerID: in.DraftOwnerID, State: "nhap", CurrentVersion: 1, Tuan: tuan, ExpiresAt: expires.UTC(),
		CreatedAt: created.UTC()}
	record, err := r.pairPaperRecord(ctx, paper)
	if err != nil {
		return PairPaper{}, err
	}
	return *record, nil
}

// GetPairPaper is get_pair_paper: `session.get(PairPaper, id)`, nil without
// another statement when there is none, then pairPaperRecord.
func (r Repository) GetPairPaper(ctx context.Context, paperID string) (*PairPaper, error) {
	return r.readPairPaper(ctx, paperID, "")
}

// LockPairPaper is lock_pair_paper: GetPairPaper with FOR UPDATE on the paper
// statement only (with_for_update bypasses the identity map, so it is always
// issued). The record reads after it take no lock.
func (r Repository) LockPairPaper(ctx context.Context, paperID string) (*PairPaper, error) {
	return r.readPairPaper(ctx, paperID, " FOR UPDATE")
}

func (r Repository) readPairPaper(ctx context.Context, paperID, suffix string) (*PairPaper, error) {
	paper, err := scanPairPaper(r.Q.QueryRow(ctx, `SELECT `+pairPaperLabelled+`
		   FROM pair_papers
		  WHERE pair_papers.id = $1::UUID`+suffix, paperID))
	if err != nil || paper == nil {
		return nil, err
	}
	return r.pairPaperRecord(ctx, paper)
}

// ListPairPapers is list_pair_papers: every paper of the context, newest
// first with the id breaking ties, all read before the first record; then
// each paper's pairPaperRecord in that order.
func (r Repository) ListPairPapers(ctx context.Context, contextID string) ([]PairPaper, error) {
	rows, err := r.Q.Query(ctx, `SELECT `+pairPaperColumns+`
		   FROM pair_papers
		  WHERE pair_papers.context_id = $1::UUID ORDER BY pair_papers.created_at DESC, pair_papers.id`, contextID)
	if err != nil {
		return nil, err
	}
	papers := []*PairPaper{}
	for rows.Next() {
		paper, err := scanPairPaper(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		papers = append(papers, paper)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []PairPaper{}
	for _, paper := range papers {
		record, err := r.pairPaperRecord(ctx, paper)
		if err != nil {
			return nil, err
		}
		out = append(out, *record)
	}
	return out, nil
}

// pairVersionKey is the primary key of a version, as `session.get` binds it.
func pairVersionKey(paperID string, version int64) []column {
	return []column{{"paper_id", "::UUID", paperID}, {"version", "::INTEGER", sqlInteger(version)}}
}

// pairVersionRow is the PairPaperVersion columns the writers compare.
type pairVersionRow struct {
	content []byte
	lyDo    *string
	sentAt  *time.Time
	sentBy  *string
}

// getPairVersion is `session.get(PairPaperVersion, (paper_id, version))`.
func (r Repository) getPairVersion(ctx context.Context, paperID string, version int64) (*pairVersionRow, error) {
	var v pairVersionRow
	err := r.Q.QueryRow(ctx,
		`SELECT pair_paper_versions.paper_id AS pair_paper_versions_paper_id,
		        pair_paper_versions.version AS pair_paper_versions_version,
		        pair_paper_versions.content AS pair_paper_versions_content,
		        pair_paper_versions.ly_do AS pair_paper_versions_ly_do,
		        pair_paper_versions.nguon AS pair_paper_versions_nguon,
		        pair_paper_versions.author_type AS pair_paper_versions_author_type,
		        pair_paper_versions.sent_at AS pair_paper_versions_sent_at,
		        pair_paper_versions.sent_by AS pair_paper_versions_sent_by,
		        pair_paper_versions.created_at AS pair_paper_versions_created_at
		   FROM pair_paper_versions
		  WHERE pair_paper_versions.paper_id = $1::UUID AND pair_paper_versions.version = $2::INTEGER`,
		paperID, sqlInteger(version)).
		Scan(new(string), new(int64), &v.content, &v.lyDo, new([]byte), new(string), &v.sentAt, &v.sentBy, new(time.Time))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpdatePairDraft is update_pair_draft: version 1 by key; nothing more when
// there is none; otherwise content (when not equal as Python dicts) and ly_do
// (when it changes) in one UPDATE, none when neither does. A sent version is
// refused by pair_paper_versions_immutable (55000) even when the UPDATE would
// write the stored values back, because the unit of work still sends it when
// one column differs.
func (r Repository) UpdatePairDraft(ctx context.Context, paperID string, content json.RawMessage, lyDo *string) error {
	stored, err := r.getPairVersion(ctx, paperID, 1)
	if err != nil || stored == nil {
		return err
	}
	var sets []column
	if !pythonJSONEqual(stored.content, content) {
		sets = append(sets, column{"content", "::JSONB", string(content)})
	}
	if !sameText(stored.lyDo, lyDo) {
		sets = append(sets, column{"ly_do", "::VARCHAR", optionalText(lyDo)})
	}
	return r.updateRow(ctx, "pair_paper_versions", sets, pairVersionKey(paperID, 1))
}

// PaperVersionInput is add_paper_version's keyword arguments.
type PaperVersionInput struct {
	PaperID    string
	Version    int64
	Content    json.RawMessage
	LyDo       *string
	Nguon      json.RawMessage
	AuthorType string
	SentAt     *time.Time
	SentBy     *string
	Now        time.Time
}

func (r Repository) insertVersion(ctx context.Context, in PaperVersionInput) error {
	var sentAt any
	if in.SentAt != nil {
		sentAt = pythonInstant(*in.SentAt)
	}
	_, err := r.Q.Exec(ctx, renderInsert("pair_paper_versions", []insertColumn{{"paper_id", "::UUID"},
		{"version", "::INTEGER"}, {"content", "::JSONB"}, {"ly_do", "::VARCHAR"}, {"nguon", "::JSONB"},
		{"author_type", "::VARCHAR"}, {"sent_at", "::TIMESTAMP WITH TIME ZONE"}, {"sent_by", "::UUID"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		in.PaperID, sqlInteger(in.Version), string(in.Content), optionalText(in.LyDo), string(in.Nguon), in.AuthorType,
		sentAt, optionalText(in.SentBy), pythonInstant(in.Now))
	return err
}

// AddPaperVersion is add_paper_version: one INSERT of every column. A
// version number already taken fails on pair_paper_versions_pkey, a missing
// paper on fk_pair_paper_versions_paper, and version_sent_by_human_has_sender
// refuses a human version sent by nobody or sent_at without author.
func (r Repository) AddPaperVersion(ctx context.Context, in PaperVersionInput) error {
	return r.insertVersion(ctx, in)
}

// MarkVersionSent is mark_version_sent: the version by key; nothing more when
// there is none or it was sent; otherwise sent_at now and sent_by (when it
// changes) in one UPDATE.
func (r Repository) MarkVersionSent(ctx context.Context, paperID string, version int64, sentBy *string, now time.Time) error {
	stored, err := r.getPairVersion(ctx, paperID, version)
	if err != nil || stored == nil || stored.sentAt != nil {
		return err
	}
	sets := []column{{"sent_at", "::TIMESTAMP WITH TIME ZONE", pythonInstant(now)}}
	if !sameText(stored.sentBy, sentBy) {
		sets = append(sets, column{"sent_by", "::UUID", optionalText(sentBy)})
	}
	return r.updateRow(ctx, "pair_paper_versions", sets, pairVersionKey(paperID, version))
}

// PaperStateInput is set_paper_state's arguments. CurrentVersion nil is the
// keyword left out; RecordedByID is read only for state "da_di".
type PaperStateInput struct {
	PaperID        string
	State          string
	Now            time.Time
	CurrentVersion *int64
	RecordedByID   *string
}

// SetPaperState is set_paper_state: `session.get(PairPaper, id)` (no lock);
// nothing more when there is none; otherwise state, current_version when
// given, and for "da_di" done_recorded_by_id and done_recorded_at now, each
// only when it changes, in one UPDATE (none when nothing does).
// paper_done_has_recorder refuses "da_di" without a recorder.
func (r Repository) SetPaperState(ctx context.Context, in PaperStateInput) error {
	stored, err := scanPairPaper(r.Q.QueryRow(ctx, `SELECT `+pairPaperLabelled+`
		   FROM pair_papers
		  WHERE pair_papers.id = $1::UUID`, in.PaperID))
	if err != nil || stored == nil {
		return err
	}
	var sets []column
	if stored.State != in.State {
		sets = append(sets, column{"state", "::VARCHAR", in.State})
	}
	if in.CurrentVersion != nil && *in.CurrentVersion != stored.CurrentVersion {
		sets = append(sets, column{"current_version", "::INTEGER", sqlInteger(*in.CurrentVersion)})
	}
	if in.State == "da_di" {
		if !sameText(stored.DoneRecordedByID, in.RecordedByID) {
			sets = append(sets, column{"done_recorded_by_id", "::UUID", optionalText(in.RecordedByID)})
		}
		recorded := pythonInstant(in.Now)
		if !sameInstant(stored.DoneRecordedAt, &recorded) {
			sets = append(sets, column{"done_recorded_at", "::TIMESTAMP WITH TIME ZONE", recorded})
		}
	}
	return r.updateRow(ctx, "pair_papers", sets, []column{{"id", "::UUID", in.PaperID}})
}

// MarkPaperViewed is mark_paper_viewed: the view by (paper, version, person);
// its stored seen_at when there is one, otherwise an INSERT and now.
func (r Repository) MarkPaperViewed(ctx context.Context, paperID string, version int64, personID string, now time.Time) (time.Time, error) {
	var seen time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT pair_paper_views.paper_id AS pair_paper_views_paper_id,
		        pair_paper_views.version AS pair_paper_views_version,
		        pair_paper_views.person_id AS pair_paper_views_person_id,
		        pair_paper_views.seen_at AS pair_paper_views_seen_at
		   FROM pair_paper_views
		  WHERE pair_paper_views.paper_id = $1::UUID AND pair_paper_views.version = $2::INTEGER
		    AND pair_paper_views.person_id = $3::UUID`, paperID, sqlInteger(version), personID).
		Scan(new(string), new(int64), new(string), &seen)
	switch {
	case err == nil:
		return seen.UTC(), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, err
	}
	seen = pythonInstant(now)
	if _, err := r.Q.Exec(ctx, renderInsert("pair_paper_views", []insertColumn{{"paper_id", "::UUID"},
		{"version", "::INTEGER"}, {"person_id", "::UUID"}, {"seen_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		paperID, sqlInteger(version), personID, seen); err != nil {
		return time.Time{}, err
	}
	return seen.UTC(), nil
}

// PaperResponseInput is add_paper_response's keyword arguments.
type PaperResponseInput struct {
	PaperID  string
	Version  int64
	PersonID string
	Kind     string
	Now      time.Time
}

// AddPaperResponse is add_paper_response: for "dong_y" only, this person's
// agreement to this version first (`scalars(...).first()`, no LIMIT), and
// Conflict paper_already_agreed raised from nothing when there is one; then
// the INSERT. response_kind_known and pair_paper_response_is_participant
// refuse in PostgreSQL.
func (r Repository) AddPaperResponse(ctx context.Context, in PaperResponseInput) error {
	if in.Kind == "dong_y" {
		var existing string
		err := r.Q.QueryRow(ctx,
			`SELECT pair_paper_responses.id, pair_paper_responses.paper_id, pair_paper_responses.version,
			        pair_paper_responses.person_id, pair_paper_responses.kind, pair_paper_responses.created_at
			   FROM pair_paper_responses
			  WHERE pair_paper_responses.paper_id = $1::UUID AND pair_paper_responses.version = $2::INTEGER
			    AND pair_paper_responses.person_id = $3::UUID AND pair_paper_responses.kind = $4::VARCHAR`,
			in.PaperID, sqlInteger(in.Version), in.PersonID, "dong_y").
			Scan(&existing, new(string), new(int64), new(string), new(string), new(time.Time))
		switch {
		case err == nil:
			return &Conflict{Code: "paper_already_agreed"}
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	_, err = r.Q.Exec(ctx, renderInsert("pair_paper_responses", []insertColumn{{"id", "::UUID"}, {"paper_id", "::UUID"},
		{"version", "::INTEGER"}, {"person_id", "::UUID"}, {"kind", "::VARCHAR"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		id, in.PaperID, sqlInteger(in.Version), in.PersonID, in.Kind, pythonInstant(in.Now))
	return err
}

// LinkPaperOuting is link_paper_outing: the paper's link; Conflict
// paper_outing_exists raised from nothing when there is one, otherwise the
// INSERT. uq_pair_paper_outings_outing refuses an outing already linked to
// another paper; pair_paper_outing_link_valid checks the rest at COMMIT.
func (r Repository) LinkPaperOuting(ctx context.Context, paperID string, version int64, outingID string, now time.Time) error {
	existing, err := r.GetPaperOuting(ctx, paperID)
	if err != nil {
		return err
	}
	if existing != nil {
		return &Conflict{Code: "paper_outing_exists"}
	}
	_, err = r.Q.Exec(ctx, renderInsert("pair_paper_outings", []insertColumn{{"paper_id", "::UUID"},
		{"version", "::INTEGER"}, {"outing_id", "::UUID"}, {"linked_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		paperID, sqlInteger(version), outingID, pythonInstant(now))
	return err
}

// GetPaperOuting is get_paper_outing: `session.get(PairPaperOuting, paper_id)`
// and its outing id, nil when there is none.
func (r Repository) GetPaperOuting(ctx context.Context, paperID string) (*string, error) {
	var outing string
	err := r.Q.QueryRow(ctx,
		`SELECT pair_paper_outings.paper_id AS pair_paper_outings_paper_id,
		        pair_paper_outings.version AS pair_paper_outings_version,
		        pair_paper_outings.outing_id AS pair_paper_outings_outing_id,
		        pair_paper_outings.linked_at AS pair_paper_outings_linked_at
		   FROM pair_paper_outings
		  WHERE pair_paper_outings.paper_id = $1::UUID`, paperID).
		Scan(new(string), new(int64), &outing, new(time.Time))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &outing, nil
}

// AddPaperKeep is add_paper_keep: one INSERT, and the record of what was
// written. keep_line_not_blank refuses a line with no non-space character.
func (r Repository) AddPaperKeep(ctx context.Context, paperID, personID, line string, now time.Time) (PairKeep, error) {
	id, err := newUUID()
	if err != nil {
		return PairKeep{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx, renderInsert("pair_paper_keeps", []insertColumn{{"id", "::UUID"}, {"paper_id", "::UUID"},
		{"person_id", "::UUID"}, {"line", "::VARCHAR"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		id, paperID, personID, line, created); err != nil {
		return PairKeep{}, err
	}
	return PairKeep{ID: id, PersonID: personID, Line: line, CreatedAt: created.UTC()}, nil
}

// PairPaperCloseCounts is close_open_pair_papers' dict, keys "bo" then "huy".
type PairPaperCloseCounts struct {
	Bo, Huy int64
}

// pairPaperOpenStates is the tuple close_open_pair_papers passes to in_(),
// in its order.
var pairPaperOpenStates = []string{"nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"}

// CloseOpenPairPapers is close_open_pair_papers: every paper of the context
// in an open state (the stored state: an expired sheet is still closed), no
// ORDER BY, locked FOR UPDATE; then state 'bo' for a draft and 'huy' for the
// rest, as one executemany in primary key order (none for no paper).
func (r Repository) CloseOpenPairPapers(ctx context.Context, contextID string, now time.Time) (PairPaperCloseCounts, error) {
	args := []any{contextID}
	for _, state := range pairPaperOpenStates {
		args = append(args, state)
	}
	rows, err := r.Q.Query(ctx, `SELECT `+pairPaperColumns+`
		   FROM pair_papers
		  WHERE pair_papers.context_id = $1::UUID AND pair_papers.state IN (`+
		varcharPlaceholders(2, len(pairPaperOpenStates))+`) FOR UPDATE`, args...)
	if err != nil {
		return PairPaperCloseCounts{}, err
	}
	next := map[string]string{}
	var counts PairPaperCloseCounts
	for rows.Next() {
		paper, err := scanPairPaper(rows)
		if err != nil {
			rows.Close()
			return PairPaperCloseCounts{}, err
		}
		if paper.State == "nhap" {
			next[paper.ID] = "bo"
			counts.Bo++
		} else {
			next[paper.ID] = "huy"
			counts.Huy++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return PairPaperCloseCounts{}, err
	}
	ids := make([]string, 0, len(next))
	for id := range next {
		ids = append(ids, id)
	}
	sortStrings(ids)
	for _, id := range ids {
		if err := r.updateRow(ctx, "pair_papers", []column{{"state", "::VARCHAR", next[id]}},
			[]column{{"id", "::UUID", id}}); err != nil {
			return PairPaperCloseCounts{}, err
		}
	}
	return counts, nil
}

// ---------------------------------------------------------------------------
// Python dict() and == over stored JSON
// ---------------------------------------------------------------------------

// pythonDictOrEmpty is `dict(value or {})` over json.loads(raw), as JSON
// object text. Python's truthiness and dict() decide: null, false, 0, "", []
// and {} are empty; an object is itself; true and any other number raise
// TypeError; a non-empty string raises ValueError; a list builds a dict from
// its elements, each of which must be a two-item sequence (a list of two, a
// string of two characters, an object with two keys) whose first item is the
// key, later keys overwriting earlier values in place. An element that is not
// a sequence, or a key that is a list or an object, is TypeError; a sequence
// of another length is ValueError. A key that is a number, true, false or null
// is valid Python but cannot be a JSON key: ErrUnrepresentable.
func pythonDictOrEmpty(raw []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	empty := json.RawMessage(`{}`)
	if len(trimmed) == 0 {
		return empty, nil
	}
	switch trimmed[0] {
	case 'n', 'f':
		return empty, nil
	case 't':
		return nil, ErrPythonTypeError
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return nil, err
		}
		if s == "" {
			return empty, nil
		}
		return nil, ErrPythonValueError
	case '{':
		keys, err := jsonObjectKeys(trimmed)
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			return empty, nil
		}
		return json.RawMessage(append([]byte{}, trimmed...)), nil
	case '[':
		var elements []json.RawMessage
		if err := json.Unmarshal(trimmed, &elements); err != nil {
			return nil, err
		}
		if len(elements) == 0 {
			return empty, nil
		}
		return pythonDictFromPairs(elements)
	}
	if jsonNumberIsZero(string(trimmed)) {
		return empty, nil
	}
	return nil, ErrPythonTypeError
}

func pythonDictFromPairs(elements []json.RawMessage) (json.RawMessage, error) {
	var order []string
	values := map[string]json.RawMessage{}
	for _, element := range elements {
		element = bytes.TrimSpace(element)
		var key string
		var value json.RawMessage
		switch element[0] {
		case '"':
			var s string
			if err := json.Unmarshal(element, &s); err != nil {
				return nil, err
			}
			if utf8.RuneCountInString(s) != 2 {
				return nil, ErrPythonValueError
			}
			first, size := utf8.DecodeRuneInString(s)
			encoded, _ := json.Marshal(s[size:])
			key, value = string(first), encoded
		case '{':
			keys, err := jsonObjectKeys(element)
			if err != nil {
				return nil, err
			}
			if len(keys) != 2 {
				return nil, ErrPythonValueError
			}
			encoded, _ := json.Marshal(keys[1])
			key, value = keys[0], encoded
		case '[':
			var items []json.RawMessage
			if err := json.Unmarshal(element, &items); err != nil {
				return nil, err
			}
			if len(items) != 2 {
				return nil, ErrPythonValueError
			}
			head := bytes.TrimSpace(items[0])
			switch head[0] {
			case '[', '{':
				return nil, ErrPythonTypeError
			case '"':
				if err := json.Unmarshal(head, &key); err != nil {
					return nil, err
				}
			default:
				return nil, ErrUnrepresentable
			}
			value = bytes.TrimSpace(items[1])
		default:
			return nil, ErrPythonTypeError
		}
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = value
	}
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range order {
		if i > 0 {
			b.WriteString(", ")
		}
		encoded, _ := json.Marshal(key)
		b.Write(encoded)
		b.WriteString(": ")
		b.Write(values[key])
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// jsonObjectKeys is the keys of one JSON object text, in order.
func jsonObjectKeys(object []byte) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(object))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		keys = append(keys, token.(string))
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// pythonJSONEqual is Python == between json.loads(a) and json.loads(b):
// objects by key set and values, lists item by item, numbers by exact value
// across int and float, true and false equal to 1 and 0.
func pythonJSONEqual(a, b []byte) bool {
	decode := func(raw []byte) (any, bool) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var v any
		if err := decoder.Decode(&v); err != nil {
			return nil, false
		}
		return v, true
	}
	x, okX := decode(a)
	y, okY := decode(b)
	return okX && okY && pythonValueEqual(x, y)
}

func pythonValueEqual(x, y any) bool {
	switch a := x.(type) {
	case nil:
		return y == nil
	case string:
		b, ok := y.(string)
		return ok && a == b
	case []any:
		b, ok := y.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i := range a {
			if !pythonValueEqual(a[i], b[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		b, ok := y.(map[string]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for k, v := range a {
			w, present := b[k]
			if !present || !pythonValueEqual(v, w) {
				return false
			}
		}
		return true
	}
	p, okP := pythonNumber(x)
	q, okQ := pythonNumber(y)
	return okP && okQ && p.equal(q)
}

// number is a json.loads number or bool as Python compares it: an exact
// rational, or an infinity.
type number struct {
	rat *big.Rat
	inf int
}

func (n number) equal(m number) bool {
	if n.inf != 0 || m.inf != 0 {
		return n.inf == m.inf
	}
	return n.rat.Cmp(m.rat) == 0
}

func pythonNumber(v any) (number, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return number{rat: big.NewRat(1, 1)}, true
		}
		return number{rat: new(big.Rat)}, true
	case json.Number:
		literal := x.String()
		if strings.ContainsAny(literal, ".eE") {
			f, _ := strconv.ParseFloat(literal, 64)
			if math.IsInf(f, 0) {
				if f > 0 {
					return number{inf: 1}, true
				}
				return number{inf: -1}, true
			}
			return number{rat: new(big.Rat).SetFloat64(f)}, true
		}
		n, ok := new(big.Rat).SetString(literal)
		return number{rat: n}, ok
	}
	return number{}, false
}
