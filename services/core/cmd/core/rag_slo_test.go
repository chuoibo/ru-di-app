package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mobile/services/core/internal/rag/nap"
)

// /slo is 503 before the first reading, 200 on a good one with a live
// loop, 503 when the loop stops passing; /livez follows the loop only.
func TestSloHandler(t *testing.T) {
	gs := &nap.GiamSat{}
	h := sloHandler(gs)
	get := func(path string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Code
	}
	if get("/slo") != http.StatusServiceUnavailable || get("/livez") != http.StatusServiceUnavailable {
		t.Fatal("healthy before anything ran")
	}
	now := time.Now()
	gs.Nhip(now)
	gs.Ghi(now, nap.DoTuoi{LechGiay: 3}, nil)
	if get("/slo") != http.StatusOK || get("/livez") != http.StatusOK {
		t.Fatal("a fresh index with a live loop is not healthy")
	}
	gs.Nhip(now.Add(-2 * time.Minute))
	if get("/slo") != http.StatusServiceUnavailable || get("/livez") != http.StatusServiceUnavailable {
		t.Fatal("a stopped loop is healthy")
	}
}
