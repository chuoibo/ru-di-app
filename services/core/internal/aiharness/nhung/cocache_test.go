package nhung

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// demNhung counts what reaches the provider; cho blocks each call until
// closed; loi fails the next call.
type demNhung struct {
	Stub
	goi, texts atomic.Int64
	cho        chan struct{}
	loi        atomic.Bool
}

func (d *demNhung) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	d.goi.Add(1)
	d.texts.Add(int64(len(texts)))
	if d.cho != nil {
		<-d.cho
	}
	if d.loi.CompareAndSwap(true, false) {
		return nil, errors.New("down")
	}
	return d.Stub.Nhung(ctx, texts, tv)
}

func TestCoCacheTrungTruot(t *testing.T) {
	ctx := context.Background()
	in := &demNhung{}
	c := CoCache{Inner: in, Bo: MoiBoNhoCau(2)}
	a, err := c.Nhung(ctx, []string{"lẩu nấm đà lạt"}, CauHoi)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := c.Nhung(ctx, []string{" lẩu nấm đà lạt "}, CauHoi) // NFC + trim: the prompt the model sees
	if in.goi.Load() != 1 || c.Bo.Trung() != 1 || c.Bo.Truot() != 1 || Cosine(a[0], b[0]) < 0.9999 {
		t.Fatalf("a repeated query: %d calls, %d hits, %d misses", in.goi.Load(), c.Bo.Trung(), c.Bo.Truot())
	}
	b[0][0] = 42
	again, _ := c.Nhung(ctx, []string{"lẩu nấm đà lạt"}, CauHoi)
	if again[0][0] == 42 {
		t.Fatal("a caller's change reached the cache")
	}
	// One text, two tasks: two prompts, two entries.
	if _, err := c.Nhung(ctx, []string{"lẩu nấm đà lạt"}, HoiDap); err != nil || in.goi.Load() != 2 {
		t.Fatalf("another task was answered from the cache: %d calls", in.goi.Load())
	}
	// A batch: hits answered, the misses in one call; then the LRU (2) has
	// dropped the oldest.
	vs, err := c.Nhung(ctx, []string{"lẩu nấm đà lạt", "bún bò", "cà phê"}, CauHoi)
	if err != nil || len(vs) != 3 || in.goi.Load() != 3 || in.texts.Load() != 4 {
		t.Fatalf("a mixed batch: %d calls, %d texts, %v", in.goi.Load(), in.texts.Load(), err)
	}
	if _, err := c.Nhung(ctx, []string{"lẩu nấm đà lạt"}, HoiDap); err != nil || in.goi.Load() != 4 {
		t.Fatalf("the evicted entry was still served: %d calls", in.goi.Load())
	}
	// An error is not cached.
	in.loi.Store(true)
	if _, err := c.Nhung(ctx, []string{"phở"}, CauHoi); err == nil {
		t.Fatal("the failure was swallowed")
	}
	if _, err := c.Nhung(ctx, []string{"phở"}, CauHoi); err != nil || in.goi.Load() != 6 {
		t.Fatalf("a failure was cached: %d calls, %v", in.goi.Load(), err)
	}
	// Documents are not cached.
	trung := c.Bo.Trung()
	_, _ = c.NhungTaiLieu(ctx, []TaiLieuVao{{NoiDung: "x"}})
	_, _ = c.NhungTaiLieu(ctx, []TaiLieuVao{{NoiDung: "x"}})
	if c.Bo.Trung() != trung {
		t.Fatalf("documents went through the cache: %d hits", c.Bo.Trung())
	}
}

// Concurrent misses on one query make one provider call.
func TestCoCacheGopTruot(t *testing.T) {
	in := &demNhung{cho: make(chan struct{})}
	c := CoCache{Inner: in, Bo: MoiBoNhoCau(0)}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = c.Nhung(context.Background(), []string{"lẩu"}, CauHoi)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(in.cho)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if in.goi.Load() != 1 {
		t.Fatalf("%d calls for one query asked 8 times at once", in.goi.Load())
	}
}

// Outside the turn's budget: three searches of one query on a budget of two
// all run, and only the first spends.
func TestCoCacheNgoaiNganSach(t *testing.T) {
	d := &DemLuot{}
	d.con.Store(2)
	ctx := VoiDemLuot(context.Background(), d)
	c := CoCache{Inner: TheoLuot{Inner: &demNhung{}}, Bo: MoiBoNhoCau(0)}
	for i := 0; i < 3; i++ {
		if _, err := c.Nhung(ctx, []string{"lẩu"}, CauHoi); err != nil {
			t.Fatalf("search %d: %v", i+1, err)
		}
	}
	if d.ConLai() != 1 {
		t.Fatalf("budget left %d, want 1", d.ConLai())
	}
}
