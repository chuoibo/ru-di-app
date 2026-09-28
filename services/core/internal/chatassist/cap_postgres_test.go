//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/auth"
)

// Rủ Đi AI in a chat of two (design docs/claude/2026-09-28/ai-chat-hai-nguoi.md,
// slice S1) against a real database: a pair takes `hoi` and nothing else; a
// pair whose other person blocked, was blocked or deleted their account is
// refused at the route and again by the worker, before it reads and before
// it publishes; a group is unchanged.

// kichCap is one direct answer: the router, the answer, the verifier.
func kichCap() []llm.Buoc {
	return []llm.Buoc{ruNhom(nil), {Text: "Chào hai bạn, cần gì cứ gọi mình nhé."}, kiemNhom("khong_thong_tin")}
}

// setupCap is setupNhomGo with the room turned into a pair of its two
// members, the fixture chatlegacychange's pair tests use.
func setupCap(t *testing.T, kich ...llm.Buoc) nhomGo {
	t.Helper()
	n := setupNhomGo(t, tools.NguonDuLieu{}, kich...)
	n.f.exec(t, `UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, n.f.context, n.f.person+":"+n.f.peer)
	return n
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

// chan writes a block between the two, by whom it names.
func (f fixture) chan_(t *testing.T, by, other string) {
	t.Helper()
	f.exec(t, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES($1,$2,$3,'blocked',$2,now())`, newID(), by, other)
}

// goiCap posts a `hoi` with a trigger and a shared turn; it does not run the
// worker.
func (n nhomGo) goiCap(t *testing.T, lenh string) (*Invocation, int, string) {
	t.Helper()
	trigger := n.f.tinTag(t, n.f.context, n.f.person)
	a := n.f.tinTrongPhong(t, n.f.context, "Tối nay đi đâu")
	w := n.f.request("POST", n.f.route(), n.f.token, map[string]any{"logical_id": newID(), "command": lenh, "prompt": "@Rủ Đi tối nay đi đâu", "trigger_message_id": trigger, "boi_canh": goiThu(luotThu(a, "Tối nay đi đâu"))})
	if w.Code != 202 {
		var e struct{ Code string }
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		return nil, w.Code, e.Code
	}
	var v Invocation
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return &v, w.Code, ""
}

func (n nhomGo) chayViec(t *testing.T) {
	t.Helper()
	if ok, err := n.f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
}

// trangThai is the job's status, code, and how many AI cards the room holds.
func (n nhomGo) trangThai(t *testing.T, id string) (string, string, int) {
	t.Helper()
	var status string
	var code *string
	var cards int
	err := n.f.pool.QueryRow(context.Background(), `SELECT status,code,(SELECT count(*) FROM messages WHERE context_id=$2 AND kind='ai_card') FROM chat_ai_invocations WHERE id=$1`, id, n.f.context).Scan(&status, &code, &cards)
	if err != nil {
		t.Fatal(err)
	}
	c := ""
	if code != nil {
		c = *code
	}
	return status, c, cards
}

func maTraVe(body string) string {
	var e struct{ Code string }
	_ = json.Unmarshal([]byte(body), &e)
	return e.Code
}

// A pair's `hoi` is taken and answered in the thread: 202, the worker runs
// the Go engine, the card is a reply to the tag message, the brain is never
// asked for an answer. chat-capabilities says so: hoi and mention on, plan
// and chia_bill off with group_plan_only.
func TestCapHoiChayToiTraLoi(t *testing.T) {
	n := setupCap(t, kichCap()...)
	w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
	requireCode(t, w, 200)
	var caps struct {
		AI struct {
			Plan     map[string]any `json:"plan"`
			ChiaBill map[string]any `json:"chia_bill"`
			Hoi      map[string]any `json:"hoi"`
			Mention  bool           `json:"mention"`
		} `json:"ai"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.AI.Hoi["available"] != true || caps.AI.Hoi["reason"] != nil || !caps.AI.Mention ||
		caps.AI.Plan["available"] != false || caps.AI.Plan["reason"] != "group_plan_only" ||
		caps.AI.ChiaBill["available"] != false || caps.AI.ChiaBill["reason"] != "group_plan_only" {
		t.Fatalf("khả năng của cặp: %s", w.Body.String())
	}
	job, code, ma := n.goiCap(t, "hoi")
	if job == nil {
		t.Fatalf("cặp hoi: %d %s", code, ma)
	}
	n.chayViec(t)
	k := n.ket(t, job.ID)
	if k.status != "succeeded" || k.reply == nil || *k.reply != *job.TriggerMessageID {
		t.Fatalf("%s %v %v", k.status, k.code, k.reply)
	}
	var the theTraLoi
	if err := json.Unmarshal(k.card, &the); err != nil {
		t.Fatal(err)
	}
	if the.Kind != "tra_loi" || the.Payload.Lenh != "hoi" || !strings.Contains(string(the.Payload.Phan[0]), "Chào hai bạn") {
		t.Fatalf("thẻ: %s", k.card)
	}
	if n.brain.calls != 0 {
		t.Fatalf("brain được hỏi %d lần trên engine Go: %v", n.brain.calls, n.brain.bodies)
	}
	// The other person reads their own capabilities the same way.
	w = n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.peerToken, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"hoi":{"available":true,"reason":null}`) {
		t.Fatalf("người kia: %d %s", w.Code, w.Body.String())
	}
}

// plan and chia_bill stay the group's: 409 group_plan_only before anything
// is written or the provider probed.
func TestCapPlanChiaBillBiTuChoi(t *testing.T) {
	n := setupCap(t)
	for _, lenh := range []string{"plan", "chia_bill"} {
		if job, code, ma := n.goiCap(t, lenh); job != nil || code != 409 || ma != "group_plan_only" {
			t.Fatalf("%s trong cặp: %d %s", lenh, code, ma)
		}
	}
	if got := n.f.capabilityCalls.Load(); n.f.demLoiGoi(t) != 0 || got != 0 {
		t.Fatalf("ghi %d lời gọi, dò nhà cung cấp %d lần", n.f.demLoiGoi(t), got)
	}
}

// Someone who is not in the pair gets 403, the membership refusal.
func TestCapNguoiNgoai(t *testing.T) {
	n := setupCap(t)
	stranger, token := newID(), "synthetic-"+newID()
	n.f.exec(t, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic stranger')`, stranger)
	n.f.exec(t, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()+interval '1 hour')`, newID(), stranger, auth.TokenDigest(token))
	trigger := n.f.tinTag(t, n.f.context, n.f.person)
	w := n.f.request("POST", n.f.route(), token, map[string]any{"logical_id": newID(), "command": "hoi", "prompt": "x", "trigger_message_id": trigger})
	if w.Code != 403 || maTraVe(w.Body.String()) != "membership_required" {
		t.Fatalf("người ngoài: %d %s", w.Code, w.Body.String())
	}
	w = n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", token, nil)
	if w.Code != 403 {
		t.Fatalf("khả năng cho người ngoài: %d", w.Code)
	}
}

// A block before the call, whoever blocked whom, refuses the call and the
// capabilities with 403; so does the other person's deleted account.
func TestCapChanHoacXoaTruocKhiGoi(t *testing.T) {
	for name, dong := range map[string]func(n nhomGo, t *testing.T){
		"tôi chặn":       func(n nhomGo, t *testing.T) { n.f.chan_(t, n.f.person, n.f.peer) },
		"người kia chặn": func(n nhomGo, t *testing.T) { n.f.chan_(t, n.f.peer, n.f.person) },
		"người kia xoá":  func(n nhomGo, t *testing.T) { n.f.exec(t, `UPDATE people SET deleted_at=now() WHERE id=$1`, n.f.peer) },
		"người kia rời phòng": func(n nhomGo, t *testing.T) {
			n.f.exec(t, `UPDATE memberships SET state='left',left_at=now() WHERE context_id=$1 AND person_id=$2`, n.f.context, n.f.peer)
		},
	} {
		t.Run(name, func(t *testing.T) {
			n := setupCap(t)
			dong(n, t)
			if job, code, ma := n.goiCap(t, "hoi"); job != nil || code != 403 || ma != "membership_required" {
				t.Fatalf("%d %s", code, ma)
			}
			w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
			if w.Code != 403 || maTraVe(w.Body.String()) != "membership_required" {
				t.Fatalf("khả năng: %d %s", w.Code, w.Body.String())
			}
			// Refused at preflight: nothing written, the provider never probed.
			if got := n.f.capabilityCalls.Load(); n.f.demLoiGoi(t) != 0 || got != 0 {
				t.Fatalf("ghi %d lời gọi, dò nhà cung cấp %d lần", n.f.demLoiGoi(t), got)
			}
		})
	}
	// A declined request after an old block is not a lift: the latest edge
	// that was not declined decides, as in chatlegacychange.
	n := setupCap(t)
	n.f.chan_(t, n.f.peer, n.f.person)
	n.f.exec(t, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at,created_at) VALUES($1,$2,$3,'declined',$3,now(),now()+interval '1 second')`, newID(), n.f.person, n.f.peer)
	if job, code, _ := n.goiCap(t, "hoi"); job != nil || code != 403 {
		t.Fatalf("chặn cũ bị một lời từ chối gỡ: %d", code)
	}
}

// A block between the call and the worker's reads: the worker refuses
// before any model call, and nothing is posted.
func TestCapChanTruocKhiWorkerDoc(t *testing.T) {
	n := setupCap(t, kichCap()...)
	job, code, ma := n.goiCap(t, "hoi")
	if job == nil {
		t.Fatalf("%d %s", code, ma)
	}
	n.f.chan_(t, n.f.peer, n.f.person)
	n.chayViec(t)
	status, c, cards := n.trangThai(t, job.ID)
	if status != "failed" || c != "sharing_unavailable" || cards != 0 || n.stub.SoGoi() != 0 {
		t.Fatalf("%s %s thẻ %d, mô hình %d lần", status, c, cards, n.stub.SoGoi())
	}
}

// A block while the model writes: the worker asks again at publish and
// posts nothing; the job fails with the generic sharing code.
func TestCapChanGiuaLuot(t *testing.T) {
	for name, dong := range map[string]func(n nhomGo, t *testing.T){
		"chặn": func(n nhomGo, t *testing.T) { n.f.chan_(t, n.f.person, n.f.peer) },
		"xoá":  func(n nhomGo, t *testing.T) { n.f.exec(t, `UPDATE people SET deleted_at=now() WHERE id=$1`, n.f.peer) },
	} {
		t.Run(name, func(t *testing.T) {
			n := setupCap(t, kichCap()...)
			job, code, ma := n.goiCap(t, "hoi")
			if job == nil {
				t.Fatalf("%d %s", code, ma)
			}
			n.f.handler.truocChot = func(context.Context) { dong(n, t) }
			n.chayViec(t)
			status, c, cards := n.trangThai(t, job.ID)
			if status != "failed" || c != "sharing_unavailable" || cards != 0 || n.stub.SoGoi() != 3 {
				t.Fatalf("%s %s thẻ %d, mô hình %d lần", status, c, cards, n.stub.SoGoi())
			}
		})
	}
}

// A group is unchanged: a block between two of its members is theirs, not
// the room's, and `hoi` still runs to its answer; chat-capabilities still
// offers plan and chia_bill.
func TestCapNhomKhongDoi(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{}, kichCap()...)
	n.f.chan_(t, n.f.peer, n.f.person)
	w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"plan":{"available":true,"reason":null}`) || !strings.Contains(w.Body.String(), `"hoi":{"available":true,"reason":null}`) {
		t.Fatalf("khả năng nhóm: %d %s", w.Code, w.Body.String())
	}
	job, code, ma := n.goiCap(t, "hoi")
	if job == nil {
		t.Fatalf("%d %s", code, ma)
	}
	n.chayViec(t)
	if status, c, cards := n.trangThai(t, job.ID); status != "succeeded" || cards != 1 {
		t.Fatalf("%s %s %d", status, c, cards)
	}
}
