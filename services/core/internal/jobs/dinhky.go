package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

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
	// Chay runs one pass. It may use any connection of pool; the lock is held
	// on a connection of its own.
	Chay func(ctx context.Context, pool *pgxpool.Pool) error
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

// MotLuot runs one pass of d while holding
// pg_try_advisory_lock(hashtextextended('jobs:'||ten,0)) on a connection of its
// own. ran is false when another process holds the lock: that pass is
// skipped, not queued, because the holder is doing the same work. The lock is
// released before the connection goes back to the pool; a connection that
// cannot confirm the release is closed instead, which releases it too.
func MotLuot(ctx context.Context, pool *pgxpool.Pool, d DinhKy) (ran bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, passTimeout)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	var got bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended('jobs:'||$1,0))`, d.Ten).Scan(&got); err != nil {
		conn.Release()
		return false, err
	}
	if !got {
		conn.Release()
		return false, nil
	}
	defer func() {
		unlock, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		var released bool
		if e := conn.QueryRow(unlock, `SELECT pg_advisory_unlock(hashtextextended('jobs:'||$1,0))`, d.Ten).Scan(&released); e != nil || !released {
			_ = conn.Hijack().Close(unlock)
			return
		}
		conn.Release()
	}()
	return true, d.Chay(ctx, pool)
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
