package aigate

import (
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const pkgRag = "mobile/services/core/internal/rag"

// ragAllowed is the retrieval index's allowlist (design 04 §8.1), one root
// for the whole package: every function of internal/rag and its
// subpackages, called or passed as a value, followed into every package it
// reaches.
//
// Reason for each entry: rag is the only writer of its own tables (the six
// rag_* tables, the migrations table included), and it indexes the place
// catalogue, so it reads `places` (the rows it snapshots and the live row it
// checks every hit against) and `destinations` (ResolveDestination's closed
// list of names and boxes). Nothing else: not `messages`, not a money table,
// not `person_interests` or any per-person taste, not `saved_places`,
// `posts`, `pair_shared_constraints` or any `nep_*` table. A group's taste
// reaches a retrieval only as an argument the caller computed.
var ragAllowed = map[string]bool{
	"rag_schema_migrations": true, "rag_index_versions": true, "rag_docs": true, "rag_chunks": true,
	"rag_tombstones": true, "rag_query_log": true,
	"places": true, "destinations": true,
}

// ragViolations lists the tables SQL-looking strings name outside the
// allowlist, sorted.
func ragViolations(strs []string) []string {
	var out []string
	for _, name := range sortedKeys(ragTables(strs)) {
		if !ragAllowed[name] {
			out = append(out, name)
		}
	}
	return out
}

// ragTables is tables() taught three things rag's SQL does that name no
// table, each excluded narrowly and each with a canary below:
//   - a CTE declared in the same statement (`WITH loc AS (`, `, ts AS (`):
//     its name is not a table; the tables inside its body are still read,
//     and a schema-qualified name never matches a CTE;
//   - a set-returning function after FROM or JOIN (`FROM unnest(...)`): a
//     name followed by "(" there is a call. After INTO the "(" is a column
//     list and the name is a table (the canary with `expenses` holds this);
//   - `ON CONFLICT ... DO UPDATE SET`, where `set` follows `update`.
//
// Every other gate keeps the plain tables().
var (
	cteName     = regexp.MustCompile(`(?i)(?:\bwith|,)\s+([a-z_][a-z0-9_]*)\s+as\s*\(`)
	doUpdateSet = regexp.MustCompile(`(?i)\bdo\s+update\s+set\b`)
	sqlTableKw  = regexp.MustCompile(`(?i)\b(from|join|update|into)\s+([a-z_][a-z0-9_.]*)`)
)

func ragTables(strs []string) map[string][]string {
	out := map[string][]string{}
	for _, s := range strs {
		if !sqlLike.MatchString(s) {
			continue
		}
		scan := doUpdateSet.ReplaceAllString(s, "do nothing")
		ctes := map[string]bool{}
		for _, m := range cteName.FindAllStringSubmatch(scan, -1) {
			ctes[strings.ToLower(m[1])] = true
		}
		for _, loc := range sqlTableKw.FindAllStringSubmatchIndex(scan, -1) {
			keyword := strings.ToLower(scan[loc[2]:loc[3]])
			name := strings.ToLower(scan[loc[4]:loc[5]])
			call := (keyword == "from" || keyword == "join") && strings.HasPrefix(strings.TrimLeft(scan[loc[5]:], " \t\n"), "(")
			if ctes[name] || call {
				continue
			}
			out[name] = append(out[name], s)
		}
	}
	return out
}

func ragRoots(g *graph) []*types.Func {
	var roots []*types.Func
	for _, f := range g.byName {
		pkg := f.Pkg()
		if pkg != nil && (pkg.Path() == pkgRag || strings.HasPrefix(pkg.Path(), pkgRag+"/")) {
			roots = append(roots, f)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].FullName() < roots[j].FullName() })
	return roots
}

func TestRagReadsOnlyTheCatalogueAcrossPackages(t *testing.T) {
	g := load(t)
	roots := ragRoots(g)
	if len(roots) < 30 {
		t.Fatalf("only %d functions in internal/rag; the load is incomplete", len(roots))
	}
	c := g.reach(roots...)
	for _, must := range []string{
		"(" + pkgRag + ".Kho).Retrieve",
		"(" + pkgRag + ".Kho).DanhSachNgan",
		pkgRag + ".Build",
		pkgRag + ".Evaluate",
		pkgRag + ".Promote",
		pkgRag + ".Rollback",
		// The reads it makes through the repository are inside the walk.
		"(mobile/services/core/internal/repo.Repository).ListPlaces",
		"(mobile/services/core/internal/repo.Repository).PlacesByID",
		"(mobile/services/core/internal/repo.Repository).ListDestinations",
	} {
		if !c.funcs[must] {
			t.Fatalf("the rag closure never reaches %s; the walk is broken", must)
		}
	}
	// `destinations` is read by repo.ListDestinations, whose SQL is built
	// from a column constant and a " FROM destinations" literal that carries
	// no SELECT of its own, so no string names it; the reach check above
	// holds that read instead.
	used := ragTables(c.strings)
	for _, need := range []string{"places", "rag_docs", "rag_chunks", "rag_tombstones", "rag_index_versions"} {
		if _, ok := used[need]; !ok {
			t.Fatalf("no SQL naming %s found on rag's path; the extraction has slipped (tables %v)", need, sortedKeys(used))
		}
	}
	for _, name := range ragViolations(c.strings) {
		t.Errorf("rag's path reaches table %s: %q", name, used[name][0])
	}
	for name := range c.funcs {
		for _, forbidden := range []string{"service.GroupTaste", "internal/chatassist.", "internal/aiharness.", "internal/brain."} {
			if strings.Contains(name, forbidden) {
				t.Errorf("rag's path reaches %s", name)
			}
		}
	}
	t.Logf("rag closure: %d functions, tables %v", len(c.funcs), sortedKeys(used))
}

// Canaries: the rag gate goes red when rag's path reads messages -- as SQL in
// its own strings, and through a function of another package that reads
// them -- and stays green on its own closure.
func TestRagGateGoesRedOnAReadOfMessages(t *testing.T) {
	g := load(t)
	c := g.reach(ragRoots(g)...)
	if v := ragViolations(c.strings); len(v) != 0 {
		t.Fatalf("identity: the real closure is red: %v", v)
	}
	injected := append(append([]string{}, c.strings...), "SELECT m.id, m.body FROM messages m WHERE m.context_id = $1")
	if v := ragViolations(injected); len(v) != 1 || v[0] != "messages" {
		t.Fatalf("a read of messages on rag's path went unseen: %v", v)
	}
	// Structural: the same roots plus one chat function that reads messages.
	withChat := g.reach(append(ragRoots(g), g.root(t, pkgChat+".kiemTrigger"))...)
	if v := ragViolations(withChat.strings); !contains(v, "messages") {
		t.Fatalf("a function reading messages joined rag's closure and the gate stayed green: %v", v)
	}
	for _, other := range []string{"SELECT tags FROM person_interests WHERE person_id=$1", "SELECT * FROM nep_su_that", "INSERT INTO expenses(id) VALUES($1)"} {
		if len(ragViolations([]string{other})) == 0 {
			t.Errorf("not caught: %s", other)
		}
	}
	// The three exclusions of ragTables stay narrow.
	for q, want := range map[string]string{
		"WITH loc AS (SELECT d.doc_id FROM rag_docs d) SELECT * FROM loc":                                                     "rag_docs",
		"WITH x AS (SELECT body FROM messages) SELECT * FROM x":                                                               "messages",
		"WITH messages AS (SELECT 1) SELECT m.body FROM public.messages m":                                                    "public.messages",
		"SELECT k FROM unnest($1::text[]) k JOIN messages m ON m.id = k":                                                      "messages",
		"INSERT INTO rag_tombstones(doc_id) VALUES($1) ON CONFLICT (doc_id) DO UPDATE SET reason=$2":                          "rag_tombstones",
		"INSERT INTO rag_tombstones(doc_id) VALUES($1) ON CONFLICT (doc_id) DO UPDATE SET reason=(SELECT body FROM messages)": "messages",
	} {
		got := sortedKeys(ragTables([]string{q}))
		if !contains(got, want) {
			t.Errorf("%q: tables %v, want %s among them", q, got, want)
		}
		if want == "rag_docs" && len(got) != 1 {
			t.Errorf("%q: a CTE name was taken for a table: %v", q, got)
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// rag imports no engine, chat or brain package (design 04 §8.1): no import
// cycle with the engine that will call it, and no way to the chat room or a
// model from inside retrieval. Only the leaf aiharness/llm may come later,
// so that a model call it makes is counted.
func TestRagImportsNoEngineChatOrBrain(t *testing.T) {
	fset := token.NewFileSet()
	files := 0
	err := filepath.WalkDir("../rag", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files++
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range []string{"/internal/chatassist", "/internal/brain", "/internal/chatv2", "/internal/chatlegacychange", "/internal/aiharness"} {
				if strings.Contains(p, bad) && p != module+"/internal/aiharness/llm" {
					t.Errorf("%s imports %s", path, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 8 {
		t.Fatalf("only %d rag files scanned", files)
	}
}
