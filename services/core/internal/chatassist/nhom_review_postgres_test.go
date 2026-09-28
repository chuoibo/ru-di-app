//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/tools"
)

// The review of slices 9/11 against a real database: the group engine's
// guards that only the database can exercise.

// A brain host says `hoi` is not available, whatever else it serves (the
// reviewer's mutant G9: capabilities advertised hoi wherever plan was
// available).
func TestKhaNangBrainKhongCoHoi(t *testing.T) {
	f := setup(t, nil)
	w := f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil)
	requireCode(t, w, 200)
	b := w.Body.String()
	if !strings.Contains(b, `"plan":{"available":true,"reason":null}`) {
		t.Fatalf("identity: the brain host must still offer plan: %s", b)
	}
	if !strings.Contains(b, `"hoi":{"available":false,"reason":"provider_unavailable"}`) {
		t.Fatalf("a brain host advertises hoi: %s", b)
	}
}

// Configuration skew (finding 2.6): the serving process takes `hoi` (its
// flag says the group runs on the Go engine) while the worker runs the
// brain. The worker refuses the job, fail closed, before any read and
// without asking the brain.
func TestHoiTrenWorkerBrainThatBai(t *testing.T) {
	brain := &nepGia{}
	f := setup(t, brain.serve)
	f.handler.WithNhomGo()
	trigger := f.tinTag(t, f.context, f.person)
	w := f.request("POST", f.route(), f.token, map[string]any{"logical_id": newID(), "command": "hoi", "prompt": "@Rủ Đi tối nay đi đâu", "trigger_message_id": trigger})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	f.handler.nhomGo, f.handler.nhomEngine = false, nil // the worker's own flag: the brain
	if ok, err := f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var status, code string
	if err := f.pool.QueryRow(context.Background(), `SELECT status,COALESCE(code,'') FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "provider_unavailable" || brain.calls != 0 {
		t.Fatalf("hoi on a brain worker: %s/%s, brain asked %d times", status, code, brain.calls)
	}
}

// A job is held to the membership row that created it (the reviewer's mutant
// G7): a job whose membership row is no longer the caller's live one ends
// sharing_unavailable, without a model call.
func TestNhomEngineViecGiuDungHangThanhVien(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{}, ruNhom(nil), llm.Buoc{Text: "Chào cả nhóm."}, kiemNhom("khong_thong_tin"))
	trigger := n.f.tinTag(t, n.f.context, n.f.person)
	w := n.f.request("POST", n.f.route(), n.f.token, map[string]any{"logical_id": newID(), "command": "hoi", "prompt": "@Rủ Đi chào", "trigger_message_id": trigger})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	ctx := context.Background()
	// An earlier, closed membership of the same person in the same room: the
	// job now names that row, not the live one.
	cu := newID()
	if _, err := n.f.pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin,left_at) VALUES($1,$2,$3,'left','member','named',clock_timestamp())`, cu, n.f.context, n.f.person); err != nil {
		t.Fatal(err)
	}
	if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET membership_id=$2 WHERE id=$1`, job.ID, cu); err != nil {
		t.Fatal(err)
	}
	if ok, err := n.f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var status, code string
	if err := n.f.pool.QueryRow(ctx, `SELECT status,COALESCE(code,'') FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "sharing_unavailable" || n.stub.SoGoi() != 0 {
		t.Fatalf("a job on another membership row: %s/%s, %d model calls", status, code, n.stub.SoGoi())
	}
}

// The split draft reads the stored text of a shared message, never the
// client's copy (finding 2.3): the bundle says «5tr», the database says
// «850k»; the reading gets the stored words and the draft bills 850.000đ.
func TestNhomEngineChiaBillDocChuDaLuu(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{},
		ruNhom(map[string]any{"tien": "split_draft", "y_dinh": []string{"chia_bill_draft"}}),
		llm.Buoc{Text: `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000}]}`},
		llm.Buoc{Text: `{"khoan":[{"so":1,"ket":"ho_tro"}]}`})
	a := n.f.tinTrongPhong(t, n.f.context, "Mình trả lẩu 850k")
	id, _ := n.hoi(t, "chia_bill", "/chia-bill", goiThu(luotThu(a, "Mình trả lẩu 5tr")))
	k := n.ket(t, id)
	if k.status != "succeeded" || k.soGoi != 3 {
		t.Fatalf("%s %v %+v", k.status, k.code, k)
	}
	req := string(n.stub.YeuCau()[1])
	if strings.Contains(req, "5tr") || !strings.Contains(req, "lẩuˆ850k") {
		t.Fatalf("the reading got the client's copy, not the stored message:\n%s", req)
	}
	if !strings.Contains(string(k.result), `"amount_vnd": 850000`) && !strings.Contains(string(k.result), `"amount_vnd":850000`) {
		t.Fatalf("result: %s", k.result)
	}
	// A shared message the server cannot read as text (deleted since) bills
	// nobody: the reading is not offered it, and with nothing else to read
	// the request alone stands.
	n2 := setupNhomGo(t, tools.NguonDuLieu{},
		ruNhom(map[string]any{"tien": "split_draft", "y_dinh": []string{"chia_bill_draft"}}),
		llm.Buoc{Text: `{"khoan":[]}`})
	b := n2.f.tinTrongPhong(t, n2.f.context, "Mình trả lẩu 850k")
	if _, err := n2.f.pool.Exec(context.Background(), `UPDATE messages SET kind='deleted',body=NULL,deleted_at=clock_timestamp() WHERE id=$1`, b); err != nil {
		t.Fatal(err)
	}
	id2, _ := n2.hoi(t, "chia_bill", "/chia-bill", goiThu(luotThu(b, "Mình trả lẩu 850k")))
	k2 := n2.ket(t, id2)
	if k2.status != "succeeded" || k2.result != nil {
		t.Fatalf("%s %v result %s", k2.status, k2.code, k2.result)
	}
	if req := string(n2.stub.YeuCau()[1]); strings.Contains(req, `<du_lieu nguon=\"tin_nhan\">`) || strings.Contains(req, "lẩuˆ850k") {
		t.Fatalf("a deleted message was offered to the reading:\n%s", req)
	}
}
