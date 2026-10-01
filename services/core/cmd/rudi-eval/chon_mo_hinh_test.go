//go:build eval

package main

import (
	"context"
	"strings"
	"testing"

	"mobile/services/core/internal/aieval"
)

// `that` measures the door production uses: agy-proxy when AGY_PROXY_URL is
// set (llm.GeminiFromEnv's order); `ghi` records from a loopback Gemini only.
// A test binary may build only loopback clients, so a `that` run always
// stops here: with agy configured it gets past the model (agy on 127.0.0.1)
// and stops at the embedder, which stays on the Gemini API.
func TestNhaCungCapChonCuaProduction(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct {
		cheDo string
		kv    map[string]string
		want  string
	}{
		"agy thiếu khoá":         {aieval.MoHinhThat, map[string]string{"GEMINI_API_KEY": "k", "AGY_PROXY_URL": "http://127.0.0.1:9"}, "AGY_PROXY_KEY"},
		"agy có đường dẫn":       {aieval.MoHinhThat, map[string]string{"GEMINI_API_KEY": "k", "AGY_PROXY_URL": "http://127.0.0.1:9/v1", "AGY_PROXY_KEY": "t"}, "origin"},
		"agy máy thật dưới test": {aieval.MoHinhThat, map[string]string{"GEMINI_API_KEY": "k", "AGY_PROXY_URL": "http://192.0.2.1:20131", "AGY_PROXY_KEY": "t"}, "loopback agy-proxy"},
		"agy qua, dừng ở nhúng":  {aieval.MoHinhThat, map[string]string{"GEMINI_API_KEY": "k", "AGY_PROXY_URL": "http://127.0.0.1:9", "AGY_PROXY_KEY": "t"}, "nhung"},
		"ghi không qua agy":      {aieval.MoHinhGhi, map[string]string{"GEMINI_API_KEY": "k", "MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9", "AGY_PROXY_URL": "http://127.0.0.1:9", "AGY_PROXY_KEY": "t"}, "không qua agy-proxy"},
	} {
		cu := getenv
		getenv = func(k string) string { return c.kv[k] }
		_, _, err := dungNhaCungCap(ctx, c.cheDo)
		getenv = cu
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, muốn lỗi nhắc %q", name, err, c.want)
		}
	}
}
