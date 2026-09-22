//go:build e2e

package chate2e

import (
	"net/http"
	"sync"
	"testing"
)

// TestToHenNhapChung covers D1-D5: the shared sheet a group edits together
// between deciding and committing.
//
// The reviewer who scored the screen 32/40 named this as the gap, with a close
// criterion worth quoting: from a decided poll a person must be able to see
// *which* sheet is receiving the choice, open it, edit it, and watch that same
// sheet become the kèo. These cases are that sentence, made executable.
func TestToHenNhapChung(t *testing.T) {
	stack := Load(t)
	group := stack.GroupID
	author := stack.As(t, 0)
	peer := stack.As(t, 1)
	observer := stack.As(t, 2)

	// A decided poll is the starting point: the sheet exists to receive a
	// choice the group already made.
	vote := author.Expect(201, "POST", "/contexts/"+group+"/votes", map[string]any{
		"question": "E2E D đi đâu? " + newKey()[:6],
		"options":  []any{map[string]any{"label": "Đà Lạt"}, map[string]any{"label": "Vũng Tàu"}},
	}, Idem(newKey()))
	voteID := vote.Str(t, "id")
	options := vote.List(t, "options")
	first, _ := options[0].(map[string]any)
	optionID, _ := first["id"].(string)
	peer.Expect(200, "POST", "/votes/"+voteID+"/ballots",
		map[string]any{"option_id": optionID}, Idem(newKey()))

	body := func(extra map[string]any) map[string]any {
		out := map[string]any{
			"title":                 "E2E tờ hẹn chung",
			"starts_on":             "2026-10-03",
			"ends_on":               "2026-10-04",
			"headcount":             4,
			"budget_per_person_vnd": 500000,
			"stops": []any{
				map[string]any{"time_text": "08:00", "label": "Chợ đêm"},
				map[string]any{"time_text": "11:30", "label": "Quán cà phê"},
			},
		}
		for key, value := range extra {
			out[key] = value
		}
		return out
	}

	t.Run("D1a bình chọn chưa chốt thì chưa có gì để nhận", func(t *testing.T) {
		response := author.Do("POST", "/contexts/"+group+"/shared-drafts",
			body(map[string]any{"from_vote_id": voteID}), Idem(newKey()))
		if response.Status != http.StatusConflict {
			t.Fatalf("bình chọn còn mở phải 409, nhận %d — %s", response.Status, response.trim())
		}
		if code, _ := response.JSON["code"].(string); code != "draft_vote_open" {
			t.Fatalf("từ chối sai lý do: %s", response.trim())
		}
	})

	author.Expect(200, "POST", "/votes/"+voteID+"/close", nil, Idem(newKey()))

	var draftID, anchorID string

	t.Run("D1 từ bình chọn đã chốt mở được tờ hẹn chung", func(t *testing.T) {
		created := author.Expect(201, "POST", "/contexts/"+group+"/shared-drafts",
			body(map[string]any{"from_vote_id": voteID}), Idem(newKey()))
		draftID = created.Str(t, "id")
		anchorID = created.Str(t, "message_id")

		if got := created.Str(t, "source_vote_id"); got != voteID {
			t.Fatalf("tờ hẹn không trỏ về bình chọn đã chốt: %q", got)
		}
		if created.Num(t, "revision") != 1 {
			t.Fatalf("tờ mới phải ở bản 1, nhận %d", created.Num(t, "revision"))
		}
		if created.Str(t, "status") != "open" {
			t.Fatalf("tờ mới phải mở, nhận %q", created.Str(t, "status"))
		}

		// The sheet has to be visible in the thread, not just in a side table:
		// that is how the room learns which sheet took the choice.
		listed := peer.Expect(200, "GET", "/contexts/"+group+"/messages?limit=20", nil)
		for _, raw := range listed.List(t, "messages") {
			message, _ := raw.(map[string]any)
			if message["id"] != anchorID {
				continue
			}
			card, _ := message["card"].(map[string]any)
			if card == nil || card["kind"] != "itinerary" {
				t.Fatalf("neo không mang thẻ tờ hẹn: %v", message["card"])
			}
			payload, _ := card["payload"].(map[string]any)
			draft, _ := payload["draft"].(map[string]any)
			if draft == nil || draft["id"] != draftID {
				t.Fatalf("thẻ không khai nó là bản nháp chung: %v", payload)
			}
			return
		}
		t.Fatalf("người khác không thấy neo %s trong luồng", anchorID[:8])
	})

	t.Run("D5 một bình chọn chỉ nuôi một tờ đang mở", func(t *testing.T) {
		response := author.Do("POST", "/contexts/"+group+"/shared-drafts",
			body(map[string]any{"from_vote_id": voteID}), Idem(newKey()))
		if response.Status != http.StatusConflict {
			t.Fatalf("tờ thứ hai cho cùng bình chọn phải 409, nhận %d — %s",
				response.Status, response.trim())
		}
	})

	t.Run("D2 hai người sửa cùng lúc: một người thắng, người kia biết mình cũ", func(t *testing.T) {
		var wait sync.WaitGroup
		results := make([]Response, 2)
		for slot, user := range []int{0, 1} {
			wait.Add(1)
			go func(slot, user int) {
				defer wait.Done()
				results[slot] = stack.As(t, user).Do("PATCH",
					"/contexts/"+group+"/shared-drafts/"+draftID,
					map[string]any{"revision": 1, "headcount": 6 + slot}, Idem(newKey()))
			}(slot, user)
		}
		wait.Wait()

		ok, stale := 0, 0
		for _, response := range results {
			switch response.Status {
			case http.StatusOK:
				ok++
			case http.StatusConflict:
				stale++
				if code, _ := response.JSON["code"].(string); code != "draft_revision_stale" {
					t.Fatalf("xung đột sai lý do: %s", response.trim())
				}
			default:
				t.Fatalf("sửa đồng thời trả %d — %s", response.Status, response.trim())
			}
		}
		if ok != 1 || stale != 1 {
			t.Fatalf("mong đúng một người ghi được và một người bị báo cũ, nhận %d/%d", ok, stale)
		}

		// The loser re-reads and edits again: the flow has to be recoverable,
		// not just safe.
		current := peer.Expect(200, "GET", "/contexts/"+group+"/shared-drafts/"+draftID, nil)
		retry := peer.Expect(200, "PATCH", "/contexts/"+group+"/shared-drafts/"+draftID,
			map[string]any{"revision": current.Num(t, "revision"), "title": "E2E tờ hẹn chung đã sửa"},
			Idem(newKey()))
		if retry.Str(t, "title") != "E2E tờ hẹn chung đã sửa" {
			t.Fatalf("sửa lại trên bản mới không ăn: %s", retry.trim())
		}
	})

	t.Run("D2b thiếu số bản là từ chối, không phải ghi đè", func(t *testing.T) {
		response := author.Do("PATCH", "/contexts/"+group+"/shared-drafts/"+draftID,
			map[string]any{"title": "Ghi đè không hỏi"}, Idem(newKey()))
		if response.Status != http.StatusBadRequest {
			t.Fatalf("sửa mà không nói đang xem bản nào phải 400, nhận %d — %s",
				response.Status, response.trim())
		}
	})

	t.Run("D3 người thứ ba thấy bản mới qua feed thay đổi", func(t *testing.T) {
		stream := observer.Stream(group, observer.Head(group))
		defer stream.Close()

		current := author.Expect(200, "GET", "/contexts/"+group+"/shared-drafts/"+draftID, nil)
		author.Expect(200, "PATCH", "/contexts/"+group+"/shared-drafts/"+draftID,
			map[string]any{"revision": current.Num(t, "revision"), "headcount": 8}, Idem(newKey()))

		stream.Await(syncWait, func(seen []Change) bool { return Contains(seen, anchorID) })

		snapshot := observer.Expect(200, "POST", "/contexts/"+group+"/changes/snapshot",
			map[string]any{"message_ids": []string{anchorID}, "vote_ids": []string{}})
		message := messageOf(t, snapshot, anchorID)
		card, _ := message["card"].(map[string]any)
		payload, _ := card["payload"].(map[string]any)
		draft, _ := payload["draft"].(map[string]any)
		if draft == nil {
			t.Fatalf("ảnh chụp mất khối bản nháp: %v", payload)
		}
		if headcount, _ := draft["headcount"].(float64); int(headcount) != 8 {
			t.Fatalf("người thứ ba thấy số người %v, phải là 8", draft["headcount"])
		}
	})

	t.Run("D4 chính tờ đó trở thành kèo", func(t *testing.T) {
		promotion := map[string]any{
			"source_message_id":     anchorID,
			"title":                 "E2E kèo từ tờ hẹn chung",
			"starts_on":             "2026-10-03",
			"ends_on":               "2026-10-04",
			"headcount":             8,
			"budget_per_person_vnd": 500000,
		}
		created := author.Expect(201, "POST", "/contexts/"+group+"/plan-promotions",
			promotion, Idem(newKey()))
		outing := created.Str(t, "outing_id")

		// Same sheet, same decision, from the other person: one kèo, not two.
		again := peer.Expect(200, "POST", "/contexts/"+group+"/plan-promotions",
			promotion, Idem(newKey()))
		if again.Str(t, "outing_id") != outing {
			t.Fatalf("chốt lại ra kèo khác: %s và %s", outing[:8], again.Str(t, "outing_id")[:8])
		}

		settled := author.Expect(200, "GET", "/contexts/"+group+"/shared-drafts/"+draftID, nil)
		if settled.Str(t, "status") != "promoted" {
			t.Fatalf("tờ đã thành kèo nhưng vẫn ở trạng thái %q", settled.Str(t, "status"))
		}

		// A promoted sheet is history. Editing it would rewrite a decision the
		// group already acted on.
		stale := author.Do("PATCH", "/contexts/"+group+"/shared-drafts/"+draftID,
			map[string]any{"revision": settled.Num(t, "revision"), "headcount": 9}, Idem(newKey()))
		if stale.Status != http.StatusConflict {
			t.Fatalf("sửa tờ đã thành kèo phải 409, nhận %d — %s", stale.Status, stale.trim())
		}
		discard := author.Do("POST", "/contexts/"+group+"/shared-drafts/"+draftID+"/discard",
			nil, Idem(newKey()))
		if discard.Status != http.StatusConflict {
			t.Fatalf("bỏ tờ đã thành kèo phải 409, nhận %d — %s", discard.Status, discard.trim())
		}
	})

	t.Run("D7 chốt bằng đúng hình dạng client gửi", func(t *testing.T) {
		// D4 chốt bằng hình dạng thuận tay của một bài test: không có `stops`,
		// để máy chủ tự dựng chặng từ thẻ. Màn hình thì gửi chặng của chính tờ
		// hẹn, với `at`/`label`/`place_name`. Hai đường đó khác nhau, và lượt
		// trước đường của màn hình chưa từng được chạy ở tầng nào — nên khi nó
		// hỏng thì mọi cổng vẫn xanh.
		vote := author.Expect(201, "POST", "/contexts/"+group+"/votes", map[string]any{
			"question": "E2E D7 đi đâu? " + newKey()[:6],
			"options":  []any{map[string]any{"label": "Đà Lạt"}, map[string]any{"label": "Vũng Tàu"}},
		}, Idem(newKey()))
		voteID := vote.Str(t, "id")
		options := vote.List(t, "options")
		first, _ := options[0].(map[string]any)
		peer.Expect(200, "POST", "/votes/"+voteID+"/ballots",
			map[string]any{"option_id": first["id"]}, Idem(newKey()))
		author.Expect(200, "POST", "/votes/"+voteID+"/close", nil, Idem(newKey()))

		created := author.Expect(201, "POST", "/contexts/"+group+"/shared-drafts",
			body(map[string]any{"from_vote_id": voteID}), Idem(newKey()))

		stops := []any{}
		for _, raw := range created.List(t, "stops") {
			stop, _ := raw.(map[string]any)
			stops = append(stops, map[string]any{
				"at": stop["time_text"], "label": stop["label"], "place_name": nil,
			})
		}
		promoted := author.Expect(201, "POST", "/contexts/"+group+"/plan-promotions", map[string]any{
			"source_message_id":     created.Str(t, "message_id"),
			"title":                 created.Str(t, "title"),
			"starts_on":             created.JSON["starts_on"],
			"ends_on":               created.JSON["ends_on"],
			"headcount":             created.JSON["headcount"],
			"budget_per_person_vnd": created.JSON["budget_per_person_vnd"],
			"stops":                 stops,
		}, Idem(newKey()))
		if promoted.Str(t, "outing_id") == "" {
			t.Fatalf("chốt bằng hình dạng client không trả kèo: %s", promoted.trim())
		}
		settled := author.Expect(200, "GET",
			"/contexts/"+group+"/shared-drafts/"+created.Str(t, "id"), nil)
		if settled.Str(t, "status") != "promoted" {
			t.Fatalf("tờ hẹn vẫn ở %q sau khi chốt", settled.Str(t, "status"))
		}

		// The card is what a screen reads, and it used to disagree with the
		// row: outing_id set while the embedded draft block still said "open".
		// A screen believing the card then offered to keep editing a sheet that
		// was already a kèo.
		anchor := peer.Expect(200, "POST", "/contexts/"+group+"/changes/snapshot",
			map[string]any{"message_ids": []string{created.Str(t, "message_id")}, "vote_ids": []string{}})
		message := messageOf(t, anchor, created.Str(t, "message_id"))
		card, _ := message["card"].(map[string]any)
		payload, _ := card["payload"].(map[string]any)
		if payload["outing_id"] == nil {
			t.Fatalf("thẻ sau khi chốt không mang outing_id: %v", payload)
		}
		draft, _ := payload["draft"].(map[string]any)
		if draft == nil {
			t.Fatalf("thẻ mất khối bản nháp sau khi chốt: %v", payload)
		}
		if draft["status"] != "promoted" {
			t.Fatalf("thẻ tự mâu thuẫn: outing_id đã có nhưng draft.status = %v", draft["status"])
		}
	})

	t.Run("D6 người ngoài không thấy và không sửa được tờ hẹn", func(t *testing.T) {
		outsider := stack.As(t, len(stack.Users)-1)
		for _, call := range []struct {
			method, path string
			body         map[string]any
		}{
			{"GET", "/contexts/" + group + "/shared-drafts/" + draftID, nil},
			{"PATCH", "/contexts/" + group + "/shared-drafts/" + draftID, map[string]any{"revision": 1, "headcount": 2}},
			{"POST", "/contexts/" + group + "/shared-drafts/" + draftID + "/discard", nil},
		} {
			response := outsider.Do(call.method, call.path, call.body, Idem(newKey()))
			if response.Status != http.StatusForbidden {
				t.Fatalf("%s %s cho người ngoài phải 403, nhận %d — %s",
					call.method, call.path, response.Status, response.trim())
			}
		}
	})
}
