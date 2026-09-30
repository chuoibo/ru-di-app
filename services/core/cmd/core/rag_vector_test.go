package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/rag/nap"
)

// Arguments are checked before any connection opens; enrichment has no
// default ceiling; a verdict names the version it was given.
func TestRagVectorArgumentsAreCheckedFirst(t *testing.T) {
	for _, args := range [][]string{
		{"v-build"}, {"v-build", "nep"}, {"v-build", "place", "--fast"}, {"v-eval", "x"}, {"v-promote", "0"},
		{"v-rollback"}, {"v-enrich"}, {"v-enrich", "--tran-goi", "0"}, {"v-enrich", "--tran-goi", "5000"},
		{"v-review"}, {"v-review", "approve"}, {"v-review", "approve", "p1"}, {"v-dlq", "drop"}, {"v-status", "x"}, {"v-bogus"},
	} {
		var out, errb bytes.Buffer
		if code := run(append([]string{"rag"}, args...), func(string) string { return "" }, &out, &errb); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	for _, ok := range [][]string{{"v-build", "place", "--auto"}, {"v-build", "manual"}, {"v-enrich", "--tran-goi", "40"},
		{"v-review", "list", "--all"}, {"v-review", "reject", "p1", "0123456789abcdef"}, {"v-dlq", "retry"}, {"v-rollback", "place"}} {
		if _, err := parseRagVector(ok); err != nil {
			t.Errorf("%v refused: %v", ok, err)
		}
	}
}

// geminiGhi is a loopback Gemini batchEmbedContents that records the text
// of every request.
type geminiGhi struct {
	mu    sync.Mutex
	texts []string
}

func (g *geminiGhi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body struct {
		Requests []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"requests"`
	}
	_ = json.Unmarshal(raw, &body)
	embs := make([]map[string]any, len(body.Requests))
	g.mu.Lock()
	for i, req := range body.Requests {
		for _, p := range req.Content.Parts {
			g.texts = append(g.texts, p.Text)
		}
		vals := make([]float64, nhung.Dims)
		vals[i%nhung.Dims] = 1
		embs[i] = map[string]any{"values": vals}
	}
	g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
}

// F4: the ingest's dense adapter over the REAL embedder (aiharness/nhung's
// Gemini client, against a loopback server) puts gemini-embedding-2's task
// prefix on the wire exactly once, for documents and for queries: the
// prefix belongs to nhung alone.
func TestNapDenseMotTienTo(t *testing.T) {
	f := &geminiGhi{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	g, err := nhung.NewGemini(context.Background(), "test-key", srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	d := napDense{e: g}
	if _, err := d.NhungTaiLieu(context.Background(), []nap.TaiLieu{{TieuDe: "Quán A", Chu: "lẩu"}, {Chu: "phở"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.NhungCauHoi(context.Background(), []string{"lẩu"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"title: Quán A | text: lẩu", "title: none | text: phở", "task: search result | query: lẩu"}
	if strings.Join(f.texts, "\n") != strings.Join(want, "\n") {
		t.Fatalf("wire texts %q, want %q", f.texts, want)
	}
	for _, s := range f.texts {
		if strings.Count(s, "title:")+strings.Count(s, "task:") != 1 {
			t.Fatalf("a prefix written twice: %q", s)
		}
	}
}

// The production encoder refuses a door that serves another model or
// dimensionality than the committed configuration.
func TestRagDenseTuChoiLechCauHinh(t *testing.T) {
	cfg, _ := nap.MacDinh()
	if cfg.Dense.Model != nhung.Model || cfg.Dense.Dims != nhung.Dims {
		t.Fatalf("the committed configuration %s/%d is not the embedding door's %s/%d", cfg.Dense.Model, cfg.Dense.Dims, nhung.Model, nhung.Dims)
	}
	off := cfg
	off.Dense.Dims = 1536
	_, err := ragDense(context.Background(), func(k string) string {
		switch k {
		case "GEMINI_API_KEY":
			return "k"
		case "MOBILE_GEMINI_BASE_URL":
			return "http://127.0.0.1:9"
		}
		return ""
	}, off)
	if err == nil || !strings.Contains(err.Error(), "committed configuration") {
		t.Fatalf("a mismatched door was accepted: %v", err)
	}
	if enc, err := ragDense(context.Background(), func(k string) string {
		if k == EnvRagDense {
			return "stub"
		}
		return ""
	}, cfg); err != nil || enc.Dims() != cfg.Dense.Dims {
		t.Fatalf("stub: %v", err)
	}
}
