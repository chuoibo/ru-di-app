//go:build postgres

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

// The operator's path end to end, through the binary's own entry point:
// migrate-rag, build, eval, promote, status, tombstone, rollback -- in a
// private schema, on invented places.
func TestRagCLILifecycle(t *testing.T) {
	ctx := context.Background()
	base := testdb.Pool(t)
	tx, err := base.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('rag_test_pg_trgm'))`); err == nil {
		_, err = tx.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	_, _ = rand.Read(suffix[:])
	schema := fmt.Sprintf("core_rag_cli_%x", suffix)
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, table := range []string{"destinations", "places"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO destinations(id,name,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES ('d-hoi-an','Hội An',15.88,108.33,15.84,108.28,15.92,108.38,10)`,
		`INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,price_min_vnd,price_max_vnd,open_hours,source) VALUES
		 ('ha-cli-mot','d-hoi-an','Quán Thử Một','cafe','["cà phê"]',15.88,108.33,20000,40000,'07:00 – 22:00','seed'),
		 ('ha-cli-hai','d-hoi-an','Quán Thử Hai','quan-an-local','["cơm gà"]',15.88,108.33,40000,80000,'10:00 – 21:00','seed')`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	raw := strings.TrimSpace(os.Getenv("CORE_TEST_DATABASE_URL"))
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	databaseURL := raw + sep + "search_path=" + url.QueryEscape(schema+",public")
	env := func(k string) string {
		if k == "MOBILE_DATABASE_URL" {
			return databaseURL
		}
		return ""
	}
	core := func(want int, args ...string) map[string]any {
		t.Helper()
		var out, errs bytes.Buffer
		if code := run(args, env, &out, &errs); code != want {
			t.Fatalf("%v: exit %d, want %d: %s %s", args, code, want, out.String(), errs.String())
		}
		got := map[string]any{}
		_ = json.Unmarshal(out.Bytes(), &got)
		return got
	}
	core(1, "rag", "build") // no schema yet: refused, with the command to run
	if s := core(0, "rag", "status"); s["installed"] != false || s["degraded"] != true {
		t.Fatalf("status before migrating: %v", s)
	}
	core(0, "migrate-rag")
	core(0, "migrate-rag")
	b := core(0, "rag", "build")
	if b["docs"] != float64(2) || b["chunks"] != float64(2) {
		t.Fatalf("build: %v", b)
	}
	v := fmt.Sprint(int64(b["phien_ban"].(float64)))
	core(1, "rag", "promote", v) // not evaluated yet
	if g := core(0, "rag", "eval", v); g["dat"] != true || g["vi_pham"] != float64(0) {
		t.Fatalf("eval: %v", g)
	}
	core(0, "rag", "promote", v)
	if s := core(0, "rag", "status"); s["active"] != b["phien_ban"] || s["degraded"] != false || s["docs"] != float64(2) {
		t.Fatalf("status: %v", s)
	}
	core(0, "rag", "tombstone", "ha-cli-hai", "takedown")
	core(2, "rag", "tombstone", "ha-cli-hai", "vi_sao_khong")
	core(2, "rag", "tombstone", "ha-cli-hai", "unsafe")
	b2 := core(0, "rag", "build")
	core(0, "rag", "eval", fmt.Sprint(int64(b2["phien_ban"].(float64))))
	core(0, "rag", "promote", fmt.Sprint(int64(b2["phien_ban"].(float64))))
	if r := core(0, "rag", "rollback"); r["active"] != b["phien_ban"] {
		t.Fatalf("rollback: %v", r)
	}
	if s := core(0, "rag", "status"); s["bia"].(map[string]any)["takedown"] != float64(1) {
		t.Fatalf("status after rollback: %v", s)
	}
	// The takedown is lifted by hand, and only by hand: once.
	if u := core(0, "rag", "untombstone", "ha-cli-hai"); u["lifted"] != "takedown" {
		t.Fatalf("untombstone: %v", u)
	}
	core(1, "rag", "untombstone", "ha-cli-hai")
	if s := core(0, "rag", "status"); s["bia"].(map[string]any)["takedown"] != nil {
		t.Fatalf("status after untombstone: %v", s)
	}
}
