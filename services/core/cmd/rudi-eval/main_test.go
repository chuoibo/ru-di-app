//go:build eval

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"mobile/services/core/internal/aieval"
	"mobile/services/core/internal/aieval/giagemini"
	"mobile/services/core/internal/aiharness/llm"
)

const (
	boGoc      = "../../internal/aieval/testdata/corpus/nep-kich-ban.json"
	kichBanGoc = "../../internal/aieval/testdata/kich_ban"
)

func goi(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	rc := chay(context.Background(), args, strings.NewReader(stdin), &out, &errw)
	return rc, out.String(), errw.String()
}

// demRa is a transport that counts and refuses every request.
type demRa struct{ n atomic.Int64 }

func (d *demRa) RoundTrip(*http.Request) (*http.Response, error) {
	d.n.Add(1)
	return nil, errors.New("rudi-eval test: no request may leave the process")
}

// chiLoopback forwards requests to the loopback stand-ins and counts and
// refuses every other.
type chiLoopback struct {
	cu http.RoundTripper
	n  atomic.Int64
}

func (c *chiLoopback) RoundTrip(r *http.Request) (*http.Response, error) {
	if h := r.URL.Hostname(); h == "127.0.0.1" || h == "localhost" || h == "::1" {
		return c.cu.RoundTrip(r)
	}
	c.n.Add(1)
	return nil, errors.New("rudi-eval test: no request may leave the loopback")
}

// Invariant 10, at run time: with a key in the environment, `kich-ban` runs
// the whole corpus green and not one HTTP request leaves the process. genai's
// client would go through http.DefaultTransport; here that transport refuses
// and counts.
func TestKichBanKhongMoKetNoi(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "khoa-gia-khong-duoc-dung")
	d := &demRa{}
	cu := http.DefaultTransport
	http.DefaultTransport = d
	defer func() { http.DefaultTransport = cu }()
	rc, out, errw := goi(t, "", "--mo-hinh", "kich-ban", "--bo", boGoc)
	if rc != raXanh {
		t.Fatalf("thoát %d:\n%s", rc, errw)
	}
	if n := d.n.Load(); n != 0 {
		t.Fatalf("%d yêu cầu HTTP rời tiến trình", n)
	}
	dong := strings.Split(strings.TrimSpace(out), "\n")
	var cuoi struct {
		TongKet struct {
			Xanh   bool `json:"xanh"`
			SoCa   int  `json:"so_ca"`
			Canary struct {
				Dat   bool     `json:"dat"`
				Truot []string `json:"truot"`
			} `json:"canary"`
			DongNhat struct {
				Dat bool `json:"dat"`
			} `json:"dong_nhat"`
		} `json:"tong_ket"`
	}
	if err := json.Unmarshal([]byte(dong[len(dong)-1]), &cuoi); err != nil {
		t.Fatal(err)
	}
	tk := cuoi.TongKet
	if !tk.Xanh || tk.SoCa < 20 || !tk.Canary.Dat || strings.Join(tk.Canary.Truot, ",") != "khong_bia_dia_diem" || !tk.DongNhat.Dat {
		t.Fatalf("tổng kết: %+v", tk)
	}
	if !strings.HasSuffix(strings.TrimSpace(errw), "canary đỏ đúng chỗ; đồng nhất xanh") {
		t.Fatalf("stderr: %s", errw)
	}
}

// Bad invocations are refused before anything runs.
func TestThamSoSai(t *testing.T) {
	for _, args := range [][]string{{"--bo", boGoc}, {"--mo-hinh", "la", "--bo", boGoc}, {"--mo-hinh", "kich-ban", "--bo", boGoc, "--chi-buoc", "hieu"},
		{"--mo-hinh", "kich-ban", "--bo", boGoc, "--lap", "0"}, {"--mo-hinh", "kich-ban", "thua"}, {"--mo-hinh", "phat-lai"},
		{"--mo-hinh", "ghi"}, {"--mo-hinh", "that", "--bo", boGoc, "--chi-buoc", "tra_loi", "--tran-goi", "9"}} {
		if rc, _, _ := goi(t, "", args...); rc != raSai {
			t.Errorf("%v: thoát %d", args, rc)
		}
	}
}

const boT1Hieu = "../../internal/aieval/testdata/hieu/t1-hieu.json"

// moiTruongGia points the provider env at loopback stand-ins and scripts the
// model with the T1 router set's outputs, in case order.
func moiTruongGia(t *testing.T) (*giagemini.May, *giagemini.XepLai) {
	t.Helper()
	may, xl := giagemini.Moi(), giagemini.MoiXepLai()
	t.Cleanup(may.Close)
	t.Cleanup(xl.Close)
	t.Setenv("GEMINI_API_KEY", "khoa-gia-khong-duoc-dung")
	t.Setenv("MOBILE_GEMINI_BASE_URL", may.URL())
	t.Setenv("MOBILE_RERANK_URL", xl.URL())
	raw, err := os.ReadFile(boT1Hieu)
	if err != nil {
		t.Fatal(err)
	}
	b, err := aieval.DocBoHieu(raw)
	if err != nil {
		t.Fatal(err)
	}
	var kich []llm.Buoc
	for _, c := range b.Ca {
		for _, r := range c.Ra {
			kich = append(kich, llm.Buoc{Text: string(r)})
		}
	}
	may.Dat(llm.NewStub(kich...))
	return may, xl
}

func thuMuc(t *testing.T, out string) string {
	t.Helper()
	dong := strings.Split(strings.TrimSpace(out), "\n")
	var r struct {
		ThuMuc string `json:"thu_muc"`
	}
	if err := json.Unmarshal([]byte(dong[len(dong)-1]), &r); err != nil || r.ThuMuc == "" {
		t.Fatalf("không có thu_muc: %v %q", err, out)
	}
	return r.ThuMuc
}

// Invariant 10 at run time, with a key AND a reachable stand-in in the
// environment: `kich-ban` and `phat-lai` never enter the provider door
// (soLanDungNhaCungCap) and send no request; `ghi` enters it once. The
// replay of the recorded router set reproduces its grades with 0 calls.
func TestKichBanPhatLaiKhongDungClient(t *testing.T) {
	may, xl := moiTruongGia(t)
	cu := http.DefaultTransport
	d := &chiLoopback{cu: cu}
	http.DefaultTransport = d
	defer func() { http.DefaultTransport = cu }()
	dem := func() int64 { return may.SoYeuCau() + xl.SoYeuCau() + d.n.Load() }
	truoc := soLanDungNhaCungCap.Load()
	if rc, _, errw := goi(t, "", "--mo-hinh", "kich-ban", "--bo", boGoc); rc != raXanh || soLanDungNhaCungCap.Load() != truoc || dem() != 0 {
		t.Fatalf("kich-ban: thoát %d, cửa nhà cung cấp %d, %d yêu cầu\n%s", rc, soLanDungNhaCungCap.Load()-truoc, dem(), errw)
	}
	goc := t.TempDir()
	rc, out, errw := goi(t, "", "--mo-hinh", "ghi", "--chi-buoc", "hieu", "--bo", boT1Hieu, "--tran-goi", "400", "--out", goc, "--git-sha", "abc1234", "--cay", "sach")
	if rc != raXanh || soLanDungNhaCungCap.Load() != truoc+1 {
		t.Fatalf("ghi: thoát %d, cửa %d\n%s", rc, soLanDungNhaCungCap.Load()-truoc, errw)
	}
	dir := thuMuc(t, out)
	sau := dem()
	if sau == 0 || d.n.Load() != 0 {
		t.Fatalf("ghi: %d yêu cầu tới máy giả, %d ra ngoài", sau, d.n.Load())
	}
	rc, out, errw = goi(t, "", "--mo-hinh", "phat-lai", "--bang", dir, "--out", goc, "--git-sha", "abc1234")
	if rc != raXanh || soLanDungNhaCungCap.Load() != truoc+1 || dem() != sau {
		t.Fatalf("phat-lai: thoát %d, cửa %d, %d yêu cầu mới\n%s", rc, soLanDungNhaCungCap.Load()-truoc-1, dem()-sau, errw)
	}
	m, err := aieval.DocManifest(thuMuc(t, out))
	if err != nil || m.SoVoiNguon == nil || !m.SoVoiNguon.Trung || m.Goi.DaDung != 0 {
		t.Fatalf("phát lại: %v %+v", err, m.SoVoiNguon)
	}
}

// --tran-goi is required and hard: no flag, or an estimate over it, is
// refused before the provider door; --du-toan prints the estimate and
// builds nothing.
func TestThatCanTranGoi(t *testing.T) {
	moiTruongGia(t)
	truoc := soLanDungNhaCungCap.Load()
	if rc, _, errw := goi(t, "", "--mo-hinh", "that", "--bo", boGoc); rc != raSai || !strings.Contains(errw, "--tran-goi N bắt buộc") {
		t.Fatalf("thiếu trần: %d\n%s", rc, errw)
	}
	if rc, _, errw := goi(t, "", "--mo-hinh", "that", "--bo", boGoc, "--tran-goi", "10"); rc != raSai || !strings.Contains(errw, "TỪ CHỐI: dự toán") {
		t.Fatalf("dự toán > trần: %d\n%s", rc, errw)
	}
	rc, out, _ := goi(t, "", "--mo-hinh", "that", "--bo", boGoc, "--lap", "2", "--du-toan")
	var r struct {
		DuToan aieval.DuToan `json:"du_toan"`
	}
	if rc != raXanh || json.Unmarshal([]byte(out), &r) != nil || r.DuToan.Lap != 2 || r.DuToan.Tran != r.DuToan.SoLuot*(8+2+2)+1 {
		t.Fatalf("--du-toan: %d %s", rc, out)
	}
	if soLanDungNhaCungCap.Load() != truoc {
		t.Fatal("cửa nhà cung cấp được mở trước khi dự toán qua trần")
	}
}

// `that` is the real API: inside a test binary the constructor refuses the
// real host (testing.Testing), so the door opens and sends nothing; a
// missing key is refused with where to add it, echoing no value.
func TestThatDuoiTestBiTuChoi(t *testing.T) {
	moiTruongGia(t)
	t.Setenv("MOBILE_GEMINI_BASE_URL", "")
	d := &demRa{}
	cu := http.DefaultTransport
	http.DefaultTransport = d
	defer func() { http.DefaultTransport = cu }()
	rc, _, errw := goi(t, "", "--mo-hinh", "that", "--bo", boGoc, "--tran-goi", "100000", "--out", t.TempDir())
	if rc != raSai || !strings.Contains(errw, "loopback") || d.n.Load() != 0 {
		t.Fatalf("that dưới go test: %d, %d yêu cầu\n%s", rc, d.n.Load(), errw)
	}
	t.Setenv("GEMINI_API_KEY", "")
	rc, _, errw = goi(t, "", "--mo-hinh", "that", "--bo", boGoc, "--tran-goi", "100000", "--out", t.TempDir())
	if rc != raSai || !strings.Contains(errw, "cài đặt môi trường") {
		t.Fatalf("thiếu khoá: %d\n%s", rc, errw)
	}
	// ghi with a non-loopback override is refused by the constructor.
	t.Setenv("GEMINI_API_KEY", "khoa-gia-khong-duoc-dung")
	t.Setenv("MOBILE_GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/")
	if rc, _, errw := goi(t, "", "--mo-hinh", "ghi", "--bo", boGoc, "--tran-goi", "100000", "--out", t.TempDir()); rc != raSai || d.n.Load() != 0 {
		t.Fatalf("ghi tới máy thật: %d\n%s", rc, errw)
	}
}

// A canary that is not red where the corpus says turns the verdict red; a
// corpus without its canary is refused before anything runs.
func TestCanaryPhaiDoDungCho(t *testing.T) {
	raw, err := os.ReadFile(boGoc)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var canary map[string]any
	for _, c := range m["ca"].([]any) {
		if c.(map[string]any)["case_id"] == "00-canary-phai-do" {
			canary = c.(map[string]any)
		}
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "corpus"), 0o755); err != nil {
		t.Fatal(err)
	}
	viet := func() string {
		b, _ := json.Marshal(m)
		p := filepath.Join(dir, "corpus", "bo.json")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Predicted at the wrong check: the run is red.
	canary["phai_do_o"] = []any{"chu"}
	rc, _, errw := goi(t, "", "--mo-hinh", "kich-ban", "--bo", viet(), "--kich-ban", kichBanGoc)
	if rc != raDo || !strings.Contains(errw, "canary SAI") {
		t.Fatalf("canary đỏ sai chỗ vẫn qua: %d\n%s", rc, errw)
	}
	// A canary whose script turns green: red too.
	canary["phai_do_o"] = []any{"khong_bia_dia_diem"}
	canary["kich_ban"] = map[string]any{"dung": "tra-loi-cuoi-tuan"}
	rc, _, errw = goi(t, "", "--mo-hinh", "kich-ban", "--bo", viet(), "--kich-ban", kichBanGoc)
	if rc != raDo {
		t.Fatalf("canary xanh vẫn qua: %d\n%s", rc, errw)
	}
	// No canary at all: refused.
	delete(canary, "canh_gac")
	delete(canary, "phai_do_o")
	rc, _, errw = goi(t, "", "--mo-hinh", "kich-ban", "--bo", viet(), "--kich-ban", kichBanGoc)
	if rc != raSai || !strings.Contains(errw, "canary") {
		t.Fatalf("thiếu canary: %d\n%s", rc, errw)
	}
}

// The line protocol of design 06 §3.2: constants, one case, and the ops of
// later slices answered with an error rather than silence.
func TestGiaoThucDong(t *testing.T) {
	raw, err := os.ReadFile(boGoc)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Ca []json.RawMessage `json:"ca"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var dongNhat json.RawMessage
	for _, c := range m.Ca {
		if bytes.Contains(c, []byte(`"case_id": "00-dong-nhat"`)) || bytes.Contains(c, []byte(`"case_id":"00-dong-nhat"`)) {
			dongNhat = c
		}
	}
	if dongNhat == nil {
		t.Fatal("không thấy ca đồng nhất")
	}
	var nen bytes.Buffer
	if err := json.Compact(&nen, dongNhat); err != nil {
		t.Fatal(err)
	}
	in := `{"op":"hang"}` + "\n" + `{"op":"chay","ca":` + nen.String() + `,"lap":2}` + "\n" + `{"op":"hieu"}` + "\n" + `{"op":"la"}` + "\n"
	rc, out, errw := goi(t, in, "--mo-hinh", "kich-ban", "--kich-ban", kichBanGoc)
	dong := strings.Split(strings.TrimSpace(out), "\n")
	if rc != raDo || len(dong) != 5 {
		t.Fatalf("thoát %d, %d dòng:\n%s\n%s", rc, len(dong), out, errw)
	}
	if !strings.Contains(dong[0], `"max_model_calls_per_turn":8`) || !strings.Contains(dong[0], `"cong_cu":{"nep":["search_places",`) {
		t.Fatalf("hang: %s", dong[0])
	}
	for i := 1; i <= 2; i++ {
		var r struct {
			CaID string `json:"case_id"`
			Lap  int    `json:"lap"`
			Dat  bool   `json:"dat"`
		}
		if err := json.Unmarshal([]byte(dong[i]), &r); err != nil || r.CaID != "00-dong-nhat" || r.Lap != i || !r.Dat {
			t.Fatalf("chay %d: %v %s", i, err, dong[i])
		}
	}
	if !strings.Contains(dong[3], `"loi"`) || !strings.Contains(dong[3], "lát 9") || !strings.Contains(dong[4], "op lạ") {
		t.Fatalf("op sau: %s / %s", dong[3], dong[4])
	}
	// Review round 2 (nit): a run that is red makes the exit red, even
	// when every line was read and answered. The identity case, played by a
	// script whose words differ from what the case pins, is such a run; the
	// same case with its own script exits green.
	if rc, out, errw := goi(t, `{"op":"chay","ca":`+nen.String()+`}`+"\n", "--mo-hinh", "kich-ban", "--kich-ban", kichBanGoc); rc != raXanh {
		t.Fatalf("ca xanh: thoát %d\n%s\n%s", rc, out, errw)
	}
	do := strings.Replace(nen.String(), `"dung":"tra-loi-ngan"`, `"dung":"tra-loi-khac"`, 1)
	if do == nen.String() {
		t.Fatal("không đổi được kịch bản của ca đồng nhất")
	}
	if rc, out, errw := goi(t, `{"op":"chay","ca":`+do+`}`+"\n", "--mo-hinh", "kich-ban", "--kich-ban", kichBanGoc); rc != raDo || !strings.Contains(out, `"dat":false`) {
		t.Fatalf("lượt đỏ vẫn thoát %d:\n%s\n%s", rc, out, errw)
	}
	// A line that is not JSON stops the protocol.
	if rc, _, _ := goi(t, "{không phải json\n", "--mo-hinh", "kich-ban", "--kich-ban", kichBanGoc); rc != raSai {
		t.Fatalf("dòng hỏng: thoát %d", rc)
	}
}

// --chi-buoc hieu runs the router's T1 set green with no request leaving the
// process, and refuses a converted T3 set, which needs a real model.
func TestChiBuocHieu(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "khoa-gia-khong-duoc-dung")
	d := &demRa{}
	cu := http.DefaultTransport
	http.DefaultTransport = d
	defer func() { http.DefaultTransport = cu }()
	rc, out, errw := goi(t, "", "--mo-hinh", "kich-ban", "--chi-buoc", "hieu", "--bo", "../../internal/aieval/testdata/hieu/t1-hieu.json")
	if rc != raXanh || d.n.Load() != 0 {
		t.Fatalf("thoát %d, %d yêu cầu:\n%s", rc, d.n.Load(), errw)
	}
	dong := strings.Split(strings.TrimSpace(out), "\n")
	var cuoi struct {
		TongKet struct {
			SoCa int `json:"so_ca"`
			Dat  int `json:"dat"`
		} `json:"tong_ket"`
	}
	if err := json.Unmarshal([]byte(dong[len(dong)-1]), &cuoi); err != nil || cuoi.TongKet.SoCa < 20 || cuoi.TongKet.Dat != cuoi.TongKet.SoCa {
		t.Fatalf("%v %+v", err, cuoi)
	}
	rc, _, errw = goi(t, "", "--mo-hinh", "kich-ban", "--chi-buoc", "hieu", "--bo", "../../internal/aieval/testdata/hieu/tien_v3.json")
	if rc != raSai || !strings.Contains(errw, "router thật") {
		t.Fatalf("thoát %d:\n%s", rc, errw)
	}
	for _, args := range [][]string{{"--mo-hinh", "kich-ban", "--chi-buoc", "hieu"}, {"--mo-hinh", "kich-ban", "--chi-buoc", "tra_loi", "--bo", boGoc}} {
		if rc, _, _ := goi(t, "", args...); rc != raSai {
			t.Errorf("%v: thoát %d", args, rc)
		}
	}
}
