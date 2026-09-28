//go:build milvus

package hybrid_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

// rerankGia serves the /rerank contract on loopback with deterministic
// scores: the share of the query's words a document contains, less a small
// step by position so no two scores tie. The tier checks the wiring (the
// order follows the scores, one call per turn, nothing degraded), not a
// model's judgement, and no test may call a paid provider (ADR-0049 §4).
// It returns the base URL and the number of calls served.
func rerankGia(t *testing.T) (string, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rerank" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var in struct {
			Query     string   `json:"query"`
			Documents []string `json:"documents"`
			TopN      int      `json:"top_n"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		calls.Add(1)
		words := strings.Fields(strings.ToLower(in.Query))
		type ket struct {
			Index int     `json:"index"`
			Score float64 `json:"relevance_score"`
		}
		out := make([]ket, len(in.Documents))
		for i, d := range in.Documents {
			d = strings.ToLower(d)
			hit := 0
			for _, w := range words {
				if strings.Contains(d, w) {
					hit++
				}
			}
			score := 0.0
			if len(words) > 0 {
				score = float64(hit) / float64(len(words))
			}
			out[i] = ket{Index: i, Score: score*0.9 + 0.09/float64(i+2)}
		}
		sort.SliceStable(out, func(a, b int) bool { return out[a].Score > out[b].Score })
		if in.TopN > 0 && in.TopN < len(out) {
			out = out[:in.TopN]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": out})
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &calls
}
