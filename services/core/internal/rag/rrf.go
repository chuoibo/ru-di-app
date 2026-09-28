package rag

import "sort"

// RRF constants (design 04 §5.3): k = 60, and a score in integers,
// ⌊10⁹/(60+rank)⌋, so a golden pins the fused order to the unit and a tie is
// broken by id, never by float noise.
const (
	rrfK  = 60
	rrfTu = 1_000_000_000
)

// hang is one entry of a ranked list: an id and its rank, 1 for the best.
// Ties share a rank.
type hang struct {
	ID   string
	Hang int
}

// diemRRF is one list's contribution for a rank.
func diemRRF(rank int) int64 { return rrfTu / int64(rrfK+rank) }

// fuse is reciprocal rank fusion over the lists: each id scores the sum of
// its lists' contributions; best first, ties by id.
func fuse(lists ...[]hang) []Hit {
	score := map[string]int64{}
	for _, list := range lists {
		for _, e := range list {
			score[e.ID] += diemRRF(e.Hang)
		}
	}
	out := make([]Hit, 0, len(score))
	for id, s := range score {
		out = append(out, Hit{ID: id, Diem: s})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Diem != out[j].Diem {
			return out[i].Diem > out[j].Diem
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// xepDiem turns scored ids into a ranked list: higher score is a better rank,
// equal scores share one (dense), and the list is cut to the first n ids in
// (score, id) order.
func xepDiem(scores map[string]float64, n int) []hang {
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if len(ids) > n {
		ids = ids[:n]
	}
	out := make([]hang, len(ids))
	rank := 0
	for i, id := range ids {
		if i == 0 || scores[id] != scores[ids[i-1]] {
			rank++
		}
		out[i] = hang{ID: id, Hang: rank}
	}
	return out
}
