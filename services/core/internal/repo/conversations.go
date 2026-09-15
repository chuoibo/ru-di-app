package repo

// One person's conversation list and the two-person conversation (ADR-0021
// §2.5): get_pair_context, create_pair_context,
// list_person_context_summaries and count_unread_messages of
// SqlAlchemyApiRepository.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/domain/direct"
)

// pairContextSavepoint is the name SQLAlchemy gives a session's first begin_nested.
const pairContextSavepoint = "sa_savepoint_1"

// contextColumns is `select(Context)`: every mapped column in declaration
// order, unlabelled.
const contextColumns = `contexts.id, contexts.display_name, contexts.created_by_id, contexts.theme, contexts.kind,
       contexts.pair_key, contexts.created_at`

func scanContextRecord(row pgx.Row) (*ContextRecord, error) {
	var c ContextRecord
	err := row.Scan(&c.ID, &c.DisplayName, &c.CreatedByID, &c.Theme, &c.Kind, &c.PairKey, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return &c, nil
}

// GetPairContext is get_pair_context: `session.scalar` of the context with
// this pair_key (no LIMIT; uq_contexts_pair_key keeps it to one row).
func (r Repository) GetPairContext(ctx context.Context, pairKey string) (*ContextRecord, error) {
	return scanContextRecord(r.Q.QueryRow(ctx,
		`SELECT `+contextColumns+`
		   FROM contexts
		  WHERE contexts.pair_key = $1::VARCHAR`, pairKey))
}

// PairContextInput is create_pair_context's keyword arguments.
type PairContextInput struct {
	PairKey     string
	MemberIDs   []string
	CreatedByID string
	Now         time.Time
}

var pairMembershipColumns = []insertColumn{{"id", "::UUID"}, {"context_id", "::UUID"}, {"person_id", "::UUID"},
	{"state", ""}, {"role", ""}, {"origin", ""}, {"invited_by_id", "::UUID"},
	{"joined_at", "::TIMESTAMP WITH TIME ZONE"}, {"left_at", "::TIMESTAMP WITH TIME ZONE"}}

// CreatePairContext is create_pair_context: a pair and its ACTIVE
// memberships, one savepoint.
//
// Statements, in Python's order:
//  1. SAVEPOINT;
//  2. the first flush: the context INSERT (client-side uuid4, display_name "",
//     kind "pair"), theme and created_at read back with RETURNING;
//  3. the second flush: every membership in one multi-row INSERT (uuid4 ids,
//     role member, origin named, invited_by the creator, joined_at now,
//     left_at NULL), created_at read back with RETURNING; nothing for no
//     members;
//  4. RELEASE SAVEPOINT.
//
// Any failure is ROLLBACK TO SAVEPOINT first. A unique violation of
// uq_contexts_pair_key is Conflict PAIR_EXISTS; every other error, integrity
// violations included, is returned as is.
func (r Repository) CreatePairContext(ctx context.Context, in PairContextInput) (ContextRecord, error) {
	id, err := newUUID()
	if err != nil {
		return ContextRecord{}, err
	}
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+pairContextSavepoint); err != nil {
		return ContextRecord{}, err
	}
	failed := func(cause error) (ContextRecord, error) {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+pairContextSavepoint); err != nil {
			return ContextRecord{}, err
		}
		if pg := integrityViolation(cause); pg != nil && pg.ConstraintName == "uq_contexts_pair_key" {
			return ContextRecord{}, &Conflict{Code: "PAIR_EXISTS", Err: pg}
		}
		return ContextRecord{}, cause
	}
	key := in.PairKey
	c := ContextRecord{ID: id, CreatedByID: in.CreatedByID, Kind: "pair", PairKey: &key}
	if err := r.Q.QueryRow(ctx,
		`INSERT INTO contexts (id, display_name, created_by_id, kind, pair_key)
		 VALUES ($1::UUID, $2::VARCHAR, $3::UUID, $4::VARCHAR, $5::VARCHAR)
		 RETURNING contexts.theme, contexts.created_at`,
		id, "", in.CreatedByID, "pair", in.PairKey).Scan(&c.Theme, &c.CreatedAt); err != nil {
		return failed(err)
	}
	if len(in.MemberIDs) > 0 {
		joined := pythonInstant(in.Now)
		args := make([]any, 0, len(in.MemberIDs)*len(pairMembershipColumns))
		for _, person := range in.MemberIDs {
			membership, err := newUUID()
			if err != nil {
				return ContextRecord{}, err
			}
			args = append(args, membership, id, person, "active", "member", "named", in.CreatedByID, joined, nil)
		}
		rows, err := r.Q.Query(ctx, renderInsert("memberships", pairMembershipColumns, len(in.MemberIDs))+
			` RETURNING memberships.created_at, memberships.id`, args...)
		if err != nil {
			return failed(err)
		}
		for rows.Next() {
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return failed(err)
		}
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+pairContextSavepoint); err != nil {
		return ContextRecord{}, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return c, nil
}

// LastMessage is LastMessageRecord.
type LastMessage struct {
	ID                string
	Kind              string
	Preview           string
	AuthorID          *string
	AuthorDisplayName *string
	CreatedAt         time.Time
}

// PersonContextSummary is PersonContextSummaryRecord.
type PersonContextSummary struct {
	ID                     string
	DisplayName            string
	MemberCount            int64
	MyRole                 string
	MyState                string
	MembershipID           string
	JoinedAt               *time.Time
	LastMessage            *LastMessage
	UnreadCount            int64
	Theme                  string
	Kind                   string
	CounterpartID          *string
	CounterpartDisplayName *string
}

// ListPersonContextSummaries is list_person_context_summaries.
//
// Statements, in Python's order:
//  1. the person's memberships whose state is not left, joined to their
//     contexts (no ORDER BY); none answers an empty list with no other
//     statement;
//  2. the ACTIVE head-count of those contexts, one IN parameter per row;
//  3. the newest message of each (DISTINCT ON, created_at then id descending);
//  4. the authors' names, one IN parameter per distinct author, only when some
//     newest message has one;
//  5. the other members (state not left) of the pairs among them, only when
//     there is a pair;
//  6. those members' names, only when some pair has one;
//  7. for each row of 1, in that order, CountUnreadMessages.
//
// The previews are built between 6 and 7, so a card that makes
// `_message_preview` raise stops the list before any unread count.
// An author or counterpart name is the stored one, empty included, and absent
// (nil) only when the people row is missing. The answer is sorted as Python
// sorts it: conversations with a message first, newest first (by the float
// `timestamp()`), then by display name; Python's sort is stable, so rows tied
// on all three keep the order of statement 1.
func (r Repository) ListPersonContextSummaries(ctx context.Context, personID string) ([]PersonContextSummary, error) {
	type joined struct {
		m Membership
		c ContextRecord
	}
	rows, err := r.Q.Query(ctx,
		`SELECT memberships.id, memberships.context_id, memberships.person_id, memberships.state, memberships.role,
		        memberships.origin, memberships.invited_by_id, memberships.joined_at, memberships.left_at,
		        memberships.created_at, contexts.id AS id_1, contexts.display_name, contexts.created_by_id,
		        contexts.theme, contexts.kind, contexts.pair_key, contexts.created_at AS created_at_1
		   FROM memberships JOIN contexts ON contexts.id = memberships.context_id
		  WHERE memberships.person_id = $1::UUID AND memberships.state != $2`, personID, "left")
	if err != nil {
		return nil, err
	}
	var list []joined
	for rows.Next() {
		var j joined
		if err := rows.Scan(&j.m.ID, &j.m.ContextID, &j.m.PersonID, &j.m.State, &j.m.Role, &j.m.Origin,
			&j.m.InvitedByID, &j.m.JoinedAt, &j.m.LeftAt, &j.m.CreatedAt, &j.c.ID, &j.c.DisplayName, &j.c.CreatedByID,
			&j.c.Theme, &j.c.Kind, &j.c.PairKey, &j.c.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		j.m.JoinedAt = utcOptional(j.m.JoinedAt)
		list = append(list, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return []PersonContextSummary{}, nil
	}

	contextIDs := make([]string, len(list))
	var pairIDs []string
	for i, j := range list {
		contextIDs[i] = j.c.ID
		if j.c.Kind == "pair" {
			pairIDs = append(pairIDs, j.c.ID)
		}
	}
	n := len(contextIDs)

	counts := map[string]int64{}
	countRows, err := r.Q.Query(ctx,
		`SELECT memberships.context_id, count(*) AS count_1
		   FROM memberships
		  WHERE memberships.context_id IN (`+uuidPlaceholders(1, n)+`) AND memberships.state = $`+strconv.Itoa(n+1)+`
		  GROUP BY memberships.context_id`, append(uuidArgs(contextIDs), "active")...)
	if err != nil {
		return nil, err
	}
	for countRows.Next() {
		var id string
		var count int64
		if err := countRows.Scan(&id, &count); err != nil {
			countRows.Close()
			return nil, err
		}
		counts[id] = count
	}
	countRows.Close()
	if err := countRows.Err(); err != nil {
		return nil, err
	}

	type newestRow struct {
		id, contextID, kind string
		authorID, body      *string
		card                []byte
		createdAt           time.Time
	}
	var newest []newestRow
	messageRows, err := r.Q.Query(ctx,
		`SELECT DISTINCT ON (messages.context_id) messages.id, messages.context_id, messages.author_id, messages.kind,
		        messages.body, messages.image_url, messages.card, messages.reply_to_id, messages.deleted_at,
		        messages.created_at
		   FROM messages
		  WHERE messages.context_id IN (`+uuidPlaceholders(1, n)+`)
		  ORDER BY messages.context_id, messages.created_at DESC, messages.id DESC`, uuidArgs(contextIDs)...)
	if err != nil {
		return nil, err
	}
	for messageRows.Next() {
		var m newestRow
		var imageURL, replyTo *string
		var deletedAt *time.Time
		if err := messageRows.Scan(&m.id, &m.contextID, &m.authorID, &m.kind, &m.body, &imageURL, &m.card, &replyTo,
			&deletedAt, &m.createdAt); err != nil {
			messageRows.Close()
			return nil, err
		}
		m.createdAt = m.createdAt.UTC()
		newest = append(newest, m)
	}
	messageRows.Close()
	if err := messageRows.Err(); err != nil {
		return nil, err
	}

	var authors []string
	for _, m := range newest {
		if m.authorID != nil {
			authors = append(authors, *m.authorID)
		}
	}
	names, err := r.storedNames(ctx, authors)
	if err != nil {
		return nil, err
	}

	counterparts := map[string]string{}
	if len(pairIDs) > 0 {
		k := len(pairIDs)
		pairRows, err := r.Q.Query(ctx,
			`SELECT memberships.context_id, memberships.person_id
			   FROM memberships
			  WHERE memberships.context_id IN (`+uuidPlaceholders(1, k)+`)
			    AND memberships.person_id != $`+strconv.Itoa(k+1)+`::UUID AND memberships.state != $`+strconv.Itoa(k+2),
			append(uuidArgs(pairIDs), personID, "left")...)
		if err != nil {
			return nil, err
		}
		for pairRows.Next() {
			var contextID, other string
			if err := pairRows.Scan(&contextID, &other); err != nil {
				pairRows.Close()
				return nil, err
			}
			counterparts[contextID] = other
		}
		pairRows.Close()
		if err := pairRows.Err(); err != nil {
			return nil, err
		}
	}
	var others []string
	for _, j := range list {
		if other, ok := counterparts[j.c.ID]; ok {
			others = append(others, other)
		}
	}
	counterpartNames, err := r.storedNames(ctx, others)
	if err != nil {
		return nil, err
	}

	lastByContext := map[string]*LastMessage{}
	for _, m := range newest {
		preview, err := messagePreview(m.kind, m.body, m.card)
		if err != nil {
			return nil, err
		}
		last := &LastMessage{ID: m.id, Kind: m.kind, Preview: preview, AuthorID: m.authorID, CreatedAt: m.createdAt}
		if m.authorID != nil {
			if name, ok := names[*m.authorID]; ok {
				last.AuthorDisplayName = &name
			}
		}
		lastByContext[m.contextID] = last
	}

	out := make([]PersonContextSummary, 0, len(list))
	for _, j := range list {
		var otherID, otherName *string
		if other, ok := counterparts[j.c.ID]; ok {
			otherID = &other
			if name, ok := counterpartNames[other]; ok {
				otherName = &name
			}
		}
		unread, err := r.CountUnreadMessages(ctx, j.c.ID, personID)
		if err != nil {
			return nil, err
		}
		out = append(out, PersonContextSummary{
			ID: j.c.ID, DisplayName: direct.DisplayNameFor(j.c.Kind, j.c.DisplayName, otherName),
			MemberCount: counts[j.c.ID], MyRole: j.m.Role, MyState: j.m.State, MembershipID: j.m.ID,
			JoinedAt: j.m.JoinedAt, LastMessage: lastByContext[j.c.ID], UnreadCount: unread, Theme: j.c.Theme,
			Kind: j.c.Kind, CounterpartID: otherID, CounterpartDisplayName: otherName,
		})
	}
	sort.SliceStable(out, func(a, b int) bool {
		x, y := out[a], out[b]
		if (x.LastMessage == nil) != (y.LastMessage == nil) {
			return x.LastMessage != nil
		}
		if x.LastMessage != nil {
			tx, ty := -pythonTimestamp(x.LastMessage.CreatedAt), -pythonTimestamp(y.LastMessage.CreatedAt)
			if tx != ty {
				return tx < ty
			}
		}
		return x.DisplayName < y.DisplayName
	})
	return out, nil
}

// storedNames is `dict(session.execute(select(Person.id, Person.display_name)
// .where(Person.id.in_(ids))))` guarded by `if ids`: the stored names of the
// distinct ids, with no fallback, and no statement for none.
func (r Repository) storedNames(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	var distinct []string
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			distinct = append(distinct, id)
		}
	}
	if len(distinct) == 0 {
		return out, nil
	}
	rows, err := r.Q.Query(ctx,
		`SELECT people.id, people.display_name
		   FROM people
		  WHERE people.id IN (`+uuidPlaceholders(1, len(distinct))+`)`, uuidArgs(distinct)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// pythonTimestamp is datetime.timestamp() of an aware instant: its
// microseconds since the epoch divided by 10**6, correctly rounded.
func pythonTimestamp(instant time.Time) float64 {
	return float64(instant.UnixMicro()) / 1e6
}

// CountUnreadMessages is count_unread_messages: `session.get` of the person's
// read mark (every column, labelled), then one COUNT of the context's messages
// not authored by the person (an AI message, author NULL, counts), after the
// mark when there is one: `(created_at, id) > (mark's last_read_at, mark's
// last_read_message_id)`.
//
// SQLAlchemy note: a mark the same session already loaded answers from the
// identity map without its SELECT; Go reads it again.
func (r Repository) CountUnreadMessages(ctx context.Context, contextID, personID string) (int64, error) {
	var markContext, markPerson, markMessage string
	var markAt, markUpdated time.Time
	marked := true
	err := r.Q.QueryRow(ctx,
		`SELECT context_read_marks.context_id AS context_read_marks_context_id,
		        context_read_marks.person_id AS context_read_marks_person_id,
		        context_read_marks.last_read_message_id AS context_read_marks_last_read_message_id,
		        context_read_marks.last_read_at AS context_read_marks_last_read_at,
		        context_read_marks.updated_at AS context_read_marks_updated_at
		   FROM context_read_marks
		  WHERE context_read_marks.context_id = $1::UUID AND context_read_marks.person_id = $2::UUID`,
		contextID, personID).Scan(&markContext, &markPerson, &markMessage, &markAt, &markUpdated)
	if errors.Is(err, pgx.ErrNoRows) {
		marked = false
	} else if err != nil {
		return 0, err
	}
	sql := `SELECT count(*) AS count_1
	   FROM messages
	  WHERE messages.context_id = $1::UUID AND (messages.author_id IS NULL OR messages.author_id != $2::UUID)`
	args := []any{contextID, personID}
	if marked {
		sql += ` AND (messages.created_at, messages.id) > ($3::TIMESTAMP WITH TIME ZONE, $4::UUID)`
		args = append(args, markAt, markMessage)
	}
	var count int64
	if err := r.Q.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

var aiCardLabels = map[string]string{
	"itinerary":     "[Rủ Đi AI: lịch trình]",
	"places":        "[Rủ Đi AI: gợi ý địa điểm]",
	"poll":          "[Bình chọn]",
	"expense_draft": "[Rủ Đi AI: bản nháp khoản chi]",
}

const aiCardLabel = "[Rủ Đi AI]"

// messagePreview is `_message_preview(kind, body, card)`: one line a
// conversation list can show. A text message is its body stripped the way
// str.strip() strips, newlines turned into spaces, and past 80 code points
// its first 79 and an ellipsis.
func messagePreview(kind string, body *string, card []byte) (string, error) {
	switch kind {
	case "image":
		return "[Ảnh]", nil
	case "sticker":
		return "[Sticker]", nil
	case "deleted":
		return "Tin nhắn đã bị xoá", nil
	case "ai_card":
		return cardLabel(card)
	}
	text := ""
	if body != nil {
		text = *body
	}
	text = strings.ReplaceAll(strings.TrimFunc(text, isPythonSpace), "\n", " ")
	runes := []rune(text)
	if len(runes) <= 80 {
		return text, nil
	}
	return string(runes[:79]) + "…", nil
}

// cardLabel is the ai_card branch: `labels.get(card_kind or "", "[Rủ Đi AI]")`
// where card_kind is the object's "kind" and None for anything but an object.
// A falsy kind (null, false, 0, "", [] or {}) is "", a hashable one that is
// not a label's key (a number, true, another string) is the default, and a
// non-empty array or object cannot be a dict key: ErrPythonTypeError.
func cardLabel(card []byte) (string, error) {
	if card == nil {
		return aiCardLabel, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(card))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return aiCardLabel, nil
	}
	switch kind := object["kind"].(type) {
	case string:
		if label, ok := aiCardLabels[kind]; ok {
			return label, nil
		}
	case []any:
		if len(kind) > 0 {
			return "", ErrPythonTypeError
		}
	case map[string]any:
		if len(kind) > 0 {
			return "", ErrPythonTypeError
		}
	}
	return aiCardLabel, nil
}

// isPythonSpace is str.isspace() for one code point: bidirectional class WS,
// B or S, or category Zs. Unlike unicode.IsSpace it includes U+001C..U+001F.
func isPythonSpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}
