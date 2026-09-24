//go:build e2e

package chate2e

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// jobWait bounds how long a case waits for an inference job to reach a terminal
// state. A real provider is slow; the point of the wait is to distinguish
// "durable and eventually finished" from "lost".
const jobWait = 90 * time.Second

// TestGoiAiCoXacNhan covers I1-I3. The rule under test is the one in ADR-0031:
// AI receives only what a person explicitly handed it. It never reads the room.
func TestGoiAiCoXacNhan(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID
	author := stack.As(t, 0)

	t.Run("I1 lối vào ngầm cũ bị niêm phong", func(t *testing.T) {
		// Both of these used to let the model read history without anyone
		// choosing to share it. The per-message expense draft is sealed by
		// name; the automatic turn is deleted everywhere (ADR-0036 §2.1), so
		// it is not served at all.
		draft := "/contexts/" + group + "/messages/" + newUUID() + "/expense-draft"
		response := author.Do("POST", draft, map[string]any{"prompt": "x"}, Idem(newKey()))
		if response.Status != http.StatusForbidden {
			t.Fatalf("%s phải 403, nhận %d — %s", draft, response.Status, response.trim())
		}
		if code, _ := response.JSON["code"].(string); code != "explicit_invocation_required" {
			t.Fatalf("%s bị chặn nhưng sai mã: %s", draft, response.trim())
		}
		turn := "/contexts/" + group + "/ai-turn"
		response = author.Do("POST", turn, map[string]any{"prompt": "x"}, Idem(newKey()))
		if response.Status != http.StatusNotFound {
			t.Fatalf("%s phải không còn (404), nhận %d — %s", turn, response.Status, response.trim())
		}
	})

	t.Run("I2 năng lực khai báo phạm vi chia sẻ", func(t *testing.T) {
		response := author.Expect(200, "GET", "/contexts/"+group+"/chat-capabilities", nil)
		ai, ok := response.JSON["ai"].(map[string]any)
		if !ok {
			t.Fatalf("thiếu khối ai trong năng lực — %s", response.trim())
		}
		// `caller_attached` is a gate the client reads, not a label: while the
		// server says `invocation_only` the client attaches no context at all.
		// Flipping it is what turns the path on, so the value is worth pinning.
		if scope, _ := ai["share_scope"].(string); scope != "caller_attached" {
			t.Fatalf("phạm vi chia sẻ phải là caller_attached, nhận %q", scope)
		}
	})

	t.Run("I5 gói bối cảnh do người gọi trao", func(t *testing.T) {
		// The only case in this tier that sends what the screen actually sends.
		// Without it the whole context path is exercised by nothing here.
		luot := func(id, chu string) map[string]any {
			return map[string]any{"id": id, "vai": "ban", "biDanh": "Bạn 1", "loai": "chu", "luc": "2030-09-22T10:00:00Z", "chu": chu}
		}
		goi := func(items ...map[string]any) map[string]any {
			return map[string]any{"ban": 1, "nguon": "chat-nhom", "luot": items, "tongLuot": len(items), "daCat": false}
		}
		mot := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "Tao dị ứng hải sản"}, Idem(newKey())).Str(t, "id")
		hai := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "Dưới 300k thôi"}, Idem(newKey())).Str(t, "id")

		// A turn whose id is not a message of this room refuses, and refuses
		// before anything reaches the model.
		lac := author.Do("POST", "/contexts/"+group+"/ai-invocations", map[string]any{
			"logical_id": newUUID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
			"boi_canh": goi(luot(newUUID(), "Câu của phòng khác")),
		}, Idem(newKey()))
		if lac.Status != 422 {
			t.Fatalf("id tin lạ phòng phải 422, nhận %d — %s", lac.Status, lac.trim())
		}

		created := author.Do("POST", "/contexts/"+group+"/ai-invocations", map[string]any{
			"logical_id": newUUID(), "command": "plan", "prompt": "Lên kế hoạch giúp",
			"boi_canh": goi(luot(mot, "Tao dị ứng hải sản"), luot(hai, "Dưới 300k thôi")),
		}, Idem(newKey()))
		if created.Status != http.StatusAccepted && created.Status != http.StatusCreated {
			t.Fatalf("lời gọi mang bối cảnh phải 202/201, nhận %d — %s", created.Status, created.trim())
		}
		id := created.Str(t, "id")

		deadline := time.Now().Add(jobWait)
		state := ""
		for time.Now().Before(deadline) {
			response := author.Expect(200, "GET", "/contexts/"+group+"/ai-invocations/"+id, nil)
			state, _ = response.JSON["status"].(string)
			if state == "succeeded" || state == "failed" || state == "cancelled" {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if state != "succeeded" {
			t.Fatalf("job mang bối cảnh dừng ở %q, mong succeeded", state)
		}
		// The public shape of a job never carries the prompt or the context back.
		body := author.Expect(200, "GET", "/contexts/"+group+"/ai-invocations/"+id, nil).trim()
		for _, cam := range []string{"dị ứng", "300k", "boi_canh", "prompt"} {
			if strings.Contains(body, cam) {
				t.Fatalf("phản hồi job để lộ %q — %s", cam, body)
			}
		}
	})

	t.Run("I6 chia_bill đi cùng hàng đợi, ra thẻ chữ", func(t *testing.T) {
		// ADR-0036 §2.9: the same queue as plan, the existing chat-expense
		// skill, a text card a person reads, and nothing written to money.
		luot := map[string]any{"vai": "ban", "biDanh": "Bạn 1", "loai": "chu", "luc": "2030-09-22T10:00:00Z", "chu": "Tao trả 300k tiền nước"}
		luot["id"] = author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "Tao trả 300k tiền nước"}, Idem(newKey())).Str(t, "id")
		created := author.Do("POST", "/contexts/"+group+"/ai-invocations", map[string]any{
			"logical_id": newUUID(), "command": "chia_bill", "prompt": "/chia-bill",
			"boi_canh": map[string]any{"ban": 1, "nguon": "chat-nhom", "luot": []any{luot}, "tongLuot": 1, "daCat": false},
		}, Idem(newKey()))
		if created.Status != http.StatusAccepted && created.Status != http.StatusCreated {
			t.Fatalf("lời gọi chia_bill phải 202/201, nhận %d — %s", created.Status, created.trim())
		}
		id := created.Str(t, "id")
		deadline := time.Now().Add(jobWait)
		var final Response
		for time.Now().Before(deadline) {
			final = author.Expect(200, "GET", "/contexts/"+group+"/ai-invocations/"+id, nil)
			if state, _ := final.JSON["status"].(string); state == "succeeded" || state == "failed" || state == "cancelled" {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if state, _ := final.JSON["status"].(string); state != "succeeded" {
			t.Fatalf("job chia_bill dừng ở %q — %s", state, final.trim())
		}
		if command, _ := final.JSON["command"].(string); command != "chia_bill" {
			t.Fatalf("job trả lệnh %q — %s", command, final.trim())
		}
		// The public job shape still carries neither the context nor the drafts.
		for _, cam := range []string{"300000", "drafts", "boi_canh"} {
			if strings.Contains(final.trim(), cam) {
				t.Fatalf("phản hồi job để lộ %q — %s", cam, final.trim())
			}
		}
	})

	t.Run("I3 job bền và chốt ở trạng thái cuối", func(t *testing.T) {
		logical := newUUID()
		created := author.Do("POST", "/contexts/"+group+"/ai-invocations", map[string]any{
			"logical_id": logical,
			"command":    "plan",
			"prompt":     "Rủ hội đi Đà Lạt hai ngày cuối tuần, ngân sách vừa phải.",
		}, Idem(newKey()))
		if created.Status != http.StatusAccepted && created.Status != http.StatusCreated {
			t.Fatalf("gửi lời nhờ phải 202/201, nhận %d — %s", created.Status, created.trim())
		}
		id := created.Str(t, "id")

		// Same logical id is the same request, not a second job: a user who
		// taps twice must not pay for two inferences.
		again := author.Do("POST", "/contexts/"+group+"/ai-invocations", map[string]any{
			"logical_id": logical,
			"command":    "plan",
			"prompt":     "Rủ hội đi Đà Lạt hai ngày cuối tuần, ngân sách vừa phải.",
		}, Idem(newKey()))
		if again.Status >= 400 {
			t.Fatalf("gửi lại cùng logical_id bị từ chối %d — %s", again.Status, again.trim())
		}
		if again.Str(t, "id") != id {
			t.Fatalf("cùng logical_id sinh hai job: %s và %s", id[:8], again.Str(t, "id")[:8])
		}

		deadline := time.Now().Add(jobWait)
		state := ""
		for time.Now().Before(deadline) {
			response := author.Expect(200, "GET", "/contexts/"+group+"/ai-invocations/"+id, nil)
			state, _ = response.JSON["status"].(string)
			if state == "succeeded" || state == "failed" || state == "cancelled" {
				break
			}
			time.Sleep(2 * time.Second)
		}
		// The runner always puts a deterministic brain in front of the seam, so
		// "settled" is not enough here: the job must actually succeed. A job
		// stuck in queued/running is the lease machinery losing work; a failed
		// job with a stub answering means the pipeline broke, not the model.
		if state != "succeeded" {
			t.Fatalf("job %s dừng ở %q, mong succeeded (brain stub trả lời tất định)", id[:8], state)
		}
	})
}

// TestChotKeoTranhNhau covers I4. Two people confirming the same plan card at
// the same instant must produce one outing, not two, and the loser must be told
// it is the same outing rather than handed a second one.
func TestChotKeoTranhNhau(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID
	author := stack.As(t, 0)

	t.Run("I4a tin thường không phải thẻ kế hoạch nên bị từ chối", func(t *testing.T) {
		plain := author.Expect(201, "POST", "/contexts/"+group+"/messages",
			map[string]any{"kind": "text", "body": "E2E I4a chỉ là tin thường"}, Idem(newKey()))
		response := author.Do("POST", "/contexts/"+group+"/plan-promotions", map[string]any{
			"source_message_id":     plain.Str(t, "id"),
			"title":                 "E2E I4a",
			"starts_on":             "2026-10-03",
			"ends_on":               "2026-10-04",
			"headcount":             4,
			"budget_per_person_vnd": 500000,
			"stops":                 []any{map[string]any{"at": "08:00", "label": "Chợ đêm"}},
		}, Idem(newKey()))
		if response.Status != http.StatusNotFound {
			t.Fatalf("chốt từ tin thường phải 404, nhận %d — %s", response.Status, response.trim())
		}
		if code, _ := response.JSON["code"].(string); code != "plan_source_not_found" {
			t.Fatalf("từ chối sai lý do: %s", response.trim())
		}
	})

	t.Run("I4b hai người chốt cùng lúc ra đúng một kèo", func(t *testing.T) {
		card := planCard(t, author, group)

		payload := map[string]any{
			"source_message_id":     card,
			"title":                 "E2E I4b Đà Lạt",
			"starts_on":             "2026-10-03",
			"ends_on":               "2026-10-04",
			"headcount":             4,
			"budget_per_person_vnd": 500000,
		}

		var wait sync.WaitGroup
		results := make([]Response, 2)
		for slot, user := range []int{0, 1} {
			wait.Add(1)
			go func(slot, user int) {
				defer wait.Done()
				results[slot] = stack.As(t, user).Do("POST",
					"/contexts/"+group+"/plan-promotions", payload, Idem(newKey()))
			}(slot, user)
		}
		wait.Wait()

		for slot, response := range results {
			if response.Status >= 400 {
				t.Fatalf("người %d nhận %d khi chốt — %s", slot, response.Status, response.trim())
			}
		}
		if first, second := results[0].Str(t, "outing_id"), results[1].Str(t, "outing_id"); first != second {
			t.Fatalf("hai người chốt ra hai kèo: %s và %s", first[:8], second[:8])
		}
		created, replayed := 0, 0
		for _, response := range results {
			switch response.Status {
			case http.StatusCreated:
				created++
			case http.StatusOK:
				replayed++
			}
		}
		if created != 1 || replayed != 1 {
			t.Fatalf("mong đúng một 201 và một 200, nhận %d và %d",
				results[0].Status, results[1].Status)
		}

		// A different plan against the same card is a different decision and
		// must not quietly overwrite the outing the group already has.
		conflicting := map[string]any{}
		for key, value := range payload {
			conflicting[key] = value
		}
		conflicting["title"] = "E2E I4b tiêu đề khác"
		clash := stack.As(t, 1).Do("POST", "/contexts/"+group+"/plan-promotions",
			conflicting, Idem(newKey()))
		if clash.Status != http.StatusConflict {
			t.Fatalf("chốt lại với input khác phải 409, nhận %d — %s", clash.Status, clash.trim())
		}
	})
}

// planCard drives a real invocation through the worker and returns the id of
// the message carrying the resulting itinerary card. With the deterministic
// brain stub in front of the seam this is reproducible; without a provider of
// any kind it fails, which is the honest outcome.
func planCard(t *testing.T, author *Client, group string) string {
	t.Helper()
	created := author.Do("POST", "/contexts/"+group+"/ai-invocations", map[string]any{
		"logical_id": newUUID(),
		"command":    "plan",
		"prompt":     "Rủ hội đi chơi một buổi, gợi ý vài chặng.",
	}, Idem(newKey()))
	if created.Status != http.StatusAccepted && created.Status != http.StatusCreated {
		t.Fatalf("không gửi được lời nhờ: %d — %s", created.Status, created.trim())
	}
	id := created.Str(t, "id")

	deadline := time.Now().Add(jobWait)
	for time.Now().Before(deadline) {
		response := author.Expect(200, "GET", "/contexts/"+group+"/ai-invocations/"+id, nil)
		switch status, _ := response.JSON["status"].(string); status {
		case "succeeded":
			if message, _ := response.JSON["message_id"].(string); message != "" {
				return message
			}
			t.Fatalf("job xong nhưng không trỏ tới tin nào — %s", response.trim())
		case "failed", "cancelled":
			t.Fatalf("job hỏng (%s): không có thẻ kế hoạch để chốt — %s",
				response.JSON["code"], response.trim())
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("job %s không chốt sau %s", id[:8], jobWait)
	return ""
}

func newUUID() string {
	raw := newKey()
	return raw[0:8] + "-" + raw[8:12] + "-4" + raw[13:16] + "-8" + raw[17:20] + "-" + raw[20:32]
}
