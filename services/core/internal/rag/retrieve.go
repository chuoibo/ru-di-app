package rag

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/repo"
)

// Kho is the retrieval index over one database handle: a pool, or the
// transaction of the request that asks.
type Kho struct{ Q Querier }

// hydrateToiDa is how many fused candidates are checked a second time against
// their live rows before the best K are returned.
const hydrateToiDa = 60

// thoiHanSQL bounds each retrieval statement (design 04 §5.8).
const thoiHanSQL = "800ms"

// ErrPhienBan is a retrieval asked of a version that may not answer: one
// still building, failed, or unknown.
var ErrPhienBan = errors.New("rag: version cannot answer queries")

// Retrieve answers one retrieval. With an active index version it filters
// hard in SQL, ranks by RRF over the full-text, trigram and vocabulary lists,
// and checks each ranked candidate again against its live `places` row
// (closed world: an id no longer in `places` is dropped). Without one -- no
// schema, no active version, or the index failing -- it answers from the
// live rows of the destination with the same filters, and says Degraded.
// Hard filters are never relaxed on either path. It writes nothing.
func (k Kho) Retrieve(ctx context.Context, y YeuCau) (KetQua, error) {
	installed, err := Installed(ctx, k.Q)
	if err != nil {
		return KetQua{}, err
	}
	if installed {
		v, err := k.phienBanActive(ctx)
		if err != nil {
			return KetQua{}, err
		}
		if v != 0 {
			if kq, err := k.trongPhienBan(ctx, v, y); err == nil {
				return kq, nil
			}
			// The index failed (a timeout, a broken version): the live rows
			// still answer, and the caller sees Degraded.
		}
	}
	return k.song(ctx, installed, y)
}

// RetrieveVersion answers from one named version: an active one, or a built
// or evaluated one being evaluated before promotion. A building, failed or
// retired version cannot answer.
func (k Kho) RetrieveVersion(ctx context.Context, version int64, y YeuCau) (KetQua, error) {
	var state string
	err := k.Q.QueryRow(ctx, `SELECT state FROM rag_index_versions WHERE id=$1`, version).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return KetQua{}, ErrPhienBan
	}
	if err != nil {
		return KetQua{}, err
	}
	if state != "built" && state != "evaluated" && state != "active" {
		return KetQua{}, ErrPhienBan
	}
	return k.trongPhienBan(ctx, version, y)
}

func (k Kho) phienBanActive(ctx context.Context) (int64, error) {
	var v int64
	err := k.Q.QueryRow(ctx, `SELECT id FROM rag_index_versions WHERE corpus='place' AND state='active'`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

// song is the live-row path.
func (k Kho) song(ctx context.Context, installed bool, y YeuCau) (KetQua, error) {
	rows, bia, err := k.hangSong(ctx, installed, y.DiemDen)
	if err != nil {
		return KetQua{}, err
	}
	return xepSong(rows, bia, y), nil
}

// hangSong reads the live places of a destination ("" for all) and the
// tombstoned ids, which hold on this path too: a place taken down is down
// whether or not an index version answers.
func (k Kho) hangSong(ctx context.Context, installed bool, diemDen string) ([]repo.Place, map[string]bool, error) {
	filter := repo.PlaceFilter{}
	if diemDen != "" {
		filter.DestinationID = &diemDen
	}
	rows, err := repo.Repository{Q: k.Q}.ListPlaces(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	bia := map[string]bool{}
	if installed {
		ids, err := k.tombstoned(ctx)
		if err != nil {
			return nil, nil, err
		}
		bia = ids
	}
	return rows, bia, nil
}

func (k Kho) tombstoned(ctx context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := k.Q.Query(ctx, `SELECT doc_id FROM rag_tombstones WHERE corpus='place'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// trongPhienBan runs the index path inside a savepoint (or a transaction of
// its own on a pool) with a statement timeout, so a slow or failing index
// statement costs the caller's transaction nothing.
func (k Kho) trongPhienBan(ctx context.Context, version int64, y YeuCau) (kq KetQua, err error) {
	b, ok := k.Q.(Beginner)
	if !ok {
		return k.chiMuc(ctx, k.Q, version, y)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return KetQua{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	var old string
	if err = tx.QueryRow(ctx, `SELECT current_setting('statement_timeout')`).Scan(&old); err != nil {
		return KetQua{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, thoiHanSQL); err != nil {
		return KetQua{}, err
	}
	if kq, err = k.chiMuc(ctx, tx, version, y); err != nil {
		return KetQua{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, old); err != nil {
		return KetQua{}, err
	}
	return kq, tx.Commit(ctx)
}

// chiMuc is the index path proper: SQL candidates, fusion, the second check
// on live rows.
func (k Kho) chiMuc(ctx context.Context, q Querier, version int64, y YeuCau) (KetQua, error) {
	r := rangBuocCua(y)
	lists, nLoc, err := ungVienSQL(ctx, q, version, y, r)
	if err != nil {
		return KetQua{}, err
	}
	fused := fuse(lists...)
	kq := KetQua{PhienBan: version, NLoc: nLoc}
	if len(fused) > hydrateToiDa {
		fused = fused[:hydrateToiDa]
	}
	kq.NUngVien = len(fused)
	ids := make([]string, len(fused))
	for i, h := range fused {
		ids[i] = h.ID
	}
	live, err := repo.Repository{Q: q}.PlacesByID(ctx, ids)
	if err != nil {
		return KetQua{}, err
	}
	byID := map[string]repo.Place{}
	for _, p := range live {
		byID[p.ID] = p
	}
	var hits []Hit
	for _, h := range fused {
		row, ok := byID[h.ID]
		if !ok {
			continue // gone from the catalogue: the world is closed
		}
		prof, report := DungHoSo(row)
		if report.Bo {
			continue
		}
		pass, co := r.dat(prof)
		if !pass {
			continue
		}
		h.Co = co
		hits = append(hits, h)
	}
	hits = r.xepChuaRo(hits)
	if len(hits) > y.k() {
		hits = hits[:y.k()]
	}
	kq.Quan = hits
	kq.Co = append(coTruyVan(y), coHits(hits)...)
	return kq, nil
}

// Trigram similarity below which a name is not a candidate.
const nguongTrigram = 0.3

// locDieuKien is the index's hard filter: a document of the version that
// passes it may be ranked, and no other. Tombstones are anti-joined here,
// for every version alike.
const locDieuKien = `d.version_id = $1
     AND ($2::text = '' OR d.destination_id = $2::text)
     AND NOT (d.di_ung_nguon && $3::text[])
     AND d.an_kieng_nguon @> $4::text[]
     AND ($5::bigint IS NULL OR d.price_min_vnd IS NULL OR d.price_min_vnd <= $5::bigint)
     AND ($6::integer IS NULL OR d.open_week IS NULL OR d.open_week @> $6::integer)
     AND ($7::text IS NULL OR d.open_week IS NULL OR d.open_week && $7::text::int4multirange)
     AND d.canonical_id IS NULL
     AND NOT EXISTS (SELECT 1 FROM rag_tombstones t WHERE t.corpus = 'place' AND t.doc_id = d.doc_id)`

// sqlLoc is the hard filter alone: the ids a query may rank.
const sqlLoc = `SELECT d.doc_id FROM rag_docs d WHERE ` + locDieuKien

// sqlUngVien is the first check and the three ranked lists. `loc` is every
// document that passes locDieuKien; each list ranks inside it with a dense
// rank, cut in (score, id) order.
const sqlUngVien = `
WITH loc AS (
  SELECT d.doc_id, d.name_fold, d.category, d.khi_chat FROM rag_docs d WHERE ` + locDieuKien + `
), ts AS (
  SELECT c.doc_id, max(ts_rank_cd(c.tsv, to_tsquery('simple', $8::text))) AS s
    FROM rag_chunks c JOIN loc ON loc.doc_id = c.doc_id
   WHERE c.version_id = $1 AND $8::text <> '' AND c.tsv @@ to_tsquery('simple', $8::text)
   GROUP BY c.doc_id
), tg AS (
  SELECT loc.doc_id, GREATEST(word_similarity(loc.name_fold, $9::text), word_similarity($9::text, loc.name_fold)) AS s
    FROM loc WHERE $9::text <> ''
), mem AS (
  SELECT loc.doc_id,
         2 * (SELECT count(*) FROM unnest(loc.khi_chat) k WHERE k = ANY($11::text[]))
           + CASE WHEN loc.category = ANY($10::text[]) THEN 1 ELSE 0 END AS s
    FROM loc
)
SELECT 'ts', doc_id, dense_rank() OVER (ORDER BY s DESC)::integer
  FROM (SELECT doc_id, s FROM ts ORDER BY s DESC, doc_id COLLATE "C" LIMIT 50) a
UNION ALL
SELECT 'tg', doc_id, dense_rank() OVER (ORDER BY s DESC)::integer
  FROM (SELECT doc_id, s FROM tg WHERE s >= $12::float8 ORDER BY s DESC, doc_id COLLATE "C" LIMIT 50) b
UNION ALL
SELECT 'mem', doc_id, dense_rank() OVER (ORDER BY s DESC)::integer
  FROM (SELECT doc_id, s FROM mem WHERE s > 0 ORDER BY s DESC, doc_id COLLATE "C" LIMIT 200) c
UNION ALL
SELECT 'n', '', (SELECT count(*) FROM loc)::integer`

// ungVienSQL runs sqlUngVien: the three ranked lists and the count that
// passed the filters.
func ungVienSQL(ctx context.Context, q Querier, version int64, y YeuCau, r rangBuoc) ([][]hang, int, error) {
	luc, khung, ngan := r.thamSo()
	_, folded := gapTruyVan(y.Cau)
	loaiCho, khiChat := y.LoaiCho, y.KhiChat
	if loaiCho == nil {
		loaiCho = []string{}
	}
	if khiChat == nil {
		khiChat = []string{}
	}
	rows, err := q.Query(ctx, sqlUngVien, version, r.diemDen, r.diUngSQL(), nonNil(r.anKieng), ngan, luc, khung,
		tsQuery(thuatTruyVan(y)), folded, loaiCho, khiChat, nguongTrigram)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	lists := map[string][]hang{}
	n := 0
	for rows.Next() {
		var list, id string
		var rank int
		if err := rows.Scan(&list, &id, &rank); err != nil {
			return nil, 0, err
		}
		if list == "n" {
			n = rank
			continue
		}
		lists[list] = append(lists[list], hang{ID: id, Hang: rank})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rag: candidates: %w", err)
	}
	return [][]hang{lists["ts"], lists["tg"], lists["mem"]}, n, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
