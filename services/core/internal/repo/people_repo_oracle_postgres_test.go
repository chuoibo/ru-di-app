//go:build postgres

package repo

// Differential test of the W10 repository methods (people, bookmarks, blocks,
// conversations and ending an account) against the real
// SqlAlchemyApiRepository, driven through scripts/render_people_repo_oracle.py.
// Same design as pair_repo_oracle_postgres_test.go (case format, value tags,
// statement normalisation, conflict tagging, row lock probe, route steps,
// coverage), with three additions:
//
//   - A photo store per side. Every case owns a directory under a Python root,
//     bind-mounted into the container at its own path, and one under a Go
//     root, both seeded here with the same files. route.delete_own_account
//     unlinks through the real PhotoStorage in Python and storage.PhotoStorage
//     in Go, and a `media tree` probe compares the two trees after the step.
//   - Storage deletes and the service's log lines are statements (the Python
//     driver writes them into its log), so the order of files against SQL,
//     a missing file, an unlinkable one and a malformed key are compared.
//   - Erasure steps dump every table of the schema, strings equal to the
//     transaction's now() written as <transaction-now>, so a table the port
//     forgets, or touches by mistake, is a mismatch rather than a gap.
//
// TestPeopleRowLocksSerialiseRequests is Go only: the races the Python
// branches name (two bookmarks, two pairs, two blocks) and the locks erasure
// holds.
//
// Without CORE_PYTHON_IMAGE both tests skip; scripts/go_postgres_tier.sh sets
// it and refuses skips. CORE_ORACLE_PYTHON_OUT, when set, names a file the raw
// Python answer is written to.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/media/storage"
	"mobile/services/core/internal/testdb"
)

const mediaTreePrefix = "SELECT 'media tree "

func mediaTreeProbe(index int) string {
	return fmt.Sprintf("SELECT 'media tree %d' WHERE false", index)
}

// probePeopleRowLocks lists the row locks held on every table a W10 method
// locks, updates, deletes or references through a foreign key it writes.
var probePeopleRowLocks = func() string {
	var parts []string
	for _, t := range []struct{ table, key string }{
		{"people", "t.id::text"}, {"contexts", "t.id::text"}, {"memberships", "t.id::text"},
		{"friend_requests", "t.id::text"}, {"account_sessions", "t.id::text"}, {"saved_places", "t.id::text"},
		{"uploaded_images", "t.id::text"}, {"posts", "t.id::text"}, {"post_comments", "t.id::text"},
		{"post_reactions", "t.id::text"}, {"stories", "t.id::text"},
		{"story_views", "t.story_id::text || '/' || t.viewer_id::text"}, {"person_interests", "t.id::text"},
		{"context_read_marks", "t.context_id::text || '/' || t.person_id::text"},
		{"pair_paper_views", "t.paper_id::text || '/' || t.version || '/' || t.person_id::text"},
		{"pair_shared_constraints", "t.cycle_id::text || '/' || t.owner_id::text || '/' || t.kind"},
		{"active_couple_members", "t.person_id::text"}, {"account_identities", "t.id::text"},
		{"messages", "t.id::text"}, {"pair_notebook_cycles", "t.id::text"}, {"pair_papers", "t.id::text"},
		{"outings", "t.id::text"}, {"memories", "t.id::text"},
	} {
		parts = append(parts, `SELECT '`+t.table+`' AS rel, `+t.key+` AS id, array_to_string(r.modes, ',') AS modes
			FROM public.pgrowlocks('`+t.table+`') r LEFT JOIN `+t.table+` t ON t.ctid = r.locked_row`)
	}
	return `SELECT l.rel || ' ' || coalesce(l.id, '(superseded row)') || ' ' || l.modes FROM (` +
		strings.Join(parts, " UNION ALL ") + `) l ORDER BY 1`
}()

// peopleDump dumps one whole table as JSON objects: every string that spells
// the transaction's now() becomes <transaction-now>, fixture rows sort first,
// then rows by their content without the id, then by the whole row.
const peopleDumpPrefix = `SELECT (SELECT coalesce(jsonb_object_agg(`

func peopleDump(table string) string {
	return peopleDumpPrefix + `e.key, CASE WHEN jsonb_typeof(e.value) = 'string' ` +
		`AND (e.value #>> '{}') ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?[+-][0-9]{2}:[0-9]{2}$' ` +
		`AND (e.value #>> '{}')::timestamptz = now() THEN to_jsonb('` + transactionNow + `'::text) ELSE e.value END), ` +
		`'{}'::jsonb) FROM jsonb_each(to_jsonb(t)) AS e)::text FROM ` + table + ` t ORDER BY ` +
		`(coalesce(to_jsonb(t) ->> 'id', '') NOT LIKE '%-aaaa-4aaa-8aaa-aaaaaaaa%'), (to_jsonb(t) - 'id')::text, ` +
		`to_jsonb(t)::text`
}

// mediaSeed is what a case's photo store holds before its first step: a file
// for each of files, a directory where the file of each of dirs would be, and
// the key directory of each of readonly made 0555.
type mediaSeed struct {
	files, dirs, readonly []string
}

type peopleCase struct {
	socialCase
	media mediaSeed
}

// ---------------------------------------------------------------------------
// Tags and the Go side
// ---------------------------------------------------------------------------

func tLastMessage(m LastMessage) any {
	return tRecord("LastMessageRecord", "id", tUUID(m.ID), "kind", tStr(m.Kind), "preview", tStr(m.Preview),
		"author_id", optional(m.AuthorID, tUUID), "author_display_name", optional(m.AuthorDisplayName, tStr),
		"created_at", tInstant(m.CreatedAt))
}

func tContextSummary(s PersonContextSummary) any {
	return tRecord("PersonContextSummaryRecord", "id", tUUID(s.ID), "display_name", tStr(s.DisplayName),
		"member_count", tInt(s.MemberCount), "my_role", tStr(s.MyRole), "my_state", tStr(s.MyState),
		"membership_id", tUUID(s.MembershipID), "joined_at", optional(s.JoinedAt, tInstant),
		"last_message", optional(s.LastMessage, tLastMessage), "unread_count", tInt(s.UnreadCount),
		"theme", tStr(s.Theme), "kind", tStr(s.Kind), "counterpart_id", optional(s.CounterpartID, tUUID),
		"counterpart_display_name", optional(s.CounterpartDisplayName, tStr))
}

func tSavedPlace(s SavedPlace) any {
	return tRecord("SavedPlaceRecord", "id", tUUID(s.ID), "person_id", tUUID(s.PersonID), "place_id", tStr(s.PlaceID),
		"created_at", tInstant(s.CreatedAt))
}

func tErasureReport(r ErasureReport) any {
	pairs := []any{}
	for _, c := range r.Counts {
		pairs = append(pairs, []any{tStr(c.Table), tInt(c.Rows)})
	}
	return tRecord("ErasureReport", "counts", tv("dict", pairs), "storage_keys", tStrings(r.StorageKeys))
}

func peopleGoCall(repo Repository, rec *recorder, method string, a map[string]any, media string) (any, error) {
	s := func(key string) string { return argString(a, key) }
	now := func() time.Time { return argInstant(s("now")) }
	if strings.HasPrefix(method, "route.") {
		return nil, peopleRouteGo(repo, rec, method, a, media)
	}
	switch method {
	case "create_person":
		p, err := repo.CreatePerson(bg, s("person_id"), s("display_name"))
		return tPerson(&p), err
	case "rename_person":
		p, err := repo.RenamePerson(bg, s("person_id"), s("display_name"))
		return tPerson(p), err
	case "are_friends":
		ok, err := repo.AreFriends(bg, s("a"), s("b"))
		return tBool(ok), err
	case "share_active_context":
		ok, err := repo.ShareActiveContext(bg, s("a"), s("b"))
		return tBool(ok), err
	case "same_couple":
		ok, err := repo.SameCouple(bg, s("a"), s("b"))
		return tBool(ok), err
	case "profile_counts":
		c, err := repo.ProfileCounts(bg, s("person_id"))
		return tRecord("ProfileCounts", "friends", tInt(c.Friends), "contexts", tInt(c.Contexts),
			"outings", tInt(c.Outings), "places_checked_in", tInt(c.PlacesCheckedIn), "memories", tInt(c.Memories)), err
	case "list_login_providers":
		providers, err := repo.ListLoginProviders(bg, s("person_id"))
		return tStrings(providers), err
	case "get_pair_context":
		c, err := repo.GetPairContext(bg, s("pair_key"))
		return tContext(c), err
	case "create_pair_context":
		c, err := repo.CreatePairContext(bg, PairContextInput{PairKey: s("pair_key"), MemberIDs: argStrings(a["member_ids"]),
			CreatedByID: s("created_by_id"), Now: now()})
		return tContext(&c), err
	case "list_person_context_summaries":
		rows, err := repo.ListPersonContextSummaries(bg, s("person_id"))
		items := []any{}
		for _, row := range rows {
			items = append(items, tContextSummary(row))
		}
		return tSeq(items), err
	case "count_unread_messages":
		n, err := repo.CountUnreadMessages(bg, s("context_id"), s("person_id"))
		return tInt(n), err
	case "list_saved_places":
		rows, err := repo.ListSavedPlaces(bg, s("person_id"))
		items := []any{}
		for _, row := range rows {
			items = append(items, tSavedPlace(row))
		}
		return tSeq(items), err
	case "save_place":
		row, created, err := repo.SavePlace(bg, s("person_id"), s("place_id"), now())
		return tSeq([]any{tSavedPlace(row), tBool(created)}), err
	case "unsave_place":
		removed, err := repo.UnsavePlace(bg, s("person_id"), s("place_id"))
		return tBool(removed), err
	case "open_block_edge":
		e, err := repo.OpenBlockEdge(bg, s("blocker_id"), s("addressee_id"), now())
		return tFriendEdge(e), err
	case "lift_block_edge":
		e, err := repo.LiftBlockEdge(bg, s("blocker_id"), s("addressee_id"), now())
		return tFriendEdge(e), err
	case "list_blocked":
		edges, err := repo.ListBlocked(bg, s("person_id"))
		return tFriendEdges(edges), err
	case "revoke_all_account_sessions":
		n, err := repo.RevokeAllAccountSessions(bg, s("person_id"), now())
		return tInt(n), err
	case "erase_person":
		report, err := repo.ErasePerson(bg, s("person_id"), now())
		return tErasureReport(report), err
	}
	return photoGoCall(repo, method, a)
}

// peopleGoError is pairRepoGoError plus the two exceptions these methods
// raise outside PostgreSQL.
func peopleGoError(err error) map[string]any {
	plain := func(kind string) map[string]any {
		return map[string]any{"type": kind, "sqlstate": nil, "constraint": nil, "code": nil, "cause": nil}
	}
	switch {
	case errors.Is(err, storage.ErrInvalidKey):
		return plain("ValueError")
	case errors.Is(err, ErrSavedPlaceVanished):
		return plain("AssertionError")
	}
	return pairRepoGoError(err)
}

// mediaTree is the Python driver's `_tree`: one line per entry under root.
func mediaTree(t *testing.T, root string) []string {
	t.Helper()
	rows := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		mode := info.Sys().(*syscall.Stat_t).Mode
		kind, size := "?", "-"
		switch mode & syscall.S_IFMT {
		case syscall.S_IFDIR:
			kind = "d"
		case syscall.S_IFLNK:
			kind = "l"
		case syscall.S_IFREG:
			kind, size = "f", strconv.FormatInt(info.Size(), 10)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rows = append(rows, fmt.Sprintf("%s %s %s 0o%o", rel, kind, size, mode&0o7777))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	return rows
}

func runPeopleGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase, media string) []any {
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
		value, err := peopleGoCall(Repository{Q: rec}, rec, step.Call, step.Args, media)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = peopleGoError(err)
			steps = append(steps, generic(t, out))
			break
		}
		out["result"] = value
		probes := []any{}
		for _, sql := range step.Probes {
			if strings.HasPrefix(sql, mediaTreePrefix) {
				index := strings.SplitN(sql[len(mediaTreePrefix):], "'", 2)[0]
				probes = append(probes, mediaTree(t, filepath.Join(media, index)))
				continue
			}
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

// normalizePeopleSteps is normalizePairSteps for probePeopleRowLocks.
func normalizePeopleSteps(steps []any, c oracleCase) []any {
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
	}
	out := make([]any, len(steps))
	for i, step := range steps {
		out[i] = step
		if statements, ok := step.(map[string]any)["statements"].([]any); ok {
			step.(map[string]any)["statements"] = normalizePeopleStatements(statements)
		}
		probes, _ := step.(map[string]any)["probes"].([]any)
		for j, probe := range c.Steps[i].Probes {
			if j >= len(probes) {
				continue
			}
			rows, ok := probes[j].([]any)
			if !ok {
				continue
			}
			switch probe {
			case probePeopleRowLocks:
				masked := make([]string, len(rows))
				for k, row := range rows {
					masked[k] = uuidText.ReplaceAllStringFunc(row.(string), func(id string) string {
						if known[id] {
							return id
						}
						return "<generated>"
					})
				}
				sort.Strings(masked)
				sorted := make([]any, len(masked))
				for k, row := range masked {
					sorted[k] = row
				}
				probes[j] = sorted
			case probeNow:
				if len(rows) == 1 {
					if now, ok := rows[0].(string); ok && now != "" {
						out[i] = replaceText(out[i], now, transactionNow)
					}
				}
			}
		}
	}
	return out
}

// normalizePeopleStatements removes, on both sides alike, two orders the
// Python statement log does not keep from one run to the next (measured: two
// runs of the same image disagreed on both):
//   - the RETURNING list of an ORM DELETE on a composite primary key
//     (story_views in erase_person): SQLAlchemy builds it from a set of Column
//     objects, whose iteration order follows their addresses in the process;
//   - the order of list_person_context_summaries's per-conversation
//     count_unread_messages pairs (the read mark, then the COUNT, whose text
//     depends on the mark): it follows the rows of an unordered JOIN, which
//     the planner and the heap decide. The pairs are compared as a multiset.
func normalizePeopleStatements(statements []any) []any {
	out := append([]any{}, statements...)
	for i, s := range out {
		text, _ := s.(string)
		if at := strings.Index(text, " RETURNING "); strings.HasPrefix(text, "DELETE FROM ") && at >= 0 {
			columns := strings.Split(text[at+len(" RETURNING "):], ", ")
			sort.Strings(columns)
			out[i] = text[:at] + " RETURNING " + strings.Join(columns, ", ")
		}
	}
	const markPrefix = "SELECT context_read_marks.context_id AS context_read_marks_context_id"
	const countPrefix = "SELECT count(*) AS count_1 FROM messages WHERE messages.context_id"
	isPair := func(k int) bool {
		if k+1 >= len(out) {
			return false
		}
		mark, _ := out[k].(string)
		count, _ := out[k+1].(string)
		return strings.HasPrefix(mark, markPrefix) && strings.HasPrefix(count, countPrefix)
	}
	for i := 0; i < len(out); {
		if !isPair(i) {
			i++
			continue
		}
		var pairs [][2]any
		j := i
		for ; isPair(j); j += 2 {
			pairs = append(pairs, [2]any{out[j], out[j+1]})
		}
		sort.SliceStable(pairs, func(a, b int) bool { return pairs[a][1].(string) < pairs[b][1].(string) })
		for k, pair := range pairs {
			out[i+2*k], out[i+2*k+1] = pair[0], pair[1]
		}
		i = j
	}
	return out
}

// logPeopleDifferences names the first differing statement and the first
// differing row of each differing probe, after the same id binding
// compareCase does, so a mismatch in a long dump can be read.
func logPeopleDifferences(t *testing.T, c oracleCase, python, golang []any) {
	t.Helper()
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
	}
	pb := &binder{known: known, names: map[string]string{}}
	gb := &binder{known: known, names: map[string]string{}}
	at := func(items []any, k int) any {
		if k < len(items) {
			return items[k]
		}
		return "<none>"
	}
	for i := 0; i < len(python) && i < len(golang); i++ {
		p := pb.walk(python[i]).(map[string]any)
		g := gb.walk(golang[i]).(map[string]any)
		ps, _ := p["statements"].([]any)
		gs, _ := g["statements"].([]any)
		for k := 0; k < max(len(ps), len(gs)); k++ {
			if !reflect.DeepEqual(at(ps, k), at(gs, k)) {
				t.Logf("step %d (%s) statement %d\n  python: %v\n  go:     %v", i, c.Steps[i].Call, k, at(ps, k), at(gs, k))
				break
			}
		}
		pp, _ := p["probes"].([]any)
		gp, _ := g["probes"].([]any)
		for k := 0; k < len(pp) && k < len(gp); k++ {
			if reflect.DeepEqual(pp[k], gp[k]) {
				continue
			}
			rowsP, _ := pp[k].([]any)
			rowsG, _ := gp[k].([]any)
			probe := c.Steps[i].Probes[k]
			if len(probe) > 90 {
				probe = "..." + probe[len(probe)-90:]
			}
			for r := 0; r < max(len(rowsP), len(rowsG)); r++ {
				if !reflect.DeepEqual(at(rowsP, r), at(rowsG, r)) {
					t.Logf("step %d probe %d (%s) row %d of %d/%d\n  python: %v\n  go:     %v", i, k, probe, r,
						len(rowsP), len(rowsG), at(rowsP, r), at(rowsG, r))
					break
				}
			}
		}
		if !reflect.DeepEqual(p["result"], g["result"]) {
			t.Logf("step %d result\n  python: %s\n  go:     %s", i, clipJSON(p["result"]), clipJSON(g["result"]))
		}
	}
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func reversedKey(key string) string {
	parts := strings.SplitN(key, ":", 2)
	return parts[1] + ":" + parts[0]
}

func peopleRepoOracleCases(tables []string) ([]peopleCase, oracleSpec) {
	w := newPeopleWorld()
	var cases []peopleCase
	add := func(name, wantEnd string, setup []string, media mediaSeed, steps ...oracleCall) {
		cases = append(cases, peopleCase{socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps},
			wantEnd}, media})
	}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	// probeNow on every step: a read after a write in the same case returns a
	// server-default created_at, which is the transaction's now().
	readProbes := []string{probeLocks, probeWrites, probePeopleRowLocks, probeNow}
	dumps := func(names ...string) []string {
		out := []string{probeLocks, probeWrites, probePeopleRowLocks}
		for _, name := range names {
			out = append(out, peopleDump(name))
		}
		return append(out, probeNow)
	}
	everything := dumps(tables...)
	step := func(probes []string, call string, pairs ...any) oracleCall {
		return oracleCall{Call: call, Args: args(pairs...), Before: append([]string{}, writesBaseline...),
			Probes: append([]string{}, probes...)}
	}
	read := func(call string, pairs ...any) oracleCall { return step(readProbes, call, pairs...) }
	base := w.sql
	vietnam := join([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base)
	const now = peopleNow
	later := func(n int) string { return fmt.Sprintf("2030-10-01T05:%02d:00.123456Z", n) }
	const vietnamNow = "2030-10-01T12:00:00.654321+07:00"
	missing := w.missingPerson
	none := mediaSeed{}

	// --- people ------------------------------------------------------------------------
	people := dumps("people")
	add("create_person: a new id, then read back", "", base, none,
		step(people, "create_person", "person_id", w.newcomer, "display_name", "Tên mới 🌿 ' \" (dữ liệu mẫu)"),
		read("get_person", "person_id", w.newcomer))
	add("create_person: an id that is already a person", "PERSON_ALREADY_EXISTS", base, none,
		step(people, "create_person", "person_id", w.an, "display_name", "An lần hai (dữ liệu mẫu)"))
	add("create_person: under a Vietnam session TimeZone", "", vietnam, none,
		step(people, "create_person", "person_id", w.newcomer, "display_name", "Tên (dữ liệu mẫu)"))
	add("rename_person: a new name, the same name, a missing person", "", base, none,
		step(people, "rename_person", "person_id", w.ban, "display_name", "Tên khác (dữ liệu mẫu)"),
		step(people, "rename_person", "person_id", w.ban, "display_name", "Tên khác (dữ liệu mẫu)"),
		step(people, "rename_person", "person_id", missing, "display_name", "Không ai (dữ liệu mẫu)"))
	add("rename_person: an ended account", "", base, none,
		step(people, "rename_person", "person_id", w.erased, "display_name", "Hồi sinh (dữ liệu mẫu)"))
	var relations []oracleCall
	for _, pair := range [][2]string{{w.an, w.binh}, {w.binh, w.an}, {w.an, w.ban}, {w.an, w.chi}, {w.an, w.dung},
		{w.an, w.hang}, {w.an, w.stranger}, {w.binh, w.erased}, {w.an, w.an}, {w.an, missing}} {
		relations = append(relations, read("are_friends", "a", pair[0], "b", pair[1]))
	}
	for _, pair := range [][2]string{{w.an, w.binh}, {w.binh, w.an}, {w.an, w.chi}, {w.chi, w.binh}, {w.an, w.ban},
		{w.binh, w.erased}, {w.hang, w.ban}, {w.hang, w.an}, {w.an, w.an}, {w.moi, w.moi}, {w.an, missing}} {
		relations = append(relations, read("share_active_context", "a", pair[0], "b", pair[1]))
	}
	for _, pair := range [][2]string{{w.an, w.binh}, {w.binh, w.an}, {w.an, w.an}, {w.an, missing}, {w.an, w.stranger}} {
		relations = append(relations, read("same_couple", "a", pair[0], "b", pair[1]))
	}
	add("are_friends and share_active_context: every relation", "", base, none, relations...)
	var counts []oracleCall
	for _, person := range []string{w.an, w.binh, w.chi, w.erased, w.stranger, missing} {
		counts = append(counts, read("profile_counts", "person_id", person), read("list_login_providers", "person_id", person))
	}
	add("profile_counts and list_login_providers", "", base, none, counts...)

	// --- conversations ---------------------------------------------------------------------
	add("get_pair_context: a pair, its key reversed, empty, unknown", "", base, none,
		read("get_pair_context", "pair_key", w.pairKeyAnBan), read("get_pair_context", "pair_key", reversedKey(w.pairKeyAnBan)),
		read("get_pair_context", "pair_key", ""), read("get_pair_context", "pair_key", "khong-co-cap-nay"))
	conversation := dumps("contexts", "memberships")
	createPair := func(key string, members []any, by string) oracleCall {
		return step(conversation, "create_pair_context", "pair_key", key, "member_ids", members, "created_by_id", by, "now", now)
	}
	add("create_pair_context: two friends, then read back", "", base, none,
		createPair(w.pairKeyBinhDung, []any{w.binh, w.dung}, w.binh), read("get_pair_context", "pair_key", w.pairKeyBinhDung))
	add("create_pair_context: a key already taken", "PAIR_EXISTS", base, none,
		createPair(w.pairKeyAnBan, []any{w.an, w.ban}, w.an))
	add("create_pair_context: a member with no people row", "IntegrityError", base, none,
		createPair(pairKeyOf(w.binh, missing), []any{w.binh, missing}, w.binh))
	add("create_pair_context: one person twice", "IntegrityError", base, none,
		createPair("mot-nguoi-hai-lan", []any{w.chi, w.chi}, w.chi))
	add("create_pair_context: a creator with no people row", "IntegrityError", base, none,
		createPair("nguoi-tao-khong-co", []any{w.binh, w.dung}, missing))
	add("create_pair_context: nobody in it, under a Vietnam session TimeZone", "", vietnam, none,
		createPair("cap-khong-ai", []any{}, w.moi))
	var summaries []oracleCall
	for _, person := range []string{w.an, w.binh, w.chi, w.dung, w.ban, w.hang, w.erased, w.stranger, missing} {
		summaries = append(summaries, read("list_person_context_summaries", "person_id", person))
	}
	add("list_person_context_summaries: everybody", "", base, none, summaries...)
	add("list_person_context_summaries: under a Vietnam session TimeZone", "", vietnam, none,
		read("list_person_context_summaries", "person_id", w.an))
	for _, shape := range []struct{ card, want string }{
		{`{"kind": "poll"}`, ""}, {`{"kind": "expense_draft"}`, ""}, {`{"kind": ""}`, ""}, {`{"kind": {}}`, ""},
		{`{"kind": []}`, ""}, {`{"kind": 0}`, ""}, {`{"kind": 1.5}`, ""}, {`{"kind": true}`, ""}, {`{"kind": null}`, ""},
		{`{"kind": "Poll"}`, ""}, {`{}`, ""}, {`[1]`, ""}, {`"poll"`, ""}, {`null`, ""},
		{`{"kind": ["x"]}`, "TypeError"}, {`{"kind": {"a": 1}}`, "TypeError"},
	} {
		add("list_person_context_summaries: g2's newest message is an AI card "+shape.card, shape.want,
			join(base, []string{w.g2Card(shape.card)}), none, read("list_person_context_summaries", "person_id", w.binh))
	}
	var unread []oracleCall
	for _, pair := range [][2]string{{w.g, w.an}, {w.g, w.binh}, {w.g, w.dung}, {w.pAnBan, w.ban}, {w.pAnBan, w.an},
		{w.pBinhChi, w.binh}, {w.g2, w.chi}, {w.missingContext, w.an}} {
		unread = append(unread, read("count_unread_messages", "context_id", pair[0], "person_id", pair[1]))
	}
	add("count_unread_messages: a mark, no mark, one's own messages, the AI's", "", base, none, unread...)

	// --- saved places ------------------------------------------------------------------------
	bookmarks := dumps("saved_places")
	add("list_saved_places: newest first, a tie, nobody", "", base, none,
		read("list_saved_places", "person_id", w.an), read("list_saved_places", "person_id", w.binh),
		read("list_saved_places", "person_id", w.stranger))
	// One savepoint per case: SQLAlchemy numbers savepoints per connection
	// (sa_savepoint_1, _2, ...), and a request, which opens its own, never
	// reaches a second name in these routes.
	add("save_place: new, then again", "", base, none,
		step(bookmarks, "save_place", "person_id", w.binh, "place_id", "p-c", "now", now),
		step(bookmarks, "save_place", "person_id", w.binh, "place_id", "p-c", "now", later(1)))
	add("save_place: a key the catalogue does not have", "", base, none,
		step(bookmarks, "save_place", "person_id", w.binh, "place_id", "khong-co-trong-danh-muc ' \"", "now", now))
	add("save_place: a person with no people row", "AssertionError", base, none,
		step(bookmarks, "save_place", "person_id", missing, "place_id", "p-b", "now", now))
	add("save_place: under a Vietnam session TimeZone", "", vietnam, none,
		step(bookmarks, "save_place", "person_id", w.chi, "place_id", "p-a", "now", vietnamNow))
	add("unsave_place: a bookmark, again, one never saved", "", base, none,
		step(bookmarks, "unsave_place", "person_id", w.an, "place_id", "p-b"),
		step(bookmarks, "unsave_place", "person_id", w.an, "place_id", "p-b"),
		step(bookmarks, "unsave_place", "person_id", w.stranger, "place_id", "p-b"))

	// --- blocks ----------------------------------------------------------------------------------
	edges := dumps("friend_requests")
	block := func(blocker, addressee, at string) oracleCall {
		return step(edges, "open_block_edge", "blocker_id", blocker, "addressee_id", addressee, "now", at)
	}
	lift := func(blocker, addressee, at string) oracleCall {
		return step(edges, "lift_block_edge", "blocker_id", blocker, "addressee_id", addressee, "now", at)
	}
	add("open_block_edge: a friendship, then listed", "", base, none, block(w.an, w.binh, now),
		read("list_blocked", "person_id", w.an))
	add("open_block_edge: a pending request", "", base, none, block(w.an, w.chi, now))
	add("open_block_edge: a block already standing, at its own instant", "", base, none, block(w.an, w.hang, w.hangBlockedAt))
	add("open_block_edge: a block the other person put up", "", base, none, block(w.an, w.stranger, now))
	add("open_block_edge: only a declined edge", "", base, none, block(w.an, w.dung, now))
	add("open_block_edge: no edge at all, then again later", "", base, none, block(w.hang, w.dung, now),
		block(w.hang, w.dung, later(1)))
	add("open_block_edge: a person with no people row", "EDGE_EXISTS", base, none, block(w.an, missing, now))
	add("open_block_edge: oneself", "EDGE_EXISTS", base, none, block(w.chi, w.chi, now))
	add("open_block_edge: under a Vietnam session TimeZone", "", vietnam, none, block(w.chi, w.ban, vietnamNow))
	add("lift_block_edge: a block, then again", "NOT_BLOCKED", base, none, lift(w.an, w.hang, now), lift(w.an, w.hang, later(1)))
	add("lift_block_edge: somebody else's block", "ONLY_BLOCKER_MAY_UNBLOCK", base, none, lift(w.an, w.stranger, now))
	add("lift_block_edge: a friendship", "NOT_BLOCKED", base, none, lift(w.an, w.binh, now))
	add("lift_block_edge: nothing between them", "NOT_BLOCKED", base, none, lift(w.hang, w.dung, now))
	add("lift_block_edge: at the instant it was decided", "", base, none, lift(w.chi, w.binh, w.binhChiBlockedAt))
	add("list_blocked: two, one, none", "", base, none, read("list_blocked", "person_id", w.an),
		read("list_blocked", "person_id", w.chi), read("list_blocked", "person_id", w.stranger),
		read("list_blocked", "person_id", w.binh))

	// --- ending an account ----------------------------------------------------------------------------
	sessions := dumps("account_sessions")
	add("revoke_all_account_sessions: live, revoked and expired, then again, then nobody", "", base, none,
		step(sessions, "revoke_all_account_sessions", "person_id", w.an, "now", now),
		step(sessions, "revoke_all_account_sessions", "person_id", w.an, "now", later(1)),
		step(sessions, "revoke_all_account_sessions", "person_id", missing, "now", now))
	erase := func(person, at string) oracleCall {
		return step(everything, "erase_person", "person_id", person, "now", at)
	}
	add("erase_person: the only admin of a group with money, a pair notebook and photographs", "", base, none, erase(w.an, now))
	add("erase_person: a member who is not an admin and owes money", "", base, none, erase(w.binh, now))
	add("erase_person: twice, the second later", "", base, none, erase(w.an, now), erase(w.an, later(1)))
	add("erase_person: twice at the same instant", "", base, none, erase(w.an, now), erase(w.an, now))
	add("erase_person: a person with no people row", "PERSON_NOT_FOUND", base, none, erase(missing, now))
	add("erase_person: an account that ended before", "", base, none, erase(w.erased, now))
	add("erase_person: the other half of a pair, under a Vietnam session TimeZone", "", vietnam, none, erase(w.ban, vietnamNow))
	add("erase_person: somebody with nothing", "", base, none, erase(w.moi, now))

	// --- routes -------------------------------------------------------------------------------------------
	route := func(probes []string, name, actor string, pairs ...any) oracleCall {
		return step(probes, "route."+name, append([]any{"actor_id", actor, "now", now}, pairs...)...)
	}
	onPerson := func(probes []string, name, actor, person string) oracleCall {
		return route(probes, name, actor, "person_id", person)
	}
	files := mediaSeed{files: []string{w.keyAvatar, w.keyPersonal, w.keyGroup, w.keyBinh}}
	addDelete := func(name, want string, setup []string, media mediaSeed, steps func(del func(actor, at string, confirm bool) oracleCall) []oracleCall) {
		index := len(cases)
		del := func(actor, at string, confirm bool) oracleCall {
			return step(append(append([]string{}, everything...), mediaTreeProbe(index)), "route.delete_own_account",
				"actor_id", actor, "now", at, "body", args("confirm", confirm), "media_root", fmt.Sprintf("{media}/%d", index))
		}
		add(name, want, setup, media, steps(del)...)
	}
	one := func(actor string, confirm bool) func(func(string, string, bool) oracleCall) []oracleCall {
		return func(del func(string, string, bool) oracleCall) []oracleCall {
			return []oracleCall{del(actor, now, confirm)}
		}
	}

	contextsOf := func(actor string) oracleCall { return route(readProbes, "list_my_contexts", actor) }
	add("route GET /people/me/contexts: groups, pairs, a blocked pair, a pair whose other half left", "", base, none,
		contextsOf(w.an), contextsOf(w.binh), contextsOf(w.chi), contextsOf(w.ban), contextsOf(w.hang),
		contextsOf(w.stranger), contextsOf(w.erased))
	add("route GET /people/me/contexts: under a Vietnam session TimeZone", "", vietnam, none, contextsOf(w.an))
	add("route GET /people/me: counts, providers and tastes", "", base, none, route(readProbes, "get_my_profile", w.an),
		route(readProbes, "get_my_profile", w.stranger), route(readProbes, "get_my_profile", w.erased))
	add("route GET /people/me: no people row", "404:person_not_found", base, none, route(readProbes, "get_my_profile", missing))
	add("route PATCH /people/me: a trimmed name, a cleared bio, a city", "", base, none,
		route(people, "update_my_profile", w.an, "body", args("display_name", "  An mới (dữ liệu mẫu)  ", "bio", "   ",
			"city", " Huế (dữ liệu mẫu) ")))
	add("route PATCH /people/me: who may comment and who may find me", "", base, none,
		route(people, "update_my_profile", w.binh, "body", args("wall_comment_policy", "nobody", "discoverable_by_phone", false)))
	add("route PATCH /people/me: the values already stored", "", base, none,
		route(people, "update_my_profile", w.hang, "body", args("display_name", "Hằng (dữ liệu mẫu)",
			"wall_comment_policy", "nobody", "discoverable_by_phone", false)))
	add("route PATCH /people/me: no people row", "404:person_not_found", base, none,
		route(people, "update_my_profile", missing, "body", args("city", "Nơi nào đó (dữ liệu mẫu)")))
	add("route GET /people/me/saved-places: a bookmark whose place is gone", "", base, none,
		route(readProbes, "list_saved_places", w.an), route(readProbes, "list_saved_places", w.binh),
		route(readProbes, "list_saved_places", w.stranger))
	add("route PUT /people/me/saved-places: new", "", base, none, route(bookmarks, "save_place", w.binh, "place_id", "p-c"))
	add("route PUT /people/me/saved-places: already saved", "", base, none, route(bookmarks, "save_place", w.an, "place_id", "p-b"))
	add("route PUT /people/me/saved-places: not in the catalogue", "404:place_not_found", base, none,
		route(bookmarks, "save_place", w.an, "place_id", "p-da-dong-cua"))
	add("route DELETE /people/me/saved-places: a bookmark, then again", "", base, none,
		route(bookmarks, "unsave_place", w.an, "place_id", "p-b"), route(bookmarks, "unsave_place", w.an, "place_id", "p-b"))
	add("route DELETE /people/me/saved-places: not in the catalogue", "404:place_not_found", base, none,
		route(bookmarks, "unsave_place", w.an, "place_id", "p-da-dong-cua"))
	add("route GET /people/me/blocked", "", base, none, route(readProbes, "list_blocked_people", w.an),
		route(readProbes, "list_blocked_people", w.chi), route(readProbes, "list_blocked_people", w.stranger))

	addDelete("route DELETE /people/me: the only admin, two files on disk and one missing", "", base, files, one(w.an, true))
	addDelete("route DELETE /people/me: a member who owes money", "", base, files, one(w.binh, true))
	addDelete("route DELETE /people/me: a directory where a photograph should be", "", base,
		mediaSeed{files: []string{w.keyAvatar, w.keyGroup}, dirs: []string{w.keyPersonal}}, one(w.an, true))
	addDelete("route DELETE /people/me: a key directory nobody may write", "", base,
		mediaSeed{files: []string{w.keyAvatar, w.keyPersonal}, readonly: []string{w.keyPersonal}}, one(w.an, true))
	addDelete("route DELETE /people/me: a stored key that is not a storage key", "ValueError",
		join(base, []string{"UPDATE uploaded_images SET storage_key = 'anh-mau-khong-phai-khoa' WHERE id = '" + w.imgUnreadable + "'"}),
		files, one(w.an, true))
	addDelete("route DELETE /people/me: confirm is false", "422:confirm_required", base, files, one(w.an, false))
	addDelete("route DELETE /people/me: a caller with no people row", "404:person_not_found", base, files, one(missing, true))
	addDelete("route DELETE /people/me: twice", "", base, files, func(del func(string, string, bool) oracleCall) []oracleCall {
		return []oracleCall{del(w.an, now, true), del(w.an, later(1), true)}
	})
	addDelete("route DELETE /people/me: then the friend with a pair looks, and writes", "404:person_not_found", base, files,
		func(del func(string, string, bool) oracleCall) []oracleCall {
			return []oracleCall{del(w.an, now, true), contextsOf(w.ban), route(readProbes, "get_my_profile", w.ban),
				onPerson(readProbes, "open_direct_message", w.ban, w.an)}
		})
	addDelete("route DELETE /people/me: then the friend opens the profile", "403:person_not_visible", base, files,
		func(del func(string, string, bool) oracleCall) []oracleCall {
			return []oracleCall{del(w.an, now, true), onPerson(readProbes, "get_person_profile", w.ban, w.an)}
		})
	addDelete("route DELETE /people/me: the other half of the pair, under a Vietnam session TimeZone", "", vietnam, none,
		func(del func(string, string, bool) oracleCall) []oracleCall {
			return []oracleCall{del(w.ban, vietnamNow, true)}
		})

	add("route POST /people/{id}/block: a friend", "", base, none, onPerson(edges, "block_person", w.an, w.binh))
	add("route POST /people/{id}/block: somebody blocked, somebody who blocked the caller", "", base, none,
		onPerson(edges, "block_person", w.an, w.hang), onPerson(edges, "block_person", w.an, w.stranger))
	add("route POST /people/{id}/block: a declined edge", "", base, none, onPerson(edges, "block_person", w.an, w.dung))
	add("route POST /people/{id}/block: a declined edge the other way", "", base, none,
		onPerson(edges, "block_person", w.chi, w.ban))
	add("route POST /people/{id}/block: a person with nothing", "", base, none, onPerson(edges, "block_person", w.an, w.moi))
	add("route POST /people/{id}/block: an ended account", "", base, none, onPerson(edges, "block_person", w.an, w.erased))
	add("route POST /people/{id}/block: a pending request, under a Vietnam session TimeZone", "", vietnam, none,
		onPerson(edges, "block_person", w.ban, w.dung))
	add("route POST /people/{id}/block: nobody", "404:person_not_found", base, none, onPerson(edges, "block_person", w.an, missing))
	add("route POST /people/{id}/block: oneself", "403:permission_denied", base, none, onPerson(edges, "block_person", w.an, w.an))
	add("route DELETE /people/{id}/block: a block the caller put up", "", base, none,
		onPerson(edges, "unblock_person", w.an, w.hang))
	add("route DELETE /people/{id}/block: a block somebody else put up", "403:permission_denied", base, none,
		onPerson(edges, "unblock_person", w.an, w.stranger))
	add("route DELETE /people/{id}/block: a friendship", "403:permission_denied", base, none,
		onPerson(edges, "unblock_person", w.an, w.binh))
	add("route DELETE /people/{id}/block: nobody", "403:permission_denied", base, none,
		onPerson(edges, "unblock_person", w.an, missing))

	add("route POST /people/{id}/dm: a pair that exists", "", base, none, onPerson(conversation, "open_direct_message", w.an, w.ban))
	add("route POST /people/{id}/dm: a friend with no pair yet", "", base, none,
		onPerson(conversation, "open_direct_message", w.an, w.binh))
	add("route POST /people/{id}/dm: the same friend the other way, under a Vietnam session TimeZone", "", vietnam, none,
		onPerson(conversation, "open_direct_message", w.binh, w.an))
	add("route POST /people/{id}/dm: a friend whose account ended", "404:person_not_found", base, none,
		onPerson(conversation, "open_direct_message", w.binh, w.erased))
	add("route POST /people/{id}/dm: somebody who only asked", "404:person_not_found", base, none,
		onPerson(conversation, "open_direct_message", w.an, w.chi))
	add("route POST /people/{id}/dm: oneself", "422:self_direct_message", base, none,
		onPerson(conversation, "open_direct_message", w.an, w.an))
	add("route POST /people/{id}/dm: nobody", "404:person_not_found", base, none,
		onPerson(conversation, "open_direct_message", w.an, missing))
	add("route POST /people/{id}/dm: a pair the caller has left", "404:context_not_found",
		join(base, []string{"UPDATE memberships SET state = 'left', left_at = '2030-09-20T00:00:00Z' WHERE id = '" + w.mAnInAnBan + "'"}),
		none, onPerson(conversation, "open_direct_message", w.an, w.ban))

	profileOf := func(actor, person string) oracleCall {
		return onPerson(readProbes, "get_person_profile", actor, person)
	}
	add("route GET /people/{id}: oneself, a friend, groupmates behind a block, a pair", "", base, none,
		profileOf(w.an, w.an), profileOf(w.an, w.binh), profileOf(w.binh, w.chi), profileOf(w.an, w.dung), profileOf(w.hang, w.ban))
	add("route GET /people/{id}: somebody the caller blocked", "403:person_not_visible", base, none, profileOf(w.an, w.hang))
	add("route GET /people/{id}: nobody", "403:person_not_visible", base, none, profileOf(w.an, missing))
	add("route GET /people/{id}: a friend whose account ended", "404:person_not_found", base, none, profileOf(w.binh, w.erased))

	register := func(actor, person, name string) oracleCall {
		return route(people, "register_person", actor, "person_id", person, "display_name", name)
	}
	add("route PUT /people/{id}: a new id", "", base, none, register(w.an, w.newcomer, "Người mới 🌱 (dữ liệu mẫu)"))
	add("route PUT /people/{id}: the name it already has", "", base, none, register(w.an, w.ban, "Bạn thân (dữ liệu mẫu)"))
	add("route PUT /people/{id}: a rename by oneself", "", base, none, register(w.ban, w.ban, "Tên do mình đặt (dữ liệu mẫu)"))
	add("route PUT /people/{id}: a rename by somebody else", "403:permission_denied", base, none,
		register(w.an, w.ban, "Tên người khác đặt (dữ liệu mẫu)"))
	add("route PUT /people/{id}: an ended account", "404:person_not_found", base, none,
		register(w.an, w.erased, "Hồi sinh (dữ liệu mẫu)"))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// peopleMethods is every repository method and route this port covers; the
// corpus must reach each with at least one normal return in Python.
var peopleMethods = []string{
	"create_person", "rename_person", "are_friends", "share_active_context", "profile_counts", "list_login_providers",
	"get_pair_context", "create_pair_context", "list_person_context_summaries", "count_unread_messages",
	"list_saved_places", "save_place", "unsave_place", "open_block_edge", "lift_block_edge", "list_blocked",
	"revoke_all_account_sessions", "erase_person",
	"route.list_my_contexts", "route.get_my_profile", "route.update_my_profile", "route.list_saved_places",
	"route.save_place", "route.unsave_place", "route.list_blocked_people", "route.delete_own_account",
	"route.block_person", "route.unblock_person", "route.open_direct_message", "route.get_person_profile",
	"route.register_person",
}

// peopleBranches are the write branches, storage outcomes and log lines the
// corpus must reach in Python, each named by the call, the start of the
// normalised statement, and text it must contain.
var peopleBranches = []struct{ call, prefix, contains string }{
	{"create_person", "INSERT INTO people", ""},
	{"rename_person", "UPDATE people SET display_name=", ""},
	{"create_pair_context", "INSERT INTO contexts", ""},
	{"create_pair_context", "INSERT INTO memberships", ""},
	{"save_place", "INSERT INTO saved_places", ""},
	{"save_place", "ROLLBACK TO SAVEPOINT", ""},
	{"unsave_place", "DELETE FROM saved_places", ""},
	{"open_block_edge", "UPDATE friend_requests SET state=", ""},
	{"open_block_edge", "UPDATE friend_requests SET decided_by_id=", ""},
	{"open_block_edge", "UPDATE friend_requests SET decided_at=", ""},
	{"open_block_edge", "INSERT INTO friend_requests", ""},
	{"lift_block_edge", "UPDATE friend_requests SET state=", "decided_at"},
	{"lift_block_edge", "UPDATE friend_requests SET state=? WHERE", ""},
	{"revoke_all_account_sessions", "UPDATE account_sessions SET revoked_at=", ""},
	{"erase_person", "DELETE FROM post_comments", ""},
	{"erase_person", "UPDATE memberships SET", ""},
	{"erase_person", "UPDATE people SET display_name=", ""},
	{"erase_person", "UPDATE people SET deleted_at=", ""},
	{"erase_person", "INSERT INTO audit_events", ""},
	{"route.delete_own_account", "-- storage.delete", "-> True"},
	{"route.delete_own_account", "-- storage.delete", "-> False"},
	{"route.delete_own_account", "-- storage.delete", "raised IsADirectoryError"},
	{"route.delete_own_account", "-- storage.delete", "raised PermissionError"},
	{"route.delete_own_account", "-- storage.delete", "raised ValueError"},
	{"route.delete_own_account", "-- log WARNING", ""},
	{"route.delete_own_account", "-- log INFO", ""},
	{"route.open_direct_message", "INSERT INTO memberships", ""},
	{"route.block_person", "INSERT INTO friend_requests", ""},
	{"route.block_person", "UPDATE friend_requests SET", ""},
	{"route.unblock_person", "UPDATE friend_requests SET state=", ""},
	{"route.register_person", "INSERT INTO people", ""},
	{"route.register_person", "UPDATE people SET display_name=", ""},
	{"route.update_my_profile", "UPDATE people SET", ""},
	{"route.save_place", "INSERT INTO saved_places", ""},
	{"route.unsave_place", "DELETE FROM saved_places", ""},
}

// mediaRootFor makes a directory both the container's user and this process
// can write into, and makes it removable again whatever a case did to it.
func mediaRootFor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0o777)
			}
			return nil
		})
	})
	return dir
}

func seedMedia(t *testing.T, root string, seed mediaSeed) {
	t.Helper()
	mkdirs := func(path string) {
		if err := os.MkdirAll(path, 0o777); err != nil {
			t.Fatal(err)
		}
		for p := path; ; p = filepath.Dir(p) {
			if err := os.Chmod(p, 0o777); err != nil {
				t.Fatal(err)
			}
			if p == root || len(p) <= len(root) {
				break
			}
		}
	}
	keyDir := func(key string) string { return filepath.Join(root, key[:2], key[2:4]) }
	mkdirs(root)
	for _, key := range seed.files {
		mkdirs(keyDir(key))
		path := filepath.Join(keyDir(key), key)
		if err := os.WriteFile(path, []byte("anh mau "+key), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range seed.dirs {
		mkdirs(filepath.Join(keyDir(key), key))
	}
	for _, key := range seed.readonly {
		if err := os.Chmod(keyDir(key), 0o555); err != nil {
			t.Fatal(err)
		}
	}
}

func schemaTables(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(bg, `SELECT table_name::text FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' AND table_name <> 'alembic_version'
		ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 60 {
		t.Fatalf("the migrated schema has %d tables; expected the whole schema", len(tables))
	}
	return tables
}

func TestPeopleRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of people_repo_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_people_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "people_repo_oracle_")
	tables := schemaTables(t, pool)
	pyMedia, goMedia := mediaRootFor(t), mediaRootFor(t)

	cases, built := peopleRepoOracleCases(tables)
	for i, c := range cases {
		seedMedia(t, filepath.Join(pyMedia, strconv.Itoa(i)), c.media)
		seedMedia(t, filepath.Join(goMedia, strconv.Itoa(i)), c.media)
	}
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-e", "PEOPLE_ORACLE_MEDIA="+pyMedia, "-v", pyMedia+":"+pyMedia,
		"-v", scripts+":/oracle:ro", "--entrypoint", "python", image, "/oracle/render_people_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_people_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	if out := os.Getenv("CORE_ORACLE_PYTHON_OUT"); out != "" {
		if err := os.WriteFile(out, stdout.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	dumpRows := 0
	returned := map[string]int{}
	reached := make([]int, len(peopleBranches))
	routeSteps := 0
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
			call := c.Steps[j].Call
			if strings.HasPrefix(call, "route.") {
				routeSteps++
			}
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statement := normalizeSQL(entry[0].(string))
					statements = append(statements, statement)
					for b, branch := range peopleBranches {
						if call == branch.call && strings.HasPrefix(statement, branch.prefix) &&
							strings.Contains(statement, branch.contains) {
							reached[b]++
						}
					}
				}
			}
			// Go intentionally acquires the identity lock before erasure writes.
			// Keep every legacy statement and probe; add only this documented
			// security delta to the expected trace, at ErasePerson's entry.
			if call == "erase_person" || call == "route.delete_own_account" {
				for index, statement := range statements {
					text, _ := statement.(string)
					if strings.HasPrefix(text, "SELECT uploaded_images.id,") && strings.Contains(text, "WHERE uploaded_images.owner_person_id =") {
						withLock := append([]any{}, statements[:index]...)
						withLock = append(withLock, normalizeSQL("SELECT id FROM people WHERE id=$1::UUID FOR UPDATE"))
						statements = append(withLock, statements[index:]...)
						break
					}
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			if s.Error == nil {
				returned[call]++
			}
			probeRows, _ := s.Probes.([]any)
			for k, rows := range probeRows {
				if list, ok := rows.([]any); ok && strings.HasPrefix(c.Steps[j].Probes[k], peopleDumpPrefix) {
					dumpRows += len(list)
				}
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}

		pyCase := python.Cases[i]
		end := ""
		if len(pyCase.Steps) > 0 {
			if e, ok := pyCase.Steps[len(pyCase.Steps)-1].Error.(map[string]any); ok {
				end, _ = e["type"].(string)
				if code, ok := e["code"].(string); ok {
					end = code
				}
			}
		}
		if len(pyCase.Steps) != len(c.Steps) && end == "" {
			t.Errorf("case %q: python ran %d of %d steps", c.Name, len(pyCase.Steps), len(c.Steps))
		}
		if end != cases[i].wantEnd {
			var last any
			if len(pyCase.Steps) > 0 {
				last = pyCase.Steps[len(pyCase.Steps)-1].Error
			}
			t.Errorf("case %q: python ended in %q, the case is written for %q (%v)", c.Name, end, cases[i].wantEnd, last)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python refused at step %d of %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}

		t.Run(c.Name, func(t *testing.T) {
			golang := normalizePeopleSteps(runPeopleGoCase(t, pool, c, goMedia), c)
			py := normalizePeopleSteps(pySteps, c)
			before := tally.mismatches
			compareCase(t, tally, c, py, golang)
			if tally.mismatches > before {
				logPeopleDifferences(t, c, py, golang)
			}
		})
	}
	for _, method := range peopleMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	for b, branch := range peopleBranches {
		if reached[b] == 0 {
			t.Errorf("no case makes python issue, from %s: %s...%s", branch.call, branch.prefix, branch.contains)
		}
	}
	t.Logf("people repo oracle: %d cases, %d steps (%d route steps; %d results, %d refusals), %d statements, "+
		"%d probe rows of which %d table rows, %d generated ids bound, %d branches reached, %d tables dumped, %d mismatches",
		tally.cases, tally.steps, routeSteps, tally.results, tally.errors, tally.statements, tally.probeRows,
		dumpRows, tally.generated, len(peopleBranches), len(tables), tally.mismatches)
}

// ---------------------------------------------------------------------------
// Two requests at once (Go only)
// ---------------------------------------------------------------------------

func TestPeopleRowLocksSerialiseRequests(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of people_repo_oracle_postgres_test.go")
	}
	pool, _ := migratedOracleSchema(t, image, "people_locks_")
	w := newPeopleWorld()
	for _, sql := range w.sql {
		if _, err := pool.Exec(bg, sql); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	now := argInstant(peopleNow)
	begin := func(t *testing.T, lockTimeout string) pgx.Tx {
		t.Helper()
		tx, err := pool.Begin(bg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		if lockTimeout != "" {
			if _, err := tx.Exec(bg, "SET LOCAL lock_timeout = '"+lockTimeout+"'"); err != nil {
				t.Fatal(err)
			}
		}
		return tx
	}
	// waiting runs second in the background and fails the test when it
	// finishes before first commits.
	waiting := func(t *testing.T, first pgx.Tx, second func() error) error {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- second() }()
		select {
		case err := <-done:
			t.Fatalf("the second request did not wait for the first: %v", err)
		case <-time.After(300 * time.Millisecond):
		}
		if err := first.Commit(bg); err != nil {
			t.Fatal(err)
		}
		return <-done
	}

	t.Run("two first bookmarks of one place end as the first", func(t *testing.T) {
		first := begin(t, "")
		if _, created, err := (Repository{Q: first}).SavePlace(bg, w.binh, "p-c", now); err != nil || !created {
			t.Fatalf("first: %v %v", created, err)
		}
		second := begin(t, "")
		var row SavedPlace
		var created bool
		err := waiting(t, first, func() error {
			var err error
			row, created, err = (Repository{Q: second}).SavePlace(bg, w.binh, "p-c", now.Add(time.Second))
			return err
		})
		if err != nil || created || !row.CreatedAt.Equal(pythonInstant(now)) {
			t.Fatalf("second: %+v created=%v err=%v", row, created, err)
		}
	})

	t.Run("two pairs for one key: the second is PAIR_EXISTS and then reads the first", func(t *testing.T) {
		in := PairContextInput{PairKey: w.pairKeyBinhDung, MemberIDs: []string{w.binh, w.dung}, CreatedByID: w.binh, Now: now}
		first := begin(t, "")
		made, err := (Repository{Q: first}).CreatePairContext(bg, in)
		if err != nil {
			t.Fatal(err)
		}
		second := begin(t, "")
		err = waiting(t, first, func() error {
			_, err := (Repository{Q: second}).CreatePairContext(bg, in)
			return err
		})
		var conflict *Conflict
		if !errors.As(err, &conflict) || conflict.Code != "PAIR_EXISTS" {
			t.Fatalf("second: %v", err)
		}
		found, err := (Repository{Q: second}).GetPairContext(bg, w.pairKeyBinhDung)
		if err != nil || found == nil || found.ID != made.ID {
			t.Fatalf("the loser read %+v, %v", found, err)
		}
	})

	t.Run("a block locks the live edge against another block of the pair", func(t *testing.T) {
		first := begin(t, "")
		if _, err := (Repository{Q: first}).OpenBlockEdge(bg, w.an, w.binh, now); err != nil {
			t.Fatal(err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).OpenBlockEdge(bg, w.binh, w.an, now); !lockNotAvailable(err) {
			t.Fatalf("a second block of the pair did not wait: %v", err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).LiftBlockEdge(bg, w.binh, w.an, now); !lockNotAvailable(err) {
			t.Fatalf("lifting did not wait: %v", err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).GetFriendEdge(bg, w.an, w.binh); err != nil {
			t.Fatalf("a plain read must not wait: %v", err)
		}
	})

	t.Run("two blocks of a pair with no edge: the second is EDGE_EXISTS", func(t *testing.T) {
		first := begin(t, "")
		if _, err := (Repository{Q: first}).OpenBlockEdge(bg, w.hang, w.dung, now); err != nil {
			t.Fatal(err)
		}
		second := begin(t, "")
		err := waiting(t, first, func() error {
			_, err := (Repository{Q: second}).OpenBlockEdge(bg, w.dung, w.hang, now)
			return err
		})
		var conflict *Conflict
		if !errors.As(err, &conflict) || conflict.Code != "EDGE_EXISTS" {
			t.Fatalf("second: %v", err)
		}
	})

	t.Run("erasing holds the person, the memberships and the sessions", func(t *testing.T) {
		first := begin(t, "")
		if _, err := (Repository{Q: first}).ErasePerson(bg, w.ban, now); err != nil {
			t.Fatal(err)
		}
		city := "Nơi khác (dữ liệu mẫu)"
		if _, err := (Repository{Q: begin(t, "150ms")}).UpdatePersonProfile(bg, w.ban,
			ProfileChanges{City: SetText(&city)}); !lockNotAvailable(err) {
			t.Fatalf("a profile edit did not wait for the erasure: %v", err)
		}
		if _, err := begin(t, "150ms").Exec(bg, `UPDATE memberships SET role = 'admin' WHERE person_id = $1::UUID`,
			w.ban); !lockNotAvailable(err) {
			t.Fatalf("a membership change did not wait for the erasure: %v", err)
		}
		if p, err := (Repository{Q: begin(t, "150ms")}).GetPerson(bg, w.ban); err != nil || p == nil || p.DeletedAt != nil {
			t.Fatalf("a plain read must not wait and must see the row before the commit: %+v %v", p, err)
		}
	})
}
