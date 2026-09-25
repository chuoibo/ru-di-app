// Package metrics is the one writer of ai_turn_metrics, and the owner of its
// schema: its own version table (aiharness_schema_migrations) with the
// checksum pattern of chatassist/migrate.go, installed by `core migrate-chat`
// after chatassist's. It is the only aiharness package that holds SQL
// (ranh_gioi_test.go); the engine itself reads and writes no database.
//
// A row is an obs.TurnRecord: ids, closed enums, counts, durations. The worker
// writes it once per attempt, after the job's terminal transition, and a
// failure to write it never fails the job.
package metrics

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/jobs"
)

//go:embed schema.sql
var schemaSQL string

// SchemaSQL is version 1, for the gates that read what the table can hold.
func SchemaSQL() string { return schemaSQL }

// Execer is a pool or a transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Migrate installs the schema. Run by `core migrate-chat`, never by a request.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734133)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS aiharness_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM aiharness_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("AI engine metrics migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO aiharness_schema_migrations VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Reader is a pool or a transaction.
type Reader interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Installed says whether version 1 is in place: what `serve` and `work`
// require before running Nếp on the Go engine.
func Installed(ctx context.Context, q Reader) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT to_regclass('aiharness_schema_migrations') IS NOT NULL AND to_regclass('ai_turn_metrics') IS NOT NULL`).Scan(&ok)
	if err != nil || !ok {
		return false, err
	}
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM aiharness_schema_migrations WHERE version>=1)`).Scan(&ok)
	return ok, err
}

// insert names every column; metrics_test.go holds it to obs.Columns().
const insert = `INSERT INTO ai_turn_metrics(invocation_id,lan_thu,bot,lenh,guard,out_guard,ket_thuc,code,loi_mo_hinh,prompt_version,buoc,so_goi_model,so_cong_cu,tokens_in,tokens_out,tokens_cached,tokens_thoughts,luot_bo,phieu_bo,ngay_mo_ho,ky_tu_an,khong_dau,ms_trang_thai_dau,ms_tien_xu_ly,ms_mo_hinh,ms_tong) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26) ON CONFLICT (invocation_id,lan_thu) DO NOTHING`

// Ghi writes one record. An invalid record is refused before any SQL, and a
// turn stopped from outside (obs.KetThucHuy) writes nothing: a lost lease or
// a stopping worker says nothing about the model or the provider, and a row
// for it would read as a failed turn.
func Ghi(ctx context.Context, q Execer, rec obs.TurnRecord) error {
	if err := rec.Valid(); err != nil {
		return err
	}
	if rec.KetThuc == obs.KetThucHuy {
		return nil
	}
	_, err := q.Exec(ctx, insert, rec.Values()...)
	return err
}

// Retention is how long a row lives.
const Retention = 30 * 24 * time.Hour

// Xoa deletes rows past Retention.
func Xoa(ctx context.Context, q Execer) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM ai_turn_metrics WHERE created_at < clock_timestamp() - make_interval(secs => $1)`, Retention.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DinhKy is the thirty-day purge as a periodic task (jobs.DinhKy), run by
// every process that runs AI workers. A database without the schema (a host
// that never ran the Go engine) is a pass with nothing to purge, not a
// failure.
func DinhKy() jobs.DinhKy {
	return jobs.DinhKy{Ten: "aiharness.xoa_so_do", Nhip: 10 * time.Minute, Chay: func(ctx context.Context, tx pgx.Tx) error {
		ok, err := Installed(ctx, tx)
		if err != nil || !ok {
			return err
		}
		_, err = Xoa(ctx, tx)
		return err
	}}
}
