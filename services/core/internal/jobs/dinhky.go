package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DinhKy is one periodic task: a name, a period and one pass. The registry
// owns no logic (design 02 §4 step 10): the package that owns a table exports
// its task, and a worker process runs the list it is given.
type DinhKy struct {
	// Ten names the task and its lock; lowercase, dots and underscores.
	Ten string
	// Nhip is the period between passes.
	Nhip time.Duration
	// Chay runs one pass inside tx, the transaction that holds the task's
	// lock. The pass needs no other connection: a pass that held its lock on
	// one connection and worked on a second could starve a small pool, every
	// task firing at once, each holding one and waiting for another. It must
	// not commit or roll back tx; MotLuot does, and the lock ends with it.
	Chay func(ctx context.Context, tx pgx.Tx) error
}

var tenPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,63}$`)

// KiemDinhKy refuses a list a worker should not start with: an unnamed task,
// a name used twice (two tasks would share one lock), a period that is not
// positive, or no pass.
func KiemDinhKy(ds []DinhKy) error {
	seen := map[string]bool{}
	for _, d := range ds {
		if !tenPattern.MatchString(d.Ten) {
			return fmt.Errorf("jobs: periodic task name %q is invalid", d.Ten)
		}
		if seen[d.Ten] {
			return fmt.Errorf("jobs: periodic task %s is registered twice", d.Ten)
		}
		seen[d.Ten] = true
		if d.Nhip <= 0 || d.Chay == nil {
			return fmt.Errorf("jobs: periodic task %s needs a positive period and a pass", d.Ten)
		}
	}
	return nil
}

// passTimeout bounds one pass, whatever its period.
const passTimeout = time.Minute

// MotLuot runs one pass of d in one transaction on one connection: it takes
// pg_try_advisory_xact_lock(hashtextextended('jobs:'||ten,0)) there, and the
// pass runs in that same transaction. ran is false when another process
// holds the lock: that pass is skipped, not queued, because the holder is
// doing the same work. Commit or rollback ends the lock with the pass, so no
// connection ever goes back to the pool still holding it.
func MotLuot(ctx context.Context, pool *pgxpool.Pool, d DinhKy) (ran bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, passTimeout)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var got bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('jobs:'||$1,0))`, d.Ten).Scan(&got); err != nil {
		return false, err
	}
	if !got {
		return false, tx.Rollback(ctx)
	}
	if err = d.Chay(ctx, tx); err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}

// ChayDinhKy runs every task on its own ticker until ctx ends. A failed pass
// is logged by name only and tried again on the next tick. Call KiemDinhKy
// first; ChayDinhKy refuses an invalid list the same way.
func ChayDinhKy(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, ds []DinhKy) error {
	if err := KiemDinhKy(ds); err != nil {
		return err
	}
	if logger == nil {
		logger = slog.Default()
	}
	var all sync.WaitGroup
	for _, d := range ds {
		all.Add(1)
		go func(d DinhKy) {
			defer all.Done()
			timer := time.NewTicker(d.Nhip)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					if _, err := MotLuot(ctx, pool, d); err != nil && ctx.Err() == nil {
						logger.Warn("periodic task failed", "task", d.Ten)
					}
				}
			}
		}(d)
	}
	all.Wait()
	return nil
}
