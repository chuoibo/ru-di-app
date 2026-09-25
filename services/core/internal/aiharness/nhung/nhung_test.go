package nhung

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStubIsDeterministicNormalisedAndCloseForSharedWords(t *testing.T) {
	ctx := context.Background()
	vs, err := Stub{}.Nhung(ctx, []string{"quán cà phê view đồi", "ca phe view doi", "bún chả hà nội", ""}, TaiLieu)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Stub{}.Nhung(ctx, []string{"quán cà phê view đồi"}, CauHoi)
	for i, v := range vs {
		if len(v) != Dims {
			t.Fatalf("vector %d has %d dims", i, len(v))
		}
		if n := Cosine(v, v); math.Abs(n-1) > 1e-5 {
			t.Fatalf("vector %d not unit: %v", i, n)
		}
	}
	if Cosine(vs[0], again[0]) < 0.99999 {
		t.Fatal("stub is not deterministic across calls and tasks")
	}
	same, other := Cosine(vs[0], vs[1]), Cosine(vs[0], vs[2])
	if same <= other || same < 0.5 {
		t.Fatalf("folded twin %.3f should beat an unrelated text %.3f", same, other)
	}
}

func TestLiteralIsPgvectorText(t *testing.T) {
	if got := Literal([]float32{1, -0.5, 0.25}); got != "[1,-0.5,0.25]" {
		t.Fatalf("Literal = %q", got)
	}
	if _, err := ChuanHoa([]float32{0, 0}); err == nil {
		t.Fatal("a zero vector must be refused")
	}
}

func TestGeminiRefusesTheRealHostUnderTest(t *testing.T) {
	if _, err := NewGemini(context.Background(), "k", ""); err == nil {
		t.Fatal("a test binary must never build a client for the real provider")
	}
	if _, err := NewGemini(context.Background(), "k", "https://example.com/"); err == nil {
		t.Fatal("a non-loopback override must be refused")
	}
	if _, err := NewGemini(context.Background(), "", "http://127.0.0.1:1/"); err == nil {
		t.Fatal("no key must be refused")
	}
}

// A loopback fake of the embedContent endpoint: checks the task type and the
// dimensionality we send, and answers unnormalised vectors that the embedder
// must normalise.
func TestGeminiAgainstLoopbackFake(t *testing.T) {
	var gotTask string
	var gotDims float64
	var batches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		batches++
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		n := 1
		if reqs, ok := req["requests"].([]any); ok {
			n = len(reqs)
			if first, ok := reqs[0].(map[string]any); ok {
				gotTask, _ = first["taskType"].(string)
				gotDims, _ = first["outputDimensionality"].(float64)
			}
		}
		embs := make([]map[string]any, n)
		for i := range embs {
			vals := make([]float64, Dims)
			vals[i%Dims] = 3
			vals[(i+1)%Dims] = 4
			embs[i] = map[string]any{"values": vals}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
	}))
	defer srv.Close()
	g, err := NewGemini(context.Background(), "test-key", srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	texts := make([]string, MaxBatch+3)
	for i := range texts {
		texts[i] = strings.Repeat("a", i+1)
	}
	vs, err := g.Nhung(context.Background(), texts, TaiLieu)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != len(texts) || batches != 2 || g.SoGoi() != 2 {
		t.Fatalf("got %d vectors in %d batches (counter %d)", len(vs), batches, g.SoGoi())
	}
	if gotTask != string(TaiLieu) || gotDims != Dims {
		t.Fatalf("request carried task %q dims %v", gotTask, gotDims)
	}
	if n := Cosine(vs[0], vs[0]); math.Abs(n-1) > 1e-5 {
		t.Fatalf("provider vector not normalised: %v", n)
	}
}
