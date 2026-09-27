package tools

import (
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/trinho"
)

// Argument bounds.
const (
	// MaxKTool bounds a tool's k.
	MaxKTool = 20
	// MaxTruyVan is a model-written query's ceiling in runes.
	MaxTruyVan = 200
	// MaxDeXuat is how many places propose_places takes.
	MaxDeXuat = 5
	// MaxChang is how many stops propose_itinerary takes.
	MaxChang = 6
	// MaxLuaChon is how many options a poll draft takes.
	MaxLuaChon = 6
)

// PhanLoaiSuThat is the model's classification of a fact it is asked to
// remember. Go refuses to store anything but CaNhan; the MODEL decides the
// class, Go only enforces it (design 05 §6: never money, other people,
// health or sensitive traits).
type PhanLoaiSuThat string

const (
	CaNhan    PhanLoaiSuThat = "ca_nhan"
	VeTien    PhanLoaiSuThat = "tien"
	NguoiKhac PhanLoaiSuThat = "nguoi_khac"
	SucKhoe   PhanLoaiSuThat = "suc_khoe"
	NhayCam   PhanLoaiSuThat = "nhay_cam"
)

// PhanLoaiSuThats is the closed set of PhanLoaiSuThat.
var PhanLoaiSuThats = dong.Moi("phan_loai_su_that", CaNhan, VeTien, NguoiKhac, SucKhoe, NhayCam)

func i64(n int64) *int64     { return &n }
func f64(n float64) *float64 { return &n }

func chuoi(max int64, mo string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, MaxLength: i64(max), Description: mo}
}
func soK() *genai.Schema {
	return &genai.Schema{Type: genai.TypeInteger, Minimum: f64(1), Maximum: f64(MaxKTool)}
}
func danhSach(item *genai.Schema, min, max int64) *genai.Schema {
	return &genai.Schema{Type: genai.TypeArray, Items: item, MinItems: i64(min), MaxItems: i64(max)}
}
func enum(v []string) *genai.Schema { return &genai.Schema{Type: genai.TypeString, Enum: v} }
func ngay() *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, Format: "date", Pattern: `^\d{4}-\d{2}-\d{2}$`}
}
func gio() *genai.Schema { return &genai.Schema{Type: genai.TypeString, Pattern: `^\d{2}:\d{2}$`} }
func doiTuong(props map[string]*genai.Schema, order []string, required ...string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeObject, Properties: props, PropertyOrdering: order, Required: required}
}

// An id the model copies from evidence or from a closed list. Membership is
// checked by Go against the turn's ledger or list, not by the schema.
func id() *genai.Schema { return &genai.Schema{Type: genai.TypeString, MaxLength: i64(128)} }

func thamSoTimQuan() *genai.Schema {
	return doiTuong(map[string]*genai.Schema{
		"truy_van":      chuoi(MaxTruyVan, "what to look for, in your words"),
		"diem_den_id":   id(),
		"di_ung":        danhSach(enum(hieu.DiUngs.Values()), 0, hieu.MaxMucSlot),
		"an_kieng":      danhSach(enum(hieu.AnKiengs.Values()), 0, hieu.MaxMucSlot),
		"ngay_iso":      ngay(),
		"gio":           gio(),
		"ngan_sach_vnd": {Type: genai.TypeInteger, Minimum: f64(0), Maximum: f64(float64(hieu.MaxNganSachVND))},
		"loai_cho":      danhSach(enum(hieu.LoaiChos.Values()), 0, hieu.MaxMucSlot),
		"khi_chat":      danhSach(enum(hieu.KhiChats.Values()), 0, hieu.MaxMucSlot),
		"khu_vuc":       id(),
		"k":             soK(),
	}, []string{"truy_van", "diem_den_id", "di_ung", "an_kieng", "ngay_iso", "gio", "ngan_sach_vnd", "loai_cho", "khi_chat", "khu_vuc", "k"},
		"truy_van")
}

var thamSo = map[Ten]*genai.Schema{
	SearchPlaces:     thamSoTimQuan(),
	GetPlace:         doiTuong(map[string]*genai.Schema{"id": id()}, []string{"id"}, "id"),
	ListDestinations: doiTuong(map[string]*genai.Schema{}, nil),
	NearestArea: doiTuong(map[string]*genai.Schema{
		"diem_den_id": id(),
		"mo_ta":       chuoi(MaxTruyVan, "the landmark or area as the person described it"),
	}, []string{"diem_den_id", "mo_ta"}, "diem_den_id", "mo_ta"),
	SearchAppManual: doiTuong(map[string]*genai.Schema{
		"truy_van": chuoi(MaxTruyVan, "what the person wants to do in the app"),
		"k":        soK(),
	}, []string{"truy_van", "k"}, "truy_van"),
	ExplainScreen: doiTuong(map[string]*genai.Schema{}, nil),
	SuggestScreen: doiTuong(map[string]*genai.Schema{"man": id()}, []string{"man"}, "man"),
	ProposePlaces: doiTuong(map[string]*genai.Schema{
		"ids": danhSach(id(), 1, MaxDeXuat),
	}, []string{"ids"}, "ids"),
	ProposeItinerary: doiTuong(map[string]*genai.Schema{
		"ngay_iso": ngay(),
		"chang": danhSach(doiTuong(map[string]*genai.Schema{
			"id":  id(),
			"gio": gio(),
		}, []string{"id", "gio"}, "id"), 1, MaxChang),
	}, []string{"ngay_iso", "chang"}, "chang"),
	DraftPoll: doiTuong(map[string]*genai.Schema{
		"cau_hoi":  chuoi(120, "the poll question"),
		"lua_chon": danhSach(chuoi(60, ""), 2, MaxLuaChon),
	}, []string{"cau_hoi", "lua_chon"}, "cau_hoi", "lua_chon"),
	GroupSnapshot: doiTuong(map[string]*genai.Schema{}, nil),
	ListGroupOutings: doiTuong(map[string]*genai.Schema{
		"khi": enum([]string{"sap_toi", "da_qua"}),
		"k":   soK(),
	}, []string{"khi", "k"}, "khi"),
	MyUpcomingOutings: doiTuong(map[string]*genai.Schema{"k": soK()}, []string{"k"}),
	RecallMemory: doiTuong(map[string]*genai.Schema{
		"truy_van": chuoi(MaxTruyVan, "what to recall"),
		"k":        soK(),
	}, []string{"truy_van", "k"}, "truy_van"),
	RememberFact: doiTuong(map[string]*genai.Schema{
		"noi_dung":  chuoi(trinho.MaxNoiDung, "the fact: a span of the person's message, copied word for word"),
		"loai":      enum(trinho.LoaiSuThats.Values()),
		"phan_loai": enum(PhanLoaiSuThats.Values()),
		"tu_ngay":   ngay(),
		"den_ngay":  ngay(),
	}, []string{"noi_dung", "loai", "phan_loai", "tu_ngay", "den_ngay"}, "noi_dung", "loai", "phan_loai"),
	ForgetFact: doiTuong(map[string]*genai.Schema{
		"id":    id(),
		"mo_ta": chuoi(MaxTruyVan, "the person's description of what to forget"),
	}, []string{"id", "mo_ta"}),
	WhatYouRemember: doiTuong(map[string]*genai.Schema{}, nil),
	SetReminder: doiTuong(map[string]*genai.Schema{
		"outing_id": id(),
		"ngay_iso":  ngay(),
		"gio":       gio(),
	}, []string{"outing_id", "ngay_iso", "gio"}, "outing_id", "ngay_iso"),
}

// ThamSo is t's argument schema.
func ThamSo(t Ten) (*genai.Schema, bool) {
	s, ok := thamSo[t]
	return s, ok
}

// KhaiBao is t's function declaration: name, the one-line purpose, and the
// argument schema.
func KhaiBao(t Ten) (*genai.FunctionDeclaration, bool) {
	m, ok := Tra(t)
	if !ok {
		return nil, false
	}
	return &genai.FunctionDeclaration{Name: string(t), Description: m.MoTa, Parameters: thamSo[t]}, true
}
