package routes

import (
	"testing"

	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/outingsteps"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

func TestOutingRoutesBind(t *testing.T) {
	ir, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := Handlers(ir, pyval.NewRegistry(), devEnv())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"POST /outings/{outing_id}/itinerary/preview",
		"PUT /outings/{outing_id}/itinerary",
		"POST /contexts/{context_id}/outings",
		"GET /contexts/{context_id}/outings",
		"PUT /outings/{outing_id}/timeline",
		"POST /outing-stops/{stop_id}/checkins",
		"GET /outings/{outing_id}/checkins",
		"POST /outings/{outing_id}/invites",
		"POST /outings/{outing_id}/invites/{invite_id}/revoke",
		"POST /outings/{outing_id}/invites/{invite_id}/rotate",
		"POST /outing-invites/{token}/accept",
	} {
		if handlers[id] == nil {
			t.Errorf("missing handler %s", id)
		}
	}
}

func TestUnavailablePreviewJSON(t *testing.T) {
	preview := itinerary.Preview{
		Revision: 1,
		Day:      "2026-09-16",
		Status:   itinerary.StatusUnavailable,
		Issues: []journey.Issue{
			journey.NewIssue("routing_unavailable", "Chưa kết nối được dữ liệu đường đi. Thử lại sau.", nil),
		},
	}
	got, err := pyjson.Compact(wireItineraryPreview(preview))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"revision":1,"day":"2026-09-16","status":"unavailable","source":{"engine":"valhalla","graph_version":null,"traffic":"none"},"current":null,"suggestion":null,"savings":null,"issues":[{"code":"routing_unavailable","stop_id":null,"message":"Chưa kết nối được dữ liệu đường đi. Thử lại sau."}]}`
	if string(got) != want {
		t.Fatalf("preview JSON\n got %s\nwant %s", got, want)
	}
}

func TestWireOutingFieldOrder(t *testing.T) {
	view := outingsteps.OutingView{
		ID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", ContextID: "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff",
		CreatedByID: "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa", Title: "Đi Đà Lạt",
		StartsOn:  outingsteps.Date{Year: 2026, Month: 9, Day: 16},
		EndsOn:    outingsteps.Date{Year: 2026, Month: 9, Day: 18},
		Headcount: 4, BudgetPerPersonVND: 500000, Stops: []outingsteps.StopView{},
		TimelineRevision: 0, ItineraryVersion: 1, Days: []outingsteps.Day{},
	}
	got, err := pyjson.Compact(wireOutingView(view))
	if err != nil {
		t.Fatal(err)
	}
	if !jsonHasPrefix(string(got), `{"id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","context_id":`) {
		t.Fatalf("field order: %s", got)
	}
}

func jsonHasPrefix(got, prefix string) bool {
	return len(got) >= len(prefix) && got[:len(prefix)] == prefix
}
