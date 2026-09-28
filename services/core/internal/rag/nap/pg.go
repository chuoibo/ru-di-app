package nap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

// BoNhoPG is the dense and sparse caches in PostgreSQL.
type BoNhoPG struct{ Q CSDL }

// LayNhung reads cached vectors for hashes.
func (b BoNhoPG) LayNhung(ctx context.Context, model string, dims int, task string, hashes []string) (map[string][]float32, error) {
	rows, err := b.Q.Query(ctx, `SELECT content_hash, vec FROM rag_embedding_cache
		WHERE model=$1 AND dims=$2 AND task=$3 AND content_hash = ANY($4::text[])`, model, dims, task, hashes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]float32{}
	for rows.Next() {
		var h string
		var raw []byte
		if err := rows.Scan(&h, &raw); err != nil {
			return nil, err
		}
		out[h] = bytesF32(raw)
	}
	return out, rows.Err()
}

// GhiNhung stores vectors; an existing entry is kept (same key, same bytes).
func (b BoNhoPG) GhiNhung(ctx context.Context, model string, dims int, task string, vecs map[string][]float32) error {
	batch := &pgx.Batch{}
	for h, v := range vecs {
		batch.Queue(`INSERT INTO rag_embedding_cache(content_hash, model, dims, task, vec) VALUES($1,$2,$3,$4,$5)
			ON CONFLICT DO NOTHING`, h, model, dims, task, f32Bytes(v))
	}
	return b.Q.SendBatch(ctx, batch).Close()
}

// LayThua reads cached sparse vectors.
func (b BoNhoPG) LayThua(ctx context.Context, model, rev string, pruneK int, hashes []string) (map[string]VectorThua, error) {
	rows, err := b.Q.Query(ctx, `SELECT content_hash, idx, val FROM rag_sparse_cache
		WHERE model=$1 AND rev=$2 AND prune_k=$3 AND content_hash = ANY($4::text[])`, model, rev, pruneK, hashes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]VectorThua{}
	for rows.Next() {
		var h string
		var idx []int32
		var val []float32
		if err := rows.Scan(&h, &idx, &val); err != nil {
			return nil, err
		}
		v := VectorThua{Val: val}
		for _, i := range idx {
			v.Idx = append(v.Idx, uint32(i))
		}
		out[h] = v
	}
	return out, rows.Err()
}

// GhiThua stores sparse vectors.
func (b BoNhoPG) GhiThua(ctx context.Context, model, rev string, pruneK int, vecs map[string]VectorThua) error {
	batch := &pgx.Batch{}
	for h, v := range vecs {
		idx := make([]int32, len(v.Idx))
		for i, x := range v.Idx {
			idx[i] = int32(x)
		}
		batch.Queue(`INSERT INTO rag_sparse_cache(content_hash, model, rev, prune_k, idx, val) VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT DO NOTHING`, h, model, rev, pruneK, idx, v.Val)
	}
	return b.Q.SendBatch(ctx, batch).Close()
}

// DocLamGiau reads the stored enrichment of each place id.
func DocLamGiau(ctx context.Context, q Querier, ids []string) (map[string]*LamGiau, error) {
	rows, err := q.Query(ctx, `SELECT place_id, source_hash, model, prompt_version, output, can_duyet, review
		FROM place_enrichments WHERE extractor=$1 AND place_id = ANY($2::text[])`, Extractor, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*LamGiau{}
	for rows.Next() {
		var lg LamGiau
		var h []byte
		var raw []byte
		if err := rows.Scan(&lg.PlaceID, &h, &lg.Model, &lg.PromptVersion, &raw, &lg.CanDuyet, &lg.Review); err != nil {
			return nil, err
		}
		copy(lg.NguonHash[:], h)
		if err := json.Unmarshal(raw, &lg.KetQua); err != nil {
			return nil, err
		}
		out[lg.PlaceID] = &lg
	}
	return out, rows.Err()
}

// GhiLamGiau stores enrichments, replacing the place's previous row: a new
// source hash is a new judgement and goes back to `auto`.
func GhiLamGiau(ctx context.Context, q CSDL, ls []LamGiau) error {
	batch := &pgx.Batch{}
	for _, lg := range ls {
		raw, err := json.Marshal(lg.KetQua)
		if err != nil {
			return err
		}
		batch.Queue(`INSERT INTO place_enrichments(place_id, extractor, source_hash, model, prompt_version, output, chen_lenh, tin_cay, can_duyet, review)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (place_id, extractor) DO UPDATE SET source_hash=EXCLUDED.source_hash, model=EXCLUDED.model,
			  prompt_version=EXCLUDED.prompt_version, output=EXCLUDED.output, chen_lenh=EXCLUDED.chen_lenh, tin_cay=EXCLUDED.tin_cay,
			  can_duyet=EXCLUDED.can_duyet, review=EXCLUDED.review, created_at=clock_timestamp(), reviewed_at=NULL`,
			lg.PlaceID, Extractor, lg.NguonHash[:], lg.Model, lg.PromptVersion, raw, lg.KetQua.ChenLenh, string(lg.KetQua.TinCay), lg.CanDuyet, lg.Review)
	}
	return q.SendBatch(ctx, batch).Close()
}

// banDuyetSQL is the version of an enrichment a reviewer sees: 16 hex of
// sha256 over the source hash, the prompt version and the stored output.
// The queue shows it and a verdict must name it, so a verdict applies to
// exactly the output the reviewer read: if the indexer replaced the row in
// between, the verdict matches nothing and is refused.
const banDuyetSQL = `left(encode(sha256(source_hash || convert_to(prompt_version || output::text, 'UTF8')), 'hex'), 16)`

// MucDuyet is one row of the review queue: ids, enums, counts, no text but
// the closed ids, the dish names and the context lines the model wrote.
type MucDuyet struct {
	PlaceID  string        `json:"place_id"`
	Ban      string        `json:"ban"`
	TinCay   string        `json:"tin_cay"`
	ChenLenh bool          `json:"chen_lenh"`
	KetQua   KetQuaLamGiau `json:"ket_qua"`
	Review   string        `json:"review"`
	Luc      time.Time     `json:"luc"`
}

// HangDuyet lists enrichments waiting for a person (all=false), or every
// enrichment (all=true), oldest first, at most limit.
func HangDuyet(ctx context.Context, q Querier, all bool, limit int) ([]MucDuyet, error) {
	rows, err := q.Query(ctx, `SELECT place_id, `+banDuyetSQL+`, tin_cay, chen_lenh, output, review, created_at FROM place_enrichments
		WHERE extractor=$1 AND ($2 OR (can_duyet AND review='auto')) ORDER BY created_at, place_id LIMIT $3`, Extractor, all, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MucDuyet
	for rows.Next() {
		var m MucDuyet
		var raw []byte
		if err := rows.Scan(&m.PlaceID, &m.Ban, &m.TinCay, &m.ChenLenh, &raw, &m.Review, &m.Luc); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &m.KetQua); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// BanCua is the version the review queue shows for a place's enrichment
// (MucDuyet.Ban), ErrKhongCoLamGiau when it has none.
func BanCua(ctx context.Context, q Querier, placeID string) (string, error) {
	var ban string
	err := q.QueryRow(ctx, `SELECT `+banDuyetSQL+` FROM place_enrichments WHERE place_id=$1 AND extractor=$2`, placeID, Extractor).Scan(&ban)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrKhongCoLamGiau
	}
	return ban, err
}

// ErrKhongCoLamGiau: no enrichment for this place.
var ErrKhongCoLamGiau = errors.New("nap: no enrichment for this place")

// ErrBanDuyet: the enrichment is not the version the verdict names (it was
// replaced after the queue was read); list the queue again.
var ErrBanDuyet = errors.New("nap: the enrichment changed since it was listed; review the current version")

var banHopLe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Duyet records a person's verdict on the version ban of a place's
// enrichment (MucDuyet.Ban): reviewed or rejected. It never edits the
// model's output: approving makes a reviewed allergen list and diet reach
// the hard filter, rejecting withdraws the whole enrichment (its place then
// counts as allergens unknown, so a rejection can never make it pass an
// allergy filter it failed). A verdict whose version no longer matches the
// stored row changes nothing (ErrBanDuyet).
func Duyet(ctx context.Context, q Querier, placeID, ban, review string) error {
	if review != ReviewReviewed && review != ReviewRejected {
		return fmt.Errorf("%w: review %q", ErrCauHinh, review)
	}
	if !banHopLe.MatchString(ban) {
		return fmt.Errorf("%w: version %q is not 16 hex", ErrCauHinh, ban)
	}
	tag, err := q.Exec(ctx, `UPDATE place_enrichments SET review=$3, reviewed_at=clock_timestamp()
		WHERE place_id=$1 AND extractor=$2 AND `+banDuyetSQL+` = $4`, placeID, Extractor, review, ban)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var co bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM place_enrichments WHERE place_id=$1 AND extractor=$2)`,
			placeID, Extractor).Scan(&co); err != nil {
			return err
		}
		if co {
			return ErrBanDuyet
		}
		return ErrKhongCoLamGiau
	}
	// The place's chunks carry its enrichment: index it again.
	_, err = q.Exec(ctx, `INSERT INTO rag_dirty(corpus, doc_id) VALUES('place',$1)
		ON CONFLICT (corpus, doc_id) DO UPDATE SET lan = rag_dirty.lan + 1`, placeID)
	return err
}

// Chang and MaLoi are the dead-letter enums.
const (
	ChangHopDong = "hop_dong"
	ChangLamGiau = "lam_giau"
	ChangNhung   = "nhung"
	ChangThua    = "thua"
	ChangGhi     = "ghi"
	ChangXoa     = "xoa"
)

// MaLoi classifies an error into the dead-letter enum without its text.
func MaLoi(err error) string {
	switch {
	case err == nil:
		return "khac"
	case errors.Is(err, context.DeadlineExceeded):
		return "het_gio"
	case errors.Is(err, ErrCauTrucLamGiau), errors.Is(err, ErrVector):
		return "cau_truc"
	case errors.Is(err, errHetTran):
		return "het_tran"
	case errors.Is(err, errMilvus):
		return "milvus"
	}
	return "nha_cung_cap"
}

var (
	errHetTran = errors.New("nap: call ceiling reached")
	errMilvus  = errors.New("nap: vector store error")
)

// GhiDLQ counts one failure of doc at chang.
func GhiDLQ(ctx context.Context, q Querier, c Corpus, docID, chang string, err error) error {
	_, e := q.Exec(ctx, `INSERT INTO rag_ingest_dlq(corpus, doc_id, chang, ma_loi) VALUES($1,$2,$3,$4)
		ON CONFLICT (corpus, doc_id, chang) DO UPDATE SET so_lan = rag_ingest_dlq.so_lan + 1, ma_loi=EXCLUDED.ma_loi, lan_cuoi=clock_timestamp()`,
		string(c), docID, chang, MaLoi(err))
	return e
}

// XoaDLQ clears a document's dead letters once it went through.
func XoaDLQ(ctx context.Context, q Querier, c Corpus, docIDs []string) error {
	_, err := q.Exec(ctx, `DELETE FROM rag_ingest_dlq WHERE corpus=$1 AND doc_id = ANY($2::text[])`, string(c), docIDs)
	return err
}

// ThuLaiDLQ puts every dead-lettered place back into rag_dirty and clears
// the letters: `core rag v-dlq retry`.
func ThuLaiDLQ(ctx context.Context, db Beginner) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO rag_dirty(corpus, doc_id) SELECT DISTINCT corpus, doc_id FROM rag_ingest_dlq WHERE corpus='place'
		ON CONFLICT (corpus, doc_id) DO UPDATE SET lan = rag_dirty.lan + 1`)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM rag_ingest_dlq WHERE corpus='place'`); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), tx.Commit(ctx)
}

// PhienBan is one row of rag_vector_versions.
type PhienBan struct {
	ID          int64
	Corpus      Corpus
	State       string
	Collection  string
	DenseModel  string
	DenseDims   int
	SparseMode  CheDoThua
	SparseRev   string
	Chunker     string
	VanTay      string
	Docs        int
	Chunks      int
	ChangedDocs int
	ParentID    *int64
	Eval        []byte
}

const cotPhienBan = `id, corpus, state, milvus_collection, dense_model, dense_dims, sparse_mode, sparse_rev, chunker, config_fingerprint,
	COALESCE(docs,0), COALESCE(chunks,0), COALESCE(changed_docs,0), parent_id, eval`

func quetPhienBan(row pgx.Row) (PhienBan, error) {
	var p PhienBan
	var c, mode string
	err := row.Scan(&p.ID, &c, &p.State, &p.Collection, &p.DenseModel, &p.DenseDims, &mode, &p.SparseRev, &p.Chunker, &p.VanTay,
		&p.Docs, &p.Chunks, &p.ChangedDocs, &p.ParentID, &p.Eval)
	p.Corpus, p.SparseMode = Corpus(c), CheDoThua(mode)
	return p, err
}

// DocPhienBan reads one version.
func DocPhienBan(ctx context.Context, q Querier, id int64) (PhienBan, error) {
	p, err := quetPhienBan(q.QueryRow(ctx, `SELECT `+cotPhienBan+` FROM rag_vector_versions WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrTrangThai
	}
	return p, err
}

// PhienBanActive reads a corpus's active version.
func PhienBanActive(ctx context.Context, q Querier, c Corpus) (PhienBan, error) {
	p, err := quetPhienBan(q.QueryRow(ctx, `SELECT `+cotPhienBan+` FROM rag_vector_versions WHERE corpus=$1 AND state='active'`, string(c)))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrKhongActive
	}
	return p, err
}

// PhienBanSong lists a corpus's versions whose collection still exists and
// may be served or promoted: building, built, evaluated, active, and retired
// ones not yet dropped. Every tombstone deletes from all of them.
func PhienBanSong(ctx context.Context, q Querier, c Corpus) ([]PhienBan, error) {
	rows, err := q.Query(ctx, `SELECT `+cotPhienBan+` FROM rag_vector_versions
		WHERE corpus=$1 AND dropped_at IS NULL AND state <> 'failed' ORDER BY id`, string(c))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PhienBan
	for rows.Next() {
		p, err := quetPhienBan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
