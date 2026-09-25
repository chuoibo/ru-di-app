package aigate

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	aimetrics "mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/avatarfeed"
	"mobile/services/core/internal/chatassist"
	"mobile/services/core/internal/chatlegacychange"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/jobs"
)

// Slice 10 puts a database trigger on the job table: every write that makes
// a job due also writes a row into job_outbox, in the same transaction. The
// Go walk above cannot see that write -- it lives in SQL, fired by the
// database -- so a root that writes chat_ai_invocations writes job_outbox too
// without a single Go string naming it. A trigger must not become the way
// around the read gate (design 02 §3.2), so the tables such triggers reach are
// declared here, per root, with the reason each cannot carry context, and the
// gate reads the triggers out of the migrations the binary embeds.
//
// The SQL read is every migration the Go binary embeds that installs a
// trigger (TestTriggerGateReadsEveryGoTrigger holds it to that): the chat AI
// schema and the outbox, and also the legacy change feed, the avatar feed,
// chat v2 and the engine's metrics. Alembic's triggers are not the Go
// binary's and are not read.
//
// job_outbox (internal/jobs, ADR-0038 proposed): the queue's outbox. A row is
// (queue, the job's id, its enqueue number, due, expires, published): no
// payload column, no person column, no text a CHECK does not close
// (TestJobOutboxHoldsNoFreeText). Only an id ever reaches the broker. And it
// is written, never read, on these paths: the relay in internal/jobs reads it.
//
// chat_legacy_changes and chat_legacy_change_outbox (internal/chatlegacychange):
// the legacy room's change feed. The group root posts its answer card into
// messages, and the capture trigger on messages numbers that change: (room,
// sequence, entity kind, entity id, revision, deleted) and a wake row -- the
// same rows every Go chat write produces, no text a CHECK does not close
// (TestTriggerReachedTablesHoldNoFreeText). The feed's head row the root
// already writes in Go (lockFeed), so the Go gates see that one.
var (
	nepViaTrigger   = map[string]string{"job_outbox": "the queue's outbox: ids, sequence numbers and timestamps only; written by the enqueue trigger, read only by the relay"}
	groupViaTrigger = map[string]string{
		"job_outbox":                "the queue's outbox: ids, sequence numbers and timestamps only; written by the enqueue trigger, read only by the relay",
		"chat_legacy_changes":       "the legacy change feed: which message of which room changed, numbered; written by the capture trigger when the answer card is posted",
		"chat_legacy_change_outbox": "the change feed's wake rows: room and sequence only; written by the same capture trigger",
	}
)

var (
	sqlComment  = regexp.MustCompile(`(?m)--.*$`)
	sqlFunction = regexp.MustCompile(`(?is)CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+([a-z_][a-z0-9_]*)\s*\(.*?\$\$(.*?)\$\$`)
	sqlTrigger  = regexp.MustCompile(`(?is)CREATE\s+TRIGGER\s+([a-z_][a-z0-9_]*)\s+(?:BEFORE|AFTER|INSTEAD\s+OF)\s+[^;]*?\bON\s+([a-z_][a-z0-9_]*)[^;]*?EXECUTE\s+(?:FUNCTION|PROCEDURE)\s+([a-z_][a-z0-9_]*)`)
	sqlWrite    = regexp.MustCompile(`(?i)\b(?:insert\s+into|update|delete\s+from)\s+([a-z_][a-z0-9_]*)`)
	sqlCall     = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*)\s*\(`)
)

// triggerWrites reads the tables that the triggers on table write, followed
// through every function they call that the same SQL defines. Later
// definitions of a function replace earlier ones (CREATE OR REPLACE across
// versions).
func triggerWrites(sqls []string, table string) map[string][]string {
	bodies := map[string]string{}
	var rest []string
	for _, s := range sqls {
		s = sqlComment.ReplaceAllString(s, "")
		for _, m := range sqlFunction.FindAllStringSubmatch(s, -1) {
			bodies[strings.ToLower(m[1])] = m[2]
		}
		rest = append(rest, sqlFunction.ReplaceAllString(s, ""))
	}
	out := map[string][]string{}
	for _, m := range sqlTrigger.FindAllStringSubmatch(strings.Join(rest, "\n"), -1) {
		if !strings.EqualFold(m[2], table) {
			continue
		}
		trigger := strings.ToLower(m[1])
		seen := map[string]bool{}
		todo := []string{strings.ToLower(m[3])}
		for len(todo) > 0 {
			fn := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[fn] {
				continue
			}
			seen[fn] = true
			body, ok := bodies[fn]
			if !ok {
				continue
			}
			for _, w := range sqlWrite.FindAllStringSubmatch(body, -1) {
				// ON CONFLICT ... DO UPDATE SET writes the INSERT's table,
				// already counted; "set" is not a table.
				if strings.EqualFold(w[1], "set") {
					continue
				}
				out[strings.ToLower(w[1])] = append(out[strings.ToLower(w[1])], trigger)
			}
			for _, c := range sqlCall.FindAllStringSubmatch(body, -1) {
				if _, known := bodies[strings.ToLower(c[1])]; known {
					todo = append(todo, strings.ToLower(c[1]))
				}
			}
		}
	}
	return out
}

func schemaSQL() []string {
	out := append(chatassist.SchemaFiles(), jobs.SchemaSQL(), chatlegacychange.SchemaSQL(), chatv2.SchemaSQL(), aimetrics.SchemaSQL())
	return append(out, avatarfeed.SchemaFiles()...)
}

// Every SQL file under internal/ that creates a trigger is read by the gate:
// a new package with a trigger on a table the AI roots write cannot slip in
// beside it. Compared by content, so the file read is the one embedded.
func TestTriggerGateReadsEveryGoTrigger(t *testing.T) {
	read := map[string]bool{}
	for _, s := range schemaSQL() {
		read[s] = true
	}
	seen := 0
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".sql" {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !regexp.MustCompile(`(?i)\bCREATE\s+(OR\s+REPLACE\s+)?TRIGGER\b`).Match(sqlComment.ReplaceAll(raw, nil)) {
			return nil
		}
		seen++
		if !read[string(raw)] {
			t.Errorf("%s creates a trigger the gate does not read", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 7 {
		t.Fatalf("found %d SQL files with a trigger; the walk slipped", seen)
	}
}

// The tables the triggers reach hold no free text either: each column an id,
// a number, a timestamp or text a CHECK closes, read from the migration that
// creates the table.
func TestTriggerReachedTablesHoldNoFreeText(t *testing.T) {
	all := strings.Join(schemaSQL(), "\n")
	all = sqlComment.ReplaceAllString(all, "")
	all = regexp.MustCompile(`CHECK\s*\(`).ReplaceAllString(all, "CHECK (")
	tables := map[string]bool{}
	for _, m := range []map[string]string{nepViaTrigger, groupViaTrigger} {
		for table := range m {
			tables[table] = true
		}
	}
	for table := range tables {
		body := regexp.MustCompile(`(?s)CREATE TABLE ` + table + ` \((.*?)\n\);`).FindStringSubmatch(all)
		if body == nil {
			t.Errorf("cannot find CREATE TABLE %s in the embedded migrations", table)
			continue
		}
		n := 0
		for _, line := range strings.Split(body[1], "\n") {
			def := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
			if def == "" || strings.HasPrefix(def, "UNIQUE") || strings.HasPrefix(def, "PRIMARY KEY") || strings.HasPrefix(def, "FOREIGN KEY") {
				continue
			}
			n++
			if columnMayHoldText(def) {
				t.Errorf("%s column can hold free text: %s", table, def)
			}
		}
		if n < 3 {
			t.Errorf("%s: read %d columns; the parse slipped", table, n)
		}
	}
}

// writes lists the tables a closure's SQL writes.
func writes(strs []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range strs {
		if !sqlLike.MatchString(s) {
			continue
		}
		for _, w := range sqlWrite.FindAllStringSubmatch(s, -1) {
			out[strings.ToLower(w[1])] = true
		}
	}
	return out
}

// undeclared returns, sorted, what the triggers reach from the tables a root
// writes and its declaration does not name, and what it names that no
// trigger reaches (a stale entry widens the gate for nothing). A table the
// root already writes in Go is not new: the Go gates see that write.
func undeclared(written map[string]bool, declared map[string]string, sqls []string) (missing, stale []string) {
	reached := map[string]bool{}
	for table := range written {
		for t := range triggerWrites(sqls, table) {
			if !written[t] {
				reached[t] = true
			}
		}
	}
	for t := range reached {
		if _, ok := declared[t]; !ok {
			missing = append(missing, t)
		}
	}
	for t := range declared {
		if !reached[t] {
			stale = append(stale, t)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	return missing, stale
}

func TestTriggerWritesOfTheAIRootsAreDeclared(t *testing.T) {
	sqls := schemaSQL()
	found := triggerWrites(sqls, "chat_ai_invocations")
	if len(found["job_outbox"]) != 1 || found["job_outbox"][0] != "chat_ai_enqueue" {
		t.Fatalf("triggers on chat_ai_invocations reach %v; the parse slipped or the enqueue trigger moved", found)
	}
	g := load(t)
	var nep []*types.Func
	for _, r := range nepRoots {
		nep = append(nep, g.root(t, r))
	}
	var group []*types.Func
	for name, f := range g.byName {
		if strings.HasPrefix(name, "(*"+pkgChat+".Handler).") || strings.HasPrefix(name, pkgChat+".") {
			group = append(group, f)
		}
	}
	for _, root := range []struct {
		name     string
		funcs    []*types.Func
		declared map[string]string
	}{{"Nếp", nep, nepViaTrigger}, {"group", group, groupViaTrigger}} {
		w := writes(g.reach(root.funcs...).strings)
		if !w["chat_ai_invocations"] {
			t.Fatalf("%s: the closure writes no chat_ai_invocations; the walk is broken", root.name)
		}
		missing, stale := undeclared(w, root.declared, sqls)
		if len(missing) > 0 {
			t.Errorf("%s: a trigger on a table this root writes reaches %v, which is not declared", root.name, missing)
		}
		if len(stale) > 0 {
			t.Errorf("%s: declared %v, which no trigger reaches", root.name, stale)
		}
	}
}

// Canaries: a trigger that reaches a new table through a helper function is
// seen and refused undeclared; the real schema, identity, passes.
func TestTriggerWritesGateCanRed(t *testing.T) {
	sneaky := `CREATE FUNCTION helper(x uuid) RETURNS void LANGUAGE plpgsql AS $$ BEGIN INSERT INTO person_interests(person_id) VALUES (x); END $$;
-- a comment naming DELETE FROM decoy must not count
CREATE FUNCTION on_job() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM helper(NEW.person_id); RETURN NULL; END $$;
CREATE TRIGGER on_job AFTER UPDATE ON chat_ai_invocations FOR EACH ROW EXECUTE FUNCTION on_job();`
	sqls := append(schemaSQL(), sneaky)
	missing, _ := undeclared(map[string]bool{"chat_ai_invocations": true}, nepViaTrigger, sqls)
	if strings.Join(missing, ",") != "person_interests" {
		t.Fatalf("a trigger reaching person_interests through a helper went unseen: %v", missing)
	}
	missing, stale := undeclared(map[string]bool{"chat_ai_invocations": true}, nepViaTrigger, schemaSQL())
	if len(missing)+len(stale) != 0 {
		t.Fatalf("identity: missing %v stale %v", missing, stale)
	}
	if missing, _ := undeclared(map[string]bool{"chat_ai_invocations": true}, map[string]string{}, schemaSQL()); strings.Join(missing, ",") != "job_outbox" {
		t.Fatalf("an empty declaration did not miss job_outbox: %v", missing)
	}
	// A root that posts into messages reaches the legacy change feed through
	// its capture trigger (review of slice 10: the gate once read only the
	// chat AI and outbox SQL, and could not see it).
	posts := map[string]bool{"chat_ai_invocations": true, "messages": true, "chat_legacy_change_heads": true}
	if missing, _ := undeclared(posts, nepViaTrigger, schemaSQL()); strings.Join(missing, ",") != "chat_legacy_change_outbox,chat_legacy_changes" {
		t.Fatalf("posting a message without declaring the change feed: missing %v", missing)
	}
}

// In Go, only internal/jobs writes job_outbox (the relay marks rows, the
// cleanup deletes them); every other way in is the enqueue trigger, above.
// Each function's own strings and constants are read, not its closure: the
// worker reaching jobs.Don through the registry is jobs writing, not the
// worker.
func TestOnlyJobsWritesJobOutboxInGo(t *testing.T) {
	g := load(t)
	seen := 0
	for f, decl := range g.decl {
		info := g.info[f]
		ast.Inspect(decl, func(n ast.Node) bool {
			var s string
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					s, _ = strconv.Unquote(x.Value)
				}
			case *ast.Ident:
				if c, ok := info.Uses[x].(*types.Const); ok && c.Val().Kind() == constant.String {
					s = constant.StringVal(c.Val())
				}
			}
			if s == "" || !sqlLike.MatchString(s) {
				return true
			}
			for _, w := range sqlWrite.FindAllStringSubmatch(s, -1) {
				if !strings.EqualFold(w[1], "job_outbox") {
					continue
				}
				seen++
				if f.Pkg().Path() != "mobile/services/core/internal/jobs" {
					t.Errorf("%s writes job_outbox: %q", f.FullName(), s)
				}
			}
			return true
		})
	}
	if seen < 2 {
		t.Fatalf("found %d writes of job_outbox; the relay's UPDATE and Don's DELETE are two, so the scan slipped", seen)
	}
}

// The outbox cannot hold words: every column an id, a number, a timestamp, or
// text a CHECK closes.
func TestJobOutboxHoldsNoFreeText(t *testing.T) {
	sql := sqlComment.ReplaceAllString(jobs.SchemaSQL(), "")
	body := regexp.MustCompile(`(?s)CREATE TABLE job_outbox \((.*?)\n\);`).FindStringSubmatch(sql)
	if body == nil {
		t.Fatal("cannot find CREATE TABLE job_outbox in the embedded migration")
	}
	n := 0
	for _, line := range strings.Split(body[1], "\n") {
		def := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
		if def == "" || strings.HasPrefix(def, "UNIQUE") || strings.HasPrefix(def, "PRIMARY KEY") {
			continue
		}
		n++
		if columnMayHoldText(def) {
			t.Errorf("job_outbox column can hold free text: %s", def)
		}
	}
	if n != 8 {
		t.Fatalf("read %d columns, want 8; the parse slipped", n)
	}
}
