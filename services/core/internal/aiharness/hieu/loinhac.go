package hieu

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"

	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/domain/tuvung"
)

// The router's system instruction: one common template and one file per
// bot with its money definitions, intents and sources. Assembled once at
// init, static per bot and stable byte for byte, so the provider's implicit
// cache sees the same prefix every turn; everything that changes per turn
// (the clock, the lists, the message) is in the user turn.
var (
	//go:embed loi_nhac/chung.txt
	chungTxt string
	//go:embed loi_nhac/nep.txt
	nepTxt string
	//go:embed loi_nhac/nhom.txt
	nhomTxt string
)

// The per-bot sections, in file order.
var phanBot = []string{"BOT", "TIEN", "Y_DINH", "NGUON"}

// slotNhom is the member slot's rule, the group's only.
const slotNhom = `- nguoi_tham_gia: the members the plan or the split is about, as ids from danh_sach_thanh_vien ("cả nhóm trừ Minh" = every id except Minh's). Leave it out when the whole group is meant or nobody is named.
`

// catPhan reads the @@NAME sections of a bot file.
func catPhan(txt string) (map[string]string, error) {
	out := map[string]string{}
	var ten string
	var b strings.Builder
	flush := func() {
		if ten != "" {
			out[ten] = strings.TrimRight(b.String(), "\n")
		}
		b.Reset()
	}
	for _, line := range strings.Split(txt, "\n") {
		if name, ok := strings.CutPrefix(line, "@@"); ok {
			flush()
			ten = strings.TrimSpace(name)
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	flush()
	for _, p := range phanBot {
		if strings.TrimSpace(out[p]) == "" {
			return nil, fmt.Errorf("hieu: instruction section %s missing", p)
		}
	}
	if len(out) != len(phanBot) {
		return nil, fmt.Errorf("hieu: instruction has %d sections, want %d", len(out), len(phanBot))
	}
	return out, nil
}

// tuVungDong lists a vocabulary as "- id: label" lines, in declaration
// order: the model reads what each id means; no reader of tuvung runs.
func tuVungDong(v *tuvung.TuVung) string {
	var lines []string
	for _, m := range v.Muc() {
		lines = append(lines, "- "+m.ID+": "+m.Nhan)
	}
	return strings.Join(lines, "\n")
}

func ghepLoiNhac(bot obs.Bot) (string, error) {
	var txt, nhom string
	switch bot {
	case obs.BotNep:
		txt = nepTxt
	case obs.BotNhom:
		txt, nhom = nhomTxt, slotNhom
	default:
		return "", fmt.Errorf("%w: %q", ErrBot, bot)
	}
	p, err := catPhan(txt)
	if err != nil {
		return "", err
	}
	r := strings.NewReplacer(
		"{{BOT}}", p["BOT"],
		"{{TIEN}}", p["TIEN"],
		"{{Y_DINH}}", p["Y_DINH"],
		"{{NGUON}}", p["NGUON"],
		"{{NHOM_SLOT}}", nhom,
		"{{DI_UNG}}", tuVungDong(tuvung.DiUng),
		"{{AN_KIENG}}", tuVungDong(tuvung.AnKieng),
		"{{LOAI_CHO}}", tuVungDong(tuvung.LoaiCho),
		"{{KHI_CHAT}}", tuVungDong(tuvung.KhiChat),
	)
	out := strings.TrimSpace(r.Replace(chungTxt))
	if strings.Contains(out, "{{") {
		return "", fmt.Errorf("hieu: instruction for %s has an unfilled placeholder", bot)
	}
	return out, nil
}

var loiNhac = func() map[obs.Bot]string {
	out := map[obs.Bot]string{}
	for _, bot := range []obs.Bot{obs.BotNep, obs.BotNhom} {
		s, err := ghepLoiNhac(bot)
		if err != nil {
			panic(err)
		}
		out[bot] = s
	}
	return out
}()

// LoiNhac is bot's router system instruction.
func LoiNhac(bot obs.Bot) (string, error) {
	s, ok := loiNhac[bot]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrBot, bot)
	}
	return s, nil
}

// PhienBan is the first twelve hex digits of the sha256 of bot's router
// instruction: the router's prompt version, for the eval and the logs.
func PhienBan(bot obs.Bot) string {
	sum := sha256.Sum256([]byte(loiNhac[bot]))
	return hex.EncodeToString(sum[:])[:12]
}
