package nhung

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		if len(v) != Dims || Dims != 3072 {
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
	if _, err := (Stub{}).Nhung(ctx, []string{"x"}, "RETRIEVAL_QUERY"); !errors.Is(err, ErrTacVu) {
		t.Fatal("a task outside the closed set was accepted")
	}
}

func TestDinhDangWritesTheTaskIntoTheText(t *testing.T) {
	for _, c := range []struct {
		tv          TacVu
		title, text string
		want        string
	}{
		{CauHoi, "", "  quán cà phê  ", "task: search result | query: quán cà phê"},
		{TaiLieu, "", "mở cửa 7h", "title: none | text: mở cửa 7h"},
		{TaiLieu, "Cà Phê Dốc", "yên tĩnh", "title: Cà Phê Dốc | text: yên tĩnh"},
		{GiongNhau, "", "a", "task: sentence similarity | query: a"},
		{HoiDap, "", "chia bill thế nào", "task: question answering | query: chia bill thế nào"},
		// NFD input comes out NFC: the cache key and the model see one form.
		{CauHoi, "", "Đà Lạt", "task: search result | query: Đà Lạt"},
	} {
		got, err := DinhDang(c.tv, c.title, c.text)
		if err != nil || got != c.want {
			t.Errorf("DinhDang(%s,%q,%q) = %q, %v; want %q", c.tv, c.title, c.text, got, err, c.want)
		}
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

// fakeGemini is a loopback batchEmbedContents that records every request
// body and answers dims-long unnormalised vectors.
type fakeGemini struct {
	mu     sync.Mutex
	bodies []map[string]any
	paths  []string
	dims   int
}

func (f *fakeGemini) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	_ = json.Unmarshal(body, &req)
	f.mu.Lock()
	f.bodies = append(f.bodies, req)
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()
	n := 0
	if reqs, ok := req["requests"].([]any); ok {
		n = len(reqs)
	}
	embs := make([]map[string]any, n)
	for i := range embs {
		vals := make([]float64, f.dims)
		vals[i%f.dims] = 3
		vals[(i+1)%f.dims] = 4
		embs[i] = map[string]any{"values": vals}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
}

// The wire contract with the provider, pinned on the request body the SDK
// sends: batchEmbedContents on gemini-embedding-2, every request carrying the
// prefixed text and outputDimensionality 3072, and no taskType or title
// anywhere (research gemini-embedding-2 §Kiểm chứng, recommendations 1–2). A
// change of SDK that moves or drops a field turns this red.
func TestGeminiWireContract(t *testing.T) {
	f := &fakeGemini{dims: Dims}
	srv := httptest.NewServer(f)
	defer srv.Close()
	g, err := NewGemini(context.Background(), "test-key", srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	texts := make([]string, MaxBatch+3)
	for i := range texts {
		texts[i] = "quán " + strings.Repeat("a", i+1)
	}
	vs, err := g.Nhung(context.Background(), texts, CauHoi)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != len(texts) || len(f.bodies) != 2 || g.SoGoi() != 2 {
		t.Fatalf("got %d vectors in %d requests (counter %d)", len(vs), len(f.bodies), g.SoGoi())
	}
	if n := Cosine(vs[0], vs[0]); math.Abs(n-1) > 1e-5 {
		t.Fatalf("provider vector not normalised: %v", n)
	}
	if _, err := g.NhungTaiLieu(context.Background(), []TaiLieuVao{{TieuDe: "Cà Phê Dốc", NoiDung: "yên tĩnh"}, {NoiDung: "không tên"}}); err != nil {
		t.Fatal(err)
	}
	wantText := []string{"task: search result | query: quán a", "title: Cà Phê Dốc | text: yên tĩnh"}
	seen := 0
	for bi, body := range f.bodies {
		if !strings.HasSuffix(f.paths[bi], "models/"+Model+":batchEmbedContents") {
			t.Errorf("request %d went to %s", bi, f.paths[bi])
		}
		raw, _ := json.Marshal(body)
		for _, banned := range []string{"taskType", "task_type", `"title"`} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("request %d carries %s: %s", bi, banned, raw[:min(len(raw), 300)])
			}
		}
		for _, r := range body["requests"].([]any) {
			req := r.(map[string]any)
			if req["model"] != "models/"+Model {
				t.Errorf("model %v", req["model"])
			}
			if d, _ := req["outputDimensionality"].(float64); d != Dims {
				t.Errorf("outputDimensionality %v", req["outputDimensionality"])
			}
			parts := req["content"].(map[string]any)["parts"].([]any)
			text := parts[0].(map[string]any)["text"].(string)
			for _, w := range wantText {
				if text == w {
					seen++
				}
			}
			if !strings.HasPrefix(text, "task: search result | query: ") && !strings.HasPrefix(text, "title: ") {
				t.Errorf("text without its task prefix: %q", text)
			}
		}
	}
	if seen != len(wantText) {
		t.Fatalf("saw %d of the expected prefixed texts", seen)
	}
}

// A provider that answers another dimensionality (1536 values here):
// refused before any vector reaches a store.
func TestGeminiRefusesAVectorOfTheWrongLength(t *testing.T) {
	srv := httptest.NewServer(&fakeGemini{dims: 1536})
	defer srv.Close()
	g, err := NewGemini(context.Background(), "test-key", srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Nhung(context.Background(), []string{"x"}, CauHoi); !errors.Is(err, ErrSaiChieu) {
		t.Fatalf("a 1536-long vector was accepted: %v", err)
	}
}

type demGoi struct {
	Stub
	n int
}

func (d *demGoi) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	d.n++
	return d.Stub.Nhung(ctx, texts, tv)
}

func TestDemCapsTheTurnAndRemembersQueries(t *testing.T) {
	ctx := context.Background()
	inner := &demGoi{}
	d := NewDem(inner, 2)
	if _, err := d.Nhung(ctx, []string{"quán lẩu"}, CauHoi); err != nil {
		t.Fatal(err)
	}
	// The same query again, in NFD and with spaces: no request.
	if _, err := d.Nhung(ctx, []string{" quán lẩu "}, CauHoi); err != nil {
		t.Fatal(err)
	}
	if inner.n != 1 || d.SoGoi() != 1 {
		t.Fatalf("a remembered query cost a request: inner %d, counter %d", inner.n, d.SoGoi())
	}
	if _, err := d.Nhung(ctx, []string{"quán nướng"}, CauHoi); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Nhung(ctx, []string{"quán chay"}, CauHoi); !errors.Is(err, ErrHetNganSach) {
		t.Fatalf("the third request went out: %v", err)
	}
	if inner.n != 2 {
		t.Fatalf("inner saw %d requests", inner.n)
	}
}

// lechSo answers a wrong number of vectors, or a short one.
type lechSo struct {
	Stub
	them, ngan bool
}

func (l lechSo) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	vs, err := l.Stub.Nhung(ctx, texts, tv)
	switch {
	case l.them:
		vs = append(vs, vs[0])
	case l.ngan:
		vs[0] = vs[0][:10]
	default:
		vs = vs[:len(vs)-1]
	}
	return vs, err
}

func (l lechSo) NhungTaiLieu(ctx context.Context, docs []TaiLieuVao) ([][]float32, error) {
	vs, err := l.Stub.NhungTaiLieu(ctx, docs)
	return vs[:len(vs)-1], err
}

// Dem refuses an answer whose vector count or length is not the request's:
// an extra vector would index past the request (a panic before), a missing
// one would leave a nil dense vector nobody flagged.
func TestDemChecksTheVectorCount(t *testing.T) {
	ctx := context.Background()
	for name, inner := range map[string]lechSo{"extra": {them: true}, "missing": {}, "short": {ngan: true}} {
		d := NewDem(inner, 4)
		out, err := d.Nhung(ctx, []string{"quán lẩu", "quán nướng"}, CauHoi)
		if err == nil || out != nil {
			t.Errorf("%s: answer accepted: %d vectors, %v", name, len(out), err)
		}
	}
	d := NewDem(lechSo{}, 4)
	if out, err := d.NhungTaiLieu(ctx, []TaiLieuVao{{NoiDung: "a"}, {NoiDung: "b"}}); !errors.Is(err, ErrSoVector) || out != nil {
		t.Errorf("documents: a missing vector accepted: %d, %v", len(out), err)
	}
}
