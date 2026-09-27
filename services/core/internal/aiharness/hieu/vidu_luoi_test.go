package hieu

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/obs"
)

// nhungDem counts embedding requests and can fail them.
type nhungDem struct {
	nhung.Stub
	goi  int
	hong bool
}

func (n *nhungDem) Nhung(ctx context.Context, texts []string, tv nhung.TacVu) ([][]float32, error) {
	n.goi++
	if n.hong {
		return nil, errors.New("synthetic embedding failure")
	}
	return n.Stub.Nhung(ctx, texts, tv)
}

// The lazy bank makes no embedding call until a turn asks for examples; a
// failure leaves it unbuilt and the next turn tries again; once built, a
// turn costs one call (the message) and the bank is never embedded twice.
func TestKhoViDuLuoi(t *testing.T) {
	n := &nhungDem{hong: true}
	k := MoiKhoViDuLuoi(n, ViDuMacDinh)
	if n.goi != 0 {
		t.Fatalf("%d lời gọi embedding lúc dựng", n.goi)
	}
	v := vaoMau(obs.BotNep)
	if _, err := k.Chon(context.Background(), v); !errors.Is(err, ErrNhung) {
		t.Fatalf("lỗi embedding: %v", err)
	}
	n.hong, n.goi = false, 0
	got, err := k.Chon(context.Background(), v)
	if err != nil || len(got) == 0 {
		t.Fatalf("%v %d", err, len(got))
	}
	dung := n.goi
	if _, err := k.Chon(context.Background(), v); err != nil || n.goi != dung+1 {
		t.Fatalf("lượt sau dựng lại kho: %d lời gọi, trước %d", n.goi, dung)
	}
	// The router takes it, and runs without examples while it fails.
	r := Moi(WithViDuLuoi(MoiKhoViDuLuoi(&nhungDem{hong: true}, ViDuMacDinh)))
	if r.viDu == nil {
		t.Fatal("router không nhận kho lười")
	}
}

// nhungTreo hangs until its context ends or it is released.
type nhungTreo struct {
	nhung.Stub
	mu  sync.Mutex
	goi int
	tha chan struct{}
}

func (n *nhungTreo) Nhung(ctx context.Context, texts []string, tv nhung.TacVu) ([][]float32, error) {
	n.mu.Lock()
	n.goi++
	n.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-n.tha:
		return n.Stub.Nhung(ctx, texts, tv)
	}
}

// An embedder that hangs costs the examples, never the turns: every
// concurrent turn gets its answer (no examples) within the wait, none holds
// a lock across the network, and one build is in flight, not one per turn.
func TestKhoViDuLuoiKhongGiuKhoaQuaMang(t *testing.T) {
	n := &nhungTreo{tha: make(chan struct{})}
	k := MoiKhoViDuLuoi(n, ViDuMacDinh)
	k.cho, k.han = 50*time.Millisecond, time.Hour
	var wg sync.WaitGroup
	batDau := time.Now()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := k.Chon(context.Background(), vaoMau(obs.BotNep)); !errors.Is(err, ErrNhung) {
				t.Errorf("a turn got %v", err)
			}
		}()
	}
	wg.Wait()
	if d := time.Since(batDau); d > time.Second {
		t.Fatalf("turns queued behind the build: %v", d)
	}
	n.mu.Lock()
	goi := n.goi
	n.mu.Unlock()
	if goi != 1 {
		t.Fatalf("%d builds in flight, want 1", goi)
	}
	// Released: the build finishes and a later turn picks.
	close(n.tha)
	var got []ViDu
	var err error
	for i := 0; i < 100 && len(got) == 0; i++ {
		got, err = k.Chon(context.Background(), vaoMau(obs.BotNep))
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil || len(got) == 0 {
		t.Fatalf("after release: %v %d", err, len(got))
	}
	// A build has its own deadline, apart from the turn's.
	n2 := &nhungTreo{tha: make(chan struct{})}
	k2 := MoiKhoViDuLuoi(n2, ViDuMacDinh)
	k2.cho, k2.han = time.Second, 20*time.Millisecond
	if _, err := k2.Chon(context.Background(), vaoMau(obs.BotNep)); !errors.Is(err, ErrNhung) {
		t.Fatalf("a hung build: %v", err)
	}
}

// The turn's embedding budget: the example choice takes its one call from
// it, and a spent budget makes no call at all.
func TestChonTheoNganSachNhung(t *testing.T) {
	n := &nhungDem{}
	kho, err := MoiKhoViDu(context.Background(), n, ViDuMacDinh)
	if err != nil {
		t.Fatal(err)
	}
	v := vaoMau(obs.BotNep)
	v.DemNhung = nhung.MoiDemLuot()
	truoc := n.goi
	for v.DemNhung.ConLai() > 0 {
		_ = v.DemNhung.Giu()
	}
	if _, err := kho.Chon(context.Background(), v); !errors.Is(err, ErrNhung) || n.goi != truoc {
		t.Fatalf("a spent budget still embedded: %v, %d calls", err, n.goi-truoc)
	}
	v.DemNhung = nhung.MoiDemLuot()
	if _, err := kho.Chon(context.Background(), v); err != nil || n.goi != truoc+1 || v.DemNhung.ConLai() != 1 {
		t.Fatalf("%v %d %d", err, n.goi-truoc, v.DemNhung.ConLai())
	}
}
