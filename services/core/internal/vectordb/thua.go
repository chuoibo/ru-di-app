package vectordb

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
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
)

// LoaiThua names a sparse leg.
type LoaiThua string

const (
	// ThuaBM25: Milvus's BM25 function over the text field. Needs no model;
	// the default.
	ThuaBM25 LoaiThua = "bm25"
	// ThuaMILCO: learned sparse vectors from the MILCO encoder in
	// services/ai-infer. Behind MOBILE_MILCO_ENABLED, default off: its
	// weights' licence is unconfirmed (research milco.md §Kiểm chứng 11).
	ThuaMILCO LoaiThua = "milco"
)

// ThuaVec is a sparse vector: strictly increasing indices, positive finite
// values.
type ThuaVec struct {
	Chi    []uint32
	GiaTri []float32
}

// ThuaTruyVan is a query's sparse leg: the field to search and what to
// search it with (text for BM25, a vector for MILCO).
type ThuaTruyVan struct {
	Loai LoaiThua
	// Text is the BM25 query of the marked field (FBM25), and of the folded
	// field too unless TextKhongDau is set.
	Text string
	// TextKhongDau, when set, is the folded field's (FBM25KhongDau) query:
	// the person's own spelling, while Text carries the router's
	// diacritics-restored form (hieu.TruyVan.CauCoDau).
	TextKhongDau string
	Vec          ThuaVec
}

// TextCua is the BM25 query text of field f (FBM25 or FBM25KhongDau).
func (q ThuaTruyVan) TextCua(f string) string {
	if f == FBM25KhongDau && q.TextKhongDau != "" {
		return q.TextKhongDau
	}
	return q.Text
}

// Truong is the collection field the leg searches.
func (q ThuaTruyVan) Truong() string {
	if q.Loai == ThuaMILCO {
		return FSparse
	}
	return FBM25
}

// Thua is the sparse port: two adapters, one chosen by configuration.
type Thua interface {
	Loai() LoaiThua
	// TruyVan builds a query's sparse leg.
	TruyVan(ctx context.Context, cau string) (ThuaTruyVan, error)
	// TaiLieu encodes documents for the MILCO field at ingest. The BM25
	// adapter returns empty vectors: Milvus computes BM25 from the text.
	TaiLieu(ctx context.Context, texts []string) ([]ThuaVec, error)
}

// BM25 is the default sparse adapter.
type BM25 struct{}

func (BM25) Loai() LoaiThua { return ThuaBM25 }

// TruyVan passes the NFC query text to Milvus's analyzer.
func (BM25) TruyVan(_ context.Context, cau string) (ThuaTruyVan, error) {
	t := nhung.ChuanNFC(cau)
	if t == "" {
		return ThuaTruyVan{}, nhung.ErrRong
	}
	return ThuaTruyVan{Loai: ThuaBM25, Text: t}, nil
}

// TaiLieu returns one empty vector per text: the MILCO field stays empty.
func (BM25) TaiLieu(_ context.Context, texts []string) ([]ThuaVec, error) {
	return make([]ThuaVec, len(texts)), nil
}

// Environment of the MILCO adapter.
const (
	EnvMILCOBat = "MOBILE_MILCO_ENABLED"
	EnvAIInfer  = "MOBILE_AI_INFER_URL"
)

// MILCO limits: at most this many non-zeros per vector (the encoder prunes;
// a larger vector is refused, not truncated), and the index space.
const (
	MaxThuaNNZ  = 512
	MILCOChiMax = uint32(1) << 30
	MaxLoMILCO  = 32
)

// ErrThuaSai: the encoder answered something outside the contract.
var ErrThuaSai = errors.New("vectordb: sparse encoder answered outside its contract")

// MILCO calls POST {base}/v1/milco/encode on the inference service with
// {"kind": "query"|"doc", "texts": [...]} and expects {"vectors":
// [{"indices": [...], "values": [...]}]}, one per text, in order. It has its
// own timeout, never retries within a turn (the leg is dropped and the
// retrieval flagged truyhoi.NoSparse), and never logs a body.
type MILCO struct {
	base    string
	http    *http.Client
	timeout time.Duration
}

// MILCOTuEnv builds the MILCO adapter when MOBILE_MILCO_ENABLED=1, and
// returns (nil, nil) otherwise: the caller then uses BM25. The inference
// service must be named by MOBILE_AI_INFER_URL, loopback only.
func MILCOTuEnv(getenv func(string) string) (*MILCO, error) {
	if strings.TrimSpace(getenv(EnvMILCOBat)) != "1" {
		return nil, nil
	}
	return NewMILCO(getenv(EnvAIInfer), 800*time.Millisecond)
}

// NewMILCO builds the adapter for a loopback inference service.
func NewMILCO(base string, timeout time.Duration) (*MILCO, error) {
	if err := llm.CheckBaseURL(base); err != nil {
		return nil, fmt.Errorf("vectordb: %s: %w", EnvAIInfer, err)
	}
	u, _ := url.Parse(base)
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/milco/encode"
	return &MILCO{base: u.String(), http: &http.Client{}, timeout: timeout}, nil
}

func (m *MILCO) Loai() LoaiThua { return ThuaMILCO }

// TruyVan encodes the query.
func (m *MILCO) TruyVan(ctx context.Context, cau string) (ThuaTruyVan, error) {
	t := nhung.ChuanNFC(cau)
	if t == "" {
		return ThuaTruyVan{}, nhung.ErrRong
	}
	vs, err := m.goi(ctx, "query", []string{t})
	if err != nil {
		return ThuaTruyVan{}, err
	}
	return ThuaTruyVan{Loai: ThuaMILCO, Vec: vs[0]}, nil
}

// TaiLieu encodes documents in batches of MaxLoMILCO.
func (m *MILCO) TaiLieu(ctx context.Context, texts []string) ([]ThuaVec, error) {
	out := make([]ThuaVec, 0, len(texts))
	for start := 0; start < len(texts); start += MaxLoMILCO {
		end := min(start+MaxLoMILCO, len(texts))
		batch := make([]string, end-start)
		for i, t := range texts[start:end] {
			batch[i] = nhung.ChuanNFC(t)
		}
		vs, err := m.goi(ctx, "doc", batch)
		if err != nil {
			return nil, err
		}
		out = append(out, vs...)
	}
	return out, nil
}

type milcoTraLoi struct {
	Vectors []struct {
		Indices []uint32  `json:"indices"`
		Values  []float32 `json:"values"`
	} `json:"vectors"`
}

func (m *MILCO) goi(ctx context.Context, kind string, texts []string) ([]ThuaVec, error) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"kind": kind, "texts": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.base, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.http.Do(req)
	if err != nil {
		// The transport error names the URL, never the body.
		return nil, fmt.Errorf("vectordb: milco: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("%w: http %d", ErrThuaSai, resp.StatusCode)
	}
	var tl milcoTraLoi
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&tl); err != nil {
		return nil, fmt.Errorf("%w: body is not the contract's JSON", ErrThuaSai)
	}
	if len(tl.Vectors) != len(texts) {
		return nil, fmt.Errorf("%w: %d vectors for %d texts", ErrThuaSai, len(tl.Vectors), len(texts))
	}
	out := make([]ThuaVec, len(texts))
	for i, v := range tl.Vectors {
		tv := ThuaVec{Chi: v.Indices, GiaTri: v.Values}
		if err := tv.Kiem(); err != nil {
			return nil, err
		}
		out[i] = tv
	}
	return out, nil
}

// Kiem checks a sparse vector's structure: equal lengths, at most MaxThuaNNZ
// entries, strictly increasing indices below MILCOChiMax, positive finite
// values. An empty vector is valid (a text with no weighted term).
func (v ThuaVec) Kiem() error {
	if len(v.Chi) != len(v.GiaTri) || len(v.Chi) > MaxThuaNNZ {
		return fmt.Errorf("%w: %d indices, %d values", ErrThuaSai, len(v.Chi), len(v.GiaTri))
	}
	for i, c := range v.Chi {
		if c >= MILCOChiMax || (i > 0 && c <= v.Chi[i-1]) {
			return fmt.Errorf("%w: index %d out of order or range", ErrThuaSai, i)
		}
		g := float64(v.GiaTri[i])
		if !(g > 0) || math.IsInf(g, 0) {
			return fmt.Errorf("%w: value %d not a positive finite number", ErrThuaSai, i)
		}
	}
	return nil
}
