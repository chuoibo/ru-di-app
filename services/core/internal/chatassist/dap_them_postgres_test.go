package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// What the server lays over the caller's bundle, measured on a real database:
// the catalogue ranked by the group's own taste, the group's budget, and a
// roster of exactly the people still in the room.
//
// Every fact below is chosen so the wrong implementation gives a different
// answer, not just a different reason:
//
//   - by id, the restaurant sorts before the café; only the taste puts the café
//     first. The previous worker (`ORDER BY id LIMIT 40`) fails on the order.
//   - the member who left, and the one invited who never joined, both liked
//     restaurants and spend more. Count either and the two places tie (the
//     restaurant wins on id) and the budget rises to 241666. Only "active
//     members" gives café first and 175000. The invited member is the one that
//     makes the state test observable: a left member is already dropped by
//     `left_at`, an invited one only by `state`.
//   - the member who left spoke in the bundle as «Bạn 2», so the quiet member
//     who never spoke must not be handed «Bạn 2» as well: two people would
//     become one in the model's eyes.
func TestWorkerXepCatalogueTheoGuNhomVaDapRosterThanhVienConO(t *testing.T) {
	var payload map[string]any
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic provider fixture"}})
	})
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}

	quiet, gone, invited := newID(), newID(), newID()
	exec(`INSERT INTO people(id,display_name) VALUES($1,'Synthetic quiet'),($2,'Synthetic gone'),($3,'Synthetic invited')`, quiet, gone, invited)
	exec(`INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','member','named')`, newID(), f.context, quiet)
	exec(`INSERT INTO memberships(id,context_id,person_id,state,role,origin,left_at) VALUES($1,$2,$3,'left','member','named',clock_timestamp())`, newID(), f.context, gone)
	exec(`INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'invited','member','named')`, newID(), f.context, invited)

	// Taste: the caller likes cafés; the two who are not in the room liked
	// restaurants.
	exec(`INSERT INTO person_interests(id,person_id,tag) VALUES($1,$2,'cafe'),($3,$4,'an-uong'),($5,$6,'an-uong')`, newID(), f.person, newID(), gone, newID(), invited)
	// Budget: both active answerers say 100k-250k (midpoint 175000); the two
	// who are not in the room said 250k-500k (midpoint 375000).
	exec(`UPDATE people SET budget_band='vua-phai' WHERE id IN ($1,$2)`, f.person, f.peer)
	exec(`UPDATE people SET budget_band='thoai-mai' WHERE id IN ($1,$2)`, gone, invited)

	exec(`INSERT INTO destinations(id,name,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES('dest-mot','Điểm đến tổng hợp',10.7,106.7,10.6,106.6,10.8,106.8,1)`)
	// No prices, no distance, no capacity: the taste term is the only one
	// that can order these two, so the order measures the taste and nothing
	// else.
	exec(`INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,source) VALUES
		('place-a-quan-com','dest-mot','Quán cơm tổng hợp','quan-an-local','{}',10.77,106.7,'seed'),
		('place-z-ca-phe','dest-mot','Cà phê tổng hợp','cafe','{}',10.77,106.7,'seed')`)

	loiBan := f.tinTrongPhong(t, f.context, "Tối nay mình rảnh")
	var loiCu string
	if err := f.pool.QueryRow(ctx, `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','Synthetic departed line') RETURNING id`, newID(), f.context, gone).Scan(&loiCu); err != nil {
		t.Fatal(err)
	}
	luotCu := luotThu(loiCu, "Tuần sau tao về quê rồi")
	luotCu["biDanh"] = "Bạn 2"
	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Tối nay đi đâu",
		"boi_canh": goiThu(luotThu(loiBan, "Tối nay mình rảnh"), luotCu),
	}), 202)
	if ok, err := f.handler.ProcessOne(ctx); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	if payload == nil {
		t.Fatal("worker không gọi model")
	}

	var thuTu []string
	for _, p := range payload["places"].([]any) {
		thuTu = append(thuTu, p.(map[string]any)["id"].(string))
	}
	if strings.Join(thuTu, ",") != "place-z-ca-phe,place-a-quan-com" {
		t.Fatalf("catalogue gửi model theo thứ tự %v; nhóm thích cà phê thì quán cà phê phải đứng đầu", thuTu)
	}

	if got, _ := payload["budget_per_person_vnd"].(float64); got != 175000 {
		t.Fatalf("ngân sách gửi model là %v, cần 175000 (trung bình hai người đang ở trong phòng, không tính người đã rời hay mới được mời)", payload["budget_per_person_vnd"])
	}

	var ten []string
	for _, m := range payload["members"].([]any) {
		entry := m.(map[string]any)
		if len(entry) != 1 {
			t.Fatalf("một mục roster mang thêm trường: %v", entry)
		}
		ten = append(ten, entry["display_name"].(string))
	}
	if strings.Join(ten, "|") != "Mình|Bạn 1|Bạn 3" {
		t.Fatalf("roster gửi model là %q, cần [Mình Bạn 1 Bạn 3]: đúng ba người đang ở trong phòng, bút danh khớp với lượt chat, không trùng bút danh của người đã rời", ten)
	}

	raw, _ := json.Marshal(payload)
	for _, lo := range []string{f.person, f.peer, quiet, gone, invited, "Synthetic caller", "Synthetic peer", "Synthetic quiet", "Synthetic gone", "Synthetic invited"} {
		if bytes.Contains(raw, []byte(lo)) {
			t.Fatalf("payload gửi model mang %q: id hoặc tên tài khoản", lo)
		}
	}
	if bytes.Contains(raw, []byte("Synthetic departed line")) {
		t.Fatal("máy chủ đọc nội dung tin nhắn từ bảng thay vì dùng đúng chữ client gửi")
	}
}

// With no bundle ("Chỉ gửi lời nhờ") there are no aliases to borrow, and the
// roster still counts the room: the caller, then a fresh alias per member.
func TestRosterKhongCoBoiCanhVanDemDuNguoi(t *testing.T) {
	var payload map[string]any
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic provider fixture"}})
	})
	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Tối nay ăn gì",
	}), 202)
	if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var ten []string
	for _, m := range payload["members"].([]any) {
		ten = append(ten, m.(map[string]any)["display_name"].(string))
	}
	if strings.Join(ten, "|") != "Mình|Bạn 1" {
		t.Fatalf("roster là %q, cần [Mình Bạn 1]", ten)
	}
	if payload["budget_per_person_vnd"] != nil {
		t.Fatalf("không ai trả lời mức chi mà ngân sách là %v", payload["budget_per_person_vnd"])
	}
}
