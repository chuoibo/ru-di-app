package cau

import (
	"regexp"
	"strings"
	"testing"
)

// The same voice rules the app's cau-chu-goi-ai.test.mjs holds its tables to,
// here so a sentence written in Go is red before it reaches the app.
func TestMoiMaCoMotCauGiongNguoi(t *testing.T) {
	seen := map[Ma]bool{}
	byText := map[string]Ma{}
	for _, m := range Tat() {
		if seen[m] {
			t.Errorf("mã %s lặp", m)
		}
		seen[m] = true
		if !m.Valid() {
			t.Errorf("%s không Valid", m)
		}
		c := Cau(m)
		if len([]rune(strings.TrimSpace(c))) <= 20 {
			t.Errorf("%s: câu quá ngắn: %q", m, c)
		}
		if other, ok := byText[c]; ok {
			t.Errorf("%s và %s dùng chung một câu", m, other)
		}
		byText[c] = m
		for _, bad := range []*regexp.Regexp{regexp.MustCompile(`[a-z]+_[a-z_]+`), regexp.MustCompile(`(?i)lỗi`), regexp.MustCompile(`\b[45]\d\d\b`), regexp.MustCompile(`(?i)http`), regexp.MustCompile(`[—–]`)} {
			if bad.MatchString(c) {
				t.Errorf("%s: câu %q vi phạm %s", m, c, bad)
			}
		}
	}
	if len(seen) < 6 {
		t.Fatalf("chỉ %d mã", len(seen))
	}
	if Ma("khong_co").Valid() || Cau("khong_co") != "" {
		t.Fatal("mã lạ được nhận")
	}
	for _, s := range []TrangThai{DangDoc, DangNghi} {
		if !s.Valid() || CauTrangThai(s) == "" {
			t.Errorf("trạng thái %s thiếu câu", s)
		}
	}
}
