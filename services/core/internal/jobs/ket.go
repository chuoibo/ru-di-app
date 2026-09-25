package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Ket is one worker process's side of the broker: one connection, one relay
// and one consumer per queue, redialled with backoff for as long as its
// context lives. Postgres stays the truth: while Ket is down the process's
// poller claims the same jobs, so Ket only makes them start sooner.
//
// Song reports whether every consumer is attached right now. The poller reads
// it to choose its pace.
type Ket struct {
	// URL is the broker, amqp:// or amqps://. It carries a password and is
	// never logged.
	URL      string
	Topology Topology
	Pool     *pgxpool.Pool
	// Queues are the queues this process consumes; the relay publishes every
	// queue's rows regardless.
	Queues []string
	// Concurrency is each consumer's prefetch, and its bound on jobs at once.
	Concurrency int
	// Handler runs one message of queue.
	Handler func(ctx context.Context, queue string, m Message) error
	// Logger hears connection state only: never a message, never the URL.
	Logger *slog.Logger
	// Tick is the relay's idle flush period; zero means 250 ms.
	Tick time.Duration

	attached atomic.Int64
}

// Song reports whether every consumer of this process is attached.
func (k *Ket) Song() bool {
	return k != nil && len(k.Queues) > 0 && k.attached.Load() == int64(len(k.Queues))
}

// CheckURL accepts an AMQP URL without echoing it back in an error.
func CheckURL(raw string) error {
	if _, err := amqp.ParseURI(raw); err != nil {
		return errors.New("jobs: MOBILE_AMQP_URL is not a valid amqp:// or amqps:// URL")
	}
	return nil
}

// Run dials, serves and redials until ctx ends.
func (k *Ket) Run(ctx context.Context) {
	logger := k.Logger
	if logger == nil {
		logger = slog.Default()
	}
	wait := 250 * time.Millisecond
	for ctx.Err() == nil {
		conn, err := amqp.DialConfig(k.URL, amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(2 * time.Second)})
		if err != nil {
			logger.Warn("job broker unreachable; the Postgres poller carries the jobs", "retry_ms", wait.Milliseconds())
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			if wait *= 2; wait > 5*time.Second {
				wait = 5 * time.Second
			}
			continue
		}
		began := time.Now()
		logger.Info("job broker connected", "queues", len(k.Queues))
		k.serve(ctx, conn, logger)
		_ = conn.Close()
		if time.Since(began) > 10*time.Second {
			wait = 250 * time.Millisecond
		}
		if ctx.Err() == nil {
			logger.Warn("job broker connection lost; the Postgres poller carries the jobs")
		}
	}
}

// serve runs the relay and the consumers on one connection until any of them
// fails for a reason other than the database, or ctx ends.
//
// A job a consumer started runs under ctx, the process's life, not the
// connection's: a broker that drops mid-job must not cut the job short. The
// job finishes, its Ack goes nowhere, the broker redelivers the message, and
// the claim finds the job done and acknowledges it then.
func (k *Ket) serve(ctx context.Context, conn *amqp.Connection, logger *slog.Logger) {
	on, off := context.WithCancel(ctx)
	defer off()
	closed := conn.NotifyClose(make(chan *amqp.Error, 1))
	go func() {
		select {
		case <-closed:
			off()
		case <-on.Done():
		}
	}()
	var all sync.WaitGroup
	tick := k.Tick
	if tick <= 0 {
		tick = 250 * time.Millisecond
	}
	all.Add(1)
	go func() {
		defer all.Done()
		defer off()
		relay, err := NewRelay(k.Pool, conn, k.Topology)
		if err != nil {
			return
		}
		defer relay.Close()
		_ = relay.Run(on, tick)
	}()
	for _, q := range k.Queues {
		all.Add(1)
		go func(q string) {
			defer all.Done()
			for on.Err() == nil {
				err := ConsumeReady(on, conn, k.Topology, q, k.Concurrency, func(_ context.Context, m Message) error {
					return k.Handler(ctx, q, m)
				}, func(up bool) {
					if up {
						k.attached.Add(1)
					} else {
						k.attached.Add(-1)
					}
				})
				if !errors.Is(err, ErrTamDung) {
					// The channel or the connection went; take the whole
					// connection down and dial again.
					off()
					return
				}
				logger.Warn("job consumer paused: the database failed under a message", "queue", q)
				k.waitDatabase(on)
			}
		}(q)
	}
	all.Wait()
}

// waitDatabase returns once the database answers a ping, or ctx ends.
func (k *Ket) waitDatabase(ctx context.Context) {
	for ctx.Err() == nil {
		ping, cancel := context.WithTimeout(ctx, time.Second)
		err := k.Pool.Ping(ping)
		cancel()
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// ParseQueues reads MOBILE_WORKER_QUEUES: a comma list drawn from allowed, no
// repeats; empty means every allowed queue.
func ParseQueues(raw string, allowed []string) ([]string, error) {
	if raw == "" {
		return append([]string(nil), allowed...), nil
	}
	ok := map[string]bool{}
	for _, q := range allowed {
		ok[q] = true
	}
	var out []string
	seen := map[string]bool{}
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && raw[i] != ',' {
			continue
		}
		q := raw[start:i]
		start = i + 1
		if !ok[q] || seen[q] {
			return nil, fmt.Errorf("jobs: MOBILE_WORKER_QUEUES must list queues from %v without spaces or repeats, got %q", allowed, raw)
		}
		seen[q] = true
		out = append(out, q)
	}
	return out, nil
}
