package routes

import (
	"fmt"
	"math/rand"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

func callFor(target string) *endpoint.Call {
	return &endpoint.Call{Request: httptest.NewRequest("GET", target, nil)}
}

func TestPagingReadsLimitAndOffsetOffTheRawQuery(t *testing.T) {
	for _, tt := range []struct {
		target string
		want   paging
	}{
		{"/places", paging{}},
		{"/places?destination=d-tinh-79", paging{}},
		{"/places?limit=10", paging{paged: true, limit: 10}},
		{"/places?offset=40", paging{paged: true, limit: defaultPageLimit, offset: 40}},
		{"/places?limit=10&offset=20", paging{paged: true, limit: 10, offset: 20}},
		// The ceiling is clamped, not refused: a client asking for 10,000 is
		// answered with the largest page the server allows.
		{"/places?limit=10000", paging{paged: true, limit: maxPageLimit}},
	} {
		got, err := pagingOf(callFor(tt.target))
		if err != nil || got != tt.want {
			t.Errorf("%s: got %+v, %v; want %+v", tt.target, got, err, tt.want)
		}
	}
	for _, bad := range []string{"limit=0", "limit=-3", "limit=abc", "limit=", "offset=-1", "offset=1.5"} {
		if _, err := pagingOf(callFor("/places?" + bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}

// Pages cut from one list must tile it exactly: no row twice, none skipped,
// the last page ends the chain.
func TestPagesTileTheListAndTheLastOneEndsIt(t *testing.T) {
	for _, total := range []int{0, 1, 29, 30, 31, 95, 100, 101} {
		for _, limit := range []int{1, 7, 30, 100} {
			p := paging{paged: true, limit: limit}
			covered, offset := 0, 0
			for step := 0; ; step++ {
				p.offset = offset
				w, next := p.cut(total)
				if w.start != covered {
					t.Fatalf("total=%d limit=%d: page %d starts at %d, want %d", total, limit, step, w.start, covered)
				}
				covered = w.end
				if next == nil {
					break
				}
				if *next != w.end || step > total+1 {
					t.Fatalf("total=%d limit=%d: next=%d end=%d", total, limit, *next, w.end)
				}
				offset = *next
			}
			if covered != total {
				t.Fatalf("total=%d limit=%d: pages covered %d", total, limit, covered)
			}
		}
	}
	// Past the end is an empty last page, not an error and not a wrap.
	w, next := paging{paged: true, limit: 30, offset: 500}.cut(40)
	if w.start != 40 || w.end != 40 || next != nil {
		t.Fatalf("offset past the end: %+v %v", w, next)
	}
	// No paging is the whole list.
	if w, next := (paging{}).cut(12); w != (window{0, 12}) || next != nil {
		t.Fatalf("unpaged: %+v %v", w, next)
	}
}

func rated(n int, rating float64) repo.Place {
	r := rating
	return repo.Place{ID: fmt.Sprintf("p-%03d", n), DestinationID: "d-x", Name: fmt.Sprintf("Quán %d", n),
		Category: "cafe", Kinds: []string{"cà phê"}, Rating: &r, Traits: []string{}}
}

func ids(list []*pyjson.OrderedMap) []string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = placeIDOf(c)
	}
	return out
}

// The order a page is cut from must not depend on the order the database
// returned the rows in: with many equal ratings only the id tiebreak makes it
// total, and without it two pages of one list could overlap.
func TestRankingIsTotalWhateverOrderTheRowsComeIn(t *testing.T) {
	var rows []repo.Place
	for i := 0; i < 120; i++ {
		rows = append(rows, rated(i, float64(3+i%3)))
	}
	snap := &catalogueSnapshot{rows: rows, at: time.Now()}
	want, err := rankPlaces(snap, nil, "", taste.Unknown())
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(7))
	for attempt := 0; attempt < 25; attempt++ {
		shuffled := append([]repo.Place(nil), rows...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := rankPlaces(&catalogueSnapshot{rows: shuffled, at: time.Now()}, nil, "", taste.Unknown())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids(got), ids(want)) {
			t.Fatalf("attempt %d: the order depends on the input order", attempt)
		}
	}
	// Three pages of 50 cut from the list are the list, in order.
	var tiled []string
	p := paging{paged: true, limit: 50}
	for {
		w, next := p.cut(len(want))
		tiled = append(tiled, ids(want[w.start:w.end])...)
		if next == nil {
			break
		}
		p.offset = *next
	}
	if !reflect.DeepEqual(tiled, ids(want)) {
		t.Fatal("pages do not tile the ranked list")
	}
}

func TestRankingFiltersBeforeThePagesAreCut(t *testing.T) {
	rows := []repo.Place{rated(1, 4), rated(2, 5), rated(3, 3)}
	rows[1].Category = "quan-an-local"
	rows[2].Name = "Phở Hà Nội"
	snap := &catalogueSnapshot{rows: rows, at: time.Now()}
	cat := "cafe"
	got, err := rankPlaces(snap, &cat, "", taste.Unknown())
	// Rating 4 (p-001) before rating 3 (p-003); the dearer p-002 is another category.
	if err != nil || strings.Join(ids(got), ",") != "p-001,p-003" {
		t.Fatalf("category filter: %v %v", ids(got), err)
	}
	got, _ = rankPlaces(snap, nil, "phở", taste.Unknown())
	if strings.Join(ids(got), ",") != "p-003" {
		t.Fatalf("q filter: %v", ids(got))
	}
}

func TestReasonsRequestedKeepsAtMostTwelveDistinctIDsInOrder(t *testing.T) {
	ids, asked := reasonsRequested(callFor("/places?reasons_for=a,b,a,,c"))
	if !asked || strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("%v %v", ids, asked)
	}
	var many []string
	for i := 0; i < 30; i++ {
		many = append(many, fmt.Sprintf("x%d", i))
	}
	ids, _ = reasonsRequested(callFor("/places?reasons_for=" + strings.Join(many, ",")))
	if len(ids) != maxReasonRows || ids[0] != "x0" {
		t.Fatalf("%d ids", len(ids))
	}
	if _, asked := reasonsRequested(callFor("/places?destination=d-1")); asked {
		t.Fatal("a plain list request was read as a reasons request")
	}
	if got, asked := reasonsRequested(callFor("/places?reasons_for=")); !asked || len(got) != 0 {
		t.Fatalf("empty reasons_for: %v %v", got, asked)
	}
}

func TestReasonCacheKeysOnTasteAndPlaceAndIsOffWithTheCache(t *testing.T) {
	var c reasonCache
	a := taste.Profile{Basis: taste.BasisPerson, Interests: []string{"cafe"}}
	b := taste.Profile{Basis: taste.BasisPerson, Interests: []string{"cafe", "yen_tinh"}}
	text, verdict := "Hợp.", "hop"
	c.put(reasonKey(a, "p1"), reasonPair{reason: &text, verdict: &verdict}, true)
	if _, ok := c.get(reasonKey(a, "p1"), true); !ok {
		t.Fatal("stored reason not served")
	}
	if _, ok := c.get(reasonKey(b, "p1"), true); ok {
		t.Fatal("another taste was served the reason")
	}
	if _, ok := c.get(reasonKey(a, "p2"), true); ok {
		t.Fatal("another place was served the reason")
	}
	if _, ok := c.get(reasonKey(a, "p1"), false); ok {
		t.Fatal("served while the cache is off")
	}
}

func TestDestinationCoverFallsBackToTheProvinceItLiesIn(t *testing.T) {
	cover := repo.PlacePhoto{ID: "photo-1", PlaceID: "vnl-1"}
	rows := []repo.Destination{
		{ID: "d-tinh-79", Name: "TP.HCM", Lat: 10.77, Lng: 106.7, BBoxSouth: 10.3, BBoxWest: 106.3, BBoxNorth: 11.2, BBoxEast: 107.1},
		{ID: "d-tphcm", Name: "TP.HCM cũ", Lat: 10.77, Lng: 106.7, BBoxSouth: 10.68, BBoxWest: 106.6, BBoxNorth: 10.88, BBoxEast: 106.82},
		{ID: "d-khong-anh", Name: "Chỗ lạ", Lat: 0, Lng: 0},
	}
	covers := (&destinationCovers{byDestination: map[string]repo.PlacePhoto{"d-tinh-79": cover}}).withRows(rows)
	for id, want := range map[string]string{"d-tinh-79": "/places/vnl-1/photos/photo-1", "d-tphcm": "/places/vnl-1/photos/photo-1"} {
		out := pyjson.NewOrderedMap()
		covers.decorate(out, id)
		if v, _ := out.Get("cover_photo_url"); v != pyjson.String(want) {
			t.Errorf("%s: %v", id, v)
		}
	}
	out := pyjson.NewOrderedMap()
	covers.decorate(out, "d-khong-anh")
	if v, _ := out.Get("cover_photo_url"); v != (pyjson.Null{}) {
		t.Errorf("a destination with no photo must say null, got %v", v)
	}
	// A nil receiver (cover not asked for) leaves the body untouched.
	plain := pyjson.NewOrderedMap()
	(*destinationCovers)(nil).decorate(plain, "d-tphcm")
	if plain.Len() != 0 {
		t.Fatal("cover fields added to a request that did not ask")
	}
}
