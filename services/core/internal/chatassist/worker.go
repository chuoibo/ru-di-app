package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/jobs"
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
	// Which entry into the queue this claim took (enqueue_seq).
	seq int64
	// Model calls earlier attempts of this job already spent (model_calls).
	modelCalls int
}

// WorkerConfig sizes the inference workers. The defaults are what the engine
// ran with before it could be sized: two workers, a 75 second lease.
type WorkerConfig struct {
	// Workers is how many jobs this process runs at once, whichever path
	// claimed them (the broker's consumer or the Postgres poller).
	Workers int
	// Tick is how often the poller looks for any due job while no broker
	// consumer is attached.
	Tick time.Duration
	// NetEvery is how often the poller looks for jobs due for longer than
	// NetLag, broker or not: the net under a lost message or a lapsed lease.
	NetEvery time.Duration
	NetLag   time.Duration
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
	return WorkerConfig{Workers: 2, Tick: 250 * time.Millisecond, NetEvery: 2 * time.Second, NetLag: 5 * time.Second,
		Lease: 75 * time.Second, Heartbeat: 5 * time.Second, SweepEvery: 5 * time.Second}
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
// Call it before the workers start.
func (h *Handler) WithWorker(cfg WorkerConfig) *Handler {
	h.worker = cfg
	return h
}

// The queues this engine's jobs ride, one per scope: the same CASE as the
// enqueue trigger (schema_hang_doi.sql).
const (
	HangNhom = "ai.group"
	HangNep  = "ai.nep"
)

// Queues are the queues a worker of this engine may consume.
var Queues = []string{HangNhom, HangNep}

func scopeCuaHang(queue string) (string, bool) {
	switch queue {
	case HangNhom:
		return "group", true
	case HangNep:
		return scopeMe, true
	}
	return "", false
}

// WithQueues limits this process to the jobs of these queues, on the broker
// and in the poller alike: a worker that consumes only ai.nep must not poll a
// group job either. Unset means both.
func (h *Handler) WithQueues(queues []string) (*Handler, error) {
	var scopes []string
	for _, q := range queues {
		s, ok := scopeCuaHang(q)
		if !ok {
			return nil, fmt.Errorf("chatassist: no jobs ride queue %q", q)
		}
		scopes = append(scopes, s)
	}
	if len(scopes) == 0 {
		return nil, errors.New("chatassist: a worker needs at least one queue")
	}
	h.scopes = scopes
	return h, nil
}

func (h *Handler) scopeList() []string {
	if h.scopes == nil {
		return []string{"group", scopeMe}
	}
	return h.scopes
}

// WithNhipPool gives the heartbeat and the per-call model counter a pool of
// their own (design 02 §6): two statements that must not wait behind the
// jobs' own queries when the main pool is busy. Unset, they use the main
// pool.
func (h *Handler) WithNhipPool(p *pgxpool.Pool) *Handler {
	h.nhip = p
	return h
}

func (h *Handler) nhipPool() *pgxpool.Pool {
	if h.nhip != nil {
		return h.nhip
	}
	return h.pool
}

// The process's job slots, shared by the consumer and the poller.
func (h *Handler) slotChan() chan struct{} {
	h.slotsOnce.Do(func() { h.slots = make(chan struct{}, h.worker.Workers) })
	return h.slots
}

func (h *Handler) tryAcquire() bool {
	select {
	case h.slotChan() <- struct{}{}:
		return true
	default:
		return false
	}
}

func (h *Handler) acquire(ctx context.Context) bool {
	select {
	case h.slotChan() <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (h *Handler) releaseSlot() { <-h.slotChan() }

// Broker is what the poller needs to know of the broker side: whether its
// consumers are attached right now (jobs.Ket).
type Broker interface{ Song() bool }

// RunWorkers is the Postgres poller, and it always runs (design 02 §4 step
// 6). Every NetEvery it claims jobs due for longer than NetLag -- a message
// that never came, a lease that lapsed. While broker is nil or not attached
// it also claims every Tick any job that is due. It claims only while a slot
// is free, and each job runs under its lease and heartbeat. It returns once
// ctx ends and every job it started has finished or been released.
func (h *Handler) RunWorkers(ctx context.Context, broker Broker) {
	fast := time.NewTicker(h.worker.Tick)
	defer fast.Stop()
	net := time.NewTicker(h.worker.NetEvery)
	defer net.Stop()
	var running sync.WaitGroup
	defer running.Wait()
	for {
		select {
		case <-ctx.Done():
			return
		case <-fast.C:
			if brokerUp(broker) {
				continue
			}
			h.poll(ctx, 0, broker, &running)
		case <-net.C:
			h.poll(ctx, h.worker.NetLag, broker, &running)
		}
	}
}

func brokerUp(b Broker) bool { return b != nil && b.Song() }

// poll claims due jobs while a slot is free, and starts each one. While the
// broker is down, a slot that finishes its job claims the next due one at
// once instead of waiting for the next tick: the tick only bounds how long a
// job waits when every slot was idle.
func (h *Handler) poll(ctx context.Context, lag time.Duration, broker Broker, running *sync.WaitGroup) {
	for ctx.Err() == nil {
		if !h.tryAcquire() {
			return
		}
		j, ok, err := h.claimNext(ctx, lag)
		if err != nil || !ok {
			h.releaseSlot()
			return
		}
		running.Add(1)
		go func() {
			defer running.Done()
			defer h.releaseSlot()
			for {
				_ = h.runJob(ctx, j)
				if ctx.Err() != nil || brokerUp(broker) {
					return
				}
				next, ok, err := h.claimNext(ctx, 0)
				if err != nil || !ok {
					return
				}
				j = next
			}
		}()
	}
}

// XuLyTin is the broker consumer's handler (jobs.Ket.Handler). It claims the
// job the message names at the enqueue it names, and runs it. It returns nil
// once the job's terminal transaction committed, or when the claim found
// nothing to do -- a duplicate, a job done or cancelled, a message from an
// earlier entry into the queue, a job another worker holds -- and the message
// is acknowledged then. A database error is jobs.ErrTamDung: the message is
// requeued and the consumer pauses while the poller carries on.
func (h *Handler) XuLyTin(ctx context.Context, queue string, m jobs.Message) error {
	scope, ok := scopeCuaHang(queue)
	if !ok {
		return jobs.ErrMalformed
	}
	if !h.acquire(ctx) {
		return ctx.Err()
	}
	defer h.releaseSlot()
	j, ok, err := h.claimTin(ctx, m.Ref, m.Seq, scope)
	if err != nil {
		return errors.Join(jobs.ErrTamDung, err)
	}
	if !ok {
		return nil
	}
	if err = h.runJob(ctx, j); err != nil && !errors.Is(err, aiharness.ErrHuy) {
		return errors.Join(jobs.ErrTamDung, err)
	}
	return nil
}

// DinhKy is the sweep as a periodic task (jobs.DinhKy). Every process that
// serves or works runs it, so the fifteen-minute bound on shared plaintext
// holds even with the worker fleet scaled to zero.
func (h *Handler) DinhKy() jobs.DinhKy {
	return jobs.DinhKy{Ten: "chatassist.sweep", Nhip: h.worker.SweepEvery, Chay: func(ctx context.Context, _ *pgxpool.Pool) error {
		return h.Sweep(ctx)
	}}
}

// Sweep bounds plaintext retention to the sharing window and fails the jobs a
// lapsed lease left behind with no way forward: no attempts left, or content
// already out (a second worker must not write the answer again). One process
// sweeps at a time: the pass holds pg_try_advisory_xact_lock, and a process
// that finds it taken skips, since the holder does the same idempotent work.
func (h *Handler) Sweep(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var mine bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('chatassist:sweep',0))`).Scan(&mine); err != nil {
		return err
	}
	if !mine {
		return tx.Commit(ctx)
	}
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
	// Lease after first content (design 02 §4 step 7): a worker that died
	// after its first part or delta left is not replaced; the job fails, what
	// was shown stays, and the reader is told it was cut off.
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code='worker_interrupted',lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE status='running' AND lease_until<clock_timestamp() AND (attempts>=3 OR first_token_at IS NOT NULL)`)
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
	return h.claimNext(ctx, 0)
}

// ClaimByID claims the job a broker message names, at the enqueue it names,
// and runs it: claimByID(ref, seq) of design 02 §4 step 3. False with no error
// means there was nothing to run.
func (h *Handler) ClaimByID(ctx context.Context, id string, seq int64) (bool, error) {
	j, ok, err := h.claimTin(ctx, id, seq, "")
	if err != nil || !ok {
		return ok, err
	}
	return true, h.runJob(ctx, j)
}

// What makes a job claimable at all: queued, or running under a lapsed lease
// before any content left it; attempts left; its sharing window open.
const claimable = `(status='queued' OR (status='running' AND lease_until<clock_timestamp() AND first_token_at IS NULL)) AND attempts<3 AND share_expires_at>clock_timestamp()`

// The one UPDATE every claim runs; only the choice of candidate differs.
const claimSet = `UPDATE chat_ai_invocations j SET status='running',attempts=attempts+1,lease_id=$1,lease_until=clock_timestamp()+make_interval(secs => $2),updated_at=clock_timestamp() FROM candidate c WHERE j.id=c.id RETURNING j.id,j.scope,COALESCE(j.context_id::text,''),j.person_id,COALESCE(j.membership_id::text,''),j.session_digest,j.prompt,j.boi_canh,j.command,COALESCE(j.trigger_message_id::text,''),COALESCE(j.so_tin_doc,0),j.created_at,j.attempts,j.enqueue_seq,j.model_calls`

// claimPoll takes the oldest claimable job due at least $3 seconds ago, of the
// scopes in $4.
const claimPoll = `WITH candidate AS (SELECT id FROM chat_ai_invocations WHERE ` + claimable + ` AND available_at<=clock_timestamp()-make_interval(secs => $3) AND scope=ANY($4) ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1) ` + claimSet

// claimMessage takes exactly the job $3 at enqueue $4, when it is due. $5 is
// the scope its queue carries ("" for any): a message cannot claim a job of
// another queue.
const claimMessage = `WITH candidate AS (SELECT id FROM chat_ai_invocations WHERE id=$3 AND enqueue_seq=$4 AND ($5='' OR scope=$5) AND ` + claimable + ` AND available_at<=clock_timestamp() FOR UPDATE SKIP LOCKED) ` + claimSet

// claimNext takes the oldest claimable job of this process's queues due at
// least lag ago.
func (h *Handler) claimNext(ctx context.Context, lag time.Duration) (work, bool, error) {
	return h.claimWith(ctx, claimPoll, lag.Seconds(), h.scopeList())
}

// claimTin takes the job a message names, at the enqueue it names.
func (h *Handler) claimTin(ctx context.Context, id string, seq int64, scope string) (work, bool, error) {
	return h.claimWith(ctx, claimMessage, id, seq, scope)
}

func (h *Handler) claimWith(ctx context.Context, sql string, args ...any) (work, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return work{}, false, err
	}
	defer tx.Rollback(ctx)
	var j work
	j.lease = newID()
	err = tx.QueryRow(ctx, sql, append([]any{j.lease, h.worker.Lease.Seconds()}, args...)...).Scan(&j.id, &j.scope, &j.conversation, &j.person, &j.member, &j.digest, &j.prompt, &j.goi, &j.command, &j.trigger, &j.soTin, &j.createdAt, &j.attempt, &j.seq, &j.modelCalls)
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
				tag, err := h.nhipPool().Exec(beat, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()+make_interval(secs => $3) WHERE id=$1 AND lease_id=$2 AND status='running'`, j.id, j.lease, h.worker.Lease.Seconds())
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

// Why a job's context was cancelled, when it was not its own clock.
var (
	// errDungTho: the worker is stopping (SIGTERM). The job goes back to the
	// queue rather than failing.
	errDungTho = errors.New("chatassist: the worker is stopping")
	// errMatLease: the heartbeat found the job no longer this worker's.
	errMatLease = errors.New("chatassist: the job's lease is gone")
)

// dangDung reports whether ctx was cancelled because the worker is stopping.
func dangDung(ctx context.Context) bool { return errors.Is(context.Cause(ctx), errDungTho) }

// runJob runs one claimed job under its time budget and heartbeat. The job's
// context is its own: stopping the worker (ctx ending) cancels it with
// errDungTho, and whatever the job was doing then, it is released back to the
// queue instead of failed (design 02 §4 step 9). The release matches nothing
// when the job had already ended, or had content out.
func (h *Handler) runJob(ctx context.Context, j work) error {
	jobCtx, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	defer cancel(nil)
	stopWatch := context.AfterFunc(ctx, func() { cancel(errDungTho) })
	defer stopWatch()
	jobCtx, cancelTime := context.WithTimeout(jobCtx, 70*time.Second)
	defer cancelTime()
	stop := h.heartbeat(jobCtx, j, func() { cancel(errMatLease) })
	defer stop()
	err := h.process(jobCtx, j)
	if dangDung(jobCtx) {
		return h.release(jobCtx, j)
	}
	return err
}

// release hands a job back to the queue when its worker stops: queued, the
// attempt it used given back, the lease cleared, due now. The enqueue trigger
// numbers the new entry, so another worker gets a message for it at once
// instead of waiting on the poller. A job whose content already went out is
// not released -- no second worker may start it over -- and the sweep fails
// it as worker_interrupted once its lease lapses.
func (h *Handler) release(ctx context.Context, j work) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := h.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='queued',attempts=attempts-1,lease_id=NULL,lease_until=NULL,available_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1 AND lease_id=$2 AND status='running' AND first_token_at IS NULL`, j.id, j.lease)
	return err
}

// retryLater puts a job whose turn failed on a transient provider error back
// in the queue after a backoff, when every condition of design 02 §4 step 5
// holds: no content out yet, attempts left, model calls left, and the retry
// still inside thirty seconds of the question. It reports whether it did; the
// caller fails the job otherwise. The trigger enqueues it, due at the end of
// the backoff, so the relay publishes it only then.
func (h *Handler) retryLater(ctx context.Context, j work) (bool, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tag, err := h.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='queued',lease_id=NULL,lease_until=NULL,available_at=clock_timestamp()+make_interval(secs => $3),updated_at=clock_timestamp() WHERE id=$1 AND lease_id=$2 AND status='running' AND first_token_at IS NULL AND attempts<3 AND model_calls<$4 AND clock_timestamp()+make_interval(secs => $3)<created_at+interval '30 seconds'`, j.id, j.lease, choLai(j.attempt).Seconds(), llm.MaxModelCallsPerTurn)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// choLai is the wait before the next attempt: one second after the first,
// four after the second, each within ±20 % so retries of one outage spread.
func choLai(attempt int) time.Duration {
	base := time.Second
	if attempt >= 2 {
		base = 4 * time.Second
	}
	return time.Duration(float64(base) * (0.8 + 0.4*rand.Float64()))
}

// giuLuot is Turn.GiuLuot for j (design 01 §2, design 02 §6): one model call
// taken in the job's row before it goes out, under this worker's lease, never
// past llm.MaxModelCallsPerTurn across every attempt of the job. Zero rows --
// the ceiling, or a lease gone -- is no call.
func (h *Handler) giuLuot(j work) func(context.Context) error {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var n int
		err := h.nhipPool().QueryRow(ctx, `UPDATE chat_ai_invocations SET model_calls=model_calls+1 WHERE id=$1 AND lease_id=$2 AND model_calls<$3 RETURNING model_calls`, j.id, j.lease, llm.MaxModelCallsPerTurn).Scan(&n)
		if errors.Is(err, pgx.ErrNoRows) {
			return llm.ErrHetNganSach
		}
		return err
	}
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

// finishFailure fails a group job with code -- unless the worker is stopping,
// in which case whatever failed failed because of the stop, and the job is
// released instead.
func (h *Handler) finishFailure(ctx context.Context, j work, code string) error {
	if dangDung(ctx) {
		return h.release(ctx, j)
	}
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
