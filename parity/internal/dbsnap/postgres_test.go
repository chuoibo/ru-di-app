//go:build postgres

package dbsnap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/parity/internal/normalize"
)

// Run with: PARITY_TEST_DSN=postgresql://... go test -tags postgres ./internal/dbsnap/
// The tag is an explicit request for the live layer, so a missing DSN fails
// instead of skipping.
func connect(t *testing.T, env string) *pgx.Conn {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Fatalf("%s is not set", env)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect %s: %v", env, err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func letters(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = 'a' + b[i]%26
	}
	return string(b)
}

// throwawaySchema creates an empty schema that is dropped when the test ends.
func throwawaySchema(t *testing.T, conn *pgx.Conn, prefix string) string {
	t.Helper()
	schema := "dbsnap_" + prefix + "_" + letters(t, 8)
	exec(t, conn, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	return schema
}

func exec(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func snapshot(t *testing.T, conn *pgx.Conn, schema string) *Snap {
	t.Helper()
	s, err := SnapshotWith(context.Background(), conn, Options{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func onlyRow(t *testing.T, s *Snap, relation string) Row {
	t.Helper()
	rel := s.Relation(relation)
	if rel == nil || len(rel.Rows) != 1 {
		t.Fatalf("%s: want exactly one row, got %+v", relation, rel)
	}
	return rel.Rows[0]
}

func changeOf(t *testing.T, c *Change, relation string) RelationChange {
	t.Helper()
	for _, rc := range c.Changed {
		if rc.Relation == relation {
			return rc
		}
	}
	t.Fatalf("%s did not change; changed: %v", relation, c.Changed)
	return RelationChange{}
}

func TestSnapshotRendering(t *testing.T) {
	conn := connect(t, "PARITY_TEST_DSN")
	s := throwawaySchema(t, conn, "render")
	for _, ddl := range []string{
		`CREATE TABLE %[1]s.things (id uuid PRIMARY KEY, label text NOT NULL, at timestamptz,
			naive timestamp, doc jsonb, note json, digest bytea)`,
		`CREATE TABLE %[1]s.idempotency_keys (id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			scope text NOT NULL, idempotency_key text NOT NULL, response_status int,
			response_body bytea, response_media_type text)`,
		`CREATE TABLE %[1]s.tags (label text)`,
		`CREATE TABLE %[1]s.pairs (a text, b int, v text, PRIMARY KEY (b, a))`,
		`CREATE TABLE %[1]s.ledger (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), entry text NOT NULL)`,
		`CREATE FUNCTION %[1]s.forbid_change() RETURNS trigger LANGUAGE plpgsql AS
			$$BEGIN RAISE EXCEPTION 'append-only'; END$$`,
		`CREATE TRIGGER ledger_append_only BEFORE UPDATE OR DELETE ON %[1]s.ledger
			FOR EACH ROW EXECUTE FUNCTION %[1]s.forbid_change()`,
		`CREATE VIEW %[1]s.tag_counts AS SELECT label, count(*) AS n FROM %[1]s.tags GROUP BY label`,
	} {
		exec(t, conn, fmt.Sprintf(ddl, s))
	}
	// A connection default that must not leak into rendering, and must
	// survive the snapshot.
	exec(t, conn, "SET TimeZone = 'Asia/Ho_Chi_Minh'")

	s0 := snapshot(t, conn, s)

	id := newUUID(t)
	body := []byte(`{"thing_id":"` + id + `","at":"2026-09-14T10:00:05.120000+00:00"}`)
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO %[1]s.things VALUES ($1, 'first', '2026-09-14T10:00:05.12Z', '2026-09-14T10:00:05',
			'{"zeta": 1, "b": 2, "b": 3}', '{"zeta": 1, "b": 2}', sha256('token'::bytea))`, []any{id}},
		{`INSERT INTO %[1]s.things (id, label, at) VALUES (gen_random_uuid(), 'second', 'infinity')`, nil},
		{`INSERT INTO %[1]s.idempotency_keys (scope, idempotency_key, response_status, response_body, response_media_type)
			VALUES ('person:an', 'done', 201, $1, 'application/json'), ('person:an', 'pending', NULL, NULL, NULL)`, []any{body}},
		{`INSERT INTO %[1]s.tags VALUES ('x'), ('x'), ('y')`, nil},
		{`INSERT INTO %[1]s.pairs VALUES ('p', 2, 'v')`, nil},
		{`INSERT INTO %[1]s.ledger (entry) VALUES ('opened')`, nil},
	} {
		exec(t, conn, fmt.Sprintf(stmt.sql, s), stmt.args...)
	}
	s1 := snapshot(t, conn, s)
	t.Logf("snapshot of %d relations took %s", len(s1.Relations), s1.Took)

	var zone string
	if err := conn.QueryRow(context.Background(), "SHOW TimeZone").Scan(&zone); err != nil || zone != "Asia/Ho_Chi_Minh" {
		t.Errorf("session TimeZone after snapshot = %q, %v", zone, err)
	}

	var names []string
	for _, rel := range s1.Relations {
		names = append(names, rel.Name+":"+rel.Kind+":"+strings.Join(rel.PrimaryKey, ","))
	}
	want := []string{"idempotency_keys:table:id", "ledger:table:id", "pairs:table:b,a", "tag_counts:view:", "tags:table:", "things:table:id"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Errorf("relations %v, want %v", names, want)
	}

	things := s1.Relation("things")
	var first, second Row
	for _, r := range things.Rows {
		if strings.Contains(r.Raw, `"first"`) {
			first = r
		} else {
			second = r
		}
	}
	for _, check := range []struct{ in, want string }{
		{first.Raw, `"at":"2026-09-14T10:00:05.12+00:00"`},
		{first.Text, `"at":"2026-09-14T10:00:05.120000+00:00"`},
		{first.Raw, `"naive":"2026-09-14T10:00:05"`},
		{first.Text, `"naive":"2026-09-14T10:00:05.000000"`},
		{first.Raw, `"doc":{"b": 3, "zeta": 1}`},   // jsonb: canonical order, last duplicate wins
		{first.Raw, `"note":{"zeta": 1, "b": 2}`},  // json: input text kept
		{second.Text, `"at":"infinity"`},           // not an instant, left alone
		{first.Key, `["` + id + `"]`},              // key rendered by json_build_array
		{onlyRow(t, s1, "pairs").Key, `[2, "p"]`},  // composite key in index order
		{onlyRow(t, s1, "ledger").Raw, `"opened"`}, // append-only table read without writing
		{first.Raw, `"digest":"\\x` + hexSHA256("token") + `"`},
	} {
		if !strings.Contains(check.in, check.want) {
			t.Errorf("%s\n  does not contain %s", check.in, check.want)
		}
	}

	d1 := Delta(s0, s1)
	for relation, count := range map[string]int{"things": 2, "idempotency_keys": 2, "tags": 3, "tag_counts": 2, "pairs": 1, "ledger": 1} {
		if got := len(changeOf(t, d1, relation).Inserted); got != count {
			t.Errorf("%s inserted %d rows, want %d", relation, got, count)
		}
	}
	var decoded []StoredResponse
	for _, sr := range d1.StoredResponses() {
		if sr.Body != nil {
			decoded = append(decoded, sr)
		}
	}
	if len(decoded) != 1 || string(decoded[0].Body) != string(body) || !decoded[0].JSON || decoded[0].Status != 201 {
		t.Errorf("stored responses %+v", d1.StoredResponses())
	}
	stored := changeOf(t, d1, "idempotency_keys")
	if joined := strings.Join(texts(stored.Inserted), "\n"); !strings.Contains(joined, `"response_body":{"$bytea_json":`+string(body)+`}`) {
		t.Errorf("response body not decoded into text:\n%s", joined)
	}

	exec(t, conn, fmt.Sprintf(`UPDATE %s.things SET label = 'renamed' WHERE id = $1`, s), id)
	exec(t, conn, fmt.Sprintf(`DELETE FROM %[1]s.tags WHERE ctid = (SELECT ctid FROM %[1]s.tags WHERE label = 'x' LIMIT 1)`, s))
	exec(t, conn, fmt.Sprintf(`INSERT INTO %s.ledger (entry) VALUES ('closed')`, s))
	d2 := Delta(s1, snapshot(t, conn, s))

	if rc := changeOf(t, d2, "things"); len(rc.Updated) != 1 || !strings.Contains(rc.Updated[0].After.Text, `"renamed"`) || len(rc.Inserted)+len(rc.Deleted) != 0 {
		t.Errorf("things change %+v", rc)
	}
	if rc := changeOf(t, d2, "tags"); len(rc.Deleted) != 1 || rc.Deleted[0].Text != `{"label":"x"}` || len(rc.Inserted) != 0 {
		t.Errorf("tags change %+v", rc)
	}
	if rc := changeOf(t, d2, "tag_counts"); !(len(rc.Deleted) == 1 && rc.Deleted[0].Text == `{"label":"x","n":2}` &&
		len(rc.Inserted) == 1 && rc.Inserted[0].Text == `{"label":"x","n":1}`) {
		t.Errorf("view change %+v", rc)
	}
	if rc := changeOf(t, d2, "ledger"); len(rc.Inserted) != 1 {
		t.Errorf("ledger change %+v", rc)
	}
	if len(d2.Changed) != 4 {
		t.Errorf("unexpected relations changed: %+v", d2.Changed)
	}
}

func hexSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// stackRun is what one simulated stack wrote over two steps.
type stackRun struct {
	token  string
	deltas []*Change
}

// runStack applies the same two steps to its own schema, with its own random
// ids, instants and session token, the way two independent stacks would.
func runStack(t *testing.T, conn *pgx.Conn, extraUpdate bool) stackRun {
	t.Helper()
	s := throwawaySchema(t, conn, "stack")
	for _, ddl := range []string{
		`CREATE TABLE %[1]s.groups (id uuid PRIMARY KEY, name text NOT NULL,
			settings jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT clock_timestamp())`,
		`CREATE TABLE %[1]s.members (group_id uuid NOT NULL REFERENCES %[1]s.groups, person text NOT NULL,
			share bigint NOT NULL, PRIMARY KEY (group_id, person))`,
		`CREATE TABLE %[1]s.audit_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), group_id uuid NOT NULL,
			kind text NOT NULL, at timestamptz NOT NULL)`,
		`CREATE FUNCTION %[1]s.forbid_change() RETURNS trigger LANGUAGE plpgsql AS
			$$BEGIN RAISE EXCEPTION 'append-only'; END$$`,
		`CREATE TRIGGER audit_append_only BEFORE UPDATE OR DELETE ON %[1]s.audit_events
			FOR EACH ROW EXECUTE FUNCTION %[1]s.forbid_change()`,
		`CREATE TABLE %[1]s.idempotency_keys (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), scope text NOT NULL,
			idempotency_key text NOT NULL, response_status int, response_body bytea, token_digest bytea)`,
		`CREATE TABLE %[1]s.tags (label text)`,
		`CREATE VIEW %[1]s.group_totals AS SELECT g.id AS group_id, count(m.person) AS members,
			COALESCE(sum(m.share), 0) AS total
			FROM %[1]s.groups g LEFT JOIN %[1]s.members m ON m.group_id = g.id GROUP BY g.id`,
	} {
		exec(t, conn, fmt.Sprintf(ddl, s))
	}
	run := stackRun{token: letters(t, 32)}
	s0 := snapshot(t, conn, s)

	exec(t, conn, fmt.Sprintf(`WITH g AS (
			INSERT INTO %[1]s.groups (id, name, settings)
			VALUES (gen_random_uuid(), 'Da Lat', '{"currency": "VND", "b": 1}') RETURNING id, created_at
		), m AS (
			INSERT INTO %[1]s.members SELECT g.id, v.person, v.share
			FROM g, (VALUES ('an', 150000), ('binh', 50000)) AS v(person, share)
		), a AS (
			INSERT INTO %[1]s.audit_events (group_id, kind, at) SELECT id, 'group.created', created_at FROM g
		)
		INSERT INTO %[1]s.idempotency_keys (scope, idempotency_key, response_status, response_body, token_digest)
		SELECT 'person:an', 'create-group', 201,
			convert_to(json_build_object('id', g.id, 'created_at',
				to_char(g.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"+00:00"'))::text, 'UTF8'),
			sha256(convert_to($1::text, 'UTF8'))
		FROM g`, s), run.token)
	exec(t, conn, fmt.Sprintf(`INSERT INTO %s.tags VALUES ('x'), ('x'), ('y')`, s))
	s1 := snapshot(t, conn, s)

	settings := ""
	if extraUpdate {
		settings = `, settings = '{"currency": "VND", "b": 2}'`
	}
	exec(t, conn, fmt.Sprintf(`UPDATE %s.members SET share = 100000 WHERE person = 'an'`, s))
	exec(t, conn, fmt.Sprintf(`UPDATE %s.groups SET name = 'Da Lat 2'%s`, s, settings))
	exec(t, conn, fmt.Sprintf(`INSERT INTO %[1]s.audit_events (group_id, kind, at)
		SELECT id, 'member.updated', created_at + interval '1 minute' FROM %[1]s.groups`, s))
	exec(t, conn, fmt.Sprintf(`DELETE FROM %[1]s.tags WHERE ctid = (SELECT ctid FROM %[1]s.tags WHERE label = 'x' LIMIT 1)`, s))
	s2 := snapshot(t, conn, s)

	run.deltas = []*Change{Delta(s0, s1), Delta(s1, s2)}
	return run
}

// normaliseRun is what the runner does per stack: observe the stored
// response (standing in for the HTTP body) and every change, then apply.
func normaliseRun(t *testing.T, run stackRun, nameDigest bool) (changes []*Change, bodies []string) {
	t.Helper()
	binder := normalize.NewBinder()
	if nameDigest {
		if err := binder.Name(hexSHA256(run.token), "digest:session"); err != nil {
			t.Fatal(err)
		}
	}
	for _, delta := range run.deltas {
		for _, sr := range delta.StoredResponses() {
			if err := binder.Observe(string(sr.Body)); err != nil {
				t.Fatal(err)
			}
		}
		for _, text := range delta.Texts() {
			if err := binder.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, delta := range run.deltas {
		changes = append(changes, delta.Normalise(binder.Apply))
		for _, sr := range delta.StoredResponses() {
			bodies = append(bodies, binder.Apply(string(sr.Body)))
		}
	}
	return changes, bodies
}

func TestTwoStacksNormaliseEqual(t *testing.T) {
	conn := connect(t, "PARITY_TEST_DSN")
	ref := runStack(t, conn, false)
	cand := runStack(t, conn, false)
	extra := runStack(t, conn, true)

	refChanges, refBodies := normaliseRun(t, ref, true)
	candChanges, candBodies := normaliseRun(t, cand, true)
	for step := range refChanges {
		if diffs := Compare(refChanges[step], candChanges[step]); len(diffs) > 0 {
			t.Errorf("step %d: identical work differs:\n%v", step+1, diffs)
		}
	}
	if len(refBodies) != 1 || refBodies[0] != candBodies[0] || !strings.Contains(refBodies[0], "<uuid#") ||
		!strings.Contains(refBodies[0], "|f6|+00:00>") {
		t.Errorf("stored bodies: reference %v, candidate %v", refBodies, candBodies)
	}
	if !strings.Contains(strings.Join(refChanges[0].Texts(), "\n"), "<digest:session>") {
		t.Error("named digest was not replaced in the database texts")
	}

	// Without naming the digest, the random token shows: that is the
	// documented limit of hex bytea, not a flake.
	unnamedRef, _ := normaliseRun(t, ref, false)
	unnamedCand, _ := normaliseRun(t, cand, false)
	diffs := Compare(unnamedRef[0], unnamedCand[0])
	if len(diffs) != 1 || diffs[0].Relation != "idempotency_keys" || diffs[0].Kind != KindInserted {
		t.Errorf("unnamed digest: want one idempotency_keys difference, got %v", diffs)
	}

	extraChanges, _ := normaliseRun(t, extra, true)
	if diffs := Compare(refChanges[0], extraChanges[0]); len(diffs) > 0 {
		t.Errorf("step 1 should still be equal: %v", diffs)
	}
	diffs = Compare(refChanges[1], extraChanges[1])
	if len(diffs) != 1 || diffs[0].Relation != "groups" || diffs[0].Kind != KindUpdated ||
		!strings.Contains(diffs[0].OnlyCandidate[0], `"settings":{"b": 2, "currency": "VND"}`) {
		t.Errorf("extra update: want one groups update difference, got %v", diffs)
	}
}

// TestLiveStacks snapshots the two migrated databases scripts/parity_stacks.sh
// brings up and checks their seeded state normalises equal. It is skipped
// unless both DSNs are exported from the stacks env file.
func TestLiveStacks(t *testing.T) {
	if os.Getenv("PARITY_REF_DSN") == "" || os.Getenv("PARITY_CAND_DSN") == "" {
		t.Skip("PARITY_REF_DSN / PARITY_CAND_DSN not set")
	}
	var normalised []*Change
	for _, env := range []string{"PARITY_REF_DSN", "PARITY_CAND_DSN"} {
		conn := connect(t, env)
		var took []time.Duration
		var snaps []*Snap
		for i := 0; i < 7; i++ {
			s, err := Snapshot(context.Background(), conn)
			if err != nil {
				t.Fatal(err)
			}
			took = append(took, s.Took)
			snaps = append(snaps, s)
		}
		sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
		last := snaps[len(snaps)-1]
		rows, bytes, byKind := 0, 0, map[string]int{}
		for _, rel := range last.Relations {
			rows += len(rel.Rows)
			byKind[rel.Kind]++
			for _, r := range rel.Rows {
				bytes += len(r.Raw)
			}
		}
		t.Logf("%s: %v relations, %d rows, %d bytes; snapshot min %s median %s max %s",
			env, byKind, rows, bytes, took[0], took[len(took)/2], took[len(took)-1])
		if !Delta(snaps[0], last).Empty() {
			t.Errorf("%s: two snapshots of an idle database differ: %v", env, Delta(snaps[0], last).Changed)
		}
		full := Delta(nil, last)
		binder := normalize.NewBinder()
		for _, text := range full.Texts() {
			if err := binder.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
		normalised = append(normalised, full.Normalise(binder.Apply))
	}
	diffs := Compare(normalised[0], normalised[1])
	for i, d := range diffs {
		if i == 20 {
			t.Errorf("… and %d more differences", len(diffs)-20)
			break
		}
		t.Errorf("%s", d)
	}
}
