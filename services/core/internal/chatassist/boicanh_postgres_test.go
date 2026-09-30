package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The bundle the caller hands over, measured against a real database.
//
// The cases the existing file already covers all send NO bundle, so none of
// them says anything about this path. Adding behaviour and leaning on a green
// suite that never exercises it is the exact shape of a blind gate.

func luotThu(id, chu string) map[string]any {
	return map[string]any{"id": id, "vai": "ban", "biDanh": "Bạn 1", "loai": "chu", "luc": "2030-09-22T10:00:00Z", "chu": chu}
}

func goiThu(luot ...map[string]any) map[string]any {
	if luot == nil {
		luot = []map[string]any{}
	}
	return map[string]any{"ban": 1, "nguon": "chat-nhom", "luot": luot, "tongLuot": len(luot), "daCat": false}
}

func (f fixture) tinTrongPhong(t *testing.T, room, chu string) string {
	t.Helper()
	id := newID()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text',$4)`, id, room, f.peer, chu); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f fixture) demLoiGoi(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_ai_invocations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The published card says «đã đọc N tin», and everyone in the room reads that
// line. This is what makes N a statement about real messages of THIS room
// rather than a number the caller asserted about itself.
func TestBoiCanhChiNhanTinCuaPhongNay(t *testing.T) {
	m := &mayGia{}
	f := setup(t, m)
	ctx := context.Background()
	phongKhac := newID()
	if _, err := f.pool.Exec(ctx, `INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES($1,'Synthetic other room','group',$2)`, phongKhac, f.person); err != nil {
		t.Fatal(err)
	}
	lac := f.tinTrongPhong(t, phongKhac, "Synthetic message of another room")
	truoc := f.demLoiGoi(t)

	w := f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
		"boi_canh": goiThu(luotThu(lac, "Câu của phòng khác")),
	})
	requireCode(t, w, 422)
	if !bytes.Contains(w.Body.Bytes(), []byte("boi_canh_mismatch")) {
		t.Fatalf("mã từ chối sai: %s", w.Body.String())
	}
	if f.demLoiGoi(t) != truoc {
		t.Fatal("một lời gọi bị ghi lại dù bối cảnh đã bị từ chối")
	}
	if m.SoGoi() != 0 {
		t.Fatal("bối cảnh chưa qua kiểm mà đã tới provider")
	}

	// An id that never existed lands on the same refusal, so the answer never
	// tells anybody whether a given message id is real.
	w = f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
		"boi_canh": goiThu(luotThu(newID(), "Tin không tồn tại")),
	})
	requireCode(t, w, 422)
}

// Refuse, never truncate. Silently dropping turns server-side would make the
// sentence the person just read above the send button false at the one moment
// it mattered.
func TestBoiCanhVuotHanBiTuChoiChuKhongBiCat(t *testing.T) {
	f := setup(t, nil)
	truoc := f.demLoiGoi(t)
	qua := make([]map[string]any, 0, maxLuot+1)
	for i := 0; i <= maxLuot; i++ {
		qua = append(qua, luotThu(newID(), fmt.Sprintf("Câu %d", i)))
	}
	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Quá nhiều lượt", "boi_canh": goiThu(qua...),
	}), 413)

	dai := make([]rune, maxChuMoiLuot+1)
	for i := range dai {
		dai[i] = 'ừ'
	}
	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Một lượt quá dài",
		"boi_canh": goiThu(luotThu(newID(), string(dai))),
	}), 413)

	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Bản sai",
		"boi_canh": map[string]any{"ban": 2, "nguon": "chat-nhom", "luot": []any{}, "tongLuot": 0, "daCat": false},
	}), 400)

	if f.demLoiGoi(t) != truoc {
		t.Fatal("một lời gọi bị ghi lại dù bối cảnh bị từ chối")
	}
}

// Four terminal paths, one property: the handed-over context never outlives the
// prompt. The CHECK makes a missed site an error rather than a silent leak, and
// this proves each site is actually reached.
func TestBoiCanhBiQuetOMoiDuongKetThuc(t *testing.T) {
	for _, ca := range []struct {
		ten  string
		chay func(t *testing.T, f fixture, id string)
	}{
		{"xong", func(t *testing.T, f fixture, id string) {
			if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
				t.Fatalf("worker: %v %v", ok, err)
			}
		}},
		{"huỷ", func(t *testing.T, f fixture, id string) {
			requireCode(t, f.request("POST", f.route()+"/"+id+"/cancel", f.token, nil), 200)
		}},
		{"hết hạn chia sẻ", func(t *testing.T, f fixture, id string) {
			if _, err := f.pool.Exec(context.Background(), `UPDATE chat_ai_invocations SET share_expires_at=clock_timestamp()-interval '1 second'`); err != nil {
				t.Fatal(err)
			}
			if _, err := f.handler.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
		}},
		{"rời nhóm", func(t *testing.T, f fixture, id string) {
			if _, err := f.pool.Exec(context.Background(), `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE id=$1`, f.member); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			f := setup(t, nil)
			tin := f.tinTrongPhong(t, f.context, "Tao dị ứng hải sản")
			w := f.request("POST", f.route(), f.token, map[string]any{
				"logical_id": newID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
				"boi_canh": goiThu(luotThu(tin, "Tao dị ứng hải sản")),
			})
			requireCode(t, w, 202)
			var v Invocation
			_ = json.Unmarshal(w.Body.Bytes(), &v)
			var con int
			if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_ai_invocations WHERE boi_canh IS NOT NULL`).Scan(&con); err != nil {
				t.Fatal(err)
			}
			if con != 1 {
				t.Fatal("bối cảnh không được lưu lại để worker dùng")
			}
			ca.chay(t, f, v.ID)
			if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_ai_invocations WHERE boi_canh IS NOT NULL`).Scan(&con); err != nil {
				t.Fatal(err)
			}
			if con != 0 {
				t.Fatalf("bối cảnh còn lại sau «%s»", ca.ten)
			}
		})
	}
}

// The database refuses to let a future edit scrub the prompt and forget the
// context. Without this the property above is a promise; with it, it is a rule.
func TestRangBuocChanQuenQuetBoiCanh(t *testing.T) {
	f := setup(t, nil)
	tin := f.tinTrongPhong(t, f.context, "Dưới 300k thôi")
	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
		"boi_canh": goiThu(luotThu(tin, "Dưới 300k thôi")),
	}), 202)
	_, err := f.pool.Exec(context.Background(), `UPDATE chat_ai_invocations SET prompt=NULL`)
	if err == nil {
		t.Fatal("quét prompt mà bỏ quên bối cảnh lẽ ra phải hỏng ngay tại statement")
	}
}

// Idempotency has to cover the bundle, or the same question asked again over
// newer messages replays the old answer with a 200 and the person believes the
// AI just read what they just said.
func TestGoiLaiVoiChatMoiLaXungDotChuKhongPhaiPhatLai(t *testing.T) {
	f := setup(t, nil)
	mot := f.tinTrongPhong(t, f.context, "Quận 1 cho tiện")
	hai := f.tinTrongPhong(t, f.context, "Đừng lẩu nữa")
	logical := newID()
	than := map[string]any{"logical_id": logical, "command": "plan", "prompt": "Lên kế hoạch giúp", "boi_canh": goiThu(luotThu(mot, "Quận 1 cho tiện"))}
	requireCode(t, f.request("POST", f.route(), f.token, than), 202)
	// Same bundle, same words: a replay, not a second job.
	requireCode(t, f.request("POST", f.route(), f.token, than), 200)
	// Same words, newer conversation: a different question, and saying 200 here
	// would be answering it with the previous card.
	than["boi_canh"] = goiThu(luotThu(mot, "Quận 1 cho tiện"), luotThu(hai, "Đừng lẩu nữa"))
	requireCode(t, f.request("POST", f.route(), f.token, than), 409)
}

// What the model is handed: the shared turns in reading order, the caller's own
// words after them, and no account id anywhere.
func TestModelNhanDungDoanChatDuocChiaSeTheoThuTuDoc(t *testing.T) {
	m := &mayGia{}
	f := setup(t, m)
	cu := f.tinTrongPhong(t, f.context, "Tao dị ứng hải sản")
	moi := f.tinTrongPhong(t, f.context, "Dưới 300k thôi")
	requireCode(t, f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
		"boi_canh": goiThu(luotThu(cu, "Tao dị ứng hải sản"), luotThu(moi, "Dưới 300k thôi")),
	}), 202)
	if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	// The router's request: the first the model hears. Shared words arrive
	// datamarked (a space is ˆ), so they read as data, never as a sentence
	// addressed to the model.
	f.may.mu.Lock()
	router := string(f.may.yeu[0])
	f.may.mu.Unlock()
	a, b, c := strings.Index(router, "Taoˆdịˆứngˆhảiˆsản"), strings.Index(router, "Dướiˆ300kˆthôi"), strings.Index(router, "hoạch")
	if a < 0 || b < 0 || c < 0 || !(a < b) {
		t.Fatalf("thứ tự đọc sai: %d %d %d", a, b, c)
	}
	if heard := m.TatCa(); strings.Contains(heard, f.peer) || strings.Contains(heard, f.person) {
		t.Fatal("id tài khoản lọt vào lời gửi model")
	}
}
