package metrics

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/obs"
)

// columnLines are the column definitions of CREATE TABLE ai_turn_metrics.
func columnLines(t *testing.T) map[string]string {
	t.Helper()
	body := regexp.MustCompile(`(?s)CREATE TABLE ai_turn_metrics \((.*)\);\s*CREATE INDEX`).FindStringSubmatch(schemaSQL)
	if body == nil {
		t.Fatal("không đọc được CREATE TABLE ai_turn_metrics")
	}
	out := map[string]string{}
	for _, line := range strings.Split(body[1], "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
		if line == "" || strings.HasPrefix(line, "PRIMARY KEY") {
			continue
		}
		name := strings.Fields(line)[0]
		out[name] = line
	}
	return out
}

// The table holds no free text, read from the SQL itself: every column is a
// uuid, a number, a boolean or a timestamp, or text held by a CHECK to a
// closed list or a fixed shape. There is no json, jsonb, bytea or varchar.
func TestBangKhongChuaChuTuDo(t *testing.T) {
	cols := columnLines(t)
	if len(cols) < 25 {
		t.Fatalf("chỉ %d cột", len(cols))
	}
	closed := regexp.MustCompile(`^\w+ (text|char\(\d+\))( NOT NULL)? CHECK \(\w+ (IN \('[^']*'(,'[^']*')*\)|~ '\^\[0-9a-f\]\{\d+\}\$')\)$`)
	plain := regexp.MustCompile(`^\w+ (uuid|smallint|integer|boolean|timestamptz)\b`)
	for name, def := range cols {
		switch {
		case plain.MatchString(def):
		case closed.MatchString(def):
		default:
			t.Errorf("cột %s có thể chứa chữ tự do: %s", name, def)
		}
	}
	sql := regexp.MustCompile(`(?m)^\s*--.*$`).ReplaceAllString(schemaSQL, "")
	if regexp.MustCompile(`(?i)\b(jsonb?|bytea|varchar|character varying)\b`).MatchString(sql) {
		t.Error("schema có kiểu chứa được chữ tự do")
	}
	// Canary: the check is red on a column that could carry words.
	for _, bad := range []string{"note text", "detail text NOT NULL", "tool_args jsonb", "prompt varchar(200)"} {
		if plain.MatchString(bad) || closed.MatchString(bad) {
			t.Errorf("cổng không bắt được %q", bad)
		}
	}
}

// The row, the log line and the INSERT name the same columns in the same
// order, and the code list in the CHECK is exactly cau's.
func TestCotKhopBanGhi(t *testing.T) {
	m := regexp.MustCompile(`INSERT INTO ai_turn_metrics\(([^)]*)\)`).FindStringSubmatch(insert)
	if m == nil || m[1] != strings.Join(obs.Columns(), ",") {
		t.Fatalf("INSERT lệch obs.Columns():\n%s\n%s", m[1], strings.Join(obs.Columns(), ","))
	}
	cols := columnLines(t)
	want := append(obs.Columns(), "created_at")
	var got []string
	for c := range cols {
		got = append(got, c)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("schema lệch bản ghi:\n%v\n%v", got, want)
	}
	codes := regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(cols["code"], -1)
	var inSQL, inGo []string
	for _, c := range codes {
		inSQL = append(inSQL, c[1])
	}
	for _, c := range cau.Tat() {
		inGo = append(inGo, string(c))
	}
	sort.Strings(inSQL)
	sort.Strings(inGo)
	if strings.Join(inSQL, ",") != strings.Join(inGo, ",") {
		t.Fatalf("CHECK code lệch cau.Tat(): %v vs %v (mã mới cần migration version mới)", inSQL, inGo)
	}
}
