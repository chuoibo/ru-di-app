// Package nepnho is Nếp's long-term memory (design 05 §6, ADR-0041, draft
// ADR-0043): the only writer of every nep_* table, and the adapter that
// implements trinho.TriNho over the mem0 sidecar (services/ai-infer), the
// only writer of the memory collection in Milvus.
//
// Policy lives here, in Go; the sidecar holds words and vectors and decides
// nothing:
//   - Nho recalls only when the person's toggle is on, and only facts with a
//     live receipt in nep_su_that: Postgres is the source of truth for what
//     exists, the sidecar's owner filter is not trusted alone.
//   - Ghi writes only when called, and only when the toggle is on. Whether
//     the person asked to be remembered is the router's decision (intent
//     `remember`) and the tool's data gate, made before this is called; this
//     code checks the toggle and the structure, never the words.
//   - Quen is a saga: sidecar delete, its answer of zero remaining, Go's own
//     listing without what was deleted, then the receipt; retried on the
//     `memory` job lane until then. A fact is hidden the moment it is asked
//     for.
//   - LietKe lists everything held: facts, facts being deleted, orphans the
//     sidecar holds without a receipt, and the typed events of the last 30
//     days.
//
// Nothing here reads a word to decide anything: no keyword list decides what
// is worth keeping, what a «forget» means or which fact answers a question
// (docs/architecture/03-ai-engine-hop-dong.md, «Luật không heuristic»).
package nepnho

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/aiharness/trinho"
)

// Errors the tools and routes turn into fixed sentences.
var (
	// ErrTat: the memory toggle is off; nothing is written.
	ErrTat = errors.New("nepnho: memory is off for this person")
	// ErrDaQuen: the person forgot this exact fact; it is not written again.
	ErrDaQuen = errors.New("nepnho: this fact was forgotten")
	// ErrDay: the person already has MaxSuThat live facts.
	ErrDay = errors.New("nepnho: memory is full")
	// ErrDangXoa: the forget is recorded and the fact hidden, but the
	// deletion is not counted complete yet; the pass finishes it.
	ErrDangXoa = errors.New("nepnho: deletion recorded, not yet complete")
	// ErrKhongCauHinh: this host runs no memory sidecar.
	ErrKhongCauHinh = errors.New("nepnho: no memory service on this host")
	// ErrKhongGhi: the extraction stored nothing from the sentence (the
	// model classified it as not the person's own outing preference).
	ErrKhongGhi = errors.New("nepnho: nothing in the sentence was kept")
	// ErrKhongLoiNguoi: the fact is not the person's own words (noi_ro);
	// the extraction is never given anything else as theirs.
	ErrKhongLoiNguoi = errors.New("nepnho: only the person's own words are written")
)

const (
	// MaxSuThat is the most live facts one person holds (design 05 §8).
	MaxSuThat = 200
	// MaxNho is the most facts one recall asks the sidecar for: its search
	// takes top_k 1..10 (services/ai-infer SearchReq). A tool asking for
	// more (tools.MaxKTool is 20) gets the ten best.
	MaxNho = 10
	// GiuQuen is how long a tombstone stops the same fact being written.
	GiuQuen = 365 * 24 * time.Hour
	// GiuSuKien is the typed event window.
	GiuSuKien = 30 * 24 * time.Hour
)

// XoaNganHan purges the person's short-term buffers (internal/aictx), so a
// forget-all or an account deletion leaves no derived copy in Redis.
type XoaNganHan interface {
	XoaNguoi(ctx context.Context, nguoi string) (int, error)
}

// Kho implements trinho.TriNho for Nếp.
type Kho struct {
	pool     *pgxpool.Pool
	kho      KhoNho
	khoaQuen []byte
	nganHan  XoaNganHan
	now      func() time.Time
}

var _ trinho.TriNho = (*Kho)(nil)

// Moi builds the adapter. kho may be nil: a host without the memory
// service still serves the toggle, the events and the deletion of what
// Postgres holds. khoaQuen is the tombstone HMAC key, at least 32 bytes.
func Moi(pool *pgxpool.Pool, kho KhoNho, khoaQuen []byte) (*Kho, error) {
	if len(khoaQuen) < 32 {
		return nil, errors.New("nepnho: the tombstone key must be at least 32 bytes")
	}
	return &Kho{pool: pool, kho: kho, khoaQuen: append([]byte(nil), khoaQuen...), now: time.Now}, nil
}

// VoiNganHan adds the short-term purge to forget-all and account deletion.
func (k *Kho) VoiNganHan(x XoaNganHan) *Kho { k.nganHan = x; return k }

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func kiemNguoi(nguoi string) error {
	if !uuidPattern.MatchString(nguoi) {
		return errors.New("nepnho: person id must be a UUID")
	}
	return nil
}

// khoa takes the per-person lock every write of this package holds, so a
// forget never races a write or a toggle of the same person.
func khoa(ctx context.Context, tx pgx.Tx, nguoi string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('nepnho:'||$1,0))`, nguoi)
	return err
}

// bamQuen is the tombstone of a fact: HMAC-SHA256 under the server key of
// the person and the fact's words in NFC. Equality of normalised strings, a
// data rule; it guesses no meaning.
func (k *Kho) bamQuen(nguoi, noiDung string) []byte {
	m := hmac.New(sha256.New, k.khoaQuen)
	m.Write([]byte(nguoi))
	m.Write([]byte{0})
	m.Write([]byte(norm.NFC.String(noiDung)))
	return m.Sum(nil)
}

func (k *Kho) batKhong(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, nguoi string) (bool, error) {
	var on bool
	err := q.QueryRow(ctx, `SELECT nho FROM nep_cai_dat WHERE person_id=$1`, nguoi).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return on, err
}

type bienNhan struct {
	id      string
	loai    trinho.LoaiSuThat
	nguon   trinho.NguonSuThat
	tuLuc   time.Time
	denLuc  *time.Time
	dangXoa bool
}

func (b bienNhan) suThat(noiDung string) trinho.SuThat {
	return trinho.SuThat{ID: b.id, NoiDung: noiDung, Loai: b.loai, TuLuc: b.tuLuc, DenLuc: b.denLuc, Nguon: b.nguon}
}

// songTrong returns, in the sidecar's order, the items with a live receipt
// of nguoi that are in force now. Items without one are dropped: the sidecar
// is an index, the receipts are the ledger.
func (k *Kho) songTrong(ctx context.Context, nguoi string, items []MucNho, canBat bool) ([]trinho.SuThat, error) {
	var ids []string
	for _, it := range items {
		if uuidPattern.MatchString(it.ID) {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := k.pool.Query(ctx, `SELECT s.id::text, s.loai, s.nguon, s.tu_luc, s.den_luc
	  FROM nep_su_that s
	 WHERE s.person_id=$1 AND s.id = ANY($2::uuid[]) AND s.dang_xoa_at IS NULL
	   AND (s.den_luc IS NULL OR s.den_luc > $3)
	   AND ($4 = false OR EXISTS (SELECT 1 FROM nep_cai_dat c WHERE c.person_id=$1 AND c.nho))`,
		nguoi, ids, k.now(), canBat)
	if err != nil {
		return nil, err
	}
	song, err := docBienNhan(rows)
	if err != nil {
		return nil, err
	}
	var out []trinho.SuThat
	for _, it := range items {
		if b, ok := song[it.ID]; ok {
			out = append(out, b.suThat(it.Text))
			delete(song, it.ID) // a duplicated item is listed once
		}
	}
	return out, nil
}

func docBienNhan(rows pgx.Rows) (map[string]bienNhan, error) {
	defer rows.Close()
	out := map[string]bienNhan{}
	for rows.Next() {
		var b bienNhan
		var loai, nguon string
		if err := rows.Scan(&b.id, &loai, &nguon, &b.tuLuc, &b.denLuc); err != nil {
			return nil, err
		}
		b.loai, b.nguon = trinho.LoaiSuThat(loai), trinho.NguonSuThat(nguon)
		out[b.id] = b
	}
	return out, rows.Err()
}

// Nho implements trinho.TriNho. Toggle off: nothing, no error, and the
// sidecar never hears the question.
func (k *Kho) Nho(ctx context.Context, nguoi, cau string, n int) ([]trinho.SuThat, error) {
	if err := kiemNguoi(nguoi); err != nil {
		return nil, err
	}
	if cau == "" || n < 1 {
		return nil, errors.New("nepnho: recall needs a query and at least one fact")
	}
	n = min(n, MaxNho)
	on, err := k.batKhong(ctx, k.pool, nguoi)
	if err != nil || !on {
		return nil, err
	}
	if k.kho == nil {
		return nil, ErrKhongCauHinh
	}
	items, err := k.kho.Tim(ctx, nguoi, cau, n)
	if err != nil {
		return nil, err
	}
	return k.songTrong(ctx, nguoi, items, true)
}

// Ghi implements trinho.TriNho. The sentence goes to the sidecar's
// extraction, which may keep it as one or several memories or keep nothing
// (ErrKhongGhi). Every memory kept gets its receipt; the first is returned.
// A kept memory whose words were forgotten before is taken back out.
func (k *Kho) Ghi(ctx context.Context, nguoi string, moi trinho.SuThatMoi) (trinho.SuThat, error) {
	if err := kiemNguoi(nguoi); err != nil {
		return trinho.SuThat{}, err
	}
	if err := moi.Kiem(); err != nil {
		return trinho.SuThat{}, err
	}
	// The sidecar's extraction reads its input as the person's own words
	// (role user; docs/architecture/03-ai-engine-hop-dong.md). Only a fact
	// the person said (noi_ro) is: the engine's remember_fact stores a span
	// of the person's message taken from the message itself
	// (tools.kiemGhiNho). A fact learnt any other way has no writer yet,
	// and is never sent under the person's name.
	if moi.Nguon != trinho.NoiRo {
		return trinho.SuThat{}, ErrKhongLoiNguoi
	}
	if k.kho == nil {
		return trinho.SuThat{}, ErrKhongCauHinh
	}
	if err := k.truocGhi(ctx, nguoi, k.bamQuen(nguoi, moi.NoiDung)); err != nil {
		return trinho.SuThat{}, err
	}
	ms, _, err := k.kho.Them(ctx, nguoi, moi.NoiDung)
	if err != nil {
		return trinho.SuThat{}, err
	}
	var giu []trinho.SuThat
	var loi error
	for _, m := range ms {
		s := trinho.SuThat{ID: m.ID, NoiDung: m.Text, Loai: moi.Loai, TuLuc: moi.TuLuc, DenLuc: moi.DenLuc, Nguon: moi.Nguon}
		// The extraction's own closed kind wins over the tool's when it is
		// one of ours; otherwise the tool's.
		if trinho.LoaiSuThats.Co(trinho.LoaiSuThat(m.Loai)) {
			s.Loai = trinho.LoaiSuThat(m.Loai)
		}
		if !uuidPattern.MatchString(m.ID) || utf8.RuneCountInString(m.Text) == 0 || utf8.RuneCountInString(m.Text) > trinho.MaxNoiDung {
			// Not a memory the ledger can hold: take it back out.
			k.boGhi(ctx, nguoi, s, false)
			loi = fmt.Errorf("%w: memory outside the ledger's shape", ErrDichVuSai)
			continue
		}
		if err := k.sauGhi(ctx, nguoi, s); err != nil {
			k.boGhi(ctx, nguoi, s, true)
			if loi == nil || errors.Is(err, ErrTat) {
				loi = err
			}
			continue
		}
		giu = append(giu, s)
	}
	if len(giu) == 0 {
		if loi != nil {
			return trinho.SuThat{}, loi
		}
		return trinho.SuThat{}, ErrKhongGhi
	}
	return giu[0], nil
}

func (k *Kho) truocGhi(ctx context.Context, nguoi string, bam []byte) error {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := khoa(ctx, tx, nguoi); err != nil {
		return err
	}
	on, err := k.batKhong(ctx, tx, nguoi)
	if err != nil {
		return err
	}
	if !on {
		return ErrTat
	}
	var quen bool
	var song int
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM nep_quen WHERE person_id=$1 AND khoa_bam=$2 AND den > $3),
	  (SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND deleted_at IS NULL)`, nguoi, bam, k.now()).Scan(&quen, &song); err != nil {
		return err
	}
	if quen {
		return ErrDaQuen
	}
	if song >= MaxSuThat {
		return ErrDay
	}
	return tx.Commit(ctx)
}

// sauGhi records the receipt, under the lock, only if the toggle is still
// on (a toggle turned off while the sidecar wrote wins), the extracted words
// were not forgotten before, and the person is below MaxSuThat.
func (k *Kho) sauGhi(ctx context.Context, nguoi string, s trinho.SuThat) error {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := khoa(ctx, tx, nguoi); err != nil {
		return err
	}
	on, err := k.batKhong(ctx, tx, nguoi)
	if err != nil {
		return err
	}
	if !on {
		return ErrTat
	}
	var quen bool
	var song int
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM nep_quen WHERE person_id=$1 AND khoa_bam=$2 AND den > $3),
	  (SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND deleted_at IS NULL)`, nguoi, k.bamQuen(nguoi, s.NoiDung), k.now()).Scan(&quen, &song); err != nil {
		return err
	}
	if quen {
		return ErrDaQuen
	}
	if song >= MaxSuThat {
		return ErrDay
	}
	if _, err := tx.Exec(ctx, `INSERT INTO nep_su_that(id, person_id, loai, nguon, tu_luc, den_luc) VALUES($1,$2,$3,$4,$5,$6)`,
		s.ID, nguoi, string(s.Loai), string(s.Nguon), s.TuLuc, s.DenLuc); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// boGhi takes back a memory the ledger will not hold. The sidecar deletes it
// now and Go checks its own listing; if either fails and the id is one the
// ledger can hold, a hidden receipt and a one-fact deletion are recorded, so
// the memory lane deletes it: nothing stays in the store unledgered. An id
// the ledger cannot hold is only deleted now, and shows up as held without
// a receipt in LietKe until a forget-all.
func (k *Kho) boGhi(ctx context.Context, nguoi string, s trinho.SuThat, soCai bool) {
	if _, err := k.kho.Xoa(ctx, nguoi, s.ID); err == nil {
		if con, err := k.kho.LietKe(ctx, nguoi); err == nil && !coID(con, s.ID) {
			return
		}
	}
	if !soCai {
		return
	}
	ctx = context.WithoutCancel(ctx)
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if khoa(ctx, tx, nguoi) != nil {
		return
	}
	now := k.now()
	if _, err := tx.Exec(ctx, `INSERT INTO nep_su_that(id, person_id, loai, nguon, tu_luc, den_luc, dang_xoa_at) VALUES($1,$2,$3,$4,$5,$6,$7)
	  ON CONFLICT (id) DO UPDATE SET dang_xoa_at = COALESCE(nep_su_that.dang_xoa_at, EXCLUDED.dang_xoa_at)`,
		s.ID, nguoi, string(s.Loai), string(s.Nguon), s.TuLuc, s.DenLuc, now); err != nil {
		return
	}
	if _, err := moViec(ctx, tx, nguoi, "mot", &s.ID); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func coID(ms []MucNho, id string) bool {
	for _, m := range ms {
		if m.ID == id {
			return true
		}
	}
	return false
}

// Quen implements trinho.TriNho. A forget is always allowed, toggle on or
// off. MoTa is resolved by the sidecar's embedding similarity to the single
// nearest live fact of the person: never by keyword. Deleting a fact the
// person did not mean errs toward holding less.
func (k *Kho) Quen(ctx context.Context, nguoi string, q trinho.QuenGi) (int, error) {
	if err := kiemNguoi(nguoi); err != nil {
		return 0, err
	}
	if err := q.Kiem(); err != nil {
		return 0, err
	}
	if k.kho == nil {
		return 0, ErrKhongCauHinh
	}
	id, noiDung := q.ID, ""
	if q.MoTa != "" {
		items, err := k.kho.Tim(ctx, nguoi, q.MoTa, 1)
		if err != nil {
			return 0, err
		}
		song, err := k.songTrong(ctx, nguoi, items, false)
		if err != nil || len(song) == 0 {
			return 0, err
		}
		id, noiDung = song[0].ID, song[0].NoiDung
	} else {
		if !uuidPattern.MatchString(id) {
			return 0, nil
		}
		// The words, for the tombstone. Best effort: without the sidecar the
		// fact is still hidden and deleted, only not tombstoned.
		if items, err := k.kho.LietKe(ctx, nguoi); err == nil {
			for _, it := range items {
				if it.ID == id {
					noiDung = it.Text
				}
			}
		}
	}
	viec, ok, err := k.anQuen(ctx, nguoi, id, noiDung)
	if err != nil || !ok {
		return 0, err
	}
	xong, err := k.ChayNgay(ctx, viec)
	if err != nil || !xong {
		return 0, ErrDangXoa
	}
	return 1, nil
}

// anQuen hides the fact, writes its tombstone and records the deletion, in
// one transaction under the person's lock. ok is false when the person has
// no live fact with this id.
func (k *Kho) anQuen(ctx context.Context, nguoi, id, noiDung string) (string, bool, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)
	if err := khoa(ctx, tx, nguoi); err != nil {
		return "", false, err
	}
	now := k.now()
	tag, err := tx.Exec(ctx, `UPDATE nep_su_that SET dang_xoa_at=$3 WHERE id=$2 AND person_id=$1 AND dang_xoa_at IS NULL`, nguoi, id, now)
	if err != nil {
		return "", false, err
	}
	if tag.RowsAffected() == 0 {
		return "", false, nil
	}
	if noiDung != "" {
		if _, err := tx.Exec(ctx, `INSERT INTO nep_quen(person_id, khoa_bam, den) VALUES($1,$2,$3)
		  ON CONFLICT (person_id, khoa_bam) DO UPDATE SET den=EXCLUDED.den`, nguoi, k.bamQuen(nguoi, noiDung), now.Add(GiuQuen)); err != nil {
			return "", false, err
		}
	}
	viec, err := moViec(ctx, tx, nguoi, "mot", &id)
	if err != nil {
		return "", false, err
	}
	return viec, true, tx.Commit(ctx)
}

// moViec opens a deletion, or returns the one already open for the same
// person, scope and fact.
func moViec(ctx context.Context, tx pgx.Tx, nguoi, phamVi string, suThat *string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO nep_xoa(person_id, pham_vi, su_that_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING id::text`,
		nguoi, phamVi, suThat).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE person_id=$1 AND pham_vi=$2 AND su_that_id IS NOT DISTINCT FROM $3::uuid AND buoc='cho'`,
			nguoi, phamVi, suThat).Scan(&id)
	}
	return id, err
}

// LietKe implements trinho.TriNho: everything held about the person.
func (k *Kho) LietKe(ctx context.Context, nguoi string) (trinho.TatCa, error) {
	if err := kiemNguoi(nguoi); err != nil {
		return trinho.TatCa{}, err
	}
	var out trinho.TatCa
	on, err := k.batKhong(ctx, k.pool, nguoi)
	if err != nil {
		return out, err
	}
	out.Bat = on
	rows, err := k.pool.Query(ctx, `SELECT id::text, loai, nguon, tu_luc, den_luc, dang_xoa_at IS NOT NULL
	  FROM nep_su_that WHERE person_id=$1 AND deleted_at IS NULL`, nguoi)
	if err != nil {
		return out, err
	}
	bn := map[string]bienNhan{}
	for rows.Next() {
		var b bienNhan
		var loai, nguon string
		if err := rows.Scan(&b.id, &loai, &nguon, &b.tuLuc, &b.denLuc, &b.dangXoa); err != nil {
			rows.Close()
			return out, err
		}
		b.loai, b.nguon = trinho.LoaiSuThat(loai), trinho.NguonSuThat(nguon)
		bn[b.id] = b
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	if k.kho == nil {
		if len(bn) > 0 {
			// Receipts without the service that holds their words: listing
			// them without words would not be the whole truth.
			return out, ErrKhongCauHinh
		}
	} else {
		items, err := k.kho.LietKe(ctx, nguoi)
		if err != nil {
			return out, err
		}
		seen := map[string]bool{}
		for _, it := range items {
			if seen[it.ID] {
				continue
			}
			seen[it.ID] = true
			b, ok := bn[it.ID]
			switch {
			case !ok:
				out.KhongSo = append(out.KhongSo, trinho.SuThat{ID: it.ID, NoiDung: it.Text, Loai: trinho.LoaiSuThat(it.Loai)})
			case b.dangXoa:
				out.DangXoa = append(out.DangXoa, b.suThat(it.Text))
			default:
				out.SuThat = append(out.SuThat, b.suThat(it.Text))
			}
		}
		// A fact being deleted may be gone from the sidecar while Milvus has
		// not been counted clean: still listed, without words.
		for id, b := range bn {
			if b.dangXoa && !seen[id] {
				out.DangXoa = append(out.DangXoa, b.suThat(""))
			}
		}
		sort.Slice(out.DangXoa, func(i, j int) bool { return out.DangXoa[i].ID < out.DangXoa[j].ID })
	}
	ev, err := k.pool.Query(ctx, `SELECT loai, count(*) FROM nep_su_kien WHERE person_id=$1 AND luc > $2 GROUP BY loai ORDER BY loai`,
		nguoi, k.now().Add(-GiuSuKien))
	if err != nil {
		return out, err
	}
	defer ev.Close()
	for ev.Next() {
		var loai string
		var n int
		if err := ev.Scan(&loai, &n); err != nil {
			return out, err
		}
		if out.SuKien == nil {
			out.SuKien = map[trinho.LoaiSuKien]int{}
		}
		out.SuKien[trinho.LoaiSuKien(loai)] = n
	}
	return out, ev.Err()
}
