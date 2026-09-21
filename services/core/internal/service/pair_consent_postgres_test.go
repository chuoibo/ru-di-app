//go:build postgres

package service

// PairChatConsent on real PostgreSQL, twice over:
//
//   - TestPairChatConsentAnswersFromTheRows pins the answer and the number of
//     statements for every branch of _pair_chat_consent, with no Python;
//   - TestPairChatConsentOracle runs the real ApiService._pair_chat_consent
//     (scripts/render_pair_consent_oracle.py, the service clock pinned to the
//     case's `now`) and PairChatConsent on the same seeded schema, and compares
//     the answer, every statement and the probes, as
//     internal/repo/oracle_postgres_test.go does for repository methods. It
//     skips without CORE_PYTHON_IMAGE; scripts/go_postgres_tier.sh sets it and
//     refuses skips.
//
// The oracle's helpers live in package repo's test files, which a service test
// cannot import; the few it needs are restated here.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

var bg = context.Background()

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

func sqlLiteral(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	case int:
		return strconv.Itoa(x)
	}
	panic(fmt.Sprintf("sqlLiteral: %T", v))
}

func fixtureID(kind, n int) string {
	return fmt.Sprintf("%02x%06x-aaaa-4aaa-8aaa-aaaaaaaaaaaa", kind, n)
}

type consentWorld struct {
	sql                                             []string
	a, b, c, d, e                                   string
	group, ab, ac, bc, ad, ae, bd, be, de, missing  string
	deadline                                        time.Time
	notebooks, cycles, proposals, consents, members int
}

func (w *consentWorld) insert(table string, pairs ...any) {
	cols, vals := []string{}, []string{}
	for i := 0; i < len(pairs); i += 2 {
		cols = append(cols, pairs[i].(string))
		vals = append(vals, sqlLiteral(pairs[i+1]))
	}
	w.sql = append(w.sql, "INSERT INTO "+table+" ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(vals, ", ")+")")
}

const created = "2030-01-01T00:00:00.123456Z"

func (w *consentWorld) person(n int, name string, band any) string {
	id := fixtureID(0xa0, n)
	w.insert("people", "id", id, "display_name", name, "budget_band", band, "created_at", created)
	return id
}

func (w *consentWorld) member(contextID, person, state string) {
	w.members++
	pairs := []any{"id", fixtureID(0xb0, w.members), "context_id", contextID, "person_id", person, "state", state,
		"created_at", created}
	switch state {
	case "active":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z")
	case "left":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z", "left_at", "2030-01-03T00:00:00Z")
	}
	w.insert("memberships", pairs...)
}

// pair makes a pair context whose memberships are given as person, state,
// person, state, ...
func (w *consentWorld) pair(n int, x, y string, roster ...string) string {
	id := fixtureID(0xc0, n)
	w.insert("contexts", "id", id, "display_name", "", "created_by_id", x, "created_at", created,
		"kind", "pair", "pair_key", min(x, y)+":"+max(x, y))
	for i := 0; i < len(roster); i += 2 {
		w.member(id, roster[i], roster[i+1])
	}
	return id
}

func (w *consentWorld) notebook(contextID string) string {
	w.notebooks++
	id := fixtureID(0x90, w.notebooks)
	w.insert("pair_notebooks", "id", id, "context_id", contextID, "context_kind", "pair", "created_at", created)
	return id
}

func (w *consentWorld) cycle(notebook, state string, people ...string) string {
	w.cycles++
	id := fixtureID(0x91, w.cycles)
	pairs := []any{"id", id, "notebook_id", notebook, "state", state, "created_at", created}
	if state == "closed" {
		pairs = append(pairs, "opened_at", created, "closed_at", "2030-06-01T00:00:00Z")
	}
	w.insert("pair_notebook_cycles", pairs...)
	for _, person := range people {
		w.insert("pair_cycle_participants", "cycle_id", id, "person_id", person, "created_at", created)
	}
	return id
}

// offer makes one proposal and a consent row per (person, granted_at,
// revoked_at) triple.
func (w *consentWorld) offer(cycle, purpose, expires string, answers ...any) {
	w.proposals++
	id := fixtureID(0x92, w.proposals)
	w.insert("pair_consent_proposals", "id", id, "cycle_id", cycle, "purpose", purpose,
		"proposed_by_id", answers[0], "created_at", created, "expires_at", expires)
	for i := 0; i < len(answers); i += 3 {
		w.consents++
		w.insert("pair_consents", "id", fixtureID(0x93, w.consents), "proposal_id", id, "person_id", answers[i],
			"granted_at", answers[i+1], "revoked_at", answers[i+2], "created_at", created)
	}
}

func newConsentWorld() *consentWorld {
	w := &consentWorld{}
	const granted, far = "2030-01-05T00:00:00Z", "2030-12-01T00:00:00Z"
	w.a = w.person(0x11, "An (dữ liệu mẫu)", "vua-phai")
	w.b = w.person(0x12, "Bình (dữ liệu mẫu)", nil)
	w.c = w.person(0x13, "Chi (dữ liệu mẫu)", "tiet-kiem")
	w.d = w.person(0x14, "Dũng (dữ liệu mẫu)", nil)
	w.e = w.person(0x15, "", "")
	w.missing = fixtureID(0xc0, 0xff)

	w.group = fixtureID(0xc0, 0x31)
	w.insert("contexts", "id", w.group, "display_name", "Nhóm (dữ liệu mẫu)", "created_by_id", w.a, "created_at", created)
	w.member(w.group, w.a, "active")
	w.member(w.group, w.b, "active")

	// Both granted doc_chat in the live cycle, with a deadline to stand on.
	w.deadline = time.Date(2030, 2, 9, 0, 0, 0, 0, time.UTC).Add(500 * time.Millisecond)
	w.ab = w.pair(0x32, w.a, w.b, w.a, "active", w.b, "active")
	cycle := w.cycle(w.notebook(w.ab), "active", w.b, w.a)
	w.offer(cycle, "doc_chat", "2030-02-09T00:00:00.5Z", w.a, granted, nil, w.b, granted, nil)

	// No notebook.
	w.ac = w.pair(0x33, w.a, w.c, w.a, "active", w.c, "active")

	// Only a closed cycle, whose grants must not count.
	w.bc = w.pair(0x34, w.b, w.c, w.b, "active", w.c, "active")
	closed := w.cycle(w.notebook(w.bc), "closed", w.b, w.c)
	w.offer(closed, "doc_chat", far, w.b, granted, nil, w.c, granted, nil)

	// A pending cycle between a and d; d has left and e is now a member. The
	// cycle's list answers yes; the members' list would answer no.
	w.ad = w.pair(0x35, w.a, w.d, w.a, "active", w.d, "left", w.e, "active")
	pending := w.cycle(w.notebook(w.ad), "pending", w.a, w.d)
	w.offer(pending, "doc_chat", far, w.a, granted, nil, w.d, granted, nil)

	// One side revoked.
	w.ae = w.pair(0x36, w.a, w.e, w.a, "active", w.e, "active")
	live := w.cycle(w.notebook(w.ae), "active", w.a, w.e)
	w.offer(live, "doc_chat", far, w.a, granted, nil, w.e, granted, "2030-01-06T00:00:00Z")

	// A notebook that never opened a cycle: participants come from members.
	w.bd = w.pair(0x37, w.b, w.d, w.b, "active", w.d, "active")
	w.notebook(w.bd)

	// A live cycle of one person who granted; e is only invited.
	w.be = w.pair(0x38, w.b, w.e, w.b, "active", w.e, "invited")
	solo := w.cycle(w.notebook(w.be), "active", w.b)
	w.offer(solo, "doc_chat", far, w.b, granted, nil)

	// Lower rungs from both, and doc_chat unanswered by one.
	w.de = w.pair(0x39, w.d, w.e, w.d, "active", w.e, "active")
	rungs := w.cycle(w.notebook(w.de), "active", w.d, w.e)
	w.offer(rungs, "lap_so", far, w.d, granted, nil, w.e, granted, nil)
	w.offer(rungs, "bat_doi", far, w.d, granted, nil, w.e, granted, nil)
	w.offer(rungs, "doc_chat", far, w.d, granted, nil, w.e, nil, nil)
	return w
}

func seed(t *testing.T, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, sql []string) {
	t.Helper()
	for _, statement := range sql {
		if _, err := q.Exec(bg, statement); err != nil {
			t.Fatalf("seed: %v\n%s", err, statement)
		}
	}
}

// statementLog notes every statement the repository issues.
type statementLog struct {
	repo.Querier
	log []string
}

func (r *statementLog) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.log = append(r.log, sql)
	return r.Querier.Exec(ctx, sql, args...)
}

func (r *statementLog) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.log = append(r.log, sql)
	return r.Querier.Query(ctx, sql, args...)
}

func (r *statementLog) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.log = append(r.log, sql)
	return r.Querier.QueryRow(ctx, sql, args...)
}

// ---------------------------------------------------------------------------
// Go only
// ---------------------------------------------------------------------------

func TestPairChatConsentAnswersFromTheRows(t *testing.T) {
	w := newConsentWorld()
	tx := testdb.Tx(t)
	seed(t, tx, w.sql)
	yes, no := true, false
	micro := time.Microsecond
	before := w.deadline.Add(-4 * 24 * time.Hour)
	cases := []struct {
		name       string
		context    string
		now        time.Time
		want       *bool
		statements int
	}{
		{"a group is not a pair", w.group, before, nil, 1},
		{"a missing context", w.missing, before, nil, 1},
		{"a pair with no notebook", w.ac, before, &no, 2},
		{"both granted, well inside the window", w.ab, before, &yes, 9},
		{"both granted, one microsecond before the deadline", w.ab, w.deadline.Add(-micro), &yes, 9},
		{"both granted, exactly at the deadline", w.ab, w.deadline, &no, 9},
		{"both granted, one microsecond after, in +07:00", w.ab, w.deadline.Add(micro).In(time.FixedZone("", 7*3600)), &no, 9},
		{"both granted, 999 nanoseconds before the deadline", w.ab, w.deadline.Add(-999), &yes, 9},
		{"only a closed cycle", w.bc, before, &no, 5},
		{"the cycle's people, not the members", w.ad, before, &yes, 9},
		{"one side revoked", w.ae, before, &no, 9},
		{"no cycle: the members, and no consent", w.bd, before, &no, 5},
		{"a cycle of one", w.be, before, &no, 9},
		{"lower rungs never open doc_chat", w.de, before, &no, 9},
	}
	for _, c := range cases {
		log := &statementLog{Querier: tx}
		got, err := PairChatConsent(bg, repo.Repository{Q: log}, c.context, c.now)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: got %v, want %v", c.name, show(got), show(c.want))
		}
		if len(log.log) != c.statements {
			t.Errorf("%s: %d statements, want %d", c.name, len(log.log), c.statements)
		}
	}
}

func show(b *bool) string {
	if b == nil {
		return "None"
	}
	return strconv.FormatBool(*b)
}

// ---------------------------------------------------------------------------
// Oracle
// ---------------------------------------------------------------------------

const (
	probeLocks = `SELECT c.relname || ' ' || l.mode FROM pg_locks l JOIN pg_class c ON c.oid = l.relation
WHERE l.pid = pg_backend_pid() AND l.locktype = 'relation' AND c.relkind = 'r'
AND c.relnamespace = current_schema()::text::regnamespace ORDER BY 1`
	probeWrites = `SELECT s.relname || ' ins=' || (s.n_tup_ins - coalesce(b.n_tup_ins, 0))
|| ' upd=' || (s.n_tup_upd - coalesce(b.n_tup_upd, 0)) || ' del=' || (s.n_tup_del - coalesce(b.n_tup_del, 0))
FROM pg_stat_xact_user_tables s LEFT JOIN oracle_writes b ON b.relid = s.relid
WHERE s.schemaname = current_schema()
AND (s.n_tup_ins - coalesce(b.n_tup_ins, 0)) + (s.n_tup_upd - coalesce(b.n_tup_upd, 0)) + (s.n_tup_del - coalesce(b.n_tup_del, 0)) > 0
ORDER BY 1`
)

var writesBaseline = []string{
	`DROP TABLE IF EXISTS pg_temp.oracle_writes`,
	`CREATE TEMP TABLE oracle_writes AS SELECT relid, n_tup_ins, n_tup_upd, n_tup_del FROM pg_stat_xact_user_tables`,
}

type consentCall struct {
	Call   string         `json:"call"`
	Args   map[string]any `json:"args"`
	Before []string       `json:"before"`
	Probes []string       `json:"probes"`
}

type consentCase struct {
	Name  string        `json:"name"`
	Setup []string      `json:"setup"`
	Steps []consentCall `json:"steps"`
}

type consentSpec struct {
	Clock []string      `json:"clock"`
	Cases []consentCase `json:"cases"`
}

func consentCases() consentSpec {
	w := newConsentWorld()
	// isoformat(timespec="microseconds"); the layout is split so the repository
	// guard does not read its fraction and offset as one digit run.
	iso := func(x time.Time) string { return x.Format("2006-01-02T15:04:05.000000" + "-07:00") }
	before := iso(w.deadline.Add(-4 * 24 * time.Hour))
	var cases []consentCase
	add := func(name, context, now string, setup ...string) {
		// One call per case: session.get would answer a second read of the
		// same context from the identity map, which Go does not model.
		cases = append(cases, consentCase{Name: name, Setup: append(append([]string{}, setup...), w.sql...),
			Steps: []consentCall{{Call: "service._pair_chat_consent", Args: map[string]any{"context_id": context, "now": now},
				Before: append([]string{}, writesBaseline...), Probes: []string{probeLocks, probeWrites}}}})
	}
	micro := time.Microsecond
	add("a group is not a pair", w.group, before)
	add("a missing context", w.missing, before)
	add("a pair with no notebook", w.ac, before)
	add("both granted, well inside the window", w.ab, before)
	add("both granted, one microsecond before the deadline", w.ab, iso(w.deadline.Add(-micro)))
	add("both granted, exactly at the deadline", w.ab, iso(w.deadline))
	add("both granted, one microsecond after, in +07:00", w.ab, iso(w.deadline.Add(micro).In(time.FixedZone("", 7*3600))))
	add("both granted, under a Vietnam session TimeZone", w.ab, before, "SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'")
	add("only a closed cycle", w.bc, before)
	add("the cycle's people, not the members", w.ad, before)
	add("one side revoked", w.ae, before)
	add("no cycle: the members, and no consent", w.bd, before)
	add("a cycle of one", w.be, before)
	add("lower rungs never open doc_chat", w.de, before)
	add("the members only when no cycle is live", w.bd, iso(w.deadline.Add(300*24*time.Hour)))
	// No fixture at all: the context read finds nothing.
	cases = append(cases, consentCase{Name: "an empty schema", Setup: []string{}, Steps: []consentCall{{
		Call: "service._pair_chat_consent", Args: map[string]any{"context_id": w.ab, "now": before},
		Before: append([]string{}, writesBaseline...), Probes: []string{probeLocks, probeWrites}}}})
	return consentSpec{Clock: []string{}, Cases: cases}
}

var (
	pythonBind = regexp.MustCompile(`%\(\w+\)s`)
	goBind     = regexp.MustCompile(`\$\d+`)
)

func normalizeSQL(sql string) string {
	sql = pythonBind.ReplaceAllString(sql, "?")
	sql = goBind.ReplaceAllString(sql, "?")
	sql = strings.Join(strings.Fields(sql), " ")
	sql = strings.ReplaceAll(sql, "( ", "(")
	return strings.ReplaceAll(sql, " )", ")")
}

func generic(t *testing.T, v any) any {
	t.Helper()
	payload, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func goFailure(err error) map[string]any {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return map[string]any{"type": "PgError " + pg.Code, "sqlstate": pg.Code, "constraint": pg.ConstraintName}
	}
	return map[string]any{"type": "go error: " + err.Error(), "sqlstate": nil, "constraint": nil}
}

func runGoConsentCase(t *testing.T, pool *pgxpool.Pool, c consentCase) []any {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	seed(t, tx, c.Setup)
	steps := []any{}
	for _, step := range c.Steps {
		seed(t, tx, step.Before)
		now, err := time.Parse(time.RFC3339Nano, step.Args["now"].(string))
		if err != nil {
			t.Fatal(err)
		}
		log := &statementLog{Querier: tx}
		answer, err := PairChatConsent(bg, repo.Repository{Q: log}, step.Args["context_id"].(string), now)
		statements := []any{}
		for _, sql := range log.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = goFailure(err)
			steps = append(steps, generic(t, out))
			break
		}
		if answer != nil {
			out["result"] = map[string]any{"bool": *answer}
		}
		probes := []any{}
		for _, sql := range step.Probes {
			rows, err := tx.Query(bg, sql)
			if err != nil {
				t.Fatalf("probe: %v\n%s", err, sql)
			}
			texts, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				t.Fatal(err)
			}
			probes = append(probes, append([]string{}, texts...))
		}
		out["probes"] = probes
		steps = append(steps, generic(t, out))
	}
	return steps
}

func pythonDatabaseURL(raw, schema string) string {
	url := raw
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(url, prefix) {
			url = "postgresql+psycopg://" + strings.TrimPrefix(url, prefix)
			break
		}
	}
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return url + separator + "options=-csearch_path=" + schema
}

func lastBytes(out []byte) string {
	if len(out) > 3000 {
		return string(out[len(out)-3000:])
	}
	return string(out)
}

func TestPairChatConsentOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of pair_consent_postgres_test.go")
	}
	base := testdb.Pool(t)
	raw := strings.TrimSpace(os.Getenv("CORE_TEST_DATABASE_URL"))
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_pair_consent_oracle.py")); err != nil {
		t.Fatal(err)
	}
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "consent_oracle_" + hex.EncodeToString(suffix[:])
	if _, err := base.Exec(bg, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(bg, "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	migrate := osexec.Command("docker", "run", "--rm", "--network", "host",
		"-e", "MOBILE_DATABASE_URL="+pythonDatabaseURL(raw, schema), image, "alembic", "upgrade", "head")
	if out, err := migrate.CombinedOutput(); err != nil {
		t.Fatalf("alembic upgrade head into %s: %v\n%s", schema, err, lastBytes(out))
	}
	config, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(bg, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	payload, err := json.Marshal(consentCases())
	if err != nil {
		t.Fatal(err)
	}
	var spec consentSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+pythonDatabaseURL(raw, schema), "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_pair_consent_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_pair_consent_oracle.py: %v\n%s", err, lastBytes(stderr.Bytes()))
	}
	var python struct {
		Cases []struct {
			Name  string `json:"name"`
			Steps []struct {
				Result     any     `json:"result"`
				Error      any     `json:"error"`
				Warnings   []any   `json:"warnings"`
				Statements [][]any `json:"statements"`
				Probes     any     `json:"probes"`
			} `json:"steps"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	outcomes := map[string]int{}
	statements, probeRows, mismatches := 0, 0, 0
	for i, c := range spec.Cases {
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for _, s := range python.Cases[i].Steps {
			logged := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					logged = append(logged, normalizeSQL(entry[0].(string)))
				}
			}
			statements += len(logged)
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			outcomes[fmt.Sprint(s.Result)]++
			if probes, ok := s.Probes.([]any); ok {
				for _, rows := range probes {
					probeRows += len(rows.([]any))
				}
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": logged, "probes": s.Probes}))
		}
		golang := runGoConsentCase(t, pool, c)
		t.Run(c.Name, func(t *testing.T) {
			if len(pySteps) != len(golang) {
				mismatches++
				t.Fatalf("python ran %d steps, go %d", len(pySteps), len(golang))
			}
			for j := range pySteps {
				p, g := pySteps[j].(map[string]any), golang[j].(map[string]any)
				for _, key := range []string{"result", "error", "warnings", "statements", "probes"} {
					if !reflect.DeepEqual(p[key], g[key]) {
						mismatches++
						pj, _ := json.Marshal(p[key])
						gj, _ := json.Marshal(g[key])
						t.Errorf("%s differs\n  python: %s\n  go:     %s", key, pj, gj)
					}
				}
			}
		})
	}
	for _, want := range []string{"<nil>", "map[bool:true]", "map[bool:false]"} {
		if outcomes[want] == 0 {
			t.Fatalf("no case where python answered %s: the corpus cannot tell the branches apart (%v)", want, outcomes)
		}
	}
	t.Logf("pair chat consent oracle: %d cases, outcomes %v, %d statements, %d probe rows, %d mismatches",
		len(spec.Cases), outcomes, statements, probeRows, mismatches)
}
