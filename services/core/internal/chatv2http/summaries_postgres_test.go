//go:build postgres

package chatv2http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"mobile/services/core/internal/auth"
)

// The conversation list's view of v2 rooms (GET /v2/chat/summaries): the last
// message's sequence, time and sender from what the lane records in the
// clear, marks and commits not counted as messages, and the unread count past
// the furthest point the person's devices read. Synthetic data only.
func TestPostgresSummariesForTheConversationList(t *testing.T) {
	f := liveSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(f.handler(ctx))
	defer server.Close()
	send := func(i int) {
		t.Helper()
		if code, body := liveRequest(t, server.URL, f.people[i].token, "POST", "/v2/chat/"+f.conversation+"/events", f.envelope(i)); code != 201 && code != 200 {
			t.Fatalf("send: %d %s", code, body)
		}
	}
	type room struct {
		ConversationID string  `json:"conversation_id"`
		LastSequence   int64   `json:"last_sequence"`
		LastActorID    *string `json:"last_actor_id"`
		Unread         int     `json:"unread"`
		ReadSequence   int64   `json:"read_sequence"`
	}
	read := func(i int) []room {
		t.Helper()
		code, body := liveRequest(t, server.URL, f.people[i].token, "GET", "/v2/chat/summaries", nil)
		if code != 200 {
			t.Fatalf("summaries: %d %s", code, body)
		}
		var out struct {
			Rooms []room `json:"rooms"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		return out.Rooms
	}
	send(0)
	send(0)
	send(1)
	an := read(0)
	if len(an) != 1 || an[0].ConversationID != f.conversation || an[0].LastSequence != 3 || an[0].LastActorID == nil || *an[0].LastActorID != f.people[1].id || an[0].Unread != 1 {
		t.Fatalf("An: %+v", an)
	}
	if chi := read(2); len(chi) != 1 || chi[0].Unread != 3 {
		t.Fatalf("Chi: %+v", chi)
	}
	// A read mark is an event of the lane, not a message: unread goes to 0
	// and the last message stays where it was.
	if code, body := liveRequest(t, server.URL, f.people[0].token, "PUT", "/v2/chat/"+f.conversation+"/marks", map[string]any{"device_id": f.people[0].device, "kind": "read", "sequence": 3}); code != 200 {
		t.Fatalf("mark: %d %s", code, body)
	}
	if an = read(0); an[0].Unread != 0 || an[0].ReadSequence != 3 || an[0].LastSequence != 3 {
		t.Fatalf("An after reading: %+v", an)
	}
	// A device taken out reads nothing more: no live device, nothing unread.
	liveExec(t, f.pool, `UPDATE chat_v2_devices SET revoked_at=now() WHERE id=$1`, f.people[1].device)
	if binh := read(1); len(binh) != 1 || binh[0].Unread != 0 {
		t.Fatalf("Bình with no live device: %+v", binh)
	}
	// Someone who left the group no longer sees the room; a stranger never did.
	liveExec(t, f.pool, `UPDATE memberships SET state='left', left_at=now() WHERE id=$1`, f.people[2].membership)
	if chi := read(2); len(chi) != 0 {
		t.Fatalf("Chi after leaving: %+v", chi)
	}
	stranger := "synthetic-" + newID()
	strangerID := newID()
	liveExec(t, f.pool, `INSERT INTO people(id,display_name)VALUES($1,'Synthetic stranger')`, strangerID)
	liveExec(t, f.pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at)VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, newID(), strangerID, auth.TokenDigest(stranger))
	code, body := liveRequest(t, server.URL, stranger, "GET", "/v2/chat/summaries", nil)
	if code != 200 || string(body) != `{"rooms":[]}` {
		t.Fatalf("stranger: %d %s", code, body)
	}
}
