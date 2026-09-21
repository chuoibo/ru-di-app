package routes

import (
	"math"
	"sort"
	"testing"
)

// The parity corpus for GET /destinations cannot reach the id half of the sort
// key: it needs two destinations at one point, and the seed has none, so a
// mutant deleting that half left every one of the file's 28 steps EQUAL. These
// cases are what stands in for it. The control in the same run -- truncating
// the distance instead of rounding it -- produced 8 differences, so the gap is
// in the reachable data rather than in the corpus.
func TestNearerDestinationBreaksAnExactTieOnID(t *testing.T) {
	for _, tt := range []struct {
		name string
		iKM  float64
		iID  string
		jKM  float64
		jID  string
		want bool
	}{
		{"closer wins regardless of id", 1, "z", 2, "a", true},
		{"further loses regardless of id", 2, "a", 1, "z", false},
		{"exact tie falls to the lower id", 5, "d-a", 5, "d-b", true},
		{"exact tie the other way round", 5, "d-b", 5, "d-a", false},
		{"zero distance tie still orders by id", 0, "d-a", 0, "d-b", true},
		// A difference of one ulp is not a tie: the distance still decides.
		{"one ulp apart is not a tie", 5, "d-z", math.Nextafter(5, 10), "d-a", true},
	} {
		if got := nearerDestination(tt.iKM, tt.iID, tt.jKM, tt.jID); got != tt.want {
			t.Errorf("%s: nearerDestination(%v, %q, %v, %q) = %v, want %v",
				tt.name, tt.iKM, tt.iID, tt.jKM, tt.jID, got, tt.want)
		}
	}
}

// sort.Slice is not stable, so co-located destinations would come back in an
// arbitrary order without the id half. Python sorts on the tuple and is stable
// besides, so it never wavers; this asserts Go does not either, over enough
// repetitions that an unstable sort would have to be lucky every time.
func TestCoLocatedDestinationsSortDeterministically(t *testing.T) {
	const want = "d-alpha,d-beta,d-gamma,d-delta"
	for attempt := 0; attempt < 64; attempt++ {
		rows := []struct {
			km float64
			id string
		}{
			{12.5, "d-gamma"},
			{12.5, "d-alpha"},
			{99.0, "d-delta"},
			{12.5, "d-beta"},
		}
		sort.Slice(rows, func(i, j int) bool {
			return nearerDestination(rows[i].km, rows[i].id, rows[j].km, rows[j].id)
		})
		got := rows[0].id + "," + rows[1].id + "," + rows[2].id + "," + rows[3].id
		if got != want {
			t.Fatalf("attempt %d: got %q, want %q", attempt, got, want)
		}
	}
}
