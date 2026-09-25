package huongdan

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// A small manual known to be good: two tab screens and a money screen
// (finance, so the flag agrees with the route). Every refusal below is this
// fixture with one defect, so a checker that refuses everything fails the
// identity case and a checker gone blind fails the rest. The money screen's
// doors are labelled edges of the code in _rut.json (canh): «Mở tiền» from a,
// «Về A» back.
func mauDung() map[string]string {
	return map[string]string{
		"data/_rut.json": `{"routes":[
  {"canh":[{"den":"finance","nhan":"Mở tiền"}],"di_toi":["b","finance"],"man":"a","nhan":["Màn A","Mở tiền","Nút B"],"tep":["app/(tabs)/a.tsx"]},
  {"canh":[],"di_toi":[],"man":"b","nhan":[],"tep":["app/(tabs)/b.tsx"]},
  {"canh":[{"den":"a","nhan":"Về A"}],"di_toi":["a"],"man":"finance","nhan":["Về A"],"tep":["app/finance.tsx"]},
  {"canh":[],"di_toi":["a"],"man":"welcome","nhan":[],"tep":["app/welcome.tsx"]}
]}
`,
		"data/a.md": `---json
{"man":"a","tieu_de":"Màn A","nhanUI":["Nút B","Mở tiền"],"di_toi":[{"nhan":"Nút B","man":"b"},{"nhan":"Mở tiền","man":"finance"}],"tien":false}
---
Tổng quan của màn A.

## Đi sang B

1. Bấm «Nút B».

## Mở màn tiền

1. Bấm «Mở tiền».
2. Xem xong thì quay lại.
`,
		"data/tien.md": `---json
{"man":"finance","tieu_de":"Tiền","nhanUI":["Về A","Mở tiền"],"di_toi":[{"nhan":"Về A","man":"a"}],"tien":true}
---
Màn tiền. Nếp chỉ chỉ đường tới đây.

## Tới màn này và đi tiếp

- Từ màn A: bấm «Mở tiền».
- Xong thì bấm «Về A».
`,
	}
}

func napTu(m map[string]string) (*SoTay, error) {
	fsys := fstest.MapFS{}
	for k, v := range m {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return nap(fsys)
}

func TestNapMauDung(t *testing.T) {
	s, err := napTu(mauDung())
	if err != nil {
		t.Fatalf("identity: the good fixture must load: %v", err)
	}
	a := s.theoMan("a")
	if len(a) != 2 || a[0].ID != "a/di-sang-b" || a[1].ID != "a/mo-man-tien" {
		t.Fatalf("sections of a: %+v", a)
	}
	if got := a[1].Buoc; len(got) != 2 || got[0] != "Bấm «Mở tiền»." {
		t.Fatalf("steps: %q", got)
	}
	if got := a[1].Nhan; len(got) != 1 || got[0] != "Mở tiền" {
		t.Fatalf("labels: %q", got)
	}
	tien := s.theoMan("finance")
	if len(tien) != 1 || !tien[0].Tien || tien[0].ID != "tien/toi-man-nay-va-di-tiep" {
		t.Fatalf("money screen: %+v", tien)
	}
	// b is a tab with no manual: reachable from a by its di_toi label.
	if b, ok := s.duongToi("a", "b"); !ok || len(b) != 1 || b[0].Nhan != "Nút B" {
		t.Fatalf("a -> b: %+v %v", b, ok)
	}
	if len(s.ban) != 12 {
		t.Fatalf("ban %q", s.ban)
	}
}

// A heading is read like a step: a label it quotes is one of the section's
// labels (Doan.Nhan), ahead of the body's, in order of first use, even when
// the body never quotes it.
func TestTieuDeTrichNhanVaoDoanNhan(t *testing.T) {
	m := mauDung()
	m["data/a.md"] = strings.Replace(m["data/a.md"], "## Mở màn tiền", "## Mở màn tiền, không phải «Nút B»", 1)
	s, err := napTu(m)
	if err != nil {
		t.Fatalf("identity: a heading quoting a declared label must load: %v", err)
	}
	a := s.theoMan("a")
	if len(a) != 2 || a[1].ID != "a/mo-man-tien-khong-phai-nut-b" {
		t.Fatalf("sections of a: %+v", a)
	}
	if got := a[1].Nhan; !reflect.DeepEqual(got, []string{"Nút B", "Mở tiền"}) {
		t.Fatalf("labels of %s: %q, want the heading's «Nút B» first", a[1].ID, got)
	}
	// The section above it quotes «Nút B» in its body only, and keeps it.
	if got := a[0].Nhan; !reflect.DeepEqual(got, []string{"Nút B"}) {
		t.Fatalf("labels of %s: %q", a[0].ID, got)
	}
}

func TestNapTuChoiMoiLoi(t *testing.T) {
	type sua func(m map[string]string)
	thay := func(tep, cu, moi string) sua {
		return func(m map[string]string) {
			if !strings.Contains(m[tep], cu) {
				panic("fixture drifted: " + cu)
			}
			m[tep] = strings.Replace(m[tep], cu, moi, 1)
		}
	}
	cases := []struct {
		ten string
		sua sua
		loi string
	}{
		// Malformed front matter.
		{"dòng đầu không phải ---json", thay("data/a.md", "---json\n", "---\n"), "dòng đầu phải là ---json"},
		{"thiếu --- đóng", thay("data/a.md", "\n---\nTổng quan", "\nTổng quan"), "thiếu dòng --- đóng front matter"},
		{"JSON hỏng", thay("data/a.md", `"tien":false}`, `"tien":false`), "front matter không đọc được"},
		{"khoá lạ (tên cũ nut)", thay("data/a.md", `"tien":false}`, `"tien":false,"nut":[]}`), `unknown field "nut"`},
		{"thiếu khoá tien", thay("data/a.md", `,"tien":false}`, `}`), "front matter thiếu tien"},
		{"thiếu khoá di_toi", thay("data/tien.md", `"di_toi":[{"nhan":"Về A","man":"a"}],`, ``), "front matter thiếu di_toi"},
		// Steps as bullets, so the digit rule of a money body does not fire first.
		{"cờ tien không theo route (màn thường)", func(m map[string]string) {
			m["data/a.md"] = strings.NewReplacer("1. ", "- ", "2. ", "- ").Replace(m["data/a.md"])
			thay("data/a.md", `"tien":false}`, `"tien":true}`)(m)
		}, "a.md: tien phải là false cho màn «a»"},
		{"cờ tien không theo route (màn tiền)", thay("data/tien.md", `"tien":true}`, `"tien":false}`), "tien.md: tien phải là true cho màn «finance»"},
		// Screens and routes.
		{"màn không có trong _rut.json", thay("data/a.md", `"man":"a"`, `"man":"khong-co"`), "màn «khong-co» không có trong _rut.json"},
		{"di_toi tới route lạ", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút B","man":"c"}`), "di_toi «Nút B» tới «c» không có trong _rut.json"},
		{"di_toi bằng nhãn không khai", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút X","man":"b"}`), "di_toi đi bằng nhãn «Nút X» không khai trong nhanUI"},
		{"hai sổ một màn", func(m map[string]string) { m["data/a2.md"] = m["data/a.md"] }, "màn «a» đã có sổ tay a.md"},
		{"di_toi về chính màn", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút B","man":"b"},{"nhan":"Nút B","man":"a"}`), "a.md: di_toi «Nút B» về chính màn «a»"},
		{"di_toi không phải cạnh của mã", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút B","man":"welcome"}`), "a.md: di_toi «Nút B» từ «a» tới «welcome» không phải cạnh nào của mã"},
		// Body.
		{"trích nhãn không khai", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút Z»."), "trích «Nút Z» mà nhãn không khai trong nhanUI"},
		// «» pair up on every line, in order (review 13 round 3, NF3). Each
		// case breaks one part of the rule: a « left open at the end of the
		// line, a » with no « before it, a « opened again before it closed,
		// marks reversed, a quote across two lines, and one outside the steps.
		{"« không đóng trên dòng", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút B."), `a.md: « và » không thành cặp trên dòng: "1. Bấm «Nút B."`},
		{"» thừa", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút B»»."), `a.md: « và » không thành cặp trên dòng: "1. Bấm «Nút B»»."`},
		{"« mở lại trước khi đóng", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút Z «Nút B»."), `a.md: « và » không thành cặp trên dòng: "1. Bấm «Nút Z «Nút B»."`},
		{"» « đảo ngược", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm »Nút Z«."), `a.md: « và » không thành cặp trên dòng: "1. Bấm »Nút Z«."`},
		{"«…» vắt qua hai dòng", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút\nZ»."), `a.md: « và » không thành cặp trên dòng: "1. Bấm «Nút"`},
		{"» « đảo ngược ở tổng quan", thay("data/a.md", "Tổng quan của màn A.", "Tổng quan của màn A, có nút »Nút Z«."), `a.md: « và » không thành cặp trên dòng: "Tổng quan của màn A, có nút »Nút Z«."`},
		{"quá năm bước", thay("data/a.md", "1. Bấm «Nút B».", "1. a\n2. b\n3. c\n4. d\n5. e\n6. Bấm «Nút B»."), "có 6 bước, tối đa 5"},
		{"mục không có bước", thay("data/a.md", "1. Bấm «Nút B».", "Bấm «Nút B»."), "mục «Đi sang B» không có bước nào"},
		{"tiêu đề ###", thay("data/a.md", "## Đi sang B", "### Đi sang B"), "chỉ dùng tiêu đề «## »"},
		{"tiêu đề mục trích nhãn không khai", thay("data/a.md", "## Đi sang B", "## Đi sang «Nút Z»"), "tiêu đề mục «Đi sang «Nút Z»» trích «Nút Z» mà nhãn không khai trong nhanUI"},
		{"thiếu tổng quan", thay("data/a.md", "Tổng quan của màn A.\n", ""), "thiếu đoạn tổng quan"},
		{"tổng quan trích nhãn không khai", thay("data/a.md", "Tổng quan của màn A.", "Tổng quan của màn A, có nút «Nút Z»."), "a.md: tổng quan trích «Nút Z» mà nhãn không khai trong nhanUI"},
		{"hai mục cùng id", thay("data/a.md", "## Mở màn tiền", "## Đi sang B"), "hai mục cùng id a/di-sang-b"},
		{"không có mục nào", thay("data/tien.md", "## Tới màn này và đi tiếp\n", ""), "chưa có mục ## nào"},
		// Money screens: navigation only.
		{"màn tiền hai mục", thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\n\n## Chia tiền\n\n- Bấm «Về A»."), "màn tiền chỉ được có một mục chỉ đường, đang có 2"},
		{"màn tiền có chữ số", thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A» lần 2."), "màn tiền không được có chữ số"},
		{"màn tiền có chữ số ở tổng quan", thay("data/tien.md", "Màn tiền. Nếp", "Màn tiền của 2 người. Nếp"), "tien.md: màn tiền không được có chữ số"},
		{"màn tiền đổi tiêu đề mục", thay("data/tien.md", "## Tới màn này và đi tiếp", "## Chuyển khoản cho người ứng rồi báo là đã trả xong"), "màn tiền: tiêu đề mục phải là «Tới màn này và đi tiếp», đang là «Chuyển khoản cho người ứng rồi báo là đã trả xong»"},
		// The bypass of review 13 on the fixture, in the form the self-edge
		// rule does not see: the payment button declared as a way out to a
		// real neighbour and printed on the screen, but no button with that
		// label leads there.
		{"màn tiền: nút trả tiền khai làm lối ra", func(m map[string]string) {
			thay("data/_rut.json", `"man":"finance","nhan":["Về A"]`, `"man":"finance","nhan":["Về A","Đánh dấu đã trả"]`)(m)
			thay("data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","Đánh dấu đã trả"]`)(m)
			thay("data/tien.md", `{"nhan":"Về A","man":"a"}`, `{"nhan":"Về A","man":"a"},{"nhan":"Đánh dấu đã trả","man":"a"}`)(m)
			thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\n- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả».")(m)
		}, "tien.md: màn tiền: lối ra «Đánh dấu đã trả» tới «a» không phải nút nào của «finance» dẫn tới đó"},
		{"màn tiền: lối vào không phải nút", thay("data/_rut.json", `"canh":[{"den":"finance","nhan":"Mở tiền"}]`, `"canh":[]`), "tien.md: màn tiền: lối vào «Mở tiền» từ a.md không phải nút nào của «a» dẫn tới đây"},
		{"màn tiền dạy cách trả", func(m map[string]string) {
			thay("data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","Tiền đã về"]`)(m)
			thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\n- Nhận được tiền thì bấm «Tiền đã về».")(m)
		}, "màn tiền: bước không chỉ lối vào hay lối ra nào"},
		{"màn tiền có văn xuôi", thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\nChuyển khoản cho người ứng."), "màn tiền: dòng không phải bước chỉ đường"},
		// A door on the step does not carry another label with it: the payment
		// button beside «Về A» is how-to-pay text (review 13 round 2, P9).
		{"màn tiền: bước nêu cửa kèm nút trả tiền", func(m map[string]string) {
			thay("data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","Đánh dấu đã trả"]`)(m)
			thay("data/tien.md", "- Xong thì bấm «Về A».", "- Chuyển khoản xong thì bấm «Đánh dấu đã trả», rồi bấm «Về A».")(m)
		}, "tien.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào"},
		// The same with the door first: every quote is read, not only those
		// before the first door.
		{"màn tiền: bước nêu cửa trước, nút trả tiền sau", func(m map[string]string) {
			thay("data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","Đánh dấu đã trả"]`)(m)
			thay("data/tien.md", "- Xong thì bấm «Về A».", "- Bấm «Về A» sau khi đã bấm «Đánh dấu đã trả».")(m)
		}, "tien.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào"},
		// Keys: encoding/json keeps the last of two and ignores case, so each
		// of these would load as a different manual than the one reviewed.
		{"khoá trùng", thay("data/a.md", `"tien":false}`, `"tien":false,"tien":false}`), "a.md: front matter có khoá trùng «tien»"},
		{"khoá trùng trong di_toi", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút B","man":"welcome","man":"b"}`), "a.md: front matter có khoá trùng «man»"},
		{"nhanUI thứ hai trên màn tiền", thay("data/tien.md", `"tien":true}`, `"tien":true,"nhanUI":["Về A","Mở tiền","Đánh dấu đã trả"]}`), "tien.md: front matter có khoá trùng «nhanUI»"},
		{"khoá sai hoa thường", thay("data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"NhanUI":["Về A","Mở tiền"]`), "tien.md: front matter: khoá «NhanUI» phải viết đúng hoa thường"},
		{"khoá sai hoa thường trong di_toi", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"Nhan":"Nút B","man":"b"}`), "a.md: front matter: khoá «Nhan» phải viết đúng hoa thường"},
		// _rut.json.
		{"_rut.json khoá lạ", thay("data/_rut.json", `{"routes":[`, `{"extra":1,"routes":[`), `_rut.json: json: unknown field "extra"`},
		{"_rut.json cạnh tới route lạ", thay("data/_rut.json", `"di_toi":["b","finance"],"man":"a"`, `"di_toi":["zzz","finance"],"man":"a"`), "«a» đi tới «zzz» không có trong cây route"},
		{"_rut.json route trùng", thay("data/_rut.json", `"man":"b"`, `"man":"a"`), "route «a» trùng"},
		{"_rut.json cạnh có nhãn ngoài di_toi", thay("data/_rut.json", `{"den":"a","nhan":"Về A"}`, `{"den":"welcome","nhan":"Về A"}`), "cạnh có nhãn «Về A» của «finance» tới «welcome» không có trong di_toi"},
		{"_rut.json cạnh có nhãn không in trên màn", thay("data/_rut.json", `"man":"finance","nhan":["Về A"]`, `"man":"finance","nhan":[]`), "cạnh có nhãn «Về A» của «finance» mang nhãn không có trong nhan"},
		{"thiếu _rut.json", func(m map[string]string) { delete(m, "data/_rut.json") }, "đọc data/_rut.json"},
		{"không có sổ tay nào", func(m map[string]string) { delete(m, "data/a.md"); delete(m, "data/tien.md") }, "không có file sổ tay nào"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			m := mauDung()
			c.sua(m)
			_, err := napTu(m)
			if err == nil {
				t.Fatalf("loaded; want error containing %q", c.loi)
			}
			if !strings.Contains(err.Error(), c.loi) {
				t.Fatalf("error %q; want it to contain %q", err, c.loi)
			}
		})
	}
}

// kiemKhoaDau on its own: a key is recognised after every kind of value,
// including a nested array or object, so a duplicate is found wherever it
// sits, and text inside a string is never structure.
func TestKiemKhoaDau(t *testing.T) {
	for _, c := range []struct{ fm, loi string }{
		{`{"man":"a","tieu_de":"{\"nhanUI\": [\"x\"]}","nhanUI":["{","}"],"di_toi":[{"nhan":"x","man":"b"},{"nhan":"y","man":"c"}],"tien":false}`, ""},
		{`{"nhanUI":[],"tien":true,"tien":false}`, "front matter có khoá trùng «tien»"},
		{`{"di_toi":[{"nhan":"x","man":"b"}],"man":"a","man":"b"}`, "front matter có khoá trùng «man»"},
		{`{"di_toi":[{"nhan":"x","man":"b","nhan":"y"}]}`, "front matter có khoá trùng «nhan»"},
		{`{"man":"a","Tien":true}`, "front matter: khoá «Tien» phải viết đúng hoa thường như tên khoá"},
	} {
		got := ""
		if err := kiemKhoaDau(c.fm); err != nil {
			got = err.Error()
		}
		if got != c.loi {
			t.Errorf("kiemKhoaDau(%s) = %q, want %q", c.fm, got, c.loi)
		}
	}
}

// Package init runs phaiNap on the embedded data. Fed a broken manual it
// panics, so a broken manual can never load into a running process.
func TestPhaiNapPanicsOnABrokenManual(t *testing.T) {
	m := mauDung()
	m["data/a.md"] = strings.Replace(m["data/a.md"], `"man":"a"`, `"man":"khong-co"`, 1)
	fsys := fstest.MapFS{}
	for k, v := range m {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "huongdan: a.md: màn «khong-co» không có trong _rut.json") {
			t.Fatalf("recovered %v", r)
		}
	}()
	phaiNap(fsys)
}

// The embedded data, copied and broken one way at a time: the real manual
// goes through the same rules as the fixture.
// banSaoDuLieu is a writable copy of the embedded data.
func banSaoDuLieu(t *testing.T) fstest.MapFS {
	t.Helper()
	goc := fstest.MapFS{}
	err := fs.WalkDir(duLieu, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(duLieu, p)
		goc[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return goc
}

func TestDuLieuThatBiSuaBiTuChoi(t *testing.T) {
	goc := banSaoDuLieu(t)
	if _, err := nap(goc); err != nil {
		t.Fatalf("identity: the embedded data must load: %v", err)
	}
	type sua struct{ tep, cu, moi string }
	// The bypass of review 13, exactly as the reviewer made it on a scratch
	// copy of tai-chinh.md: the payment button «Đánh dấu đã trả» (a real
	// label, Bill.tsx) added to nhanUI, declared as a di_toi back to finance
	// itself, and a step telling the person to transfer and press it.
	const nhanTC = `"nhanUI": ["Tài chính của tôi", "Cá nhân", "Xem quyết toán"]`
	const buoc2TC = "- Ở mục chi theo nhóm, bấm «Xem quyết toán» để mở màn quyết toán của nhóm."
	themNhan := func(n string) sua {
		return sua{"data/tai-chinh.md", nhanTC, strings.TrimSuffix(nhanTC, "]") + `, "` + n + `"]`}
	}
	themBuoc := func(b string) sua { return sua{"data/tai-chinh.md", buoc2TC, buoc2TC + "\n" + b} }
	nhanTra := themNhan("Đánh dấu đã trả")
	buocTra := themBuoc("- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả».")
	diToiTra := func(den string) sua {
		return sua{"data/tai-chinh.md", `{"nhan": "Xem quyết toán", "man": "settlements/[id]"}`,
			`{"nhan": "Xem quyết toán", "man": "settlements/[id]"}, {"nhan": "Đánh dấu đã trả", "man": "` + den + `"}`}
	}
	cases := []struct {
		ten string
		sua []sua
		loi string
	}{
		{"man của chat-nhom", []sua{{"data/chat-nhom.md", `"man": "groups/[id]/chat"`, `"man": "groups/[id]/khong-co"`}}, "chat-nhom.md: màn «groups/[id]/khong-co» không có trong _rut.json"},
		{"di_toi của keo", []sua{{"data/keo.md", `{"nhan": "Chặng …", "man": "places/[id]"}`, `{"nhan": "Chặng …", "man": "places/[id]/khong-co"}`}}, "keo.md: di_toi «Chặng …» tới «places/[id]/khong-co» không có trong _rut.json"},
		{"bước trả tiền không nêu cửa", []sua{themBuoc("- Đọc số trên màn rồi chuyển khoản cho người ứng.")}, "tai-chinh.md: màn tiền: bước không chỉ lối vào hay lối ra nào"},
		// Review 13 round 2, P9 exactly: the payment button quoted on a line
		// that also names the door «Xem quyết toán».
		{"nút trả tiền cạnh một cửa", []sua{nhanTra, themBuoc("- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả», rồi bấm «Xem quyết toán».")},
			"tai-chinh.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào"},
		{"cửa trước, nút trả tiền sau", []sua{nhanTra, themBuoc("- Bấm «Xem quyết toán», chuyển khoản xong thì bấm «Đánh dấu đã trả».")},
			"tai-chinh.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào"},
		// The line as it stood before this rule: «Chi theo nhóm» is the heading
		// the button sits under, printed on the screen, and not a door.
		{"tiêu đề mục trên màn không phải cửa", []sua{themNhan("Chi theo nhóm"), {"data/tai-chinh.md", buoc2TC, "- Ở mục «Chi theo nhóm», bấm «Xem quyết toán» để mở màn quyết toán của nhóm."}},
			"tai-chinh.md: màn tiền: bước trích «Chi theo nhóm», không phải lối vào hay lối ra nào"},
		// Review 13 round 2, P5b: the heading declared as a way out. The
		// extractor pairs onAction with action only, so no button labelled
		// «Chi theo nhóm» leads to settlements/[id] in _rut.json any more.
		{"tiêu đề mục khai làm lối ra", []sua{themNhan("Chi theo nhóm"),
			{"data/tai-chinh.md", `{"nhan": "Xem quyết toán", "man": "settlements/[id]"}`, `{"nhan": "Xem quyết toán", "man": "settlements/[id]"}, {"nhan": "Chi theo nhóm", "man": "settlements/[id]"}`},
			themBuoc("- Đọc số ở «Chi theo nhóm» rồi chuyển khoản cho người ứng.")},
			"tai-chinh.md: màn tiền: lối ra «Chi theo nhóm» tới «settlements/[id]» không phải nút nào của «finance» dẫn tới đó"},
		// Review 13 round 2, N7: a second nhanUI key, which encoding/json let
		// win over the first.
		{"nhanUI thứ hai", []sua{{"data/tai-chinh.md", `"tien": true`, `"tien": true,` + "\n  " + strings.TrimSuffix(nhanTC, "]") + `, "Đánh dấu đã trả"]`}},
			"tai-chinh.md: front matter có khoá trùng «nhanUI»"},
		{"khoá nut", []sua{{"data/ca-nhan.md", "---json\n{", "---json\n{\n  \"nut\": [],"}}, `ca-nhan.md: front matter không đọc được: json: unknown field "nut"`},
		// Review 13 round 3, probe Q1 exactly: the payment button between
		// reversed marks, beside a door. The counts match and reTrich saw only
		// the door, so every label rule passed it.
		{"dấu «» đảo ngược quanh nút trả tiền", []sua{themBuoc("- Bấm «Xem quyết toán», chuyển khoản xong thì bấm »Đánh dấu đã trả«.")},
			`tai-chinh.md: « và » không thành cặp trên dòng: "- Bấm «Xem quyết toán», chuyển khoản xong thì bấm »Đánh dấu đã trả«."`},
		{"lách của review 13, đúng nguyên bản", []sua{nhanTra, diToiTra("finance"), buocTra}, "tai-chinh.md: di_toi «Đánh dấu đã trả» về chính màn «finance»"},
		{"lách qua một cạnh thật của mã", []sua{nhanTra, diToiTra("settlements/[id]"), buocTra}, "tai-chinh.md: màn tiền: lối ra «Đánh dấu đã trả» tới «settlements/[id]» không phải nút nào của «finance» dẫn tới đó"},
		{"lách qua một cạnh mã không có", []sua{nhanTra, diToiTra("messages"), buocTra}, "tai-chinh.md: di_toi «Đánh dấu đã trả» từ «finance» tới «messages» không phải cạnh nào của mã"},
		// On chia-hoa-don the payment button IS printed (Bill.tsx is one of the
		// route's files), so «the label is in the route's _rut.json nhan» would
		// let it through; only «a button with that label leads there» refuses it.
		{"lách trên màn có in nút trả tiền", []sua{
			{"data/chia-hoa-don.md", `"nhanUI": ["Chia hóa đơn", "Tạo mới", "Chia bill buổi này", "Xem quyết toán", "Về Tin nhắn"]`,
				`"nhanUI": ["Chia hóa đơn", "Tạo mới", "Chia bill buổi này", "Xem quyết toán", "Về Tin nhắn", "Đánh dấu đã trả"]`},
			{"data/chia-hoa-don.md", `{"nhan": "Về Tin nhắn", "man": "messages"}`, `{"nhan": "Về Tin nhắn", "man": "messages"}, {"nhan": "Đánh dấu đã trả", "man": "settlements/[id]"}`},
			{"data/chia-hoa-don.md", "- Ghi xong, bấm «Xem quyết toán» để mở màn quyết toán, hoặc «Về Tin nhắn».",
				"- Ghi xong, bấm «Xem quyết toán» để mở màn quyết toán, hoặc «Về Tin nhắn».\n- Chuyển khoản xong thì bấm «Đánh dấu đã trả»."},
		}, "chia-hoa-don.md: màn tiền: lối ra «Đánh dấu đã trả» tới «settlements/[id]» không phải nút nào của «smart-split/[id]/review» dẫn tới đó"},
		{"tiêu đề mục màn tiền đổi thành cách trả", []sua{{"data/tai-chinh.md", "## Tới màn này và đi tiếp", "## Chuyển khoản cho người ứng rồi báo là đã trả xong"}}, "tai-chinh.md: màn tiền: tiêu đề mục phải là «Tới màn này và đi tiếp»"},
	}
	for _, c := range cases {
		m := fstest.MapFS{}
		for k, v := range goc {
			m[k] = v
		}
		for _, x := range c.sua {
			cu := string(m[x.tep].Data)
			if !strings.Contains(cu, x.cu) {
				t.Fatalf("%s: %s no longer contains %q", c.ten, x.tep, x.cu)
			}
			m[x.tep] = &fstest.MapFile{Data: []byte(strings.Replace(cu, x.cu, x.moi, 1))}
		}
		_, err := nap(m)
		if err == nil || !strings.Contains(err.Error(), c.loi) {
			t.Errorf("%s: error %v; want %q", c.ten, err, c.loi)
		}
	}
}

// A step on a money screen may name the place a person starts from by its
// title alone, when that screen's manual has a way here and the title is
// printed on it; a title of any other screen is not a door.
func TestManTienCuaLaTieuDe(t *testing.T) {
	chiTieuDe := func(m map[string]string, tieuDe string) {
		m["data/tien.md"] = strings.Replace(m["data/tien.md"], `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","`+tieuDe+`"]`, 1)
		m["data/tien.md"] = strings.Replace(m["data/tien.md"], "- Từ màn A: bấm «Mở tiền».", "- Bắt đầu từ «"+tieuDe+"».", 1)
	}
	m := mauDung()
	chiTieuDe(m, "Màn A")
	if _, err := napTu(m); err != nil {
		t.Fatalf("identity: a step quoting only the start screen's title must load: %v", err)
	}
	khongPhaiCua := "màn tiền: bước không chỉ lối vào hay lối ra nào: \"Bắt đầu từ «"

	// The same title, no longer printed on a.
	m = mauDung()
	chiTieuDe(m, "Màn A")
	m["data/_rut.json"] = strings.Replace(m["data/_rut.json"], `"nhan":["Màn A","Mở tiền","Nút B"]`, `"nhan":["Mở tiền","Nút B"]`, 1)
	if _, err := napTu(m); err == nil || !strings.Contains(err.Error(), khongPhaiCua+"Màn A»") {
		t.Errorf("title not printed on its screen: %v", err)
	}

	// The title of a screen with no way here.
	m = mauDung()
	m["data/_rut.json"] = strings.Replace(m["data/_rut.json"], "\n]}", `,
  {"canh":[],"di_toi":[],"man":"c","nhan":["Màn C"],"tep":["app/c.tsx"]}
]}`, 1)
	m["data/c.md"] = "---json\n" + `{"man":"c","tieu_de":"Màn C","nhanUI":[],"di_toi":[],"tien":false}` + "\n---\nMàn C.\n\n## Xem C\n\n1. Xem.\n"
	chiTieuDe(m, "Màn C")
	if _, err := napTu(m); err == nil || !strings.Contains(err.Error(), khongPhaiCua+"Màn C»") {
		t.Errorf("unrelated screen's title: %v", err)
	}

	// The title of another money screen that does have a way here: a money
	// screen is not where Nếp tells a person to start from.
	m = mauDung()
	m["data/_rut.json"] = strings.Replace(m["data/_rut.json"], "\n]}", `,
  {"canh":[{"den":"finance","nhan":"Về tài chính"}],"di_toi":["finance"],"man":"settlements/[id]","nhan":["Quyết toán","Về tài chính"],"tep":["app/settlements/[id]/index.tsx"]}
]}`, 1)
	m["data/q.md"] = "---json\n" + `{"man":"settlements/[id]","tieu_de":"Quyết toán","nhanUI":["Về tài chính"],"di_toi":[{"nhan":"Về tài chính","man":"finance"}],"tien":true}` +
		"\n---\nMàn quyết toán.\n\n## Tới màn này và đi tiếp\n\n- Bấm «Về tài chính».\n"
	if _, err := napTu(m); err != nil {
		t.Fatalf("identity: a second money screen with a door must load: %v", err)
	}
	chiTieuDe(m, "Quyết toán")
	if _, err := napTu(m); err == nil || !strings.Contains(err.Error(), khongPhaiCua+"Quyết toán»") {
		t.Errorf("money screen's title: %v", err)
	}

	// The money screen's own title, beside a real door, printed on it or not
	// (review 13 round 3, NF2). On the embedded data both money titles are
	// doors in as well, so only a fixture where «Tiền» is none tells a rule
	// that counts the own title as a door from one that does not.
	doi := func(m map[string]string, tep, cu, moi string) {
		t.Helper()
		if !strings.Contains(m[tep], cu) {
			t.Fatalf("fixture drifted: %s no longer contains %q", tep, cu)
		}
		m[tep] = strings.Replace(m[tep], cu, moi, 1)
	}
	for _, inTrenMan := range []bool{false, true} {
		m = mauDung()
		doi(m, "data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","Tiền"]`)
		doi(m, "data/tien.md", "- Xong thì bấm «Về A».", "- Ở «Tiền», bấm «Về A».")
		if inTrenMan {
			doi(m, "data/_rut.json", `"man":"finance","nhan":["Về A"]`, `"man":"finance","nhan":["Tiền","Về A"]`)
		}
		if _, err := napTu(m); err == nil || !strings.Contains(err.Error(), `tien.md: màn tiền: bước trích «Tiền», không phải lối vào hay lối ra nào: "Ở «Tiền», bấm «Về A»."`) {
			t.Errorf("own title (printed on the screen: %v): %v", inTrenMan, err)
		}
	}
}

// A tab-bar edge is an edge of the code although no route file navigates it:
// the bar is drawn by app/(tabs)/_layout.tsx, which is not a route.
func TestDiToiQuaThanhTab(t *testing.T) {
	m := mauDung()
	m["data/_rut.json"] = strings.Replace(m["data/_rut.json"], `"di_toi":["b","finance"],"man":"a"`, `"di_toi":["finance"],"man":"a"`, 1)
	if _, err := napTu(m); err != nil {
		t.Fatalf("identity: a -> b over the tab bar must load: %v", err)
	}
	m["data/_rut.json"] = strings.Replace(m["data/_rut.json"], `"tep":["app/(tabs)/b.tsx"]`, `"tep":["app/b.tsx"]`, 1)
	if _, err := napTu(m); err == nil || !strings.Contains(err.Error(), "a.md: di_toi «Nút B» từ «a» tới «b» không phải cạnh nào của mã") {
		t.Fatalf("b no longer a tab: %v", err)
	}
}

// Every entry of canhNgoaiRut is used, is not already an edge the code shows,
// starts on a tab (the reason is the tab bar), and the source it cites still
// makes it real. Without it the embedded manual does not load.
func TestCanhNgoaiRut(t *testing.T) {
	doc := func(p ...string) string {
		raw, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "..", "..", "apps", "mobile"}, p...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	if bar := doc("src", "rudi", "ui", "RudiTabBar.tsx"); !strings.Contains(bar, `router.push("/create")`) || !strings.Contains(bar, `accessibilityLabel="Tạo mới"`) {
		t.Fatal("RudiTabBar no longer pushes /create from a «Tạo mới» button")
	}
	if !strings.Contains(doc("app", "(tabs)", "_layout.tsx"), "<RudiTabBar") {
		t.Fatal("the tab layout no longer renders RudiTabBar")
	}
	if len(canhNgoaiRut) == 0 {
		t.Fatal("empty: the embedded plan -> create would not load")
	}
	for cap, lyDo := range canhNgoaiRut {
		if strings.TrimSpace(lyDo) == "" {
			t.Errorf("%v has no reason", cap)
		}
		if soTay.banDo.laCanhMa(cap[0], cap[1]) {
			t.Errorf("%v is already an edge of the code: the entry is stale", cap)
		}
		if !soTay.banDo.tab[cap[0]] || cap[1] != "create" {
			t.Errorf("%v is not a tab-bar «Tạo mới» edge", cap)
		}
		dung := false
		for _, tr := range soTay.trang {
			for _, d := range tr.diToi {
				dung = dung || (tr.man == cap[0] && d.Man == cap[1])
			}
		}
		if !dung {
			t.Errorf("%v is used by no manual", cap)
		}
	}
	cu := canhNgoaiRut
	canhNgoaiRut = map[[2]string]string{}
	defer func() { canhNgoaiRut = cu }()
	if _, err := nap(duLieu); err == nil || !strings.Contains(err.Error(), "len-plan.md: di_toi «Tạo mới» từ «plan» tới «create» không phải cạnh nào của mã") {
		t.Fatalf("without the entry: %v", err)
	}
}

func TestDuLieuNhungDayDu(t *testing.T) {
	if len(soTay.trang) != 13 {
		t.Fatalf("%d manuals embedded, want 13", len(soTay.trang))
	}
	if len(soTay.doan) != 54 {
		t.Fatalf("%d sections embedded, want 54", len(soTay.doan))
	}
	if len(soTay.cacMan) != 50 {
		t.Fatalf("%d routes in _rut.json, want 50", len(soTay.cacMan))
	}
	for _, d := range soTay.doan {
		if !soTay.coMan[d.Man] || d.TieuDe == "" || len(d.Buoc) == 0 || len(d.Buoc) > MaxBuoc {
			t.Errorf("section %+v", d)
		}
		if !strings.HasPrefix(d.ID, soTay.trangCua[d.Man].ten+"/") {
			t.Errorf("id %s not under its file", d.ID)
		}
	}
}
