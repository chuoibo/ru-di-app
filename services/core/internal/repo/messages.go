package repo

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Message is MessageRecord.
type Message struct {
	ID        string
	ContextID string
	AuthorID  *string
	Kind      string
	Body      *string
	ImageURL  *string
	Card      json.RawMessage
	CreatedAt time.Time
	ReplyToID *string
	DeletedAt *time.Time
}

// MessagePage is MessagePage.
type MessagePage struct {
	Messages []Message
	HasMore  bool
}

// MessageCursor is list_messages' before/after tuple.
type MessageCursor struct {
	CreatedAt time.Time
	ID        string
}

// Reaction is ReactionRecord.
type Reaction struct {
	MessageID string
	PersonID  string
	Kind      string
}

// ReadMark is ReadMarkRecord.
type ReadMark struct {
	ContextID         string
	PersonID          string
	LastReadMessageID string
	LastReadAt        time.Time
	UpdatedAt         time.Time
}

// Mapped column order of db.models.Message — SQLAlchemy SELECTs every one.
const messageColumns = `messages.id, messages.context_id, messages.author_id, messages.kind,
	messages.body, messages.image_url, messages.card, messages.reply_to_id, messages.deleted_at,
	messages.created_at`

func scanMessage(row pgx.Row) (*Message, error) {
	var m Message
	var card []byte
	err := row.Scan(&m.ID, &m.ContextID, &m.AuthorID, &m.Kind, &m.Body, &m.ImageURL,
		&card, &m.ReplyToID, &m.DeletedAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Card = jsonOrNone(card)
	m.CreatedAt = m.CreatedAt.UTC()
	m.DeletedAt = utcOptional(m.DeletedAt)
	return &m, nil
}

// CreateMessage is create_message: one INSERT with a client-side uuid4.
func (r Repository) CreateMessage(ctx context.Context, in MessageInput) (Message, error) {
	id, err := newUUID()
	if err != nil {
		return Message{}, err
	}
	now := pythonInstant(in.Now)
	_, err = r.Q.Exec(ctx,
		`INSERT INTO messages (id, context_id, author_id, kind, body, image_url, card, reply_to_id, deleted_at, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5::VARCHAR, $6::VARCHAR, $7::JSONB, $8::UUID, $9::TIMESTAMP WITH TIME ZONE, $10::TIMESTAMP WITH TIME ZONE)`,
		id, in.ContextID, in.AuthorID, in.Kind, in.Body, in.ImageURL, jsonbArg(in.Card), in.ReplyToID, nil, now)
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: id, ContextID: in.ContextID, AuthorID: in.AuthorID, Kind: in.Kind,
		Body: in.Body, ImageURL: in.ImageURL, Card: in.Card, CreatedAt: now,
		ReplyToID: in.ReplyToID,
	}, nil
}

// MessageInput is create_message's keyword arguments.
type MessageInput struct {
	ContextID string
	AuthorID  *string
	Kind      string
	Body      *string
	ImageURL  *string
	Card      json.RawMessage
	Now       time.Time
	ReplyToID *string
}

func jsonbArg(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

// GetMessage is get_message: session.get by id.
func (r Repository) GetMessage(ctx context.Context, messageID string) (*Message, error) {
	return scanMessage(r.Q.QueryRow(ctx,
		`SELECT `+messageColumns+` FROM messages WHERE messages.id = $1::UUID`, messageID))
}

// GetMessagesByIDs is get_messages_by_ids: one IN query, dict by id.
func (r Repository) GetMessagesByIDs(ctx context.Context, messageIDs []string) (map[string]Message, error) {
	out := map[string]Message{}
	wanted := make([]string, 0, len(messageIDs))
	for _, id := range messageIDs {
		if id != "" {
			wanted = append(wanted, id)
		}
	}
	if len(wanted) == 0 {
		return out, nil
	}
	rows, err := r.Q.Query(ctx,
		`SELECT `+messageColumns+` FROM messages WHERE messages.id IN (`+uuidPlaceholders(1, len(wanted))+`)`,
		uuidArgs(wanted)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		if m != nil {
			out[m.ID] = *m
		}
	}
	return out, rows.Err()
}

// SoftDeleteMessage is soft_delete_message: lock, refuse if already deleted,
// flip the row, drop reactions.
func (r Repository) SoftDeleteMessage(ctx context.Context, messageID string, now time.Time) (*Message, error) {
	m, err := scanMessage(r.Q.QueryRow(ctx,
		`SELECT `+messageColumns+` FROM messages WHERE messages.id = $1::UUID FOR UPDATE`, messageID))
	if err != nil || m == nil {
		return m, err
	}
	if m.Kind == "deleted" {
		return nil, &Conflict{Code: "MESSAGE_ALREADY_DELETED"}
	}
	deleted := pythonInstant(now)
	if err := r.execUpdate(ctx,
		`UPDATE messages SET kind = $1, body = $2, image_url = $3, card = $4, deleted_at = $5::TIMESTAMP WITH TIME ZONE
		  WHERE messages.id = $6::UUID`,
		"deleted", nil, nil, nil, deleted, messageID); err != nil {
		return nil, err
	}
	if _, err := r.Q.Exec(ctx, `DELETE FROM message_reactions WHERE message_id = $1::UUID`, messageID); err != nil {
		return nil, err
	}
	m.Kind = "deleted"
	m.Body, m.ImageURL, m.Card, m.DeletedAt = nil, nil, nil, &deleted
	return m, nil
}

// ListMessages is list_messages: keyset by (created_at, id), limit+1.
func (r Repository) ListMessages(ctx context.Context, contextID string, limit int, before, after *MessageCursor) (MessagePage, error) {
	sql := `SELECT ` + messageColumns + ` FROM messages WHERE messages.context_id = $1::UUID`
	args := []any{contextID}
	next := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	order := " ORDER BY messages.created_at DESC, messages.id DESC"
	if before != nil {
		sql += " AND (messages.created_at, messages.id) < (" + next(before.CreatedAt) +
			"::TIMESTAMP WITH TIME ZONE, " + next(before.ID) + "::UUID)"
	} else if after != nil {
		sql += " AND (messages.created_at, messages.id) > (" + next(after.CreatedAt) +
			"::TIMESTAMP WITH TIME ZONE, " + next(after.ID) + "::UUID)"
		order = " ORDER BY messages.created_at ASC, messages.id ASC"
	}
	sql += order + " LIMIT " + next(limit+1) + "::INTEGER"
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return MessagePage{}, err
	}
	defer rows.Close()
	var found []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return MessagePage{}, err
		}
		found = append(found, *m)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, err
	}
	page := MessagePage{HasMore: len(found) > limit, Messages: []Message{}}
	end := pythonSliceEnd(len(found), limit)
	page.Messages = append(page.Messages, found[:end]...)
	return page, nil
}

// AddReaction is add_reaction: lock the message, then insert or no-op on unique.
func (r Repository) AddReaction(ctx context.Context, messageID, personID, kind string, now time.Time) (bool, error) {
	m, err := scanMessage(r.Q.QueryRow(ctx,
		`SELECT `+messageColumns+` FROM messages WHERE messages.id = $1::UUID FOR UPDATE`, messageID))
	if err != nil {
		return false, err
	}
	if m == nil {
		return false, &Conflict{Code: "MESSAGE_NOT_FOUND"}
	}
	if m.Kind == "deleted" {
		return false, &Conflict{Code: "MESSAGE_DELETED"}
	}
	id, err := newUUID()
	if err != nil {
		return false, err
	}
	const savepoint = "sa_savepoint_1"
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+savepoint); err != nil {
		return false, err
	}
	_, err = r.Q.Exec(ctx,
		`INSERT INTO message_reactions (id, message_id, person_id, kind, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5::TIMESTAMP WITH TIME ZONE)`,
		id, messageID, personID, kind, pythonInstant(now))
	if err != nil {
		if _, rb := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+savepoint); rb != nil {
			return false, rb
		}
		if pg := integrityViolation(err); pg != nil && strings.HasPrefix(pg.Code, "23") {
			return false, nil
		}
		return false, err
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+savepoint); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveReaction is remove_reaction.
func (r Repository) RemoveReaction(ctx context.Context, messageID, personID, kind string) (bool, error) {
	var id string
	err := r.Q.QueryRow(ctx,
		`SELECT message_reactions.id FROM message_reactions
		  WHERE message_reactions.message_id = $1::UUID
		    AND message_reactions.person_id = $2::UUID
		    AND message_reactions.kind = $3`,
		messageID, personID, kind).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = r.Q.Exec(ctx, `DELETE FROM message_reactions WHERE message_reactions.id = $1::UUID`, id)
	return err == nil, err
}

// ListReactions is list_reactions: every reaction on these messages, oldest first.
func (r Repository) ListReactions(ctx context.Context, messageIDs []string) ([]Reaction, error) {
	if len(messageIDs) == 0 {
		return []Reaction{}, nil
	}
	rows, err := r.Q.Query(ctx,
		`SELECT message_reactions.message_id, message_reactions.person_id, message_reactions.kind
		   FROM message_reactions
		  WHERE message_reactions.message_id IN (`+uuidPlaceholders(1, len(messageIDs))+`)
		  ORDER BY message_reactions.created_at, message_reactions.id`,
		uuidArgs(messageIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reaction{}
	for rows.Next() {
		var row Reaction
		if err := rows.Scan(&row.MessageID, &row.PersonID, &row.Kind); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SetReadMark is set_read_mark: insert or move forward only.
func (r Repository) SetReadMark(ctx context.Context, contextID, personID string, message Message, now time.Time) (ReadMark, error) {
	var mark ReadMark
	err := r.Q.QueryRow(ctx,
		`SELECT context_read_marks.context_id, context_read_marks.person_id,
		        context_read_marks.last_read_message_id, context_read_marks.last_read_at,
		        context_read_marks.updated_at
		   FROM context_read_marks
		  WHERE context_read_marks.context_id = $1::UUID AND context_read_marks.person_id = $2::UUID`,
		contextID, personID).
		Scan(&mark.ContextID, &mark.PersonID, &mark.LastReadMessageID, &mark.LastReadAt, &mark.UpdatedAt)
	updated := pythonInstant(now)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = r.Q.Exec(ctx,
			`INSERT INTO context_read_marks (context_id, person_id, last_read_message_id, last_read_at, updated_at)
			 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::TIMESTAMP WITH TIME ZONE, $5::TIMESTAMP WITH TIME ZONE)`,
			contextID, personID, message.ID, message.CreatedAt, updated)
		if err != nil {
			return ReadMark{}, err
		}
		return ReadMark{ContextID: contextID, PersonID: personID, LastReadMessageID: message.ID,
			LastReadAt: message.CreatedAt, UpdatedAt: updated}, nil
	}
	if err != nil {
		return ReadMark{}, err
	}
	mark.LastReadAt = mark.LastReadAt.UTC()
	mark.UpdatedAt = mark.UpdatedAt.UTC()
	if messageNewer(message, mark) {
		if err := r.execUpdate(ctx,
			`UPDATE context_read_marks SET last_read_message_id = $1::UUID, last_read_at = $2::TIMESTAMP WITH TIME ZONE, updated_at = $3::TIMESTAMP WITH TIME ZONE
			  WHERE context_read_marks.context_id = $4::UUID AND context_read_marks.person_id = $5::UUID`,
			message.ID, message.CreatedAt, updated, contextID, personID); err != nil {
			return ReadMark{}, err
		}
		mark.LastReadMessageID, mark.LastReadAt, mark.UpdatedAt = message.ID, message.CreatedAt, updated
	}
	return mark, nil
}

func messageNewer(message Message, mark ReadMark) bool {
	if message.CreatedAt.After(mark.LastReadAt) {
		return true
	}
	if message.CreatedAt.Before(mark.LastReadAt) {
		return false
	}
	return bytes.Compare(uuidBytes(message.ID), uuidBytes(mark.LastReadMessageID)) > 0
}

func uuidBytes(id string) []byte {
	hexes := make([]byte, 0, 32)
	for i := 0; i < len(id); i++ {
		if id[i] != '-' {
			hexes = append(hexes, id[i])
		}
	}
	out, err := hex.DecodeString(string(hexes))
	if err != nil {
		return nil
	}
	return out
}
