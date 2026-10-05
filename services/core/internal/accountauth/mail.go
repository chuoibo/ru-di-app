package accountauth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type mailPayload struct{ Email, Code, Purpose string }

// errMaybeSent is a failure after the whole message reached the server: it
// may have been accepted, so the day's allowance it took is kept.
var errMaybeSent = errors.New("smtp_delivery_failed")

type Sender interface {
	Send(context.Context, mailPayload) error
}
type smtpSender struct{ host, port, user, password, from string }

func smtpFromEnv(getenv func(string) string) (Sender, error) {
	s := smtpSender{getenv("MOBILE_EMAIL_SMTP_HOST"), getenv("MOBILE_EMAIL_SMTP_PORT"), getenv("MOBILE_EMAIL_SMTP_USER"), getenv("MOBILE_EMAIL_SMTP_PASSWORD"), getenv("MOBILE_EMAIL_FROM")}
	if s.port == "" {
		s.port = "587"
	}
	from, err := mail.ParseAddress(s.from)
	if s.host == "" || s.user == "" || s.password == "" || err != nil || from.Address != s.from || strings.ContainsAny(s.host+s.port+s.user+s.from, "\r\n") {
		return nil, fmt.Errorf("managed auth requires authenticated TLS SMTP and a verified sender")
	}
	return s, nil
}
func (s smtpSender) Send(ctx context.Context, p mailPayload) error {
	if _, err := Email(p.Email); err != nil {
		return err
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.host, s.port))
	if err != nil {
		return fmt.Errorf("smtp_connect_failed")
	}
	defer conn.Close()
	deadline := time.Now().Add(8 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	tlsConfig := &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
	if s.port == "465" {
		secured := tls.Client(conn, tlsConfig)
		if err = secured.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("smtp_tls_failed")
		}
		conn = secured
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp_greeting_failed")
	}
	defer c.Close()
	if s.port != "465" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp_tls_required")
		}
		if err = c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp_tls_failed")
		}
	}
	if err = c.Auth(smtp.PlainAuth("", s.user, s.password, s.host)); err != nil {
		return fmt.Errorf("smtp_auth_failed")
	}
	if err = c.Mail(s.from); err != nil {
		return fmt.Errorf("smtp_sender_failed")
	}
	if err = c.Rcpt(p.Email); err != nil {
		return fmt.Errorf("smtp_recipient_failed")
	}
	data, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp_data_failed")
	}
	subject := mime.QEncoding.Encode("UTF-8", "Mã xác minh Rủ Đi")
	body := "From: " + s.from + "\r\nTo: " + p.Email + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\nMã xác minh của bạn: " + p.Code + "\r\nMã hết hạn sau 5 phút. Không chia sẻ mã với người khác.\r\nNếu bạn không yêu cầu, hãy bỏ qua email này.\r\n"
	if _, err = data.Write([]byte(body)); err != nil {
		return fmt.Errorf("smtp_write_failed")
	}
	if err = data.Close(); err != nil {
		// The message went out whole; whether the server kept it is unknown.
		return errMaybeSent
	}
	_ = c.Quit()
	return nil
}

// RunMail claims with SKIP LOCKED. A bounded lease allows another replica to retry
// after a crash; delivery is at-least-once, while proof consumption is exactly once.
func (h *Handler) RunMail(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	clean := time.NewTicker(time.Minute)
	defer clean.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			h.deliverOne(ctx)
		case <-clean.C:
			cleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := h.pool.Exec(cleanup, `DELETE FROM account_challenges WHERE id IN(SELECT id FROM account_challenges WHERE expires_at<clock_timestamp()-interval '1 day' ORDER BY expires_at LIMIT 1000)`)
			if err != nil {
				h.mailWarning("cleanup", 0)
			}
			h.mailSummary(cleanup)
			cancel()
		}
	}
}
func (h *Handler) deliverOne(ctx context.Context) {
	if h.cfg.Sender == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	lease, err := newID()
	if err != nil {
		return
	}
	var id string
	var payload []byte
	var attempts int
	err = h.pool.QueryRow(ctx, `WITH due AS(SELECT o.id FROM account_mail_outbox o JOIN account_challenges c ON c.id=o.challenge_id WHERE o.done_at IS NULL AND o.expires_at>clock_timestamp()+interval '12 seconds' AND c.consumed_at IS NULL AND o.attempts<8 AND o.next_attempt_at<=clock_timestamp() AND (o.lease_until IS NULL OR o.lease_until<clock_timestamp()) ORDER BY o.next_attempt_at LIMIT 1 FOR UPDATE OF o SKIP LOCKED) UPDATE account_mail_outbox o SET lease_id=$1,lease_until=clock_timestamp()+interval '30 seconds',attempts=o.attempts+1 FROM due WHERE o.id=due.id RETURNING o.id::text,o.payload_cipher,o.attempts`, lease).Scan(&id, &payload, &attempts)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.mailWarning("claim", 0)
		}
		return
	}
	var mail mailPayload
	err = h.cfg.Vault.open("mail:"+id, payload, &mail)
	if err == nil {
		err = h.reserveMail(ctx, mail.Purpose)
	}
	if err == nil {
		err = h.cfg.Sender.Send(ctx, mail)
		if err != nil && !errors.Is(err, errMaybeSent) {
			// Refused before the message left: give the allowance back.
			if undoErr := h.cfg.Limits.Undo(ctx, h.rateKey("smtp-day", mailDay())); undoErr != nil {
				h.mailWarning("quota", attempts)
			}
		}
	}
	if err == nil {
		_, updateErr := h.pool.Exec(ctx, `UPDATE account_mail_outbox SET done_at=clock_timestamp(),payload_cipher='\x',lease_until=NULL WHERE id=$1 AND lease_id=$2`, id, lease)
		if updateErr != nil {
			h.mailWarning("acknowledgement", attempts)
		}
	} else {
		h.mailWarning("delivery", attempts)
		retry := time.Duration(1<<min(attempts, 6)) * time.Second
		_, updateErr := h.pool.Exec(ctx, `UPDATE account_mail_outbox SET lease_until=NULL,next_attempt_at=clock_timestamp()+($3::double precision*interval '1 second') WHERE id=$1 AND lease_id=$2`, id, lease, retry.Seconds())
		if updateErr != nil {
			h.mailWarning("reschedule", attempts)
		}
	}
}

func mailDay() string { return time.Now().UTC().Format("2006-01-02") }
func (h *Handler) mailPerDay() int {
	if h.cfg.MailPerDay > 0 {
		return h.cfg.MailPerDay
	}
	return 300
}

// mailOpen refuses new codes once the provider's day is spent, instead of
// issuing codes whose mail can never leave; the answer is the same whether
// or not the address has an account. Registrations stop at four fifths of
// the day, so a flood of sign-ups cannot take recovery codes with it.
func (h *Handler) mailOpen(ctx context.Context, kind string) error {
	if h.cfg.Limits == nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	sent, err := h.cfg.Limits.Count(ctx, h.rateKey("smtp-day", mailDay()))
	if err != nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	limit := h.mailPerDay()
	if kind == "register" {
		limit -= limit / 5
	}
	if sent >= limit {
		return problem(503, "mail_unavailable")
	}
	return nil
}

// reserveMail takes one message from the provider's day before it is sent
// (audit 2026-10-05, CURRENT-AUTH-MAIL-01). Counting and spending used to be
// separate steps on either side of Send, so replicas draining the queue
// together could all see room and all send; Allow counts and checks in one
// step. A registration may use four fifths of the day, at send time too, so
// queued sign-ups cannot take the recovery codes' share.
func (h *Handler) reserveMail(ctx context.Context, purpose string) error {
	if h.cfg.Limits == nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	key := h.rateKey("smtp-day", mailDay())
	limit := h.mailPerDay()
	if purpose == "register" {
		limit -= limit / 5
	}
	allowed, err := h.cfg.Limits.Allow(ctx, key, limit, 26*time.Hour)
	if err != nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	if !allowed {
		_ = h.cfg.Limits.Undo(ctx, key)
		return problem(503, "mail_unavailable")
	}
	return nil
}

// Only fixed categories and aggregate counts enter logs; provider errors may
// contain addresses or message bodies and are deliberately never logged.
func (h *Handler) mailWarning(stage string, attempt int) {
	if h.cfg.Logger != nil {
		h.cfg.Logger.Warn("account mail worker failed", "stage", stage, "attempt", attempt)
	}
}
func (h *Handler) mailSummary(ctx context.Context) {
	if h.cfg.Logger == nil {
		return
	}
	var pending, expired, exhausted int
	err := h.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE o.expires_at>clock_timestamp() AND o.attempts<8), count(*) FILTER (WHERE o.expires_at<=clock_timestamp()), count(*) FILTER (WHERE o.attempts>=8 AND o.expires_at>clock_timestamp()) FROM account_mail_outbox o JOIN account_challenges c ON c.id=o.challenge_id WHERE o.done_at IS NULL AND c.consumed_at IS NULL`).Scan(&pending, &expired, &exhausted)
	if err != nil {
		h.mailWarning("summary", 0)
		return
	}
	h.cfg.Logger.Info("account mail outbox", "pending", pending, "expired", expired, "exhausted", exhausted)
}
