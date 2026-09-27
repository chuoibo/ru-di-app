package traloi

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/crag"
	"mobile/services/core/internal/aiharness/guard"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// traLoiHe is the answer step's static system instruction.
//
//go:embed traloi.txt
var traLoiHe string

// Data block names of the answer step's user turns. Ours, never data.
const (
	KhoiCauHoi   prompts.Nguon = "cau_hoi"
	KhoiDanhGia  prompts.Nguon = "danh_gia"
	KhoiPhatHien prompts.Nguon = "phat_hien"
)

// DuTruTraLoi is how many model calls an answer needs to be released: the
// answer and its verification. With fewer left, no answer call is made (it
// could not be verified) and the fallback stands.
const DuTruTraLoi = 2

// maxRuneSua bounds the refused draft carried back as the model's turn.
const maxRuneSua = 6000

// MaxChoTrongPhan bounds the places part (tools.ProposePlaces: up to five).
const MaxChoTrongPhan = 5

// Vao is one retrieval turn's input, built by the engine from the router's
// output: the person's words and the request (source, the router's query
// text, hard constraints as ids and integers, soft preferences).
type Vao struct {
	Cau    string
	YeuCau truyhoi.YeuCau
}

// BoPhan are the answer step's parts. Verifier nil means VerifierLLM.
type BoPhan struct {
	crag.BoPhan
	Verifier kiemchung.Verifier
}

// KetThuc is how the step ended.
type KetThuc string

const (
	// DaTraLoi: a verified answer.
	DaTraLoi KetThuc = "tra_loi"
	// DaHoiLai: the fixed clarifying question about unmet constraints.
	DaHoiLai KetThuc = "hoi_lai"
	// DaTuChoi: the fixed refusal naming the unmet constraints.
	DaTuChoi KetThuc = "tu_choi"
	// DuPhong: the answer failed its checks (twice, or with no budget left
	// to regenerate, or the verifier's output was refused): the fixed
	// fallback sentence and, for places, the grounded cards.
	DuPhong KetThuc = "du_phong"
	// BiChan: the released text failed the last privacy format check.
	BiChan KetThuc = "bi_chan"
)

// NguonGoc is the provenance every part carries: the sources of the
// evidence it rests on, how many items, and the tools that returned them.
type NguonGoc struct {
	Nguon       []truyhoi.Nguon `json:"nguon"`
	SoBangChung int             `json:"so_bang_chung"`
	CongCu      []tools.Ten     `json:"cong_cu"`
}

// Kind values of a part: the same strings as aiharness.PhanKind.
const (
	KindText   = "text"
	KindPlaces = "places"
)

// Phan is one grounded part of the answer, ready for Sink.Phan.
type Phan struct {
	Kind string
	JSON json.RawMessage
}

type phanText struct {
	T        string   `json:"t"`
	NguonGoc NguonGoc `json:"nguon_goc"`
}

type phanPlaces struct {
	IDs      []string `json:"ids"`
	NguonGoc NguonGoc `json:"nguon_goc"`
}

// DauVet is what the step counted: numbers and enums only, never text.
type DauVet struct {
	// SoGoi is how many model calls this step made (grader, answers,
	// verifications).
	SoGoi int
	// SoBan is how many answer drafts were generated (0, 1 or 2).
	SoBan int
	// SoKiem is how many verifier calls ran.
	SoKiem int
	// SinhLai: the one regeneration ran.
	SinhLai bool
	// ViPham is the findings count of each draft, in order.
	ViPham []int
	// PhatHien are each draft's findings, in order.
	PhatHien []PhatHien
	// TokenLa, TokenHong count place tokens of the RELEASED answer that
	// named no evidence, or were broken (shown as «một chỗ» or removed).
	TokenLa, TokenHong int
	// KiemHong: the verifier's output was refused (fail closed).
	KiemHong bool
	// HetNganSach: the answer or the regeneration was skipped for budget.
	HetNganSach bool
}

// KetQua is the step's result.
type KetQua struct {
	KetThuc KetThuc
	// Chu is the released text.
	Chu string
	// LuaChon are the constraints offered as chips after a question back.
	LuaChon []truyhoi.RangBuoc
	Phan    []Phan
	Crag    crag.KetQua
	Vet     DauVet
}

// ErrNguonKhongHo: the step handles the places catalogue and the app manual.
var ErrNguonKhongHo = errors.New("traloi: unsupported retrieval source")

// CongCuCua is the tool a retrieval of n is recorded under.
func CongCuCua(n truyhoi.Nguon) (tools.Ten, error) {
	switch n {
	case truyhoi.Places:
		return tools.SearchPlaces, nil
	case truyhoi.Manual:
		return tools.SearchAppManual, nil
	}
	return "", fmt.Errorf("%w: %q", ErrNguonKhongHo, n)
}

// Chay runs the step: crag.TruyHoi, then the answer, checked, verified,
// regenerated at most once, or the fallback. Every model call goes through
// dem; every retrieval is a tool call on sc. A provider or budget error of a
// call the step cannot do without is returned; the grader's and the
// verifier's refused outputs are not errors (the grade is skipped, the
// answer falls back).
func Chay(ctx context.Context, v Vao, bp BoPhan, sc *tools.SoCai, dem *llm.Dem) (KetQua, error) {
	truoc := dem.SoGoi()
	kq, err := chay(ctx, v, bp, sc, dem)
	kq.Vet.SoGoi = dem.SoGoi() - truoc
	return kq, err
}

func chay(ctx context.Context, v Vao, bp BoPhan, sc *tools.SoCai, dem *llm.Dem) (KetQua, error) {
	cong, err := CongCuCua(v.YeuCau.Nguon)
	if err != nil {
		return KetQua{}, err
	}
	if bp.Verifier == nil {
		bp.Verifier = kiemchung.VerifierLLM{}
	}
	r, err := crag.TruyHoi(ctx, v.YeuCau, bp.BoPhan, cong, sc, dem)
	kq := KetQua{Crag: r}
	if err != nil {
		return kq, err
	}
	if len(r.BangChung) == 0 {
		return coDinh(kq, DaTuChoi, khongDat(r), cong), nil
	}
	if dem.ConLai() < DuTruTraLoi {
		kq.Vet.HetNganSach = true
		return duPhong(kq, cong, sc), nil
	}
	bi := make([]string, 0, len(r.BangChung))
	for _, b := range r.BangChung {
		if a, ok := sc.BiDanh(b.ID); ok {
			bi = append(bi, a)
		}
	}
	req := YeuCauTraLoi(v, r, bi, sc)
	for lan := 1; lan <= 2; lan++ {
		raw, err := cautruc.Goi(ctx, dem, req)
		kq.Vet.SoBan++
		if err != nil {
			return kq, err
		}
		b, xong, err := motBan(ctx, lan, raw, bi, r, bp, sc, dem, &kq)
		if err != nil {
			return kq, err
		}
		if xong != nil {
			return *xong, nil
		}
		kq.Vet.PhatHien = append(kq.Vet.PhatHien, b.ph)
		kq.Vet.ViPham = append(kq.Vet.ViPham, b.ph.SoViPham())
		// Fail closed: only a draft the verifier ran on and passed is
		// released; a refused verifier output releases nothing.
		if b.kiemHong {
			return duPhong(kq, cong, sc), nil
		}
		if b.kiemDat && (b.ph.Sach() || (lan == 2 && b.ph.ChiToken())) {
			return traLoi(kq, b, r, cong, sc), nil
		}
		if lan == 2 {
			return duPhong(kq, cong, sc), nil
		}
		if dem.ConLai() < DuTruTraLoi {
			kq.Vet.HetNganSach = true
			return duPhong(kq, cong, sc), nil
		}
		kq.Vet.SinhLai = true
		req = SuaLai(req, raw, b.ph)
	}
	return duPhong(kq, cong, sc), nil
}

// banDaKiem is a draft with its verification state.
type banDaKiem struct {
	ban
	// kiemDat: the verifier ran and passed it.
	kiemDat bool
	// kiemHong: the verifier's output was refused.
	kiemHong bool
}

// motBan checks one draft. xong is set when the draft ends the step on its
// own (a question back or a refusal: fixed sentences, nothing to verify).
func motBan(ctx context.Context, lan int, raw string, bi []string, r crag.KetQua, bp BoPhan, sc *tools.SoCai, dem *llm.Dem, kq *KetQua) (banDaKiem, *KetQua, error) {
	cong, _ := CongCuCua(r.YeuCau.Nguon)
	n, err := Doc([]byte(raw), bi)
	if err != nil {
		return banDaKiem{ban: ban{ph: PhatHien{LoiCauTruc: true}}}, nil, nil
	}
	switch n.HanhDong {
	case HoiLai:
		x := coDinh(*kq, DaHoiLai, n.RangBuocKhongDat, cong)
		return banDaKiem{}, &x, nil
	case TuChoi:
		x := coDinh(*kq, DaTuChoi, n.RangBuocKhongDat, cong)
		return banDaKiem{}, &x, nil
	}
	b := banDaKiem{ban: kiemBan(n, sc)}
	// The verifier runs on a draft that could be released: a clean one, or
	// on the last attempt one whose only findings are place tokens. A first
	// draft with any finding is regenerated whatever the verifier says, so
	// no call is spent on it.
	if !b.ph.Sach() && (lan == 1 || !b.ph.ChiToken()) {
		return b, nil, nil
	}
	kq.Vet.SoKiem++
	p, err := bp.Verifier.PhanTu(ctx, b.cau, bangChungCua(b.ids, sc), dem)
	if err != nil {
		if errors.Is(err, kiemchung.ErrCauTruc) {
			kq.Vet.KiemHong = true
			b.kiemHong = true
			return b, nil, nil
		}
		return b, nil, err
	}
	b.themPhanTu(p)
	b.kiemDat = p.Dat()
	return b, nil, nil
}

// bangChungCua is the evidence the verifier reads: the items the draft
// cites, or every item of the turn when it cites none (a sentence can still
// state a fact the verifier must check against something).
func bangChungCua(ids []string, sc *tools.SoCai) []truyhoi.BangChung {
	if len(ids) == 0 {
		ids = sc.IDs()
	}
	out := make([]truyhoi.BangChung, 0, len(ids))
	for _, id := range ids {
		if b, _, ok := sc.Lay(id); ok {
			out = append(out, b)
		}
	}
	return out
}

// YeuCauTraLoi is the answer step's first request, byte-stable for the same
// input.
func YeuCauTraLoi(v Vao, r crag.KetQua, bi []string, sc *tools.SoCai) *model.LLMRequest {
	return cautruc.YeuCau(strings.TrimSpace(traLoiHe), NoiDungTraLoi(v, r, sc), LuocDo(bi), MaxTokensTraLoi)
}

// NoiDungTraLoi is the answer step's user turn.
func NoiDungTraLoi(v Vao, r crag.KetQua, sc *tools.SoCai) string {
	parts := []string{
		prompts.BocDuLieuDanhDau(KhoiCauHoi, v.Cau),
		prompts.BocDuLieu(crag.KhoiYeuCau, crag.MoTaYeuCau(r.YeuCau)),
		cautruc.KhoiBangChung(crag.KhoiBangChung, r.BangChung, func(i int, b truyhoi.BangChung) string {
			a, _ := sc.BiDanh(b.ID)
			return a
		}),
	}
	if r.DanhGia != nil || r.Vong > 0 {
		var lines []string
		if r.DanhGia != nil {
			lines = append(lines, "ket_luan: "+string(r.DanhGia.KetLuan))
			if len(r.DanhGia.RangBuocThieu) > 0 {
				lines = append(lines, "rang_buoc_thieu: "+noiRangBuoc(r.DanhGia.RangBuocThieu))
			}
		}
		if len(r.NoiLong) > 0 {
			lines = append(lines, "da_bo_so_thich_mem: "+noiRangBuoc(r.NoiLong))
		}
		if r.Buoc == crag.DaVietLai {
			lines = append(lines, "da_viet_lai_truy_van: co")
		}
		parts = append(parts, prompts.BocDuLieu(KhoiDanhGia, strings.Join(lines, "\n")))
	}
	if b := moTaBiLoai(r.BiLoai); b != "" {
		parts = append(parts, prompts.BocDuLieu(crag.KhoiBiLoai, b))
	}
	parts = append(parts, "Trả lời câu hỏi trong khối cau_hoi theo schema.")
	return strings.Join(parts, "\n\n")
}

func noiRangBuoc(rs []truyhoi.RangBuoc) string {
	s := make([]string, len(rs))
	for i, r := range rs {
		s[i] = string(r)
	}
	return strings.Join(s, ", ")
}

func moTaBiLoai(m map[truyhoi.RangBuoc]int) string {
	var lines []string
	for _, r := range crag.RangBuocs.Values() {
		if n := m[truyhoi.RangBuoc(r)]; n > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d", r, n))
		}
	}
	return strings.Join(lines, "\n")
}

// SuaLai is the regeneration request: the first request, the refused draft
// as the model's turn, and the findings as a data block. The findings are
// indices, aliases, enums and flags (PhatHien): no text the verifier or any
// evidence wrote crosses into this prompt.
func SuaLai(req *model.LLMRequest, raw string, ph PhatHien) *model.LLMRequest {
	if utf8.RuneCountInString(raw) > maxRuneSua {
		raw = string([]rune(raw)[:maxRuneSua])
	}
	j, _ := json.Marshal(ph)
	return cautruc.Them(req, raw, prompts.BocDuLieu(KhoiPhatHien, string(j))+
		"\n\nCâu trả lời JSON ở trên bị từ chối vì các lỗi trong khối phat_hien. Viết lại toàn bộ JSON, sửa đúng những lỗi đó, theo schema và luật.")
}

// khongDat is what a refusal with no evidence names: the grader's unmet
// constraints, then every hard constraint the retrieval counted removals
// for, in the closed set's order.
func khongDat(r crag.KetQua) []truyhoi.RangBuoc {
	co := map[truyhoi.RangBuoc]bool{}
	if r.DanhGia != nil {
		for _, x := range r.DanhGia.RangBuocThieu {
			co[x] = true
		}
	}
	for x, n := range r.BiLoai {
		if n > 0 {
			co[x] = true
		}
	}
	var out []truyhoi.RangBuoc
	for _, x := range crag.RangBuocs.Values() {
		if co[truyhoi.RangBuoc(x)] {
			out = append(out, truyhoi.RangBuoc(x))
		}
	}
	return out
}

// khongLaLuaChon are constraints never offered as a chip to change: an
// allergy or a diet is the person's body, not a search setting.
var khongLaLuaChon = map[truyhoi.RangBuoc]bool{truyhoi.RBDiUng: true, truyhoi.RBAnKieng: true}

func tenRangBuoc(rs []truyhoi.RangBuoc) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

func nguonGoc(ids []string, sc *tools.SoCai) NguonGoc {
	ng := NguonGoc{Nguon: []truyhoi.Nguon{}, CongCu: []tools.Ten{}}
	coN, coC := map[truyhoi.Nguon]bool{}, map[tools.Ten]bool{}
	for _, id := range ids {
		b, t, ok := sc.Lay(id)
		if !ok {
			continue
		}
		ng.SoBangChung++
		if !coN[b.Nguon] {
			coN[b.Nguon] = true
			ng.Nguon = append(ng.Nguon, b.Nguon)
		}
		if !coC[t] {
			coC[t] = true
			ng.CongCu = append(ng.CongCu, t)
		}
	}
	return ng
}

// nguonGocTruyHoi is the provenance of a fixed sentence about a retrieval:
// the source searched, every item it returned, the tool that ran.
func nguonGocTruyHoi(r crag.KetQua, cong tools.Ten) NguonGoc {
	return NguonGoc{Nguon: []truyhoi.Nguon{r.YeuCau.Nguon}, SoBangChung: len(r.BangChung), CongCu: []tools.Ten{cong}}
}

func phanChu(t string, ng NguonGoc) Phan {
	j, _ := json.Marshal(phanText{T: t, NguonGoc: ng})
	return Phan{Kind: KindText, JSON: j}
}

func phanCho(ids []string, ng NguonGoc) Phan {
	j, _ := json.Marshal(phanPlaces{IDs: ids, NguonGoc: ng})
	return Phan{Kind: KindPlaces, JSON: j}
}

// coDinh ends with a fixed sentence about the constraints rs.
func coDinh(kq KetQua, k KetThuc, rs []truyhoi.RangBuoc, cong tools.Ten) KetQua {
	kq.KetThuc = k
	if k == DaHoiLai {
		kq.Chu = cau.HoiLai(tenRangBuoc(rs))
		for _, r := range rs {
			if !khongLaLuaChon[r] {
				kq.LuaChon = append(kq.LuaChon, r)
			}
		}
	} else {
		kq.Chu = cau.TuChoi(tenRangBuoc(rs))
	}
	kq.Phan = []Phan{phanChu(kq.Chu, nguonGocTruyHoi(kq.Crag, cong))}
	return kq
}

// placeIDs are the places among ids, at most MaxChoTrongPhan.
func placeIDs(ids []string, sc *tools.SoCai) []string {
	var out []string
	for _, id := range ids {
		if b, _, ok := sc.Lay(id); ok && b.Nguon == truyhoi.Places && len(out) < MaxChoTrongPhan {
			out = append(out, id)
		}
	}
	return out
}

// duPhong ends with the fixed fallback: for places, the sentence and the
// cards of the retrieval's first items (grounded by construction: they
// are the ledger's own); for the manual, the sentence alone.
func duPhong(kq KetQua, cong tools.Ten, sc *tools.SoCai) KetQua {
	kq.KetThuc = DuPhong
	kq.Vet.TokenLa, kq.Vet.TokenHong = 0, 0
	ng := nguonGocTruyHoi(kq.Crag, cong)
	if kq.Crag.YeuCau.Nguon != truyhoi.Places {
		kq.Chu = cau.DuPhongHuongDan
		kq.Phan = []Phan{phanChu(kq.Chu, ng)}
		return kq
	}
	var ids []string
	for _, b := range kq.Crag.BangChung {
		ids = append(ids, b.ID)
	}
	ids = placeIDs(ids, sc)
	kq.Chu = cau.DuPhong
	kq.Phan = []Phan{phanChu(kq.Chu, ng), phanCho(ids, nguonGoc(ids, sc))}
	return kq
}

// traLoi releases a verified draft: the relaxation note first when a round
// relaxed soft constraints, the sentences, the places it cites as a card
// part; the privacy format check runs last on the whole released text.
func traLoi(kq KetQua, b banDaKiem, r crag.KetQua, cong tools.Ten, sc *tools.SoCai) KetQua {
	cauRa := b.cau
	if note := cau.DaNoiLong(tenRangBuoc(r.NoiLong)); note != "" {
		cauRa = append([]string{note}, cauRa...)
	}
	chu := strings.Join(cauRa, " ")
	if guard.DinhDang(chu) != guard.RaSach {
		kq.KetThuc = BiChan
		kq.Chu = cau.Cau(cau.TraLoiBiChan)
		kq.Phan = []Phan{phanChu(kq.Chu, nguonGocTruyHoi(r, cong))}
		return kq
	}
	kq.KetThuc, kq.Chu = DaTraLoi, chu
	kq.Vet.TokenLa, kq.Vet.TokenHong = b.tokenLa, b.tokenHong
	kq.Phan = []Phan{phanChu(chu, nguonGoc(b.ids, sc))}
	if ids := placeIDs(b.ids, sc); len(ids) > 0 {
		kq.Phan = append(kq.Phan, phanCho(ids, nguonGoc(ids, sc)))
	}
	return kq
}
