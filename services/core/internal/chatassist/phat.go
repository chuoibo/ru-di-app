package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/guard"
	"mobile/services/core/internal/aistream"
)

// Streaming (slice 11; design 02 §5.1). A job the worker claims writes what
// happens to it into one Redis stream, which the SSE routes read (sse.go):
//
//   - trang_thai when the job is claimed, and the engine's statuses;
//   - delta, only as the engine's output guard window releases it (Nếp), or
//     the group's finished text through the same window as one final delta;
//   - lam_lai when the job goes back to the queue before any content;
//   - xong only after the transaction that ends the job has committed:
//     {message_id} for a group job, {text,chips,nguon} for Nếp, whose answer
//     otherwise lives in the sealed result only;
//   - that_bai{code} after a failed ending, huy after a cancellation.
//
// Postgres stays the truth: a lost stream only sends the reader back to
// polling. MOBILE_REDIS_URL empty means no stream at all (h.stream nil).

// WithStream turns streaming on. stream is written by this process's
// workers; hub (nil in `core work`, which serves no route) feeds this
// process's SSE readers; stop closes when the process stops, and every open
// stream then ends with ket_noi_lai.
func (h *Handler) WithStream(stream *aistream.Stream, hub *aistream.Hub, stop <-chan struct{}) *Handler {
	h.stream, h.hub, h.dungSSE = stream, hub, stop
	h.nhipPhat = aiharness.NhipPhat
	return h
}

// nhipPhat paces a finished text that did not stream (the brain's) as it is
// released after its commit, like the engine's verified answer
// (aiharness.NhipPhat, guard.PhatTheoNhip): the reader's client shows it
// progressively. At most guard.TranNhip per text.
func (h *Handler) nhipSauChot() time.Duration { return h.nhipPhat }

// khoa is the one place a job's stream key is chosen (design 02 §5.1). Nếp
// (scope me) writes its invocation key. A group job in the legacy lane writes
// its room's key, each entry naming the invocation (inv), so the whole room
// can read one stream. Every other group job -- lane v2, or a lane this
// binary does not know -- writes its invocation key: a v2 (E2EE) job never
// writes the room key. The SSE routes read through the same function.
func khoa(keys aistream.Keys, j work) (key string, maxLen int64, inv string, err error) {
	if j.scope != scopeMe && j.lane == laneLegacy {
		key, err = keys.Room(j.conversation)
		return key, aistream.MaxLenRoom, j.id, err
	}
	key, err = keys.Invocation(j.id)
	return key, aistream.MaxLenInvocation, "", err
}

// maPhong is the code a room's stream carries for a failure. Every output
// guard code becomes the generic ai_tu_choi there (design 02 §3.6): the room
// never learns why an answer was stopped.
func maPhong(code string) string {
	switch cau.Ma(code) {
	case cau.TraLoiBiChan, cau.NepKhongChamTien, cau.NepLuiManTien:
		return maChanChung
	}
	return code
}

// maChanChung is the one code a group job ends with when the output guard stops
// its answer: the room and the job row see only that (design 01 §3.3). Its
// sentence is in aiharness/cau (bangNhom), which the app is held to.
const maChanChung = string(cau.TuChoiNhom)

// luongViec is one job's stream. Nil when streaming is off; every method is
// then a no-op. It is the engine's Sink for a job the Go engine runs.
type luongViec struct {
	w     *aistream.Writer
	phong bool
	mu    sync.Mutex
	cau   string
}

var _ aiharness.Sink = (*luongViec)(nil)

// moLuong opens j's stream, or returns nil when streaming is off. Before the
// first content leaves, first_token_at is set under j's lease (design 02 §4
// step 7): from then on the job is never released or claimed again, so no
// second worker writes the answer over what readers already saw. ctx is the
// job's: a job whose lease is gone writes no content.
func (h *Handler) moLuong(ctx context.Context, j work) *luongViec {
	if h.stream == nil {
		return nil
	}
	key, maxLen, inv, err := khoa(h.stream.Keys, j)
	if err != nil {
		return nil
	}
	return &luongViec{phong: inv != "", w: h.stream.NewWriter(aistream.WriterOptions{
		Key: key, MaxLen: maxLen, Inv: inv, ExpireAt: j.shareExpires,
		BeforeContent: func() error { return h.danhDauNoiDung(ctx, j) },
	})}
}

// errKhongCoNoiDung: first_token_at could not be set under this lease.
var errKhongCoNoiDung = errors.New("chatassist: the job's lease no longer covers its first content")

// danhDauNoiDung sets first_token_at, synchronously, before the first content
// of j goes out. Zero rows (the lease is gone) or an error refuses the
// content.
func (h *Handler) danhDauNoiDung(ctx context.Context, j work) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	tag, err := h.nhipPool().Exec(ctx, `UPDATE chat_ai_invocations SET first_token_at=clock_timestamp() WHERE id=$1 AND lease_id=$2 AND first_token_at IS NULL`, j.id, j.lease)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errKhongCoNoiDung
	}
	return nil
}

// TrangThai writes trang_thai{cau}, once per change. n is not in the event
// (design 01 §2).
func (l *luongViec) TrangThai(ma cau.TrangThai, _ int) { l.trangThai(string(ma)) }

func (l *luongViec) trangThai(ma string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	same := l.cau == ma
	l.cau = ma
	l.mu.Unlock()
	if !same {
		l.w.Ghi(aistream.TrangThai, aistream.TrangThaiData{Cau: ma})
	}
}

// Phan writes phan{kind,json}.
func (l *luongViec) Phan(_ int, kind aiharness.PhanKind, v json.RawMessage) {
	if l == nil {
		return
	}
	l.w.Ghi(aistream.Phan, map[string]any{"kind": string(kind), "json": v})
}

// Delta writes delta{p,text}. Only the output guard window calls it.
func (l *luongViec) Delta(p int, text string) {
	if l == nil {
		return
	}
	l.w.Ghi(aistream.Delta, aistream.DeltaData{P: p, Text: text})
}

// LamLai writes lam_lai; the writer refuses it once content went out.
func (l *luongViec) LamLai() {
	if l == nil {
		return
	}
	l.w.Ghi(aistream.LamLai, struct{}{})
}

// nhaThe releases a posted card's text through the output guard's window,
// after the card committed (so no BeforeContent: the job has ended and no
// lease is left to mark), paced nhip apart.
func (l *luongViec) nhaThe(card []byte, nhip time.Duration) {
	if l == nil || !l.w.SauChot() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*guard.TranNhip)
	defer cancel()
	chuTheNhom(ctx, l, card, nhip)
}

// nhaChu releases a sealed Nếp answer that did not stream (the brain's)
// through the output guard's window, paced nhip apart, after its commit. The
// job's own context is not used: after the commit its heartbeat finds no
// lease and cancels it.
func (l *luongViec) nhaChu(text string, nhip time.Duration) {
	if l == nil || !l.w.SauChot() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*guard.TranNhip)
	defer cancel()
	_, _ = guard.PhatTheoNhip(ctx, l, 0, guard.DauRa{}, maxTraLoiNep, "", text, nhip)
}

// xongNhom ends a group job's stream after its card committed.
func (l *luongViec) xongNhom(messageID string) {
	if l == nil {
		return
	}
	l.w.Ghi(aistream.Xong, map[string]string{"message_id": messageID})
	l.w.Dong()
}

// xongNep ends a Nếp job's stream after its sealed answer committed: the
// answer rides the invocation key only.
func (l *luongViec) xongNep(text string) {
	if l == nil {
		return
	}
	l.w.Ghi(aistream.Xong, xongNepData(&text))
	l.w.Dong()
}

// xongNepData is xong{text,chips,nguon} for Nếp. Nếp has no chips and no
// sources yet; the fields are there, empty, so the shape is the contract's.
func xongNepData(text *string) map[string]any {
	return map[string]any{"text": text, "chips": []string{}, "nguon": []string{}}
}

// thatBai ends the stream after the job's failure committed.
func (l *luongViec) thatBai(code string) {
	if l == nil {
		return
	}
	if l.phong {
		code = maPhong(code)
	}
	l.w.Ghi(aistream.ThatBai, aistream.ThatBaiData{Code: code})
	l.w.Dong()
}

// huy ends the stream of a cancelled job.
func (l *luongViec) huy() {
	if l == nil {
		return
	}
	l.w.Ghi(aistream.Huy, struct{}{})
	l.w.Dong()
}

// dong writes what still waits and stops the writer, with no ending: the job
// goes on elsewhere (released, retried later) or its ending is someone
// else's to tell.
func (l *luongViec) dong() {
	if l == nil {
		return
	}
	l.w.Dong()
}

// chotNoiDung settles, as the worker stops, whether the job's stream carries
// content (aistream.Writer.ChanNoiDung): true when it does and the job must
// finish; false when none went and none can follow, so the job may go back
// to the queue. A job with no stream has no content.
func (l *luongViec) chotNoiDung() bool {
	return l != nil && l.w.ChanNoiDung()
}

// sink is the engine's Sink for a job: its stream, or nothing.
func (l *luongViec) sink() aiharness.Sink {
	if l == nil {
		return aiharness.BoQua{}
	}
	return l
}

// baoMatLease tells the stream how a job whose lease this worker lost
// ended: cancelled (the caller, or a membership revoked) is huy; failed by
// someone else (the sweep) is that_bai with its code. Anything else -- the
// job queued again, or another worker's -- is not this worker's to tell.
func (h *Handler) baoMatLease(j work) {
	if j.luong == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var status string
	var code *string
	if err := h.pool.QueryRow(ctx, `SELECT status,code FROM chat_ai_invocations WHERE id=$1`, j.id).Scan(&status, &code); err != nil {
		return
	}
	switch {
	case status == "cancelled":
		j.luong.huy()
	case status == "failed" && code != nil:
		j.luong.thatBai(*code)
	}
}
