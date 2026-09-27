//go:build milvus

package rerank

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// The retrieval tier (scripts/go_milvus_tier.sh) runs the reranker too: the
// mandatory golden check of the served model against an independent
// reference, through this adapter's own request path.

type golden struct {
	Tolerance float64 `json:"tolerance"`
	Cases     []struct {
		Name  string `json:"name"`
		Query string `json:"query"`
		Docs  []struct {
			Label string  `json:"label"`
			Text  string  `json:"text"`
			Ref   float64 `json:"ref"`
		} `json:"docs"`
	} `json:"cases"`
}

// TestRerankGoldenQuaServer: for each case, the server's P(yes) through
// Diem is within the tolerance of the numpy reference for every document,
// and the full ranking is identical. A GGUF without its classifier head, a
// template missing its suffix or a runtime regression changes scores by far
// more than 0.02 (bring-up: q8_0 worst 1.35e-2).
func TestRerankGoldenQuaServer(t *testing.T) {
	base := strings.TrimSpace(os.Getenv("MOBILE_TEST_RERANK_URL"))
	if base == "" {
		if os.Getenv("CORE_REQUIRE_MILVUS_TESTS") == "1" {
			t.Fatal("CORE_REQUIRE_MILVUS_TESTS=1 but MOBILE_TEST_RERANK_URL is empty")
		}
		t.Skip("MOBILE_TEST_RERANK_URL not set; run scripts/go_milvus_tier.sh")
	}
	raw, err := os.ReadFile("testdata/golden_qwen3_0.6b.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil || len(g.Cases) < 3 {
		t.Fatalf("golden file: %v (%d cases)", err, len(g.Cases))
	}
	q, err := Moi(base, "", 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	worst := 0.0
	for _, c := range g.Cases {
		docs := make([]string, len(c.Docs))
		for i, d := range c.Docs {
			docs[i] = d.Text
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		got, err := q.Diem(ctx, c.Query, docs)
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		order := func(score func(i int) float64) []int {
			idx := make([]int, len(docs))
			for i := range idx {
				idx[i] = i
			}
			sort.SliceStable(idx, func(a, b int) bool { return score(idx[a]) > score(idx[b]) })
			return idx
		}
		gotOrder := order(func(i int) float64 { return got[i] })
		refOrder := order(func(i int) float64 { return c.Docs[i].Ref })
		for i := range gotOrder {
			if gotOrder[i] != refOrder[i] {
				t.Errorf("%s: server ranking %v, reference %v", c.Name, gotOrder, refOrder)
				break
			}
		}
		for i, d := range c.Docs {
			diff := math.Abs(got[i] - d.Ref)
			worst = math.Max(worst, diff)
			if diff > g.Tolerance {
				t.Errorf("%s/%s: server %.5f, reference %.5f", c.Name, d.Label, got[i], d.Ref)
			}
		}
	}
	t.Logf("%d cases, worst |server - reference| = %.2e (tolerance %.2g)", len(g.Cases), worst, g.Tolerance)
}
