package rag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/repo"
)

// Errors of the version lifecycle.
var (
	ErrTrangThai     = errors.New("rag: the version is not in the state this step needs")
	ErrKhongCoActive = errors.New("rag: no active version")
	ErrKhongCoCha    = errors.New("rag: the active version has no version to roll back to")
	ErrLyDo          = errors.New("rag: unknown tombstone reason")
)

// DanhGia is what the evaluation gate measured on one version.
type DanhGia struct {
	Docs   int `json:"docs"`
	Chunks int `json:"chunks"`
	// Thieu are live, safe, untombstoned places the version lacks; Thua are
	// documents whose place is gone (a query drops them on the second check).
	Thieu int `json:"thieu"`
	Thua  int `json:"thua"`
	// ThamDo is how many hard-filter probes ran; ViPham how many (probe,
	// place) pairs the index let through that the live row breaks. The gate
	// needs 0: violation is absolute (design 04 §8.3).
	ThamDo int `json:"tham_do"`
	ViPham int `json:"vi_pham"`
	// LoaiNham are pairs the index filtered out that the live row would
	// pass: lost recall, reported, not a failure.
	LoaiNham int  `json:"loai_nham"`
	Dat      bool `json:"dat"`
}

// Evaluate runs the gate on a built version and moves it to `evaluated` when
// it passes, `failed` when it does not. The gate holds on any catalogue, not
// only the golden fixture: every document is accounted for against live
// `places`, and every hard filter (each allergen, each diet, five budget
// ceilings, four times on each day of the week) is run through the version's
// own SQL filter in every destination and checked place by place against
// the live row. Relevance -- recall, nDCG, MRR -- is measured on the golden
// set in the tests, not here.
func Evaluate(ctx context.Context, db Beginner, version int64) (DanhGia, error) {
	var g DanhGia
	tx, err := db.Begin(ctx)
	if err != nil {
		return g, err
	}
	defer tx.Rollback(ctx)
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM rag_index_versions WHERE id=$1 FOR UPDATE`, version).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return g, ErrTrangThai
		}
		return g, err
	}
	if state != "built" {
		return g, ErrTrangThai
	}
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM rag_docs WHERE version_id=$1), (SELECT count(*) FROM rag_chunks WHERE version_id=$1)`, version).Scan(&g.Docs, &g.Chunks); err != nil {
		return g, err
	}
	rows, err := repo.Repository{Q: tx}.ListPlaces(ctx, repo.PlaceFilter{})
	if err != nil {
		return g, err
	}
	bia, err := Kho{Q: tx}.tombstoned(ctx)
	if err != nil {
		return g, err
	}
	live := map[string]HoSo{}
	for _, row := range rows {
		if h, rep := DungHoSo(row); !rep.Bo {
			live[row.ID] = h
		}
	}
	inVersion := map[string]bool{}
	dests := map[string]bool{}
	docRows, err := tx.Query(ctx, `SELECT doc_id, destination_id FROM rag_docs WHERE version_id=$1`, version)
	if err != nil {
		return g, err
	}
	for docRows.Next() {
		var id, dest string
		if err := docRows.Scan(&id, &dest); err != nil {
			docRows.Close()
			return g, err
		}
		inVersion[id] = true
		dests[dest] = true
		if _, ok := live[id]; !ok {
			g.Thua++
		}
	}
	docRows.Close()
	if err := docRows.Err(); err != nil {
		return g, err
	}
	for id := range live {
		if !inVersion[id] && !bia[id] {
			g.Thieu++
		}
	}
	for dest := range dests {
		for _, y := range thamDo(dest) {
			g.ThamDo++
			r := rangBuocCua(y)
			passed, err := locSQL(ctx, tx, version, r)
			if err != nil {
				return g, err
			}
			for id := range passed {
				if h, ok := live[id]; ok {
					if ok, _ := r.dat(h); !ok {
						g.ViPham++
					}
				}
			}
			for id, h := range live {
				if h.DiemDen != dest || !inVersion[id] || bia[id] || passed[id] {
					continue
				}
				if ok, _ := r.dat(h); ok {
					g.LoaiNham++
				}
			}
		}
	}
	g.Dat = g.Docs > 0 && g.Thieu == 0 && g.ViPham == 0
	next := "failed"
	if g.Dat {
		next = "evaluated"
	}
	body, _ := json.Marshal(g)
	if _, err = tx.Exec(ctx, `UPDATE rag_index_versions SET state=$2, eval=$3 WHERE id=$1`, version, next, body); err != nil {
		return g, err
	}
	return g, tx.Commit(ctx)
}

// thamDo are the gate's probes for one destination: one per hard filter
// value, never combined, so a violation names its filter.
func thamDo(dest string) []YeuCau {
	var out []YeuCau
	for _, a := range tuvung.DiUng.IDs() {
		out = append(out, YeuCau{DiemDen: dest, DiUng: []string{a}})
	}
	for _, d := range tuvung.AnKieng.IDs() {
		out = append(out, YeuCau{DiemDen: dest, AnKieng: []string{d}})
	}
	for _, b := range []int64{30_000, 60_000, 100_000, 200_000, 500_000} {
		b := b
		out = append(out, YeuCau{DiemDen: dest, NganSach: &b})
	}
	for day := 0; day < 7; day++ {
		for _, m := range []int{7 * 60, 12 * 60, 19*60 + 30, 23*60 + 30} {
			at := day*24*60 + m
			out = append(out, YeuCau{DiemDen: dest, Luc: &at})
		}
	}
	return out
}

func locSQL(ctx context.Context, q Querier, version int64, r rangBuoc) (map[string]bool, error) {
	luc, khung, ngan := r.thamSo()
	rows, err := q.Query(ctx, sqlLoc, version, r.diemDen, r.diUngSQL(), nonNil(r.anKieng), ngan, luc, khung)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// khoaTrangThai serialises every change of which version is active.
const khoaTrangThai = `SELECT pg_advisory_xact_lock(hashtext('rag_index_state'))`

// Promote makes an evaluated version the active one, and retires the version
// it replaces, which becomes its parent: what Rollback returns to. Both flips
// happen in one transaction under one advisory lock, so there is never a
// moment with two active versions or none.
func Promote(ctx context.Context, db Beginner, version int64) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, khoaTrangThai); err != nil {
		return err
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM rag_index_versions WHERE id=$1`, version).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTrangThai
		}
		return err
	}
	if state != "evaluated" {
		return ErrTrangThai
	}
	var old *int64
	if err = tx.QueryRow(ctx, `UPDATE rag_index_versions SET state='retired', retired_at=clock_timestamp() WHERE corpus='place' AND state='active' RETURNING id`).Scan(&old); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_index_versions SET state='active', promoted_at=clock_timestamp(), parent_id=COALESCE($2, parent_id) WHERE id=$1`, version, old); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Rollback retires the active version and makes its parent active again, in
// one transaction under the same lock. Tombstones are not touched: they
// belong to no version, so a place removed since the parent was built stays
// removed.
func Rollback(ctx context.Context, db Beginner) (from, to int64, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, khoaTrangThai); err != nil {
		return 0, 0, err
	}
	var parent *int64
	err = tx.QueryRow(ctx, `SELECT id, parent_id FROM rag_index_versions WHERE corpus='place' AND state='active'`).Scan(&from, &parent)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, ErrKhongCoActive
	}
	if err != nil {
		return 0, 0, err
	}
	if parent == nil {
		return 0, 0, ErrKhongCoCha
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM rag_index_versions WHERE id=$1`, *parent).Scan(&state); err != nil {
		return 0, 0, err
	}
	if state != "retired" {
		return 0, 0, fmt.Errorf("%w: parent %d is %s", ErrKhongCoCha, *parent, state)
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_index_versions SET state='retired', retired_at=clock_timestamp() WHERE id=$1`, from); err != nil {
		return 0, 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_index_versions SET state='active', promoted_at=clock_timestamp(), retired_at=NULL WHERE id=$1`, *parent); err != nil {
		return 0, 0, err
	}
	return from, *parent, tx.Commit(ctx)
}

// LyDoBia are the reasons a place leaves the index.
var LyDoBia = []string{"unsafe", "takedown", "closed", "source_deleted"}

// Tombstone removes a place from every version, past and future, until the
// tombstone is deleted. A reason given by hand replaces one a build wrote,
// so a takedown is never lifted by the next build.
func Tombstone(ctx context.Context, q Querier, docID, reason string) error {
	ok := false
	for _, r := range LyDoBia {
		ok = ok || r == reason
	}
	if !ok || docID == "" {
		return ErrLyDo
	}
	_, err := q.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) VALUES('place',$1,$2)
		ON CONFLICT (corpus,doc_id) DO UPDATE SET reason=EXCLUDED.reason, created_at=clock_timestamp()`, docID, reason)
	return err
}

// TrangThai is `core rag status`.
type TrangThai struct {
	Installed bool  `json:"installed"`
	Active    int64 `json:"active"`
	// Degraded: no active version, so Retrieve answers from live rows.
	Degraded      bool           `json:"degraded"`
	TheoTrangThai map[string]int `json:"theo_trang_thai"`
	Docs          int            `json:"docs"`
	Chunks        int            `json:"chunks"`
	Bia           map[string]int `json:"bia"`
}

// Status reports the index without changing it.
func Status(ctx context.Context, q Querier) (TrangThai, error) {
	s := TrangThai{TheoTrangThai: map[string]int{}, Bia: map[string]int{}}
	ok, err := Installed(ctx, q)
	if err != nil || !ok {
		s.Degraded = true
		return s, err
	}
	s.Installed = true
	if s.Active, err = (Kho{Q: q}).phienBanActive(ctx); err != nil {
		return s, err
	}
	s.Degraded = s.Active == 0
	if err = dem(ctx, q, `SELECT state, count(*) FROM rag_index_versions GROUP BY state`, s.TheoTrangThai); err != nil {
		return s, err
	}
	if err = dem(ctx, q, `SELECT reason, count(*) FROM rag_tombstones GROUP BY reason`, s.Bia); err != nil {
		return s, err
	}
	if s.Active != 0 {
		err = q.QueryRow(ctx, `SELECT (SELECT count(*) FROM rag_docs WHERE version_id=$1), (SELECT count(*) FROM rag_chunks WHERE version_id=$1)`, s.Active).Scan(&s.Docs, &s.Chunks)
	}
	return s, err
}

func dem(ctx context.Context, q Querier, sql string, into map[string]int) error {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		into[k] = n
	}
	return rows.Err()
}

// NhatKy is one row of rag_query_log: ids, enums, counts, a duration. It has
// no field for words, by construction.
type NhatKy struct {
	YDinh    string
	PhienBan int64
	NLoc     int
	NUngVien int
	Vong     int
	ChamLai  bool
	KetQua   string
	Thoi     time.Duration
	Co       []string
}

// GhiNhatKy writes one query log row. The engine calls it after a turn; the
// public search does not log.
func GhiNhatKy(ctx context.Context, q Querier, n NhatKy) error {
	var version any
	if n.PhienBan != 0 {
		version = n.PhienBan
	}
	_, err := q.Exec(ctx, `INSERT INTO rag_query_log(corpus,y_dinh,version_id,n_loc,n_ung_vien,vong,cham_lai,ket_qua,ms,co)
		VALUES('place',$1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		n.YDinh, version, n.NLoc, n.NUngVien, n.Vong, n.ChamLai, n.KetQua, int(n.Thoi/time.Millisecond), nonNil(n.Co))
	return err
}
