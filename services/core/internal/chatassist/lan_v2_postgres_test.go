//go:build postgres

package chatassist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"mobile/services/core/internal/chatv2"
)

// The group AI in an end-to-end room (ADR-0057 §6), against a real database:
// the server checks the trigger and the shared turns against what the lane
// records in the clear, never posts into the room, keeps the answer for the
// caller's device only until it is delivered, and gives every member the
// answer's digest. Synthetic data only.

type phongV2 struct {
	f                   fixture
	thietBi, thietBiBan string
	seq                 int64
}

// laV2 puts the fixture's room on the chat v2 lane, with one device each.
func laV2(t *testing.T, f fixture) *phongV2 {
	t.Helper()
	ctx := context.Background()
	if err := chatv2.Migrate(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	p := &phongV2{f: f, thietBi: newID(), thietBiBan: newID()}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO chat_v2_devices(id,person_id,signing_key) VALUES($1,$2,$4),($3,$5,$4)`, p.thietBi, f.person, p.thietBiBan, make([]byte, 32), f.peer)
	exec(`INSERT INTO chat_v2_conversations(context_id,epoch,ready,last_sequence) VALUES($1,1,true,0)`, f.context)
	return p
}

// tin records one envelope on the lane, sent by person's device under
// logical; it returns the logical id and the sequence it took.
func (p *phongV2) tin(t *testing.T, person, logical string) (string, int64) {
	t.Helper()
	ctx := context.Background()
	device := p.thietBi
	if person == p.f.peer {
		device = p.thietBiBan
	}
	p.seq++
	if _, err := p.f.pool.Exec(ctx, `INSERT INTO chat_v2_events(context_id,sequence,kind,actor_id,body) VALUES($1,$2,'envelope',$3,'{}')`, p.f.context, p.seq, person); err != nil {
		t.Fatal(err)
	}
	if _, err := p.f.pool.Exec(ctx, `INSERT INTO chat_v2_sends(context_id,device_id,logical_send_id,digest,sequence) VALUES($1,$2,$3,$4,$5)`, p.f.context, device, logical, make([]byte, 32), p.seq); err != nil {
		t.Fatal(err)
	}
	if _, err := p.f.pool.Exec(ctx, `UPDATE chat_v2_conversations SET last_sequence=$2 WHERE context_id=$1`, p.f.context, p.seq); err != nil {
		t.Fatal(err)
	}
	return logical, p.seq
}

func TestAiTrongPhongV2(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	p := laV2(t, f)
	w := f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil)
	requireCode(t, w, 200)
	if !strings.Contains(w.Body.String(), `"protocol":"v2"`) || !strings.Contains(w.Body.String(), `"share_scope":"caller_attached"`) {
		t.Fatalf("chat-capabilities của phòng v2: %s", w.Body.String())
	}
	a, _ := p.tin(t, f.peer, newID())
	b, _ := p.tin(t, f.person, newID())
	trigger, _ := p.tin(t, f.person, newID())
	w = f.goiTag(f.token, newID(), trigger, map[string]any{"boi_canh": goiThu(luotThu(a, "Ăn lẩu đi"), luotThu(b, "Q1 nha"))})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if job.TriggerV2 == nil || *job.TriggerV2 != trigger || job.TriggerMessageID != nil || job.SoTinDoc == nil || *job.SoTinDoc != 2 {
		t.Fatalf("lời gọi v2 sai hình: %s", w.Body.String())
	}
	var messagesBefore int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE context_id=$1`, f.context).Scan(&messagesBefore)
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var messagesAfter int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE context_id=$1`, f.context).Scan(&messagesAfter)
	if messagesAfter != messagesBefore {
		t.Fatal("máy chủ đăng câu trả lời vào phòng v2 dưới dạng chữ rõ")
	}
	w = f.request("GET", f.route()+"/"+job.ID, f.token, nil)
	requireCode(t, w, 200)
	var got Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Status != "succeeded" || got.MessageID != nil || got.TheV2 == nil || got.TheDigest == nil {
		t.Fatalf("câu trả lời v2 không chờ máy người gọi: %s", w.Body.String())
	}
	sum := sha256.Sum256([]byte(*got.TheV2))
	if hex.EncodeToString(sum[:]) != *got.TheDigest {
		t.Fatal("digest không phải sha256 của đúng các byte thẻ")
	}
	var the theTraLoi
	if err := json.Unmarshal([]byte(*got.TheV2), &the); err != nil || the.Payload.InvocationID != job.ID || the.Payload.Doc.SoTin != 2 {
		t.Fatalf("thẻ v2 sai hình: %s", *got.TheV2)
	}
	// Before delivery the room has nothing to check: no receipt yet.
	if w = f.request("GET", f.route()+"/"+job.ID+"/receipt", f.peerToken, nil); w.Code != 404 {
		t.Fatalf("biên nhận trước khi giao: %d %s", w.Code, w.Body.String())
	}
	digest := *got.TheDigest
	if w = f.request("GET", f.route()+"/"+job.ID, f.peerToken, nil); w.Code != 404 {
		t.Fatalf("thành viên khác đọc được lời gọi của người gọi: %d", w.Code)
	}
	// Delivery: only the caller's envelope carrying this answer (its logical
	// id is the invocation's), not the peer's, not another of the caller's.
	_, cuaBan := p.tin(t, f.peer, job.ID)
	_, tinKhac := p.tin(t, f.person, newID())
	for _, seq := range []int64{cuaBan, tinKhac, 999} {
		if w = f.request("POST", f.route()+"/"+job.ID+"/delivered", f.token, map[string]any{"sequence": seq}); w.Code != 422 || maTuChoi(w) != "delivery_mismatch" {
			t.Fatalf("giao vào sequence %d: %d %s", seq, w.Code, w.Body.String())
		}
	}
	_, the1 := p.tin(t, f.person, job.ID)
	w = f.request("POST", f.route()+"/"+job.ID+"/delivered", f.token, map[string]any{"sequence": the1})
	requireCode(t, w, 200)
	got = Invocation{}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.TheV2 != nil || got.DeliveredSequence == nil || *got.DeliveredSequence != the1 || got.TheDigest == nil {
		t.Fatalf("sau khi giao máy chủ còn giữ câu trả lời: %s", w.Body.String())
	}
	var held *string
	_ = f.pool.QueryRow(ctx, `SELECT v2_result FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&held)
	if held != nil {
		t.Fatal("v2_result còn trong DB sau khi giao")
	}
	requireCode(t, f.request("POST", f.route()+"/"+job.ID+"/delivered", f.token, map[string]any{"sequence": the1}), 200)
	if w = f.request("POST", f.route()+"/"+job.ID+"/delivered", f.token, map[string]any{"sequence": the1 + 1}); w.Code != 409 {
		t.Fatalf("giao lần hai vào chỗ khác: %d", w.Code)
	}
	// After delivery: any member reads the digest and who asked, never the answer.
	w = f.request("GET", f.route()+"/"+job.ID+"/receipt", f.peerToken, nil)
	requireCode(t, w, 200)
	if !strings.Contains(w.Body.String(), digest) || !strings.Contains(w.Body.String(), f.person) || strings.Contains(w.Body.String(), "the_v2") {
		t.Fatalf("biên nhận cho thành viên khác: %s", w.Body.String())
	}
	// chia_bill cannot be checked on this lane: off, and said so.
	w = f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil)
	if !strings.Contains(w.Body.String(), `"chia_bill":{"available":false,"reason":"chia_bill_unavailable_encrypted"}`) {
		t.Fatalf("chia_bill trong phòng v2: %s", w.Body.String())
	}
	trig2, _ := p.tin(t, f.person, newID())
	if w = f.goiTag(f.token, newID(), trig2, map[string]any{"command": "chia_bill"}); w.Code != 409 || maTuChoi(w) != "chia_bill_unavailable_encrypted" {
		t.Fatalf("chia_bill trong phòng v2: %d %s", w.Code, w.Body.String())
	}
	// What can write into the legacy room stays refused on this lane.
	if w = f.request("GET", "/contexts/"+f.context+"/shared-drafts/"+newID(), f.token, nil); w.Code != 409 || maTuChoi(w) != "encrypted_invocation_required" {
		t.Fatalf("tờ hẹn chung trong phòng v2: %d %s", w.Code, w.Body.String())
	}
}

func TestAiTrongPhongV2TuChoi(t *testing.T) {
	f := setup(t, nil)
	p := laV2(t, f)
	cuaBan, _ := p.tin(t, f.peer, newID())
	cuaToi, _ := p.tin(t, f.person, newID())
	// A logical id two people sent under names nobody.
	chung := newID()
	p.tin(t, f.person, chung)
	p.tin(t, f.peer, chung)
	cases := []struct {
		ten     string
		trigger string
		extra   map[string]any
		code    int
		ma      string
	}{
		{"tin tag của người khác", cuaBan, nil, 422, "trigger_khong_hop_le"},
		{"tin tag của làn cũ", f.tinTag(t, f.context, f.person), nil, 422, "trigger_khong_hop_le"},
		{"lượt không thuộc phòng", cuaToi, map[string]any{"boi_canh": goiThu(luotThu(newID(), "lạ"))}, 422, "boi_canh_mismatch"},
		{"lượt hai người cùng gửi", cuaToi, map[string]any{"boi_canh": goiThu(luotThu(chung, "ai nói?"))}, 422, "boi_canh_mismatch"},
	}
	for _, c := range cases {
		if w := f.goiTag(f.token, newID(), c.trigger, c.extra); w.Code != c.code || maTuChoi(w) != c.ma {
			t.Errorf("%s: %d %s", c.ten, w.Code, w.Body.String())
		}
	}
	body := map[string]any{"logical_id": newID(), "command": "plan", "prompt": "tối nay ăn gì?"}
	if w := f.request("POST", f.route(), f.token, body); w.Code != 400 || maTuChoi(w) != "trigger_required" {
		t.Errorf("không tin tag: %d %s", w.Code, w.Body.String())
	}
	if n := f.demLoiGoi(t); n != 0 {
		t.Fatalf("có %d hàng sau các lần từ chối", n)
	}
}
