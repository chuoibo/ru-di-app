package aiharness

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// daoNguoc is a reranker that reverses its input and counts its calls.
type daoNguoc struct {
	mu  sync.Mutex
	goi int
	cau []string
	loi error
}

func (d *daoNguoc) XepLai(_ context.Context, cau string, bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error) {
	d.mu.Lock()
	d.goi++
	d.cau = append(d.cau, cau)
	d.mu.Unlock()
	if d.loi != nil {
		return nil, d.loi
	}
	out := append([]truyhoi.BangChung(nil), bc...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out, nil
}

// timXepLai stands for the shared hybrid retriever: it reranks with the
// reranker its turn carries, defers to a caller that reranks, and says
// no_rerank otherwise (internal/hybrid does the same, tested there).
type timXepLai struct {
	mu   sync.Mutex
	hoan []bool
	y    []truyhoi.YeuCau
}

func (r *timXepLai) Tim(ctx context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	xl, hoan := truyhoi.XepLaiTrong(ctx)
	r.mu.Lock()
	r.hoan, r.y = append(r.hoan, hoan), append(r.y, y)
	r.mu.Unlock()
	bc := []truyhoi.BangChung{quanGia("q-1", "Quán Gió Đồi"), quanGia("q-2", "Tiệm Sương Sớm"), quanGia("q-3", "Nhà Lá")}
	kq := truyhoi.KetQuaTruyHoi{}
	coDau := y.Cau
	if y.CauCoDau != "" {
		coDau = y.CauCoDau
	}
	switch {
	case hoan:
	case xl == nil:
		kq.Degraded = append(kq.Degraded, truyhoi.NoRerank)
	default:
		out, err := xl.XepLai(ctx, coDau, bc, 0)
		if err != nil {
			kq.Degraded = append(kq.Degraded, truyhoi.NoRerank)
		} else {
			bc = out
		}
	}
	kq.BangChung = bc
	return kq, nil
}

func chayXepLai(t *testing.T, tim truyhoi.Retriever, xl truyhoi.Reranker, turn Turn, kich ...llm.Buoc) moTa {
	t.Helper()
	stub := llm.NewStub(kich...)
	var buf bytes.Buffer
	opts := []Option{WithModel(stub), WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))), WithMaKiem(maKiem),
		WithRetryWait(func(int) time.Duration { return 0 }), WithClock(func() time.Time { return luc }),
		WithNguon(tools.NguonDuLieu{Quan: tim, Cho: testkit.Cho{DiemDens: []truyhoi.BangChung{{ID: ddDaLat, Truong: map[string]string{"ten": "Đà Lạt"}}}}})}
	if xl != nil {
		opts = append(opts, WithXepLai(xl))
	}
	e, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	sink := &ghi{}
	res, runErr := e.Run(context.Background(), turn, sink)
	return moTa{stub: stub, sink: sink, log: &buf, res: res, err: runErr}
}

// The tools' retriever reranks with the engine's reranker, counted against
// one budget per turn: three searches on one step, two reranked, the third
// in RRF order and flagged no_rerank to the model.
func TestXepLaiQuaCongCuMotNganSach(t *testing.T) {
	const q = "quan lau da lat"
	const qCoDau = "quán lẩu Đà Lạt"
	router := ru{huong: "tac_tu", yDinh: []string{"find_places"}, slots: map[string]any{"diem_den_id": ddDaLat},
		canTruyHoi: []string{"places"}, truyVan: []map[string]string{{"nguon": "places", "cau": q, "cau_co_dau": qCoDau}}}.buoc()
	var cac []*genai.FunctionCall
	for k := 1; k <= 3; k++ {
		cac = append(cac, &genai.FunctionCall{Name: "search_places", Args: map[string]any{"truy_van": q, "k": k + 1}})
	}
	xl := &daoNguoc{}
	tim := &timXepLai{}
	turn := luotCoBan()
	m := chayXepLai(t, tim, xl, turn, router, llm.Buoc{CacGoi: cac}, dung(false, "Bạn thử Nhà Lá nhé."), kiemDat())
	if m.err != nil {
		t.Fatalf("%v", m.err)
	}
	if xl.goi != llm.MaxRerankCallsPerTurn || m.res.Record.SoXepLai != llm.MaxRerankCallsPerTurn || len(tim.y) != 3 {
		t.Fatalf("reranker calls %d, recorded %d, retrievals %d", xl.goi, m.res.Record.SoXepLai, len(tim.y))
	}
	// The router's diacritics-restored form reaches the retriever and the
	// reranker with the search text it belongs to.
	for i, y := range tim.y {
		if y.Cau != q || y.CauCoDau != qCoDau || tim.hoan[i] {
			t.Fatalf("retrieval %d: %+v (deferred %v)", i, y, tim.hoan[i])
		}
	}
	if xl.cau[0] != qCoDau {
		t.Fatalf("the reranker scored against %q", xl.cau[0])
	}
	if c := string(m.stub.YeuCau()[2]); strings.Count(c, "no_rerank") != 1 {
		t.Fatalf("the model is told of %d unreranked searches, want 1", strings.Count(c, "no_rerank"))
	}
}

// The retrieval path's corrective loop reranks the merged candidates of
// every router query once, with the same counted reranker: the retriever
// defers (no second rerank of the same candidates) and is asked for
// UngVienXepLai of them.
func TestXepLaiDuongTruyHoiMotLan(t *testing.T) {
	router := ru{huong: "truy_hoi_mot_buoc", yDinh: []string{"find_places"}, slots: map[string]any{"diem_den_id": ddDaLat},
		canTruyHoi: []string{"places"}, truyVan: []map[string]string{
			{"nguon": "places", "cau": "quan yen tinh", "cau_co_dau": "quán yên tĩnh"},
			{"nguon": "places", "cau": "cà phê vắng"},
		}}.buoc()
	xl := &daoNguoc{}
	tim := &timXepLai{}
	m := chayXepLai(t, tim, xl, luotCoBan(), router, cham("du", nil, nil, ""), traLoiCau("Bạn thử [[p:p1]] nhé.", "p1"), kiemHoTro(1, "ho_tro"))
	if m.err != nil {
		t.Fatalf("%v", m.err)
	}
	if xl.goi != 1 || m.res.Record.SoXepLai != 1 || len(tim.y) != 2 {
		t.Fatalf("reranker calls %d, recorded %d, retrievals %d", xl.goi, m.res.Record.SoXepLai, len(tim.y))
	}
	for i, y := range tim.y {
		if !tim.hoan[i] || y.K != truyhoi.UngVienXepLai {
			t.Fatalf("retrieval %d: deferred %v, k %d", i, tim.hoan[i], y.K)
		}
	}
	if tim.y[0].CauCoDau != "quán yên tĩnh" || tim.y[1].Cau != "cà phê vắng" || tim.y[1].CauCoDau != "" {
		t.Fatalf("query forms: %+v", tim.y)
	}
}

// Unconfigured, or failing: the retrieval keeps the RRF order and says
// no_rerank; the turn answers, and nothing retries the reranker.
func TestXepLaiVangHoacHong(t *testing.T) {
	for name, xl := range map[string]*daoNguoc{"unset": nil, "failing": {loi: errors.New("timeout")}} {
		router := ru{huong: "tac_tu", yDinh: []string{"find_places"}, slots: map[string]any{"diem_den_id": ddDaLat},
			canTruyHoi: []string{"places"}, truyVan: []map[string]string{{"nguon": "places", "cau": "quán lẩu"}}}.buoc()
		tim := &timXepLai{}
		var r truyhoi.Reranker
		if xl != nil {
			r = xl
		}
		m := chayXepLai(t, tim, r, luotCoBan(), router, goiCC("search_places", map[string]any{"truy_van": "quán lẩu"}),
			dung(false, "Bạn thử Quán Gió Đồi nhé."), kiemDat())
		if m.err != nil {
			t.Fatalf("%s: %v", name, m.err)
		}
		c := string(m.stub.YeuCau()[2])
		if !strings.Contains(c, "no_rerank") || !strings.Contains(c, `\"id\":\"p1\"`) {
			t.Fatalf("%s: the search result does not say no_rerank:\n%s", name, c)
		}
		if xl != nil && xl.goi != 1 {
			t.Fatalf("%s: %d reranker calls (no retry within a turn)", name, xl.goi)
		}
	}
}
