// Package thuoctinhtest builds, for real-PostgreSQL tests, the tables the
// re-check reads -- a private schema with copies of places and
// destinations, the outbox, the lexical index and the ingestion schema
// migrated into it (naptest.Pool) -- and writes places and their
// enrichments through the ingest's own writer (rag/nap), so a test sees the
// same rows production does and never writes the shared public tables.
package thuoctinhtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/repo"
)

// Kho is a private schema at the ingestion schema's head.
func Kho(t *testing.T) *pgxpool.Pool { t.Helper(); return naptest.Pool(t) }

// ThemNoi inserts an invented place with the columns the re-check reads:
// destination, price (nil: unknown), opening hours (nil: unknown), and its
// text.
func ThemNoi(t *testing.T, pool *pgxpool.Pool, id, diemDen, moTa string, giaMin *int64, gio *string) {
	t.Helper()
	p := repo.Place{ID: id, DestinationID: diemDen, Name: "Quán " + id, Category: "quan-an-local", Kinds: []string{},
		Traits: []string{}, Lat: new(11.94), Lng: new(108.44), PriceMinVND: giaMin, OpenHours: gio, Description: &moTa, Source: "seed"}
	naptest.Chen(t, pool, p)
}

// LamGiau writes the place's enrichment as the ingest stores it, keyed by
// the live row's source hash: diUng nil is «khong_ro» (allergens unknown),
// otherwise the model's certain list. duyet approves it by the version the
// review queue shows, as a person would.
func LamGiau(t *testing.T, pool *pgxpool.Pool, id string, diUng, anKieng []string, duyet bool) {
	t.Helper()
	ctx := context.Background()
	p, err := repo.Repository{Q: pool}.GetPlace(ctx, id)
	if err != nil || p == nil {
		t.Fatalf("%s: %v", id, err)
	}
	h, _ := nap.DungHoSo(*p)
	k := nap.KetQuaLamGiau{DiUng: diUng, DiUngRo: diUng != nil, AnKieng: anKieng, AnKiengRo: true, TinCay: nap.TinCayCao}
	lg := nap.LamGiau{PlaceID: id, NguonHash: h.NguonHash, Model: "stub", PromptVersion: nap.PromptVersion(), KetQua: k,
		CanDuyet: true, Review: nap.ReviewAuto}
	if err := nap.GhiLamGiau(ctx, pool, []nap.LamGiau{lg}); err != nil {
		t.Fatal(err)
	}
	if !duyet {
		return
	}
	ban, err := nap.BanCua(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := nap.Duyet(ctx, pool, id, ban, nap.ReviewReviewed); err != nil {
		t.Fatal(err)
	}
}
