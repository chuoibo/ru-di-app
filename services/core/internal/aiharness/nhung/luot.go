package nhung

import (
	"context"
)

// The turn's embedding budget travels with the turn's context, so an
// embedder shared by every turn (the retrieval adapter's) still counts each
// call against the turn that made it: one budget, MaxEmbedCallsPerTurn,
// for the router's example choice and every retrieval of the turn alike.

type khoaDemLuot struct{}

// VoiDemLuot is ctx carrying the turn's counter d.
func VoiDemLuot(ctx context.Context, d *DemLuot) context.Context {
	return context.WithValue(ctx, khoaDemLuot{}, d)
}

// DemLuotTu is the counter ctx carries, nil outside a turn (nil counts
// nothing: the ingest's offline calls).
func DemLuotTu(ctx context.Context) *DemLuot {
	d, _ := ctx.Value(khoaDemLuot{}).(*DemLuot)
	return d
}

// TheoLuot is an embedder shared across turns that takes every provider
// request from the budget of the turn the call's context belongs to
// (DemLuotTu), refusing with ErrHetLuotNhung once it is spent, and checks
// the answer's vector count. A caller that gets the refusal runs without
// its dense leg and says so (truyhoi.NoVector).
type TheoLuot struct{ Inner Nhung }

func (t TheoLuot) Model() string { return t.Inner.Model() }
func (t TheoLuot) Dims() int     { return t.Inner.Dims() }
func (t TheoLuot) SoGoi() int64  { return t.Inner.SoGoi() }

func giuLuot(ctx context.Context, n int) error {
	d := DemLuotTu(ctx)
	for i := 0; i < soYeuCau(n); i++ {
		if err := d.Giu(); err != nil {
			return err
		}
	}
	return nil
}

// Nhung embeds under the turn's budget.
func (t TheoLuot) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	if err := giuLuot(ctx, len(texts)); err != nil {
		return nil, err
	}
	vs, err := t.Inner.Nhung(ctx, texts, tv)
	if err != nil {
		return nil, err
	}
	if err := kiemSoVector(vs, len(texts), t.Inner.Dims()); err != nil {
		return nil, err
	}
	return vs, nil
}

// NhungTaiLieu embeds documents under the turn's budget.
func (t TheoLuot) NhungTaiLieu(ctx context.Context, docs []TaiLieuVao) ([][]float32, error) {
	if len(docs) == 0 {
		return nil, ErrRong
	}
	if err := giuLuot(ctx, len(docs)); err != nil {
		return nil, err
	}
	vs, err := t.Inner.NhungTaiLieu(ctx, docs)
	if err != nil {
		return nil, err
	}
	if err := kiemSoVector(vs, len(docs), t.Inner.Dims()); err != nil {
		return nil, err
	}
	return vs, nil
}
