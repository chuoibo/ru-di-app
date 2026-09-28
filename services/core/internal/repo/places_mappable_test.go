package repo

import "testing"

// MappablePoint draws the line app.places.geo_precision.mappable_point, the
// ingest's Record.MappablePoint and the phone's veDuocLenBanDo draw.
func TestMappablePointKeepsOnlyAPointThatSaysWhereThePlaceIs(t *testing.T) {
	lat, lng := 10.7769, 106.7009
	precision := func(s string) *string { return &s }
	cases := []struct {
		name  string
		place Place
		keeps bool
	}{
		{"rooftop", Place{Lat: &lat, Lng: &lng, GeoPrecision: precision("rooftop")}, true},
		{"street", Place{Lat: &lat, Lng: &lng, GeoPrecision: precision("street")}, true},
		{"legacy seed, no precision", Place{Lat: &lat, Lng: &lng}, true},
		{"ward centroid", Place{Lat: &lat, Lng: &lng, GeoPrecision: precision("ward_centroid")}, false},
		{"province centroid", Place{Lat: &lat, Lng: &lng, GeoPrecision: precision("province_centroid")}, false},
		{"model guess", Place{Lat: &lat, Lng: &lng, GeoPrecision: precision("suy_luan")}, false},
		{"none", Place{Lat: &lat, Lng: &lng, GeoPrecision: precision("none")}, false},
		{"no point", Place{GeoPrecision: precision("rooftop")}, false},
		{"half a point", Place{Lat: &lat, GeoPrecision: precision("rooftop")}, false},
	}
	for _, c := range cases {
		gotLat, gotLng := c.place.MappablePoint()
		if (gotLat != nil && gotLng != nil) != c.keeps || (gotLat == nil) != (gotLng == nil) {
			t.Errorf("%s: MappablePoint() = %v, %v, want a point = %v", c.name, gotLat, gotLng, c.keeps)
		}
		if c.keeps && (*gotLat != lat || *gotLng != lng) {
			t.Errorf("%s: moved the point to %v, %v", c.name, *gotLat, *gotLng)
		}
	}
}
