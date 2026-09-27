package diary

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	book "mobile/services/core/internal/domain/diary"
	"mobile/services/core/internal/media/storage"
	"mobile/services/core/internal/testdb"
)

type fixture struct {
	pool                                                                         *pgxpool.Pool
	h                                                                            *Handler
	person, peer, outsider, room, outing, photo, token, peerToken, outsiderToken string
}

func setup(t *testing.T) fixture {
	t.Helper()
	pool := testdb.Pool(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f := fixture{pool: pool, person: uuid(), peer: uuid(), outsider: uuid(), room: uuid(), outing: uuid(), photo: uuid(), token: "synthetic-" + uuid(), peerToken: "synthetic-" + uuid(), outsiderToken: "synthetic-" + uuid()}
	f.h = New(pool, nil)
	exec := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO people(id,display_name) VALUES($1,'Synthetic organizer'),($2,'Synthetic member'),($3,'Synthetic reader')`, f.person, f.peer, f.outsider)
	exec(`INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES($1,'Synthetic diary group','group',$2)`, f.room, f.person)
	exec(`INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','admin','named'),($4,$2,$5,'active','member','named')`, uuid(), f.room, f.person, uuid(), f.peer)
	for _, s := range []struct{ person, token string }{{f.person, f.token}, {f.peer, f.peerToken}, {f.outsider, f.outsiderToken}} {
		exec(`INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()+interval '1 hour')`, uuid(), s.person, auth.TokenDigest(s.token))
	}
	exec(`INSERT INTO outings(id,context_id,created_by_id,title,starts_on,ends_on,headcount,budget_per_person_vnd) VALUES($1,$2,$3,'Synthetic diary outing','2026-01-01','2026-01-02',2,0)`, f.outing, f.room, f.person)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	st, err := storage.New()
	if err != nil {
		t.Fatal(err)
	}
	key, err := storage.NewStorageKey()
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Write(key, []byte("synthetic-photo-bytes")); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO uploaded_images(id,storage_key,context_id,uploaded_by_id,purpose,content_type,byte_size,width,height) VALUES($1,$2,$3,$4,'group','image/jpeg',21,10,10)`, f.photo, key, f.room, f.peer)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM outing_diaries WHERE outing_id=$1`, f.outing)
		pool.Exec(ctx, `DELETE FROM outing_diary_jobs WHERE outing_id=$1`, f.outing)
		pool.Exec(ctx, `DELETE FROM outing_endings WHERE outing_id=$1`, f.outing)
		pool.Exec(ctx, `DELETE FROM uploaded_images WHERE context_id=$1`, f.room)
		pool.Exec(ctx, `DELETE FROM outings WHERE id=$1`, f.outing)
		pool.Exec(ctx, `DELETE FROM memberships WHERE context_id=$1`, f.room)
		pool.Exec(ctx, `DELETE FROM contexts WHERE id=$1`, f.room)
		pool.Exec(ctx, `DELETE FROM account_sessions WHERE person_id=ANY($1::uuid[])`, []string{f.person, f.peer, f.outsider})
		pool.Exec(ctx, `DELETE FROM people WHERE id=ANY($1::uuid[])`, []string{f.person, f.peer, f.outsider})
	})
	return f
}
func (f fixture) request(method, path, token string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}
func require(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
}
func (f fixture) close(t *testing.T) {
	require(t, f.request("POST", "/outings/"+f.outing+"/ending", f.token, map[string]string{"kind": "trip"}), 200)
}
func (f fixture) doc() book.Document {
	return book.Compose(book.Source{Title: "Synthetic memory", Kind: "trip", Photos: []book.Photo{{ID: f.photo, Day: "2026-01-01"}}})
}
func (f fixture) save(t *testing.T, rev int, audience string) Book {
	w := f.request("PUT", "/outings/"+f.outing+"/diary", f.token, map[string]any{"revision": rev, "audience": audience, "document": f.doc()})
	require(t, w, 200)
	var b Book
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestPostgresEndingAuthorityAndRetry(t *testing.T) {
	f := setup(t)
	path := "/outings/" + f.outing + "/ending"
	require(t, f.request("POST", path, f.peerToken, map[string]string{"kind": "trip"}), 403)
	require(t, f.request("GET", path, f.outsiderToken, nil), 404)
	f.close(t)
	f.close(t)
	require(t, f.request("POST", path, f.token, map[string]string{"kind": "moment"}), 409)
}
func TestPostgresDiaryPrivacyIncludesImageBytes(t *testing.T) {
	f := setup(t)
	f.close(t)
	b := f.save(t, 0, "private")
	path := "/diaries/" + b.ID
	photo := path + "/photos/" + f.photo
	require(t, f.request("GET", path, f.outsiderToken, nil), 404)
	require(t, f.request("GET", photo, f.outsiderToken, nil), 404)
	require(t, f.request("GET", photo, f.token, nil), 200)
	b = f.save(t, b.Revision, "public")
	require(t, f.request("GET", path, f.outsiderToken, nil), 200)
	w := f.request("GET", photo, f.outsiderToken, nil)
	require(t, w, 200)
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("cache can retain public bytes")
	}
	b = f.save(t, b.Revision, "private")
	require(t, f.request("GET", path, f.outsiderToken, nil), 404)
	require(t, f.request("GET", photo, f.outsiderToken, nil), 404)
	require(t, f.request("DELETE", path, f.peerToken, nil), 404)
	require(t, f.request("DELETE", path, f.token, nil), 204)
	require(t, f.request("GET", photo, f.token, nil), 404)
}
func TestPostgresDiaryRevisionConflictAndForeignPhoto(t *testing.T) {
	f := setup(t)
	f.close(t)
	b := f.save(t, 0, "private")
	f.save(t, 0, "private")
	d := f.doc()
	d.Title = "New edition"
	require(t, f.request("PUT", "/outings/"+f.outing+"/diary", f.token, map[string]any{"revision": 0, "audience": "private", "document": d}), 409)
	d.CoverID = uuid()
	require(t, f.request("PUT", "/outings/"+f.outing+"/diary", f.token, map[string]any{"revision": b.Revision, "audience": "public", "document": d}), 404)
}
func TestPostgresParallelFirstSaveMakesOneBook(t *testing.T) {
	f := setup(t)
	f.close(t)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- f.request("PUT", "/outings/"+f.outing+"/diary", f.token, map[string]any{"revision": 0, "audience": "private", "document": f.doc()}).Code
		}()
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != 200 {
			t.Fatalf("retry status %d", c)
		}
	}
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM outing_diaries WHERE outing_id=$1`, f.outing).Scan(&n); err != nil || n != 1 {
		t.Fatalf("books=%d err=%v", n, err)
	}
}
func TestPostgresExplicitBundleAndModelSourceValidation(t *testing.T) {
	f := setup(t)
	f.close(t)
	called := false
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var v struct {
			Source book.Source `json:"source"`
		}
		json.NewDecoder(r.Body).Decode(&v)
		if len(v.Source.Excerpts) != 1 || v.Source.Excerpts[0] != "Synthetic explicitly selected excerpt" {
			t.Error("wrong shared bundle")
		}
		d := f.doc()
		d.CoverID = uuid()
		reply(w, 200, d)
	}))
	defer stub.Close()
	t.Setenv("MOBILE_BRAIN_URL", stub.URL)
	t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-test-token")
	f.h = New(f.pool, brain.Configured())
	source := book.Source{Title: "Synthetic", StartsOn: "2026-01-01", EndsOn: "2026-01-02", Kind: "trip", Photos: []book.Photo{{ID: f.photo, Day: "2026-01-01"}}, Places: []string{}, Excerpts: []string{"Synthetic explicitly selected excerpt"}}
	in := map[string]any{"logical_id": uuid(), "confirmed": true, "use_ai": true, "source": source}
	w := f.request("POST", "/outings/"+f.outing+"/diary-jobs", f.token, in)
	require(t, w, 202)
	var j Job
	json.Unmarshal(w.Body.Bytes(), &j)
	if _, err := f.h.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("no inference")
	}
	w = f.request("GET", "/diary-jobs/"+j.ID, f.token, nil)
	require(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &j)
	if j.Status != "failed" || j.Code == nil || *j.Code != "invalid_diary_result" {
		t.Fatalf("unselected photo accepted: %+v", j)
	}
	var cleared bool
	f.pool.QueryRow(context.Background(), `SELECT source IS NULL FROM outing_diary_jobs WHERE id=$1`, j.ID).Scan(&cleared)
	if !cleared {
		t.Fatal("retained shared transcript")
	}
	require(t, f.request("GET", "/diary-jobs/"+j.ID, f.peerToken, nil), 404)
}
func TestPostgresSourcesNeverRequireChat(t *testing.T) {
	f := setup(t)
	f.close(t)
	w := f.request("GET", "/outings/"+f.outing+"/diary-sources", f.token, nil)
	require(t, w, 200)
	var s book.Source
	json.Unmarshal(w.Body.Bytes(), &s)
	if len(s.Excerpts) != 0 || len(s.Photos) != 1 {
		t.Fatalf("wrong sources %+v", s)
	}
}

func TestPostgresAIRequiresConfirmationAndLeavesManualFallback(t *testing.T) {
	f := setup(t)
	f.close(t)
	source := book.Source{Title: "Synthetic", StartsOn: "2026-01-01", EndsOn: "2026-01-02", Kind: "trip", Photos: []book.Photo{}, Places: []string{}, Excerpts: []string{}}
	path := "/outings/" + f.outing + "/diary-jobs"
	in := map[string]any{"logical_id": uuid(), "confirmed": false, "use_ai": true, "source": source}
	require(t, f.request("POST", path, f.token, in), 422)
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM outing_diary_jobs WHERE outing_id=$1`, f.outing).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unconfirmed request persisted: count=%d err=%v", count, err)
	}
	in["confirmed"], in["use_ai"] = true, false
	for i := 0; i < 12; i++ {
		in["logical_id"] = uuid()
		require(t, f.request("POST", path, f.token, in), 202)
	}
	in["logical_id"], in["use_ai"] = uuid(), true
	require(t, f.request("POST", path, f.token, in), 429)
	in["logical_id"], in["use_ai"] = uuid(), false
	w := f.request("POST", path, f.token, in)
	require(t, w, 202)
	var j Job
	if err := json.Unmarshal(w.Body.Bytes(), &j); err != nil || j.Result == nil || j.Result.AIGenerated {
		t.Fatalf("fallback missing or presented as AI: %s", w.Body.String())
	}
}

func TestPostgresOwnerCanWithdrawAfterLeaving(t *testing.T) {
	f := setup(t)
	f.close(t)
	b := f.save(t, 0, "public")
	if _, err := f.pool.Exec(context.Background(), `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE context_id=$1 AND person_id=$2`, f.room, f.person); err != nil {
		t.Fatal(err)
	}
	require(t, f.request("PATCH", "/diaries/"+b.ID+"/audience", f.token, map[string]any{"revision": b.Revision, "audience": "private"}), 200)
	require(t, f.request("GET", "/diaries/"+b.ID+"/photos/"+f.photo, f.outsiderToken, nil), 404)
}
func TestPostgresRejoiningDoesNotReviveAIConsent(t *testing.T) {
	f := setup(t)
	f.close(t)
	source := book.Source{Title: "Synthetic", StartsOn: "2026-01-01", EndsOn: "2026-01-02", Kind: "trip", Photos: []book.Photo{}, Places: []string{}, Excerpts: []string{"Synthetic explicitly shared excerpt"}}
	w := f.request("POST", "/outings/"+f.outing+"/diary-jobs", f.token, map[string]any{"logical_id": uuid(), "confirmed": true, "use_ai": true, "source": source})
	require(t, w, 202)
	var j Job
	json.Unmarshal(w.Body.Bytes(), &j)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE context_id=$1 AND person_id=$2`, f.room, f.person); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE memberships SET state='active',left_at=NULL WHERE context_id=$1 AND person_id=$2`, f.room, f.person); err != nil {
		t.Fatal(err)
	}
	w = f.request("GET", "/diary-jobs/"+j.ID, f.token, nil)
	require(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &j)
	if j.Status != "failed" || j.Code == nil || *j.Code != "sharing_revoked" {
		t.Fatalf("revived consent %+v", j)
	}
}

func TestPostgresCoupleConsentAllowsEitherPersonToEnd(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, f.room, f.person+":"+f.peer)
	path := "/outings/" + f.outing + "/ending"
	// Two friends sharing a pair context are not implicitly a couple.
	require(t, f.request("POST", path, f.peerToken, map[string]string{"kind": "trip"}), 403)
	notebook, cycle, proposal := uuid(), uuid(), uuid()
	exec(`INSERT INTO pair_notebooks(id,context_id) VALUES($1,$2)`, notebook, f.room)
	exec(`INSERT INTO pair_notebook_cycles(id,notebook_id,state,opened_at) VALUES($1,$2,'active',clock_timestamp())`, cycle, notebook)
	t.Cleanup(func() {
		exec(`DELETE FROM active_couple_members WHERE cycle_id=$1`, cycle)
		exec(`DELETE FROM pair_consents WHERE proposal_id=$1`, proposal)
		exec(`DELETE FROM pair_consent_proposals WHERE id=$1`, proposal)
		exec(`DELETE FROM pair_cycle_participants WHERE cycle_id=$1`, cycle)
		exec(`DELETE FROM pair_notebook_cycles WHERE id=$1`, cycle)
		exec(`DELETE FROM pair_notebooks WHERE id=$1`, notebook)
	})
	exec(`INSERT INTO pair_cycle_participants(cycle_id,person_id) VALUES($1,$2),($1,$3)`, cycle, f.person, f.peer)
	exec(`INSERT INTO pair_consent_proposals(id,cycle_id,purpose,proposed_by_id,completed_at,expires_at) VALUES($1,$2,'bat_doi',$3,clock_timestamp(),clock_timestamp()+interval '1 day')`, proposal, cycle, f.person)
	exec(`INSERT INTO pair_consents(id,proposal_id,person_id,granted_at) VALUES($1,$2,$3,clock_timestamp()),($4,$2,$5,clock_timestamp())`, uuid(), proposal, f.person, uuid(), f.peer)
	// The real deferred database trigger requires both live consents here.
	exec(`INSERT INTO active_couple_members(person_id,cycle_id) VALUES($1,$3),($2,$3)`, f.person, f.peer, cycle)
	require(t, f.request("POST", path, f.peerToken, map[string]string{"kind": "trip"}), 200)
	require(t, f.request("POST", path, f.token, map[string]string{"kind": "trip"}), 200)
}

func TestPostgresAdminFallbackRequiresOrganizerToHaveLeft(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE memberships SET role='admin' WHERE context_id=$1 AND person_id=$2`, f.room, f.peer); err != nil {
		t.Fatal(err)
	}
	path := "/outings/" + f.outing + "/ending"
	require(t, f.request("POST", path, f.peerToken, map[string]string{"kind": "trip"}), 403)
	if _, err := f.pool.Exec(ctx, `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE context_id=$1 AND person_id=$2`, f.room, f.person); err != nil {
		t.Fatal(err)
	}
	require(t, f.request("POST", path, f.peerToken, map[string]string{"kind": "trip"}), 200)
}

func TestPostgresAccountErasurePurgesBooksVersionsAndJobs(t *testing.T) {
	f := setup(t)
	f.close(t)
	b := f.save(t, 0, "public")
	f.save(t, b.Revision, "private")
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `INSERT INTO outing_diary_jobs(id,outing_id,owner_id,logical_id,session_digest,digest,source,status) VALUES($1,$2,$3,$4,$5,'synthetic','{"excerpts":["synthetic private note"]}','queued')`, uuid(), f.outing, f.person, uuid(), auth.TokenDigest(f.token)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE people SET deleted_at=clock_timestamp() WHERE id=$1`, f.person); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`SELECT count(*) FROM outing_diaries WHERE id=$1`,
		`SELECT count(*) FROM outing_diary_versions WHERE diary_id=$1`,
		`SELECT count(*) FROM outing_diary_photos WHERE diary_id=$1`,
	} {
		var n int
		if err := f.pool.QueryRow(ctx, q, b.ID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("erasure retained diary data: count=%d err=%v", n, err)
		}
	}
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM outing_diary_jobs WHERE owner_id=$1`, f.person).Scan(&n); err != nil || n != 0 {
		t.Fatalf("erasure retained AI source: count=%d err=%v", n, err)
	}
	require(t, f.request("GET", "/diaries/"+b.ID, f.outsiderToken, nil), 404)
	require(t, f.request("PUT", "/outings/"+f.outing+"/diary", f.token, map[string]any{"revision": 0, "audience": "private", "document": f.doc()}), 401)
}
