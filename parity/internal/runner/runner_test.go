package runner

import (
	"context"
	"crypto/rand"
	"errors"
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
	ref, err := Execute(context.Background(), sc, stack(t, "reference", fakeAPI(t, refZone)), "t1")
	if err != nil {
		t.Fatal(err)
	}
	cand, err := Execute(context.Background(), sc, stack(t, "candidate", fakeAPI(t, candZone)), "t1")
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

func TestSessionTokensAreDeterministicAndShapedLikeTheRealOnes(t *testing.T) {
	a := SessionToken("w0/prod-sessions", "owner")
	if a != SessionToken("w0/prod-sessions", "owner") {
		t.Fatal("token not deterministic")
	}
	if a == SessionToken("w0/prod-sessions", "other") || a == SessionToken("w0/other", "owner") {
		t.Fatal("tokens collide across personas or scenarios")
	}
	if len(a) != 43 || strings.ContainsAny(a, "+/=") {
		t.Fatalf("token %q is not 43 base64url characters", a)
	}
}

func TestProdPersonasSendABearerAndDevPersonasDoNot(t *testing.T) {
	prod, err := scenario.Parse([]byte(`
id: t/prod
routes: ["GET /people/me"]
auth_mode: prod
personas: {owner: {}}
steps:
  - {id: me, as: owner, request: {method: GET, path: /people/me}}
  - {id: by_hand, as: anonymous, request: {method: GET, path: /people/me, headers: {authorization: "bearer {{token.owner}}"}}}
`))
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{"persona.owner": PersonaID("t/prod", "owner"), "token.owner": SessionToken("t/prod", "owner")}
	req, err := buildRequest(prod, prod.Steps[0], vars)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer "+vars["token.owner"] || req.Header.Get("X-Actor-ID") != "" {
		t.Fatalf("prod headers = %v", req.Header)
	}
	byHand, err := buildRequest(prod, prod.Steps[1], vars)
	if err != nil || byHand.Header.Get("Authorization") != "bearer "+vars["token.owner"] {
		t.Fatalf("hand-written bearer = %v %v", byHand.Header, err)
	}
}

func TestProdPersonasWithoutADatabaseAreASetupFailureNotADifference(t *testing.T) {
	sc, err := scenario.Parse([]byte(`
id: t/prod-no-db
routes: ["GET /people/me"]
auth_mode: prod
personas: {owner: {}}
steps:
  - {id: me, as: owner, request: {method: GET, path: /people/me}}
`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(context.Background(), sc, stack(t, "reference", fakeAPI(t, "Z")), "t1")
	if !errors.Is(err, ErrSetup) {
		t.Fatalf("err = %v, want ErrSetup", err)
	}
}

func TestPersonasRefusedOnlyWhenEveryPersonaStepIs401(t *testing.T) {
	sc, err := scenario.Parse([]byte(`
id: t/refused
routes: ["GET /people/me"]
auth_mode: prod
personas: {owner: {}}
steps:
  - {id: a, as: owner, request: {method: GET, path: /people/me}}
  - {id: b, as: anonymous, request: {method: GET, path: /people/me}}
  - {id: c, as: owner, request: {method: GET, path: /people/me}}
`))
	if err != nil {
		t.Fatal(err)
	}
	run := func(statuses ...int) *Run {
		r := &Run{}
		for _, status := range statuses {
			r.Steps = append(r.Steps, StepResult{Raw: httpclient.Response{Status: status}})
		}
		return r
	}
	if !PersonasRefused(sc, run(401, 200, 401)) {
		t.Fatal("every persona step 401 (anonymous 200) not reported")
	}
	if PersonasRefused(sc, run(401, 401, 200)) {
		t.Fatal("one persona step answered, still reported refused")
	}
	anonymousOnly, err := scenario.Parse([]byte(`
id: t/anonymous
routes: ["GET /people/me"]
auth_mode: prod
steps:
  - {id: a, as: anonymous, request: {method: GET, path: /people/me}}
`))
	if err != nil {
		t.Fatal(err)
	}
	if PersonasRefused(anonymousOnly, run(401)) {
		t.Fatal("a scenario without persona steps reported refused")
	}
}

func TestEveryRunStartsWithPeopleTheStackHasNeverSeen(t *testing.T) {
	sc, err := scenario.Parse([]byte(script))
	if err != nil {
		t.Fatal(err)
	}
	server := fakeAPI(t, "Z")
	first, err := Execute(context.Background(), sc, stack(t, "reference", server), NewNonce())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Execute(context.Background(), sc, stack(t, "reference", server), NewNonce())
	if err != nil {
		t.Fatal(err)
	}
	a, errA := pointer(first.Steps[0].Raw.Body, "/created_by_id")
	b, errB := pointer(second.Steps[0].Raw.Body, "/created_by_id")
	if errA != nil || errB != nil || a == b {
		t.Fatalf("both runs acted as %q / %q (%v %v)", a, b, errA, errB)
	}
	if diffs := Diff(first, second); len(diffs) != 0 {
		t.Fatalf("fresh personas changed the normalised transcript: %+v", diffs)
	}
	if _, err := Execute(context.Background(), sc, stack(t, "reference", server), ""); !errors.Is(err, ErrSetup) {
		t.Fatalf("empty nonce: err = %v", err)
	}
}
