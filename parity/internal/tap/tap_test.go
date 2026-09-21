package tap

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTheTapForwardsAsReceivedRecordsAndNeverReusesAConnection(t *testing.T) {
	var connections atomic.Int64
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Host", r.Host)
		w.Header().Set("X-Seen-For", strings.Join(r.Header.Values("X-Forwarded-For"), "|"))
		w.Header().Set("X-Seen-Proto", r.Header.Get("X-Forwarded-Proto"))
		_, _ = io.WriteString(w, r.Method+" "+r.RequestURI)
	}))
	upstream.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	upstream.Start()
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)

	rec := &Recorder{}
	front := httptest.NewServer(rec.Proxy(target))
	defer front.Close()
	control := httptest.NewServer(rec.Control())
	defer control.Close()
	client, err := NewClient(control.URL)
	if err != nil {
		t.Fatal(err)
	}

	send := func(method, target string) *http.Response {
		req, _ := http.NewRequest(method, front.URL+target, nil)
		req.Host = "parity.test"
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		req.Header.Set("X-Forwarded-Proto", "http")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	resp := send("GET", "/contexts/a%2Fb?x=1")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "GET /contexts/a%2Fb?x=1" || resp.Header.Get("X-Seen-Host") != "parity.test" ||
		resp.Header.Get("X-Seen-For") != "203.0.113.7" || resp.Header.Get("X-Seen-Proto") != "http" {
		t.Fatalf("forwarded as %q with %v", body, resp.Header)
	}
	resp = send("POST", "/reports")
	resp.Body.Close()

	last, entries, err := client.Since(context.Background(), 0)
	if err != nil || last != 2 || len(entries) != 2 || entries[1].Method != "POST" || entries[1].Target != "/reports" {
		t.Fatalf("since 0: %d %+v %v", last, entries, err)
	}
	last, entries, err = client.Since(context.Background(), 1)
	if err != nil || last != 2 || len(entries) != 1 || entries[0].Seq != 2 {
		t.Fatalf("since 1: %d %+v %v", last, entries, err)
	}
	if last, err := client.Last(context.Background()); err != nil || last != 2 {
		t.Fatalf("last: %d %v", last, err)
	}
	if connections.Load() != 2 {
		t.Fatalf("upstream saw %d connections for 2 requests: keep-alive is on", connections.Load())
	}

	bad, err := http.Get(control.URL + "/entries?after=-1")
	if err != nil || bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative after: %v %v", bad, err)
	}
	bad.Body.Close()
}
