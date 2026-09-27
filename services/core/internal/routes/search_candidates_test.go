package routes

import (
	"fmt"
	"reflect"
	"testing"

	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/repo"
)

func catalogueOf(n int, name func(i int) string) []repo.Place {
	rows := make([]repo.Place, n)
	for i := range rows {
		rows[i] = repo.Place{ID: fmt.Sprintf("p-%04d", i), Name: name(i), Category: "quan-an-local"}
	}
	return rows
}

// TestASmallCatalogueIsSearchedWhole: at or under the cap the prompt lists
// exactly what it listed before -- the parity seed must not notice this step.
func TestASmallCatalogueIsSearchedWhole(t *testing.T) {
	rows := catalogueOf(maxSearchCandidates, func(i int) string { return "Quán" })
	if got := searchCandidates(rows, "phở", taste.Unknown()); !reflect.DeepEqual(got, rows) {
		t.Fatal("a catalogue at the cap was narrowed or reordered")
	}
}

// TestALargeCatalogueKeepsTheMatchesInCatalogueOrder: accent-insensitive
// word hits win a place, and what is kept stays in the catalogue's order.
func TestALargeCatalogueKeepsTheMatchesInCatalogueOrder(t *testing.T) {
	rows := catalogueOf(3000, func(i int) string {
		if i%500 == 7 {
			return "Phở Bò Gia Truyền"
		}
		return "Cà phê sân vườn"
	})
	got := searchCandidates(rows, "pho bo", taste.Unknown())
	if len(got) != maxSearchCandidates {
		t.Fatalf("kept %d, want %d", len(got), maxSearchCandidates)
	}
	matches := 0
	for i, row := range got {
		if row.Name == "Phở Bò Gia Truyền" {
			matches++
		}
		if i > 0 && got[i-1].ID >= row.ID {
			t.Fatalf("not in catalogue order at %d: %s then %s", i, got[i-1].ID, row.ID)
		}
	}
	if matches != 6 {
		t.Errorf("kept %d of the 6 «Phở Bò» rows", matches)
	}
}

func TestFoldVietnamese(t *testing.T) {
	for in, want := range map[string]string{"Phở Đặc Biệt": "pho dac biet", "Bánh Xèo": "banh xeo"} {
		if got := foldVietnamese(in); got != want {
			t.Errorf("fold(%q) = %q, want %q", in, got, want)
		}
	}
}
