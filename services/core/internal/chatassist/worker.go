package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/treejson"
)

type work struct {
	id, conversation, person, member, prompt, lease string
	digest                                          []byte
	// The context the caller handed over, exactly as it was stored. Nil when the
	// caller sent none, which is still the shape an older client produces.
	goi []byte
}

// Run owns two bounded inference workers. Leases recover a crashed worker;
// retries may repeat inference, but publication is transactionally once-only.
func (h *Handler) Run(ctx context.Context) {
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTicker(250 * time.Millisecond)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					_, _ = h.ProcessOne(ctx)
				}
			}
		}()
	}
	workers.Wait()
}

func (h *Handler) claim(ctx context.Context) (work, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return work{}, false, err
	}
	defer tx.Rollback(ctx)
	// Bound plaintext retention to the explicit sharing window, including failed jobs.
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET prompt=NULL,boi_canh=NULL,status=CASE WHEN status IN ('queued','running') THEN 'failed' ELSE status END,code=CASE WHEN status IN ('queued','running') THEN 'sharing_expired' ELSE code END,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE prompt IS NOT NULL AND share_expires_at<=clock_timestamp()`)
	if err != nil {
		return work{}, false, err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code='worker_interrupted',lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE status='running' AND lease_until<clock_timestamp() AND attempts>=3`)
	if err != nil {
		return work{}, false, err
	}
	var j work
	j.lease = newID()
	err = tx.QueryRow(ctx, `WITH candidate AS (SELECT id FROM chat_ai_invocations WHERE (status='queued' OR (status='running' AND lease_until<clock_timestamp())) AND attempts<3 AND share_expires_at>clock_timestamp() ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE chat_ai_invocations j SET status='running',attempts=attempts+1,lease_id=$1,lease_until=clock_timestamp()+interval '75 seconds',updated_at=clock_timestamp() FROM candidate c WHERE j.id=c.id RETURNING j.id,j.context_id,j.person_id,j.membership_id,j.session_digest,j.prompt,j.boi_canh`, j.lease).Scan(&j.id, &j.conversation, &j.person, &j.member, &j.digest, &j.prompt, &j.goi)
	if errors.Is(err, pgx.ErrNoRows) {
		return work{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return j, false, err
	}
	return j, true, tx.Commit(ctx)
}

// ProcessOne is also the deterministic worker entry point for PostgreSQL gates.
func (h *Handler) ProcessOne(ctx context.Context) (bool, error) {
	ctx, cancelJob := context.WithTimeout(ctx, 70*time.Second)
	defer cancelJob()
	j, ok, err := h.claim(ctx)
	if err != nil || !ok {
		return ok, err
	}
	catalogue, err := h.prepare(ctx, j)
	if err != nil {
		return true, h.finishFailure(ctx, j, "sharing_unavailable")
	}
	conversation, err := hoiThoai(j.goi, j.prompt)
	if err != nil {
		return true, h.finishFailure(ctx, j, "invalid_ai_result")
	}
	payload := pyjson.NewOrderedMap()
	payload.Set("conversation", conversation)
	// Deliberately empty, and it stays empty. The client pseudonymised the
	// speakers on the way out; the server holds the real names and could put
	// them back, but undoing a caller's privacy decision from the other side of
	// the wire is worse than either choice made openly. The speaker labels
	// inside each turn carry what a planner actually needs.
	payload.Set("members", pyjson.List{})
	payload.Set("places", catalogue)
	payload.Set("budget_per_person_vnd", pyjson.Null{})
	inference, cancel := context.WithTimeout(ctx, 60*time.Second)
	raw, err := h.brain.PostJSONContext(inference, "companion-reply", payload)
	cancel()
	if err != nil {
		return true, h.finishFailure(ctx, j, "provider_unavailable")
	}
	places := make([]*pyjson.OrderedMap, 0, len(catalogue))
	for _, v := range catalogue {
		places = append(places, v.(*pyjson.OrderedMap))
	}
	grounded, err := companion.GroundCard(treejson.To(raw), treejson.MapsTo(places))
	if err != nil {
		return true, h.finishFailure(ctx, j, "invalid_ai_result")
	}
	card, err := pyjson.Dumps(treejson.From(grounded))
	if err != nil {
		return true, h.finishFailure(ctx, j, "invalid_ai_result")
	}
	return true, h.publish(ctx, j, card)
}

func (h *Handler) prepare(ctx context.Context, j work) (pyjson.List, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	g, err := authority(ctx, tx, j.conversation, j.digest)
	if err != nil {
		return nil, err
	}
	if g.member != j.member || g.person != j.person || g.kind != "group" {
		return nil, &denied{403, "sharing_unavailable"}
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_ai_invocations WHERE id=$1 AND status='running' AND lease_id=$2 AND lease_until>clock_timestamp() AND share_expires_at>clock_timestamp())`, j.id, j.lease).Scan(&live); err != nil {
		return nil, err
	}
	if !live {
		return nil, &denied{409, "invocation_cancelled"}
	}
	// The catalogue is public. Never select chat, roster, taste, or outing history.
	//
	// Filtered to one destination, the same one v1 used: the first by sort order.
	// Without it the model was handed the first 40 rows by id, so asking about
	// Hà Nội could be answered entirely out of Đà Nẵng. The NOT EXISTS arm keeps
	// v1's behaviour on a database with no destinations at all, where the filter
	// has nothing to mean and every place is a candidate.
	rows, err := tx.Query(ctx, `WITH mac_dinh AS (SELECT id FROM destinations ORDER BY sort_order, id LIMIT 1)
		SELECT jsonb_strip_nulls(jsonb_build_object('id',id,'name',name,'address',address,'price_min_vnd',price_min_vnd,'price_max_vnd',price_max_vnd,'open_hours',open_hours,'category',category))
		FROM places
		WHERE NOT EXISTS (SELECT 1 FROM mac_dinh) OR destination_id = (SELECT id FROM mac_dinh)
		ORDER BY id LIMIT 40`)
	if err != nil {
		return nil, err
	}
	cards := []*pyjson.OrderedMap{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			rows.Close()
			return nil, err
		}
		v, e := pyjson.Loads(b)
		if e != nil {
			rows.Close()
			return nil, e
		}
		card, ok := v.(*pyjson.OrderedMap)
		if !ok {
			rows.Close()
			return nil, fmt.Errorf("catalogue row is %T, not an object", v)
		}
		cards = append(cards, card)
	}
	rows.Close()
	// Every other path that hands the catalogue to a model runs this filter;
	// this one did not, which made a place row the one way an instruction could
	// reach the model from outside a conversation. A name reading "bỏ qua hướng
	// dẫn phía trên" travelled untouched from here and from nowhere else.
	out := pyjson.List{}
	for _, card := range treejson.MapsFrom(promptsafety.Filter(treejson.MapsTo(cards))) {
		out = append(out, card)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func (h *Handler) finishFailure(ctx context.Context, j work, code string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := h.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code=$3,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND status='running' AND lease_id=$2`, j.id, j.lease, code)
	return err
}

func (h *Handler) publish(ctx context.Context, j work, card json.RawMessage) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	g, err := authority(ctx, tx, j.conversation, j.digest)
	if err != nil || g.member != j.member || g.person != j.person || g.kind != "group" {
		_ = tx.Rollback(ctx)
		return h.finishFailure(ctx, j, "sharing_unavailable")
	}
	if err = lockFeed(ctx, tx, j.conversation); err != nil {
		return err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id FROM chat_ai_invocations WHERE id=$1 AND status='running' AND lease_id=$2 AND lease_until>clock_timestamp() AND share_expires_at>clock_timestamp() FOR UPDATE`, j.id, j.lease).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Message creation and the job's terminal transition commit together. The
	// change-feed trigger captures the card in this same transaction.
	message, err := (repo.Repository{Q: tx}).CreateMessage(ctx, repo.MessageInput{ContextID: j.conversation, Kind: "ai_card", Card: card, Now: time.Now().UTC()})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET status='succeeded',message_id=$3,prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_id=$2`, j.id, j.lease, message.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
