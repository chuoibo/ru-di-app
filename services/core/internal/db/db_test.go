package db

import (
	"context"
	"strings"
	"testing"
)

func TestPoolConfigAcceptsTheSQLAlchemySpelling(t *testing.T) {
	config, err := PoolConfig(" postgresql+psycopg://mobile:secret@dbhost:5432/mobile ")
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.Host != "dbhost" || config.ConnConfig.Port != 5432 ||
		config.ConnConfig.Database != "mobile" || config.ConnConfig.User != "mobile" {
		t.Fatalf("parsed %+v", config.ConnConfig)
	}
	if config.MaxConns != maxConns {
		t.Fatalf("MaxConns = %d", config.MaxConns)
	}
}

func TestPoolConfigRefusals(t *testing.T) {
	for _, raw := range []string{"", "   ", "mysql://u:p@h/db", "sqlite:///tmp/x.db"} {
		if _, err := PoolConfig(raw); err == nil {
			t.Errorf("PoolConfig(%q) accepted", raw)
		}
	}
}

func TestPoolConfigNeverEchoesThePassword(t *testing.T) {
	_, err := PoolConfig("postgresql://mobile:hunter-secret@[bad-host/mobile")
	if err == nil {
		t.Fatal("malformed URL accepted")
	}
	if strings.Contains(err.Error(), "hunter-secret") {
		t.Fatalf("error leaks the password: %v", err)
	}
}

func TestUnitWithoutQueriesCommitsNothing(t *testing.T) {
	unit := NewUnit(nil)
	if unit.Begun() {
		t.Fatal("new unit reports begun")
	}
	if err := unit.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := unit.Rollback(context.Background()); err != nil {
		t.Fatal("rollback after commit must be a no-op")
	}
	if _, err := unit.Tx(context.Background()); err != ErrUnitFinished {
		t.Fatalf("Tx after commit = %v", err)
	}
}

func TestMaxConnsCanBeSetPerProcess(t *testing.T) {
	t.Setenv(EnvMaxConns, "3")
	config, err := PoolConfig("postgresql://u:p@h:5432/d")
	if err != nil || config.MaxConns != 3 {
		t.Fatalf("MaxConns = %v, %v; want 3", config.MaxConns, err)
	}
	t.Setenv(EnvMaxConns, "nonsense")
	config, _ = PoolConfig("postgresql://u:p@h:5432/d")
	if config.MaxConns != maxConns {
		t.Errorf("a bad value must fall back to %d, got %d", maxConns, config.MaxConns)
	}
}
