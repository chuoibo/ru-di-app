//go:build eval

package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aieval"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/rerank"
)

// The ONE place of the eval binary that may build a provider client or read
// the environment (the named exception to invariant 10, design 06 §6.1,
// held by TestKhongDungClientGenai in aieval): only chayMoHinh calls into
// this file, and only for `that` and `ghi`. `kich-ban` and `phat-lai` never
// reach it, whatever GEMINI_API_KEY holds; TestKichBanPhatLaiKhongDungClient
// counts the entries (soLanDungNhaCungCap) and the HTTP requests to prove it
// at run time.

// soLanDungNhaCungCap counts every entry into dungNhaCungCapThat.
var soLanDungNhaCungCap atomic.Int64

// getenv is the environment as this file reads it.
var getenv = os.Getenv

// ErrThieuKhoa is `that`/`ghi` without a key. The message never echoes a
// value: it names where the key is added.
var ErrThieuKhoa = errors.New("thiếu GEMINI_API_KEY: thêm nó trong cài đặt môi trường (environment settings → Edit → biến môi trường GEMINI_API_KEY) rồi mở phiên mới; không dán khoá vào lệnh hay vào repo")

// coPhuTuMoiTruong is which provider doors a `that`/`ghi` run will have,
// read before any client is built so the estimate can refuse first: the
// embedder always (the router's example bank, as in production), the
// reranker when MOBILE_RERANK_URL names one.
func coPhuTuMoiTruong() aieval.CoPhu {
	return aieval.CoPhu{Nhung: true, SoViDu: len(hieu.ViDuMacDinh), XepLai: strings.TrimSpace(getenv(rerank.EnvURL)) != ""}
}

// dungNhaCungCap builds the provider doors of a `that` or `ghi` run.
// Tests may not replace it: they count it.
func dungNhaCungCap(ctx context.Context, cheDo string) (*aieval.NoiGhi, string, error) {
	soLanDungNhaCungCap.Add(1)
	key := getenv(llm.EnvAPIKey)
	if strings.TrimSpace(key) == "" {
		return nil, "", ErrThieuKhoa
	}
	base := strings.TrimSpace(getenv(llm.EnvBaseURL))
	// `that` measures the door production uses: through agy-proxy when
	// AGY_PROXY_URL is set, as llm.GeminiFromEnv does, else the Gemini API
	// directly. `ghi` records from a loopback Gemini only, never agy.
	agy := strings.TrimSpace(getenv(llm.EnvAgyURL))
	var m model.LLM
	var nguon string
	var err error
	switch cheDo {
	case aieval.MoHinhThat:
		if base != "" {
			return nil, "", errors.New("--mo-hinh that là model thật: bỏ MOBILE_GEMINI_BASE_URL, hoặc dùng --mo-hinh ghi để ghi từ bản giả loopback")
		}
		if agy != "" {
			if strings.TrimSpace(getenv(llm.EnvAgyKey)) == "" {
				return nil, "", errors.New("AGY_PROXY_URL đã đặt mà thiếu AGY_PROXY_KEY: thêm khoá agy trong cài đặt môi trường, hoặc bỏ AGY_PROXY_URL để gọi Gemini API thẳng")
			}
			m, err = llm.NewGeminiQuaAgy(ctx, getenv(llm.EnvAgyKey), agy)
			nguon = aieval.NguonAgy
		} else {
			m, err = llm.NewGemini(ctx, key, "")
			nguon = aieval.NguonGeminiAPI
		}
	case aieval.MoHinhGhi:
		if base == "" {
			return nil, "", errors.New("--mo-hinh ghi ghi từ một Gemini loopback: đặt MOBILE_GEMINI_BASE_URL (chỉ loopback); model thật là --mo-hinh that")
		}
		if agy != "" {
			return nil, "", errors.New("--mo-hinh ghi ghi từ Gemini loopback, không qua agy-proxy: bỏ AGY_PROXY_URL")
		}
		m, err = llm.NewGemini(ctx, key, base)
		nguon = aieval.NguonLoopback
	default:
		return nil, "", errors.New("chỉ that và ghi dựng client")
	}
	// Every constructor refuses a real host inside a test binary
	// (testing.Testing) and any override that is not loopback.
	if err != nil {
		return nil, "", err
	}
	// The embedder never goes through agy-proxy (it serves no embeddings):
	// GEMINI_API_KEY and, for ghi, the loopback override. It refuses a real
	// host inside a test binary (testing.Testing).
	n, err := nhung.NewGemini(ctx, key, base)
	if err != nil {
		return nil, "", err
	}
	noi := &aieval.NoiGhi{MoHinh: m, Nhung: n}
	q, err := rerank.TuEnv(getenv)
	if err != nil {
		return nil, "", err
	}
	if q != nil {
		noi.XepLai = q
		noi.XepLaiModel = strings.TrimSpace(getenv(rerank.EnvModel))
		if noi.XepLaiModel == "" {
			noi.XepLaiModel = "mac-dinh"
		}
	}
	return noi, nguon, nil
}
