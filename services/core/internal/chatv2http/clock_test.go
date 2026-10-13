package chatv2http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
)

type clockMemoryStore struct {
	memoryStore
	now                         time.Time
	err                         error
	calls                       int
	actor, device, conversation string
	digest                      []byte
}

func (s *clockMemoryStore) Clock(_ context.Context, actor string, digest []byte, device, conversation string) (time.Time, error) {
	s.calls++
	s.actor, s.device, s.conversation = actor, device, conversation
	s.digest = append([]byte(nil), digest...)
	return s.now, s.err
}

func TestClockUsesBearerAndDatabaseSampleOnly(t *testing.T) {
	// repo-guard: allow=long-number reason=synthetic-clock-nanoseconds
	s := &clockMemoryStore{now: time.Date(2026, 10, 14, 9, 8, 7, 123456789, time.FixedZone("synthetic", 7*60*60))}
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true})
	w := request(t, h, http.MethodGet, "/v2/chat/"+conversationID+"/clock?device_id="+deviceID, nil)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	// repo-guard: allow=long-number reason=synthetic-clock-nanoseconds
	if len(out) != 1 || out["server_now"] != "2026-10-14T02:08:07.123456789Z" {
		t.Fatalf("clock response=%v", out)
	}
	if s.calls != 1 || s.actor != personID || s.device != deviceID || s.conversation != conversationID || !bytes.Equal(s.digest, auth.TokenDigest("synthetic-session")) {
		t.Fatalf("wrong permission binding: calls=%d actor=%s device=%s room=%s", s.calls, s.actor, s.device, s.conversation)
	}
	if s.sends != 0 || len(s.events) != 0 {
		t.Fatal("clock used event writes")
	}
}

func TestClockRejectsInvalidWireBeforeStoreRead(t *testing.T) {
	path := "/v2/chat/" + conversationID + "/clock"
	cases := []struct {
		name, path, body string
		status           int
	}{
		{"missing_device", path, "", http.StatusBadRequest},
		{"empty_device", path + "?device_id=", "", http.StatusUnprocessableEntity},
		{"invalid_device", path + "?device_id=bad", "", http.StatusUnprocessableEntity},
		{"uppercase_device", path + "?device_id=" + strings.ToUpper(deviceID), "", http.StatusUnprocessableEntity},
		{"invalid_room", "/v2/chat/bad/clock?device_id=" + deviceID, "", http.StatusUnprocessableEntity},
		{"duplicate_device", path + "?device_id=" + deviceID + "&device_id=" + deviceID, "", http.StatusBadRequest},
		{"history_cursor", path + "?device_id=" + deviceID + "&after=0", "", http.StatusBadRequest},
		{"history_limit", path + "?device_id=" + deviceID + "&limit=1", "", http.StatusBadRequest},
		{"url_bearer", path + "?device_id=" + deviceID + "&access_token=synthetic-session", "", http.StatusBadRequest},
		{"malformed_query", path + "?device_id=" + deviceID + "&broken=%zz", "", http.StatusBadRequest},
		{"query_semicolon", path + "?device_id=" + deviceID + ";after=0", "", http.StatusBadRequest},
		{"json_body", path + "?device_id=" + deviceID, "{}", http.StatusBadRequest},
		{"null_body", path + "?device_id=" + deviceID, "null", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &clockMemoryStore{}
			h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true})
			r := httptest.NewRequest(http.MethodGet, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer synthetic-session")
			if tc.body != "" {
				r.ContentLength = -1
				r.TransferEncoding = []string{"chunked"}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || s.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, s.calls, w.Body.String())
			}
		})
	}
}

func TestClockRequiresBearerEvenWithAnActorAuthenticator(t *testing.T) {
	for _, tc := range []struct {
		name, actor, authorization string
	}{
		{"actor_header_only", personID, ""},
		{"wrong_scheme", personID, "Basic synthetic-session"},
		{"invalid_actor", "invalid", "Bearer synthetic-session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &clockMemoryStore{}
			h := New(Options{Store: s, Experimental: true, Authenticate: func(context.Context, http.Header) (string, error) { return tc.actor, nil }})
			r := httptest.NewRequest(http.MethodGet, "/v2/chat/"+conversationID+"/clock?device_id="+deviceID, nil)
			r.Header.Set("X-Actor-ID", personID)
			if tc.authorization != "" {
				r.Header.Set("Authorization", tc.authorization)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized || s.calls != 0 {
				t.Fatalf("status=%d calls=%d", w.Code, s.calls)
			}
		})
	}
}

func TestClockOptionalCapabilityAndFailures(t *testing.T) {
	path := "/v2/chat/" + conversationID + "/clock?device_id=" + deviceID
	h := New(Options{Store: &memoryStore{}, Authenticate: fakeAuth, Experimental: true})
	if w := request(t, h, http.MethodGet, path, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing capability status=%d", w.Code)
	}
	for _, tc := range []struct {
		err    error
		status int
	}{
		{chatv2.ErrForbidden, http.StatusForbidden},
		{chatv2.ErrNotReady, http.StatusConflict},
		{errors.New("synthetic backend detail"), http.StatusServiceUnavailable},
	} {
		s := &clockMemoryStore{err: tc.err}
		h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true})
		w := request(t, h, http.MethodGet, path, nil)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "server_now") || strings.Contains(w.Body.String(), "backend detail") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
