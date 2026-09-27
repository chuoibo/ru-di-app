package aistream

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// FollowOptions bound one SSE connection.
type FollowOptions struct {
	// Reconcile re-reads Redis even without a wake, repairing a missed one.
	Reconcile time.Duration
	// Ping keeps proxies from closing an idle stream; hello tells the client
	// its period.
	Ping time.Duration
	// MaxDuration closes the stream with ket_noi_lai; the client resumes.
	MaxDuration time.Duration
	// WriteTimeout bounds each write; a client that cannot keep up is closed.
	WriteTimeout time.Duration
	// Batch is the most entries read per wake.
	Batch int64
	// Inv, when set, keeps only that invocation's entries of the key: the
	// requester of a legacy-lane group job reads the room key this way.
	Inv string
	// Authorize says whether the reader may still read; it is asked every
	// AuthorizeEvery (every reconcile tick when zero). False writes thu_hoi
	// and closes; an error closes without an event, and the client resumes.
	// Its events are the job row's ending, built from the row it read (nil
	// while the job runs): see Ending.
	Authorize      func(context.Context) (bool, []Event, error)
	AuthorizeEvery time.Duration
	// Ending is the job row's ending as the authorization at open read it
	// (nil while the job is queued or running). The row is the truth: when
	// it has ended and the stream still holds no terminal event for the
	// reader a whole reconcile tick later -- the writer gave up, the worker
	// died after content, a queued job was cancelled -- these events are
	// written without ids and the connection closes, instead of idling to
	// MaxDuration and sending the client round again (review of slice 11,
	// finding 2). Authorize's events arm the same, mid-stream.
	Ending []Event
	// Empty is asked once, for a reader starting from the beginning, when the
	// stream holds nothing for it yet. The events it returns are written
	// without ids (they are not in the stream, so no resume position may name
	// them); done closes the connection after them.
	Empty func() (events []Event, done bool)
	// Stop, once closed, ends the stream with ket_noi_lai: the process is
	// stopping and the client should come back elsewhere.
	Stop <-chan struct{}
}

// DefaultFollow is a 1 s reconcile, 15 s ping, 180 s lifetime, 10 s write
// bound, and authorization re-checked every 10 s.
func DefaultFollow() FollowOptions {
	return FollowOptions{Reconcile: time.Second, Ping: 15 * time.Second, MaxDuration: 180 * time.Second,
		WriteTimeout: 10 * time.Second, Batch: 128, AuthorizeEvery: 10 * time.Second}
}

// HelloData is hello{nhip_ms}: the ping period, so a client can tell a quiet
// stream from a dead one.
type HelloData struct {
	NhipMs int64 `json:"nhip_ms"`
}

// Follow streams key to w as SSE from after (exclusive) until a terminal
// event, the lifetime bound, the client leaving, the process stopping, or the
// reader losing its right to read. The body opens with `retry: 2000` and
// hello. It never holds a database connection while it waits: Authorize opens
// its own short transaction each time. The caller has written the response
// headers and checked who may read key.
func Follow(ctx context.Context, w http.ResponseWriter, s *Stream, hub *Hub, key, after string, opt FollowOptions) error {
	rc := http.NewResponseController(w)
	write := func(f func() error) error {
		_ = rc.SetWriteDeadline(time.Now().Add(opt.WriteTimeout))
		if err := f(); err != nil {
			return err
		}
		return rc.Flush()
	}
	writeEvent := func(e Event) error { return write(func() error { return WriteEvent(w, e) }) }
	if err := write(func() error { return WriteRetry(w, 2000) }); err != nil {
		return err
	}
	hello, _ := json.Marshal(HelloData{NhipMs: opt.Ping.Milliseconds()})
	if err := writeEvent(Event{Kind: Hello, Data: hello}); err != nil {
		return err
	}
	// Subscribed before the first read: an append between the read and the
	// subscription would otherwise wait for the reconcile tick.
	wake, cancel := hub.Subscribe(key)
	defer cancel()
	fromStart := after == ""
	if !fromStart {
		if ended, err := s.EndedAt(ctx, key, after, opt.Inv); err != nil || ended {
			return err
		}
	}
	drain := func() (done bool, n int, err error) {
		for {
			events, last, full, err := s.read(ctx, key, after, opt.Inv, opt.Batch)
			if err != nil {
				return false, n, err
			}
			if len(events) > gopTren {
				events = gopDelta(events)
			}
			for _, e := range events {
				if err = writeEvent(e); err != nil {
					return false, n, err
				}
				n++
				if e.Kind.Terminal() {
					return true, n, nil
				}
			}
			after = last
			if !full {
				return false, n, nil
			}
		}
	}
	done, n, err := drain()
	if done || err != nil {
		return err
	}
	// The row's ending, waiting one full reconcile tick for the stream's own
	// terminal event (which carries an id the client can resume from).
	hangXong, daCho := opt.Ending, false
	writeEnding := func() error {
		for _, e := range hangXong {
			e.ID = ""
			if err := writeEvent(e); err != nil {
				return err
			}
		}
		return nil
	}
	if fromStart && n == 0 && opt.Empty != nil {
		events, stop := opt.Empty()
		for _, e := range events {
			e.ID = ""
			if err := writeEvent(e); err != nil {
				return err
			}
		}
		if stop {
			return nil
		}
	}
	reconcile := time.NewTicker(opt.Reconcile)
	defer reconcile.Stop()
	ping := time.NewTicker(opt.Ping)
	defer ping.Stop()
	deadline := time.NewTimer(opt.MaxDuration)
	defer deadline.Stop()
	var authorize <-chan time.Time
	if opt.Authorize != nil {
		every := opt.AuthorizeEvery
		if every <= 0 {
			every = opt.Reconcile
		}
		t := time.NewTicker(every)
		defer t.Stop()
		authorize = t.C
	}
	reconnect := func() error {
		return writeEvent(Event{Kind: KetNoiLai, Data: json.RawMessage(`{"sau_ms":0}`)})
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-opt.Stop:
			return reconnect()
		case <-deadline.C:
			return reconnect()
		case <-ping.C:
			if err := write(func() error { return WritePing(w) }); err != nil {
				return err
			}
		case <-authorize:
			ok, ending, err := opt.Authorize(ctx)
			if err != nil {
				return err
			}
			if !ok {
				return writeEvent(Event{Kind: ThuHoi})
			}
			if hangXong == nil && ending != nil {
				hangXong = ending
			}
		case <-reconcile.C:
			if done, _, err := drain(); done || err != nil {
				return err
			}
			if hangXong != nil {
				if daCho {
					return writeEnding()
				}
				daCho = true
			}
		case <-wake:
			if done, _, err := drain(); done || err != nil {
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

// gopTren is how many entries may wait for one reader before its deltas are
// merged (design 02 §5.2): a reader that fell that far behind gets the text
// in fewer, longer events.
const gopTren = 64

// gopDelta merges each run of consecutive deltas of the same part (and the
// same invocation) into one event carrying the run's joined text under the
// id of its last entry, so a client resuming from it skips exactly what it
// received. Every other event passes as it is.
func gopDelta(events []Event) []Event {
	out := make([]Event, 0, len(events))
	// run is the delta being grown at the end of out.
	var run *DeltaData
	chot := func() {
		if run == nil {
			return
		}
		if raw, err := json.Marshal(run); err == nil {
			out[len(out)-1].Data = raw
		}
		run = nil
	}
	for _, e := range events {
		var d DeltaData
		if e.Kind != Delta || json.Unmarshal(e.Data, &d) != nil {
			chot()
			out = append(out, e)
			continue
		}
		if run != nil && run.P == d.P && out[len(out)-1].Inv == e.Inv {
			run.Text += d.Text
			out[len(out)-1].ID = e.ID
			continue
		}
		chot()
		run = &DeltaData{P: d.P, Text: d.Text}
		out = append(out, e)
	}
	chot()
	return out
}
