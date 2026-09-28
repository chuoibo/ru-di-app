//go:build postgres

package routes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

// M7 (vnlocal HANDOFF-GEO §2): only a rooftop or street point says where a
// place is. A ward or province centroid, a model's guess or no point at all
// must never become a stored point, a pin, a distance or a route. Parity
// cannot see this -- its catalogue is the seed, every row rooftop -- so these
// tests are the evidence, on the real schema Alembic built.

const (
	geoContext = "5b1f0c9e-2d4a-4c6b-9e8f-7a6b5c4d3e2f"
	geoMember  = "7d8f1c2a-3b4c-4d5e-8f90-a1b2c3d4e5f6"
)

// geoPlaces is one place per way a point can be, all in one district so a
// meeting or a map would pick any of them if precision were ignored.
var geoPlaces = []struct{ id, precision string }{
	{"geo-rooftop", "rooftop"},
	{"geo-street", "street"},
	{"geo-ward", "ward_centroid"},
	{"geo-tinh", "province_centroid"},
	{"geo-doan", "suy_luan"},
	{"geo-khong", "none"}, // no point at all
}

// geoSchema is a private schema holding the tables these routes read, with
// the catalogue above and one group whose only member is geoMember.
func geoSchema(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	var b [8]byte
	_, _ = rand.Read(b[:])
	schema := "place_geo_test_" + hex.EncodeToString(b[:])
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// LIKE ... INCLUDING ALL keeps the CHECK constraints (memories'
	// payload_matches_kind is the one under test) and drops the foreign keys.
	for _, table := range []string{
		"destinations", "places", "place_photos", "people", "person_interests", "contexts", "memberships",
		"memories", "memory_reactions", "memory_comments",
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	var rows []string
	for i, p := range geoPlaces {
		lat, lng, precision := fmt.Sprintf("10.77%02d", i), fmt.Sprintf("106.70%02d", i), "'"+p.precision+"'"
		if p.precision == "none" {
			lat, lng = "NULL", "NULL"
		}
		rows = append(rows, fmt.Sprintf(
			"('%s','d-tphcm','Quán %s','cafe','[\"cà phê\"]',%s,%s,%s,'[]','seed')", p.id, p.id, lat, lng, precision))
	}
	for _, stmt := range []string{
		`INSERT INTO destinations(id,name,province,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES
		 ('d-tphcm','TP. Hồ Chí Minh','TP. Hồ Chí Minh',10.77,106.7,10.68,106.6,10.88,106.82,20)`,
		`INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,geo_precision,traits,source) VALUES ` +
			strings.Join(rows, ","),
		`INSERT INTO people(id,display_name) VALUES ('` + geoMember + `','Minh Anh')`,
		`INSERT INTO contexts(id,display_name,created_by_id) VALUES ('` + geoContext + `','Team Sài Gòn','` + geoMember + `')`,
		`INSERT INTO memberships(id,context_id,person_id,state,role,joined_at) VALUES
		 (gen_random_uuid(),'` + geoContext + `','` + geoMember + `','active','member',now())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	return pool
}

func geoCall(t *testing.T, h http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		raw, _ := json.Marshal(body)
		reader = strings.NewReader(string(raw))
	}
	r := httptest.NewRequest(method, path, reader)
	r.Header.Set("X-Actor-ID", geoMember)
	r.Header.Set("X-Actor-Roles", "member")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// A check-in keeps the place's point only when the point says where the
// place is. Before M7 a place with no point failed payload_matches_kind (500)
// and a centroid was written into the group's wall for good.
func TestACheckinStoresOnlyAPointThatSaysWhereThePlaceIs(t *testing.T) {
	pool := geoSchema(t)
	h := handlerOn(t, pool)
	for _, p := range geoPlaces {
		code, out := geoCall(t, h, http.MethodPost, "/contexts/"+geoContext+"/checkins", map[string]any{"place_id": p.id})
		if code != 201 {
			t.Fatalf("%s: %d %v", p.id, code, out)
		}
		keeps := p.precision == "rooftop" || p.precision == "street"
		if got := out["lat"] != nil && out["lng"] != nil; got != keeps {
			t.Errorf("%s (%q): wire point kept = %v, want %v (%v, %v)", p.id, p.precision, got, keeps, out["lat"], out["lng"])
		}
		var lat, lng *float64
		if err := pool.QueryRow(context.Background(),
			`SELECT lat, lng FROM memories WHERE place_id = $1`, p.id).Scan(&lat, &lng); err != nil {
			t.Fatal(err)
		}
		if (lat != nil) != keeps || (lng != nil) != keeps {
			t.Errorf("%s (%q): stored a point = %v, want %v", p.id, p.precision, lat != nil || lng != nil, keeps)
		}
	}
}
