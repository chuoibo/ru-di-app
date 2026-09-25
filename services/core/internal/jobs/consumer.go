package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler runs one job message. Return nil once the job's terminal
// transaction committed (or the claim found nothing to do); ErrMalformed to
// dead-letter the message; ErrTamDung when the database, not the message,
// failed; any other error to have it redelivered, at most DeliveryLimit times
// before the broker dead-letters it.
type Handler func(ctx context.Context, m Message) error

// ErrTamDung is what a handler returns (wrapped or joined) when the database
// failed under a message: the message is requeued and this consumer stops
// taking more, so the process falls back to its Postgres poller until the
// database answers again (design 02 §4 step 3). Consume returns it.
var ErrTamDung = errors.New("jobs: stop consuming until the database is healthy")

// Consume runs handler over queue with at most concurrency jobs at once, until
// ctx ends or the channel closes. A message is acknowledged only after its
// handler returned.
func Consume(ctx context.Context, conn *amqp.Connection, t Topology, queue string, concurrency int, handler Handler) error {
	return ConsumeReady(ctx, conn, t, queue, concurrency, handler, nil)
}

// ConsumeReady is Consume that reports attachment: attached(true) once the
// broker registered the consumer, attached(false) as soon as deliveries stop
// -- before the jobs already running finish -- so a caller can tell
// "consuming" from "dialled" and from "draining".
//
// The handler's context is ctx, not the subscription's: when one message hits
// ErrTamDung the subscription is cancelled, but the jobs already running
// finish (or are released) on their own terms. Deliveries already buffered
// when that happens are requeued untouched.
func ConsumeReady(ctx context.Context, conn *amqp.Connection, t Topology, queue string, concurrency int, handler Handler, attached func(bool)) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err = t.Declare(ch); err != nil {
		return err
	}
	if err = ch.Qos(concurrency, 0, false); err != nil {
		return err
	}
	sub, stopSub := context.WithCancel(ctx)
	defer stopSub()
	deliveries, err := ch.ConsumeWithContext(sub, t.Queue(queue), "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	if attached != nil {
		attached(true)
	}
	var running sync.WaitGroup
	var paused atomic.Bool
	slots := make(chan struct{}, concurrency)
	defer running.Wait()
	for d := range deliveries {
		if paused.Load() {
			_ = d.Nack(false, true)
			continue
		}
		slots <- struct{}{}
		running.Add(1)
		go func(d amqp.Delivery) {
			defer running.Done()
			defer func() { <-slots }()
			m, err := Decode(d.Body)
			if err != nil {
				_ = d.Reject(false)
				return
			}
			switch err = handler(ctx, m); {
			case err == nil:
				_ = d.Ack(false)
			case errors.Is(err, ErrMalformed):
				_ = d.Reject(false)
			case errors.Is(err, ErrTamDung):
				_ = d.Nack(false, true)
				paused.Store(true)
				stopSub()
			default:
				_ = d.Nack(false, true)
			}
		}(d)
	}
	if attached != nil {
		attached(false)
	}
	running.Wait()
	if paused.Load() {
		return ErrTamDung
	}
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("jobs: consumer channel closed")
}
