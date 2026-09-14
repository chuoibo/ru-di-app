package runner

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/scenario"
)

// fakeAPI imitates two routes of the real API. zone controls how it writes
// timestamps, so one instance can play a faithful port and another a broken one.
func fakeAPI(t *testing.T, zone string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	store := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		actor := r.Header.Get("X-Actor-ID")
		switch {
		case r.Method == "POST" && r.URL.Path == "/contexts":
			if actor == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"code":"authentication_required","detail":"Missing X-Actor-ID"}`))
				return
			}
			id := uuid4()
			stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + zone
			body := fmt.Sprintf(`{"id":"%s","created_by_id":"%s","created_at":"%s"}`, id, actor, stamp)
			store[id] = body
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = w.Write([]byte(body))
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/contexts/"):
			body, ok := store[strings.TrimPrefix(r.URL.Path, "/contexts/")]
			w.Header().Set("Content-Type", "application/json")
			if !ok {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"detail":"Not Found"}`))
				return
			}
			_, _ = w.Write([]byte(body))
		default:
			w.WriteHeader(405)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func uuid4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

const script = `
id: fake/create-then-get
routes: ["POST /contexts", "GET /contexts/{context_id}"]
auth_mode: dev
personas: {owner: {}}
steps:
  - id: create
    as: owner
    request: {method: POST, path: /contexts, body_raw: '{"display_name":"x"}'}
    bind: {context_id: {from: body, pointer: /id, class: uuid}}
  - id: read
    as: owner
    request: {method: GET, path: "/contexts/{{context_id}}"}
  - id: anonymous_create
    as: anonymous
    request: {method: POST, path: /contexts}
`

func stack(t *testing.T, name string, server *httptest.Server) Stack {
	client, err := httpclient.New(server.URL, "parity.test")
	if err != nil {
		t.Fatal(err)
	}
	return Stack{Name: name, Client: client}
}

func both(t *testing.T, refZone, candZone string) []StepDiff {
	t.Helper()
	sc, err := scenario.Parse([]byte(script))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := Execute(context.Background(), sc, stack(t, "reference", fakeAPI(t, refZone)))
	if err != nil {
		t.Fatal(err)
	}
	cand, err := Execute(context.Background(), sc, stack(t, "candidate", fakeAPI(t, candZone)))
	if err != nil {
		t.Fatal(err)
	}
	return Diff(ref, cand)
}

func TestFaithfulCandidateHasNoDifferences(t *testing.T) {
	if diffs := both(t, "Z", "Z"); len(diffs) != 0 {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestZoneSpellingDifferenceIsCaughtInEveryStepThatShowsIt(t *testing.T) {
	diffs := both(t, "Z", "+00:00")
	if len(diffs) != 2 || diffs[0].StepID != "create" || diffs[1].StepID != "read" {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestPersonaIDsAreStableAndScoped(t *testing.T) {
	a := PersonaID("contexts/create-then-get", "owner")
	if a != PersonaID("contexts/create-then-get", "owner") {
		t.Fatal("persona id not deterministic")
	}
	if a == PersonaID("contexts/other", "owner") {
		t.Fatal("persona ids leak across scenarios")
	}
	if a[14] != '5' {
		t.Fatalf("not a version-5 uuid: %s", a)
	}
}

func TestPointer(t *testing.T) {
	body := []byte(`{"a":{"b/c":[{"id":"x"},{"n":42}]}}`)
	if v, err := pointer(body, "/a/b~1c/0/id"); err != nil || v != "x" {
		t.Fatalf("got %q %v", v, err)
	}
	if v, err := pointer(body, "/a/b~1c/1/n"); err != nil || v != "42" {
		t.Fatalf("got %q %v", v, err)
	}
	if _, err := pointer(body, "/a/missing"); err == nil {
		t.Fatal("missing key accepted")
	}
}
