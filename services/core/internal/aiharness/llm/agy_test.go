package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// The agy route (ADR-0049 §2.1): AGY_PROXY_URL set picks the proxy over
// GEMINI_API_KEY; the proxy's token goes as x-goog-api-key on the Gemini wire
// path, the model is the engine's one model, and a test binary never leaves
// loopback.
func TestGeminiQuaAgy(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("x-goog-api-key")
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"qua agy"}]},"finishReason":"STOP"}]}`)
	}))
	defer srv.Close()
	env := map[string]string{EnvAgyURL: srv.URL, EnvAgyKey: "vnl_synthetic", EnvAPIKey: "direct-key-never-used"}
	m, err := GeminiFromEnv(context.Background(), func(k string) string { return env[k] })
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
	}
	if text != "qua agy" || gotKey != "vnl_synthetic" || !strings.HasSuffix(gotPath, "/models/"+Model+":generateContent") {
		t.Fatalf("text=%q key=%q path=%q", text, gotKey, gotPath)
	}
	for _, bad := range []map[string]string{
		{EnvAgyURL: "http://192.0.2.7:20131", EnvAgyKey: "vnl_x"}, // not loopback under test
		{EnvAgyURL: srv.URL},                                      // no token
		{EnvAgyURL: srv.URL + "/v1", EnvAgyKey: "vnl_x"},          // a path
		{EnvAgyURL: "http://u:p@127.0.0.1:1", EnvAgyKey: "vnl_x"}, // credentials
	} {
		if _, err := GeminiFromEnv(context.Background(), func(k string) string { return bad[k] }); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}
