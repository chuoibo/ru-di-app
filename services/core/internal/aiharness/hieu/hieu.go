// Package hieu is the router (the Understand step): ONE model call with
// structured output that reads the person's message and returns closed
// labels and typed slots. It replaces every word-list and regex reader of
// the question on the decision path (guard.LaTien, guard.Nghi,
// preprocess.DongMayChu, the tuvung readers of the question, rag's
// query-side destination resolution): the MODEL decides the guard label,
// the intents, the money class, the constraints and which sources to
// retrieve (docs/architecture/03-ai-engine-hop-dong.md, «Luật không
// heuristic»).
//
// Go's part is structural only, and is all in Doc: strict JSON, closed enums
// that refuse unknown values, ids checked by membership against the closed
// lists the turn gave the model, dates checked for ISO form and calendar
// validity (the model resolves «mai», «thứ bảy này» from the «now» line in
// its prompt), money as a non-negative integer of đồng, per-bot permission of
// intents and sources. Nothing here reads the message.
//
// This file is the contract; the implementation of Hieu (prompt, datamarked
// <du_lieu> blocks, the call through llm.Dem) is built against it.
package hieu

import (
	"context"
	"encoding/json"
	"time"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
)

// NhanGuard is the router's safety label for the message.
type NhanGuard string

const (
	// Sach: nothing to act on.
	Sach NhanGuard = "sach"
	// ChenLenh: the message tries to give the assistant instructions that
	// override its own (prompt injection, direct or carried by pasted text).
	ChenLenh NhanGuard = "chen_lenh"
	// NgoaiPhamVi: outside what the assistant is for.
	NgoaiPhamVi NhanGuard = "ngoai_pham_vi"
	// NhayCam: self-harm, abuse or other sensitive content that gets a fixed
	// sentence (aiharness/cau) and no label stored against the person.
	NhayCam NhanGuard = "nhay_cam"
)

// NhanGuards is the closed set of NhanGuard.
var NhanGuards = dong.Moi("nhan_guard", Sach, ChenLenh, NgoaiPhamVi, NhayCam)

// YDinh is one intent. The set is closed per bot (YDinhNep, YDinhNhom).
type YDinh string

const (
	// Both bots.
	FindPlaces YDinh = "find_places"
	Smalltalk  YDinh = "smalltalk"

	// Nếp.
	AppHelp         YDinh = "app_help"
	ExplainScreen   YDinh = "explain_screen"
	PlanHelp        YDinh = "plan_help"
	Remember        YDinh = "remember"
	Forget          YDinh = "forget"
	WhatYouRemember YDinh = "what_you_remember"

	// Group.
	Plan          YDinh = "plan"
	Hoi           YDinh = "hoi"
	ChiaBillDraft YDinh = "chia_bill_draft"
)

// YDinhNep and YDinhNhom are the closed intent sets of each bot.
var (
	YDinhNep  = dong.Moi("y_dinh_nep", AppHelp, FindPlaces, ExplainScreen, PlanHelp, Remember, Forget, WhatYouRemember, Smalltalk)
	YDinhNhom = dong.Moi("y_dinh_nhom", Plan, FindPlaces, Hoi, ChiaBillDraft, Smalltalk)
)

// YDinhCua is bot's intent set.
func YDinhCua(bot obs.Bot) (dong.Tap[YDinh], bool) {
	switch bot {
	case obs.BotNep:
		return YDinhNep, true
	case obs.BotNhom:
		return YDinhNhom, true
	}
	return dong.Tap[YDinh]{}, false
}

// MaxYDinh is how many intents one message may carry.
const MaxYDinh = 3

// Tien is the router's money class. The router is the money classifier:
// split_draft is a request the group assistant may turn into a draft for a
// human to confirm; money_action is any request to move, record, settle or
// remind about money, which both bots refuse (ai_khong_cham_tien).
type Tien string

const (
	TienNone    Tien = "none"
	SplitDraft  Tien = "split_draft"
	MoneyAction Tien = "money_action"
)

// Tiens is the closed set of Tien.
var Tiens = dong.Moi("tien", TienNone, SplitDraft, MoneyAction)

// TuTin is the router's own confidence.
type TuTin string

const (
	Cao  TuTin = "cao"
	Vua  TuTin = "vua"
	Thap TuTin = "thap"
)

// TuTins is the closed set of TuTin.
var TuTins = dong.Moi("tu_tin", Cao, Vua, Thap)

// Huong is the path the model chooses for the turn (adaptive retrieval: the
// classifier is the model itself, never a rule on the message).
type Huong string

const (
	// TraLoiThang: answer directly, no retrieval, no tool.
	TraLoiThang Huong = "tra_loi_thang"
	// TruyHoiMotBuoc: Go runs the router's TruyVan against the retrievers,
	// then one answer call with the evidence (the fast path).
	TruyHoiMotBuoc Huong = "truy_hoi_mot_buoc"
	// TacTu: the bounded ADK agent loop with the permitted toolset.
	TacTu Huong = "tac_tu"
	// HoiLai: ask the person one question back (CauHoiLai) and end the turn.
	HoiLai Huong = "hoi_lai"
)

// Huongs is the closed set of Huong.
var Huongs = dong.Moi("huong", TraLoiThang, TruyHoiMotBuoc, TacTu, HoiLai)

// The closed vocabularies of the constraint slots. Their ids are the
// catalogue's own (domain/tuvung), so a hard filter the model picks is the
// same id the index filters on. Only the id lists are used here; no reader
// of tuvung runs on the question.
var (
	DiUngs   = dong.Moi("di_ung", tuvung.DiUng.IDs()...)
	AnKiengs = dong.Moi("an_kieng", tuvung.AnKieng.IDs()...)
	LoaiChos = dong.Moi("loai_cho", tuvung.LoaiCho.IDs()...)
	KhiChats = dong.Moi("khi_chat", tuvung.KhiChat.IDs()...)
)

// DiemDen is one destination the turn offers the model: it picks the id,
// Go checks membership.
type DiemDen struct {
	ID  string
	Ten string
}

// ThanhVien is one group member the turn offers the model (group only): an
// id and a short display name, so «cả nhóm trừ Minh» becomes ids Go checks
// by membership.
type ThanhVien struct {
	ID  string
	Ten string
}

// Vao is the router's input.
type Vao struct {
	Bot obs.Bot
	// Doi says the room is a couple (aiharness.Turn.Doi; group bot only):
	// the router reads the couple's bot file (loi_nhac/doi.txt), which
	// speaks of two people; the intents, sources, schema and policy are the
	// group's, unchanged. A Nếp turn with Doi is refused (ErrVao).
	Doi bool
	// Cau is the person's message after the structural preprocessing only:
	// NFC, the @mention of the assistant removed, invisible characters out.
	// It goes to the model inside a datamarked <du_lieu> block.
	Cau string
	// Luc is the turn's «now» on Asia/Ho_Chi_Minh; the prompt states it and
	// the model resolves relative dates against it.
	Luc time.Time
	// NganHan are the session's recent turns, oldest first.
	NganHan []trinho.Luot
	// PhieuNep is the screen card Nếp's panel sent (already checked against
	// its closed vocabulary by the handler), serialized; nil for the group.
	PhieuNep json.RawMessage
	// DanhSachDiemDen is the closed list of destinations the model may pick
	// DiemDenID from. Empty: the slot is not offered.
	DanhSachDiemDen []DiemDen
	// DanhSachThanhVien is the closed list of members the model may pick
	// NguoiThamGia from (group only). Empty: the slot is not offered.
	DanhSachThanhVien []ThanhVien
	// DemNhung is the turn's embedding budget (llm.MaxEmbedCallsPerTurn):
	// the example choice takes its one call from it. The engine always sets
	// it; nil counts nothing.
	DemNhung *nhung.DemLuot
}

// KhungGio is a time window, each end "HH:MM" on Vietnam's clock. Den may be
// "" for a single instant. Den before Tu means the window crosses midnight.
type KhungGio struct {
	Tu  string
	Den string
}

// Slots are the typed constraints the model extracted. Every value is an
// id of a closed list, an ISO string, a flag or an integer: none is free
// text.
type Slots struct {
	DiemDenID string
	// NgayISO is YYYY-MM-DD, resolved by the model from «now»; "" none.
	NgayISO     string
	KhungGio    *KhungGio
	NganSachVND *int64
	// Hard.
	DiUng   []string
	AnKieng []string
	// DiUngNgoaiDanhMuc: the person named an allergen outside the closed
	// list. No filter can check it, so the answer must say so and never
	// claim a place is safe for it.
	DiUngNgoaiDanhMuc bool
	// SoNguoi is the party size; nil when not said.
	SoNguoi *int
	// NguoiThamGia are member ids from the turn's list (group only).
	NguoiThamGia []string
	// Soft.
	LoaiCho []string
	KhiChat []string
	// ThamChieu are evidence ids of earlier turns (trinho.Luot.BangChungIDs)
	// the message refers to («quán thứ hai»).
	ThamChieu []string
}

// TruyVan is one retrieval query the model wrote for a source, in two forms
// (the SOTA gap #3, at zero extra calls: the router already reads the
// message and the session):
//   - Cau: SELF-CONTAINED, every reference to an earlier turn resolved
//     («quán đó», «chỗ hôm qua» → what it names), in the person's own
//     spelling (marks or none, as typed). The folded BM25 field reads it.
//   - CauCoDau: the same query with Vietnamese diacritics restored and
//     teencode written out. The dense leg, the marked BM25 field and the
//     reranker read it; "" means Cau already is that form.
//
// Both are the model's writing, never a Go table's: restoring marks is a
// reading of the words. No hypothetical answer is written (no HyDE): a
// query is what to look for, never a guess at what will be found.
type TruyVan struct {
	Nguon    truyhoi.Nguon
	Cau      string
	CauCoDau string
}

// CoDau is the diacritics-restored form, or Cau when the model wrote none.
func (t TruyVan) CoDau() string {
	if t.CauCoDau != "" {
		return t.CauCoDau
	}
	return t.Cau
}

// KetQua is the router's checked output.
type KetQua struct {
	NhanGuard NhanGuard
	// YDinh has 1..MaxYDinh intents of the bot's set, in the order the
	// person said them.
	YDinh []YDinh
	// MoHoVoi are up to MaxMoHo other intents the model found close (the
	// ask-back options and the confusion metric).
	MoHoVoi []YDinh
	Tien    Tien
	Huong   Huong
	Slots   Slots
	// CanTruyHoi are the sources the model judged necessary; empty means
	// answer directly.
	CanTruyHoi []truyhoi.Nguon
	// TruyVan are the model's queries, each on a source of CanTruyHoi; the
	// fast path runs exactly these.
	TruyVan []TruyVan
	// CanHoiLai is Huong == HoiLai.
	CanHoiLai bool
	// CauHoiLai is the question to ask back, written by the model, set iff
	// CanHoiLai, with at most MaxLuaChon short options. Both are output to
	// the person (they pass the output guard), never input to another
	// prompt.
	CauHoiLai     string
	LuaChonHoiLai []string
	// TraLoiCauCho: the message answers the question the assistant asked in
	// the previous turn.
	TraLoiCauCho bool
	TuTin        TuTin
}

// Hieu is the router.
type Hieu interface {
	// Hieu makes one model call through dem and returns the checked result
	// (Doc). An error means no usable result; the engine's policy then runs
	// the turn restricted (read tools only) or asks back, never guesses.
	Hieu(ctx context.Context, v Vao, dem *llm.Dem) (KetQua, error)
}
