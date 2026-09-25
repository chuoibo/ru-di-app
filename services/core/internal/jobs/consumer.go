package jobs

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler runs one job message. Return nil once the job's terminal
// transaction committed (or the claim found nothing to do); ErrMalformed to
// dead-letter the message; ErrTamDung when the database, not the message,
// failed; any other error to have it redelivered, at most DeliveryLimit times
// before the broker dead-letters it.
type Handler func(ctx context.Context, m Message) error

// ErrTamDung is what a handler returns (wrapped or joined) when the database
// failed under a message: this consumer stops taking more, so the process
// falls back to its Postgres poller until the database answers again (design
// 02 §4 step 3), and the message is run again then.
var ErrTamDung = errors.New("jobs: stop consuming until the database is healthy")

// Hooks are what a consumer reports to its caller and asks of it. Every field
// may be nil.
type Hooks struct {
	// Attached hears attachment: true once the broker registered the
	// consumer, false as soon as deliveries stop -- before the jobs already
	// running finish -- so a caller can tell "consuming" from "dialled", from
	// "draining" and from "paused".
	Attached func(up bool)
	// Paused is called when a handler returned ErrTamDung, once the consumer
	// stopped taking messages, and returns once the database answers again
	// (or ctx ends). Nil waits one second.
	Paused func(ctx context.Context)
	// DeadLettered hears each message this consumer sends to the dead-letter
	// queue, by id only (design 02 §7): a body that does not decode (its
	// broker message id when it has the relay's form, "" otherwise), a
	// handler's ErrMalformed, or the redelivery that crosses DeliveryLimit.
	// Never the body.
	DeadLettered func(id string)
}

// Consume runs handler over queue with at most concurrency jobs at once, until
// ctx ends or the channel closes. A message is acknowledged only after its
// handler returned.
func Consume(ctx context.Context, conn *amqp.Connection, t Topology, queue string, concurrency int, handler Handler) error {
	return ConsumeReady(ctx, conn, t, queue, concurrency, handler, Hooks{})
}

// ConsumeReady is Consume with hooks. It returns once ctx ends (nil) or the
// channel or connection fails (an error); a paused consumer does not return.
//
// A pause gives nothing back to the queue. When a handler returns ErrTamDung
// the consumer cancels its subscription, keeps that message and every one
// the broker had already delivered -- unacknowledged, on this channel --
// waits in hooks.Paused, runs the messages it kept, and subscribes again.
// Measured on RabbitMQ 3.12: a Nack with requeue, a channel closed and a
// connection dropped with the message unacknowledged each add one to the
// message's x-delivery-count, and at DeliveryLimit the broker dead-letters
// it. Requeueing on every pause let five short database stalls (a lock, not
// an outage) send a healthy job's message to the dead-letter queue; keeping
// the messages costs nothing against the limit. A stop while paused closes
// the channel, and the kept messages go back to the queue then, counted once.
//
// The handler's context is ctx, not the subscription's: a pause cancels the
// subscription, while the jobs already running finish (or are released) on
// their own terms.
func ConsumeReady(ctx context.Context, conn *amqp.Connection, t Topology, queue string, concurrency int, handler Handler, hooks Hooks) error {
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
	c := consumer{queue: queue, handler: handler, hooks: hooks, concurrency: concurrency}
	for {
		held, err := c.subscribe(ctx, ch, t)
		if err != nil {
			return err
		}
		if held == nil {
			if ctx.Err() != nil {
				return nil
			}
			return errors.New("jobs: consumer channel closed")
		}
		for len(held) > 0 {
			c.pause(ctx)
			if ctx.Err() != nil {
				return nil
			}
			held = c.runAll(ctx, held)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

type consumer struct {
	queue       string
	handler     Handler
	hooks       Hooks
	concurrency int
}

// outcome is what became of one delivery.
type outcome int

const (
	settled outcome = iota // acknowledged, rejected or requeued
	kept                   // the database failed under it: kept for later
)

// subscribe consumes until the channel closes, ctx ends, or a handler pauses
// the consumer. held is nil unless it paused; then it holds every message the
// consumer kept, which may be none if the pause raced a clean stop.
func (c *consumer) subscribe(ctx context.Context, ch *amqp.Channel, t Topology) (held []amqp.Delivery, err error) {
	sub, stopSub := context.WithCancel(ctx)
	defer stopSub()
	deliveries, err := ch.ConsumeWithContext(sub, t.Queue(c.queue), "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	if c.hooks.Attached != nil {
		c.hooks.Attached(true)
	}
	var running sync.WaitGroup
	var paused atomic.Bool
	var mu sync.Mutex
	keep := func(d amqp.Delivery) {
		mu.Lock()
		held = append(held, d)
		mu.Unlock()
	}
	slots := make(chan struct{}, c.concurrency)
	for d := range deliveries {
		if paused.Load() {
			// Delivered before the cancel reached the broker: kept, not run
			// and not requeued.
			keep(d)
			continue
		}
		slots <- struct{}{}
		running.Add(1)
		go func(d amqp.Delivery) {
			defer running.Done()
			defer func() { <-slots }()
			if c.settle(ctx, d) == kept {
				keep(d)
				if !paused.Swap(true) {
					stopSub()
				}
			}
		}(d)
	}
	if c.hooks.Attached != nil {
		c.hooks.Attached(false)
	}
	running.Wait()
	if !paused.Load() {
		return nil, nil
	}
	if held == nil {
		held = []amqp.Delivery{}
	}
	return held, nil
}

// pause waits for the database, through hooks.Paused.
func (c *consumer) pause(ctx context.Context) {
	if c.hooks.Paused != nil {
		c.hooks.Paused(ctx)
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
	}
}

// runAll runs the kept messages again, as many at once as the consumer may,
// and returns those the database failed under again. After the first such
// failure the rest are kept without being tried: the database is down again.
func (c *consumer) runAll(ctx context.Context, held []amqp.Delivery) []amqp.Delivery {
	var running sync.WaitGroup
	var mu sync.Mutex
	var again []amqp.Delivery
	var failed atomic.Bool
	slots := make(chan struct{}, c.concurrency)
	for _, d := range held {
		slots <- struct{}{}
		if failed.Load() || ctx.Err() != nil {
			<-slots
			mu.Lock()
			again = append(again, d)
			mu.Unlock()
			continue
		}
		running.Add(1)
		go func(d amqp.Delivery) {
			defer running.Done()
			defer func() { <-slots }()
			if c.settle(ctx, d) == kept {
				failed.Store(true)
				mu.Lock()
				again = append(again, d)
				mu.Unlock()
			}
		}(d)
	}
	running.Wait()
	return again
}

// settle runs one delivery and acknowledges, rejects or requeues it -- or
// keeps it when the database failed under it.
func (c *consumer) settle(ctx context.Context, d amqp.Delivery) outcome {
	m, err := Decode(d.Body)
	if err != nil {
		_ = d.Reject(false)
		c.deadLettered(idOf(d.MessageId))
		return settled
	}
	switch err = c.handler(ctx, m); {
	case err == nil:
		_ = d.Ack(false)
	case errors.Is(err, ErrMalformed):
		_ = d.Reject(false)
		c.deadLettered(m.ID(c.queue))
	case errors.Is(err, ErrTamDung):
		return kept
	default:
		// The broker dead-letters a message returned once more than
		// DeliveryLimit times; this return is that one when the count the
		// broker stamped on the delivery already reached the limit.
		if deliveryCount(d) >= DeliveryLimit {
			c.deadLettered(m.ID(c.queue))
		}
		_ = d.Nack(false, true)
	}
	return settled
}

func (c *consumer) deadLettered(id string) {
	if c.hooks.DeadLettered != nil {
		c.hooks.DeadLettered(id)
	}
}

// relayID is the form of every message id the relay publishes (Message.ID).
var relayID = regexp.MustCompile(`^[a-z][a-z.]{0,31}:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}:[0-9]{1,19}$`)

// idOf repeats a broker message id only when it has the relay's form: a body
// that did not decode was not published by the relay, and its id is whatever
// its publisher wrote.
func idOf(raw string) string {
	if relayID.MatchString(raw) {
		return raw
	}
	return ""
}

// deliveryCount is the x-delivery-count a quorum queue stamps on a
// redelivery: how many times the message went back before this delivery.
func deliveryCount(d amqp.Delivery) int64 {
	switch v := d.Headers["x-delivery-count"].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	}
	return 0
}
