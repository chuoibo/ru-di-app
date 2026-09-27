package hieu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// Bounds of the structured output.
const (
	// MaxCauHoiLai is the ask-back question's ceiling in runes.
	MaxCauHoiLai = 300
	// MaxLuaChon is how many options an ask-back offers; MaxChuLuaChon is
	// one option's ceiling in runes.
	MaxLuaChon    = 3
	MaxChuLuaChon = 60
	// MaxMoHo is how many close intents MoHoVoi names.
	MaxMoHo = 2
	// MaxTruyVan is how many queries the router writes; MaxChuTruyVan is
	// one query's ceiling in runes.
	MaxTruyVan    = 3
	MaxChuTruyVan = 200
	// MaxNganSachVND is the largest per-person budget accepted, in đồng.
	MaxNganSachVND int64 = 100_000_000
	// MaxSoNguoi is the largest party size accepted.
	MaxSoNguoi = 50
	// MaxMucSlot bounds each list slot.
	MaxMucSlot = 10
)

// NguonCua is the closed set of retrieval sources bot's router may ask for:
// memory is Nếp's only, group history the group's only.
func NguonCua(bot obs.Bot) (dong.Tap[truyhoi.Nguon], bool) {
	switch bot {
	case obs.BotNep:
		return nguonNep, true
	case obs.BotNhom:
		return nguonNhom, true
	}
	return dong.Tap[truyhoi.Nguon]{}, false
}

var (
	nguonNep  = dong.Moi("nguon_nep", truyhoi.Places, truyhoi.Manual, truyhoi.Memory)
	nguonNhom = dong.Moi("nguon_nhom", truyhoi.Places, truyhoi.Manual, truyhoi.GroupHistory)
)

// The JSON property names. The schema and tho's tags must agree;
// TestLuocDoKhopTho checks it.
const (
	pNhanGuard     = "nhan_guard"
	pTien          = "tien"
	pYDinh         = "y_dinh"
	pMoHoVoi       = "mo_ho_voi"
	pHuong         = "huong"
	pSlots         = "slots"
	pCanTruyHoi    = "can_truy_hoi"
	pTruyVan       = "truy_van"
	pNguon         = "nguon"
	pCau           = "cau"
	pCauCoDau      = "cau_co_dau"
	pCanHoiLai     = "can_hoi_lai"
	pCauHoiLai     = "cau_hoi_lai"
	pLuaChonHoiLai = "lua_chon_hoi_lai"
	pTraLoiCauCho  = "tra_loi_cau_cho"
	pTuTin         = "tu_tin"

	pDiemDenID         = "diem_den_id"
	pNgayISO           = "ngay_iso"
	pKhungGio          = "khung_gio"
	pTu                = "tu"
	pDen               = "den"
	pNganSachVND       = "ngan_sach_vnd"
	pDiUng             = "di_ung"
	pAnKieng           = "an_kieng"
	pDiUngNgoaiDanhMuc = "di_ung_ngoai_danh_muc"
	pSoNguoi           = "so_nguoi"
	pNguoiThamGia      = "nguoi_tham_gia"
	pLoaiCho           = "loai_cho"
	pKhiChat           = "khi_chat"
	pThamChieu         = "tham_chieu"
)

func i64(n int64) *int64     { return &n }
func f64(n float64) *float64 { return &n }
func enumStr(v []string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, Enum: v}
}
func enumList(v []string, max int64) *genai.Schema {
	return &genai.Schema{Type: genai.TypeArray, Items: enumStr(v), MaxItems: i64(max)}
}

// thamChieuHopLe is the evidence ids of the session's earlier turns, in
// order of first appearance.
func thamChieuHopLe(v Vao) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range v.NganHan {
		for _, id := range l.BangChungIDs {
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

func diemDenHopLe(v Vao) []string {
	out := make([]string, 0, len(v.DanhSachDiemDen))
	for _, d := range v.DanhSachDiemDen {
		out = append(out, d.ID)
	}
	return out
}

// thanhVienHopLe is the member ids, offered to the group's router only.
func thanhVienHopLe(v Vao) []string {
	if v.Bot != obs.BotNhom {
		return nil
	}
	out := make([]string, 0, len(v.DanhSachThanhVien))
	for _, m := range v.DanhSachThanhVien {
		out = append(out, m.ID)
	}
	return out
}

// ErrBot: a bot outside obs.Bot.
var ErrBot = errors.New("hieu: unknown bot")

// LuocDo is the response schema of the router call for this turn: the one
// source of truth for the structured output (GenerateContentConfig.
// ResponseSchema, or an ADK llmagent's OutputSchema). It depends on the turn
// only through the closed id lists it offers (destinations, members, earlier
// evidence) and the bot's intent and source sets; for the same Vao it is the
// same value, byte for byte once marshalled. Safety, money and the path come
// first in the ordering, so the model settles policy before it writes
// queries.
func LuocDo(v Vao) (*genai.Schema, error) {
	yd, ok := YDinhCua(v.Bot)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrBot, v.Bot)
	}
	ng, _ := NguonCua(v.Bot)

	slotProps := map[string]*genai.Schema{}
	var slotOrder []string
	them := func(name string, s *genai.Schema) {
		slotProps[name] = s
		slotOrder = append(slotOrder, name)
	}
	if ids := diemDenHopLe(v); len(ids) > 0 {
		them(pDiemDenID, enumStr(ids))
	}
	them(pNgayISO, &genai.Schema{Type: genai.TypeString, Format: "date", Pattern: `^\d{4}-\d{2}-\d{2}$`,
		Description: "YYYY-MM-DD, resolved from the «now» line of the prompt"})
	them(pKhungGio, &genai.Schema{Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			pTu:  {Type: genai.TypeString, Pattern: `^\d{2}:\d{2}$`},
			pDen: {Type: genai.TypeString, Pattern: `^\d{2}:\d{2}$`},
		},
		PropertyOrdering: []string{pTu, pDen},
		Required:         []string{pTu},
	})
	them(pNganSachVND, &genai.Schema{Type: genai.TypeInteger, Minimum: f64(0), Maximum: f64(float64(MaxNganSachVND)),
		Description: "per person, whole đồng"})
	them(pDiUng, enumList(DiUngs.Values(), MaxMucSlot))
	them(pAnKieng, enumList(AnKiengs.Values(), MaxMucSlot))
	them(pDiUngNgoaiDanhMuc, &genai.Schema{Type: genai.TypeBoolean,
		Description: "true when the person names an allergen that is not in the di_ung list"})
	them(pSoNguoi, &genai.Schema{Type: genai.TypeInteger, Minimum: f64(1), Maximum: f64(MaxSoNguoi)})
	if ids := thanhVienHopLe(v); len(ids) > 0 {
		them(pNguoiThamGia, enumList(ids, int64(len(ids))))
	}
	them(pLoaiCho, enumList(LoaiChos.Values(), MaxMucSlot))
	them(pKhiChat, enumList(KhiChats.Values(), MaxMucSlot))
	if ids := thamChieuHopLe(v); len(ids) > 0 {
		them(pThamChieu, enumList(ids, MaxMucSlot))
	}

	truyVan := &genai.Schema{Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			pNguon: enumStr(ng.Values()),
			pCau: {Type: genai.TypeString, MaxLength: i64(MaxChuTruyVan),
				Description: "the query standing on its own: references to earlier turns resolved, in the person's own spelling"},
			pCauCoDau: {Type: genai.TypeString, MaxLength: i64(MaxChuTruyVan),
				Description: "the same query with Vietnamese diacritics restored and teencode spelled out"},
		},
		PropertyOrdering: []string{pNguon, pCau, pCauCoDau},
		Required:         []string{pNguon, pCau},
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			pNhanGuard: enumStr(NhanGuards.Values()),
			pTien:      enumStr(Tiens.Values()),
			pYDinh: {Type: genai.TypeArray, Items: enumStr(yd.Values()),
				MinItems: i64(1), MaxItems: i64(MaxYDinh)},
			pMoHoVoi: enumList(yd.Values(), MaxMoHo),
			pHuong:   enumStr(Huongs.Values()),
			pSlots: {Type: genai.TypeObject, Properties: slotProps,
				PropertyOrdering: slotOrder},
			pCanTruyHoi:    enumList(ng.Values(), int64(ng.Len())),
			pTruyVan:       {Type: genai.TypeArray, Items: truyVan, MaxItems: i64(MaxTruyVan)},
			pCanHoiLai:     {Type: genai.TypeBoolean},
			pCauHoiLai:     {Type: genai.TypeString, MaxLength: i64(MaxCauHoiLai)},
			pLuaChonHoiLai: {Type: genai.TypeArray, MaxItems: i64(MaxLuaChon), Items: &genai.Schema{Type: genai.TypeString, MaxLength: i64(MaxChuLuaChon)}},
			pTraLoiCauCho:  {Type: genai.TypeBoolean},
			pTuTin:         enumStr(TuTins.Values()),
		},
		PropertyOrdering: []string{pNhanGuard, pTien, pYDinh, pMoHoVoi, pHuong, pSlots, pCanTruyHoi, pTruyVan,
			pCanHoiLai, pCauHoiLai, pLuaChonHoiLai, pTraLoiCauCho, pTuTin},
		Required: []string{pNhanGuard, pTien, pYDinh, pHuong, pSlots, pCanTruyHoi, pTruyVan,
			pCanHoiLai, pTraLoiCauCho, pTuTin},
	}, nil
}

// tho is the wire form. Pointers mark required fields so a missing one is
// told apart from a zero value.
type tho struct {
	NhanGuard     *string       `json:"nhan_guard"`
	Tien          *string       `json:"tien"`
	YDinh         []string      `json:"y_dinh"`
	MoHoVoi       []string      `json:"mo_ho_voi"`
	Huong         *string       `json:"huong"`
	Slots         *slotTho      `json:"slots"`
	CanTruyHoi    *[]string     `json:"can_truy_hoi"`
	TruyVan       *[]truyVanTho `json:"truy_van"`
	CanHoiLai     *bool         `json:"can_hoi_lai"`
	CauHoiLai     string        `json:"cau_hoi_lai"`
	LuaChonHoiLai []string      `json:"lua_chon_hoi_lai"`
	TraLoiCauCho  *bool         `json:"tra_loi_cau_cho"`
	TuTin         *string       `json:"tu_tin"`
}

type truyVanTho struct {
	Nguon    *string `json:"nguon"`
	Cau      *string `json:"cau"`
	CauCoDau *string `json:"cau_co_dau"`
}

type slotTho struct {
	DiemDenID         string       `json:"diem_den_id"`
	NgayISO           string       `json:"ngay_iso"`
	KhungGio          *khungGioTho `json:"khung_gio"`
	NganSachVND       *int64       `json:"ngan_sach_vnd"`
	DiUng             []string     `json:"di_ung"`
	AnKieng           []string     `json:"an_kieng"`
	DiUngNgoaiDanhMuc bool         `json:"di_ung_ngoai_danh_muc"`
	SoNguoi           *int         `json:"so_nguoi"`
	NguoiThamGia      []string     `json:"nguoi_tham_gia"`
	LoaiCho           []string     `json:"loai_cho"`
	KhiChat           []string     `json:"khi_chat"`
	ThamChieu         []string     `json:"tham_chieu"`
}

type khungGioTho struct {
	Tu  string `json:"tu"`
	Den string `json:"den"`
}

// ErrCauTruc is what every structural refusal of Doc wraps.
var ErrCauTruc = errors.New("hieu: router output refused")

func loi(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrCauTruc}, a...)...)
}

// Doc reads the model's JSON for turn v. It is strict: unknown fields,
// unknown enum values, ids outside the turn's lists, a malformed date or
// time, a negative or oversized budget, too many or repeated intents, a
// source the bot may not use, or fields that contradict each other (a path
// and its queries, an ask-back and its text) all refuse the WHOLE result. A
// hard constraint is never silently dropped: a caller that cannot trust the
// allergy slot must not search as if there were none. These are checks of
// structure and consistency between the model's own fields; none reads the
// message.
func Doc(raw []byte, v Vao) (KetQua, error) {
	yd, ok := YDinhCua(v.Bot)
	if !ok {
		return KetQua{}, fmt.Errorf("%w: %q", ErrBot, v.Bot)
	}
	ng, _ := NguonCua(v.Bot)

	var t tho
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return KetQua{}, loi("json: %v", err)
	}
	if dec.More() {
		return KetQua{}, loi("trailing data")
	}
	if t.NhanGuard == nil || t.Tien == nil || t.YDinh == nil || t.Huong == nil || t.Slots == nil ||
		t.CanTruyHoi == nil || t.TruyVan == nil || t.CanHoiLai == nil || t.TraLoiCauCho == nil || t.TuTin == nil {
		return KetQua{}, loi("a required field is missing")
	}

	var kq KetQua
	var err error
	if kq.NhanGuard, err = NhanGuards.Parse(*t.NhanGuard); err != nil {
		return KetQua{}, loi("%v", err)
	}
	if kq.Tien, err = Tiens.Parse(*t.Tien); err != nil {
		return KetQua{}, loi("%v", err)
	}
	if len(t.YDinh) < 1 || len(t.YDinh) > MaxYDinh {
		return KetQua{}, loi("%d intents", len(t.YDinh))
	}
	if kq.YDinh, err = yd.ParseAll(t.YDinh); err != nil {
		return KetQua{}, loi("%v", err)
	}
	if len(t.MoHoVoi) > MaxMoHo {
		return KetQua{}, loi("%d close intents", len(t.MoHoVoi))
	}
	if kq.MoHoVoi, err = yd.ParseAll(t.MoHoVoi); err != nil {
		return KetQua{}, loi("%v", err)
	}
	for _, m := range kq.MoHoVoi {
		for _, y := range kq.YDinh {
			if m == y {
				return KetQua{}, loi("mo_ho_voi repeats the intent %q", m)
			}
		}
	}
	if len(kq.MoHoVoi) == 0 {
		kq.MoHoVoi = nil
	}
	if kq.Huong, err = Huongs.Parse(*t.Huong); err != nil {
		return KetQua{}, loi("%v", err)
	}
	if kq.CanTruyHoi, err = ng.ParseAll(*t.CanTruyHoi); err != nil {
		return KetQua{}, loi("%v", err)
	}
	if kq.TruyVan, err = docTruyVan(*t.TruyVan, ng, kq.CanTruyHoi); err != nil {
		return KetQua{}, err
	}
	switch kq.Huong {
	case TraLoiThang:
		if len(kq.CanTruyHoi) > 0 || len(kq.TruyVan) > 0 {
			return KetQua{}, loi("a direct answer with retrieval")
		}
	case TruyHoiMotBuoc:
		if len(kq.TruyVan) == 0 {
			return KetQua{}, loi("a one-step retrieval with no query")
		}
	}
	kq.CanHoiLai = *t.CanHoiLai
	if kq.CanHoiLai != (kq.Huong == HoiLai) {
		return KetQua{}, loi("can_hoi_lai must equal huong == hoi_lai")
	}
	if kq.CanHoiLai != (t.CauHoiLai != "") || (!kq.CanHoiLai && len(t.LuaChonHoiLai) > 0) {
		return KetQua{}, loi("cau_hoi_lai and its options must be set iff can_hoi_lai")
	}
	if utf8.RuneCountInString(t.CauHoiLai) > MaxCauHoiLai || !utf8.ValidString(t.CauHoiLai) {
		return KetQua{}, loi("cau_hoi_lai too long")
	}
	kq.CauHoiLai = t.CauHoiLai
	if len(t.LuaChonHoiLai) > MaxLuaChon {
		return KetQua{}, loi("%d ask-back options", len(t.LuaChonHoiLai))
	}
	for _, c := range t.LuaChonHoiLai {
		if n := utf8.RuneCountInString(c); n == 0 || n > MaxChuLuaChon || !utf8.ValidString(c) {
			return KetQua{}, loi("an ask-back option is empty or too long")
		}
	}
	if len(t.LuaChonHoiLai) > 0 {
		kq.LuaChonHoiLai = append([]string(nil), t.LuaChonHoiLai...)
	}
	kq.TraLoiCauCho = *t.TraLoiCauCho
	if kq.TuTin, err = TuTins.Parse(*t.TuTin); err != nil {
		return KetQua{}, loi("%v", err)
	}
	if kq.Slots, err = docSlots(*t.Slots, v); err != nil {
		return KetQua{}, err
	}
	return kq, nil
}

func docTruyVan(ts []truyVanTho, ng dong.Tap[truyhoi.Nguon], can []truyhoi.Nguon) ([]TruyVan, error) {
	if len(ts) > MaxTruyVan {
		return nil, loi("%d queries", len(ts))
	}
	coCan := map[truyhoi.Nguon]bool{}
	for _, n := range can {
		coCan[n] = true
	}
	var out []TruyVan
	for _, q := range ts {
		if q.Nguon == nil || q.Cau == nil {
			return nil, loi("a query field is missing")
		}
		n, err := ng.Parse(*q.Nguon)
		if err != nil {
			return nil, loi("%v", err)
		}
		if !coCan[n] {
			return nil, loi("a query on %q, which can_truy_hoi does not name", n)
		}
		if c := utf8.RuneCountInString(*q.Cau); c == 0 || c > MaxChuTruyVan || !utf8.ValidString(*q.Cau) {
			return nil, loi("a query is empty or too long")
		}
		tv := TruyVan{Nguon: n, Cau: *q.Cau}
		if q.CauCoDau != nil {
			// Optional, but never empty when written: a blank second form
			// is a malformed field, not «the same».
			if c := utf8.RuneCountInString(*q.CauCoDau); c == 0 || c > MaxChuTruyVan || !utf8.ValidString(*q.CauCoDau) {
				return nil, loi("a query's diacritics form is empty or too long")
			}
			if *q.CauCoDau != *q.Cau {
				tv.CauCoDau = *q.CauCoDau
			}
		}
		out = append(out, tv)
	}
	return out, nil
}

func trongDanhSach(ids []string, danhSach []string, ten string) error {
	co := make(map[string]bool, len(danhSach))
	for _, id := range danhSach {
		co[id] = true
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !co[id] || seen[id] {
			return loi("%s %q is not in the turn's list or repeats", ten, id)
		}
		seen[id] = true
	}
	return nil
}

func docList[T ~string](tap dong.Tap[T], ss []string) ([]string, error) {
	if len(ss) > MaxMucSlot {
		return nil, loi("%s has %d items", tap.Ten(), len(ss))
	}
	if _, err := tap.ParseAll(ss); err != nil {
		return nil, loi("%v", err)
	}
	if len(ss) == 0 {
		return nil, nil
	}
	return append([]string(nil), ss...), nil
}

// NgayHopLe reports whether s is a calendar date in the exact form
// YYYY-MM-DD.
func NgayHopLe(s string) bool {
	d, err := time.Parse(time.DateOnly, s)
	return err == nil && d.Format(time.DateOnly) == s
}

// GioHopLe reports whether s is a clock time in the exact form HH:MM.
func GioHopLe(s string) bool {
	d, err := time.Parse("15:04", s)
	return err == nil && d.Format("15:04") == s
}

func docSlots(s slotTho, v Vao) (Slots, error) {
	var out Slots
	if s.DiemDenID != "" {
		if err := trongDanhSach([]string{s.DiemDenID}, diemDenHopLe(v), pDiemDenID); err != nil {
			return Slots{}, err
		}
		out.DiemDenID = s.DiemDenID
	}
	if s.NgayISO != "" {
		if !NgayHopLe(s.NgayISO) {
			return Slots{}, loi("ngay_iso %q", s.NgayISO)
		}
		out.NgayISO = s.NgayISO
	}
	if s.KhungGio != nil {
		k := *s.KhungGio
		if !GioHopLe(k.Tu) || (k.Den != "" && (!GioHopLe(k.Den) || k.Den == k.Tu)) {
			return Slots{}, loi("khung_gio %q-%q", k.Tu, k.Den)
		}
		out.KhungGio = &KhungGio{Tu: k.Tu, Den: k.Den}
	}
	if s.NganSachVND != nil {
		n := *s.NganSachVND
		if n < 0 || n > MaxNganSachVND {
			return Slots{}, loi("ngan_sach_vnd %d", n)
		}
		out.NganSachVND = &n
	}
	var err error
	if out.DiUng, err = docList(DiUngs, s.DiUng); err != nil {
		return Slots{}, err
	}
	if out.AnKieng, err = docList(AnKiengs, s.AnKieng); err != nil {
		return Slots{}, err
	}
	out.DiUngNgoaiDanhMuc = s.DiUngNgoaiDanhMuc
	if s.SoNguoi != nil {
		n := *s.SoNguoi
		if n < 1 || n > MaxSoNguoi {
			return Slots{}, loi("so_nguoi %d", n)
		}
		out.SoNguoi = &n
	}
	if err := trongDanhSach(s.NguoiThamGia, thanhVienHopLe(v), pNguoiThamGia); err != nil {
		return Slots{}, err
	}
	if len(s.NguoiThamGia) > 0 {
		out.NguoiThamGia = append([]string(nil), s.NguoiThamGia...)
	}
	if out.LoaiCho, err = docList(LoaiChos, s.LoaiCho); err != nil {
		return Slots{}, err
	}
	if out.KhiChat, err = docList(KhiChats, s.KhiChat); err != nil {
		return Slots{}, err
	}
	if len(s.ThamChieu) > MaxMucSlot {
		return Slots{}, loi("tham_chieu has %d items", len(s.ThamChieu))
	}
	if err := trongDanhSach(s.ThamChieu, thamChieuHopLe(v), pThamChieu); err != nil {
		return Slots{}, err
	}
	if len(s.ThamChieu) > 0 {
		out.ThamChieu = append([]string(nil), s.ThamChieu...)
	}
	return out, nil
}
