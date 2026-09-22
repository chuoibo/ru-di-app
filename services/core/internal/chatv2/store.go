package chatv2

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool   *pgxpool.Pool
	writes *writeAdmission
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, writes: newWriteAdmission(pool.Config().MaxConns)}
}

type access struct {
	key                []byte
	epoch, last, first int64
}

// convLock is how far a caller goes on the conversation row itself. Every
// message in a group passes through that one row, so whatever a caller holds
// on it, every other member of the group queues behind.
//
// lockNone is not a weaker check. The send path re-states its epoch and
// readiness conditions inside the sequence bump, where PostgreSQL re-evaluates
// them against the version it just locked; the guarantee moves into one
// statement instead of spanning three client round trips.
type convLock int

const (
	lockNone convLock = iota
	lockShare
	lockUpdate
)

// authorize locks the live permission rows before the sequence row. The
// membership/device invalidation triggers use this same order. No persisted
// receipt is read until the current membership and device have been checked.
func authorize(ctx context.Context, tx pgx.Tx, actor, device, conversation string) (access, error) {
	return authorizeWithLock(ctx, tx, actor, device, conversation, true)
}

func authorizeWithLock(ctx context.Context, tx pgx.Tx, actor, device, conversation string, exclusive bool) (access, error) {
	mode := lockShare
	if exclusive {
		mode = lockUpdate
	}
	return authorizeSessionWithLock(ctx, tx, actor, device, conversation, mode, nil)
}

func authorizeSessionWithLock(ctx context.Context, tx pgx.Tx, actor, device, conversation string, mode convLock, sessionDigest []byte) (access, error) {
	var a access
	if !ValidID(actor) || !ValidID(device) || !ValidID(conversation) {
		return a, ErrInvalid
	}
	batch := &pgx.Batch{}
	batch.Queue(`SELECT id::text FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, actor)
	if sessionDigest != nil {
		batch.Queue(`SELECT person_id::text FROM account_sessions WHERE token_digest=$1 AND person_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, sessionDigest, actor)
	}
	batch.Queue(`SELECT signing_key FROM chat_v2_devices WHERE id=$1 AND person_id=$2 AND revoked_at IS NULL FOR SHARE`, device, actor)
	batch.Queue(`SELECT id::text FROM memberships WHERE context_id=$1 AND person_id=$2 AND state='active' AND left_at IS NULL ORDER BY id FOR SHARE`, conversation, actor)
	batch.Queue(`SELECT membership_id::text,first_sequence FROM chat_v2_members WHERE context_id=$1 AND device_id=$2 FOR SHARE`, conversation, device)
	batch.Queue(`SELECT kind FROM contexts WHERE id=$1`, conversation)
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	var person, membership, kind string
	err := br.QueryRow().Scan(&person)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrForbidden
	}
	if err != nil {
		return a, err
	}
	if sessionDigest != nil {
		if err = br.QueryRow().Scan(&person); errors.Is(err, pgx.ErrNoRows) {
			return a, ErrForbidden
		} else if err != nil {
			return a, err
		}
	}
	if err = br.QueryRow().Scan(&a.key); errors.Is(err, pgx.ErrNoRows) {
		return a, ErrForbidden
	} else if err != nil {
		return a, err
	}
	rows, err := br.Query()
	if err != nil {
		return a, err
	}
	active := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return a, err
		}
		active[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	if err = br.QueryRow().Scan(&membership, &a.first); errors.Is(err, pgx.ErrNoRows) {
		return a, ErrForbidden
	} else if err != nil {
		return a, err
	}
	if err = br.QueryRow().Scan(&kind); err != nil {
		return a, err
	}
	if err = br.Close(); err != nil {
		return a, err
	}
	if !active[membership] {
		return a, ErrForbidden
	}
	if kind == "pair" {
		rows, e := tx.Query(ctx, `SELECT f.state FROM friend_requests f WHERE
   ((f.requester_id=$1 AND f.addressee_id IN (SELECT person_id FROM memberships WHERE context_id=$2))
   OR (f.addressee_id=$1 AND f.requester_id IN (SELECT person_id FROM memberships WHERE context_id=$2)))
   ORDER BY f.id FOR SHARE OF f`, actor, conversation)
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
	// Readers must hold a stable epoch/permission boundary, but they must not
	// serialize with every other recipient. SHARE still excludes sequence
	// writers and roster invalidation until this read transaction finishes.
	lock := ""
	switch mode {
	case lockShare:
		lock = " FOR SHARE"
	case lockUpdate:
		lock = " FOR UPDATE"
	}
	err = tx.QueryRow(ctx, `SELECT epoch,last_sequence,ready FROM chat_v2_conversations WHERE context_id=$1`+lock, conversation).Scan(&a.epoch, &a.last, &ready)
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

type sendReceipt struct {
	device, logical string
	digest          []byte
}

func appendEvent(ctx context.Context, tx pgx.Tx, conversation, actor, kind string, epoch int64, body any, receipt *sendReceipt) (Event, error) {
	var e Event
	encoded, err := json.Marshal(body)
	if err != nil {
		return e, err
	}
	// The sequence, event and outbox are one statement as well as one
	// transaction. The outbox relay publishes wake hints after this commit;
	// the writer must never acquire PostgreSQL's global NOTIFY commit lock.
	// Returning the inserted row avoids another read round trip per envelope.
	var bodyJSON []byte
	var device, logical any
	var digest []byte
	if receipt != nil {
		device, logical, digest = receipt.device, receipt.logical, receipt.digest
	}
	err = tx.QueryRow(ctx, `WITH bumped AS (
 UPDATE chat_v2_conversations SET last_sequence=last_sequence+1 WHERE context_id=$1 AND ready AND epoch=$8 RETURNING last_sequence
), inserted AS (
 INSERT INTO chat_v2_events(context_id,sequence,kind,actor_id,body)
 SELECT $1,last_sequence,$2,$3,$4 FROM bumped
 RETURNING sequence,kind,actor_id::text,body,created_at
), queued AS (
 INSERT INTO chat_v2_outbox(context_id,sequence) SELECT $1,sequence FROM inserted
), deduplicated AS (
 INSERT INTO chat_v2_sends(context_id,device_id,logical_send_id,digest,sequence)
 SELECT $1,$5::uuid,$6::uuid,$7::bytea,sequence FROM inserted WHERE $5::uuid IS NOT NULL
)
SELECT sequence,kind,actor_id,body,created_at FROM inserted`, conversation, kind, actor, encoded, device, logical, digest, epoch).Scan(&e.Sequence, &e.Kind, &e.ActorID, &bodyJSON, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// The guard matched nothing: between authorization and this statement
		// the conversation was rekeyed or a roster change cleared `ready`. Read
		// the row back to say which, so the caller sees the same error it would
		// have seen had the check happened under a lock held the whole way.
		return e, appendGuardError(ctx, tx, conversation, epoch)
	}
	if err != nil {
		return e, err
	}
	err = decodeBody(&e, bodyJSON)
	return e, err
}

// appendGuardError runs only on the rare losing race, so it may spend a round
// trip to be precise rather than collapse two different causes into one code.
func appendGuardError(ctx context.Context, tx pgx.Tx, conversation string, epoch int64) error {
	var current int64
	var ready bool
	err := tx.QueryRow(ctx, `SELECT epoch,ready FROM chat_v2_conversations WHERE context_id=$1`, conversation).Scan(&current, &ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotReady
	}
	if err != nil {
		return err
	}
	if !ready {
		return ErrNotReady
	}
	if current != epoch {
		return ErrEpoch
	}
	// The row still satisfies the guard, so the bump should have matched. Do
	// not report success on a write that did not happen.
	return errors.New("chat_v2_sequence_bump_vanished")
}

func (s *Store) Send(ctx context.Context, actor string, envelope Envelope) (SendResult, error) {
	return s.send(ctx, actor, nil, envelope)
}

// SendSession revalidates the bearer grant after admission, under the same
// transaction and lock order as membership, device and event persistence.
func (s *Store) SendSession(ctx context.Context, actor string, digest []byte, envelope Envelope) (SendResult, error) {
	if len(digest) != 32 {
		return SendResult{}, ErrForbidden
	}
	return s.send(ctx, actor, digest, envelope)
}

func (s *Store) send(ctx context.Context, actor string, sessionDigest []byte, envelope Envelope) (SendResult, error) {
	var result SendResult
	preimage, err := SigningBytes(envelope)
	if err != nil {
		return result, err
	}
	if len(envelope.Signature) != ed25519.SignatureSize {
		return result, ErrInvalid
	}
	if !ValidID(actor) {
		return result, ErrInvalid
	}
	// Two copies of one logical send can now reach the receipt insert together,
	// because the conversation row no longer serializes them beforehand. The
	// primary key still decides, and the loser re-runs once: its second attempt
	// finds the receipt and takes the replay path, which compares digests and
	// so can still tell a genuine retry from a different message reusing an id.
	result, err = s.sendOnce(ctx, actor, sessionDigest, envelope, preimage)
	if isDuplicateReceipt(err) {
		return s.sendOnce(ctx, actor, sessionDigest, envelope, preimage)
	}
	return result, err
}

// isDuplicateReceipt reports the one race the send path retries. Any other
// unique violation is a real defect and must not be replayed into silence.
func isDuplicateReceipt(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "chat_v2_sends_pkey"
}

func (s *Store) sendOnce(ctx context.Context, actor string, sessionDigest []byte, envelope Envelope, preimage []byte) (SendResult, error) {
	var result SendResult
	release, err := s.writes.acquire(ctx, envelope.ConversationID)
	if err != nil {
		return result, err
	}
	defer release()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	a, err := authorizeSessionWithLock(ctx, tx, actor, envelope.DeviceID, envelope.ConversationID, lockNone, sessionDigest)
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
	result.Event, err = appendEvent(ctx, tx, envelope.ConversationID, actor, "envelope", a.epoch, envelope, &sendReceipt{device: envelope.DeviceID, logical: envelope.LogicalSendID, digest: digest[:]})
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
	a, err := authorizeWithLock(ctx, tx, actor, device, conversation, false)
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
	return s.mark(ctx, actor, device, conversation, kind, sequence, nil)
}

// MarkSession applies the same session boundary to durable receipt writes.
func (s *Store) MarkSession(ctx context.Context, actor string, digest []byte, device, conversation, kind string, sequence int64) (Mark, error) {
	if len(digest) != 32 {
		return Mark{}, ErrForbidden
	}
	return s.mark(ctx, actor, device, conversation, kind, sequence, digest)
}

func (s *Store) mark(ctx context.Context, actor, device, conversation, kind string, sequence int64, sessionDigest []byte) (Mark, error) {
	mark := Mark{DeviceID: device}
	if (kind != "read" && kind != "delivered") || sequence < 0 || !ValidID(actor) || !ValidID(device) {
		return mark, ErrInvalid
	}
	release, err := s.writes.acquire(ctx, conversation)
	if err != nil {
		return mark, err
	}
	defer release()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mark, err
	}
	defer tx.Rollback(ctx)
	a, err := authorizeSessionWithLock(ctx, tx, actor, device, conversation, lockUpdate, sessionDigest)
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
	if _, err = appendEvent(ctx, tx, conversation, actor, "mark", a.epoch, mark, nil); err != nil {
		return mark, err
	}
	return mark, tx.Commit(ctx)
}
