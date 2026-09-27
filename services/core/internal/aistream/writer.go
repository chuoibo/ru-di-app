package aistream

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// The data of the events a job writes (contract §3).
type (
	// TrangThaiData is trang_thai{cau}.
	TrangThaiData struct {
		Cau string `json:"cau"`
	}
	// DeltaData is delta{p,text}: text of answer part p, released by the
	// engine's output guard window.
	DeltaData struct {
		P    int    `json:"p"`
		Text string `json:"text"`
	}
	// ThatBaiData is that_bai{code}.
	ThatBaiData struct {
		Code string `json:"code"`
	}
)

// LogValue keeps answer text out of any log line a delta reaches: only its
// size is logged (design 02 §5.1).
func (d DeltaData) LogValue() slog.Value {
	return slog.StringValue("[" + strconv.Itoa(len(d.Text)) + " bytes]")
}

// WriterOptions are one job's stream: which key it writes, and how.
type WriterOptions struct {
	Key    string
	MaxLen int64
	// Inv is set when Key is a room key: every entry names its invocation,
	// and Tin and SoTin, when set, the message it answers and how many
	// shared messages it reads (Nhan).
	Inv   string
	Tin   string
	SoTin int
	// SauChotThoi holds every content event (phan, delta) back until SauChot:
	// a room key's text reaches the whole room, so none of it may go before
	// the job's ending committed (contract §4.1: a group's deltas come after
	// its card is posted, just before xong). A content event before then is
	// refused and does not settle anything: SauChot still opens the stream.
	SauChotThoi bool
	// ExpireAt is the job's sharing window end.
	ExpireAt time.Time
	// Flush is how long a delta waits for more before it goes (60 ms);
	// FlushBytes sends earlier once that much text waits (256 bytes).
	Flush      time.Duration
	FlushBytes int
	// Timeout bounds one write, the XADDs and the PUBLISH in one pipeline
	// (300 ms). MaxFailures consecutive failed writes make the writer a
	// no-op for the rest of the job (2): the job still ends in Postgres, and
	// the reader falls back to polling.
	Timeout     time.Duration
	MaxFailures int
	// BeforeContent runs once, synchronously, before the job's first content
	// event (phan or delta) is queued: the worker marks first_token_at under
	// its lease there. An error drops every content event of the job -- text
	// the database does not know went out must not go out.
	BeforeContent func() error
}

// Writer is one job's side of its stream. Events keep their order; deltas of
// one part are merged while they wait. Safe for concurrent use.
type Writer struct {
	s   *Stream
	opt WriterOptions

	flushMu sync.Mutex // one write in flight, in order

	mu      sync.Mutex
	pending []Entry
	bytes   int
	timer   *time.Timer
	// noiDung is 0 before any content, 1 once content may go, -1 once
	// BeforeContent refused it.
	noiDung  int
	failures int
	dead     bool
	closed   bool
}

// NewWriter opens the writer of one job's stream.
func (s *Stream) NewWriter(opt WriterOptions) *Writer {
	if opt.Flush <= 0 {
		opt.Flush = 60 * time.Millisecond
	}
	if opt.FlushBytes <= 0 {
		opt.FlushBytes = 256
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 300 * time.Millisecond
	}
	if opt.MaxFailures <= 0 {
		opt.MaxFailures = 2
	}
	return &Writer{s: s, opt: opt}
}

// jobKinds are the events a job writes. hello, thu_hoi and ket_noi_lai belong
// to one reader's connection, never to the stream.
var jobKinds = map[Kind]bool{TrangThai: true, Phan: true, Delta: true, LamLai: true, Xong: true, ThatBai: true, Huy: true}

// Ghi queues one event and reports whether it was taken. A status, a part, a
// restart and an ending are written before Ghi returns; a delta waits up to
// Flush for more of its part. After an ending nothing more is taken; a
// restart after content is refused (lam_lai only before the first content).
func (w *Writer) Ghi(kind Kind, data any) bool {
	if !jobKinds[kind] {
		return false
	}
	w.mu.Lock()
	// A writer that gave up takes nothing more: in particular no content, so
	// BeforeContent never marks a job whose text reaches no reader.
	closed := w.closed || w.dead
	w.mu.Unlock()
	if closed || ((kind == Phan || kind == Delta) && !w.choNoiDung()) {
		return false
	}
	w.mu.Lock()
	if w.closed || (kind == LamLai && w.noiDung == 1) {
		w.mu.Unlock()
		return false
	}
	merged := false
	if d, ok := data.(DeltaData); ok && kind == Delta {
		if n := len(w.pending); n > 0 && w.pending[n-1].Kind == Delta {
			if last, ok := w.pending[n-1].Data.(DeltaData); ok && last.P == d.P {
				last.Text += d.Text
				w.pending[n-1].Data = last
				merged = true
			}
		}
		w.bytes += len(d.Text)
	}
	if !merged {
		w.pending = append(w.pending, Entry{Kind: kind, Data: data})
	}
	now := kind != Delta || w.bytes >= w.opt.FlushBytes
	if kind.Terminal() {
		w.closed = true
	}
	if !now && w.timer == nil {
		w.timer = time.AfterFunc(w.opt.Flush, w.flush)
	}
	w.mu.Unlock()
	if now {
		w.flush()
	}
	return true
}

// choNoiDung runs BeforeContent once and says whether content may go.
func (w *Writer) choNoiDung() bool {
	w.mu.Lock()
	state := w.noiDung
	w.mu.Unlock()
	if state != 0 {
		return state == 1
	}
	if w.opt.SauChotThoi {
		return false
	}
	// Serialized with writes, so two first deltas never both run it.
	w.flushMu.Lock()
	defer w.flushMu.Unlock()
	w.mu.Lock()
	state = w.noiDung
	w.mu.Unlock()
	if state != 0 {
		return state == 1
	}
	ok := w.opt.BeforeContent == nil || w.opt.BeforeContent() == nil
	w.mu.Lock()
	if ok {
		w.noiDung = 1
	} else {
		w.noiDung = -1
	}
	w.mu.Unlock()
	return ok
}

// flush writes what waits, in one pipeline. A failed write is put back in
// front of what came since, so no event is lost or reordered by one failure;
// MaxFailures in a row and the writer gives up.
func (w *Writer) flush() {
	w.flushMu.Lock()
	defer w.flushMu.Unlock()
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	batch := w.pending
	w.pending, w.bytes = nil, 0
	dead := w.dead
	w.mu.Unlock()
	if dead || len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), w.opt.Timeout)
	_, err := w.s.appendBatch(ctx, w.opt.Key, w.opt.MaxLen, Nhan{Inv: w.opt.Inv, Tin: w.opt.Tin, SoTin: w.opt.SoTin}, batch, w.opt.ExpireAt)
	cancel()
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		w.failures = 0
		return
	}
	w.failures++
	if w.failures >= w.opt.MaxFailures {
		w.dead = true
		w.pending, w.bytes = nil, 0
		return
	}
	w.pending = append(batch, w.pending...)
	w.bytes = 0
	for _, e := range w.pending {
		if d, ok := e.Data.(DeltaData); ok {
			w.bytes += len(d.Text)
		}
	}
}

// Dong writes whatever still waits and stops the writer.
func (w *Writer) Dong() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.flush()
}

// Chet reports whether the writer gave up after failed writes.
func (w *Writer) Chet() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dead
}

// ChanNoiDung settles, atomically with the first content, whether this
// job's stream carries content: it reports true when content already went
// (BeforeContent accepted it), and otherwise refuses every content event
// from now on and reports false. A stopping worker asks it to tell a job it
// may hand back (no content, and none can follow) from one that must finish
// (design 02 §4 step 9): asking CoNoiDung instead would race a first delta
// whose BeforeContent is marking the row right then.
func (w *Writer) ChanNoiDung() bool {
	w.flushMu.Lock()
	defer w.flushMu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.noiDung == 0 {
		w.noiDung = -1
	}
	return w.noiDung == 1
}

// SauChot lets content go without BeforeContent: the job's ending already
// committed, so no lease is left to mark and no second worker can take the
// job over (design 02 §5.1: a finished card's text goes after its commit,
// just before xong). It reports false when content was refused earlier.
func (w *Writer) SauChot() bool {
	w.flushMu.Lock()
	defer w.flushMu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.noiDung == 0 {
		w.noiDung = 1
	}
	return w.noiDung == 1
}

// CoNoiDung reports whether content went into the stream (BeforeContent
// accepted it).
func (w *Writer) CoNoiDung() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.noiDung == 1
}
