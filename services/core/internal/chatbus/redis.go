// Package chatbus distributes metadata-only wake hints. PostgreSQL events and
// each replica's reconciliation cursor remain the delivery authority.
package chatbus

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"mobile/services/core/internal/chatv2"
)

type Bus struct {
	client  *redis.Client
	channel string
}
type Hint struct {
	Conversation string `json:"conversation"`
	Sequence     int64  `json:"sequence"`
}

// Postgres uses the durable outbox without requiring a separate broker.
// NOTIFY runs in the relay transaction, never in the message writer's commit.
func Postgres() *Bus { return &Bus{} }

var namespacePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)

// New rejects credential-bearing parse errors rather than returning URL details.
func New(rawURL, namespace string) (*Bus, error) {
	if !namespacePattern.MatchString(namespace) {
		return nil, errors.New("invalid chat bus namespace")
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid chat bus configuration")
	}
	options.DialTimeout = 500 * time.Millisecond
	options.ReadTimeout = 500 * time.Millisecond
	options.WriteTimeout = 500 * time.Millisecond
	options.ContextTimeoutEnabled = true
	options.MaxRetries = -1
	options.PoolSize = 4
	options.DisableIdentity = true
	return &Bus{client: redis.NewClient(options), channel: "rudi:" + namespace + ":chat-v2"}, nil
}
func (b *Bus) Close() error {
	if b.client == nil {
		return nil
	}
	return b.client.Close()
}

// Listen reconnects on failure. A missed notification is intentionally not
// replayed here: the gateway's database reconciliation repairs that gap.
func (b *Bus) Listen(ctx context.Context, wake func(string), ready chan<- struct{}) {
	announced := false
	for ctx.Err() == nil {
		subscription := b.client.Subscribe(ctx, b.channel)
		stopWatch := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = subscription.Close()
			case <-stopWatch:
			}
		}()
		_, err := subscription.Receive(ctx)
		if err == nil && !announced && ready != nil {
			close(ready)
			announced = true
		}
		for err == nil && ctx.Err() == nil {
			var m *redis.Message
			m, err = subscription.ReceiveMessage(ctx)
			if err != nil {
				break
			}
			if len(m.Payload) > 256 {
				continue
			}
			var h Hint
			if json.Unmarshal([]byte(m.Payload), &h) == nil && chatv2.ValidID(h.Conversation) && h.Sequence > 0 {
				wake(h.Conversation)
			}
		}
		close(stopWatch)
		_ = subscription.Close()
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Flush claims a bounded outbox batch. A crash after Publish but before Commit
// produces a duplicate hint, never a lost durable event. Published is a relay
// checkpoint only; it says nothing about delivery to individual subscribers.
func (b *Bus) Flush(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT context_id::text,sequence FROM chat_v2_outbox WHERE published_at IS NULL ORDER BY context_id,sequence FOR UPDATE SKIP LOCKED LIMIT 256`)
	if err != nil {
		return 0, err
	}
	hints := map[string]int64{}
	ids := []string{}
	seqs := []int64{}
	for rows.Next() {
		var h Hint
		if err = rows.Scan(&h.Conversation, &h.Sequence); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, h.Conversation)
		seqs = append(seqs, h.Sequence)
		if h.Sequence > hints[h.Conversation] {
			hints[h.Conversation] = h.Sequence
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, tx.Commit(ctx)
	}
	rooms := make([]string, 0, len(hints))
	if b.client != nil {
		pipeline := b.client.Pipeline()
		for room, sequence := range hints {
			raw, _ := json.Marshal(Hint{Conversation: room, Sequence: sequence})
			pipeline.Publish(ctx, b.channel, raw)
		}
		if _, err = pipeline.Exec(ctx); err != nil {
			return 0, errors.New("chat bus publish unavailable")
		}
	}
	for room := range hints {
		rooms = append(rooms, room)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_notify('rudi_chat_v2', room) FROM unnest($1::text[]) AS room`, rooms); err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_v2_outbox o SET published_at=clock_timestamp() FROM unnest($1::uuid[],$2::bigint[]) AS batch(context_id,sequence) WHERE o.context_id=batch.context_id AND o.sequence=batch.sequence`, ids, seqs)
	if err != nil {
		return 0, err
	}
	return len(ids), tx.Commit(ctx)
}

func (b *Bus) Relay(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = b.Flush(ctx, pool)
		}
	}
}
