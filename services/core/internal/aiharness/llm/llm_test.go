package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/obs"
)

func TestChiNhanBaseURLLoopback(t *testing.T) {
	for _, ok := range []string{"http://127.0.0.1:9999", "http://localhost:8080/", "https://[::1]:4443/v", "http://127.8.9.10"} {
		if err := CheckBaseURL(ok); err != nil {
			t.Errorf("%s bị từ chối: %v", ok, err)
		}
	}
	for _, bad := range []string{"https://generativelanguage.googleapis.com/", "http://10.0.0.1", "http://example.com", "http://user:pw@127.0.0.1", "ftp://127.0.0.1", "127.0.0.1:80", "", "http://localhost.evil.com"} {
		if err := CheckBaseURL(bad); err == nil {
			t.Errorf("%q được nhận", bad)
		}
	}
}

// Under `go test` the real host is unreachable by construction: no key, or no
// loopback override, and no client is built.
func TestTestKhongDungDuocGeminiThat(t *testing.T) {
	if _, err := NewGemini(context.Background(), "", "http://127.0.0.1:1"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("thiếu khoá: %v", err)
	}
	if m, err := NewGemini(context.Background(), "synthetic-key", ""); err == nil || m != nil {
		t.Fatal("một binary test dựng được client tới Gemini thật")
	}
	if _, err := NewGemini(context.Background(), "synthetic-key", "https://generativelanguage.googleapis.com/"); err == nil {
		t.Fatal("base URL không phải loopback được nhận")
	}
	env := map[string]string{EnvAPIKey: "synthetic-key"}
	if _, err := GeminiFromEnv(context.Background(), func(k string) string { return env[k] }); err == nil {
		t.Fatal("GeminiFromEnv không có base loopback vẫn dựng client dưới go test")
	}
}

// The real genai transport, against a loopback stand-in for the REST API: the
// model name, the key header and the response parsing are genai's own, not a
// stub's.
func TestGeminiQuaLoopback(t *testing.T) {
	var gotPath, gotKey string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("x-goog-api-key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Chào bạn"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":3}}`)
	}))
	defer srv.Close()
	m, err := NewGemini(context.Background(), "synthetic-key", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	req := &model.LLMRequest{Model: Model, Contents: []*genai.Content{genai.NewContentFromText("xin chào", "user")}}
	var text string
	for resp, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatal(err)
		}
		text = resp.Content.Parts[0].Text
		if resp.UsageMetadata.PromptTokenCount != 11 {
			t.Errorf("usage %+v", resp.UsageMetadata)
		}
	}
	if text != "Chào bạn" || gotKey != "synthetic-key" || !strings.HasSuffix(gotPath, "/models/"+Model+":generateContent") {
		t.Fatalf("text=%q key=%q path=%q", text, gotKey, gotPath)
	}
	if body["contents"] == nil {
		t.Fatalf("thân yêu cầu: %v", body)
	}
}

// A prompt the provider blocks comes back with no candidate, only
// promptFeedback. ADK's non-streaming call turns that into a bare error; the
// real transport, driven into it through loopback, must surface it as
// ErrKhongUngVien, classified as the provider's safety refusal.
func TestGeminiChanCauHoi(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"promptFeedback":{"blockReason":"SAFETY"},"usageMetadata":{"promptTokenCount":9}}`)
	}))
	defer srv.Close()
	m, err := NewGemini(context.Background(), "synthetic-key", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	req := &model.LLMRequest{Model: Model, Contents: []*genai.Content{genai.NewContentFromText("xin chào", "user")}}
	var got error
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		got = err
	}
	if !errors.Is(got, ErrKhongUngVien) || PhanLoai(got) != obs.LoiSafety {
		t.Fatalf("không ứng viên: %v (%s)", got, PhanLoai(got))
	}
}

func chay(t *testing.T, d *Dem) (string, error) {
	t.Helper()
	var text string
	var last error
	for resp, err := range d.GenerateContent(context.Background(), &model.LLMRequest{Model: Model}, false) {
		if err != nil {
			last = err
			continue
		}
		text = resp.Content.Parts[0].Text
	}
	return text, last
}

func khongCho(int) time.Duration { return 0 }

// Retries go through the counter: 429 then an answer is two calls.
func TestDemTinhCaLanThuLai(t *testing.T) {
	stub := NewStub(Buoc{Loi: genai.APIError{Code: 429}}, Buoc{Text: "ok"})
	d := NewDem(stub, MaxModelCallsPerTurn, nil).WithWait(khongCho)
	text, err := chay(t, d)
	if err != nil || text != "ok" || d.SoGoi() != 2 || stub.SoGoi() != 2 {
		t.Fatalf("text=%q err=%v đếm=%d stub=%d", text, err, d.SoGoi(), stub.SoGoi())
	}
	// Two retries at most: the third 503 is the answer.
	stub = NewStub(Buoc{Loi: genai.APIError{Code: 503}}, Buoc{Loi: genai.APIError{Code: 503}}, Buoc{Loi: genai.APIError{Code: 503}}, Buoc{Text: "không tới"})
	d = NewDem(stub, MaxModelCallsPerTurn, nil).WithWait(khongCho)
	if _, err := chay(t, d); PhanLoai(err) != obs.Loi5xx || d.SoGoi() != 3 {
		t.Fatalf("err=%v đếm=%d", err, d.SoGoi())
	}
	// A 400 is not retried.
	stub = NewStub(Buoc{Loi: genai.APIError{Code: 400}}, Buoc{Text: "không tới"})
	d = NewDem(stub, MaxModelCallsPerTurn, nil).WithWait(khongCho)
	if _, err := chay(t, d); err == nil || d.SoGoi() != 1 {
		t.Fatalf("400: err=%v đếm=%d", err, d.SoGoi())
	}
}

// The ceiling holds across retries: with one call left a 429 is not retried
// into a second request, the turn is out of budget.
func TestDemChanTaiTran(t *testing.T) {
	stub := NewStub(Buoc{Loi: genai.APIError{Code: 429}}, Buoc{Text: "không tới"})
	d := NewDem(stub, 1, nil).WithWait(khongCho)
	if _, err := chay(t, d); !errors.Is(err, ErrHetNganSach) || stub.SoGoi() != 1 {
		t.Fatalf("err=%v stub=%d", err, stub.SoGoi())
	}
	d = NewDem(NewStub(), 0, nil)
	if _, err := chay(t, d); !errors.Is(err, ErrHetNganSach) {
		t.Fatalf("trần 0: %v", err)
	}
	// The durable hold runs before each call and can refuse it.
	held := 0
	d = NewDem(NewStub(Buoc{Text: "a"}), 5, func(context.Context) error {
		held++
		return fmt.Errorf("giữ không được")
	})
	if _, err := chay(t, d); err == nil || held != 1 {
		t.Fatalf("giữ: err=%v held=%d", err, held)
	}
}

func TestPhanLoai(t *testing.T) {
	for _, c := range []struct {
		err  error
		want obs.LoiMoHinh
	}{
		{nil, obs.LoiKhong},
		{context.DeadlineExceeded, obs.LoiTimeout},
		{fmt.Errorf("x: %w", genai.APIError{Code: 429}), obs.Loi429},
		{genai.APIError{Code: 500}, obs.Loi5xx},
		{&genai.APIError{Code: 503}, obs.Loi5xx},
		{genai.APIError{Code: 400}, obs.LoiKhac},
		{errors.New("boom"), obs.LoiKhac},
		{fmt.Errorf("flow: %w", ErrKhongUngVien), obs.LoiSafety},
	} {
		if got := PhanLoai(c.err); got != c.want {
			t.Errorf("%v: %s, muốn %s", c.err, got, c.want)
		}
	}
}
