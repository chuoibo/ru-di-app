// Package nhung turns text into embedding vectors: the one door every
// embedding in core goes through, for the place index (rag) and for Nếp's
// memory (nepnho) alike, so both measure similarity in the same space.
//
// Two implementations. Gemini calls gemini-embedding-001 from Go through
// google.golang.org/genai, under the same rules as the text model in
// aiharness/llm: the key comes from GEMINI_API_KEY, an override base URL may
// only name a loopback host, and a test binary can never reach the real
// provider. Stub is deterministic and needs nothing: every test, gate and eval
// run uses it, and it is good enough to exercise retrieval code (texts that
// share syllables land close), not to measure retrieval quality.
//
// Vectors are always L2-normalised here, because a reduced output
// dimensionality (Matryoshka truncation) leaves Gemini's vectors unnormalised
// and cosine distance in pgvector assumes nothing.
package nhung

import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/rag/xephang"
)

// Model is the embedding model; Dims its output dimensionality here.
const (
	Model = "gemini-embedding-001"
	Dims  = 768
	// MaxBatch is how many texts one request carries.
	MaxBatch = 100
)

// TacVu is the embedding task: documents and queries are embedded
// asymmetrically by the provider.
type TacVu string

const (
	TaiLieu   TacVu = "RETRIEVAL_DOCUMENT"
	CauHoi    TacVu = "RETRIEVAL_QUERY"
	GiongNhau TacVu = "SEMANTIC_SIMILARITY"
)

// Nhung embeds texts. The result has one vector of Dims() floats per text, in
// order, each of unit length.
type Nhung interface {
	Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error)
	Model() string
	Dims() int
	// SoGoi is how many provider requests were made (0 for the stub).
	SoGoi() int64
}

// ErrRong: nothing to embed or a vector came back empty.
var ErrRong = errors.New("nhung: empty input or empty vector")

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

// Nhung embeds in batches of MaxBatch.
func (g *Gemini) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	dims := int32(Dims)
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += MaxBatch {
		end := min(start+MaxBatch, len(texts))
		contents := make([]*genai.Content, 0, end-start)
		for _, t := range texts[start:end] {
			contents = append(contents, genai.NewContentFromText(t, genai.RoleUser))
		}
		g.soGoi.Add(1)
		resp, err := g.client.Models.EmbedContent(ctx, Model, contents, &genai.EmbedContentConfig{
			TaskType:             string(tv),
			OutputDimensionality: &dims,
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Embeddings) != end-start {
			return nil, errors.New("nhung: provider returned a different number of vectors")
		}
		for _, e := range resp.Embeddings {
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
// is normalised. Texts sharing words land close; a query and a document are
// embedded the same way whatever the task.
type Stub struct{}

func (Stub) Model() string { return "stub-" + Model }
func (Stub) Dims() int     { return Dims }
func (Stub) SoGoi() int64  { return 0 }

func (Stub) Nhung(_ context.Context, texts []string, _ TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, Dims)
		for _, term := range xephang.Thuat(t) {
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
		out[i] = n
	}
	return out, nil
}

// ChuanHoa returns v scaled to unit length.
func ChuanHoa(v []float32) ([]float32, error) {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if len(v) == 0 || s == 0 {
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
