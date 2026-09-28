package tools

import (
	"errors"
	"strconv"
	"strings"
	"sync"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// ErrHetLuotGoi: the turn has spent MaxToolCallsPerTurn.
var ErrHetLuotGoi = errors.New("tools: tool call budget for this turn is spent")

// SoCai is one turn's ledger: every evidence item a tool returned, by id,
// with the tool that produced it first, and the tool call counts. Grounding
// (kiemchung.KiemTra) checks an answer by set membership against it. Safe
// for concurrent use: ADK runs a step's tool calls in parallel.
type SoCai struct {
	mu    sync.Mutex
	max   int
	tong  int
	dem   map[Ten]int
	muc   map[string]mucSoCai
	thuTu []string
	lap   map[string]bool
	// bi and tuBi map evidence ids to their aliases and back; soBi counts
	// aliases per prefix.
	bi, tuBi map[string]string
	soBi     map[string]int
}

// tiepDau is the alias prefix of each source: p1, p2… for places, m1… for
// manual sections, f1… for facts, g1… for group history, d1… for a couple's
// shared taste, b1… otherwise.
var tiepDau = map[truyhoi.Nguon]string{
	truyhoi.Places: "p", truyhoi.Manual: "m", truyhoi.Memory: "f", truyhoi.GroupHistory: "g", truyhoi.GuDoi: "d",
}

// GuDaDung are the people whose taste this turn's ledger holds (the couple's
// tool returned it: the model read it), by person id, in the order it came.
// Every one of them is re-checked before the answer is published and named
// on its card (ADR-0048 §5).
func (s *SoCai) GuDaDung() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, id := range s.thuTu {
		m := s.muc[id]
		if m.bc.Nguon != truyhoi.GuDoi {
			continue
		}
		if nguoi, ok := strings.CutPrefix(id, IDGuDoi); ok && nguoi != idGuChung {
			out = append(out, nguoi)
		}
	}
	return out
}

type mucSoCai struct {
	bc truyhoi.BangChung
	tu Ten
}

// MoiSoCai is an empty ledger with room for bot's tool calls:
// llm.MaxToolCallsNep for Nếp, llm.MaxToolCallsPerTurn otherwise.
func MoiSoCai(bot obs.Bot) *SoCai {
	max := llm.MaxToolCallsPerTurn
	if bot == obs.BotNep {
		max = llm.MaxToolCallsNep
	}
	return &SoCai{max: max, dem: map[Ten]int{}, muc: map[string]mucSoCai{}, lap: map[string]bool{},
		bi: map[string]string{}, tuBi: map[string]string{}, soBi: map[string]int{}}
}

// LapLai reports whether a call of t with these arguments was already made
// this turn, and records it. thamSo must be the arguments as canonical JSON
// (keys sorted, as encoding/json marshals a map); an identical repeat is
// answered from the earlier result with {"lap_lai":true} instead of running
// again. This is structural equality of a call, not a reading of its words.
func (s *SoCai) LapLai(t Ten, thamSo []byte) bool {
	k := string(t) + "\x00" + string(thamSo)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lap[k] {
		return true
	}
	s.lap[k] = true
	return false
}

// Giu counts one call of t before it runs; the call past the budget is
// refused with ErrHetLuotGoi and not counted.
func (s *SoCai) Giu(t Ten) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tong >= s.max {
		return ErrHetLuotGoi
	}
	s.tong++
	s.dem[t]++
	return nil
}

// Ghi records the evidence t returned. An id already in the ledger keeps its
// first record: the same place from a second search is the same evidence.
// An item with an empty id is not evidence and is dropped.
func (s *SoCai) Ghi(t Ten, bc []truyhoi.BangChung) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range bc {
		if b.ID == "" {
			continue
		}
		if _, ok := s.muc[b.ID]; ok {
			continue
		}
		s.muc[b.ID] = mucSoCai{bc: b, tu: t}
		dau, ok := tiepDau[b.Nguon]
		if !ok {
			dau = "b"
		}
		s.soBi[dau]++
		bi := dau + strconv.Itoa(s.soBi[dau])
		s.bi[b.ID], s.tuBi[bi] = bi, b.ID
		s.thuTu = append(s.thuTu, b.ID)
	}
}

// BiDanh is the alias of evidence id (p1, m2, …). Prompts show evidence
// under aliases only and offer them as a closed enum, so the model cannot
// write a real id at all and an invented one is a schema error (research
// reflection-verification §5.2).
func (s *SoCai) BiDanh(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bi[id]
	return b, ok
}

// TuBiDanh is the evidence id behind an alias.
func (s *SoCai) TuBiDanh(bi string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.tuBi[bi]
	return id, ok
}

// Co reports whether id is evidence of this turn.
func (s *SoCai) Co(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.muc[id]
	return ok
}

// Lay returns the evidence id and the tool that produced it.
func (s *SoCai) Lay(id string) (truyhoi.BangChung, Ten, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.muc[id]
	return m.bc, m.tu, ok
}

// IDs are the evidence ids in the order they were first recorded.
func (s *SoCai) IDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.thuTu...)
}

// SoGoi is how many calls of t were counted.
func (s *SoCai) SoGoi(t Ten) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dem[t]
}

// TongGoi is how many tool calls were counted.
func (s *SoCai) TongGoi() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tong
}
