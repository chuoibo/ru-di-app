package nhatky

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"iter"
	"reflect"
	"strings"
	"sync"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/motluot"
	book "mobile/services/core/internal/domain/diary"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_nhatky.json was rendered by scripts/render_diary_golden.py
// from the real diary_gemini.compose_diary, driven by scripted answers,
// before ADR-0052 deleted it: every request the loop made and what it
// returned or raised.

// ghi is a model that answers from a script and keeps every request.
type ghi struct {
	mu     sync.Mutex
	script []string
	seen   []*model.LLMRequest
}

func (g *ghi) Name() string { return "ghi" }

func (g *ghi) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		g.mu.Lock()
		g.seen = append(g.seen, req)
		if len(g.script) == 0 {
			g.mu.Unlock()
			yield(nil, errors.New("script exhausted"))
			return
		}
		text := g.script[0]
		g.script = g.script[1:]
		g.mu.Unlock()
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: text}}}}, nil)
	}
}

func (g *ghi) calls() []any {
	out := []any{}
	for _, req := range g.seen {
		cfg := req.Config
		name := "?"
		switch cfg.SystemInstruction.Parts[0].Text {
		case huongDanViet:
			name = "viet"
		case huongDanKiem:
			name = "kiem"
		}
		parts := []any{}
		for _, c := range req.Contents {
			for _, p := range c.Parts {
				if p.InlineData != nil {
					parts = append(parts, map[string]any{"blob": map[string]any{"mime": p.InlineData.MIMEType, "data": string(p.InlineData.Data)}})
				} else {
					parts = append(parts, map[string]any{"text": p.Text})
				}
			}
		}
		out = append(out, map[string]any{
			"instruction": name, "temperature": float64(*cfg.Temperature), "max_output_tokens": int64(cfg.MaxOutputTokens),
			"mime": cfg.ResponseMIMEType, "parts": parts,
		})
	}
	return out
}

func replay(t *testing.T, args map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args["source"])
	if err != nil {
		t.Fatal(err)
	}
	// The golden lost Python's key order; production sends the bundle in
	// book.Source's field order, which is the order the Python cases wrote.
	var typed book.Source
	if json.Unmarshal(raw, &typed) == nil {
		raw, _ = json.Marshal(typed)
	}
	source, err := pyjson.Loads(raw)
	if err != nil {
		t.Fatal(err)
	}
	var anh []Anh
	for _, item := range args["images"].([]any) {
		m := item.(map[string]any)
		anh = append(anh, Anh{ID: m["id"].(string), MIME: m["mime"].(string), Data: []byte(m["data"].(string))})
	}
	g := &ghi{}
	for _, a := range args["answers"].([]any) {
		g.script = append(g.script, a.(string))
	}
	var result map[string]any
	parts, err := Phan(source, anh)
	if err == nil {
		var doc *pyjson.OrderedMap
		doc, err = Viet(context.Background(), motluot.Moi(g, 1).Luot(Luot), parts)
		if err == nil {
			text, _ := pyjson.DumpsText(doc, false)
			var v any
			if err := json.Unmarshal([]byte(text), &v); err != nil {
				t.Fatal(err)
			}
			result = map[string]any{"ok": v}
		}
	}
	if err != nil {
		// Python's classes differ (JSONDecodeError is a ValueError); the
		// brain answered every one of them with the same 422.
		result = map[string]any{"raised": "ValueError"}
	}
	return map[string]any{"calls": g.calls(), "result": result}
}

// normal turns decoded oracle values into what encoding/json gives, so the
// two sides compare with reflect.DeepEqual.
func normal(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func TestVietKhopPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_nhatky.json")
	var c struct {
		Prompt string `json:"prompt_sha256_16"`
		Check  string `json:"check_sha256_16"`
	}
	if err := json.Unmarshal(files[0].Constants, &c); err != nil {
		t.Fatal(err)
	}
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:])[:16] }
	if sum(huongDanViet) != c.Prompt || sum(huongDanKiem) != c.Check {
		t.Fatal("the instructions are not Python's, byte for byte")
	}
	n := 0
	for _, f := range files {
		for _, tc := range f.Cases {
			args, err := tc.PlainArgs()
			if err != nil {
				t.Fatal(err)
			}
			want, _, err := tc.Outcome()
			if err != nil {
				t.Fatal(err)
			}
			wantMap := normal(want).(map[string]any)
			if r, ok := wantMap["result"].(map[string]any)["raised"]; ok && strings.HasSuffix(r.(string), "Error") {
				wantMap["result"] = map[string]any{"raised": "ValueError"}
			}
			got := normal(replay(t, args))
			if !reflect.DeepEqual(got, wantMap) {
				gj, _ := json.MarshalIndent(got, "", " ")
				wj, _ := json.MarshalIndent(wantMap, "", " ")
				t.Errorf("%s:\n go %s\n py %s", tc.Name, gj, wj)
			}
			n++
		}
	}
	if n < 16 {
		t.Fatalf("only %d cases", n)
	}
}

func TestTranAnh(t *testing.T) {
	src := pyjson.NewOrderedMap()
	big := make([]byte, MaxByteAnh/2+1)
	if _, err := Phan(src, []Anh{{"a", "image/png", big}, {"b", "image/png", big}}); !errors.Is(err, ErrQuaLon) {
		t.Fatalf("over the cap: %v", err)
	}
	if _, err := Phan(src, []Anh{{"a", "image/png", big}}); err != nil {
		t.Fatal(err)
	}
}
