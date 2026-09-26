package chatassist

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Nếp reads nothing for context (ADR-0036 §4, ADR-0033 §4).
//
// `khong_doc_chat_test.go` holds the group path to «never read the message
// text». Nếp's rule is stricter: it may not read ANY table for context. Not
// the conversation, not taste, not history, not the roster, not a display
// name. What the model hears is what the device sent.
//
// So this gate walks every function reachable from Nếp's three entry points
// (the two handlers and the worker step) through the package's own call graph,
// collects every string literal in them, and holds the SQL in those literals to
// the three tables the path needs to exist: the session, the person row it
// locks, and the job row itself. Calling `prepare`, `roster` or `authority`
// from here would pull memberships, places or messages into the closure and
// turn this red at the call site, which is the point: the property decays
// through a helper, not through a line anyone reads as «reading chat».
var (
	nepGoc      = []string{"nepCreate", "nepGet", "processNep"}
	nepBangDuoc = map[string]bool{"chat_ai_invocations": true, "account_sessions": true, "people": true}
	// A column the path has no business naming even on an allowed table.
	nepCotCam = regexp.MustCompile(`(?i)\b(display_name|body|interests|budget\w*)\b`)
	sqlBang   = regexp.MustCompile(`(?i)\b(?:from|join|update|into)\s+([a-z_][a-z0-9_.]*)`)
)

// dongGoi parses the package's non-test sources into two namespaces, the way
// Go resolves them: `h.name(` can only be a method, a bare `name(` only a
// function. Folding them together would let the local `cancel()` of a
// `context.WithTimeout` resolve to the `cancel` HANDLER and drag the group's
// room checks into Nếp's closure -- a false red that teaches people to ignore
// the gate.
func dongGoi(t *testing.T) (method, fn map[string]*ast.FuncDecl) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	method, fn = map[string]*ast.FuncDecl{}, map[string]*ast.FuncDecl{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fs, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			if x, ok := d.(*ast.FuncDecl); ok {
				if x.Recv != nil {
					method[x.Name.Name] = x
				} else {
					fn[x.Name.Name] = x
				}
			}
		}
	}
	return method, fn
}

// baoDong returns the reachable functions ("m:" methods, "f:" functions) and
// every string literal in them, following package-level string constants
// referenced by name too.
func baoDong(t *testing.T, hang map[string]string, goc []string) ([]string, []string) {
	t.Helper()
	method, fn := dongGoi(t)
	daThay := map[string]bool{}
	var chu []string
	todo := []string{}
	for _, g := range goc {
		todo = append(todo, "m:"+g, "f:"+g)
	}
	for len(todo) > 0 {
		khoa := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if daThay[khoa] {
			continue
		}
		var decl *ast.FuncDecl
		switch {
		case strings.HasPrefix(khoa, "m:"):
			decl = method[khoa[2:]]
		case strings.HasPrefix(khoa, "f:"):
			decl = fn[khoa[2:]]
		case strings.HasPrefix(khoa, "c:"):
			daThay[khoa] = true
			chu = append(chu, hang[khoa[2:]])
			continue
		}
		if decl == nil {
			continue
		}
		daThay[khoa] = true
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					if s, err := strconv.Unquote(x.Value); err == nil {
						chu = append(chu, s)
					}
				}
			case *ast.CallExpr:
				switch f := x.Fun.(type) {
				case *ast.Ident:
					todo = append(todo, "f:"+f.Name)
				case *ast.SelectorExpr:
					if recv, ok := f.X.(*ast.Ident); ok && recv.Name == "h" {
						todo = append(todo, "m:"+f.Sel.Name)
					}
				}
			case *ast.Ident:
				if _, ok := hang[x.Name]; ok {
					todo = append(todo, "c:"+x.Name)
				}
			}
			return true
		})
	}
	names := make([]string, 0, len(daThay))
	for n := range daThay {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, chu
}

// hangChuoi collects package-level string constants, so a query hidden behind
// a name (`nepColumns`, `columns`) is still read.
func hangChuoi(t *testing.T) map[string]string {
	t.Helper()
	files, _ := filepath.Glob("*.go")
	fs := token.NewFileSet()
	out := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fs, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, spec := range g.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if s, err := strconv.Unquote(lit.Value); err == nil {
								out[name.Name] = s
							}
						}
					}
				}
			}
		}
	}
	return out
}

func viPham(chu []string) []string {
	var sai []string
	for _, s := range chu {
		for _, m := range sqlBang.FindAllStringSubmatch(s, -1) {
			if !nepBangDuoc[strings.ToLower(m[1])] {
				sai = append(sai, "bảng "+m[1]+" trong: "+s)
			}
		}
		if sqlBang.MatchString(s) && nepCotCam.MatchString(s) {
			sai = append(sai, "cột cấm trong: "+s)
		}
	}
	return sai
}

func TestNepKhongDocBangNaoDeLayNguCanh(t *testing.T) {
	method, _ := dongGoi(t)
	for _, g := range nepGoc {
		if _, ok := method[g]; !ok {
			t.Fatalf("không thấy %s; cổng đang đọc sai chỗ", g)
		}
	}
	names, chu := baoDong(t, hangChuoi(t), nepGoc)
	// A closure that reached nothing proves nothing. The path has to reach the
	// session check and the job row, or the walk itself is broken.
	for _, can := range []string{"f:phien", "m:nepXong", "m:nepThatBai", "m:nepConSong", "c:nepColumns"} {
		found := false
		for _, n := range names {
			if n == can {
				found = true
			}
		}
		if !found {
			t.Fatalf("bao đóng không chạm %s; đồ thị gọi đang hỏng: %v", can, names)
		}
	}
	t.Logf("bao đóng Nếp (%d hàm/hằng): %v", len(names), names)
	var coSQL int
	for _, s := range chu {
		if sqlBang.MatchString(s) {
			coSQL++
		}
	}
	t.Logf("%d câu SQL trên đường Nếp", coSQL)
	if coSQL < 5 {
		t.Fatalf("chỉ thấy %d câu SQL trên đường Nếp; mẫu đã trượt", coSQL)
	}
	for _, v := range viPham(chu) {
		t.Errorf("đường Nếp đọc thứ ngoài phiên và hàng việc của chính nó: %s", v)
	}
	// Named, so a helper that drags room context in is caught even before its
	// SQL changes: these are the functions that lay room state on a question.
	for _, cam := range []string{"m:prepare", "f:roster", "f:authority", "f:thuocPhong", "f:tacGia", "f:hoiThoai", "m:publish", "m:begin", "m:preflight"} {
		for _, n := range names {
			if n == cam {
				t.Errorf("đường Nếp gọi tới %s, là hàm đắp ngữ cảnh của phòng", cam)
			}
		}
	}
}

// The gate is only worth having if it can fail: fed the group path, which
// legitimately reads rooms, it must go red.
func TestCongNepThucSuDoDuoc(t *testing.T) {
	_, chu := baoDong(t, hangChuoi(t), []string{"prepare"})
	if len(viPham(chu)) == 0 {
		t.Fatal("cổng Nếp không thấy gì sai trên prepare, vốn đọc memberships và gu nhóm")
	}
	if len(viPham([]string{"SELECT id, body FROM messages WHERE context_id=$1"})) == 0 {
		t.Fatal("cổng không bắt được một câu đọc messages ngay trước mắt")
	}
	if len(viPham([]string{"SELECT display_name FROM people WHERE id=$1"})) == 0 {
		t.Fatal("cổng không bắt được một câu đọc tên hiển thị")
	}
	if v := viPham([]string{"SELECT id FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE"}); len(v) != 0 {
		t.Fatalf("câu khoá hàng người gọi bị coi là đọc ngữ cảnh: %v", v)
	}
}
