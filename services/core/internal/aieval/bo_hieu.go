package aieval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
)

// The router's eval sets (design 06 M3 «định tuyến», research
// intent-routing §6): one message per case, what the router must label, and
// nothing else. Two kinds live under testdata/hieu:
//
//   - T1 (t1-hieu.json): every case carries the scripted router output
//     (Ra), so a run proves the router's pipeline -- the request it sends,
//     the strict read, the repair, the policy -- and nothing about the
//     model. The model is llm.Stub.
//   - T3 sets converted from the spent sealed corpora of the word-list era
//     (tien_v2/v3, di_ung_v2/v3): no scripted output; they measure a REAL
//     router, run by a caller that builds the model (this package never
//     builds a client). Spent means their words were read while rules were
//     written against them: a regression measure, never a release gate --
//     the gate is a fresh sealed half (ADR-0044 §4.1).
//
// The scorers compare closed labels only. They read no message.

// BoHieu is one router eval set.
type BoHieu struct {
	Bo       string    `json:"bo"`
	PhienBan int       `json:"phien_ban"`
	MoTa     string    `json:"mo_ta"`
	Nguon    *NguonBo  `json:"nguon,omitempty"`
	Luc      time.Time `json:"luc"`
	// BaoTrum maps a family id to the ids it covers (the corpus's own
	// convention): a required member counts as read when its family is.
	BaoTrum map[string][]string `json:"bao_trum,omitempty"`
	Ca      []CaHieu            `json:"ca"`
}

// NguonBo pins the corpus a converted set came from.
type NguonBo struct {
	Tep    string `json:"tep"`
	Sha256 string `json:"sha256"`
}

// CaHieu is one router case.
type CaHieu struct {
	ID     string  `json:"id"`
	Bot    obs.Bot `json:"bot"`
	Cau    string  `json:"cau"`
	Nhom   string  `json:"nhom"`
	GhiChu string  `json:"ghi_chu,omitempty"`
	// Doi is a couple's turn (hieu.Vao.Doi; group bot only): the router
	// reads the couple's instruction, with the group's schema and policy.
	Doi bool `json:"doi,omitempty"`
	// Ra is the scripted router output(s), in call order: T1 only.
	Ra     []json.RawMessage `json:"ra,omitempty"`
	KyVong KyVongHieu        `json:"ky_vong"`
}

// KyVongHieu is what a case expects. A nil or empty field is not checked.
type KyVongHieu struct {
	// Tien is the set of acceptable money classes.
	Tien []string `json:"tien,omitempty"`
	// YDinh is the exact intent set (order ignored).
	YDinh     []string `json:"y_dinh,omitempty"`
	Huong     string   `json:"huong,omitempty"`
	NhanGuard string   `json:"nhan_guard,omitempty"`
	// DiUng and AnKieng are the ids that must be read; CoDiUng says the
	// case checks them even when both are empty (nothing may be read).
	CoDiUng bool     `json:"co_di_ung,omitempty"`
	DiUng   []string `json:"di_ung,omitempty"`
	AnKieng []string `json:"an_kieng,omitempty"`
	// DocThuaChapNhan are extra ids a safe reading may add.
	DocThuaChapNhan []string `json:"doc_thua_chap_nhan,omitempty"`
	// DocThuaAnToan: the corpus marks over-reading this case as safe.
	DocThuaAnToan bool `json:"doc_thua_an_toan,omitempty"`
	// QuyetDinh is the expected policy (T1).
	QuyetDinh *hieu.QuyetDinh `json:"quyet_dinh,omitempty"`
	// LoiHieu: the router must fail with the rephrase fallback (T1).
	LoiHieu bool `json:"loi_hieu,omitempty"`
	// SoGoi is the exact number of model calls (T1).
	SoGoi int `json:"so_goi,omitempty"`
}

var dangIDHieu = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// DocBoHieu reads a set strictly and checks every expectation against the
// router's closed vocabularies.
func DocBoHieu(raw []byte) (BoHieu, error) {
	var b BoHieu
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return BoHieu{}, fmt.Errorf("bộ hiểu: %w", err)
	}
	if b.Bo == "" || b.Luc.IsZero() || len(b.Ca) == 0 {
		return BoHieu{}, errors.New("bộ hiểu: thiếu bo, luc hay ca")
	}
	seen := map[string]bool{}
	for _, c := range b.Ca {
		if !dangIDHieu.MatchString(c.ID) || seen[c.ID] {
			return BoHieu{}, fmt.Errorf("bộ hiểu %s: id %q sai dạng hoặc trùng", b.Bo, c.ID)
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Cau) == "" || c.Nhom == "" {
			return BoHieu{}, fmt.Errorf("ca %s: thiếu cau hay nhom", c.ID)
		}
		yd, ok := hieu.YDinhCua(c.Bot)
		if !ok {
			return BoHieu{}, fmt.Errorf("ca %s: bot %q", c.ID, c.Bot)
		}
		if c.Doi && c.Bot != obs.BotNhom {
			return BoHieu{}, fmt.Errorf("ca %s: cặp đôi chỉ ở bot nhom", c.ID)
		}
		k := c.KyVong
		for _, x := range k.Tien {
			if _, err := hieu.Tiens.Parse(x); err != nil {
				return BoHieu{}, fmt.Errorf("ca %s: %w", c.ID, err)
			}
		}
		if _, err := yd.ParseAll(k.YDinh); err != nil {
			return BoHieu{}, fmt.Errorf("ca %s: %w", c.ID, err)
		}
		if k.Huong != "" {
			if _, err := hieu.Huongs.Parse(k.Huong); err != nil {
				return BoHieu{}, fmt.Errorf("ca %s: %w", c.ID, err)
			}
		}
		if k.NhanGuard != "" {
			if _, err := hieu.NhanGuards.Parse(k.NhanGuard); err != nil {
				return BoHieu{}, fmt.Errorf("ca %s: %w", c.ID, err)
			}
		}
		if _, err := hieu.DiUngs.ParseAll(k.DiUng); err != nil {
			return BoHieu{}, fmt.Errorf("ca %s: %w", c.ID, err)
		}
		if _, err := hieu.AnKiengs.ParseAll(k.AnKieng); err != nil {
			return BoHieu{}, fmt.Errorf("ca %s: %w", c.ID, err)
		}
		for _, x := range k.DocThuaChapNhan {
			if !hieu.DiUngs.Co(x) && !hieu.AnKiengs.Co(x) {
				return BoHieu{}, fmt.Errorf("ca %s: doc_thua_chap_nhan %q ngoài danh mục", c.ID, x)
			}
		}
		for _, r := range c.Ra {
			if !json.Valid(r) {
				return BoHieu{}, fmt.Errorf("ca %s: ra không phải JSON", c.ID)
			}
		}
	}
	return b, nil
}

// TruotHieu is one failed check of a case.
type TruotHieu struct {
	Kiem    string `json:"kiem"`
	ChiTiet string `json:"chi_tiet"`
}

func coTrong(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func tapBang(a []string, b []string) bool {
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, ",") == strings.Join(y, ",")
}

// docDu reports the required ids pred leaves unread and the ids it adds
// that neither the case accepts nor a family rule explains.
func docDu(req, pred, chapNhan []string, baoTrum map[string][]string) (thieu, thua []string) {
	ho := map[string]string{}
	for dau, ms := range baoTrum {
		for _, m := range ms {
			ho[m] = dau
		}
	}
	for _, r := range req {
		if !coTrong(pred, r) && !(ho[r] != "" && coTrong(pred, ho[r])) {
			thieu = append(thieu, r)
		}
	}
	for _, p := range pred {
		if coTrong(req, p) || coTrong(chapNhan, p) {
			continue
		}
		// A family head read for a required member is the safe side.
		giai := false
		for _, m := range baoTrum[p] {
			if coTrong(req, m) {
				giai = true
			}
		}
		if !giai {
			thua = append(thua, p)
		}
	}
	return thieu, thua
}

// ChamHieu scores one router result against a case: closed labels only.
func ChamHieu(k KyVongHieu, bot obs.Bot, kq hieu.KetQua, baoTrum map[string][]string) []TruotHieu {
	var out []TruotHieu
	them := func(kiem, f string, a ...any) { out = append(out, TruotHieu{kiem, fmt.Sprintf(f, a...)}) }
	if len(k.Tien) > 0 && !coTrong(k.Tien, string(kq.Tien)) {
		them("tien", "%s, cần một trong %v", kq.Tien, k.Tien)
	}
	if len(k.YDinh) > 0 {
		var got []string
		for _, y := range kq.YDinh {
			got = append(got, string(y))
		}
		if !tapBang(got, k.YDinh) {
			them("y_dinh", "%v, cần %v", got, k.YDinh)
		}
	}
	if k.Huong != "" && string(kq.Huong) != k.Huong {
		them("huong", "%s, cần %s", kq.Huong, k.Huong)
	}
	if k.NhanGuard != "" && string(kq.NhanGuard) != k.NhanGuard {
		them("nhan_guard", "%s, cần %s", kq.NhanGuard, k.NhanGuard)
	}
	if k.CoDiUng || len(k.DiUng) > 0 || len(k.AnKieng) > 0 {
		if thieu, thua := docDu(k.DiUng, kq.Slots.DiUng, k.DocThuaChapNhan, baoTrum); len(thieu) > 0 || len(thua) > 0 {
			if len(thieu) > 0 {
				them("di_ung_thieu", "thiếu %v (đọc %v)", thieu, kq.Slots.DiUng)
			}
			if len(thua) > 0 {
				them("di_ung_thua", "thừa %v", thua)
			}
		}
		if thieu, thua := docDu(k.AnKieng, kq.Slots.AnKieng, k.DocThuaChapNhan, nil); len(thieu) > 0 || len(thua) > 0 {
			if len(thieu) > 0 {
				them("an_kieng_thieu", "thiếu %v (đọc %v)", thieu, kq.Slots.AnKieng)
			}
			if len(thua) > 0 {
				them("an_kieng_thua", "thừa %v", thua)
			}
		}
	}
	if k.QuyetDinh != nil {
		if q := hieu.QuyetDinhCho(bot, kq); q != *k.QuyetDinh {
			them("quyet_dinh", "%+v, cần %+v", q, *k.QuyetDinh)
		}
	}
	return out
}

// KetQuaHieu is one case's run.
type KetQuaHieu struct {
	ID    string      `json:"id"`
	Nhom  string      `json:"nhom"`
	Dat   bool        `json:"dat"`
	SoGoi int         `json:"so_goi"`
	Loi   string      `json:"loi,omitempty"`
	Truot []TruotHieu `json:"truot,omitempty"`
}

// MaTran counts a binary label: positive is the refused class.
type MaTran struct {
	TP int `json:"tp"`
	FP int `json:"fp"`
	FN int `json:"fn"`
	TN int `json:"tn"`
}

// TongKetHieu is a set's result: counts only, no message.
type TongKetHieu struct {
	Bo       string `json:"bo"`
	PhienBan string `json:"phien_ban_loi_nhac"`
	SoCa     int    `json:"so_ca"`
	Dat      int    `json:"dat"`
	// LoiHieu counts runs where the router gave no usable result.
	LoiHieu int `json:"loi_hieu"`
	// Tien is money_action-or-split (refused by Nếp) against the label.
	Tien MaTran `json:"tien"`
	// DiUngThieu counts cases with a required allergen unread (the unsafe
	// error); DiUngThua cases with one read that is not there.
	DiUngThieu int            `json:"di_ung_thieu"`
	DiUngThua  int            `json:"di_ung_thua"`
	AnKiengSai int            `json:"an_kieng_sai"`
	TruotTheo  map[string]int `json:"truot_theo_kiem"`
	TongGoi    int            `json:"tong_goi"`
	MaxGoi     int            `json:"max_goi"`
	CaVuotTran int            `json:"ca_vuot_tran"`
}

// ChayBoHieu runs every case of b through h, each with its own call counter
// over the model moHinh(c) gives it (a scripted stub for T1, the caller's
// real model for T3), and scores it. It never builds a model.
func ChayBoHieu(ctx context.Context, h hieu.Hieu, b BoHieu, moHinh func(CaHieu) model.LLM, moiCa func(KetQuaHieu)) TongKetHieu {
	tk := TongKetHieu{Bo: b.Bo, SoCa: len(b.Ca), TruotTheo: map[string]int{}}
	for _, c := range b.Ca {
		tk.PhienBan = hieu.PhienBan(c.Bot)
		dem := llm.NewDem(moHinh(c), llm.MaxModelCallsPerTurn, nil).WithWait(func(int) time.Duration { return 0 })
		kq, err := h.Hieu(ctx, VaoHieu(c, b.Luc), dem)
		r := KetQuaHieu{ID: c.ID, Nhom: c.Nhom, SoGoi: dem.SoGoi()}
		tk.TongGoi += r.SoGoi
		tk.MaxGoi = max(tk.MaxGoi, r.SoGoi)
		if r.SoGoi > llm.MaxModelCallsPerTurn {
			tk.CaVuotTran++
		}
		switch {
		case err != nil && c.KyVong.LoiHieu:
			if _, ok := hieu.MaLoi(err); !ok {
				r.Truot = append(r.Truot, TruotHieu{"loi_hieu", err.Error()})
			}
		case err != nil:
			tk.LoiHieu++
			r.Loi = err.Error()
			r.Truot = append(r.Truot, TruotHieu{"loi", err.Error()})
		case c.KyVong.LoiHieu:
			r.Truot = append(r.Truot, TruotHieu{"loi_hieu", "router trả kết quả, cần thất bại có kiểu"})
		default:
			r.Truot = ChamHieu(c.KyVong, c.Bot, kq, b.BaoTrum)
			if len(c.KyVong.Tien) > 0 && c.Bot == obs.BotNep {
				nhan := !coTrong(c.KyVong.Tien, string(hieu.TienNone))
				doan := kq.Tien != hieu.TienNone
				switch {
				case nhan && doan:
					tk.Tien.TP++
				case !nhan && doan:
					tk.Tien.FP++
				case nhan && !doan:
					tk.Tien.FN++
				default:
					tk.Tien.TN++
				}
			}
		}
		if c.KyVong.SoGoi > 0 && r.SoGoi != c.KyVong.SoGoi {
			r.Truot = append(r.Truot, TruotHieu{"so_goi", fmt.Sprintf("%d, cần %d", r.SoGoi, c.KyVong.SoGoi)})
		}
		for _, t := range r.Truot {
			tk.TruotTheo[t.Kiem]++
			switch t.Kiem {
			case "di_ung_thieu":
				tk.DiUngThieu++
			case "di_ung_thua":
				tk.DiUngThua++
			case "an_kieng_thieu", "an_kieng_thua":
				tk.AnKiengSai++
			}
		}
		r.Dat = len(r.Truot) == 0
		if r.Dat {
			tk.Dat++
		}
		if moiCa != nil {
			moiCa(r)
		}
	}
	return tk
}

// VaoHieu is the router input of a case: its bot, its class (a couple's
// turn is Doi) and its message, at the set's instant.
func VaoHieu(c CaHieu, luc time.Time) hieu.Vao {
	return hieu.Vao{Bot: c.Bot, Doi: c.Doi, Cau: c.Cau, Luc: luc}
}

// StubHieu is the scripted model of a T1 case: its Ra, in order. A case
// with no Ra (a T3 case) gets an empty script, so running it on the stub is
// red rather than silently answered.
func StubHieu(c CaHieu) *llm.Stub {
	var kich []llm.Buoc
	for _, r := range c.Ra {
		kich = append(kich, llm.Buoc{Text: string(r)})
	}
	return llm.NewStub(kich...)
}

// KiemYeuCauHieu holds one recorded router request to the T1 invariants:
// the «now» line and the calendar are there, the message is in its
// datamarked block and last, and the structured-output schema is set.
func KiemYeuCauHieu(canon []byte, c CaHieu) []TruotHieu {
	var out []TruotHieu
	var r struct {
		Contents []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		Config struct {
			ResponseSchema    json.RawMessage `json:"responseSchema"`
			ResponseMIMEType  string          `json:"responseMimeType"`
			SystemInstruction struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"systemInstruction"`
		} `json:"config"`
	}
	if err := json.Unmarshal(canon, &r); err != nil || len(r.Contents) == 0 || len(r.Contents[0].Parts) == 0 {
		return []TruotHieu{{"yeu_cau", "không đọc được yêu cầu"}}
	}
	body := r.Contents[0].Parts[0].Text
	if !strings.Contains(body, "Bây giờ: ") || !strings.Contains(body, "(hôm nay)") {
		out = append(out, TruotHieu{"dong_bay_gio", "yêu cầu thiếu dòng «bây giờ» hoặc lịch"})
	}
	if !strings.HasSuffix(body, prompts.BocDuLieuDanhDau(prompts.CauHoi, c.Cau)) {
		out = append(out, TruotHieu{"cau_hoi_danh_dau", "câu hỏi không nằm cuối trong khối đã đánh dấu"})
	}
	if r.Config.ResponseMIMEType != "application/json" || len(r.Config.ResponseSchema) == 0 {
		out = append(out, TruotHieu{"luoc_do", "thiếu structured output"})
	}
	// The router reads its class's instruction: a couple's turn the
	// couple's, any other its bot's (two classes, 2026-09-28).
	var he strings.Builder
	for _, p := range r.Config.SystemInstruction.Parts {
		he.WriteString(p.Text)
	}
	if want, err := hieu.LoiNhacCua(VaoHieu(c, time.Time{})); err != nil || he.String() != want {
		out = append(out, TruotHieu{"loi_nhac", "router không đọc lời nhắc của lớp phòng"})
	}
	return out
}

// Converters from the spent corpora of the word-list era. Label names map
// to the catalogue's ids (domain/tuvung); a label with no id is an error, so
// a corpus that grows a label cannot be converted silently short.
var nhanID = map[string]string{
	"tôm": "tom", "cua": "cua", "cá": "ca", "mực": "muc", "động vật có vỏ": "oc_so", "hải sản": "hai_san",
	"đậu phộng": "dau_phong", "các loại hạt": "hat_cay", "sữa": "sua", "trứng": "trung", "mè": "me",
	"gluten": "lua_mi", "đậu nành": "dau_nanh",
	"chay": "chay", "thuần chay": "thuan_chay", "halal": "halal",
}

func ids(labels []string) ([]string, []string, error) {
	var du, ak []string
	for _, l := range labels {
		id, ok := nhanID[l]
		if !ok {
			return nil, nil, fmt.Errorf("nhãn %q không có id", l)
		}
		if hieu.AnKiengs.Co(id) {
			ak = append(ak, id)
		} else {
			du = append(du, id)
		}
	}
	return du, ak, nil
}

func shaCua(raw []byte) string {
	s := sha256.Sum256(raw)
	return hex.EncodeToString(s[:])
}

// LucBoHieu is the instant every converted set routes at.
var LucBoHieu = time.Date(2026, 9, 25, 14, 5, 0, 0, time.FixedZone("ICT", 7*3600))

// ChuyenTien converts a money corpus (tien / khong_tien) into a Nếp router
// set: a «tien» sentence must be refused (money_action, or split_draft,
// which Nếp refuses too); a «khong_tien» one must be none.
func ChuyenTien(raw []byte, ten, tep string, phienBan int) (BoHieu, error) {
	var tho struct {
		GhiChu string `json:"ghi_chu"`
		Tien   []struct {
			Cau  string `json:"cau"`
			Nhom string `json:"nhom"`
		} `json:"tien"`
		KhongTien []struct {
			Cau  string `json:"cau"`
			Nhom string `json:"nhom"`
			LyDo string `json:"ly_do"`
		} `json:"khong_tien"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tho); err != nil {
		return BoHieu{}, err
	}
	b := BoHieu{Bo: ten, PhienBan: phienBan, Luc: LucBoHieu, Nguon: &NguonBo{Tep: tep, Sha256: shaCua(raw)},
		MoTa: "Router Nếp: nhãn tien chuyển từ corpus tiền đã lộ (T3, đo hồi quy, không phải cổng). «tien» phải bị từ chối (money_action hoặc split_draft, Nếp từ chối cả hai); «khong_tien» phải là none."}
	for i, c := range tho.Tien {
		b.Ca = append(b.Ca, CaHieu{ID: fmt.Sprintf("%s-t-%03d", ten, i+1), Bot: obs.BotNep, Cau: c.Cau, Nhom: "tien/" + c.Nhom,
			KyVong: KyVongHieu{Tien: []string{string(hieu.MoneyAction), string(hieu.SplitDraft)}}})
	}
	for i, c := range tho.KhongTien {
		b.Ca = append(b.Ca, CaHieu{ID: fmt.Sprintf("%s-k-%03d", ten, i+1), Bot: obs.BotNep, Cau: c.Cau, Nhom: "khong_tien/" + c.Nhom, GhiChu: c.LyDo,
			KyVong: KyVongHieu{Tien: []string{string(hieu.TienNone)}}})
	}
	return b, nil
}

// ChuyenDiUng converts an allergy corpus's asker half (nguoi_hoi) into a
// Nếp router set of allergen and diet slot labels. The place half (quan) is
// ingestion's and is not converted.
func ChuyenDiUng(raw []byte, ten, tep string, phienBan int) (BoHieu, error) {
	var tho struct {
		QuyUoc   map[string]json.RawMessage `json:"quy_uoc"`
		NguoiHoi []struct {
			ID      string   `json:"id"`
			Cau     string   `json:"cau"`
			DiUng   []string `json:"di_ung"`
			AnKieng []string `json:"an_kieng"`
			Loai    string   `json:"loai"`
			AnToan  *bool    `json:"an_toan_neu_doc_thua"`
			DocThua []string `json:"doc_thua_chap_nhan"`
			GhiChu  string   `json:"ghi_chu"`
		} `json:"nguoi_hoi"`
		Quan json.RawMessage `json:"quan"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tho); err != nil {
		return BoHieu{}, err
	}
	b := BoHieu{Bo: ten, PhienBan: phienBan, Luc: LucBoHieu, Nguon: &NguonBo{Tep: tep, Sha256: shaCua(raw)},
		MoTa:    "Router Nếp: slot di_ung/an_kieng chuyển từ nửa người hỏi của corpus dị ứng đã lộ (T3, đo hồi quy, không phải cổng). Thiếu một dị nguyên bắt buộc là lỗi không an toàn; đọc thừa được đếm riêng. Nửa quán thuộc tầng nạp dữ liệu, không chuyển.",
		BaoTrum: map[string][]string{"hai_san": {"tom", "cua", "ca", "muc", "oc_so"}}}
	for _, c := range tho.NguoiHoi {
		du, ak, err := ids(append(append([]string(nil), c.DiUng...), c.AnKieng...))
		if err != nil {
			return BoHieu{}, fmt.Errorf("%s: %w", c.ID, err)
		}
		thuaDu, thuaAk, err := ids(c.DocThua)
		if err != nil {
			return BoHieu{}, fmt.Errorf("%s: %w", c.ID, err)
		}
		k := KyVongHieu{CoDiUng: true, DiUng: du, AnKieng: ak, DocThuaChapNhan: append(thuaDu, thuaAk...)}
		if c.AnToan != nil {
			k.DocThuaAnToan = *c.AnToan
		}
		b.Ca = append(b.Ca, CaHieu{ID: ten + "-" + strings.ToLower(c.ID), Bot: obs.BotNep, Cau: c.Cau, Nhom: c.Loai, GhiChu: c.GhiChu, KyVong: k})
	}
	return b, nil
}

// MaHoaBoHieu is a set's canonical file form.
func MaHoaBoHieu(b BoHieu) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
