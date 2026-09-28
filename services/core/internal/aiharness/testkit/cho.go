package testkit

import (
	"context"
	"sync"

	"mobile/services/core/internal/aiharness/truyhoi"
)

// Cho is an in-memory catalogue reader (the tools' DocCho port): fixed
// destinations, places and areas. Like the other fakes it ranks nothing.
type Cho struct {
	DiemDens []truyhoi.BangChung
	Quans    []truyhoi.BangChung
	KhuVucs  map[string][]truyhoi.BangChung
}

// Quan returns the places with these ids, in the order asked, skipping ids
// the catalogue does not have.
func (c Cho) Quan(_ context.Context, ids []string) ([]truyhoi.BangChung, error) {
	var out []truyhoi.BangChung
	for _, id := range ids {
		for _, q := range c.Quans {
			if q.ID == id {
				out = append(out, q)
			}
		}
	}
	return out, nil
}

// DiemDen returns every destination.
func (c Cho) DiemDen(context.Context) ([]truyhoi.BangChung, error) {
	return append([]truyhoi.BangChung(nil), c.DiemDens...), nil
}

// KhuVuc returns the areas of one destination.
func (c Cho) KhuVuc(_ context.Context, diemDenID string) ([]truyhoi.BangChung, error) {
	return append([]truyhoi.BangChung(nil), c.KhuVucs[diemDenID]...), nil
}

// TheoLuot is a Retriever that answers its i-th request with KetQua[i] (the
// last one again past the end), whatever the query says, and records every
// request: a test scripts a first retrieval and a corrective round, and
// asserts the hard constraints reached both unchanged.
type TheoLuot struct {
	mu     sync.Mutex
	KetQua []truyhoi.KetQuaTruyHoi
	Da     []truyhoi.YeuCau
}

var _ truyhoi.Retriever = (*TheoLuot)(nil)

// Tim returns the scripted answer for this call.
func (r *TheoLuot) Tim(_ context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	if err := y.Kiem(); err != nil {
		return truyhoi.KetQuaTruyHoi{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Da = append(r.Da, y)
	if len(r.KetQua) == 0 {
		return truyhoi.KetQuaTruyHoi{}, nil
	}
	i := len(r.Da) - 1
	if i >= len(r.KetQua) {
		i = len(r.KetQua) - 1
	}
	return r.KetQua[i], nil
}

// SoLan is how many requests the retriever saw.
func (r *TheoLuot) SoLan() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Da)
}
