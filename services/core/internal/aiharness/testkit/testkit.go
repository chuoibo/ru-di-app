// Package testkit holds in-memory fakes of the engine's ports for tests:
// TriNho and NganHan (trinho) and a scripted Retriever (truyhoi). They keep
// the ports' data rules (one owner per fact, hard delete, newest turns
// only) so a test that passes on a fake does not rely on a rule an adapter
// breaks. They rank nothing: Nho returns the newest facts, not the most
// relevant ones, because relevance is the real adapter's (embedding,
// reranker) and a fake that guessed it by words would teach tests a
// heuristic. A fake is never evidence about a database (CLAUDE.md).
package testkit

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// TriNho is an in-memory trinho.TriNho.
type TriNho struct {
	mu    sync.Mutex
	next  int
	nguoi map[string][]trinho.SuThat
}

// MoiTriNho is an empty store.
func MoiTriNho() *TriNho { return &TriNho{nguoi: map[string][]trinho.SuThat{}} }

var _ trinho.TriNho = (*TriNho)(nil)

// Nho returns the k newest facts of nguoi; cau is ignored (see the package
// comment).
func (t *TriNho) Nho(_ context.Context, nguoi, _ string, k int) ([]trinho.SuThat, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	all := t.nguoi[nguoi]
	var out []trinho.SuThat
	for i := len(all) - 1; i >= 0 && len(out) < k; i-- {
		out = append(out, all[i])
	}
	return out, nil
}

// Ghi stores one fact after moi.Kiem.
func (t *TriNho) Ghi(_ context.Context, nguoi string, moi trinho.SuThatMoi) (trinho.SuThat, error) {
	if err := moi.Kiem(); err != nil {
		return trinho.SuThat{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	s := trinho.SuThat{ID: "f" + strconv.Itoa(t.next), NoiDung: moi.NoiDung, Loai: moi.Loai, TuLuc: moi.TuLuc, DenLuc: moi.DenLuc, Nguon: moi.Nguon}
	t.nguoi[nguoi] = append(t.nguoi[nguoi], s)
	return s, nil
}

// Quen hard-deletes by id, or by exact content for MoTa (the real adapter
// resolves a description by model or embedding; the fake only matches the
// stored sentence exactly).
func (t *TriNho) Quen(_ context.Context, nguoi string, q trinho.QuenGi) (int, error) {
	if err := q.Kiem(); err != nil {
		return 0, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var giu []trinho.SuThat
	n := 0
	for _, s := range t.nguoi[nguoi] {
		if (q.ID != "" && s.ID == q.ID) || (q.MoTa != "" && s.NoiDung == q.MoTa) {
			n++
			continue
		}
		giu = append(giu, s)
	}
	t.nguoi[nguoi] = giu
	return n, nil
}

// LietKe lists every fact of nguoi, oldest first.
func (t *TriNho) LietKe(_ context.Context, nguoi string) (trinho.TatCa, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return trinho.TatCa{SuThat: append([]trinho.SuThat(nil), t.nguoi[nguoi]...)}, nil
}

// NganHan is an in-memory trinho.NganHan.
type NganHan struct {
	mu    sync.Mutex
	phien map[string][]trinho.Luot
}

// MoiNganHan is an empty store.
func MoiNganHan() *NganHan { return &NganHan{phien: map[string][]trinho.Luot{}} }

var _ trinho.NganHan = (*NganHan)(nil)

// Doc returns the newest trinho.MaxLuotNganHan turns, oldest first.
func (n *NganHan) Doc(_ context.Context, phien string) ([]trinho.Luot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	l := n.phien[phien]
	if len(l) > trinho.MaxLuotNganHan {
		l = l[len(l)-trinho.MaxLuotNganHan:]
	}
	return append([]trinho.Luot(nil), l...), nil
}

// Them appends a turn with a known speaker.
func (n *NganHan) Them(_ context.Context, phien string, l trinho.Luot) error {
	if !trinho.VaiLuots.Co(l.Vai) {
		return fmt.Errorf("testkit: speaker %q", l.Vai)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.phien[phien] = append(n.phien[phien], l)
	return nil
}

// Xoa drops the session.
func (n *NganHan) Xoa(_ context.Context, phien string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.phien, phien)
	return nil
}

// Retriever answers from a script keyed by source and query text, and
// records every request it saw so a test can assert the hard constraints
// reached it unchanged.
type Retriever struct {
	mu      sync.Mutex
	KichBan map[truyhoi.Nguon]map[string]truyhoi.KetQuaTruyHoi
	Da      []truyhoi.YeuCau
}

var _ truyhoi.Retriever = (*Retriever)(nil)

// Tim returns the scripted answer, or an empty one.
func (r *Retriever) Tim(_ context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	if err := y.Kiem(); err != nil {
		return truyhoi.KetQuaTruyHoi{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Da = append(r.Da, y)
	return r.KichBan[y.Nguon][y.Cau], nil
}
