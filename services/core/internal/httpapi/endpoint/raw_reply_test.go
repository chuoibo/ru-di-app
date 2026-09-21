package endpoint

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyval"
	guestweb "mobile/services/core/internal/web/guest"
	"mobile/services/core/ownership"
)

var guestToken = strings.Repeat("t", 43)

// frontFor serves one route from Go through dispatch, as front does for
// POST /reports.
func frontFor(t *testing.T, id string, status int, serve Serve) http.Handler {
	t.Helper()
	ir, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	route, err := ir.Bind(id, pyval.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(route, status, serve, Env{Mode: ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(nil) }, Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	routes, err := router.New(manifest.Routes)
	if err != nil {
		t.Fatal(err)
	}
	var served []ownership.Route
	for _, row := range manifest.Routes {
		if row.ID == id {
			served = append(served, row)
		}
	}
	h, err := dispatch.New(dispatch.Options{
		Router: routes, Served: served, Handlers: map[string]http.Handler{id: handler},
		Python: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("%s %s went to Python", r.Method, r.RequestURI)
		}),
		CORS: cors.New("", false), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: func(next http.Handler) http.Handler { return next },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func exchange(h http.Handler, method, target, host string, headers map[string]string, body string) *http.Response {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if v := recover(); v != nil && v != http.ErrAbortHandler {
				panic(v)
			}
		}()
		h.ServeHTTP(rec, req)
	}()
	return rec.Result()
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func requireGuestHeaders(t *testing.T, what string, header http.Header) {
	t.Helper()
	for _, pair := range [][2]string{{"Cache-Control", "no-store"}, {"Referrer-Policy", "no-referrer"}, {"X-Robots-Tag", "noindex, nofollow"}} {
		if got := header.Values(pair[0]); len(got) != 1 || got[0] != pair[1] {
			t.Fatalf("%s: %s is %v", what, pair[0], got)
		}
	}
}

// A redirect goes out as Starlette frames it: no body, content-length 0,
// location, and no content-type; an HTML page with its bytes, length and
// media type. Either way the route's declared status does not apply.
func TestARawReplyIsSentAsItsRouteFramedIt(t *testing.T) {
	target := "/g/" + guestToken + "/xin-cach-tinh"
	redirect := guestweb.SeeOther(guestweb.EvidenceRequestedURL(guestToken, "{x}"))
	h := frontFor(t, "POST /g/{token}/xin-cach-tinh", 200, func(context.Context, *Call) (Reply, error) {
		return Reply{Raw: &redirect}, nil
	})
	response := exchange(h, "POST", target, "parity.test", map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, "obligation_id=x")
	body := readBody(t, response)
	wantLocation := "/g/" + guestToken + "/doi-so-tien?obligation_id=%7Bx%7D"
	if response.StatusCode != 303 || body != "" {
		t.Fatalf("redirect: %d %q", response.StatusCode, body)
	}
	if got := response.Header.Values("Content-Length"); len(got) != 1 || got[0] != "0" {
		t.Fatalf("redirect Content-Length %v", got)
	}
	if got := response.Header.Values("Location"); len(got) != 1 || got[0] != wantLocation {
		t.Fatalf("redirect Location %v", got)
	}
	if _, typed := response.Header["Content-Type"]; typed {
		t.Fatalf("redirect carries Content-Type %v", response.Header["Content-Type"])
	}
	requireGuestHeaders(t, "redirect", response.Header)

	view := map[string]any{"claimed_person_display_name": "A <b>", "recorded_by_display_name": "B", "already_reported": false, "can_object": true}
	page, err := guestweb.NotMePage(view, guestToken)
	if err != nil {
		t.Fatal(err)
	}
	h = frontFor(t, "GET /g/{token}/khong-phai-toi", 201, func(context.Context, *Call) (Reply, error) {
		return Reply{Raw: &page, Status: 418}, nil
	})
	response = exchange(h, "GET", "/g/"+guestToken+"/khong-phai-toi", "parity.test", nil, "")
	body = readBody(t, response)
	if response.StatusCode != 200 || body != string(page.Body) || !strings.Contains(body, "A &lt;b&gt;") {
		t.Fatalf("page: %d %q", response.StatusCode, body)
	}
	if response.Header.Get("Content-Length") != strconv.Itoa(len(page.Body)) || response.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("page headers %v", response.Header)
	}
	requireGuestHeaders(t, "page", response.Header)
}

// api_problem_handler answers guest_link_not_found with the broken-link page
// only when request.url.path is a guest path, and that path comes from the
// Host header: "x/y" moves it off /g, "[" makes urlsplit raise.
func TestGuestLinkNotFoundIsThePageOnlyWhereRequestURLPathIsAGuestPath(t *testing.T) {
	notFound := func(context.Context, *Call) (Reply, error) {
		return Reply{}, Refuse(404, "guest_link_not_found", "Guest link does not exist")
	}
	page, err := guestweb.LinkBrokenPage()
	if err != nil {
		t.Fatal(err)
	}
	target := "/g/" + guestToken + "?obligation_id=x"
	h := frontFor(t, "GET /g/{token}", 200, notFound)

	response := exchange(h, "GET", target, "parity.test", nil, "")
	body := readBody(t, response)
	if response.StatusCode != 404 || body != string(page.Body) || strings.Contains(body, guestToken) {
		t.Fatalf("page: %d %q", response.StatusCode, body)
	}
	if response.Header.Get("Content-Type") != "text/html; charset=utf-8" || response.Header.Get("Content-Length") != strconv.Itoa(len(page.Body)) {
		t.Fatalf("page headers %v", response.Header)
	}
	requireGuestHeaders(t, "page", response.Header)

	const problemJSON = `{"code":"guest_link_not_found","detail":"Guest link does not exist"}`
	response = exchange(h, "GET", target, "x/y", nil, "")
	if body := readBody(t, response); response.StatusCode != 404 || body != problemJSON ||
		response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Host x/y: %d %q %v", response.StatusCode, body, response.Header)
	}
	requireGuestHeaders(t, "Host x/y", response.Header)

	response = exchange(h, "GET", target, "[", nil, "")
	if body := readBody(t, response); response.StatusCode != 500 || body != "Internal Server Error" {
		t.Fatalf("Host [: %d %q", response.StatusCode, body)
	}
	requireGuestHeaders(t, "Host [", response.Header)

	other := frontFor(t, "GET /g/{token}", 200, func(context.Context, *Call) (Reply, error) {
		return Reply{}, Refuse(404, "guest_obligation_not_found", "Obligation is outside this link")
	})
	response = exchange(other, "GET", target, "[", nil, "")
	if body := readBody(t, response); response.StatusCode != 404 || body != `{"code":"guest_obligation_not_found","detail":"Obligation is outside this link"}` {
		t.Fatalf("another code never reads request.url: %d %q", response.StatusCode, body)
	}

	valid := `{"target_type":"post","target_id":"` + targetID + `","reason":"other","note":null}`
	response = exchange(frontFor(t, "POST /reports", 201, notFound), "POST", "/reports", "parity.test", signedIn, valid)
	if body := readBody(t, response); response.StatusCode != 404 || body != problemJSON {
		t.Fatalf("off /g: %d %q", response.StatusCode, body)
	}
}
