//go:build postgres

package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/testdb"
)

// The two suggestion cards and the reel on their own, now that their Python
// twins are gone (ADR-0052) and parity has nothing to compare them with: the
// membership gate before the model, the keyless answer, and a card the
// model wrote reaching the wire only after grounding on the catalogue.

const (
	goiyPlace   = "p-nuong-thu"
	goiyOutside = "0d00aaaa-bbbb-4ccc-8ddd-eeeeeeeeeed0"
)

// goiySchema is every table of the migrated schema, empty, in a schema of
// its own, holding the expense-draft group (draftSchema's ids), a short
// conversation between its two members and one catalogue place.
func goiySchema(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := fmt.Sprintf("goiy_test_%d", time.Now().UnixNano())
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	rows, err := base.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public'`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		name := pgx.Identifier{table}.Sanitize()
		if _, err := base.Exec(ctx, fmt.Sprintf("CREATE TABLE %s.%s (LIKE public.%s INCLUDING ALL)", ident, name, name)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, stmt := range []string{
		`INSERT INTO people(id,display_name) VALUES ('` + draftOwner + `','Chủ nhóm'),('` + draftMember + `','Bạn'),('` + draftStranger + `','Người lạ')`,
		`INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES ('` + draftRoom + `','Nhóm thử','group','` + draftOwner + `')`,
		`INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES
		 (gen_random_uuid(),'` + draftRoom + `','` + draftOwner + `','active','admin','named'),
		 (gen_random_uuid(),'` + draftRoom + `','` + draftMember + `','active','member','named')`,
		`INSERT INTO messages(id,context_id,author_id,kind,body,created_at) VALUES
		 (gen_random_uuid(),'` + draftRoom + `','` + draftOwner + `','text','Tối nay đi ăn nướng không',now()-interval '2 minutes'),
		 (gen_random_uuid(),'` + draftRoom + `','` + draftMember + `','text','Đi, gần gần thôi nha',now()-interval '1 minute')`,
		`INSERT INTO places(id,destination_id,name,category,source,price_min_vnd,price_max_vnd,open_hours)
		 VALUES ('` + goiyPlace + `','da-lat','Tiệm Nướng Thử','quan-an-local','seed',150000,250000,'16:00 – 23:00')`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func getJSON(h http.Handler, path, actor string) (int, map[string]any) {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if actor != "" {
		r.Header.Set("X-Actor-ID", actor)
		r.Header.Set("X-Actor-Roles", "member")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestGoiYVaReelChanTruocModel(t *testing.T) {
	pool := goiySchema(t)
	stub := llm.NewStub()
	h := draftHandler(t, pool, motluot.Moi(stub, 1))
	paths := []string{
		"/contexts/" + draftRoom + "/suggestion",
		"/contexts/" + draftRoom + "/contextual-suggestion",
		"/contexts/" + draftRoom + "/albums/" + goiyOutside + "/reel",
	}
	for _, path := range paths {
		if status, _ := getJSON(h, path, ""); status != 401 {
			t.Errorf("%s anonymous: %d", path, status)
		}
		if status, _ := getJSON(h, path, draftStranger); status != 403 {
			t.Errorf("%s stranger: %d", path, status)
		}
	}
	// A group with no finished trip has no history to suggest from, and an
	// outing that is not the group's has no album: neither is worth a call.
	if status, body := getJSON(h, paths[0], draftOwner); status != 200 || body["reason"] != "no_history" {
		t.Errorf("no history: %d %v", status, body)
	}
	if status, body := getJSON(h, paths[2], draftOwner); status != 404 || body["code"] != "album_not_found" {
		t.Errorf("foreign outing: %d %v", status, body)
	}
	if stub.SoGoi() != 0 {
		t.Fatalf("a refusal reached the model %d times", stub.SoGoi())
	}
}

func TestGoiYTheoBoiCanhQuaModel(t *testing.T) {
	pool := goiySchema(t)
	path := "/contexts/" + draftRoom + "/contextual-suggestion"
	card := func(place string) string {
		return `{"kind":"outing_suggestion","payload":{"title":"Tối nay nướng","when_text":"19:00","stops":[{"place_id":"` + place + `","time_text":"19:00","note":"Gần, hợp ý cả nhóm"}]}}`
	}
	for _, c := range []struct {
		name   string
		may    *motluot.May
		reason any
	}{
		{"không khoá", nil, "unavailable"},
		{"model nói null", motluot.Moi(llm.NewStub(llm.Buoc{Text: "null"}), 1), "unavailable"},
		{"quán ngoài danh mục", motluot.Moi(llm.NewStub(llm.Buoc{Text: card("p-khong-co")}), 1), "ungrounded"},
	} {
		status, body := getJSON(draftHandler(t, pool, c.may), path, draftOwner)
		if status != 200 || body["suggested"] != false || body["reason"] != c.reason || body["source"] != "none" {
			t.Errorf("%s: %d %v", c.name, status, body)
		}
	}
	stub := llm.NewStub(llm.Buoc{Text: "```json\n" + card(goiyPlace) + "\n```"})
	status, body := getJSON(draftHandler(t, pool, motluot.Moi(stub, 1)), path, draftMember)
	if status != 200 || body["suggested"] != true || body["source"] != "ai" || body["title"] != "Tối nay nướng" {
		t.Fatalf("%d %v", status, body)
	}
	stops := body["stops"].([]any)
	if len(stops) != 1 || stops[0].(map[string]any)["place"].(map[string]any)["id"] != goiyPlace {
		t.Fatalf("stops: %v", stops)
	}
	req := string(stub.YeuCau()[0])
	if !strings.Contains(req, "gần gần thôi") || !strings.Contains(req, goiyPlace) {
		t.Fatal("the model missed the conversation or the catalogue")
	}
	for _, who := range []string{draftOwner, draftMember, "Chủ nhóm"} {
		if strings.Contains(req, who) {
			t.Fatalf("the prompt named a member: %s", who)
		}
	}
}
