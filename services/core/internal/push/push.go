// Package push wakes a person's app when something new waits in a room, and
// says nothing else (ADR-0024, ADR-0041 §11, ADR-0057 §7): the payload names
// the room and the sequence, never a sender, a title or a word of content. The
// app fetches and, on the v2 lane, decrypts on the device.
package push

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid   = errors.New("push_invalid")
	ErrForbidden = errors.New("push_forbidden")
	ErrConflict  = errors.New("push_token_taken")
)

var expoToken = regexp.MustCompile(`^(Exponent|Expo)PushToken\[[A-Za-z0-9_-]{10,180}\]$`)
var uuidText = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

type Store struct{ Pool *pgxpool.Pool }

// Registration is one installation asking to be woken.
type Registration struct {
	InstallationID string `json:"installation_id"`
	Platform       string `json:"platform"`
	Token          string `json:"expo_push_token"`
}

// Register binds an installation's token to the bearer's person and session.
// The same installation re-registering moves to the new token and session; a
// token already held by another installation is refused (it never changes
// owner between two installs).
func (s Store) Register(ctx context.Context, sessionDigest []byte, r Registration) (string, error) {
	if !uuidText.MatchString(r.InstallationID) || (r.Platform != "android" && r.Platform != "ios") || !expoToken.MatchString(r.Token) {
		return "", ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var session, person string
	err = tx.QueryRow(ctx, `SELECT s.id::text, s.person_id::text FROM account_sessions s JOIN people p ON p.id=s.person_id
		WHERE s.token_digest=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() AND p.deleted_at IS NULL
		FOR SHARE OF s, p`, sessionDigest).Scan(&session, &person)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrForbidden
	}
	if err != nil {
		return "", err
	}
	var owner string
	err = tx.QueryRow(ctx, `SELECT installation_id::text FROM push_devices WHERE expo_push_token=$1 FOR UPDATE`, r.Token).Scan(&owner)
	if err == nil && owner != r.InstallationID {
		return "", ErrConflict
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `INSERT INTO push_devices(id,person_id,session_id,installation_id,platform,expo_push_token)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT (installation_id) DO UPDATE SET person_id=EXCLUDED.person_id, session_id=EXCLUDED.session_id,
		  platform=EXCLUDED.platform, expo_push_token=EXCLUDED.expo_push_token, last_seen_at=clock_timestamp(), revoked_at=NULL
		RETURNING id::text`, id, person, session, r.InstallationID, r.Platform, r.Token).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrConflict
		}
		return "", err
	}
	return id, tx.Commit(ctx)
}

// Unregister stops waking one installation of the bearer's person.
func (s Store) Unregister(ctx context.Context, sessionDigest []byte, installation string) error {
	if !uuidText.MatchString(installation) {
		return ErrInvalid
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE push_devices d SET revoked_at=clock_timestamp()
		FROM account_sessions s WHERE s.token_digest=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()
		  AND d.person_id=s.person_id AND d.installation_id=$2 AND d.revoked_at IS NULL`, sessionDigest, installation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

// Enqueue reads the chat v2 log past each room's cursor and queues one wake
// per other member per room (the newest sequence wins). It reads chatv2's
// tables and writes only its own.
func (s Store) Enqueue(ctx context.Context, limit int) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5758)`); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT e.context_id::text, e.sequence, e.actor_id::text
		  FROM chat_v2_events e LEFT JOIN push_chat_cursor c ON c.conversation_id = e.context_id
		 WHERE e.kind IN ('envelope','commit') AND e.sequence > coalesce(c.sequence, 0)
		 ORDER BY e.created_at, e.context_id, e.sequence LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type event struct {
		room, actor string
		seq         int64
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.room, &e.seq, &e.actor); err != nil {
			rows.Close()
			return 0, err
		}
		events = append(events, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	queued := 0
	for _, e := range events {
		tag, err := tx.Exec(ctx, `INSERT INTO push_outbox(id,person_id,conversation_id,sequence)
			SELECT gen_random_uuid(), m.person_id, $1, $2 FROM memberships m
			 WHERE m.context_id=$1 AND m.state='active' AND m.left_at IS NULL AND m.person_id<>$3
			   AND EXISTS (SELECT 1 FROM push_devices d WHERE d.person_id=m.person_id AND d.revoked_at IS NULL)
			ON CONFLICT (person_id, conversation_id) WHERE sent_at IS NULL DO UPDATE SET sequence=greatest(push_outbox.sequence, EXCLUDED.sequence)`,
			e.room, e.seq, e.actor)
		if err != nil {
			return 0, err
		}
		queued += int(tag.RowsAffected())
		if _, err := tx.Exec(ctx, `INSERT INTO push_chat_cursor(conversation_id,sequence) VALUES($1,$2)
			ON CONFLICT (conversation_id) DO UPDATE SET sequence=greatest(push_chat_cursor.sequence, EXCLUDED.sequence)`, e.room, e.seq); err != nil {
			return 0, err
		}
	}
	return queued, tx.Commit(ctx)
}

// Message is one Expo push: no title, no body beyond a fixed sentence, no
// name; data names the room and sequence for the app to fetch.
type Message struct {
	To       string            `json:"to"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Data     map[string]string `json:"data"`
	Priority string            `json:"priority"`
	Sound    string            `json:"sound"`
}

// WakeText is the one sentence a push may say.
const WakeText = "Có tin nhắn mới"

// Sender hands messages to the push service.
type Sender interface {
	Send(ctx context.Context, messages []Message) error
}

// LogSender counts and logs nothing else: never a token, never a room.
type LogSender struct{ Logger *slog.Logger }

func (l LogSender) Send(_ context.Context, messages []Message) error {
	if l.Logger != nil {
		l.Logger.Info("push handed to log", "count", len(messages))
	}
	return nil
}

// ExpoSender posts to the Expo push API; an access token is optional.
type ExpoSender struct {
	Client      *http.Client
	URL         string
	AccessToken string
}

func (e ExpoSender) Send(ctx context.Context, messages []Message) error {
	body, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	url := e.URL
	if url == "" {
		url = "https://exp.host/--/api/v2/push/send"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("push transport: %T", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("push service answered %d", resp.StatusCode)
	}
	return nil
}

// SendPending hands up to limit pending wakes to the sender, one message per
// live device, and marks them sent in the same transaction; a failed hand-off
// leaves them pending with one more attempt (five, then dropped).
func (s Store) SendPending(ctx context.Context, sender Sender, limit int) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT o.id::text, o.conversation_id::text, o.sequence, d.expo_push_token
		  FROM push_outbox o JOIN push_devices d ON d.person_id=o.person_id AND d.revoked_at IS NULL
		 WHERE o.sent_at IS NULL AND o.attempts < 5 AND o.id IN (
		   SELECT id FROM push_outbox WHERE sent_at IS NULL AND attempts < 5 ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED)
		 ORDER BY o.created_at, d.id`, limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	seen := map[string]bool{}
	var messages []Message
	for rows.Next() {
		var id, room, token string
		var seq int64
		if err := rows.Scan(&id, &room, &seq, &token); err != nil {
			rows.Close()
			return 0, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
		messages = append(messages, Message{To: token, Title: "Rủ Đi", Body: WakeText, Priority: "high", Sound: "default",
			Data: map[string]string{"t": "chat", "c": room, "s": fmt.Sprint(seq)}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(messages) == 0 {
		return 0, tx.Commit(ctx)
	}
	if err := sender.Send(ctx, messages); err != nil {
		if _, uerr := tx.Exec(ctx, `UPDATE push_outbox SET attempts=attempts+1 WHERE id = ANY($1::uuid[])`, ids); uerr != nil {
			return 0, uerr
		}
		return 0, errors.Join(err, tx.Commit(ctx))
	}
	if _, err := tx.Exec(ctx, `UPDATE push_outbox SET sent_at=clock_timestamp() WHERE id = ANY($1::uuid[])`, ids); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM push_outbox WHERE sent_at < clock_timestamp()-interval '1 day' OR attempts >= 5`); err != nil {
		return 0, err
	}
	return len(messages), tx.Commit(ctx)
}

// Run enqueues and sends every interval until ctx ends.
func (s Store) Run(ctx context.Context, sender Sender, every time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if _, err := s.Enqueue(ctx, 500); err != nil && logger != nil {
			logger.Warn("push enqueue failed", "error", err.Error())
		}
		if _, err := s.SendPending(ctx, sender, 100); err != nil && logger != nil {
			logger.Warn("push send failed", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
