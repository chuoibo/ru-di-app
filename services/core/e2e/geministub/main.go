//go:build e2e

// Command geministub is a deterministic stand-in for the Gemini REST API,
// used only by the chat end-to-end tier. The core's real genai transport
// talks to it (GEMINI_API_KEY + MOBILE_GEMINI_BASE_URL on loopback), so the
// tier drives the Go engine the product runs -- router, tools, verifier --
// with nothing but the model's words replaced (ADR-0052 removed the Python
// brain this tier used to stub).
//
// Why a stub rather than the real provider: the AI cases in this tier are
// about the *plumbing* -- consent, durable jobs, lease recovery, atomic
// promotion, who may see a result. None of that is a question about model
// quality, and all of it becomes untestable if the answer changes run to run
// or if a run needs a paid key. The real provider is exercised separately
// (cmd/vnlocal-thu, the vnlocal stack).
//
// It answers each request by its shape, the way the engine asks:
//   - the router (response schema with nhan_guard): one neutral routing;
//     the command sends a /plan to the tools and a split to its reading;
//   - an agent step (tools declared): search_places first, then
//     propose_itinerary with the first place found, then the answer that
//     names it -- the itinerary GroundReply accepts, never an invented id;
//   - the split reading (khoan items with so_tien_vnd): the first shared
//     message's "<n>k" as n thousand đồng, quoted from that message;
//   - every verifier: supported.
//
// Embeddings are deterministic unit vectors of the product's width.
package main

import (
	"encoding/json"
	"hash/fnv"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"mobile/services/core/internal/aiharness/nhung"
)

// nghinDong is "<n>k", the one money form the split stub reads.
var nghinDong = regexp.MustCompile(`(\d{1,7})k`)

type part struct {
	Text             string         `json:"text,omitempty"`
	FunctionCall     map[string]any `json:"functionCall,omitempty"`
	FunctionResponse map[string]any `json:"functionResponse,omitempty"`
}

type content struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

type schema struct {
	Properties map[string]*schema `json:"properties"`
	Items      *schema            `json:"items"`
}

type request struct {
	Contents         []content `json:"contents"`
	Tools            []any     `json:"tools"`
	GenerationConfig struct {
		ResponseSchema *schema `json:"responseSchema"`
	} `json:"generationConfig"`
	SystemInstruction *content `json:"systemInstruction"`
}

func (r request) co(ten string) bool {
	s := r.GenerationConfig.ResponseSchema
	if s == nil {
		return false
	}
	_, ok := s.Properties[ten]
	return ok
}

// khoanCo says whether the response's khoan items carry field ten.
func (r request) khoanCo(ten string) bool {
	s := r.GenerationConfig.ResponseSchema
	if s == nil || s.Properties["khoan"] == nil || s.Properties["khoan"].Items == nil {
		return false
	}
	_, ok := s.Properties["khoan"].Items.Properties[ten]
	return ok
}

// allText is every text part of the request, joined.
func (r request) allText() string {
	var b strings.Builder
	for _, c := range r.Contents {
		for _, p := range c.Parts {
			b.WriteString(p.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// calledSoFar is the function names the model already called, in order.
func (r request) calledSoFar() []string {
	var out []string
	for _, c := range r.Contents {
		for _, p := range c.Parts {
			if p.FunctionCall != nil {
				if n, _ := p.FunctionCall["name"].(string); n != "" {
					out = append(out, n)
				}
			}
		}
	}
	return out
}

func text(s string) []part { return []part{{Text: s}} }

func call(name string, args map[string]any) []part {
	return []part{{FunctionCall: map[string]any{"name": name, "args": args}}}
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// router is one neutral routing for every turn: a clean question to answer
// directly. The command decides the rest in the engine (epLenh): a /plan
// still goes to the tools, a split to the split reading. Reading the
// request's words instead would read the router's own worked examples.
func router() string {
	return mustJSON(map[string]any{"nhan_guard": "sach", "tien": "none", "y_dinh": []string{"smalltalk"}, "huong": "tra_loi_thang",
		"slots": map[string]any{}, "can_truy_hoi": []string{}, "truy_van": []any{}, "can_hoi_lai": false, "tra_loi_cau_cho": false, "tu_tin": "cao"})
}

func answer(r request) []part {
	switch {
	case r.co("nhan_guard"):
		return text(router())
	case r.co("menh_de"):
		// One verdict per sentence the draft has; the stub's answers are one
		// sentence each.
		return text(mustJSON(map[string]any{"menh_de": []any{map[string]any{"so": 1, "bang_chung_ids": []string{}, "ket": "khong_thong_tin"}}, "hua_hanh_dong_khong_co": false, "tien": false}))
	case r.khoanCo("ket"):
		return text(mustJSON(map[string]any{"khoan": []any{map[string]any{"so": 1, "ket": "ho_tro"}}}))
	case r.khoanCo("so_tien_vnd"):
		m := nghinDong.FindStringSubmatch(r.allText())
		if m == nil {
			return text(`{"khoan":[]}`)
		}
		n, _ := strconv.ParseInt(m[1], 10, 64)
		return text(mustJSON(map[string]any{"khoan": []any{map[string]any{"tin": "t1", "tieu_de": "tiền nước", "so_tien_goc": m[0], "so_tien_vnd": n * 1000}}}))
	case len(r.Tools) > 0:
		called := r.calledSoFar()
		switch len(called) {
		case 0:
			return call("search_places", map[string]any{"truy_van": "nướng"})
		case 1:
			return call("propose_itinerary", map[string]any{"chang": []any{map[string]any{"id": "p1", "gio": "19:00"}}})
		}
		return text("Cả nhóm ghé [[p:p1]] lúc bảy giờ tối nhé.")
	}
	return text("Chào cả nhóm, cần gì cứ gọi mình nhé.")
}

func generate(w http.ResponseWriter, body []byte) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		http.Error(w, `{"error":{"code":400,"message":"bad request","status":"INVALID_ARGUMENT"}}`, http.StatusBadRequest)
		return
	}
	parts := answer(r)
	// What the stub answered and, for a tool step, what the last tool said:
	// the one line that explains a card the tier did not expect. Synthetic
	// data only.
	var last string
	if n := len(r.Contents); n > 0 {
		for _, p := range r.Contents[n-1].Parts {
			if p.FunctionResponse != nil {
				last = mustJSON(p.FunctionResponse)
			}
		}
	}
	if len(last) > 400 {
		last = last[:400]
	}
	log.Printf("tools=%d called=%v -> %s | last=%s", len(r.Tools), r.calledSoFar(), mustJSON(parts), last)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"candidates":    []any{map[string]any{"content": content{Role: "model", Parts: parts}, "finishReason": "STOP"}},
		"usageMetadata": map[string]any{"promptTokenCount": 10, "candidatesTokenCount": 5},
	})
}

func embed(w http.ResponseWriter, body []byte) {
	var req struct {
		Requests []json.RawMessage `json:"requests"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Requests) == 0 {
		http.Error(w, `{"error":{"code":400,"message":"bad request","status":"INVALID_ARGUMENT"}}`, http.StatusBadRequest)
		return
	}
	embs := make([]map[string]any, len(req.Requests))
	for i, raw := range req.Requests {
		// A unit vector that depends on the text, so two texts are neither
		// identical nor orthogonal.
		v := make([]float64, nhung.Dims)
		f := fnv.New32a()
		_, _ = f.Write(raw)
		v[0], v[1+int(f.Sum32()%uint32(nhung.Dims-1))] = 1, 1
		for j := range v {
			v[j] /= math.Sqrt2
		}
		embs[i] = map[string]any{"values": v}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
}

func main() {
	listen := os.Getenv("GEMINI_STUB_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8791"
	}
	if !strings.HasPrefix(listen, "127.0.0.1:") {
		log.Fatalf("geministub chỉ nghe trên loopback, nhận %q", listen)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
		switch {
		case strings.HasSuffix(r.URL.Path, ":generateContent"):
			generate(w, body)
		case strings.HasSuffix(r.URL.Path, ":batchEmbedContents"):
			embed(w, body)
		default:
			http.Error(w, `{"error":{"code":404,"message":"geministub: unknown path","status":"NOT_FOUND"}}`, http.StatusNotFound)
		}
	})
	log.Printf("geministub nghe trên %s", listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}
