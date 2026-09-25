//go:build postgres

package chatassist

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/llm"
)

// The queue's database half (slice 10; design 02 §3.2, §4, §9): the enqueue
// triggers, the claim by (id, enqueue_seq), the poller's due time, the lease
// after first content, retryLater and release, and the model-call ceiling in
// the row. No broker here: the outbox row is what these tests read.

// The trigger enqueues when, and only when, a job enters 'queued': a new
// question, /retry, retryLater, release. A heartbeat, a cancel and a finished
// job never do (M1 turns red at the heartbeat step).
func TestTriggerVaoHangChiKhiVaoQueued(t *testing.T) {
	f := setup(t, nil)
	f.handler.WithWorker(fastWorker())
	ctx := context.Background()
	var het time.Time
	g := f.create(t)
	if err := f.pool.QueryRow(ctx, `SELECT share_expires_at FROM chat_ai_invocations WHERE id=$1`, g.ID).Scan(&het); err != nil {
		t.Fatal(err)
	}
	coHang := func(buoc, id string, want ...int64) []dongOutbox {
		t.Helper()
		rows := f.outbox(t, id)
		var seqs []int64
		for _, r := range rows {
			seqs = append(seqs, r.seq)
		}
		if fmt.Sprint(seqs) != fmt.Sprint(want) {
			t.Fatalf("%s: outbox %v, muốn %v", buoc, seqs, want)
		}
		return rows
	}
	rows := coHang("tạo (nhóm)", g.ID, 1)
	if rows[0].queue != HangNhom || rows[0].expires == nil || !rows[0].expires.Equal(het) {
		t.Fatalf("tạo: %+v, muốn ai.group hết hạn cùng cửa sổ chia sẻ %v", rows[0], het)
	}
	code, nep, raw := f.nepPost(t, f.token, nepThan("tối nay đi đâu?"))
	if code != 202 {
		t.Fatalf("nep: %d %s", code, raw)
	}
	if rows := coHang("tạo (Nếp)", nep.ID, 1); rows[0].queue != HangNep {
		t.Fatalf("tạo Nếp: %+v", rows[0])
	}

	j, ok, err := f.handler.claimTin(ctx, g.ID, 1, "group")
	if err != nil || !ok || j.seq != 1 {
		t.Fatalf("claim: %v %v %+v", ok, err, j)
	}
	coHang("claim", g.ID, 1)
	lease := leaseUntil(t, f, g.ID)
	stop := f.handler.heartbeat(ctx, j, func() { t.Error("heartbeat lost a lease it holds") })
	time.Sleep(350 * time.Millisecond)
	stop()
	if later := leaseUntil(t, f, g.ID); !later.After(lease) {
		t.Fatal("heartbeat did not renew; this step would prove nothing")
	}
	coHang("heartbeat", g.ID, 1)
	if _, _, seq, _ := f.trangThai(t, g.ID); seq != 1 {
		t.Fatalf("heartbeat moved enqueue_seq to %d", seq)
	}

	if err = f.handler.finishFailure(ctx, j, "provider_unavailable"); err != nil {
		t.Fatal(err)
	}
	coHang("thất bại", g.ID, 1)
	requireCode(t, f.request("POST", f.route()+"/"+g.ID+"/retry", f.token, nil), 200)
	coHang("/retry", g.ID, 1, 2)

	if j, ok, err = f.handler.claimTin(ctx, g.ID, 2, "group"); err != nil || !ok {
		t.Fatalf("claim seq 2: %v %v", ok, err)
	}
	if later, err := f.handler.retryLater(ctx, j); err != nil || !later {
		t.Fatalf("retryLater: %v %v", later, err)
	}
	rows = coHang("retryLater", g.ID, 1, 2, 3)
	// The second attempt waits 4 s ±20 % before the third.
	if cho := time.Until(rows[2].due); cho < 3*time.Second || cho > 5*time.Second {
		t.Fatalf("retryLater: outbox row due in %v, want the second backoff, about 4 s", cho)
	}

	if _, err = f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET available_at=clock_timestamp() WHERE id=$1`, g.ID); err != nil {
		t.Fatal(err)
	}
	if j, ok, err = f.handler.claimTin(ctx, g.ID, 3, "group"); err != nil || !ok {
		t.Fatalf("claim seq 3: %v %v", ok, err)
	}
	if err = f.handler.release(ctx, j); err != nil {
		t.Fatal(err)
	}
	rows = coHang("release", g.ID, 1, 2, 3, 4)
	if cho := time.Until(rows[3].due); cho > 100*time.Millisecond {
		t.Fatalf("release: due in %v, want now", cho)
	}
	if status, attempts, _, _ := f.trangThai(t, g.ID); status != "queued" || attempts != 2 {
		t.Fatalf("release: status=%s attempts=%d, want queued with the attempt given back (2)", status, attempts)
	}

	if ran, err := f.handler.ClaimByID(ctx, g.ID, 4); err != nil || !ran {
		t.Fatalf("run seq 4: %v %v", ran, err)
	}
	if status, _, _, _ := f.trangThai(t, g.ID); status != "succeeded" {
		t.Fatalf("status %s", status)
	}
	coHang("xong", g.ID, 1, 2, 3, 4)

	c := f.create(t)
	if _, ok, err = f.handler.claimTin(ctx, c.ID, 1, "group"); err != nil || !ok {
		t.Fatalf("claim to cancel: %v %v", ok, err)
	}
	requireCode(t, f.request("POST", f.route()+"/"+c.ID+"/cancel", f.token, nil), 200)
	coHang("huỷ", c.ID, 1)
}

// A heartbeat renews the lease and nothing else: the job's entry number and
// its outbox rows are what they were before it (M1 turns red here, and at the
// claim step of the test above).
func TestHeartbeatKhongTaoDongOutbox(t *testing.T) {
	f := setup(t, nil)
	f.handler.WithWorker(fastWorker())
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "nhịp tim" })[0]
	j, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
	if !ok || err != nil {
		t.Fatalf("claim: %v %v", ok, err)
	}
	before := f.outbox(t, id)
	_, _, seq, _ := f.trangThai(t, id)
	lease := leaseUntil(t, f, id)
	stop := f.handler.heartbeat(ctx, j, func() { t.Error("heartbeat lost a lease it holds") })
	time.Sleep(350 * time.Millisecond)
	stop()
	if later := leaseUntil(t, f, id); !later.After(lease) {
		t.Fatal("heartbeat did not renew; this test would prove nothing")
	}
	after := f.outbox(t, id)
	_, _, seqAfter, _ := f.trangThai(t, id)
	if len(after) != len(before) || seqAfter != seq {
		t.Fatalf("heartbeat created outbox rows: %d -> %d, enqueue_seq %d -> %d", len(before), len(after), seq, seqAfter)
	}
}

// A message names one entry into the queue. Two consumers holding the same
// message (a redelivery) claim it exactly once, a hundred times over, and a
// message from another entry claims nothing.
func TestClaimTheoRefVaSeqDungMotNguoiThang(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	ids := f.chenNep(t, 100, func(i int) string { return fmt.Sprintf("câu hỏi số %d", i) })
	var thang atomic.Int64
	for _, id := range ids {
		var wg sync.WaitGroup
		var cua atomic.Int64
		start := make(chan struct{})
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
				if err != nil {
					t.Error(err)
				}
				if ok {
					cua.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		if cua.Load() != 1 {
			t.Fatalf("job %s: %d winners, want exactly 1", id, cua.Load())
		}
		thang.Add(cua.Load())
	}
	if thang.Load() != 100 {
		t.Fatalf("%d winners over 100 jobs", thang.Load())
	}
	// A message of another entry, or of another queue, claims nothing.
	more := f.chenNep(t, 1, func(int) string { return "câu hỏi lẻ" })
	for _, c := range []struct {
		seq   int64
		scope string
	}{{0, scopeMe}, {2, scopeMe}, {1, "group"}} {
		if _, ok, err := f.handler.claimTin(ctx, more[0], c.seq, c.scope); ok || err != nil {
			t.Fatalf("seq %d scope %s claimed: %v %v", c.seq, c.scope, ok, err)
		}
	}
	if _, ok, err := f.handler.claimTin(ctx, more[0], 1, scopeMe); !ok || err != nil {
		t.Fatalf("the right message claims nothing: %v %v", ok, err)
	}
	t.Logf("100 jobs x 2 concurrent claims: %d winners", thang.Load())
}

// The poller claims a job only once it is due, the safety net only once it
// has been due for NetLag, and a worker only jobs of its own queues.
func TestPollerTonTrongAvailableAt(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "đợi tới hạn" })[0]
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET available_at=clock_timestamp()+interval '3 seconds' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.handler.claimNext(ctx, 0); ok || err != nil {
		t.Fatalf("claimed before due: %v %v", ok, err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET available_at=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.handler.claimNext(ctx, 5*time.Second); ok || err != nil {
		t.Fatalf("the safety net took a job due only 1 s ago: %v %v", ok, err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET available_at=clock_timestamp()-interval '6 seconds' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.handler.WithQueues([]string{HangNhom}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.handler.claimNext(ctx, 0); ok || err != nil {
		t.Fatalf("an ai.group worker polled a Nếp job: %v %v", ok, err)
	}
	if _, err := f.handler.WithQueues([]string{HangNep}); err != nil {
		t.Fatal(err)
	}
	j, ok, err := f.handler.claimNext(ctx, 5*time.Second)
	if !ok || err != nil || j.id != id {
		t.Fatalf("the safety net missed a job due 6 s ago: %v %v", ok, err)
	}
}

// Lease after first content (design 02 §4 step 7): a lapsed lease is claimed
// again only while no content has left the worker; with content out, the
// sweep fails the job as worker_interrupted instead (M6 turns red here).
func TestLeaseHetSauNoiDungDauKhongChayLai(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	ids := f.chenNep(t, 2, func(i int) string { return fmt.Sprintf("worker chết %d", i) })
	for _, id := range ids {
		if _, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe); !ok || err != nil {
			t.Fatalf("claim: %v %v", ok, err)
		}
	}
	coNoiDung, chuaCo := ids[0], ids[1]
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=ANY($1)`, ids); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET first_token_at=clock_timestamp() WHERE id=$1`, coNoiDung); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.handler.claimTin(ctx, coNoiDung, 1, scopeMe); ok || err != nil {
		t.Fatalf("a job with content out was claimed again by its message: %v %v", ok, err)
	}
	j, ok, err := f.handler.claimNext(ctx, 0)
	if err != nil || !ok || j.id != chuaCo {
		t.Fatalf("the poller took %q (ok=%v err=%v), want only the job with no content out", j.id, ok, err)
	}
	if _, ok, err = f.handler.claimNext(ctx, 0); ok || err != nil {
		t.Fatalf("a second claim: %v %v", ok, err)
	}
	if err = f.handler.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	var status, code string
	if err = f.pool.QueryRow(ctx, `SELECT status, code FROM chat_ai_invocations WHERE id=$1`, coNoiDung).Scan(&status, &code); err != nil || status != "failed" || code != "worker_interrupted" {
		t.Fatalf("after the sweep: %s %s %v", status, code, err)
	}
}

// retryLater puts a job back only when every condition of design 02 §4 step 5
// holds; each one alone stops it.
func TestRetryLaterDuDieuKienMoiThuLai(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	for _, c := range []struct {
		name, set string
		want      bool
	}{
		{"đủ điều kiện", ``, true},
		{"đã có nội dung", `first_token_at=clock_timestamp()`, false},
		{"hết lượt thử", `attempts=3`, false},
		{"hết lời gọi model", fmt.Sprintf("model_calls=%d", llm.MaxModelCallsPerTurn), false},
		{"quá 30 s từ lúc hỏi", `created_at=clock_timestamp()-interval '29.5 seconds'`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			id := f.chenNep(t, 1, func(int) string { return c.name })[0]
			j, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
			if !ok || err != nil {
				t.Fatalf("claim: %v %v", ok, err)
			}
			if c.set != "" {
				if _, err = f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET `+c.set+` WHERE id=$1`, id); err != nil {
					t.Fatal(err)
				}
			}
			got, err := f.handler.retryLater(ctx, j)
			if err != nil || got != c.want {
				t.Fatalf("retryLater=%v %v, want %v", got, err, c.want)
			}
			status, _, seq, _ := f.trangThai(t, id)
			if c.want && (status != "queued" || seq != 2) || !c.want && (status != "running" || seq != 1) {
				t.Fatalf("status=%s seq=%d", status, seq)
			}
		})
	}
}

// /retry is a new turn the caller pressed for: no content out, no model call
// spent, due now.
func TestRetryDatLaiNoiDungVaLoiGoi(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	g := f.create(t)
	j, ok, err := f.handler.claimTin(ctx, g.ID, 1, "group")
	if !ok || err != nil {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET first_token_at=clock_timestamp(), model_calls=5, available_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, g.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.handler.finishFailure(ctx, j, "provider_unavailable"); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.request("POST", f.route()+"/"+g.ID+"/retry", f.token, nil), 200)
	var coNoiDung bool
	var calls int
	var cho float64
	if err = f.pool.QueryRow(ctx, `SELECT first_token_at IS NOT NULL, model_calls, EXTRACT(EPOCH FROM clock_timestamp()-available_at) FROM chat_ai_invocations WHERE id=$1`, g.ID).Scan(&coNoiDung, &calls, &cho); err != nil {
		t.Fatal(err)
	}
	if coNoiDung || calls != 0 || cho > 1 {
		t.Fatalf("/retry left first_token_at=%v model_calls=%d due %.1fs ago", coNoiDung, calls, cho)
	}
}

// The ceiling lives in the row: eight goroutines holding the same lease and
// asking for calls at once get exactly MaxModelCallsPerTurn between them,
// and a lease that is not the job's gets none.
func TestModelCallsKhongVuotTranKhiTranhNhau(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "tranh lời gọi" })[0]
	j, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
	if !ok || err != nil {
		t.Fatalf("claim: %v %v", ok, err)
	}
	stranger := j
	stranger.lease = newID()
	if err := f.handler.giuLuot(stranger)(ctx); !errors.Is(err, llm.ErrHetNganSach) {
		t.Fatalf("a foreign lease took a call: %v", err)
	}
	giu := f.handler.giuLuot(j)
	var duoc, tuChoi atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range llm.MaxModelCallsPerTurn {
				switch err := giu(ctx); {
				case err == nil:
					duoc.Add(1)
				case errors.Is(err, llm.ErrHetNganSach):
					tuChoi.Add(1)
				default:
					t.Error(err)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	_, _, _, calls := f.trangThai(t, id)
	if duoc.Load() != int64(llm.MaxModelCallsPerTurn) || calls != llm.MaxModelCallsPerTurn {
		t.Fatalf("granted %d, row says %d, want exactly %d", duoc.Load(), calls, llm.MaxModelCallsPerTurn)
	}
	t.Logf("8 goroutines x %d asks: %d granted, %d refused", llm.MaxModelCallsPerTurn, duoc.Load(), tuChoi.Load())
}

// The ceiling holds across attempts: a job that already spent every call
// never reaches the model again.
func TestModelCallsHetTuLanTruocKhongGoiNua(t *testing.T) {
	f := setup(t, nil)
	stub := llm.NewStub(llm.Buoc{Text: "không được gọi"})
	f.nepTrenEngine(t, stub)
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "đi đâu?" })[0]
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET model_calls=$2 WHERE id=$1`, id, llm.MaxModelCallsPerTurn); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	var status, code string
	if err := f.pool.QueryRow(ctx, `SELECT status, code FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "ai_het_ngan_sach" || stub.SoGoi() != 0 {
		t.Fatalf("status=%s code=%s model calls=%d", status, code, stub.SoGoi())
	}
}

type gioiHanGia struct {
	cho bool
	loi error
	hoi atomic.Int64
}

func (g *gioiHanGia) Xin(context.Context, string) (bool, error) {
	g.hoi.Add(1)
	return g.cho, g.loi
}

// The rate limiter refusing before any content is a transient failure: the
// job goes back to the queue, no call is made or counted. A limiter that
// cannot answer lets the call go (fail open).
func TestGioiHanTuChoiThiThuLaiSau(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	stub := llm.NewStub(llm.Buoc{Text: "Đi dạo hồ nhé."})
	tuChoi := &gioiHanGia{}
	f.nepTrenEngine(t, stub, aiharness.WithGioiHan(tuChoi))
	id := f.chenNep(t, 1, func(int) string { return "đi đâu?" })[0]
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	status, attempts, seq, calls := f.trangThai(t, id)
	var lop string
	if err := f.pool.QueryRow(ctx, `SELECT loi_mo_hinh FROM ai_turn_metrics WHERE invocation_id=$1`, id).Scan(&lop); err != nil {
		t.Fatal(err)
	}
	if status != "queued" || attempts != 1 || seq != 2 || calls != 0 || stub.SoGoi() != 0 || tuChoi.hoi.Load() != 1 || lop != "429" {
		t.Fatalf("refused: status=%s attempts=%d seq=%d model_calls=%d stub=%d asked=%d class=%s", status, attempts, seq, calls, stub.SoGoi(), tuChoi.hoi.Load(), lop)
	}
	// Redis down: the limiter errs, the call goes.
	f.nepTrenEngine(t, stub, aiharness.WithGioiHan(&gioiHanGia{loi: errors.New("redis: connection refused")}))
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET available_at=clock_timestamp() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	if status, _, _, calls = f.trangThai(t, id); status != "succeeded" || calls != 1 || stub.SoGoi() != 1 {
		t.Fatalf("fail open: status=%s model_calls=%d stub=%d", status, calls, stub.SoGoi())
	}
}

// The two enqueue triggers are the only way into job_outbox (design 02 §3.2):
// read from the catalogue, not from the migration text. On the job table,
// exactly the BEFORE trigger that numbers an entry and the AFTER trigger
// that writes its row; jobs_them is the only function that names the table,
// and only the AFTER trigger's function calls jobs_them; no trigger, rule or
// other function reaches job_outbox.
func TestHaiTriggerLaDuongGhiDuyNhatVaoOutbox(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	var triggers []string
	rows, err := f.pool.Query(ctx, `SELECT t.tgname||':'||CASE WHEN (t.tgtype & 2)<>0 THEN 'before' ELSE 'after' END||':'||p.proname
		FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid
		WHERE NOT t.tgisinternal AND t.tgrelid='chat_ai_invocations'::regclass ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		triggers = append(triggers, s)
	}
	rows.Close()
	if got := strings.Join(triggers, ","); got != "chat_ai_enqueue:after:chat_ai_enqueue,chat_ai_enqueue_seq:before:chat_ai_enqueue_seq" {
		t.Fatalf("triggers on chat_ai_invocations: %s", got)
	}
	names := func(pattern string) []string {
		t.Helper()
		rows, err := f.pool.Query(ctx, `SELECT p.proname FROM pg_proc p WHERE p.pronamespace=current_schema()::regnamespace AND p.prosrc ~* $1 ORDER BY 1`, pattern)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err = rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return out
	}
	if got := names(`\mjob_outbox\M`); fmt.Sprint(got) != "[jobs_them]" {
		t.Fatalf("functions naming job_outbox: %v", got)
	}
	if got := names(`\mjobs_them\M`); fmt.Sprint(got) != "[chat_ai_enqueue]" {
		t.Fatalf("functions calling jobs_them: %v", got)
	}
	var callers []string
	rows, err = f.pool.Query(ctx, `SELECT c.relname||'.'||t.tgname FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_class c ON c.oid=t.tgrelid
		WHERE NOT t.tgisinternal AND c.relnamespace=current_schema()::regnamespace AND p.proname IN ('jobs_them','chat_ai_enqueue')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		callers = append(callers, s)
	}
	rows.Close()
	sort.Strings(callers)
	if fmt.Sprint(callers) != "[chat_ai_invocations.chat_ai_enqueue]" {
		t.Fatalf("triggers that reach jobs_them: %v", callers)
	}
	var onOutbox int
	if err = f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgrelid='job_outbox'::regclass)
		+ (SELECT count(*) FROM pg_rewrite WHERE ev_class='job_outbox'::regclass)`).Scan(&onOutbox); err != nil || onOutbox != 0 {
		t.Fatalf("triggers or rules on job_outbox: %d %v", onOutbox, err)
	}
}
