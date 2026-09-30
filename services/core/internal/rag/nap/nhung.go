package nap

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"sort"

	"mobile/services/core/internal/rag/xephang"
)

// TaiLieu is one document to embed: a title and its text.
type TaiLieu struct {
	TieuDe string
	Chu    string
}

// NhungTaiLieu embeds documents on the dense leg. The production adapter
// wraps aiharness/nhung (the engine's one door to the embedding model; rag
// may not import it) and lives in cmd/core. It passes the title and the
// text as they are: the task prefix gemini-embedding-2 needs is written
// once, inside aiharness/nhung (DinhDang), never here. Every vector it
// returns is checked here (KiemVector) before it is stored.
type NhungTaiLieu interface {
	Model() string
	Dims() int
	NhungTaiLieu(ctx context.Context, docs []TaiLieu) ([][]float32, error)
	// SoGoi is how many provider requests were made (0 for a stub).
	SoGoi() int64
}

// NhungCauHoi embeds queries on the dense leg (the eval gate's side of the
// retrieval; the serving side is the retrieval adapter's).
type NhungCauHoi interface {
	NhungCauHoi(ctx context.Context, qs []string) ([][]float32, error)
}

// KiemVector checks one returned vector: exactly dims values, all finite,
// not all zero; and returns it L2-normalised (a truncated Matryoshka vector
// is not unit length). A provider that silently answers 3072 values for a
// request of the configured size is refused here, before Milvus would.
func KiemVector(v []float32, dims int) ([]float32, error) {
	if len(v) != dims {
		return nil, fmt.Errorf("%w: %d values, want %d", ErrVector, len(v), dims)
	}
	var s float64
	for _, x := range v {
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("%w: non-finite value", ErrVector)
		}
		s += f * f
	}
	if s == 0 {
		return nil, fmt.Errorf("%w: zero vector", ErrVector)
	}
	inv := 1 / math.Sqrt(s)
	out := make([]float32, dims)
	for i, x := range v {
		out[i] = float32(float64(x) * inv)
	}
	return out, nil
}

// StubDense is the deterministic dense encoder of every test, gate and
// offline eval: each folded syllable and syllable pair (xephang.Thuat) is
// hashed into N buckets with a sign. Texts sharing words land close; it is
// good enough to exercise the pipeline and to pin numbers, not to measure
// retrieval quality. Documents embed title and text together.
type StubDense struct{ N int }

func (s StubDense) Model() string { return fmt.Sprintf("stub-dense-%d", s.N) }
func (s StubDense) Dims() int     { return s.N }
func (s StubDense) SoGoi() int64  { return 0 }

func (s StubDense) NhungTaiLieu(_ context.Context, docs []TaiLieu) ([][]float32, error) {
	out := make([][]float32, len(docs))
	for i, d := range docs {
		out[i] = s.vec(d.TieuDe + "\n" + d.Chu)
	}
	return out, nil
}

func (s StubDense) NhungCauHoi(_ context.Context, qs []string) ([][]float32, error) {
	out := make([][]float32, len(qs))
	for i, q := range qs {
		out[i] = s.vec(q)
	}
	return out, nil
}

func (s StubDense) vec(text string) []float32 {
	v := make([]float32, s.N)
	for _, term := range xephang.Thuat(text) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(term))
		sum := h.Sum64()
		w := float32(1)
		if sum>>63 == 1 {
			w = -1
		}
		v[sum%uint64(s.N)] += w
	}
	n, err := KiemVector(v, s.N)
	if err != nil {
		n = make([]float32, s.N)
		n[0] = 1
	}
	return n
}

// BoNhoNhung is the dense cache: content hash → vector, per model, dims and
// task. Only a hash missing here costs a provider call (research
// sdlc-production §C2 S8a).
type BoNhoNhung interface {
	LayNhung(ctx context.Context, model string, dims int, task string, hashes []string) (map[string][]float32, error)
	GhiNhung(ctx context.Context, model string, dims int, task string, vecs map[string][]float32) error
}

// TaskTaiLieu is the cache's task key for documents: the mechanism is part
// of it, so switching prefix-in-text for TaskType can never serve an old
// vector.
func TaskTaiLieu(cfg CauHinh) string { return "doc:" + cfg.Dense.CoChe }

// NhungHang fills rows[i].Dense: from the cache where the content hash is
// there, from enc in batches of cfg.Dense.Lo otherwise, and writes what it
// computed back. It returns how many rows needed the encoder. enc's model
// and dims must be the configuration's.
func NhungHang(ctx context.Context, enc NhungTaiLieu, cache BoNhoNhung, cfg CauHinh, rows []Hang) (int, error) {
	if enc.Dims() != cfg.Dense.Dims {
		return 0, fmt.Errorf("%w: encoder has %d dims, configuration %d", ErrCauHinh, enc.Dims(), cfg.Dense.Dims)
	}
	task := TaskTaiLieu(cfg)
	var hashes []string
	seen := map[string]bool{}
	for _, r := range rows {
		if !seen[r.ContentHash] {
			seen[r.ContentHash] = true
			hashes = append(hashes, r.ContentHash)
		}
	}
	sort.Strings(hashes)
	have := map[string][]float32{}
	if cache != nil && len(hashes) > 0 {
		got, err := cache.LayNhung(ctx, enc.Model(), enc.Dims(), task, hashes)
		if err != nil {
			return 0, err
		}
		for h, v := range got {
			if len(v) == enc.Dims() {
				have[h] = v
			}
		}
	}
	var need []int
	queued := map[string]bool{}
	for i, r := range rows {
		if _, ok := have[r.ContentHash]; !ok && !queued[r.ContentHash] {
			queued[r.ContentHash] = true
			need = append(need, i)
		}
	}
	fresh := map[string][]float32{}
	for start := 0; start < len(need); start += cfg.Dense.Lo {
		idx := need[start:min(start+cfg.Dense.Lo, len(need))]
		docs := make([]TaiLieu, len(idx))
		for j, i := range idx {
			docs[j] = TaiLieu{TieuDe: rows[i].TieuDe, Chu: rows[i].Text}
		}
		vecs, err := enc.NhungTaiLieu(ctx, docs)
		if err != nil {
			return 0, err
		}
		if len(vecs) != len(idx) {
			return 0, fmt.Errorf("%w: %d vectors for %d documents", ErrVector, len(vecs), len(idx))
		}
		for j, i := range idx {
			v, err := KiemVector(vecs[j], cfg.Dense.Dims)
			if err != nil {
				return 0, err
			}
			fresh[rows[i].ContentHash] = v
			have[rows[i].ContentHash] = v
		}
	}
	if cache != nil && len(fresh) > 0 {
		if err := cache.GhiNhung(ctx, enc.Model(), enc.Dims(), task, fresh); err != nil {
			return 0, err
		}
	}
	for i := range rows {
		rows[i].Dense = have[rows[i].ContentHash]
		rows[i].DenseModel = enc.Model()
	}
	return len(fresh), nil
}

// f32Bytes encodes a vector as little-endian float32, the cache's storage.
func f32Bytes(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func bytesF32(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}
