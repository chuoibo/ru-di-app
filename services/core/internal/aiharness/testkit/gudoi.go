package testkit

import (
	"context"
	"sync"

	"mobile/services/core/internal/gudoi"
)

// GuDoi is an in-memory tools.DocDoi: the taste the port would return for
// any room (the eligibility decision itself is gudoi's and aidoc's, tested
// there); it records which rooms were read.
type GuDoi struct {
	mu  sync.Mutex
	Gu  []gudoi.Gu
	Hoi []string
}

// GuDoi returns a copy of Gu.
func (g *GuDoi) GuDoi(_ context.Context, phong string) ([]gudoi.Gu, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Hoi = append(g.Hoi, phong)
	out := make([]gudoi.Gu, 0, len(g.Gu))
	for _, x := range g.Gu {
		out = append(out, gudoi.Gu{NguoiID: x.NguoiID, The: append([]string(nil), x.The...)})
	}
	return out, nil
}

// SoLanHoi is how many times the port was read.
func (g *GuDoi) SoLanHoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.Hoi)
}
