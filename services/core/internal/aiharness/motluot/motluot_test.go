package motluot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func noWait(int) time.Duration { return 0 }

// A process with no model is a nil *May, and every call through it is the
// closed "not configured" error — never a request.
func TestKhongKhoaLaMayNil(t *testing.T) {
	y, err := TuEnv(context.Background(), env(nil), 4)
	if err != nil || y != nil {
		t.Fatalf("keyless: %v, %v", y, err)
	}
	if y.CoMay() {
		t.Fatal("a nil May has a model")
	}
	_, err = y.Luot(3).Goi(context.Background(), cautruc.YeuCau(llm.BuocTrichXuat, "he", "x", nil, 64))
	if !errors.Is(err, ErrChuaCauHinh) {
		t.Fatalf("nil May answered %v", err)
	}
}

// Half a configuration refuses to build: the process must not start and
// then fail every call.
func TestCauHinhNuaVoiBiTuChoi(t *testing.T) {
	for _, bad := range []map[string]string{
		{llm.EnvAgyURL: "http://127.0.0.1:1"},                           // no token
		{llm.EnvAgyURL: "http://192.0.2.7:20131", llm.EnvAgyKey: "vnl"}, // not loopback under test
		{llm.EnvAPIKey: "k", llm.EnvBaseURL: "http://192.0.2.7:1"},      // override off loopback
	} {
		if y, err := TuEnv(context.Background(), env(bad), 4); err == nil {
			t.Errorf("%v built %v", bad, y)
		}
	}
}

// Retries pass the budget: two 429s and an answer is three requests out;
// a budget of one gives the provider's error back after the first.
func TestNganSachDemCaLanThuLai(t *testing.T) {
	stub := llm.NewStub(llm.Buoc{Loi: genai.APIError{Code: 429}}, llm.Buoc{Loi: genai.APIError{Code: 429}}, llm.Buoc{Text: `{"ok":true}`})
	l := Moi(stub, 1).WithWait(noWait).Luot(3)
	text, err := l.Goi(context.Background(), cautruc.YeuCau(llm.BuocTrichXuat, "he", "x", nil, 64))
	if err != nil || text != `{"ok":true}` || l.SoGoi() != 3 {
		t.Fatalf("text=%q err=%v calls=%d", text, err, l.SoGoi())
	}
	stub = llm.NewStub(llm.Buoc{Loi: genai.APIError{Code: 429}}, llm.Buoc{Text: "không tới"})
	l = Moi(stub, 1).WithWait(noWait).Luot(1)
	var api genai.APIError
	if _, err := l.Goi(context.Background(), cautruc.YeuCau(llm.BuocTrichXuat, "he", "x", nil, 64)); !errors.As(err, &api) || api.Code != 429 || stub.SoGoi() != 1 {
		t.Fatalf("err=%v stub calls=%d", err, stub.SoGoi())
	}
}

// One seat: a second call waits for it and gives up with its context.
func TestGheGioiHanSongSong(t *testing.T) {
	stub := llm.NewStub(llm.Buoc{Text: "a", Cho: 300 * time.Millisecond}, llm.Buoc{Text: "b"})
	y := Moi(stub, 1)
	done := make(chan error, 1)
	go func() {
		_, err := y.Luot(1).Goi(context.Background(), cautruc.YeuCau(llm.BuocViet, "he", "1", nil, 64))
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := y.Luot(1).Goi(ctx, cautruc.YeuCau(llm.BuocViet, "he", "2", nil, 64)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second call while the seat is taken: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if stub.SoGoi() != 1 {
		t.Fatalf("%d requests left; the waiting call must not have gone out", stub.SoGoi())
	}
}

// An image goes out inline, on the Gemini wire, with the MIME type the
// caller gave — through the agy door built from the environment.
func TestAnhDiQuaAgyInline(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"{\"ok\":1}"}]},"finishReason":"STOP"}]}`)
	}))
	defer srv.Close()
	y, err := TuEnv(context.Background(), env(map[string]string{llm.EnvAgyURL: srv.URL, llm.EnvAgyKey: "vnl_synthetic"}), 2)
	if err != nil || y == nil {
		t.Fatal(y, err)
	}
	png := []byte("\x89PNG\r\n\x1a\nnot-really")
	req := cautruc.YeuCauPhan(llm.BuocDocAnh, "đọc ảnh", []*genai.Part{{Text: "ảnh:"}, cautruc.Anh("image/png", png)}, &genai.Schema{Type: genai.TypeObject}, 128)
	text, err := y.Luot(1).Goi(context.Background(), cautruc.NhietDo(req, 0))
	if err != nil || text != `{"ok":1}` {
		t.Fatal(text, err)
	}
	wire, _ := json.Marshal(body)
	if !strings.Contains(string(wire), `"inlineData":{"data":"`+base64.StdEncoding.EncodeToString(png)+`","mimeType":"image/png"}`) {
		t.Fatalf("no inline image on the wire: %s", wire)
	}
	if !strings.Contains(string(wire), `"temperature":0`) {
		t.Fatalf("temperature not sent: %s", wire)
	}
	if req.Config.Temperature != nil {
		t.Fatal("NhietDo modified the request it was given")
	}
}
