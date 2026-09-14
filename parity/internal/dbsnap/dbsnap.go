// Package dbsnap records what a scenario step did to a stack's database, so
// the parity harness can compare mechanism (rows written, updated, deleted)
// and not only HTTP output.
//
// Per stack: Snapshot once before the first step and again after every step;
// Delta(previous, next) is that step's Change. The runner observes
// Change.Texts() with the same normalize.Binder it uses for that stack's
// responses (response first, then the change), and once every step has been
// observed it calls Change.Normalise(binder.Apply) and Compare(reference,
// candidate).
//
// # How values render
//
// Row.Raw is row_to_json exactly as PostgreSQL writes it, inside a READ ONLY,
// REPEATABLE READ transaction whose settings are pinned with SET LOCAL
// (TimeZone UTC, bytea_output hex, DateStyle ISO, IntervalStyle postgres,
// extra_float_digits 1), so server or connection defaults cannot make two
// stacks render the same data differently. Nothing is written: append-only
// tables with BEFORE UPDATE/DELETE triggers are safe to snapshot.
//
// Row.Text is Raw after two lossless, type-directed rewrites of top-level
// columns (types come from the catalog, never from guessing at strings):
//
//   - timestamptz and timestamp values get their fraction padded to six
//     digits. PostgreSQL trims trailing zeros (120000 µs renders as ".12"),
//     which would make the normalize shape (f2 vs f6) depend on the clock.
//     Every database instant is therefore f6, and "+00:00" in UTC.
//   - bytea columns named in Options.JSONByteaColumns (by default
//     idempotency_keys.response_body) are decoded: bytes that are valid UTF-8
//     JSON are embedded verbatim as {"$bytea_json":...}, other valid UTF-8 as
//     {"$bytea_utf8":"..."}. Otherwise the ids and instants inside a cached
//     response would hide in hex and could never be normalised.
//
// Every other bytea value stays hex, as "\\x..." inside the JSON text. Digests
// of random tokens (account_sessions, guest links) therefore differ between
// stacks unless the runner names them: the service stores sha256(token), whose
// literal is hex.EncodeToString(sum), and Binder.Name on that literal binds it.
//
// jsonb columns are embedded as jsonb::text, which is jsonb's canonical form:
// keys ordered by length then bytes, ": " and ", " separators, duplicate keys
// collapsed. Both stacks re-serialise identically, so database deltas never
// test JSON key order or spacing; HTTP bodies do. json (not jsonb) columns
// keep their input text.
package dbsnap

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Conn is what Snapshot needs from a connection. *pgx.Conn and *pgxpool.Pool
// both satisfy it.
type Conn interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// Options adjusts a snapshot. The zero value snapshots the application schema.
type Options struct {
	// Schema is the namespace to read; empty means "public".
	Schema string
	// JSONByteaColumns lists "table.column" bytea columns whose bytes are
	// decoded into Row.Text. nil means idempotency_keys.response_body; an
	// empty non-nil slice decodes nothing.
	JSONByteaColumns []string
}

var defaultJSONByteaColumns = []string{storedResponseTable + ".response_body"}

// Snap is every row of every table and view in one schema at one moment.
type Snap struct {
	Schema    string
	Relations []*Relation // sorted by name, byte order
	Took      time.Duration
}

// Relation is one table, view or materialized view.
type Relation struct {
	Name       string
	Kind       string   // "table", "view" or "matview"
	PrimaryKey []string // key columns in index order; empty compares rows as a multiset
	Columns    []Column // in attribute order, which is row_to_json's key order
	Rows       []Row    // in no particular order
}

// Column is one attribute. Type is the base type name, with domains resolved
// one level (e.g. "timestamptz", "bytea", "jsonb").
type Column struct {
	Name string
	Type string
}

// Row is one row as JSON text.
type Row struct {
	Key  string // json_build_array of the primary key values; "" without a key
	Raw  string // row_to_json output, never rewritten or normalised
	Text string // Raw after the canonical rewrites; what deltas compare
}

// Relation returns the named relation, or nil.
func (s *Snap) Relation(name string) *Relation {
	if s == nil {
		return nil
	}
	i := sort.Search(len(s.Relations), func(i int) bool { return s.Relations[i].Name >= name })
	if i < len(s.Relations) && s.Relations[i].Name == name {
		return s.Relations[i]
	}
	return nil
}

// pinSettings fixes every setting that changes how row_to_json renders a
// value. set_config(..., true) is SET LOCAL: it ends with the transaction.
const pinSettings = `SELECT set_config('TimeZone', 'UTC', true),
       set_config('bytea_output', 'hex', true),
       set_config('DateStyle', 'ISO, MDY', true),
       set_config('IntervalStyle', 'postgres', true),
       set_config('extra_float_digits', '1', true)`

const transactionGuard = `SELECT current_setting('transaction_read_only'),
       current_setting('transaction_isolation')`

// catalogQuery lists tables, views and materialized views with their primary
// key and columns. Partitioned parents are skipped because their partitions
// are listed as tables in their own right.
const catalogQuery = `
SELECT c.relname::text,
       c.relkind::text,
       COALESCE((
         SELECT array_agg(a.attname::text ORDER BY k.ord)
         FROM pg_index i
         CROSS JOIN LATERAL unnest(i.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord)
         JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
         WHERE i.indrelid = c.oid AND i.indisprimary
       ), '{}'::text[]),
       COALESCE((
         SELECT array_agg(a.attname::text ORDER BY a.attnum)
         FROM pg_attribute a
         WHERE a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
       ), '{}'::text[]),
       COALESCE((
         SELECT array_agg((CASE WHEN t.typtype = 'd' THEN bt.typname ELSE t.typname END)::text
                          ORDER BY a.attnum)
         FROM pg_attribute a
         JOIN pg_type t ON t.oid = a.atttypid
         LEFT JOIN pg_type bt ON bt.oid = t.typbasetype
         WHERE a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
       ), '{}'::text[])
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relkind IN ('r', 'v', 'm')
ORDER BY c.relname COLLATE "C"`

var relationKinds = map[string]string{"r": "table", "v": "view", "m": "matview"}

// rowAlias names the row variable in each row query. It must not collide with
// a column name, or row_to_json would receive that column instead of the row.
const rowAlias = "_dbsnap_row_"

// Snapshot reads every relation of the public schema.
func Snapshot(ctx context.Context, conn Conn) (*Snap, error) {
	return SnapshotWith(ctx, conn, Options{})
}

// SnapshotWith reads every relation of opts.Schema in one read-only,
// repeatable-read transaction, so all relations reflect the same instant.
func SnapshotWith(ctx context.Context, conn Conn, opts Options) (*Snap, error) {
	start := time.Now()
	schema := opts.Schema
	if schema == "" {
		schema = "public"
	}
	decodedColumns := opts.JSONByteaColumns
	if decodedColumns == nil {
		decodedColumns = defaultJSONByteaColumns
	}
	decoded := make(map[string]bool, len(decodedColumns))
	for _, name := range decodedColumns {
		decoded[name] = true
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("dbsnap: begin: %w", err)
	}
	// Read-only: there is never anything to commit.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, pinSettings); err != nil {
		return nil, fmt.Errorf("dbsnap: pin settings: %w", err)
	}
	var readOnly, isolation string
	if err := tx.QueryRow(ctx, transactionGuard).Scan(&readOnly, &isolation); err != nil {
		return nil, fmt.Errorf("dbsnap: read transaction mode: %w", err)
	}
	if readOnly != "on" || isolation != "repeatable read" {
		return nil, fmt.Errorf("dbsnap: transaction is read_only=%s isolation=%s, want on/repeatable read", readOnly, isolation)
	}

	relations, err := readCatalog(ctx, tx, schema)
	if err != nil {
		return nil, err
	}
	for _, rel := range relations {
		if err := readRows(ctx, tx, schema, rel, planRewrites(rel, decoded)); err != nil {
			return nil, err
		}
	}
	return &Snap{Schema: schema, Relations: relations, Took: time.Since(start)}, nil
}

func readCatalog(ctx context.Context, tx pgx.Tx, schema string) ([]*Relation, error) {
	rows, err := tx.Query(ctx, catalogQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("dbsnap: catalog: %w", err)
	}
	defer rows.Close()
	var relations []*Relation
	for rows.Next() {
		var name, relkind string
		var primaryKey, columnNames, columnTypes []string
		if err := rows.Scan(&name, &relkind, &primaryKey, &columnNames, &columnTypes); err != nil {
			return nil, fmt.Errorf("dbsnap: catalog: %w", err)
		}
		if len(columnNames) != len(columnTypes) {
			return nil, fmt.Errorf("dbsnap: catalog: %s has %d column names but %d types", name, len(columnNames), len(columnTypes))
		}
		rel := &Relation{Name: name, Kind: relationKinds[relkind], PrimaryKey: primaryKey}
		for i := range columnNames {
			rel.Columns = append(rel.Columns, Column{Name: columnNames[i], Type: columnTypes[i]})
		}
		relations = append(relations, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dbsnap: catalog: %w", err)
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].Name < relations[j].Name })
	return relations, nil
}

func rowQuery(schema string, rel *Relation) string {
	from := pgx.Identifier{schema, rel.Name}.Sanitize()
	if rel.Kind == "table" {
		// ONLY: an inheritance child is snapshotted as its own table.
		from = "ONLY " + from
	}
	key := "''"
	if len(rel.PrimaryKey) > 0 {
		columns := make([]string, len(rel.PrimaryKey))
		for i, column := range rel.PrimaryKey {
			columns[i] = rowAlias + "." + pgx.Identifier{column}.Sanitize()
		}
		key = "json_build_array(" + strings.Join(columns, ", ") + ")::text"
	}
	return fmt.Sprintf("SELECT %s, row_to_json(%s)::text FROM %s AS %s", key, rowAlias, from, rowAlias)
}

func readRows(ctx context.Context, tx pgx.Tx, schema string, rel *Relation, plan map[string]rewriteKind) error {
	rows, err := tx.Query(ctx, rowQuery(schema, rel))
	if err != nil {
		return fmt.Errorf("dbsnap: read %s: %w", rel.Name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.Key, &row.Raw); err != nil {
			return fmt.Errorf("dbsnap: read %s: %w", rel.Name, err)
		}
		row.Text, err = canonicalText(row.Raw, plan)
		if err != nil {
			return fmt.Errorf("dbsnap: canonical %s: %w", rel.Name, err)
		}
		rel.Rows = append(rel.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("dbsnap: read %s: %w", rel.Name, err)
	}
	return nil
}
