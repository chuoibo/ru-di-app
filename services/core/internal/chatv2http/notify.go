package chatv2http

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listen receives commit-time wakeups from every replica. The durable stream
// also reconciles periodically, so an interrupted LISTEN cannot lose events.
// No event contents or credentials are included in notifications or logs.
func (h *Handler) Listen(ctx context.Context, pool *pgxpool.Pool) {
	for ctx.Err() == nil {
		conn, err := pool.Acquire(ctx)
		if err == nil {
			_, err = conn.Exec(ctx, "LISTEN rudi_chat_v2")
			for err == nil {
				var notificationConversation string
				n, waitErr := conn.Conn().WaitForNotification(ctx)
				err = waitErr
				if n != nil {
					notificationConversation = n.Payload
				}
				if notificationConversation != "" {
					h.Wake(notificationConversation)
				}
			}
			// A connection with LISTEN state never goes back into the query pool.
			_ = conn.Conn().Close(context.Background())
			conn.Release()
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
