package jobs

import (
	"errors"
	"regexp"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Topology names the broker objects of one deployment namespace ("rudi" in
// production; tests use a fresh one each run).
type Topology struct{ ns string }

var nsPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func NewTopology(ns string) (Topology, error) {
	if !nsPattern.MatchString(ns) {
		return Topology{}, errors.New("jobs: invalid namespace")
	}
	return Topology{ns: ns}, nil
}

func (t Topology) Exchange() string          { return t.ns + ".jobs" }
func (t Topology) DeadExchange() string      { return t.ns + ".dlx" }
func (t Topology) Queue(q string) string     { return t.ns + "." + q }
func (t Topology) DeadQueue(q string) string { return t.ns + "." + q + ".dlq" }

// DeliveryLimit is how many times a message is delivered before it is dead-lettered.
const DeliveryLimit = 5

// Declare creates the exchanges and quorum queues. Idempotent: every
// connection runs it, so a fresh broker is usable after one connect.
func (t Topology) Declare(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(t.Exchange(), "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(t.DeadExchange(), "direct", true, false, false, false, nil); err != nil {
		return err
	}
	for _, q := range Queues {
		dead := amqp.Table{"x-queue-type": "quorum", "x-message-ttl": int64(7 * 24 * 3600 * 1000)}
		if _, err := ch.QueueDeclare(t.DeadQueue(q), true, false, false, false, dead); err != nil {
			return err
		}
		if err := ch.QueueBind(t.DeadQueue(q), q, t.DeadExchange(), false, nil); err != nil {
			return err
		}
		args := amqp.Table{
			"x-queue-type":              "quorum",
			"x-delivery-limit":          int64(DeliveryLimit),
			"x-dead-letter-exchange":    t.DeadExchange(),
			"x-dead-letter-routing-key": q,
		}
		if _, err := ch.QueueDeclare(t.Queue(q), true, false, false, false, args); err != nil {
			return err
		}
		if err := ch.QueueBind(t.Queue(q), q, t.Exchange(), false, nil); err != nil {
			return err
		}
	}
	return nil
}
