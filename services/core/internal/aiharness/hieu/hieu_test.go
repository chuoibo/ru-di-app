package hieu

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

func vaoMau(bot obs.Bot) Vao {
	return Vao{
		Bot: bot,
		Cau: "tối mai đi đâu",
		Luc: time.Date(2026, 9, 25, 14, 5, 0, 0, time.FixedZone("ICT", 7*3600)),
		NganHan: []trinho.Luot{
			{Vai: trinho.TroLy, Chu: "…", BangChungIDs: []string{"p1", "p2"}},
			{Vai: trinho.Toi, Chu: "…"},
			{Vai: trinho.TroLy, Chu: "…", BangChungIDs: []string{"p2", "p3"}},
		},
		DanhSachDiemDen:   []DiemDen{{ID: "da-lat", Ten: "Đà Lạt"}, {ID: "sai-gon", Ten: "Sài Gòn"}},
		DanhSachThanhVien: []ThanhVien{{ID: "u1", Ten: "An"}, {ID: "u2", Ten: "Bình"}},
	}
}

// vaoDoi is a couple's turn: the group bot, two people, Doi set.
func vaoDoi() Vao {
	v := vaoMau(obs.BotNhom)
	v.Doi = true
	v.Cau = "tối mai hai đứa mình đi đâu"
	return v
}

// hopLe is a valid group router output; every refusal case below edits it
// in exactly one place.
const hopLe = `{"nhan_guard":"sach","tien":"none","y_dinh":["find_places","smalltalk"],"mo_ho_voi":["plan"],
 "huong":"truy_hoi_mot_buoc",
 "slots":{"diem_den_id":"da-lat","ngay_iso":"2026-09-26","khung_gio":{"tu":"19:00","den":"21:30"},
  "ngan_sach_vnd":150000,"di_ung":["tom"],"an_kieng":["chay"],"di_ung_ngoai_danh_muc":false,"so_nguoi":4,
  "nguoi_tham_gia":["u1"],"loai_cho":["cafe"],"khi_chat":["yen_tinh"],"tham_chieu":["p2"]},
 "can_truy_hoi":["places"],"truy_van":[{"nguon":"places","cau":"quán cà phê yên tĩnh ở Đà Lạt"}],
 "can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"cao"}`

func TestParseTuChoi(t *testing.T) {
	cases := []struct {
		name string
		f    func(string) error
		ok   []string
		bad  []string
	}{
		{"nhan_guard", func(s string) error { _, e := NhanGuards.Parse(s); return e },
			[]string{"sach", "chen_lenh", "ngoai_pham_vi", "nhay_cam"}, []string{"", "SACH", "clean", "injection"}},
		{"y_dinh_nep", func(s string) error { _, e := YDinhNep.Parse(s); return e },
			[]string{"app_help", "find_places", "explain_screen", "plan_help", "remember", "forget", "what_you_remember", "smalltalk"},
			[]string{"plan", "hoi", "chia_bill_draft", "remind", "chat_answer"}},
		{"y_dinh_nhom", func(s string) error { _, e := YDinhNhom.Parse(s); return e },
			[]string{"plan", "find_places", "hoi", "chia_bill_draft", "smalltalk"},
			[]string{"app_help", "remember", "explain_screen", "poll_draft"}},
		{"tien", func(s string) error { _, e := Tiens.Parse(s); return e },
			[]string{"none", "split_draft", "money_action"}, []string{"transfer", "None", "split"}},
		{"huong", func(s string) error { _, e := Huongs.Parse(s); return e },
			[]string{"tra_loi_thang", "truy_hoi_mot_buoc", "tac_tu", "hoi_lai"}, []string{"ngoai_pham_vi", "agent"}},
		{"tu_tin", func(s string) error { _, e := TuTins.Parse(s); return e },
			[]string{"cao", "vua", "thap"}, []string{"high", "trung_binh"}},
		{"nguon", func(s string) error { _, e := truyhoi.Nguons.Parse(s); return e },
			[]string{"places", "manual", "memory", "group_history"}, []string{"web", "chat"}},
	}
	for _, c := range cases {
		for _, s := range c.ok {
			if err := c.f(s); err != nil {
				t.Errorf("%s: %q refused: %v", c.name, s, err)
			}
		}
		for _, s := range c.bad {
			if err := c.f(s); !errors.Is(err, dong.ErrLa) {
				t.Errorf("%s: %q accepted", c.name, s)
			}
		}
	}
}

func TestDocHopLe(t *testing.T) {
	kq, err := Doc([]byte(hopLe), vaoMau(obs.BotNhom))
	if err != nil {
		t.Fatal(err)
	}
	if kq.NhanGuard != Sach || !reflect.DeepEqual(kq.YDinh, []YDinh{FindPlaces, Smalltalk}) || kq.Tien != TienNone ||
		!reflect.DeepEqual(kq.MoHoVoi, []YDinh{Plan}) || kq.Huong != TruyHoiMotBuoc ||
		kq.Slots.DiemDenID != "da-lat" || kq.Slots.NgayISO != "2026-09-26" || *kq.Slots.NganSachVND != 150000 ||
		*kq.Slots.SoNguoi != 4 || !reflect.DeepEqual(kq.Slots.NguoiThamGia, []string{"u1"}) ||
		!reflect.DeepEqual(kq.Slots.DiUng, []string{"tom"}) || !reflect.DeepEqual(kq.Slots.ThamChieu, []string{"p2"}) ||
		!reflect.DeepEqual(kq.CanTruyHoi, []truyhoi.Nguon{truyhoi.Places}) ||
		!reflect.DeepEqual(kq.TruyVan, []TruyVan{{Nguon: truyhoi.Places, Cau: "quán cà phê yên tĩnh ở Đà Lạt"}}) || kq.TuTin != Cao {
		t.Fatalf("%+v", kq)
	}
	// A direct answer: empty slots, no source, no query.
	min := `{"nhan_guard":"sach","tien":"none","y_dinh":["smalltalk"],"huong":"tra_loi_thang","slots":{},
	 "can_truy_hoi":[],"truy_van":[],"can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"vua"}`
	if kq, err := Doc([]byte(min), vaoMau(obs.BotNep)); err != nil || len(kq.CanTruyHoi) != 0 || kq.Slots.DiUng != nil || kq.MoHoVoi != nil {
		t.Fatalf("%+v %v", kq, err)
	}
	// An ask-back with options.
	hoi := `{"nhan_guard":"sach","tien":"none","y_dinh":["plan"],"huong":"hoi_lai","slots":{},
	 "can_truy_hoi":[],"truy_van":[],"can_hoi_lai":true,"cau_hoi_lai":"Đi hôm nào?","lua_chon_hoi_lai":["Thứ bảy","Chủ nhật"],
	 "tra_loi_cau_cho":false,"tu_tin":"thap"}`
	if kq, err := Doc([]byte(hoi), vaoMau(obs.BotNhom)); err != nil || !kq.CanHoiLai || len(kq.LuaChonHoiLai) != 2 {
		t.Fatalf("%+v %v", kq, err)
	}
}

// Each case mutates the valid output in one place; every one must refuse
// the whole result.
func TestDocTuChoi(t *testing.T) {
	cases := map[string]struct {
		bot      obs.Bot
		from, to string
	}{
		"unknown field":           {obs.BotNhom, `"tu_tin":"cao"`, `"tu_tin":"cao","ghi_chu":"x"`},
		"unknown guard":           {obs.BotNhom, `"sach"`, `"clean"`},
		"intent of other bot":     {obs.BotNep, `"find_places","smalltalk"`, `"find_places","plan"`},
		"repeated intent":         {obs.BotNhom, `"find_places","smalltalk"`, `"find_places","find_places"`},
		"four intents":            {obs.BotNhom, `"find_places","smalltalk"`, `"find_places","smalltalk","plan","hoi"`},
		"no intent":               {obs.BotNhom, `"find_places","smalltalk"`, ``},
		"close intent repeats":    {obs.BotNhom, `["plan"]`, `["smalltalk"]`},
		"three close intents":     {obs.BotNhom, `["plan"]`, `["plan","hoi","chia_bill_draft"]`},
		"unknown money":           {obs.BotNhom, `"tien":"none"`, `"tien":"transfer"`},
		"missing money":           {obs.BotNhom, `"tien":"none",`, ``},
		"unknown path":            {obs.BotNhom, `"truy_hoi_mot_buoc"`, `"agent"`},
		"one-step with no query":  {obs.BotNhom, `[{"nguon":"places","cau":"quán cà phê yên tĩnh ở Đà Lạt"}]`, `[]`},
		"direct with retrieval":   {obs.BotNhom, `"truy_hoi_mot_buoc"`, `"tra_loi_thang"`},
		"query on unnamed source": {obs.BotNhom, `{"nguon":"places","cau"`, `{"nguon":"manual","cau"`},
		"query on forbidden src":  {obs.BotNhom, `{"nguon":"places","cau"`, `{"nguon":"memory","cau"`},
		"empty query":             {obs.BotNhom, `"quán cà phê yên tĩnh ở Đà Lạt"`, `""`},
		"missing queries":         {obs.BotNhom, `"truy_van":[{"nguon":"places","cau":"quán cà phê yên tĩnh ở Đà Lạt"}],`, ``},
		"destination not given":   {obs.BotNhom, `"da-lat"`, `"ha-noi"`},
		"date not calendar":       {obs.BotNhom, `"2026-09-26"`, `"2026-02-30"`},
		"date not padded":         {obs.BotNhom, `"2026-09-26"`, `"2026-9-26"`},
		"date relative":           {obs.BotNhom, `"2026-09-26"`, `"mai"`},
		"hour malformed":          {obs.BotNhom, `"19:00"`, `"7pm"`},
		"empty window":            {obs.BotNhom, `"21:30"`, `"19:00"`},
		"negative budget":         {obs.BotNhom, `150000`, `-1`},
		"budget over ceiling":     {obs.BotNhom, `150000`, strconv.FormatInt(MaxNganSachVND+1, 10)},
		"fractional budget":       {obs.BotNhom, `150000`, `150000.5`},
		"party of zero":           {obs.BotNhom, `"so_nguoi":4`, `"so_nguoi":0`},
		"member not given":        {obs.BotNhom, `["u1"]`, `["u9"]`},
		"members for Nếp":         {obs.BotNep, `"find_places","smalltalk"`, `"find_places","smalltalk"`},
		"unknown allergen":        {obs.BotNhom, `["tom"]`, `["tom","gluten-free"]`},
		"repeated allergen":       {obs.BotNhom, `["tom"]`, `["tom","tom"]`},
		"unknown diet":            {obs.BotNhom, `["chay"]`, `["keto"]`},
		"free-text place kind":    {obs.BotNhom, `["cafe"]`, `["quán cà phê"]`},
		"reference not earlier":   {obs.BotNhom, `["p2"]`, `["p9"]`},
		"memory for the group":    {obs.BotNhom, `"can_truy_hoi":["places"]`, `"can_truy_hoi":["places","memory"]`},
		"ask-back text alone":     {obs.BotNhom, `"can_hoi_lai":false`, `"can_hoi_lai":false,"cau_hoi_lai":"Đi mấy người?"`},
		"ask-back without path":   {obs.BotNhom, `"can_hoi_lai":false`, `"can_hoi_lai":true,"cau_hoi_lai":"Đi mấy người?"`},
		"options without ask":     {obs.BotNhom, `"can_hoi_lai":false`, `"can_hoi_lai":false,"lua_chon_hoi_lai":["a"]`},
		"unknown confidence":      {obs.BotNhom, `"tu_tin":"cao"`, `"tu_tin":"high"`},
		"trailing data":           {obs.BotNhom, `"cao"}`, `"cao"}{}`},
	}
	for name, c := range cases {
		raw := hopLe
		if !strings.Contains(raw, c.from) {
			t.Fatalf("%s: %q not in the sample", name, c.from)
		}
		raw = strings.Replace(raw, c.from, c.to, 1)
		if name == "members for Nếp" {
			// Nếp has no member list: the same valid ids are refused.
			raw = strings.Replace(raw, `"mo_ho_voi":["plan"]`, `"mo_ho_voi":["plan_help"]`, 1)
		}
		if _, err := Doc([]byte(raw), vaoMau(c.bot)); !errors.Is(err, ErrCauTruc) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
	if _, err := Doc([]byte(hopLe), vaoMau("khach")); !errors.Is(err, ErrBot) {
		t.Errorf("unknown bot: %v", err)
	}
}

func props(s *genai.Schema) []string {
	var out []string
	for k := range s.Properties {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func tags(v any) []string {
	var out []string
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		out = append(out, strings.Split(rt.Field(i).Tag.Get("json"), ",")[0])
	}
	sort.Strings(out)
	return out
}

// The schema and the reader name the same fields.
func TestLuocDoKhopTho(t *testing.T) {
	s, err := LuocDo(vaoMau(obs.BotNhom))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		s    *genai.Schema
		v    any
	}{
		{"top level", s, tho{}},
		{"slots", s.Properties[pSlots], slotTho{}},
		{"khung_gio", s.Properties[pSlots].Properties[pKhungGio], khungGioTho{}},
		{"truy_van", s.Properties[pTruyVan].Items, truyVanTho{}},
	} {
		if got, want := props(c.s), tags(c.v); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: schema %v, reader %v", c.name, got, want)
		}
		for _, name := range append(append([]string{}, c.s.PropertyOrdering...), c.s.Required...) {
			if c.s.Properties[name] == nil {
				t.Errorf("%s: names %q, absent", c.name, name)
			}
		}
		if len(c.s.PropertyOrdering) != len(c.s.Properties) {
			t.Errorf("%s: ordering %d of %d", c.name, len(c.s.PropertyOrdering), len(c.s.Properties))
		}
	}
	if s.PropertyOrdering[0] != pNhanGuard || s.PropertyOrdering[1] != pTien {
		t.Errorf("safety and money must come first: %v", s.PropertyOrdering)
	}
}

func TestLuocDoTheoLuot(t *testing.T) {
	nhom, _ := LuocDo(vaoMau(obs.BotNhom))
	nep, _ := LuocDo(vaoMau(obs.BotNep))
	if got := nhom.Properties[pYDinh].Items.Enum; !reflect.DeepEqual(got, YDinhNhom.Values()) {
		t.Errorf("group intents %v", got)
	}
	if got := nep.Properties[pYDinh].Items.Enum; !reflect.DeepEqual(got, YDinhNep.Values()) {
		t.Errorf("Nếp intents %v", got)
	}
	if got := nep.Properties[pCanTruyHoi].Items.Enum; !reflect.DeepEqual(got, []string{"places", "manual", "memory"}) {
		t.Errorf("Nếp sources %v", got)
	}
	if got := nhom.Properties[pTruyVan].Items.Properties[pNguon].Enum; !reflect.DeepEqual(got, []string{"places", "manual", "group_history"}) {
		t.Errorf("group query sources %v", got)
	}
	if got := nhom.Properties[pSlots].Properties[pDiemDenID].Enum; !reflect.DeepEqual(got, []string{"da-lat", "sai-gon"}) {
		t.Errorf("destinations %v", got)
	}
	if got := nhom.Properties[pSlots].Properties[pThamChieu].Items.Enum; !reflect.DeepEqual(got, []string{"p1", "p2", "p3"}) {
		t.Errorf("references %v", got)
	}
	if got := nhom.Properties[pSlots].Properties[pNguoiThamGia].Items.Enum; !reflect.DeepEqual(got, []string{"u1", "u2"}) {
		t.Errorf("members %v", got)
	}
	if nep.Properties[pSlots].Properties[pNguoiThamGia] != nil {
		t.Error("Nếp's router offered group members")
	}
	// No list given: the slot is not offered at all.
	v := vaoMau(obs.BotNhom)
	v.DanhSachDiemDen, v.NganHan, v.DanhSachThanhVien = nil, nil, nil
	bare, _ := LuocDo(v)
	for _, p := range []string{pDiemDenID, pThamChieu, pNguoiThamGia} {
		if bare.Properties[pSlots].Properties[p] != nil {
			t.Errorf("an empty id list must drop %s, not offer an empty enum", p)
		}
	}
	// Byte-stable: the same turn gives the same schema.
	a, _ := json.Marshal(nhom)
	b, _ := LuocDo(vaoMau(obs.BotNhom))
	bb, _ := json.Marshal(b)
	if string(a) != string(bb) {
		t.Error("schema not stable")
	}
	if _, err := LuocDo(Vao{Bot: "khach"}); !errors.Is(err, ErrBot) {
		t.Error("unknown bot accepted")
	}
}
