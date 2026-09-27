// Package trinho is the engine's port to memory: long-term facts about one
// person (TriNho, Nếp only, ADR-0041) and the short-term turns of one open
// session (NganHan, both bots). This package holds the contract only; the
// adapters (the nepnho Postgres writer, a mem0 sidecar, Redis for short-term
// turns) live elsewhere and implement these interfaces. In-memory fakes for
// tests are in aiharness/testkit.
//
// What reaches memory, and what a recall means, is decided by the model
// (the router's intent, the remember_fact / forget_fact / recall_memory
// tools): no adapter decides by word lists what is worth keeping, what a
// «forget» refers to or which fact answers a question. An adapter may only
// validate structure (closed Loai, lengths, owner) and enforce the rules
// below, which are data rules, not language understanding.
//
// Rules every adapter keeps:
//   - A fact belongs to exactly one person; nothing reads across people.
//   - «Forget» is a hard delete (with a hashed tombstone in nepnho), never a
//     flag.
//   - Money, other people, chat text, coordinates and sensitive traits are
//     never stored (design 05 §6); the remember_fact tool refuses them by
//     the model's own structured classification plus the output-format
//     checks, not by keyword.
package trinho

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness/dong"
)

// LoaiSuThat is the closed kind of a long-term fact (design 05 §6).
type LoaiSuThat string

const (
	ThichDanhMuc   LoaiSuThat = "thich_danh_muc"
	ThichDiaDiem   LoaiSuThat = "thich_dia_diem"
	NeDiaDiem      LoaiSuThat = "ne_dia_diem"
	DiemDenQuen    LoaiSuThat = "diem_den_quen"
	KhungGioHayDi  LoaiSuThat = "khung_gio_hay_di"
	PhuongTien     LoaiSuThat = "phuong_tien"
	ThoiLuongChang LoaiSuThat = "thoi_luong_chang"
	NhipLenKeo     LoaiSuThat = "nhip_len_keo"
	DieuDaDan      LoaiSuThat = "dieu_da_dan"
)

// LoaiSuThats is the closed set of LoaiSuThat.
var LoaiSuThats = dong.Moi("loai_su_that",
	ThichDanhMuc, ThichDiaDiem, NeDiaDiem, DiemDenQuen, KhungGioHayDi,
	PhuongTien, ThoiLuongChang, NhipLenKeo, DieuDaDan)

// NguonSuThat is how a fact was learnt (design 05 §5).
type NguonSuThat string

const (
	// HanhVi: from what the person did in the app.
	HanhVi NguonSuThat = "hanh_vi"
	// NoiRo: the person said it and asked to be remembered.
	NoiRo NguonSuThat = "noi_ro"
	// HoiDap: the person answered a question Nếp asked.
	HoiDap NguonSuThat = "hoi_dap"
)

// NguonSuThats is the closed set of NguonSuThat.
var NguonSuThats = dong.Moi("nguon_su_that", HanhVi, NoiRo, HoiDap)

// MaxNoiDung is the longest fact sentence in runes (nep_su_that.cau).
const MaxNoiDung = 160

// SuThat is one long-term fact about the person who owns it.
type SuThat struct {
	ID      string
	NoiDung string
	Loai    LoaiSuThat
	// TuLuc is when the fact became true; DenLuc, when set, when it stops
	// (valid time, not the time Nếp wrote it).
	TuLuc  time.Time
	DenLuc *time.Time
	Nguon  NguonSuThat
}

// TatCa is everything remembered about one person, for what_you_remember:
// listed in full by Go, never summarised by the model (ADR-0041).
//
// Every field after SuThat was added by the nepnho adapter (infra
// memory-policy): what_you_remember must be truthful about everything held,
// not only the facts ready for recall.
type TatCa struct {
	SuThat []SuThat
	// DangXoa are facts the person asked to forget whose deletion has not
	// been counted complete yet: still held, never recalled.
	DangXoa []SuThat
	// KhongSo are facts the store holds without a receipt (a write whose
	// ledger row failed): never recalled, listed because they are held,
	// and deleted with the next forget-all.
	KhongSo []SuThat
	// SuKien counts the typed app events held, by kind, in their 30-day
	// window. Counts only: an event holds ids and enums, never words.
	SuKien map[LoaiSuKien]int
	// Bat is the memory toggle.
	Bat bool
}

// LoaiSuKien is the closed kind of a typed app event (design 05 §6, source
// 1): a kind and typed ids or enums, never words. Added with TatCa.SuKien.
type LoaiSuKien string

const (
	MoDiaDiem      LoaiSuKien = "mo_dia_diem"
	LuuDiaDiem     LoaiSuKien = "luu_dia_diem"
	BoLuu          LoaiSuKien = "bo_luu"
	ThemChang      LoaiSuKien = "them_chang"
	ChonPhuongTien LoaiSuKien = "chon_phuong_tien"
	ChonThoiLuong  LoaiSuKien = "chon_thoi_luong"
	LocDanhMuc     LoaiSuKien = "loc_danh_muc"
	ChonDiemDen    LoaiSuKien = "chon_diem_den"
	TaoKeo         LoaiSuKien = "tao_keo"
	CheckIn        LoaiSuKien = "check_in"
)

// LoaiSuKiens is the closed set of LoaiSuKien.
var LoaiSuKiens = dong.Moi("loai_su_kien",
	MoDiaDiem, LuuDiaDiem, BoLuu, ThemChang, ChonPhuongTien, ChonThoiLuong,
	LocDanhMuc, ChonDiemDen, TaoKeo, CheckIn)

// QuenGi names what to forget: exactly one of ID (a fact the person picked)
// or MoTa (the person's own description, which the adapter resolves by the
// model or by embedding similarity, never by keyword).
type QuenGi struct {
	ID   string
	MoTa string
}

// ErrQuenMoHo: a QuenGi with both or neither field set.
var ErrQuenMoHo = errors.New("trinho: forget needs exactly one of ID or MoTa")

// Kiem checks the structure of q.
func (q QuenGi) Kiem() error {
	if (q.ID == "") == (q.MoTa == "") {
		return ErrQuenMoHo
	}
	return nil
}

// TriNho is long-term memory for one person at a time (Nếp only). nguoi is
// the person id; every method is scoped to it.
type TriNho interface {
	// Nho recalls at most k facts relevant to cau (the query the model
	// wrote, or the person's words), best first. Relevance is the adapter's
	// ranking (embedding, reranker), never a word list.
	Nho(ctx context.Context, nguoi, cau string, k int) ([]SuThat, error)
	// Ghi stores one fact the person stated about themself and returns it
	// with its id. The adapter calls moi.Kiem first.
	Ghi(ctx context.Context, nguoi string, moi SuThatMoi) (SuThat, error)
	// Quen hard-deletes the facts q names and returns how many went.
	Quen(ctx context.Context, nguoi string, q QuenGi) (int, error)
	// LietKe lists every fact of the person.
	LietKe(ctx context.Context, nguoi string) (TatCa, error)
}

// SuThatMoi is a fact to write: the content, its closed kind and its valid
// time, all produced by the model through the remember_fact tool's schema
// and checked for structure only.
type SuThatMoi struct {
	NoiDung string
	Loai    LoaiSuThat
	TuLuc   time.Time
	DenLuc  *time.Time
	Nguon   NguonSuThat
}

// ErrNoiDungDai: a fact longer than MaxNoiDung runes, or empty.
var ErrNoiDungDai = errors.New("trinho: fact content empty or longer than MaxNoiDung")

// ErrThoiGian: DenLuc not after TuLuc, or TuLuc unset.
var ErrThoiGian = errors.New("trinho: fact valid time is empty or inverted")

// Kiem checks the structure of a fact to write: content length, closed kind
// and source, valid time. It reads no meaning from the content.
func (m SuThatMoi) Kiem() error {
	if n := utf8.RuneCountInString(m.NoiDung); n == 0 || n > MaxNoiDung {
		return ErrNoiDungDai
	}
	if !LoaiSuThats.Co(m.Loai) {
		return fmt.Errorf("%w: loai %q", dong.ErrLa, m.Loai)
	}
	if !NguonSuThats.Co(m.Nguon) {
		return fmt.Errorf("%w: nguon %q", dong.ErrLa, m.Nguon)
	}
	if m.TuLuc.IsZero() || (m.DenLuc != nil && !m.DenLuc.After(m.TuLuc)) {
		return ErrThoiGian
	}
	return nil
}

// VaiLuot is who spoke a short-term turn.
type VaiLuot string

const (
	// Toi: the person the assistant answers.
	Toi VaiLuot = "toi"
	// TroLy: the assistant.
	TroLy VaiLuot = "tro_ly"
)

// VaiLuots is the closed set of VaiLuot.
var VaiLuots = dong.Moi("vai_luot", Toi, TroLy)

// Luot is one short-term turn. BangChungIDs are the evidence ids the
// assistant's turn cited, so a later «the second one» can be resolved by the
// model to an id that Go then checks by membership (hieu.Slots.ThamChieu).
type Luot struct {
	Vai          VaiLuot
	Chu          string
	Luc          time.Time
	BangChungIDs []string
}

// MaxLuotNganHan is how many turns Doc returns at most: the newest ones,
// four exchanges of the person and the assistant (research intent-routing
// §4.8). Them keeps at least this many. The group's short-term memory holds
// only turns that called the assistant and its answers, never other chat.
const MaxLuotNganHan = 8

// NganHan is the short-term memory of one open session (phien), which
// expires on its own (a TTL in the adapter); nothing in it is long-term.
type NganHan interface {
	// Doc returns the session's turns, oldest first, at most MaxLuotNganHan.
	Doc(ctx context.Context, phien string) ([]Luot, error)
	// Them appends one turn.
	Them(ctx context.Context, phien string, l Luot) error
	// Xoa drops the session.
	Xoa(ctx context.Context, phien string) error
}
