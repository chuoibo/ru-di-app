// Package testmilvus connects live Milvus tests (-tags milvus) to the server
// scripts/go_milvus_tier.sh provides.
//
// Without MOBILE_TEST_MILVUS_ADDR a test skips, unless the tier set
// CORE_REQUIRE_MILVUS_TESTS=1, in which case a missing server fails it. A
// skip is not a pass (CLAUDE.md), and the tier refuses to be green on skips.
package testmilvus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/milvus-io/milvus/client/v3/entity"

	"mobile/services/core/internal/vectordb"
)

// Environment the tier sets.
const (
	EnvAddr     = "MOBILE_TEST_MILVUS_ADDR"
	EnvUser     = "MOBILE_TEST_MILVUS_USER"
	EnvPassword = "MOBILE_TEST_MILVUS_PASSWORD"
	EnvRequire  = "CORE_REQUIRE_MILVUS_TESTS"
)

// Config is the test server's connection.
func Config(t *testing.T) vectordb.Config {
	t.Helper()
	addr := strings.TrimSpace(os.Getenv(EnvAddr))
	if addr == "" {
		if os.Getenv(EnvRequire) == "1" {
			t.Fatal(EnvRequire + "=1 but " + EnvAddr + " is empty")
		}
		t.Skip(EnvAddr + " not set; run scripts/go_milvus_tier.sh")
	}
	user := strings.TrimSpace(os.Getenv(EnvUser))
	if user == "" {
		user = "root"
	}
	return vectordb.Config{Addr: addr, User: user, Password: os.Getenv(EnvPassword)}
}

// TienTo is a fresh collection prefix: a letter and eight hex digits.
func TienTo() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "t" + hex.EncodeToString(b) + "_"
}

// Ket connects with a fresh prefix and Strong reads, and drops everything
// under the prefix when the test ends.
func Ket(t *testing.T) *vectordb.Milvus {
	t.Helper()
	cfg := Config(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m, err := vectordb.Ket(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m.TienTo = TienTo()
	m.NhatQuan = entity.ClStrong
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := m.DonTienTo(ctx); err != nil {
			t.Errorf("cleanup of %s*: %v", m.TienTo, err)
		}
		_ = m.Dong(ctx)
	})
	return m
}
