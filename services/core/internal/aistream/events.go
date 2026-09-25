// Package aistream carries an AI answer from the worker that writes it to the
// people watching it, token by token: a Redis Stream per invocation (the
// requester, Nếp) and per room (onlookers in the legacy lane), a per-process
// hub woken by Redis pub/sub, and an SSE encoder for the HTTP side.
//
// Postgres stays the truth (ADR-0031 §2, amended by the ADR-0038 proposal): the
// published message or the sealed result is what counts, and a lost stream only
// means a client falls back to polling. Stream entries expire with the sharing
// window, are never written to disk (Redis runs without persistence) and are
// never logged.
//
// The event vocabulary is closed (docs/architecture/03-ai-engine-hop-dong.md).
package aistream

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
)

// Kind is one event of the closed stream vocabulary.
type Kind string

const (
	Hello     Kind = "hello"
	TrangThai Kind = "trang_thai"
	Phan      Kind = "phan"
	Delta     Kind = "delta"
	LamLai    Kind = "lam_lai"
	Xong      Kind = "xong"
	ThatBai   Kind = "that_bai"
	Huy       Kind = "huy"
	ThuHoi    Kind = "thu_hoi"
	KetNoiLai Kind = "ket_noi_lai"
)

var kinds = map[Kind]bool{Hello: true, TrangThai: true, Phan: true, Delta: true, LamLai: true,
	Xong: true, ThatBai: true, Huy: true, ThuHoi: true, KetNoiLai: true}

// Valid reports whether k is in the closed vocabulary.
func (k Kind) Valid() bool { return kinds[k] }

// Terminal kinds end an invocation's stream.
func (k Kind) Terminal() bool { return k == Xong || k == ThatBai || k == Huy || k == ThuHoi }

// Event is one stream entry. ID is the Redis entry id, which is also the SSE id
// a client resumes from.
type Event struct {
	ID   string
	Kind Kind
	Data json.RawMessage
}

var (
	entryID = regexp.MustCompile(`^[0-9]{1,20}-[0-9]{1,20}$`)
	// ErrKind refuses an event outside the vocabulary.
	ErrKind = errors.New("aistream: unknown event kind")
)

// ValidID reports whether s is a Redis stream entry id, the only form an SSE
// id or a resume position may take here.
func ValidID(s string) bool { return entryID.MatchString(s) }

// WriteEvent writes one SSE event. The data is compacted JSON: JSON escapes
// every newline inside strings, so after compaction the payload is one line
// and no text inside an answer can start a forged `event:` or `id:` line.
func WriteEvent(w io.Writer, e Event) error {
	if !e.Kind.Valid() {
		return ErrKind
	}
	if e.ID != "" && !ValidID(e.ID) {
		return fmt.Errorf("aistream: invalid event id")
	}
	data := e.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return fmt.Errorf("aistream: event data is not JSON: %w", err)
	}
	var b bytes.Buffer
	if e.ID != "" {
		b.WriteString("id: " + e.ID + "\n")
	}
	b.WriteString("event: " + string(e.Kind) + "\n")
	b.WriteString("data: ")
	b.Write(compact.Bytes())
	b.WriteString("\n\n")
	_, err := w.Write(b.Bytes())
	return err
}

// WritePing writes the heartbeat comment line.
func WritePing(w io.Writer) error {
	_, err := io.WriteString(w, ": ping\n\n")
	return err
}

// WriteRetry tells the client how long to wait before reconnecting. It is an
// SSE body field, never a response header.
func WriteRetry(w io.Writer, ms int) error {
	_, err := fmt.Fprintf(w, "retry: %d\n\n", ms)
	return err
}
