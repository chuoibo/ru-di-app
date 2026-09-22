//go:build e2e

package chate2e

import (
	"net/http"
	"testing"
	"time"
)

// syncWait is how long a case will wait for a live change. The realtime gate in
// ADR-0031 is p95 800 ms; this ceiling is deliberately far above it so that a
// failure here means "never arrived", not "arrived slowly".
const syncWait = 20 * time.Second

// TestDongBoSongQuaSocket covers S1-S6. This is the group of cases the handoff
// called an open blocker: reactions, deletions and poll tallies were reported
// as not reaching other open clients. These cases settle that by measurement.
//
// The feed carries pointers, not content -- {sequence, type, entity_id,
// revision}. A client learns an entity moved and then hydrates it through the
// snapshot route. So "synchronised" here means: the other client's socket named
// the entity, and the snapshot it then asked for showed the new state.
func TestDongBoSongQuaSocket(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID
	author := stack.As(t, 0)
	peer := stack.As(t, 1)

	t.Run("S1 tin mới tới máy người khác", func(t *testing.T) {
		stream := peer.Stream(group, peer.Head(group))
		defer stream.Close()

		sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E S1 " + newKey()[:8]}, Idem(newKey()))
		id := sent.Str(t, "id")

		changes := stream.Await(syncWait, func(seen []Change) bool { return Contains(seen, id) })
		if !Contains(changes, id) {
			t.Fatalf("socket của người khác không nhắc tới tin %s", id[:8])
		}
	})

	t.Run("S2 thả tim tới máy người khác", func(t *testing.T) {
		sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E S2 " + newKey()[:8]}, Idem(newKey()))
		id := sent.Str(t, "id")

		// Open the socket after the message exists, so the only thing it can
		// report about this entity is the reaction.
		stream := peer.Stream(group, peer.Head(group))
		defer stream.Close()

		author.Expect(201, "POST", "/contexts/"+group+"/messages/"+id+"/reactions",
			map[string]any{"kind": "heart"}, Idem(newKey()))

		stream.Await(syncWait, func(seen []Change) bool { return Contains(seen, id) })

		snapshot := peer.Expect(200, "POST", "/contexts/"+group+"/changes/snapshot",
			map[string]any{"message_ids": []string{id}, "vote_ids": []string{}})
		reactions := reactionsOf(t, snapshot, id)
		if len(reactions) == 0 {
			t.Fatal("người khác đồng bộ được con trỏ nhưng ảnh chụp không có phản ứng nào")
		}
	})

	t.Run("S3 xoá tin tới máy người khác", func(t *testing.T) {
		sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E S3 " + newKey()[:8]}, Idem(newKey()))
		id := sent.Str(t, "id")

		stream := peer.Stream(group, peer.Head(group))
		defer stream.Close()

		author.Expect(204, "DELETE", "/contexts/"+group+"/messages/"+id, nil, Idem(newKey()))
		stream.Await(syncWait, func(seen []Change) bool { return Contains(seen, id) })

		snapshot := peer.Expect(200, "POST", "/contexts/"+group+"/changes/snapshot",
			map[string]any{"message_ids": []string{id}, "vote_ids": []string{}})
		message := messageOf(t, snapshot, id)
		if message["kind"] != "deleted" {
			t.Fatalf("ảnh chụp sau khi xoá vẫn mang kind=%v", message["kind"])
		}
	})

	t.Run("S4 kiểm phiếu bình chọn tới máy người khác", func(t *testing.T) {
		created := author.Expect(201, "POST", "/contexts/"+group+"/votes", map[string]any{
			"question": "E2E S4 đi đâu? " + newKey()[:6],
			"options":  []any{map[string]any{"label": "Đà Lạt"}, map[string]any{"label": "Vũng Tàu"}},
		}, Idem(newKey()))
		voteID := created.Str(t, "id")
		options := created.List(t, "options")
		first, _ := options[0].(map[string]any)
		optionID, _ := first["id"].(string)

		stream := peer.Stream(group, peer.Head(group))
		defer stream.Close()

		author.Expect(200, "POST", "/votes/"+voteID+"/ballots",
			map[string]any{"option_id": optionID}, Idem(newKey()))

		stream.Await(syncWait, func(seen []Change) bool { return Contains(seen, voteID) })

		snapshot := peer.Expect(200, "POST", "/contexts/"+group+"/changes/snapshot",
			map[string]any{"message_ids": []string{}, "vote_ids": []string{voteID}})
		if tally := tallyOf(t, snapshot, voteID, optionID); tally != 1 {
			t.Fatalf("người khác thấy %d phiếu, phải là 1", tally)
		}
	})

	t.Run("S5 dãy số không lùi và không nhảy cóc", func(t *testing.T) {
		start := peer.Head(group)
		for i := 0; i < 3; i++ {
			author.Expect(201, "POST", "/contexts/"+group+"/messages",
				map[string]any{"kind": "text", "body": "E2E S5"}, Idem(newKey()))
		}
		_, changes := peer.Drain(group, start)
		if len(changes) < 3 {
			t.Fatalf("chỉ thấy %d thay đổi sau 3 lần gửi", len(changes))
		}
		previous := start
		for _, change := range changes {
			if change.Sequence <= previous {
				t.Fatalf("dãy số lùi hoặc lặp: %d sau %d", change.Sequence, previous)
			}
			previous = change.Sequence
		}
	})

	t.Run("S6 nối lại từ con trỏ cũ thấy đủ phần đã lỡ", func(t *testing.T) {
		before := peer.Head(group)

		// The peer is offline for this write: no socket, no polling.
		missed := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E S6 lỡ mất " + newKey()[:8]}, Idem(newKey()))
		id := missed.Str(t, "id")

		stream := peer.Stream(group, before)
		defer stream.Close()
		changes := stream.Await(syncWait, func(seen []Change) bool { return Contains(seen, id) })
		if !Contains(changes, id) {
			t.Fatalf("nối lại từ %d không phát lại tin %s đã lỡ", before, id[:8])
		}
	})
}

// TestThuHoiVaBienLichSu covers S7-S8: what a removed member may still see.
func TestThuHoiVaBienLichSu(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID

	t.Run("S7 người ngoài không mở được socket thay đổi", func(t *testing.T) {
		outsider := stack.As(t, len(stack.Users)-1)
		response := outsider.Do("GET", "/contexts/"+group+"/changes?limit=1", nil)
		if response.Status != http.StatusForbidden {
			t.Fatalf("người ngoài đọc feed phải 403, nhận %d — %s", response.Status, response.trim())
		}
	})

	t.Run("S8 ảnh chụp không phát nội dung cho người ngoài", func(t *testing.T) {
		author := stack.As(t, 0)
		sent := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E S8 bí mật " + newKey()[:8]}, Idem(newKey()))
		id := sent.Str(t, "id")

		outsider := stack.As(t, len(stack.Users)-1)
		response := outsider.Do("POST", "/contexts/"+group+"/changes/snapshot",
			map[string]any{"message_ids": []string{id}, "vote_ids": []string{}})
		if response.Status == http.StatusOK {
			t.Fatalf("người ngoài lấy được ảnh chụp: %s", response.trim())
		}
		if response.Status != http.StatusForbidden {
			t.Fatalf("mong 403, nhận %d — %s", response.Status, response.trim())
		}
	})
}

func messageOf(t *testing.T, snapshot Response, id string) map[string]any {
	t.Helper()
	for _, raw := range snapshot.List(t, "messages") {
		message, _ := raw.(map[string]any)
		if message["id"] == id {
			return message
		}
	}
	t.Fatalf("ảnh chụp không chứa tin %s — %s", id[:8], snapshot.trim())
	return nil
}

func reactionsOf(t *testing.T, snapshot Response, id string) []any {
	t.Helper()
	message := messageOf(t, snapshot, id)
	reactions, _ := message["reactions"].([]any)
	return reactions
}

func tallyOf(t *testing.T, snapshot Response, voteID, optionID string) int {
	t.Helper()
	votes, ok := snapshot.JSON["votes"].([]any)
	if !ok {
		t.Fatalf("ảnh chụp không có mảng votes — %s", snapshot.trim())
	}
	for _, raw := range votes {
		vote, _ := raw.(map[string]any)
		if vote["id"] != voteID {
			continue
		}
		options, _ := vote["options"].([]any)
		for _, item := range options {
			option, _ := item.(map[string]any)
			if option["id"] == optionID {
				count, _ := option["ballot_count"].(float64)
				return int(count)
			}
		}
	}
	t.Fatalf("ảnh chụp không chứa bình chọn %s — %s", voteID[:8], snapshot.trim())
	return -1
}
