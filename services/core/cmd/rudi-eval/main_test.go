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

// The modes design 06 names for later slices are refused, not faked.
func TestMoHinhChuaCo(t *testing.T) {
	for _, m := range []string{"that", "ghi", "phat-lai"} {
		rc, _, errw := goi(t, "", "--mo-hinh", m, "--bo", boGoc)
		if rc != raSai || !strings.Contains(errw, "chưa có ở lát 6b") {
			t.Errorf("%s: %d %s", m, rc, errw)
		}
	}
	for _, args := range [][]string{{"--bo", boGoc}, {"--mo-hinh", "kich-ban", "--bo", boGoc, "--chi-buoc", "hieu"}, {"--mo-hinh", "kich-ban", "--bo", boGoc, "--lap", "0"}, {"--mo-hinh", "kich-ban", "thua"}} {
		if rc, _, _ := goi(t, "", args...); rc != raSai {
			t.Errorf("%v: thoát %d", args, rc)
		}
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
	if !strings.Contains(dong[0], `"max_model_calls_per_turn":8`) || !strings.Contains(dong[0], `"cong_cu":{"nep":[]}`) {
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
