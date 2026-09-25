//go:build e2e

// Command brainstub is a deterministic stand-in for the Python brain seam,
// used only by the chat end-to-end tier.
//
// Why a stub rather than the real provider: the AI cases in this tier are about
// the *plumbing* -- consent, durable jobs, lease recovery, atomic promotion,
// who is allowed to see a result. None of that is a question about model
// quality, and all of it becomes untestable if the answer changes run to run or
// if a run needs a paid key. The real provider is exercised separately, by hand,
// and that evidence is recorded as its own run.
//
// The stub answers exactly what `companion.GroundCard` will accept: an
// itinerary whose stops name places from the catalogue the core just sent. It
// never invents a place id, because a stub that bypassed grounding would hide
// the very check the grounding step exists to perform.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// nghinDong is "<n>k", the one money form the chat-expense stub reads.
var nghinDong = regexp.MustCompile(`\b(\d{1,7})k\b`)

func main() {
	listen := os.Getenv("BRAIN_STUB_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8791"
	}
	token := os.Getenv("MOBILE_INTERNAL_TOKEN")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// The core probes capabilities before it will accept an invocation at all,
	// so a stub that only answers companion-reply still reads as "no provider".
	mux.HandleFunc("/internal/brain/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("X-Internal-Token") != token {
			http.Error(w, `{"code":"forbidden"}`, http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plan": map[string]any{"available": true, "reason": nil},
		})
	})
	mux.HandleFunc("/internal/brain/v1/companion-reply", func(w http.ResponseWriter, r *http.Request) {
		// The seam is token-gated in production; refusing here keeps the tier
		// honest about that, instead of proving a door nobody locked.
		if token != "" && r.Header.Get("X-Internal-Token") != token {
			http.Error(w, `{"code":"forbidden"}`, http.StatusForbidden)
			return
		}
		var payload struct {
			Places []map[string]any `json:"places"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&payload); err != nil {
			http.Error(w, `{"code":"bad_request"}`, http.StatusBadRequest)
			return
		}
		ids := []string{}
		for _, place := range payload.Places {
			if id, ok := place["id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
			if len(ids) == 2 {
				break
			}
		}
		if len(ids) == 0 {
			// No catalogue means the core sent nothing groundable. Saying so
			// is more useful than emitting a card that will be refused.
			http.Error(w, `{"code":"no_catalogue"}`, http.StatusUnprocessableEntity)
			return
		}
		stops := make([]map[string]any, 0, len(ids))
		for index, id := range ids {
			stops = append(stops, map[string]any{
				"place_id":  id,
				"time_text": fmt.Sprintf("%02d:00", 8+index*3),
				"note":      "Chặng dựng sẵn cho tầng E2E",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind": "itinerary",
			"payload": map[string]any{
				"title": "Tờ hẹn dựng sẵn cho tầng E2E",
				"stops": stops,
			},
		})
	})
	// chat-expense is the existing skill chia_bill reads each shared message
	// through. The stub reads "<n>k" as n thousand đồng, an integer, and
	// anything else as "not an expense". Like the real skill, it never names a
	// person: who paid is the core's answer, from the message author.
	mux.HandleFunc("/internal/brain/v1/chat-expense", func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("X-Internal-Token") != token {
			http.Error(w, `{"code":"forbidden"}`, http.StatusForbidden)
			return
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&payload); err != nil {
			http.Error(w, `{"code":"brain_request_invalid"}`, http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		m := nghinDong.FindStringSubmatch(payload.Text)
		if m == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"is_expense": false, "title": nil, "amount_vnd": nil, "needs_review": false})
			return
		}
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || n <= 0 || n > 1_000_000 {
			_ = json.NewEncoder(w).Encode(map[string]any{"is_expense": false, "title": nil, "amount_vnd": nil, "needs_review": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"is_expense":   true,
			"title":        "Khoản dựng sẵn cho tầng E2E",
			"amount_vnd":   n * 1000,
			"needs_review": true,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":"not_found"}`, http.StatusNotFound)
	})

	if !strings.HasPrefix(listen, "127.0.0.1:") {
		log.Fatalf("brainstub chỉ nghe trên loopback, nhận %q", listen)
	}
	log.Printf("brainstub nghe trên %s", listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}
