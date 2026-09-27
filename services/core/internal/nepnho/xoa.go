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

// ChayNgay runs one open deletion now, whatever its due time (in the
// caller's request, or for the job that names it), and reports whether its
// receipt was written. A row another attempt holds is left to it.
func (k *Kho) ChayNgay(ctx context.Context, viec string) (bool, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	v, err := quetViec(tx.QueryRow(ctx, `SELECT `+cotViec+` FROM nep_xoa WHERE id=$1 AND buoc='cho' FOR UPDATE SKIP LOCKED`, viec))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	xong, err := k.buoc(ctx, tx, v)
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
func (k *Kho) LuotXoa(ctx context.Context, tx pgx.Tx) (int, error) {
	rows, err := tx.Query(ctx, `SELECT `+cotViec+` FROM nep_xoa WHERE buoc = 'cho' AND chay_luc <= $1
	  ORDER BY chay_luc LIMIT $2 FOR UPDATE SKIP LOCKED`, k.now(), MaxViecMoiLuot)
	if err != nil {
		return 0, err
	}
	var ds []viecXoa
	for rows.Next() {
		v, err := quetViec(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		ds = append(ds, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, v := range ds {
		if _, err := k.buoc(ctx, tx, v); err != nil {
			return len(ds), err
		}
	}
	return len(ds), nil
}

// buoc runs v inside tx (which holds v's row lock) and reports whether the
// receipt was written. A step failure is recorded on the row, which
// enqueues the next attempt; only a database error is returned.
func (k *Kho) buoc(ctx context.Context, tx pgx.Tx, v viecXoa) (bool, error) {
	n, err := k.xoaVaDem(ctx, tx, v)
	var b errBuoc
	if errors.As(err, &b) {
		_, dbErr := tx.Exec(ctx, `UPDATE nep_xoa SET loi=$2, lan_thu=lan_thu+1, chay_luc=$3 WHERE id=$1`,
			v.id, b.ma, k.now().Add(lui(v.lanThu)))
		return false, dbErr
	}
	if err != nil {
		return false, err
	}
	return true, k.bienNhan(ctx, tx, v, n)
}

// xoaVaDem deletes in the sidecar and counts, and returns how many rows the
// sidecar deleted. q is the transaction holding v's lock (a periodic pass
// uses no second connection). It returns an errBuoc for anything short of zero rows
// left.
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

// DinhKyDon drops typed events past their 30-day window and tombstones past
// their year, hourly.
func (k *Kho) DinhKyDon() jobs.DinhKy {
	return jobs.DinhKy{Ten: "nep.don", Nhip: time.Hour, Chay: func(ctx context.Context, tx pgx.Tx) error {
		now := k.now()
		if _, err := tx.Exec(ctx, `DELETE FROM nep_su_kien WHERE luc <= $1`, now.Add(-GiuSuKien)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM nep_quen WHERE den <= $1`, now)
		return err
	}}
}
