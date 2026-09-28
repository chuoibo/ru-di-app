//go:build postgres

package chatassist

import (
	"context"
	"errors"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/llm"
)

// Streaming's database half (slice 11; design 02 §4 step 7, §5.1): before the
// first content of a job leaves, first_token_at is set -- once, and only under
// the lease that runs the job. From then on the job is never released,
// retried later or handed back to the queue: no second worker may write the
// answer over what readers already saw.
func TestFirstTokenAtMotLanDuoiLease(t *testing.T) {
	f := setup(t, nil)
	f.handler.WithWorker(fastWorker())
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	j, ok, err := f.handler.claimNext(ctx, 0)
	if err != nil || !ok || j.id != id {
		t.Fatalf("claim: %v %v", ok, err)
	}
	dauTien := func() *time.Time {
		t.Helper()
		var at *time.Time
		if err := f.pool.QueryRow(ctx, `SELECT first_token_at FROM chat_ai_invocations WHERE id=$1`, id).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	// Another lease: refused, nothing set.
	khac := j
	khac.lease = newID()
	if err := f.handler.danhDauNoiDung(ctx, khac); !errors.Is(err, errKhongCoNoiDung) || dauTien() != nil {
		t.Fatalf("another lease marked content: %v %v", err, dauTien())
	}
	// This lease: set.
	if err := f.handler.danhDauNoiDung(ctx, j); err != nil {
		t.Fatal(err)
	}
	at := dauTien()
	if at == nil {
		t.Fatal("first_token_at not set")
	}
	// Once: a second mark is refused and does not move it.
	time.Sleep(5 * time.Millisecond)
	if err := f.handler.danhDauNoiDung(ctx, j); !errors.Is(err, errKhongCoNoiDung) || !dauTien().Equal(*at) {
		t.Fatalf("marked twice: %v", err)
	}
	// Released after content: refused; the job stays this worker's.
	if err := f.handler.release(ctx, j); err != nil {
		t.Fatal(err)
	}
	if status, attempts, _, _ := f.trangThai(t, id); status != "running" || attempts != 1 {
		t.Fatalf("release after content: %s, %d attempts", status, attempts)
	}
	// Retried later after content: refused.
	if later, err := f.handler.retryLater(ctx, j); err != nil || later {
		t.Fatalf("retryLater after content: %v %v", later, err)
	}
	// A failed terminal write after content ends the lease instead of handing
	// the job back, and the sweep fails it as worker_interrupted.
	if err := f.handler.traLai(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err := f.handler.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	var status, code string
	if err := f.pool.QueryRow(ctx, `SELECT status,code FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "worker_interrupted" {
		t.Fatalf("after content and a lost lease: %s/%s", status, code)
	}
	if _, ok, err := f.handler.claimNext(ctx, 0); err != nil || ok {
		t.Fatalf("a job with content out was claimed again: %v %v", ok, err)
	}
}

// With no stream (MOBILE_REDIS_URL empty) nothing streams and nothing is
// marked: a job the worker loses before its end can still run again.
func TestKhongStreamKhongDanhDau(t *testing.T) {
	f := setup(t, nil)
	f.handler.WithWorker(fastWorker())
	f.nepTrenEngine(t, llm.NewStub(kichNep("Tối nay bạn thử ra bờ hồ đi dạo một vòng rồi ghé quán chè ấm bụng nhé.", 0, nil)...))
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	if ok, err := f.handler.ProcessOne(ctx); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var status string
	var at *time.Time
	if err := f.pool.QueryRow(ctx, `SELECT status,first_token_at FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &at); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || at != nil {
		t.Fatalf("%s, first_token_at %v", status, at)
	}
}
