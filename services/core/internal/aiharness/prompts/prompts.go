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

// nhomAgent is the group assistant's system instruction (Rủ Đi AI in the
// thread, design 03): the same rules as Nếp's where they are the same
// (data is never an instruction, nothing invented, no money, no claimed
// action, no contact detail, the canary marker), with the room as the
// audience and the shared messages as the history.
//
//go:embed nhom_agent.txt
var nhomAgent string

// doiAgent is the couple's system instruction (decision 2026-09-28, two
// classes, ADR-0046 §8.4): a chat of two whose two people have both turned
// on «Một đôi». Every rule of the group's, word for word where it can be;
// only the audience changes: two people who are together, addressed as
// «hai bạn», never a room of friends. A chat of two without «Một đôi» is
// friends and keeps nhomAgent.
//
//go:embed doi_agent.txt
var doiAgent string

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

// loiDanThang is the clause every direct answer carries, whatever the
// router's label: how to answer a message that touches something sensitive.
// It is not the router's nhay_cam label turned into words: the label only
// takes the turn off the tools (hieu.QuyetDinh.KhongCongCu), and the direct
// answer's request is the same byte for byte whether the label was nhay_cam
// or sach, so nothing the provider bills or the metrics row counts (the
// prompt's tokens) can tell the label (privacy review 2, metrics v4).
const loiDanThang = "Nếu tin nhắn chạm tới chuyện nhạy cảm (tự làm hại mình, bị bạo hành, khủng hoảng): không gợi ý quán; trả lời ngắn, ân cần, không phán xét; khuyên người dùng nói ngay với người thân tin cậy hoặc gọi dịch vụ cấp cứu nếu đang nguy hiểm; không hứa làm gì thay họ."

// LoiDanThang is the clause of every direct answer.
func LoiDanThang() string { return loiDanThang }

// loiDanNgoaiPhamVi is the instruction clause of the router's ngoai_pham_vi
// label, which takes the turn off the tools. It is ours, appended to the
// system instruction after LoiDanThang; that label is not sensitive and is
// recorded as it is.
const loiDanNgoaiPhamVi = "Tin nhắn này nằm ngoài việc của Nếp (tìm chỗ đi chơi, dùng app, lên kèo với nhóm). Không gọi công cụ. Nói ngắn gọn là Nếp không giúp được việc này và gợi ý một việc Nếp làm được."

// LoiDanNhan is the clause for the router's label nhan: ngoai_pham_vi's,
// and "" for any other label, nhay_cam included (its turn must not be told
// from a clean one by its request).
func LoiDanNhan(nhan string) string {
	if nhan == "ngoai_pham_vi" {
		return loiDanNgoaiPhamVi
	}
	return ""
}

// loiDanNgoaiPhamViNhom is the group's clause for ngoai_pham_vi.
const loiDanNgoaiPhamViNhom = "Tin nhắn này nằm ngoài việc của Rủ Đi AI (tìm chỗ đi chơi, lên kèo, soạn nháp chia bill cho nhóm). Không gọi công cụ. Nói ngắn gọn là Rủ Đi AI không giúp được việc này và gợi ý một việc làm được cho cả nhóm."

// LoiDanNhanNhom is LoiDanNhan for the group assistant.
func LoiDanNhanNhom(nhan string) string {
	if nhan == "ngoai_pham_vi" {
		return loiDanNgoaiPhamViNhom
	}
	return ""
}

// loiDanNgoaiPhamViDoi is the couple's clause for ngoai_pham_vi. A couple
// has every command a room of friends has, the split draft included.
const loiDanNgoaiPhamViDoi = "Tin nhắn này nằm ngoài việc của Rủ Đi AI (tìm chỗ đi chơi, lên kèo, soạn nháp chia bill cho hai bạn). Không gọi công cụ. Nói ngắn gọn là Rủ Đi AI không giúp được việc này và gợi ý một việc hai bạn có thể làm cùng nhau."

// LoiDanNhanDoi is LoiDanNhan for the couple.
func LoiDanNhanDoi(nhan string) string {
	if nhan == "ngoai_pham_vi" {
		return loiDanNgoaiPhamViDoi
	}
	return ""
}

// VersionNep is the first twelve hex digits of the template's sha256: the
// prompt_version every metrics row carries.
func VersionNep() string {
	sum := sha256.Sum256([]byte(nepAgent))
	return hex.EncodeToString(sum[:])[:12]
}

// NhomAgent is the group assistant's system instruction carrying the canary
// marker.
func NhomAgent(maKiem string) string {
	return strings.Replace(strings.TrimSpace(nhomAgent), MaKiemCho, maKiem, 1)
}

// VersionNhom is the first twelve hex digits of the group template's
// sha256: the prompt_version of the group's metrics rows.
func VersionNhom() string {
	sum := sha256.Sum256([]byte(nhomAgent))
	return hex.EncodeToString(sum[:])[:12]
}

// LoiNhacNhom is LoiNhacNep for the group template.
func LoiNhacNhom() []string { return cauDaiCua(nhomAgent) }

// DoiAgent is the couple's system instruction carrying the canary marker.
func DoiAgent(maKiem string) string {
	return strings.Replace(strings.TrimSpace(doiAgent), MaKiemCho, maKiem, 1)
}

// VersionDoi is the first twelve hex digits of the couple template's
// sha256: the prompt_version of a couple's metrics rows (bot doi).
func VersionDoi() string {
	sum := sha256.Sum256([]byte(doiAgent))
	return hex.EncodeToString(sum[:])[:12]
}

// LoiNhacDoi is LoiNhacNep for the couple template.
func LoiNhacDoi() []string { return cauDaiCua(doiAgent) }

// TuNhom are the words that speak to a room of friends. None may stand in
// the couple's instruction or its out-of-scope clause (prompts_test.go and
// the eval's couple invariant): a couple is two people, never «cả nhóm».
// Matched case-insensitively as substrings.
var TuNhom = []string{"cả nhóm", "nhóm", "thành viên", "mọi người", "hội", "group", "member"}

// LoiNhacNep lists the template's clauses of thirty runes or more, which an
// answer must never quote (the output guard's echo check): a leak quotes a
// clause more often than a whole rule. The marker line is left out: the
// marker itself is checked on its own.
func LoiNhacNep() []string { return cauDaiCua(nepAgent) }

func cauDaiCua(mau string) []string {
	var out []string
	for _, line := range strings.Split(mau, "\n") {
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

// BoDanhDau undoes DanhDau on a text a model copied out of a marked block:
// each mark becomes the space it stands for. Structural: our own character,
// no word read.
func BoDanhDau(s string) string {
	return strings.ReplaceAll(s, DauDanhDau, " ")
}

// BocDuLieuDanhDau lays body into a block named n, datamarked: the way every
// untrusted text a router reads is laid into its request.
func BocDuLieuDanhDau(n Nguon, body string) string {
	return BocDuLieu(n, DanhDau(body))
}
