package aieval

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aieval/giagemini"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/truyhoi"
)

type chunk struct {
	text    string
	partial bool
	usage   int32
	err     error
}

func goiHet(t *testing.T, m model.LLM, req *model.LLMRequest, stream bool) []chunk {
	t.Helper()
	var out []chunk
	for resp, err := range m.GenerateContent(context.Background(), req, stream) {
		c := chunk{err: err}
		if resp != nil {
			c.partial = resp.Partial
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				c.text = resp.Content.Parts[0].Text
			}
			if resp.UsageMetadata != nil {
				c.usage = resp.UsageMetadata.CandidatesTokenCount
			}
		}
		out = append(out, c)
	}
	return out
}

// A streamed call through the real genai transport: every chunk ADK yields
// (the partial ones and the aggregate) is recorded with its usage, and the
// replay yields the same sequence without a request. The same key asked
// without streaming is bang_lech, and so is a request never recorded.
func TestCassetteGhiPhatLaiLuong(t *testing.T) {
	may := giagemini.Moi()
	defer may.Close()
	may.Dat(llm.NewStub(llm.Buoc{Text: "Quán lẩu nấm ở Quận 3 mở tới 22 giờ.", Usage: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 50, CandidatesTokenCount: 12}}))
	g, err := llm.NewGemini(context.Background(), "khoa-gia", may.URL())
	if err != nil {
		t.Fatal(err)
	}
	bang := NewBangGhi(NoiGhi{MoHinh: g}, nil)
	req := &model.LLMRequest{Model: llm.Model, Contents: []*genai.Content{genai.NewContentFromText("quán nào?", "user")},
		Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText("bạn là Nếp", "")}}
	ghi := goiHet(t, bang.LLM("ca", 1), req, true)
	if may.SoLuong() != 1 || len(ghi) < 3 {
		t.Fatalf("luồng: %d yêu cầu, %d chunk %+v", may.SoLuong(), len(ghi), ghi)
	}
	tep := bang.Tep()
	if len(tep.Goi) != 1 || len(tep.Goi[0].PhanHoi) != len(ghi) || !tep.Goi[0].Stream {
		t.Fatalf("cassette: %+v", tep.Goi)
	}
	tok, ok, err := TokenCuaGoi(tep.Goi[0])
	if err != nil || !ok || tok.Ra != 12 || tok.Vao != 50 {
		t.Fatalf("token của lời gọi luồng: %+v %v %v", tok, ok, err)
	}
	lai, err := NewBangPhatLai(tep)
	if err != nil {
		t.Fatal(err)
	}
	pl := goiHet(t, lai.LLM("ca", 1), req, true)
	if len(pl) != len(ghi) {
		t.Fatalf("phát lại %d chunk, ghi %d", len(pl), len(ghi))
	}
	for i := range pl {
		if pl[i].text != ghi[i].text || pl[i].partial != ghi[i].partial || pl[i].usage != ghi[i].usage || (pl[i].err == nil) != (ghi[i].err == nil) {
			t.Fatalf("chunk %d: %+v ≠ %+v", i, pl[i], ghi[i])
		}
	}
	if may.SoYeuCau() != 1 || lai.SoGoiRa() != 0 {
		t.Fatal("phát lại gọi ra mạng")
	}
	// Same key, other streaming mode: bang_lech, not the recording.
	lai2, _ := NewBangPhatLai(tep)
	if c := goiHet(t, lai2.LLM("ca", 1), req, false); len(c) != 1 || !errors.Is(c[0].err, ErrBangLech) {
		t.Fatalf("stream=false trên bản ghi stream=true: %+v", c)
	}
	// A request never recorded: bang_lech, still no request.
	khac := &model.LLMRequest{Model: llm.Model, Contents: []*genai.Content{genai.NewContentFromText("câu khác", "user")}}
	lai3, _ := NewBangPhatLai(tep)
	if c := goiHet(t, lai3.LLM("ca", 1), khac, true); len(c) != 1 || !errors.Is(c[0].err, ErrBangLech) || len(lai3.Lech()) != 1 {
		t.Fatalf("yêu cầu lạ: %+v", c)
	}
	// Another scope (another case, another lap) does not see this one's
	// recordings.
	lai4, _ := NewBangPhatLai(tep)
	if c := goiHet(t, lai4.LLM("ca", 2), req, true); !errors.Is(c[0].err, ErrBangLech) {
		t.Fatalf("lap 2 đọc bản ghi của lap 1: %+v", c)
	}
	if may.SoYeuCau() != 1 {
		t.Fatal("bang_lech rơi xuống mạng")
	}
}

// The key leaves out exactly the documented volatile fields: ADK's
// function call ids change the key of nothing; any other change does.
func TestKhoaBoTruongDoi(t *testing.T) {
	mk := func(id, text string) *model.LLMRequest {
		return &model.LLMRequest{Model: llm.Model, Contents: []*genai.Content{
			genai.NewContentFromText(text, "user"),
			{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: id, Name: "search_places", Args: map[string]any{"q": "lẩu"}}}}},
			{Role: "user", Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: id, Name: "search_places", Response: map[string]any{"ok": true}}}}},
		}}
	}
	h1, _, _ := KhoaYeuCau(mk("adk-1", "quán?"))
	h2, _, _ := KhoaYeuCau(mk("adk-2", "quán?"))
	h3, _, _ := KhoaYeuCau(mk("adk-1", "quán nào?"))
	if h1 != h2 || h1 == h3 {
		t.Fatalf("khoá: id đổi %v, chữ đổi %v", h1 == h2, h1 != h3)
	}
}

// The reranker cassette: the order and scores replay by id, with no call.
func TestCassetteXepLai(t *testing.T) {
	bc := []truyhoi.BangChung{{ID: "q1", Truong: map[string]string{"ten": "Lẩu A"}}, {ID: "q2", Truong: map[string]string{"ten": "Nướng B"}}, {ID: "q3", Truong: map[string]string{"ten": "Lẩu C"}}}
	dao := xepLaiDao{}
	bang := NewBangGhi(NoiGhi{MoHinh: llm.NewStub(), XepLai: dao, XepLaiModel: "dao"}, nil)
	ctx := VoiPham(context.Background(), "ca@1")
	ghi, err := bang.XepLai().XepLai(ctx, "lẩu", bc, 2)
	if err != nil || len(ghi) != 2 || ghi[0].ID != "q3" {
		t.Fatalf("%+v %v", ghi, err)
	}
	lai, _ := NewBangPhatLai(bang.Tep())
	pl, err := lai.XepLai().XepLai(ctx, "lẩu", bc, 2)
	if err != nil || len(pl) != 2 || pl[0].ID != "q3" || pl[0].DiemXepLai != ghi[0].DiemXepLai || pl[1].ID != ghi[1].ID {
		t.Fatalf("phát lại: %+v %v", pl, err)
	}
	if _, err := lai.XepLai().XepLai(ctx, "nướng", bc, 2); !errors.Is(err, ErrBangLech) {
		t.Fatalf("truy vấn lạ: %v", err)
	}
	// A cassette recorded without a reranker replays without one.
	if NewBangGhi(NoiGhi{MoHinh: llm.NewStub()}, nil).XepLai() != nil {
		t.Fatal("không có reranker mà cassette dựng một")
	}
}

type xepLaiDao struct{}

func (xepLaiDao) XepLai(_ context.Context, _ string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	out := make([]truyhoi.BangChung, 0, len(bc))
	for i := len(bc) - 1; i >= 0; i-- {
		x := bc[i]
		x.DiemXepLai = float64(i+1) / 10
		out = append(out, x)
	}
	return out[:topN], nil
}

func chuoi(s string) *string { return &s }

// Cost in exact rationals: 1 234 567 input tokens at 0.10 and 89 012 output
// tokens at 0.40 per million is 123456.7/1e6 + 35604.8/1e6 = 0.1590615 USD
// exactly, which rounds once to 0.159062 (half up at the 7th decimal); at
// 25 400 đồng per dollar, 4040.1621 đồng → 4040. A float64 sum carries a
// binary rounding error in its last digits on the way.
func TestChiPhiPhanSoChinhXac(t *testing.T) {
	g := &BangGia{GhiChu: "thử", DonVi: "USD", TyGiaVND: chuoi("25400"), Model: map[string]GiaMot{
		llm.Model: {VaoMoiTrieu: chuoi("0.10"), RaMoiTrieu: chuoi("0.40"), CacheMoiTrieu: chuoi("0.025"), Nguon: chuoi("trang giá"), NgayXacNhan: chuoi("2026-09-27")},
	}}
	if err := g.Kiem(); err != nil {
		t.Fatal(err)
	}
	cp := TinhChiPhi(g, llm.Model, Token{Vao: 1_234_567, Ra: 89_012}, 7)
	if !cp.CoGia || cp.ChinhXac != new(big.Rat).SetFrac64(1590615, 10_000_000).RatString() {
		t.Fatalf("chính xác: %+v", cp)
	}
	if cp.Tong != "0.159062 USD" || cp.TongDong != "4040 đồng" || cp.MoiLuot != "0.022723 USD" {
		t.Fatalf("làm tròn: %+v", cp)
	}
	// No price: «chưa có giá», never 0.
	g2 := &BangGia{GhiChu: "thử", DonVi: "USD", Model: map[string]GiaMot{llm.Model: {}}}
	if cp2 := TinhChiPhi(g2, llm.Model, Token{Vao: 10}, 1); cp2.CoGia || cp2.Tong != ChuaCoGia || !strings.Contains(strings.Join(cp2.ThieuGia, ","), "vao_moi_trieu") {
		t.Fatalf("thiếu giá: %+v", cp2)
	}
	// A price without who confirmed it is refused.
	g3 := &BangGia{GhiChu: "thử", DonVi: "USD", Model: map[string]GiaMot{llm.Model: {VaoMoiTrieu: chuoi("0.10")}}}
	if g3.Kiem() == nil {
		t.Fatal("giá không nguồn được nhận")
	}
	// Half up, exactly once.
	if LamTron(big.NewRat(5, 1000), 2) != "0.01" || LamTron(big.NewRat(4999, 1_000_000), 2) != "0.00" || LamTron(big.NewRat(0, 1), 3) != "0.000" {
		t.Fatal("làm tròn nửa lên")
	}
}

// The committed price file parses, and holds no price nobody confirmed.
func TestGiaModelCanNguoiXacNhan(t *testing.T) {
	g, err := DocGia("testdata/gia-model.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.GhiChu, "CẦN NGƯỜI XÁC NHẬN") {
		t.Fatal("ghi_chu không nói giá cần người xác nhận")
	}
	for ten, m := range g.Model {
		if m.VaoMoiTrieu != nil || m.RaMoiTrieu != nil || m.CacheMoiTrieu != nil {
			t.Errorf("%s: có giá trong file đã commit -- giá phải do người chép từ trang giá, kèm nguồn và ngày", ten)
		}
	}
	if _, ok := g.Model[llm.Model]; !ok {
		t.Fatalf("gia-model.json thiếu %s", llm.Model)
	}
}
