// Package aiharness is the AI engine: one seam, Engine.Run, that the worker
// and the eval both call (ADR-0037 §2.3). A turn goes through fixed stages:
//
//  1. preprocess (deterministic): NFC, @mentions out, invisible characters
//     out, whitespace collapsed; the «now» line from the instant the question
//     was stored, and the dates it mentions resolved on Vietnam's clock;
//  2. guard (deterministic): the money screen first, then the money law, then
//     the injection patterns on every untrusted source -- the question is
//     flagged, a panel turn or a slip string that trips them is dropped;
//  3. the model, through ADK: one llmagent and one runner for this turn only,
//     under the per-turn call counter and the step budget;
//  4. the output guard on the whole answer; Run returns the Result, or an
//     error whose code (MaCua) has a sentence fixed in aiharness/cau.
//
// S1 (slice 6) runs Nếp only, with no tools and no streaming: the Sink hears
// status events only, never a Phan, a Delta or a LamLai. How the turn ended is
// Run's return value, never a Sink event (design 01 §2): the transport emits
// `xong` after the worker's transaction commits, and `that_bai` after the
// worker fails the job. The engine reads and writes no database; the worker
// stores the answer and the metrics row.
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

	"google.golang.org/adk/model"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/guard"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/preprocess"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/domain/nepphieu"
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
	Luc      time.Time
	LoiNho   string
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
	Text   string
	Record obs.TurnRecord
}

// Loi is a turn that ended without an answer.
type Loi struct{ Ma cau.Ma }

func (e *Loi) Error() string { return "aiharness: turn ended with " + string(e.Ma) }

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
// loopback MOBILE_GEMINI_BASE_URL, if any).
func FromEnv(ctx context.Context, getenv func(string) string, logger *slog.Logger) (*Engine, error) {
	m, err := llm.GeminiFromEnv(ctx, getenv)
	if err != nil {
		return nil, err
	}
	return New(WithModel(m), WithLogger(logger))
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
		PromptVersion: obs.PromptVersion(prompts.VersionNep()),
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

func (e *Engine) nep(ctx context.Context, t Turn, s Sink, rec *obs.TurnRecord, batDau time.Time) (Result, error) {
	// The money screen first, before anything else is read (ADR-0033 §2.2).
	if t.PhieuNep != nil && nepphieu.PhaiLui(t.PhieuNep.Man) {
		rec.Guard = obs.GuardRefused
		return Result{}, &Loi{Ma: cau.NepLuiManTien}
	}
	hoi := preprocess.LamSach(t.LoiNho)
	rec.KyTuAn += hoi.KyTuAn
	rec.KhongDau = hoi.KhongDau
	if hoi.Chu == "" {
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	// The money law: a request to act on money costs no model call.
	if guard.LaTien(hoi.Chu) {
		rec.Guard = obs.GuardRefused
		return Result{}, &Loi{Ma: cau.NepKhongChamTien}
	}
	// The person's own question is flagged, not dropped: it is theirs, it
	// goes as data, and the output guard stands behind it.
	if nghi, _ := guard.Nghi(hoi.Chu); nghi {
		rec.Guard = obs.GuardRestricted
	}
	phieu := e.phieu(t.PhieuNep, rec)
	luot := locLuot(t.LuotNep, rec)
	mayChu, moHo := preprocess.DongMayChu(hoi.Chu, t.Luc)
	rec.NgayMoHo = moHo
	var blocks []string
	if phieu != "" {
		blocks = append(blocks, prompts.BocDuLieu(prompts.PhieuManHinh, phieu))
	}
	blocks = append(blocks, prompts.BocDuLieu(prompts.MayChu, strings.Join(mayChu, "\n")), prompts.BocDuLieu(prompts.CauHoi, hoi.Chu))
	cuoi := strings.Join(blocks, "\n\n")

	conLai := llm.MaxModelCallsPerTurn - t.DaGoiTruoc
	if conLai <= 0 {
		return Result{}, &Loi{Ma: cau.HetNganSach}
	}
	dem := llm.NewDem(e.model, conLai, t.GiuLuot)
	if e.cho != nil {
		dem.WithWait(e.cho)
	}
	rec.MsTienXuLy = ms(e.now().Sub(batDau))
	s.TrangThai(cau.DangNghi, soTinNep)
	var td agent.TheoDoi
	cfg := agent.CauHinh{
		Ten:             nepTen,
		Instruction:     prompts.NepAgent(e.maKiem),
		NhietDo:         nepNhietDo,
		MaxOutputTokens: nepMaxTokens,
		MaxBuoc:         nepMaxBuoc,
		ConLai:          dem.ConLai,
	}
	moHinh := e.now()
	runCtx, cancel := context.WithTimeout(ctx, e.han)
	text, err := agent.Chay(runCtx, dem, cfg, luot, cuoi, &td)
	// Stopped from outside (the heartbeat cancelled the job, or the worker is
	// stopping) is told apart from running out of time: the turn's own
	// deadline, or the job's, is the budget; a cancellation is neither.
	huy := errors.Is(ctx.Err(), context.Canceled)
	hetGio := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	cancel()
	rec.MsMoHinh = ms(e.now().Sub(moHinh))
	rec.SoGoiMoHinh = dem.SoGoi()
	snap := td.Snapshot()
	rec.Buoc, rec.TokensIn, rec.TokensOut, rec.TokensCache, rec.TokensNghi = snap.Buoc, snap.TokensIn, snap.TokensOut, snap.TokensCache, snap.TokensNghi
	switch {
	case err == nil:
	case huy:
		return Result{}, ErrHuy
	case errors.Is(err, llm.ErrHetNganSach) || errors.Is(err, agent.ErrHetBuoc):
		return Result{}, &Loi{Ma: cau.HetNganSach}
	case hetGio:
		rec.LoiMoHinh = obs.LoiTimeout
		return Result{}, &Loi{Ma: cau.HetNganSach}
	case errors.Is(err, llm.ErrKhongUngVien):
		// The provider blocked the prompt itself: its safety refusal, the
		// same as a candidate withheld for safety below.
		rec.LoiMoHinh = obs.LoiSafety
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	default:
		rec.LoiMoHinh = llm.PhanLoai(err)
		return Result{}, &Loi{Ma: cau.ProviderUnavailable}
	}
	if llm.BiChanAnToan(snap.Finish) {
		rec.LoiMoHinh = obs.LoiSafety
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
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

// sach cleans one slip string and says whether it may go to the model.
func sach(s string, rec *obs.TurnRecord) (string, bool) {
	c := preprocess.LamSach(s)
	rec.KyTuAn += c.KyTuAn
	if c.Chu == "" {
		return "", false
	}
	if nghi, _ := guard.Nghi(c.Chu); nghi {
		rec.PhieuBo++
		return "", false
	}
	return c.Chu, true
}

// phieu renders the slip, one field per line, dropping any string the guard
// flags. The order and the number format are fixed, so the request is stable
// byte for byte.
func (e *Engine) phieu(p *PhieuNep, rec *obs.TurnRecord) string {
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

// locLuot turns the panel session into ADK events. The session is split into
// exchanges -- a question and the answers after it -- and a flagged turn drops
// its whole exchange: an answer to a dropped question, or a question whose
// answer was forged, is out of context either way. A device can send back a
// «nep» turn it wrote itself, so Nếp's own turns are guarded like the
// person's (design 01 §3.2).
func locLuot(ls []LuotNep, rec *obs.TurnRecord) []agent.Luot {
	type trao struct {
		luot []agent.Luot
		bo   bool
	}
	var ds []*trao
	for _, l := range ls {
		c := preprocess.LamSach(l.Chu)
		rec.KyTuAn += c.KyTuAn
		nguoi := l.Vai == vaiToi
		if nguoi || len(ds) == 0 {
			ds = append(ds, &trao{})
		}
		cur := ds[len(ds)-1]
		nghi, _ := guard.Nghi(c.Chu)
		if c.Chu == "" || nghi || (l.Vai != vaiToi && l.Vai != vaiNep) {
			cur.bo = true
		}
		// A leading answer with no question before it has nothing to answer.
		if !nguoi && len(cur.luot) == 0 {
			cur.bo = true
		}
		chu := c.Chu
		if nguoi {
			chu = prompts.BocDuLieu(prompts.CauHoi, c.Chu)
		} else {
			chu = strings.NewReplacer("<", "＜", ">", "＞").Replace(c.Chu)
		}
		cur.luot = append(cur.luot, agent.Luot{Nguoi: nguoi, Chu: chu})
	}
	var out []agent.Luot
	for _, d := range ds {
		if d.bo {
			rec.LuotBo += len(d.luot)
			continue
		}
		out = append(out, d.luot...)
	}
	return out
}
