// Package chatv2 persists opaque signed envelopes. It does not implement MLS,
// enroll devices, or establish that ciphertext was encrypted correctly.
package chatv2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"time"
)

const Protocol = "rudi-chat-v2-mls"
const MaxCiphertext = 256 * 1024
const MaxPageSize = 100

// MaxPageBytes bounds the aggregate encoded event payload, including a
// conservative allowance for event and page metadata.
const MaxPageBytes = 512 * 1024

var (
	ErrInvalid   = errors.New("chat_v2_invalid")
	ErrForbidden = errors.New("chat_v2_forbidden")
	ErrConflict  = errors.New("chat_v2_send_conflict")
	ErrNotReady  = errors.New("chat_v2_not_ready")
	ErrEpoch     = errors.New("chat_v2_stale_epoch")
)

type Envelope struct {
	ConversationID string `json:"conversation_id"`
	DeviceID       string `json:"device_id"`
	LogicalSendID  string `json:"logical_send_id"`
	Protocol       string `json:"protocol"`
	Epoch          int64  `json:"epoch"`
	Ciphertext     []byte `json:"ciphertext"`
	Signature      []byte `json:"signature"`
}

type Event struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Envelope  *Envelope `json:"envelope,omitempty"`
	Mark      *Mark     `json:"mark,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SendResult struct {
	Event    Event `json:"event"`
	Replayed bool  `json:"replayed"`
}

type Page struct {
	Events       []Event `json:"events"`
	NextSequence int64   `json:"next_sequence"`
	HasMore      bool    `json:"has_more"`
}

type Mark struct {
	DeviceID  string `json:"device_id"`
	Delivered int64  `json:"delivered"`
	Read      int64  `json:"read"`
}

// ValidID accepts only the canonical lower-case UUID wire spelling.
func ValidID(v string) bool {
	if len(v) != 36 || v != strings.ToLower(v) {
		return false
	}
	for i, c := range v {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	// repo-guard: allow=long-number reason=synthetic-zero-uuid
	return v != "00000000-0000-0000-0000-000000000000"
}

// SigningBytes is the versioned, length-prefixed signature preimage. The
// ciphertext itself is authenticated; no JSON re-serialization is involved.
func SigningBytes(e Envelope) ([]byte, error) {
	if !ValidID(e.ConversationID) || !ValidID(e.DeviceID) || !ValidID(e.LogicalSendID) || e.Protocol != Protocol || e.Epoch < 1 || len(e.Ciphertext) < 1 || len(e.Ciphertext) > MaxCiphertext {
		return nil, ErrInvalid
	}
	var b bytes.Buffer
	b.WriteString("RUDI-CHAT-ENVELOPE\x00v2\x00")
	for _, s := range []string{e.ConversationID, e.DeviceID, e.LogicalSendID, e.Protocol} {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(s)))
		b.WriteString(s)
	}
	_ = binary.Write(&b, binary.BigEndian, e.Epoch)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(e.Ciphertext)))
	b.Write(e.Ciphertext)
	return b.Bytes(), nil
}
