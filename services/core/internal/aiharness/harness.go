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
// The Sink hears the statuses, then -- only once the answer has passed the
// verifier and every structural check (draft, then verify, then stream:
// slice 11) -- the verified text in Deltas through the output guard's
// 48-rune window (phatRa). No unverified byte ever reaches it, and never a
// Phan or a LamLai. How the turn ended is Run's return value,
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

	// The group turn (Bot nhom; design 03): what the worker read from the
	// stored job and the room, nothing looked up by the engine.
	//
	// Phong is the room; Lane its transport as the server derived it
	// ("legacy" or "v2"): for a v2 room no shared turn reaches the model or
	// the short-term store. SoTin is how many shared turns the server
	// confirmed belong to the room (so_tin_doc), the n the card says it read.
	Phong string
	Lane  string
	SoTin int
	// LuotNhom are the turns the caller explicitly shared, in the bundle's
	// order (the recent messages and the reply chain to an earlier answer);
	// the tag message itself is LoiNho.
	LuotNhom []LuotNhom
	// ThanhVien are the room's active members, the caller among them, under
	// the roster's labels.
	ThanhVien []ThanhVienNhom
}

// Lanes of a room (chat_ai_invocations.lane).
const (
	LaneLegacy = "legacy"
	LaneV2     = "v2"
)

// LuotNhom is one shared turn of a group invocation.
type LuotNhom struct {
	// ID is the message's id (checked to belong to the room at create).
	ID string
	// Vai is who wrote it: "toi" (the caller), "ban" (another member) or
	// "ai" (an earlier answer of the assistant).
	Vai string
	// Ten is the writer's label as the room knows them (the roster's, safe
	// to show a model); "" for the caller and the assistant.
	Ten string
	Chu string
	// TacGia is the author's person id as messages.author_id has it, "" when
	// the server could not confirm one: a split draft bills only a
	// confirmed author, never a name the model wrote.
	TacGia string
	// ChuMayChu is the message's text as the server stores it (legacy lane
	// only; "" when the server has none to read, as in a v2 room). A split
	// draft reads an expense only from this text, never from the client's
	// copy in Chu, so the words that bill an author are the author's own
	// (review of slices 9/11, finding 2.3). "" means no payer attribution.
	ChuMayChu string
}

// ThanhVienNhom is one active member of the room.
type ThanhVienNhom struct {
	// ID is the person id; it never reaches a prompt (the router sees the
	// alias m1, m2, …).
	ID  string
	Ten string
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
	// it, and only on an answer the verifier already passed (phatRa).
	Delta(p int, text string)
	// LamLai -> lam_lai, only before the first Delta. S1 never calls it.
	LamLai()
}

// BoQua is a Sink that discards everything: the worker's when no stream is
// configured.
type BoQua struct{}

func (BoQua) TrangThai(cau.TrangThai, int)        {}
func (BoQua) Phan(int, PhanKind, json.RawMessage) {}
func (BoQua) Delta(int, string)                   {}
func (BoQua) LamLai()                             {}

// ChiTrangThai passes the statuses and a restart on to its Sink and drops
// the text: the group worker's Sink while a turn runs. A group's answer
// reaches its readers only after its card is posted (contract §4.1), from
// the card itself, so nothing the turn releases may go anywhere before
// then. Like BoQua it is never paced: nobody reads the text it drops.
type ChiTrangThai struct{ Sink }

func (ChiTrangThai) Phan(int, PhanKind, json.RawMessage) {}
func (ChiTrangThai) Delta(int, string)                   {}

// Result is a finished turn. Record is filled on failure too. S1 has the text
// only; the parts, chips and sources of design 01 §2 come with the slices
// that produce them.
type Result struct {
	Text string
	// LuaChon are the short options of a question back (router path only),
	// for the panel to offer as chips; empty otherwise.
	LuaChon []string
	Record  obs.TurnRecord

	// The group's answer: the parts of its `tra_loi` card in order, each in
	// the raw form companion.GroundReply grounds ({"kind","payload"}); the
	// catalogue ids those parts name, whose rows the worker loads for that
	// grounding; and a split draft's drafts for the invocation's result
	// column (nil otherwise). All three are nil for Nếp.
	Phan       []json.RawMessage
	QuanIDs    []string
	KetQuaNhap json.RawMessage
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
	// nhipPhat paces the release of a verified answer (phatRa); 0 releases
	// it at once.
	nhipPhat time.Duration
	// nganHanNhom buffers a legacy-lane group turn's shared messages for
	// the tool part (nil: in memory). Never Nếp's store port: the group
	// path names neither nganHan nor hoSo (internal/aigate).
	nganHanNhom NganHanNhom
}

// NganHanNhom is the short-term memory a group turn buffers its shared
// messages in (production: aictx.Kho): one key per invocation, named by
// PhienLuotNhom, which refuses any lane but the legacy one.
type NganHanNhom interface {
	trinho.NganHan
	PhienLuotNhom(phong, luot, lane string) (string, error)
}

// WithNganHanNhom sets where a group turn buffers its shared messages
// (production: aictx.Kho).
func WithNganHanNhom(n NganHanNhom) Option { return func(e *Engine) { e.nganHanNhom = n } }

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

// NhipPhat is the production pause between chunks of a verified answer
// (WithNhipPhat): 16 runes every 25 ms, the whole answer within
// guard.TranNhip.
const NhipPhat = 25 * time.Millisecond

// WithNhipPhat paces the release of a verified answer to the Sink, so a
// client that renders each Delta shows it progressively (slice 11). Unset,
// the answer leaves at once (tests, the eval).
func WithNhipPhat(d time.Duration) Option { return func(e *Engine) { e.nhipPhat = d } }

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
	switch t.Bot {
	case obs.BotNhom:
		return e.RunNhom(ctx, t, s)
	case obs.BotNep:
		return e.boc(ctx, t, s, obs.PromptVersion(prompts.VersionNep()), soTinNep, e.nepVaPhat)
	}
	// A bot outside the closed set is a wiring mistake, answered without a
	// model call.
	return e.boc(ctx, t, s, obs.PromptVersion(prompts.VersionNep()), soTinNep,
		func(context.Context, Turn, Sink, *obs.TurnRecord, time.Time) (Result, error) {
			return Result{}, &Loi{Ma: cau.InvalidAIResult}
		})
}

// boc runs one bot's turn: the record, the first status before any I/O, the
// ending and the one log line. It names neither bot's path, so the group's
// entry (RunNhom) reaches nothing of Nếp's through it.
func (e *Engine) boc(ctx context.Context, t Turn, s Sink, pv obs.PromptVersion, soTin int,
	chay func(context.Context, Turn, Sink, *obs.TurnRecord, time.Time) (Result, error)) (Result, error) {
	batDau := e.now()
	rec := obs.TurnRecord{
		InvocationID: obs.ID(t.InvocationID), LanThu: t.LanThu, Bot: t.Bot, Lenh: t.Lenh,
		Guard: obs.GuardProceed, OutGuard: obs.OutNone, LoiMoHinh: obs.LoiKhong,
		PromptVersion: pv, KetKiem: obs.KiemKhongChay,
	}
	// The first status goes out before any I/O: the panel's «thinking»
	// state never waits on the model.
	s.TrangThai(cau.DangDoc, soTin)
	rec.MsTrangThaiDau = ms(e.now().Sub(batDau))
	res, err := chay(ctx, t, s, &rec, batDau)
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
	return e.kiemDauRaK(khuonNep(), text, rec)
}

// khuon is a bot's output shape: its answer ceiling in runes, the clauses of
// its instruction an answer must never quote, and the fixed sentence that
// ends a text the streaming window stops part-way.
type khuon struct {
	maxChu  int
	loiNhac []string
	cauChan string
}

func khuonNep() khuon {
	return khuon{maxChu: nepMaxChu, loiNhac: prompts.LoiNhacNep(), cauChan: cau.Cau(cau.TraLoiBiChan)}
}

// nepVaPhat is Nếp's turn, then the release of its verified answer
// (phatRa).
func (e *Engine) nepVaPhat(ctx context.Context, t Turn, s Sink, rec *obs.TurnRecord, batDau time.Time) (Result, error) {
	res, err := e.nep(ctx, t, s, rec, batDau)
	if err == nil {
		res, err = e.phatRa(ctx, res, s, rec, khuonNep())
	}
	return res, err
}

// kiemDauRaK is kiemDauRa for the output shape k.
func (e *Engine) kiemDauRaK(k khuon, text string, rec *obs.TurnRecord) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		rec.LoiMoHinh = obs.LoiBadResp
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > k.maxChu {
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	// The streaming window's scan (the whole-answer checks, and the head of
	// every prompt clause), so a text that passes here also passes the
	// window it streams through (phatRa) and nothing is written for an
	// answer the window would then withhold.
	if guard.KiemCuaSo(guard.DauRa{MaKiem: e.maKiem, LoiNhac: k.loiNhac}, text) != guard.RaSach {
		rec.OutGuard = obs.OutChan
		return Result{}, &Loi{Ma: cau.TraLoiBiChan}
	}
	return Result{Text: text}, nil
}

// phatRa streams an answer that has passed the verifier and the structural
// checks to the Sink, through the output guard's 48-rune window (slice 11),
// a chunk at a time and e.nhipPhat apart so the panel shows it progressively
// (guard.PhatTheoNhip). Nothing reaches the Sink before this point: the draft
// is verified whole and only then released (design: draft, verify, stream).
// The window's scan reads the text whole once more first (it also stops on
// the head of a prompt clause, which kiemDauRa already ran), so a text it
// would stop leaves nothing; if the window ever stopped one part-way, the
// part that left stays and the fixed sentence ends it, and Result.Text is
// what the Deltas carried, joined.
func (e *Engine) phatRa(ctx context.Context, res Result, s Sink, rec *obs.TurnRecord, k khuon) (Result, error) {
	nhip := e.nhipPhat
	switch s.(type) {
	case BoQua, ChiTrangThai:
		// Nobody reads a discarding Sink: no pause is worth its latency.
		nhip = 0
	}
	kq, err := guard.PhatTheoNhip(ctx, s, 0, guard.DauRa{MaKiem: e.maKiem, LoiNhac: k.loiNhac}, k.maxChu, k.cauChan, res.Text, nhip)
	switch {
	case err != nil:
		return Result{}, ErrHuy
	case kq.Chan != guard.RaSach:
		rec.OutGuard = obs.OutChan
		if kq.DaNha == "" {
			return Result{}, &Loi{Ma: cau.TraLoiBiChan}
		}
	case kq.KhongHopLe || kq.Chu == "":
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	res.Text = kq.Chu
	return res, nil
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
