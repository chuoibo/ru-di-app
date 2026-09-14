//go:build postgres

package repo

// Differential test of GetContext, ListMembers, BudgetBandsByPerson and
// GetPairNotebook against the real SqlAlchemyApiRepository, driven through
// scripts/render_pair_consent_oracle.py (render_repo_oracle.py plus these
// methods). Same design as oracle_postgres_test.go, whose case format, value
// tags, statement normalisation, probes and comparison it reuses: a private
// schema migrated by Alembic from the image, each case seeded with identical
// literal SQL in a rolled-back transaction on each side, then the returned
// value, every statement and the probes compared.
//
// Without CORE_PYTHON_IMAGE the test skips; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

func tContext(c *ContextRecord) any {
	if c == nil {
		return nil
	}
	return tRecord("ContextRecord", "id", tUUID(c.ID), "display_name", tStr(c.DisplayName),
		"created_by_id", tUUID(c.CreatedByID), "created_at", tInstant(c.CreatedAt), "theme", tStr(c.Theme),
		"kind", tStr(c.Kind), "pair_key", optional(c.PairKey, tStr))
}

func tMembership(m Membership) any {
	return tRecord("MembershipRecord", "id", tUUID(m.ID), "context_id", tUUID(m.ContextID),
		"person_id", tUUID(m.PersonID), "display_name", tStr(m.DisplayName), "state", tStr(m.State),
		"role", tStr(m.Role), "origin", tStr(m.Origin), "invited_by_id", optional(m.InvitedByID, tUUID),
		"joined_at", optional(m.JoinedAt, tInstant), "left_at", optional(m.LeftAt, tInstant),
		"created_at", tInstant(m.CreatedAt))
}

func tInt32(n int) any { return tInt(int64(n)) }

func tNotebook(n *PairNotebook) any {
	if n == nil {
		return nil
	}
	people := []any{}
	for _, p := range n.Participants {
		people = append(people, tUUID(p))
	}
	consents := []any{}
	for _, c := range n.Consents {
		consents = append(consents, tRecord("PairConsentRecord", "proposal_id", tUUID(c.ProposalID),
			"person_id", tUUID(c.PersonID), "purpose", tStr(c.Purpose), "granted_at", optional(c.GrantedAt, tInstant),
			"revoked_at", optional(c.RevokedAt, tInstant), "proposal_expires_at", tInstant(c.ProposalExpiresAt),
			"terms_version", tInt32(c.TermsVersion)))
	}
	proposals := []any{}
	for _, p := range n.Proposals {
		proposals = append(proposals, tRecord("PairProposalRecord", "id", tUUID(p.ID), "cycle_id", tUUID(p.CycleID),
			"purpose", tStr(p.Purpose), "proposed_by_id", tUUID(p.ProposedByID), "terms_version", tInt32(p.TermsVersion),
			"completed_at", optional(p.CompletedAt, tInstant), "created_at", tInstant(p.CreatedAt),
			"expires_at", tInstant(p.ExpiresAt)))
	}
	constraints := []any{}
	for _, c := range n.Constraints {
		constraints = append(constraints, tRecord("PairConstraintRecord", "owner_id", tUUID(c.OwnerID),
			"kind", tStr(c.Kind), "content", tStr(c.Content), "version", tInt32(c.Version),
			"updated_at", tInstant(c.UpdatedAt)))
	}
	return tRecord("PairNotebookRecord", "id", tUUID(n.ID), "context_id", tUUID(n.ContextID),
		"cycle_id", optional(n.CycleID, tUUID), "cycle_state", optional(n.CycleState, tStr),
		"terms_version", tInt32(n.TermsVersion), "participants", tSeq(people), "consents", tSeq(consents),
		"proposals", tSeq(proposals), "constraints", tSeq(constraints))
}

func pairGoCall(repo Repository, method string, a map[string]any) (any, error) {
	switch method {
	case "get_context":
		c, err := repo.GetContext(bg, argString(a, "context_id"))
		return tContext(c), err
	case "list_members":
		rows, err := repo.ListMembers(bg, argString(a, "context_id"))
		items := []any{}
		for _, m := range rows {
			items = append(items, tMembership(m))
		}
		return tSeq(items), err
	case "budget_bands_by_person":
		bands, err := repo.BudgetBandsByPerson(bg, argStrings(a["person_ids"]))
		pairs := []any{}
		for _, b := range bands {
			pairs = append(pairs, []any{tUUID(b.PersonID), tStr(b.Band)})
		}
		return tv("dict", pairs), err
	case "get_pair_notebook":
		n, err := repo.GetPairNotebook(bg, argString(a, "context_id"))
		return tNotebook(n), err
	}
	return goCall(repo, method, a)
}

func pairOracleCases() oracleSpec {
	w := newPairWorld()
	var cases []oracleCase
	add := func(name string, setup []string, steps ...oracleCall) {
		cases = append(cases, oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps})
	}
	read := func(method string, args map[string]any) oracleCall {
		return oracleCall{Call: method, Args: args, Before: append([]string{}, writesBaseline...),
			Probes: []string{probeLocks, probeWrites}}
	}
	ctxArg := func(id string) map[string]any { return map[string]any{"context_id": id} }
	people := func(ids ...string) map[string]any {
		list := []any{}
		for _, id := range ids {
			list = append(list, id)
		}
		return map[string]any{"person_ids": list}
	}
	vietnam := append([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, w.sql...)

	// session.get answers a second read of one id from the identity map, so
	// each case reads a given context at most once.
	add("get_context: a group, a pair and a missing id", w.sql,
		read("get_context", ctxArg(w.group)), read("get_context", ctxArg(w.ab)), read("get_context", ctxArg(w.missingContext)))
	add("get_context: under a Vietnam session TimeZone", vietnam, read("get_context", ctxArg(w.fb)))
	add("list_members: order, ended rows, erased and unnamed people", w.sql,
		read("list_members", ctxArg(w.group)), read("list_members", ctxArg(w.ad)), read("list_members", ctxArg(w.bd)),
		read("list_members", ctxArg(w.missingContext)), read("list_members", ctxArg(w.group)))
	add("list_members: under a Vietnam session TimeZone", vietnam, read("list_members", ctxArg(w.group)))
	add("list_members: empty tables", nil, read("list_members", ctxArg(w.group)))
	add("budget_bands_by_person: NULL and empty bands, duplicates, missing, none", w.sql,
		read("budget_bands_by_person", people(w.a, w.c, w.e, w.missingPerson, w.a, w.f, w.b, w.d)),
		read("budget_bands_by_person", people()),
		read("budget_bands_by_person", people(w.c)),
		read("budget_bands_by_person", people(w.missingPerson, w.missingPerson)),
		read("budget_bands_by_person", people(w.d, w.b, w.e)))
	add("get_pair_notebook: a live cycle after a closed one, ties in every order", w.sql,
		read("get_pair_notebook", ctxArg(w.ab)))
	add("get_pair_notebook: only a closed cycle", w.sql, read("get_pair_notebook", ctxArg(w.bc)))
	add("get_pair_notebook: a pending cycle whose people have since changed", w.sql,
		read("get_pair_notebook", ctxArg(w.ad)))
	add("get_pair_notebook: revoked and unanswered rows", w.sql,
		read("get_pair_notebook", ctxArg(w.fb)), read("get_pair_notebook", ctxArg(w.ae)))
	add("get_pair_notebook: no cycle, no notebook, a group, a missing id", w.sql,
		read("get_pair_notebook", ctxArg(w.bd)), read("get_pair_notebook", ctxArg(w.ac)),
		read("get_pair_notebook", ctxArg(w.group)), read("get_pair_notebook", ctxArg(w.missingContext)))
	add("get_pair_notebook: under a Vietnam session TimeZone", vietnam, read("get_pair_notebook", ctxArg(w.ab)))
	add("the reads group_taste makes for a pair, in its order", w.sql,
		read("get_context", ctxArg(w.ab)), read("get_pair_notebook", ctxArg(w.ab)), read("list_members", ctxArg(w.ab)),
		read("list_members", ctxArg(w.ab)), read("interests_by_person", people(w.a, w.b)),
		read("budget_bands_by_person", people(w.a, w.b)))
	return oracleSpec{Clock: []string{}, Cases: cases}
}

func runPairGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	for _, sql := range c.Setup {
		if _, err := tx.Exec(bg, sql); err != nil {
			t.Fatalf("setup: %v\n%s", err, sql)
		}
	}
	steps := []any{}
	for _, step := range c.Steps {
		for _, sql := range step.Before {
			if _, err := tx.Exec(bg, sql); err != nil {
				t.Fatalf("before: %v\n%s", err, sql)
			}
		}
		rec := &recorder{Querier: tx}
		value, err := pairGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = goError(err)
			steps = append(steps, generic(t, out))
			break
		}
		out["result"] = value
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

// migratedOracleSchema creates a private schema in the tier database, runs
// Alembic into it from the image, and returns a pool whose search_path is
// that schema plus the Python URL of the same schema.
func migratedOracleSchema(t *testing.T, image, prefix string) (*pgxpool.Pool, string) {
	t.Helper()
	base := testdb.Pool(t)
	raw := strings.TrimSpace(os.Getenv("CORE_TEST_DATABASE_URL"))
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := prefix + hex.EncodeToString(suffix[:])
	if _, err := base.Exec(bg, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(bg, "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	migrate := osexec.Command("docker", "run", "--rm", "--network", "host",
		"-e", "MOBILE_DATABASE_URL="+pythonURL(raw, schema), image, "alembic", "upgrade", "head")
	if out, err := migrate.CombinedOutput(); err != nil {
		t.Fatalf("alembic upgrade head into %s: %v\n%s", schema, err, tail(out))
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
	return pool, pythonURL(raw, schema)
}

func TestPairConsentRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of pair_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_pair_consent_oracle.py")); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "pair_oracle_")

	payload, err := json.Marshal(pairOracleCases())
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_pair_consent_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_pair_consent_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	nonNull := 0
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for _, s := range python.Cases[i].Steps {
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statements = append(statements, normalizeSQL(entry[0].(string)))
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			if s.Result != nil {
				nonNull++
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}
		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, pySteps, runPairGoCase(t, pool, c))
		})
	}
	if tally.errors != 0 || nonNull < tally.steps/2 {
		t.Fatalf("the corpus should read, not refuse: %d refusals, %d non-null results of %d steps", tally.errors, nonNull, tally.steps)
	}
	t.Logf("pair consent repo oracle: %d cases, %d steps (%d results, %d non-null, %d refusals), %d statements, "+
		"%d probe rows, %d mismatches", tally.cases, tally.steps, tally.results, nonNull, tally.errors,
		tally.statements, tally.probeRows, tally.mismatches)
}
