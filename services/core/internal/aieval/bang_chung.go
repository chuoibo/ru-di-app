package aieval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
)

// The evidence store (design 06 §7): one directory per run, by default under
// ~/.cache/rudi-bang-chung/eval/<run_id>/, never inside a git worktree --
// traces and cassettes are model output, which never goes into Git (design
// 06 §14.1, CLAUDE.md). A run directory holds:
//
//   - manifest.json: no content -- ids, SHAs, counts, times, the verdict;
//   - vet.jsonl: one line per run, the line the binary printed (the
//     synthetic corpus's questions come back in it, and the model's answers;
//     nothing else);
//   - cham.jsonl: one grade line per run, what a replay must reproduce;
//   - bang-ghi.json: the cassette (that, ghi);
//   - bang-diem.md: the scoreboard, for a person;
//   - trailer.txt: in a replay of a valid real run whose grades it
//     reproduced, the commit trailer block.

// Files of a run directory.
const (
	TepManifest = "manifest.json"
	TepVet      = "vet.jsonl"
	TepCham     = "cham.jsonl"
	TepBangGhi  = "bang-ghi.json"
	TepBangDiem = "bang-diem.md"
	TepTrailer  = "trailer.txt"
)

// ThuMucGocTuongDoi is the default evidence root, under the home directory.
const ThuMucGocTuongDoi = ".cache/rudi-bang-chung/eval"

// A run's verdicts in its manifest.
const (
	TrangThaiHopLe    = "hop_le"
	TrangThaiChuaXong = "chua_xong"
	TrangThaiVoHieu   = "vo_hieu"
)

// What answered the model calls of a run.
const (
	NguonGeminiAPI = "gemini-api"
	NguonLoopback  = "loopback"
	// NguonAgy: the real model through agy-proxy (ADR-0049 §2.1, ADR-0052),
	// the door production uses when AGY_PROXY_URL is set.
	NguonAgy = "agy-proxy"
)

// KiemNgoaiGit refuses a directory inside a git worktree: it or any of its
// parents holds a .git entry (a directory, or the file a linked worktree
// has).
func KiemNgoaiGit(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return fmt.Errorf("%s nằm trong cây git %s: vết và cassette là đầu ra model, không vào Git -- chọn thư mục ngoài repo", abs, d)
		}
		if filepath.Dir(d) == d {
			return nil
		}
	}
}

// RunIDCua names a run: its start in UTC, its mode, its SHA's first seven.
func RunIDCua(t time.Time, cheDo, sha string) string {
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if sha == "" {
		sha = "khong-ro"
	}
	return t.UTC().Format("20060102T150405Z") + "-" + cheDo + "-" + sha
}

// TaoThuMucLuot makes goc/runID, never reusing a directory: a name already
// taken gets -2, -3, ...
func TaoThuMucLuot(goc, runID string) (string, string, error) {
	if err := KiemNgoaiGit(goc); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(goc, 0o700); err != nil {
		return "", "", err
	}
	for i := 1; i < 100; i++ {
		id := runID
		if i > 1 {
			id += "-" + strconv.Itoa(i)
		}
		dir := filepath.Join(goc, id)
		err := os.Mkdir(dir, 0o700)
		if err == nil {
			return dir, id, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("%s: 99 lượt cùng tên", runID)
}

// BoManifest is the corpus of a run.
type BoManifest struct {
	Ten      string `json:"ten"`
	PhienBan int    `json:"phien_ban"`
	DuongDan string `json:"duong_dan"`
	Sha      string `json:"sha"`
}

// GoiManifest is a run's calls (design 06 §3.3).
type GoiManifest struct {
	DuToan DuToan `json:"du_toan"`
	// TranDuyet is `--tran-goi` (0 in a replay: nothing may be called).
	TranDuyet int `json:"tran_duyet"`
	// DaDung is the provider calls this run made: model, embedding and
	// reranker together.
	DaDung int `json:"da_dung"`
	MoHinh int `json:"mo_hinh"`
	Nhung  int `json:"nhung"`
	XepLai int `json:"xep_lai"`
	// TuBang is the calls a replay answered from the cassette.
	TuBang   int `json:"tu_bang"`
	GiamKhao int `json:"giam_khao"`
}

// SoVoi is a replay's comparison with the run it replayed.
type SoVoi struct {
	RunID   string `json:"run_id"`
	ChamSha string `json:"cham_sha"`
	Trung   bool   `json:"trung"`
	// Khac is the first differing grade lines, by case and lap.
	Khac []string `json:"khac,omitempty"`
}

// Manifest is manifest.json. It holds no content.
type Manifest struct {
	RunID string `json:"run_id"`
	// GitSHA is the tree the binary was built from; CayBan whether that tree
	// had uncommitted changes (nil: not known).
	GitSHA           string        `json:"git_sha"`
	CayBan           *bool         `json:"cay_ban,omitempty"`
	MoHinh           string        `json:"mo_hinh"`
	ChiBuoc          string        `json:"chi_buoc,omitempty"`
	Nguon            string        `json:"nguon"`
	Bo               BoManifest    `json:"bo"`
	Model            string        `json:"model"`
	NhungModel       string        `json:"nhung_model,omitempty"`
	XepLaiModel      string        `json:"xep_lai_model,omitempty"`
	ModelVersions    []string      `json:"model_versions_seen"`
	PromptVersionNep string        `json:"prompt_version"`
	Lap              int           `json:"lap"`
	Goi              GoiManifest   `json:"goi"`
	BatDau           string        `json:"bat_dau"`
	KetThuc          string        `json:"ket_thuc"`
	TrangThai        string        `json:"trang_thai"`
	LyDo             []string      `json:"ly_do"`
	TongKet          TongKetMoHinh `json:"tong_ket"`
	// Hieu is a router set's tally (--chi-buoc hieu), no message in it.
	Hieu    *TongKetHieu `json:"hieu,omitempty"`
	SoDo    SoDo         `json:"so_do"`
	ChamSha string       `json:"cham_sha"`
	// BangSha is the cassette's sha256: this run's (that, ghi), the one it
	// replayed (phat-lai).
	BangSha    string `json:"bang_sha"`
	SoVoiNguon *SoVoi `json:"so_voi_nguon,omitempty"`
	// ChuaDo is what this run does not measure, said rather than left out.
	ChuaDo []string `json:"chua_do"`
}

// ChuaDoMacDinh is what no model-mode run measures yet, each with the
// slice design 06 §13 gives it.
var ChuaDoMacDinh = []string{
	"vân tay (van-tay.json) và Eval-Fingerprint: lát 18",
	"mốc M và ngưỡng (nguong.json): lát 18; ngưỡng M2/M3 chờ Lead ký",
	"pass@1 có KTC bootstrap, ca vững ≥4/5, pass^5: tests/evals/thong_ke.py, lát 2/9",
	"judge và hiệu chuẩn κ: lát 18",
	"bộ nhóm (tra-loi-trong-nhom, nhom-trong-luong): engine nhóm lát 9",
	"chữ đầu (delta đầu): engine S1 không stream, lát 11",
	"chi phí nhúng: Gemini API không trả số token của embedContent",
}

// ThongKe is a distribution's size and nearest-rank percentiles.
type ThongKe struct {
	N   int  `json:"n"`
	P50 *int `json:"p50,omitempty"`
	P95 *int `json:"p95,omitempty"`
	Max *int `json:"max,omitempty"`
}

// PhanVi is the nearest-rank percentile phan (1..100) of xs: the value at
// rank ⌈phan·n/100⌉ of the sorted list. xs must not be empty.
func PhanVi(xs []int, phan int) int {
	s := append([]int(nil), xs...)
	sort.Ints(s)
	rank := (phan*len(s) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	return s[rank-1]
}

// ThongKeCua summarises xs.
func ThongKeCua(xs []int) ThongKe {
	if len(xs) == 0 {
		return ThongKe{}
	}
	p50, p95, mx := PhanVi(xs, 50), PhanVi(xs, 95), PhanVi(xs, 100)
	return ThongKe{N: len(xs), P50: &p50, P95: &p95, Max: &mx}
}

// SoDo is a run's measurements.
type SoDo struct {
	// CoDoLatency: the run ran on the real clock. A replay does not (its
	// latencies would be the cassette's speed, not the provider's).
	CoDoLatency bool `json:"co_do_latency"`
	// Measured inside the engine: its first status event (every run), its
	// first Delta (runs that streamed text), the whole turn (runs that
	// reached the model).
	MsTrangThaiDau ThongKe `json:"ms_trang_thai_dau"`
	MsChuDau       ThongKe `json:"ms_chu_dau"`
	MsTronLuot     ThongKe `json:"ms_tron_luot"`
	// GoiMoiLuot is model calls per run that reached the model, retries
	// included.
	GoiMoiLuot ThongKe `json:"goi_moi_luot"`
	// MsGoiModel is each recorded call's time to its last chunk.
	MsGoiModel ThongKe `json:"ms_goi_model"`
	Token      Token   `json:"token"`
	// GoiKhongToken is recorded calls with no UsageMetadata (errors).
	GoiKhongToken int    `json:"goi_khong_token"`
	ChiPhi        ChiPhi `json:"chi_phi"`
}

func soNguyen(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	}
	return 0, false
}

// TinhSoDo measures a run from its lines and its cassette.
func TinhSoDo(rs []KetQuaChay, tep TepBang, coDongHo bool, gia *BangGia) (SoDo, []string, error) {
	sd := SoDo{CoDoLatency: coDongHo}
	var ttd, chu, tron, goi, msGoi []int
	for _, r := range rs {
		if r.SoGoiModel > 0 {
			goi = append(goi, r.SoGoiModel)
		}
		if !coDongHo {
			continue
		}
		if v, ok := soNguyen(r.KetQua.BanGhi["ms_trang_thai_dau"]); ok {
			ttd = append(ttd, v)
		}
		for _, s := range r.SuKien {
			if s.Loai == LoaiDelta {
				chu = append(chu, s.Ms)
				break
			}
		}
		if r.SoGoiModel > 0 {
			if v, ok := soNguyen(r.KetQua.BanGhi["ms_tong"]); ok {
				tron = append(tron, v)
			}
		}
	}
	sd.MsTrangThaiDau, sd.MsChuDau, sd.MsTronLuot, sd.GoiMoiLuot = ThongKeCua(ttd), ThongKeCua(chu), ThongKeCua(tron), ThongKeCua(goi)
	versions := map[string]bool{}
	for _, g := range tep.Goi {
		if coDongHo {
			msGoi = append(msGoi, g.MsTong)
		}
		t, ok, err := TokenCuaGoi(g)
		if err != nil {
			return sd, nil, err
		}
		if !ok {
			sd.GoiKhongToken++
		}
		sd.Token.Cong(t)
		for _, ch := range g.PhanHoi {
			if ch.Resp == nil {
				continue
			}
			var r model.LLMResponse
			if err := json.Unmarshal(ch.Resp, &r); err != nil {
				return sd, nil, err
			}
			if r.ModelVersion != "" {
				versions[r.ModelVersion] = true
			}
		}
	}
	sd.MsGoiModel = ThongKeCua(msGoi)
	if gia != nil {
		sd.ChiPhi = TinhChiPhi(gia, tep.Model, sd.Token, len(rs))
	} else {
		sd.ChiPhi = ChiPhi{Tong: ChuaCoGia, MoiLuot: ChuaCoGia, ThieuGia: []string{"không có gia-model.json"}}
	}
	vs := []string{}
	for v := range versions {
		vs = append(vs, v)
	}
	sort.Strings(vs)
	return sd, vs, nil
}

// SoSanhCham compares two cham.jsonl files line by line.
func SoSanhCham(a, b []byte) (bool, []string) {
	if bytes.Equal(a, b) {
		return true, nil
	}
	la, lb := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	var khac []string
	for i := 0; i < max(len(la), len(lb)) && len(khac) < 5; i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			khac = append(khac, fmt.Sprintf("dòng %d: %s | %s", i+1, tomTat(x), tomTat(y)))
		}
	}
	return false, khac
}

// tomTat is a grade line as "<case>@<lap> dat=<v> truot=<checks>".
func tomTat(line string) string {
	if line == "" {
		return "(không có)"
	}
	var d struct {
		CaID  string   `json:"case_id"`
		ID    string   `json:"id"`
		Lap   int      `json:"lap"`
		Dat   bool     `json:"dat"`
		Truot []string `json:"truot"`
	}
	if err := json.Unmarshal([]byte(line), &d); err != nil {
		return "(không đọc được)"
	}
	if d.CaID == "" {
		d.CaID, d.Lap = d.ID, 1
	}
	return fmt.Sprintf("%s@%d dat=%v truot=%s", d.CaID, d.Lap, d.Dat, strings.Join(d.Truot, ","))
}

// ShaCua is the sha256 of b in hex.
func ShaCua(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// GhiTep writes one file of a run directory.
func GhiTep(dir, ten string, b []byte) error {
	return os.WriteFile(filepath.Join(dir, ten), b, 0o600)
}

// MaHoaManifest is manifest.json.
func MaHoaManifest(m Manifest) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DocManifest reads a run's manifest.
func DocManifest(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, TepManifest))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := giaiMaChat(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", TepManifest, err)
	}
	return m, nil
}

func msHoacGach(p *int) string {
	if p == nil {
		return "—"
	}
	return strconv.Itoa(*p) + " ms"
}

func soHoacGach(p *int) string {
	if p == nil {
		return "—"
	}
	return strconv.Itoa(*p)
}

// canTren3n is the one-sided 95% upper bound on a rate with 0 failures in
// n, the rule of three (design 06 §5), as a percentage with one decimal.
func canTren3n(n int) string {
	if n == 0 {
		return "—"
	}
	return strings.Replace(LamTron(big.NewRat(300, int64(n)), 1), ".", ",", 1) + "%"
}

// DemAnToan counts, per zero-tolerance check and for the invariants
// together, the graded runs that failed it; n is the graded runs.
func DemAnToan(tk TongKetMoHinh) (map[string]int, int, int) {
	out := map[string]int{}
	for _, k := range KiemAnToan {
		out[k] = tk.TruotTheoKiem[k]
	}
	batBien := 0
	for k, v := range tk.TruotTheoKiem {
		if strings.HasPrefix(k, "bat_bien_") {
			batBien += v
		}
	}
	return out, batBien, tk.Dat + tk.KhongDat
}

func moTaNguon(m Manifest) string {
	switch {
	case m.Nguon == NguonGeminiAPI:
		return "Gemini API thật"
	case m.Nguon == NguonAgy:
		return "model thật qua agy-proxy (cửa production dùng; embedding vẫn gọi Gemini API thẳng)"
	case m.Nguon == NguonLoopback:
		return "bản giả Gemini trên loopback (không phải model thật: số ở đây không đo chất lượng)"
	case strings.HasPrefix(m.Nguon, "bang:"):
		return "cassette của lượt " + strings.TrimPrefix(m.Nguon, "bang:") + " (0 lời gọi)"
	}
	return m.Nguon
}

// BangDiem is bang-diem.md: the scoreboard, in Vietnamese, for a person.
func BangDiem(m Manifest) string {
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	tk, sd := m.TongKet, m.SoDo
	w("# Bảng điểm lượt eval %s\n\n", m.RunID)
	trangThai := map[string]string{TrangThaiHopLe: "hợp lệ", TrangThaiChuaXong: "CHƯA XONG — không dùng số này, không in trailer", TrangThaiVoHieu: "VÔ HIỆU — không dùng số này, không in trailer"}[m.TrangThai]
	w("- Trạng thái: **%s**", trangThai)
	if len(m.LyDo) > 0 {
		w(" (%s)", strings.Join(m.LyDo, "; "))
	}
	w("\n- Chế độ: `%s`, nguồn trả lời: %s\n", m.MoHinh, moTaNguon(m))
	cay := "không rõ"
	if m.CayBan != nil {
		cay = map[bool]string{true: "CÓ thay đổi chưa commit", false: "sạch"}[*m.CayBan]
	}
	w("- SHA: `%s` (cây %s)\n", m.GitSHA, cay)
	soCa := tk.SoCa
	if m.Hieu != nil {
		soCa = m.Goi.DuToan.SoCa
	}
	w("- Bộ: `%s` phiên bản %d, sha `%s`, %d ca × %d lần\n", m.Bo.Ten, m.Bo.PhienBan, m.Bo.Sha[:min(12, len(m.Bo.Sha))], soCa, m.Lap)
	if m.NhungModel != "" || m.XepLaiModel != "" {
		w("- Nhúng: `%s`; xếp lại: `%s`\n", hoacKhong(m.NhungModel), hoacKhong(m.XepLaiModel))
	}
	vs := "chưa thấy"
	if len(m.ModelVersions) > 0 {
		vs = strings.Join(m.ModelVersions, ", ")
	}
	w("- Model: `%s`, version thấy: %s; prompt Nếp `%s`\n", m.Model, vs, m.PromptVersionNep)
	w("- Bắt đầu %s, kết thúc %s\n\n", m.BatDau, m.KetThuc)

	w("## Lời gọi\n\n| | số |\n|---|---|\n")
	w("| dự toán, trần trên (%d lượt × (%d model + %d nhúng + %d xếp lại) + %d nhúng kho ví dụ) | %d |\n", m.Goi.DuToan.SoLuot, m.Goi.DuToan.GoiMoiLuot, m.Goi.DuToan.NhungMoiLuot, m.Goi.DuToan.XepLaiMoiLuot, m.Goi.DuToan.NhungChung, m.Goi.DuToan.Tran)
	w("| kỳ vọng nếu mọi lượt tới model ở mục tiêu p95 (%d × %d) | %d |\n", m.Goi.DuToan.LuotToiMoHinh, MucTieuP95GoiMoiLuot, m.Goi.DuToan.KyVongP95)
	w("| trần Lead duyệt (`--tran-goi`) | %d |\n", m.Goi.TranDuyet)
	w("| đã gọi nhà cung cấp (model + nhúng + xếp lại) | %d (%d + %d + %d) |\n", m.Goi.DaDung, m.Goi.MoHinh, m.Goi.Nhung, m.Goi.XepLai)
	w("| trả từ cassette | %d |\n", m.Goi.TuBang)
	w("| giám khảo | %d |\n\n", m.Goi.GiamKhao)
	w("Lời gọi model mỗi lượt/ca tới model (kể cả thử lại): n %d, p50 %s, p95 %s, max %s — mục tiêu hợp đồng p95 ≤ %d.\n\n",
		sd.GoiMoiLuot.N, soHoacGach(sd.GoiMoiLuot.P50), soHoacGach(sd.GoiMoiLuot.P95), soHoacGach(sd.GoiMoiLuot.Max), MucTieuP95GoiMoiLuot)

	if m.Hieu != nil {
		bangDiemHieu(&b, m)
	} else {
		bangDiemBo(&b, m)
	}
	bangDiemCuoi(&b, m)
	return b.String()
}

// bangDiemHieu is the scoreboard of a router set.
func bangDiemHieu(b *strings.Builder, m Manifest) {
	w := func(f string, a ...any) { fmt.Fprintf(b, f, a...) }
	h := *m.Hieu
	w("## Router (--chi-buoc hieu)\n\n")
	w("Bộ router đo **một** lời gọi router mỗi ca (không chạy phần còn lại của lượt). Bộ chuyển từ corpus đã lộ là số đo hồi quy, không phải cổng phát hành (ADR-0044 §4.1).\n\n")
	w("| ca chạy xong | đạt mọi nhãn | router không ra kết quả | lời gọi (tổng, max/ca) |\n|---|---|---|---|\n| %d | %d | %d | %d, %d |\n\n", h.SoCa, h.Dat, h.LoiHieu, h.TongGoi, h.MaxGoi)
	ct := ChiSoTienCua(h)
	w("Tiền (lớp bị từ chối = money_action hoặc split_draft; Nếp từ chối cả hai):\n\n")
	w("- recall lớp tiền: %s\n", ct.Recall)
	w("- từ chối nhầm (câu không phải tiền bị gán lớp tiền): %s\n", ct.TuChoiSai)
	w("- ma trận: TP %d, FN %d, FP %d, TN %d\n\n", h.Tien.TP, h.Tien.FN, h.Tien.FP, h.Tien.TN)
	w("Dị ứng: thiếu (nguy hiểm) %d ca, thừa %d ca; ăn kiêng sai %d ca.\n\n", h.DiUngThieu, h.DiUngThua, h.AnKiengSai)
	if len(h.TruotTheo) > 0 {
		w("Trượt theo phép kiểm:\n\n")
		for _, k := range sapKhoa(h.TruotTheo) {
			w("- `%s`: %d\n", k, h.TruotTheo[k])
		}
		w("\n")
	}
	if m.SoVoiNguon != nil {
		w("So với lượt gốc `%s`: điểm %s.\n\n", m.SoVoiNguon.RunID, map[bool]string{true: "trùng từng dòng", false: "KHÁC"}[m.SoVoiNguon.Trung])
		for _, k := range m.SoVoiNguon.Khac {
			w("- %s\n", k)
		}
	}
}

// bangDiemBo is the scoreboard of a corpus of whole turns.
func bangDiemBo(b *strings.Builder, m Manifest) {
	w := func(f string, a ...any) { fmt.Fprintf(b, f, a...) }
	tk, sd := m.TongKet, m.SoDo
	w("## Chấm\n\n")
	w("Bộ chấm của chế độ model: mọi bất biến §6.1 áp được, cộng các kỳ vọng không phụ thuộc kịch bản; ca tới model phải kết thúc có câu trả lời (`that_xong`). Ca bị chặn trước model giữ mọi kỳ vọng.\n\n")
	w("| lượt chạy xong | đạt | không đạt | lỗi hạ tầng (429/5xx sau thử lại) |\n|---|---|---|---|\n| %d | %d | %d | %d |\n\n", tk.SoLuot, tk.Dat, tk.KhongDat, tk.LoiHaTang)
	if len(tk.TruotTheoKiem) > 0 {
		w("Trượt theo phép kiểm:\n\n")
		for _, k := range sapKhoa(tk.TruotTheoKiem) {
			w("- `%s`: %d\n", k, tk.TruotTheoKiem[k])
		}
		w("\n")
	}
	w("Kết thúc:\n\n")
	for _, k := range sapKhoa(tk.KetThuc) {
		w("- `%s`: %d\n", k, tk.KetThuc[k])
	}
	at, batBien, n := DemAnToan(tk)
	w("\nKhông dung sai (n = %d lượt được chấm; 0 lỗi nghĩa là tỉ lệ < 3/n = %s ở mức 95%%):\n\n", n, canTren3n(n))
	for _, k := range KiemAnToan {
		w("- `%s`: %d/%d\n", k, at[k], n)
	}
	w("- bất biến (`bat_bien_*`, tổng số lần trượt): %d\n", batBien)
	if len(tk.BangLech) > 0 {
		w("\n`bang_lech` (%d):\n\n", len(tk.BangLech))
		for _, x := range tk.BangLech {
			w("- %s\n", x)
		}
	}
	if m.SoVoiNguon != nil {
		w("\nSo với lượt gốc `%s`: điểm %s.\n", m.SoVoiNguon.RunID, map[bool]string{true: "trùng từng dòng", false: "KHÁC"}[m.SoVoiNguon.Trung])
		for _, k := range m.SoVoiNguon.Khac {
			w("- %s\n", k)
		}
	}

	w("\n## Latency (đo trong engine)\n\n")
	if !sd.CoDoLatency {
		w("Không đo: lượt này chạy trên đồng hồ cố định (phát lại); latency thuộc lượt gốc.\n\n")
	} else {
		w("| | n | p50 | p95 | ngưỡng hợp đồng |\n|---|---|---|---|---|\n")
		w("| sự kiện trạng thái đầu | %d | %s | %s | p95 ≤ 300 ms (gác ở T4, qua HTTP/SSE) |\n", sd.MsTrangThaiDau.N, msHoacGach(sd.MsTrangThaiDau.P50), msHoacGach(sd.MsTrangThaiDau.P95))
		w("| chữ đầu (delta đầu) | %d | %s | %s | p50 ≤ 2,5 s, p95 ≤ 5 s |\n", sd.MsChuDau.N, msHoacGach(sd.MsChuDau.P50), msHoacGach(sd.MsChuDau.P95))
		w("| trọn lượt (lượt tới model) | %d | %s | %s | plan p95 ≤ 8 s |\n", sd.MsTronLuot.N, msHoacGach(sd.MsTronLuot.P50), msHoacGach(sd.MsTronLuot.P95))
		w("| một lời gọi model | %d | %s | %s | — |\n\n", sd.MsGoiModel.N, msHoacGach(sd.MsGoiModel.P50), msHoacGach(sd.MsGoiModel.P95))
		if sd.MsChuDau.N == 0 {
			w("Chữ đầu: n = 0 vì engine S1 không stream (lát 11); con số này chưa đo được, không phải bằng 0.\n\n")
		}
	}
}

// bangDiemCuoi is what every scoreboard ends with: tokens, cost, what is
// not measured.
func bangDiemCuoi(b *strings.Builder, m Manifest) {
	w := func(f string, a ...any) { fmt.Fprintf(b, f, a...) }
	sd := m.SoDo
	w("## Token và chi phí\n\n")
	w("Token (từ UsageMetadata của từng lời gọi đã ghi): vào %d, cache %d, ra %d (trong đó nghĩ %d); lời gọi không có số token: %d.\n\n",
		sd.Token.Vao, sd.Token.Cache, sd.Token.Ra, sd.Token.Nghi, sd.GoiKhongToken)
	if sd.ChiPhi.CoGia {
		w("Chi phí (tính bằng phân số chính xác, làm tròn một lần khi in): tổng %s, mỗi lượt %s", sd.ChiPhi.Tong, sd.ChiPhi.MoiLuot)
		if sd.ChiPhi.TongDong != "" {
			w("; ≈ %s, mỗi lượt %s", sd.ChiPhi.TongDong, sd.ChiPhi.MoiLuotDong)
		}
		w(". Giá lấy từ gia-model.json, do người xác nhận.\n\n")
	} else {
		w("Chi phí: **%s** (%s). Điền giá đã xác nhận vào `services/core/internal/aieval/testdata/gia-model.json`.\n\n", ChuaCoGia, strings.Join(sd.ChiPhi.ThieuGia, ", "))
	}

	w("## Chưa đo\n\n")
	for _, c := range m.ChuaDo {
		w("- %s\n", c)
	}
	w("\n## Không chứng minh\n\nCorpus là dữ liệu tổng hợp do người thiết kế viết: xanh ở đây là «không thấy lỗi trong các mẫu này», không phải «hữu ích với người thật». Ngưỡng M2/M3 chờ Lead ký; bảng này báo số, không tự gác mốc nào.\n")
}

func hoacKhong(s string) string {
	if s == "" {
		return "không có"
	}
	return s
}

func sapKhoa(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Trailer is the commit trailer block of a valid real run (design 06 §3.5),
// once a replay of it reproduced its grades with no call. ok is false when
// either condition fails: then there is no trailer (design 06 §7).
func Trailer(goc, phatLai Manifest) (string, bool) {
	if goc.MoHinh != MoHinhThat || goc.TrangThai != TrangThaiHopLe || phatLai.TrangThai != TrangThaiHopLe ||
		phatLai.SoVoiNguon == nil || !phatLai.SoVoiNguon.Trung || phatLai.Goi.DaDung != 0 || phatLai.SoVoiNguon.RunID != goc.RunID ||
		goc.GitSHA == "" || phatLai.GitSHA != goc.GitSHA || (goc.CayBan != nil && *goc.CayBan) {
		return "", false
	}
	tk, sd := goc.TongKet, goc.SoDo
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	vs := "?"
	if len(goc.ModelVersions) > 0 {
		vs = strings.Join(goc.ModelVersions, ",")
	}
	w("Eval-Run: %s bo=%s@%s lap=%d goi=%d/%d model=%s@%s sha=%s\n", goc.RunID, goc.Bo.Ten, goc.Bo.Sha[:min(12, len(goc.Bo.Sha))], goc.Lap, goc.Goi.DaDung, goc.Goi.TranDuyet, goc.Model, vs, goc.GitSHA)
	w("Eval-Phat-Lai: %s goi=0 cham=%s trung\n", phatLai.RunID, goc.ChamSha[:min(12, len(goc.ChamSha))])
	if goc.Hieu != nil {
		h := *goc.Hieu
		ct := ChiSoTienCua(h)
		w("Eval-Router: dat %d/%d loi-router %d goi-max %d\n", h.Dat, h.SoCa, h.LoiHieu, h.MaxGoi)
		w("Eval-Tien: recall %s tu-choi-nham %s\n", wilsonGon(ct.Recall), wilsonGon(ct.TuChoiSai))
		w("Eval-Di-Ung: thieu %d thua %d an-kieng-sai %d\n", h.DiUngThieu, h.DiUngThua, h.AnKiengSai)
		w("Eval-Chi-Phi: %s token vao=%d cache=%d ra=%d\n", sd.ChiPhi.Tong, sd.Token.Vao, sd.Token.Cache, sd.Token.Ra)
		w("Eval-Chua-Do: một lời gọi router mỗi ca, không phải cả lượt; bộ đã lộ = đo hồi quy, không phải cổng (ADR-0044 §4.1); mốc M chờ Lead ký\n")
		return b.String(), true
	}
	var truot []string
	for _, k := range sapKhoa(tk.TruotTheoKiem) {
		truot = append(truot, fmt.Sprintf("%s:%d", k, tk.TruotTheoKiem[k]))
	}
	if len(truot) == 0 {
		truot = []string{"-"}
	}
	w("Eval-Ket-Qua: dat %d/%d ha-tang %d truot %s\n", tk.Dat, tk.Dat+tk.KhongDat, tk.LoiHaTang, strings.Join(truot, ","))
	at, batBien, n := DemAnToan(tk)
	w("Eval-An-Toan: ma-kiem %d/%d tan-cong-canary %d/%d bia-dia-diem %d/%d bat-bien %d (0/n nghĩa là <%s)\n",
		at[KiemMaKiem], n, at[KiemTanCongCanary], n, at[KiemKhongBiaDiaDiem], n, batBien, canTren3n(n))
	w("Eval-Latency: trang-thai-p95 %s chu-dau-p50 %s chu-dau-p95 %s tron-luot-p95 %s goi-p95 %s\n",
		msGon(sd.MsTrangThaiDau.P95), msGon(sd.MsChuDau.P50), msGon(sd.MsChuDau.P95), msGon(sd.MsTronLuot.P95), soHoacGach(sd.GoiMoiLuot.P95))
	if sd.ChiPhi.CoGia {
		w("Eval-Chi-Phi: %s (%s/lượt)", sd.ChiPhi.Tong, sd.ChiPhi.MoiLuot)
		if sd.ChiPhi.TongDong != "" {
			w(" ≈ %s", sd.ChiPhi.TongDong)
		}
		w(" token vao=%d cache=%d ra=%d\n", sd.Token.Vao, sd.Token.Cache, sd.Token.Ra)
	} else {
		w("Eval-Chi-Phi: %s token vao=%d cache=%d ra=%d\n", ChuaCoGia, sd.Token.Vao, sd.Token.Cache, sd.Token.Ra)
	}
	w("Eval-Chua-Do: vân tay và Eval-Fingerprint (lát 18); mốc M (nguong.json, Lead ký); KTC bootstrap (lát 2/9); judge; chữ đầu (S1 không stream)\n")
	return b.String(), true
}

func wilsonGon(w Wilson) string {
	if w.N == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d=%.3f[%.3f,%.3f]", w.K, w.N, w.P, w.Lo, w.Hi)
}

func msGon(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p) + "ms"
}
