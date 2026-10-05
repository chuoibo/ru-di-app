package nepnho

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/aiharness/trinho"
)

// CongBoBan is the version of the disclosure text the app shows before the
// toggle turns on (ADR-0041 §2.5, nep/cong-bo.ts). Changing the words means
// raising it: every person is asked again, and nep_cai_dat keeps the version
// each one agreed to, as the record of consent.
const CongBoBan = 1

// ErrCongBoCu: turning memory on with a disclosure version other than the
// current one.
var ErrCongBoCu = errors.New("nepnho: the disclosure agreed to is not the current one")

// ErrQuaNhanh: a second event batch within NhipSuKien.
var ErrQuaNhanh = errors.New("nepnho: event batches too fast")

// CaiDat is the person's memory settings.
type CaiDat struct {
	Nho       bool
	CongBoBan *int16
	CongBoAt  *time.Time
}

// DocCaiDat reads the settings; a person who never chose has memory off.
func (k *Kho) DocCaiDat(ctx context.Context, nguoi string) (CaiDat, error) {
	if err := kiemNguoi(nguoi); err != nil {
		return CaiDat{}, err
	}
	var c CaiDat
	err := k.pool.QueryRow(ctx, `SELECT nho, cong_bo_ban, cong_bo_at FROM nep_cai_dat WHERE person_id=$1`, nguoi).Scan(&c.Nho, &c.CongBoBan, &c.CongBoAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaiDat{}, nil
	}
	return c, err
}

// ErrPhienHet is a consent write whose person was erased, or whose session
// ended, after the request was authenticated: nothing is written.
var ErrPhienHet = errors.New("nepnho: the person or the session is gone")

// Bat turns memory on with the disclosure version the person agreed to.
func (k *Kho) Bat(ctx context.Context, nguoi string, ban int) error {
	return k.bat(ctx, nguoi, ban, nil)
}

// BatTheoPhien is Bat for a request: the person and the bearer's session are
// read again, locked, inside the transaction that writes the consent (audit
// 2026-10-05, PER-NEPNHO-01). Before, authentication released its locks first,
// and an account erased in between got its consent row written back.
func (k *Kho) BatTheoPhien(ctx context.Context, digest []byte, nguoi string, ban int) error {
	return k.bat(ctx, nguoi, ban, digest)
}

func (k *Kho) bat(ctx context.Context, nguoi string, ban int, digest []byte) error {
	if err := kiemNguoi(nguoi); err != nil {
		return err
	}
	if ban != CongBoBan {
		return ErrCongBoCu
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := khoa(ctx, tx, nguoi); err != nil {
		return err
	}
	// Person first, then session: the order erasure and authentication lock
	// in. FOR SHARE waits for an erasure holding the person FOR UPDATE and
	// then sees its deleted_at.
	var live bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE)`, nguoi).Scan(&live); err != nil {
		return err
	}
	if live && digest != nil {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_sessions WHERE token_digest=$1 AND person_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE)`, digest, nguoi).Scan(&live); err != nil {
			return err
		}
	}
	if !live {
		return ErrPhienHet
	}
	now := k.now()
	if _, err := tx.Exec(ctx, `INSERT INTO nep_cai_dat(person_id, nho, cong_bo_ban, cong_bo_at, cap_nhat_at) VALUES($1,true,$2,$3,$3)
	  ON CONFLICT (person_id) DO UPDATE SET nho=true, cong_bo_ban=EXCLUDED.cong_bo_ban, cong_bo_at=EXCLUDED.cong_bo_at, cap_nhat_at=EXCLUDED.cap_nhat_at`,
		nguoi, ban, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Tat turns memory off and forgets everything (design 05 §6 «Xoá»): the
// events and tombstones go in this transaction, every fact is hidden, and one
// forget-all deletion is recorded and tried at once. It returns the
// deletion's id and whether its receipt was written.
func (k *Kho) Tat(ctx context.Context, nguoi string) (string, bool, error) {
	return k.xoaHet(ctx, nguoi, true)
}

// QuenHet forgets everything and leaves the toggle as it is.
func (k *Kho) QuenHet(ctx context.Context, nguoi string) (string, bool, error) {
	return k.xoaHet(ctx, nguoi, false)
}

func (k *Kho) xoaHet(ctx context.Context, nguoi string, tat bool) (string, bool, error) {
	if err := kiemNguoi(nguoi); err != nil {
		return "", false, err
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)
	if err := khoa(ctx, tx, nguoi); err != nil {
		return "", false, err
	}
	now := k.now()
	if tat {
		if _, err := tx.Exec(ctx, `INSERT INTO nep_cai_dat(person_id, nho, cap_nhat_at) VALUES($1,false,$2)
		  ON CONFLICT (person_id) DO UPDATE SET nho=false, cap_nhat_at=EXCLUDED.cap_nhat_at`, nguoi, now); err != nil {
			return "", false, err
		}
	}
	for _, q := range []string{`DELETE FROM nep_su_kien WHERE person_id=$1`, `DELETE FROM nep_quen WHERE person_id=$1`} {
		if _, err := tx.Exec(ctx, q, nguoi); err != nil {
			return "", false, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE nep_su_that SET dang_xoa_at=$2 WHERE person_id=$1 AND dang_xoa_at IS NULL`, nguoi, now); err != nil {
		return "", false, err
	}
	viec, err := moViec(ctx, tx, nguoi, "tat_ca", nil)
	if err != nil {
		return "", false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, err
	}
	xong, err := k.ChayNgay(ctx, viec)
	if err != nil {
		return viec, false, nil
	}
	return viec, xong, nil
}

// SuKien is one typed app event: a closed kind and the one typed reference
// that kind carries. Never words.
type SuKien struct {
	Loai          trinho.LoaiSuKien
	Luc           time.Time
	DiaDiemID     string
	DiemDenID     string
	DanhMuc       string
	PhuongTien    string
	ThoiLuongPhut int
}

const (
	// MaxSuKienMoiLo is the largest event batch.
	MaxSuKienMoiLo = 50
	// NhipSuKien is the least time between two batches of one person.
	NhipSuKien = 10 * time.Second
)

var (
	idThamChieu = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	danhMucHinh = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	phuongTiens = map[string]bool{"motorbike": true, "car": true, "walk": true}
)

// ErrSuKienSai: an event outside its closed shape.
var ErrSuKienSai = errors.New("nepnho: event outside its closed shape")

// Kiem checks an event's structure: its kind carries exactly its own typed
// reference. The same shape the table's CHECK holds.
func (s SuKien) Kiem() error {
	if !trinho.LoaiSuKiens.Co(s.Loai) || s.Luc.IsZero() {
		return ErrSuKienSai
	}
	co := map[string]bool{"dia_diem": s.DiaDiemID != "", "diem_den": s.DiemDenID != "", "danh_muc": s.DanhMuc != "",
		"phuong_tien": s.PhuongTien != "", "thoi_luong": s.ThoiLuongPhut != 0}
	can := ""
	switch s.Loai {
	case trinho.MoDiaDiem, trinho.LuuDiaDiem, trinho.BoLuu, trinho.ThemChang, trinho.CheckIn:
		can = "dia_diem"
		if !idThamChieu.MatchString(s.DiaDiemID) {
			return ErrSuKienSai
		}
	case trinho.ChonDiemDen:
		can = "diem_den"
		if !idThamChieu.MatchString(s.DiemDenID) {
			return ErrSuKienSai
		}
	case trinho.LocDanhMuc:
		can = "danh_muc"
		if !danhMucHinh.MatchString(s.DanhMuc) {
			return ErrSuKienSai
		}
	case trinho.ChonPhuongTien:
		can = "phuong_tien"
		if !phuongTiens[s.PhuongTien] {
			return ErrSuKienSai
		}
	case trinho.ChonThoiLuong:
		can = "thoi_luong"
		if s.ThoiLuongPhut < 5 || s.ThoiLuongPhut > 1440 {
			return ErrSuKienSai
		}
	}
	for field, set := range co {
		if set && field != can {
			return fmt.Errorf("%w: %s carries %s", ErrSuKienSai, s.Loai, field)
		}
	}
	return nil
}

func rong(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GhiSuKien records a batch of events when memory is on. Off: nothing is
// written and ghi is false (the route refuses with 409). Each time is clamped to [now-7d, now+5m].
func (k *Kho) GhiSuKien(ctx context.Context, nguoi string, ds []SuKien) (ghi bool, err error) {
	if err := kiemNguoi(nguoi); err != nil {
		return false, err
	}
	if len(ds) == 0 || len(ds) > MaxSuKienMoiLo {
		return false, ErrSuKienSai
	}
	for _, s := range ds {
		if err := s.Kiem(); err != nil {
			return false, err
		}
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := khoa(ctx, tx, nguoi); err != nil {
		return false, err
	}
	var on bool
	var truoc *time.Time
	err = tx.QueryRow(ctx, `SELECT nho, su_kien_at FROM nep_cai_dat WHERE person_id=$1`, nguoi).Scan(&on, &truoc)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !on) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	now := k.now()
	if truoc != nil && now.Sub(*truoc) < NhipSuKien {
		return false, ErrQuaNhanh
	}
	batch := &pgx.Batch{}
	for _, s := range ds {
		luc := s.Luc
		if luc.Before(now.Add(-7 * 24 * time.Hour)) {
			luc = now.Add(-7 * 24 * time.Hour)
		}
		if luc.After(now.Add(5 * time.Minute)) {
			luc = now.Add(5 * time.Minute)
		}
		var phut any
		if s.ThoiLuongPhut != 0 {
			phut = s.ThoiLuongPhut
		}
		batch.Queue(`INSERT INTO nep_su_kien(person_id, loai, luc, dia_diem_id, diem_den_id, danh_muc, phuong_tien, thoi_luong_phut)
		  VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, nguoi, string(s.Loai), luc, rong(s.DiaDiemID), rong(s.DiemDenID), rong(s.DanhMuc), rong(s.PhuongTien), phut)
	}
	batch.Queue(`UPDATE nep_cai_dat SET su_kien_at=$2 WHERE person_id=$1`, nguoi, now)
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
