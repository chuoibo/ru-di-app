package aieval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aieval/giagemini"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/rerank"
)

const (
	boNep      = "testdata/corpus/nep-kich-ban.json"
	kichBanDir = "testdata/kich_ban"
	boT1Hieu   = "testdata/hieu/t1-hieu.json"
)

// cuaGia are the provider doors over loopback stand-ins: the real genai
// transport for the model and the embedder, the real reranker client.
type cuaGia struct {
	may *giagemini.May
	xl  *giagemini.XepLai
	noi *NoiGhi
}

func moCuaGia(t *testing.T) *cuaGia {
	t.Helper()
	may, xl := giagemini.Moi(), giagemini.MoiXepLai()
	t.Cleanup(may.Close)
	t.Cleanup(xl.Close)
	ctx := context.Background()
	m, err := llm.NewGemini(ctx, "khoa-gia-loopback", may.URL())
	if err != nil {
		t.Fatal(err)
	}
	n, err := nhung.NewGemini(ctx, "khoa-gia-loopback", may.URL())
	if err != nil {
		t.Fatal(err)
	}
	q, err := rerank.Moi(xl.URL(), "gia-reranker", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return &cuaGia{may: may, xl: xl, noi: &NoiGhi{MoHinh: m, Nhung: n, XepLai: q, XepLaiModel: "gia-reranker"}}
}

func (c *cuaGia) soYeuCau() int64 { return c.may.SoYeuCau() + c.xl.SoYeuCau() }

// theoKichBan answers each case with its correct script, through HTTP.
func theoKichBan(t *testing.T, c *cuaGia) func(Ca, int) {
	kbs, err := DocKichBan(kichBanDir)
	if err != nil {
		t.Fatal(err)
	}
	return func(ca Ca, _ int) { c.may.Dat(kbs[ca.KichBan.Dung].Stub(maKiemCua(ca.CaID))) }
}

func khong(int) time.Duration { return 0 }

func ghiBoNep(t *testing.T, c *cuaGia, tran, lap int) KetQuaLuotMoHinh {
	t.Helper()
	kq, err := ChayLuotMoHinh(context.Background(), YeuCauLuot{CheDo: MoHinhGhi, BoDuongDan: boNep, KichBanDir: kichBanDir,
		Lap: lap, TranGoi: tran, Goc: t.TempDir(), GitSHA: "abc1234def", Noi: c.noi, Nguon: NguonLoopback,
		Cho: khong, TruocCa: theoKichBan(t, c)})
	if err != nil {
		t.Fatal(err)
	}
	return kq
}

func phatLai(t *testing.T, dir, bo, chiBuoc string) KetQuaLuotMoHinh {
	t.Helper()
	kq, err := ChayLuotMoHinh(context.Background(), YeuCauLuot{CheDo: MoHinhPhatLai, ChiBuoc: chiBuoc, BoDuongDan: bo, KichBanDir: kichBanDir,
		Goc: t.TempDir(), GitSHA: "abc1234def", BangTu: dir})
	if err != nil {
		t.Fatal(err)
	}
	return kq
}

func docTep(t *testing.T, dir, ten string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ten))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// T2 (design 06 §6, §8.2): a corpus recorded through the real genai
// transport, embedder and reranker client against loopback stand-ins, then
// replayed: every grade line identical, and not one request reaches any
// stand-in during the replay. The recording holds model calls, the example
// bank and the per-turn embeddings, and reranker calls.
func TestGhiRoiPhatLaiTrungDiem(t *testing.T) {
	c := moCuaGia(t)
	goc := ghiBoNep(t, c, 5000, 1)
	m := goc.Manifest
	if m.TrangThai == TrangThaiChuaXong || m.TongKet.SoLuot != m.TongKet.SoCa || m.TongKet.SoCa < 40 {
		t.Fatalf("lượt ghi: %s %v, %d/%d lượt", m.TrangThai, m.LyDo, m.TongKet.SoLuot, m.TongKet.SoCa)
	}
	// The stand-in plays each case's correct script, so the model-mode
	// grader sees a known answer: the canary case (a script that invents a
	// place) is red at khong_bia_dia_diem and nowhere else, the three cases
	// whose script plays a provider that stays down are infrastructure
	// errors (which void the run: 3/56 > 5%), every other case is green.
	if m.TongKet.KhongDat != 1 || len(m.TongKet.TruotTheoKiem) != 1 || m.TongKet.TruotTheoKiem[KiemKhongBiaDiaDiem] != 1 ||
		m.TongKet.LoiHaTang != 3 || m.TrangThai != TrangThaiVoHieu {
		t.Fatalf("chấm chế độ model trên kịch bản đúng: %s, đạt %d, không đạt %d, hạ tầng %d, trượt %v",
			m.TrangThai, m.TongKet.Dat, m.TongKet.KhongDat, m.TongKet.LoiHaTang, m.TongKet.TruotTheoKiem)
	}
	if !strings.Contains(string(docTep(t, goc.Dir, TepCham)), `{"case_id":"00-canary-phai-do","lap":1,"dat":false,"loi_ha_tang":false,"truot":["khong_bia_dia_diem"]`) {
		t.Fatal("dòng điểm của canary không đỏ đúng chỗ")
	}
	if int64(m.Goi.DaDung) != c.soYeuCau() || m.Goi.MoHinh == 0 || m.Goi.Nhung < 2 || m.Goi.XepLai == 0 {
		t.Fatalf("lời gọi: manifest %+v, phía máy giả %d (sinh %d, nhúng %d, xếp lại %d)", m.Goi, c.soYeuCau(), c.may.SoSinh(), c.may.SoNhung(), c.xl.SoYeuCau())
	}
	if len(m.ModelVersions) != 1 || m.ModelVersions[0] != giagemini.ModelVersion {
		t.Fatalf("model version: %v", m.ModelVersions)
	}
	tep, _, err := DocBang(filepath.Join(goc.Dir, TepBangGhi))
	if err != nil {
		t.Fatal(err)
	}
	if len(tep.Goi) != m.Goi.MoHinh || len(tep.XepLai) != m.Goi.XepLai || tep.NhungModel != nhung.Model || tep.XepLaiModel != "gia-reranker" {
		t.Fatalf("cassette: %d model, %d xếp lại, nhúng %q", len(tep.Goi), len(tep.XepLai), tep.NhungModel)
	}
	coChung := false
	for _, g := range tep.Nhung {
		coChung = coChung || g.Pham == PhamChung
	}
	if !coChung {
		t.Fatal("kho ví dụ không được ghi trong phạm vi chung")
	}
	truoc := c.soYeuCau()
	lai := phatLai(t, goc.Dir, boNep, "")
	p := lai.Manifest
	if c.soYeuCau() != truoc || p.Goi.DaDung != 0 {
		t.Fatalf("phát lại gọi ra %d lần (manifest %d)", c.soYeuCau()-truoc, p.Goi.DaDung)
	}
	if p.SoVoiNguon == nil || !p.SoVoiNguon.Trung || p.ChamSha != m.ChamSha {
		t.Fatalf("điểm phát lại khác: %+v", p.SoVoiNguon)
	}
	if p.Goi.TuBang != len(tep.Goi)+len(tep.Nhung)+len(tep.XepLai) || len(p.TongKet.BangLech) != 0 {
		t.Fatalf("trả từ cassette %d, bang_lech %v", p.Goi.TuBang, p.TongKet.BangLech)
	}
	if p.TrangThai != m.TrangThai {
		t.Fatalf("trạng thái %s ≠ %s (%v)", p.TrangThai, m.TrangThai, p.LyDo)
	}
	// A recording from a stand-in never earns a trailer: only `that` does.
	if _, err := os.Stat(filepath.Join(lai.Dir, TepTrailer)); !os.IsNotExist(err) {
		t.Fatal("phát lại của lượt ghi loopback in trailer")
	}
	bd := string(docTep(t, lai.Dir, TepBangDiem))
	if !strings.Contains(bd, "trùng từng dòng") || !strings.Contains(bd, ChuaCoGia) {
		t.Fatalf("bảng điểm:\n%s", bd)
	}
}

// A cassette that lost one recording: the replay asks for it, gets
// bang_lech, and the run is void; still nothing reaches the network.
func TestPhatLaiThieuBanGhiLaBangLech(t *testing.T) {
	c := moCuaGia(t)
	goc := ghiBoNep(t, c, 5000, 1)
	p := filepath.Join(goc.Dir, TepBangGhi)
	tep, _, err := DocBang(p)
	if err != nil {
		t.Fatal(err)
	}
	tep.Goi = tep.Goi[1:]
	raw, _ := MaHoaBang(tep)
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	truoc := c.soYeuCau()
	lai := phatLai(t, goc.Dir, boNep, "")
	m := lai.Manifest
	if c.soYeuCau() != truoc {
		t.Fatal("bang_lech rơi xuống mạng")
	}
	if m.TrangThai != TrangThaiVoHieu || len(m.TongKet.BangLech) == 0 || m.TongKet.TruotTheoKiem[KiemBangLech] == 0 {
		t.Fatalf("thiếu bản ghi mà không đỏ: %s %v", m.TrangThai, m.LyDo)
	}
	if _, err := os.Stat(filepath.Join(lai.Dir, TepTrailer)); !os.IsNotExist(err) {
		t.Fatal("lượt bang_lech có trailer")
	}
}

// The ceiling is hard: with N = 7 exactly 7 provider requests reach the
// stand-ins, the run is unfinished, and it says why.
func TestTranGoiDungDungN(t *testing.T) {
	c := moCuaGia(t)
	kq := ghiBoNep(t, c, 7, 1)
	m := kq.Manifest
	if c.soYeuCau() != 7 || m.Goi.DaDung != 7 {
		t.Fatalf("trần 7: máy giả thấy %d, manifest %d", c.soYeuCau(), m.Goi.DaDung)
	}
	if m.TrangThai != TrangThaiChuaXong || m.TongKet.SoLuot >= m.TongKet.SoCa || !strings.Contains(strings.Join(m.LyDo, ";"), "chạm trần 7") {
		t.Fatalf("chạm trần mà: %s %v (%d lượt)", m.TrangThai, m.LyDo, m.TongKet.SoLuot)
	}
	if !strings.Contains(string(docTep(t, kq.Dir, TepBangDiem)), "CHƯA XONG") {
		t.Fatal("bảng điểm không nói chưa xong")
	}
}

// The watchdog stops the run at N even when N falls inside a turn: the
// call past N is refused inside the process (ErrTranGoi), never made.
func TestTranGoiTuChoiLoiGoiQuaN(t *testing.T) {
	calls := 0
	tran := NewTranGoi(2, func() { calls++ })
	m := tran.BocLLM(llm.NewStub(llm.Buoc{Text: "a"}, llm.Buoc{Text: "b"}, llm.Buoc{Text: "c"}))
	var loi []error
	for i := 0; i < 3; i++ {
		for _, err := range m.GenerateContent(context.Background(), &model.LLMRequest{Model: llm.Model}, false) {
			loi = append(loi, err)
		}
	}
	if loi[0] != nil || loi[1] != nil || loi[2] != ErrTranGoi || tran.DaDung() != 2 || calls != 1 || !tran.DaCham() {
		t.Fatalf("lỗi %v, đã dùng %d, khiCham %d", loi, tran.DaDung(), calls)
	}
	n := tran.BocNhung(nhung.Stub{})
	if _, err := n.Nhung(context.Background(), []string{"x"}, nhung.CauHoi); err != ErrTranGoi {
		t.Fatalf("nhúng qua trần: %v", err)
	}
}

// --du-toan is the exact upper bound the counters enforce: cases × laps ×
// (MaxModelCallsPerTurn + MaxEmbedCallsPerTurn + MaxRerankCallsPerTurn),
// plus the example bank once.
func TestDuToanDocDuHang(t *testing.T) {
	kbs, err := DocKichBan(kichBanDir)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := DocBo(boNep, kbs)
	if err != nil {
		t.Fatal(err)
	}
	n := len(b.Ca)
	d := TinhDuToan(b, 3, CoPhu{Nhung: true, SoViDu: 29, XepLai: true})
	if d.Tran != n*3*(8+2+2)+1 || d.GoiMoiLuot != llm.MaxModelCallsPerTurn || d.NhungMoiLuot != llm.MaxEmbedCallsPerTurn || d.XepLaiMoiLuot != llm.MaxRerankCallsPerTurn {
		t.Fatalf("dự toán %+v cho %d ca", d, n)
	}
	if d0 := TinhDuToan(b, 1, CoPhu{}); d0.Tran != n*8 {
		t.Fatalf("không nhúng, không xếp lại: %+v", d0)
	}
	raw, _ := os.ReadFile(boT1Hieu)
	bh, err := DocBoHieu(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dh := TinhDuToanHieu(bh, CoPhu{Nhung: true, SoViDu: 29, XepLai: true}); dh.Tran != len(bh.Ca)*(8+1)+1 || dh.XepLaiMoiLuot != 0 {
		t.Fatalf("dự toán router %+v", dh)
	}
	// The estimate must cover what a run really spent.
	c := moCuaGia(t)
	m := ghiBoNep(t, c, 5000, 1).Manifest
	if m.Goi.DaDung > m.Goi.DuToan.Tran {
		t.Fatalf("đã dùng %d > dự toán %d", m.Goi.DaDung, m.Goi.DuToan.Tran)
	}
}

// Router sets in the model modes: recorded, then replayed with identical
// grades and zero calls; the money metrics come with Wilson intervals.
func TestHieuGhiRoiPhatLai(t *testing.T) {
	c := moCuaGia(t)
	raw, _ := os.ReadFile(boT1Hieu)
	bh, err := DocBoHieu(raw)
	if err != nil {
		t.Fatal(err)
	}
	var kich []llm.Buoc
	for _, ca := range bh.Ca {
		for _, r := range ca.Ra {
			kich = append(kich, llm.Buoc{Text: string(r)})
		}
	}
	c.may.Dat(llm.NewStub(kich...))
	kq, err := ChayLuotMoHinh(context.Background(), YeuCauLuot{CheDo: MoHinhGhi, ChiBuoc: ChiBuocHieu, BoDuongDan: boT1Hieu,
		TranGoi: 1000, Goc: t.TempDir(), GitSHA: "abc1234def", Noi: c.noi, Nguon: NguonLoopback, Cho: khong})
	if err != nil {
		t.Fatal(err)
	}
	m := kq.Manifest
	if m.Hieu == nil || m.Hieu.SoCa != len(bh.Ca) || m.Goi.DaDung != int(c.soYeuCau()) || m.Goi.XepLai != 0 {
		t.Fatalf("lượt router: %+v %+v", m.Hieu, m.Goi)
	}
	if m.Hieu.Dat != m.Hieu.SoCa {
		t.Logf("router trên máy giả: %d/%d đạt (kịch bản T1 qua HTTP)", m.Hieu.Dat, m.Hieu.SoCa)
	}
	truoc := c.soYeuCau()
	lai := phatLai(t, kq.Dir, boT1Hieu, ChiBuocHieu)
	if c.soYeuCau() != truoc || !lai.Manifest.SoVoiNguon.Trung || lai.Manifest.TrangThai != m.TrangThai {
		t.Fatalf("phát lại router: %d lời gọi, %+v, %s/%s %v", c.soYeuCau()-truoc, lai.Manifest.SoVoiNguon, lai.Manifest.TrangThai, m.TrangThai, lai.Manifest.LyDo)
	}
	bd := string(docTep(t, kq.Dir, TepBangDiem))
	if !strings.Contains(bd, "recall lớp tiền") || !strings.Contains(bd, "Wilson") {
		t.Fatalf("bảng điểm router:\n%s", bd)
	}
}

// Wilson on known values (81/263 → 0.25529–0.36621, worked by hand from the
// score formula) and the
// edges.
func TestWilson(t *testing.T) {
	w := WilsonCua(81, 263)
	if w.Lo < 0.25528 || w.Lo > 0.25530 || w.Hi < 0.36620 || w.Hi > 0.36622 {
		t.Fatalf("%+v", w)
	}
	if w0 := WilsonCua(0, 80); w0.Lo != 0 || w0.Hi < 0.045 || w0.Hi > 0.046 {
		t.Fatalf("0/80: %+v", w0)
	}
	if WilsonCua(0, 0).String() != "— (n = 0)" {
		t.Fatal("n = 0")
	}
}

// The trailer exists only for a valid `that` run whose replay at the same
// SHA reproduced every grade with no call.
func TestTrailerChiKhiThatVaTrung(t *testing.T) {
	sach := false
	goc := Manifest{RunID: "r1", MoHinh: MoHinhThat, TrangThai: TrangThaiHopLe, GitSHA: "abc", CayBan: &sach,
		Bo: BoManifest{Ten: "nep", Sha: strings.Repeat("a", 64)}, ChamSha: strings.Repeat("b", 64), Model: llm.Model}
	lai := Manifest{RunID: "r2", MoHinh: MoHinhPhatLai, TrangThai: TrangThaiHopLe, GitSHA: "abc", SoVoiNguon: &SoVoi{RunID: "r1", Trung: true}}
	if tr, ok := Trailer(goc, lai); !ok || !strings.HasPrefix(tr, "Eval-Run: r1 ") || !strings.Contains(tr, "Eval-Phat-Lai: r2 goi=0") || !strings.Contains(tr, "Eval-Chi-Phi: "+ChuaCoGia) {
		t.Fatalf("trailer: %q %v", tr, ok)
	}
	hong := map[string]func(g, l *Manifest){
		"ghi":           func(g, l *Manifest) { g.MoHinh = MoHinhGhi },
		"chua_xong":     func(g, l *Manifest) { g.TrangThai = TrangThaiChuaXong },
		"khac_diem":     func(g, l *Manifest) { l.SoVoiNguon = &SoVoi{RunID: "r1", Trung: false} },
		"co_goi":        func(g, l *Manifest) { l.Goi.DaDung = 1 },
		"khac_sha":      func(g, l *Manifest) { l.GitSHA = "def" },
		"cay_ban":       func(g, l *Manifest) { b := true; g.CayBan = &b },
		"luot_khac":     func(g, l *Manifest) { l.SoVoiNguon = &SoVoi{RunID: "r9", Trung: true} },
		"phat_lai_hong": func(g, l *Manifest) { l.TrangThai = TrangThaiVoHieu },
	}
	for ten, f := range hong {
		g, l := goc, lai
		f(&g, &l)
		if _, ok := Trailer(g, l); ok {
			t.Errorf("%s: vẫn có trailer", ten)
		}
	}
}

// A run directory is never inside a git worktree.
func TestKhoBangChungNgoaiGit(t *testing.T) {
	if _, _, err := TaoThuMucLuot(".", "x"); err == nil {
		t.Fatal("thư mục trong repo được nhận")
	}
	dir, id, err := TaoThuMucLuot(t.TempDir(), "x")
	if err != nil || id != "x" {
		t.Fatal(err)
	}
	if _, id2, _ := TaoThuMucLuot(filepath.Dir(dir), "x"); id2 != "x-2" {
		t.Fatalf("tên trùng: %s", id2)
	}
}

// The manifest carries no content: no question, no answer.
func TestManifestKhongNoiDung(t *testing.T) {
	c := moCuaGia(t)
	kq := ghiBoNep(t, c, 5000, 1)
	raw := string(docTep(t, kq.Dir, TepManifest))
	var bo struct {
		Ca []struct {
			DauVao struct {
				Cau string `json:"cau"`
			} `json:"dau_vao"`
		} `json:"ca"`
	}
	braw, _ := os.ReadFile(boNep)
	_ = json.Unmarshal(braw, &bo)
	for _, ca := range bo.Ca {
		if len(ca.DauVao.Cau) > 8 && strings.Contains(raw, ca.DauVao.Cau) {
			t.Fatalf("manifest chứa câu hỏi %q", ca.DauVao.Cau)
		}
	}
}

// The stage read off a request agrees with the script's stage tag on every
// request of every correct-script run of the T1 corpus: the model modes
// grade by the same stages T1 does. (A wrong script may tag a step for a
// stage the engine never reached, or run past its end; those requests say
// nothing about the reader.)
func TestChangTuYeuCauKhopKichBan(t *testing.T) {
	kbs, err := DocKichBan(kichBanDir)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := DocBo(boNep, kbs)
	if err != nil {
		t.Fatal(err)
	}
	soYeuCau, dem := 0, map[string]int{}
	for _, c := range b.Ca {
		for _, k := range []string{c.KichBan.Dung} {
			l, _, err := chayKichBan(context.Background(), c, kbs[k], 1)
			if err != nil {
				t.Fatal(err)
			}
			for i, y := range l.YeuCau {
				soYeuCau++
				got, want := ChangTuYeuCau(y.SystemInstruction), l.Chang[i]
				if got != want {
					t.Errorf("%s/%s yêu cầu %d: đọc ra chặng %q, kịch bản ghi %q", c.CaID, k, i+1, got, want)
				}
				dem[got]++
			}
		}
	}
	for _, ch := range []string{ChangHieu, ChangTraLoi, ChangCham, ChangTraLoiCauTruc, ChangKiem} {
		if dem[ch] == 0 {
			t.Errorf("không yêu cầu nào thuộc chặng %s: phép so không thấy chặng đó", ch)
		}
	}
	t.Logf("%d yêu cầu, theo chặng %v", soYeuCau, dem)
}
