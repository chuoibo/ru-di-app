//go:build e2e

package chate2e

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
)

func newKey() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}

// TestPhienVaQuyenTruyCap covers A1-A3: who is allowed to speak at all.
func TestPhienVaQuyenTruyCap(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID

	t.Run("A1 phiên hợp lệ đọc được phòng của mình", func(t *testing.T) {
		response := stack.As(t, 0).Do("GET", "/contexts/"+group+"/messages?limit=1", nil)
		if response.Status != http.StatusOK {
			t.Fatalf("thành viên đọc phòng mình phải 200, nhận %d", response.Status)
		}
	})

	t.Run("A2 không có bearer thì 401, không phải 200 rỗng", func(t *testing.T) {
		response := stack.Anonymous(t).Do("GET", "/contexts/"+group+"/messages?limit=1", nil)
		if response.Status != http.StatusUnauthorized {
			t.Fatalf("thiếu bearer phải 401, nhận %d — %s", response.Status, response.trim())
		}
	})

	t.Run("A3 bearer bịa bị từ chối", func(t *testing.T) {
		forged := stack.WithToken(t, "khong-phai-phien-that-"+newKey())
		response := forged.Do("GET", "/contexts/"+group+"/messages?limit=1", nil)
		if response.Status != http.StatusUnauthorized {
			t.Fatalf("token bịa phải 401, nhận %d — %s", response.Status, response.trim())
		}
	})
}

// TestGuiVaDocTin covers M1-M5: the write path and its boundaries.
func TestGuiVaDocTin(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID
	author := stack.As(t, 0)
	reader := stack.As(t, 1)

	t.Run("M1 gửi rồi đọc lại thấy đúng nội dung", func(t *testing.T) {
		body := "E2E M1 " + newKey()[:8]
		sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": body}, Idem(newKey()))
		id := sent.Str(t, "id")

		listed := reader.Expect(200, "GET", "/contexts/"+group+"/messages?limit=20", nil)
		for _, raw := range listed.List(t, "messages") {
			message, _ := raw.(map[string]any)
			if message["id"] == id {
				if message["body"] != body {
					t.Fatalf("nội dung đọc lại khác lúc gửi: %v", message["body"])
				}
				return
			}
		}
		t.Fatalf("người khác không thấy tin %s vừa gửi", id[:8])
	})

	t.Run("M2 cùng Idempotency-Key không sinh tin thứ hai", func(t *testing.T) {
		key := newKey()
		body := "E2E M2 " + key[:8]
		first := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": body}, Idem(key))
		second := author.Do("POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": body}, Idem(key))

		if second.Status != http.StatusCreated && second.Status != http.StatusOK {
			t.Fatalf("lần gửi lại phải là phát lại, nhận %d — %s", second.Status, second.trim())
		}
		if first.Str(t, "id") != second.Str(t, "id") {
			t.Fatalf("cùng khoá mà sinh hai tin: %s và %s",
				first.Str(t, "id")[:8], second.Str(t, "id")[:8])
		}
		if second.Header.Get("Idempotency-Replayed") == "" {
			t.Log("ghi chú: không có header Idempotency-Replayed; định danh trùng vẫn đúng")
		}

		// The count is the thing that matters: a duplicate row would still
		// return the same id if the handler deduplicated after writing.
		listed := reader.Expect(200, "GET", "/contexts/"+group+"/messages?limit=50", nil)
		seen := 0
		for _, raw := range listed.List(t, "messages") {
			if message, _ := raw.(map[string]any); message["body"] == body {
				seen++
			}
		}
		if seen != 1 {
			t.Fatalf("thân tin %q xuất hiện %d lần, phải đúng 1", body, seen)
		}
	})

	t.Run("M3 phân trang không hụt và không trùng", func(t *testing.T) {
		marker := newKey()[:8]
		ids := map[string]bool{}
		for i := 0; i < 5; i++ {
			sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
				map[string]any{"kind": "text", "body": fmt.Sprintf("E2E M3 %s #%d", marker, i)},
				Idem(newKey()))
			ids[sent.Str(t, "id")] = true
		}

		found := map[string]int{}
		cursor := ""
		for page := 0; page < 20; page++ {
			// The list route pages backwards through history with `before`.
			// `cursor` is not a parameter it knows, and an unknown parameter is
			// ignored rather than refused -- which returns page one forever.
			path := "/contexts/" + group + "/messages?limit=3"
			if cursor != "" {
				path += "&before=" + cursor
			}
			response := reader.Expect(200, "GET", path, nil)
			for _, raw := range response.List(t, "messages") {
				message, _ := raw.(map[string]any)
				id, _ := message["id"].(string)
				if ids[id] {
					found[id]++
				}
			}
			next, _ := response.JSON["next_cursor"].(string)
			more, _ := response.JSON["has_more"].(bool)
			if next == "" || !more || len(found) == len(ids) {
				break
			}
			cursor = next
		}
		if len(found) != len(ids) {
			t.Fatalf("phân trang hụt: gửi %d, thấy %d", len(ids), len(found))
		}
		for id, times := range found {
			if times != 1 {
				t.Fatalf("tin %s xuất hiện %d lần khi lật trang", id[:8], times)
			}
		}
	})

	t.Run("M4 xoá để lại bia mộ, không phải khoảng trống", func(t *testing.T) {
		sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E M4 sẽ bị xoá " + newKey()[:8]}, Idem(newKey()))
		id := sent.Str(t, "id")
		author.Expect(204, "DELETE", "/contexts/"+group+"/messages/"+id, nil, Idem(newKey()))

		listed := reader.Expect(200, "GET", "/contexts/"+group+"/messages?limit=20", nil)
		for _, raw := range listed.List(t, "messages") {
			message, _ := raw.(map[string]any)
			if message["id"] != id {
				continue
			}
			if message["kind"] != "deleted" {
				t.Fatalf("tin đã xoá phải mang kind=deleted, nhận %v", message["kind"])
			}
			if body, _ := message["body"].(string); body != "" {
				t.Fatalf("tin đã xoá vẫn trả nội dung cũ: %q", body)
			}
			return
		}
		t.Fatal("tin đã xoá biến mất hẳn khỏi trang: người đọc mất mốc hội thoại")
	})

	t.Run("M5 người ngoài nhóm bị 403, không phải 404", func(t *testing.T) {
		outsider := stack.As(t, len(stack.Users)-1)
		response := outsider.Do("GET", "/contexts/"+group+"/messages?limit=1", nil)
		if response.Status != http.StatusForbidden {
			t.Fatalf("người ngoài phải 403, nhận %d — %s", response.Status, response.trim())
		}
		write := outsider.Do("POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "không được phép"}, Idem(newKey()))
		if write.Status != http.StatusForbidden {
			t.Fatalf("người ngoài ghi phải 403, nhận %d — %s", write.Status, write.trim())
		}
	})
}
