package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
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
	dap, err := h.prepare(ctx, j)
	if err != nil {
		return true, h.finishFailure(ctx, j, "sharing_unavailable")
	}
	conversation, err := hoiThoai(j.goi, j.prompt, dap.toi)
	if err != nil {
		return true, h.finishFailure(ctx, j, "invalid_ai_result")
	}
	catalogue := pyjson.List{}
	for _, place := range dap.places {
		catalogue = append(catalogue, place)
	}
	payload := pyjson.NewOrderedMap()
	payload.Set("conversation", conversation)
	payload.Set("members", dap.members)
	payload.Set("places", catalogue)
	if dap.budget == nil {
		payload.Set("budget_per_person_vnd", pyjson.Null{})
	} else {
		payload.Set("budget_per_person_vnd", pyjson.NewInt(*dap.budget))
	}
	inference, cancel := context.WithTimeout(ctx, 60*time.Second)
	raw, err := h.brain.PostJSONContext(inference, "companion-reply", payload)
	cancel()
	if err != nil {
		return true, h.finishFailure(ctx, j, "provider_unavailable")
	}
	grounded, err := companion.GroundCard(treejson.To(raw), treejson.MapsTo(dap.places))
	if err != nil {
		return true, h.finishFailure(ctx, j, "invalid_ai_result")
	}
	card, err := pyjson.Dumps(treejson.From(grounded))
	if err != nil {
		return true, h.finishFailure(ctx, j, "invalid_ai_result")
	}
	return true, h.publish(ctx, j, card)
}

// dapThem is what the server lays on top of the caller's bundle (ADR-0034
// §2.3): only things it owns and never encrypted. It never holds a word of the
// conversation; that arrives from the client or not at all.
type dapThem struct {
	// The catalogue the model may choose from, best match for the group first.
	places []*pyjson.OrderedMap
	// Who is in the room, by display name where one is safe (see roster).
	members pyjson.List
	// The caller's label in that roster, which the transcript uses too.
	toi string
	// The group's stated per-person budget, nil when nobody answered.
	budget *int64
}

func (h *Handler) prepare(ctx context.Context, j work) (dapThem, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return dapThem{}, err
	}
	defer tx.Rollback(ctx)
	g, err := authority(ctx, tx, j.conversation, j.digest)
	if err != nil {
		return dapThem{}, err
	}
	if g.member != j.member || g.person != j.person || g.kind != "group" {
		return dapThem{}, &denied{403, "sharing_unavailable"}
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_ai_invocations WHERE id=$1 AND status='running' AND lease_id=$2 AND lease_until>clock_timestamp() AND share_expires_at>clock_timestamp())`, j.id, j.lease).Scan(&live); err != nil {
		return dapThem{}, err
	}
	if !live {
		return dapThem{}, &denied{409, "invocation_cancelled"}
	}
	// Roster, taste, budget and the public catalogue: the four things the
	// server owns and never encrypted. Never the conversation.
	//
	// The catalogue is the one v1 handed the model, computed by the same code:
	// the default destination's places, ranked by the group's own taste, cut
	// to forty, through promptsafety. The earlier version here took the first
	// forty rows by id, so a group that only drinks coffee could be handed
	// forty restaurants and no café, and the model had nothing better to pick.
	store := repo.Repository{Q: tx}
	group, err := service.GroupTaste(ctx, store, j.conversation, time.Now().UTC())
	if err != nil {
		return dapThem{}, err
	}
	places, err := service.ModelPlaceRows(ctx, store, group)
	if err != nil {
		return dapThem{}, err
	}
	members, toi, err := roster(ctx, tx, store, j.conversation, j.person, j.goi)
	if err != nil {
		return dapThem{}, err
	}
	return dapThem{places: places, members: members, toi: toi, budget: group.BudgetPerPersonVND}, tx.Commit(ctx)
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
