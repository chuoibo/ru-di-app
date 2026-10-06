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
	// The token: free, ours, or held by an installation that was revoked (an
	// app reinstalled after signing out), which gives it up. A live other
	// installation keeps it (it never changes owner between two installs).
	var owner string
	var ownerRevoked bool
	err = tx.QueryRow(ctx, `SELECT installation_id::text, revoked_at IS NOT NULL FROM push_devices WHERE expo_push_token=$1 FOR UPDATE`, r.Token).Scan(&owner, &ownerRevoked)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return "", err
	case owner == r.InstallationID:
	case ownerRevoked:
		if _, err := tx.Exec(ctx, `DELETE FROM push_devices WHERE installation_id=$1`, owner); err != nil {
			return "", err
		}
	default:
		return "", ErrConflict
	}
	// The installation: new, ours, or revoked (another person signed out of
	// this phone). A live installation of another person is never taken over
	// (security review 05/10).
	var holder string
	var holderRevoked bool
	err = tx.QueryRow(ctx, `SELECT person_id::text, revoked_at IS NOT NULL FROM push_devices WHERE installation_id=$1 FOR UPDATE`, r.InstallationID).Scan(&holder, &holderRevoked)
	if err == nil && holder != person && !holderRevoked {
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

// Enqueue wakes the other members of each room whose chat v2 log moved past
// push's cursor: one wake per person per room, the newest sequence wins, the
// author of the newest event is not woken. It walks the rooms by their
// sequence counter and reads one event per room by primary key -- never a
// scan of the log -- reading chatv2's tables and writing only its own.
func (s Store) Enqueue(ctx context.Context, limit int) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5758)`); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT c.context_id::text, coalesce(p.sequence, 0), c.last_sequence
		  FROM chat_v2_conversations c LEFT JOIN push_chat_cursor p ON p.conversation_id = c.context_id
		 WHERE c.last_sequence > coalesce(p.sequence, 0) LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type moved struct {
		room       string
		from, upto int64
	}
	var rooms []moved
	for rows.Next() {
		var m moved
		if err := rows.Scan(&m.room, &m.from, &m.upto); err != nil {
			rows.Close()
			return 0, err
		}
		rooms = append(rooms, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	queued := 0
	for _, m := range rooms {
		var seq int64
		var actor string
		err := tx.QueryRow(ctx, `SELECT sequence, actor_id::text FROM chat_v2_events
			WHERE context_id=$1 AND sequence>$2 AND sequence<=$3 AND kind IN ('envelope','commit')
			ORDER BY sequence DESC LIMIT 1`, m.room, m.from, m.upto).Scan(&seq, &actor)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Only marks moved the counter: nothing to wake anyone for.
		case err != nil:
			return 0, err
		default:
			tag, err := tx.Exec(ctx, `INSERT INTO push_outbox(id,person_id,conversation_id,sequence)
				SELECT gen_random_uuid(), m.person_id, $1, $2 FROM memberships m
				 WHERE m.context_id=$1 AND m.state='active' AND m.left_at IS NULL AND m.person_id<>$3
				   AND EXISTS (SELECT 1 FROM push_devices d JOIN account_sessions s ON s.id=d.session_id
				                AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()
				               WHERE d.person_id=m.person_id AND d.revoked_at IS NULL)
				ON CONFLICT (person_id, conversation_id) WHERE sent_at IS NULL DO UPDATE SET sequence=greatest(push_outbox.sequence, EXCLUDED.sequence)`,
				m.room, seq, actor)
			if err != nil {
				return 0, err
			}
			queued += int(tag.RowsAffected())
		}
		if _, err := tx.Exec(ctx, `INSERT INTO push_chat_cursor(conversation_id,sequence) VALUES($1,$2)
			ON CONFLICT (conversation_id) DO UPDATE SET sequence=greatest(push_chat_cursor.sequence, EXCLUDED.sequence)`, m.room, m.upto); err != nil {
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

const (
	// leaseFor is how long a claimed wake waits for its hand-off result.
	leaseFor = time.Minute
	// handOffBudget bounds one round of hand-offs, whatever the service does.
	handOffBudget = 20 * time.Second
)

// SendPending claims up to limit pending wakes in one short transaction
// (counting the attempt), hands them off one wake at a time outside any
// transaction within handOffBudget, and records the results in another short
// one (security review 05/10: network I/O never holds row locks). A token the
// push service refuses fails its own wake alone; a wake whose hand-off did not
// finish is retried when its lease lapses; five attempts, then it is dropped.
func (s Store) SendPending(ctx context.Context, sender Sender, limit int) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM push_outbox WHERE sent_at < clock_timestamp()-interval '1 day' OR (sent_at IS NULL AND attempts >= 5 AND leased_until < clock_timestamp())`); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `WITH claimed AS (
		  UPDATE push_outbox SET attempts=attempts+1, leased_until=clock_timestamp()+$2::interval
		   WHERE id IN (SELECT id FROM push_outbox WHERE sent_at IS NULL AND attempts < 5
		                  AND (leased_until IS NULL OR leased_until < clock_timestamp())
		                ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED)
		  RETURNING id, person_id, conversation_id, sequence, created_at)
		SELECT c.id::text, c.conversation_id::text, c.sequence, d.expo_push_token
		  FROM claimed c
		  JOIN push_devices d ON d.person_id=c.person_id AND d.revoked_at IS NULL
		  JOIN account_sessions s ON s.id=d.session_id AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()
		 ORDER BY c.created_at, c.id, d.id`, limit, leaseFor.String())
	if err != nil {
		return 0, err
	}
	var order []string
	byWake := map[string][]Message{}
	for rows.Next() {
		var id, room, token string
		var seq int64
		if err := rows.Scan(&id, &room, &seq, &token); err != nil {
			rows.Close()
			return 0, err
		}
		if _, seen := byWake[id]; !seen {
			order = append(order, id)
		}
		byWake[id] = append(byWake[id], Message{To: token, Title: "Rủ Đi", Body: WakeText, Priority: "high", Sound: "default",
			Data: map[string]string{"t": "chat", "c": room, "s": fmt.Sprint(seq)}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	budget, cancel := context.WithTimeout(ctx, handOffBudget)
	defer cancel()
	var delivered []string
	var failures []error
	sent := 0
	for _, id := range order {
		if budget.Err() != nil {
			break
		}
		if err := sender.Send(budget, byWake[id]); err != nil {
			failures = append(failures, err)
			continue
		}
		delivered = append(delivered, id)
		sent += len(byWake[id])
	}
	if len(delivered) > 0 {
		if _, err := s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE push_outbox SET sent_at=clock_timestamp(), leased_until=NULL WHERE id = ANY($1::uuid[])`, delivered); err != nil {
			return sent, err
		}
	}
	return sent, errors.Join(failures...)
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
