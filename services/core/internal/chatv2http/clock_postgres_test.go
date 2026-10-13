//go:build postgres

package chatv2http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/testdb"
)

// No public fallback: migration metadata, triggers and all fixture rows belong
// to this schema. The third person has a live session/device but no membership.
func clockSetup(t *testing.T) liveWorld {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "chat_clock_" + strings.ReplaceAll(newID(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests", "account_sessions"} {
		liveExec(t, pool, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	if err := chatv2.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f := liveWorld{pool: pool, conversation: newID()}
	for i := 0; i < 3; i++ {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		p := livePerson{id: newID(), device: newID(), membership: newID(), token: "synthetic-clock-" + newID(), key: key}
		f.people = append(f.people, p)
		liveExec(t, pool, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic clock participant')`, p.id)
		liveExec(t, pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()+interval '1 day')`, newID(), p.id, auth.TokenDigest(p.token))
		liveExec(t, pool, `INSERT INTO chat_v2_devices(id,person_id,signing_key) VALUES($1,$2,$3)`, p.device, p.id, []byte(pub))
	}
	liveExec(t, pool, `INSERT INTO contexts(id,display_name,created_by_id) VALUES($1,'Synthetic clock room',$2)`, f.conversation, f.people[0].id)
	for _, p := range f.people[:2] {
		liveExec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','member','named')`, p.membership, f.conversation, p.id)
	}
	liveExec(t, pool, `INSERT INTO chat_v2_conversations(context_id,epoch,ready) VALUES($1,1,true)`, f.conversation)
	for _, p := range f.people[:2] {
		liveExec(t, pool, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence) VALUES($1,$2,$3,1)`, f.conversation, p.device, p.membership)
	}
	return f
}

func clockHTTP(h http.Handler, token, conversation, device string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/v2/chat/"+conversation+"/clock?device_id="+device, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func clockDatabaseTime(t *testing.T, ctx context.Context, pool *pgxpool.Pool) time.Time {
	t.Helper()
	var now time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	return now
}

// Compare every fixture row, including migration receipts, plus the schema's
// relations/columns/functions/triggers. A count alone misses in-place changes.
func clockDatabaseState(t *testing.T, f liveWorld) map[string]string {
	t.Helper()
	ctx := context.Background()
	state := map[string]string{}
	rows, err := f.pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=current_schema() ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range tables {
		var data string
		q := `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM ` + pgx.Identifier{name}.Sanitize() + ` t`
		if err := f.pool.QueryRow(ctx, q).Scan(&data); err != nil {
			t.Fatal(err)
		}
		state[name] = data
	}
	for name, q := range map[string]string{
		"relations":   `SELECT COALESCE(jsonb_agg(jsonb_build_array(c.oid,c.relname,c.relkind,c.relnatts) ORDER BY c.oid),'[]'::jsonb)::text FROM pg_class c WHERE c.relnamespace=current_schema()::regnamespace`,
		"columns":     `SELECT COALESCE(jsonb_agg(jsonb_build_array(a.attrelid,a.attnum,a.attname,a.atttypid,a.attnotnull,pg_get_expr(d.adbin,d.adrelid)) ORDER BY a.attrelid,a.attnum),'[]'::jsonb)::text FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE c.relnamespace=current_schema()::regnamespace AND a.attnum>0 AND NOT a.attisdropped`,
		"functions":   `SELECT COALESCE(jsonb_agg(jsonb_build_array(p.oid,pg_get_functiondef(p.oid)) ORDER BY p.oid),'[]'::jsonb)::text FROM pg_proc p WHERE p.pronamespace=current_schema()::regnamespace`,
		"triggers":    `SELECT COALESCE(jsonb_agg(jsonb_build_array(t.oid,pg_get_triggerdef(t.oid)) ORDER BY t.oid),'[]'::jsonb)::text FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid WHERE c.relnamespace=current_schema()::regnamespace`,
		"constraints": `SELECT COALESCE(jsonb_agg(jsonb_build_array(c.oid,pg_get_constraintdef(c.oid)) ORDER BY c.oid),'[]'::jsonb)::text FROM pg_constraint c WHERE c.connamespace=current_schema()::regnamespace`,
	} {
		var data string
		if err := f.pool.QueryRow(ctx, q).Scan(&data); err != nil {
			t.Fatal(err)
		}
		state[name] = data
	}
	return state
}

func TestPostgresClockHTTPUsesDatabaseTimeAndChangesNothing(t *testing.T) {
	f := clockSetup(t)
	ctx := context.Background()
	s := chatv2.NewStore(f.pool)
	p := f.people[0]
	var sent chatv2.SendResult
	for i := 0; i < 3; i++ {
		var err error
		sent, err = s.SendSession(ctx, p.id, auth.TokenDigest(p.token), f.envelope(0))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.MarkSession(ctx, p.id, auth.TokenDigest(p.token), p.device, f.conversation, "read", sent.Event.Sequence); err != nil {
		t.Fatal(err)
	}
	liveExec(t, f.pool, `UPDATE chat_v2_members SET first_sequence=3 WHERE context_id=$1 AND device_id=$2`, f.conversation, f.people[1].device)
	before := clockDatabaseState(t, f)
	h := New(Options{Store: s, Authenticate: Sessions(f.pool), Experimental: true})
	for _, person := range f.people[:2] {
		lower := clockDatabaseTime(t, ctx, f.pool)
		w := clockHTTP(h, person.token, f.conversation, person.device)
		upper := clockDatabaseTime(t, ctx, f.pool)
		var out map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		now, err := time.Parse(time.RFC3339Nano, out["server_now"])
		if w.Code != http.StatusOK || err != nil || len(out) != 1 || now.Before(lower) || now.After(upper) || now.Before(sent.Event.CreatedAt) || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("clock status=%d sample=%s lower=%s upper=%s parse=%v", w.Code, out["server_now"], lower, upper, err)
		}
	}
	if after := clockDatabaseState(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal("clock changed chat rows or schema metadata")
	}
	// Sampling never moves the late member's historical visibility boundary.
	page, err := s.Events(ctx, f.people[1].id, f.people[1].device, f.conversation, 0, 100)
	if err != nil || len(page.Events) != 2 || page.Events[0].Sequence != 3 {
		t.Fatalf("clock changed first_sequence history: events=%d err=%v", len(page.Events), err)
	}
	// Reads remain possible while roster repair is needed (ADR-0057 §3.4).
	liveExec(t, f.pool, `UPDATE chat_v2_conversations SET ready=false WHERE context_id=$1`, f.conversation)
	if w := clockHTTP(h, p.token, f.conversation, p.device); w.Code != http.StatusOK {
		t.Fatalf("clock required send readiness: %d %s", w.Code, w.Body.String())
	}
}

func TestPostgresClockHTTPRejectsRevokedOrUnboundReaders(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, liveWorld)
		person int
		device int
		token  string
		status int
	}{
		{name: "missing_bearer", token: "missing", status: http.StatusUnauthorized},
		{name: "unknown_bearer", token: "unknown", status: http.StatusUnauthorized},
		{name: "nonmember", person: 2, device: 2, status: http.StatusForbidden},
		{name: "another_person_device", device: 1, status: http.StatusForbidden},
		{name: "revoked_session", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1`, f.people[0].id)
		}, status: http.StatusUnauthorized},
		{name: "expired_session", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `UPDATE account_sessions SET created_at=clock_timestamp()-interval '1 day',expires_at=clock_timestamp()-interval '1 second' WHERE person_id=$1`, f.people[0].id)
		}, status: http.StatusUnauthorized},
		{name: "revoked_device", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `UPDATE chat_v2_devices SET revoked_at=clock_timestamp() WHERE id=$1`, f.people[0].device)
		}, status: http.StatusForbidden},
		{name: "deleted_account", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `UPDATE people SET deleted_at=clock_timestamp() WHERE id=$1`, f.people[0].id)
		}, status: http.StatusUnauthorized},
		{name: "left_membership", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE id=$1`, f.people[0].membership)
		}, status: http.StatusForbidden},
		{name: "missing_room_device", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `DELETE FROM chat_v2_members WHERE context_id=$1 AND device_id=$2`, f.conversation, f.people[0].device)
		}, status: http.StatusForbidden},
		{name: "blocked_pair", mutate: func(t *testing.T, f liveWorld) {
			liveExec(t, f.pool, `UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, f.conversation, f.people[0].id+":"+f.people[1].id)
			liveExec(t, f.pool, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_at,decided_by_id) VALUES($1,$2,$3,'blocked',clock_timestamp(),$2)`, newID(), f.people[1].id, f.people[0].id)
		}, status: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := clockSetup(t)
			if tc.mutate != nil {
				tc.mutate(t, f)
			}
			before := clockDatabaseState(t, f)
			token := f.people[tc.person].token
			if tc.token == "missing" {
				token = ""
			} else if tc.token == "unknown" {
				token = "synthetic-unknown-session"
			}
			h := New(Options{Store: chatv2.NewStore(f.pool), Authenticate: Sessions(f.pool), Experimental: true})
			w := clockHTTP(h, token, f.conversation, f.people[tc.device].device)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "server_now") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if after := clockDatabaseState(t, f); !reflect.DeepEqual(before, after) {
				t.Fatal("refused clock changed chat rows or schema metadata")
			}
		})
	}
}

func TestPostgresClockStoreBindsSessionActorAndCanonicalIDs(t *testing.T) {
	f := clockSetup(t)
	s := chatv2.NewStore(f.pool)
	p := f.people[0]
	ctx := context.Background()
	if _, err := s.Clock(ctx, p.id, auth.TokenDigest(f.people[1].token), p.device, f.conversation); !errors.Is(err, chatv2.ErrForbidden) {
		t.Fatalf("another actor's bearer admitted: %v", err)
	}
	for _, tc := range []struct {
		actor, device, conversation string
		digest                      []byte
	}{
		{p.id, p.device, f.conversation, nil},
		{p.id, p.device, f.conversation, []byte("short")},
		{strings.ToUpper(personID), p.device, f.conversation, auth.TokenDigest(p.token)},
		{p.id, "bad-device", f.conversation, auth.TokenDigest(p.token)},
		{p.id, p.device, "bad-room", auth.TokenDigest(p.token)},
	} {
		if _, err := s.Clock(ctx, tc.actor, tc.digest, tc.device, tc.conversation); !errors.Is(err, chatv2.ErrInvalid) {
			t.Fatalf("invalid clock arguments accepted: %v", err)
		}
	}
}

func TestPostgresClockStoreRechecksSessionAfterActorResolution(t *testing.T) {
	for _, revoke := range []string{"revoked_at=clock_timestamp()", "created_at=clock_timestamp()-interval '1 day',expires_at=clock_timestamp()-interval '1 second'"} {
		t.Run(revoke, func(t *testing.T) {
			f := clockSetup(t)
			ctx := context.Background()
			p := f.people[0]
			headers := http.Header{"Authorization": []string{"Bearer " + p.token}}
			actor, err := Sessions(f.pool)(ctx, headers)
			if err != nil || actor != p.id {
				t.Fatalf("initial actor resolution: %v", err)
			}
			liveExec(t, f.pool, `UPDATE account_sessions SET `+revoke+` WHERE person_id=$1`, p.id)
			now, err := chatv2.NewStore(f.pool).Clock(ctx, actor, auth.TokenDigest(p.token), p.device, f.conversation)
			if !errors.Is(err, chatv2.ErrForbidden) || !now.IsZero() {
				t.Fatalf("reused pre-revocation actor: sample=%v err=%v", now, err)
			}
		})
	}
}

func TestPostgresClockSamplesAfterPermissionWaitAndRejectsExpiry(t *testing.T) {
	for _, expire := range []bool{false, true} {
		name := "fresh_sample_after_wait"
		if expire {
			name = "expired_during_wait"
		}
		t.Run(name, func(t *testing.T) {
			f := clockSetup(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			p := f.people[0]
			blocker, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback(ctx)
			if _, err := blocker.Exec(ctx, `SELECT context_id FROM chat_v2_conversations WHERE context_id=$1 FOR UPDATE`, f.conversation); err != nil {
				t.Fatal(err)
			}
			cfg := f.pool.Config().Copy()
			app := "clock_wait_" + strings.ReplaceAll(newID(), "-", "")
			cfg.ConnConfig.RuntimeParams["application_name"] = app
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				cancel()
				_ = blocker.Rollback(context.Background())
				pool.Close()
			}()
			if expire {
				liveExec(t, f.pool, `UPDATE account_sessions SET expires_at=clock_timestamp()+interval '2 seconds' WHERE person_id=$1`, p.id)
			}
			type result struct {
				now time.Time
				err error
			}
			done := make(chan result, 1)
			go func() {
				now, err := chatv2.NewStore(pool).Clock(ctx, p.id, auth.TokenDigest(p.token), p.device, f.conversation)
				done <- result{now, err}
			}()
			blocked := false
			for !blocked {
				if err := f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, app).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				select {
				case r := <-done:
					t.Fatalf("clock crossed held permission boundary: %v", r.err)
				default:
				}
				if !blocked {
					select {
					case <-time.After(5 * time.Millisecond):
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
				}
			}
			if expire {
				expired := false
				for !expired {
					if err := f.pool.QueryRow(ctx, `SELECT expires_at<=clock_timestamp() FROM account_sessions WHERE person_id=$1`, p.id).Scan(&expired); err != nil {
						t.Fatal(err)
					}
					if !expired {
						select {
						case <-time.After(5 * time.Millisecond):
						case <-ctx.Done():
							t.Fatal(ctx.Err())
						}
					}
				}
			}
			lower := clockDatabaseTime(t, ctx, f.pool)
			if err := blocker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case r := <-done:
				if expire {
					if !errors.Is(r.err, chatv2.ErrForbidden) || !r.now.IsZero() {
						t.Fatalf("expired session got clock: %v %v", r.now, r.err)
					}
				} else if upper := clockDatabaseTime(t, ctx, f.pool); r.err != nil || r.now.Before(lower) || r.now.After(upper) {
					t.Fatalf("clock used transaction-start time: %v lower=%v upper=%v err=%v", r.now, lower, upper, r.err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
