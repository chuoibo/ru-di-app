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

func TestManualPageHeadingIsReadableWithoutChangingSource(t *testing.T) {
	s := Source{Title: "Synthetic trip", Kind: "trip", Photos: []Photo{{ID: "selected", Day: "2026-09-29", Caption: "Synthetic caption"}}}
	d := Compose(s)
	if d.Pages[0].Heading != "Ngày 29/09/2026" || s.Photos[0].Day != "2026-09-29" || d.Pages[0].Text != s.Photos[0].Caption {
		t.Fatalf("date presentation changed the source: %+v", d)
	}
	d.Pages[0].Heading = "Ngày mình muốn nhớ"
	if err := Validate(d, map[string]bool{"selected": true}); err != nil || d.Pages[0].Heading != "Ngày mình muốn nhớ" {
		t.Fatalf("custom heading changed: %+v, %v", d, err)
	}
	if got := dayHeading("not-a-date"); got != "not-a-date" {
		t.Fatalf("invented a date: %q", got)
	}
}

// QA UI-154, books saved before readable headings: the stored ISO title reads
// as a day; a title somebody wrote is untouched.
func TestTenTrangReadsStoredDates(t *testing.T) {
	for in, want := range map[string]string{"2026-09-29": "Ngày 29/09/2026", "2026-02-30": "2026-02-30", "Chiều bên hồ": "Chiều bên hồ", "": ""} {
		if got := TenTrang(in); got != want {
			t.Errorf("TenTrang(%q) = %q, want %q", in, got, want)
		}
	}
}
