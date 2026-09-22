package ingest

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every fixture below is invented. Real feed rows describe real businesses and
// carry real platform post ids, and neither belongs in the repository -- the
// data lives in the database, the reader lives here.
func validLine(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	row := map[string]any{
		"schema_version": "place.v1",
		"place_id":       "plc_fixture_one",
		"source":         "vnlocal-ai",
		"ten_chuan":      "Quán Ví Dụ",
		"ten_bien_the":   []any{"Quan Vi Du"},
		"loai":           "cafe",
		"category":       []any{"ca phe"},
		"province_code":  79,
		"province_name":  "Thành phố Hồ Chí Minh",
		"dia_chi":        "1 Đường Ví Dụ",
		"so_bai":         1,
		"posts": []any{map[string]any{
			"platform":   "tiktok",
			"post_id":    "post-fixture-one",
			"source_url": "https://www.tiktok.com/@/video/post-fixture-one",
		}},
		"frames":     []any{},
		"geo":        nil,
		"created_at": "2026-09-13T11:39:33.556000+00:00",
		"updated_at": "2026-09-22T08:47:24.056000+00:00",
	}
	if mutate != nil {
		mutate(row)
	}
	line, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	return line
}

func TestAValidRowParses(t *testing.T) {
	rec, unknown, reject := Parse(validLine(t, nil))
	if reject != nil {
		t.Fatalf("a valid row was refused: %s", reject)
	}
	if len(unknown) != 0 {
		t.Errorf("unexpected unknown keys: %v", unknown)
	}
	if rec.PlaceID != "plc_fixture_one" || rec.CategoryFor() != "cafe" {
		t.Errorf("parsed wrong: %+v", rec)
	}
	if rec.HasPoint() {
		t.Error("a row with geo:null must not claim a point")
	}
}

// TestEachDefectGetsItsOwnReason is the point of having codes at all. A
// rejected row has to be answerable with "why", and a test that only asserts
// "it was refused" passes when every defect produces the same useless code.
func TestEachDefectGetsItsOwnReason(t *testing.T) {
	cases := []struct {
		name   string
		want   string
		mutate func(map[string]any)
	}{
		{"no place id", RejectNoPlaceID, func(r map[string]any) {
			r["place_id"] = "   "
		}},
		{"no name", RejectNoName, func(r map[string]any) {
			r["ten_chuan"] = ""
		}},
		{"a kind the feed does not have", RejectUnknownKind, func(r map[string]any) {
			r["loai"] = "khach_san"
		}},
		{"no posts to anchor identity on", RejectNoPosts, func(r map[string]any) {
			r["posts"] = []any{}
		}},
		{"a post with no platform", RejectPostNoPlatform, func(r map[string]any) {
			r["posts"] = []any{map[string]any{"post_id": "x"}}
		}},
		{"a platform we do not read", RejectPostBadPlatform, func(r map[string]any) {
			r["posts"] = []any{map[string]any{"platform": "facebook", "post_id": "x"}}
		}},
		{"a post with no id", RejectPostNoID, func(r map[string]any) {
			r["posts"] = []any{map[string]any{"platform": "tiktok", "post_id": " "}}
		}},
		{"the wrong contract", RejectSchemaVersion, func(r map[string]any) {
			r["schema_version"] = "place.v2"
		}},
		{"half a point", RejectGeoHalfPoint, func(r map[string]any) {
			r["geo"] = map[string]any{"lat": 10.5, "precision": "rooftop"}
		}},
		{"a point that will not say how it was found", RejectGeoNoPrecision,
			func(r map[string]any) {
				r["geo"] = map[string]any{"lat": 10.5, "lng": 106.5}
			}},
		{"an invented precision", RejectGeoBadPrecision, func(r map[string]any) {
			r["geo"] = map[string]any{
				"lat": 10.5, "lng": 106.5, "precision": "probably"}
		}},
		{"a coordinate source that is neither URL nor OSM ref",
			RejectGeoBadSource, func(r map[string]any) {
				r["geo"] = map[string]any{
					"lat": 10.5, "lng": 106.5, "precision": "rooftop",
					"nguon": []any{"somebody told me"}}
			}},
		{"a frame with no size", RejectFrameNoSize, func(r map[string]any) {
			r["frames"] = []any{map[string]any{
				"storage_key": "plc/x.jpg", "content_type": "image/jpeg",
				"file_exists": true, "byte_size": 100, "width": 0, "height": 0}}
		}},
		{"a frame of a type we cannot read", RejectFrameBadType,
			func(r map[string]any) {
				r["frames"] = []any{map[string]any{
					"storage_key": "plc/x.tif", "content_type": "image/tiff",
					"file_exists": true, "byte_size": 100,
					"width": 10, "height": 10}}
			}},
		{"somebody's words, quoted", RejectVerbatimQuote, func(r map[string]any) {
			r["review"] = map[string]any{
				"mo_ta_tong_quan": "…",
				"bang_chung":      []any{"Mê cái trân châu nhỏ lắm lun"}}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, reject := Parse(validLine(t, testCase.mutate))
			if reject == nil {
				t.Fatalf("expected %s, the row was accepted", testCase.want)
			}
			if reject.Code != testCase.want {
				t.Fatalf("expected %s, got %s", testCase.want, reject)
			}
		})
	}
}

func TestBrokenJSONIsARejectNotAPanic(t *testing.T) {
	_, _, reject := Parse([]byte(`{"place_id": `))
	if reject == nil || reject.Code != RejectBadJSON {
		t.Fatalf("expected %s, got %v", RejectBadJSON, reject)
	}
}

// TestAnUnfamiliarFieldIsCountedNotRefused: the feed's shape moves with its
// prompt versions. A reader that stopped on an unfamiliar field would go down
// on a change that does not concern it; one that ignores it silently would
// never notice the drift. Counted, then.
func TestAnUnfamiliarFieldIsCountedNotRefused(t *testing.T) {
	rec, unknown, reject := Parse(validLine(t, func(r map[string]any) {
		r["dia_chi_web"] = "somewhere the feed started sending"
	}))
	if reject != nil {
		t.Fatalf("an unfamiliar field must not refuse the row: %s", reject)
	}
	if rec == nil {
		t.Fatal("no record")
	}
	if len(unknown) != 1 || unknown[0] != "dia_chi_web" {
		t.Errorf("drift not reported: %v", unknown)
	}
}

// TestAFrameTheFeedSaysIsMissingIsSkipped: the feed marks a frame whose file it
// can no longer find. That is a frame to skip, not a row to throw away -- the
// place is still a place without one of its pictures.
func TestAFrameTheFeedSaysIsMissingIsSkipped(t *testing.T) {
	_, _, reject := Parse(validLine(t, func(r map[string]any) {
		r["frames"] = []any{map[string]any{
			"storage_key": "", "content_type": "", "file_exists": false}}
	}))
	if reject != nil {
		t.Fatalf("a missing frame must not refuse the place: %s", reject)
	}
}

func TestCoordinateSourcesAcceptBothShapes(t *testing.T) {
	for _, source := range []string{
		"osm:way/40219", "osm:node/12", "osm:relation/7",
		"https://foody.vn/somewhere", "http://example.test/a",
	} {
		_, _, reject := Parse(validLine(t, func(r map[string]any) {
			r["geo"] = map[string]any{
				"lat": 10.5, "lng": 106.5, "precision": "rooftop",
				"nguon": []any{source}}
		}))
		if reject != nil {
			t.Errorf("%s was refused: %s", source, reject)
		}
	}
	// An OSM reference is not a URL and must not be validated as one, but it
	// still has a shape: a bare object id says nothing about which object.
	_, _, reject := Parse(validLine(t, func(r map[string]any) {
		r["geo"] = map[string]any{
			"lat": 10.5, "lng": 106.5, "precision": "rooftop",
			"nguon": []any{"osm:40219"}}
	}))
	if reject == nil || reject.Code != RejectGeoBadSource {
		t.Errorf("a reference with no object type was accepted: %v", reject)
	}
}

// TestSevenKindsBecomeFour locks the mapping, including the two judgements in
// it that are easy to get wrong later.
func TestSevenKindsBecomeFour(t *testing.T) {
	cases := []struct {
		loai     string
		category []string
		want     string
	}{
		{"cafe", nil, "cafe"},
		// Named after the dish, but an eatery: every row the feed labels
		// `mon_an` has an address and its own free-text label says `quan_an`.
		{"mon_an", nil, "quan-an-local"},
		{"quan_an", nil, "quan-an-local"},
		{"khu_am_thuc", nil, "quan-an-local"},
		{"diem_tham_quan", nil, "vui-choi"},
		{"trai_nghiem", nil, "vui-choi"},
		{"giai_tri", nil, "vui-choi"},
		{"giai_tri", []string{"cocktail bar"}, "di-choi-dem"},
		{"giai_tri", []string{"hidden bar"}, "di-choi-dem"},
		{"giai_tri", []string{"karaoke"}, "di-choi-dem"},
		// A play starts at eight and so does a bar. Sorting them together would
		// put "kịch nói" under a heading promising something else.
		{"giai_tri", []string{"san khau kich"}, "vui-choi"},
		{"giai_tri", []string{"kich noi"}, "vui-choi"},
		// A water park and a children's playground are the largest part of
		// `giai_tri`, and neither is a night out.
		{"giai_tri", []string{"khu vui choi tre em"}, "vui-choi"},
		{"giai_tri", []string{"cong vien nuoc"}, "vui-choi"},
	}
	for _, testCase := range cases {
		name := testCase.loai + "/" + strings.Join(testCase.category, ",")
		t.Run(name, func(t *testing.T) {
			rec := &Record{Loai: testCase.loai, Category: testCase.category}
			if got := rec.CategoryFor(); got != testCase.want {
				t.Errorf("%s → %s, want %s", name, got, testCase.want)
			}
		})
	}
}

// TestHavingAPointAndBeingFindableAreDifferentClaims. Nearly half the located
// rows carry a province centroid, because their address was a landmark rather
// than a location. They have coordinates. They are not anywhere.
func TestHavingAPointAndBeingFindableAreDifferentClaims(t *testing.T) {
	cases := []struct {
		precision string
		mappable  bool
	}{
		{"rooftop", true},
		{"street", true},
		{"ward_centroid", true},
		{"province_centroid", false},
		{"suy_luan", false},
		{"none", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.precision, func(t *testing.T) {
			rec, _, reject := Parse(validLine(t, func(r map[string]any) {
				r["geo"] = map[string]any{
					"lat": 10.5, "lng": 106.5, "precision": testCase.precision}
			}))
			if reject != nil {
				t.Fatalf("refused: %s", reject)
			}
			if !rec.HasPoint() {
				t.Fatal("the row does carry coordinates")
			}
			if got := rec.MappablePoint(); got != testCase.mappable {
				t.Errorf("%s: mappable=%v, want %v",
					testCase.precision, got, testCase.mappable)
			}
		})
	}
}

// TestOnlyRooftopDuplicatesAreSuspect. Seventeen places on one street share
// that street's point, and that is what `street` means. Treating those as
// duplicates would merge a whole road into one restaurant.
func TestOnlyRooftopDuplicatesAreSuspect(t *testing.T) {
	for precision, suspect := range map[string]bool{
		"rooftop": true, "street": false,
		"ward_centroid": false, "province_centroid": false,
	} {
		rec, _, reject := Parse(validLine(t, func(r map[string]any) {
			r["geo"] = map[string]any{
				"lat": 10.5, "lng": 106.5, "precision": precision}
		}))
		if reject != nil {
			t.Fatalf("%s refused: %s", precision, reject)
		}
		if got := rec.DuplicateCoordinatesAreSuspect(); got != suspect {
			t.Errorf("%s: suspect=%v, want %v", precision, got, suspect)
		}
	}
}

// TestDriftInsideTheEvidenceBagIsSeenToo. The feed grows `geo` faster than it
// grows the top level: ward_code, ta_loai_bo and truy_van all arrived after
// the first delivery. A drift detector that only watched the top level would
// have missed every one of them.
func TestDriftInsideTheEvidenceBagIsSeenToo(t *testing.T) {
	_, unknown, reject := Parse(validLine(t, func(r map[string]any) {
		r["geo"] = map[string]any{
			"lat": 10.5, "lng": 106.5, "precision": "rooftop",
			"ward_code": 26743, "do_cao_met": 12,
		}
	}))
	if reject != nil {
		t.Fatalf("a new evidence field must not refuse the row: %s", reject)
	}
	if len(unknown) != 1 || unknown[0] != "geo.do_cao_met" {
		t.Errorf("drift inside geo not reported: %v", unknown)
	}
}

// An absent evidence field is not drift: it means that step produced nothing.
func TestAnAbsentEvidenceFieldIsNotDrift(t *testing.T) {
	_, unknown, reject := Parse(validLine(t, func(r map[string]any) {
		r["geo"] = map[string]any{"lat": 10.5, "lng": 106.5, "precision": "street"}
	}))
	if reject != nil || len(unknown) != 0 {
		t.Errorf("reject=%v unknown=%v", reject, unknown)
	}
}

// TestAClaimedPrecisionIsLoweredNotRefused. One row arrived as lat=10 labelled
// `street`. A whole degree is about 111 km, so the label promised a road and
// delivered a province. The place is real and the coordinate is usable; only
// the word was wrong, and the right word is computable -- so it is corrected
// and counted, not thrown away.
func TestAClaimedPrecisionIsLoweredNotRefused(t *testing.T) {
	cases := []struct {
		name      string
		lat, lng  any
		claimed   string
		want      string
		corrected bool
	}{
		{"a whole degree calling itself a street", 10, 106, "street", "province_centroid", true},
		{"a whole degree calling itself a ward", 10, 106, "ward_centroid", "province_centroid", true},
		{"a whole degree is honest about a province", 10, 106, "province_centroid", "province_centroid", false},
		{"three decimals calling itself a rooftop", 10.758, 106.660, "rooftop", "ward_centroid", true},
		{"three decimals is honest about a ward", 10.758, 106.660, "ward_centroid", "ward_centroid", false},
		{"four decimals may claim a rooftop", 10.7584, 106.6601, "rooftop", "rooftop", false},
		{"one coordinate rounded drags the pair down", 10.7584, 106, "rooftop", "province_centroid", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rec, _, reject := Parse(validLine(t, func(r map[string]any) {
				r["geo"] = map[string]any{
					"lat": testCase.lat, "lng": testCase.lng,
					"precision": testCase.claimed}
			}))
			if reject != nil {
				t.Fatalf("a real place must not be lost over a label: %s", reject)
			}
			_, corrected := rec.CapPrecision()
			if corrected != testCase.corrected {
				t.Errorf("corrected=%v, want %v", corrected, testCase.corrected)
			}
			if rec.Geo.Precision != testCase.want {
				t.Errorf("precision %s, want %s", rec.Geo.Precision, testCase.want)
			}
		})
	}
}
