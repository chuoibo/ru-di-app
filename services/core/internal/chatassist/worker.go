package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
)

type work struct {
	id, conversation, person, member, prompt, lease, command string
	// `group` or `me`. A personal job has no room and no membership, so
	// conversation and member are empty for it.
	scope  string
	digest []byte
	// The context the caller handed over, exactly as it was stored. Nil when the
	// caller sent none, which is still the shape an older client produces.
	goi []byte
	// The `@Rủ Đi` message the answer replies to; "" for a job from a client
	// that names none, which then publishes the card it always did.
	trigger string
	// How many shared turns the server confirmed at create (so_tin_doc).
	soTin int
	// When the question was stored: the only «now» the Go engine reads.
	createdAt time.Time
	// Which attempt this claim is (1 for the first).
	attempt int
}

// WorkerConfig sizes the inference workers. The defaults are what the engine
// ran with before it could be sized: two workers, a 75 second lease.
type WorkerConfig struct {
	// Workers is how many jobs this process runs at once.
	Workers int
	// Tick is how often an idle worker looks for a job.
	Tick time.Duration
	// Lease is how long a claim holds a job without a heartbeat.
	Lease time.Duration
	// Heartbeat renews the lease while a job runs, and notices a job that was
	// cancelled or revoked underneath it.
	Heartbeat time.Duration
	// SweepEvery runs the retention and lease sweeps.
	SweepEvery time.Duration
}

// Environment variables WorkerConfigFromEnv reads.
const (
	EnvWorkers      = "MOBILE_AI_WORKERS"
	EnvLeaseSeconds = "MOBILE_AI_LEASE_SECONDS"
)

// DefaultWorkerConfig is two workers on a 75 second lease. The lease stays 75
// seconds until every process that can claim a job renews it by heartbeat: a
// shorter lease on a replica without heartbeat would let a second worker take
// a job the first is still running.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{Workers: 2, Tick: 250 * time.Millisecond, Lease: 75 * time.Second, Heartbeat: 5 * time.Second, SweepEvery: 5 * time.Second}
}

// WorkerConfigFromEnv reads MOBILE_AI_WORKERS (1..64) and
// MOBILE_AI_LEASE_SECONDS (15..300). Anything else is refused, never clamped:
// a typo in a worker count is a startup error, not a quietly different fleet.
func WorkerConfigFromEnv(getenv func(string) string) (WorkerConfig, error) {
	cfg := DefaultWorkerConfig()
	if raw := getenv(EnvWorkers); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 64 {
			return WorkerConfig{}, fmt.Errorf("%s must be an integer from 1 to 64, got %q", EnvWorkers, raw)
		}
		cfg.Workers = n
	}
	if raw := getenv(EnvLeaseSeconds); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 15 || n > 300 {
			return WorkerConfig{}, fmt.Errorf("%s must be an integer from 15 to 300, got %q", EnvLeaseSeconds, raw)
		}
		cfg.Lease = time.Duration(n) * time.Second
	}
	// At least three renewals fit in one lease, so one slow statement never
	// lets a live job lapse.
	if third := cfg.Lease / 3; cfg.Heartbeat > third {
		cfg.Heartbeat = third
	}
	return cfg, nil
}

// WithWorker sets the worker configuration. New uses DefaultWorkerConfig.
func (h *Handler) WithWorker(cfg WorkerConfig) *Handler {
	h.worker = cfg
	return h
}

// Run is RunSweeper plus RunWorkers: what a process that both serves and
// works runs. Leases recover a crashed worker; retries may repeat inference,
// but publication is transactionally once-only.
func (h *Handler) Run(ctx context.Context) {
	var both sync.WaitGroup
	both.Add(2)
	go func() { defer both.Done(); h.RunSweeper(ctx) }()
	go func() { defer both.Done(); h.RunWorkers(ctx) }()
	both.Wait()
}

// RunWorkers owns the bounded inference workers.
func (h *Handler) RunWorkers(ctx context.Context) {
	var workers sync.WaitGroup
	for i := 0; i < h.worker.Workers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTicker(h.worker.Tick)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					j, ok, err := h.claimNext(ctx, "")
					if err == nil && ok {
						_ = h.runJob(ctx, j)
					}
				}
			}
		}()
	}
	workers.Wait()
}

// RunSweeper runs the retention and lease sweeps on their own clock. A process
// that serves but runs no workers still runs it: the fifteen-minute bound on
// shared plaintext must hold even when the worker fleet is scaled to zero.
func (h *Handler) RunSweeper(ctx context.Context) {
	timer := time.NewTicker(h.worker.SweepEvery)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_ = h.Sweep(ctx)
		}
	}
}

// Sweep bounds plaintext retention to the sharing window and fails jobs whose
// last lease lapsed with no attempts left.
func (h *Handler) Sweep(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Bound plaintext retention to the explicit sharing window, including failed jobs.
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET prompt=NULL,boi_canh=NULL,status=CASE WHEN status IN ('queued','running') THEN 'failed' ELSE status END,code=CASE WHEN status IN ('queued','running') THEN 'sharing_expired' ELSE code END,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE prompt IS NOT NULL AND share_expires_at<=clock_timestamp()`)
	if err != nil {
		return err
	}
	// A sealed personal answer is delivered, not kept: it goes when the sharing
	// window it was produced under closes (ADR-0036 §2.8).
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET result=NULL,updated_at=clock_timestamp() WHERE scope='me' AND result IS NOT NULL AND share_expires_at<=clock_timestamp()`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code='worker_interrupted',lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE status='running' AND lease_until<clock_timestamp() AND attempts>=3`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// claim is Sweep then claimNext: the deterministic entry the PostgreSQL gates
// drive, with the sweeps a worker used to run on every tick.
func (h *Handler) claim(ctx context.Context) (work, bool, error) {
	if err := h.Sweep(ctx); err != nil {
		return work{}, false, err
	}
	return h.claimNext(ctx, "")
}

// ClaimByID claims one named job, the entry a broker consumer uses when a
// message names the job to run. It claims only what claimNext would: a queued
// job, or a running one whose lease lapsed, with attempts left and its sharing
// window open. False with no error means someone else has it, or it is done.
func (h *Handler) ClaimByID(ctx context.Context, id string) (bool, error) {
	j, ok, err := h.claimNext(ctx, id)
	if err != nil || !ok {
		return ok, err
	}
	return true, h.runJob(ctx, j)
}

// claimNext takes the oldest claimable job, or exactly the job named by id.
func (h *Handler) claimNext(ctx context.Context, id string) (work, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return work{}, false, err
	}
	defer tx.Rollback(ctx)
	var j work
	j.lease = newID()
	err = tx.QueryRow(ctx, `WITH candidate AS (SELECT id FROM chat_ai_invocations WHERE (status='queued' OR (status='running' AND lease_until<clock_timestamp())) AND attempts<3 AND share_expires_at>clock_timestamp() AND ($3='' OR id::text=$3) ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE chat_ai_invocations j SET status='running',attempts=attempts+1,lease_id=$1,lease_until=clock_timestamp()+make_interval(secs => $2),updated_at=clock_timestamp() FROM candidate c WHERE j.id=c.id RETURNING j.id,j.scope,COALESCE(j.context_id::text,''),j.person_id,COALESCE(j.membership_id::text,''),j.session_digest,j.prompt,j.boi_canh,j.command,COALESCE(j.trigger_message_id::text,''),COALESCE(j.so_tin_doc,0),j.created_at,j.attempts`, j.lease, h.worker.Lease.Seconds(), id).Scan(&j.id, &j.scope, &j.conversation, &j.person, &j.member, &j.digest, &j.prompt, &j.goi, &j.command, &j.trigger, &j.soTin, &j.createdAt, &j.attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return work{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return j, false, err
	}
	return j, true, tx.Commit(ctx)
}

// heartbeat renews the job's lease until stop is called. When the renewal
// finds the job no longer running under this lease -- cancelled, revoked with
// its membership, or taken over after a lapse -- it cancels the job's context,
// so the model call stops within one beat instead of running to its timeout.
// A failed statement is retried on the next beat; only a definite "not ours
// any more" cancels.
func (h *Handler) heartbeat(ctx context.Context, j work, cancelJob context.CancelFunc) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		timer := time.NewTicker(h.worker.Heartbeat)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-timer.C:
				beat, cancel := context.WithTimeout(ctx, 2*time.Second)
				tag, err := h.pool.Exec(beat, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()+make_interval(secs => $3) WHERE id=$1 AND lease_id=$2 AND status='running'`, j.id, j.lease, h.worker.Lease.Seconds())
				cancel()
				if err == nil && tag.RowsAffected() == 0 {
					cancelJob()
					return
				}
			}
		}
	}()
	return func() { close(done); <-finished }
}

// runJob runs one claimed job under its time budget and heartbeat.
func (h *Handler) runJob(ctx context.Context, j work) error {
	ctx, cancelJob := context.WithTimeout(ctx, 70*time.Second)
	defer cancelJob()
	stop := h.heartbeat(ctx, j, cancelJob)
	defer stop()
	return h.process(ctx, j)
}

// ProcessOne is also the deterministic worker entry point for PostgreSQL gates.
func (h *Handler) ProcessOne(ctx context.Context) (bool, error) {
	j, ok, err := h.claim(ctx)
	if err != nil || !ok {
		return ok, err
	}
	return true, h.runJob(ctx, j)
}

// process runs one claimed job to its terminal transition.
func (h *Handler) process(ctx context.Context, j work) error {
	// Before prepare: a personal job has no room, and nothing the server owns
	// about a room is laid on top of it (ADR-0036 §4).
	if j.scope == scopeMe {
		return h.processNep(ctx, j)
	}
	dap, err := h.prepare(ctx, j)
	if err != nil {
		return h.finishFailure(ctx, j, "sharing_unavailable")
	}
	if j.command == lenhChiaBill {
		return h.processChiaBill(ctx, j, dap)
	}
	conversation, err := hoiThoai(j.goi, j.prompt, dap.toi)
	if err != nil {
		return h.finishFailure(ctx, j, "invalid_ai_result")
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
		return h.finishFailure(ctx, j, "provider_unavailable")
	}
	card, err := theCuaViec(j, treejson.To(raw), treejson.MapsTo(dap.places))
	if err != nil {
		return h.finishFailure(ctx, j, "invalid_ai_result")
	}
	return h.publish(ctx, j, card, nil)
}

// dapThem is what the server lays on top of the caller's bundle (ADR-0036
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
	// chia_bill only: who wrote each shared turn (message id -> person id),
	// read from `messages.author_id`, never from the bundle's own claim.
	authors map[string]string
	// chia_bill only: the active members, the proposed "shared by" of every
	// draft, as v1 proposed it.
	memberships []repo.Membership
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
	store := repo.Repository{Q: tx}
	if j.command == lenhChiaBill {
		// Splitting a bill needs who is in the room and who wrote what; it has
		// no use for taste, budget or the catalogue, so it never reads them.
		out := dapThem{authors: map[string]string{}}
		if out.memberships, err = store.ListMembers(ctx, j.conversation); err != nil {
			return dapThem{}, err
		}
		if len(j.goi) > 0 {
			var bc bundle
			if err = json.Unmarshal(j.goi, &bc); err != nil {
				return dapThem{}, err
			}
			if out.authors, err = tacGia(ctx, tx, j.conversation, &bc); err != nil {
				return dapThem{}, err
			}
		}
		return out, tx.Commit(ctx)
	}
	// Roster, taste, budget and the public catalogue: the four things the
	// server owns and never encrypted. Never the conversation.
	//
	// The catalogue is the one v1 handed the model, computed by the same code:
	// the default destination's places, ranked by the group's own taste, cut
	// to forty, through promptsafety. The earlier version here took the first
	// forty rows by id, so a group that only drinks coffee could be handed
	// forty restaurants and no café, and the model had nothing better to pick.
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

// publish posts the card and closes the job in one transaction. result is the
// structured outcome kept on the invocation row (chia_bill's drafts); nil
// leaves the column NULL, which is what a plan job has always stored.
func (h *Handler) publish(ctx context.Context, j work, card json.RawMessage, result json.RawMessage) error {
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
	// The feed head first, then the trigger, then the job: the order every Go
	// chat write takes (chatlegacychange.BeforeWrite). See giuTrigger.
	if err = lockFeed(ctx, tx, j.conversation); err != nil {
		return err
	}
	there, err := giuTrigger(ctx, tx, j)
	if err != nil {
		return err
	}
	if !there {
		_ = tx.Rollback(ctx)
		return h.finishFailure(ctx, j, "trigger_deleted")
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
	// change-feed trigger captures the card in this same transaction. With a
	// trigger the card is a reply to it, the way a member answers in a thread;
	// the author stays NULL, and the card itself says who wrote it.
	input := repo.MessageInput{ContextID: j.conversation, Kind: "ai_card", Card: card, Now: time.Now().UTC()}
	if j.trigger != "" {
		input.ReplyToID = &j.trigger
	}
	message, err := (repo.Repository{Q: tx}).CreateMessage(ctx, input)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET status='succeeded',message_id=$3,result=$4,prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_id=$2`, j.id, j.lease, message.ID, ketQuaHoacNull(result))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
