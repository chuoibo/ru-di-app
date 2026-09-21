package chatlegacychange

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/routes"
)

var ErrAuthentication = errors.New("authentication_required")
var ErrForbidden = errors.New("chat_unavailable")
var ErrCursor = errors.New("cursor_invalid")

type Change struct {
	Sequence int64  `json:"sequence"`
	Type     string `json:"type"`
	EntityID string `json:"entity_id"`
	Revision int64  `json:"revision"`
}
type Page struct {
	ActorID      string   `json:"-"`
	ContextID    string   `json:"context_id"`
	Changes      []Change `json:"changes"`
	NextSequence int64    `json:"next_sequence"`
	Watermark    int64    `json:"watermark"`
	HasMore      bool     `json:"has_more"`
}
type SnapshotRequest struct {
	MessageIDs []string `json:"message_ids"`
	VoteIDs    []string `json:"vote_ids"`
}
type Snapshot struct {
	ContextID string            `json:"context_id"`
	Watermark int64             `json:"watermark"`
	Messages  []json.RawMessage `json:"messages"`
	Votes     []json.RawMessage `json:"votes"`
}
type Store struct{ Pool *pgxpool.Pool }

// authorize and every projection share one fresh read-only snapshot. Concurrent
// revocation can finish first, but this read cannot include later committed data.
func authorize(ctx context.Context, tx pgx.Tx, headers http.Header, room string) (string, error) {
	token, problem := auth.BearerToken(headers)
	if problem != nil {
		return "", ErrAuthentication
	}
	var actor string
	err := tx.QueryRow(ctx, `SELECT person_id FROM account_sessions WHERE token_digest=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, auth.TokenDigest(token)).Scan(&actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAuthentication
	}
	if err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id FROM people WHERE id=$1::uuid AND deleted_at IS NULL`, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAuthentication
	}
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `SELECT person_id FROM account_sessions WHERE token_digest=$1 AND person_id=$2::uuid AND revoked_at IS NULL AND expires_at>clock_timestamp()`, auth.TokenDigest(token), actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAuthentication
	}
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM memberships WHERE context_id=$1::uuid AND person_id=$2::uuid AND state='active' AND left_at IS NULL`, room, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrForbidden
	}
	if err != nil {
		return "", err
	}
	var kind string
	err = tx.QueryRow(ctx, `SELECT kind FROM contexts WHERE id=$1::uuid`, room).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrForbidden
	}
	if err != nil {
		return "", err
	}
	if kind == "pair" {
		var other string
		err = tx.QueryRow(ctx, `SELECT person_id FROM memberships WHERE context_id=$1::uuid AND person_id<>$2::uuid AND state='active' AND left_at IS NULL ORDER BY person_id LIMIT 1`, room, actor).Scan(&other)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrForbidden
		}
		if err != nil {
			return "", err
		}
		err = tx.QueryRow(ctx, `SELECT id FROM people WHERE id=$1::uuid AND deleted_at IS NULL`, other).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrForbidden
		}
		if err != nil {
			return "", err
		}
		var state string
		err = tx.QueryRow(ctx, `SELECT state FROM friend_requests WHERE ((requester_id=$1::uuid AND addressee_id=$2::uuid) OR (requester_id=$2::uuid AND addressee_id=$1::uuid)) AND state<>'declined' ORDER BY created_at DESC LIMIT 1`, actor, other).Scan(&state)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
		if state == "blocked" {
			return "", ErrForbidden
		}
	}
	return actor, nil
}
func (s Store) begin(ctx context.Context, h http.Header, room string) (pgx.Tx, string, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, "", err
	}
	actor, err := authorize(ctx, tx, h, room)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, "", err
	}
	return tx, actor, nil
}

func commitRead(ctx context.Context, tx pgx.Tx, headers http.Header) error {
	token, problem := auth.BearerToken(headers)
	if problem != nil {
		return ErrAuthentication
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT coalesce((SELECT expires_at>clock_timestamp() FROM account_sessions WHERE token_digest=$1),false)`, auth.TokenDigest(token)).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return ErrAuthentication
	}
	return tx.Commit(ctx)
}
func watermark(ctx context.Context, tx pgx.Tx, room string) (int64, error) {
	var n int64
	err := tx.QueryRow(ctx, `SELECT coalesce((SELECT sequence FROM chat_legacy_change_heads WHERE context_id=$1::uuid),0)`, room).Scan(&n)
	return n, err
}
func (s Store) Changes(ctx context.Context, h http.Header, room string, after int64, limit int) (Page, error) {
	out := Page{ContextID: room, Changes: []Change{}, NextSequence: after}
	tx, actor, err := s.begin(ctx, h, room)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out.ActorID = actor
	out.Watermark, err = watermark(ctx, tx, room)
	if err != nil {
		return out, err
	}
	if after < 0 || after > out.Watermark || limit < 1 || limit > 100 {
		return out, ErrCursor
	}
	rows, err := tx.Query(ctx, `SELECT sequence,entity_type,entity_id,revision FROM chat_legacy_changes WHERE context_id=$1::uuid AND sequence>$2 ORDER BY sequence LIMIT $3`, room, after, limit+1)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c Change
		if err = rows.Scan(&c.Sequence, &c.Type, &c.EntityID, &c.Revision); err != nil {
			rows.Close()
			return out, err
		}
		out.Changes = append(out.Changes, c)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	if len(out.Changes) > limit {
		out.HasMore = true
		out.Changes = out.Changes[:limit]
	}
	if len(out.Changes) > 0 {
		out.NextSequence = out.Changes[len(out.Changes)-1].Sequence
	}
	return out, commitRead(ctx, tx, h)
}

// Snapshot reads a consistent watermark and batches all hydration, including
// reactions, quotes, vote options and ballots. It never performs one query/ID.
func (s Store) Snapshot(ctx context.Context, h http.Header, room string, in SnapshotRequest) (Snapshot, error) {
	out := Snapshot{ContextID: room, Messages: []json.RawMessage{}, Votes: []json.RawMessage{}}
	if len(in.MessageIDs)+len(in.VoteIDs) > 100 {
		return out, ErrCursor
	}
	tx, actor, err := s.begin(ctx, h, room)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out.Watermark, err = watermark(ctx, tx, room)
	if err != nil {
		return out, err
	}
	r := repo.Repository{Q: tx}
	var messages []repo.Message
	if len(in.MessageIDs)+len(in.VoteIDs) == 0 {
		page, e := r.ListMessages(ctx, room, 50, nil, nil)
		if e != nil {
			return out, e
		}
		messages = page.Messages
	} else {
		records, e := r.GetMessagesByIDs(ctx, in.MessageIDs)
		if e != nil {
			return out, e
		}
		for _, id := range in.MessageIDs {
			if m, ok := records[id]; ok && m.ContextID == room {
				messages = append(messages, m)
			}
		}
	}
	ids, quotedIDs := []string{}, []string{}
	voteSet := map[string]bool{}
	for _, id := range in.VoteIDs {
		voteSet[id] = true
	}
	for _, m := range messages {
		ids = append(ids, m.ID)
		if m.ReplyToID != nil {
			quotedIDs = append(quotedIDs, *m.ReplyToID)
		}
		var card struct {
			Kind    string `json:"kind"`
			Payload struct {
				VoteID string `json:"vote_id"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(m.Card, &card)
		if card.Kind == "poll" && validUUID(card.Payload.VoteID) {
			voteSet[card.Payload.VoteID] = true
		}
	}
	reactions, err := r.ListReactions(ctx, ids)
	if err != nil {
		return out, err
	}
	quoted, err := r.GetMessagesByIDs(ctx, quotedIDs)
	if err != nil {
		return out, err
	}
	for id, m := range quoted {
		if m.ContextID != room {
			delete(quoted, id)
		}
	}
	revisions, deleted, err := entityRevisions(ctx, tx, room, "message", append(ids, in.MessageIDs...))
	if err != nil {
		return out, err
	}
	known := map[string]bool{}
	for _, m := range messages {
		known[m.ID] = true
		b, e := routes.ChatSnapshotMessage(m, reactions, quoted, actor, revisions[m.ID])
		if e != nil {
			return out, e
		}
		out.Messages = append(out.Messages, b)
	}
	for id, at := range deleted {
		if !known[id] {
			m := repo.Message{ID: id, ContextID: room, Kind: "deleted", CreatedAt: at, DeletedAt: &at}
			b, e := routes.ChatSnapshotMessage(m, nil, nil, actor, revisions[id])
			if e != nil {
				return out, e
			}
			out.Messages = append(out.Messages, b)
		}
	}
	voteIDs := []string{}
	for id := range voteSet {
		voteIDs = append(voteIDs, id)
	}
	sort.Strings(voteIDs)
	votes, err := readVotes(ctx, tx, room, voteIDs)
	if err != nil {
		return out, err
	}
	revisions, deleted, err = entityRevisions(ctx, tx, room, "vote", voteIDs)
	if err != nil {
		return out, err
	}
	for _, v := range votes {
		b, e := routes.ChatSnapshotVote(v, actor, revisions[v.ID])
		if e != nil {
			return out, e
		}
		out.Votes = append(out.Votes, b)
		delete(deleted, v.ID)
	}
	for id := range deleted {
		b, _ := json.Marshal(map[string]any{"id": id, "context_id": room, "revision": revisions[id], "deleted": true})
		out.Votes = append(out.Votes, b)
	}
	return out, commitRead(ctx, tx, h)
}
func entityRevisions(ctx context.Context, tx pgx.Tx, room, kind string, ids []string) (map[string]int64, map[string]time.Time, error) {
	revisions := map[string]int64{}
	deleted := map[string]time.Time{}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(entity_id) entity_id,revision,deleted,created_at FROM chat_legacy_changes WHERE context_id=$1::uuid AND entity_type=$2 AND entity_id=ANY($3::uuid[]) ORDER BY entity_id,sequence DESC`, room, kind, ids)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		var gone bool
		var at time.Time
		if err = rows.Scan(&id, &n, &gone, &at); err != nil {
			return nil, nil, err
		}
		revisions[id] = n
		if gone {
			deleted[id] = at
		}
	}
	return revisions, deleted, rows.Err()
}
func readVotes(ctx context.Context, tx pgx.Tx, room string, ids []string) ([]repo.Vote, error) {
	out := []repo.Vote{}
	index := map[string]int{}
	rows, err := tx.Query(ctx, `SELECT id,context_id,outing_id,created_by_id,question,created_at,closed_at,closed_by_id FROM votes WHERE context_id=$1::uuid AND id=ANY($2::uuid[]) ORDER BY id`, room, ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v repo.Vote
		if err = rows.Scan(&v.ID, &v.ContextID, &v.OutingID, &v.CreatedByID, &v.Question, &v.CreatedAt, &v.ClosedAt, &v.ClosedByID); err != nil {
			rows.Close()
			return nil, err
		}
		v.Options = []repo.VoteOption{}
		v.Ballots = []repo.VoteBallot{}
		index[v.ID] = len(out)
		out = append(out, v)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	allowed := []string{}
	for _, v := range out {
		allowed = append(allowed, v.ID)
	}
	rows, err = tx.Query(ctx, `SELECT id,vote_id,position,label,place_name FROM vote_options WHERE vote_id=ANY($1::uuid[]) ORDER BY vote_id,position`, allowed)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var o repo.VoteOption
		if err = rows.Scan(&o.ID, &o.VoteID, &o.Position, &o.Label, &o.PlaceName); err != nil {
			rows.Close()
			return nil, err
		}
		i := index[o.VoteID]
		out[i].Options = append(out[i].Options, o)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT id,vote_id,option_id,voter_id,created_at,updated_at FROM vote_ballots WHERE vote_id=ANY($1::uuid[]) ORDER BY vote_id,created_at,id`, allowed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b repo.VoteBallot
		if err = rows.Scan(&b.ID, &b.VoteID, &b.OptionID, &b.VoterID, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		i := index[b.VoteID]
		out[i].Ballots = append(out[i].Ballots, b)
	}
	return out, rows.Err()
}
