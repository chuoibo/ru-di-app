package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mobile/parity/internal/httpclient"

	"mobile/parity/internal/scenario"
)

func TestFilterAuthNeedsTheStacksModeAndCountsWhatItLeavesOut(t *testing.T) {
	all := []*scenario.Scenario{{ID: "a", AuthMode: "dev"}, {ID: "b", AuthMode: "prod"}, {ID: "c", AuthMode: "dev"}}
	for _, mode := range []string{"", "maybe"} {
		if _, err := filterAuth(all, mode, &bytes.Buffer{}); err == nil {
			t.Fatalf("mode %q accepted", mode)
		}
	}
	var out bytes.Buffer
	kept, err := filterAuth(all, "dev", &out)
	if err != nil || len(kept) != 2 || kept[0].ID != "a" || kept[1].ID != "c" {
		t.Fatalf("kept = %v, err = %v", kept, err)
	}
	if !strings.Contains(out.String(), "2 scenario(s) for this mode, 1 left for the other") {
		t.Fatalf("output = %q", out.String())
	}
	if _, err := filterAuth(all[:1], "prod", &bytes.Buffer{}); err == nil {
		t.Fatal("a mode with no scenario ran green")
	}
}

func TestAuthModeCheckTellsDevFromProdStacks(t *testing.T) {
	stack := func(prod bool) *httpclient.Client {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path != "/people/me":
				w.WriteHeader(404)
			case prod && r.Header.Get("Authorization") == "":
				w.WriteHeader(401)
			case !prod && r.Header.Get("X-Actor-ID") != "":
				w.WriteHeader(200)
			default:
				w.WriteHeader(401)
			}
		}))
		t.Cleanup(server.Close)
		client, err := httpclient.New(server.URL, "parity.test")
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	for _, tc := range []struct {
		prod bool
		mode string
		ok   bool
	}{{false, "dev", true}, {true, "prod", true}, {false, "prod", false}, {true, "dev", false}} {
		err := checkAuthMode(context.Background(), "reference", stack(tc.prod), tc.mode)
		if (err == nil) != tc.ok {
			t.Fatalf("prod stack %v, --auth %s: err = %v", tc.prod, tc.mode, err)
		}
	}
}
