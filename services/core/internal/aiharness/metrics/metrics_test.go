package metrics

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

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
	// Version 2 adds columns, one ADD COLUMN per line.
	for _, line := range strings.Split(schemaV2SQL, "\n") {
		line = strings.TrimSpace(line)
		def, ok := strings.CutPrefix(line, "ADD COLUMN ")
		if !ok {
			continue
		}
		def = strings.TrimSuffix(strings.TrimSuffix(def, ";"), ",")
		out[strings.Fields(def)[0]] = def
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
	closed := regexp.MustCompile(`^\w+ (text|char\(\d+\))( NOT NULL)?( DEFAULT '[a-z_]*')? CHECK \(\w+ (IN \('[^']*'(,'[^']*')*\)|~ '\^\[0-9a-f\]\{\d+\}\$')\)$`)
	// A text array only as a subset of a closed list.
	closedArray := regexp.MustCompile(`^\w+ text\[\] NOT NULL DEFAULT '\{\}' CHECK \(\w+ <@ ARRAY\['[a-z_]+'(,'[a-z_]+')*\]::text\[\]\)$`)
	plain := regexp.MustCompile(`^\w+ (uuid|smallint|integer|boolean|timestamptz)\b`)
	for name, def := range cols {
		switch {
		case plain.MatchString(def):
		case closed.MatchString(def):
		case closedArray.MatchString(def):
		default:
			t.Errorf("cột %s có thể chứa chữ tự do: %s", name, def)
		}
	}
	sql := regexp.MustCompile(`(?m)^\s*--.*$`).ReplaceAllString(schemaSQL+schemaV2SQL+schemaV3SQL, "")
	if regexp.MustCompile(`(?i)\b(jsonb?|bytea|varchar|character varying)\b`).MatchString(sql) {
		t.Error("schema có kiểu chứa được chữ tự do")
	}
	// Canary: the check is red on a column that could carry words.
	for _, bad := range []string{"note text", "detail text NOT NULL", "tool_args jsonb", "prompt varchar(200)",
		"notes text[] NOT NULL DEFAULT '{}'", "tags text[] NOT NULL DEFAULT '{}' CHECK (cardinality(tags) < 3)"} {
		if plain.MatchString(bad) || closed.MatchString(bad) || closedArray.MatchString(bad) {
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

// Version 3 is the last word on nhan_guard: its CHECK admits exactly what a
// record may hold (obs.NhanGuards and ""), and never nhay_cam.
func TestNhanNhayCamKhongLuu(t *testing.T) {
	m := regexp.MustCompile(`ADD CONSTRAINT ai_turn_metrics_nhan_guard_check CHECK \(nhan_guard IN \(([^)]*)\)\)`).FindStringSubmatch(schemaV3SQL)
	if m == nil {
		t.Fatal("version 3 does not replace the nhan_guard CHECK")
	}
	want := []string{"''"}
	for _, v := range obs.NhanGuards {
		want = append(want, "'"+string(v)+"'")
	}
	if m[1] != strings.Join(want, ",") || strings.Contains(m[1], "nhay_cam") {
		t.Fatalf("CHECK %s, want %s", m[1], strings.Join(want, ","))
	}
	if obs.NhanGuard("nhay_cam").Valid() {
		t.Fatal("a record may hold nhay_cam")
	}
	if v := cacPhienBan(); len(v) != PhienBan || v[len(v)-1] != schemaV5SQL {
		t.Fatal("PhienBan is not the last version")
	}
	// Version 5's path list is exactly obs.Duongs.
	want5 := []string{"''"}
	for _, d := range obs.Duongs {
		want5 = append(want5, "'"+string(d)+"'")
	}
	if !strings.Contains(schemaV5SQL, "CHECK (duong IN ("+strings.Join(want5, ",")+"))") {
		t.Fatalf("version 5's duong CHECK is not obs.Duongs %v", want5)
	}
	// Version 4 keeps the table from holding router columns without a label
	// (the row a nhay_cam turn wrote under version 3), and rewrites them.
	for _, s := range []string{"CHECK (nhan_guard <> '' OR (huong = '' AND tien = '' AND y_dinh = '' AND so_y_dinh = 0))",
		"SET nhan_guard = 'sach'", "WHERE nhan_guard = '' AND (huong <> '' OR tien <> '' OR y_dinh <> '' OR so_y_dinh <> 0)"} {
		if !strings.Contains(schemaV4SQL, s) {
			t.Fatalf("version 4 lacks %q", s)
		}
	}
}

// execDem counts the statements it is given and runs none.
type execDem struct{ n int }

func (e *execDem) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	e.n++
	return pgconn.CommandTag{}, nil
}

// A turn stopped from outside has no row: nothing reaches the database, so a
// lost lease never reads as a failed turn. A failed turn beside it does.
func TestHuyKhongCoHang(t *testing.T) {
	rec := obs.TurnRecord{
		InvocationID: "0b7d3a1c-5f2e-4c1a-9e3b-2d6f8a4c1e90", LanThu: 1, Bot: obs.BotNep, Lenh: obs.LenhHoi,
		Guard: obs.GuardProceed, OutGuard: obs.OutNone, KetThuc: obs.KetThucHuy, LoiMoHinh: obs.LoiKhong,
		PromptVersion: "0123456789ab", KetKiem: obs.KiemKhongChay,
	}
	var e execDem
	if err := Ghi(context.Background(), &e, rec); err != nil || e.n != 0 {
		t.Fatalf("lượt huỷ: err=%v, %d câu SQL", err, e.n)
	}
	rec.KetThuc, rec.Code = obs.KetThucThatBai, obs.Code(cau.ProviderUnavailable)
	if err := Ghi(context.Background(), &e, rec); err != nil || e.n != 1 {
		t.Fatalf("lượt thất bại: err=%v, %d câu SQL", err, e.n)
	}
	// The table's ket_thuc CHECK holds exactly the endings that get a row.
	got := regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(columnLines(t)["ket_thuc"], -1)
	var inSQL []string
	for _, c := range got {
		inSQL = append(inSQL, c[1])
	}
	sort.Strings(inSQL)
	if strings.Join(inSQL, ",") != string(obs.KetThucThatBai)+","+string(obs.KetThucXong) || !obs.KetThucHuy.Valid() {
		t.Fatalf("CHECK ket_thuc: %v", inSQL)
	}
}
