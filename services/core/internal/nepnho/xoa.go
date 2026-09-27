package nepnho

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/jobs"
)

// The deletion saga. A nep_xoa row is its whole state, so a crash between
// two steps loses nothing: the next attempt picks the row up again.
//
//	cho → sidecar delete (one fact: delete; all: delete_all; the account:
//	      purge_user, which also compacts) → the sidecar must answer
//	      remaining 0 → Go lists the person's memories itself and what was
//	      deleted must be absent → short-term purge (all, account) → receipt
//	      (nep_su_that.deleted_at, so_da_xoa, da_xoa_at) → xong
//
// Any other outcome is a closed failure code, lan_thu+1 and chay_luc pushed
// out by the backoff, which enqueues the next attempt on the `memory` lane
// (schema.sql, nep_xoa_enqueue): retried until the count is zero, never a
// receipt before.

// Closed failure codes of nep_xoa.loi.
const (
	loiDichVuVang = "dich_vu_vang"
	loiDichVuLoi  = "dich_vu_loi"
	loiConHang    = "con_hang"
)

// HangNho is the job lane of the deletion saga.
const HangNho = "memory"

// errBuoc carries a failure code out of a step.
type errBuoc struct{ ma string }

func (e errBuoc) Error() string { return "nepnho: deletion step failed: " + e.ma }

func maLoi(err error) error {
	switch {
	case errors.Is(err, ErrConHang):
		return errBuoc{loiConHang}
	case errors.Is(err, ErrDichVuVang):
		return errBuoc{loiDichVuVang}
	}
	return errBuoc{loiDichVuLoi}
}

type viecXoa struct {
	id     string
	nguoi  string
	phamVi string
	suThat *string
	buoc   string
	lanThu int
}

// lui is the retry backoff: 30 s doubling to one hour.
func lui(lanThu int) time.Duration {
	d := 30 * time.Second
	for i := 0; i < lanThu && d < time.Hour; i++ {
		d *= 2
	}
	if d > time.Hour {
		d = time.Hour
	}
	return d
}

const cotViec = `id::text, person_id::text, pham_vi, su_that_id::text, buoc, lan_thu`

func quetViec(row pgx.Row) (viecXoa, error) {
	var v viecXoa
	var nguoi *string
	err := row.Scan(&v.id, &nguoi, &v.phamVi, &v.suThat, &v.buoc, &v.lanThu)
	if nguoi != nil {
		v.nguoi = *nguoi
	}
	return v, err
}

// khoaViecSQL is the advisory lock key of one deletion. Whoever holds it
// runs that deletion's attempt: the sidecar is called while it is held, with
// no row of nep_xoa locked (re-review memory minor 5: a transaction holding
// FOR UPDATE through a purge of up to 50 s pinned a row and a connection in
// a transaction). The row is locked only for the record, re-checked still
// open, in a short transaction after the calls.
const khoaViecSQL = `hashtextextended('nep_xoa:'||$1::text,0)`

// ChayNgay runs one open deletion now, whatever its due time (in the
// caller's request, or for the job that names it), and reports whether its
// receipt was written. A deletion another attempt holds is left to it. The
// sidecar is called outside any transaction: the attempt holds only the
// deletion's session advisory lock, on a connection idle between the calls.
func (k *Kho) ChayNgay(ctx context.Context, viec string) (bool, error) {
	conn, err := k.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	var giu bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+khoaViecSQL+`)`, viec).Scan(&giu); err != nil {
		conn.Release()
		return false, err
	}
	if !giu {
		conn.Release()
		return false, nil
	}
	defer func() {
		// A session lock outlives the request on a pooled connection: one
		// that cannot be released closes the connection with it.
		if _, err := conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(`+khoaViecSQL+`)`, viec); err != nil {
			_ = conn.Hijack().Close(context.WithoutCancel(ctx))
			return
		}
		conn.Release()
	}()
	v, err := quetViec(conn.QueryRow(ctx, `SELECT `+cotViec+` FROM nep_xoa WHERE id=$1 AND buoc='cho'`, viec))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	n, loiBuoc := k.xoaVaDem(ctx, conn, v)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	xong, err := k.ghiBuoc(ctx, tx, v, n, loiBuoc)
	if err != nil {
		return false, err
	}
	return xong, tx.Commit(ctx)
}

// XuLyTin runs one message of the memory lane (jobs.Ket's handler for
// HangNho): the deletion its ref names, if still open. A step that fails is
// recorded, and its next attempt is already enqueued by then: the message is
// done either way. Only a database failure asks the consumer to pause.
func (k *Kho) XuLyTin(ctx context.Context, m jobs.Message) error {
	if !uuidPattern.MatchString(m.Ref) {
		return jobs.ErrMalformed
	}
	if _, err := k.ChayNgay(ctx, m.Ref); err != nil {
		return errors.Join(jobs.ErrTamDung, err)
	}
	return nil
}

// MaxViecMoiLuot bounds the deletions one pass works on: each may wait on
// the sidecar, and the pass runs inside one transaction.
const MaxViecMoiLuot = 4

// DinhKyXoa is the periodic safety net that runs every due deletion,
// account deletions queued by the trigger included, when the broker could
// not: the same step as the memory lane, from Postgres.
func (k *Kho) DinhKyXoa() jobs.DinhKy {
	return jobs.DinhKy{Ten: "nep.xoa", Nhip: time.Minute, Chay: func(ctx context.Context, tx pgx.Tx) error {
		_, err := k.LuotXoa(ctx, tx)
		return err
	}}
}

// LuotXoa runs one pass inside tx over due deletions and returns how many it
// worked on. A failing deletion records its code and backoff and does not
// stop the others.
//
// The pass takes each deletion's advisory lock for the transaction (a
// deletion another attempt holds is skipped), calls the sidecar for all of
// them with no row locked, and only then records each one, re-checked
// still open under its row lock: the pass works on its one connection
// (jobs.DinhKy), and no row is locked while the sidecar answers.
func (k *Kho) LuotXoa(ctx context.Context, tx pgx.Tx) (int, error) {
	rows, err := tx.Query(ctx, `SELECT `+cotViec+` FROM nep_xoa WHERE buoc = 'cho' AND chay_luc <= $1
	  ORDER BY chay_luc LIMIT $2`, k.now(), 4*MaxViecMoiLuot)
	if err != nil {
		return 0, err
	}
	var ungVien []viecXoa
	for rows.Next() {
		v, err := quetViec(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		ungVien = append(ungVien, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var ds []viecXoa
	for _, v := range ungVien {
		if len(ds) == MaxViecMoiLuot {
			break
		}
		var giu bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(`+khoaViecSQL+`)`, v.id).Scan(&giu); err != nil {
			return 0, err
		}
		if giu {
			ds = append(ds, v)
		}
	}
	type ketQua struct {
		n   int
		err error
	}
	kq := make([]ketQua, len(ds))
	for i, v := range ds {
		kq[i].n, kq[i].err = k.xoaVaDem(ctx, tx, v)
	}
	for i, v := range ds {
		if _, err := k.ghiBuoc(ctx, tx, v, kq[i].n, kq[i].err); err != nil {
			return len(ds), err
		}
	}
	return len(ds), nil
}

// ghiBuoc records the outcome of v's sidecar step inside tx, and reports
// whether the receipt was written: the row is locked and re-checked still
// open first (another attempt may have closed it). A step failure (errBuoc)
// is recorded on the row, which enqueues the next attempt; only a database
// error is returned.
func (k *Kho) ghiBuoc(ctx context.Context, tx pgx.Tx, v viecXoa, n int, loiBuoc error) (bool, error) {
	var buoc string
	err := tx.QueryRow(ctx, `SELECT buoc FROM nep_xoa WHERE id=$1 FOR UPDATE`, v.id).Scan(&buoc)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && buoc != "cho") {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var b errBuoc
	if errors.As(loiBuoc, &b) {
		_, dbErr := tx.Exec(ctx, `UPDATE nep_xoa SET loi=$2, lan_thu=lan_thu+1, chay_luc=$3 WHERE id=$1`,
			v.id, b.ma, k.now().Add(lui(v.lanThu)))
		return false, dbErr
	}
	if loiBuoc != nil {
		return false, loiBuoc
	}
	return true, k.bienNhan(ctx, tx, v, n)
}

// xoaVaDem deletes in the sidecar and counts, and returns how many rows the
// sidecar deleted. q is the connection or transaction of the attempt (a
// periodic pass uses no second connection); no row is locked while it runs.
// It returns an errBuoc for anything short of zero rows left.
func (k *Kho) xoaVaDem(ctx context.Context, q Reader, v viecXoa) (int, error) {
	if k.kho == nil {
		// No memory service on this host. A person with no receipt ever
		// has nothing in any store this host could have written: the
		// deletion is complete with a zero count. Anyone else waits for
		// the service.
		var co bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM nep_su_that WHERE person_id=$1 AND deleted_at IS NULL)`, v.nguoi).Scan(&co); err != nil {
			return 0, err
		}
		if co {
			return 0, errBuoc{loiDichVuVang}
		}
		return 0, nil
	}
	var n int
	var err error
	switch v.phamVi {
	case "mot":
		n, err = k.kho.Xoa(ctx, v.nguoi, *v.suThat)
	case "tat_ca":
		n, err = k.kho.XoaHet(ctx, v.nguoi)
	default:
		n, err = k.kho.XoaSach(ctx, v.nguoi)
	}
	if err != nil {
		return 0, maLoi(err)
	}
	// The sidecar said zero remain. Go counts for itself: the listing is
	// read at the collection's Strong consistency.
	con, err := k.kho.LietKe(ctx, v.nguoi)
	if err != nil {
		return 0, maLoi(err)
	}
	for _, m := range con {
		if v.phamVi != "mot" || m.ID == *v.suThat {
			return 0, errBuoc{loiConHang}
		}
	}
	if v.phamVi != "mot" && k.nganHan != nil {
		if _, err := k.nganHan.XoaNguoi(ctx, v.nguoi); err != nil {
			return 0, errBuoc{loiDichVuLoi}
		}
	}
	return n, nil
}

// bamNguoi is what a finished account deletion's receipts keep instead of
// the person id.
func (k *Kho) bamNguoi(nguoi string) []byte {
	m := hmac.New(sha256.New, k.khoaQuen)
	m.Write([]byte("nep_xoa\x00"))
	m.Write([]byte(nguoi))
	return m.Sum(nil)
}

// bienNhan writes the receipt of v and closes it.
func (k *Kho) bienNhan(ctx context.Context, tx pgx.Tx, v viecXoa, deleted int) error {
	now := k.now()
	switch v.phamVi {
	case "mot":
		if _, err := tx.Exec(ctx, `UPDATE nep_su_that SET deleted_at=$3 WHERE id=$2 AND person_id=$1 AND deleted_at IS NULL`, v.nguoi, *v.suThat, now); err != nil {
			return err
		}
	case "tat_ca":
		if _, err := tx.Exec(ctx, `UPDATE nep_su_that SET dang_xoa_at=COALESCE(dang_xoa_at,$2), deleted_at=$2 WHERE person_id=$1 AND deleted_at IS NULL`, v.nguoi, now); err != nil {
			return err
		}
	case "tai_khoan":
		// The account is gone: its receipts go too, every other open
		// deletion of the person is answered by this one, and the ledger
		// rows keep a keyed hash instead of the person id.
		if _, err := tx.Exec(ctx, `DELETE FROM nep_su_that WHERE person_id=$1`, v.nguoi); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE nep_xoa SET buoc='xong', so_da_xoa=COALESCE(so_da_xoa,0), da_xoa_at=COALESCE(da_xoa_at,$2), loi=NULL
		  WHERE person_id=$1 AND buoc='cho' AND id<>$3`, v.nguoi, now, v.id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE nep_xoa SET buoc='xong', so_da_xoa=$3, da_xoa_at=$4, loi=NULL WHERE id=$1 AND person_id=$2`,
			v.id, v.nguoi, deleted, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE nep_xoa SET person_id=NULL, nguoi_bam=$2 WHERE person_id=$1`, v.nguoi, k.bamNguoi(v.nguoi))
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE nep_xoa SET buoc='xong', so_da_xoa=$2, da_xoa_at=$3, loi=NULL WHERE id=$1`, v.id, deleted, now)
	return err
}

// DinhKyDon is the hourly consolidation: typed events past their 30-day
// window and tombstones past their year are dropped, and every fact past
// its end (den_luc) is hidden and handed to the deletion saga, one 'mot'
// deletion each, like a forget (contract: «Fact bị thay hoặc hết hạn xoá
// ngay khi củng cố»; re-review memory minor 3). An expired fact is not
// tombstoned: the person may say it again.
func (k *Kho) DinhKyDon() jobs.DinhKy {
	return jobs.DinhKy{Ten: "nep.don", Nhip: time.Hour, Chay: func(ctx context.Context, tx pgx.Tx) error {
		now := k.now()
		if _, err := tx.Exec(ctx, `DELETE FROM nep_su_kien WHERE luc <= $1`, now.Add(-GiuSuKien)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM nep_quen WHERE den <= $1`, now); err != nil {
			return err
		}
		_, err := k.XoaHetHan(ctx, tx)
		return err
	}}
}

// XoaHetHan hides every live fact past its den_luc and opens its one-fact
// deletion (the memory lane and the pass delete it from the sidecar and
// write the receipt), in tx. It returns how many facts it handed over.
func (k *Kho) XoaHetHan(ctx context.Context, tx pgx.Tx) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `WITH het AS (
	    UPDATE nep_su_that SET dang_xoa_at = $1
	     WHERE den_luc IS NOT NULL AND den_luc <= $1 AND dang_xoa_at IS NULL AND deleted_at IS NULL
	    RETURNING id, person_id),
	  mo AS (
	    INSERT INTO nep_xoa(person_id, pham_vi, su_that_id) SELECT person_id, 'mot', id FROM het
	    ON CONFLICT DO NOTHING RETURNING 1)
	  SELECT count(*) FROM het`, k.now()).Scan(&n)
	return n, err
}
