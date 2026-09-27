// Package aiharness is the AI engine: one seam, Engine.Run, that the worker
// and the eval both call (ADR-0037 §2.3). Nếp's turn goes through fixed
// stages (dinhtuyen.go), and no stage reads MEANING from the person's words
// by a word list or a pattern (the owner's rule, 2026-09-25): the MODEL
// decides, Go checks structure, budgets, permissions and set membership.
//
//  1. the money screen of the slip (structural: the route the device sent),
//     then preprocess: NFC, invisible characters out, the @mention of the
//     assistant out;
//  2. the router (hieu): ONE structured call that labels the guard, the
//     intents, the money class, the path, the hard constraints and the
//     dates; its money_action ends the turn with the fixed refusal, its
//     chen_lenh keeps the text as data but leaves only the read tools;
//  3. the path the router chose: a direct answer, the retrieval path (crag's
//     one corrective round, then the grounded structured answer of traloi),
//     or the tools (tactu: the fast path or the bounded ADK loop);
//  4. the verifier (kiemchung) in a fresh context on every released prose,
//     which carries the judgement of claimed actions and money, then the
//     output guard's structural checks (canary, quoted instruction, phone,
//     email, account or card number formats). Run returns the Result, or an
//     error whose code (MaCua) has a sentence fixed in aiharness/cau.
//
// The Sink hears status events only, never a Phan, a Delta or a LamLai
// (streaming is built elsewhere). How the turn ended is Run's return value,
// never a Sink event (design 01 §2): the transport emits `xong` after the
// worker's transaction commits, and `that_bai` after the worker fails the
// job. The engine reads and writes no database: its data comes through the
// tools' ports (tools.NguonDuLieu), and the worker stores the answer and the
// metrics row.
package aiharness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/crag"
	"mobile/services/core/internal/aiharness/guard"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/preprocess"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// Nếp's generation settings and bounds (design 01 §3.4b, §3.5, §5).
const (
	nepTen         = "nep"
	nepNhietDo     = 0.4
	nepMaxTokens   = 768
	nepMaxBuoc     = 3
	nepMaxChu      = 2000
	hanLuot        = 40 * time.Second
	vaiToi, vaiNep = "toi", "nep"
)

// NepMaxChu is Nếp's answer ceiling in runes, exported for the eval
// (aieval/hang.go), which reads the engine's numbers instead of restating
// them (design 06 §2).
const NepMaxChu = nepMaxChu

// Nhip mirrors the slip's timing (keo/nhip-keo.ts).
type Nhip struct {
	Kieu      string
	ConNgay   *int
	TruocNgay *int
}

// PhieuNep is the context slip the device sent, already checked by the
// handler against the closed vocabulary of nep/phieu.ts.
type PhieuNep struct {
	Man    string
	TieuDe string
	Nhip   *Nhip
	LoaiSo string
	SoLieu map[string]any
	GoiY   []string
}

// LuotNep is one earlier turn of the open panel session.
type LuotNep struct {
	Vai string // "toi" or "nep"
	Chu string
}

// Turn is one question.
type Turn struct {
	Bot          obs.Bot
	InvocationID string
	// LanThu is the job's attempt number, for the metrics row.
	LanThu int
	Lenh   obs.Lenh
	// Luc is chat_ai_invocations.created_at: the only source of «now».
	Luc    time.Time
	LoiNho string
	// NguoiHoi is the asking person's id, from the job (never from a model
	// argument): the scope of Nếp's own-outings and memory tools.
	NguoiHoi string
	PhieuNep *PhieuNep
	LuotNep  []LuotNep
	// DaGoiTruoc is the model calls earlier attempts of the same job spent.
	DaGoiTruoc int
	// GiuLuot, when set, holds one model call on the job's durable counter
	// before the call goes out; nil counts in memory only (slices 6 and 9).
	GiuLuot func(context.Context) error
}

// PhanKind is the kind of one grounded part of an answer, the `kind` of the
// stream event phan{kind,json} and of the parts of a `tra_loi` card (contract
// §3).
type PhanKind string

const (
	PhanText         PhanKind = "text"
	PhanPlaces       PhanKind = "places"
	PhanItinerary    PhanKind = "itinerary"
	PhanExpenseDraft PhanKind = "expense_draft"
)

// Sink receives what a turn emits while it runs, and only that (design 01
// §2). It never hears how the turn ended: `xong` and `that_bai` belong to the
// transport, and `xong` goes out only after the worker's transaction that
// stores the answer has committed -- so an answer for a revoked session or a
// cancelled job is never streamed before the database refuses it. The ending
// is Run's return value.
type Sink interface {
	// TrangThai -> trang_thai{cau}. n is how many messages the turn reads;
	// it is not in the event (the caller's SSE takes it from its own bundle,
	// the room frame carries it in the so_tin envelope, design 02 §5.3).
	TrangThai(ma cau.TrangThai, n int)
	// Phan -> phan{kind,json}, only once that part is grounded. S1 never
	// calls it.
	Phan(i int, kind PhanKind, v json.RawMessage)
	// Delta -> delta{p,text}; only the output guard's streaming window calls
	// it. S1 never calls it.
	Delta(p int, text string)
	// LamLai -> lam_lai, only before the first Delta. S1 never calls it.
	LamLai()
}

// BoQua is a Sink that discards everything: the worker's, until streaming.
type BoQua struct{}

func (BoQua) TrangThai(cau.TrangThai, int)        {}
func (BoQua) Phan(int, PhanKind, json.RawMessage) {}
func (BoQua) Delta(int, string)                   {}
func (BoQua) LamLai()                             {}

// Result is a finished turn. Record is filled on failure too. S1 has the text
// only; the parts, chips and sources of design 01 §2 come with the slices
// that produce them.
type Result struct {
	Text string
	// LuaChon are the short options of a question back (router path only),
	// for the panel to offer as chips; empty otherwise.
	LuaChon []string
	Record  obs.TurnRecord
}

// Loi is a turn that ended without an answer.
type Loi struct {
	Ma cau.Ma
	// TamThoi marks a provider failure worth trying again after a pause: a
	// 429 or a 5xx that the model layer already retried, or the per-model
	// rate limiter refusing the call. The worker decides whether the job
	// still may (design 02 §4 step 5); the engine only says what happened.
	TamThoi bool
}

func (e *Loi) Error() string { return "aiharness: turn ended with " + string(e.Ma) }

// TamThoi reports whether err is a transient provider failure (Loi.TamThoi).
func TamThoi(err error) bool {
	var l *Loi
	return errors.As(err, &l) && l.TamThoi
}

// ErrHuy is a turn stopped from outside: the job's context was cancelled
// because its lease is gone (cancelled, revoked, taken over) or the worker is
// stopping. It is not a failure of the turn, the model or the provider, and
// has no job code: the job is not this worker's to end. It wraps
// context.Canceled.
var ErrHuy = fmt.Errorf("aiharness: turn stopped from outside: %w", context.Canceled)

// MaCua is the job code for err: the turn's own code, or provider_unavailable
// for anything else. ErrHuy has none; callers check it first.
func MaCua(err error) cau.Ma {
	var l *Loi
	if errors.As(err, &l) {
		return l.Ma
	}
	return cau.ProviderUnavailable
}

// Engine runs turns. Safe for concurrent use: nothing per turn lives on it.
type Engine struct {
	model  model.LLM
	logger *slog.Logger
	maKiem string
	now    func() time.Time
	cho    func(int) time.Duration
	han    time.Duration
	// gioiHan, when set, is asked before every model call (the per-model
	// rate limiter, design 02 §6); nil lets every call through.
	gioiHan llm.GioiHan
	// hieu is the router every Nếp turn goes through (dinhtuyen.go). New
	// sets hieu.Moi() (no worked examples) when no option gives one.
	hieu hieu.Hieu
	// nguon are the tools' data ports; a nil port makes its tools answer
	// loi_nguon, and the retrieval path falls back to the tools.
	nguon tools.NguonDuLieu
	// xepLai is the reranker (nil: truyhoi.Passthrough, flagged no_rerank).
	xepLai truyhoi.Reranker
	// cham grades a retrieval (nil: crag.ChamLLM); kiem verifies an answer
	// (nil: kiemchung.VerifierLLM).
	cham crag.Cham
	kiem kiemchung.Verifier
	// quyen is the tool permission table (nil: tools.MacDinh).
	quyen *tools.Quyen
	// hoSo lays the person's own recalled facts into a Nếp turn's data
	// block (nil: none). Nếp only: the group path never calls it.
	hoSo HoSo
	// nganHan buffers a Nếp turn's device session for the tool part (nil:
	// the turns stay in this process's memory, phienThietBi).
	nganHan NganHanLuot
}

// HoSo is personalization for one Nếp turn (production: nepnho.Kho). It
// returns at most five of the person's own facts, none when the person's
// memory toggle is off or nothing is recalled. The engine lays them into
// the turn as memory evidence (tools.BoiCanh.NapTriNho): in the ledger
// under aliases, before the verifier, datamarked in the answer's prompt.
// The engine never asks it for the group.
type HoSo interface {
	HoSoNep(ctx context.Context, nguoi, cau string) ([]trinho.SuThat, error)
}

// NganHanLuot is the short-term memory a Nếp turn buffers its device
// session in (production: aictx.Kho on the redis-ai instance): one key per
// turn, named by PhienLuot, written when the turn starts and dropped when it
// ends.
type NganHanLuot interface {
	trinho.NganHan
	PhienLuot(nguoi, luot string) (string, error)
}

// Option configures an Engine.
type Option func(*Engine)

// WithModel sets the model; tests pass an llm.Stub, never a network model.
func WithModel(m model.LLM) Option { return func(e *Engine) { e.model = m } }

// WithLogger sets where the one line per turn goes.
func WithLogger(l *slog.Logger) Option { return func(e *Engine) { e.logger = l } }

// WithMaKiem fixes the canary marker (tests); by default it is random per
// process.
func WithMaKiem(s string) Option { return func(e *Engine) { e.maKiem = s } }

// WithClock sets the clock durations are measured with (tests).
func WithClock(now func() time.Time) Option { return func(e *Engine) { e.now = now } }

// WithRetryWait sets the waits between model retries (tests pass zero).
func WithRetryWait(cho func(int) time.Duration) Option { return func(e *Engine) { e.cho = cho } }

// WithGioiHan sets the per-model call rate limiter. A refusal before any
// content ends the turn as a transient provider failure (Loi.TamThoi); a
// limiter that cannot answer lets the call through.
func WithGioiHan(g llm.GioiHan) Option { return func(e *Engine) { e.gioiHan = g } }

// WithHieu sets the router (production: hieu.Moi with the worked examples).
func WithHieu(h hieu.Hieu) Option { return func(e *Engine) { e.hieu = h } }

// WithNguon sets the tools' data ports (production: internal/aidoc over the
// pool, which holds the database code the engine may not).
func WithNguon(n tools.NguonDuLieu) Option { return func(e *Engine) { e.nguon = n } }

// WithXepLai sets the reranker of the retrieval path.
func WithXepLai(x truyhoi.Reranker) Option { return func(e *Engine) { e.xepLai = x } }

// WithCham sets the retrieval grader (tests; production uses crag.ChamLLM).
func WithCham(c crag.Cham) Option { return func(e *Engine) { e.cham = c } }

// WithKiem sets the verifier (tests; production uses kiemchung.VerifierLLM).
func WithKiem(k kiemchung.Verifier) Option { return func(e *Engine) { e.kiem = k } }

// WithQuyen sets the tool permission table (tests).
func WithQuyen(q *tools.Quyen) Option { return func(e *Engine) { e.quyen = q } }

// WithHoSo sets personalization for Nếp's turns (production: nepnho.Kho).
func WithHoSo(h HoSo) Option { return func(e *Engine) { e.hoSo = h } }

// WithNganHan sets where a Nếp turn buffers its device session (production:
// aictx.Kho).
func WithNganHan(n NganHanLuot) Option { return func(e *Engine) { e.nganHan = n } }

// withHanLuot shortens the turn deadline (tests).
func withHanLuot(d time.Duration) Option { return func(e *Engine) { e.han = d } }

// New builds an engine. A model is required.
func New(opts ...Option) (*Engine, error) {
	e := &Engine{logger: slog.Default(), now: time.Now, han: hanLuot}
	for _, o := range opts {
		o(e)
	}
	if e.model == nil {
		return nil, errors.New("aiharness: no model")
	}
	if e.hieu == nil {
		e.hieu = hieu.Moi()
	}
	if e.cham == nil {
		e.cham = crag.ChamLLM{}
	}
	if e.kiem == nil {
		e.kiem = kiemchung.VerifierLLM{}
	}
	if e.maKiem == "" {
		var b [6]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, err
		}
		e.maKiem = hex.EncodeToString(b[:])
	}
	return e, nil
}

// FromEnv builds the production engine: Gemini from GEMINI_API_KEY (and a
// loopback MOBILE_GEMINI_BASE_URL, if any), with any further options (the
// worker passes its rate limiter).
func FromEnv(ctx context.Context, getenv func(string) string, logger *slog.Logger, opts ...Option) (*Engine, error) {
	m, err := llm.GeminiFromEnv(ctx, getenv)
	if err != nil {
		return nil, err
	}
	return New(append([]Option{WithModel(m), WithLogger(logger)}, opts...)...)
}

func ms(d time.Duration) int { return int(d / time.Millisecond) }

// soTinNep is the n of Nếp's status events: a personal question reads no
// room messages.
const soTinNep = 0

// Run runs one turn to its end and returns how it ended; the Sink hears only
// what happened on the way.
func (e *Engine) Run(ctx context.Context, t Turn, s Sink) (Result, error) {
	batDau := e.now()
	rec := obs.TurnRecord{
		InvocationID: obs.ID(t.InvocationID), LanThu: t.LanThu, Bot: t.Bot, Lenh: t.Lenh,
		Guard: obs.GuardProceed, OutGuard: obs.OutNone, LoiMoHinh: obs.LoiKhong,
		PromptVersion: obs.PromptVersion(prompts.VersionNep()), KetKiem: obs.KiemKhongChay,
	}
	// The first status goes out before any I/O: the panel's «thinking»
	// state never waits on the model.
	s.TrangThai(cau.DangDoc, soTinNep)
	rec.MsTrangThaiDau = ms(e.now().Sub(batDau))
	var res Result
	var err error
	if t.Bot == obs.BotNep {
		res, err = e.nep(ctx, t, s, &rec, batDau)
	} else {
		// The group bot moves onto the engine in slice 9; until then a group
		// turn here is a wiring mistake, answered without a model call.
		err = &Loi{Ma: cau.InvalidAIResult}
	}
	rec.MsTong = ms(e.now().Sub(batDau))
	switch {
	case err == nil:
		rec.KetThuc = obs.KetThucXong
	case errors.Is(err, ErrHuy):
		// Stopped from outside: no code, and no provider class either.
		rec.KetThuc = obs.KetThucHuy
	default:
		rec.KetThuc = obs.KetThucThatBai
		rec.Code = obs.Code(MaCua(err))
	}
	obs.Log(ctx, e.logger, rec)
	res.Record = rec
	return res, err
}

// kiemDauRa holds a text about to be released to the panel's shape and to
// the output guard's STRUCTURAL checks: not empty, valid UTF-8, within
// Nếp's length, no canary marker, no quoted clause of the instruction, and
// no phone number, email, bank account or card number (data-format
// validation for privacy). Whether the text claims an action or money is
// the verifier's judgement (kiemchung), never a phrase rule here.
func (e *Engine) kiemDauRa(text string, rec *obs.TurnRecord) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		rec.LoiMoHinh = obs.LoiBadResp
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > nepMaxChu {
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	if (guard.DauRa{MaKiem: e.maKiem, LoiNhac: prompts.LoiNhacNep()}).Kiem(text) != guard.RaSach {
		rec.OutGuard = obs.OutChan
		return Result{}, &Loi{Ma: cau.TraLoiBiChan}
	}
	return Result{Text: text}, nil
}

// loiMoHinh is how a failed model stage ends the turn: stopped from
// outside, out of budget or time, blocked by the provider, or the provider
// failing (transient for a 429 or a 5xx).
func loiMoHinh(err error, huy, hetGio bool, rec *obs.TurnRecord) error {
	switch {
	case huy:
		return ErrHuy
	case errors.Is(err, llm.ErrHetNganSach) || errors.Is(err, agent.ErrHetBuoc) || errors.Is(err, tools.ErrHetLuotGoi):
		return &Loi{Ma: cau.HetNganSach}
	case hetGio:
		rec.LoiMoHinh = obs.LoiTimeout
		return &Loi{Ma: cau.HetNganSach}
	case errors.Is(err, llm.ErrKhongUngVien), errors.Is(err, cautruc.ErrBiChan):
		// The provider blocked the prompt itself, or withheld a structured
		// answer: its safety refusal, the same as a candidate withheld for
		// safety.
		rec.LoiMoHinh = obs.LoiSafety
		return &Loi{Ma: cau.InvalidAIResult}
	default:
		rec.LoiMoHinh = llm.PhanLoai(err)
		return &Loi{Ma: cau.ProviderUnavailable, TamThoi: rec.LoiMoHinh == obs.Loi429 || rec.LoiMoHinh == obs.Loi5xx}
	}
}

// sach cleans one slip string structurally (NFC, invisible characters) and
// counts what it removed. Nothing is dropped for its words: the text goes
// to the model as data.
func sach(s string, rec *obs.TurnRecord) (string, bool) {
	c := preprocess.LamSach(s)
	rec.KyTuAn += c.KyTuAn
	return c.Chu, c.Chu != ""
}

// renderPhieu renders the slip, one field per line. The order and the
// number format are fixed, so the request is stable byte for byte.
func renderPhieu(p *PhieuNep, rec *obs.TurnRecord) string {
	if p == nil {
		return ""
	}
	var lines []string
	if v, ok := sach(p.Man, rec); ok {
		lines = append(lines, "man: "+v)
	}
	if v, ok := sach(p.TieuDe, rec); ok {
		lines = append(lines, "tieuDe: "+v)
	}
	if p.Nhip != nil {
		if v, ok := sach(p.Nhip.Kieu, rec); ok {
			line := "nhip: " + v
			if p.Nhip.ConNgay != nil {
				line += ", conNgay=" + strconv.Itoa(*p.Nhip.ConNgay)
			}
			if p.Nhip.TruocNgay != nil {
				line += ", truocNgay=" + strconv.Itoa(*p.Nhip.TruocNgay)
			}
			lines = append(lines, line)
		}
	}
	if v, ok := sach(p.LoaiSo, rec); ok {
		lines = append(lines, "loaiSo: "+v)
	}
	if len(p.SoLieu) > 0 {
		keys := make([]string, 0, len(p.SoLieu))
		for k := range p.SoLieu {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			switch v := p.SoLieu[k].(type) {
			case float64:
				if v == float64(int64(v)) {
					parts = append(parts, k+"="+strconv.FormatInt(int64(v), 10))
				} else {
					parts = append(parts, k+"="+strconv.FormatFloat(v, 'f', -1, 64))
				}
			case int:
				parts = append(parts, k+"="+strconv.Itoa(v))
			case string:
				if c, ok := sach(v, rec); ok {
					parts = append(parts, k+"="+c)
				}
			}
		}
		if len(parts) > 0 {
			lines = append(lines, "soLieu: "+strings.Join(parts, ", "))
		}
	}
	var goiY []string
	for _, g := range p.GoiY {
		if v, ok := sach(g, rec); ok {
			goiY = append(goiY, v)
		}
	}
	if len(goiY) > 0 {
		lines = append(lines, "goiY: "+strings.Join(goiY, " | "))
	}
	return strings.Join(lines, "\n")
}
