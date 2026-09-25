package aigate

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The AI paths promise what they read (ADR-0036 §2.3–§2.4, §4; ADR-0033 §4):
// the group engine never reads the text of a message, and Nếp reads no table
// for context at all. chatassist/khong_doc_chat_test.go and
// nep_khong_doc_test.go check those promises inside the chatassist package,
// following `h.name(` calls. That walk cannot see three ways the property
// decays, and the AI engine that is coming uses all three:
//
//   - a call into another package (a repository method, a service helper, a
//     tool in an agent registry): its SQL lives in that package's files;
//   - a method value (`h.mux.HandleFunc(pattern, h.nepCreate)`, a tool
//     registered as `functiontool.New(..., r.searchPlaces)`): no call syntax;
//   - an interface call: the concrete method is chosen at run time.
//
// This gate loads the whole module with types, builds a call graph from every
// use of a function object (calls and values alike, interface methods resolved
// to every module method that implements them), and holds the SQL in string
// literals and constants of each root's closure to that root's allowlist.

const module = "mobile/services/core"

type graph struct {
	decl   map[*types.Func]*ast.FuncDecl
	info   map[*types.Func]*types.Info
	byName map[string]*types.Func
	// Concrete module methods by name, for resolving interface calls.
	methods map[string][]*types.Func
}

var (
	loadOnce sync.Once
	loaded   *graph
	loadErr  error
)

func load(t *testing.T) *graph {
	t.Helper()
	loadOnce.Do(func() {
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
				packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
			Dir: "../..",
		}
		pkgs, err := packages.Load(cfg, module+"/...")
		if err != nil {
			loadErr = err
			return
		}
		g := &graph{decl: map[*types.Func]*ast.FuncDecl{}, info: map[*types.Func]*types.Info{},
			byName: map[string]*types.Func{}, methods: map[string][]*types.Func{}}
		for _, p := range pkgs {
			for _, e := range p.Errors {
				loadErr = e
				return
			}
			for _, f := range p.Syntax {
				for _, d := range f.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok {
						continue
					}
					obj, ok := p.TypesInfo.Defs[fd.Name].(*types.Func)
					if !ok {
						continue
					}
					g.decl[obj] = fd
					g.info[obj] = p.TypesInfo
					g.byName[obj.FullName()] = obj
					if fd.Recv != nil {
						g.methods[obj.Name()] = append(g.methods[obj.Name()], obj)
					}
				}
			}
		}
		loaded = g
	})
	if loadErr != nil {
		t.Fatalf("cannot load the module with types: %v", loadErr)
	}
	return loaded
}

func (g *graph) root(t *testing.T, name string) *types.Func {
	t.Helper()
	f, ok := g.byName[name]
	if !ok {
		t.Fatalf("root %s not found: the gate is looking at the wrong place", name)
	}
	return f
}

// implementers resolves an interface method to every module method with its
// name whose receiver implements the interface. Over-approximating errs red.
func (g *graph) implementers(m *types.Func) []*types.Func {
	sig, ok := m.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	iface, ok := sig.Recv().Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	var out []*types.Func
	for _, c := range g.methods[m.Name()] {
		recv := c.Type().(*types.Signature).Recv().Type()
		if types.Implements(recv, iface) {
			out = append(out, c)
			continue
		}
		if named, ok := recv.(*types.Named); ok && types.Implements(types.NewPointer(named), iface) {
			out = append(out, c)
		}
	}
	return out
}

type closure struct {
	funcs   map[string]bool
	strings []string
}

// reach walks every module function the roots can reach.
func (g *graph) reach(roots ...*types.Func) closure {
	out := closure{funcs: map[string]bool{}}
	seen := map[*types.Func]bool{}
	todo := append([]*types.Func(nil), roots...)
	for len(todo) > 0 {
		f := todo[len(todo)-1].Origin()
		todo = todo[:len(todo)-1]
		if seen[f] {
			continue
		}
		seen[f] = true
		decl, info := g.decl[f], g.info[f]
		if decl == nil {
			// Outside the module, or an interface method: resolve and move on.
			for _, impl := range g.implementers(f) {
				todo = append(todo, impl)
			}
			continue
		}
		out.funcs[f.FullName()] = true
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					if s, err := strconv.Unquote(x.Value); err == nil {
						out.strings = append(out.strings, s)
					}
				}
			case *ast.Ident:
				switch obj := info.Uses[x].(type) {
				case *types.Func:
					// Every function object, called or passed as a value. One
					// outside the module has no body here and is dropped, unless
					// it is an interface method some module type implements.
					todo = append(todo, obj)
				case *types.Const:
					if obj.Val().Kind() == constant.String {
						out.strings = append(out.strings, constant.StringVal(obj.Val()))
					}
				}
			}
			return true
		})
	}
	return out
}

var (
	sqlTable = regexp.MustCompile(`(?i)\b(?:from|join|update|into)\s+([a-z_][a-z0-9_.]*)`)
	sqlLike  = regexp.MustCompile(`(?i)\b(select|insert|update|delete)\b`)
	// A read of the messages table, and the text column inside it.
	readsMessages = regexp.MustCompile(`(?is)select\b[^;]*?\bfrom\s+messages\b`)
	messageText   = regexp.MustCompile(`(?i)\bbody\b`)
	// Columns Nếp has no business naming even on a table it may touch.
	nepForbiddenColumn = regexp.MustCompile(`(?i)\b(display_name|body|interests|budget\w*)\b`)
)

// tables returns the tables named by SQL-looking strings.
func tables(strs []string) map[string][]string {
	out := map[string][]string{}
	for _, s := range strs {
		if !sqlLike.MatchString(s) {
			continue
		}
		for _, m := range sqlTable.FindAllStringSubmatch(s, -1) {
			name := strings.ToLower(m[1])
			out[name] = append(out[name], s)
		}
	}
	return out
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

const (
	pkgChat = "mobile/services/core/internal/chatassist"
)

var nepRoots = []string{
	"(*" + pkgChat + ".Handler).nepCreate",
	"(*" + pkgChat + ".Handler).nepGet",
	"(*" + pkgChat + ".Handler).processNep",
}

// Nếp reads no table for context: the session, the person row it locks, and
// its own job row -- followed into every package it reaches.
func TestNepReadsNothingForContextAcrossPackages(t *testing.T) {
	g := load(t)
	var roots []*types.Func
	for _, r := range nepRoots {
		roots = append(roots, g.root(t, r))
	}
	c := g.reach(roots...)
	for _, must := range []string{
		"(*" + pkgChat + ".Handler).nepXong",
		pkgChat + ".phien",
		"(*mobile/services/core/internal/brain.Client).PostJSONContext",
	} {
		if !c.funcs[must] {
			t.Fatalf("closure never reaches %s; the walk is broken", must)
		}
	}
	allowed := map[string]bool{"chat_ai_invocations": true, "account_sessions": true, "people": true}
	used := tables(c.strings)
	if len(used) == 0 {
		t.Fatal("no SQL found on Nếp's path; the extraction has slipped")
	}
	for _, name := range sortedKeys(used) {
		if !allowed[name] {
			t.Errorf("Nếp's path reaches table %s: %q", name, used[name][0])
		}
	}
	for _, s := range c.strings {
		if sqlLike.MatchString(s) && nepForbiddenColumn.MatchString(s) {
			t.Errorf("Nếp's path names a forbidden column: %q", s)
		}
	}
	for _, forbidden := range []string{".prepare", ".roster", ".authority", ".thuocPhong", ".tacGia", ".hoiThoai", ".publish", "service.GroupTaste", "service.ModelPlaceRows"} {
		for name := range c.funcs {
			if strings.HasSuffix(name, forbidden) || strings.Contains(name, forbidden+"(") {
				t.Errorf("Nếp's path reaches %s, a function that lays room context on a question", name)
			}
		}
	}
	t.Logf("Nếp closure: %d functions, tables %v", len(c.funcs), sortedKeys(used))
}

// The group engine may touch `messages` (ownership of a shared turn, the
// author of a turn, publishing a card) but never read the text -- anywhere it
// can reach, in any package. Everything the Handler can do is a root.
func TestGroupEngineNeverReadsMessageTextAcrossPackages(t *testing.T) {
	g := load(t)
	var roots []*types.Func
	for name, f := range g.byName {
		if strings.HasPrefix(name, "(*"+pkgChat+".Handler).") || strings.HasPrefix(name, pkgChat+".") {
			roots = append(roots, f)
		}
	}
	if len(roots) < 30 {
		t.Fatalf("only %d roots in chatassist; the load is incomplete", len(roots))
	}
	c := g.reach(roots...)
	// The in-thread answer (ADR-0039) added two reads of `messages`: the
	// trigger check and publish's lock on the trigger. Both must be inside the
	// walk, or they are reads no gate looks at.
	for _, must := range []string{pkgChat + ".kiemTrigger", pkgChat + ".giuTrigger"} {
		if !c.funcs[must] {
			t.Fatalf("the group closure never reaches %s; a read of messages is outside the gate", must)
		}
	}
	reads := 0
	for _, s := range c.strings {
		for _, q := range readsMessages.FindAllString(s, -1) {
			reads++
			if messageText.MatchString(q) {
				t.Errorf("the AI engine can reach a read of message text: %q", q)
			}
		}
	}
	if reads == 0 {
		t.Fatal("no read of messages found; the ownership check reads one, so the pattern slipped")
	}
	t.Logf("group closure: %d functions, %d reads of messages", len(c.funcs), reads)
}

// Canaries: the walk must see what the in-package gates could not.
func TestTheWalkSeesMethodValuesOtherPackagesAndInterfaces(t *testing.T) {
	g := load(t)
	// Method value only: New registers nepCreate as `h.nepCreate`, never calls it.
	if c := g.reach(g.root(t, pkgChat+".New")); !c.funcs["(*"+pkgChat+".Handler).nepCreate"] {
		t.Fatal("a handler registered as a method value is invisible to the walk")
	}
	// Another package's SQL: prepare's taste comes from service/repo files.
	prep := g.reach(g.root(t, "(*"+pkgChat+".Handler).prepare"))
	used := tables(prep.strings)
	if _, ok := used["person_interests"]; !ok {
		t.Fatalf("prepare reads per-person interests through service.GroupTaste in another package, and the walk missed it; tables seen: %v", sortedKeys(used))
	}
	// And the Nếp allowlist, fed the group path, goes red.
	allowed := map[string]bool{"chat_ai_invocations": true, "account_sessions": true, "people": true}
	red := false
	for name := range used {
		if !allowed[name] {
			red = true
		}
	}
	if !red {
		t.Fatal("the Nếp allowlist found nothing wrong on prepare, which reads memberships and taste")
	}
	// Interface dispatch: the web session handler reaches its store only
	// through the Backend interface, and the store holds the session SQL.
	lookup := g.reach(g.root(t, "(*mobile/services/core/internal/websession.Handler).resume"))
	if !lookup.funcs["(mobile/services/core/internal/websession.Store).Lookup"] {
		t.Fatal("a method reached only through an interface is invisible to the walk")
	}
	if messageText.MatchString(readsMessages.FindString("SELECT id, author_id FROM messages WHERE id=$1")) {
		t.Fatal("a read of ids only was taken for a read of text")
	}
	if !messageText.MatchString(readsMessages.FindString("SELECT m.id, m.body FROM messages m WHERE m.context_id=$1")) {
		t.Fatal("a read of text went unseen")
	}
}
