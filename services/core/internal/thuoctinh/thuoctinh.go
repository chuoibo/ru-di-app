// Package thuoctinh is the retrieval re-check's reader: for the place ids a
// Milvus search returned, it reads the live rows from PostgreSQL -- the
// source of truth the vector index is a copy of -- and derives each place's
// hard-constraint attributes exactly the way the ingest does, from the same
// table the ingest writes:
//
//   - the `places` row (destination, price, hours; closed world: an id with
//     no row is absent);
//   - its enrichment in place_enrichments, applied by rag/nap.ApDung against
//     the row's current source hash (so a stale, rejected, quarantined or
//     unreviewed enrichment moves the place only toward exclusion, and an
//     allergen list counts as certain only once a person reviewed it);
//   - its tombstone in rag_tombstones, if any.
//
// It writes nothing: rag/nap is the only writer of place_enrichments and of
// the index. The hybrid retriever checks every hit against Doc's answer, so
// a stale index can lose a result but never show one that breaks a hard
// constraint (docs/architecture/03 §8.4).
package thuoctinh

import (
	"context"
	"slices"

	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/vectordb"
)

// Querier is a pool or a transaction.
type Querier = repo.Querier

// Hang is one live place as the re-check reads it: the constraint
// attributes, and the evidence fields an answer may quote.
type Hang struct {
	ThuocTinh vectordb.ThuocTinh
	Ten       string
	Loai      string
	DiaChi    string
	GiaMinVND *int64
	GiaMaxVND *int64
	// Gio is the live opening-hours text ("" when none). GioRo and GiaRo
	// say whether hours and price are known: the evidence carries
	// «chưa rõ» for an unknown one no hard constraint asked about.
	Gio   string
	GioRo bool
	GiaRo bool
}

// ChuaRo are the evidence flags of the attributes Hang does not know:
// "gio_chua_ro" for unknown hours, "gia_chua_ro" for an unknown price. A
// place with an unknown attribute under a hard constraint on it never gets
// this far (it is filtered out); without the constraint it stays, and the
// answer says the attribute is not known.
func (h Hang) ChuaRo() []string {
	var out []string
	if !h.GioRo {
		out = append(out, "gio_chua_ro")
	}
	if !h.GiaRo {
		out = append(out, "gia_chua_ro")
	}
	return out
}

// DocTu is Doc bound to q, the form the hybrid retriever takes.
func DocTu(q Querier) func(ctx context.Context, ids []string) (map[string]Hang, error) {
	return func(ctx context.Context, ids []string) (map[string]Hang, error) { return Doc(ctx, q, ids) }
}

// Doc reads the live rows of ids. A place whose naming fields SafeDeep
// refuses, or that carries a tombstone, reads as removed; a place with no
// usable enrichment reads as allergens unknown, no diet: it passes no
// allergy or diet constraint.
func Doc(ctx context.Context, q Querier, ids []string) (map[string]Hang, error) {
	out := map[string]Hang{}
	if len(ids) == 0 {
		return out, nil
	}
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	places, err := repo.Repository{Q: q}.PlacesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	enr, err := nap.DocLamGiau(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	dm, err := nap.DocDanhMuc(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	bia := map[string]bool{}
	rows, err := q.Query(ctx, `SELECT doc_id FROM rag_tombstones WHERE corpus = 'place' AND doc_id = ANY($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		bia[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, p := range places {
		h, bo := nap.DungHoSo(p)
		t := nap.ApDung(enr[p.ID], h.NguonHash)
		tt := vectordb.ThuocTinh{DiemDen: p.DestinationID, AnKieng: slices.Clone(t.AnKieng),
			GiaMinVND: vectordb.GiaKhongRo, GoBo: bo || bia[p.ID]}
		if t.DiUngRo {
			tt.DiUng = slices.Clone(t.DiUng)
		} else {
			tt.DiUng = []string{vectordb.KhongRo}
		}
		// The same rule as the index (napkho.ThuocTinh): unclassified is
		// exactly [khong_ro], out under a category filter.
		if d := dm[p.ID]; len(d) > 0 {
			tt.DanhMuc = slices.Clone(d)
		} else {
			tt.DanhMuc = []string{vectordb.KhongRo}
		}
		// The index's reading (nap.DoanQuan, napkho): the maximum, or the
		// minimum when only one figure is known; unknown without a minimum.
		tt.GiaMaxVND = vectordb.GiaKhongRo
		if p.PriceMinVND != nil {
			tt.GiaMinVND = *p.PriceMinVND
			tt.GiaMaxVND = *p.PriceMinVND
			if p.PriceMaxVND != nil {
				tt.GiaMaxVND = *p.PriceMaxVND
			}
		}
		if h.Lich != nil {
			tt.OSlots = nap.MoO(*h.Lich)
		}
		r := Hang{ThuocTinh: tt, Ten: p.Name, Loai: p.Category, GiaMinVND: p.PriceMinVND, GiaMaxVND: p.PriceMaxVND,
			GioRo: h.Lich != nil, GiaRo: p.PriceMinVND != nil}
		if p.Address != nil {
			r.DiaChi = *p.Address
		}
		if p.OpenHours != nil {
			r.Gio = *p.OpenHours
		}
		out[p.ID] = r
	}
	return out, nil
}
