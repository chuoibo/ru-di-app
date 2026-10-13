package companion

import (
	"fmt"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/tree"
)

// Random retains catalogue rows and their provenance, with the existing
// five-place ceiling. Other workflows cannot be smuggled into its card.
func TestRandomGroundsOnlyCatalogueChoices(t *testing.T) {
	m := meta
	m.Command = "random"
	var rows []*tree.OrderedMap
	var ids []string
	for i := 1; i <= MaxPlaces+1; i++ {
		id := fmt.Sprintf("r%d", i)
		ids = append(ids, `"`+id+`"`)
		rows = append(rows, parse(t, fmt.Sprintf(`{"id":%q,"name":%q,"source":"curated","source_ref":"synthetic-catalogue"}`, id, "Quán "+id)).(*tree.OrderedMap))
	}
	parts := []tree.Value{
		parse(t, `{"kind":"itinerary","payload":{"title":"No","stops":[{"place_id":"r1","time_text":"19:00","note":""}]}}`),
		parse(t, `{"kind":"expense_draft","payload":{"so_khoan":1,"da_ghi":[]}}`),
		parse(t, `{"kind":"places","payload":{"intro":"","place_ids":[`+strings.Join(ids, ",")+`]}}`),
		parse(t, `{"kind":"text","payload":{"text":"Bạn xem các địa điểm trước khi chơi nhé."}}`),
	}
	card, err := GroundReply(m, parts, rows)
	if err != nil {
		t.Fatal(err)
	}
	s := dump(t, card)
	if !strings.Contains(s, `"lenh": "random"`) || strings.Contains(s, `"kind": "itinerary"`) || strings.Contains(s, `"kind": "expense_draft"`) || strings.Contains(s, `"id": "r6"`) {
		t.Fatalf("random card escaped its scope or ceiling: %s", s)
	}
	for _, want := range []string{`"name": "Quán r1"`, `"source": "curated"`, `"source_ref": "synthetic-catalogue"`, `"omitted_place_count": 1`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing catalogue evidence %s: %s", want, s)
		}
	}
	card, err = GroundReply(m, []tree.Value{
		parse(t, `{"kind":"places","payload":{"intro":"","place_ids":["invented-place"]}}`),
		parse(t, `{"kind":"text","payload":{"text":"Mình chưa tìm được địa điểm phù hợp."}}`),
	}, rows)
	if err != nil || strings.Contains(dump(t, card), "invented-place") || strings.Contains(dump(t, card), `"kind": "places"`) {
		t.Fatalf("invented choice survived: %v %s", err, dump(t, card))
	}
}
