// Package giagemini is a loopback stand-in for the Gemini REST API and for
// the reranker's /rerank, for the eval's offline tests: the real genai
// transport (llm.NewGemini, nhung.NewGemini with a loopback base URL) and
// the real reranker client talk to it, so what a cassette records in those
// tests is what the SDK really sent and parsed. It answers generateContent
// and streamGenerateContent (SSE) from a model.LLM the test sets (a scripted
// stub), batchEmbedContents with nhung.Stub's deterministic vectors, and
// /rerank with a fixed deterministic order. It counts every request, so a
// test can prove a replay sent none.
//
// It is test tooling that lives outside a _test file only so that the eval
// package's tests and the eval binary's tests share one stand-in. Nothing
// in production imports it (aigate's dependency listing would show it).
package giagemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
)

// ModelVersion is what the stand-in reports as the model's version.
const ModelVersion = "gia-loopback-001"

// May is the stand-in Gemini.
type May struct {
	srv *httptest.Server

	mu  sync.Mutex
	src model.LLM

	sinh, luong, nhungN atomic.Int64
}

// Moi starts a stand-in on a loopback port.
func Moi() *May {
	m := &May{}
	m.srv = httptest.NewServer(http.HandlerFunc(m.phuc))
	return m
}

// URL is the base URL to hand the genai client (loopback).
func (m *May) URL() string { return m.srv.URL + "/" }

// Close stops the server.
func (m *May) Close() { m.srv.Close() }

// Dat sets the model the next generate calls are answered from.
func (m *May) Dat(src model.LLM) {
	m.mu.Lock()
	m.src = src
	m.mu.Unlock()
}

// SoYeuCau is every request the stand-in answered: generate, stream and
// embed together.
func (m *May) SoYeuCau() int64 { return m.sinh.Load() + m.luong.Load() + m.nhungN.Load() }

// SoSinh, SoLuong, SoNhung split it.
func (m *May) SoSinh() int64  { return m.sinh.Load() }
func (m *May) SoLuong() int64 { return m.luong.Load() }
func (m *May) SoNhung() int64 { return m.nhungN.Load() }

func (m *May) phuc(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	switch {
	case strings.HasSuffix(r.URL.Path, ":batchEmbedContents"):
		m.nhungN.Add(1)
		m.nhung(w, body)
	case strings.HasSuffix(r.URL.Path, ":streamGenerateContent"):
		m.luong.Add(1)
		m.sinhRa(w, r.Context(), true)
	case strings.HasSuffix(r.URL.Path, ":generateContent"):
		m.sinh.Add(1)
		m.sinhRa(w, r.Context(), false)
	default:
		http.Error(w, `{"error":{"code":404,"message":"giagemini: unknown path","status":"NOT_FOUND"}}`, http.StatusNotFound)
	}
}

func (m *May) sinhRa(w http.ResponseWriter, ctx context.Context, stream bool) {
	m.mu.Lock()
	src := m.src
	m.mu.Unlock()
	if src == nil {
		loiAPI(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	var resp *model.LLMResponse
	var err error
	for rr, e := range src.GenerateContent(ctx, &model.LLMRequest{Model: llm.Model}, false) {
		resp, err = rr, e
		break
	}
	var api genai.APIError
	switch {
	case errors.As(err, &api):
		loiAPI(w, api.Code, api.Status)
		return
	case errors.Is(err, llm.ErrKhongUngVien):
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"promptFeedback":{"blockReason":"SAFETY"},"usageMetadata":{"promptTokenCount":9}}`)
		return
	case err != nil || resp == nil:
		// A script run past its end: a request the stand-in refuses,
		// not retried (400).
		loiAPI(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if !stream {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(phanHoi(resp.Content, resp.FinishReason, resp.UsageMetadata))
		return
	}
	// Streamed: the text in up to three chunks, the finish reason and the
	// usage on the last, as the API sends them.
	w.Header().Set("Content-Type", "text/event-stream")
	for i, c := range chiaNho(resp.Content) {
		var fin genai.FinishReason
		var u *genai.GenerateContentResponseUsageMetadata
		if i == len(chiaNho(resp.Content))-1 {
			fin, u = resp.FinishReason, resp.UsageMetadata
		}
		raw, _ := json.Marshal(phanHoi(c, fin, u))
		fmt.Fprintf(w, "data: %s\n\n", raw)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func phanHoi(c *genai.Content, fin genai.FinishReason, u *genai.GenerateContentResponseUsageMetadata) map[string]any {
	cand := map[string]any{"content": c}
	if fin != "" {
		cand["finishReason"] = fin
	}
	out := map[string]any{"candidates": []any{cand}, "modelVersion": ModelVersion}
	if u != nil {
		out["usageMetadata"] = u
	}
	return out
}

// chiaNho splits a one-text content into up to three chunks; any other
// content goes whole.
func chiaNho(c *genai.Content) []*genai.Content {
	if c == nil || len(c.Parts) != 1 || c.Parts[0].Text == "" {
		return []*genai.Content{c}
	}
	r := []rune(c.Parts[0].Text)
	n := min(3, len(r))
	var out []*genai.Content
	for i := 0; i < n; i++ {
		part := string(r[i*len(r)/n : (i+1)*len(r)/n])
		out = append(out, &genai.Content{Role: c.Role, Parts: []*genai.Part{{Text: part}}})
	}
	return out
}

func loiAPI(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": "giagemini", "status": status}})
}

func (m *May) nhung(w http.ResponseWriter, body []byte) {
	var req struct {
		Requests []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Requests) == 0 {
		loiAPI(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	texts := make([]string, len(req.Requests))
	for i, r := range req.Requests {
		for _, p := range r.Content.Parts {
			texts[i] += p.Text
		}
		// The embedder writes the task as a prefix; the stand-in embeds
		// what follows it, so texts that share words land close.
		if j := strings.LastIndex(texts[i], "| query: "); j >= 0 {
			texts[i] = texts[i][j+len("| query: "):]
		}
	}
	vs, _ := nhung.Stub{}.Nhung(context.Background(), texts, nhung.GiongNhau)
	embs := make([]map[string]any, len(vs))
	for i, v := range vs {
		embs[i] = map[string]any{"values": v}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
}

// XepLai is the stand-in reranker (POST /rerank): it scores documents by
// how many runes they share with the query, deterministically.
type XepLai struct {
	srv *httptest.Server
	n   atomic.Int64
}

// MoiXepLai starts a stand-in reranker on a loopback port.
func MoiXepLai() *XepLai {
	x := &XepLai{}
	x.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rerank" {
			http.NotFound(w, r)
			return
		}
		x.n.Add(1)
		var req struct {
			Query     string   `json:"query"`
			Documents []string `json:"documents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		type kq struct {
			Index int     `json:"index"`
			Score float64 `json:"relevance_score"`
		}
		out := make([]kq, len(req.Documents))
		for i, d := range req.Documents {
			chung := 0
			for _, c := range req.Query {
				if strings.ContainsRune(d, c) {
					chung++
				}
			}
			out[i] = kq{Index: i, Score: float64(chung) / float64(len([]rune(req.Query))+1)}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": out})
	}))
	return x
}

// URL is the reranker's base URL.
func (x *XepLai) URL() string { return x.srv.URL }

// SoYeuCau is the requests it answered.
func (x *XepLai) SoYeuCau() int64 { return x.n.Load() }

// Close stops it.
func (x *XepLai) Close() { x.srv.Close() }
