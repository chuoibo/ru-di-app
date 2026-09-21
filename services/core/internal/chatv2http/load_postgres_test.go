//go:build postgres

package chatv2http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// A slow reader must not delay other recipients. This exercises actual server
// processes, bearer sessions, the store, and PostgreSQL rather than a fake store.
func TestPostgresSlowConsumerIsolationResumeAndLiveRevocation(t *testing.T) {
	f := liveSetup(t)
	first, second := startChatProcess(t, f), startChatProcess(t, f)
	slow := liveDial(t, first.ready.URL, f, 1, 0)
	fast := liveDial(t, second.ready.URL, f, 2, 0)
	if readFrame(t, slow).Type != "ready" || readFrame(t, fast).Type != "ready" {
		t.Fatal("sockets not ready")
	}
	path := "/v2/chat/" + f.conversation + "/events"
	status, _ := liveRequest(t, first.ready.URL, f.people[0].token, http.MethodPost, path, f.envelope(0))
	if status != 201 {
		t.Fatalf("initial send=%d", status)
	}
	if page := readFrame(t, slow).Page; page == nil || page.NextSequence != 1 {
		t.Fatal("slow client did not receive first page")
	}
	// The slow client intentionally never ACKs its first page.
	if page := readFrame(t, fast).Page; page == nil || page.NextSequence != 1 {
		t.Fatal("fast client did not receive first page")
	}
	if err := wsjson.Write(context.Background(), fast, ack{Type: "ack", Sequence: 1}); err != nil {
		t.Fatal(err)
	}
	status, _ = liveRequest(t, first.ready.URL, f.people[0].token, http.MethodPost, path, f.envelope(0))
	if status != 201 {
		t.Fatalf("second send=%d", status)
	}
	if page := readFrame(t, fast).Page; page == nil || page.NextSequence != 2 {
		t.Fatal("slow reader delayed fast recipient")
	}
	if err := wsjson.Write(context.Background(), fast, ack{Type: "ack", Sequence: 2}); err != nil {
		t.Fatal(err)
	}
	// Revoke a real bearer while a quiet socket remains connected.
	liveExec(t, f.pool, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, f.people[2].id)
	revokeCtx, cancelRevoke := context.WithTimeout(context.Background(), 2*time.Second)
	_, _, err := fast.Read(revokeCtx)
	cancelRevoke()
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("revoked socket survived: %v", err)
	}
	status, _ = liveRequest(t, second.ready.URL, f.people[2].token, http.MethodPost, path, f.envelope(2))
	if status != 401 {
		t.Fatalf("revoked sender status=%d", status)
	}
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 12*time.Second)
	_, _, err = slow.Read(timeoutCtx)
	cancelTimeout()
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("unacknowledged page did not close: %v", err)
	}
	// Resume from the page persisted by the client before the lost ACK.
	resumed := liveDial(t, second.ready.URL, f, 1, 1)
	if readFrame(t, resumed).Type != "ready" {
		t.Fatal("resume not ready")
	}
	page := readFrame(t, resumed).Page
	if page == nil || len(page.Events) != 1 || page.Events[0].Sequence != 2 || page.NextSequence != 2 {
		t.Fatalf("wrong resumed page: %+v", page)
	}
}
