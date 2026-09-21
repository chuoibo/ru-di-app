package chatv2

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type access struct {
	key                []byte
	epoch, last, first int64
}

// authorize locks the live permission rows before the sequence row. The
// membership/device invalidation triggers use this same order. No persisted
// receipt is read until the current membership and device have been checked.
func authorize(ctx context.Context, tx pgx.Tx, actor, device, conversation string) (access, error) {
	var a access
	if !ValidID(actor) || !ValidID(device) || !ValidID(conversation) {
		return a, ErrInvalid
	}
	var kind string
	err := tx.QueryRow(ctx, `SELECT d.signing_key,cm.first_sequence,c.kind
 FROM chat_v2_devices d
 JOIN people p ON p.id=d.person_id
 JOIN chat_v2_members cm ON cm.device_id=d.id AND cm.context_id=$3
 JOIN memberships m ON m.id=cm.membership_id AND m.person_id=p.id AND m.context_id=cm.context_id
 JOIN contexts c ON c.id=cm.context_id
 WHERE p.id=$1 AND p.deleted_at IS NULL AND d.id=$2 AND d.revoked_at IS NULL
 AND m.state='active' AND m.left_at IS NULL FOR SHARE OF p,d,m,cm`, actor, device, conversation).Scan(&a.key, &a.first, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrForbidden
	}
	if err != nil {
		return a, err
	}
	if kind == "pair" {
		rows, e := tx.Query(ctx, `SELECT f.state FROM friend_requests f WHERE
   ((f.requester_id=$1 AND f.addressee_id IN (SELECT person_id FROM memberships WHERE context_id=$2))
   OR (f.addressee_id=$1 AND f.requester_id IN (SELECT person_id FROM memberships WHERE context_id=$2)))
   FOR SHARE OF f`, actor, conversation)
		if e != nil {
			return a, e
		}
		blocked := false
		for rows.Next() {
			var state string
			if e = rows.Scan(&state); e != nil {
				rows.Close()
				return a, e
			}
			blocked = blocked || state == "blocked"
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return a, e
		}
		if blocked {
			return a, ErrForbidden
		}
	}
	var ready bool
	err = tx.QueryRow(ctx, `SELECT epoch,last_sequence,ready FROM chat_v2_conversations WHERE context_id=$1 FOR UPDATE`, conversation).Scan(&a.epoch, &a.last, &ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotReady
	}
	if err != nil {
		return a, err
	}
	if !ready {
		return a, ErrNotReady
	}
	return a, nil
}

func readEvent(ctx context.Context, tx pgx.Tx, conversation string, seq int64) (Event, error) {
	var e Event
	var body []byte
	err := tx.QueryRow(ctx, `SELECT sequence,kind,actor_id::text,body,created_at FROM chat_v2_events WHERE context_id=$1 AND sequence=$2`, conversation, seq).Scan(&e.Sequence, &e.Kind, &e.ActorID, &body, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	err = decodeBody(&e, body)
	return e, err
}
func decodeBody(e *Event, body []byte) error {
	if e.Kind == "envelope" {
		e.Envelope = &Envelope{}
		return json.Unmarshal(body, e.Envelope)
	}
	e.Mark = &Mark{}
	return json.Unmarshal(body, e.Mark)
}
func appendEvent(ctx context.Context, tx pgx.Tx, conversation, actor, kind string, body any) (Event, error) {
	var e Event
	encoded, err := json.Marshal(body)
	if err != nil {
		return e, err
	}
	err = tx.QueryRow(ctx, `UPDATE chat_v2_conversations SET last_sequence=last_sequence+1 WHERE context_id=$1 RETURNING last_sequence`, conversation).Scan(&e.Sequence)
	if err != nil {
		return e, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_v2_events(context_id,sequence,kind,actor_id,body) VALUES($1,$2,$3,$4,$5)`, conversation, e.Sequence, kind, actor, encoded)
	if err != nil {
		return e, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_v2_outbox(context_id,sequence) VALUES($1,$2)`, conversation, e.Sequence)
	if err != nil {
		return e, err
	}
	// NOTIFY is a latency hint emitted only on commit. Durable catch-up never
	// depends on receiving it; the outbox remains available for future fanout.
	_, err = tx.Exec(ctx, `SELECT pg_notify('rudi_chat_v2',$1)`, conversation)
	if err != nil {
		return e, err
	}
	return readEvent(ctx, tx, conversation, e.Sequence)
}

func (s *Store) Send(ctx context.Context, actor string, envelope Envelope) (SendResult, error) {
	var result SendResult
	preimage, err := SigningBytes(envelope)
	if err != nil {
		return result, err
	}
	if len(envelope.Signature) != ed25519.SignatureSize {
		return result, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	a, err := authorize(ctx, tx, actor, envelope.DeviceID, envelope.ConversationID)
	if err != nil {
		return result, err
	}
	if !ed25519.Verify(a.key, preimage, envelope.Signature) {
		return result, ErrForbidden
	}
	digest := sha256.Sum256(preimage)
	var existing []byte
	var seq int64
	err = tx.QueryRow(ctx, `SELECT digest,sequence FROM chat_v2_sends WHERE context_id=$1 AND device_id=$2 AND logical_send_id=$3`, envelope.ConversationID, envelope.DeviceID, envelope.LogicalSendID).Scan(&existing, &seq)
	if err == nil {
		if seq < a.first {
			return result, ErrForbidden
		}
		if !bytes.Equal(existing, digest[:]) {
			return result, ErrConflict
		}
		result.Event, err = readEvent(ctx, tx, envelope.ConversationID, seq)
		if err != nil {
			return result, err
		}
		result.Replayed = true
		return result, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	// An acknowledged send may be replayed after a legitimate epoch advance;
	// a new ciphertext must always target the currently approved epoch.
	if envelope.Epoch != a.epoch {
		return result, ErrEpoch
	}
	result.Event, err = appendEvent(ctx, tx, envelope.ConversationID, actor, "envelope", envelope)
	if err != nil {
		return result, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_v2_sends(context_id,device_id,logical_send_id,digest,sequence) VALUES($1,$2,$3,$4,$5)`, envelope.ConversationID, envelope.DeviceID, envelope.LogicalSendID, digest[:], result.Event.Sequence)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func (s *Store) Events(ctx context.Context, actor, device, conversation string, after int64, limit int) (Page, error) {
	page := Page{Events: []Event{}, NextSequence: after}
	if after < 0 || limit < 1 || limit > MaxPageSize {
		return page, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return page, err
	}
	defer tx.Rollback(ctx)
	a, err := authorize(ctx, tx, actor, device, conversation)
	if err != nil {
		return page, err
	}
	if after > a.last {
		return page, ErrInvalid
	}
	if page.NextSequence < a.first-1 {
		page.NextSequence = a.first - 1
	}
	rows, err := tx.Query(ctx, `SELECT sequence,kind,actor_id::text,body,created_at FROM chat_v2_events WHERE context_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, conversation, page.NextSequence, limit+1)
	if err != nil {
		return page, err
	}
	pageBytes := 256 // Reserve the page wrapper and cursor metadata.
	for rows.Next() {
		var e Event
		var body []byte
		if err = rows.Scan(&e.Sequence, &e.Kind, &e.ActorID, &body, &e.CreatedAt); err != nil {
			rows.Close()
			return page, err
		}
		// JSONB text includes the envelope with its base64 ciphertext. Reserve
		// additional bytes for Event metadata; do not skip the first excluded
		// event when the client resumes from NextSequence.
		eventBytes := len(body) + 256
		if len(page.Events) == limit || (len(page.Events) > 0 && pageBytes+eventBytes > MaxPageBytes) {
			page.HasMore = true
			break
		}
		if err = decodeBody(&e, body); err != nil {
			rows.Close()
			return page, err
		}
		page.Events = append(page.Events, e)
		pageBytes += eventBytes
		page.NextSequence = e.Sequence
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	return page, tx.Commit(ctx)
}

func (s *Store) Mark(ctx context.Context, actor, device, conversation, kind string, sequence int64) (Mark, error) {
	mark := Mark{DeviceID: device}
	if (kind != "read" && kind != "delivered") || sequence < 0 {
		return mark, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mark, err
	}
	defer tx.Rollback(ctx)
	a, err := authorize(ctx, tx, actor, device, conversation)
	if err != nil {
		return mark, err
	}
	if sequence > a.last || sequence < a.first-1 {
		return mark, ErrInvalid
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_v2_marks(context_id,device_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, conversation, device)
	if err != nil {
		return mark, err
	}
	err = tx.QueryRow(ctx, `SELECT delivered,read_sequence FROM chat_v2_marks WHERE context_id=$1 AND device_id=$2`, conversation, device).Scan(&mark.Delivered, &mark.Read)
	if err != nil {
		return mark, err
	}
	old := mark
	if sequence > mark.Delivered {
		mark.Delivered = sequence
	}
	if kind == "read" && sequence > mark.Read {
		mark.Read = sequence
	}
	if mark == old {
		return mark, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE chat_v2_marks SET delivered=$3,read_sequence=$4 WHERE context_id=$1 AND device_id=$2`, conversation, device, mark.Delivered, mark.Read)
	if err != nil {
		return mark, err
	}
	if _, err = appendEvent(ctx, tx, conversation, actor, "mark", mark); err != nil {
		return mark, err
	}
	return mark, tx.Commit(ctx)
}
