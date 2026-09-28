package obs

// The router-path columns of a turn (metrics schema version 2): what the
// model decided, as closed labels, and what the engine did with it, as
// closed labels and counts. None of them can carry a word of the question
// or the answer. The values mirror hieu's and tools' closed sets; the tests
// of those packages hold them equal (obs cannot import them: they import
// obs).

// NhanGuard is the router's safety label; "" when, and only when, no
// router result exists (the money screen refused first, or the router
// failed). A nhay_cam turn is recorded as the clean turn it cannot be told
// from ("sach", with the canonical small-talk labels on the direct path the
// label forces; aiharness.ghiNhanRouter): the row names the invocation and
// the invocation names the person, so neither the label nor any column
// shaped by it may be stored (metrics schema v3, v4).
type NhanGuard string

// NhanGuards are the router's labels a record may hold: every label but
// nhay_cam.
var NhanGuards = []NhanGuard{"sach", "chen_lenh", "ngoai_pham_vi"}

func (v NhanGuard) Valid() bool { return v == "" || coTrong(NhanGuards, v) }

// YDinh is the first intent the router named; "" when none.
type YDinh string

// YDinhs are every intent of both bots.
var YDinhs = []YDinh{"find_places", "smalltalk", "app_help", "explain_screen", "plan_help", "remember", "forget",
	"what_you_remember", "plan", "hoi", "chia_bill_draft"}

func (v YDinh) Valid() bool { return v == "" || coTrong(YDinhs, v) }

// Tien is the router's money class; "" when no router result exists.
type Tien string

// Tiens are the router's money classes.
var Tiens = []Tien{"none", "split_draft", "money_action"}

func (v Tien) Valid() bool { return v == "" || coTrong(Tiens, v) }

// Huong is the path the router chose; "" when no router result exists.
type Huong string

// Huongs are the router's paths.
var Huongs = []Huong{"tra_loi_thang", "truy_hoi_mot_buoc", "tac_tu", "hoi_lai"}

func (v Huong) Valid() bool { return v == "" || coTrong(Huongs, v) }

// Duong is what the engine did after the router.
type Duong string

const (
	// DuongKhong: the turn ended before the router decided anything.
	DuongKhong Duong = ""
	// DuongTuChoiTien: the model classed a money action; fixed refusal.
	DuongTuChoiTien Duong = "tu_choi_tien"
	// DuongHoiLai: the router's one question back.
	DuongHoiLai Duong = "hoi_lai"
	// DuongThang: a direct answer, no tool.
	DuongThang Duong = "thang"
	// DuongNhanh: the fast path dispatched one tool (explain_screen).
	DuongNhanh Duong = "nhanh"
	// DuongTacTu: the bounded agent loop, the model choosing tools.
	DuongTacTu Duong = "tac_tu"
	// DuongTruyHoi: retrieval with the grader's corrective round and the
	// grounded, verified answer (crag + traloi).
	DuongTruyHoi Duong = "truy_hoi"
	// DuongNhapChiaBill: the group's split draft (metrics schema v5): one
	// structured reading of the shared messages, our template around it,
	// nothing written.
	DuongNhapChiaBill Duong = "nhap_chia_bill"
)

// Duongs are the engine's paths, "" excluded.
var Duongs = []Duong{DuongTuChoiTien, DuongHoiLai, DuongThang, DuongNhanh, DuongTacTu, DuongTruyHoi, DuongNhapChiaBill}

func (v Duong) Valid() bool { return v == DuongKhong || coTrong(Duongs, v) }

// KetKiem is the verifier's verdict on what was released or withheld.
type KetKiem string

const (
	// KiemKhongChay: no verifier call ran (a refusal, a question back, a
	// fixed sentence, or a turn that failed before it).
	KiemKhongChay KetKiem = "khong_chay"
	// KiemDat: the verifier passed the released answer.
	KiemDat KetKiem = "dat"
	// KiemKhongDat: the verifier found an unsupported sentence, a claimed
	// action or money; what it judged was not released.
	KiemKhongDat KetKiem = "khong_dat"
	// KiemHong: the verifier's output was refused; nothing was released.
	KiemHong KetKiem = "hong"
)

// KetKiems are the verdicts.
var KetKiems = []KetKiem{KiemKhongChay, KiemDat, KiemKhongDat, KiemHong}

func (v KetKiem) Valid() bool { return coTrong(KetKiems, v) }

// CongCu is one tool name of the registry.
type CongCu string

// CongCus are the registry's tool names (tools.Tens, held equal by a test
// in tools).
var CongCus = []CongCu{"search_places", "get_place", "list_destinations", "nearest_area", "group_snapshot",
	"list_group_outings", "search_app_manual", "explain_screen", "propose_places", "propose_itinerary", "draft_poll",
	"suggest_screen", "my_upcoming_outings", "recall_memory", "remember_fact", "forget_fact", "what_you_remember",
	"set_reminder"}

func (v CongCu) Valid() bool { return coTrong(CongCus, v) }

// CacCongCu are the tools a turn ran, each once, in registry order.
type CacCongCu []CongCu

// Valid holds every name to the registry, once each.
func (c CacCongCu) Valid() bool {
	seen := map[CongCu]bool{}
	for _, v := range c {
		if !v.Valid() || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func coTrong[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
