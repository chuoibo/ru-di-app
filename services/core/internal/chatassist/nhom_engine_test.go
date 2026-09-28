package chatassist

import (
	"encoding/json"
	"testing"

	"mobile/services/core/internal/repo"
)

// Only the room's ACTIVE members reach the engine (the reviewer's mutant G3):
// a member who left, or who was only invited, is neither in the router's
// roster nor among the people a split draft is shared by. The caller comes
// first.
func TestThanhVienNhomChiNguoiDangO(t *testing.T) {
	ms := []repo.Membership{
		{PersonID: "p-lan", State: "active", DisplayName: "Lan"},
		{PersonID: "p-cu", State: "left", DisplayName: "Người cũ"},
		{PersonID: "p-toi", State: "active", DisplayName: "Tú"},
		{PersonID: "p-moi", State: "invited", DisplayName: "Mới mời"},
		{PersonID: "p-toi", State: "left", DisplayName: "Tú cũ"},
	}
	got := thanhVienNhom(ms, "p-toi")
	if len(got) != 2 || got[0].ID != "p-toi" || got[1].ID != "p-lan" {
		t.Fatalf("members for the engine: %+v", got)
	}
	for _, m := range got {
		if m.ID == "p-cu" || m.ID == "p-moi" {
			t.Fatalf("a member not in the room reached the engine: %+v", m)
		}
	}
}

// The bundle's turns reach the engine with the server's author and, for a
// turn whose stored text the server read, that text beside the client's
// copy; an earlier answer of the assistant carries neither.
func TestLuotNhomChuMayChu(t *testing.T) {
	goi, _ := json.Marshal(map[string]any{"ban": 1, "nguon": "chat-nhom", "tongLuot": 3, "daCat": false, "luot": []map[string]any{
		{"id": "a", "vai": "ban", "biDanh": "Lan", "loai": "chu", "luc": "2030-09-22T10:00:00Z", "chu": "Mình trả lẩu 5tr"},
		{"id": "b", "vai": "ai", "loai": "chu", "luc": "2030-09-22T10:00:00Z", "chu": "Gợi ý quán"},
		{"id": "c", "vai": "ban", "biDanh": "Minh", "loai": "chu", "luc": "2030-09-22T10:00:00Z", "chu": "Mình trả taxi 120k"},
	}})
	authors := map[string]string{"a": "p-lan", "b": "p-x", "c": "p-minh"}
	texts := map[string]string{"a": "Mình trả lẩu 850k", "b": "stored", "c": ""}
	ls, err := luotNhom(goi, authors, texts)
	if err != nil || len(ls) != 3 {
		t.Fatalf("%v %+v", err, ls)
	}
	if ls[0].Chu != "Mình trả lẩu 5tr" || ls[0].ChuMayChu != "Mình trả lẩu 850k" || ls[0].TacGia != "p-lan" {
		t.Fatalf("turn a: %+v", ls[0])
	}
	if ls[1].TacGia != "" || ls[1].ChuMayChu != "" {
		t.Fatalf("an earlier answer carries an author or stored text: %+v", ls[1])
	}
	if ls[2].ChuMayChu != "" {
		t.Fatalf("turn c has no stored text: %+v", ls[2])
	}
}
