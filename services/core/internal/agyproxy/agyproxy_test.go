package agyproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func stub(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, "vnl_test")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRefusesNonLoopbackUnderTest(t *testing.T) {
	if _, err := New("http://192.0.2.1:20131", "vnl_x"); err == nil {
		t.Fatal("a test binary reached a non-loopback proxy")
	}
	if _, err := New("http://u:p@127.0.0.1:1", "vnl_x"); err == nil {
		t.Fatal("credentials in the URL accepted")
	}
	if _, err := FromEnv(func(string) string { return "" }); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("empty env: %v", err)
	}
}

func TestRunSendsBearerAndBody(t *testing.T) {
	var got map[string]any
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agy/run" || r.Header.Get("Authorization") != "Bearer vnl_test" {
			t.Errorf("path %s auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		io.WriteString(w, `{"text":"Hà Nội","json":{"a":1},"grounding":{"queries":["q"],"sources":[{"title":"t","uri":"u"}]},"run_id":"r1","finish_reason":"STOP"}`)
	})
	r, err := c.Run(context.Background(), RunRequest{
		Model: ModelLow, Input: "?", GoogleSearch: true, JSONSchema: json.RawMessage(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["google_search"] != true || got["input"] != "?" || got["json_schema"] == nil {
		t.Fatalf("body %v", got)
	}
	if _, ok := got["system_instruction"]; ok {
		t.Fatal("empty system_instruction was sent")
	}
	if r.Text != "Hà Nội" || string(r.JSON) != `{"a":1}` || r.RunID != "r1" || len(r.Grounding.Sources) != 1 {
		t.Fatalf("result %+v", r)
	}
}

func TestHealthSendsNoToken(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("token sent to /health")
		}
		io.WriteString(w, `{"ok":true}`)
	})
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		status int
		header string
		body   string
		check  func(*Error) bool
	}{
		{429, "7", `{"detail":"no free seat"}`, func(e *Error) bool { return e.Busy() && e.RetryAfter == 7*time.Second && e.Detail == "no free seat" }},
		{401, "", `{"detail":"bad key"}`, func(e *Error) bool { return e.Unauthorized() }},
		{502, "", `{"detail":{"run_id":"abc","message":"upstream"}}`, func(e *Error) bool { return e.RunID == "abc" && e.Detail == "upstream" }},
		{503, "", `not json`, func(e *Error) bool { return e.Busy() && e.Detail == "not json" }},
	}
	for _, tc := range cases {
		c := stub(t, func(w http.ResponseWriter, r *http.Request) {
			if tc.header != "" {
				w.Header().Set("Retry-After", tc.header)
			}
			w.WriteHeader(tc.status)
			io.WriteString(w, tc.body)
		})
		_, err := c.Models(context.Background())
		var e *Error
		if !errors.As(err, &e) || e.Status != tc.status || !tc.check(e) {
			t.Errorf("%d: %#v", tc.status, err)
		}
	}
}

func TestChatWebSearchAndGrounding(t *testing.T) {
	var got map[string]any
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		io.WriteString(w, `{"choices":[{"message":{"content":"xanh"},"finish_reason":"stop"}],"vnlocal":{"grounding":{"queries":["q"]}}}`)
	})
	r, err := c.Chat(context.Background(), ChatRequest{Model: ModelLow, Messages: []Message{{Role: "user", Content: "?"}}, WebSearch: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["web_search_options"]; !ok {
		t.Fatalf("web_search_options missing: %v", got)
	}
	if _, ok := got["WebSearch"]; ok {
		t.Fatal("Go field name leaked onto the wire")
	}
	if r.Content != "xanh" || r.Grounding == nil || r.Grounding.Queries[0] != "q" {
		t.Fatalf("%+v", r)
	}
}

// The model's parts go back byte for byte: they carry thought signatures.
func TestContinueKeepsModelPartsVerbatim(t *testing.T) {
	part := json.RawMessage(`{"functionCall":{"name":"f","args":{"x":1}},"thoughtSignature":"c2ln"}`)
	first := AgentRequest{Model: ModelMedium, Input: "hỏi", Tools: []Tool{{Name: "f"}}}
	next, err := Continue(first, AgentStep{Parts: []json.RawMessage{part}}, []FunctionResult{{Name: "f", Response: map[string]any{"y": 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if next.Input != "" || len(next.Contents) != 3 || len(next.Tools) != 1 {
		t.Fatalf("%+v", next)
	}
	want := []string{
		`{"role":"user","parts":[{"text":"hỏi"}]}`,
		`{"role":"model","parts":[` + string(part) + `]}`,
		`{"role":"user","parts":[{"functionResponse":{"name":"f","response":{"y":2}}}]}`,
	}
	for i, w := range want {
		if string(next.Contents[i]) != w {
			t.Errorf("contents[%d]\n got %s\nwant %s", i, next.Contents[i], w)
		}
	}
	// A second step appends and does not repeat the question.
	again, err := Continue(next, AgentStep{Parts: []json.RawMessage{part}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Contents) != 5 || len(first.Contents) != 0 {
		t.Fatalf("history %d, first mutated %d", len(again.Contents), len(first.Contents))
	}
}
