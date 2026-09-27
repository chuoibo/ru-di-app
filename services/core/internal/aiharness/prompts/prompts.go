// Package prompts holds the engine's system instructions as files embedded in
// the binary, and the one way data is laid into a request.
//
// The system instruction is separate from the data (genai's
// system_instruction, set through ADK's InstructionProvider), static per bot
// and stable byte for byte, apart from the leak-canary marker the engine
// fills in once per process. Everything a person, a device or the server
// supplies goes into the user turn inside <du_lieu nguon="..."> blocks, with
// '<' and '>' in the data turned into their fullwidth forms, so data can
// never close its own block or open a tag of ours.
package prompts

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// nepAgent is migrated from services/api/app/api/nep_gemini.py (_PROMPT):
// the same rules, with the data blocks in place of one JSON blob and a plain
// text answer in place of the {"text"} schema.
//
//go:embed nep_agent.txt
var nepAgent string

// congCu is the clause every tool-using step adds to its bot's system
// instruction: tool results and earlier turns are data, evidence goes by
// alias, one repair, drafts need a human tap.
//
//go:embed cong_cu.txt
var congCu string

// CongCu is the tool clause.
func CongCu() string { return strings.TrimSpace(congCu) }

// MaKiemCho is where the canary marker goes.
const MaKiemCho = "{{MA_KIEM}}"

// NepAgent is Nếp's system instruction carrying the canary marker.
func NepAgent(maKiem string) string {
	return strings.Replace(strings.TrimSpace(nepAgent), MaKiemCho, maKiem, 1)
}

// The instruction clauses of the router's two labels that take the turn off
// the tools (hieu.QuyetDinh.KhongCongCu). They are ours, appended to the
// system instruction; the label itself is never stored against the person.
const (
	loiDanNhayCam     = "Tin nhắn này chạm tới chuyện nhạy cảm (tự làm hại mình, bị bạo hành, khủng hoảng). Không gọi công cụ, không gợi ý quán. Trả lời ngắn, ân cần, không phán xét; khuyên người dùng nói ngay với người thân tin cậy hoặc gọi dịch vụ cấp cứu nếu đang nguy hiểm. Không hứa làm gì thay họ."
	loiDanNgoaiPhamVi = "Tin nhắn này nằm ngoài việc của Nếp (tìm chỗ đi chơi, dùng app, lên kèo với nhóm). Không gọi công cụ. Nói ngắn gọn là Nếp không giúp được việc này và gợi ý một việc Nếp làm được."
)

// LoiDanNhan is the clause for the router's label nhan ("" for any other
// label).
func LoiDanNhan(nhan string) string {
	switch nhan {
	case "nhay_cam":
		return loiDanNhayCam
	case "ngoai_pham_vi":
		return loiDanNgoaiPhamVi
	}
	return ""
}

// VersionNep is the first twelve hex digits of the template's sha256: the
// prompt_version every metrics row carries.
func VersionNep() string {
	sum := sha256.Sum256([]byte(nepAgent))
	return hex.EncodeToString(sum[:])[:12]
}

// LoiNhacNep lists the template's clauses of thirty runes or more, which an
// answer must never quote (the output guard's echo check): a leak quotes a
// clause more often than a whole rule. The marker line is left out: the
// marker itself is checked on its own.
func LoiNhacNep() []string {
	var out []string
	for _, line := range strings.Split(nepAgent, "\n") {
		if strings.Contains(line, MaKiemCho) {
			continue
		}
		for _, s := range strings.FieldsFunc(line, func(r rune) bool { return strings.ContainsRune(".,:;()", r) }) {
			s = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), "- "))
			if utf8.RuneCountInString(s) >= 30 {
				out = append(out, s)
			}
		}
	}
	return out
}

// Nguon names a data block. Closed: a block's name is ours, never data.
type Nguon string

const (
	PhieuManHinh Nguon = "phieu_man_hinh"
	MayChu       Nguon = "may_chu"
	CauHoi       Nguon = "cau_hoi"
	// NganHan is the session's earlier turns (short-term memory).
	NganHan Nguon = "ngan_han"
	// DiemDen is the closed list of destinations a router may pick from.
	DiemDen Nguon = "danh_sach_diem_den"
	// ThanhVien is the closed list of group members a router may pick from.
	ThanhVien Nguon = "danh_sach_thanh_vien"
	// ViDu is the worked examples a router is shown (ours, not a person's).
	ViDu Nguon = "vi_du"
	// LoiCauTruc is why the previous structured output was refused.
	LoiCauTruc Nguon = "loi_cau_truc"
	// KetQuaCongCu holds what a tool returned: catalogue, manual, memory
	// and group rows are data the model reads, never instructions.
	KetQuaCongCu Nguon = "ket_qua_cong_cu"
	// LichSu holds the short-term turns of the session (Nếp's panel, the
	// group's reply chain): earlier words are data, never instructions.
	LichSu Nguon = "lich_su"
	// TriNho is the person's own long-term facts (nepnho.DungHoSo), Nếp
	// only and only while the person's memory toggle is on. Added by infra
	// memory-policy.
	TriNho Nguon = "tri_nho"
	// TruyVan holds the router's search texts for the tool part: the only
	// free texts a tool call may carry once a tool has returned data (the
	// taint invariant, tools.BoiCanh.kiemTaint).
	TruyVan Nguon = "truy_van"
)

var fullwidth = strings.NewReplacer("<", "＜", ">", "＞")

// BocDuLieu lays body into a block named n.
func BocDuLieu(n Nguon, body string) string {
	return `<du_lieu nguon="` + string(n) + `">` + "\n" + fullwidth.Replace(body) + "\n</du_lieu>"
}

// DauDanhDau is the datamarking character (spotlighting, Hines et al. 2024):
// inside a marked block every run of spaces between words is this character,
// so text that came from outside reads, token by token, as marked data and
// can never pass for a line of the system instruction. It is U+02C6, a
// modifier letter nobody types in Vietnamese; any copy of it in the data is
// removed before marking, so data cannot forge or hide the mark.
const DauDanhDau = "ˆ"

// DanhDau datamarks body: the marker removed wherever it stands, then the
// words of each line joined by it. Lines stay lines, so a list reads as one.
func DanhDau(body string) string {
	body = strings.ReplaceAll(body, DauDanhDau, "")
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), DauDanhDau)
	}
	return strings.Join(lines, "\n")
}

// BocDuLieuDanhDau lays body into a block named n, datamarked: the way every
// untrusted text a router reads is laid into its request.
func BocDuLieuDanhDau(n Nguon, body string) string {
	return BocDuLieu(n, DanhDau(body))
}
