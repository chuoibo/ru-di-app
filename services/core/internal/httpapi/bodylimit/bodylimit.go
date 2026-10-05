// Package bodylimit reads a generic route's request body once, up front,
// with a byte cap and a time budget, before idempotency or authentication
// touch it.
//
// Before this layer every Go-served generic route read the whole body with
// io.ReadAll ahead of authentication (endpoint) and ahead of the endpoint
// (idempotency drain), with no byte cap and no deadline: an anonymous caller
// could hold a connection and goroutine open with a slow body, or park a large
// one in memory (audit 2026-10-05, RS-07). services/api answers the same
// refusals byte for byte (app/api/body_limit.py): this layer sits where that
// middleware sits, inside the guest privacy headers and outside idempotency.
package bodylimit

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"mobile/services/core/internal/httpapi/mw/servererror"
	"mobile/services/core/internal/pyjson"
)

const (
	// DefaultBytes caps a JSON or form body. The largest legitimate generic
	// body is a few kilobytes of JSON; a megabyte leaves room for any of them.
	DefaultBytes int64 = 1 << 20
	// UploadBytes caps the image upload routes: the sanitizer's 10 MiB image
	// plus room for the multipart framing, so an image one byte too large
	// still reaches the route and gets its own answer.
	UploadBytes int64 = 10<<20 + 64<<10
	// ReadTimeout is the whole budget for receiving a body.
	ReadTimeout = 30 * time.Second
	// DiscardBytes is how much of a refused body is read and dropped before
	// the 413, as app/api/body_limit.py's _discard does: answering while the
	// client is still sending makes net/http close the connection under it,
	// and the client sees a reset instead of the answer (parity w0/body-limit
	// under load, 2026-10-05). Past this, or past the deadline, the
	// connection is given up.
	DiscardBytes int64 = 64 << 20

	CodeTooLarge = "request_body_too_large"
	CodeTimeout  = "request_body_timeout"
	detailLarge  = "The request body is larger than this endpoint accepts."
	detailSlow   = "The request body did not arrive in time."
)

// UploadRoutes are the generic routes that take an image upload.
var UploadRoutes = map[string]bool{
	"POST /contexts/{context_id}/photos": true,
	"POST /people/{person_id}/avatar":    true,
	"POST /people/me/photos":             true,
	"POST /receipts/scan":                true,
	"POST /screenshots/scan":             true,
}

// For is the cap of one route.
func For(routeID string) int64 {
	if UploadRoutes[routeID] {
		return UploadBytes
	}
	return DefaultBytes
}

// Wrap reads the body (at most limit bytes, within ReadTimeout), refuses a
// larger or slower one, and hands next a request whose body is in memory.
func Wrap(limit int64, timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		// Not every ResponseWriter can set a deadline (a test recorder cannot);
		// the cap still holds there. On every refusal the deadline stays: after
		// the answer net/http drains what is left of a small body before
		// reusing the connection, and that drain must not wait forever on a
		// client that stopped sending.
		deadlineSet := rc.SetReadDeadline(time.Now().Add(timeout)) == nil
		if r.ContentLength > limit {
			discard(r.Body)
			problem(w, http.StatusRequestEntityTooLarge, CodeTooLarge, detailLarge)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				// net/http closes a connection whose read timed out, so this
				// answer usually never leaves; it is written for a writer that
				// can still deliver it.
				problem(w, http.StatusRequestTimeout, CodeTimeout, detailSlow)
				return
			}
			// A body that breaks off or is malformed fails the request as the
			// endpoint's own read did before this layer existed.
			servererror.Raise(err)
		}
		if int64(len(data)) > limit {
			discard(r.Body)
			problem(w, http.StatusRequestEntityTooLarge, CodeTooLarge, detailLarge)
			return
		}
		if deadlineSet {
			// The whole body is in; the route's own work has its own budget.
			_ = rc.SetReadDeadline(time.Time{})
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(data))
		next.ServeHTTP(w, r)
	})
}

// discard reads what is left of a refused body, at most DiscardBytes, under
// the read deadline already set; any error just ends it.
func discard(body io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, DiscardBytes))
}

// problem answers like the idempotency layer does: json.dumps with its
// defaults, application/json, an explicit length.
func problem(w http.ResponseWriter, status int, code, detail string) {
	document := pyjson.NewOrderedMap()
	document.Set("code", pyjson.String(code))
	document.Set("detail", pyjson.String(detail))
	encoded, err := pyjson.Dumps(document)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Content-Length", strconv.Itoa(len(encoded)))
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}
