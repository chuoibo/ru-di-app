package aistream

import (
	"context"
	"encoding/json"
	"regexp"
	"time"
)

// The room's onlookers (slice 12; design 02 §5.3). Every member of a
// legacy-lane room already holds the change feed's WebSocket, with its
// authentication, its revocation and its reconnects; the room key's events
// ride that socket as `ai` frames instead of a second SSE per member. This
// file turns the room key into those frames. Who may read which room, and
// whether a room is in the legacy lane at all, is the socket's business
// (internal/chatlegacychange): it starts TheoPhong only for a member of a
// legacy-lane room who asked for it.

// KhungPhong is one `ai` frame of the room WebSocket:
//
//	{"type":"ai","inv":"…","tin":"…","so_tin":n,"id":"<redis id>","e":"…","d":{…}}
//
// A change-feed page has no `type`, so a client tells the two apart safely.
// No frame is acknowledged and none moves the feed's cursor.
type KhungPhong struct {
	Type  string          `json:"type"`
	Inv   string          `json:"inv"`
	Tin   string          `json:"tin,omitempty"`
	SoTin int             `json:"so_tin,omitempty"`
	ID    string          `json:"id"`
	E     Kind            `json:"e"`
	D     json.RawMessage `json:"d"`
}

// LoaiKhung is the frame's type.
const LoaiKhung = "ai"

// phongKinds are the events the room sees (contract §4.5, design 02 §5.3):
// hello, thu_hoi and ket_noi_lai belong to one SSE reader, never to the room.
var phongKinds = map[Kind]bool{TrangThai: true, Phan: true, Delta: true, LamLai: true, Xong: true, ThatBai: true, Huy: true}

var (
	maPhongMau = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
	uuidMau    = regexp.MustCompile(`^(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// soPhanToiDa bounds a delta's part index: no answer has more parts than
// the card may hold (companion.GroundReply keeps three), with room to spare.
const soPhanToiDa = 8

// khungPhong is e as the room sees it, or false when the room sees nothing
// of it. The data is rebuilt field by field from what each kind may carry --
// never passed through -- so an entry that grew a field (a Nếp ending's
// text, chips, sources) cannot reach the room by accident:
//
//   - trang_thai{cau}, a status code;
//   - delta{p,text}, the text the output guard's window released;
//   - phan{kind,json}, a grounded part;
//   - xong{message_id}, the posted card's id and nothing else;
//   - that_bai{code}, the code the writer already made generic for a room;
//   - lam_lai{}, huy{}.
func khungPhong(e Event) (KhungPhong, bool) {
	if !phongKinds[e.Kind] || !idPattern.MatchString(e.Inv) || !ValidID(e.ID) {
		return KhungPhong{}, false
	}
	var d any
	switch e.Kind {
	case TrangThai:
		var v TrangThaiData
		if json.Unmarshal(e.Data, &v) != nil || !maPhongMau.MatchString(v.Cau) {
			return KhungPhong{}, false
		}
		d = v
	case Delta:
		var v DeltaData
		if json.Unmarshal(e.Data, &v) != nil || v.P < 0 || v.P >= soPhanToiDa || v.Text == "" {
			return KhungPhong{}, false
		}
		d = v
	case Phan:
		var v struct {
			Kind string          `json:"kind"`
			JSON json.RawMessage `json:"json"`
		}
		if json.Unmarshal(e.Data, &v) != nil || !maPhongMau.MatchString(v.Kind) || !json.Valid(v.JSON) {
			return KhungPhong{}, false
		}
		d = v
	case Xong:
		var v struct {
			MessageID string `json:"message_id"`
		}
		if json.Unmarshal(e.Data, &v) != nil || !uuidMau.MatchString(v.MessageID) {
			return KhungPhong{}, false
		}
		d = v
	case ThatBai:
		var v ThatBaiData
		if json.Unmarshal(e.Data, &v) != nil || !maPhongMau.MatchString(v.Code) {
			return KhungPhong{}, false
		}
		d = v
	default: // lam_lai, huy
		d = struct{}{}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return KhungPhong{}, false
	}
	return KhungPhong{Type: LoaiKhung, Inv: e.Inv, Tin: e.Tin, SoTin: e.SoTin, ID: e.ID, E: e.Kind, D: raw}, true
}

// TheoPhongOptions bound one member's frames.
type TheoPhongOptions struct {
	// Reconcile re-reads the room key even without a wake (1 s), repairing a
	// missed wake and a Redis that was briefly away.
	Reconcile time.Duration
	// Batch is the most entries read per wake (128).
	Batch int64
}

// DefaultTheoPhong is a 1 s reconcile and 128 entries per read.
func DefaultTheoPhong() TheoPhongOptions {
	return TheoPhongOptions{Reconcile: time.Second, Batch: 128}
}

// maxPhatLai bounds what a (re)connecting member is replayed: the room key
// holds about MaxLenRoom entries (trimmed approximately), all younger than
// RoomWindow.
const maxPhatLai = 2 * MaxLenRoom

// TheoPhong sends contextID's room key to gui as frames until ctx ends or gui
// fails, whose error it returns. It holds no database connection and never
// logs an entry.
//
// Joining (and every reconnect, which is a new join) replays the key from
// its start: the invocations that already ended are left out -- their card
// is in the feed, or they posted nothing -- and each one still running
// arrives as its events so far, runs of deltas merged into one per part.
// A client therefore drops what it drew for the room when its socket
// reconnects and redraws from the replay; ids keep their order within one
// connection only (design 02 §5.3, «người vào muộn»). Then it follows live,
// woken by the hub, and deltas merge again only for a reader more than 64
// entries behind.
//
// A Redis that cannot answer is not the member's fault: the socket stays
// open for the feed, and the next tick reads again from where it was.
func (s *Stream) TheoPhong(ctx context.Context, hub *Hub, contextID string, opt TheoPhongOptions, gui func(KhungPhong) error) error {
	key, err := s.Keys.Room(contextID)
	if err != nil {
		return err
	}
	if opt.Reconcile <= 0 {
		opt.Reconcile = time.Second
	}
	if opt.Batch <= 0 {
		opt.Batch = 128
	}
	wake, cancel := hub.Subscribe(key)
	defer cancel()
	send := func(events []Event) error {
		for _, e := range events {
			if k, ok := khungPhong(e); ok {
				if err := gui(k); err != nil {
					return err
				}
			}
		}
		return nil
	}
	after, joined := "", false
	tick := time.NewTicker(opt.Reconcile)
	defer tick.Stop()
	for {
		if !joined {
			events, last, err := s.phatLai(ctx, key, opt.Batch)
			if err == nil {
				joined, after = true, last
				if err = send(events); err != nil {
					return err
				}
			}
		} else {
			for {
				events, last, full, err := s.read(ctx, key, after, "", opt.Batch)
				if err != nil {
					break
				}
				after = last
				if len(events) > gopTren {
					events = gopDelta(events)
				}
				if err = send(events); err != nil {
					return err
				}
				if !full {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-wake:
		case <-tick.C:
		}
	}
}

// phatLai reads the whole room key (up to maxPhatLai entries) and returns
// the events of the invocations that have not ended, deltas merged, and the
// id of the last entry read, for the live reads to start after.
func (s *Stream) phatLai(ctx context.Context, key string, batch int64) ([]Event, string, error) {
	var all []Event
	after := ""
	for len(all) < maxPhatLai {
		events, last, full, err := s.read(ctx, key, after, "", batch)
		if err != nil {
			return nil, after, err
		}
		all = append(all, events...)
		after = last
		if !full {
			break
		}
	}
	ended := map[string]bool{}
	for _, e := range all {
		if e.Kind.Terminal() {
			ended[e.Inv] = true
		}
	}
	live := all[:0]
	for _, e := range all {
		if !ended[e.Inv] {
			live = append(live, e)
		}
	}
	return gopDelta(live), after, nil
}
