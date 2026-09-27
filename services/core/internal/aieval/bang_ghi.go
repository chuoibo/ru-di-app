package aieval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/rerank"
)

// The cassette (design 06 §2 bang_ghi.go, §3.4). In mode ghi it forwards
// every model call to the model it wraps and records the request's key and
// every chunk the call yielded, errors and usage included; in mode phat-lai it
// serves those chunks back and has no model to forward to at all. A request it
// has no recording for is ErrBangLech -- red, never a network call.
//
// Key (§3.4): (scope, RequestHash, occurrence). The scope is one run of one
// case, "<case_id>@<lap>": the same request may be asked once per lap and
// answered differently each time, and a retry asks the same request again
// inside one run. The occurrence counts, from 1, how many times that hash has
// been asked in that scope. RequestHash is the sha256 of KhoaYeuCau: the
// canonical request of llm.Canon (model, contents, the generation config with
// its system instruction and tool declarations, the declared tool names)
// with the fields in BoKhoiKhoa removed.

// PhienBanBang is the cassette file's format version.
const PhienBanBang = 1

// A cassette's modes.
const (
	CheDoGhi     = "ghi"
	CheDoPhatLai = "phat-lai"
)

// KiemBangLech is the check a replay fails when the cassette does not hold
// what the run asks (design 06 §11): a key it has no recording for, the same
// key asked in the other streaming mode, or a recorded call the replay never
// asked for.
const KiemBangLech = "bang_lech"

// ErrBangLech is what a replayed call answers when the cassette holds no
// recording for it. It is never followed by a network call: in phat-lai the
// cassette has no model to forward to.
var ErrBangLech = errors.New("aieval: bang_lech: the cassette holds no recorded answer for this request")

// BoKhoiKhoa names every field of the canonical request the key leaves out,
// and why. Nothing else is removed.
var BoKhoiKhoa = []string{
	// llm.Canon already drops it: headers the SDK and ADK add, base URL and
	// timeout are the wire, not the request.
	"config.httpOptions",
	// ADK draws a fresh "adk-<uuid>" for a call the model left without an
	// id, so the same turn carries a different id on every run.
	"contents[].parts[].functionCall.id",
	"contents[].parts[].functionResponse.id",
}

// TepBang is a cassette file.
type TepBang struct {
	PhienBan int `json:"phien_ban"`
	// Model is the name the wrapped model answered to; Backend the backend
	// it told ADK it was (a replay answers the same, so ADK prepares the
	// same request).
	Model   string `json:"model"`
	Backend string `json:"backend"`
	// NhungModel and NhungDims describe the wrapped embedder, when there was
	// one.
	NhungModel string `json:"nhung_model,omitempty"`
	NhungDims  int    `json:"nhung_dims,omitempty"`
	// XepLaiModel names the wrapped reranker, when there was one; a replay
	// wires a reranker exactly when the recording had one.
	XepLaiModel string `json:"xep_lai_model,omitempty"`
	// BoKhoiKhoa is the list above as it was when the file was written.
	BoKhoiKhoa []string    `json:"bo_khoi_khoa"`
	Goi        []GoiGhi    `json:"goi"`
	Nhung      []NhungGhi  `json:"nhung"`
	XepLai     []XepLaiGhi `json:"xep_lai"`
}

// GoiGhi is one recorded model call.
type GoiGhi struct {
	Pham    string `json:"pham"`
	ReqHash string `json:"req_hash"`
	Lan     int    `json:"lan"`
	Stream  bool   `json:"stream"`
	// YeuCau is the keyed request, compact, for whoever reads a bang_lech.
	YeuCau  json.RawMessage `json:"yeu_cau"`
	PhanHoi []ChunkGhi      `json:"phan_hoi"`
	// MsChunkDau and MsTong are how long the wrapped model took to its first
	// chunk and to its last (the trace of design 06 §2 vet.go, per call).
	MsChunkDau int `json:"ms_chunk_dau"`
	MsTong     int `json:"ms_tong"`
}

// ChunkGhi is one (response, error) pair a call yielded, in order.
type ChunkGhi struct {
	Resp json.RawMessage `json:"resp,omitempty"`
	Loi  *LoiGhi         `json:"loi,omitempty"`
}

// LoiGhi is a recorded error, by class. A provider's message is never kept:
// it can echo the question or the key (llm.PhanLoai drops it for the same
// reason).
type LoiGhi struct {
	Loai   string `json:"loai"`
	Code   int    `json:"code,omitempty"`
	Status string `json:"status,omitempty"`
}

// Classes of LoiGhi.
const (
	loiAPI          = "api"
	loiKhongUngVien = "khong_ung_vien"
	loiHetGio       = "het_gio"
	loiHuy          = "huy"
	loiTranGoi      = "tran_goi"
	loiKhac         = "khac"
)

// errKhacGhi replays an error recorded without a class of its own.
var errKhacGhi = errors.New("aieval: a provider error, recorded without its message")

// NhungGhi is one recorded embedding call.
type NhungGhi struct {
	Pham    string          `json:"pham"`
	ReqHash string          `json:"req_hash"`
	Lan     int             `json:"lan"`
	YeuCau  json.RawMessage `json:"yeu_cau"`
	Vec     [][]float32     `json:"vec,omitempty"`
	Loi     *LoiGhi         `json:"loi,omitempty"`
	MsTong  int             `json:"ms_tong"`
}

// Bang is one run's cassette: the recordings, the occurrence counters, and
// in mode ghi the model and embedder it forwards to. Safe for concurrent use.
type Bang struct {
	cheDo       string
	inner       model.LLM
	nhungInner  nhung.Nhung
	xepLaiInner truyhoi.Reranker
	now         func() time.Time

	mu        sync.Mutex
	tep       TepBang
	lan       map[string]int
	lanN      map[string]int
	lanX      map[string]int
	chiMuc    map[string]int
	chiMucN   map[string]int
	chiMucX   map[string]int
	daDung    map[int]bool
	daDungN   map[int]bool
	daDungX   map[int]bool
	lech      []string
	soGoiRa   int
	soNhungRa int64
}

// NoiGhi is what a recording cassette forwards to: the model (required),
// and the embedder and the reranker when the run has them.
type NoiGhi struct {
	MoHinh model.LLM
	Nhung  nhung.Nhung
	// XepLai and XepLaiModel are the reranker and the model it serves.
	XepLai      truyhoi.Reranker
	XepLaiModel string
}

// NewBangGhi records every call to what n names. now times each call; nil
// is time.Now.
func NewBangGhi(n NoiGhi, now func() time.Time) *Bang {
	if now == nil {
		now = time.Now
	}
	b := &Bang{cheDo: CheDoGhi, inner: n.MoHinh, nhungInner: n.Nhung, xepLaiInner: n.XepLai, now: now,
		lan: map[string]int{}, lanN: map[string]int{}, lanX: map[string]int{}}
	b.tep = TepBang{PhienBan: PhienBanBang, Model: n.MoHinh.Name(), Backend: backendCua(n.MoHinh).String(),
		BoKhoiKhoa: append([]string(nil), BoKhoiKhoa...), Goi: []GoiGhi{}, Nhung: []NhungGhi{}, XepLai: []XepLaiGhi{}}
	if n.Nhung != nil {
		b.tep.NhungModel, b.tep.NhungDims = n.Nhung.Model(), n.Nhung.Dims()
	}
	if n.XepLai != nil {
		b.tep.XepLaiModel = n.XepLaiModel
		if b.tep.XepLaiModel == "" {
			b.tep.XepLaiModel = "khong-ro"
		}
	}
	return b
}

// NewBangPhatLai serves the recordings of t and forwards nothing.
func NewBangPhatLai(t TepBang) (*Bang, error) {
	if t.PhienBan != PhienBanBang {
		return nil, fmt.Errorf("aieval: cassette format %d, this binary reads %d", t.PhienBan, PhienBanBang)
	}
	if fmt.Sprint(t.BoKhoiKhoa) != fmt.Sprint(BoKhoiKhoa) {
		return nil, fmt.Errorf("aieval: the cassette was keyed without %v, this binary keys without %v", t.BoKhoiKhoa, BoKhoiKhoa)
	}
	if _, ok := backendTheoTen[t.Backend]; !ok {
		return nil, fmt.Errorf("aieval: cassette backend %q unknown", t.Backend)
	}
	b := &Bang{cheDo: CheDoPhatLai, now: time.Now, tep: t, lan: map[string]int{}, lanN: map[string]int{}, lanX: map[string]int{},
		chiMuc: map[string]int{}, chiMucN: map[string]int{}, chiMucX: map[string]int{},
		daDung: map[int]bool{}, daDungN: map[int]bool{}, daDungX: map[int]bool{}}
	for i, g := range t.Goi {
		k := khoaDay(g.Pham, g.ReqHash, g.Lan)
		if _, dup := b.chiMuc[k]; dup {
			return nil, fmt.Errorf("aieval: cassette holds two recordings of %s", k)
		}
		b.chiMuc[k] = i
	}
	for i, g := range t.Nhung {
		k := khoaDay(g.Pham, g.ReqHash, g.Lan)
		if _, dup := b.chiMucN[k]; dup {
			return nil, fmt.Errorf("aieval: cassette holds two embedding recordings of %s", k)
		}
		b.chiMucN[k] = i
	}
	for i, g := range t.XepLai {
		k := khoaDay(g.Pham, g.ReqHash, g.Lan)
		if _, dup := b.chiMucX[k]; dup {
			return nil, fmt.Errorf("aieval: cassette holds two rerank recordings of %s", k)
		}
		b.chiMucX[k] = i
	}
	return b, nil
}

// CheDo is the cassette's mode.
func (b *Bang) CheDo() string { return b.cheDo }

// Tep is a copy of the recordings, for writing out.
func (b *Bang) Tep() TepBang {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.tep
	t.Goi = append([]GoiGhi{}, b.tep.Goi...)
	t.Nhung = append([]NhungGhi{}, b.tep.Nhung...)
	t.XepLai = append([]XepLaiGhi{}, b.tep.XepLai...)
	return t
}

// SoGoiRa is how many calls the cassette forwarded to what it wraps. It is
// 0 in phat-lai by construction: there is nothing to forward to.
func (b *Bang) SoGoiRa() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.soGoiRa
}

// SoTuBang is how many calls a replay answered from the recordings.
func (b *Bang) SoTuBang() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.daDung) + len(b.daDungN) + len(b.daDungX)
}

// Lech lists every replayed call the cassette could not answer.
func (b *Bang) Lech() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.lech...)
}

// ChuaDung lists, in phat-lai, every recording no call asked for. A replay
// of the whole run that leaves one unasked did not make the calls the
// recorded run made: that is a bang_lech too.
func (b *Bang) ChuaDung() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	if b.cheDo != CheDoPhatLai {
		return out
	}
	for i, g := range b.tep.Goi {
		if !b.daDung[i] {
			out = append(out, "model "+khoaDoc(g.Pham, g.ReqHash, g.Lan))
		}
	}
	for i, g := range b.tep.Nhung {
		if !b.daDungN[i] {
			out = append(out, "nhúng "+khoaDoc(g.Pham, g.ReqHash, g.Lan))
		}
	}
	for i, g := range b.tep.XepLai {
		if !b.daDungX[i] {
			out = append(out, "xếp lại "+khoaDoc(g.Pham, g.ReqHash, g.Lan))
		}
	}
	return out
}

// LLM is the cassette's model for one run of one case.
func (b *Bang) LLM(caID string, lap int) *CassetteLLM {
	return &CassetteLLM{bang: b, pham: PhamCua(caID, lap)}
}

// PhamCua is the scope of one run of one case. Case ids cannot hold '@'.
func PhamCua(caID string, lap int) string { return caID + "@" + strconv.Itoa(lap) }

func khoaDay(pham, hash string, lan int) string {
	return pham + "\x00" + hash + "\x00" + strconv.Itoa(lan)
}

func khoaDoc(pham, hash string, lan int) string {
	return fmt.Sprintf("%s %s lần %d", pham, hash[:min(12, len(hash))], lan)
}

// demLan counts one more asking of hash in pham and returns its occurrence.
func (b *Bang) demLan(m map[string]int, pham, hash string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := pham + "\x00" + hash
	m[k]++
	return m[k]
}

// KhoaYeuCau is a request's key: its RequestHash, and the keyed request as
// compact JSON with sorted keys.
func KhoaYeuCau(req *model.LLMRequest) (string, []byte, error) {
	canon, err := llm.Canon(req)
	if err != nil {
		return "", nil, err
	}
	return khoaTuCanon(canon)
}

func khoaTuCanon(canon []byte) (string, []byte, error) {
	dec := json.NewDecoder(bytes.NewReader(canon))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", nil, err
	}
	boTruongDoi(v)
	raw, err := jsonGon(v)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}

// boTruongDoi removes the function call and function response ids of every
// part of every content (BoKhoiKhoa); httpOptions is already gone (Canon).
func boTruongDoi(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	contents, _ := m["contents"].([]any)
	for _, c := range contents {
		cm, _ := c.(map[string]any)
		parts, _ := cm["parts"].([]any)
		for _, p := range parts {
			pm, _ := p.(map[string]any)
			for _, f := range []string{"functionCall", "functionResponse"} {
				if fm, ok := pm[f].(map[string]any); ok {
					delete(fm, "id")
				}
			}
		}
	}
}

// jsonGon is compact JSON with sorted keys and no HTML escaping.
func jsonGon(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// googleLLM is what ADK asks a model for its backend.
type googleLLM interface {
	GetGoogleLLMVariant() genai.Backend
}

func backendCua(m any) genai.Backend {
	if v, ok := m.(googleLLM); ok {
		return v.GetGoogleLLMVariant()
	}
	return genai.BackendUnspecified
}

var backendTheoTen = map[string]genai.Backend{
	genai.BackendUnspecified.String(): genai.BackendUnspecified,
	genai.BackendGeminiAPI.String():   genai.BackendGeminiAPI,
	genai.BackendVertexAI.String():    genai.BackendVertexAI,
}

// banSao is req as the wrapped model may change it without the change
// reaching the caller: ADK's gemini model appends a content when the last is
// not the user's and writes its headers into the config's HTTP options, genai
// writes role "user" into the system instruction it was handed, and the
// per-turn counter hands the same request to every retry. Without the copy a
// retry would be keyed on what the provider layer did to the first try,
// which a replay (no provider layer) never does
// (TestGhiRoiPhatLaiTrungDiem, cases 11 and 13, go red without it).
func banSao(req *model.LLMRequest) *model.LLMRequest {
	out := *req
	out.Contents = make([]*genai.Content, len(req.Contents))
	for i, c := range req.Contents {
		out.Contents[i] = banSaoContent(c)
	}
	if req.Config != nil {
		cfg := *req.Config
		cfg.SystemInstruction = banSaoContent(cfg.SystemInstruction)
		if cfg.HTTPOptions != nil {
			h := *cfg.HTTPOptions
			h.Headers = h.Headers.Clone()
			cfg.HTTPOptions = &h
		}
		out.Config = &cfg
	}
	return &out
}

func banSaoContent(c *genai.Content) *genai.Content {
	if c == nil {
		return nil
	}
	x := *c
	x.Parts = make([]*genai.Part, len(c.Parts))
	for i, p := range c.Parts {
		if p != nil {
			pp := *p
			p = &pp
		}
		x.Parts[i] = p
	}
	return &x
}

func ghiLoi(err error) *LoiGhi {
	var v genai.APIError
	var p *genai.APIError
	switch {
	case errors.As(err, &v):
		return &LoiGhi{Loai: loiAPI, Code: v.Code, Status: v.Status}
	case errors.As(err, &p) && p != nil:
		return &LoiGhi{Loai: loiAPI, Code: p.Code, Status: p.Status}
	case errors.Is(err, llm.ErrKhongUngVien):
		return &LoiGhi{Loai: loiKhongUngVien}
	case errors.Is(err, ErrTranGoi):
		return &LoiGhi{Loai: loiTranGoi}
	case errors.Is(err, context.DeadlineExceeded):
		return &LoiGhi{Loai: loiHetGio}
	case errors.Is(err, context.Canceled):
		return &LoiGhi{Loai: loiHuy}
	}
	return &LoiGhi{Loai: loiKhac}
}

// phatLoi is the error a recorded class replays as: the same value the
// engine and the per-turn counter test for (a 429 is retried again, a
// blocked prompt is the provider's safety refusal again).
func phatLoi(l *LoiGhi) error {
	switch l.Loai {
	case loiAPI:
		return genai.APIError{Code: l.Code, Status: l.Status}
	case loiKhongUngVien:
		return llm.ErrKhongUngVien
	case loiTranGoi:
		return ErrTranGoi
	case loiHetGio:
		return context.DeadlineExceeded
	case loiHuy:
		return context.Canceled
	}
	return errKhacGhi
}

// CassetteLLM is the cassette as the model of one run of one case: an ADK
// model.LLM. Like llm.Stub it keeps the canonical form of every request it
// was handed, which the invariants read.
type CassetteLLM struct {
	bang *Bang
	pham string

	mu   sync.Mutex
	yeu  [][]byte
	dap  []string
	lech []string
}

var _ model.LLM = (*CassetteLLM)(nil)

// DapAn returns, per request in order, the text the model answered it with
// (thoughts left out; "" for a call that ended in an error): in the model
// modes the scorer reads the place tokens an answer step wrote from here,
// as it reads them from the script in T1.
func (c *CassetteLLM) DapAn() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.dap...)
}

// chuCua is a response's text, thoughts left out.
func chuCua(r *model.LLMResponse) string {
	if r == nil || r.Content == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Content.Parts {
		if p != nil && !p.Thought {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// ghiDap wraps yield so the call's answer text lands in slot i: the last
// complete response's text, or the partial chunks joined when no complete
// one came.
func (c *CassetteLLM) ghiDap(i int, yield func(*model.LLMResponse, error) bool) (func(*model.LLMResponse, error) bool, func()) {
	var tron, manh strings.Builder
	coTron := false
	return func(r *model.LLMResponse, err error) bool {
			if r != nil {
				if r.Partial {
					manh.WriteString(chuCua(r))
				} else {
					tron.Reset()
					tron.WriteString(chuCua(r))
					coTron = true
				}
			}
			return yield(r, err)
		}, func() {
			t := manh.String()
			if coTron {
				t = tron.String()
			}
			c.mu.Lock()
			c.dap[i] = t
			c.mu.Unlock()
		}
}

// Name is the recorded model's name.
func (c *CassetteLLM) Name() string { return c.bang.tep.Model }

// GetGoogleLLMVariant is the backend the recorded model reported, in both
// modes, so ADK prepares the request of a replay as it prepared the
// recorded one. Like llm.Dem the cassette has no Client method: a live
// session opened from it would pass neither the recording nor the ceiling.
func (c *CassetteLLM) GetGoogleLLMVariant() genai.Backend { return backendTheoTen[c.bang.tep.Backend] }

// YeuCau returns the canonical JSON of every request, in order.
func (c *CassetteLLM) YeuCau() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte(nil), c.yeu...)
}

// Lech lists the calls of this run the cassette could not answer.
func (c *CassetteLLM) Lech() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lech...)
}

// GenerateContent records (ghi) or replays (phat-lai) one call.
func (c *CassetteLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		canon, err := llm.Canon(req)
		if err != nil {
			yield(nil, err)
			return
		}
		c.mu.Lock()
		c.yeu = append(c.yeu, canon)
		c.dap = append(c.dap, "")
		i := len(c.dap) - 1
		c.mu.Unlock()
		yield, xong := c.ghiDap(i, yield)
		defer xong()
		hash, khoa, err := khoaTuCanon(canon)
		if err != nil {
			yield(nil, err)
			return
		}
		lan := c.bang.demLan(c.bang.lan, c.pham, hash)
		if c.bang.cheDo == CheDoPhatLai {
			c.phatLai(ctx, hash, lan, stream, yield)
			return
		}
		c.ghi(ctx, req, hash, khoa, lan, stream, yield)
	}
}

func (c *CassetteLLM) ghi(ctx context.Context, req *model.LLMRequest, hash string, khoa []byte, lan int, stream bool, yield func(*model.LLMResponse, error) bool) {
	b := c.bang
	g := GoiGhi{Pham: c.pham, ReqHash: hash, Lan: lan, Stream: stream, YeuCau: khoa, PhanHoi: []ChunkGhi{}}
	batDau := b.now()
	defer func() {
		g.MsTong = int(b.now().Sub(batDau) / time.Millisecond)
		b.mu.Lock()
		b.tep.Goi = append(b.tep.Goi, g)
		b.soGoiRa++
		b.mu.Unlock()
	}()
	for resp, err := range b.inner.GenerateContent(ctx, banSao(req), stream) {
		if len(g.PhanHoi) == 0 {
			g.MsChunkDau = int(b.now().Sub(batDau) / time.Millisecond)
		}
		var ch ChunkGhi
		if resp != nil {
			raw, merr := json.Marshal(resp)
			if merr != nil {
				// A response the cassette cannot keep is a cassette that
				// cannot replay this run: say so rather than record a hole.
				err = errors.Join(err, fmt.Errorf("aieval: cannot record a response: %w", merr))
			} else {
				ch.Resp = raw
			}
		}
		if err != nil {
			ch.Loi = ghiLoi(err)
		}
		g.PhanHoi = append(g.PhanHoi, ch)
		if !yield(resp, err) {
			return
		}
	}
}

func (c *CassetteLLM) phatLai(ctx context.Context, hash string, lan int, stream bool, yield func(*model.LLMResponse, error) bool) {
	b := c.bang
	k := khoaDay(c.pham, hash, lan)
	b.mu.Lock()
	i, ok := b.chiMuc[k]
	var g GoiGhi
	if ok {
		g = b.tep.Goi[i]
	}
	lech := ""
	switch {
	case !ok:
		lech = "không có bản ghi cho " + khoaDoc(c.pham, hash, lan)
	case g.Stream != stream:
		lech = fmt.Sprintf("%s được ghi với stream=%v, lần này stream=%v", khoaDoc(c.pham, hash, lan), g.Stream, stream)
	default:
		b.daDung[i] = true
	}
	if lech != "" {
		b.lech = append(b.lech, lech)
	}
	b.mu.Unlock()
	if lech != "" {
		c.mu.Lock()
		c.lech = append(c.lech, lech)
		c.mu.Unlock()
		yield(nil, fmt.Errorf("%w: %s", ErrBangLech, lech))
		return
	}
	for _, ch := range g.PhanHoi {
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		var resp *model.LLMResponse
		if ch.Resp != nil {
			resp = &model.LLMResponse{}
			if err := json.Unmarshal(ch.Resp, resp); err != nil {
				yield(nil, fmt.Errorf("aieval: a recorded response does not decode: %w", err))
				return
			}
		}
		var err error
		if ch.Loi != nil {
			err = phatLoi(ch.Loi)
		}
		if !yield(resp, err) {
			return
		}
	}
}

// Scopes of calls made outside any run of a case: the example bank the
// router is built with is embedded once per process, before the first case.
const PhamChung = "chung"

type khoaPham struct{}

// VoiPham is ctx carrying the scope of one run of one case, which the
// cassette's embedder and reranker key their calls by: both are shared by
// every turn (the router's example bank holds its embedder for the whole
// process), so the scope travels with the call, not with the value.
func VoiPham(ctx context.Context, pham string) context.Context {
	return context.WithValue(ctx, khoaPham{}, pham)
}

// phamTu is the scope ctx carries, PhamChung outside a run.
func phamTu(ctx context.Context) string {
	if p, ok := ctx.Value(khoaPham{}).(string); ok && p != "" {
		return p
	}
	return PhamChung
}

// CassetteNhung is the cassette as the embedder, so the router's example
// choice (and any hybrid retrieval or memory recall wired to it) replays like
// the model does. The key is (scope from ctx, sha256 of {model, dims, kind,
// task, inputs}, occurrence).
type CassetteNhung struct{ bang *Bang }

var _ nhung.Nhung = (*CassetteNhung)(nil)

// Nhung is the cassette's embedder (one per cassette; scoped by ctx).
func (b *Bang) Nhung() *CassetteNhung { return &CassetteNhung{bang: b} }

// Model is the recorded embedder's model.
func (c *CassetteNhung) Model() string { return c.bang.tep.NhungModel }

// Dims is the recorded embedder's dimensionality.
func (c *CassetteNhung) Dims() int { return c.bang.tep.NhungDims }

// SoGoi is the provider requests the cassette forwarded to its embedder: 0
// on replay.
func (c *CassetteNhung) SoGoi() int64 {
	b := c.bang
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.soNhungRa
}

// Nhung records (ghi) or replays (phat-lai) one embedding call.
func (c *CassetteNhung) Nhung(ctx context.Context, texts []string, tv nhung.TacVu) ([][]float32, error) {
	return c.goi(ctx, map[string]any{"loai": "nhung", "tac_vu": string(tv), "texts": texts}, func(n nhung.Nhung) ([][]float32, error) {
		return n.Nhung(ctx, texts, tv)
	})
}

// NhungTaiLieu records (ghi) or replays (phat-lai) one document embedding.
func (c *CassetteNhung) NhungTaiLieu(ctx context.Context, docs []nhung.TaiLieuVao) ([][]float32, error) {
	vao := make([][2]string, len(docs))
	for i, d := range docs {
		vao[i] = [2]string{d.TieuDe, d.NoiDung}
	}
	return c.goi(ctx, map[string]any{"loai": "tai_lieu", "docs": vao}, func(n nhung.Nhung) ([][]float32, error) {
		return n.NhungTaiLieu(ctx, docs)
	})
}

func (c *CassetteNhung) goi(ctx context.Context, yeuCau map[string]any, that func(nhung.Nhung) ([][]float32, error)) ([][]float32, error) {
	b := c.bang
	yeuCau["model"], yeuCau["dims"] = b.tep.NhungModel, b.tep.NhungDims
	khoa, err := jsonGon(yeuCau)
	if err != nil {
		return nil, err
	}
	hash := ShaCua(khoa)
	pham := phamTu(ctx)
	lan := b.demLan(b.lanN, pham, hash)
	if b.cheDo == CheDoPhatLai {
		k := khoaDay(pham, hash, lan)
		b.mu.Lock()
		i, ok := b.chiMucN[k]
		var g NhungGhi
		if ok {
			g = b.tep.Nhung[i]
			b.daDungN[i] = true
		} else {
			b.lech = append(b.lech, "nhúng: không có bản ghi cho "+khoaDoc(pham, hash, lan))
		}
		b.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("%w: embedding %s", ErrBangLech, khoaDoc(pham, hash, lan))
		}
		if g.Loi != nil {
			return nil, phatLoi(g.Loi)
		}
		out := make([][]float32, len(g.Vec))
		for i, v := range g.Vec {
			out[i] = append([]float32(nil), v...)
		}
		return out, nil
	}
	if b.nhungInner == nil {
		return nil, errors.New("aieval: this cassette wraps no embedder")
	}
	g := NhungGhi{Pham: pham, ReqHash: hash, Lan: lan, YeuCau: khoa}
	truoc := b.nhungInner.SoGoi()
	batDau := b.now()
	vs, err := that(b.nhungInner)
	g.MsTong = int(b.now().Sub(batDau) / time.Millisecond)
	daGoi := b.nhungInner.SoGoi() - truoc
	if err != nil {
		g.Loi = ghiLoi(err)
	} else {
		g.Vec = vs
	}
	b.mu.Lock()
	b.tep.Nhung = append(b.tep.Nhung, g)
	b.soNhungRa += daGoi
	b.soGoiRa += int(daGoi)
	b.mu.Unlock()
	return vs, err
}

// XepLaiGhi is one recorded reranker call: the order it answered, as the ids
// of the evidence it was given, and their scores.
type XepLaiGhi struct {
	Pham    string          `json:"pham"`
	ReqHash string          `json:"req_hash"`
	Lan     int             `json:"lan"`
	YeuCau  json.RawMessage `json:"yeu_cau"`
	IDs     []string        `json:"ids"`
	Diem    []float64       `json:"diem"`
	Loi     *LoiGhi         `json:"loi,omitempty"`
	MsTong  int             `json:"ms_tong"`
}

// CassetteXepLai is the cassette as the reranker of the retrieval path
// (truyhoi.Reranker), so a replay reorders evidence exactly as the recorded
// reranker did without an HTTP call. Key: (scope from ctx, sha256 of {model,
// query, evidence ids and fields, topN}, occurrence).
type CassetteXepLai struct{ bang *Bang }

var _ truyhoi.Reranker = (*CassetteXepLai)(nil)

// XepLai is the cassette's reranker, nil when the recorded run had none
// (the engine then keeps truyhoi.Passthrough, as the recorded run did).
func (b *Bang) XepLai() *CassetteXepLai {
	if b.tep.XepLaiModel == "" {
		return nil
	}
	return &CassetteXepLai{bang: b}
}

// loiBoQua is the class of a reranker failure after which the input order
// was kept (rerank.ErrBoQua).
const loiBoQua = "bo_qua"

// XepLai records (ghi) or replays (phat-lai) one reranker call.
func (c *CassetteXepLai) XepLai(ctx context.Context, cau string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	b := c.bang
	type muc struct {
		ID     string            `json:"id"`
		Nguon  string            `json:"nguon"`
		Truong map[string]string `json:"truong"`
	}
	ms := make([]muc, len(bc))
	theoID := make(map[string]truyhoi.BangChung, len(bc))
	for i, x := range bc {
		ms[i] = muc{x.ID, string(x.Nguon), x.Truong}
		theoID[x.ID] = x
	}
	khoa, err := jsonGon(map[string]any{"model": b.tep.XepLaiModel, "cau": cau, "bc": ms, "top_n": topN})
	if err != nil {
		return nil, err
	}
	hash := ShaCua(khoa)
	pham := phamTu(ctx)
	lan := b.demLan(b.lanX, pham, hash)
	if b.cheDo == CheDoPhatLai {
		k := khoaDay(pham, hash, lan)
		b.mu.Lock()
		i, ok := b.chiMucX[k]
		var g XepLaiGhi
		if ok {
			g = b.tep.XepLai[i]
			b.daDungX[i] = true
		} else {
			b.lech = append(b.lech, "xếp lại: không có bản ghi cho "+khoaDoc(pham, hash, lan))
		}
		b.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("%w: rerank %s", ErrBangLech, khoaDoc(pham, hash, lan))
		}
		out := make([]truyhoi.BangChung, 0, len(g.IDs))
		for j, id := range g.IDs {
			x, ok := theoID[id]
			if !ok || j >= len(g.Diem) {
				return nil, fmt.Errorf("%w: rerank recording names %q, not in the evidence", ErrBangLech, id)
			}
			x.DiemXepLai = g.Diem[j]
			out = append(out, x)
		}
		if g.Loi != nil {
			if g.Loi.Loai == loiBoQua {
				return out, fmt.Errorf("%w (recorded)", rerank.ErrBoQua)
			}
			return out, phatLoi(g.Loi)
		}
		return out, nil
	}
	if b.xepLaiInner == nil {
		return nil, errors.New("aieval: this cassette wraps no reranker")
	}
	g := XepLaiGhi{Pham: pham, ReqHash: hash, Lan: lan, YeuCau: khoa, IDs: []string{}, Diem: []float64{}}
	batDau := b.now()
	out, err := b.xepLaiInner.XepLai(ctx, cau, bc, topN)
	g.MsTong = int(b.now().Sub(batDau) / time.Millisecond)
	for _, x := range out {
		g.IDs = append(g.IDs, x.ID)
		g.Diem = append(g.Diem, x.DiemXepLai)
	}
	if err != nil {
		if errors.Is(err, rerank.ErrBoQua) {
			g.Loi = &LoiGhi{Loai: loiBoQua}
		} else {
			g.Loi = ghiLoi(err)
		}
	}
	b.mu.Lock()
	b.tep.XepLai = append(b.tep.XepLai, g)
	b.soGoiRa++
	b.mu.Unlock()
	return out, err
}

// DocBang reads a cassette file and returns it with the sha256 of its bytes.
func DocBang(path string) (TepBang, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return TepBang{}, "", err
	}
	var t TepBang
	if err := giaiMaChat(raw, &t); err != nil {
		return TepBang{}, "", fmt.Errorf("%s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return t, hex.EncodeToString(sum[:]), nil
}

// MaHoaBang is the file form of a cassette: indented JSON, recordings in
// the order they were made.
func MaHoaBang(t TepBang) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
