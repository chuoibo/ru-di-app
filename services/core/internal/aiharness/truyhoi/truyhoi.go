// Package truyhoi is the engine's retrieval port: what the engine asks
// (YeuCau), what comes back (BangChung, KetQuaTruyHoi) and the two
// interfaces an adapter implements (Retriever, Reranker). It is transport
// agnostic on purpose: the lexical adapter over package rag, the Milvus
// hybrid dense+sparse adapter and the Qwen reranker over HTTP (infra track)
// all implement these same types, and the engine never learns which one
// answered except through the Degraded flags.
//
// Who decides what (docs/architecture/03-ai-engine-hop-dong.md, «Luật không
// heuristic»):
//   - WHETHER to retrieve, from WHICH source, with WHAT query text and WHICH
//     constraints: the model (router slots, tool arguments). No adapter
//     reads the query's words to find a destination, an allergy, a diet or a
//     time; those arrive already extracted as ids in Cung.
//   - Cung (hard constraints) are applied by the adapter as non-relaxable
//     filters. No retry, rerank or corrective round widens them.
//   - Mem (soft preferences) only rank; a corrective round may drop one of
//     them when the model's grader proposes it (crag.DanhGia.NoiLong).
//   - Ranking is the adapter's (BM25, dense, sparse, RRF, reranker). That
//     is retrieval, not understanding: it orders evidence, it never decides
//     what the person asked for.
package truyhoi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mobile/services/core/internal/aiharness/dong"
)

// Nguon is a retrieval source. The router's CanTruyHoi lists the ones the
// model judged necessary; an empty list means answer directly.
type Nguon string

const (
	// Places: the catalogue index (rag).
	Places Nguon = "places"
	// Manual: the app manual (huongdan).
	Manual Nguon = "manual"
	// Memory: the person's long-term facts (trinho.TriNho), Nếp only.
	Memory Nguon = "memory"
	// GroupHistory: the group's outings and snapshot, group assistant only.
	GroupHistory Nguon = "group_history"
)

// Nguons is the closed set of Nguon.
var Nguons = dong.Moi("nguon_truy_hoi", Places, Manual, Memory, GroupHistory)

// RangBuoc names one constraint of a retrieval, hard or soft. The CRAG
// grader reports missing constraints and proposes relaxations with these
// names (crag).
type RangBuoc string

// Hard constraints: never relaxed.
const (
	RBDiemDen  RangBuoc = "diem_den"
	RBDiUng    RangBuoc = "di_ung"
	RBAnKieng  RangBuoc = "an_kieng"
	RBMoLuc    RangBuoc = "mo_luc"
	RBNganSach RangBuoc = "ngan_sach"
)

// Soft constraints: a corrective round may relax one.
const (
	RBKhiChat RangBuoc = "khi_chat"
	RBLoaiCho RangBuoc = "loai_cho"
	RBKhuVuc  RangBuoc = "khu_vuc"
)

// RangBuocCungs is the closed set of hard constraint names.
var RangBuocCungs = dong.Moi("rang_buoc_cung", RBDiemDen, RBDiUng, RBAnKieng, RBMoLuc, RBNganSach)

// RangBuocMems is the closed set of soft constraint names.
var RangBuocMems = dong.Moi("rang_buoc_mem", RBKhiChat, RBLoaiCho, RBKhuVuc)

// Cung are the hard constraints the model extracted. Every field is an id,
// an instant or an integer; none is free text. Zero values mean «not
// constrained».
type Cung struct {
	// DiemDenID is a destination id from the turn's closed list.
	DiemDenID string
	// DiUng and AnKieng are ids of the closed allergen and diet vocabularies
	// (hieu.DiUngs, hieu.AnKiengs). A place that contains an allergen, or
	// its family, is out; a place must declare every diet.
	DiUng   []string
	AnKieng []string
	// MoLuc is an instant the place must be open at, computed by Go from
	// the model's ISO date and the start of a time the model gave with no
	// end, and the turn's «now».
	MoLuc *time.Time
	// MoTrong is a window the place must be open at SOME point of, computed
	// by Go from the model's ISO date and its time window (both ends). A
	// window the model gave is never narrowed to its first instant: «tối
	// nay» (18:00–22:00) keeps a place that opens at 19:00.
	MoTrong *KhungMo
	// NganSachVND is the ceiling per person in whole đồng.
	NganSachVND *int64
}

// KhungMo is a window [Tu, Den) of instants; Den is after Tu, and a window
// that crosses midnight ends on the next day.
type KhungMo struct {
	Tu, Den time.Time
}

// MaxKhungMo bounds a window: a day and a night.
const MaxKhungMo = 24 * time.Hour

// ErrXungDot: two hard constraints that cannot both hold (two destinations,
// two different open instants).
var ErrXungDot = errors.New("truyhoi: conflicting hard constraints")

// HopChat merges the router's hard constraints c with those a tool call
// carries (o), stricter wins: allergens and diets are united, the budget is
// the lower one, a destination or an open instant set on one side is kept,
// and set on both sides must agree. The model can add a constraint through a
// tool argument but never remove one the router extracted (research
// agentic-rag-tools §4.3). Order of c's lists is kept, o's new ids follow.
func (c Cung) HopChat(o Cung) (Cung, error) {
	out := Cung{DiUng: hop(c.DiUng, o.DiUng), AnKieng: hop(c.AnKieng, o.AnKieng)}
	switch {
	case c.DiemDenID == "" || c.DiemDenID == o.DiemDenID:
		out.DiemDenID = o.DiemDenID
	case o.DiemDenID == "":
		out.DiemDenID = c.DiemDenID
	default:
		return Cung{}, fmt.Errorf("%w: destination", ErrXungDot)
	}
	switch {
	case c.MoLuc == nil:
		out.MoLuc = o.MoLuc
	case o.MoLuc == nil || o.MoLuc.Equal(*c.MoLuc):
		out.MoLuc = c.MoLuc
	default:
		return Cung{}, fmt.Errorf("%w: open instant", ErrXungDot)
	}
	switch {
	case c.MoTrong == nil:
		out.MoTrong = o.MoTrong
	case o.MoTrong == nil || (o.MoTrong.Tu.Equal(c.MoTrong.Tu) && o.MoTrong.Den.Equal(c.MoTrong.Den)):
		out.MoTrong = c.MoTrong
	default:
		return Cung{}, fmt.Errorf("%w: open window", ErrXungDot)
	}
	// An instant and a window both set: the instant must fall inside the
	// window (it is then the stricter of the two, and both stand).
	if out.MoLuc != nil && out.MoTrong != nil && (out.MoLuc.Before(out.MoTrong.Tu) || !out.MoLuc.Before(out.MoTrong.Den)) {
		return Cung{}, fmt.Errorf("%w: open instant outside the window", ErrXungDot)
	}
	switch {
	case c.NganSachVND == nil:
		out.NganSachVND = o.NganSachVND
	case o.NganSachVND == nil || *c.NganSachVND <= *o.NganSachVND:
		out.NganSachVND = c.NganSachVND
	default:
		out.NganSachVND = o.NganSachVND
	}
	return out, nil
}

func hop(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(a)+len(b))
	var out []string
	for _, s := range append(append([]string(nil), a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// Mem are soft preferences: they rank, never filter.
type Mem struct {
	LoaiCho []string
	KhiChat []string
	KhuVuc  string
}

// Bo returns m without the soft constraint r (one corrective round).
func (m Mem) Bo(r RangBuoc) (Mem, error) {
	switch r {
	case RBLoaiCho:
		m.LoaiCho = nil
	case RBKhiChat:
		m.KhiChat = nil
	case RBKhuVuc:
		m.KhuVuc = ""
	default:
		return m, fmt.Errorf("%w: %q is not a soft constraint", dong.ErrLa, r)
	}
	return m, nil
}

// MaxK bounds how many evidence items one retrieval returns.
const MaxK = 50

// YeuCau is one retrieval.
type YeuCau struct {
	Nguon Nguon
	// Cau is the query text: what the model wrote as the tool argument, or
	// the person's own words on the fast path. Adapters embed it and rank
	// with it; they never parse constraints out of it.
	Cau string
	// CauCoDau is the router's diacritics-restored form of Cau ("" when the
	// router wrote none, or Cau already has its marks). Written by the
	// model, never computed: restoring Vietnamese diacritics is a reading
	// of the words. The dense leg, the marked BM25 field and the reranker
	// use it; the folded BM25 field keeps Cau, the person's own spelling.
	CauCoDau string
	Cung     Cung
	Mem      Mem
	// K is how many items to return; 0 means the adapter's default.
	K int
	// DiUngNgoaiDanhMuc is the router's flag: the person named an allergen
	// outside the closed list, which no filter here can check. It reaches
	// the grader and the answer step with the request (crag.MoTaYeuCau).
	DiUngNgoaiDanhMuc bool
}

// ErrYeuCau: a structurally invalid request.
var ErrYeuCau = errors.New("truyhoi: invalid request")

// Kiem checks a request's structure: known source, K in range, no negative
// budget. Membership of ids (destination, allergens) is checked by the
// caller against the turn's closed lists before the request is built.
func (y YeuCau) Kiem() error {
	if !Nguons.Co(y.Nguon) {
		return fmt.Errorf("%w: nguon %q", ErrYeuCau, y.Nguon)
	}
	if y.K < 0 || y.K > MaxK {
		return fmt.Errorf("%w: k %d", ErrYeuCau, y.K)
	}
	if y.Cung.NganSachVND != nil && *y.Cung.NganSachVND < 0 {
		return fmt.Errorf("%w: negative budget", ErrYeuCau)
	}
	if k := y.Cung.MoTrong; k != nil && (!k.Den.After(k.Tu) || k.Den.Sub(k.Tu) > MaxKhungMo) {
		return fmt.Errorf("%w: open window", ErrYeuCau)
	}
	return nil
}

// BangChung is one evidence item, the only thing an answer may cite.
type BangChung struct {
	// ID is the item's stable id (a place id, a manual section id, a fact
	// id, an outing id). Grounding checks answers by membership of this id.
	ID    string
	Nguon Nguon
	// Diem is the adapter's retrieval score (a fused RRF value, a BM25
	// score). The slice order is the ranking; Diem is informational and
	// never compared across adapters.
	Diem float64
	// DiemXepLai is the reranker's relevance score when a reranker ordered
	// the item (0 otherwise). It only explains the order: nothing may
	// threshold on it (a small reranker scores relevant documents and
	// lexical traps alike near 1), so it never replaces Diem.
	DiemXepLai float64
	// Truong are the evidence fields an answer may quote: ten, gia, gio,
	// nhan_nut, dia_chi, … Numbers and hours an answer states are checked
	// against these (kiemchung.KiemTra).
	Truong map[string]string
	// PhienBanChiMuc names the index version that answered ("" for live
	// rows), so a golden and a log row can say which index they saw.
	PhienBanChiMuc string
}

// CoSuyGiam flags a degraded retrieval: the adapter answered with less than
// its full pipeline.
type CoSuyGiam string

const (
	LexicalOnly CoSuyGiam = "lexical_only"
	NoVector    CoSuyGiam = "no_vector"
	NoSparse    CoSuyGiam = "no_sparse"
	NoRerank    CoSuyGiam = "no_rerank"
)

// CoSuyGiams is the closed set of CoSuyGiam.
var CoSuyGiams = dong.Moi("co_suy_giam", LexicalOnly, NoVector, NoSparse, NoRerank)

// KetQuaTruyHoi is a retrieval's answer.
type KetQuaTruyHoi struct {
	BangChung []BangChung
	Degraded  []CoSuyGiam
	// BiLoai counts, per hard constraint, the candidates it removed, so the
	// model can tell the person why there are few results instead of
	// guessing, and never relaxes on its own. Counts only: no ids, no text.
	BiLoai map[RangBuoc]int
}

// Retriever answers one request. It applies Cung as filters, ranks by Cau
// and Mem, and reports every degradation. It must not widen Cung on a
// shortfall: returning fewer items is the correct answer.
type Retriever interface {
	Tim(ctx context.Context, y YeuCau) (KetQuaTruyHoi, error)
}

// Reranker reorders evidence against the query and keeps the best topN. It
// never adds an item, and never drops one except by the topN cut.
type Reranker interface {
	XepLai(ctx context.Context, cau string, bc []BangChung, topN int) ([]BangChung, error)
}

// Passthrough is the Reranker used when no reranker is configured: it keeps
// the order and cuts at topN (topN <= 0 keeps all). The caller adds
// NoRerank to the result's flags.
type Passthrough struct{}

// XepLai keeps the order.
func (Passthrough) XepLai(_ context.Context, _ string, bc []BangChung, topN int) ([]BangChung, error) {
	if topN <= 0 || topN >= len(bc) {
		return append([]BangChung(nil), bc...), nil
	}
	return append([]BangChung(nil), bc[:topN]...), nil
}

// UngVienXepLai is how many candidates, in retrieval order, a reranker is
// given at most (research qwen-reranker.md §5.3: 30 in, the request's k
// out). Candidates past it are cut before the reranker, never after.
const UngVienXepLai = 30

// xepLaiLuot is the turn's reranker as the context carries it.
type xepLaiLuot struct {
	r Reranker
	// hoan: the caller reranks what the retriever returns (the corrective
	// loop reranks the merged candidates of every query once), so the
	// retriever neither reranks nor flags NoRerank.
	hoan bool
}

type khoaXepLai struct{}

// VoiXepLai is ctx carrying the turn's reranker (a counted one: the engine
// wraps the configured reranker in rerank.Dem with MaxRerankCallsPerTurn).
// A retriever shared across turns reranks with the one its turn carries,
// so every rerank call of a turn, whichever path makes it, is counted
// against one budget.
func VoiXepLai(ctx context.Context, r Reranker) context.Context {
	return context.WithValue(ctx, khoaXepLai{}, xepLaiLuot{r: r})
}

// HoanXepLai is ctx telling the retriever that its caller reranks: the
// retriever returns its candidates in retrieval order (up to the request's
// K) and adds no NoRerank flag, which the caller adds if its own rerank
// does not happen.
func HoanXepLai(ctx context.Context) context.Context {
	x, _ := ctx.Value(khoaXepLai{}).(xepLaiLuot)
	x.hoan = true
	return context.WithValue(ctx, khoaXepLai{}, x)
}

// XepLaiTrong is the turn's reranker ctx carries (nil: none), and whether
// the caller reranks instead (HoanXepLai).
func XepLaiTrong(ctx context.Context) (r Reranker, hoan bool) {
	x, _ := ctx.Value(khoaXepLai{}).(xepLaiLuot)
	return x.r, x.hoan
}
