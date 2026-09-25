package chatlegacychange

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/httpapi/endpoint"
)

// BeforeWrite gives opt-in Go chat transactions a common lock order: the
// context sequence head before any message, reaction, vote or ballot lock.
// Capture triggers remain writer-neutral while ownership is unchanged.
func BeforeWrite(ctx context.Context, call *endpoint.Call) error {
	if call.Request.Method == http.MethodGet || call.Request.Method == http.MethodHead || call.Actor == nil {
		return nil
	}
	parts := strings.Split(strings.Trim(call.Request.URL.Path, "/"), "/")
	chatMutation := len(parts) >= 3 && parts[0] == "contexts" && (parts[2] == "messages" || parts[2] == "votes")
	chatMutation = chatMutation || len(parts) >= 2 && (parts[0] == "messages" || parts[0] == "votes")
	if !chatMutation {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	params := call.Scope.Params
	room := params["context_id"]
	message, vote := params["message_id"], params["vote_id"]
	if room == "" && message == "" && vote == "" {
		return nil
	}
	tx, err := call.Unit.Tx(ctx)
	if err != nil {
		return err
	}
	if room == "" {
		if message != "" {
			err = tx.QueryRow(ctx, `SELECT context_id FROM messages WHERE id=$1::uuid`, message).Scan(&room)
		} else {
			err = tx.QueryRow(ctx, `SELECT context_id FROM votes WHERE id=$1::uuid`, vote).Scan(&room)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	var member bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships WHERE context_id=$1::uuid AND person_id=$2::uuid AND state='active' AND left_at IS NULL)`, room, call.Actor.ID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_legacy_change_heads(context_id,sequence) VALUES($1::uuid,0) ON CONFLICT(context_id) DO NOTHING`, room)
	if err != nil {
		return err
	}
	var n int64
	return tx.QueryRow(ctx, `SELECT sequence FROM chat_legacy_change_heads WHERE context_id=$1::uuid FOR UPDATE`, room).Scan(&n)
}
