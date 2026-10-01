package aigate

import (
	"go/ast"
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

// Nếp's memory is never reachable from the group assistant (ADR-0041;
// research milvus.md §5: the partition key is not a security boundary):
// no function the group engine can reach touches the memory vector store
// (vectordb.SoTriNho, Milvus.TriNho) or names a nep_ table or collection in
// its strings. And the hybrid retriever, which the group path is about to
// call, never reaches the memory store either.

const (
	pkgVectordb = module + "/internal/vectordb"
	pkgHybrid   = module + "/internal/hybrid"
)

// nepCollection is the memory collection's alias or a physical version of
// it named in a string.
var nepCollection = regexp.MustCompile(`\bnep_memories(?:__v[0-9]+)?\b`)

func memoryViolations(c closure) []string {
	var out []string
	for name := range c.funcs {
		if strings.HasPrefix(name, "(*"+pkgVectordb+".SoTriNho).") || name == "(*"+pkgVectordb+".Milvus).TriNho" {
			out = append(out, "reaches "+name)
		}
	}
	// A nep_ table in SQL (tables() reads FROM/JOIN/INTO/UPDATE), and the
	// memory collection by name. Other nep_ strings (metric labels such as
	// nep_khong_cham_tien) are not stores.
	for name, qs := range tables(c.strings) {
		if strings.HasPrefix(name, "nep_") {
			out = append(out, "SQL on "+name+": "+strconv.Quote(qs[0][:min(len(qs[0]), 80)]))
		}
	}
	for _, s := range c.strings {
		if m := nepCollection.FindString(s); m != "" {
			out = append(out, "names the memory collection "+m)
		}
	}
	sort.Strings(out)
	return out
}

// groupRoots are the group bot's handlers and its job -- the same list
// TestGroupNeverReachesMemoryStores walks (doc_gate_test.go). Functions of
// package chatassist that serve both bots (the job dispatcher, the worker)
// reach Nếp's memory through the engine's ports by design; the group's own
// entry points must not.
func groupRoots(g *graph) []*types.Func {
	var roots []*types.Func
	for _, name := range []string{"capabilities", "create", "list", "get", "retry", "cancel", "promote", "promotion",
		"draftCreate", "draftGet", "draftPatch", "draftDiscard", "chuanBiNhom", "processNhomEngine"} {
		if f, ok := g.byName["(*"+pkgChat+".Handler)."+name]; ok {
			roots = append(roots, f)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].FullName() < roots[j].FullName() })
	return roots
}

func pkgRoots(g *graph, pkg string) []*types.Func {
	var roots []*types.Func
	for _, f := range g.byName {
		if p := f.Pkg(); p != nil && p.Path() == pkg {
			roots = append(roots, f)
		}
	}
	return roots
}

func TestGroupPathNeverReachesMemory(t *testing.T) {
	g := load(t)
	roots := groupRoots(g)
	if len(roots) != 14 {
		t.Fatalf("%d of the 14 group roots found; the load is incomplete", len(roots))
	}
	if v := memoryViolations(g.reach(roots...)); len(v) != 0 {
		t.Fatalf("the group engine can reach Nếp's memory: %v", v)
	}
	hy := pkgRoots(g, pkgHybrid)
	if len(hy) < 5 {
		t.Fatalf("only %d hybrid functions", len(hy))
	}
	c := g.reach(hy...)
	if !c.funcs["(*"+pkgVectordb+".Milvus).Tim"] {
		t.Fatal("the hybrid closure never reaches Milvus.Tim; the walk is broken")
	}
	if v := memoryViolations(c); len(v) != 0 {
		t.Fatalf("the hybrid retriever can reach the memory store: %v", v)
	}
}

// Canaries: the same roots plus the memory search go red; a nep_ table in a
// string goes red; the identity above stays green.
func TestGroupMemoryGateGoesRed(t *testing.T) {
	g := load(t)
	withMem := g.reach(append(groupRoots(g), g.root(t, "(*"+pkgVectordb+".SoTriNho).Tim"))...)
	if v := memoryViolations(withMem); len(v) == 0 {
		t.Fatal("a group closure reaching the memory search stayed green")
	}
	withAlias := g.reach(append(groupRoots(g), g.root(t, "("+pkgVectordb+".Kho).Alias"))...)
	if v := memoryViolations(withAlias); len(v) == 0 {
		t.Fatal("a group closure reaching the memory alias name stayed green")
	}
	injected := closure{funcs: map[string]bool{}, strings: []string{"SELECT cau FROM nep_su_that WHERE nguoi=$1"}}
	if v := memoryViolations(injected); len(v) != 1 {
		t.Fatalf("a nep_ table in SQL went unseen: %v", v)
	}
}

// One writer per table: place_enrichments -- the attributes the index
// carries and the retrieval re-check reads (thuoctinh only reads it) -- is
// written only by the ingest (internal/rag/nap), nhung_cache only by
// internal/nhungcache.
var writeTo = regexp.MustCompile(`(?i)\b(?:insert\s+into|update|delete\s+from|truncate)\s+(place_enrichments|nhung_cache)\b`)

func TestRetrievalTablesHaveOneWriter(t *testing.T) {
	owner := map[string]string{"place_enrichments": "../rag/nap/", "nhung_cache": "../nhungcache/"}
	seen := map[string]int{}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			for _, m := range writeTo.FindAllStringSubmatch(s, -1) {
				table := strings.ToLower(m[1])
				seen[table]++
				if !strings.HasPrefix(filepath.ToSlash(path), owner[table]) {
					t.Errorf("%s writes %s; its one writer is %s", path, table, owner[table])
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen["place_enrichments"] == 0 || seen["nhung_cache"] == 0 {
		t.Fatalf("no writer found at all (%v): the scan is broken", seen)
	}
	for _, w := range []string{"INSERT INTO place_enrichments(place_id) VALUES($1)", "UPDATE place_enrichments SET review=$1"} {
		if writeTo.FindString(w) == "" {
			t.Fatalf("canary: a write was not recognised: %s", w)
		}
	}
	if writeTo.FindString("SELECT output FROM place_enrichments WHERE place_id=$1") != "" {
		t.Fatal("canary: a read counted as a write")
	}
}
