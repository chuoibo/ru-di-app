package jobs

import (
	"context"
	"errors"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler runs one job message. Return nil once the job's terminal
// transaction committed (or the claim found nothing to do); ErrMalformed to
// dead-letter the message; any other error to have it redelivered, at most
// DeliveryLimit times before the broker dead-letters it.
type Handler func(ctx context.Context, m Message) error

// Consume runs handler over queue with at most concurrency jobs at once, until
// ctx ends or the channel closes. A message is acknowledged only after its
// handler returned.
func Consume(ctx context.Context, conn *amqp.Connection, t Topology, queue string, concurrency int, handler Handler) error {
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
	deliveries, err := ch.ConsumeWithContext(ctx, t.Queue(queue), "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	var running sync.WaitGroup
	slots := make(chan struct{}, concurrency)
	defer running.Wait()
	for d := range deliveries {
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
			default:
				_ = d.Nack(false, true)
			}
		}(d)
	}
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("jobs: consumer channel closed")
}
