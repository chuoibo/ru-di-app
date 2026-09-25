package aieval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
)

// The two sentinel cases every corpus carries (design 06 §12, row T1): the
// canary, whose correct script is built to fail exactly the checks it names,
// and the identity case, which must pass. A corpus without either cannot tell
// a working eval from one that passes everything or nothing.
const (
	CaCanary   = "00-canary-phai-do"
	CaDongNhat = "00-dong-nhat"
)

// Values of Ca.CanhGac.
const (
	CanhGacCanary   = "canary"
	CanhGacDongNhat = "dong_nhat"
)

// Bo is one corpus file.
type Bo struct {
	Bo       string `json:"bo"`
	PhienBan int    `json:"phien_ban"`
	MoTa     string `json:"mo_ta"`
	Ca       []Ca   `json:"ca"`
}

// Ca is one case (design 06 §3.1, the fields Nếp at S1 uses).
type Ca struct {
	CaID  string   `json:"case_id"`
	Nhom  []string `json:"nhom"`
	BeMat string   `json:"be_mat"`
	Lenh  string   `json:"lenh"`
	// LucHoi is the instant the question was stored, RFC 3339 with +07:00:
	// the only source of «now» the engine gets (Turn.Luc).
	LucHoi string `json:"luc_hoi"`
	// CanhGac marks the two sentinel cases.
	CanhGac string `json:"canh_gac,omitempty"`
	// PhaiDoO is, for the canary, the exact set of checks its correct script
	// must fail.
	PhaiDoO []string  `json:"phai_do_o,omitempty"`
	DauVao  DauVao    `json:"dau_vao"`
	KyVong  KyVong    `json:"ky_vong"`
	KichBan KichBanCa `json:"kich_ban"`
}

// DauVao is what the device sent, as the worker stores it (chatassist
// goiNep plus the prompt), and the calls earlier attempts already spent.
type DauVao struct {
	LoiNho     string `json:"loi_nho"`
	Phieu      *Phieu `json:"phieu,omitempty"`
	Luot       []Luot `json:"luot,omitempty"`
	DaGoiTruoc int    `json:"da_goi_truoc,omitempty"`
}

// Phieu mirrors the device's context slip (nep/phieu.ts), keys and all.
type Phieu struct {
	Man    string         `json:"man"`
	TieuDe string         `json:"tieuDe,omitempty"`
	Nhip   *Nhip          `json:"nhip,omitempty"`
	LoaiSo string         `json:"loaiSo,omitempty"`
	SoLieu map[string]any `json:"soLieu,omitempty"`
	GoiY   []string       `json:"goiY,omitempty"`
}

// Nhip mirrors keo/nhip-keo.ts.
type Nhip struct {
	Kieu      string `json:"kieu"`
	ConNgay   *int   `json:"conNgay,omitempty"`
	TruocNgay *int   `json:"truocNgay,omitempty"`
}

// Luot is one earlier turn of the open panel session.
type Luot struct {
	Vai string `json:"vai"`
	Chu string `json:"chu"`
}

// KyVong is what the correct script must produce.
type KyVong struct {
	KetThuc    string `json:"ket_thuc"`
	Ma         string `json:"ma,omitempty"`
	Guard      string `json:"guard"`
	OutGuard   string `json:"out_guard"`
	SoGoiModel int    `json:"so_goi_model"`
	// SuKien is every Sink event, in order, as SuKien.Nhan renders it.
	SuKien  []string `json:"su_kien"`
	LuotBo  int      `json:"luot_bo"`
	PhieuBo int      `json:"phieu_bo"`
	MayCham MayCham  `json:"may_cham"`
	TanCong TanCong  `json:"tan_cong"`
}

// MayCham is the rule checks on the request and the answer.
type MayCham struct {
	// Chu, when set, is the exact answer.
	Chu *string `json:"chu,omitempty"`
	// YeuCauChua must be in every request; YeuCauKhongChua in none.
	YeuCauChua      []string `json:"yeu_cau_chua,omitempty"`
	YeuCauKhongChua []string `json:"yeu_cau_khong_chua,omitempty"`
}

// TanCong is the attack a case plants.
type TanCong struct {
	// Canary strings must reach no request, no answer, no Sink event, no log
	// line and no record.
	Canary []string `json:"canary,omitempty"`
}

// KichBanCa names the case's scripts: the correct one, and wrong ones that
// must each fail the check they name (the scorer's own canaries).
type KichBanCa struct {
	Dung string  `json:"dung"`
	Sai  []SaiCa `json:"sai,omitempty"`
}

// SaiCa is one wrong script and the check it must fail.
type SaiCa struct {
	KichBan   string `json:"kich_ban"`
	PhaiTruot string `json:"phai_truot"`
}

var (
	dangCaID = regexp.MustCompile(`^[0-9]{2}-[a-z0-9]+(?:-[a-z0-9]+)*$`)
	dangNhom = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// giaiMaChat decodes one JSON document refusing unknown fields and trailing
// data: a misspelt key is a red corpus, not a silently ignored expectation.
func giaiMaChat(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("dữ liệu thừa sau tài liệu JSON")
	}
	return nil
}

// DocBo reads and checks a corpus file against the scripts it names, and
// returns the sha256 of the bytes it read.
func DocBo(path string, kbs map[string]KichBan) (Bo, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Bo{}, "", err
	}
	var b Bo
	if err := giaiMaChat(raw, &b); err != nil {
		return Bo{}, "", fmt.Errorf("%s: %w", path, err)
	}
	if err := b.Kiem(kbs); err != nil {
		return Bo{}, "", fmt.Errorf("%s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return b, hex.EncodeToString(sum[:]), nil
}

// GiaiMaCa decodes one case as strictly as a corpus decodes it (the `chay`
// op of the binary's line protocol).
func GiaiMaCa(raw []byte, kbs map[string]KichBan) (Ca, error) {
	var c Ca
	if err := giaiMaChat(raw, &c); err != nil {
		return Ca{}, err
	}
	return c, c.Kiem(kbs)
}

// Kiem checks the corpus is consistent on its own and against the engine's
// closed sets and the scripts: T0 for the corpus.
func (b Bo) Kiem(kbs map[string]KichBan) error {
	if b.Bo == "" || b.PhienBan < 1 {
		return errors.New("thiếu tên bộ hoặc phiên bản")
	}
	if len(b.Ca) == 0 {
		return errors.New("bộ không có ca nào")
	}
	seen := map[string]bool{}
	canary, dongNhat := 0, 0
	for _, c := range b.Ca {
		if seen[c.CaID] {
			return fmt.Errorf("case_id %q trùng", c.CaID)
		}
		seen[c.CaID] = true
		if err := c.Kiem(kbs); err != nil {
			return fmt.Errorf("ca %s: %w", c.CaID, err)
		}
		switch c.CanhGac {
		case CanhGacCanary:
			canary++
		case CanhGacDongNhat:
			dongNhat++
		}
	}
	if canary != 1 || !seen[CaCanary] {
		return fmt.Errorf("bộ phải có đúng một ca canary %s (có %d)", CaCanary, canary)
	}
	if dongNhat != 1 || !seen[CaDongNhat] {
		return fmt.Errorf("bộ phải có đúng một ca đồng nhất %s (có %d)", CaDongNhat, dongNhat)
	}
	return nil
}

// Kiem checks one case.
func (c Ca) Kiem(kbs map[string]KichBan) error {
	if !dangCaID.MatchString(c.CaID) {
		return fmt.Errorf("case_id %q sai dạng", c.CaID)
	}
	if len(c.Nhom) == 0 {
		return errors.New("thiếu nhom")
	}
	for _, n := range c.Nhom {
		if !dangNhom.MatchString(n) {
			return fmt.Errorf("nhom %q sai dạng", n)
		}
	}
	// S1 runs Nếp only, and Nếp has one command.
	if c.BeMat != string(obs.BotNep) {
		return fmt.Errorf("be_mat %q: lát này chỉ có nep", c.BeMat)
	}
	if c.Lenh != string(obs.LenhHoi) {
		return fmt.Errorf("lenh %q: Nếp chỉ có hoi", c.Lenh)
	}
	if _, err := c.Luc(); err != nil {
		return err
	}
	switch c.CanhGac {
	case "":
		if len(c.PhaiDoO) > 0 {
			return errors.New("phai_do_o chỉ dành cho ca canary")
		}
	case CanhGacCanary:
		if c.CaID != CaCanary {
			return fmt.Errorf("ca canary phải tên %s", CaCanary)
		}
		if len(c.PhaiDoO) == 0 {
			return errors.New("ca canary không nói phải đỏ ở đâu")
		}
		for _, k := range c.PhaiDoO {
			if !KiemCo(k) {
				return fmt.Errorf("phai_do_o %q không phải phép kiểm nào", k)
			}
		}
	case CanhGacDongNhat:
		if c.CaID != CaDongNhat || len(c.PhaiDoO) > 0 {
			return fmt.Errorf("ca đồng nhất phải tên %s và không có phai_do_o", CaDongNhat)
		}
	default:
		return fmt.Errorf("canh_gac %q lạ", c.CanhGac)
	}
	if c.DauVao.DaGoiTruoc < 0 || c.DauVao.DaGoiTruoc > llm.MaxModelCallsPerTurn {
		return errors.New("da_goi_truoc ngoài trần")
	}
	if err := c.KyVong.kiem(); err != nil {
		return err
	}
	if _, ok := kbs[c.KichBan.Dung]; !ok {
		return fmt.Errorf("không có kịch bản %q", c.KichBan.Dung)
	}
	for _, s := range c.KichBan.Sai {
		if _, ok := kbs[s.KichBan]; !ok {
			return fmt.Errorf("không có kịch bản sai %q", s.KichBan)
		}
		if !KiemCo(s.PhaiTruot) {
			return fmt.Errorf("phai_truot %q không phải phép kiểm nào", s.PhaiTruot)
		}
	}
	return nil
}

// Luc is the case's instant. It must carry Vietnam's offset, so the corpus
// reads as the person's wall clock and the invariant on «now» is not
// comparing an instant with itself in the same zone by accident.
func (c Ca) Luc() (time.Time, error) {
	t, err := time.Parse(time.RFC3339, c.LucHoi)
	if err != nil {
		return time.Time{}, fmt.Errorf("luc_hoi: %w", err)
	}
	if _, off := t.Zone(); off != 7*3600 {
		return time.Time{}, fmt.Errorf("luc_hoi %q không theo giờ +07:00", c.LucHoi)
	}
	return t, nil
}

func (k KyVong) kiem() error {
	switch obs.KetThuc(k.KetThuc) {
	case obs.KetThucXong:
		if k.Ma != "" {
			return errors.New("ket_thuc xong mà có ma")
		}
	case obs.KetThucThatBai:
		if !cau.Ma(k.Ma).Valid() {
			return fmt.Errorf("ma %q không có trong aiharness/cau", k.Ma)
		}
	default:
		return fmt.Errorf("ket_thuc %q lạ", k.KetThuc)
	}
	if !obs.Guard(k.Guard).Valid() || !obs.OutGuard(k.OutGuard).Valid() {
		return errors.New("guard hoặc out_guard ngoài tập đóng")
	}
	if k.SoGoiModel < 0 || k.SoGoiModel > llm.MaxModelCallsPerTurn {
		return errors.New("so_goi_model ngoài trần")
	}
	if len(k.SuKien) == 0 {
		return errors.New("thiếu su_kien: lượt nào cũng phát ít nhất một trạng thái")
	}
	if k.LuotBo < 0 || k.PhieuBo < 0 {
		return errors.New("luot_bo, phieu_bo âm")
	}
	// A request expectation on a turn that makes no call is vacuous.
	if k.SoGoiModel == 0 && (len(k.MayCham.YeuCauChua) > 0 || len(k.MayCham.YeuCauKhongChua) > 0) {
		return errors.New("kỳ vọng trên yêu cầu mà lượt không gọi mô hình")
	}
	return nil
}
