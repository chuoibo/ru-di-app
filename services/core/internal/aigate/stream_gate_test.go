package aigate

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Slice 11 streams answers (design 01 §3.5, design 02 §5.1). Two promises hold
// the stream to what the output guard read, and to the one writer:
//
//   - only the output guard's 48-rune window (aiharness/guard) calls Delta, the
//     Sink method that puts answer text into a stream: every other package
//     that could reach a Sink hands text to the window, never to the stream;
//   - only internal/aistream appends to a Redis stream (XADD): the key a job
//     writes, and whether a v2 job ever touches a room key, is decided in one
//     place.
//
// Both are read off the whole module with types (non-test files only), so a
// method value, an interface call or another package's helper is seen as well
// as a direct call.

const (
	pkgGuard    = "mobile/services/core/internal/aiharness/guard"
	pkgAistream = "mobile/services/core/internal/aistream"
	pkgRedis    = "github.com/redis/go-redis/v9"
)

// laSinkDelta says whether f is a Delta(int, string) method with no result:
// the Sink's, a type implementing it, or the window's own receiver interface.
func laSinkDelta(f *types.Func) bool {
	if f == nil || f.Name() != "Delta" {
		return false
	}
	sig, ok := f.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || sig.Results().Len() != 0 || sig.Params().Len() != 2 {
		return false
	}
	a, b := sig.Params().At(0).Type(), sig.Params().At(1).Type()
	return types.Identical(a, types.Typ[types.Int]) && types.Identical(b, types.Typ[types.String])
}

// suDung lists, as "caller -> method", every use of a method sel picks out in
// the bodies of fns: calls and method values alike.
func suDung(decls map[*types.Func]*ast.FuncDecl, infos map[*types.Func]*types.Info, pick func(*types.Func) bool) []string {
	var out []string
	for f, decl := range decls {
		info := infos[f]
		ast.Inspect(decl, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			var m *types.Func
			if s := info.Selections[sel]; s != nil {
				m, _ = s.Obj().(*types.Func)
			} else {
				m, _ = info.Uses[sel.Sel].(*types.Func)
			}
			if pick(m) {
				out = append(out, f.Pkg().Path()+" "+f.FullName()+" -> "+m.FullName())
			}
			return true
		})
	}
	sort.Strings(out)
	return out
}

func ngoai(uses []string, pkg string) []string {
	var out []string
	for _, u := range uses {
		if !strings.HasPrefix(u, pkg+" ") {
			out = append(out, u)
		}
	}
	return out
}

// Only the window calls Sink.Delta.
func TestChiGuardGoiSinkDelta(t *testing.T) {
	g := load(t)
	uses := suDung(g.decl, g.info, laSinkDelta)
	if len(uses) == 0 {
		t.Fatal("no call of Delta found at all: the window's own call is one, so the scan is broken")
	}
	for _, u := range ngoai(uses, pkgGuard) {
		t.Errorf("Delta called outside the output guard window: %s", u)
	}
	t.Logf("Delta uses: %v", uses)
}

// Only aistream issues XADD: a go-redis XAdd call, or a raw "XADD" command.
func TestChiAistreamGhiXADD(t *testing.T) {
	g := load(t)
	uses := suDung(g.decl, g.info, func(f *types.Func) bool {
		return f != nil && f.Name() == "XAdd" && f.Pkg() != nil && strings.HasPrefix(f.Pkg().Path(), pkgRedis)
	})
	for f, decl := range g.decl {
		ast.Inspect(decl, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && strings.EqualFold(strings.TrimSpace(s), "xadd") {
					uses = append(uses, f.Pkg().Path()+" "+f.FullName()+" -> raw XADD")
				}
			}
			return true
		})
	}
	if len(uses) == 0 {
		t.Fatal("no XADD found at all: aistream's own is one, so the scan is broken")
	}
	for _, u := range ngoai(uses, pkgAistream) {
		t.Errorf("XADD outside aistream: %s", u)
	}
	t.Logf("XADD uses: %v", uses)
}

// The two SSE roots (design 02 §5.2) read what their authorization needs and
// the job row, followed into every package they reach (aistream's reader
// included): the Nếp stream only Nếp's tables -- chat_ai_invocations,
// account_sessions, people, and job_outbox should a trigger ever reach it --
// and the group stream the tables the group's own authorization reads
// (authority) plus the job row. Neither reads a message.
func TestSSERootsDocDungBangCuaMinh(t *testing.T) {
	g := load(t)
	nep := g.reach(g.root(t, "(*"+pkgChat+".Handler).nepEvents"))
	for _, must := range []string{pkgChat + ".phien", "mobile/services/core/internal/aistream.Follow", "(*" + pkgChat + ".Handler).phucVuSSE"} {
		if !nep.funcs[must] {
			t.Fatalf("the Nếp stream's closure never reaches %s; the walk is broken", must)
		}
	}
	nepAllowed := map[string]bool{"chat_ai_invocations": true, "account_sessions": true, "people": true, "job_outbox": true}
	used := tables(nep.strings)
	if len(used) == 0 {
		t.Fatal("no SQL on the Nếp stream's path; the extraction slipped")
	}
	for _, name := range sortedKeys(used) {
		if !nepAllowed[name] {
			t.Errorf("the Nếp stream reaches table %s: %q", name, used[name][0])
		}
	}
	for name := range nep.funcs {
		for _, forbidden := range []string{".authority", ".prepare", ".roster", ".begin"} {
			if strings.HasSuffix(name, forbidden) {
				t.Errorf("the Nếp stream reaches %s, the room's authorization", name)
			}
		}
	}
	nhom := g.reach(g.root(t, "(*"+pkgChat+".Handler).suKienNhom"))
	if !nhom.funcs[pkgChat+".authority"] {
		t.Fatal("the group stream never reaches authority; the walk is broken")
	}
	groupAllowed := map[string]bool{"chat_ai_invocations": true}
	for name := range tables(g.reach(g.root(t, pkgChat+".authority")).strings) {
		groupAllowed[name] = true
	}
	nhomUsed := tables(nhom.strings)
	for _, name := range sortedKeys(nhomUsed) {
		if !groupAllowed[name] {
			t.Errorf("the group stream reaches table %s: %q", name, nhomUsed[name][0])
		}
	}
	if _, ok := nhomUsed["messages"]; ok {
		t.Error("the group stream reads messages")
	}
	// Canary: the group stream held to the Nếp allowlist is red -- it reads
	// memberships and contexts -- so the allowlist above can refuse.
	red := false
	for name := range nhomUsed {
		if !nepAllowed[name] {
			red = true
		}
	}
	if !red {
		t.Fatalf("the Nếp allowlist found nothing wrong on the group stream, which reads %v", sortedKeys(nhomUsed))
	}
	t.Logf("Nếp stream tables %v; group stream tables %v", sortedKeys(used), sortedKeys(nhomUsed))
}

// Canary: a package outside the guard that calls Delta -- directly, through
// an interface, or as a method value -- is seen; the window's call alone is
// identity. Type-checked from source, so the scan runs on what it runs on in
// the module.
func TestSinkDeltaGateCanRed(t *testing.T) {
	src := `package chatassist
type Sink interface{ Delta(p int, text string); LamLai() }
type ghi struct{}
func (ghi) Delta(int, string) {}
func (ghi) LamLai() {}
func thang(s Sink) { s.Delta(0, "x") }
func giaTri(g ghi) { f := g.Delta; f(0, "y") }
func khac(s Sink) { s.LamLai() }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Defs: map[*ast.Ident]types.Object{}}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("mobile/services/core/internal/chatassist", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	decls := map[*types.Func]*ast.FuncDecl{}
	infos := map[*types.Func]*types.Info{}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			f := info.Defs[fd.Name].(*types.Func)
			decls[f], infos[f] = fd, info
		}
	}
	bad := ngoai(suDung(decls, infos, laSinkDelta), pkgGuard)
	if len(bad) != 2 || !strings.Contains(strings.Join(bad, " "), "thang") || !strings.Contains(strings.Join(bad, " "), "giaTri") {
		t.Fatalf("the gate saw %v, want the direct call and the method value", bad)
	}
	// Identity: the same calls inside the guard package are allowed.
	if len(ngoai(suDung(decls, infos, laSinkDelta), pkg.Path())) != 0 {
		t.Fatal("calls inside the allowed package were refused")
	}
}

// Every write of answer text into a Redis stream (review of slice 11,
// finding 5): not only Sink.Delta, but the stream kinds that carry text
// (aistream.Delta, Phan and Xong), a DeltaData literal, a Writer.Ghi call,
// and Stream.Append/AppendBatch. Outside internal/aistream each is allowed
// only in the one method of chatassist's job stream that writes it -- whose
// text the output guard's window hands it (Delta), or that ends a job after
// its commit (xongNhom, xongNep) -- and dauTuHang, which builds the row's
// ending for one reader's connection, never for Redis. A raw write of the
// brain's answer with Writer.Ghi(aistream.Delta, ...) is red here.
var ghiLuongChoPhep = map[string]map[string]string{
	"const Delta":       {"(*" + pkgChat + ".luongViec).Delta": "the window's Deltas"},
	"const Phan":        {"(*" + pkgChat + ".luongViec).Phan": "the engine's grounded parts"},
	"DeltaData literal": {"(*" + pkgChat + ".luongViec).Delta": "the window's Deltas"},
	"const Xong": {
		"(*" + pkgChat + ".luongViec).xongNhom": "after the card committed",
		"(*" + pkgChat + ".luongViec).xongNep":  "after the sealed answer committed",
		pkgChat + ".dauTuHang":                  "the row's ending, for one connection, never Redis",
	},
	"Writer.Ghi": {
		"(*" + pkgChat + ".luongViec).Delta":     "",
		"(*" + pkgChat + ".luongViec).Phan":      "",
		"(*" + pkgChat + ".luongViec).trangThai": "",
		"(*" + pkgChat + ".luongViec).LamLai":    "",
		"(*" + pkgChat + ".luongViec).xongNhom":  "",
		"(*" + pkgChat + ".luongViec).xongNep":   "",
		"(*" + pkgChat + ".luongViec).thatBai":   "",
		"(*" + pkgChat + ".luongViec).huy":       "",
	},
	"Stream.Append":      {},
	"Stream.AppendBatch": {},
}

// ghiLuong lists, as "what caller", every text-carrying stream write outside
// aistream.
func ghiLuong(g *graph) map[string][]string {
	out := map[string][]string{}
	for f, decl := range g.decl {
		if f.Pkg() == nil || f.Pkg().Path() == pkgAistream {
			continue
		}
		info := g.info[f]
		ghi := func(what string) { out[what] = append(out[what], f.FullName()) }
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if c, ok := info.Uses[x].(*types.Const); ok && c.Pkg() != nil && c.Pkg().Path() == pkgAistream {
					switch c.Name() {
					case "Delta", "Phan", "Xong":
						ghi("const " + c.Name())
					}
				}
			case *ast.CompositeLit:
				if tv, ok := info.Types[x]; ok {
					if nt, ok := tv.Type.(*types.Named); ok && nt.Obj().Pkg() != nil && nt.Obj().Pkg().Path() == pkgAistream && nt.Obj().Name() == "DeltaData" {
						ghi("DeltaData literal")
					}
				}
			case *ast.SelectorExpr:
				s := info.Selections[x]
				if s == nil {
					return true
				}
				m, ok := s.Obj().(*types.Func)
				if !ok || m.Pkg() == nil || m.Pkg().Path() != pkgAistream {
					return true
				}
				recv := m.Type().(*types.Signature).Recv()
				if recv == nil {
					return true
				}
				t := recv.Type()
				if p, ok := t.(*types.Pointer); ok {
					t = p.Elem()
				}
				nt, ok := t.(*types.Named)
				if !ok {
					return true
				}
				switch name := nt.Obj().Name() + "." + m.Name(); name {
				case "Writer.Ghi", "Stream.Append", "Stream.AppendBatch":
					ghi(name)
				}
			}
			return true
		})
	}
	return out
}

func TestMoiDuongGhiLuongQuaCuaSo(t *testing.T) {
	g := load(t)
	found := ghiLuong(g)
	if len(found["const Delta"]) == 0 || len(found["Writer.Ghi"]) == 0 || len(found["DeltaData literal"]) == 0 {
		t.Fatalf("the scan found no Delta kind, Writer.Ghi or DeltaData at all (%v): chatassist's own are some, so it is broken", found)
	}
	for what, callers := range found {
		allowed, known := ghiLuongChoPhep[what]
		if !known {
			t.Fatalf("scan produced an unknown kind %q", what)
		}
		for _, c := range callers {
			if _, ok := allowed[c]; !ok {
				t.Errorf("%s outside the job stream's own methods: %s", what, c)
			}
		}
	}
	// Canary: held to an empty allowlist the same scan is red, so the
	// allowlist above is what lets the real writers through.
	red := 0
	for what, callers := range found {
		if len(ghiLuongChoPhep[what]) > 0 {
			red += len(callers)
		}
	}
	if red == 0 {
		t.Fatal("nothing would be refused with an empty allowlist: the scan sees nothing")
	}
	t.Logf("stream writes outside aistream: %v", found)
}

// Design 02 §9: no log call in aistream or jobs takes answer text. Any
// argument of a log/slog call there -- a key, a value, a field -- whose name
// or string reads text, delta, chunk or payload is red, except inside len():
// a size may be logged. DeltaData logs only its size (LogValue), which
// aistream's own test holds.
func TestLogAistreamJobsKhongNhanChu(t *testing.T) {
	g := load(t)
	cam := regexp.MustCompile(`(?i)text|delta|chunk|payload`)
	var bad []string
	calls := 0
	for f, decl := range g.decl {
		if f.Pkg() == nil || (f.Pkg().Path() != pkgAistream && f.Pkg().Path() != module+"/internal/jobs") {
			continue
		}
		info := g.info[f]
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			var fn *types.Func
			if s := info.Selections[sel]; s != nil {
				fn, _ = s.Obj().(*types.Func)
			} else {
				fn, _ = info.Uses[sel.Sel].(*types.Func)
			}
			if fn == nil || fn.Pkg() == nil || (fn.Pkg().Path() != "log/slog" && fn.Pkg().Path() != "log") {
				return true
			}
			calls++
			for _, a := range call.Args {
				ast.Inspect(a, func(m ast.Node) bool {
					switch x := m.(type) {
					case *ast.CallExpr:
						// len(text) is a size, which may be logged.
						if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "len" {
							return false
						}
					case *ast.Ident:
						if cam.MatchString(x.Name) {
							bad = append(bad, f.FullName()+": "+x.Name)
						}
					case *ast.BasicLit:
						if s, err := strconv.Unquote(x.Value); err == nil && cam.MatchString(s) {
							bad = append(bad, f.FullName()+": "+s)
						}
					}
					return true
				})
			}
			return true
		})
	}
	for _, b := range bad {
		t.Errorf("a log call in aistream/jobs takes answer text: %s", b)
	}
	if calls == 0 {
		t.Fatal("no log call found in aistream or jobs: jobs logs, so the scan is broken")
	}
	t.Logf("%d log calls read", calls)
}
