//go:build postgres

package avatarfeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

var bg = context.Background()

func id() string {
	s, e := repo.NewUUID()
	if e != nil {
		panic(e)
	}
	return s
}

func exec(t *testing.T, q repo.Querier, sql string, a ...any) {
	t.Helper()
	if _, e := q.Exec(bg, sql, a...); e != nil {
		t.Fatal(e)
	}
}

// world: anh and binh are active in one room, chi is in none, dung left it.
type world struct {
	pool                 *pgxpool.Pool
	store                Store
	anh, binh, chi, dung string
	tokens               map[string]string
}

func setup(t *testing.T) world {
	t.Helper()
	base := testdb.Pool(t)
	schema := "avatar_feed_" + strings.ReplaceAll(id(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	exec(t, base, "CREATE SCHEMA "+quoted)
	t.Cleanup(func() { exec(t, base, "DROP SCHEMA "+quoted+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, e := pgxpool.NewWithConfig(bg, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "account_sessions", "uploaded_images"} {
		exec(t, pool, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	if e = Migrate(bg, pool); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(bg, pool); e != nil {
		t.Fatalf("second migrate must be a no-op: %v", e)
	}
	w := world{pool: pool, store: Store{Pool: pool}, anh: id(), binh: id(), chi: id(), dung: id(), tokens: map[string]string{}}
	for _, p := range []string{w.anh, w.binh, w.chi, w.dung} {
		token := "synthetic-" + id()
		w.tokens[p] = token
		exec(t, pool, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic avatar person')`, p)
		exec(t, pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, id(), p, auth.TokenDigest(token))
	}
	room := id()
	exec(t, pool, `INSERT INTO contexts(id,display_name,created_by_id) VALUES($1,'Synthetic room',$2)`, room, w.anh)
	for _, p := range []string{w.anh, w.binh} {
		exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','member','named')`, id(), room, p)
	}
	exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin,left_at) VALUES($1,$2,$3,'left','member','named',now())`, id(), room, w.dung)
	return w
}

func (w world) avatar(t *testing.T, owner string, at time.Time) string {
	t.Helper()
	image := id()
	exec(t, w.pool, `INSERT INTO uploaded_images(id,storage_key,owner_person_id,uploaded_by_id,content_type,byte_size,width,height,created_at,purpose)
		VALUES($1,$2,$3,$3,'image/png',1,1,1,$4,'avatar')`, image, "synthetic/"+image, owner, at)
	return image
}

func (w world) header(p string) http.Header {
	return http.Header{"Authorization": []string{"Bearer " + w.tokens[p]}}
}

func TestPostgresVersionsFollowTheAvatarRouteRules(t *testing.T) {
	w := setup(t)
	now := time.Now()
	w.avatar(t, w.anh, now.Add(-time.Hour))
	newest := w.avatar(t, w.anh, now)
	// A newer personal photo is never the avatar.
	exec(t, w.pool, `INSERT INTO uploaded_images(id,storage_key,owner_person_id,uploaded_by_id,content_type,byte_size,width,height,purpose)
		VALUES($1,$2,$3,$3,'image/png',1,1,1,'personal')`, id(), "synthetic/p-"+id(), w.anh)
	w.avatar(t, w.chi, now)
	w.avatar(t, w.dung, now)

	actor, err := w.store.Authenticate(bg, w.header(w.binh))
	if err != nil || actor != w.binh {
		t.Fatalf("authenticate: %v %v", actor, err)
	}
	got, err := w.store.Versions(bg, actor, []string{w.anh, w.binh, w.chi, w.dung})
	if err != nil {
		t.Fatal(err)
	}
	if v := got[w.anh]; v == nil || *v != newest {
		t.Errorf("anh = %v, want the newest avatar row %s", v, newest)
	}
	if v, present := got[w.binh]; !present || v != nil {
		t.Errorf("binh (self, no avatar) = %v present %v, want explicit null", v, present)
	}
	if _, present := got[w.chi]; present {
		t.Error("chi shares no group with binh and must be absent")
	}
	if _, present := got[w.dung]; present {
		t.Error("dung left the only shared group and must be absent")
	}
	exec(t, w.pool, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, w.binh)
	if _, err = w.store.Authenticate(bg, w.header(w.binh)); err != ErrAuthentication {
		t.Errorf("revoked session: %v, want ErrAuthentication", err)
	}
}

func readAvatar(t *testing.T, conn *websocket.Conn, within time.Duration) (Event, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(bg, within)
	defer cancel()
	for {
		var e Event
		if err := wsjson.Read(ctx, conn, &e); err != nil {
			return Event{}, false
		}
		if e.Type == "avatar" {
			return e, true
		}
	}
}

// The whole path on a real database: INSERT -> trigger -> NOTIFY -> Listen ->
// Audience -> the socket of a co-member, and nobody else's.
func TestPostgresNewAvatarReachesCoMembersOnly(t *testing.T) {
	w := setup(t)
	ctx, stop := context.WithCancel(bg)
	defer stop()
	h := New(w.store, w.pool, ctx, nil)
	go h.Listen()
	srv := httptest.NewServer(h)
	defer srv.Close()
	dial := func(p string) *websocket.Conn {
		c, _, err := websocket.Dial(bg, strings.Replace(srv.URL, "http", "ws", 1)+"/people/avatars/stream", &websocket.DialOptions{HTTPHeader: w.header(p)})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.CloseNow() })
		return c
	}
	binh, chi := dial(w.binh), dial(w.chi)
	// Registered and the listener attached before the write, or the hint is lost by design.
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.mu.Lock()
		n := len(h.conns)
		h.mu.Unlock()
		var listening bool
		_ = w.pool.QueryRow(bg, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE query = 'LISTEN avatar_changes')`).Scan(&listening)
		if n == 2 && listening {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("setup never settled: conns=%d listening=%v", n, listening)
		}
		time.Sleep(20 * time.Millisecond)
	}
	start := time.Now()
	image := w.avatar(t, w.anh, time.Now())
	e, ok := readAvatar(t, binh, 3*time.Second)
	if !ok || e.PersonID != w.anh || e.AvatarID == nil || *e.AvatarID != image {
		t.Fatalf("binh got %+v ok=%v, want anh's new avatar %s", e, ok, image)
	}
	t.Logf("insert -> co-member socket: %v", time.Since(start))
	if e, ok := readAvatar(t, chi, 500*time.Millisecond); ok {
		t.Fatalf("chi shares no group with anh and received %+v", e)
	}
}
