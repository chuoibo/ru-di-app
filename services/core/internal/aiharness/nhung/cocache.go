package nhung

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"sync"
	"sync/atomic"
)

// The query-vector cache (plan P5): a search embeds the person's query, the
// slowest step of a search (~650 ms to Gemini). The same text under the same
// model, dimensionality and task always embeds to the same vector, so a
// repeated query is answered from memory. The stack runs one core, so the
// cache is in the process (an LRU, ~25 MB at 2,048 vectors of 3,072
// floats); a second replica would want a shared one.
//
// It holds no text: keys are sha256 of the model, the dimensionality and
// the exact prompt the model would see (DinhDang: the task prefix included,
// so one text under two tasks is two entries). Only queries go through it
// (Nhung); documents pass straight to the inner embedder. An error is never
// cached. Concurrent misses on one key make one provider call.

// MacDinhMucCache is the cache's default size.
const MacDinhMucCache = 2048

// BoNhoCau is the cache's store, shared by every CoCache over it. Safe for
// concurrent use.
type BoNhoCau struct {
	max int

	mu      sync.Mutex
	ll      *list.List
	m       map[[32]byte]*list.Element
	dangGoi map[[32]byte]*cuocGoi

	trung, truot atomic.Int64
}

type mucCache struct {
	k [32]byte
	v []float32
}

type cuocGoi struct {
	xong chan struct{}
	v    []float32
	err  error
}

// MoiBoNhoCau is a store of at most max vectors (MacDinhMucCache when max
// is 0 or less).
func MoiBoNhoCau(max int) *BoNhoCau {
	if max <= 0 {
		max = MacDinhMucCache
	}
	return &BoNhoCau{max: max, ll: list.New(), m: map[[32]byte]*list.Element{}, dangGoi: map[[32]byte]*cuocGoi{}}
}

// Trung and Truot count hits and misses (content-free).
func (b *BoNhoCau) Trung() int64 { return b.trung.Load() }
func (b *BoNhoCau) Truot() int64 { return b.truot.Load() }

func (b *BoNhoCau) lay(k [32]byte) ([]float32, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.m[k]
	if !ok {
		return nil, false
	}
	b.ll.MoveToFront(e)
	return e.Value.(*mucCache).v, true
}

func (b *BoNhoCau) dat(k [32]byte, v []float32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if e, ok := b.m[k]; ok {
		e.Value.(*mucCache).v = v
		b.ll.MoveToFront(e)
		return
	}
	b.m[k] = b.ll.PushFront(&mucCache{k: k, v: v})
	for b.ll.Len() > b.max {
		last := b.ll.Back()
		b.ll.Remove(last)
		delete(b.m, last.Value.(*mucCache).k)
	}
}

// CoCache is Inner with its queries answered from Bo where it can. Wrap it
// OUTSIDE the turn's budget (TheoLuot): a hit costs the turn no embedding
// call.
type CoCache struct {
	Inner Nhung
	Bo    *BoNhoCau
}

func (c CoCache) Model() string { return c.Inner.Model() }
func (c CoCache) Dims() int     { return c.Inner.Dims() }
func (c CoCache) SoGoi() int64  { return c.Inner.SoGoi() }

// NhungTaiLieu is the inner embedder's: documents are not cached here.
func (c CoCache) NhungTaiLieu(ctx context.Context, docs []TaiLieuVao) ([][]float32, error) {
	return c.Inner.NhungTaiLieu(ctx, docs)
}

func (c CoCache) khoa(tv TacVu, text string) ([32]byte, error) {
	p, err := DinhDang(tv, "", text)
	if err != nil {
		return [32]byte{}, err
	}
	h := sha256.New()
	h.Write([]byte(c.Inner.Model()))
	h.Write([]byte{0})
	var d [8]byte
	binary.LittleEndian.PutUint64(d[:], uint64(c.Inner.Dims()))
	h.Write(d[:])
	h.Write([]byte(p))
	var k [32]byte
	copy(k[:], h.Sum(nil))
	return k, nil
}

// Nhung answers each text from the store, and embeds the rest in one call to
// Inner (one call per text already in flight elsewhere is joined, not
// repeated). Every vector returned is the caller's own copy.
func (c CoCache) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	if c.Bo == nil {
		return c.Inner.Nhung(ctx, texts, tv)
	}
	keys := make([][32]byte, len(texts))
	out := make([][]float32, len(texts))
	var can []int // indexes this call embeds
	var cho []*cuocGoi
	var choIdx []int
	mine := map[[32]byte]*cuocGoi{}
	for i, t := range texts {
		k, err := c.khoa(tv, t)
		if err != nil {
			return nil, err
		}
		keys[i] = k
		if v, ok := c.Bo.lay(k); ok {
			c.Bo.trung.Add(1)
			out[i] = slices.Clone(v)
			continue
		}
		c.Bo.mu.Lock()
		if g, ok := c.Bo.dangGoi[k]; ok {
			c.Bo.mu.Unlock()
			if _, own := mine[k]; !own {
				c.Bo.trung.Add(1)
			}
			cho, choIdx = append(cho, g), append(choIdx, i)
			continue
		}
		g := &cuocGoi{xong: make(chan struct{})}
		c.Bo.dangGoi[k] = g
		c.Bo.mu.Unlock()
		mine[k] = g
		c.Bo.truot.Add(1)
		can = append(can, i)
	}
	if len(can) > 0 {
		in := make([]string, len(can))
		for j, i := range can {
			in[j] = texts[i]
		}
		vs, err := c.Inner.Nhung(ctx, in, tv)
		if err == nil && len(vs) != len(in) {
			err = ErrRong
		}
		for j, i := range can {
			g := mine[keys[i]]
			if err != nil {
				g.err = err
			} else {
				g.v = vs[j]
				c.Bo.dat(keys[i], vs[j])
				out[i] = slices.Clone(vs[j])
			}
		}
		c.Bo.mu.Lock()
		for _, i := range can {
			delete(c.Bo.dangGoi, keys[i])
		}
		c.Bo.mu.Unlock()
		for _, i := range can {
			if g := mine[keys[i]]; g.xong != nil {
				select {
				case <-g.xong:
				default:
					close(g.xong)
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}
	for j, g := range cho {
		select {
		case <-g.xong:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if g.err != nil {
			return nil, g.err
		}
		out[choIdx[j]] = slices.Clone(g.v)
	}
	return out, nil
}
