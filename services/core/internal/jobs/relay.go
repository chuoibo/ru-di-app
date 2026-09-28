package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Relay publishes due outbox rows. A row is marked published only after the
// broker confirmed it and did not return it as unroutable, in the same
// transaction that held the row: a crash between the two produces a
// duplicate message, never a lost job. Several relays are safe together
// (SKIP LOCKED), and a duplicate is harmless because a consumer claims a job by
// id and enqueue sequence.
type Relay struct {
	pool     *pgxpool.Pool
	topology Topology
	ch       *amqp.Channel
	returns  chan amqp.Return
	// ConfirmTimeout bounds the wait for the broker's confirms of one batch.
	ConfirmTimeout time.Duration
}

// batchSize is how many rows one Flush publishes at most.
const batchSize = 256

// NewRelay opens a confirm-mode channel on conn and declares the topology.
func NewRelay(pool *pgxpool.Pool, conn *amqp.Connection, t Topology) (*Relay, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if err = ch.Confirm(false); err != nil {
		return nil, err
	}
	if err = t.Declare(ch); err != nil {
		return nil, err
	}
	// Sized for four full batches, and drained before and after every batch,
	// so it never fills: the library delivers a return by a blocking send, so
	// a full channel would stall its reader and every confirm behind it.
	return &Relay{pool: pool, topology: t, ch: ch, returns: ch.NotifyReturn(make(chan amqp.Return, 4*batchSize)),
		ConfirmTimeout: 2 * time.Second}, nil
}

// Close closes the relay's channel.
func (r *Relay) Close() error { return r.ch.Close() }

type pending struct {
	id    int64
	queue string
	msg   Message
}

// ErrNotConfirmed means the broker did not take a batch, or returned part of
// it as unroutable; only what it took was marked.
var ErrNotConfirmed = errors.New("jobs: broker did not confirm the batch")

// drainReturns empties the return channel without blocking and names the
// messages that were in it.
func (r *Relay) drainReturns() map[string]bool {
	returned := map[string]bool{}
	for {
		select {
		case ret, ok := <-r.returns:
			if !ok {
				return returned
			}
			returned[ret.MessageId] = true
		default:
			return returned
		}
	}
}

// Flush publishes one batch of up to 256 due rows and returns how many it
// marked published. Rows past their expiry are deleted unpublished (Don): the
// job they point at has closed its sharing window and cannot run anyway.
//
// A message the broker returns as unroutable (mandatory, no queue bound) is
// not marked: its row stays due and a later Flush publishes it again. The
// broker sends basic.return before the ack of the same message, so once every
// confirm of the batch is in, every return of the batch is in the channel.
func (r *Relay) Flush(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = Don(ctx, tx); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT id,queue,ref_id::text,enqueue_seq FROM job_outbox WHERE published_at IS NULL AND available_at<=clock_timestamp() ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 256`)
	if err != nil {
		return 0, err
	}
	batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (pending, error) {
		var p pending
		err := row.Scan(&p.id, &p.queue, &p.msg.Ref, &p.msg.Seq)
		p.msg.V = 1
		return p, err
	})
	if err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, tx.Commit(ctx)
	}
	// Returns left by an earlier batch that failed before its own drain
	// belong to rows that were never marked; they say nothing about this one.
	_ = r.drainReturns()
	confirms := make([]*amqp.DeferredConfirmation, 0, len(batch))
	for _, p := range batch {
		body, err := p.msg.Encode()
		if err != nil {
			return 0, err
		}
		dc, err := r.ch.PublishWithDeferredConfirmWithContext(ctx, r.topology.Exchange(), p.queue, true, false, amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			MessageId:    p.msg.ID(p.queue),
			Body:         body,
		})
		if err != nil {
			return 0, err
		}
		confirms = append(confirms, dc)
	}
	wait, stop := context.WithTimeout(ctx, r.ConfirmTimeout)
	defer stop()
	for _, dc := range confirms {
		ok, err := dc.WaitContext(wait)
		if err != nil || !ok {
			return 0, ErrNotConfirmed
		}
	}
	// A mandatory message nobody could route came back as basic.return
	// before its ack; its row is not marked.
	returned := r.drainReturns()
	ids := make([]int64, 0, len(batch))
	for _, p := range batch {
		if !returned[p.msg.ID(p.queue)] {
			ids = append(ids, p.id)
		}
	}
	if len(ids) > 0 {
		if _, err = tx.Exec(ctx, `UPDATE job_outbox SET published_at=clock_timestamp() WHERE id=ANY($1)`, ids); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	if len(ids) < len(batch) {
		return len(ids), ErrNotConfirmed
	}
	return len(ids), nil
}

// Run flushes on every outbox notification and at least every tick, until ctx
// ends (nil), the channel closes, or its listening connection fails (an
// error either way). Ket then dials the broker again after a closed channel,
// and listens again on a new database connection after a failed one.
//
// LISTEN holds its connection for as long as the relay runs, so that
// connection is the relay's own, opened with the pool's settings but outside
// it, and closed when Run returns: a pooled one would be one fewer for the
// jobs for the life of the process, and would go back to the pool still
// listening. Each flush takes a pooled connection for its transaction only.
func (r *Relay) Run(ctx context.Context, tick time.Duration) error {
	conn, err := pgx.ConnectConfig(ctx, r.pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer func() {
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = conn.Close(closing)
	}()
	if _, err = conn.Exec(ctx, `LISTEN job_outbox`); err != nil {
		return err
	}
	closed := r.ch.NotifyClose(make(chan *amqp.Error, 1))
	for {
		for {
			n, err := r.Flush(ctx)
			if err != nil || n < batchSize {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case e := <-closed:
			if e != nil {
				return e
			}
			return errors.New("jobs: relay channel closed")
		default:
		}
		wait, cancel := context.WithTimeout(ctx, tick)
		_, err := conn.WaitForNotification(wait)
		// Read before cancel(), which ends wait as well: read after it, a
		// connection that failed looks like a tick that passed, and the
		// loop flushes as fast as the dead connection answers.
		idle := wait.Err() != nil
		cancel()
		if err != nil && !idle {
			// The listening connection itself failed, not the wait: a new
			// Run listens on a new one.
			return err
		}
	}
}
