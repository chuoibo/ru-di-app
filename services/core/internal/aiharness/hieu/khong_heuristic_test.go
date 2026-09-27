package hieu

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The owner's rule (2026-09-25), held on the code rather than promised: no
// function of the router reads the MEANING of the person's message. The
// message (Vao.Cau) may only be checked for emptiness, laid into its
// datamarked block, or embedded to pick examples; the router imports no
// regexp, guard or preprocess, and calls no reader of tuvung or thoigian
// (only the id lists and labels, and the clock's arithmetic and format).
var (
	// docCau are the functions that may touch Vao.Cau, and why.
	docCau = map[string]string{
		"NoiDung": "empty check and datamarked block",
		"Chon":    "embedding for example choice",
	}
	// hamChoCau are the only calls the message may be an argument of.
	hamChoCau = map[string]bool{
		"strings.TrimSpace": true,
		"mobile/services/core/internal/aiharness/prompts.BocDuLieuDanhDau": true,
	}
	camImport = []string{"regexp", "mobile/services/core/internal/aiharness/guard", "mobile/services/core/internal/aiharness/preprocess", "mobile/services/core/internal/domain/chatintent", "mobile/services/core/internal/rag"}
	// The only functions of these packages the router may call.
	choGoi = map[string]map[string]bool{
		"mobile/services/core/internal/domain/tuvung":   {"Muc": true, "IDs": true},
		"mobile/services/core/internal/domain/thoigian": {"Now": true, "DongBayGio": true, "Cong": true, "String": true},
	}
)

func TestKhongDocNghiaBangTu(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports, Tests: false}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load: %v", pkgs)
	}
	p := pkgs[0]
	for path := range p.Imports {
		for _, cam := range camImport {
			if path == cam || strings.HasPrefix(path, cam+"/") {
				t.Errorf("the router imports %s", path)
			}
		}
	}
	vao := p.Types.Scope().Lookup("Vao")
	if vao == nil {
		t.Fatal("no Vao")
	}
	var cauField *types.Var
	st := vao.Type().Underlying().(*types.Struct)
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Name() == "Cau" {
			cauField = st.Field(i)
		}
	}
	if cauField == nil {
		t.Fatal("no Vao.Cau")
	}
	// Even inside the functions allowed to touch it, the message may only
	// be an argument of an emptiness check or of the datamarked block, or
	// the one element of the texts handed to the embedder: any other use
	// (a Contains, a reader, a copy into a variable) is red.
	laCau := func(e ast.Expr) (*ast.SelectorExpr, bool) {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}
		s := p.TypesInfo.Selections[sel]
		return sel, s != nil && s.Obj() == cauField
	}
	tenHam := func(c *ast.CallExpr) string {
		var id *ast.Ident
		switch f := c.Fun.(type) {
		case *ast.SelectorExpr:
			id = f.Sel
		case *ast.Ident:
			id = f
		default:
			return ""
		}
		fn, ok := p.TypesInfo.Uses[id].(*types.Func)
		if !ok || fn.Pkg() == nil {
			return ""
		}
		return fn.Pkg().Path() + "." + fn.Name()
	}
	choDung := map[*ast.SelectorExpr]bool{}
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ham := tenHam(c)
			for _, a := range c.Args {
				if sel, ok := laCau(a); ok && hamChoCau[ham] {
					choDung[sel] = true
				}
				if lit, ok := a.(*ast.CompositeLit); ok && ham == "mobile/services/core/internal/aiharness/nhung.Nhung" && len(lit.Elts) == 1 {
					if sel, ok := laCau(lit.Elts[0]); ok {
						choDung[sel] = true
					}
				}
			}
			return true
		})
	}
	touched := map[string]bool{}
	calls := 0
	for _, f := range p.Syntax {
		for _, d := range f.Decls {
			// Package-level initialisers are inspected too, under the name
			// "var": a reader hidden in one would run just the same.
			ten, than := "var", ast.Node(d)
			if fn, ok := d.(*ast.FuncDecl); ok {
				if fn.Body == nil {
					continue
				}
				ten, than = fn.Name.Name, fn.Body
			}
			ast.Inspect(than, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if s := p.TypesInfo.Selections[sel]; s != nil && s.Obj() == cauField {
					touched[ten] = true
					if _, ok := docCau[ten]; !ok {
						t.Errorf("%s reads Vao.Cau", ten)
					}
					if !choDung[sel] {
						t.Errorf("%s uses Vao.Cau other than as an argument of %v or the embedding call", ten, hamChoCau)
					}
				}
				obj := p.TypesInfo.Uses[sel.Sel]
				fnObj, ok := obj.(*types.Func)
				if !ok || fnObj.Pkg() == nil {
					return true
				}
				if allowed, ok := choGoi[fnObj.Pkg().Path()]; ok {
					calls++
					if !allowed[fnObj.Name()] {
						t.Errorf("%s calls %s.%s", ten, fnObj.Pkg().Name(), fnObj.Name())
					}
				}
				return true
			})
		}
	}
	var names []string
	for n := range touched {
		names = append(names, n)
	}
	sort.Strings(names)
	// The canary: the check can see the reads that are allowed, so its
	// silence about the others means something.
	if len(names) != len(docCau) || calls == 0 {
		t.Fatalf("the check saw Vao.Cau read in %v and %d vocabulary/clock calls: it is blind", names, calls)
	}
}
