package diary

import "testing"

func TestDocumentRefusesUnselectedPhotosAndExecutableLayout(t *testing.T) {
	s := Source{Title: "Synthetic trip", Kind: "trip", Photos: []Photo{{ID: "selected", Day: "2026-09-20"}}}
	d := Compose(s)
	allowed := map[string]bool{"selected": true}
	if err := Validate(d, allowed); err != nil {
		t.Fatal(err)
	}
	d.Pages[0].PhotoIDs = []string{"not-shared"}
	if Validate(d, allowed) == nil {
		t.Fatal("accepted an unshared photo")
	}
	d = Compose(s)
	d.Pages[0].Layout = "script"
	if Validate(d, allowed) == nil {
		t.Fatal("accepted executable layout")
	}
}
func TestFallbackDoesNotInventVisitedPlaces(t *testing.T) {
	d := Compose(Source{Title: "Synthetic evening", Kind: "moment", Places: []string{"Planned place"}})
	if len(d.Pages) != 1 || d.Pages[0].Text != "" || d.AIGenerated {
		t.Fatalf("invented story: %+v", d)
	}
	if SuggestedKind("2026-09-20", "2026-09-21") != "trip" || SuggestedKind("2026-09-20", "2026-09-20") != "moment" {
		t.Fatal("wrong suggestion")
	}
}
