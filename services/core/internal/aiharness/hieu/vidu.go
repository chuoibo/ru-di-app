package hieu

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/obs"
)

// Dynamic few-shot (research intent-routing §4.7): the router is shown the
// K worked examples closest to the message by EMBEDDING similarity, at most
// MaxViDuMoiYDinh per first intent so one label cannot crowd the prompt.
// Never chosen by words. The bank is synthetic and written by hand; no
// person's message is in it.
const (
	SoViDu          = 4
	MaxViDuMoiYDinh = 2
)

//go:embed vi_du.json
var viDuJSON []byte

// ViDu is one worked example: a message and the router output for it.
type ViDu struct {
	Bot obs.Bot
	Cau string
	// Ra is the compact JSON a router should write; Doc accepts it.
	Ra json.RawMessage
	// dau is the first intent, for the diversity bound.
	dau YDinh
}

// luc is the instant examples are checked against: their outputs carry no
// date, so any valid instant does.
var luc = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// DocViDu reads and checks a bank: every output must pass Doc for its bot
// with empty turn lists, so no example can teach an id the turn did not give.
func DocViDu(raw []byte) ([]ViDu, error) {
	var tho struct {
		GhiChu string `json:"ghi_chu"`
		ViDu   []struct {
			Bot string          `json:"bot"`
			Cau string          `json:"cau"`
			Ra  json.RawMessage `json:"ra"`
		} `json:"vi_du"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tho); err != nil {
		return nil, fmt.Errorf("hieu: example bank: %w", err)
	}
	var out []ViDu
	seen := map[string]bool{}
	for i, d := range tho.ViDu {
		bot := obs.Bot(d.Bot)
		kq, err := Doc(d.Ra, Vao{Bot: bot, Cau: d.Cau, Luc: luc})
		if err != nil {
			return nil, fmt.Errorf("hieu: example %d: %w", i+1, err)
		}
		if d.Cau == "" || seen[d.Bot+"\x00"+d.Cau] {
			return nil, fmt.Errorf("hieu: example %d is empty or repeated", i+1)
		}
		seen[d.Bot+"\x00"+d.Cau] = true
		var buf bytes.Buffer
		if err := json.Compact(&buf, d.Ra); err != nil {
			return nil, err
		}
		out = append(out, ViDu{Bot: bot, Cau: d.Cau, Ra: buf.Bytes(), dau: kq.YDinh[0]})
	}
	return out, nil
}

// ViDuMacDinh is the embedded bank, checked at init.
var ViDuMacDinh = func() []ViDu {
	v, err := DocViDu(viDuJSON)
	if err != nil {
		panic(err)
	}
	return v
}()

// KhoViDu is a bank with its vectors, embedded once when it is built.
type KhoViDu struct {
	nhung nhung.Nhung
	vd    []ViDu
	vec   [][]float32
}

// ErrNhung: the embedder failed or answered with the wrong shape.
var ErrNhung = errors.New("hieu: example embedding failed")

// MoiKhoViDu embeds the bank's messages (one batch per nhung.MaxBatch).
func MoiKhoViDu(ctx context.Context, n nhung.Nhung, vd []ViDu) (*KhoViDu, error) {
	k := &KhoViDu{nhung: n, vd: vd}
	for i := 0; i < len(vd); i += nhung.MaxBatch {
		end := min(i+nhung.MaxBatch, len(vd))
		var texts []string
		for _, d := range vd[i:end] {
			texts = append(texts, d.Cau)
		}
		vecs, err := n.Nhung(ctx, texts, nhung.GiongNhau)
		if err != nil || len(vecs) != len(texts) {
			return nil, fmt.Errorf("%w: %v", ErrNhung, err)
		}
		k.vec = append(k.vec, vecs...)
	}
	return k, nil
}

// Chon embeds v's message (one embedding call) and returns the SoViDu
// closest examples of v's bot, most similar first, at most MaxViDuMoiYDinh
// per first intent. Ties keep bank order.
func (k *KhoViDu) Chon(ctx context.Context, v Vao) ([]ViDu, error) {
	if err := v.DemNhung.Giu(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNhung, err)
	}
	// The message's embedding has a deadline of its own, well inside the
	// turn's: a slow embedder costs the examples, never the turn.
	ctx, cancel := context.WithTimeout(ctx, HanNhungCau)
	defer cancel()
	vecs, err := k.nhung.Nhung(ctx, []string{v.Cau}, nhung.GiongNhau)
	if err != nil || len(vecs) != 1 {
		return nil, fmt.Errorf("%w: %v", ErrNhung, err)
	}
	type ung struct {
		i int
		d float64
	}
	var us []ung
	for i, d := range k.vd {
		if d.Bot == v.Bot {
			us = append(us, ung{i, nhung.Cosine(vecs[0], k.vec[i])})
		}
	}
	sort.SliceStable(us, func(a, b int) bool { return us[a].d > us[b].d })
	dem := map[YDinh]int{}
	var out []ViDu
	for _, u := range us {
		d := k.vd[u.i]
		if dem[d.dau] >= MaxViDuMoiYDinh {
			continue
		}
		dem[d.dau]++
		out = append(out, d)
		if len(out) == SoViDu {
			break
		}
	}
	return out, nil
}

// HanNhungCau bounds the message's embedding for the example choice.
const HanNhungCau = 3 * time.Second

// Defaults of a lazy bank: the build's own deadline, apart from any turn,
// and how long a turn waits for a build in flight before routing without
// examples.
const (
	HanDungKho = 10 * time.Second
	ChoKho     = 2 * time.Second
)

// KhoViDuLuoi is a bank embedded on its first use, not when the process
// starts: startup opens no connection to the embedding provider. One build
// runs at a time (the first turn that needs it starts it), under its own
// deadline and detached from the turn that started it; no lock is ever
// held across the network. A turn waits for a build in flight at most
// ChoKho, then routes without examples (Router.HieuVet). A failed build
// leaves the bank unbuilt, so a later turn starts another.
type KhoViDuLuoi struct {
	n  nhung.Nhung
	vd []ViDu
	// han and cho are HanDungKho and ChoKho (tests shorten them).
	han, cho time.Duration

	mu   sync.Mutex
	kho  *KhoViDu
	dang chan struct{} // closed when the build in flight ends; nil when none
}

// MoiKhoViDuLuoi is a lazy bank of vd embedded through n.
func MoiKhoViDuLuoi(n nhung.Nhung, vd []ViDu) *KhoViDuLuoi {
	return &KhoViDuLuoi{n: n, vd: vd, han: HanDungKho, cho: ChoKho}
}

// Chon picks from the bank, starting its build when none is built or in
// flight, and waiting for a build in flight at most ChoKho.
func (k *KhoViDuLuoi) Chon(ctx context.Context, v Vao) ([]ViDu, error) {
	k.mu.Lock()
	kho, dang := k.kho, k.dang
	if kho == nil && dang == nil {
		dang = make(chan struct{})
		k.dang = dang
		go k.dung(dang)
	}
	k.mu.Unlock()
	if kho == nil {
		t := time.NewTimer(k.cho)
		defer t.Stop()
		select {
		case <-dang:
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %v", ErrNhung, ctx.Err())
		case <-t.C:
			return nil, fmt.Errorf("%w: the example bank is still being built", ErrNhung)
		}
		k.mu.Lock()
		kho = k.kho
		k.mu.Unlock()
		if kho == nil {
			return nil, fmt.Errorf("%w: the example bank could not be built", ErrNhung)
		}
	}
	return kho.Chon(ctx, v)
}

// dung builds the bank under its own deadline and publishes it.
func (k *KhoViDuLuoi) dung(dang chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), k.han)
	defer cancel()
	kho, err := MoiKhoViDu(ctx, k.n, k.vd)
	k.mu.Lock()
	if err == nil {
		k.kho = kho
	}
	k.dang = nil
	k.mu.Unlock()
	close(dang)
}
