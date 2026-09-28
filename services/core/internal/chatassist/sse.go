package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/aistream"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
)

// The two SSE routes (slice 11; design 02 §5.2): the requester of a group
// invocation, and the person who asked Nếp, read the answer while it is
// written. Authorization is one short transaction, committed before the first
// byte, and again every 10 s; no database connection is held while a stream
// waits. The stream is Redis; the job row in Postgres stays the truth, and a
// reader the stream cannot serve is told so (503) and polls.

// Bounds on open streams (design 02 §6).
const (
	sseMoiNguoi   = 5
	sseMoiProcess = 2000
	// sseMoMoiPhut: openings per person per minute, counted in Redis (fail
	// open).
	sseMoMoiPhut = 30
	// sseChoLai is the Retry-After of a refusal the reader may try again.
	sseChoLai = "2"
	// sseXacThucHan bounds each authorization transaction.
	sseXacThucHan = 5 * time.Second
)

// trangThaiXepHang is the one status the transport adds to the engine's
// (design 02 §3.6): the job is queued or running and nothing of it is in the
// stream yet.
const trangThaiXepHang = "dang_xep_hang"

// sucChuaSSE counts open streams per person and per process.
type sucChuaSSE struct {
	mu        sync.Mutex
	theoNguoi map[string]int
	tong      int
	// tran overrides sseMoiProcess (tests).
	tran int
}

func (c *sucChuaSSE) giu(person string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tran := sseMoiProcess
	if c.tran > 0 {
		tran = c.tran
	}
	if c.tong >= tran || c.theoNguoi[person] >= sseMoiNguoi {
		return false
	}
	if c.theoNguoi == nil {
		c.theoNguoi = map[string]int{}
	}
	c.theoNguoi[person]++
	c.tong++
	return true
}

func (c *sucChuaSSE) tra(person string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.theoNguoi[person]--
	if c.theoNguoi[person] <= 0 {
		delete(c.theoNguoi, person)
	}
	c.tong--
}

// dongSSE is what one stream needs to know of its job, read in the
// authorization transaction.
type dongSSE struct {
	j       work
	status  string
	code    *string
	message *string
	text    *string
}

// suKienNhom is GET /contexts/{context}/ai-invocations/{id}/events: the
// requester's stream of their own group invocation. A legacy-lane job is read
// from the room's key, its own entries only; any other lane from the
// invocation's key (khoa).
func (h *Handler) suKienNhom(w http.ResponseWriter, r *http.Request) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		refuse(w, 401, "authentication_required")
		return
	}
	digest := auth.TokenDigest(token)
	room, id := r.PathValue("context"), r.PathValue("id")
	doc := func(ctx context.Context) (dongSSE, string, error) {
		ctx, cancel := context.WithTimeout(ctx, sseXacThucHan)
		defer cancel()
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			return dongSSE{}, "", err
		}
		defer tx.Rollback(ctx)
		g, err := authority(ctx, tx, room, digest)
		if err != nil {
			return dongSSE{}, "", err
		}
		// Asked again at every re-check of an open stream: a pair blocked
		// while its answer is being written stops being streamed.
		if err = phongAi(ctx, tx, g); err != nil {
			return dongSSE{}, "", err
		}
		d := dongSSE{j: work{id: id, conversation: room}}
		err = tx.QueryRow(ctx, `SELECT scope,lane,status,code,message_id::text FROM chat_ai_invocations WHERE id=$1 AND context_id=$2 AND person_id=$3 AND membership_id=$4`, id, room, g.person, g.member).Scan(&d.j.scope, &d.j.lane, &d.status, &d.code, &d.message)
		if errors.Is(err, pgx.ErrNoRows) {
			return dongSSE{}, "", &denied{404, "invocation_not_found"}
		}
		if err != nil {
			return dongSSE{}, "", err
		}
		return d, g.person, tx.Commit(ctx)
	}
	h.phucVuSSE(w, r, doc)
}

// nepEvents is GET /me/nep/ai-invocations/{id}/events: the stream of a
// question the caller asked Nếp. It reads the session, the person and the
// job row, nothing else (aigate holds it to Nếp's tables). Anyone else's id
// reads as absent, as on nepGet.
func (h *Handler) nepEvents(w http.ResponseWriter, r *http.Request) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		refuse(w, 401, "authentication_required")
		return
	}
	digest := auth.TokenDigest(token)
	id := r.PathValue("id")
	doc := func(ctx context.Context) (dongSSE, string, error) {
		ctx, cancel := context.WithTimeout(ctx, sseXacThucHan)
		defer cancel()
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			return dongSSE{}, "", err
		}
		defer tx.Rollback(ctx)
		person, err := phien(ctx, tx, digest)
		if err != nil {
			return dongSSE{}, "", err
		}
		d := dongSSE{j: work{id: id, scope: scopeMe}}
		err = tx.QueryRow(ctx, `SELECT status,code,result->>'text' FROM chat_ai_invocations WHERE id=$1 AND scope='me' AND context_id IS NULL AND person_id=$2`, id, person).Scan(&d.status, &d.code, &d.text)
		if errors.Is(err, pgx.ErrNoRows) {
			return dongSSE{}, "", &denied{404, "invocation_not_found"}
		}
		if err != nil {
			return dongSSE{}, "", err
		}
		return d, person, tx.Commit(ctx)
	}
	h.phucVuSSE(w, r, doc)
}

// phucVuSSE serves one stream: authorize (doc, one short transaction), the
// bounds, then aistream.Follow from the resume position, re-authorizing with
// doc every 10 s.
func (h *Handler) phucVuSSE(w http.ResponseWriter, r *http.Request, doc func(context.Context) (dongSSE, string, error)) {
	d, person, err := doc(r.Context())
	if err != nil {
		failure(w, err)
		return
	}
	if h.stream == nil || h.hub == nil {
		tuChoiLuong(w, 503, "stream_unavailable")
		return
	}
	if ok, _ := h.stream.MoLuot(r.Context(), person, sseMoMoiPhut); !ok {
		w.Header().Set("Retry-After", "60")
		refuse(w, 429, "stream_rate_limited")
		return
	}
	if !h.sucChua.giu(person) {
		tuChoiLuong(w, 503, "stream_capacity")
		return
	}
	defer h.sucChua.tra(person)
	key, _, inv, err := khoa(h.stream.Keys, d.j)
	if err != nil {
		failure(w, invalid("invalid_invocation"))
		return
	}
	// Redis must answer before the 200: a stream that cannot be read is a
	// refusal the client turns into polling, not an empty 200.
	probe, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	_, err = h.stream.Read(probe, key, "", 1)
	cancel()
	if err != nil {
		tuChoiLuong(w, 503, "stream_unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	opt := aistream.DefaultFollow()
	if h.sseXacThucMoi > 0 {
		opt.AuthorizeEvery = h.sseXacThucMoi
	}
	opt.Inv = inv
	opt.Stop = h.dungSSE
	// The row is the truth: its ending, read at open and at every
	// re-authorization, ends a stream that never got its own (review of
	// slice 11, finding 2).
	opt.Authorize = func(ctx context.Context) (bool, []aistream.Event, error) {
		d, _, err := doc(ctx)
		var no *denied
		if errors.As(err, &no) {
			return false, nil, nil
		}
		if err != nil {
			return false, nil, err
		}
		return true, ketThucHang(d), nil
	}
	opt.Ending = ketThucHang(d)
	opt.Empty = func() ([]aistream.Event, bool) { return dauTuHang(d) }
	_ = aistream.Follow(r.Context(), w, h.stream, h.hub, key, aistream.ResumeFrom(r), opt)
}

// ketThucHang is the row's ending as stream events, or nil while the job is
// queued or running.
func ketThucHang(d dongSSE) []aistream.Event {
	if d.status == "queued" || d.status == "running" {
		return nil
	}
	events, _ := dauTuHang(d)
	return events
}

// tuChoiLuong refuses a stream the reader may ask for again shortly.
func tuChoiLuong(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Retry-After", sseChoLai)
	refuse(w, status, code)
}

// dauTuHang is what a reader starting from the beginning hears when the
// stream holds nothing of the job yet, built from the row the authorization
// read: a queued or running job is waiting its turn (trang_thai
// dang_xep_hang, so the «thinking» state never waits on the queue); an ended
// one -- its stream expired, or never written because Redis was down -- ends
// the connection with the row's own ending.
func dauTuHang(d dongSSE) ([]aistream.Event, bool) {
	ev := func(kind aistream.Kind, v any) aistream.Event {
		raw, _ := json.Marshal(v)
		return aistream.Event{Kind: kind, Data: raw}
	}
	switch d.status {
	case "queued", "running":
		return []aistream.Event{ev(aistream.TrangThai, aistream.TrangThaiData{Cau: trangThaiXepHang})}, false
	case "succeeded":
		if d.j.scope == scopeMe {
			return []aistream.Event{ev(aistream.Xong, xongNepData(d.text))}, true
		}
		return []aistream.Event{ev(aistream.Xong, map[string]any{"message_id": d.message})}, true
	case "cancelled":
		return []aistream.Event{ev(aistream.Huy, struct{}{})}, true
	default:
		code := ""
		if d.code != nil {
			code = *d.code
		}
		return []aistream.Event{ev(aistream.ThatBai, aistream.ThatBaiData{Code: code})}, true
	}
}
