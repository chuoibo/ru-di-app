package chatlegacychange

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"mobile/services/core/internal/aistream"
)

// The room's `ai` frames (slice 12; design 02 §5.3). A member who is already
// on this WebSocket for the change feed watches the Rủ Đi AI answer being
// written through the same socket, instead of opening a second stream:
//
//   - opt-in: the authenticate frame says {"ai":true}; a connection
//     authenticated by header, or a client that does not ask, gets none;
//   - authorization is the feed's own: the pump starts after the first page,
//     which ran authorize (session, person, active membership), and every
//     later page runs it again each reconcile tick (1 s) -- a member who is
//     removed loses the connection and its pump with it;
//   - only a legacy-lane room: the lane is read in the page's own snapshot,
//     and a room found in v2 (end to end encrypted) never starts a pump, or
//     stops the one it had. The room key is never written for a v2 job
//     anyway (chatassist's khoa), so this is the second wall, not the first;
//   - what flows is aistream.TheoPhong: the closed room vocabulary, each
//     event rebuilt field by field, and the text only as the output guard's
//     window released it after the card was posted;
//   - no acknowledgement, no cursor: a frame never moves `after`, and the
//     feed's ack reader is untouched;
//   - bounded: the feed's own slots (1000 connections a process, 5 a person),
//     at most MaxAiMoiPhong pumps a room a process (beyond it the member
//     still gets the feed and the card, only not the words as they come),
//     each write within 10 s or the connection closes, and a replay of at
//     most the room key's window (15 minutes, about 2048 entries);
//   - nothing of a frame is ever logged.

// MaxAiMoiPhong is how many members of one room one process pumps `ai`
// frames to at once.
const MaxAiMoiPhong = 256

type phongAi struct {
	stream *aistream.Stream
	hub    *aistream.Hub
	opt    aistream.TheoPhongOptions
	toiDa  int
	// ghiHan bounds one frame's write.
	ghiHan time.Duration

	mu       sync.Mutex
	moiPhong map[string]int
}

// WithAi turns the room's `ai` frames on: stream is the answers' Redis
// streams, hub this process's wake hub (the one the SSE routes use). Nil
// stream leaves them off.
func (h *Handler) WithAi(stream *aistream.Stream, hub *aistream.Hub) *Handler {
	if stream == nil || hub == nil {
		h.ai = nil
		return h
	}
	h.ai = &phongAi{stream: stream, hub: hub, opt: aistream.DefaultTheoPhong(), toiDa: MaxAiMoiPhong, ghiHan: 10 * time.Second, moiPhong: map[string]int{}}
	return h
}

// bomAi is one connection's pump.
type bomAi struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// dung stops the pump and waits for it. Nil does nothing.
func (b *bomAi) dung() {
	if b == nil {
		return
	}
	b.cancel()
	<-b.done
}

// mo starts room's pump on conn, or returns nil when the room already has
// its share of pumps in this process. A frame that cannot be written in
// time closes the connection (dong): the client reconnects and is replayed.
func (p *phongAi) mo(ctx context.Context, conn *websocket.Conn, room string, dong func()) *bomAi {
	p.mu.Lock()
	if p.moiPhong[room] >= p.toiDa {
		p.mu.Unlock()
		return nil
	}
	p.moiPhong[room]++
	p.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	b := &bomAi{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(b.done)
		defer func() {
			p.mu.Lock()
			if p.moiPhong[room]--; p.moiPhong[room] <= 0 {
				delete(p.moiPhong, room)
			}
			p.mu.Unlock()
		}()
		err := p.stream.TheoPhong(ctx, p.hub, room, p.opt, func(k aistream.KhungPhong) error {
			wctx, stop := context.WithTimeout(ctx, p.ghiHan)
			defer stop()
			return wsjson.Write(wctx, conn, k)
		})
		if err != nil && ctx.Err() == nil {
			dong()
		}
	}()
	return b
}
