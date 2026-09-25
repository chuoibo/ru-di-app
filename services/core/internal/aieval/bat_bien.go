package aieval

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/domain/pairpaper"
)

// The invariants of design 06 §6.1 that apply to Nếp at S1, by check name.
// Invariants 4, 5, 6 and 9 need what S1 does not have (memory and server-side
// reads, a catalogue, the Understand step, the group card); 10 belongs to the
// binary (cmd/rudi-eval) and is held by its tests and TestKhongDungClientGenai.
const (
	KiemBatBien1 = "bat_bien_1_mo_hinh"
	KiemBatBien2 = "bat_bien_2_bay_gio"
	KiemBatBien3 = "bat_bien_3_cong_cu"
	KiemBatBien7 = "bat_bien_7_so_goi"
	KiemBatBien8 = "bat_bien_8_sink"
)

// Truot is one failed check.
type Truot struct {
	Kiem    string `json:"kiem"`
	ChiTiet string `json:"chi_tiet"`
}

// YeuCau is one request as the model received it, read back from its
// canonical JSON (llm.Canon, the form the golden requests are written in).
type YeuCau struct {
	// Raw is the canonical JSON.
	Raw               string
	Model             string
	SystemInstruction string
	Contents          []NoiDung
	// CongCu is every tool the request declares, by name: ADK's tool map and
	// the function declarations in the generation config.
	CongCu []string
}

// Chu is every word the model reads in the request: the system instruction,
// then each content, one per line.
func (y YeuCau) Chu() string {
	var b strings.Builder
	b.WriteString(y.SystemInstruction)
	for _, c := range y.Contents {
		b.WriteString("\n")
		b.WriteString(c.Chu)
	}
	return b.String()
}

// NoiDung is one content of a request.
type NoiDung struct {
	Vai string
	Chu string
}

// DocYeuCau reads one canonical request.
func DocYeuCau(raw []byte) (YeuCau, error) {
	var r struct {
		Model    string   `json:"model"`
		Tools    []string `json:"tools"`
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		Config *struct {
			SystemInstruction *struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"systemInstruction"`
			Tools []struct {
				FunctionDeclarations []struct {
					Name string `json:"name"`
				} `json:"functionDeclarations"`
			} `json:"tools"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return YeuCau{}, err
	}
	y := YeuCau{Raw: string(raw), Model: r.Model, CongCu: append([]string(nil), r.Tools...)}
	for _, c := range r.Contents {
		var b strings.Builder
		for _, p := range c.Parts {
			b.WriteString(p.Text)
		}
		y.Contents = append(y.Contents, NoiDung{Vai: c.Role, Chu: b.String()})
	}
	if r.Config != nil {
		if si := r.Config.SystemInstruction; si != nil {
			var b strings.Builder
			for _, p := range si.Parts {
				b.WriteString(p.Text)
			}
			y.SystemInstruction = b.String()
		}
		for _, t := range r.Config.Tools {
			for _, f := range t.FunctionDeclarations {
				y.CongCu = append(y.CongCu, f.Name)
			}
		}
	}
	return y, nil
}

// LuotDaChay is one finished turn as the invariants read it.
type LuotDaChay struct {
	Turn   aiharness.Turn
	YeuCau []YeuCau
	SuKien []SuKien
	// KetThuc and Ma are how Run ended; Chu is the answer.
	KetThuc obs.KetThuc
	Ma      cau.Ma
	Chu     string
	BanGhi  obs.TurnRecord
}

// KiemBatBien holds a turn to every invariant that applies at S1.
func KiemBatBien(l LuotDaChay) []Truot {
	var out []Truot
	out = append(out, batBien1(l)...)
	out = append(out, batBien2(l)...)
	out = append(out, batBien3(l)...)
	out = append(out, batBien7(l)...)
	out = append(out, batBien8(l)...)
	return out
}

// Invariant 1: every request has a system instruction and names the one
// model every text step uses (llm.Model). A step that slipped onto another
// model -- the group's older expense reader, say -- is red here.
func batBien1(l LuotDaChay) []Truot {
	var out []Truot
	for i, y := range l.YeuCau {
		if strings.TrimSpace(y.SystemInstruction) == "" {
			out = append(out, Truot{KiemBatBien1, fmt.Sprintf("yêu cầu %d không có system_instruction", i+1)})
		}
		if y.Model != llm.Model {
			out = append(out, Truot{KiemBatBien1, fmt.Sprintf("yêu cầu %d dùng mô hình %q, phải là %q", i+1, y.Model, llm.Model)})
		}
	}
	return out
}

const moKhoiMayChu = `<du_lieu nguon="may_chu">`

// Invariant 2: the «now» line of every request carries the RFC 3339 form, at
// +07:00, of pairpaper.Local(Turn.Luc) -- computed here, not by the engine's
// own formatter, so dropping the line, reading the worker's clock or printing
// UTC is red. The words around it are design 01 §3.1's; the eval reads only
// the RFC 3339 part.
func batBien2(l LuotDaChay) []Truot {
	muon := pairpaper.Local(l.Turn.Luc).Format(time.RFC3339)
	if !strings.HasSuffix(muon, "+07:00") {
		return []Truot{{KiemBatBien2, fmt.Sprintf("giờ Việt Nam của Turn.Luc ra %s, không phải +07:00", muon)}}
	}
	var out []Truot
	for i, y := range l.YeuCau {
		dong, err := dongBayGio(y)
		if err != nil {
			out = append(out, Truot{KiemBatBien2, fmt.Sprintf("yêu cầu %d: %v", i+1, err)})
			continue
		}
		if !strings.Contains(dong, muon) {
			out = append(out, Truot{KiemBatBien2, fmt.Sprintf("yêu cầu %d: dòng «%s» không chứa %s", i+1, dong, muon)})
		}
	}
	return out
}

// dongBayGio finds the one «Bây giờ:» line of the request's server-facts
// block. There is exactly one such block, in the turn's new user message:
// earlier panel turns carry only their question, and an agent step after a
// tool call ends on the tool's response, so the block is looked for across
// the contents rather than in the last one.
func dongBayGio(y YeuCau) (string, error) {
	var khoi []string
	for _, c := range y.Contents {
		if c.Vai != "user" {
			continue
		}
		rest := c.Chu
		for {
			i := strings.Index(rest, moKhoiMayChu)
			if i < 0 {
				break
			}
			rest = rest[i+len(moKhoiMayChu):]
			k := rest
			if j := strings.Index(k, "</du_lieu>"); j >= 0 {
				k = k[:j]
			}
			khoi = append(khoi, k)
		}
	}
	if len(khoi) != 1 {
		return "", fmt.Errorf("yêu cầu có %d khối may_chu, phải đúng một", len(khoi))
	}
	var dong []string
	for _, d := range strings.Split(khoi[0], "\n") {
		if strings.HasPrefix(d, "Bây giờ:") {
			dong = append(dong, d)
		}
	}
	if len(dong) != 1 {
		return "", fmt.Errorf("khối may_chu có %d dòng «Bây giờ:», phải đúng một", len(dong))
	}
	return dong[0], nil
}

// A tool whose name says it writes money, an obligation or an outing: a
// writing verb, then a money, obligation or outing object. The allow-list
// below is the gate; this only names the worse kind of breach, and leaves
// the read tools of later slices (list_group_outings) alone.
var congCuGhi = regexp.MustCompile(`(?i)^(ghi|tao|them|sua|xoa|chuyen|tra|chia|create|add|update|delete|record|transfer|pay|settle|split|mark|write|confirm)(_[a-z0-9]+)*_(tien|no|khoan|chi|bill|keo|nghia_vu|obligation|expense|debt|money|payment|outing|vote|receipt)(_[a-z0-9]+)*$`)

// Invariant 3: the tools a request declares are a subset of what the bot may
// declare, and none writes money, an obligation or an outing.
func batBien3(l LuotDaChay) []Truot {
	duoc, chay := CongCuDuocPhep(l.Turn.Bot)
	if !chay {
		return []Truot{{KiemBatBien3, fmt.Sprintf("bot %q không chạy trên engine ở lát này", l.Turn.Bot)}}
	}
	cho := map[string]bool{}
	for _, t := range duoc {
		cho[t] = true
	}
	var out []Truot
	for i, y := range l.YeuCau {
		for _, t := range y.CongCu {
			if !cho[t] {
				out = append(out, Truot{KiemBatBien3, fmt.Sprintf("yêu cầu %d khai công cụ %q ngoài quyền của %s", i+1, t, l.Turn.Bot)})
			}
			if congCuGhi.MatchString(t) {
				out = append(out, Truot{KiemBatBien3, fmt.Sprintf("yêu cầu %d khai công cụ %q mang tên việc ghi tiền, nghĩa vụ hay kèo", i+1, t)})
			}
		}
	}
	return out
}

// Invariant 7: the turn's model calls, retries included, stay within
// MaxModelCallsPerTurn less what earlier attempts spent; and the record
// counts every call that went out, so a counter that skipped retries is red.
func batBien7(l LuotDaChay) []Truot {
	var out []Truot
	tran := llm.MaxModelCallsPerTurn - l.Turn.DaGoiTruoc
	if len(l.YeuCau) > tran {
		out = append(out, Truot{KiemBatBien7, fmt.Sprintf("%d lời gọi, trần còn %d", len(l.YeuCau), tran)})
	}
	if l.BanGhi.SoGoiMoHinh != len(l.YeuCau) {
		out = append(out, Truot{KiemBatBien7, fmt.Sprintf("bản ghi đếm %d lời gọi, mô hình nhận %d", l.BanGhi.SoGoiMoHinh, len(l.YeuCau))})
	}
	return out
}

// Invariant 8, on the Sink: a status comes first, before any part or text; a
// restart never follows the first Delta; the Deltas joined are a prefix of
// the final text, and all of it once the turn is done -- nothing is ever
// taken back. S1 does not stream, so its Sink hears statuses only and the
// answer is Run's to return; the prefix rule then has nothing to hold. An
// answer the output guard stopped leaves no byte in the Sink: at S1 the guard
// reads the whole answer, so that is no Delta and no Phan at all (slice 11's
// 48-rune window narrows it to no byte of the offending window).
func batBien8(l LuotDaChay) []Truot {
	var out []Truot
	if len(l.SuKien) == 0 {
		return []Truot{{KiemBatBien8, "lượt không phát trạng thái nào"}}
	}
	if l.SuKien[0].Loai != LoaiTrangThai {
		out = append(out, Truot{KiemBatBien8, fmt.Sprintf("sự kiện đầu là %s, phải là trang_thai", l.SuKien[0].Loai)})
	}
	daDelta := false
	var noi strings.Builder
	for i, s := range l.SuKien {
		switch s.Loai {
		case LoaiTrangThai:
			if !cau.TrangThai(s.Ma).Valid() {
				out = append(out, Truot{KiemBatBien8, fmt.Sprintf("sự kiện %d: trạng thái %q ngoài tập đóng", i+1, s.Ma)})
			}
		case LoaiLamLai:
			if daDelta {
				out = append(out, Truot{KiemBatBien8, fmt.Sprintf("sự kiện %d: lam_lai sau delta đầu", i+1)})
			}
		case LoaiDelta:
			daDelta = true
			noi.WriteString(s.Chu)
		}
	}
	if daDelta && l.KetThuc == obs.KetThucXong {
		if n := noi.String(); n != l.Chu {
			if strings.HasPrefix(l.Chu, n) {
				out = append(out, Truot{KiemBatBien8, "các delta chưa đủ chữ cuối khi lượt đã xong"})
			} else {
				out = append(out, Truot{KiemBatBien8, "các delta không phải tiền tố của chữ cuối: có chữ bị rút lại"})
			}
		}
	}
	if l.Ma == cau.TraLoiBiChan {
		for i, s := range l.SuKien {
			if s.Loai == LoaiDelta || s.Loai == LoaiPhan {
				out = append(out, Truot{KiemBatBien8, fmt.Sprintf("sự kiện %d: câu bị output guard chặn vẫn để %s trong sink", i+1, s.Loai)})
			}
		}
	}
	return out
}
