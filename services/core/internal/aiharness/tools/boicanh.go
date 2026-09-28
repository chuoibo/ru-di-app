package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// MaxSua is how many invalid tool calls a turn is answered with a
// correctable error. The call after them is refused and the turn goes
// straight to its final answer with the evidence it has (research
// agentic-rag-tools §2.3.6: one repair, then stop).
const MaxSua = 1

// ChuaCo: the tool exists in the registry but is not built yet
// (set_reminder until slice 17).
const ChuaCo LoiTool = "chua_co"

// gioVN is Vietnam's clock. Vietnam keeps no daylight saving, so a fixed
// zone is exact and needs no tz database in the container.
var gioVN = time.FixedZone("ICT", 7*3600)

// BoiCanh is one turn's tool context: who asks (from the job, never from a
// model argument), the router's hard constraints, the closed lists the turn
// offered the model, the data ports, the ledger and the drafts. One per
// turn; safe for the concurrent tool calls of one agent step.
type BoiCanh struct {
	Bot obs.Bot
	// NguoiHoi is the asking person's id, from the job. Nếp's memory and
	// own-outings tools are scoped to it.
	NguoiHoi string
	// NhomID is the group the question was asked in (group bot only).
	NhomID string
	// Doi says the room is a couple (aiharness.Turn.Doi, read by the worker
	// from the database): only then is a couple's tool (scope Doi, the
	// shared taste of ADR-0048) declared, offered or run.
	Doi bool
	// NhanDoi is each member's label as the turn's roster gives it (never a
	// name a tool reads from the database), by person id: what a couple's
	// tool shows the model beside a person's taste.
	NhanDoi map[string]string
	// LoiNguoiHoi is the person's own message of this turn, cleaned
	// structurally (preprocess.LamSach). A fact remember_fact stores is a
	// span of it, found by exact identity and taken from it (kiemGhiNho):
	// never the model's paraphrase, never text from the history, the slip
	// or a tool's data.
	LoiNguoiHoi string
	// Man is the screen Nếp's panel reports (already checked by the
	// handler); "" for the group.
	Man string
	// Luc is the turn's «now».
	Luc time.Time
	// HanChe is the policy's proceed_restricted: read tools only.
	HanChe bool
	// YDinh are the router's intents for this turn, read by the model from
	// the person's own message. A memory write is offered only when the
	// person asked for it: remember_fact only under hieu.Remember,
	// forget_fact only under hieu.Forget. Text that reaches the model as
	// data (a place, the manual, a remembered fact, the slip, the history)
	// cannot add an intent, so it can never be what makes a write possible.
	YDinh []hieu.YDinh
	// Cung and Mem are the router's constraints (RangBuocTuRouter). Every
	// search merges its own arguments INTO Cung, stricter wins.
	Cung truyhoi.Cung
	Mem  truyhoi.Mem
	// DiUngNgoaiDanhMuc is the router's flag: an allergen outside the closed
	// list was named, so no filter could check it.
	DiUngNgoaiDanhMuc bool
	// DiemDen is the closed list of destination ids offered this turn.
	DiemDen []string
	// Slots and TruyVan are the router's own output for this turn: the
	// values it extracted from the person's words and the search texts it
	// wrote (each with its diacritics-restored form). From the agent's
	// second step on they are the only constraint values and free texts a
	// tool argument may carry (kiemTaint), and a search whose text is one
	// of them uses its restored form too.
	Slots   hieu.Slots
	TruyVan []hieu.TruyVan
	// ThamChieu are evidence ids of earlier turns, offered to the model as
	// t1, t2, … (never as ids).
	ThamChieu []string
	Nguon     NguonDuLieu
	// Quyen is the permission table; nil is MacDinh.
	Quyen *Quyen
	// Che are permitted tools this turn masks: still declared (the bot's
	// declarations stay one prefix for the implicit cache), never allowed
	// on a step and refused if called anyway («che, không gỡ»). The group
	// masks draft_poll until a poll draft has a card kind (design 03 §4.3).
	Che    []Ten
	SoCai  *SoCai
	HanGoi time.Duration

	mu sync.Mutex
	// rieng is the bot's own tool table (ChoNep, ChoNhom); nil offers the
	// common tools only.
	rieng  map[Ten]congCu
	sai    int
	epCuoi bool
	// buoc is the agent step whose tool calls run now (DatBuoc; 0 for the
	// fast path's direct dispatch, which the router decided).
	buoc int
	// chuBuocDau are the free texts of calls made on the first step, written
	// before any tool result was read; idHang the catalogue row ids tools
	// returned this turn (destinations, areas).
	chuBuocDau []string
	idHang     map[string]bool
	// cuoi: the agent's step with function calling off has started; a
	// function call the model returns anyway is refused, never run.
	cuoi bool
	// ngoai: a tool of this turn returned data other than the person's
	// remembered facts (the catalogue's places, other members' outings,
	// catalogue rows, the manual). From then on no tool with a side effect
	// runs this turn: text that came in as data can never be what triggers
	// a write (the owner's rule on prompt injection,
	// docs/architecture/03-ai-engine-hop-dong.md §8).
	ngoai bool
	// nho: a tool returned remembered facts. A remembered fact is data too:
	// no new fact is written after it. Only forget_fact still runs, since
	// forgetting a fact the person asked to forget (router intent
	// hieu.Forget, required whatever was read) needs the alias recall
	// showed, and deleting only ever narrows what is kept.
	nho bool
	// choNho are the memory writes the model asked for, queued in order
	// and committed only by CamKet, after the answer passed every check.
	choNho []thaoTacNho
	cho    map[string]choGhi
	daCo   map[string]map[string]any
	nhap   BanNhap
	loiGoi []LoiGoi
}

// thaoTacNho is one queued memory write: exactly one of ghi or quen.
type thaoTacNho struct {
	ghi  *trinho.SuThatMoi
	quen *trinho.QuenGi
}

// DatBuocCuoi marks the agent's last step (function calling off) as
// started: every tool call from now on is refused (agent.CauHinh.BuocCuoi).
func (bc *BoiCanh) DatBuocCuoi() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.cuoi = true
}

// SoChoNho is how many memory writes are queued.
func (bc *BoiCanh) SoChoNho() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return len(bc.choNho)
}

// CamKet commits the queued memory changes through the memory port. The
// engine calls it once, after the answer was verified and passed the output
// checks; a turn that releases nothing never calls it, so nothing it queued
// is ever written. The queue is emptied either way.
//
// A turn queues at most one new fact (xepNho), and it is written LAST,
// after every forget: a failure withholds the answer, and it can never
// leave a fact written by a turn whose answer was withheld (re-review
// minor 5). What a failure can leave is a forget already done, which only
// ever narrows what is kept.
func (bc *BoiCanh) CamKet(ctx context.Context) error {
	bc.mu.Lock()
	ops := bc.choNho
	bc.choNho = nil
	bc.mu.Unlock()
	if len(ops) == 0 {
		return nil
	}
	if l := bc.laNep(); l != nil {
		return l
	}
	if bc.Nguon.TriNho == nil {
		return &loiTS{loi: LoiNguon}
	}
	var ghi *trinho.SuThatMoi
	for _, op := range ops {
		if op.ghi != nil {
			ghi = op.ghi
			continue
		}
		if _, err := bc.Nguon.TriNho.Quen(ctx, bc.NguoiHoi, *op.quen); err != nil {
			return err
		}
	}
	if ghi != nil {
		if _, err := bc.Nguon.TriNho.Ghi(ctx, bc.NguoiHoi, *ghi); err != nil {
			return err
		}
	}
	return nil
}

// NguonViec is the source of the verifier's items that describe a memory
// change queued this turn (ViecCho): the server's own record, never
// evidence of the ledger.
const NguonViec truyhoi.Nguon = "viec_da_xep"

// ViecCho describes the memory changes queued this turn for the verifier,
// one item each: what will be done once the answer is released (closed
// values of ours) and what it concerns (the person's own words, or the
// alias of the fact to forget). With them the verifier can tell an answer
// that says it will remember or forget exactly that from a promise of an
// action no tool performed, which it still withholds when nothing is
// queued (re-review minor 6).
func (bc *BoiCanh) ViecCho() []truyhoi.BangChung {
	bc.mu.Lock()
	ops := append([]thaoTacNho(nil), bc.choNho...)
	bc.mu.Unlock()
	var out []truyhoi.BangChung
	for i, op := range ops {
		b := truyhoi.BangChung{ID: "viec-" + strconv.Itoa(i+1), Nguon: NguonViec, Truong: map[string]string{}}
		switch {
		case op.ghi != nil:
			b.Truong["viec"] = "se_ghi_nho_khi_tra_loi"
			b.Truong["noi_dung"] = op.ghi.NoiDung
		case op.quen.ID != "":
			b.Truong["viec"] = "se_quen_khi_tra_loi"
			if bi, ok := bc.SoCai.BiDanh(op.quen.ID); ok {
				b.Truong["su_that"] = bi
			}
		default:
			b.Truong["viec"] = "se_quen_khi_tra_loi"
			b.Truong["mo_ta"] = op.quen.MoTa
		}
		out = append(out, b)
	}
	return out
}

// NapTriNho lays the person's recalled facts (personalization) into the
// turn the way a recall tool's result would be: each fact becomes memory
// evidence of the ledger under its alias (f1, f2, …), so the verifier
// judges a sentence built on it against it, and the block the answer reads
// renders them under those aliases with every value datamarked. A
// remembered fact is data: from now on no memory write runs this turn
// (nho; forget_fact alone survives, as after a recall). "" when there is
// no fact.
func (bc *BoiCanh) NapTriNho(ss []trinho.SuThat) string {
	if len(ss) == 0 {
		return ""
	}
	bc.mu.Lock()
	bc.khoiTao()
	bc.nho = true
	bc.mu.Unlock()
	bcs := make([]truyhoi.BangChung, 0, len(ss))
	for _, s := range ss {
		bcs = append(bcs, bangChungSuThat(s))
	}
	bc.SoCai.Ghi(RecallMemory, bcs)
	var ghi []truyhoi.BangChung
	var bis []string
	for _, b := range bcs {
		if bi, ok := bc.SoCai.BiDanh(b.ID); ok {
			ghi, bis = append(ghi, b), append(bis, bi)
		}
	}
	if len(ghi) == 0 {
		return ""
	}
	return cautruc.KhoiBangChung(prompts.TriNho, ghi, func(i int, _ truyhoi.BangChung) string { return bis[i] })
}

// xepNho queues one memory change. A second new fact in the same turn is
// refused (the person's message gives one span; CamKet writes it last so a
// failure never leaves half a turn's writes). Checked under the lock: two
// calls of one agent step run concurrently.
func (bc *BoiCanh) xepNho(op thaoTacNho) *loiTS {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if op.ghi != nil {
		for _, c := range bc.choNho {
			if c.ghi != nil {
				return &loiTS{loi: KhongDuocPhep, truong: "noi_dung", yeuCau: "one fact per turn, and one is already queued"}
			}
		}
	}
	bc.choNho = append(bc.choNho, op)
	return nil
}

// LoiGoi is one refused or failed tool call, for the turn's record: the
// tool name as the model wrote it and the closed code. No arguments.
type LoiGoi struct {
	Ten string
	Loi LoiTool
}

// HanGoiMacDinh bounds one tool call.
const HanGoiMacDinh = 3 * time.Second

func (bc *BoiCanh) quyen() *Quyen {
	if bc.Quyen == nil {
		return MacDinh
	}
	return bc.Quyen
}

func (bc *BoiCanh) khoiTao() {
	if bc.cho == nil {
		bc.cho = map[string]choGhi{}
		bc.daCo = map[string]map[string]any{}
	}
	if bc.SoCai == nil {
		bc.SoCai = MoiSoCai(bc.Bot)
	}
}

// DuocPhep is the toolset of this turn: the bot's permitted tools, only the
// read ones under proceed_restricted, and a memory write only when the
// router read the person asking for it (BoiCanh.YDinh).
func (bc *BoiCanh) DuocPhep() []Ten {
	var out []Ten
	for _, t := range bc.quyenPhong(bc.HanChe) {
		if y, ghi := yDinhGhi[t]; ghi && !coYDinh(bc.YDinh, y) {
			continue
		}
		if coTen(bc.Che, t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// quyenPhong is the bot's granted toolset for this room: a couple's turn
// (Doi) adds the couple's own tools, every other room never has them.
func (bc *BoiCanh) quyenPhong(hanChe bool) []Ten {
	if bc.Doi {
		return bc.quyen().DuocPhepDoi(bc.Bot, hanChe)
	}
	return bc.quyen().DuocPhep(bc.Bot, hanChe)
}

func coTen(ts []Ten, t Ten) bool {
	for _, x := range ts {
		if x == t {
			return true
		}
	}
	return false
}

// yDinhGhi is the intent each memory write needs in the router's output.
var yDinhGhi = map[Ten]hieu.YDinh{RememberFact: hieu.Remember, ForgetFact: hieu.Forget}

func coYDinh(ys []hieu.YDinh, y hieu.YDinh) bool {
	for _, x := range ys {
		if x == y {
			return true
		}
	}
	return false
}

// EpTraLoi reports whether the next model step must be the final answer
// (function calling off): the repair was spent or the tool budget is.
func (bc *BoiCanh) EpTraLoi() bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.epCuoi
}

// Nhap is a copy of the drafts the turn's tools built.
func (bc *BoiCanh) Nhap() BanNhap {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.nhap.sao()
}

// CacLoiGoi are the turn's refused or failed tool calls, in order.
func (bc *BoiCanh) CacLoiGoi() []LoiGoi {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return append([]LoiGoi(nil), bc.loiGoi...)
}

// loiTS is a refusal of a call's arguments: the closed code, and for the
// model to correct itself the argument that failed and a fixed requirement
// sentence of ours. Neither carries the value the model sent.
type loiTS struct {
	loi     LoiTool
	truong  string
	yeuCau  string
	sua     bool // counts against MaxSua
	ketThuc bool // the turn must answer now
}

func thamSoSai(truong, yeuCau string) *loiTS {
	return &loiTS{loi: ThamSoSai, truong: truong, yeuCau: yeuCau, sua: true}
}

// tuChoi renders a refusal and records it.
func (bc *BoiCanh) tuChoi(ten string, l *loiTS) map[string]any {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if l.sua {
		bc.sai++
		if bc.sai > MaxSua {
			l.ketThuc = true
		}
	}
	if l.ketThuc {
		bc.epCuoi = true
	}
	bc.loiGoi = append(bc.loiGoi, LoiGoi{Ten: ten, Loi: l.loi})
	out := map[string]any{"loi": string(l.loi)}
	if l.truong != "" {
		out["truong"] = l.truong
	}
	if l.yeuCau != "" {
		out["yeu_cau"] = l.yeuCau
	}
	if l.ketThuc {
		out["tra_loi_ngay"] = true
	}
	return out
}

// khoa is a call's canonical key: its name and its arguments as sorted
// JSON (encoding/json sorts map keys).
func khoa(ten string, args map[string]any) (string, []byte) {
	raw, err := json.Marshal(args)
	if err != nil || args == nil {
		raw = []byte("{}")
	}
	return ten + "\x00" + string(raw), raw
}

// truoc is everything before a tool runs: the name is a registered tool the
// bot may call this turn; the arguments pass the schema and the turn's
// membership checks; an identical call is answered from its first result;
// the call fits the budget. It returns the refusal or cached answer, or nil
// to let the tool run.
func (bc *BoiCanh) truoc(ten string, args map[string]any) map[string]any {
	bc.mu.Lock()
	bc.khoiTao()
	bc.mu.Unlock()
	t, err := Tens.Parse(ten)
	if err != nil {
		return bc.tuChoi(ten, &loiTS{loi: KhongDuocPhep, sua: true})
	}
	coTrongBo := false
	for _, d := range bc.DuocPhep() {
		coTrongBo = coTrongBo || d == t
	}
	if !coTrongBo {
		return bc.tuChoi(ten, &loiTS{loi: KhongDuocPhep, sua: true})
	}
	cc, ok := bc.congCu(t)
	if !ok {
		return bc.tuChoi(ten, &loiTS{loi: KhongDuocPhep, sua: true})
	}
	bc.mu.Lock()
	cuoi, ngoai, nho := bc.cuoi || bc.epCuoi, bc.ngoai, bc.nho
	bc.mu.Unlock()
	if cuoi {
		// The step that must answer, or a turn told to answer now: no tool
		// runs, whatever the model returned.
		return bc.tuChoi(ten, &loiTS{loi: KhongDuocPhep, ketThuc: true})
	}
	if m, _ := Tra(t); (m.Lop == TriNho || m.Lop == Nhac) && (ngoai || (nho && t != ForgetFact)) {
		return bc.tuChoi(ten, &loiTS{loi: KhongDuocPhep})
	}
	k, raw := khoa(ten, args)
	r, _ := LuocDoJSON(t)
	sao := banSao(args)
	if err := r.Validate(sao); err != nil {
		return bc.tuChoi(ten, thamSoSai("", "arguments must match the declared schema"))
	}
	if l := bc.kiemTaint(t, sao); l != nil {
		return bc.tuChoi(ten, l)
	}
	if _, l := cc.kiem(bc, raw); l != nil {
		return bc.tuChoi(ten, l)
	}
	if bc.SoCai.LapLai(t, raw) {
		bc.mu.Lock()
		defer bc.mu.Unlock()
		out := map[string]any{"lap_lai": true}
		for kk, v := range bc.daCo[k] {
			out[kk] = v
		}
		return out
	}
	if err := bc.SoCai.Giu(t); err != nil {
		return bc.tuChoi(ten, &loiTS{loi: HetLuotGoi, ketThuc: true})
	}
	return nil
}

// banSao is args with every value as JSON decodes it, so the validator
// sees float64 numbers whatever Go type a caller built the map with.
func banSao(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	raw, _ := json.Marshal(args)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

// ketQuaTho is what a tool produced before it is recorded: the evidence, in
// order, and our own structured fields (counts, enums, flags).
type ketQuaTho struct {
	bangChung []truyhoi.BangChung
	them      map[string]any
	// coDuLieu renders the data block even when no evidence came back, so
	// an empty search reads as an empty list, not as a missing one.
	coDuLieu bool
	// hang are catalogue rows that are not evidence (destinations, areas):
	// public ids the model passes back as arguments, rendered as they are.
	hang []map[string]string
}

type choGhi struct {
	ten Ten
	kq  ketQuaTho
}

// gac parks a tool's raw result under its call id until AfterTool records
// it.
func (bc *BoiCanh) gac(goiID string, t Ten, kq ketQuaTho) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.khoiTao()
	bc.cho[goiID] = choGhi{ten: t, kq: kq}
}

// sau records a parked result: its evidence goes into the ledger and it is
// rendered for the model, evidence under aliases only, inside a data block.
// A call id with nothing parked is not ours to render: nil.
func (bc *BoiCanh) sau(goiID string, args map[string]any) map[string]any {
	bc.mu.Lock()
	c, ok := bc.cho[goiID]
	delete(bc.cho, goiID)
	bc.mu.Unlock()
	if !ok {
		return nil
	}
	return bc.ghi(c.ten, args, c.kq)
}

// danhDauTruong is one field value as a model reads it in a tool result:
// on one line, cut at cautruc.MaxRuneTruong runes, datamarked. The ledger
// keeps the value as it came; only what the model reads is marked.
func danhDauTruong(v string) string {
	v = strings.Join(strings.Fields(v), " ")
	if utf8.RuneCountInString(v) > cautruc.MaxRuneTruong {
		v = string([]rune(v)[:cautruc.MaxRuneTruong])
	}
	return prompts.DanhDau(v)
}

// jsonTho is v as JSON with <, > and & left as they are, so the block's
// own escape (prompts.BocDuLieu turns them full-width) is the one that
// applies: a closing tag in the data reads as text, never as \u003c the
// model could decode back into markup.
func jsonTho(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(buf.String(), "\n")
}

func (bc *BoiCanh) ghi(t Ten, args map[string]any, kq ketQuaTho) map[string]any {
	bc.SoCai.Ghi(t, kq.bangChung)
	// Any data a tool returns is text the person did not write this turn:
	// the writes stop here (only forget_fact survives remembered facts).
	ngoai, nho := kq.hang != nil, false
	for _, b := range kq.bangChung {
		if b.Nguon == truyhoi.Memory {
			nho = true
		} else {
			ngoai = true
		}
	}
	bc.mu.Lock()
	bc.ngoai = bc.ngoai || ngoai
	bc.nho = bc.nho || nho
	for _, h := range kq.hang {
		if id := h["id"]; id != "" {
			if bc.idHang == nil {
				bc.idHang = map[string]bool{}
			}
			bc.idHang[id] = true
		}
	}
	bc.mu.Unlock()
	out := map[string]any{}
	for k, v := range kq.them {
		out[k] = v
	}
	if len(kq.bangChung) > 0 || kq.coDuLieu {
		items := make([]map[string]string, 0, len(kq.bangChung))
		for _, b := range kq.bangChung {
			bi, ok := bc.SoCai.BiDanh(b.ID)
			if !ok {
				continue
			}
			m := map[string]string{}
			for k, v := range b.Truong {
				m[k] = danhDauTruong(v)
			}
			m["id"] = bi
			items = append(items, m)
		}
		out["du_lieu"] = prompts.BocDuLieu(prompts.KetQuaCongCu, jsonTho(items))
		out["so_muc"] = len(items)
	} else if kq.hang != nil {
		hang := make([]map[string]string, 0, len(kq.hang))
		for _, h := range kq.hang {
			m := map[string]string{}
			for k, v := range h {
				m[k] = danhDauTruong(v)
			}
			hang = append(hang, m)
		}
		out["du_lieu"] = prompts.BocDuLieu(prompts.KetQuaCongCu, jsonTho(hang))
		out["so_muc"] = len(kq.hang)
	}
	k, _ := khoa(string(t), args)
	bc.mu.Lock()
	bc.daCo[k] = out
	bc.mu.Unlock()
	return out
}

// loiChay maps a failed tool run to its closed code, records it, and
// renders it. The error's text never reaches the model.
func (bc *BoiCanh) loiChay(ten string, err error) map[string]any {
	var l *loiTS
	switch {
	case errors.As(err, &l):
	case errors.Is(err, context.DeadlineExceeded):
		l = &loiTS{loi: HetHan}
	case errors.Is(err, errKhongThay):
		l = &loiTS{loi: KhongThay}
	default:
		l = &loiTS{loi: LoiNguon}
	}
	return bc.tuChoi(ten, l)
}

func (l *loiTS) Error() string { return "tools: " + string(l.loi) + " " + l.truong }

var errKhongThay = errors.New("tools: not found")

// Goi runs one tool call through the same checks, budget and ledger as an
// agent step does: the fast path's direct dispatch. The answer is what the
// model would have read.
func (bc *BoiCanh) Goi(ctx context.Context, t Ten, args map[string]any) map[string]any {
	if r := bc.truoc(string(t), args); r != nil {
		return r
	}
	_, raw := khoa(string(t), args)
	cc, _ := bc.congCu(t)
	a, l := cc.kiem(bc, raw)
	if l != nil {
		return bc.tuChoi(string(t), l)
	}
	kq, err := bc.chay(ctx, cc, a)
	if err != nil {
		return bc.loiChay(string(t), err)
	}
	return bc.ghi(t, args, kq)
}

func (bc *BoiCanh) chay(ctx context.Context, cc congCu, a any) (ketQuaTho, error) {
	han := bc.HanGoi
	if han <= 0 {
		han = HanGoiMacDinh
	}
	ctx, cancel := context.WithTimeout(ctx, han)
	defer cancel()
	return cc.chay(ctx, bc, a)
}

// giaiBiDanh resolves an evidence alias of this turn to its id, requiring
// the evidence to come from nguon.
func (bc *BoiCanh) giaiBiDanh(bi string, nguon truyhoi.Nguon) (string, bool) {
	id, ok := bc.SoCai.TuBiDanh(bi)
	if !ok {
		return "", false
	}
	b, _, _ := bc.SoCai.Lay(id)
	return id, b.Nguon == nguon
}

// giaiThamChieu resolves t1, t2, … to the earlier turns' evidence ids.
func (bc *BoiCanh) giaiThamChieu(bi string) (string, bool) {
	if len(bi) < 2 || bi[0] != 't' {
		return "", false
	}
	n, err := strconv.Atoi(bi[1:])
	if err != nil || n < 1 || n > len(bc.ThamChieu) || strconv.Itoa(n) != bi[1:] {
		return "", false
	}
	return bc.ThamChieu[n-1], true
}

// BiDanhThamChieu is the alias of each earlier-turn evidence id, for the
// short-term block of the prompt.
func (bc *BoiCanh) BiDanhThamChieu() map[string]string {
	out := map[string]string{}
	for i, id := range bc.ThamChieu {
		out[id] = "t" + strconv.Itoa(i+1)
	}
	return out
}

func coTrong(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// RangBuocTuRouter turns the router's checked slots into the turn's hard
// constraints and soft preferences, by exact arithmetic on what the model
// wrote: its ISO date (or, with a time and no date, the turn's own date on
// Vietnam's clock) at the times it gave. A window with both ends becomes a
// window the place must be open at some point of (Cung.MoTrong; an end at
// or before the start is the next day's); a time with no end becomes an
// instant (Cung.MoLuc). A date with no time sets neither: «open at some
// point that day» is not a constraint Go invents.
func RangBuocTuRouter(s hieu.Slots, luc time.Time) (truyhoi.Cung, truyhoi.Mem, error) {
	c := truyhoi.Cung{DiemDenID: s.DiemDenID, NganSachVND: s.NganSachVND}
	c.DiUng = append([]string(nil), s.DiUng...)
	c.AnKieng = append([]string(nil), s.AnKieng...)
	if len(c.DiUng) == 0 {
		c.DiUng = nil
	}
	if len(c.AnKieng) == 0 {
		c.AnKieng = nil
	}
	if k := s.KhungGio; k != nil {
		tu, err := moLuc(s.NgayISO, k.Tu, luc)
		if err != nil {
			return truyhoi.Cung{}, truyhoi.Mem{}, err
		}
		if k.Den == "" {
			c.MoLuc = &tu
		} else {
			den, err := moLuc(s.NgayISO, k.Den, luc)
			if err != nil {
				return truyhoi.Cung{}, truyhoi.Mem{}, err
			}
			if !den.After(tu) {
				den = den.AddDate(0, 0, 1)
			}
			c.MoTrong = &truyhoi.KhungMo{Tu: tu, Den: den}
		}
	}
	m := truyhoi.Mem{LoaiCho: append([]string(nil), s.LoaiCho...), KhiChat: append([]string(nil), s.KhiChat...)}
	if len(m.LoaiCho) == 0 {
		m.LoaiCho = nil
	}
	if len(m.KhiChat) == 0 {
		m.KhiChat = nil
	}
	return c, m, nil
}

// moLuc is the instant of ngayISO (or luc's date on Vietnam's clock when
// empty) at gio, on Vietnam's clock.
func moLuc(ngayISO, gio string, luc time.Time) (time.Time, error) {
	if ngayISO == "" {
		ngayISO = luc.In(gioVN).Format(time.DateOnly)
	}
	if !hieu.NgayHopLe(ngayISO) || !hieu.GioHopLe(gio) {
		return time.Time{}, fmt.Errorf("tools: date %q or time %q", ngayISO, gio)
	}
	return time.ParseInLocation(time.DateOnly+" 15:04", ngayISO+" "+gio, gioVN)
}

// sapXep returns ks sorted, for stable output.
func sapXep(ks []string) []string {
	out := append([]string(nil), ks...)
	sort.Strings(out)
	return out
}
