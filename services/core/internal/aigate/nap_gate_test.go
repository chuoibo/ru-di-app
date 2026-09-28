package aigate

import (
	"go/types"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The vector ingestion pipeline (internal/rag/nap) writes only rd_*
// collections: never a memory collection (nep_*, mem0's defaults) nor a
// memory table. Its SQL is held by the rag gate; its Milvus names are held
// here, on the same closure.

const (
	pkgNap    = pkgRag + "/nap"
	pkgNapKho = "mobile/services/core/internal/vectordb/napkho"
)

var tenTriNho = regexp.MustCompile(`(?i)(^|[^a-z])(nep_[a-z0-9_]*|mem0[a-z0-9_]*|memories)($|[^a-z0-9_])`)

func triNhoTrong(strs []string) []string {
	var out []string
	for _, s := range strs {
		if tenTriNho.MatchString(s) {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func TestNapKhongChamTriNho(t *testing.T) {
	g := load(t)
	c := g.reach(ragRoots(g)...)
	for _, must := range []string{
		"(" + pkgNap + ".Nap).Dung",
		"(" + pkgNap + ".Nap).Promote",
		"(" + pkgNap + ".ChiMuc).MotLuot",
		"(*" + pkgNapKho + ".Kho).Upsert",
	} {
		if !c.funcs[must] {
			t.Fatalf("the rag closure never reaches %s; the walk is broken", must)
		}
	}
	var rd int
	for _, s := range c.strings {
		if strings.HasPrefix(s, "rd_") {
			rd++
		}
	}
	if rd == 0 {
		t.Fatal("no rd_ name on the ingestion path; the extraction slipped")
	}
	if bad := triNhoTrong(c.strings); len(bad) > 0 {
		t.Fatalf("the ingestion path names memory stores: %q", bad)
	}
	// Canary: a memory collection's name on the path goes red; the identity
	// (the real strings) stays green above.
	for _, s := range []string{"nep_ky_uc", "rd_x OR nep_su_that", "mem0migrations", "memories"} {
		if len(triNhoTrong(append(append([]string(nil), c.strings...), s))) != 1 {
			t.Fatalf("an injected %q went unseen", s)
		}
	}
	for _, s := range []string{"rd_place__v1", "nephew", "nepnho"} {
		if len(triNhoTrong([]string{s})) != 0 {
			t.Fatalf("%q read as a memory store", s)
		}
	}
}

// Neither assistant's path (Nếp's roots, the group handler) reaches a
// function that writes the vector index: building, promoting, indexing,
// enrichment verdicts and every Milvus write are the ingestion's alone.
func TestTroLyKhongVietChiMucVector(t *testing.T) {
	g := load(t)
	writers := []string{
		"(" + pkgNap + ".Nap).Dung", "(" + pkgNap + ".Nap).Promote", "(" + pkgNap + ".Nap).Rollback",
		"(" + pkgNap + ".Nap).DoiChieu", "(" + pkgNap + ".ChiMuc).MotLuot", pkgNap + ".GhiLamGiau", pkgNap + ".Duyet",
		pkgNap + ".GhiHang",
		"(*" + pkgNapKho + ".Kho).Upsert", "(*" + pkgNapKho + ".Kho).XoaID",
		"(*" + pkgNapKho + ".Kho).TaoCollection", "(*" + pkgNapKho + ".Kho).DatAlias",
		"(*" + pkgNapKho + ".Kho).XoaCollection",
	}
	var nep []*types.Func
	for _, r := range nepRoots {
		nep = append(nep, g.root(t, r))
	}
	var group []*types.Func
	for name, f := range g.byName {
		if strings.HasPrefix(name, "(*"+pkgChat+".Handler).") || strings.HasPrefix(name, pkgChat+".") {
			group = append(group, f)
		}
	}
	for _, root := range []struct {
		name  string
		funcs []*types.Func
	}{{"Nếp", nep}, {"group", group}} {
		c := g.reach(root.funcs...)
		for _, w := range writers {
			if c.funcs[w] {
				t.Errorf("%s's path reaches the vector index writer %s", root.name, w)
			}
		}
	}
	// Canary: joining an ingestion function to the group's roots is seen.
	withNap := g.reach(append(group, g.root(t, "("+pkgNap+".ChiMuc).MotLuot"))...)
	if !withNap.funcs["(*"+pkgNapKho+".Kho).Upsert"] && !withNap.funcs[pkgNap+".GhiLamGiau"] {
		t.Fatal("an indexer joined to the group's roots went unseen")
	}
}
