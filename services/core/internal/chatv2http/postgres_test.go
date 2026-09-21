//go:build postgres

package chatv2http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/testdb"
)

type livePerson struct {
	id, device, membership, token string
	key                           ed25519.PrivateKey
}
type liveWorld struct {
	pool         *pgxpool.Pool
	conversation string
	people       []livePerson
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	s := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[:8], s[8:12], s[12:16], s[16:20], s[20:])
}

func liveSetup(t *testing.T) liveWorld {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "chat_http_" + strings.ReplaceAll(newID(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// These are real migrated column/check definitions. Full legacy migration
	// is exercised by the bootstrap runner; this fixture isolates every row.
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests", "account_sessions"} {
		if _, err := pool.Exec(ctx, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
	}
	if err := chatv2.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f := liveWorld{pool: pool, conversation: newID()}
	for i := 0; i < 3; i++ {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		p := livePerson{id: newID(), device: newID(), membership: newID(), token: "synthetic-" + newID(), key: key}
		f.people = append(f.people, p)
		liveExec(t, pool, `INSERT INTO people(id,display_name)VALUES($1,'Synthetic chat participant')`, p.id)
		liveExec(t, pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at)VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, newID(), p.id, auth.TokenDigest(p.token))
		liveExec(t, pool, `INSERT INTO chat_v2_devices(id,person_id,signing_key)VALUES($1,$2,$3)`, p.device, p.id, []byte(pub))
	}
	liveExec(t, pool, `INSERT INTO contexts(id,display_name,created_by_id)VALUES($1,'Synthetic HTTP lab',$2)`, f.conversation, f.people[0].id)
	for _, p := range f.people {
		liveExec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin)VALUES($1,$2,$3,'active','member','named')`, p.membership, f.conversation, p.id)
	}
	// Explicit synthetic provisioning is not a production enrollment flow.
	liveExec(t, pool, `INSERT INTO chat_v2_conversations(context_id,epoch,ready)VALUES($1,1,true)`, f.conversation)
	for _, p := range f.people {
		liveExec(t, pool, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence)VALUES($1,$2,$3,1)`, f.conversation, p.device, p.membership)
	}
	return f
}
func liveExec(t *testing.T, p *pgxpool.Pool, q string, a ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), q, a...); err != nil {
		t.Fatal(err)
	}
}
func (f liveWorld) handler(ctx context.Context) *Handler {
	return New(Options{Store: chatv2.NewStore(f.pool), Authenticate: Sessions(f.pool), Experimental: true, Context: ctx, ReconcileInterval: 20 * time.Millisecond})
}
func (f liveWorld) envelope(i int) chatv2.Envelope {
	p := f.people[i]
	e := chatv2.Envelope{ConversationID: f.conversation, DeviceID: p.device, LogicalSendID: newID(), Protocol: chatv2.Protocol, Epoch: 1, Ciphertext: []byte("synthetic opaque wire fixture; no MLS claim")}
	b, err := chatv2.SigningBytes(e)
	if err != nil {
		panic(err)
	}
	e.Signature = ed25519.Sign(p.key, b)
	return e
}
func liveRequest(t *testing.T, server, token, method, path string, value any) (int, []byte) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	r, err := http.NewRequest(method, server+path, strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, raw
}
func liveDial(t *testing.T, url string, f liveWorld, index int, after int64) *websocket.Conn {
	t.Helper()
	p := f.people[index]
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http")+fmt.Sprintf("/v2/chat/%s/stream?device_id=%s&after=%d&limit=2", f.conversation, p.device, after), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + p.token}}, Subprotocols: []string{chatv2.Protocol}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}
func TestPostgresTwoReplicasThreePeopleAndRestart(t *testing.T) {
	f := liveSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := f.handler(ctx), f.handler(ctx)
	go b.Listen(ctx, f.pool)
	s1, s2 := httptest.NewServer(a), httptest.NewServer(b)
	defer s1.Close()
	defer s2.Close()
	c := liveDial(t, s2.URL, f, 1, 0)
	if readFrame(t, c).Type != "ready" {
		t.Fatal("no ready")
	}
	path := "/v2/chat/" + f.conversation + "/events"
	first := f.envelope(0)
	for i := 0; i < 5; i++ {
		e := first
		if i != 0 {
			e = f.envelope(i % 3)
		}
		status, _ := liveRequest(t, s1.URL, f.people[i%3].token, "POST", path, e)
		if status != 201 {
			t.Fatalf("send status %d", status)
		}
	}
	last := int64(0)
	for last < 5 {
		v := readFrame(t, c)
		if v.Type != "events" || v.Page == nil {
			t.Fatal(v)
		}
		for _, e := range v.Page.Events {
			if e.Sequence != last+1 {
				t.Fatalf("sequence %d after %d", e.Sequence, last)
			}
			last = e.Sequence
		}
		if err := wsjson.Write(context.Background(), c, ack{Type: "ack", Sequence: last}); err != nil {
			t.Fatal(err)
		}
	}
	c.CloseNow()
	s1.Close()
	// A fresh handler carries no in-memory receipt, cursor, or event data.
	restarted := httptest.NewServer(f.handler(ctx))
	defer restarted.Close()
	status, body := liveRequest(t, restarted.URL, f.people[0].token, "POST", path, first)
	var replay chatv2.SendResult
	_ = json.Unmarshal(body, &replay)
	if status != 200 || !replay.Replayed || replay.Event.Sequence != 1 {
		t.Fatalf("restart replay status=%d result=%+v", status, replay)
	}
	resume := liveDial(t, restarted.URL, f, 1, 3)
	readFrame(t, resume)
	p := readFrame(t, resume)
	if p.Page == nil || len(p.Page.Events) != 2 || p.Page.Events[0].Sequence != 4 || p.Page.NextSequence != 5 {
		t.Fatal(p)
	}
	var eventCount, outboxCount int
	if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM chat_v2_events),(SELECT count(*) FROM chat_v2_outbox)`).Scan(&eventCount, &outboxCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 5 || outboxCount != 5 {
		t.Fatalf("events=%d outbox=%d", eventCount, outboxCount)
	}
}
func TestPostgresSessionRevocationClosesQuietSocket(t *testing.T) {
	f := liveSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := httptest.NewServer(f.handler(ctx))
	defer s.Close()
	c := liveDial(t, s.URL, f, 0, 0)
	readFrame(t, c)
	liveExec(t, f.pool, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, f.people[0].id)
	wait, cancelRead := context.WithTimeout(ctx, 3*time.Second)
	defer cancelRead()
	_, _, err := c.Read(wait)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("revoked socket %v", err)
	}
	status, _ := liveRequest(t, s.URL, f.people[0].token, "POST", "/v2/chat/"+f.conversation+"/events", f.envelope(0))
	if status != 401 {
		t.Fatalf("revoked post=%d", status)
	}
}
