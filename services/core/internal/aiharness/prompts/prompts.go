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

// MaKiemCho is where the canary marker goes.
const MaKiemCho = "{{MA_KIEM}}"

// NepAgent is Nếp's system instruction carrying the canary marker.
func NepAgent(maKiem string) string {
	return strings.Replace(strings.TrimSpace(nepAgent), MaKiemCho, maKiem, 1)
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
)

var fullwidth = strings.NewReplacer("<", "＜", ">", "＞")

// BocDuLieu lays body into a block named n.
func BocDuLieu(n Nguon, body string) string {
	return `<du_lieu nguon="` + string(n) + `">` + "\n" + fullwidth.Replace(body) + "\n</du_lieu>"
}
