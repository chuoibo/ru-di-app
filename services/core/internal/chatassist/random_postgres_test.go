//go:build postgres

package chatassist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/traloi"
	"mobile/services/core/internal/aiharness/truyhoi"
)

func TestRandomSourceCapabilityAndRefusal(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{})
	w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
	requireCode(t, w, 200)
	if !strings.Contains(w.Body.String(), `"random":{"available":false,"reason":"random_source_unavailable"}`) {
		t.Fatalf("a provider without retrieval advertises random: %s", w.Body.String())
	}
	if _, code, ma := n.goiCap(t, "random"); code != 503 || ma != "random_source_unavailable" {
		t.Fatalf("source-less search accepted: %d %s", code, ma)
	}
	if n.f.demLoiGoi(t) != 0 {
		t.Fatal("source-less search entered the queue")
	}
}

func TestRandomMigrationV8KeepsOlderRows(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{Quan: &testkit.TheoLuot{}})
	ctx := context.Background()
	job, code, ma := n.goiCap(t, "plan")
	if code != 202 || job == nil {
		t.Fatalf("old plan: %d %s", code, ma)
	}
	if _, err := n.f.pool.Exec(ctx, `ALTER TABLE chat_ai_invocations DROP CONSTRAINT chat_ai_command_scope, ADD CONSTRAINT chat_ai_command_scope CHECK ((scope='group' AND command IN ('plan','chia_bill','hoi')) OR (scope='me' AND command='hoi'))`); err != nil {
		t.Fatal(err)
	}
	if _, err := n.f.pool.Exec(ctx, `DELETE FROM chat_ai_schema_migrations WHERE version=8`); err != nil {
		t.Fatal(err)
	}
	if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET command='random' WHERE id=$1`, job.ID); err == nil {
		t.Fatal("version 7 accepted random")
	}
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, n.f.pool); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET command='random' WHERE id=$1`, job.ID); err != nil {
		t.Fatalf("version 8 refused random: %v", err)
	}
	if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET command='randomize' WHERE id=$1`, job.ID); err == nil {
		t.Fatal("version 8 admitted an unknown command")
	}
	if n.f.demLoiGoi(t) != 1 {
		t.Fatal("migration lost the older invocation")
	}
	if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_schema_migrations SET digest='changed' WHERE version=8`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, n.f.pool); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("version 8 checksum changed silently: %v", err)
	}
}

// Exercise distinct producer and worker handlers, the actual queue,
// PostgreSQL catalogue, command metrics and immutable v2 result receipt.
// The producer makes no inference; Go publishes no plaintext message.
func TestRandomSearchV2CatalogueAndReceipt(t *testing.T) {
	ctx := context.Background()
	dd, first, second := "dd-"+newID(), "q-"+newID(), "q-"+newID()
	retr := &testkit.TheoLuot{KetQua: []truyhoi.KetQuaTruyHoi{{BangChung: []truyhoi.BangChung{
		{ID: first, Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán Một"}},
		{ID: second, Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán Hai"}},
	}}}}
	n := setupNhomGo(t, tools.NguonDuLieu{Quan: retr, Cho: testkit.Cho{DiemDens: []truyhoi.BangChung{{ID: dd, Truong: map[string]string{"ten": "Đà Lạt"}}}}},
		ruNhom(map[string]any{"y_dinh": []string{"plan"}, "slots": map[string]any{"diem_den_id": dd}}),
		llm.Buoc{Text: `{"ket_luan":"du","rang_buoc_thieu":[]}`},
		llm.Buoc{Text: `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"Bạn xem [[p:p1]] và [[p:p2]] nhé.","bang_chung":["p1","p2"],"trich":[]}]}`},
		kiemNhom("ho_tro", "e1", "e2"))
	worker := n.f.handler
	producerRetr := &testkit.TheoLuot{}
	n.f.handler = New(n.f.pool).WithCoMay().WithRandomNguon(producerRetr)
	if _, err := n.f.pool.Exec(ctx, `INSERT INTO destinations(id,name,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES($1,'Đà Lạt',11.94,108.44,11.8,108.3,12.1,108.6,0)`, dd); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ id, name string }{{first, "Quán Một"}, {second, "Quán Hai"}} {
		if _, err := n.f.pool.Exec(ctx, `INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,geo_precision,traits,description,source,price_min_vnd) VALUES($1,$2,$3,'quan_an','[]'::jsonb,11.9,108.4,'rooftop','[]'::jsonb,'Địa điểm tổng hợp để kiểm thử','curated',50000)`, q.id, dd, q.name); err != nil {
			t.Fatal(err)
		}
	}
	p := laV2(t, n.f)
	w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
	requireCode(t, w, 200)
	if !strings.Contains(w.Body.String(), `"random":{"available":true,"reason":null}`) {
		t.Fatalf("configured catalogue search is unavailable: %s", w.Body.String())
	}
	trigger, _ := p.tin(t, n.f.person, newID())
	body := map[string]any{"logical_id": newID(), "command": "random", "prompt": "/random quán ăn Đà Lạt", "trigger_message_id": trigger, "boi_canh": goiThu()}
	w = n.f.request("POST", n.f.route(), n.f.token, body)
	requireCode(t, w, 202)
	var job Invocation
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	requireCode(t, n.f.request("POST", n.f.route(), n.f.token, body), 200)
	body["command"] = "plan"
	requireCode(t, n.f.request("POST", n.f.route(), n.f.token, body), 409)
	if n.stub.SoGoi() != 0 || len(producerRetr.Da) != 0 || len(retr.Da) != 0 {
		t.Fatal("queueing performed model inference or catalogue retrieval")
	}
	if ok, err := worker.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("random worker: %v %v", ok, err)
	}
	if len(producerRetr.Da) != 0 || n.stub.SoGoi() != 4 {
		t.Fatalf("search ran on the wrong process: producer retrievals=%d worker model calls=%d", len(producerRetr.Da), n.stub.SoGoi())
	}
	w = n.f.request("GET", n.f.route()+"/"+job.ID, n.f.token, nil)
	requireCode(t, w, 200)
	var got Invocation
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "succeeded" || got.TheV2 == nil || got.TheDigest == nil || got.MessageID != nil {
		t.Fatalf("random v2 result: %s", w.Body.String())
	}
	card := *got.TheV2
	for _, want := range []string{`"lenh": "random"`, `"kind": "places"`, `"id": "` + first + `"`, `"id": "` + second + `"`, `"name": "Qu`, `"source": "curated"`} {
		if !strings.Contains(card, want) {
			t.Fatalf("random lacks grounded field %s: %s", want, card)
		}
	}
	if strings.Contains(card, `"kind": "itinerary"`) || strings.Contains(card, `"kind": "expense_draft"`) {
		t.Fatalf("random made another workflow: %s", card)
	}
	sum := sha256.Sum256([]byte(card))
	digest := hex.EncodeToString(sum[:])
	if digest != *got.TheDigest {
		t.Fatal("random result digest is not the exact card bytes")
	}
	var command, version string
	if err := n.f.pool.QueryRow(ctx, `SELECT lenh,prompt_version FROM ai_turn_metrics WHERE invocation_id=$1`, job.ID).Scan(&command, &version); err != nil || command != "random" || version != traloi.VersionRandom() {
		t.Fatalf("random metrics: %q %q %v", command, version, err)
	}
	var plaintext int
	if err := n.f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE context_id=$1 AND kind='ai_card'`, n.f.context).Scan(&plaintext); err != nil || plaintext != 0 {
		t.Fatalf("random published plaintext: %d %v", plaintext, err)
	}
	_, sequence := p.tin(t, n.f.person, job.ID)
	requireCode(t, n.f.request("POST", n.f.route()+"/"+job.ID+"/delivered", n.f.token, map[string]any{"sequence": sequence}), 200)
	w = n.f.request("GET", n.f.route()+"/"+job.ID+"/receipt", n.f.peerToken, nil)
	requireCode(t, w, 200)
	if !strings.Contains(w.Body.String(), digest) || !strings.Contains(w.Body.String(), n.f.person) {
		t.Fatalf("random receipt changed provenance: %s", w.Body.String())
	}
}
