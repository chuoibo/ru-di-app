//go:build milvus

package hybrid_test

import (
	"context"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/rerank"
)

// The production wiring, end to end on real services: the reranker built
// from the MOBILE_RERANK_* environment (as cmd/core builds it), counted per
// turn and carried by the turn's context into the shared Kho (as the engine
// carries it, aiharness.WithXepLai), scoring the router's
// diacritics-restored query while the folded BM25 field reads the
// person's unmarked one. The reranker is the loopback stand-in (rerankGia):
// this checks the wiring, never a model.
func TestHybridRerankTheoLuot(t *testing.T) {
	base, served := rerankGia(t)
	timeout := "10s"
	env := map[string]string{rerank.EnvURL: base, rerank.EnvTimeout: timeout}
	q, err := rerank.TuEnv(func(k string) string { return env[k] })
	if err != nil || q == nil {
		t.Fatalf("reranker from the environment: %v", err)
	}
	k, _, _ := dung(t, sinh(7, 8))
	dem := rerank.NewDem(q, llm.MaxRerankCallsPerTurn)
	ctx, cancel := context.WithTimeout(truyhoi.VoiXepLai(context.Background(), dem), 2*time.Minute)
	defer cancel()
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "quan lau yen tinh", CauCoDau: "quán lẩu yên tĩnh", K: 3}
	kq, err := k.Tim(ctx, y)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Degraded) != 0 || len(kq.BangChung) == 0 {
		t.Fatalf("degraded %v, %d items (timeout %s)", kq.Degraded, len(kq.BangChung), timeout)
	}
	for i := 1; i < len(kq.BangChung); i++ {
		if kq.BangChung[i].DiemXepLai > kq.BangChung[i-1].DiemXepLai {
			t.Fatalf("not in the reranker's order: %+v", kq.BangChung)
		}
	}
	if dem.SoGoi() != 1 || q.ThongKe().Loi != 0 || served.Load() != 1 {
		t.Fatalf("turn counter %d, reranker %+v", dem.SoGoi(), q.ThongKe())
	}
	// A retrieval whose caller reranks (the corrective loop) makes no call.
	if _, err := k.Tim(truyhoi.HoanXepLai(ctx), y); err != nil || dem.SoGoi() != 1 {
		t.Fatalf("deferred retrieval: %v, %d calls", err, dem.SoGoi())
	}
	t.Logf("reranked %d items (model name sent %q) at %s, scores %.3f..%.3f", len(kq.BangChung), q.Model(), timeout,
		kq.BangChung[0].DiemXepLai, kq.BangChung[len(kq.BangChung)-1].DiemXepLai)
}
