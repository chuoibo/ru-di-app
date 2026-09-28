package aiharness

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/chiabill"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/money"
)

// The group's split draft (design 03 §4.3): ONE structured reading of the
// shared messages (chiabill), then chiaBillParts, a pure function, builds
// the draft. AI does not touch money: nothing is recorded, no expense, no
// obligation. The draft goes on the invocation's result column for a person
// to confirm in the app, where the confirm flow and the allocator decide;
// the card only shows the proposal.

// nhapChiaBill runs the reading, checks it, verifies it and builds the
// card. Only messages whose author the server confirmed AND whose text the
// server itself stores are offered (a payer is never a name the model
// wrote, and the words that bill an author are the stored message's, never
// the client's copy of it), and the caller's own request, paid by the
// caller. Every amount must be supported by the message it names, first
// structurally (chiabill.Doc) and then by the draft's verifier in a fresh
// context (chiabill.Kiem); an amount either does not support ends the turn
// with the fixed question back, and a verifier that cannot be read ends it
// with no draft: nothing the verifier did not pass leaves the server.
func (e *Engine) nhapChiaBill(ctx context.Context, t Turn, rec *obs.TurnRecord, dem *llm.Dem, hoi string, chung []luotNhomSach, kq hieu.KetQua) (Result, error) {
	// The reading and its verifier.
	if dem.ConLai() < 2 {
		return Result{}, &Loi{Ma: cau.HetNganSach}
	}
	v := chiabill.Vao{LoiNho: hoi}
	tacGia := map[string]string{chiabill.BiDanhLoiNho: t.NguoiHoi}
	nguonID := map[string]string{}
	for _, c := range chung {
		if c.tacGia == "" || c.luot.Vai == trinho.TroLy {
			continue
		}
		if c.chuMayChu == "" {
			// The server cannot read this message's text (a v2 room, a
			// message since removed): no payer attribution from it.
			continue
		}
		bi := chiabill.BiDanhTin(len(v.Tin))
		ten := c.ten
		if c.tacGia == t.NguoiHoi {
			ten = tenThanhVien(t.ThanhVien, t.NguoiHoi)
		}
		v.Tin = append(v.Tin, chiabill.Tin{BiDanh: bi, Ten: ten, Chu: c.chuMayChu})
		tacGia[bi], nguonID[bi] = c.tacGia, c.id
	}
	ks, err := chiabill.Goi(ctx, dem, v)
	if err != nil {
		switch {
		case errors.Is(err, chiabill.ErrSoTienKhongKhop):
			// The model named an amount its message does not state: ask back,
			// build nothing.
			return theMotChu(cau.NhomChuaChacSoTien), nil
		case errors.Is(err, chiabill.ErrCauTruc) || errors.Is(err, chiabill.ErrVao):
			rec.LoiMoHinh = obs.LoiBadResp
			return Result{}, &Loi{Ma: cau.InvalidAIResult}
		}
		return Result{}, err
	}
	khoan := make([]khoanNhap, 0, len(ks))
	for _, k := range ks {
		khoan = append(khoan, khoanNhap{tieuDe: k.TieuDe, soTien: k.SoTienVND, nguoiTra: tacGia[k.Tin], nguonTin: nguonID[k.Tin]})
	}
	nguoi := nguoiChia(t.ThanhVien, kq.Slots.NguoiThamGia)
	n, err := chiaBillPartsCuoi(khoan, nguoi, t.ThanhVien, cauPhong(t, cuoiNhapNhom, cuoiNhapDoi))
	if errors.Is(err, errKhongKhoan) {
		return theMotChu(cauPhong(t, cau.NhomChuaThayKhoan, cau.DoiChuaThayKhoan)), nil
	}
	if err != nil {
		rec.LoiMoHinh = obs.LoiBadResp
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	// The verifier, in a fresh context, on every item before any of it
	// leaves. It fails closed: an unreadable output releases nothing, an
	// unsupported item withholds the whole draft.
	dat, err := chiabill.Kiem(ctx, dem, v, ks, tenThanhVien(t.ThanhVien, t.NguoiHoi))
	switch {
	case errors.Is(err, chiabill.ErrCauTruc):
		rec.KetKiem, rec.LoiMoHinh = obs.KiemHong, obs.LoiBadResp
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	case err != nil:
		return Result{}, err
	case !dat:
		rec.KetKiem = obs.KiemKhongDat
		return theMotChu(cau.NhomChuaChacSoTien), nil
	}
	rec.KetKiem = obs.KiemDat
	// Our template around verbatim spans and integers: the structural
	// output checks still run (no contact detail, no marker, the length).
	if _, err := e.kiemDauRaK(khuonNhom(t.Doi), n.chu, rec); err != nil {
		return Result{}, err
	}
	res := theMotChu(n.chu)
	res.Phan = append(res.Phan, phan(PhanExpenseDraft, map[string]any{"so_khoan": len(khoan), "da_ghi": []int{}}))
	res.KetQuaNhap = n.ketQua
	return res, nil
}

// nguoiChia is who shares the draft: the members the router named
// (nguoi_tham_gia, aliases of the turn's closed list), else every active
// member.
func nguoiChia(ts []ThanhVienNhom, biDanh []string) []string {
	var out []string
	for i, m := range ts {
		if len(biDanh) == 0 || coChuoi(biDanh, biDanhThanhVien(i)) {
			out = append(out, m.ID)
		}
	}
	sort.Strings(out)
	return out
}

func tenThanhVien(ts []ThanhVienNhom, id string) string {
	for _, m := range ts {
		if m.ID == id {
			return m.Ten
		}
	}
	return ""
}

// khoanNhap is one expense of the draft.
type khoanNhap struct {
	tieuDe   string
	soTien   int64
	nguoiTra string
	// nguonTin is the shared message it came from; "" for the request.
	nguonTin string
}

// nhapChia is the built draft.
type nhapChia struct {
	chu string
	// chiaDeu is the equal-split preview, in the participants' order;
	// Σ = tong exactly.
	chiaDeu []allocator.Share
	tong    int64
	ketQua  json.RawMessage
}

var (
	errKhongKhoan   = errors.New("aiharness: no expense to split")
	errKhongNguoi   = errors.New("aiharness: nobody to share the draft")
	errTongVuotTran = errors.New("aiharness: the draft's total is past the ledger's bound")
	errLechTong     = errors.New("aiharness: the preview does not add up to the total")
)

const nguoiKhongTen = "Một người trong nhóm"

// The draft's closing line: a room of friends', and a couple's.
const (
	cuoiNhapNhom = "Mọi người xem lại rồi xác nhận ở mục Chia bill. Rủ Đi AI không tự ghi khoản nào."
	cuoiNhapDoi  = "Hai bạn xem lại rồi xác nhận ở mục Chia bill. Rủ Đi AI không tự ghi khoản nào."
)

// chiaBillParts builds the split draft from the checked items, a pure
// function of its arguments (money law 3). Every amount is whole đồng in an
// int64 (law 1: no float, no Decimal, even in between); the total is their
// sum, refused past allocator.MaxAmountVND; the equal-split preview is the
// domain allocator's own apportionment of that total over the participants
// (largest remainder), and it must add up to the total exactly (law 2), or
// nothing is built. The card text is our template around the payer's label,
// the amount and the verbatim title; the result keeps v1's expense_draft
// shape, every draft needs_review, plus the preview.
func chiaBillParts(ks []khoanNhap, nguoi []string, ts []ThanhVienNhom) (nhapChia, error) {
	return chiaBillPartsCuoi(ks, nguoi, ts, cuoiNhapNhom)
}

// chiaBillPartsCuoi is chiaBillParts with the template's closing line, ours,
// chosen by the room's class: who confirms is «mọi người» in a room of
// friends and «hai bạn» for a couple. Nothing else of the draft changes.
func chiaBillPartsCuoi(ks []khoanNhap, nguoi []string, ts []ThanhVienNhom, cuoi string) (nhapChia, error) {
	if len(ks) == 0 {
		return nhapChia{}, errKhongKhoan
	}
	if len(nguoi) == 0 {
		return nhapChia{}, errKhongNguoi
	}
	var tong int64
	items := make([]allocator.Item, 0, len(ks))
	for i, k := range ks {
		if k.soTien < 1 || k.soTien > int64(allocator.MaxAmountVND) || tong > int64(allocator.MaxAmountVND)-k.soTien {
			return nhapChia{}, errTongVuotTran
		}
		tong += k.soTien
		items = append(items, allocator.Item{ItemID: "k" + strconv.Itoa(i+1), AmountVND: money.VND(k.soTien), SharedBy: nguoi})
	}
	r, err := allocator.Allocate(allocator.Expense{Participants: nguoi, TotalVND: money.VND(tong), Items: items})
	if err != nil {
		return nhapChia{}, err
	}
	var sum int64
	for _, a := range r.Allocations {
		sum += int64(a.AmountVND)
	}
	if sum != tong || len(r.Allocations) != len(nguoi) {
		return nhapChia{}, errLechTong
	}
	ten := map[string]string{}
	for _, m := range ts {
		ten[m.ID] = m.Ten
	}
	dong := []string{"Đề xuất chia bill, chưa ghi vào sổ:"}
	for _, k := range ks {
		tra := ten[k.nguoiTra]
		if tra == "" {
			tra = nguoiKhongTen
		}
		d := "• " + catChuNhom(tra, 24) + " trả " + dinhDangDong(k.soTien)
		if k.tieuDe != "" {
			d += ": " + catChuNhom(k.tieuDe, chiabill.MaxChuTieuDe)
		}
		dong = append(dong, d)
	}
	dong = append(dong, "Tổng "+dinhDangDong(tong)+". Chia đều cho "+strconv.Itoa(len(nguoi))+" người: "+moTaChiaDeu(r.Allocations)+".")
	dong = append(dong, cuoi)
	kq, err := ketQuaNhapJSON(ks, nguoi, r.Allocations)
	if err != nil {
		return nhapChia{}, err
	}
	return nhapChia{chu: strings.Join(dong, "\n"), chiaDeu: r.Allocations, tong: tong, ketQua: kq}, nil
}

// moTaChiaDeu says the preview in counts, largest share first: «2 người
// 283.334đ, 1 người 283.332đ». No name: the room reads it.
func moTaChiaDeu(as []allocator.Share) string {
	dem := map[int64]int{}
	var muc []int64
	for _, a := range as {
		v := int64(a.AmountVND)
		if dem[v] == 0 {
			muc = append(muc, v)
		}
		dem[v]++
	}
	sort.Slice(muc, func(i, j int) bool { return muc[i] > muc[j] })
	parts := make([]string, 0, len(muc))
	for _, v := range muc {
		parts = append(parts, strconv.Itoa(dem[v])+" người "+dinhDangDong(v))
	}
	return strings.Join(parts, ", ")
}

// ketQuaNhapJSON is the invocation's result: v1's expense_draft drafts
// (who paid, how much, the proposed people, the source message, every one
// for review) and the equal-split preview.
func ketQuaNhapJSON(ks []khoanNhap, nguoi []string, as []allocator.Share) (json.RawMessage, error) {
	type draft struct {
		Title           string   `json:"title"`
		AmountVND       int64    `json:"amount_vnd"`
		PaidByID        string   `json:"paid_by_id"`
		SharedBy        []string `json:"shared_by"`
		SourceMessageID *string  `json:"source_message_id"`
		NeedsReview     bool     `json:"needs_review"`
	}
	type phanChia struct {
		PersonID  string `json:"person_id"`
		AmountVND int64  `json:"amount_vnd"`
	}
	out := struct {
		Kind    string     `json:"kind"`
		Drafts  []draft    `json:"drafts"`
		ChiaDeu []phanChia `json:"chia_deu"`
	}{Kind: "expense_draft"}
	for _, k := range ks {
		d := draft{Title: k.tieuDe, AmountVND: k.soTien, PaidByID: k.nguoiTra, SharedBy: append([]string(nil), nguoi...), NeedsReview: true}
		if k.nguonTin != "" {
			s := k.nguonTin
			d.SourceMessageID = &s
		}
		out.Drafts = append(out.Drafts, d)
	}
	for _, a := range as {
		out.ChiaDeu = append(out.ChiaDeu, phanChia{PersonID: a.ParticipantID, AmountVND: int64(a.AmountVND)})
	}
	return json.Marshal(out)
}

// dinhDangDong writes whole đồng with dot grouping: 1250000 -> "1.250.000đ".
func dinhDangDong(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String() + "đ"
}

// catChuNhom cuts s to n runes with an ellipsis.
func catChuNhom(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n-1])) + "…"
}
