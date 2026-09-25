package aieval

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
)

// Kinds of Sink event, the names of the stream events they become (design 01
// §2, contract §3).
const (
	LoaiTrangThai = "trang_thai"
	LoaiPhan      = "phan"
	LoaiDelta     = "delta"
	LoaiLamLai    = "lam_lai"
)

// SuKien is one Sink event, with the milliseconds since the recorder was
// made.
type SuKien struct {
	Loai string `json:"loai"`
	Ms   int    `json:"ms"`
	// TrangThai.
	Ma string `json:"ma,omitempty"`
	N  *int   `json:"n,omitempty"`
	// Phan (I, Kind, JSON) and Delta (P, Chu).
	I    *int            `json:"i,omitempty"`
	Kind string          `json:"kind,omitempty"`
	JSON json.RawMessage `json:"json,omitempty"`
	P    *int            `json:"p,omitempty"`
	Chu  string          `json:"chu,omitempty"`
}

// Nhan is the event as a case's su_kien expectation writes it: the kind and
// its closed fields, never the free text of a Delta or a Phan.
func (s SuKien) Nhan() string {
	switch s.Loai {
	case LoaiTrangThai:
		n := 0
		if s.N != nil {
			n = *s.N
		}
		return s.Loai + ":" + s.Ma + ":" + strconv.Itoa(n)
	case LoaiPhan:
		return s.Loai + ":" + s.Kind
	default:
		return s.Loai
	}
}

// GhiLai is a Sink that records every event. Safe for concurrent use.
type GhiLai struct {
	mu     sync.Mutex
	now    func() time.Time
	batDau time.Time
	ds     []SuKien
}

var _ aiharness.Sink = (*GhiLai)(nil)

// NewGhiLai makes a recorder timed by now (the eval passes a fixed clock in
// kich-ban mode, so a run is repeatable byte for byte).
func NewGhiLai(now func() time.Time) *GhiLai {
	if now == nil {
		now = time.Now
	}
	return &GhiLai{now: now, batDau: now()}
}

func (g *GhiLai) them(s SuKien) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s.Ms = int(g.now().Sub(g.batDau) / time.Millisecond)
	g.ds = append(g.ds, s)
}

func intp(v int) *int { return &v }

// TrangThai records a status.
func (g *GhiLai) TrangThai(ma cau.TrangThai, n int) {
	g.them(SuKien{Loai: LoaiTrangThai, Ma: string(ma), N: intp(n)})
}

// Phan records a grounded part.
func (g *GhiLai) Phan(i int, kind aiharness.PhanKind, v json.RawMessage) {
	g.them(SuKien{Loai: LoaiPhan, I: intp(i), Kind: string(kind), JSON: append(json.RawMessage(nil), v...)})
}

// Delta records released text.
func (g *GhiLai) Delta(p int, text string) {
	g.them(SuKien{Loai: LoaiDelta, P: intp(p), Chu: text})
}

// LamLai records the restart marker.
func (g *GhiLai) LamLai() { g.them(SuKien{Loai: LoaiLamLai}) }

// SuKien returns a copy of every event, in order.
func (g *GhiLai) SuKien() []SuKien {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]SuKien(nil), g.ds...)
}
