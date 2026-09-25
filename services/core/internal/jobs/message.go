package jobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Queues are the lanes of work. Priority comes from separate queues with their
// own consumer concurrency, not from message priority.
var Queues = []string{"ai.group", "ai.nep", "memory", "notify"}

// Message is the whole body a job message carries: which row, which enqueue.
type Message struct {
	V   int    `json:"v"`
	Ref string `json:"ref"`
	Seq int64  `json:"seq"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ErrMalformed marks a body no consumer should ever retry.
var ErrMalformed = errors.New("jobs: malformed message")

// Encode renders the body; at most 128 bytes.
func (m Message) Encode() ([]byte, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Decode accepts exactly the fields Encode writes.
func Decode(body []byte) (Message, error) {
	if len(body) > 128 {
		return Message{}, ErrMalformed
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var m Message
	if err := dec.Decode(&m); err != nil || dec.More() {
		return Message{}, ErrMalformed
	}
	if err := m.check(); err != nil {
		return Message{}, ErrMalformed
	}
	return m, nil
}

func (m Message) check() error {
	if m.V != 1 || !uuidPattern.MatchString(m.Ref) || m.Seq < 0 {
		return ErrMalformed
	}
	return nil
}

// ID is the AMQP message id: one enqueue of one row in one queue.
func (m Message) ID(queue string) string { return fmt.Sprintf("%s:%s:%d", queue, m.Ref, m.Seq) }
