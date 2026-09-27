package traloi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The owner's rule (docs/architecture/03-ai-engine-hop-dong.md §8) held
// structurally over the retrieval-answer path: no package of it compiles a
// regular expression or imports a reader of words (the vocabulary readers,
// query-side destination resolution, keyword date parsing, the old
// instruction patterns), and from the guard it uses only the data-format
// check. Markup delimiters ([[p:…]], «…») are read with strings.Index, a
// parser of our own format.
var (
	goiDuong  = []string{".", "../crag", "../kiemchung", "../cautruc"}
	camImport = []string{
		"regexp",
		"mobile/services/core/internal/domain/tuvung",
		"mobile/services/core/internal/domain/thoigian",
		"mobile/services/core/internal/domain/chatintent",
		"mobile/services/core/internal/rag",
		"mobile/services/core/internal/aiharness/preprocess",
		"mobile/services/core/internal/domain/promptsafety",
	}
	// guardDuoc is all this path may name in package guard.
	guardDuoc = map[string]bool{"DinhDang": true, "RaSach": true}
)

func TestKhongHeuristic(t *testing.T) {
	// Canary: every banned path names a package that exists, so a wrong
	// path (the review found one) cannot leave the ban blind.
	for _, cam := range camImport {
		rel, ok := strings.CutPrefix(cam, "mobile/services/core/internal/")
		if !ok {
			continue
		}
		if fi, err := os.Stat(filepath.Join("..", "..", rel)); err != nil || !fi.IsDir() {
			t.Errorf("banned import %s names no package: the ban is blind", cam)
		}
	}
	fset := token.NewFileSet()
	files := 0
	for _, dir := range goiDuong {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			files++
			guardTen := ""
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				for _, cam := range camImport {
					if p == cam || strings.HasPrefix(p, cam+"/") {
						t.Errorf("%s imports %s", path, p)
					}
				}
				if p == "mobile/services/core/internal/aiharness/guard" {
					guardTen = "guard"
					if imp.Name != nil {
						guardTen = imp.Name.Name
					}
				}
			}
			if guardTen == "" {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				s, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if id, ok := s.X.(*ast.Ident); ok && id.Name == guardTen && !guardDuoc[s.Sel.Name] {
					t.Errorf("%s uses guard.%s: only the data-format check belongs on this path", path, s.Sel.Name)
				}
				return true
			})
		}
	}
	if files < 10 {
		t.Fatalf("read %d files: the walk is broken", files)
	}
}
