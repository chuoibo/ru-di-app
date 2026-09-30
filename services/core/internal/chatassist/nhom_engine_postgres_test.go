//go:build postgres

package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	aimetrics "mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The group assistant end to end on the Go engine against a real database: `hoi` taken with a trigger, the card a `tra_loi`
// reply to the tag message grounded by GroundReply against the catalogue rows
// the worker read, a split draft's drafts on the result column billed to the
// author messages.author_id names, the money refusal posted as the fixed
// sentence, one metrics row per turn.

type nhomGo struct {
	f    fixture
	stub *llm.Stub
}

func ruNhom(o map[string]any) llm.Buoc {
	base := map[string]any{"nhan_guard": "sach", "tien": "none", "y_dinh": []string{"smalltalk"}, "huong": "tra_loi_thang",
		"slots": map[string]any{}, "can_truy_hoi": []string{}, "truy_van": []any{}, "can_hoi_lai": false, "tra_loi_cau_cho": false, "tu_tin": "cao"}
	for k, v := range o {
		base[k] = v
	}
	raw, _ := json.Marshal(base)
	return llm.Buoc{Text: string(raw)}
}

func kiemNhom(ket string, ids ...string) llm.Buoc {
	if ids == nil {
		ids = []string{}
	}
	raw, _ := json.Marshal(map[string]any{"menh_de": []any{map[string]any{"so": 1, "bang_chung_ids": ids, "ket": ket}}, "hua_hanh_dong_khong_co": false, "tien": false})
	return llm.Buoc{Text: string(raw)}
}

func setupNhomGo(t *testing.T, nguon tools.NguonDuLieu, kich ...llm.Buoc) nhomGo {
	t.Helper()
	f := setup(t, nil)
	if err := aimetrics.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	stub := llm.NewStub(kich...)
	var buf bytes.Buffer
	engine, err := aiharness.New(aiharness.WithModel(stub), aiharness.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))),
		aiharness.WithMaKiem("pg9nhom7x2kq"), aiharness.WithRetryWait(func(int) time.Duration { return 0 }), aiharness.WithNguon(nguon))
	if err != nil {
		t.Fatal(err)
	}
	f.handler.WithNhomEngine(engine)
	return nhomGo{f: f, stub: stub}
}

// hoi posts a group `hoi` with a trigger and the shared turns, runs the
// worker once and returns the job id.
func (n nhomGo) hoi(t *testing.T, lenh, prompt string, goi map[string]any) (string, string) {
	t.Helper()
	trigger := n.f.tinTag(t, n.f.context, n.f.person)
	w := n.f.request("POST", n.f.route(), n.f.token, map[string]any{"logical_id": newID(), "command": lenh, "prompt": prompt, "trigger_message_id": trigger, "boi_canh": goi})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if ok, err := n.f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	return job.ID, trigger
}

type ketNhom struct {
	status  string
	code    *string
	reply   *string
	card    []byte
	result  []byte
	version string
	soGoi   int
	duong   string
	// bot is the metrics row's bot: nhom for a room of friends, doi for a
	// couple (metrics v6, two classes).
	bot string
}

func (n nhomGo) ket(t *testing.T, id string) ketNhom {
	t.Helper()
	var k ketNhom
	err := n.f.pool.QueryRow(context.Background(), `SELECT j.status,j.code,m.reply_to_id::text,m.card,j.result FROM chat_ai_invocations j LEFT JOIN messages m ON m.id=j.message_id WHERE j.id=$1`, id).
		Scan(&k.status, &k.code, &k.reply, &k.card, &k.result)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.f.pool.QueryRow(context.Background(), `SELECT prompt_version,so_goi_model,duong,bot FROM ai_turn_metrics WHERE invocation_id=$1 AND bot IN ('nhom','doi')`, id).Scan(&k.version, &k.soGoi, &k.duong, &k.bot); err != nil {
		t.Fatalf("hàng số đo nhóm: %v", err)
	}
	return k
}

func TestNhomQuaEngineGo(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{}, ruNhom(nil), llm.Buoc{Text: "Chào cả nhóm, cần gì cứ gọi mình nhé."}, kiemNhom("khong_thong_tin"))
	a := n.f.tinTrongPhong(t, n.f.context, "Tối nay đi đâu mọi người")
	id, trigger := n.hoi(t, "hoi", "@Rủ Đi chào cả nhóm", goiThu(luotThu(a, "Tối nay đi đâu mọi người")))
	k := n.ket(t, id)
	if k.status != "succeeded" || k.reply == nil || *k.reply != trigger {
		t.Fatalf("%s %v %v", k.status, k.code, k.reply)
	}
	var the theTraLoi
	if err := json.Unmarshal(k.card, &the); err != nil {
		t.Fatal(err)
	}
	if the.Kind != "tra_loi" || the.Payload.Lenh != "hoi" || the.Payload.Doc.SoTin != 1 || len(the.Payload.Phan) != 1 ||
		!strings.Contains(string(the.Payload.Phan[0]), "Chào cả nhóm") {
		t.Fatalf("thẻ: %s", k.card)
	}
	if k.version != prompts.VersionNhom() || k.soGoi != 3 || k.duong != "thang" {
		t.Fatalf("số đo: %+v", k)
	}
	// The member's words reached the router as shared data; the tag
	// message's @mention did not.
	req := string(n.stub.YeuCau()[0])
	if !strings.Contains(req, "Tốiˆnayˆđiˆđâuˆmọiˆngười") {
		t.Fatal("tin chia sẻ không tới router")
	}
	if strings.Contains(req, "@Rủ Đi") {
		t.Fatal("mention tới router")
	}
}

// `hoi` belongs to the thread: refused without a trigger.
func TestNhomHoiCanTinTag(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{})
	w := n.f.request("POST", n.f.route(), n.f.token, map[string]any{"logical_id": newID(), "command": "hoi", "prompt": "x"})
	if w.Code != 400 {
		t.Fatalf("hoi không tin tag: %d", w.Code)
	}
	w = n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
	if !strings.Contains(w.Body.String(), `"hoi":{"available":true,"reason":null}`) {
		t.Fatalf("khả năng: %s", w.Body.String())
	}
}

// A split draft on the Go engine: one reading, the payer the author the
// database names for the shared turn, Σ preview = total, a pointer part.
func TestNhomQuaEngineGoChiaBill(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{},
		ruNhom(map[string]any{"tien": "split_draft", "y_dinh": []string{"chia_bill_draft"}}),
		llm.Buoc{Text: `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000}]}`},
		llm.Buoc{Text: `{"khoan":[{"so":1,"ket":"ho_tro"}]}`})
	a := n.f.tinTrongPhong(t, n.f.context, "Mình trả lẩu 850k")
	id, _ := n.hoi(t, "chia_bill", "/chia-bill", goiThu(luotThu(a, "Mình trả lẩu 850k")))
	k := n.ket(t, id)
	if k.status != "succeeded" || k.duong != "nhap_chia_bill" || k.soGoi != 3 {
		t.Fatalf("%s %v %+v", k.status, k.code, k)
	}
	var kq struct {
		Drafts []struct {
			PaidByID        string  `json:"paid_by_id"`
			AmountVND       int64   `json:"amount_vnd"`
			SourceMessageID *string `json:"source_message_id"`
		} `json:"drafts"`
		ChiaDeu []struct {
			AmountVND int64 `json:"amount_vnd"`
		} `json:"chia_deu"`
	}
	if err := json.Unmarshal(k.result, &kq); err != nil {
		t.Fatal(err)
	}
	if len(kq.Drafts) != 1 || kq.Drafts[0].PaidByID != n.f.peer || kq.Drafts[0].SourceMessageID == nil || *kq.Drafts[0].SourceMessageID != a {
		t.Fatalf("drafts: %s", k.result)
	}
	var tong int64
	for _, c := range kq.ChiaDeu {
		tong += c.AmountVND
	}
	if tong != 850000 || len(kq.ChiaDeu) != 2 {
		t.Fatalf("Σ %d over %d", tong, len(kq.ChiaDeu))
	}
	// jsonb orders keys its own way: read the part, not its bytes.
	c := string(k.card)
	if !strings.Contains(c, `{"kind": "expense_draft", "payload": {"da_ghi": [], "so_khoan": 1}}`) || strings.Contains(c, "850000") || strings.Contains(c, n.f.peer) {
		t.Fatalf("thẻ: %s", k.card)
	}
}

// money_action: the fixed sentence posted as the reply, one call, the job
// succeeded (an answer, not a failure).
func TestNhomQuaEngineGoTuChoiTien(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{}, ruNhom(map[string]any{"tien": "money_action"}), llm.Buoc{Text: "không được gọi"})
	id, _ := n.hoi(t, "hoi", "@Rủ Đi nhắc Minh chuyển tiền cho mình", goiThu())
	k := n.ket(t, id)
	if k.status != "succeeded" || k.soGoi != 1 || k.duong != "tu_choi_tien" || !strings.Contains(string(k.card), "Rủ Đi AI không làm việc tiền nong") {
		t.Fatalf("%s %+v %s", k.status, k, k.card)
	}
	_ = cau.NhomKhongChamTien
}

// A places answer: the card's places part carries the catalogue row the
// worker read from `places`, grounded by GroundReply.
func TestNhomQuaEngineGoQuanTuDanhMuc(t *testing.T) {
	quan := "q-" + newID()
	dd := "dd-" + newID()
	retr := &testkit.TheoLuot{KetQua: []truyhoi.KetQuaTruyHoi{{BangChung: []truyhoi.BangChung{{ID: quan, Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán Gió Đồi"}}}}}}
	nguon := tools.NguonDuLieu{Quan: retr, Cho: testkit.Cho{DiemDens: []truyhoi.BangChung{{ID: dd, Truong: map[string]string{"ten": "Đà Lạt"}}}}}
	n := setupNhomGo(t, nguon,
		ruNhom(map[string]any{"y_dinh": []string{"find_places"}, "huong": "truy_hoi_mot_buoc", "slots": map[string]any{"diem_den_id": dd},
			"can_truy_hoi": []string{"places"}, "truy_van": []any{map[string]string{"nguon": "places", "cau": "quán Đà Lạt"}}}),
		llm.Buoc{Text: `{"ket_luan":"du","rang_buoc_thieu":[]}`},
		llm.Buoc{Text: `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"Cả nhóm thử [[p:p1]] nhé.","bang_chung":["p1"],"trich":[]}]}`},
		kiemNhom("ho_tro", "e1"))
	ctx := context.Background()
	if _, err := n.f.pool.Exec(ctx, `INSERT INTO destinations(id,name,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES($1,'Đà Lạt',11.94,108.44,11.8,108.3,12.1,108.6,0)`, dd); err != nil {
		t.Fatal(err)
	}
	if _, err := n.f.pool.Exec(ctx, `INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,geo_precision,traits,description,source,price_min_vnd) VALUES($1,$2,'Quán Gió Đồi','quan_an','[]'::jsonb,11.9,108.4,'rooftop','[]'::jsonb,'Quán yên tĩnh','curated',50000)`, quan, dd); err != nil {
		t.Fatal(err)
	}
	id, _ := n.hoi(t, "hoi", "@Rủ Đi quán nào ở Đà Lạt", goiThu())
	k := n.ket(t, id)
	if k.status != "succeeded" || k.duong != "truy_hoi" {
		t.Fatalf("%s %v %+v", k.status, k.code, k)
	}
	c := string(k.card)
	if !strings.Contains(c, `"kind": "places"`) || !strings.Contains(c, `"id": "`+quan+`"`) || !strings.Contains(c, `"price_min_vnd": 50000`) || !strings.Contains(c, "Cả nhóm thử Quán Gió Đồi nhé.") {
		t.Fatalf("thẻ không mang hàng danh mục: %s", c)
	}
}
