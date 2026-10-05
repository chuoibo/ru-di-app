//go:build postgres

package dieuchinh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
)

func (w *world) session(t *testing.T, person string) string {
	t.Helper()
	token, err := mintToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(context.Background(), `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES(gen_random_uuid(),$1,$2,'genesis',now()+interval '1 hour')`,
		person, auth.TokenDigest(token)); err != nil {
		t.Fatal(err)
	}
	return token
}

type served struct {
	t       *testing.T
	handler http.Handler
	passed  []string
}

func (s *served) do(method, path, bearer, contentType, body string) *httptest.ResponseRecorder {
	s.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, r)
	return rec
}

func newServed(t *testing.T, w *world) *served {
	s := &served{t: t}
	next := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.passed = append(s.passed, r.Method+" "+r.URL.Path)
		rw.WriteHeader(299)
	})
	h := New(w.pool, "prod")
	s.handler = h.Wrap(next, func(next http.Handler) http.Handler { return next })
	return s
}

// The whole round over HTTP: the proposer's answer carries the review links;
// a member reads the amendment, a stranger cannot tell the batch exists; each
// guest's page shows their own lines and nobody else's; their answers apply
// it; the old obligation then refuses a receipt with its successor's id, and
// a receipt confirmed on it before counts on the board's successor.
func TestAmendmentsOverHTTP(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	s := newServed(t, w)
	an, dung := w.session(t, w.an), w.session(t, w.dung)

	var dungOld, binhOld string
	if err := w.pool.QueryRow(ctx, `SELECT o.id::text FROM collection_obligations o JOIN collection_batch_versions v ON v.id=o.batch_version_id WHERE v.batch_id=$1::uuid AND o.sender_id=$2::uuid`, w.batch, w.dung).Scan(&dungOld); err != nil {
		t.Fatal(err)
	}
	if err := w.pool.QueryRow(ctx, `SELECT o.id::text FROM collection_obligations o JOIN collection_batch_versions v ON v.id=o.batch_version_id WHERE v.batch_id=$1::uuid AND o.sender_id=$2::uuid`, w.batch, w.binh).Scan(&binhOld); err != nil {
		t.Fatal(err)
	}
	// An confirms Dũng's 50 000 before anything changes.
	if _, err := w.pool.Exec(ctx, `INSERT INTO receipt_confirmations(id,obligation_id,confirmed_by_id,amount_vnd,idempotency_key,confirmed_at) VALUES(gen_random_uuid(),$1,$2,50000,gen_random_uuid(),now())`, dungOld, w.an); err != nil {
		t.Fatal(err)
	}

	path := "/batches/" + w.batch + "/amendments"
	body := `{"expense_id":"` + w.firstExpense + `","reason":"Bình gọi thêm <b>món</b>","allocations":{"` + w.an + `":100000,"` + w.binh + `":150000,"` + w.chi + `":50000}}`
	if rec := s.do("POST", path, an, "application/json", strings.Replace(body, "150000", "150000.5", 1)); rec.Code != 422 {
		t.Fatalf("a fraction of a đồng: %d %s", rec.Code, rec.Body)
	}
	if rec := s.do("POST", path, "", "application/json", body); rec.Code != 401 {
		t.Fatalf("no bearer: %d", rec.Code)
	}
	rec := s.do("POST", path, an, "application/json", body)
	if rec.Code != 201 {
		t.Fatalf("propose: %d %s", rec.Code, rec.Body)
	}
	var proposed Proposed
	if err := json.Unmarshal(rec.Body.Bytes(), &proposed); err != nil || len(proposed.Links) != 2 {
		t.Fatalf("proposal: %s", rec.Body)
	}
	binhPage, chiPage := "", ""
	for _, l := range proposed.Links {
		if l.SenderID == w.binh {
			binhPage = l.Path
		} else {
			chiPage = l.Path
		}
	}

	if rec := s.do("GET", path, dung, "", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"pending_person_ids"`) {
		t.Fatalf("a member's list: %d %s", rec.Code, rec.Body)
	}
	var stranger string
	_ = w.pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Người lạ (dữ liệu mẫu)') RETURNING id::text`).Scan(&stranger)
	if rec := s.do("GET", path, w.session(t, stranger), "", ""); rec.Code != 404 {
		t.Fatalf("a stranger's list: %d", rec.Code)
	}

	page := s.do("GET", binhPage, "", "", "")
	text := page.Body.String()
	if page.Code != 200 || !strings.Contains(text, "100.000đ") || !strings.Contains(text, "150.000đ") {
		t.Fatalf("Bình's page: %d %s", page.Code, text)
	}
	if strings.Contains(text, ">50.000đ") || strings.Contains(text, "Chi (dữ liệu mẫu)") || strings.Count(text, "class=\"field\"") != 1 {
		t.Fatalf("Bình's page shows Chi's line: %s", text)
	}
	if strings.Contains(text, "<b>món</b>") || !strings.Contains(text, "&lt;b&gt;món&lt;/b&gt;") {
		t.Fatalf("the reason is not escaped: %s", text)
	}
	if page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("guest privacy headers: %v", page.Header())
	}
	form := url.Values{"tra_loi": {"dong_y"}}.Encode()
	if rec := s.do("POST", binhPage, "", "application/x-www-form-urlencoded", form); rec.Code != 303 {
		t.Fatalf("Bình answers: %d", rec.Code)
	}
	if rec := s.do("POST", "/batches/"+w.batch+"/amendments/"+proposed.AmendmentID+"/decision", dung, "application/json", `{"accept":true}`); rec.Code != 403 {
		t.Fatalf("Dũng answered for nobody: %d %s", rec.Code, rec.Body)
	}
	if rec := s.do("POST", chiPage, "", "application/x-www-form-urlencoded", form); rec.Code != 303 {
		t.Fatalf("Chi answers: %d", rec.Code)
	}
	if rec := s.do("GET", chiPage, "", "", ""); rec.Code != 303 || rec.Header().Get("Location") != strings.TrimSuffix(chiPage, "/dieu-chinh") {
		t.Fatalf("an applied review page: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// Without a member's bearer, nothing is said about the replacement: the
	// route answers as it always has (security review 2026-10-05).
	for _, bearer := range []string{"", w.session(t, stranger)} {
		if rec := s.do("POST", "/obligations/"+binhOld+"/confirm-receipt", bearer, "application/json", `{}`); rec.Code != 299 {
			t.Fatalf("a non-member learned of the replacement: %d %s", rec.Code, rec.Body)
		}
	}
	rec = s.do("POST", "/obligations/"+binhOld+"/confirm-receipt", an, "application/json", `{}`)
	var superseded map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &superseded)
	if rec.Code != 409 || superseded["code"] != "obligation_superseded" || superseded["successor_obligation_id"] == "" {
		t.Fatalf("a receipt on a replaced obligation: %d %s", rec.Code, rec.Body)
	}
	if rec := s.do("POST", "/obligations/"+superseded["successor_obligation_id"]+"/confirm-receipt", an, "application/json", `{}`); rec.Code != 299 {
		t.Fatalf("the successor's receipt did not reach the route: %d", rec.Code)
	}

	board := func(ctx context.Context) map[string]string {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		b, err := (repo.Repository{Q: tx}).ListBatchObligations(ctx, w.batch)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, o := range b.Obligations {
			out[o.SenderID] = o.Status
		}
		return out
	}
	if got := board(repo.WithSuccessions(ctx))[w.dung]; got != "confirmed" {
		t.Fatalf("Dũng's receipt did not follow to the successor: %s", got)
	}
	if got := board(ctx)[w.dung]; got != "outstanding" {
		t.Fatalf("unmarked, the board is not Python's statement: %s", got)
	}
}

// A review token lives no longer than the guest link it replaced (security
// review 2026-10-05): past that expiry its page and its answer are gone.
func TestAReviewLinkEndsWithTheLinkItReplaced(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	if _, err := w.pool.Exec(ctx, `UPDATE guest_links SET expires_at = $2 WHERE token_digest = $1`, auth.TokenDigest(w.tokens[w.chi]), w.now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := w.store(w.now.Add(time.Minute)).Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Chia lại",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 150_000, w.chi: 50_000}})
	if err != nil {
		t.Fatal(err)
	}
	chi := tokenOf(t, got.Links, w.chi)
	h := New(w.pool, "prod")
	h.Store.Now = func() time.Time { return w.now.Add(3 * time.Hour) }
	s := &served{t: t, handler: h.Wrap(http.NotFoundHandler(), func(n http.Handler) http.Handler { return n })}
	if rec := s.do("GET", "/g/"+chi+"/dieu-chinh", "", "", ""); rec.Code != 404 {
		t.Fatalf("an expired review page: %d", rec.Code)
	}
	var expires time.Time
	_ = w.pool.QueryRow(ctx, `SELECT expires_at FROM collection_amendment_links WHERE token_digest=$1`, auth.TokenDigest(chi)).Scan(&expires)
	if !expires.Equal(w.now.Add(2 * time.Hour)) {
		t.Fatalf("the review token outlives its link: %v", expires)
	}
}
