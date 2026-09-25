package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"regexp"
	"testing"
)

func mau() TurnRecord {
	return TurnRecord{
		InvocationID: "0b7d3a1c-5f2e-4c1a-9e3b-2d6f8a4c1e90", LanThu: 1, Bot: BotNep, Lenh: LenhHoi,
		Guard: GuardProceed, OutGuard: OutNone, KetThuc: KetThucXong, LoiMoHinh: LoiKhong,
		PromptVersion: "0123456789ab", Buoc: 1, SoGoiMoHinh: 1, TokensIn: 120, TokensOut: 40, MsTong: 5,
	}
}

// The reflection gate: a free-string field cannot be added to the record
// without going red here. Strings must be named types with a closed Valid;
// everything else must be an int or a bool. No slice, map, struct, pointer or
// interface, which could smuggle text past the string rule.
func TestBanGhiKhongCoTruongChuTuDo(t *testing.T) {
	typ := reflect.TypeOf(TurnRecord{})
	vtype := reflect.TypeOf((*validator)(nil)).Elem()
	colShape := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	seen := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		col := f.Tag.Get("col")
		if !colShape.MatchString(col) || seen[col] {
			t.Errorf("%s: tag col %q thiếu, sai dạng hoặc lặp", f.Name, col)
		}
		seen[col] = true
		switch f.Type.Kind() {
		case reflect.String:
			if f.Type == reflect.TypeOf("") {
				t.Errorf("%s là string trần: chữ tự do", f.Name)
			}
			if !f.Type.Implements(vtype) {
				t.Errorf("%s (%s) không có Valid(): không đóng", f.Name, f.Type)
			}
		case reflect.Int, reflect.Bool:
		default:
			t.Errorf("%s có kiểu %s: chỉ enum, số nguyên và bool", f.Name, f.Type)
		}
	}
	if len(seen) < 20 {
		t.Fatalf("chỉ %d trường; phép phản chiếu nhìn sai chỗ", len(seen))
	}
}

// Every string field actually refuses text. Each field is set, in turn, to a
// question a person might type; Valid must be red for every one.
func TestMoiTruongChuoiTuChoiChu(t *testing.T) {
	cau := "Tối nay đi đâu? Gọi cho Minh nhé"
	if err := mau().Valid(); err != nil {
		t.Fatalf("bản ghi mẫu hỏng: %v", err)
	}
	v := reflect.ValueOf(mau())
	n := 0
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() != reflect.String {
			continue
		}
		r := mau()
		reflect.ValueOf(&r).Elem().Field(i).SetString(cau)
		if r.Valid() == nil {
			t.Errorf("%s nhận câu chữ tự do", v.Type().Field(i).Name)
		}
		n++
	}
	if n < 9 {
		t.Fatalf("chỉ thử %d trường chuỗi", n)
	}
	neg := mau()
	neg.TokensIn = -1
	if neg.Valid() == nil {
		t.Fatal("số âm được nhận")
	}
}

// One JSON line per turn, keys exactly the columns, and an invalid record is
// logged with no fields at all.
func TestMotDongLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	Log(context.Background(), logger, mau())
	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("không phải một dòng JSON: %s", buf.String())
	}
	if line["msg"] != "ai_turn" {
		t.Fatalf("msg %v", line["msg"])
	}
	for _, c := range Columns() {
		if _, ok := line[c]; !ok {
			t.Errorf("thiếu %s", c)
		}
	}
	if got, want := len(line), len(Columns())+3; got != want { // time, level, msg
		t.Errorf("%d khoá, muốn %d: %v", got, want, line)
	}
	buf.Reset()
	bad := mau()
	bad.Bot = "Tối nay đi đâu"
	Log(context.Background(), logger, bad)
	if bytes.Contains(buf.Bytes(), []byte("Tối nay")) || !bytes.Contains(buf.Bytes(), []byte("ai_turn_invalid")) {
		t.Fatalf("bản ghi hỏng lọt chữ: %s", buf.String())
	}
}
