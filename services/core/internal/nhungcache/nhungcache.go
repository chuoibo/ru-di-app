// Package nhungcache is the PostgreSQL cache of document embeddings for
// public content (the place catalogue, the app manual). A rebuild, a new
// index version or a re-run of the ingest pays the provider only for text it
// has not embedded before under the same model, dimensionality and prompt
// version.
//
// It never holds private content: the store is a closed set of the two
// public corpora, checked here and by the table's CHECK constraint, and
// Nếp's memory vectors are embedded without it (their vectors live with
// their owner's row and die with it).
package nhungcache

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mobile/services/core/internal/aiharness/nhung"
)

//go:embed schema_1.sql
var schema1SQL string

var migrations = []struct {
	version int
	sql     string
}{
	{1, schema1SQL},
}

// Querier is a pool or a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Beginner opens the migration's transaction.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Migrate installs the cache: its own checksummed version table under its
// own advisory lock.
func Migrate(ctx context.Context, db Beginner) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('nhung_cache_schema_migration'))`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS nhung_cache_schema_migrations(version integer PRIMARY KEY, digest text NOT NULL)`); err != nil {
		return err
	}
	for _, m := range migrations {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(m.sql)))
		var old string
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM nhung_cache_schema_migrations WHERE version=$1),'')`, m.version).Scan(&old); err != nil {
			return err
		}
		if old != "" {
			if old != digest {
				return fmt.Errorf("nhungcache: migration %d checksum mismatch", m.version)
			}
			continue
		}
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO nhung_cache_schema_migrations VALUES($1,$2)`, m.version, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Kho is a public corpus. The set is closed.
type Kho string

const (
	DiaDiem  Kho = "places"
	HuongDan Kho = "manual"
)

// ErrKhoRieng: a corpus outside the two public ones.
var ErrKhoRieng = errors.New("nhungcache: only public corpora (places, manual) are cached")

func (k Kho) kiem() error {
	if k != DiaDiem && k != HuongDan {
		return fmt.Errorf("%w: %q", ErrKhoRieng, k)
	}
	return nil
}

// Khoa is the cache key: sha256 over the model, the dimensionality, the
// prompt version and the prompt (already NFC and carrying its task prefix,
// nhung.DinhDang), each length-prefixed so no two tuples collide by
// concatenation.
func Khoa(model string, dims int, promptVersion, prompt string) [32]byte {
	h := sha256.New()
	for _, s := range []string{model, fmt.Sprint(dims), promptVersion, prompt} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		h.Write(n[:])
		h.Write([]byte(s))
	}
	var k [32]byte
	copy(k[:], h.Sum(nil))
	return k
}

func maHoa(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func giaiMa(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}

// CoCache embeds public documents through the cache.
type CoCache struct {
	Inner nhung.Nhung
	DB    Querier
	// PromptVersion is nhung.PromptVersion unless a test pins another.
	PromptVersion string

	trung, thieu atomic.Int64
}

// DemTrung and DemThieu are the hit and miss counters.
func (c *CoCache) DemTrung() int64 { return c.trung.Load() }
func (c *CoCache) DemThieu() int64 { return c.thieu.Load() }

func (c *CoCache) pv() string {
	if c.PromptVersion != "" {
		return c.PromptVersion
	}
	return nhung.PromptVersion
}

// NhungCongKhai embeds the documents of a public corpus: hits from the
// cache, misses from the embedder in one call, written back.
func (c *CoCache) NhungCongKhai(ctx context.Context, kho Kho, docs []nhung.TaiLieuVao) ([][]float32, error) {
	if err := kho.kiem(); err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, nhung.ErrRong
	}
	model, dims := c.Inner.Model(), c.Inner.Dims()
	keys := make([][]byte, len(docs))
	for i, d := range docs {
		p, _ := nhung.DinhDang(nhung.TaiLieu, d.TieuDe, d.NoiDung)
		k := Khoa(model, dims, c.pv(), p)
		keys[i] = k[:]
	}
	out := make([][]float32, len(docs))
	rows, err := c.DB.Query(ctx, `SELECT khoa, vec FROM nhung_cache WHERE khoa = ANY($1::bytea[]) AND model=$2 AND dims=$3 AND prompt_version=$4`,
		keys, model, dims, c.pv())
	if err != nil {
		return nil, err
	}
	have := map[string][]float32{}
	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return nil, err
		}
		have[string(k)] = giaiMa(v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var miss []nhung.TaiLieuVao
	var at []int
	for i, k := range keys {
		if v, ok := have[string(k)]; ok && len(v) == dims {
			out[i] = v
			c.trung.Add(1)
			continue
		}
		miss = append(miss, docs[i])
		at = append(at, i)
	}
	if len(miss) == 0 {
		return out, nil
	}
	c.thieu.Add(int64(len(miss)))
	vs, err := c.Inner.NhungTaiLieu(ctx, miss)
	if err != nil {
		return nil, err
	}
	if len(vs) != len(miss) {
		return nil, errors.New("nhungcache: embedder returned a different number of vectors")
	}
	for j, v := range vs {
		if len(v) != dims {
			return nil, nhung.ErrSaiChieu
		}
		i := at[j]
		out[i] = v
		if _, err := c.DB.Exec(ctx, `INSERT INTO nhung_cache(khoa, kho, model, dims, prompt_version, vec) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (khoa) DO NOTHING`,
			keys[i], string(kho), model, dims, c.pv(), maHoa(v)); err != nil {
			return nil, err
		}
	}
	return out, nil
}
