package huongdan

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// A small manual known to be good: two tab screens and a money screen. Every
// refusal below is this fixture with one defect, so a checker that refuses
// everything fails the identity case and a checker gone blind fails the rest.
func mauDung() map[string]string {
	return map[string]string{
		"data/_rut.json": `{"routes":[
  {"di_toi":["b"],"man":"a","nhan":[],"tep":["app/(tabs)/a.tsx"]},
  {"di_toi":[],"man":"b","nhan":[],"tep":["app/(tabs)/b.tsx"]},
  {"di_toi":["a"],"man":"tien","nhan":[],"tep":["app/tien.tsx"]},
  {"di_toi":["a"],"man":"welcome","nhan":[],"tep":["app/welcome.tsx"]}
]}
`,
		"data/a.md": `---json
{"man":"a","tieu_de":"Màn A","nhanUI":["Nút B","Mở tiền"],"di_toi":[{"nhan":"Nút B","man":"b"},{"nhan":"Mở tiền","man":"tien"}],"tien":false}
---
Tổng quan của màn A.

## Đi sang B

1. Bấm «Nút B».

## Mở màn tiền

1. Bấm «Mở tiền».
2. Xem xong thì quay lại.
`,
		"data/tien.md": `---json
{"man":"tien","tieu_de":"Tiền","nhanUI":["Về A","Mở tiền"],"di_toi":[{"nhan":"Về A","man":"a"}],"tien":true}
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
	tien := s.theoMan("tien")
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
		// Screens and routes.
		{"màn không có trong _rut.json", thay("data/a.md", `"man":"a"`, `"man":"khong-co"`), "màn «khong-co» không có trong _rut.json"},
		{"di_toi tới route lạ", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút B","man":"c"}`), "di_toi «Nút B» tới «c» không có trong _rut.json"},
		{"di_toi bằng nhãn không khai", thay("data/a.md", `{"nhan":"Nút B","man":"b"}`, `{"nhan":"Nút X","man":"b"}`), "di_toi đi bằng nhãn «Nút X» không khai trong nhanUI"},
		{"hai sổ một màn", func(m map[string]string) { m["data/a2.md"] = m["data/a.md"] }, "màn «a» đã có sổ tay a.md"},
		// Body.
		{"trích nhãn không khai", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút Z»."), "trích «Nút Z» mà nhãn không khai trong nhanUI"},
		{"« và » lệch", thay("data/a.md", "1. Bấm «Nút B».", "1. Bấm «Nút B."), "số « và » không khớp"},
		{"quá năm bước", thay("data/a.md", "1. Bấm «Nút B».", "1. a\n2. b\n3. c\n4. d\n5. e\n6. Bấm «Nút B»."), "có 6 bước, tối đa 5"},
		{"mục không có bước", thay("data/a.md", "1. Bấm «Nút B».", "Bấm «Nút B»."), "mục «Đi sang B» không có bước nào"},
		{"tiêu đề ###", thay("data/a.md", "## Đi sang B", "### Đi sang B"), "chỉ dùng tiêu đề «## »"},
		{"thiếu tổng quan", thay("data/a.md", "Tổng quan của màn A.\n", ""), "thiếu đoạn tổng quan"},
		{"hai mục cùng id", thay("data/a.md", "## Mở màn tiền", "## Đi sang B"), "hai mục cùng id a/di-sang-b"},
		{"không có mục nào", thay("data/tien.md", "## Tới màn này và đi tiếp\n", ""), "chưa có mục ## nào"},
		// Money screens: navigation only.
		{"màn tiền hai mục", thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\n\n## Chia tiền\n\n- Bấm «Về A»."), "màn tiền chỉ được có một mục chỉ đường, đang có 2"},
		{"màn tiền có chữ số", thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A» lần 2."), "màn tiền không được có chữ số"},
		{"màn tiền dạy cách trả", func(m map[string]string) {
			thay("data/tien.md", `"nhanUI":["Về A","Mở tiền"]`, `"nhanUI":["Về A","Mở tiền","Tiền đã về"]`)(m)
			thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\n- Nhận được tiền thì bấm «Tiền đã về».")(m)
		}, "màn tiền: bước không chỉ lối vào hay lối ra nào"},
		{"màn tiền có văn xuôi", thay("data/tien.md", "- Xong thì bấm «Về A».", "- Xong thì bấm «Về A».\nChuyển khoản cho người ứng."), "màn tiền: dòng không phải bước chỉ đường"},
		// _rut.json.
		{"_rut.json khoá lạ", thay("data/_rut.json", `{"routes":[`, `{"extra":1,"routes":[`), `_rut.json: json: unknown field "extra"`},
		{"_rut.json cạnh tới route lạ", thay("data/_rut.json", `{"di_toi":["b"],"man":"a"`, `{"di_toi":["zzz"],"man":"a"`), "«a» đi tới «zzz» không có trong cây route"},
		{"_rut.json route trùng", thay("data/_rut.json", `"man":"b"`, `"man":"a"`), "route «a» trùng"},
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
func TestDuLieuThatBiSuaBiTuChoi(t *testing.T) {
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
	if _, err := nap(goc); err != nil {
		t.Fatalf("identity: the embedded data must load: %v", err)
	}
	cases := []struct{ tep, cu, moi, loi string }{
		{"data/chat-nhom.md", `"man": "groups/[id]/chat"`, `"man": "groups/[id]/khong-co"`, "chat-nhom.md: màn «groups/[id]/khong-co» không có trong _rut.json"},
		{"data/keo.md", `{"nhan": "Chặng …", "man": "places/[id]"}`, `{"nhan": "Chặng …", "man": "places/[id]/khong-co"}`, "keo.md: di_toi «Chặng …» tới «places/[id]/khong-co» không có trong _rut.json"},
		{"data/tai-chinh.md", "- Mở tab «Cá nhân», bấm «Tài chính của tôi».", "- Mở tab «Cá nhân», bấm «Tài chính của tôi».\n- Đọc số ở «Chi theo nhóm» rồi chuyển khoản.", "tai-chinh.md: màn tiền: bước không chỉ lối vào hay lối ra nào"},
		{"data/ca-nhan.md", "---json\n{", "---json\n{\n  \"nut\": [],", `ca-nhan.md: front matter không đọc được: json: unknown field "nut"`},
	}
	for _, c := range cases {
		m := fstest.MapFS{}
		for k, v := range goc {
			m[k] = v
		}
		cu := string(goc[c.tep].Data)
		if !strings.Contains(cu, c.cu) {
			t.Fatalf("%s no longer contains %q", c.tep, c.cu)
		}
		m[c.tep] = &fstest.MapFile{Data: []byte(strings.Replace(cu, c.cu, c.moi, 1))}
		_, err := nap(m)
		if err == nil || !strings.Contains(err.Error(), c.loi) {
			t.Errorf("%s: error %v; want %q", c.tep, err, c.loi)
		}
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
