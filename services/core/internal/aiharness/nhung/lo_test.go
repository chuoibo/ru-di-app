package nhung

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFileVaoDungDinhDangOnline(t *testing.T) {
	raw, err := FileVao([]TaiLieuLo{{Khoa: "h1", TieuDe: "Quán A", NoiDung: "cà phê yên tĩnh"}, {Khoa: "h2", NoiDung: "bún bò"}})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d lines", len(lines))
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &d); err != nil || d["key"] != "h1" {
		t.Fatalf("line 0: %s", lines[0])
	}
	want, _ := DinhDang(TaiLieu, "Quán A", "cà phê yên tĩnh")
	if !strings.Contains(lines[0], `"text":"`+want+`"`) || !strings.Contains(lines[0], `"output_dimensionality":1536`) {
		t.Fatalf("the batch text is not the online door's prefix: %s", lines[0])
	}
	if strings.Contains(lines[0], "task_type") || strings.Contains(lines[0], "title\":") {
		t.Fatal("a task or title field was sent beside the in-text prefix")
	}
	if !strings.Contains(lines[1], "title: none | text: bún bò") {
		t.Fatalf("no title: %s", lines[1])
	}
	if _, err := FileVao([]TaiLieuLo{{Khoa: "x"}, {Khoa: "x"}}); err == nil {
		t.Fatal("a repeated key was accepted")
	}
}

func TestDocKetQuaLo(t *testing.T) {
	vec := make([]float32, Dims)
	vec[0], vec[1] = 3, 4 // not unit length: must come back normalised
	good, _ := json.Marshal(map[string]any{"key": "h1", "response": map[string]any{"embedding": map[string]any{"values": vec}}})
	short, _ := json.Marshal(map[string]any{"key": "h2", "response": map[string]any{"embedding": map[string]any{"values": []float32{1}}}})
	failed := `{"key":"h3","error":{"code":400,"message":"bad"}}`
	kq, err := DocKetQuaLo([]byte(string(good) + "\n" + string(short) + "\n" + failed + "\n"))
	if err != nil || kq.TrangThai != LoXong || len(kq.Vecs) != 1 || kq.LoiDong != 2 {
		t.Fatalf("%+v %v", kq, err)
	}
	if v := kq.Vecs["h1"]; v[0] < 0.599 || v[0] > 0.601 || v[1] < 0.799 || v[1] > 0.801 {
		t.Fatalf("not normalised: %v %v", v[0], v[1])
	}
	if _, err := DocKetQuaLo([]byte("not json\n")); err == nil {
		t.Fatal("an unreadable line was accepted")
	}
}

func TestLoChiLoopbackKhiTest(t *testing.T) {
	if _, err := NewLo(context.Background(), "k", ""); err == nil {
		t.Fatal("a test binary reached the real batch API")
	}
	if _, err := NewLo(context.Background(), "k", "https://example.com"); err == nil {
		t.Fatal("a non-loopback base URL was accepted")
	}
}

// TestXemTaiKetQuaQuaLoopback drives Xem through the real SDK transport
// against a loopback server answering with the shapes the Batch API returned
// on 2026-09-28 (job metadata with the result file, then the JSONL). The
// first real run failed here: the SDK downloads by DownloadURI, not Name.
func TestXemTaiKetQuaQuaLoopback(t *testing.T) {
	vec := make([]float32, Dims)
	vec[0] = 1
	line, _ := json.Marshal(map[string]any{"key": "h1", "response": map[string]any{"embedding": map[string]any{"values": vec}}})
	var downloaded bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/batches/j1"):
			_, _ = io.WriteString(w, `{"name":"batches/j1","metadata":{"@type":"type.googleapis.com/google.ai.generativelanguage.v1main.EmbedContentBatch","model":"models/gemini-embedding-2","output":{"responsesFile":"files/batch-j1"},"state":"BATCH_STATE_SUCCEEDED","name":"batches/j1"},"done":true,"response":{"@type":"type.googleapis.com/google.ai.generativelanguage.v1main.EmbedContentBatchOutput","responsesFile":"files/batch-j1"}}`)
		case strings.Contains(r.URL.Path, "files/batch-j1:download"):
			downloaded = true
			_, _ = w.Write(append(line, '\n'))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	l, err := NewLo(context.Background(), "k", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	kq, err := l.Xem(context.Background(), "batches/j1")
	if err != nil || kq.TrangThai != LoXong || len(kq.Vecs["h1"]) != Dims || !downloaded {
		t.Fatalf("%+v %v downloaded=%v", kq, err, downloaded)
	}
}
