//go:build postgres

package chatv2http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/ownership"
)

// ADR-0057 §8.2 on a real PostgreSQL: a member's legacy write to a room on the
// v2 lane is refused; reading the legacy history, a room without the lane, a
// stranger and an anonymous caller all reach the route unchanged.
func TestLegacyWritesFailClosedOnAV2Room(t *testing.T) {
	f := liveSetup(t)
	g := LegacyGuard{Store: chatv2.NewStore(f.pool), Authenticate: Sessions(f.pool), Resolve: resolveForTest}
	plain := newID()
	liveExec(t, f.pool, `INSERT INTO contexts(id,display_name,created_by_id)VALUES($1,'Phòng chưa mã hoá',$2)`, plain, f.people[0].id)
	liveExec(t, f.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin)VALUES($1,$2,$3,'active','member','named')`, newID(), plain, f.people[0].id)
	stranger := newID()
	liveExec(t, f.pool, `INSERT INTO people(id,display_name)VALUES($1,'Người lạ')`, stranger)
	strangerToken := "synthetic-" + newID()
	liveExec(t, f.pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at)VALUES($1,$2,sha256($3::bytea),'genesis',now()+interval '1 day')`, newID(), stranger, strangerToken)

	guard := func(method, path, token string) (bool, int, string) {
		r := httptest.NewRequest(method, path, strings.NewReader("{}"))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		answered := g.Guard(w, r)
		return answered, w.Code, w.Body.String()
	}
	member := f.people[0].token
	room := "/contexts/" + f.conversation
	upper := "/contexts/" + strings.ToUpper(f.conversation)
	bare := "/contexts/" + strings.ReplaceAll(f.conversation, "-", "")
	for _, tc := range []struct{ method, path string }{
		{"POST", room + "/messages"},
		// Spelled as the route itself still accepts it (security review 05/10).
		{"POST", upper + "/messages"},
		{"POST", bare + "/messages"},
		{"POST", "/contexts/{" + f.conversation + "}/messages"},
		{"DELETE", room + "/messages/" + newID()},
		{"POST", room + "/messages/" + newID() + "/reactions"},
		{"DELETE", room + "/messages/" + newID() + "/reactions/like"},
		{"PUT", room + "/read-mark"},
		{"POST", room + "/messages/" + newID() + "/expense-draft"},
	} {
		answered, code, body := guard(tc.method, tc.path, member)
		if !answered || code != http.StatusConflict || !strings.Contains(body, "conversation_is_e2ee") {
			t.Fatalf("%s %s: answered=%v %d %s", tc.method, tc.path, answered, code, body)
		}
	}
	for _, tc := range []struct{ method, path, token string }{
		{"GET", room + "/messages", member},
		{"POST", "/contexts/" + plain + "/messages", member},
		{"POST", room + "/messages", ""},
		{"POST", room + "/messages", strangerToken},
		{"POST", room + "/memories/" + newID() + "/reactions", member},
	} {
		if answered, code, _ := guard(tc.method, tc.path, tc.token); answered {
			t.Fatalf("%s %s answered %d; the route must answer", tc.method, tc.path, code)
		}
	}
}

// resolveForTest is the front door's own decision, from the real manifest.
func resolveForTest(r *http.Request) (string, map[string]string, bool) {
	manifest, err := ownership.Load()
	if err != nil {
		panic(err)
	}
	table, err := router.New(manifest.Routes)
	if err != nil {
		panic(err)
	}
	d := table.DecideTarget(r.Method, r.RequestURI, r.Host, "http")
	return d.RouteID, d.Params, d.Kind == router.KindFull
}
