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

// HopLe uses the text only the ways the rule allows.
func HopLe(t aiharness.Turn) string {
	hoi := preprocess.LamSach(t.LoiNho)
	if hoi.Chu == "" || len(hoi.Chu) > 2000 {
		return ""
	}
	return prompts.BocDuLieuDanhDau(prompts.CauHoi, strings.TrimSpace(hoi.Chu))
}
