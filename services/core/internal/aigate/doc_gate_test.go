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

	aimetrics "mobile/services/core/internal/aiharness/metrics"
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
	// varInit is every package-level variable's initializer: a registry
	// built there (a tool table, a handler map) holds functions no call
	// syntax names, and a walk that skipped it would not see them.
	varInit map[*types.Var]khoiTao
}

type khoiTao struct {
	expr ast.Expr
	info *types.Info
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
			byName: map[string]*types.Func{}, methods: map[string][]*types.Func{}, varInit: map[*types.Var]khoiTao{}}
		for _, p := range pkgs {
			for _, e := range p.Errors {
				loadErr = e
				return
			}
			for _, f := range p.Syntax {
				for _, d := range f.Decls {
					if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.VAR {
						for _, spec := range gd.Specs {
							vs := spec.(*ast.ValueSpec)
							for i, name := range vs.Names {
								v, ok := p.TypesInfo.Defs[name].(*types.Var)
								if !ok || i >= len(vs.Values) {
									continue
								}
								g.varInit[v] = khoiTao{expr: vs.Values[i], info: p.TypesInfo}
							}
						}
					}
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
	seenVar := map[*types.Var]bool{}
	todo := append([]*types.Func(nil), roots...)
	var visit func(n ast.Node, info *types.Info)
	visit = func(root ast.Node, info *types.Info) {
		ast.Inspect(root, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					if s, err := strconv.Unquote(x.Value); err == nil {
						out.strings = append(out.strings, s)
					}
				}
			case *ast.BinaryExpr:
				// A query built by concatenation («SELECT " + cols + " FROM
				// outings …») is read whole too: its verb and its table
				// may sit in different literals.
				if x.Op == token.ADD {
					if s, ok := noiChuoi(x, info); ok {
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
				case *types.Var:
					// A package-level registry: walk its initializer once.
					if k, ok := g.varInit[obj]; ok && !seenVar[obj] {
						seenVar[obj] = true
						visit(k.expr, k.info)
					}
				}
			}
			return true
		})
	}
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
		visit(decl, info)
	}
	return out
}

// noiChuoi joins the string leaves of a concatenation (literals and string
// constants), a space standing for any other operand; ok is false when no
// leaf is a string.
func noiChuoi(e *ast.BinaryExpr, info *types.Info) (string, bool) {
	var parts []string
	co := false
	var di func(ast.Expr)
	di = func(x ast.Expr) {
		switch v := x.(type) {
		case *ast.BinaryExpr:
			if v.Op == token.ADD {
				di(v.X)
				di(v.Y)
				return
			}
		case *ast.ParenExpr:
			di(v.X)
			return
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				if s, err := strconv.Unquote(v.Value); err == nil {
					parts, co = append(parts, s), true
					return
				}
			}
		case *ast.Ident:
			if c, ok := info.Uses[v].(*types.Const); ok && c.Val().Kind() == constant.String {
				parts, co = append(parts, constant.StringVal(c.Val())), true
				return
			}
		}
		parts = append(parts, " ")
	}
	di(e)
	return strings.Join(parts, ""), co
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
	// The Nếp stream (slice 11): its authorization and the job row.
	"(*" + pkgChat + ".Handler).nepEvents",
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
		// The Go engine (MOBILE_AI_ENGINE_NEP=go) and its metrics writer are
		// on the path too; a walk that stops at the engine proves nothing
		// about it.
		"(*mobile/services/core/internal/aiharness.Engine).Run",
		"mobile/services/core/internal/aiharness/metrics.Ghi",
	} {
		if !c.funcs[must] {
			t.Fatalf("closure never reaches %s; the walk is broken", must)
		}
	}
	allowed := map[string]bool{"chat_ai_invocations": true, "account_sessions": true, "people": true}
	for name := range nepCongCuDoc {
		allowed[name] = true
	}
	// The tools' read ports bring the catalogue index's queries (CTEs and
	// unnest()), read with the rag gate's narrow exclusions.
	used := ragTables(c.strings)
	if len(used) == 0 {
		t.Fatal("no SQL found on Nếp's path; the extraction has slipped")
	}
	// Canaries: every allowlisted tool table is reached (an entry no path
	// needs is a stale permission), the walk sees the tool registry built in
	// a package-level variable (tools.congCus) and a query built by
	// concatenation (aidoc's own outings).
	for name := range nepCongCuDoc {
		if _, ok := used[name]; !ok {
			t.Errorf("allowlist entry %s is reached by nothing on Nếp's path: drop it", name)
		}
	}
	for _, must := range []string{"mobile/services/core/internal/aiharness/tools.chayChuyenCuaToi", "(mobile/services/core/internal/aidoc.Doc).ChuyenDiSapToi"} {
		if !c.funcs[must] {
			t.Fatalf("closure never reaches %s: a registry in a package-level variable is invisible to the walk", must)
		}
	}
	concat := false
	for _, q := range used["outings"] {
		concat = concat || (strings.Contains(q, "SELECT") && strings.Contains(q, "memberships"))
	}
	if !concat {
		t.Fatal("the outings query is built by concatenation and the walk did not read it whole")
	}
	for _, name := range sortedKeys(used) {
		if nepWriteOnly[name] {
			for _, q := range used[name] {
				if v := writeOnlyViolation(name, q); v != "" {
					t.Errorf("Nếp's path %s: %q", v, q)
				}
			}
			continue
		}
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

// nepCongCuDoc are the tables Nếp's tools may READ, each with why (the
// owner's rule of 2026-09-25 wires the model-chosen tools into Nếp's path;
// contract docs/architecture/03-ai-engine-hop-dong.md §8, permission table
// tools/testdata/quyen.golden.json). None is room context: no message, no
// roster, no taste, no budget. Every read runs in a READ ONLY transaction
// (internal/aidoc) and only when the MODEL called the tool; the walk is
// static and cannot see the permission table, so the group tools' tables
// (outings, memberships) show up here too -- the permission test in
// aiharness/tools (TestTuChoiTenVaQuyen) is what keeps Nếp from calling
// group_snapshot or list_group_outings.
var nepCongCuDoc = map[string]string{
	// The shared catalogue: search_places, get_place, the retrieval path.
	"places": "catalogue rows, the same for every person; get_place and the retriever's cards",
	// Its lexical index (rag.Retrieve behind aidoc.Lexical).
	"rag_docs":              "catalogue index: hard filters in SQL, never a person's data",
	"rag_chunks":            "catalogue index: BM25 ranking of the model's query",
	"rag_index_versions":    "catalogue index: which version is active",
	"rag_tombstones":        "catalogue index: removed places are never shown",
	"rag_schema_migrations": "catalogue index: whether the index is installed",
	// The hybrid retriever's re-check (aidoc.ThuocTinhSong → thuoctinh.Doc)
	// reads a place's enrichment the way the ingest applies it.
	"place_enrichments": "catalogue enrichment: a place's closed-id allergens, diets and review verdict, the same for every person; the hybrid re-check's truth",
	// list_destinations, nearest_area, the router's closed destination list.
	"destinations": "the destinations the app covers, the same for every person",
	// my_upcoming_outings: the ASKING person's own upcoming outings, scoped by
	// the job's person id (never a model argument), titles, dates and
	// headcount only; and the group tools no Nếp turn may call.
	"outings":     "my_upcoming_outings: the asker's own outings (title, dates, headcount), model-called only",
	"memberships": "my_upcoming_outings: which groups the asker is an active member of, model-called only",
	// The asker's OWN long-term memory (internal/nepnho, ADR-0043 draft):
	// every statement is scoped by the job's person id, never a model
	// argument; none of these tables holds a word (the words live in the
	// memory sidecar); recall and personalization return nothing while the
	// person's toggle is off. Reached by the memory tools and by
	// personalization (aiharness.HoSo), never by the group
	// (TestGroupNeverReachesMemoryStores).
	"nep_cai_dat": "the asker's memory toggle and consent version: recall, personalization and writes are gated on it",
	"nep_su_that": "receipts of the asker's own facts (mem0 id, kind, times; no words): only a fact with a live receipt is recalled",
	"nep_quen":    "keyed hashes of facts the asker forgot: a forgotten fact is not written again",
	"nep_su_kien": "counts by kind of the asker's own typed app events for what_you_remember: kinds and ids, no words",
	"nep_xoa":     "the asker's deletion ledger: forget_fact runs its saga (ids, closed codes, counts)",
}

// nepWriteOnly are the tables Nếp's path may write and never read, each named
// here with why it cannot carry context.
//
// ai_turn_metrics (aiharness/metrics, ADR-0037 §2.8): one row per turn the Go
// engine ran, written after the job ended. It is not context: the path only
// INSERTs into it, so nothing in it can reach a model; and it cannot hold
// words, so nothing of a question or an answer can be kept there either --
// every column is an id, a number, a boolean, a timestamp, or text held by a
// CHECK to a closed list (TestAiTurnMetricsHoldsNoFreeText below, and the
// live catalogue check in aiharness/metrics).
var nepWriteOnly = map[string]bool{"ai_turn_metrics": true}

var insertOnly = regexp.MustCompile(`(?is)^\s*insert\s+into\s+([a-z_][a-z0-9_.]*)\s*\(`)

// writeOnlyViolation says what is wrong with q naming a write-only table, or
// "" when q is an INSERT into it and names no other table.
func writeOnlyViolation(table, q string) string {
	m := insertOnly.FindStringSubmatch(q)
	if m == nil || !strings.EqualFold(m[1], table) {
		return "does more than INSERT into " + table
	}
	for _, other := range sqlTable.FindAllStringSubmatch(q, -1) {
		if !strings.EqualFold(other[1], table) {
			return "reads " + other[1] + " while writing " + table
		}
	}
	return ""
}

// columnMayHoldText says whether a column definition of CREATE TABLE could
// store words: anything but an id, a number, a boolean or a timestamp, unless
// a CHECK holds it to a closed list or a hex shape.
var (
	colPlain  = regexp.MustCompile(`^\w+ (uuid|smallint|integer|bigint|boolean|timestamptz)\b`)
	colClosed = regexp.MustCompile(`^\w+ (text|char\(\d+\))( NOT NULL)? CHECK \(\w+ (IN \('[a-z0-9_.]*'(,'[a-z0-9_.]*')*\)|~ '\^\[0-9a-f\]\{\d+\}\$')\)$`)
)

func columnMayHoldText(def string) bool {
	return !colPlain.MatchString(def) && !colClosed.MatchString(def)
}

// The table Nếp writes holds no free text, read from the migration the binary
// embeds.
func TestAiTurnMetricsHoldsNoFreeText(t *testing.T) {
	sql := regexp.MustCompile(`(?m)^\s*--.*$`).ReplaceAllString(aimetrics.SchemaSQL(), "")
	body := regexp.MustCompile(`(?s)CREATE TABLE ai_turn_metrics \((.*?)\);`).FindStringSubmatch(sql)
	if body == nil {
		t.Fatal("cannot find CREATE TABLE ai_turn_metrics in the embedded migration")
	}
	n := 0
	for _, line := range strings.Split(body[1], "\n") {
		def := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
		if def == "" || strings.HasPrefix(def, "PRIMARY KEY") {
			continue
		}
		n++
		if columnMayHoldText(def) {
			t.Errorf("ai_turn_metrics column can hold free text: %s", def)
		}
	}
	if n < 20 {
		t.Fatalf("read only %d columns; the parse slipped", n)
	}
	if regexp.MustCompile(`(?i)\b(jsonb?|bytea|varchar|character varying)\b`).MatchString(sql) {
		t.Error("the migration declares a type that can hold free text")
	}
}

// Canaries for the two rules above: a read of the write-only table, a join
// through it, and a column that could carry words are all red.
func TestWriteOnlyAndNoFreeTextCanRed(t *testing.T) {
	if writeOnlyViolation("ai_turn_metrics", "INSERT INTO ai_turn_metrics(invocation_id,bot) VALUES($1,$2)") != "" {
		t.Fatal("the metrics INSERT itself is refused")
	}
	for _, q := range []string{
		"SELECT prompt_version FROM ai_turn_metrics WHERE invocation_id=$1",
		"INSERT INTO ai_turn_metrics(invocation_id) SELECT id FROM messages",
		"UPDATE ai_turn_metrics SET code=$2",
	} {
		if writeOnlyViolation("ai_turn_metrics", q) == "" {
			t.Errorf("not caught: %s", q)
		}
	}
	for _, def := range []string{"note text", "detail text NOT NULL", "tool_args jsonb", "cau text CHECK (char_length(cau) <= 160)", "bot text NOT NULL CHECK (bot IN ('nep','Tối nay đi đâu'))"} {
		if !columnMayHoldText(def) {
			t.Errorf("not caught: %s", def)
		}
	}
	if columnMayHoldText("bot text NOT NULL CHECK (bot IN ('nep','nhom'))") || columnMayHoldText("lan_thu smallint NOT NULL") {
		t.Fatal("a closed column was taken for free text")
	}
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
	reads, chuDaLuu := 0, 0
	for _, s := range c.strings {
		for _, q := range readsMessages.FindAllString(s, -1) {
			reads++
			if messageText.MatchString(q) {
				// The one named read (review of slices 9/11, finding 2.3): the
				// split draft's stored text of the messages the caller shared,
				// by their ids, in this room, word for word.
				if s == chuDaLuuGhim {
					chuDaLuu++
					continue
				}
				t.Errorf("the AI engine can reach a read of message text: %q", q)
			}
		}
	}
	if reads == 0 {
		t.Fatal("no read of messages found; the ownership check reads one, so the pattern slipped")
	}
	if chuDaLuu != 1 || !c.funcs[pkgChat+".chuDaLuu"] {
		t.Errorf("the split draft's pinned read of stored text: seen %d times, reached through chuDaLuu %v", chuDaLuu, c.funcs[pkgChat+".chuDaLuu"])
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

// The memory tools are Nếp's alone (scope me): nothing the group bot's
// handlers and jobs can reach runs one, in any package, while Nếp's job
// does reach them (the canary: the walk sees the tool registry's function
// values). Defence in depth behind the permission table, before the group
// bot moves onto the engine (privacy review 7).
func TestGroupNeverReachesMemoryTools(t *testing.T) {
	g := load(t)
	const tools = "mobile/services/core/internal/aiharness/tools"
	memory := []string{tools + ".chayNho", tools + ".chayGhiNho", tools + ".chayQuen", tools + ".chayNhoGi",
		"(*" + tools + ".BoiCanh).CamKet"}
	var nep []*types.Func
	for _, r := range nepRoots {
		nep = append(nep, g.root(t, r))
	}
	cn := g.reach(nep...)
	for _, m := range memory {
		if !cn.funcs[m] {
			t.Fatalf("Nếp's closure never reaches %s: the walk is blind to the registry", m)
		}
	}
	var group []*types.Func
	for _, name := range []string{"capabilities", "create", "list", "get", "retry", "cancel", "promote", "promotion",
		"draftCreate", "draftGet", "draftPatch", "draftDiscard", "prepare", "processChiaBill", "processNhomEngine"} {
		group = append(group, g.root(t, "(*"+pkgChat+".Handler)."+name))
	}
	cg := g.reach(group...)
	if !cg.funcs["(*"+pkgChat+".Handler).prepare"] || len(cg.funcs) < 50 {
		t.Fatalf("the group closure is too small (%d functions)", len(cg.funcs))
	}
	for _, m := range memory {
		if cg.funcs[m] {
			t.Errorf("the group bot can reach %s", m)
		}
	}
}

// The group bot reaches no memory store: no function of internal/nepnho
// (the long-term ledger, its sidecar client, personalization), none of the
// short-term store's Nếp keys (aictx.PhienNep, PhienLuot, XoaNguoi), from
// any handler or job the group can run. Nếp's job reaches every one of them
// (the canary: the walk follows the engine's HoSo and NganHanLuot ports and
// the tools' TriNho port through their interfaces to the adapters).
func TestGroupNeverReachesMemoryStores(t *testing.T) {
	g := load(t)
	const (
		nepnho = "mobile/services/core/internal/nepnho"
		aictx  = "mobile/services/core/internal/aictx"
	)
	nepKeys := []string{aictx + ".PhienNep", "(*" + aictx + ".Kho).PhienLuot", "(*" + aictx + ".Kho).XoaNguoi"}
	var nep []*types.Func
	for _, r := range nepRoots {
		nep = append(nep, g.root(t, r))
	}
	cn := g.reach(nep...)
	for _, must := range append([]string{
		"(*" + nepnho + ".Kho).HoSoNep",
		"(*" + nepnho + ".Kho).Nho",
		"(*" + nepnho + ".Kho).Quen",
		"(*" + nepnho + ".KhachHTTP).Tim",
		"(*" + nepnho + ".KhachHTTP).Xoa",
		"(*" + aictx + ".Kho).Them",
	}, nepKeys[:2]...) {
		if !cn.funcs[must] {
			t.Fatalf("Nếp's closure never reaches %s: the walk is blind to the memory ports", must)
		}
	}
	var group []*types.Func
	for _, name := range []string{"capabilities", "create", "list", "get", "retry", "cancel", "promote", "promotion",
		"draftCreate", "draftGet", "draftPatch", "draftDiscard", "prepare", "processChiaBill", "processNhomEngine"} {
		group = append(group, g.root(t, "(*"+pkgChat+".Handler)."+name))
	}
	cg := g.reach(group...)
	if len(cg.funcs) < 50 {
		t.Fatalf("the group closure is too small (%d functions)", len(cg.funcs))
	}
	bad := nepnhoTrong(g, cg, nepnho)
	for _, k := range nepKeys {
		if cg.funcs[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("the group bot can reach %s", b)
	}
	// Since slice 9 the group's worker root runs the engine
	// (processNhomEngine → Engine.RunNhom): the walk must see the group's
	// tools, its split reading and its short-term buffer, or the check above
	// proves nothing about the engine.
	for _, must := range []string{
		"(*mobile/services/core/internal/aiharness.Engine).RunNhom",
		"mobile/services/core/internal/aiharness/tools.chayAnhNhom",
		"mobile/services/core/internal/aiharness/chiabill.Goi",
		"(*" + aictx + ".Kho).PhienLuotNhom",
	} {
		if !cg.funcs[must] {
			t.Errorf("the group closure never reaches %s: the walk is blind to the group's engine path", must)
		}
	}
	// Canary: the same roots with Nếp's personalization go red.
	withHoSo := g.reach(append(group, g.root(t, "(*"+nepnho+".Kho).HoSoNep"))...)
	if len(nepnhoTrong(g, withHoSo, nepnho)) == 0 {
		t.Fatal("a group closure reaching Nếp's personalization stayed green")
	}
}

// nepnhoTrong is every function of package nepnho in closure c, except an
// Error method reached only as the builtin error interface's that calls
// nothing of the module: the walk resolves every err.Error() to every module
// error type, and such a method formats a string and touches no store.
func nepnhoTrong(g *graph, c closure, nepnho string) []string {
	var bad []string
	for name := range c.funcs {
		if !strings.Contains(name, nepnho+".") && !strings.Contains(name, nepnho+")") {
			continue
		}
		if strings.HasSuffix(name, ").Error") {
			if f, ok := g.byName[name]; ok && len(g.reach(f).funcs) == 1 {
				continue
			}
		}
		bad = append(bad, name)
	}
	return bad
}

// chuDaLuuGhim is the one read of message text the group closure may reach,
// pinned word for word (chatassist.cauDocChuDaLuu): only this room, only the
// ids the caller shared, only a live text message with a confirmed author.
const chuDaLuuGhim = `SELECT id::text, body FROM messages WHERE context_id=$1 AND id = ANY($2::uuid[]) AND author_id IS NOT NULL AND deleted_at IS NULL AND kind='text' AND body IS NOT NULL`
