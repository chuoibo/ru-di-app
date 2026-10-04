//go:build postgres

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These boot tests retain production defaults, independently of unrelated
// feature tests that explicitly disable the account module.
func accountBootEnv(t *testing.T) map[string]string {
	t.Helper()
	redis := os.Getenv("CORE_TEST_REDIS_URL")
	if redis == "" {
		t.Fatal("account boot evidence requires real Redis")
	}
	key := func() string {
		var bytes [32]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			t.Fatal(err)
		}
		return base64.StdEncoding.EncodeToString(bytes[:])
	}
	database := chatSchemaURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Shadow every legacy table changed by account migration. Never retire
	// public identities used by other packages in this parallel tier.
	if _, err = pool.Exec(ctx, `CREATE TABLE account_identities (LIKE public.account_identities INCLUDING ALL); CREATE TABLE managed_auth_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"MOBILE_DATABASE_URL": database, "MOBILE_CHAT_CHANGES_CANDIDATE": "0",
		"MOBILE_PYTHON_UPSTREAM":        "http://127.0.0.1:9",
		"MOBILE_ACCOUNT_ENCRYPTION_KEY": key(), "MOBILE_ACCOUNT_LOOKUP_KEY": key(),
		"MOBILE_AUTH_REDIS_URL":  redis,
		"MOBILE_EMAIL_SMTP_HOST": "127.0.0.1", "MOBILE_EMAIL_SMTP_PORT": "9",
		"MOBILE_EMAIL_SMTP_USER": "synthetic-fixture", "MOBILE_EMAIL_SMTP_PASSWORD": "synthetic-fixture",
		// repo-guard: allow=email reason=synthetic-reserved-test-domain
		"MOBILE_EMAIL_FROM": "fixture@example.test",
	}
}

func TestAccountProductionBootRefusesIncompleteConfig(t *testing.T) {
	for _, c := range []struct{ name, key, value, want string }{
		{"missing encryption", "MOBILE_ACCOUNT_ENCRYPTION_KEY", "", "two distinct"},
		{"Redis unavailable", "MOBILE_AUTH_REDIS_URL", "redis://127.0.0.1:1/0", "Redis unavailable"},
		{"SMTP missing", "MOBILE_EMAIL_SMTP_PASSWORD", "", "TLS SMTP"},
		{"schema missing", "", "", "migrate-accounts"},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := accountBootEnv(t)
			if c.key != "" {
				env[c.key] = c.value
			}
			listen, live := freeAddresses(t)
			env["MOBILE_CORE_LISTEN"], env["MOBILE_CORE_LIVENESS_LISTEN"] = listen, live
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			logs := &lockedBuffer{}
			if code := serveUntil(ctx, func(k string) string { return env[k] }, logs); code != 1 || !strings.Contains(logs.String(), c.want) {
				t.Fatalf("production boot did not refuse %s: code %d, %s", c.name, code, logs.String())
			}
			if accepts(listen) || live200(live) {
				t.Fatal("incomplete auth boot accepted traffic")
			}
		})
	}
}

func TestAccountProductionBootMountsDefaultAndRetiresPhone(t *testing.T) {
	env := accountBootEnv(t)
	logs := &lockedBuffer{}
	if code := run([]string{"migrate-accounts"}, func(k string) string { return env[k] }, io.Discard, logs); code != 0 {
		t.Fatalf("migration exit %d: %s", code, logs.String())
	}
	// An empty flag uses the real production default, overriding startCore's
	// feature-test isolation. No dev authentication mode is supplied.
	env["MOBILE_ACCOUNT_AUTH_ENABLED"] = ""
	core := startCore(t, env)
	response, err := http.Post(core.base+"/auth/login", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatalf("managed login not mounted: HTTP %d", response.StatusCode)
	}
	for _, path := range []string{"/auth/otp/request", "/identity/person-id", "/sessions", "/moi"} {
		response, err := http.Post(core.base+path, "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 410 {
			t.Fatalf("retired path %s: HTTP %d", path, response.StatusCode)
		}
	}
	if code := core.stop(); code != 0 {
		t.Fatalf("clean stop exit %d: %s", code, core.logs.String())
	}
}
