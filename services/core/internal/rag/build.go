package rag

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/repo"
)

// BaoCaoDung is what one build did, in counts only (design 04 §6.1 step 11).
type BaoCaoDung struct {
	PhienBan int64 `json:"phien_ban"`
	// Docs and Chunks are what the version holds.
	Docs   int `json:"docs"`
	Chunks int `json:"chunks"`
	// BoKhongAnToan are rows SafeDeep dropped: tombstoned `unsafe`.
	BoKhongAnToan int `json:"bo_khong_an_toan"`
	// CachLy are fields SafeDeep quarantined across all rows.
	CachLy int `json:"cach_ly"`
	// DaXoa are ids of the active version no longer in `places`: tombstoned
	// `source_deleted`.
	DaXoa int `json:"da_xoa"`
	// GoBia are `unsafe`/`source_deleted` tombstones lifted because the row
	// is back and safe. `takedown` and `closed` are only ever lifted by hand.
	GoBia int `json:"go_bia"`
}

// Build snapshots the live catalogue into a new index version: `building`
// while it runs, `built` at the end, `failed` if it errs. Every row goes
// through SafeDeep; a row it drops is tombstoned `unsafe`, a quarantined
// field never reaches a chunk. The version's rows are inserted, ANALYZEd and
// marked built in one transaction, so no query ever sees half a version, and
// no query path reads a version that is not active or named for evaluation.
func Build(ctx context.Context, db Beginner) (BaoCaoDung, error) {
	var report BaoCaoDung
	tx, err := db.Begin(ctx)
	if err != nil {
		return report, err
	}
	var parent *int64
	if err = tx.QueryRow(ctx, `SELECT id FROM rag_index_versions WHERE corpus='place' AND state='active'`).Scan(&parent); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return report, err
	}
	if err = tx.QueryRow(ctx, `INSERT INTO rag_index_versions(corpus,state,chunker,parent_id) VALUES('place','building',$1,$2) RETURNING id`, Chunker, parent).Scan(&report.PhienBan); err != nil {
		_ = tx.Rollback(ctx)
		return report, err
	}
	if err = tx.Commit(ctx); err != nil {
		return report, err
	}
	if err = dung(ctx, db, &report, parent); err != nil {
		if tx2, e := db.Begin(ctx); e == nil {
			_, _ = tx2.Exec(ctx, `UPDATE rag_index_versions SET state='failed' WHERE id=$1 AND state='building'`, report.PhienBan)
			_ = tx2.Commit(ctx)
		}
		return report, err
	}
	return report, nil
}

func dung(ctx context.Context, db Beginner, report *BaoCaoDung, parent *int64) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := repo.Repository{Q: tx}.ListPlaces(ctx, repo.PlaceFilter{})
	if err != nil {
		return err
	}
	v := report.PhienBan
	batch := &pgx.Batch{}
	digest := sha256.New()
	var unsafe, safeIDs []string
	live := map[string]bool{}
	for _, row := range rows {
		live[row.ID] = true
		h, rep := DungHoSo(row)
		if rep.Bo {
			unsafe = append(unsafe, row.ID)
			continue
		}
		report.CachLy += len(rep.CachLy)
		safeIDs = append(safeIDs, row.ID)
		var week any
		if h.Lich != nil {
			week = h.Lich.SQL()
		}
		batch.Queue(`INSERT INTO rag_docs(version_id,doc_id,destination_id,category,price_min_vnd,price_max_vnd,open_week,
			di_ung_nguon,an_kieng_nguon,khi_chat,name_fold,lat,lng,license)
			VALUES($1,$2,$3,$4,$5,$6,$7::text::int4multirange,$8,$9,$10,$11,$12,$13,$14)`,
			v, h.ID, h.DiemDen, h.LoaiCho, h.GiaMin, h.GiaMax, week,
			nonNil(h.DiUng), nonNil(h.AnKieng), nonNil(h.KhiChat), h.TenGap, h.Lat, h.Lng, h.License)
		report.Docs++
		for _, d := range h.Doan {
			batch.Queue(`INSERT INTO rag_chunks(version_id,chunk_id,doc_id,facet,body,search_text,content_hash) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				v, d.ChunkID, h.ID, d.Facet, d.Body, d.ChuTim, d.Hash[:])
			fmt.Fprintf(digest, "%s\x00%x\x00", d.ChunkID, d.Hash)
			report.Chunks++
		}
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	report.BoKhongAnToan = len(unsafe)
	if len(unsafe) > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) SELECT 'place', unnest($1::text[]), 'unsafe' ON CONFLICT (corpus,doc_id) DO NOTHING`, unsafe); err != nil {
			return err
		}
	}
	if parent != nil {
		var gone []string
		prev, err := tx.Query(ctx, `SELECT doc_id FROM rag_docs WHERE version_id=$1`, *parent)
		if err != nil {
			return err
		}
		for prev.Next() {
			var id string
			if err := prev.Scan(&id); err != nil {
				prev.Close()
				return err
			}
			if !live[id] {
				gone = append(gone, id)
			}
		}
		prev.Close()
		if err := prev.Err(); err != nil {
			return err
		}
		sort.Strings(gone)
		if len(gone) > 0 {
			tag, err := tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) SELECT 'place', unnest($1::text[]), 'source_deleted' ON CONFLICT (corpus,doc_id) DO NOTHING`, gone)
			if err != nil {
				return err
			}
			report.DaXoa = int(tag.RowsAffected())
		}
	}
	if len(safeIDs) > 0 {
		tag, err := tx.Exec(ctx, `DELETE FROM rag_tombstones WHERE corpus='place' AND reason IN ('unsafe','source_deleted') AND doc_id = ANY($1::text[])`, safeIDs)
		if err != nil {
			return err
		}
		report.GoBia = int(tag.RowsAffected())
	}
	if _, err = tx.Exec(ctx, `ANALYZE rag_docs`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `ANALYZE rag_chunks`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_index_versions SET state='built', source_digest=$2 WHERE id=$1 AND state='building'`, v, digest.Sum(nil)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
