//go:build postgres

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
)

// Exercise the actual serve composition: a configured split producer can
// queue random, but attaching its read ports must never start inference or
// a worker. A keyless producer still refuses before inserting any job.
func TestServeSplitRandomReadinessAndQueueOnly(t *testing.T) {
	for _, provider := range []string{"gemini", "agy only", "keyless"} {
		t.Run(provider, func(t *testing.T) {
			configured := provider != "keyless"
			databaseURL := chatSchemaURL(t)
			migrateChatInto(t, databaseURL)
			ctx := context.Background()
			pool, err := pgxpool.New(ctx, databaseURL)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			var person, trigger string
			if err = pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Synthetic split caller') RETURNING id::text`).Scan(&person); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES($1,'Synthetic split room','group',$2)`, bootRoom, person); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES(gen_random_uuid(),$1,$2,'active','admin','named')`, bootRoom, person); err != nil {
				t.Fatal(err)
			}
			const token = "synthetic-split-producer-token"
			if _, err = pool.Exec(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES(gen_random_uuid(),$1,$2,'genesis',clock_timestamp()+interval '1 hour')`, person, auth.TokenDigest(token)); err != nil {
				t.Fatal(err)
			}
			if err = pool.QueryRow(ctx, `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES(gen_random_uuid(),$1,$2,'text','/random quán ăn Đà Lạt') RETURNING id::text`, bootRoom, person).Scan(&trigger); err != nil {
				t.Fatal(err)
			}
			var modelCalls atomic.Int32
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				modelCalls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				http.Error(w, "a producer must not call the model", http.StatusInternalServerError)
			}))
			defer model.Close()
			python := newPythonStub(t)
			env := map[string]string{"MOBILE_PYTHON_UPSTREAM": python.URL, "MOBILE_DATABASE_URL": databaseURL, EnvInprocWorker: "0"}
			switch provider {
			case "gemini":
				env["GEMINI_API_KEY"], env["MOBILE_GEMINI_BASE_URL"] = "synthetic-key", model.URL
			case "agy only":
				// No Gemini key: queue admission must not build the worker's
				// embedding client or its full execution engine.
				env["AGY_PROXY_URL"], env["AGY_PROXY_KEY"] = model.URL, "synthetic-agy-key"
			}
			core := startCore(t, env)
			client := &http.Client{Timeout: 2 * time.Second}
			request := func(method, path string, body []byte) (int, []byte) {
				t.Helper()
				r, err := http.NewRequest(method, core.base+path, bytes.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(r)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				raw, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				return resp.StatusCode, raw
			}
			status, raw := request("GET", "/contexts/"+bootRoom+"/chat-capabilities", nil)
			want := `"random":{"available":false,"reason":"provider_unavailable"}`
			if configured {
				want = `"random":{"available":true,"reason":null}`
			}
			if status != 200 || !strings.Contains(string(raw), want) {
				t.Fatalf("split readiness: %d %s, want %s", status, raw, want)
			}
			body, err := json.Marshal(map[string]string{"logical_id": "8ea13a29-7dd0-4db0-8874-58ec42fd087b", "command": "random", "prompt": "/random quán ăn Đà Lạt", "trigger_message_id": trigger})
			if err != nil {
				t.Fatal(err)
			}
			status, raw = request("POST", "/contexts/"+bootRoom+"/ai-invocations", body)
			if configured {
				if status != 202 || !strings.Contains(string(raw), `"status":"queued"`) {
					t.Fatalf("configured split producer refused: %d %s", status, raw)
				}
			} else if status != 503 || !strings.Contains(string(raw), `"code":"provider_unavailable"`) {
				t.Fatalf("keyless split producer accepted: %d %s", status, raw)
			}
			// Longer than the 250 ms no-broker poll interval: an accidentally
			// started worker would claim the row and call the model.
			time.Sleep(600 * time.Millisecond)
			var total, untouched, cards, metrics int
			if err = pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE status='queued' AND attempts=0 AND lease_id IS NULL AND result IS NULL AND v2_result IS NULL), (SELECT count(*) FROM messages WHERE kind='ai_card'), (SELECT count(*) FROM ai_turn_metrics) FROM chat_ai_invocations`).Scan(&total, &untouched, &cards, &metrics); err != nil {
				t.Fatal(err)
			}
			wantJobs := 0
			if configured {
				wantJobs = 1
			}
			if total != wantJobs || untouched != wantJobs || cards != 0 || metrics != 0 || modelCalls.Load() != 0 {
				t.Fatalf("producer ran work: jobs=%d untouched=%d cards=%d metrics=%d model calls=%d", total, untouched, cards, metrics, modelCalls.Load())
			}
			if logs := core.logs.String(); !strings.Contains(logs, "AI jobs run in `core work`, not in this process") || strings.Contains(logs, "AI engine runs in this process") {
				t.Fatalf("split worker policy changed: %s", logs)
			}
			if code := core.stop(); code != 0 {
				t.Fatalf("serve stopped with %d", code)
			}
		})
	}
}
