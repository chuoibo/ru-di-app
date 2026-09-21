//go:build e2e

package chate2e

import (
	"sync"
	"testing"
)

// TestMocDaDoc covers R1-R2. The read mark is the one piece of chat state two
// devices of the same person race over, so it is the one place where "it works
// on my machine" and "it works with two tabs open" genuinely differ.
func TestMocDaDoc(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID
	author := stack.As(t, 0)
	reader := stack.As(t, 1)

	t.Run("R1 mốc chỉ tiến, không bao giờ lùi", func(t *testing.T) {
		older := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E R1 cũ"}, Idem(newKey()))
		newer := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E R1 mới"}, Idem(newKey()))

		reader.Expect(200, "PUT", "/contexts/"+group+"/read-mark",
			map[string]any{"message_id": newer.Str(t, "id")}, Idem(newKey()))

		// Marking the older message must not drag the watermark backwards:
		// an out-of-order ACK from a slow device would otherwise resurrect
		// messages the person already read.
		back := reader.Expect(200, "PUT", "/contexts/"+group+"/read-mark",
			map[string]any{"message_id": older.Str(t, "id")}, Idem(newKey()))

		if got := back.Str(t, "last_read_message_id"); got != newer.Str(t, "id") {
			t.Fatalf("mốc đã lùi về %s, phải giữ ở %s", got[:8], newer.Str(t, "id")[:8])
		}
	})

	t.Run("R2 hai thiết bị cùng người ghi đồng thời vẫn ra một mốc", func(t *testing.T) {
		target := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E R2 " + newKey()[:8]}, Idem(newKey()))
		id := target.Str(t, "id")

		// Two clients, same bearer: two devices of one person. Before the
		// atomic UPSERT landed, a concurrent first read could lose one write.
		deviceA := stack.As(t, 1)
		deviceB := stack.As(t, 1)

		var wait sync.WaitGroup
		results := make([]Response, 2)
		for index, device := range []*Client{deviceA, deviceB} {
			wait.Add(1)
			go func(slot int, client *Client) {
				defer wait.Done()
				results[slot] = client.Do("PUT", "/contexts/"+group+"/read-mark",
					map[string]any{"message_id": id}, Idem(newKey()))
			}(index, device)
		}
		wait.Wait()

		for slot, response := range results {
			if response.Status != 200 {
				t.Fatalf("thiết bị %d nhận %d — %s", slot, response.Status, response.trim())
			}
		}
		settled := reader.Expect(200, "PUT", "/contexts/"+group+"/read-mark",
			map[string]any{"message_id": id}, Idem(newKey()))
		if got := settled.Str(t, "last_read_message_id"); got != id {
			t.Fatalf("sau hai lần ghi đồng thời mốc nằm ở %s, phải là %s", got[:8], id[:8])
		}
	})
}
