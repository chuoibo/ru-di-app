//go:build postgres && drill

package chatv2http

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
)

// The ADR-0057 protocol end to end: real MLS devices (the Rust crate through
// its C ABI, packages/chat-crypto-ffi/examples/drill.rs) against the real Go
// lane on PostgreSQL. Enrollment, key packages, bootstrap, claim, commit,
// Welcome, messages both ways, a third member added and a departed one
// removed -- every roster step verified by the devices against the roster the
// server attests. Run by scripts/chat_drill.sh. Synthetic data only.

type drill struct {
	t   *testing.T
	in  *json.Encoder
	out *bufio.Scanner
}

func startDrill(t *testing.T) *drill {
	bin := os.Getenv("CHAT_DRILL_BIN")
	if bin == "" {
		t.Fatal("CHAT_DRILL_BIN is not set; run scripts/chat_drill.sh")
	}
	cmd := exec.Command(bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Wait() })
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	return &drill{t: t, in: json.NewEncoder(stdin), out: scanner}
}

func (d *drill) do(request map[string]any) map[string]any {
	d.t.Helper()
	if err := d.in.Encode(request); err != nil {
		d.t.Fatal(err)
	}
	if !d.out.Scan() {
		d.t.Fatalf("drill closed: %v", d.out.Err())
	}
	var answer map[string]any
	if err := json.Unmarshal(d.out.Bytes(), &answer); err != nil {
		d.t.Fatal(err)
	}
	if e, bad := answer["error"]; bad {
		d.t.Fatalf("drill %v: %v", request["fn"], e)
	}
	return answer
}

func (d *drill) call(client, method string, args map[string]any) map[string]any {
	d.t.Helper()
	return d.do(map[string]any{"client": client, "fn": "call", "method": method, "args": args})
}

type person struct {
	name, id, device, token string
}

func TestDrillTheProtocolEndToEnd(t *testing.T) {
	f := liveSetup(t)
	ctx := context.Background()
	d := startDrill(t)
	server := httptest.NewServer(f.handler(ctx))
	defer server.Close()
	room := newID()
	liveExec(t, f.pool, `INSERT INTO contexts(id,display_name,created_by_id)VALUES($1,'Phòng diễn tập',$2)`, room, f.people[0].id)
	people := []person{}
	for i, name := range []string{"an", "binh", "chi"} {
		p := person{name: name, id: f.people[i].id, device: newID(), token: f.people[i].token}
		people = append(people, p)
		d.do(map[string]any{"client": name, "fn": "new", "actor": p.id, "device": p.device})
		enrollment := d.call(name, "enrollment", map[string]any{})
		card := enrollment["card"].(map[string]any)
		toBytes := func(v any) []byte {
			raw := v.([]any)
			b := make([]byte, len(raw))
			for i, x := range raw {
				b[i] = byte(x.(float64))
			}
			return b
		}
		status, body := liveRequest(t, server.URL, p.token, "POST", "/v2/chat/devices", map[string]any{
			"device_id": p.device, "mls_signature_key": toBytes(card["mls_signature_key"]),
			"transport_signature_key": toBytes(card["transport_signature_key"]), "proof": enrollment["proof"], "label": "Máy diễn tập"})
		if status != 201 {
			t.Fatalf("enroll %s: %d %s", name, status, body)
		}
		kp := d.call(name, "key_package", map[string]any{})["key_package"]
		if status, body := liveRequest(t, server.URL, p.token, "POST", "/v2/chat/devices/"+p.device+"/key-packages", map[string]any{"key_packages": []any{kp}}); status != 200 {
			t.Fatalf("publish %s: %d %s", name, status, body)
		}
	}
	an, binh, chi := people[0], people[1], people[2]
	// Only An and Bình are in the room for now.
	liveExec(t, f.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin)VALUES($1,$2,$3,'active','admin','named'),($4,$2,$5,'active','member','named')`,
		newID(), room, an.id, newID(), binh.id)

	get := func(p person, path string, into any) {
		t.Helper()
		status, body := liveRequest(t, server.URL, p.token, "GET", path, nil)
		if status != 200 {
			t.Fatalf("GET %s: %d %s", path, status, body)
		}
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatal(err)
		}
	}
	// commit stages a change in Rust, posts it, and acknowledges it in Rust.
	commit := func(p person, method string, args map[string]any, added []string, removed []string) chatv2.CommitResult {
		t.Helper()
		args["conversation_id"] = room
		args["logical_send_id"] = newID()
		bundle := d.call(p.name, method, args)
		req := map[string]any{"envelope": bundle["envelope"], "added": added, "removed": removed}
		if w, ok := bundle["welcome"].(string); ok {
			req["welcome"] = w
		}
		status, body := liveRequest(t, server.URL, p.token, "POST", "/v2/chat/"+room+"/commits", req)
		if status != 201 {
			t.Fatalf("commit by %s: %d %s", p.name, status, body)
		}
		var result chatv2.CommitResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		d.call(p.name, "acknowledge_commit", map[string]any{"envelope": bundle["envelope"]})
		return result
	}
	claim := func(p person, targets ...string) []any {
		t.Helper()
		status, body := liveRequest(t, server.URL, p.token, "POST", "/v2/chat/"+room+"/key-packages/claim", map[string]any{"device_id": p.device, "targets": targets})
		if status != 200 {
			t.Fatalf("claim by %s: %d %s", p.name, status, body)
		}
		var out struct {
			Claims []chatv2.Claim `json:"claims"`
		}
		_ = json.Unmarshal(body, &out)
		members := []any{}
		for _, c := range out.Claims {
			members = append(members, map[string]any{"card": c.Card, "key_package": c.KeyPackage})
		}
		return members
	}
	join := func(p person) {
		t.Helper()
		var mail struct {
			Welcomes []chatv2.WelcomeView `json:"welcomes"`
		}
		get(p, "/v2/chat/devices/"+p.device+"/welcomes", &mail)
		if len(mail.Welcomes) != 1 {
			t.Fatalf("%s welcomes: %d", p.name, len(mail.Welcomes))
		}
		w := mail.Welcomes[0]
		d.call(p.name, "join_group", map[string]any{"conversation_id": room, "welcome": w.Welcome, "roster": w.Roster})
		r, _ := http.NewRequest("DELETE", server.URL+"/v2/chat/devices/"+p.device+"/welcomes/"+w.ID, nil)
		r.Header.Set("Authorization", "Bearer "+p.token)
		resp, err := http.DefaultClient.Do(r)
		if err != nil || resp.StatusCode != 204 {
			t.Fatalf("ack welcome: %v %v", resp, err)
		}
		resp.Body.Close()
	}
	var cursors = map[string]int64{}
	// read pulls a device's new events and feeds them to its Rust client:
	// envelopes as messages, commits with the roster the server attests.
	read := func(p person) []map[string]any {
		t.Helper()
		var page chatv2.Page
		get(p, "/v2/chat/"+room+"/events?device_id="+p.device+"&after="+strconv.FormatInt(cursors[p.name], 10)+"&limit=100", &page)
		got := []map[string]any{}
		for _, e := range page.Events {
			cursors[p.name] = e.Sequence
			switch {
			case e.Envelope != nil && e.Envelope.DeviceID != p.device:
				got = append(got, d.do(map[string]any{"client": p.name, "fn": "receive", "envelope": e.Envelope, "roster": nil}))
			case e.Commit != nil && e.Commit.Envelope.DeviceID != p.device:
				got = append(got, d.do(map[string]any{"client": p.name, "fn": "receive", "envelope": e.Commit.Envelope, "roster": e.Commit.Roster}))
			}
		}
		return got
	}
	send := func(p person, body string) {
		t.Helper()
		var epoch struct {
			Epoch int64 `json:"epoch"`
		}
		raw, _ := json.Marshal(d.call(p.name, "epoch", map[string]any{"conversation_id": room}))
		_ = json.Unmarshal(raw, &epoch)
		envelope := d.do(map[string]any{"client": p.name, "fn": "encrypt", "conversation_id": room, "logical_send_id": newID(),
			"operation": map[string]any{"type": "text", "body": body}})
		status, out := liveRequest(t, server.URL, p.token, "POST", "/v2/chat/"+room+"/events", envelope)
		if status != 201 {
			t.Fatalf("send by %s: %d %s", p.name, status, out)
		}
		d.call(p.name, "acknowledge_sent", map[string]any{"envelope": envelope})
	}

	// An opens the lane and adds Bình.
	if status, body := liveRequest(t, server.URL, an.token, "POST", "/v2/chat/"+room+"/bootstrap", map[string]any{"device_id": an.device}); status != 200 {
		t.Fatalf("bootstrap: %d %s", status, body)
	}
	d.do(map[string]any{"client": "an", "fn": "create_group", "conversation_id": room})
	if r := commit(an, "stage_add", map[string]any{"members": claim(an, binh.device)}, []string{binh.device}, nil); !r.Ready {
		t.Fatalf("after adding Bình the room is not ready: %+v", r)
	}
	join(binh)
	cursors["binh"] = 1

	send(an, "Chào Bình, tối nay đi ăn không?")
	if got := read(binh); len(got) != 1 || got[0]["operation"].(map[string]any)["body"] != "Chào Bình, tối nay đi ăn không?" {
		t.Fatalf("Bình read: %v", got)
	}
	send(binh, "Đi chứ!")
	if got := read(an); len(got) != 1 || got[0]["operation"].(map[string]any)["body"] != "Đi chứ!" {
		t.Fatalf("An read: %v", got)
	}

	// Chi joins the group: the room stops being ready until a member adds her
	// device; Bình follows that commit against the server's roster.
	liveExec(t, f.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin)VALUES($1,$2,$3,'active','member','named')`, newID(), room, chi.id)
	var roster chatv2.RosterView
	get(an, "/v2/chat/"+room+"/roster?device_id="+an.device, &roster)
	if roster.Ready || len(roster.Expected) != 3 {
		t.Fatalf("roster after Chi joined: %+v", roster)
	}
	commit(an, "stage_add", map[string]any{"members": claim(an, chi.device)}, []string{chi.device}, nil)
	if got := read(binh); len(got) != 1 || got[0]["kind"] != "commit" {
		t.Fatalf("Bình followed the add: %v", got)
	}
	join(chi)
	cursors["chi"] = cursors["binh"]
	send(chi, "Cho mình đi với")
	for _, p := range []person{an, binh} {
		if got := read(p); len(got) != 1 || got[0]["operation"].(map[string]any)["body"] != "Cho mình đi với" {
			t.Fatalf("%s read Chi: %v", p.name, got)
		}
	}

	// Bình leaves: An removes his device; Chi follows; Bình's next read is refused.
	liveExec(t, f.pool, `UPDATE memberships SET state='left', left_at=now() WHERE context_id=$1 AND person_id=$2`, room, binh.id)
	if r := commit(an, "stage_remove", map[string]any{"device_id": binh.device}, nil, []string{binh.device}); !r.Ready {
		t.Fatalf("after removing Bình: %+v", r)
	}
	if got := read(chi); len(got) != 1 || got[0]["kind"] != "commit" {
		t.Fatalf("Chi followed the removal: %v", got)
	}
	if status, _ := liveRequest(t, server.URL, binh.token, "GET", "/v2/chat/"+room+"/events?device_id="+binh.device+"&after=0&limit=10", nil); status != 403 {
		t.Fatalf("Bình still reads after leaving: %d", status)
	}
	send(an, "Hai đứa mình đi nhé")
	if got := read(chi); len(got) != 1 || got[0]["operation"].(map[string]any)["body"] != "Hai đứa mình đi nhé" {
		t.Fatalf("Chi read after the removal: %v", got)
	}
	_ = auth.TokenDigest
	_ = time.Second
}
