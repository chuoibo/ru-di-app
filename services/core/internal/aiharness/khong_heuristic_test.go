package aiharness

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The owner's rule (2026-09-25), held on the code of the whole engine path
// rather than promised: no function reads the MEANING of the turn's text by
// word lists or patterns. The text is followed as data from where it enters
// (the person's message and history, the queries and memory text the model
// wrote) through every variable, field, parameter and return value it
// flows into, across the engine's packages. Wherever it goes, it may only:
//
//   - be cleaned structurally or cut (preprocess.LamSach, strings.TrimSpace,
//     strings.Fields / Join, huongdan's rune cap, rag/xephang's folding into
//     index terms);
//   - be laid into a datamarked block (prompts.BocDuLieuDanhDau, DanhDau);
//   - be checked for emptiness, length or UTF-8 validity, or for the data
//     formats of privacy (guard.DinhDang: phone, email, account, card);
//   - be handed to a ranker (BM25, the reranker, the embedder, a retriever,
//     the memory port), which scores it and decides nothing.
//
// Any other use is red: a strings.Contains, a regexp, a comparison with a
// word, a switch on it, a map lookup by it, a call into a vocabulary reader.
// The walk sees package-level initialisers too, and follows closures.
var (
	// goiDongCo are the engine path's packages.
	goiDongCo = []string{
		"mobile/services/core/internal/aiharness",
		"mobile/services/core/internal/aiharness/agent",
		"mobile/services/core/internal/aiharness/cautruc",
		"mobile/services/core/internal/aiharness/crag",
		"mobile/services/core/internal/aiharness/hieu",
		"mobile/services/core/internal/aiharness/kiemchung",
		"mobile/services/core/internal/aiharness/tactu",
		"mobile/services/core/internal/aiharness/tools",
		"mobile/services/core/internal/aiharness/traloi",
		"mobile/services/core/internal/aiharness/trinho",
		"mobile/services/core/internal/aiharness/truyhoi",
		"mobile/services/core/internal/aidoc",
		"mobile/services/core/internal/huongdan",
	}
	// nguonChu are the fields that carry the turn's text into the engine.
	nguonChu = []string{
		"mobile/services/core/internal/aiharness.Turn.LoiNho",
		"mobile/services/core/internal/aiharness.LuotNep.Chu",
		"mobile/services/core/internal/aiharness/hieu.Vao.Cau",
		"mobile/services/core/internal/aiharness/hieu.TruyVan.Cau",
		"mobile/services/core/internal/aiharness/hieu.TruyVan.CauCoDau",
		"mobile/services/core/internal/aiharness/trinho.Luot.Chu",
		"mobile/services/core/internal/aiharness/truyhoi.YeuCau.Cau",
		"mobile/services/core/internal/aiharness/truyhoi.YeuCau.CauCoDau",
		"mobile/services/core/internal/aiharness/tactu.Vao.Cau",
		"mobile/services/core/internal/aiharness/traloi.Vao.Cau",
		"mobile/services/core/internal/huongdan.Hoi.Cau",
		"mobile/services/core/internal/aiharness/tools.thamSoTim.TruyVan",
		"mobile/services/core/internal/aiharness/tools.thamSoSoTay.TruyVan",
		"mobile/services/core/internal/aiharness/tools.thamSoNho.TruyVan",
		"mobile/services/core/internal/aiharness/tools.thamSoGhiNho.NoiDung",
		"mobile/services/core/internal/aiharness/tools.thamSoQuen.MoTa",
		// The model's own prose: the answer, its draft sentences, the
		// question back and its options. The phrase rules that once
		// guessed from it whether an answer claims an action or moves
		// money are gone (the verifier judges that); none may come back.
		"mobile/services/core/internal/aiharness/tactu.Ra.Text",
		"mobile/services/core/internal/aiharness/traloi.Cau.Chu",
		"mobile/services/core/internal/aiharness/traloi.KetQua.Chu",
		"mobile/services/core/internal/aiharness/hieu.KetQua.CauHoiLai",
		"mobile/services/core/internal/aiharness/hieu.KetQua.LuaChonHoiLai",
	}
	// ketQuaChu are the calls whose result is the model's prose.
	ketQuaChu = []string{
		"mobile/services/core/internal/aiharness/agent.Chay#r0",
	}
	// hamBien are the calls whose result is the text again, cleaned or cut.
	hamBien = map[string]bool{
		"mobile/services/core/internal/aiharness/preprocess.LamSach": true, // its Chu
		"builtin.append":    true,
		"strings.TrimSpace": true,
		"strings.Fields":    true,
		"strings.Join":      true,
		"mobile/services/core/internal/huongdan.catCau":    true,
		"mobile/services/core/internal/rag/xephang.AmTiet": true,
	}
	// hamCauTruc are the engine's own parsers of STRUCTURE in the model's
	// prose, each reviewed by hand and named here with what it reads: the
	// walk does not descend into them, and their result is the text again.
	// They split at punctuation and line breaks, and parse the markup our
	// own schema defines ([[p:alias]] tokens, «…» button labels); none
	// holds a word.
	hamCauTruc = map[string]string{
		"mobile/services/core/internal/aiharness/traloi.tachCau":        "[[p:…]] tokens and «…» labels (our schema's markup)",
		"mobile/services/core/internal/aiharness/traloi.GhepVanXuoi":    "renders [[p:…]] tokens from the ledger",
		"mobile/services/core/internal/aiharness/traloi.TachCauVanXuoi": "splits sentences at . ! ? … and line breaks",
	}
	// truongKhongChu are fields a structural parser fills with ids of the
	// turn's closed lists, not prose: they are looked up by set membership
	// (grounding), which the rule allows, and carry no taint.
	truongKhongChu = map[string]string{
		"mobile/services/core/internal/aiharness/traloi.phanTich.biDanh":    "aliases, checked against the turn's closed enum",
		"mobile/services/core/internal/aiharness/traloi.phanTich.nhan":      "«…» labels, checked against the manual's label set",
		"mobile/services/core/internal/aiharness/kiemchung.TuyenBo.NhanNut": "«…» labels, checked against the manual's label set",
	}
	// hamNhan are the calls that may consume the text and decide nothing:
	// a datamarked block, a length or validity check, the privacy format
	// check, a ranker.
	hamNhan = map[string]string{
		"mobile/services/core/internal/aiharness/prompts.BocDuLieuDanhDau":     "datamarked block",
		"mobile/services/core/internal/aiharness/prompts.DanhDau":              "datamarking",
		"unicode/utf8.RuneCountInString":                                       "length",
		"unicode/utf8.ValidString":                                             "validity",
		"mobile/services/core/internal/aiharness/guard.DinhDang":               "privacy data-format check",
		"(mobile/services/core/internal/aiharness/guard.DauRa).Kiem":           "output guard: canary marker, quoted instruction, privacy formats",
		"(*mobile/services/core/internal/rag/xephang.ChiMuc).Tim":              "BM25 ranking",
		"(mobile/services/core/internal/aiharness/nhung.Nhung).Nhung":          "embedding",
		"(mobile/services/core/internal/aiharness/trinho.TriNho).Nho":          "memory ranking",
		"(mobile/services/core/internal/aiharness/truyhoi.Reranker).XepLai":    "reranking",
		"(mobile/services/core/internal/aiharness/truyhoi.Passthrough).XepLai": "reranking (identity)",
		"(mobile/services/core/internal/aiharness/trinho.SuThatMoi).Kiem":      "length check",
		"(mobile/services/core/internal/aiharness/trinho.QuenGi).Kiem":         "exactly-one check",
		"(mobile/services/core/internal/aiharness/trinho.NganHan).Them":        "short-term memory write",
		"(mobile/services/core/internal/aiharness/tools.phienThietBi).Them":    "short-term memory write (dropped)",
		"(mobile/services/core/internal/aiharness.phienThietBi).Them":          "short-term memory write (dropped)",
		"(mobile/services/core/internal/aiharness/testkit.NganHan).Them":       "short-term memory write",
		"(*mobile/services/core/internal/aiharness/tools.BoiCanh).giaiBiDanh":  "alias lookup (ids, not text)",
		"(*mobile/services/core/internal/aiharness/tools.BoiCanh).Goi":         "tool dispatch (arguments validated by schema)",
		"(*mobile/services/core/internal/aiharness/hieu.KhoViDu).Chon":         "embedding for example choice",
		"(*mobile/services/core/internal/aiharness/hieu.KhoViDuLuoi).Chon":     "embedding for example choice",
		"(mobile/services/core/internal/aiharness/truyhoi.Retriever).Tim":      "retrieval",
		"(mobile/services/core/internal/aiharness/truyhoi.Passthrough).Tim":    "retrieval",
		"(*mobile/services/core/internal/aiharness/tools.SoCai).Ghi":           "ledger",
		"(mobile/services/core/internal/aiharness/crag.Cham).DanhGia":          "grader (model call)",
		"(mobile/services/core/internal/aiharness/kiemchung.Verifier).PhanTu":  "verifier (model call)",
		"(mobile/services/core/internal/aiharness/hieu.Hieu).Hieu":             "router (model call)",
		"mobile/services/core/internal/aiharness/agent.Chay":                   "model call",
		"mobile/services/core/internal/aiharness/cautruc.Goi":                  "model call",
		"(mobile/services/core/internal/aiharness/llm.Model).GenerateContent":  "model call",
		"(*mobile/services/core/internal/aiharness/llm.Dem).GenerateContent":   "model call",
	}
)

// viPham is one use of the turn's text outside the rule.
type viPham struct {
	ham, cho string
}

func (v viPham) String() string { return v.ham + ": " + v.cho }

// dongChu is the analysis over a set of loaded packages.
type dongChu struct {
	pkgs    []*packages.Package
	truong  map[string]bool // tainted field keys
	thamSo  map[string]bool // tainted parameter keys (func#pi)
	ketQua  map[string]bool // tainted result keys (func#ri)
	bien    map[types.Object]bool
	litBien map[*ast.FuncLit]types.Object
	them    bool
}

func moiDongChu(pkgs []*packages.Package) *dongChu {
	d := &dongChu{pkgs: pkgs, truong: map[string]bool{}, thamSo: map[string]bool{}, ketQua: map[string]bool{},
		bien: map[types.Object]bool{}, litBien: map[*ast.FuncLit]types.Object{}}
	for _, k := range nguonChu {
		d.truong[k] = true
	}
	for _, k := range ketQuaChu {
		d.ketQua[k] = true
	}
	return d
}

func khoaTruong(sel *types.Selection) string {
	if sel == nil || sel.Kind() != types.FieldVal {
		return ""
	}
	t := sel.Recv()
	for {
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
			continue
		}
		break
	}
	// The field's declaring struct: walk the index path.
	for _, i := range sel.Index()[:len(sel.Index())-1] {
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			return ""
		}
		t = st.Field(i).Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return ""
	}
	return n.Obj().Pkg().Path() + "." + n.Obj().Name() + "." + sel.Obj().Name()
}

func khoaHam(fn *types.Func) string { return fn.FullName() }

// hamGoi is the callee of c: its key, and the *types.Func or local var.
func (d *dongChu) hamGoi(p *packages.Package, c *ast.CallExpr) (string, *types.Func, types.Object) {
	var id *ast.Ident
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.IndexExpr:
		if s, ok := f.X.(*ast.SelectorExpr); ok {
			id = s.Sel
		} else if i, ok := f.X.(*ast.Ident); ok {
			id = i
		}
	}
	if id == nil {
		return "", nil, nil
	}
	switch o := p.TypesInfo.Uses[id].(type) {
	case *types.Func:
		if o.Origin() != nil {
			o = o.Origin()
		}
		return khoaHam(o), o, nil
	case *types.Var:
		return fmt.Sprintf("bien@%d", o.Pos()), nil, o
	case *types.Builtin:
		return "builtin." + o.Name(), nil, nil
	}
	return "", nil, nil
}

func laDongCo(key string) bool {
	k := strings.TrimLeft(key, "(*")
	for _, g := range goiDongCo {
		if strings.HasPrefix(k, g+".") {
			return true
		}
	}
	return false
}

// nhiem reports whether e carries the turn's text.
func (d *dongChu) nhiem(p *packages.Package, e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return d.nhiem(p, x.X)
	case *ast.Ident:
		o := p.TypesInfo.Uses[x]
		if o == nil {
			o = p.TypesInfo.Defs[x]
		}
		return o != nil && d.bien[o]
	case *ast.SelectorExpr:
		sel := p.TypesInfo.Selections[x]
		k := khoaTruong(sel)
		if truongKhongChu[k] != "" {
			return false
		}
		if k != "" && d.truong[k] {
			return true
		}
		// A text field of a value that carries the text (the result of
		// preprocess.LamSach on the message: its Chu, not its counts).
		return sel != nil && sel.Kind() == types.FieldVal && laChu(sel.Type()) && d.nhiem(p, x.X)
	case *ast.IndexExpr:
		return d.nhiem(p, x.X)
	case *ast.SliceExpr:
		return d.nhiem(p, x.X)
	case *ast.StarExpr:
		return d.nhiem(p, x.X)
	case *ast.UnaryExpr:
		return d.nhiem(p, x.X)
	case *ast.TypeAssertExpr:
		return d.nhiem(p, x.X)
	case *ast.BinaryExpr:
		return x.Op == token.ADD && (d.nhiem(p, x.X) || d.nhiem(p, x.Y))
	case *ast.CompositeLit:
		if _, ok := p.TypesInfo.TypeOf(x).Underlying().(*types.Slice); ok {
			for _, el := range x.Elts {
				if d.nhiem(p, el) {
					return true
				}
			}
		}
		return false
	case *ast.CallExpr:
		if tv, ok := p.TypesInfo.Types[x.Fun]; ok && tv.IsType() {
			return len(x.Args) == 1 && d.nhiem(p, x.Args[0])
		}
		key, _, _ := d.hamGoi(p, x)
		if hamBien[key] || hamCauTruc[key] != "" {
			for _, a := range x.Args {
				if d.nhiem(p, a) {
					return true
				}
			}
			return false
		}
		return d.ketQua[key+"#r0"]
	}
	return false
}

// laChu: a string or a slice of strings.
func laChu(t types.Type) bool {
	if s, ok := t.Underlying().(*types.Slice); ok {
		t = s.Elem()
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

func (d *dongChu) danh(o types.Object) {
	if o != nil && !d.bien[o] {
		d.bien[o] = true
		d.them = true
	}
}

func (d *dongChu) danhKhoa(m map[string]bool, k string) {
	if k != "" && !m[k] {
		m[k] = true
		d.them = true
	}
}

// lanTruyen spreads the taint one step over every package; true when it
// learnt something.
func (d *dongChu) lanTruyen() bool {
	d.them = false
	for _, p := range d.pkgs {
		// Parameters of the package's own functions, by key.
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					fn, _ := p.TypesInfo.Defs[x.Name].(*types.Func)
					if fn == nil {
						return true
					}
					sig := fn.Type().(*types.Signature)
					for i := 0; i < sig.Params().Len(); i++ {
						if d.thamSo[fmt.Sprintf("%s#p%d", khoaHam(fn), i)] {
							d.danh(sig.Params().At(i))
						}
					}
				case *ast.AssignStmt:
					for i, r := range x.Rhs {
						if lit, ok := r.(*ast.FuncLit); ok && i < len(x.Lhs) {
							if id, ok := x.Lhs[i].(*ast.Ident); ok {
								o := p.TypesInfo.Defs[id]
								if o == nil {
									o = p.TypesInfo.Uses[id]
								}
								d.litBien[lit] = o
							}
						}
					}
				}
				return true
			})
		}
		for _, f := range p.Syntax {
			var stack []ast.Node
			ast.Inspect(f, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, n)
				switch x := n.(type) {
				case *ast.FuncLit:
					if o := d.litBien[x]; o != nil {
						key := fmt.Sprintf("bien@%d", o.Pos())
						sig := p.TypesInfo.TypeOf(x).(*types.Signature)
						for i := 0; i < sig.Params().Len(); i++ {
							if d.thamSo[fmt.Sprintf("%s#p%d", key, i)] {
								d.danh(sig.Params().At(i))
							}
						}
					}
				case *ast.AssignStmt:
					if len(x.Rhs) == 1 && len(x.Lhs) > 1 {
						if c, ok := x.Rhs[0].(*ast.CallExpr); ok {
							key, _, _ := d.hamGoi(p, c)
							for i, l := range x.Lhs {
								if d.ketQua[fmt.Sprintf("%s#r%d", key, i)] {
									d.danhLhs(p, l)
								}
							}
						}
						return true
					}
					for i, r := range x.Rhs {
						if i < len(x.Lhs) && d.nhiem(p, r) {
							d.danhLhs(p, x.Lhs[i])
						}
					}
				case *ast.ValueSpec:
					for i, v := range x.Values {
						if i < len(x.Names) && d.nhiem(p, v) {
							d.danh(p.TypesInfo.Defs[x.Names[i]])
						}
					}
				case *ast.RangeStmt:
					if d.nhiem(p, x.X) && x.Value != nil {
						d.danhLhs(p, x.Value)
					}
				case *ast.CompositeLit:
					for _, el := range x.Elts {
						kv, ok := el.(*ast.KeyValueExpr)
						if !ok || !d.nhiem(p, kv.Value) {
							continue
						}
						if id, ok := kv.Key.(*ast.Ident); ok {
							if v, ok := p.TypesInfo.Uses[id].(*types.Var); ok && v.IsField() {
								t := p.TypesInfo.TypeOf(x)
								if pt, ok := t.(*types.Pointer); ok {
									t = pt.Elem()
								}
								if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil {
									d.danhKhoa(d.truong, n.Obj().Pkg().Path()+"."+n.Obj().Name()+"."+v.Name())
								}
							}
						}
					}
				case *ast.CallExpr:
					key, _, _ := d.hamGoi(p, x)
					if key == "" || !(laDongCo(key) || strings.HasPrefix(key, "bien@")) || hamCauTruc[key] != "" {
						return true
					}
					for i, a := range x.Args {
						if d.nhiem(p, a) {
							d.danhKhoa(d.thamSo, fmt.Sprintf("%s#p%d", key, i))
						}
					}
				case *ast.ReturnStmt:
					key := d.hamBaoQuanh(p, stack)
					for i, r := range x.Results {
						if key != "" && d.nhiem(p, r) {
							d.danhKhoa(d.ketQua, fmt.Sprintf("%s#r%d", key, i))
						}
					}
				}
				return true
			})
		}
	}
	return d.them
}

func (d *dongChu) danhLhs(p *packages.Package, l ast.Expr) {
	switch x := l.(type) {
	case *ast.Ident:
		o := p.TypesInfo.Defs[x]
		if o == nil {
			o = p.TypesInfo.Uses[x]
		}
		d.danh(o)
	case *ast.SelectorExpr:
		d.danhKhoa(d.truong, khoaTruong(p.TypesInfo.Selections[x]))
	}
}

// hamBaoQuanh is the key of the innermost function around the stack's top.
func (d *dongChu) hamBaoQuanh(p *packages.Package, stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		switch f := stack[i].(type) {
		case *ast.FuncLit:
			if o := d.litBien[f]; o != nil {
				return fmt.Sprintf("bien@%d", o.Pos())
			}
			return ""
		case *ast.FuncDecl:
			if fn, ok := p.TypesInfo.Defs[f.Name].(*types.Func); ok {
				return khoaHam(fn)
			}
			return ""
		}
	}
	return ""
}

// kiem finds every use of the text outside the rule.
func (d *dongChu) kiem() []viPham {
	for d.lanTruyen() {
	}
	var out []viPham
	for _, p := range d.pkgs {
		for _, f := range p.Syntax {
			var stack []ast.Node
			ast.Inspect(f, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, n)
				e, ok := n.(ast.Expr)
				if !ok || len(stack) < 2 || !d.nhiem(p, e) {
					return true
				}
				// Only the outermost tainted expression of a chain is judged
				// by its context; an inner one is judged by the outer.
				if lydo := d.ngoaiLuat(p, e, stack[len(stack)-2]); lydo != "" {
					out = append(out, viPham{ham: tenHamQuanh(p, stack) + " (" + p.Fset.Position(e.Pos()).String() + ")", cho: lydo})
				}
				return true
			})
		}
	}
	return out
}

func tenHamQuanh(p *packages.Package, stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if f, ok := stack[i].(*ast.FuncDecl); ok {
			return p.PkgPath[strings.LastIndex(p.PkgPath, "/")+1:] + "." + f.Name.Name
		}
	}
	return p.PkgPath[strings.LastIndex(p.PkgPath, "/")+1:] + ".var"
}

// ngoaiLuat is why a tainted e in parent breaks the rule ("" when it does
// not).
func (d *dongChu) ngoaiLuat(p *packages.Package, e ast.Expr, parent ast.Node) string {
	switch x := parent.(type) {
	case *ast.ParenExpr, *ast.StarExpr, *ast.UnaryExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.TypeAssertExpr:
		if ix, ok := x.(*ast.IndexExpr); ok && ix.Index == e {
			return "used as an index"
		}
		return ""
	case *ast.SelectorExpr, *ast.Field:
		return ""
	case *ast.CallExpr:
		if x.Fun == e {
			return ""
		}
		if tv, ok := p.TypesInfo.Types[x.Fun]; ok && tv.IsType() {
			return ""
		}
		key, _, _ := d.hamGoi(p, x)
		switch {
		case key == "builtin.len", key == "builtin.append":
			return ""
		case hamBien[key], hamCauTruc[key] != "":
			return ""
		case hamNhan[key] != "":
			return ""
		case laDongCo(key), strings.HasPrefix(key, "bien@"):
			return ""
		}
		return "argument of " + key
	case *ast.CompositeLit:
		return ""
	case *ast.KeyValueExpr:
		if x.Key == e {
			return "used as a key"
		}
		return ""
	case *ast.AssignStmt, *ast.ValueSpec, *ast.ReturnStmt, *ast.RangeStmt:
		if r, ok := x.(*ast.RangeStmt); ok && r.X != e {
			return ""
		}
		return ""
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return ""
		}
		khac := x.X
		if khac == e {
			khac = x.Y
		}
		// Emptiness, or identity of two texts that both came from the
		// turn (did the grader rewrite the query?): neither reads a word.
		if (x.Op == token.EQL || x.Op == token.NEQ) && (laChuoiRong(khac) || d.nhiem(p, khac)) {
			return ""
		}
		return "compared with " + fmt.Sprint(x.Op)
	case *ast.SwitchStmt:
		return "switched on"
	case *ast.IfStmt, *ast.ForStmt:
		return "used as a condition"
	}
	return fmt.Sprintf("used in a %T", parent)
}

func laChuoiRong(e ast.Expr) bool {
	b, ok := e.(*ast.BasicLit)
	return ok && b.Kind == token.STRING && (b.Value == `""` || b.Value == "``")
}

func napGoi(t *testing.T, mau ...string) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports}
	pkgs, err := packages.Load(cfg, mau...)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			t.Fatalf("%s: %v", p.PkgPath, p.Errors)
		}
	}
	return pkgs
}

func TestKhongDocNghiaTrenDuongEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("loads the engine's packages from source")
	}
	pkgs := napGoi(t, goiDongCo...)
	if len(pkgs) != len(goiDongCo) {
		t.Fatalf("loaded %d of %d packages", len(pkgs), len(goiDongCo))
	}
	d := moiDongChu(pkgs)
	vp := d.kiem()
	for _, v := range vp {
		t.Errorf("the turn's text is read for meaning: %s", v)
	}
	// Canary 1: the walk followed the text where it goes. Fields it must
	// have reached from the seeds, parameters it must have tainted.
	for _, k := range []string{
		"mobile/services/core/internal/rag.YeuCau.Cau",
	} {
		if !d.truong[k] {
			t.Errorf("the walk never reached %s: it is blind", k)
		}
	}
	coThamSo := false
	for k := range d.thamSo {
		coThamSo = coThamSo || strings.Contains(k, "nepTruyHoi")
	}
	if !coThamSo || len(d.ketQua) == 0 || len(d.bien) < 10 {
		var ks []string
		for k := range d.ketQua {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		t.Fatalf("the walk tainted %d variables, results %v, nepTruyHoi's parameter %v: it is blind", len(d.bien), ks, coThamSo)
	}
	// Canary 2: the same walk is red on a package that reads the text the
	// ways the reviews found survived (a keyword slot, a keyword tool
	// choice on a closure's result, a regexp on a copy, a word switch, a
	// vocabulary reader, a map lookup by word).
	cpkgs := napGoi(t, append([]string{"./testdata/khongheuristic"}, goiDongCo...)...)
	cd := moiDongChu(cpkgs)
	theoHam := map[string]bool{}
	for _, v := range cd.kiem() {
		if strings.HasPrefix(v.ham, "khongheuristic.") {
			theoHam[v.ham[:strings.Index(v.ham, " ")]] = true
		}
	}
	for _, h := range []string{"khongheuristic.OChay", "khongheuristic.ChonToolTheoTu", "khongheuristic.RegexpQuaBien",
		"khongheuristic.SwitchTheoTu", "khongheuristic.DocTuVung", "khongheuristic.TraBangTheoTu",
		"khongheuristic.CumTuTraLoi", "khongheuristic.CumTuHoiLai", "khongheuristic.CumTuVongLap"} {
		if !theoHam[h] {
			t.Errorf("canary %s is green: the walk cannot see it", h)
		}
	}
	if theoHam["khongheuristic.HopLe"] {
		t.Error("canary HopLe (structural uses only) is red")
	}
}
