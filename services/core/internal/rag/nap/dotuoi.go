package nap

import (
	"context"
	"sort"
	"sync/atomic"
	"time"
)

// The index's freshness SLO (plan P3, owner decision 2026-10-01): what the
// place index serves may lag Postgres by at most 60 s, allergens and
// closures included. The rag-indexer reads these numbers every few seconds
// and answers /slo with them; `core rag v-status` prints them.

// SLOLechGiay is the lag the SLO allows; SLOKeoDai how long a breach must
// last before /slo turns unhealthy (one slow pass is not an outage).
const (
	SLOLechGiay = 60.0
	SLOKeoDai   = 2 * time.Minute
	// SLOIngestGiay: the ingest daemon rounds at least every 20 s; no
	// successful round for this long is a stalled feed.
	SLOIngestGiay = 180.0
)

// DoTuoi is the place index's freshness: counts and ages, no ids, no
// content.
type DoTuoi struct {
	// LechGiay is the age of the oldest change the index does not hold yet:
	// rows due now (or deferred after a failure) at priority 0 or above.
	// Rows waiting only for their vector hold current attributes and do not
	// count; nor do background re-checks.
	LechGiay float64 `json:"lech_giay"`
	CanLam   int     `json:"can_lam"`
	// ChoNhung are rows waiting for the batch door, ChoNhungGiay the age of
	// the oldest wait; Hoan rows deferred for the online budget or a
	// failing door; NenCho background re-checks.
	ChoNhung     int     `json:"cho_nhung"`
	ChoNhungGiay float64 `json:"cho_nhung_giay"`
	Hoan         int     `json:"hoan"`
	NenCho       int     `json:"nen_cho"`
	DLQ          int     `json:"dlq"`
	// Ingest is, per feed source, seconds since its last successful round
	// and how far the feed's newest row is past the cursor (absent before
	// the daemon has written any).
	Ingest []DoTreNguon `json:"ingest,omitempty"`
}

// DoTreNguon is one feed source's freshness.
type DoTreNguon struct {
	Nguon    string  `json:"nguon"`
	VongGiay float64 `json:"vong_giay"`
	TreGiay  float64 `json:"tre_giay"`
}

// DocDoTuoi reads the freshness numbers.
func DocDoTuoi(ctx context.Context, q Querier) (DoTuoi, error) {
	var d DoTuoi
	err := q.QueryRow(ctx, `
		WITH r AS (
		  SELECT noticed_at, uu_tien, cho_nhung, cho_tu,
		         cho_den IS NULL OR cho_den <= clock_timestamp()
		           OR EXISTS (SELECT 1 FROM rag_ingest_dlq q WHERE q.corpus = d.corpus AND q.doc_id = d.doc_id) AS den
		    FROM rag_dirty d WHERE corpus = 'place')
		SELECT COALESCE(EXTRACT(EPOCH FROM clock_timestamp() - min(noticed_at) FILTER (WHERE den AND uu_tien >= 0)), 0),
		       count(*) FILTER (WHERE den AND uu_tien >= 0),
		       count(*) FILTER (WHERE cho_nhung AND NOT den),
		       COALESCE(EXTRACT(EPOCH FROM clock_timestamp() - min(cho_tu) FILTER (WHERE cho_nhung AND NOT den)), 0),
		       count(*) FILTER (WHERE NOT den AND NOT cho_nhung),
		       count(*) FILTER (WHERE den AND uu_tien < 0),
		       (SELECT count(*) FROM rag_ingest_dlq WHERE corpus = 'place')
		  FROM r`).Scan(&d.LechGiay, &d.CanLam, &d.ChoNhung, &d.ChoNhungGiay, &d.Hoan, &d.NenCho, &d.DLQ)
	if err != nil {
		return d, err
	}
	var co bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('ingest_do_tre') IS NOT NULL`).Scan(&co); err != nil || !co {
		return d, err
	}
	rows, err := q.Query(ctx, `SELECT source, EXTRACT(EPOCH FROM clock_timestamp() - vong_at), tre_giay FROM ingest_do_tre ORDER BY source`)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var n DoTreNguon
		if err := rows.Scan(&n.Nguon, &n.VongGiay, &n.TreGiay); err != nil {
			return d, err
		}
		n.VongGiay = float64(int64(n.VongGiay*10)) / 10
		d.Ingest = append(d.Ingest, n)
	}
	d.LechGiay = float64(int64(d.LechGiay*10)) / 10
	d.ChoNhungGiay = float64(int64(d.ChoNhungGiay*10)) / 10
	return d, rows.Err()
}

// ViPham lists what in d breaks the SLO at this instant (sorted): the lag,
// a dead letter, a stalled ingest.
func (d DoTuoi) ViPham() []string {
	var out []string
	if d.LechGiay > SLOLechGiay {
		out = append(out, "lech")
	}
	if d.DLQ > 0 {
		out = append(out, "dlq")
	}
	for _, n := range d.Ingest {
		if n.VongGiay > SLOIngestGiay {
			out = append(out, "ingest")
			break
		}
	}
	sort.Strings(out)
	return out
}

// GiamSat holds what the rag-indexer's health endpoints answer from: the
// last pass of its loop (heartbeat), the last freshness reading, and since
// when the SLO has been broken. Safe for concurrent use.
type GiamSat struct {
	luot   atomic.Int64 // unix nanos of the loop's last pass
	ketQua atomic.Pointer[KetQuaSLO]
	tu     time.Time // since when broken; zero when not
}

// KetQuaSLO is one reading as /slo answers it.
type KetQuaSLO struct {
	Dat    bool      `json:"dat"`
	ViPham []string  `json:"vi_pham,omitempty"`
	TuGiay float64   `json:"vi_pham_tu_giay,omitempty"`
	DoTuoi DoTuoi    `json:"do_tuoi"`
	Luc    time.Time `json:"luc"`
	Loi    string    `json:"loi,omitempty"`
}

// Nhip records a pass of the loop.
func (g *GiamSat) Nhip(now time.Time) {
	if g != nil {
		g.luot.Store(now.UnixNano())
	}
}

// Song reports whether the loop passed within d.
func (g *GiamSat) Song(now time.Time, d time.Duration) bool {
	n := g.luot.Load()
	return n != 0 && now.Sub(time.Unix(0, n)) <= d
}

// Ghi records a reading taken at now (err: the reading failed) and returns
// the verdict: broken when something has broken the SLO for SLOKeoDai, or
// the reading itself failed. Only the reader goroutine calls it.
func (g *GiamSat) Ghi(now time.Time, d DoTuoi, err error) KetQuaSLO {
	k := KetQuaSLO{DoTuoi: d, Luc: now, Dat: true}
	if err != nil {
		k.Dat, k.Loi = false, MaLoi(err)
		g.ketQua.Store(&k)
		return k
	}
	k.ViPham = d.ViPham()
	switch {
	case len(k.ViPham) == 0:
		g.tu = time.Time{}
	case g.tu.IsZero():
		g.tu = now
	}
	if !g.tu.IsZero() {
		k.TuGiay = now.Sub(g.tu).Seconds()
		k.Dat = now.Sub(g.tu) < SLOKeoDai
	}
	g.ketQua.Store(&k)
	return k
}

// Doc is the last reading (nil before the first).
func (g *GiamSat) Doc() *KetQuaSLO { return g.ketQua.Load() }
