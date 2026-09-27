// Package nhung turns text into embedding vectors: the one door every
// embedding in core goes through, for the place and manual indexes
// (vectordb) and for Nếp's memory alike, so all of them measure similarity
// in the same space and one query vector serves every collection.
//
// The model is gemini-embedding-2 at 1536 dimensions for every collection
// (research gemini-embedding-2.md §6.1). On the Gemini Developer API that
// model takes its task as an instruction written into the text, not as the
// request's taskType, so this package writes the prefix itself
// (DinhDang) and never sets EmbedContentConfig.TaskType or Title; the
// request body is pinned by a wire-contract test, and every vector that
// comes back is checked to have exactly Dims values before it is used,
// because a provider that ignored outputDimensionality would answer 3072
// and the vector store would refuse the row (research §Kiểm chứng 2, 6).
//
// Two implementations. Gemini calls the model from Go through
// google.golang.org/genai, under the same rules as the text model in
// aiharness/llm: the key comes from GEMINI_API_KEY, an override base URL may
// only name a loopback host, and a test binary can never reach the real
// provider (ADR-0034 §2.6). Stub is deterministic and needs nothing: every
// test, gate and eval run uses it; it is good enough to exercise retrieval
// code (texts that share syllables land close), not to measure retrieval
// quality.
//
// Vectors are always L2-normalised here: the provider's truncated vectors
// are not documented as normalised, and cosine and inner product in the
// vector store assume unit length.
package nhung

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/rag/xephang"
)

// DemLuot counts one turn's embedding calls against
// llm.MaxEmbedCallsPerTurn, apart from its model calls. Safe for concurrent
// use.
type DemLuot struct{ con atomic.Int64 }

// MoiDemLuot is a counter holding the whole per-turn budget.
func MoiDemLuot() *DemLuot {
	d := &DemLuot{}
	d.con.Store(llm.MaxEmbedCallsPerTurn)
	return d
}

// ErrHetLuotNhung: the turn's embedding calls are spent.
var ErrHetLuotNhung = errors.New("nhung: the turn's embedding calls are spent")

// Giu takes one call from the budget, or refuses once it is spent. A nil
// counter counts nothing: a caller outside a turn (the index build).
func (d *DemLuot) Giu() error {
	if d == nil {
		return nil
	}
	if d.con.Add(-1) < 0 {
		return ErrHetLuotNhung
	}
	return nil
}

// ConLai is how many calls are left (MaxEmbedCallsPerTurn for nil).
func (d *DemLuot) ConLai() int {
	if d == nil {
		return llm.MaxEmbedCallsPerTurn
	}
	return int(max(d.con.Load(), 0))
}

// Model is the embedding model; Dims its output dimensionality for every
// collection; PromptVersion names how the task is conveyed to the model (an
// in-text prefix, version 1). All three are part of an embedding's identity:
// the cache key (nhungcache) and a collection's recorded model carry them,
// and changing any one means a re-embed into a new collection version.
const (
	Model         = "gemini-embedding-2"
	Dims          = 1536
	PromptVersion = "prefix-v1"
	// MaxBatch is how many texts one request carries. The provider's own
	// limit is not confirmed (research §Kiểm chứng 9); 100 is the defensive
	// ceiling the Go SDK does not enforce for us.
	MaxBatch = 100
)

// TacVu is the embedding task. Documents and queries are embedded
// asymmetrically: the task decides the prefix DinhDang writes.
type TacVu string

const (
	TaiLieu   TacVu = "tai_lieu"
	CauHoi    TacVu = "cau_hoi"
	GiongNhau TacVu = "giong_nhau"
)

// TaiLieuVao is one document to embed: its title ("" when it has none) and
// its text.
type TaiLieuVao struct {
	TieuDe  string
	NoiDung string
}

// Nhung embeds texts. The result has one vector of Dims() floats per input,
// in order, each of unit length.
type Nhung interface {
	// Nhung embeds texts for the task tv; a document has no title here.
	Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error)
	// NhungTaiLieu embeds documents with their titles.
	NhungTaiLieu(ctx context.Context, docs []TaiLieuVao) ([][]float32, error)
	Model() string
	Dims() int
	// SoGoi is how many provider requests were made (0 for the stub).
	SoGoi() int64
}

// ErrRong: nothing to embed or a vector came back empty.
var ErrRong = errors.New("nhung: empty input or empty vector")

// ErrSaiChieu: the provider answered a vector whose length is not Dims.
var ErrSaiChieu = errors.New("nhung: vector has the wrong number of dimensions")

// ErrTacVu: a task outside the closed set.
var ErrTacVu = errors.New("nhung: unknown task")

// ChuanNFC is the text as the model and the cache see it: Unicode NFC with
// the surrounding whitespace trimmed. Diacritics are kept.
func ChuanNFC(s string) string { return strings.TrimSpace(norm.NFC.String(s)) }

// DinhDang writes the task into the text, the way gemini-embedding-2 takes
// it on the Developer API (the prefixes of Google's recommended mapping,
// research §3 and §Kiểm chứng 3): a query «task: search result | query: …»,
// a document «title: … | text: …» with «none» for no title, a symmetric
// comparison «task: sentence similarity | query: …». The text is NFC first.
func DinhDang(tv TacVu, tieuDe, noiDung string) (string, error) {
	noiDung = ChuanNFC(noiDung)
	switch tv {
	case CauHoi:
		return "task: search result | query: " + noiDung, nil
	case GiongNhau:
		return "task: sentence similarity | query: " + noiDung, nil
	case TaiLieu:
		t := ChuanNFC(tieuDe)
		if t == "" {
			t = "none"
		}
		return "title: " + t + " | text: " + noiDung, nil
	}
	return "", fmt.Errorf("%w: %q", ErrTacVu, tv)
}

// Gemini is the real embedder.
type Gemini struct {
	client *genai.Client
	soGoi  atomic.Int64
}

// NewGemini builds the real embedder. baseURL "" means the real API, which a
// test binary is refused, exactly as for the text model.
func NewGemini(ctx context.Context, apiKey, baseURL string) (*Gemini, error) {
	if apiKey == "" {
		return nil, llm.ErrNotConfigured
	}
	if baseURL != "" {
		if err := llm.CheckBaseURL(baseURL); err != nil {
			return nil, err
		}
	} else {
		if testing.Testing() {
			return nil, errors.New("nhung: a test binary may only reach a loopback Gemini")
		}
		baseURL = llm.DefaultBaseURL
	}
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:      apiKey,
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{BaseURL: baseURL},
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	})
	if err != nil {
		return nil, err
	}
	return &Gemini{client: c}, nil
}

// FromEnv reads GEMINI_API_KEY and MOBILE_GEMINI_BASE_URL once.
func FromEnv(ctx context.Context, getenv func(string) string) (*Gemini, error) {
	return NewGemini(ctx, getenv(llm.EnvAPIKey), getenv(llm.EnvBaseURL))
}

func (g *Gemini) Model() string { return Model }
func (g *Gemini) Dims() int     { return Dims }
func (g *Gemini) SoGoi() int64  { return g.soGoi.Load() }

// Nhung embeds texts with the task's prefix, in batches of MaxBatch.
func (g *Gemini) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	prompts := make([]string, len(texts))
	for i, t := range texts {
		p, err := DinhDang(tv, "", t)
		if err != nil {
			return nil, err
		}
		prompts[i] = p
	}
	return g.gui(ctx, prompts)
}

// NhungTaiLieu embeds documents with their titles in the prefix.
func (g *Gemini) NhungTaiLieu(ctx context.Context, docs []TaiLieuVao) ([][]float32, error) {
	if len(docs) == 0 {
		return nil, ErrRong
	}
	prompts := make([]string, len(docs))
	for i, d := range docs {
		prompts[i], _ = DinhDang(TaiLieu, d.TieuDe, d.NoiDung)
	}
	return g.gui(ctx, prompts)
}

// gui sends already formatted prompts. The config carries the output
// dimensionality and nothing else: no TaskType, no Title (research
// §Kiểm chứng, recommendation 1 — never both an in-text task and a field).
func (g *Gemini) gui(ctx context.Context, prompts []string) ([][]float32, error) {
	dims := int32(Dims)
	out := make([][]float32, 0, len(prompts))
	for start := 0; start < len(prompts); start += MaxBatch {
		end := min(start+MaxBatch, len(prompts))
		contents := make([]*genai.Content, 0, end-start)
		for _, p := range prompts[start:end] {
			contents = append(contents, genai.NewContentFromText(p, genai.RoleUser))
		}
		g.soGoi.Add(1)
		resp, err := g.client.Models.EmbedContent(ctx, Model, contents, &genai.EmbedContentConfig{
			OutputDimensionality: &dims,
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Embeddings) != end-start {
			return nil, errors.New("nhung: provider returned a different number of vectors")
		}
		for _, e := range resp.Embeddings {
			if e == nil || len(e.Values) != Dims {
				n := 0
				if e != nil {
					n = len(e.Values)
				}
				return nil, fmt.Errorf("%w: got %d, want %d", ErrSaiChieu, n, Dims)
			}
			v, err := ChuanHoa(e.Values)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// Stub embeds deterministically: each syllable and syllable pair (folded, as
// xephang reads them) is hashed into Dims buckets with a sign, then the vector
// is normalised. Texts sharing words land close; the task prefix is not
// embedded (it would add the same component to every vector of a task), so
// a query and a document with the same words are close whatever the task.
type Stub struct{}

func (Stub) Model() string { return "stub-" + Model }
func (Stub) Dims() int     { return Dims }
func (Stub) SoGoi() int64  { return 0 }

// Nhung embeds texts; the task is checked but does not change the vector.
func (s Stub) Nhung(_ context.Context, texts []string, tv TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	if _, err := DinhDang(tv, "", ""); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = stubVec(t)
	}
	return out, nil
}

// NhungTaiLieu embeds title and text together.
func (s Stub) NhungTaiLieu(_ context.Context, docs []TaiLieuVao) ([][]float32, error) {
	if len(docs) == 0 {
		return nil, ErrRong
	}
	out := make([][]float32, len(docs))
	for i, d := range docs {
		out[i] = stubVec(d.TieuDe + " " + d.NoiDung)
	}
	return out, nil
}

func stubVec(t string) []float32 {
	v := make([]float32, Dims)
	for _, term := range xephang.Thuat(ChuanNFC(t)) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(term))
		sum := h.Sum64()
		w := float32(1)
		if sum>>63 == 1 {
			w = -1
		}
		v[sum%Dims] += w
	}
	n, err := ChuanHoa(v)
	if err != nil {
		// A text with no word gets a fixed unit vector rather than an
		// error: an empty field must still be storable.
		n = make([]float32, Dims)
		n[0] = 1
	}
	return n
}

// ChuanHoa returns v scaled to unit length.
func ChuanHoa(v []float32) ([]float32, error) {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if len(v) == 0 || s == 0 || math.IsNaN(s) || math.IsInf(s, 0) {
		return nil, ErrRong
	}
	inv := 1 / math.Sqrt(s)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) * inv)
	}
	return out, nil
}

// Cosine is the cosine similarity of two unit vectors.
func Cosine(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

// Literal renders a vector as a pgvector text literal, «[x,y,…]».
func Literal(v []float32) string {
	b := make([]byte, 0, len(v)*10+2)
	b = append(b, '[')
	for i, x := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendFloat(b, x)
	}
	return string(append(b, ']'))
}
