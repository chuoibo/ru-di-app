//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/auth"
)

// Rủ Đi AI in a chat of two against a real database. A chat of two is a
// room of friends (decision 2026-09-28, two classes): it takes everything a
// group takes -- `hoi`, plan, chia_bill, the shared sheet and plan
// promotion -- on the Go engine only (the brain has no path for it); a pair
// whose other person blocked, was blocked or deleted their account is
// refused at the route and again by the worker, before it reads and before
// it publishes; chat-capabilities says `cap_doi` only when the two both
// said yes to «Một đôi»; a group is unchanged.

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

// khaNang is chat-capabilities as the tests read it.
type khaNang struct {
	AI struct {
		Plan     map[string]any `json:"plan"`
		ChiaBill map[string]any `json:"chia_bill"`
		Hoi      map[string]any `json:"hoi"`
		Mention  bool           `json:"mention"`
	} `json:"ai"`
	CapDoi *bool `json:"cap_doi"`
}

func (f fixture) khaNang(t *testing.T, token string) khaNang {
	t.Helper()
	w := f.request("GET", "/contexts/"+f.context+"/chat-capabilities", token, nil)
	requireCode(t, w, 200)
	var caps khaNang
	if err := json.Unmarshal(w.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.CapDoi == nil {
		t.Fatalf("chat-capabilities thiếu cap_doi: %s", w.Body.String())
	}
	return caps
}

// batDoi writes a live notebook cycle of the room's two people and a
// «Một đôi» (bat_doi) proposal; each person in daDongY has said yes. With
// both, the proposal is completed, as the notebook's own flow leaves it.
func (f fixture) batDoi(t *testing.T, daDongY ...string) {
	t.Helper()
	notebook, cycle, proposal := newID(), newID(), newID()
	f.exec(t, `INSERT INTO pair_notebooks(id,context_id) VALUES($1,$2)`, notebook, f.context)
	f.exec(t, `INSERT INTO pair_notebook_cycles(id,notebook_id,state,opened_at) VALUES($1,$2,'active',clock_timestamp())`, cycle, notebook)
	f.exec(t, `INSERT INTO pair_cycle_participants(cycle_id,person_id) VALUES($1,$2),($1,$3)`, cycle, f.person, f.peer)
	f.exec(t, `INSERT INTO pair_consent_proposals(id,cycle_id,purpose,proposed_by_id,completed_at,expires_at) VALUES($1,$2,'bat_doi',$3,CASE WHEN $4 THEN clock_timestamp() END,clock_timestamp()+interval '1 day')`, proposal, cycle, f.person, len(daDongY) == 2)
	for _, person := range daDongY {
		f.exec(t, `INSERT INTO pair_consents(id,proposal_id,person_id,granted_at) VALUES($1,$2,$3,clock_timestamp())`, newID(), proposal, person)
	}
}

// A pair's `hoi` is taken and answered in the thread: 202, the worker runs
// the Go engine, the card is a reply to the tag message, the brain is never
// asked for an answer. chat-capabilities offers a chat of two what it
// offers a group: hoi, mention, plan and chia_bill, and no cap_doi.
func TestCapHoiChayToiTraLoi(t *testing.T) {
	n := setupCap(t, kichCap()...)
	caps := n.f.khaNang(t, n.f.token)
	if caps.AI.Hoi["available"] != true || caps.AI.Hoi["reason"] != nil || !caps.AI.Mention ||
		caps.AI.Plan["available"] != true || caps.AI.Plan["reason"] != nil ||
		caps.AI.ChiaBill["available"] != true || caps.AI.ChiaBill["reason"] != nil || *caps.CapDoi {
		t.Fatalf("khả năng của cặp: %+v", caps)
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
	// The other person reads their own capabilities the same way.
	w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.peerToken, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"hoi":{"available":true,"reason":null}`) || !strings.Contains(w.Body.String(), `"plan":{"available":true,"reason":null}`) {
		t.Fatalf("người kia: %d %s", w.Code, w.Body.String())
	}
}

// plan in a chat of two is taken (202) and runs to a card in the thread, as
// in a group.
func TestCapPlanChayToiThe(t *testing.T) {
	n := setupCap(t, kichCap()...)
	job, code, ma := n.goiCap(t, "plan")
	if job == nil || code != 202 {
		t.Fatalf("plan trong cặp: %d %s", code, ma)
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
	if the.Kind != "tra_loi" || the.Payload.Lenh != "plan" {
		t.Fatalf("thẻ: %s", k.card)
	}
}

// chia_bill in a chat of two is taken and drafts the split between the two,
// reading the stored text of the shared message (ADR-0046 §8.3 covers a
// chat of two as it covers a group): the bundle says «5tr», the database
// says «850k»; the draft bills the author 850.000đ, split in two.
func TestCapChiaBillDocChuDaLuu(t *testing.T) {
	n := setupCap(t,
		ruNhom(map[string]any{"tien": "split_draft", "y_dinh": []string{"chia_bill_draft"}}),
		llm.Buoc{Text: `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000}]}`},
		llm.Buoc{Text: `{"khoan":[{"so":1,"ket":"ho_tro"}]}`})
	a := n.f.tinTrongPhong(t, n.f.context, "Mình trả lẩu 850k")
	id, _ := n.hoi(t, "chia_bill", "/chia-bill", goiThu(luotThu(a, "Mình trả lẩu 5tr")))
	k := n.ket(t, id)
	if k.status != "succeeded" || k.duong != "nhap_chia_bill" || k.soGoi != 3 {
		t.Fatalf("%s %v %+v", k.status, k.code, k)
	}
	if req := string(n.stub.YeuCau()[1]); strings.Contains(req, "5tr") || !strings.Contains(req, "lẩuˆ850k") {
		t.Fatalf("bản đọc nhận bản sao của máy khách, không phải tin đã lưu:\n%s", req)
	}
	var kq struct {
		Drafts []struct {
			PaidByID  string `json:"paid_by_id"`
			AmountVND int64  `json:"amount_vnd"`
		} `json:"drafts"`
		ChiaDeu []struct {
			AmountVND int64 `json:"amount_vnd"`
		} `json:"chia_deu"`
	}
	if err := json.Unmarshal(k.result, &kq); err != nil {
		t.Fatal(err)
	}
	if len(kq.Drafts) != 1 || kq.Drafts[0].PaidByID != n.f.peer || kq.Drafts[0].AmountVND != 850000 || len(kq.ChiaDeu) != 2 ||
		kq.ChiaDeu[0].AmountVND+kq.ChiaDeu[1].AmountVND != 850000 {
		t.Fatalf("nháp: %s", k.result)
	}
}

// The shared sheet and «thành kèo» in a chat of two: created and promoted as
// in a group; once the pair is blocked, both are refused like every other
// route of the room.
func TestCapToHenVaThanhKeo(t *testing.T) {
	n := setupCap(t)
	sheet := map[string]any{"title": "Tối thứ 7", "stops": []map[string]any{{"time_text": "19:00", "label": "Ăn lẩu"}}}
	w := n.f.request("POST", "/contexts/"+n.f.context+"/shared-drafts", n.f.token, sheet)
	requireCode(t, w, 201)
	var draft struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	source := newID()
	n.f.exec(t, `INSERT INTO messages(id,context_id,kind,card) VALUES($1,$2,'ai_card',$3)`, source, n.f.context, `{"kind":"itinerary","payload":{"title":"Synthetic plan","stops":[{"time_text":"18:30","place":{"id":"","name":"Synthetic place"}}]}}`)
	input := promotionInput{Source: source, Title: "Synthetic confirmed plan", Starts: "2026-10-01", Ends: "2026-10-01", Headcount: 2, Budget: 100000, Stops: []planStop{{At: "18:30", Label: "Synthetic stop"}}}
	route := "/contexts/" + n.f.context + "/plan-promotions"
	requireCode(t, n.f.request("POST", route, n.f.token, input), 201)

	n.f.chan_(t, n.f.peer, n.f.person)
	w = n.f.request("POST", "/contexts/"+n.f.context+"/shared-drafts", n.f.token, sheet)
	if w.Code != 403 || maTraVe(w.Body.String()) != "membership_required" {
		t.Fatalf("tờ hẹn sau khi chặn: %d %s", w.Code, w.Body.String())
	}
	w = n.f.request("PATCH", "/contexts/"+n.f.context+"/shared-drafts/"+draft.ID, n.f.token, map[string]any{"title": "Đổi", "revision": draft.Revision})
	if w.Code != 403 || maTraVe(w.Body.String()) != "membership_required" {
		t.Fatalf("sửa tờ hẹn sau khi chặn: %d %s", w.Code, w.Body.String())
	}
	w = n.f.request("POST", "/contexts/"+n.f.context+"/shared-drafts/"+draft.ID+"/discard", n.f.token, nil)
	if w.Code != 403 || maTraVe(w.Body.String()) != "membership_required" {
		t.Fatalf("bỏ tờ hẹn sau khi chặn: %d %s", w.Code, w.Body.String())
	}
	source2 := newID()
	n.f.exec(t, `INSERT INTO messages(id,context_id,kind,card) VALUES($1,$2,'ai_card',$3)`, source2, n.f.context, `{"kind":"itinerary","payload":{"title":"Synthetic plan","stops":[{"time_text":"18:30","place":{"id":"","name":"Synthetic place"}}]}}`)
	input.Source = source2
	w = n.f.request("POST", route, n.f.token, input)
	if w.Code != 403 || maTraVe(w.Body.String()) != "membership_required" {
		t.Fatalf("thành kèo sau khi chặn: %d %s", w.Code, w.Body.String())
	}
}

// cap_doi says the room is a couple: a pair whose two people both said yes
// to «Một đôi». One yes is not enough; a group never is; the other person
// reads the same answer.
func TestCapDoiChiKhiCaHaiBatDoi(t *testing.T) {
	n := setupCap(t)
	if caps := n.f.khaNang(t, n.f.token); *caps.CapDoi {
		t.Fatal("cặp chưa có sổ đôi mà cap_doi=true")
	}
	n.f.batDoi(t, n.f.person)
	if caps := n.f.khaNang(t, n.f.token); *caps.CapDoi {
		t.Fatal("một người đồng ý mà cap_doi=true")
	}
	m := setupCap(t)
	m.f.batDoi(t, m.f.person, m.f.peer)
	for _, token := range []string{m.f.token, m.f.peerToken} {
		caps := m.f.khaNang(t, token)
		if !*caps.CapDoi || caps.AI.Plan["available"] != true || caps.AI.Hoi["available"] != true {
			t.Fatalf("cặp đôi: %+v", caps)
		}
	}
	g := setupNhomGo(t, tools.NguonDuLieu{})
	g.f.batDoi(t, g.f.person, g.f.peer)
	if caps := g.f.khaNang(t, g.f.token); *caps.CapDoi {
		t.Fatal("nhóm mà cap_doi=true")
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
			// Refused at preflight: nothing written, the model never asked.
			if n.f.demLoiGoi(t) != 0 || n.stub.SoGoi() != 0 {
				t.Fatalf("ghi %d lời gọi, model %d lần", n.f.demLoiGoi(t), n.stub.SoGoi())
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

// heThongStub reads the system instruction of every request the stub saw.
func heThongStub(t *testing.T, stub *llm.Stub) []string {
	t.Helper()
	var out []string
	for _, y := range stub.YeuCau() {
		var r struct {
			Config struct {
				SystemInstruction struct{ Parts []struct{ Text string } } `json:"systemInstruction"`
			} `json:"config"`
		}
		if err := json.Unmarshal(y, &r); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, p := range r.Config.SystemInstruction.Parts {
			b.WriteString(p.Text)
		}
		out = append(out, b.String())
	}
	return out
}

// The two classes end to end (2026-09-28, metrics v6): the worker reads the
// room's class from the database (laDoi) in the transaction that reads the
// room. A pair whose two people both said yes to «Một đôi» is a couple: its
// router and its answer read the couple's instructions, and its metrics row
// says bot doi with the couple's prompt version. A pair with one yes, or
// none, is a room of friends: the group's instructions, bot nhom, the
// group's version -- exactly a group's row. The card is the same either way.
func TestCapDoiGhiBotDoi(t *testing.T) {
	routerNhom, _ := hieu.LoiNhac(obs.BotNhom)
	for _, c := range []struct {
		ten     string
		dongY   int
		bot     obs.Bot
		version string
		router  string
		agent   string
	}{
		{"đám bạn, chưa sổ đôi", 0, obs.BotNhom, prompts.VersionNhom(), routerNhom, prompts.NhomAgent("pg9nhom7x2kq")},
		{"đám bạn, một người đồng ý", 1, obs.BotNhom, prompts.VersionNhom(), routerNhom, prompts.NhomAgent("pg9nhom7x2kq")},
		{"cặp đôi", 2, obs.BotDoi, prompts.VersionDoi(), hieu.LoiNhacDoi(), prompts.DoiAgent("pg9nhom7x2kq")},
	} {
		n := setupCap(t, kichCap()...)
		if c.dongY > 0 {
			n.f.batDoi(t, []string{n.f.person, n.f.peer}[:c.dongY]...)
		}
		if caps := n.f.khaNang(t, n.f.token); *caps.CapDoi != (c.bot == obs.BotDoi) {
			t.Fatalf("%s: cap_doi=%v", c.ten, *caps.CapDoi)
		}
		job, code, ma := n.goiCap(t, "hoi")
		if job == nil {
			t.Fatalf("%s: %d %s", c.ten, code, ma)
		}
		n.chayViec(t)
		k := n.ket(t, job.ID)
		if k.status != "succeeded" || k.reply == nil || *k.reply != *job.TriggerMessageID {
			t.Fatalf("%s: %s %v %v", c.ten, k.status, k.code, k.reply)
		}
		if k.bot != string(c.bot) || k.version != c.version || k.duong != string(obs.DuongThang) {
			t.Fatalf("%s: metrics row bot %q version %q duong %q, want %q %q", c.ten, k.bot, k.version, k.duong, c.bot, c.version)
		}
		he := heThongStub(t, n.stub)
		if len(he) != 3 || he[0] != c.router || !strings.HasPrefix(he[1], c.agent) {
			t.Fatalf("%s: the router or the answer read another class's instruction (%d requests)", c.ten, len(he))
		}
		var the theTraLoi
		if err := json.Unmarshal(k.card, &the); err != nil || the.Kind != "tra_loi" || !strings.Contains(string(the.Payload.Phan[0]), "Chào hai bạn") {
			t.Fatalf("%s: thẻ %s", c.ten, k.card)
		}
	}
}
