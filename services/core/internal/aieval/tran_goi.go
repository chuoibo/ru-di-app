package aieval

import (
	"context"
	"errors"
	"iter"
	"sync"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The run-wide ceiling on provider calls (design 06 §7): `--tran-goi N` is
// the number the Lead approved for one real run, model, embedding and
// reranker calls together. It is hard in two ways. Before the run: the estimate (DuToan),
// an upper bound, must not exceed N, or the run does not start. During the
// run: a watchdog counts every call before it leaves -- the first try and
// every retry, since it sits under the per-turn counter -- and when the call
// that brings the count to N has come back, it stops the run, which is then
// marked unfinished (chua_xong) and prints no trailer. A call past N is
// refused, never made.
//
// Reaching N counts as unfinished even when the last call was the run's
// last: the estimate is an upper bound, so a run that spends all of it has
// either an estimate that is wrong or turns that all ran to their ceiling,
// and either way its numbers are not the run the Lead approved.

// ErrTranGoi is a provider call the run-wide ceiling refused; it never left
// the process.
var ErrTranGoi = errors.New("aieval: the approved call ceiling for this run is reached; the call was not made")

// TranGoi is a run's call ceiling and its watchdog. Safe for concurrent use.
type TranGoi struct {
	max     int
	khiCham func()
	once    sync.Once

	mu     sync.Mutex
	moHinh int
	nhung  int
	xepLai int
	daCham bool
}

// NewTranGoi allows max provider calls; khiCham runs once, when the call
// that reaches max has come back or a call past it is refused (the binary
// passes the run context's cancel).
func NewTranGoi(max int, khiCham func()) *TranGoi {
	return &TranGoi{max: max, khiCham: khiCham}
}

// Max is the ceiling.
func (t *TranGoi) Max() int { return t.max }

// DaDung is the provider calls made, model, embedding and reranker
// together.
func (t *TranGoi) DaDung() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.moHinh + t.nhung + t.xepLai
}

// XepLai is the reranker calls made.
func (t *TranGoi) XepLai() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.xepLai
}

// MoHinh is the model calls made.
func (t *TranGoi) MoHinh() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.moHinh
}

// Nhung is the embedding requests made.
func (t *TranGoi) Nhung() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.nhung
}

// DaCham says whether the count has reached the ceiling.
func (t *TranGoi) DaCham() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.daCham
}

func (t *TranGoi) dung() {
	t.once.Do(func() {
		if t.khiCham != nil {
			t.khiCham()
		}
	})
}

// xin admits n calls of one kind, all or none. cham says the admitted calls
// bring the count to the ceiling: the caller stops the run (dung) once they
// have come back.
func (t *TranGoi) xin(dem *int, n int) (cham bool, err error) {
	t.mu.Lock()
	if t.daCham || t.moHinh+t.nhung+t.xepLai+n > t.max {
		t.daCham = true
		t.mu.Unlock()
		t.dung()
		return false, ErrTranGoi
	}
	*dem += n
	if t.moHinh+t.nhung+t.xepLai >= t.max {
		t.daCham, cham = true, true
	}
	t.mu.Unlock()
	return cham, nil
}

// BocLLM puts m under the ceiling: every GenerateContent is one call.
func (t *TranGoi) BocLLM(m model.LLM) model.LLM { return moHinhTran{t: t, inner: m} }

// BocNhung puts n under the ceiling: an embedding call is as many requests
// as the provider door splits it into (nhung.MaxBatch texts each).
func (t *TranGoi) BocNhung(n nhung.Nhung) nhung.Nhung { return nhungTran{t: t, inner: n} }

type moHinhTran struct {
	t     *TranGoi
	inner model.LLM
}

func (m moHinhTran) Name() string { return m.inner.Name() }

// GetGoogleLLMVariant forwards the backend; like llm.Dem there is no Client
// method, so no live session can go around the ceiling.
func (m moHinhTran) GetGoogleLLMVariant() genai.Backend { return backendCua(m.inner) }

func (m moHinhTran) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		cham, err := m.t.xin(&m.t.moHinh, 1)
		if err != nil {
			yield(nil, err)
			return
		}
		if cham {
			defer m.t.dung()
		}
		for resp, err := range m.inner.GenerateContent(ctx, req, stream) {
			if !yield(resp, err) {
				return
			}
		}
	}
}

type nhungTran struct {
	t     *TranGoi
	inner nhung.Nhung
}

func (n nhungTran) Model() string { return n.inner.Model() }
func (n nhungTran) Dims() int     { return n.inner.Dims() }
func (n nhungTran) SoGoi() int64  { return n.inner.SoGoi() }

func (n nhungTran) Nhung(ctx context.Context, texts []string, tv nhung.TacVu) ([][]float32, error) {
	cham, err := n.t.xin(&n.t.nhung, soYeuCauNhung(len(texts)))
	if err != nil {
		return nil, err
	}
	if cham {
		defer n.t.dung()
	}
	return n.inner.Nhung(ctx, texts, tv)
}

func (n nhungTran) NhungTaiLieu(ctx context.Context, docs []nhung.TaiLieuVao) ([][]float32, error) {
	cham, err := n.t.xin(&n.t.nhung, soYeuCauNhung(len(docs)))
	if err != nil {
		return nil, err
	}
	if cham {
		defer n.t.dung()
	}
	return n.inner.NhungTaiLieu(ctx, docs)
}

// soYeuCauNhung is the provider requests an embedding of n inputs takes
// (the provider door splits it into batches of nhung.MaxBatch). Zero inputs
// still count one: the ceiling counts before the door refuses.
func soYeuCauNhung(n int) int { return max(1, (n+nhung.MaxBatch-1)/nhung.MaxBatch) }

// BocXepLai puts x under the ceiling: every XepLai is one call. A call the
// reranker then answers without HTTP (an open circuit, one document) is
// still counted: the ceiling over-counts, never under-counts.
func (t *TranGoi) BocXepLai(x truyhoi.Reranker) truyhoi.Reranker { return xepLaiTran{t: t, inner: x} }

type xepLaiTran struct {
	t     *TranGoi
	inner truyhoi.Reranker
}

func (x xepLaiTran) XepLai(ctx context.Context, cau string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	cham, err := x.t.xin(&x.t.xepLai, 1)
	if err != nil {
		// Keep the input order, as a reranker that failed does: the
		// retrieval goes on flagged no_rerank; the run stops anyway.
		kept, _ := truyhoi.Passthrough{}.XepLai(ctx, cau, bc, topN)
		return kept, err
	}
	if cham {
		defer x.t.dung()
	}
	return x.inner.XepLai(ctx, cau, bc, topN)
}

// MucTieuP95GoiMoiLuot is the contract's target for model calls per turn at
// p95 (contract §3). The estimate prints what it would cost if every turn
// that reaches the model sat at that target; it is a target, never a bound.
const MucTieuP95GoiMoiLuot = 4

// DuToan is a run's call estimate (design 06 §7): the exact upper bound
// every run must fit under `--tran-goi`, and what the contract's p95 target
// would spend.
type DuToan struct {
	SoCa   int `json:"so_ca"`
	Lap    int `json:"lap"`
	SoLuot int `json:"so_luot"`
	// GoiMoiLuot is MaxModelCallsPerTurn, retries included; NhungMoiLuot
	// MaxEmbedCallsPerTurn when the run has an embedder (0 without);
	// XepLaiMoiLuot MaxRerankCallsPerTurn when it has a reranker (0
	// without); NhungChung the embedding requests made once per run, outside
	// any turn (the router's example bank); GiamKhao the judge's calls (no
	// judge yet).
	GoiMoiLuot    int `json:"goi_model_moi_luot"`
	NhungMoiLuot  int `json:"goi_nhung_moi_luot"`
	XepLaiMoiLuot int `json:"goi_xep_lai_moi_luot"`
	NhungChung    int `json:"goi_nhung_chung"`
	GiamKhao      int `json:"giam_khao"`
	// Tran = SoLuot × (GoiMoiLuot + NhungMoiLuot + XepLaiMoiLuot) +
	// NhungChung + GiamKhao.
	Tran int `json:"tran"`
	// LuotToiMoHinh is the runs whose case expects the model reached: a
	// case the pipeline refuses before the model costs nothing unless the
	// pipeline broke, which the upper bound still covers.
	LuotToiMoHinh int `json:"luot_toi_mo_hinh"`
	// KyVongP95 = LuotToiMoHinh × MucTieuP95GoiMoiLuot.
	KyVongP95 int `json:"ky_vong_p95"`
}

// CoPhu says which provider doors besides the model a run has.
type CoPhu struct {
	// Nhung: an embedder (the router's example bank, and any retrieval or
	// memory recall wired to it); SoViDu is the bank's size.
	Nhung  bool
	SoViDu int
	// XepLai: a reranker on the retrieval path.
	XepLai bool
}

func (d *DuToan) phu(p CoPhu) {
	if p.Nhung {
		d.NhungMoiLuot = llm.MaxEmbedCallsPerTurn
		d.NhungChung = soYeuCauNhung(p.SoViDu)
	}
	if p.XepLai {
		d.XepLaiMoiLuot = llm.MaxRerankCallsPerTurn
	}
	d.Tran = d.SoLuot*(d.GoiMoiLuot+d.NhungMoiLuot+d.XepLaiMoiLuot) + d.NhungChung + d.GiamKhao
}

// TinhDuToan estimates a run of every case of b, lap times each: the exact
// upper bound the per-turn counters enforce (model calls with retries,
// embedding and reranker budgets), never a guess of what a turn spends.
func TinhDuToan(b Bo, lap int, p CoPhu) DuToan {
	d := DuToan{SoCa: len(b.Ca), Lap: lap, SoLuot: len(b.Ca) * lap, GoiMoiLuot: llm.MaxModelCallsPerTurn}
	d.phu(p)
	for _, c := range b.Ca {
		if c.KyVong.SoGoiModel > 0 {
			d.LuotToiMoHinh += lap
		}
	}
	d.KyVongP95 = d.LuotToiMoHinh * MucTieuP95GoiMoiLuot
	return d
}

// TinhDuToanHieu estimates a router set (--chi-buoc hieu): one routing per
// case, bounded by the counter ChayBoHieu gives each case
// (MaxModelCallsPerTurn: the first call, the one repair, and their
// retries), plus the example choice's one embedding (no reranker: the
// router retrieves nothing).
func TinhDuToanHieu(b BoHieu, p CoPhu) DuToan {
	d := DuToan{SoCa: len(b.Ca), Lap: 1, SoLuot: len(b.Ca), GoiMoiLuot: llm.MaxModelCallsPerTurn, LuotToiMoHinh: len(b.Ca)}
	p.XepLai = false
	d.phu(p)
	if p.Nhung {
		// The router embeds the message once (KhoViDu.Chon).
		d.NhungMoiLuot = 1
		d.Tran = d.SoLuot*(d.GoiMoiLuot+d.NhungMoiLuot) + d.NhungChung + d.GiamKhao
	}
	d.KyVongP95 = d.LuotToiMoHinh * 1
	return d
}
