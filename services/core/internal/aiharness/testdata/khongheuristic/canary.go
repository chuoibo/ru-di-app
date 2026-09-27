// Package khongheuristic is the canary of the engine's no-heuristic gate
// (aiharness/khong_heuristic_test.go): each red function reads the turn's
// text for meaning the way a review found could slip through; HopLe uses
// it only the ways the rule allows. It is test data, never built into the
// engine (a testdata directory is outside ./...).
package khongheuristic

import (
	"context"
	"regexp"
	"strings"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/preprocess"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tactu"
	"mobile/services/core/internal/aiharness/traloi"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
)

// OChay injects a diet slot from a keyword (review mutant M2b).
func OChay(t aiharness.Turn) []string {
	hoi := preprocess.LamSach(t.LoiNho)
	if strings.Contains(hoi.Chu, "ăn chay") {
		return []string{"chay"}
	}
	return nil
}

// ChonToolTheoTu chooses a tool from a word of the model's query, read
// through a closure's result (review mutant M5).
func ChonToolTheoTu(kq hieu.KetQua) string {
	truyVan := func(n truyhoi.Nguon) (string, bool) {
		for _, t := range kq.TruyVan {
			if t.Nguon == n {
				return t.Cau, true
			}
		}
		return "", false
	}
	q, _ := truyVan(truyhoi.Manual)
	if strings.Contains(q, "màn này") {
		return "explain_screen"
	}
	return ""
}

var tien = regexp.MustCompile(`chuyển (tiền|khoản)`)

// RegexpQuaBien reads a copy of the message with a regular expression.
func RegexpQuaBien(v hieu.Vao) bool {
	c := strings.TrimSpace(v.Cau)
	sao := c
	return tien.MatchString(sao)
}

// SwitchTheoTu switches on the words of a history turn.
func SwitchTheoTu(l aiharness.LuotNep) int {
	for _, w := range strings.Fields(l.Chu) {
		switch w {
		case "quán":
			return 1
		}
	}
	return 0
}

// DocTuVung hands the message to a vocabulary reader.
func DocTuVung(t aiharness.Turn) []string {
	return tuvung.DiUngNguoiHoi(t.LoiNho)
}

var bang = map[string]string{"cf": "cà phê"}

// TraBangTheoTu rewrites the query by a Go table.
func TraBangTheoTu(y truyhoi.YeuCau) string {
	return bang[y.Cau]
}

// CumTuTraLoi brings back a phrase rule on the model's answer: a claimed
// transfer guessed from a word (the verifier's judgement, never Go's).
func CumTuTraLoi(ra tactu.Ra) bool {
	for _, c := range traloi.TachCauVanXuoi(ra.Text) {
		if strings.Contains(strings.ToLower(c), "đã chuyển") {
			return true
		}
	}
	return false
}

// CumTuHoiLai reads the model's question back for a word.
func CumTuHoiLai(kq hieu.KetQua) bool {
	return strings.HasPrefix(kq.CauHoiLai, "Mình đã")
}

// CumTuVongLap reads the loop's answer, as it returns, for a word.
func CumTuVongLap(ctx context.Context, cfg agent.CauHinh) bool {
	text, _ := agent.Chay(ctx, nil, cfg, nil, "", nil)
	return regexp.MustCompile(`đặt bàn`).MatchString(text)
}

// PhieuTheoTu refuses a turn from a word of the slip's trip title, text
// another member wrote (re-review mutant H2).
func PhieuTheoTu(t aiharness.Turn) bool {
	return t.PhieuNep != nil && strings.Contains(t.PhieuNep.TieuDe, "chia tiền")
}

// GoiYTheoTu reads the slip's suggested questions for a word.
func GoiYTheoTu(p aiharness.PhieuNep) bool {
	for _, g := range p.GoiY {
		if strings.HasPrefix(g, "Chia") {
			return true
		}
	}
	return false
}

// SoLieuTheoTu reads a string count of the slip for a word.
func SoLieuTheoTu(p aiharness.PhieuNep) bool {
	s, _ := p.SoLieu["trangThai"].(string)
	return s == "huỷ"
}

// BangChungTheoTu filters evidence by a word of a place's name before the
// verifier (re-review mutant H3).
func BangChungTheoTu(bcs []truyhoi.BangChung) []truyhoi.BangChung {
	var out []truyhoi.BangChung
	for _, b := range bcs {
		if v, ok := b.Truong["ten"]; ok && strings.Contains(v, "quảng cáo") {
			continue
		}
		out = append(out, b)
	}
	return out
}

// HoSoTheoTu drops the personalization when a recalled fact names a word
// (re-review mutant H4).
func HoSoTheoTu(ctx context.Context, h aiharness.HoSo) bool {
	ds, _ := h.HoSoNep(ctx, "", "")
	for _, s := range ds {
		if strings.Contains(s.NoiDung, "dị ứng") {
			return true
		}
	}
	return false
}

// SuThatTheoTu reads a remembered fact for a word.
func SuThatTheoTu(s trinho.SuThat) bool {
	return regexp.MustCompile(`dị ứng`).MatchString(s.NoiDung)
}

// HopLe uses the text only the ways the rule allows.
func HopLe(t aiharness.Turn) string {
	hoi := preprocess.LamSach(t.LoiNho)
	if hoi.Chu == "" || len(hoi.Chu) > 2000 {
		return ""
	}
	return prompts.BocDuLieuDanhDau(prompts.CauHoi, strings.TrimSpace(hoi.Chu))
}
