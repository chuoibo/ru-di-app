package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Nghe runs a pass whenever a Postgres channel is notified: the wake-up of a
// worker whose work is rows a trigger marks (rag_dirty), so a change is
// picked up in a second or two instead of on the next tick. The rows stay
// the truth -- a notification only says «look» -- so a notification lost
// while nobody listened costs nothing: a pass runs on every (re)connection,
// and at least every ToiDa without one.
type Nghe struct {
	Pool *pgxpool.Pool
	// Kenh is the channel (LISTEN kenh).
	Kenh string
	// ToiDa is the longest wait without a notification before a pass runs
	// anyway (the safety poll).
	ToiDa time.Duration
	// Gop is how long, after a notification, more are gathered before the
	// pass: one pass for a burst. Measured from the first, never extended.
	Gop time.Duration
	// Chay runs one pass. lai asks for another at once (a backlog); a
	// pass that could not run (its lock held elsewhere) should wait a
	// little itself and ask again. An error is logged; the next
	// notification or the safety poll tries again.
	Chay   func(ctx context.Context) (lai bool, err error)
	Logger *slog.Logger
}

// Run listens until ctx ends. Its connection is its own, opened with the
// pool's settings but outside it (a LISTEN holds a connection for life);
// when that connection fails it listens again on a new one after 250 ms,
// doubling to 5 s, and 250 ms again after a session that lasted 10 s.
func (n Nghe) Run(ctx context.Context) {
	logger := n.Logger
	if logger == nil {
		logger = slog.Default()
	}
	wait := 250 * time.Millisecond
	for ctx.Err() == nil {
		began := time.Now()
		err := n.phien(ctx, logger)
		if ctx.Err() != nil {
			return
		}
		if time.Since(began) > 10*time.Second {
			wait = 250 * time.Millisecond
		}
		code := "closed"
		if err != nil {
			code = "error"
		}
		logger.Warn("listener lost its database connection; listening again", "channel", n.Kenh,
			"retry_ms", wait.Milliseconds(), "code", code)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if wait *= 2; wait > 5*time.Second {
			wait = 5 * time.Second
		}
	}
}

func (n Nghe) phien(ctx context.Context, logger *slog.Logger) error {
	conn, err := pgx.ConnectConfig(ctx, n.Pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer func() {
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = conn.Close(closing)
	}()
	if _, err = conn.Exec(ctx, `LISTEN `+pgx.Identifier{n.Kenh}.Sanitize()); err != nil {
		return err
	}
	// Changes made while nobody listened.
	n.chayHet(ctx, logger)
	for {
		got, err := cho(ctx, conn, n.ToiDa)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		if got && n.Gop > 0 {
			deadline := time.Now().Add(n.Gop)
			for {
				left := time.Until(deadline)
				if left <= 0 {
					break
				}
				more, err := cho(ctx, conn, left)
				if ctx.Err() != nil {
					return nil
				}
				if err != nil {
					return err
				}
				if !more {
					break
				}
			}
		}
		n.chayHet(ctx, logger)
	}
}

// cho waits up to d for one notification: got is false when d passed. An
// error is the connection's, not the wait's.
func cho(ctx context.Context, conn *pgx.Conn, d time.Duration) (got bool, err error) {
	wait, cancel := context.WithTimeout(ctx, d)
	_, err = conn.WaitForNotification(wait)
	// Read before cancel(), which ends wait as well: read after it, a
	// connection that failed looks like a wait that passed (Relay.Run).
	idle := wait.Err() != nil
	cancel()
	if err != nil {
		if idle {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// chayHet runs passes while they ask for another.
func (n Nghe) chayHet(ctx context.Context, logger *slog.Logger) {
	for ctx.Err() == nil {
		lai, err := n.Chay(ctx)
		if err != nil {
			if ctx.Err() == nil {
				logger.Warn("listener pass failed", "channel", n.Kenh)
			}
			return
		}
		if !lai {
			return
		}
	}
}
