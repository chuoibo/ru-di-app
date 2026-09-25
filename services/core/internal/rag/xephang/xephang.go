// Package xephang ranks short texts by their words, in memory, with no SQL and
// no model: the lexical half of retrieval that the app manual (huongdan), the
// place catalogue's search text (rag) and Nếp's memory (nepnho) all share, so
// «cà phê» and «ca phe» mean one thing everywhere.
//
// A text becomes terms in three steps. It is folded exactly as the prompt
// safety oracle folds it (promptsafety.Fold: marks dropped, đ as d, lower
// case), so a query typed without diacritics finds a text written with them.
// It is cut into syllables at every character that is not a letter or a
// digit. And every pair of neighbouring syllables is added as one more term,
// joined by an underscore («ca_phe»): Vietnamese builds words from syllables,
// and the pair is what tells «cà phê» from «phê duyệt» and «bún chả» from
// «chả giò». Ranking is BM25 over those terms, ties broken by id, so the same
// index and query give the same list on every run.
package xephang

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"mobile/services/core/internal/domain/promptsafety"
)

// BM25 constants. The usual ones; the golden sets that use this package pin
// the resulting order, so changing either is a change those sets must show.
const (
	k1 = 1.2
	b  = 0.75
)

// AmTiet folds text and cuts it into syllables, in order.
func AmTiet(text string) []string {
	return strings.FieldsFunc(promptsafety.Fold(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// Thuat returns the terms of text: its syllables, then each pair of
// neighbouring syllables joined by an underscore.
func Thuat(text string) []string {
	syllables := AmTiet(text)
	terms := make([]string, 0, 2*len(syllables))
	terms = append(terms, syllables...)
	for i := 1; i < len(syllables); i++ {
		terms = append(terms, syllables[i-1]+"_"+syllables[i])
	}
	return terms
}

// ChuTimKiem is the terms of text as one space-separated string: what a
// full-text column stores so SQL matches the same terms this package ranks.
func ChuTimKiem(text string) string {
	return strings.Join(Thuat(text), " ")
}

// Doan is one text to rank: an id the caller owns and the words to match.
type Doan struct {
	ID  string
	Chu string
}

// KetQua is one ranked hit.
type KetQua struct {
	ID   string
	Diem float64
}

// ChiMuc is an immutable index over a fixed set of texts. Safe for concurrent
// use once built.
type ChiMuc struct {
	ids    []string
	tf     []map[string]int
	length []int
	df     map[string]int
	avgLen float64
}

// Dung builds the index. Ids must be unique; a repeated id is a caller bug
// and panics rather than ranking two texts under one name.
func Dung(doan []Doan) *ChiMuc {
	m := &ChiMuc{df: map[string]int{}}
	seen := map[string]bool{}
	total := 0
	for _, d := range doan {
		if seen[d.ID] {
			panic("xephang: duplicate id " + d.ID)
		}
		seen[d.ID] = true
		counts := map[string]int{}
		terms := Thuat(d.Chu)
		for _, t := range terms {
			counts[t]++
		}
		for t := range counts {
			m.df[t]++
		}
		m.ids = append(m.ids, d.ID)
		m.tf = append(m.tf, counts)
		m.length = append(m.length, len(terms))
		total += len(terms)
	}
	if len(doan) > 0 {
		m.avgLen = float64(total) / float64(len(doan))
	}
	return m
}

// Len is how many texts the index holds.
func (m *ChiMuc) Len() int { return len(m.ids) }

// Tim ranks the texts against query and returns at most k hits with a
// positive score, best first, ties broken by id. A query term repeated counts
// once: repeating a word is not asking for it harder.
func (m *ChiMuc) Tim(query string, k int) []KetQua {
	if k <= 0 || len(m.ids) == 0 {
		return nil
	}
	var terms []string
	seen := map[string]bool{}
	for _, t := range Thuat(query) {
		if !seen[t] && m.df[t] > 0 {
			seen[t] = true
			terms = append(terms, t)
		}
	}
	if len(terms) == 0 {
		return nil
	}
	n := float64(len(m.ids))
	var hits []KetQua
	for i, counts := range m.tf {
		score := 0.0
		for _, t := range terms {
			f := float64(counts[t])
			if f == 0 {
				continue
			}
			df := float64(m.df[t])
			idf := math.Log(1 + (n-df+0.5)/(df+0.5))
			norm := 1 - b + b*float64(m.length[i])/m.avgLen
			score += idf * f * (k1 + 1) / (f + k1*norm)
		}
		if score > 0 {
			hits = append(hits, KetQua{ID: m.ids[i], Diem: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Diem != hits[j].Diem {
			return hits[i].Diem > hits[j].Diem
		}
		return hits[i].ID < hits[j].ID
	})
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}
