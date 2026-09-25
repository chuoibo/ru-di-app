package aistream

import (
	"context"
	"net/http"
	"time"
)

// FollowOptions bound one SSE connection.
type FollowOptions struct {
	// Reconcile re-reads Redis even without a wake, repairing a missed one.
	Reconcile time.Duration
	// Ping keeps proxies from closing an idle stream.
	Ping time.Duration
	// MaxDuration closes the stream with ket_noi_lai; the client resumes.
	MaxDuration time.Duration
	// WriteTimeout bounds each write; a client that cannot keep up is closed.
	WriteTimeout time.Duration
	// Batch is the most entries read per wake.
	Batch int64
}

// DefaultFollow is a 1 s reconcile, 15 s ping, 180 s lifetime, 10 s write bound.
func DefaultFollow() FollowOptions {
	return FollowOptions{Reconcile: time.Second, Ping: 15 * time.Second, MaxDuration: 180 * time.Second,
		WriteTimeout: 10 * time.Second, Batch: 128}
}

// Follow streams key to w as SSE from after (exclusive) until a terminal event,
// the lifetime bound, the client leaving, or authorize returning false (checked
// on every reconcile tick; the membership may be revoked mid-stream). It never
// holds a database connection while it waits. The caller has already written
// the response headers' content type and checked who may read key.
func Follow(ctx context.Context, w http.ResponseWriter, s *Stream, hub *Hub, key, after string, opt FollowOptions, authorize func(context.Context) bool) error {
	rc := http.NewResponseController(w)
	write := func(f func() error) error {
		_ = rc.SetWriteDeadline(time.Now().Add(opt.WriteTimeout))
		if err := f(); err != nil {
			return err
		}
		return rc.Flush()
	}
	if err := write(func() error { return WriteRetry(w, 2000) }); err != nil {
		return err
	}
	wake, cancel := hub.Subscribe(key)
	defer cancel()
	reconcile := time.NewTicker(opt.Reconcile)
	defer reconcile.Stop()
	ping := time.NewTicker(opt.Ping)
	defer ping.Stop()
	deadline := time.NewTimer(opt.MaxDuration)
	defer deadline.Stop()
	drain := func() (done bool, err error) {
		for {
			events, err := s.Read(ctx, key, after, opt.Batch)
			if err != nil {
				return false, err
			}
			for _, e := range events {
				if err = write(func() error { return WriteEvent(w, e) }); err != nil {
					return false, err
				}
				after = e.ID
				if e.Kind.Terminal() {
					return true, nil
				}
			}
			if int64(len(events)) < opt.Batch {
				return false, nil
			}
		}
	}
	if after != "" {
		if ended, err := s.EndedAt(ctx, key, after); err != nil || ended {
			return err
		}
	}
	if done, err := drain(); done || err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deadline.C:
			return write(func() error { return WriteEvent(w, Event{Kind: KetNoiLai, Data: []byte(`{"sau_ms":0}`)}) })
		case <-ping.C:
			if err := write(func() error { return WritePing(w) }); err != nil {
				return err
			}
		case <-reconcile.C:
			if authorize != nil && !authorize(ctx) {
				return write(func() error { return WriteEvent(w, Event{Kind: ThuHoi}) })
			}
			if done, err := drain(); done || err != nil {
				return err
			}
		case <-wake:
			if done, err := drain(); done || err != nil {
				return err
			}
		}
	}
}

// ResumeFrom reads the client's position: the Last-Event-ID header a native
// EventSource sends, or ?after= for clients that cannot set headers. An
// invalid value is ignored (start from the beginning), never trusted.
func ResumeFrom(r *http.Request) string {
	for _, v := range []string{r.Header.Get("Last-Event-ID"), r.URL.Query().Get("after")} {
		if ValidID(v) {
			return v
		}
	}
	return ""
}
