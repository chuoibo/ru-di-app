// Package vectordb is core's only door to Milvus: the connection, the
// collection schemas as code, the hard-constraint filters, versioned
// physical collections behind aliases, deletes that are compacted away, and
// the per-person memory store. Go is the only writer of every collection
// (research sdlc-production §C); Python inference services never hold a
// Milvus credential.
//
// Milvus is a derived, rebuildable index. PostgreSQL stays the source of
// truth: every hit is re-checked against its live row by the caller
// (hybrid), so a stale index can lose a result but never show one that
// breaks a hard constraint.
//
// Rules this package keeps, each with a test:
//   - The Go SDK's client telemetry is on by default and sends error strings
//     (which may hold a filter, and so an id) to the server; the ClientConfig
//     built here always carries a non-nil TelemetryConfig{Enabled:false}
//     (research milvus.md §Kiểm chứng 4). There is one constructor.
//   - A filter expression is built only from this package's constant
//     fragments; every value reaches Milvus as a template parameter
//     (loc.go), never spliced into the expression.
//   - A physical collection is never named like an alias: Milvus resolves a
//     collection name before an alias, so an alias shadowed by a collection
//     would silently stop moving (research sdlc-production §B1).
//   - The memory collection holds no text: ids, owner, kind, time and
//     vectors only, the owner a partition key and always a filter.
package vectordb

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/milvus-io/milvus/client/v3/milvusclient"
)

// Environment variables the connection is read from.
const (
	EnvAddr     = "MOBILE_MILVUS_ADDR"
	EnvUser     = "MOBILE_MILVUS_USER"
	EnvPassword = "MOBILE_MILVUS_PASSWORD"
	// EnvPasswordFile names a mounted secret holding the password; it is
	// read when EnvPassword is empty.
	EnvPasswordFile = "MOBILE_MILVUS_PASSWORD_FILE"
	EnvDB           = "MOBILE_MILVUS_DB"
)

// Config is a Milvus connection. Auth is always on in every deployment
// (the local instance rejects anonymous and root:Milvus); a Config without a
// user is refused.
type Config struct {
	Addr     string
	User     string
	Password string
	DB       string
}

// ErrChuaCauHinh: no address, or no credentials.
var ErrChuaCauHinh = errors.New("vectordb: Milvus address or credentials not configured")

// FromEnv reads the connection from the environment: the password from
// EnvPassword, or from the file EnvPasswordFile names.
func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:     strings.TrimSpace(getenv(EnvAddr)),
		User:     strings.TrimSpace(getenv(EnvUser)),
		Password: getenv(EnvPassword),
		DB:       strings.TrimSpace(getenv(EnvDB)),
	}
	if f := strings.TrimSpace(getenv(EnvPasswordFile)); c.Password == "" && f != "" {
		raw, err := os.ReadFile(f)
		if err != nil {
			return Config{}, errors.New("vectordb: the Milvus password file is unreadable")
		}
		c.Password = strings.TrimSpace(string(raw))
	}
	if c.Addr == "" || c.User == "" || c.Password == "" {
		return Config{}, ErrChuaCauHinh
	}
	return c, nil
}

// clientConfig is the one place a milvusclient.ClientConfig is built. The
// telemetry pointer is never nil: a nil pointer falls back to the SDK's
// default, which is on.
func clientConfig(c Config) *milvusclient.ClientConfig {
	return &milvusclient.ClientConfig{
		Address:         c.Addr,
		Username:        c.User,
		Password:        c.Password,
		DBName:          c.DB,
		TelemetryConfig: &milvusclient.TelemetryConfig{Enabled: false},
	}
}

// Ket connects. The caller closes the returned client with Dong.
func Ket(ctx context.Context, c Config) (*Milvus, error) {
	if c.Addr == "" || c.User == "" || c.Password == "" {
		return nil, ErrChuaCauHinh
	}
	cli, err := milvusclient.New(ctx, clientConfig(c))
	if err != nil {
		return nil, err
	}
	return &Milvus{cli: cli}, nil
}
