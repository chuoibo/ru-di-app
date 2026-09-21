package chatv2

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Recipient binds one socket to its original authenticated actor and session.
// The digest is never serialized; every dispatch checks it against live rows.
type Recipient struct {
	ActorID, DeviceID string
	SessionDigest     []byte
	After             int64
	Limit             int
}

type Delivery struct {
	// HighWatermark is transport scheduling metadata, never an authorization cache.
	HighWatermark int64
	Page          Page
	Err           error
}

const MaxDispatchRecipients = 1000
const MaxDispatchBytes = 2 << 20

// EventsBatch shares an immutable event window, not authorization decisions.
// A fresh REPEATABLE READ, READ ONLY snapshot is the read's linearization
// point. ACLs, membership incarnation, epoch, high-watermark and events all
// belong to that one snapshot. A concurrent revoke can commit without waiting,
// but this batch cannot observe any event committed after its snapshot. Every
// subsequent batch checks a fresh snapshot; grants are never cached.
// This deliberately differs from Send/Mark, which retain write-side row locks.
func (s *Store) EventsBatch(ctx context.Context, conversation string, recipients []Recipient) ([]Delivery, error) {
	if !ValidID(conversation) || len(recipients) == 0 || len(recipients) > MaxDispatchRecipients {
		return nil, ErrInvalid
	}
	out := make([]Delivery, len(recipients))
	actors, devices, digests := []string{}, []string{}, [][]byte{}
	for i, r := range recipients {
		out[i].Page = Page{Events: []Event{}, NextSequence: r.After}
		out[i].Err = ErrForbidden
		if !ValidID(r.ActorID) || !ValidID(r.DeviceID) || len(r.SessionDigest) != 32 || r.After < 0 || r.Limit < 1 || r.Limit > MaxPageSize {
			return nil, ErrInvalid
		}
		actors = append(actors, r.ActorID)
		devices = append(devices, r.DeviceID)
		digests = append(digests, r.SessionDigest)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Pipeline all authority reads in the same snapshot. Read-only snapshots
	// avoid producing a WAL row-lock record for every recipient on every page.
	batch := &pgx.Batch{}
	batch.Queue(`SELECT id::text FROM people WHERE id=ANY($1::uuid[]) AND deleted_at IS NULL ORDER BY id`, actors)
	batch.Queue(`SELECT token_digest,person_id::text,expires_at FROM account_sessions WHERE token_digest=ANY($1::bytea[]) AND revoked_at IS NULL AND expires_at>clock_timestamp() ORDER BY id`, digests)
	batch.Queue(`SELECT id::text,person_id::text FROM chat_v2_devices WHERE id=ANY($1::uuid[]) AND revoked_at IS NULL ORDER BY id`, devices)
	batch.Queue(`SELECT id::text,person_id::text FROM memberships WHERE context_id=$1 AND person_id=ANY($2::uuid[]) AND state='active' AND left_at IS NULL ORDER BY id`, conversation, actors)
	batch.Queue(`SELECT device_id::text,membership_id::text,first_sequence FROM chat_v2_members WHERE context_id=$1 AND device_id=ANY($2::uuid[]) ORDER BY device_id`, conversation, devices)
	batch.Queue(`SELECT f.state FROM friend_requests f WHERE EXISTS(SELECT 1 FROM contexts c WHERE c.id=$1 AND c.kind='pair') AND f.requester_id IN(SELECT person_id FROM memberships WHERE context_id=$1) AND f.addressee_id IN(SELECT person_id FROM memberships WHERE context_id=$1) ORDER BY f.id`, conversation)
	batch.Queue(`SELECT epoch,last_sequence,ready FROM chat_v2_conversations WHERE context_id=$1`, conversation)
	br := tx.SendBatch(ctx, batch)
	// Closing every row set is necessary before asking pgx for the next result.
	read := func(scan func(pgx.Rows) error) error {
		rows, e := br.Query()
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			if e = scan(rows); e != nil {
				return e
			}
		}
		return rows.Err()
	}
	people := map[string]bool{}
	err = read(func(rows pgx.Rows) error { var id string; e := rows.Scan(&id); people[id] = true; return e })
	if err != nil {
		br.Close()
		return nil, err
	}
	type session struct {
		actor  string
		expiry time.Time
	}
	sessions := map[string]session{}
	err = read(func(rows pgx.Rows) error {
		var digest []byte
		var a session
		e := rows.Scan(&digest, &a.actor, &a.expiry)
		sessions[string(digest)] = a
		return e
	})
	if err != nil {
		br.Close()
		return nil, err
	}
	deviceOwners := map[string]string{}
	err = read(func(rows pgx.Rows) error {
		var id, actor string
		e := rows.Scan(&id, &actor)
		deviceOwners[id] = actor
		return e
	})
	if err != nil {
		br.Close()
		return nil, err
	}
	memberships := map[string]string{}
	err = read(func(rows pgx.Rows) error {
		var id, actor string
		e := rows.Scan(&id, &actor)
		memberships[id] = actor
		return e
	})
	if err != nil {
		br.Close()
		return nil, err
	}
	type member struct {
		membership string
		first      int64
	}
	members := map[string]member{}
	err = read(func(rows pgx.Rows) error {
		var id string
		var m member
		e := rows.Scan(&id, &m.membership, &m.first)
		members[id] = m
		return e
	})
	if err != nil {
		br.Close()
		return nil, err
	}
	blocked := false
	err = read(func(rows pgx.Rows) error {
		var state string
		e := rows.Scan(&state)
		blocked = blocked || state == "blocked"
		return e
	})
	if err != nil {
		br.Close()
		return nil, err
	}
	var epoch, last int64
	var ready bool
	err = br.QueryRow().Scan(&epoch, &last, &ready)
	closeErr := br.Close()
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotReady
	}
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if !ready || epoch < 1 {
		return nil, ErrNotReady
	}
	minAfter := last
	now := time.Now()
	for i, r := range recipients {
		out[i].HighWatermark = last
		se, ok := sessions[string(r.SessionDigest)]
		m, memberOK := members[r.DeviceID]
		if blocked || !people[r.ActorID] || !ok || se.actor != r.ActorID || !se.expiry.After(now) || deviceOwners[r.DeviceID] != r.ActorID || !memberOK || memberships[m.membership] != r.ActorID {
			continue
		}
		if r.After > last {
			out[i].Err = ErrInvalid
			continue
		}
		out[i].Err = nil
		out[i].Page.NextSequence = max(r.After, m.first-1)
		minAfter = min(minAfter, out[i].Page.NextSequence)
	}
	// The dispatcher groups nearby cursors. A bounded shared window lets a
	// recovering device make progress without unbounded history allocation.
	rows, err := tx.Query(ctx, `SELECT sequence,kind,actor_id::text,body,created_at FROM chat_v2_events WHERE context_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, conversation, minAfter, 2*MaxPageSize+1)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, 2*MaxPageSize)
	sizes := []int{}
	total := 0
	for rows.Next() {
		var e Event
		var body []byte
		if err = rows.Scan(&e.Sequence, &e.Kind, &e.ActorID, &body, &e.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		size := len(body) + 256
		if total+size > MaxDispatchBytes {
			break
		}
		if err = decodeBody(&e, body); err != nil {
			rows.Close()
			return nil, err
		}
		events = append(events, e)
		sizes = append(sizes, size)
		total += size
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Err != nil {
			continue
		}
		p := &out[i].Page
		start, end := -1, -1
		size := 256
		for j, e := range events {
			if e.Sequence <= p.NextSequence {
				continue
			}
			if start < 0 {
				start = j
				end = j
			}
			if end-start == recipients[i].Limit || (end > start && size+sizes[j] > MaxPageBytes) {
				break
			}
			end = j + 1
			size += sizes[j]
			p.NextSequence = e.Sequence
		}
		if start >= 0 {
			p.Events = events[start:end]
		}
		p.HasMore = p.NextSequence < last
	}
	// Expiry is wall-clock based, not snapshot based. A bounded read that
	// crosses its session deadline must not hand that session a page.
	var handoff time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&handoff); err != nil {
		return nil, err
	}
	for i, r := range recipients {
		if se, ok := sessions[string(r.SessionDigest)]; out[i].Err == nil && (!ok || !se.expiry.After(handoff)) {
			out[i] = Delivery{Err: ErrForbidden}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
