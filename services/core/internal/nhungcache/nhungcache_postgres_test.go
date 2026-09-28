//go:build postgres

package nhungcache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/testdb"
)

// kho is a private schema with the cache migrated twice (idempotent).
func kho(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	var b [8]byte
	_, _ = rand.Read(b[:])
	schema := "nhung_cache_test_" + hex.EncodeToString(b[:])
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, pool); err != nil {
			t.Fatalf("migration %d: %v", i+1, err)
		}
	}
	return pool
}

type demNhung struct {
	nhung.Stub
	docs int
}

func (d *demNhung) NhungTaiLieu(ctx context.Context, docs []nhung.TaiLieuVao) ([][]float32, error) {
	d.docs += len(docs)
	return d.Stub.NhungTaiLieu(ctx, docs)
}

// A second ingest of the same public text costs the provider nothing; a new
// prompt version is a miss; the table refuses a private corpus even by hand;
// an edited migration is refused.
func TestCacheRoundTripAndBoundaries(t *testing.T) {
	ctx := context.Background()
	pool := kho(t)
	inner := &demNhung{}
	c := &CoCache{Inner: inner, DB: pool}
	docs := []nhung.TaiLieuVao{{TieuDe: "Cà Phê Dốc", NoiDung: "yên tĩnh"}, {NoiDung: "lẩu nấm"}, {NoiDung: "Đà Lạt"}}
	first, err := c.NhungCongKhai(ctx, DiaDiem, docs)
	if err != nil {
		t.Fatal(err)
	}
	docs[2].NoiDung = "Đà Lạt" // NFC twin: the same key
	again, err := c.NhungCongKhai(ctx, DiaDiem, docs)
	if err != nil {
		t.Fatal(err)
	}
	if inner.docs != 3 || c.DemTrung() != 3 || c.DemThieu() != 3 {
		t.Fatalf("provider saw %d docs, hits %d, misses %d", inner.docs, c.DemTrung(), c.DemThieu())
	}
	for i := range first {
		if nhung.Cosine(first[i], again[i]) < 0.999999 {
			t.Fatalf("cached vector %d differs", i)
		}
	}
	c2 := &CoCache{Inner: inner, DB: pool, PromptVersion: "prefix-v2"}
	if _, err := c2.NhungCongKhai(ctx, HuongDan, docs[:1]); err != nil || inner.docs != 4 {
		t.Fatalf("a new prompt version hit the old cache: %d, %v", inner.docs, err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM nhung_cache`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("%d rows, %v", n, err)
	}
	k := Khoa("m", 1, "p", "x")
	if _, err := pool.Exec(ctx, `INSERT INTO nhung_cache(khoa,kho,model,dims,prompt_version,vec) VALUES($1,'memories','m',1,'p',$2)`, k[:], []byte{0, 0, 0, 0}); err == nil {
		t.Fatal("the table accepted a private corpus")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO nhung_cache(khoa,kho,model,dims,prompt_version,vec) VALUES($1,'places','m',2,'p',$2)`, k[:], []byte{0, 0, 0, 0}); err == nil {
		t.Fatal("the table accepted a vector of the wrong length")
	}
	if _, err := pool.Exec(ctx, `UPDATE nhung_cache_schema_migrations SET digest='x' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err == nil {
		t.Fatal("an edited migration was accepted")
	}
}
