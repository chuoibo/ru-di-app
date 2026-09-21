package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"mobile/parity/internal/scenario"
)

const viaScript = `
id: fake/cross-replay
routes: ["POST /things"]
auth_mode: dev
personas: {owner: {}}
steps:
  - id: python_writes
    as: owner
    via: python
    request: {method: POST, path: /things, headers: {idempotency-key: k}, body_raw: '{}'}
  - id: front_door_replays
    as: owner
    request: {method: POST, path: /things, headers: {idempotency-key: k}, body_raw: '{}'}
  - id: python_replays
    as: owner
    via: python
    request: {method: POST, path: /things, headers: {idempotency-key: k}, body_raw: '{}'}
`

func countingServer(t *testing.T, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestViaPythonStepsBypassTheFrontDoor(t *testing.T) {
	sc, err := scenario.Parse([]byte(viaScript))
	if err != nil {
		t.Fatal(err)
	}
	var frontDoor, python atomic.Int64
	candidate := stack(t, "candidate", countingServer(t, &frontDoor))
	candidate.Python = stack(t, "python", countingServer(t, &python)).Client
	if _, err := Execute(context.Background(), sc, candidate, "t1"); err != nil {
		t.Fatal(err)
	}
	if frontDoor.Load() != 1 || python.Load() != 2 {
		t.Fatalf("front door got %d requests and Python %d; want 1 and 2", frontDoor.Load(), python.Load())
	}
}

func TestAViaPythonStepWithoutAPythonClientRefusesBeforeAnyStep(t *testing.T) {
	sc, err := scenario.Parse([]byte(viaScript))
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int64
	_, err = Execute(context.Background(), sc, stack(t, "candidate", countingServer(t, &hits)), "t1")
	if !errors.Is(err, ErrSetup) {
		t.Fatalf("err = %v, want a setup failure", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests ran before the refusal", hits.Load())
	}
}
