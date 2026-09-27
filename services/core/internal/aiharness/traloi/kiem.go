package traloi

import (
	"strings"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/guard"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/tools"
)

// TruongTen is the evidence field a place token is replaced with.
const TruongTen = "ten"

// TrichLech is a stated value that differs from its evidence field. It
// carries indices and enums only: it goes back to the model as data on a
// regeneration, and no free text crosses into a prompt that way.
type TrichLech struct {
	Cau    int    `json:"cau"`
	BiDanh string `json:"bi_danh"`
	Truong Truong `json:"truong"`
}

// PhatHien is what the checks found in one draft: sentence indices
// (1-based), aliases, enums and flags, never text. It is the findings block
// of the one regeneration.
type PhatHien struct {
	// LoiCauTruc: the draft did not parse against the schema.
	LoiCauTruc bool `json:"loi_cau_truc,omitempty"`
	// BiDanhLa: sentences with a place token naming no evidence of the turn.
	BiDanhLa []int `json:"cau_co_token_la,omitempty"`
	// TokenHong: sentences with a broken token.
	TokenHong []int `json:"cau_co_token_hong,omitempty"`
	// TrichLech: stated values that differ from the evidence field.
	TrichLech []TrichLech `json:"gia_tri_lech,omitempty"`
	// TrichXa: sentences stating a value not next to its place (no token or
	// citation of that alias in the sentence) or not written in it.
	TrichXa []int `json:"cau_gia_tri_khong_canh_cho,omitempty"`
	// NhanNut: sentences quoting a button label no manual evidence carries.
	NhanNut []int `json:"cau_nhan_nut_khong_co,omitempty"`
	// DinhDang: sentences carrying a phone, account or card number, or an
	// email (format check).
	DinhDang []int `json:"cau_du_lieu_rieng_tu,omitempty"`
	// KhongHoTro: sentences the verifier judged unsupported.
	KhongHoTro []int `json:"cau_khong_duoc_bang_chung_ho_tro,omitempty"`
	// HuaHanhDong, Tien: the verifier's flags.
	HuaHanhDong bool `json:"hua_hanh_dong_khong_co,omitempty"`
	Tien        bool `json:"tien,omitempty"`
}

// ChiToken reports whether the only findings are place tokens that named
// no evidence or were broken: the kind a released answer may still carry,
// each token shown as «một chỗ» and counted (design 01 §3.5).
func (p PhatHien) ChiToken() bool {
	q := p
	q.BiDanhLa, q.TokenHong = nil, nil
	return q.Sach()
}

// Sach reports whether nothing was found.
func (p PhatHien) Sach() bool {
	return !p.LoiCauTruc && len(p.BiDanhLa) == 0 && len(p.TokenHong) == 0 && len(p.TrichLech) == 0 &&
		len(p.TrichXa) == 0 && len(p.NhanNut) == 0 && len(p.DinhDang) == 0 && len(p.KhongHoTro) == 0 &&
		!p.HuaHanhDong && !p.Tien
}

// SoViPham counts the findings: each sentence index, each value and each
// flag once.
func (p PhatHien) SoViPham() int {
	n := len(p.BiDanhLa) + len(p.TokenHong) + len(p.TrichLech) + len(p.TrichXa) + len(p.NhanNut) + len(p.DinhDang) + len(p.KhongHoTro)
	for _, b := range []bool{p.LoiCauTruc, p.HuaHanhDong, p.Tien} {
		if b {
			n++
		}
	}
	return n
}

// ban is one draft, checked and rendered.
type ban struct {
	nhap Nhap
	// cau are the sentences as the person would read them: tokens replaced
	// by names from the ledger.
	cau []string
	// ids are the evidence ids the draft cites, first citation first.
	ids       []string
	ph        PhatHien
	tokenLa   int
	tokenHong int
}

// kiemBan runs the deterministic checks on a parsed answer: token
// resolution against the ledger, kiemchung.Kiem (cited ids ⊆ evidence,
// stated values equal to the evidence field, button labels among the
// manual evidence's labels), values next to their place, and the privacy
// format check of what the model wrote.
func kiemBan(n Nhap, sc *tools.SoCai) ban {
	b := ban{nhap: n}
	daCo := map[string]bool{}
	nhan := func(id string) {
		if !daCo[id] {
			daCo[id] = true
			b.ids = append(b.ids, id)
		}
	}
	ten := func(a string) (string, bool) {
		id, ok := sc.TuBiDanh(a)
		if !ok {
			return cau.MotCho, false
		}
		bc, _, _ := sc.Lay(id)
		if t := strings.TrimSpace(bc.Truong[TruongTen]); t != "" {
			return t, true
		}
		return cau.MotCho, true
	}
	for i, c := range n.Cau {
		so := i + 1
		pt := tachCau(c.Chu)
		chu, la := pt.ghep(ten)
		b.cau = append(b.cau, chu)
		if pt.hong > 0 {
			b.tokenHong += pt.hong
			b.ph.TokenHong = append(b.ph.TokenHong, so)
		}
		if la > 0 {
			b.tokenLa += la
			b.ph.BiDanhLa = append(b.ph.BiDanhLa, so)
		}
		canh := map[string]bool{}
		var tb kiemchung.TuyenBo
		for _, a := range pt.biDanh {
			canh[a] = true
			if id, ok := sc.TuBiDanh(a); ok {
				tb.IDs = append(tb.IDs, id)
				nhan(id)
			}
		}
		for _, a := range c.BangChung {
			canh[a] = true
			id, _ := sc.TuBiDanh(a)
			tb.IDs = append(tb.IDs, id)
			nhan(id)
		}
		moHinh := pt.chuMoHinh()
		xa := false
		for _, t := range c.Trich {
			id, _ := sc.TuBiDanh(t.BiDanh)
			tb.So = append(tb.So, kiemchung.TrichSo{ID: id, Truong: string(t.Truong), GiaTri: t.GiaTri})
			nhan(id)
			if !canh[t.BiDanh] || !strings.Contains(moHinh, t.GiaTri) {
				xa = true
			}
		}
		if xa {
			b.ph.TrichXa = append(b.ph.TrichXa, so)
		}
		tb.NhanNut = pt.nhan
		k := kiemchung.Kiem(tb, sc)
		if len(k.IDNgoai) > 0 && la == 0 {
			// Every alias here passed Doc's enum, so this is a ledger that
			// changed under us: read it as a token naming nothing.
			b.ph.BiDanhLa = append(b.ph.BiDanhLa, so)
		}
		for _, l := range k.SoLech {
			a, _ := sc.BiDanh(l.ID)
			b.ph.TrichLech = append(b.ph.TrichLech, TrichLech{Cau: so, BiDanh: a, Truong: Truong(l.Truong)})
		}
		if len(k.NhanNutKhongCo) > 0 {
			b.ph.NhanNut = append(b.ph.NhanNut, so)
		}
		if guard.DinhDang(moHinh) != guard.RaSach {
			b.ph.DinhDang = append(b.ph.DinhDang, so)
		}
	}
	return b
}

// themPhanTu adds the verifier's verdict to the findings.
func (b *ban) themPhanTu(p kiemchung.PhanTu) {
	for _, m := range p.MenhDe {
		// A sentence that states nothing (khong_thong_tin) was judged and
		// holds no claim; only an unsupported one is a finding.
		if m.Ket == kiemchung.KhongHoTro {
			b.ph.KhongHoTro = append(b.ph.KhongHoTro, m.So)
		}
	}
	b.ph.HuaHanhDong, b.ph.Tien = p.HuaHanhDongKhongCo, p.Tien
}
