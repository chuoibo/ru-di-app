package rag

import (
	"context"
	"time"
)

// BiaWeb is what one DongBoBiaWeb pass changed.
type BiaWeb struct {
	// Them and Go are `web_closed` tombstones written and lifted.
	Them, Go int
	// BoQua: the retrieval schema or place_facts is not installed yet, so
	// nothing was read or written.
	BoQua bool
}

// DongBoBiaWeb makes the `web_closed` tombstones exactly the fed places the
// web says are permanently closed (place_facts.con_hoat_dong =
// 'dong_vinh_vien', written by rudi-ingest from vnlocal web-facts@2) while
// that fact is unexpired at `at`. The `places` row stays; search stops
// showing it, on every path, because every retrieval reads rag_tombstones.
//
// Any other reason already on a place is left alone: a person's `closed` or
// `takedown` outranks the web, and a build's `unsafe` or `source_deleted`
// is the build's to lift; a later pass tombstones the place once that one
// is gone. Only `web_closed` is lifted here, and only here.
func DongBoBiaWeb(ctx context.Context, q Querier, at time.Time) (BiaWeb, error) {
	var out BiaWeb
	ok, err := Installed(ctx, q)
	if err != nil {
		return out, err
	}
	var facts bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('place_facts') IS NOT NULL`).Scan(&facts); err != nil {
		return out, err
	}
	if !ok || !facts {
		out.BoQua = true
		return out, nil
	}
	tag, err := q.Exec(ctx, `
		INSERT INTO rag_tombstones(corpus, doc_id, reason)
		SELECT 'place', f.place_id, 'web_closed'
		  FROM place_facts f JOIN places p ON p.id = f.place_id
		 WHERE f.con_hoat_dong = 'dong_vinh_vien' AND f.het_han_at > $1
		ON CONFLICT (corpus, doc_id) DO NOTHING`, at)
	if err != nil {
		return out, err
	}
	out.Them = int(tag.RowsAffected())
	tag, err = q.Exec(ctx, `
		DELETE FROM rag_tombstones t
		 WHERE t.corpus = 'place' AND t.reason = 'web_closed'
		   AND NOT EXISTS (SELECT 1 FROM place_facts f
		                    WHERE f.place_id = t.doc_id AND f.con_hoat_dong = 'dong_vinh_vien'
		                      AND f.het_han_at > $1)`, at)
	if err != nil {
		return out, err
	}
	out.Go = int(tag.RowsAffected())
	return out, nil
}
