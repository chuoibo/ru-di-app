//go:build postgres

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCommunityRefusesToServeBeforeMigration(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Shadow any community schema installed by another package in this tier.
	if _, err = pool.Exec(ctx, `CREATE TABLE community_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	listen, live := freeAddresses(t)
	env := map[string]string{
		"MOBILE_CORE_LISTEN": listen, "MOBILE_CORE_LIVENESS_LISTEN": live,
		"MOBILE_PYTHON_UPSTREAM": "http://127.0.0.1:1", "MOBILE_DATABASE_URL": databaseURL,
		"MOBILE_AUTH_MODE": "prod", "MOBILE_COMMUNITY_ENABLED": "1", "MOBILE_CHAT_CHANGES_CANDIDATE": "0",
	}
	logs := &lockedBuffer{}
	if code := serveUntil(ctx, func(k string) string { return env[k] }, logs); code != 1 {
		t.Fatalf("incomplete community schema served traffic: exit %d, %s", code, logs.String())
	}
	if !strings.Contains(logs.String(), "refusing to start") || !strings.Contains(logs.String(), "migrate-community") {
		t.Fatal("startup refusal did not identify the missing migration:", logs.String())
	}
}
