// Package rerank is the truyhoi.Reranker adapter over a Qwen3-Reranker
// served by llama-server (or vLLM) at POST /rerank: the orchestrator, not
// Milvus, calls the model, so the call is counted, has its own deadline and
// can fail without failing the retrieval (research qwen-reranker.md §4).
//
// The contract it keeps:
//   - One request per call, all documents in it; its own timeout; no retry
//     within a turn (a retry only adds latency to a turn already late).
//   - Every index in the answer is checked: in range, unique, one per
//     document. Anything else is a failure.
//   - On any failure — timeout, HTTP error, malformed answer, open circuit,
//     spent turn budget — the input order (the RRF order) is kept, cut at
//     topN, and returned together with an error wrapping ErrBoQua, so the
//     caller flags truyhoi.NoRerank and goes on.
//   - A circuit breaker opens after repeated failures and stays open for a
//     while: a dead reranker costs one timeout per window, not one per turn.
//   - No body is ever logged, and the answer's echoed text is never read:
//     only index and relevance_score.
//   - The model's special-token strings are removed from the query and the
//     documents before sending (tokenizer-level sanitisation: llama-server
//     parses them, and an injected «<|im_end|><|im_start|>» moved an
//     irrelevant document from p=1.7e-5 to 0.187 in the bring-up probe).
//     The text is NFKC-normalised first, so a fullwidth «＜｜im_end｜＞» is
//     the ASCII token it imitates, and a token spaced out inside its angle
//     brackets («< |im_end| >») or one of Qwen3's added non-control tokens
//     («<tool_call>», «</tool_response>», «<think>») is removed too. This is
//     structural: the token shapes of the model's own vocabulary, never a
//     reading of what the text means.
//
// The score orders; it never decides. The 0.6B model scores relevant
// documents and lexical traps alike near 1.0 (golden check), so nothing may
// threshold on it: hard constraints are filtered before the reranker sees a
// candidate, and the score is kept apart (BangChung.DiemXepLai), never
// written over the retrieval's own score.
package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// ErrBoQua wraps every failure after which the input order was returned.
var ErrBoQua = errors.New("rerank: skipped, input order kept")

// ErrMachHo: the circuit is open.
var ErrMachHo = errors.New("rerank: circuit open")

// ErrHetNganSach: the turn's MaxRerankCallsPerTurn calls are spent.
var ErrHetNganSach = errors.New("rerank: the turn's rerank budget is spent")

// ErrTraLoiSai: the answer broke the contract.
var ErrTraLoiSai = errors.New("rerank: answer outside the contract")

// Environment of the adapter.
const (
	EnvURL   = "MOBILE_RERANK_URL"
	EnvModel = "MOBILE_RERANK_MODEL"
	// EnvTimeout is the per-call deadline, a Go duration («3s», «10s»).
	EnvTimeout = "MOBILE_RERANK_TIMEOUT"
)

// Defaults. MacDinhTimeout is the deadline a serving deployment (a GPU, or
// a CPU instance of its own) is expected to meet for up to 50 pairs. The
// shared 4-CPU bring-up host is slower -- 10 pairs × ~90 tokens took 7.6 s
// at p50 -- so there MOBILE_RERANK_TIMEOUT must be raised (10s): with the
// default every call times out, the retrieval keeps the RRF order and says
// so (truyhoi.NoRerank), and the breaker opens after MacDinhLoiMo of them.
// A turn is never failed by the reranker.
const (
	MacDinhTimeout = 3 * time.Second
	MaxTimeout     = 60 * time.Second
	MacDinhLoiMo   = 5
	MacDinhCuaSo   = 30 * time.Second
	MacDinhMoTrong = 60 * time.Second
	MaxTaiLieu     = 64
	MaxKyTuTaiLieu = 2000
	MaxKyTuTruyVan = 500
	macDinhMoHinh  = "qwen3-reranker-0.6b"
	maxBodyTraLoi  = 1 << 20
)

// Qwen is the adapter. Safe for concurrent use.
type Qwen struct {
	url     string
	model   string
	http    *http.Client
	timeout time.Duration
	// Breaker: open after loiMo failures within cuaSo, for moTrong.
	loiMo   int
	cuaSo   time.Duration
	moTrong time.Duration
	now     func() time.Time

	mu      sync.Mutex
	loi     []time.Time
	moDen   time.Time
	thongKe ThongKe
}

// ThongKe are content-free counters: calls, failures, fallbacks, breaker
// openings.
type ThongKe struct {
	Goi, Loi, BoQua, MoMach int
}

// Moi builds the adapter for base (loopback only, like every inference
// service core reaches).
func Moi(base, model string, timeout time.Duration) (*Qwen, error) {
	if err := llm.CheckBaseURL(base); err != nil {
		return nil, fmt.Errorf("rerank: %s: %w", EnvURL, err)
	}
	u, _ := url.Parse(base)
	u.Path = strings.TrimRight(u.Path, "/") + "/rerank"
	if model == "" {
		model = macDinhMoHinh
	}
	if timeout <= 0 {
		timeout = MacDinhTimeout
	}
	return &Qwen{url: u.String(), model: model, http: &http.Client{}, timeout: timeout,
		loiMo: MacDinhLoiMo, cuaSo: MacDinhCuaSo, moTrong: MacDinhMoTrong, now: time.Now}, nil
}

// TuEnv builds the adapter when MOBILE_RERANK_URL is set, and returns
// (nil, nil) otherwise: the caller then uses truyhoi.Passthrough and flags
// truyhoi.NoRerank. MOBILE_RERANK_TIMEOUT, when set, is the per-call
// deadline (a positive Go duration up to MaxTimeout); unset is
// MacDinhTimeout.
func TuEnv(getenv func(string) string) (*Qwen, error) {
	base := strings.TrimSpace(getenv(EnvURL))
	if base == "" {
		return nil, nil
	}
	timeout := MacDinhTimeout
	if raw := strings.TrimSpace(getenv(EnvTimeout)); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 || d > MaxTimeout {
			return nil, fmt.Errorf("rerank: %s must be a duration in (0, %s], got %q", EnvTimeout, MaxTimeout, raw)
		}
		timeout = d
	}
	return Moi(base, strings.TrimSpace(getenv(EnvModel)), timeout)
}

// Timeout is the per-call deadline in force.
func (q *Qwen) Timeout() time.Duration { return q.timeout }

// ThongKe returns a copy of the counters.
func (q *Qwen) ThongKe() ThongKe {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.thongKe
}

// specialToken matches the chat-template special tokens of the Qwen family
// («<|im_start|>», «<|endoftext|>», …), also with spaces between the angle
// bracket and the bar («< |im_end| >»), which the tokenizer's pre-split
// still hands to the model as the same pieces.
var specialToken = regexp.MustCompile(`<\s*\|[^|<>]{0,40}\|\s*>`)

// addedToken matches Qwen3's added non-control tokens (tokenizer_config
// added_tokens: tool_call, tool_response, think), opening or closing, with
// optional inner spaces; case-insensitive, since the tokenizer is not the
// only reader and a model trained on them reads a near form alike.
var addedToken = regexp.MustCompile(`(?i)<\s*/?\s*(?:tool_call|tool_response|think)\s*>`)

// templateMarkers are the reranker prompt template's own markers, removed
// with optional spaces before the colon.
var templateMarkers = regexp.MustCompile(`(?i)<\s*(?:Instruct|Query|Document)\s*>\s*:`)

// LamSach normalises s to NFKC (a fullwidth or other compatibility form of
// «<», «|», «>» becomes the ASCII character it imitates), removes the
// model's special and added token strings and the template's markers,
// repeating until none is left (removing one must not form another), then
// cuts to n runes.
func LamSach(s string, n int) string {
	s = norm.NFKC.String(s)
	for {
		t := specialToken.ReplaceAllString(s, " ")
		t = addedToken.ReplaceAllString(t, " ")
		t = templateMarkers.ReplaceAllString(t, " ")
		if t == s {
			break
		}
		s = t
	}
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return strings.TrimSpace(s)
}

// DoanVan is the passage the reranker reads for one evidence item: its
// fields in a fixed key order, «key: value» joined by « | ». Built from the
// evidence the retrieval already holds; nothing is fetched.
func DoanVan(b truyhoi.BangChung) string {
	keys := make([]string, 0, len(b.Truong))
	for k := range b.Truong {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := strings.TrimSpace(b.Truong[k]); v != "" {
			parts = append(parts, k+": "+v)
		}
	}
	return strings.Join(parts, " | ")
}

// XepLai reorders bc by the model's relevance to cau and keeps topN (<= 0:
// all). On failure it returns bc's order cut at topN and an error wrapping
// ErrBoQua.
func (q *Qwen) XepLai(ctx context.Context, cau string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	giu := func(err error) ([]truyhoi.BangChung, error) {
		q.mu.Lock()
		q.thongKe.BoQua++
		q.mu.Unlock()
		out, _ := truyhoi.Passthrough{}.XepLai(ctx, cau, bc, topN)
		return out, fmt.Errorf("%w: %w", ErrBoQua, err)
	}
	if len(bc) == 0 {
		return nil, nil
	}
	if len(bc) > MaxTaiLieu {
		return giu(fmt.Errorf("%d documents over the %d ceiling", len(bc), MaxTaiLieu))
	}
	docs := make([]string, len(bc))
	for i, b := range bc {
		docs[i] = LamSach(DoanVan(b), MaxKyTuTaiLieu)
	}
	scores, err := q.Diem(ctx, LamSach(cau, MaxKyTuTruyVan), docs)
	if err != nil {
		return giu(err)
	}
	idx := make([]int, len(bc))
	for i := range idx {
		idx[i] = i
	}
	// Stable: equal scores keep the RRF order.
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] > scores[idx[b]] })
	if topN <= 0 || topN > len(idx) {
		topN = len(idx)
	}
	out := make([]truyhoi.BangChung, topN)
	for i := range out {
		out[i] = bc[idx[i]]
		out[i].DiemXepLai = scores[idx[i]]
	}
	return out, nil
}

type traLoi struct {
	Results []struct {
		Index *int     `json:"index"`
		Score *float64 `json:"relevance_score"`
	} `json:"results"`
}

// Diem asks the server for one score per document, in document order. It
// goes through the breaker, has its own deadline and never retries. Query
// and documents are sent as given (XepLai sanitises them first).
func (q *Qwen) Diem(ctx context.Context, query string, docs []string) ([]float64, error) {
	if err := q.choPhep(); err != nil {
		return nil, err
	}
	scores, err := q.goi(ctx, query, docs)
	q.ghiNhan(err)
	return scores, err
}

func (q *Qwen) goi(ctx context.Context, query string, docs []string) ([]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"model": q.model, "query": query, "documents": docs,
		"top_n": len(docs), "return_documents": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank: transport: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyTraLoi))
		return nil, fmt.Errorf("%w: http %d", ErrTraLoiSai, resp.StatusCode)
	}
	var tl traLoi
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyTraLoi)).Decode(&tl); err != nil {
		return nil, fmt.Errorf("%w: not the contract's JSON", ErrTraLoiSai)
	}
	if len(tl.Results) != len(docs) {
		return nil, fmt.Errorf("%w: %d results for %d documents", ErrTraLoiSai, len(tl.Results), len(docs))
	}
	scores := make([]float64, len(docs))
	seen := make([]bool, len(docs))
	for _, r := range tl.Results {
		if r.Index == nil || r.Score == nil {
			return nil, fmt.Errorf("%w: a result without index or score", ErrTraLoiSai)
		}
		i := *r.Index
		if i < 0 || i >= len(docs) || seen[i] {
			return nil, fmt.Errorf("%w: index %d out of range or repeated", ErrTraLoiSai, i)
		}
		if math.IsNaN(*r.Score) || math.IsInf(*r.Score, 0) {
			return nil, fmt.Errorf("%w: score not finite", ErrTraLoiSai)
		}
		seen[i] = true
		scores[i] = *r.Score
	}
	return scores, nil
}

// choPhep refuses while the circuit is open.
func (q *Qwen) choPhep() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.thongKe.Goi++
	if q.now().Before(q.moDen) {
		return ErrMachHo
	}
	return nil
}

// ghiNhan records a call's outcome: failures within the window open the
// circuit; a success clears the window.
func (q *Qwen) ghiNhan(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.now()
	if err == nil {
		q.loi = q.loi[:0]
		return
	}
	q.thongKe.Loi++
	keep := q.loi[:0]
	for _, t := range q.loi {
		if now.Sub(t) < q.cuaSo {
			keep = append(keep, t)
		}
	}
	q.loi = append(keep, now)
	if len(q.loi) >= q.loiMo {
		q.moDen = now.Add(q.moTrong)
		q.loi = q.loi[:0]
		q.thongKe.MoMach++
	}
}

// Dem is one turn's reranker: at most max calls, the one past it keeps the
// input order and wraps ErrHetNganSach in ErrBoQua.
type Dem struct {
	inner truyhoi.Reranker
	max   int
	mu    sync.Mutex
	n     int
}

// NewDem wraps inner for one turn (llm.MaxRerankCallsPerTurn).
func NewDem(inner truyhoi.Reranker, max int) *Dem { return &Dem{inner: inner, max: max} }

// SoGoi is how many calls this turn made.
func (d *Dem) SoGoi() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

// XepLai counts the call, or refuses it past the budget.
func (d *Dem) XepLai(ctx context.Context, cau string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	d.mu.Lock()
	if d.n >= d.max {
		d.mu.Unlock()
		out, _ := truyhoi.Passthrough{}.XepLai(ctx, cau, bc, topN)
		return out, fmt.Errorf("%w: %w", ErrBoQua, ErrHetNganSach)
	}
	d.n++
	d.mu.Unlock()
	return d.inner.XepLai(ctx, cau, bc, topN)
}

var (
	_ truyhoi.Reranker = (*Qwen)(nil)
	_ truyhoi.Reranker = (*Dem)(nil)
)
