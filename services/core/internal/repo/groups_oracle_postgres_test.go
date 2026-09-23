//go:build postgres

package repo

// Differential test of the W3 repository methods (groups, rosters, balances,
// the memory wall) against the real SqlAlchemyApiRepository, driven through
// scripts/render_groups_repo_oracle.py. Same design as
// social_oracle_postgres_test.go, whose case format, value tags, statement
// normalisation, conflict tagging and comparison it reuses, with three
// additions:
//
//   - Row locks. probeLocks sees relation locks only, and a FOR UPDATE shows
//     there as a RowShareLock whichever rows it took. probeRowLocks asks
//     pgrowlocks, row by row, for the locks this transaction holds on the
//     tables these methods lock or reference (the FOR UPDATE of balances,
//     accept and leave, and the FOR KEY SHARE a foreign key check takes), each
//     named by the row's id, so a lock taken on the wrong rows, or not taken,
//     is a mismatch.
//   - The transaction's now(). create_context and add_member leave created_at
//     to the server default, read back with RETURNING, and each side's
//     transaction starts at its own instant. Every step also probes now(), and
//     each side replaces exactly that instant with <transaction-now> in its
//     own step before comparing; the contexts and memberships dumps do the
//     same inside PostgreSQL. A created_at that is not now() is compared as is.
//   - Sequences. flow.create_context and flow.add_member_accept run a write
//     and the accept that needs its generated id in one step, in the order
//     the service calls them.
//
// One schema serves both sides: every read here is ordered or consumed by key,
// so the plan of a rolled-back heap cannot reach a result.
//
// SQLAlchemy's session answers session.get of an object it already loaded
// from its identity map, without a statement. A case therefore loads a given
// context, person or place by id at most once, which is why the PATCH
// /contexts/{id} case below renames nothing (see UpdateContext).
//
// Without CORE_PYTHON_IMAGE the test skips; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

// probeNow is the transaction's now() in the tag format of a Python datetime.
const probeNow = `SELECT to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"+00:00"')`

const transactionNow = "<transaction-now>"

// probeRowLocks lists the row locks held on the tables these methods lock or
// reference. A row the transaction has since updated is no longer visible at
// its old ctid and is reported as superseded.
var probeRowLocks = func() string {
	var parts []string
	for _, table := range []string{"people", "contexts", "memberships", "memories", "expense_versions", "confirmed_allocations"} {
		parts = append(parts, `SELECT '`+table+`' AS rel, t.id::text AS id, array_to_string(r.modes, ',') AS modes
			FROM public.pgrowlocks('`+table+`') r LEFT JOIN `+table+` t ON t.ctid = r.locked_row`)
	}
	return `SELECT l.rel || ' ' || coalesce(l.id, '(superseded row)') || ' ' || l.modes FROM (` +
		strings.Join(parts, " UNION ALL ") + `) l ORDER BY 1`
}()

// nowDump is dumpProbe for a table whose created_at is a server default: the
// transaction's own now() is written as <transaction-now>.
func nowDump(table, order string) string {
	return `SELECT (to_jsonb(t) || jsonb_build_object('created_at', CASE WHEN t.created_at = now() ` +
		`THEN to_jsonb('` + transactionNow + `'::text) ELSE to_jsonb(t.created_at) END))::text FROM ` +
		table + ` t ORDER BY ` + order
}

// ---------------------------------------------------------------------------
// Tags and the Go side
// ---------------------------------------------------------------------------

func tMemoryComment(c MemoryComment) any {
	return tRecord("MemoryCommentRecord", "id", tUUID(c.ID), "memory_id", tUUID(c.MemoryID),
		"author_id", tUUID(c.AuthorID), "body", tStr(c.Body), "created_at", tInstant(c.CreatedAt))
}

func tBatchInputs(b BatchInputs) any {
	expenses := []any{}
	for _, e := range b.Expenses {
		allocations := []any{}
		for _, a := range e.Allocations {
			allocations = append(allocations, tRecord("AllocationRow", "id", tUUID(a.ID),
				"participant_id", tUUID(a.ParticipantID), "amount_vnd", tInt(a.AmountVND)))
		}
		expenses = append(expenses, tRecord("ConfirmedExpense", "version_id", tUUID(e.VersionID),
			"context_id", tUUID(e.ContextID), "paid_by_id", tUUID(e.PaidByID),
			"payer_acknowledgement", tStr(e.PayerAcknowledgement), "allocations", tSeq(allocations)))
	}
	ids := []any{}
	for _, id := range b.UnavailableVersionIDs {
		ids = append(ids, tUUID(id))
	}
	return tRecord("BatchInputs", "expenses", tSeq(expenses), "unavailable_version_ids", tSeq(ids))
}

func groupsGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	at := func(key string) time.Time { return argInstant(s(key)) }
	switch method {
	case "create_context":
		c, err := repo.CreateContext(bg, s("display_name"), s("created_by_id"))
		return tContext(&c), err
	case "update_context":
		var changes ContextChanges
		for _, item := range a["changes"].([]any) {
			pair := item.([]any)
			value := pair[1].(string)
			switch pair[0].(string) {
			case "display_name":
				changes.DisplayName = &value
			case "theme":
				changes.Theme = &value
			default:
				panic(pair[0])
			}
		}
		c, err := repo.UpdateContext(bg, s("context_id"), changes)
		return tContext(c), err
	case "add_member":
		role := "member"
		if given, ok := a["role"].(string); ok {
			role = given
		}
		m, err := repo.AddMember(bg, s("context_id"), s("person_id"), s("invited_by_id"), role)
		return tMembership(m), err
	case "get_membership":
		m, err := repo.GetMembership(bg, s("membership_id"))
		return nilOr(m, tMembership), err
	case "accept_membership":
		m, err := repo.AcceptMembership(bg, s("membership_id"), at("now"))
		return nilOr(m, tMembership), err
	case "leave_context":
		m, err := repo.LeaveContext(bg, s("context_id"), s("person_id"), at("now"))
		return nilOr(m, tMembership), err
	case "membership_role":
		role, err := repo.MembershipRole(bg, s("context_id"), s("person_id"))
		return optional(role, tStr), err
	case "flow.create_context":
		actor := s("actor_id")
		c, err := repo.CreateContext(bg, s("display_name"), actor)
		if err != nil {
			return nil, err
		}
		m, err := repo.AddMember(bg, c.ID, actor, actor, "admin")
		if err != nil {
			return nil, err
		}
		accepted, err := repo.AcceptMembership(bg, m.ID, at("now"))
		return tSeq([]any{tContext(&c), tMembership(m), nilOr(accepted, tMembership)}), err
	case "flow.add_member_accept":
		m, err := repo.AddMember(bg, s("context_id"), s("person_id"), s("invited_by_id"), "member")
		if err != nil {
			return nil, err
		}
		accepted, err := repo.AcceptMembership(bg, m.ID, at("now"))
		return tSeq([]any{tMembership(m), nilOr(accepted, tMembership)}), err
	case "load_batch_inputs":
		var ids []string
		if given, ok := a["expense_version_ids"].([]any); ok {
			ids = argStrings(given)
		}
		b, err := repo.LoadBatchInputs(bg, s("context_id"), ids)
		return tBatchInputs(b), err
	case "load_confirmed_receipts":
		totals, err := repo.LoadConfirmedReceipts(bg, s("context_id"))
		pairs := []any{}
		for _, total := range totals {
			// The exact decimal, as render_repo_oracle.py tags a Python int.
			pairs = append(pairs, []any{tSeq([]any{tUUID(total.SenderID), tUUID(total.RecipientID)}),
				tv("int", total.AmountVND.String())})
		}
		return tv("dict", pairs), err
	case "get_place":
		p, err := repo.GetPlace(bg, s("place_id"))
		return nilOr(p, tPlace), err
	case "create_memory":
		m, err := repo.CreateMemory(bg, MemoryInput{ContextID: s("context_id"), AuthorID: s("author_id"),
			ImageURL: s("image_url"), Caption: argOptional(a, "caption"), Now: at("now"),
			PlaceID: argOptional(a, "place_id"), PlaceName: argOptional(a, "place_name")})
		return tMemory(m), err
	case "create_checkin":
		m, err := repo.CreateCheckin(bg, CheckinInput{ContextID: s("context_id"), AuthorID: s("author_id"),
			PlaceID: s("place_id"), PlaceName: s("place_name"), Lat: optionalFloat(a, "lat"), Lng: optionalFloat(a, "lng"),
			Caption: argOptional(a, "caption"), Now: at("now")})
		return tMemory(m), err
	case "get_context_memory":
		m, err := repo.GetContextMemory(bg, s("context_id"), s("memory_id"), argOptional(a, "viewer_id"))
		return nilOr(m, tMemory), err
	case "add_memory_reaction":
		r, err := repo.AddMemoryReaction(bg, s("memory_id"), s("person_id"), at("now"))
		return tRecord("MemoryReactionRecord", "id", tUUID(r.ID), "memory_id", tUUID(r.MemoryID),
			"person_id", tUUID(r.PersonID), "created_at", tInstant(r.CreatedAt)), err
	case "remove_memory_reaction":
		removed, err := repo.RemoveMemoryReaction(bg, s("memory_id"), s("person_id"))
		return tBool(removed), err
	case "create_memory_comment":
		c, err := repo.CreateMemoryComment(bg, s("memory_id"), s("author_id"), s("body"), at("now"))
		return tMemoryComment(c), err
	case "list_memory_comments":
		comments, err := repo.ListMemoryComments(bg, s("memory_id"))
		items := []any{}
		for _, c := range comments {
			items = append(items, tMemoryComment(c))
		}
		return tSeq(items), err
	}
	return pairGoCall(repo, method, a)
}

// groupsGoError is socialGoError plus the ValueError add_member raises itself.
func groupsGoError(err error) map[string]any {
	out := socialGoError(err)
	if errors.Is(err, ErrUnknownMembershipRole) {
		out["type"] = "ValueError"
	}
	return out
}

func runGroupsGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
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
		value, err := groupsGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = groupsGoError(err)
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

// replaceText swaps every string exactly equal to from, anywhere in v.
func replaceText(v any, from, to string) any {
	switch x := v.(type) {
	case string:
		if x == from {
			return to
		}
		return x
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = replaceText(x[i], from, to)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, item := range x {
			out[k] = replaceText(item, from, to)
		}
		return out
	}
	return v
}

// normalizeGroupsSteps prepares one side's steps for compareCase:
//
//   - the instant that side's probeNow read becomes <transaction-now>;
//   - probeRowLocks rows are re-sorted with every id the case did not name
//     sorting last. PostgreSQL orders them by the raw text, where a generated
//     uuid4 falls before or after the fixture ids at random, so the same set of
//     locks would otherwise come back in a different order on each side. No
//     case generates two rows of one table that a lock probe reports with the
//     same mode, so the order among generated ids is never left to chance.
func normalizeGroupsSteps(steps []any, c oracleCase) []any {
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
	}
	sortKey := func(row string) string {
		return uuidText.ReplaceAllStringFunc(row, func(id string) string {
			if known[id] {
				return id
			}
			return "~"
		})
	}
	out := make([]any, len(steps))
	for i, step := range steps {
		out[i] = step
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
			case probeRowLocks:
				sort.SliceStable(rows, func(a, b int) bool {
					return sortKey(rows[a].(string)) < sortKey(rows[b].(string))
				})
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

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func groupsOracleCases() ([]socialCase, oracleSpec) {
	w := newGroupsWorld()
	var cases []socialCase
	add := func(name, wantEnd string, setup []string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps}, wantEnd})
	}
	read := func(method string, args map[string]any) oracleCall {
		return oracleCall{Call: method, Args: args, Before: append([]string{}, writesBaseline...),
			Probes: []string{probeLocks, probeWrites, probeRowLocks, probeNow}}
	}
	write := func(method string, args map[string]any, dumps ...string) oracleCall {
		probes := append([]string{probeLocks, probeWrites, probeRowLocks}, dumps...)
		return oracleCall{Call: method, Args: args, Before: append([]string{}, writesBaseline...),
			Probes: append(probes, probeNow)}
	}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	list := func(ids ...string) []any {
		out := []any{}
		for _, id := range ids {
			out = append(out, id)
		}
		return out
	}
	base := w.sql
	vietnam := append([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base...)
	contexts := nowDump("contexts", "t.created_at, t.display_name, t.id")
	memberships := nowDump("memberships", "t.created_at, t.context_id, t.person_id, t.id")
	memories := dumpProbe("memories", "t.created_at, t.id")
	reactions := dumpProbe("memory_reactions", "t.memory_id, t.person_id")
	comments := dumpProbe("memory_comments", "t.created_at, t.id")
	ctxArg := func(id string) map[string]any { return args("context_id", id) }

	// --- contexts -------------------------------------------------------------
	createContext := func(name, creator string) oracleCall {
		return write("create_context", args("display_name", name, "created_by_id", creator), contexts)
	}
	add("create_context: a group by a registered person", "", base, createContext("Nhóm mới 🌿 ' \" (dữ liệu mẫu)", w.la))
	add("create_context: an empty name", "", base, createContext("", w.chu))
	add("create_context: a creator with no people row", "IntegrityError", base, createContext("Không ai (dữ liệu mẫu)", w.missingPerson))
	flow := func(actor, name string) oracleCall {
		return write("flow.create_context", args("actor_id", actor, "display_name", name, "now", groupsNow), contexts, memberships)
	}
	add("flow.create_context: create, bootstrap the admin, accept", "", base, flow(w.la, "Của người lạ (dữ liệu mẫu)"))
	add("flow.create_context: by an erased account, under a Vietnam session TimeZone", "", vietnam, flow(w.xoa, "Đã xoá vẫn tạo (dữ liệu mẫu)"))
	add("flow.create_context: an actor with no people row", "IntegrityError", base, flow(w.missingPerson, "Không ai (dữ liệu mẫu)"))

	update := func(id string, changes ...[2]string) oracleCall {
		pairs := []any{}
		for _, change := range changes {
			pairs = append(pairs, []any{change[0], change[1]})
		}
		return write("update_context", args("context_id", id, "changes", pairs), contexts)
	}
	add("update_context: rename and re-theme", "", base,
		update(w.g1, [2]string{"theme", "rung-thong"}, [2]string{"display_name", "Tên mới 🌅 (dữ liệu mẫu)"}))
	add("update_context: the theme alone", "", base, update(w.g2, [2]string{"theme", "ruc-ro"}))
	add("update_context: the name alone, under a Vietnam session TimeZone", "", vietnam,
		update(w.g3, [2]string{"display_name", "Đổi tên (dữ liệu mẫu)"}))
	add("update_context: values equal to the stored ones write nothing", "", base,
		update(w.g2, [2]string{"display_name", "Nhóm hai ' \" (dữ liệu mẫu)"}, [2]string{"theme", "bien-dem"}))
	add("update_context: one equal value, one new", "", base,
		update(w.g2, [2]string{"display_name", "Nhóm hai ' \" (dữ liệu mẫu)"}, [2]string{"theme", "mac-dinh"}))
	add("update_context: no changes at all", "", base, update(w.g1))
	add("update_context: a missing context", "", base, update(w.missingContext, [2]string{"theme", "ruc-ro"}))
	add("update_context: the repository renames a pair", "", base, update(w.pair, [2]string{"display_name", "Cặp (dữ liệu mẫu)"}))
	add("update_context: a theme the check refuses", "IntegrityError", base, update(w.g1, [2]string{"theme", "tim-tim"}))
	add("update_context: a theme longer than varchar(16)", "DataError", base, update(w.g1, [2]string{"theme", "hoang-hon-hoang-hon"}))

	// --- memberships ------------------------------------------------------------
	invite := func(context, person, by string, role ...string) oracleCall {
		a := args("context_id", context, "person_id", person, "invited_by_id", by)
		if len(role) == 1 {
			a["role"] = role[0]
		}
		return write("add_member", a, memberships)
	}
	add("add_member: a stranger, as a member", "", base, invite(w.g1, w.la, w.ban))
	add("add_member: an admin role", "", base, invite(w.g3, w.la, w.la, "admin"))
	add("add_member: a person who left may be invited again", "", base, invite(w.g1, w.roi, w.chu))
	add("add_member: an erased account, into another group", "", base, invite(w.g2, w.xoa, w.hai))
	add("add_member: a person with no name", "", base, invite(w.g2, w.trong, w.chu))
	add("add_member: into a pair", "", base, invite(w.pair, w.la, w.chu))
	add("add_member: the rejoined person's open row", "MEMBERSHIP_ALREADY_OPEN", base, invite(w.g1, w.lai, w.chu))
	add("add_member: an invited person", "MEMBERSHIP_ALREADY_OPEN", base, invite(w.g1, w.moi, w.chu))
	add("add_member: an active member", "MEMBERSHIP_ALREADY_OPEN", base, invite(w.g2, w.chu, w.hai))
	add("add_member: a person with no people row", "IntegrityError", base, invite(w.g1, w.missingPerson, w.chu))
	add("add_member: a missing context", "IntegrityError", base, invite(w.missingContext, w.la, w.chu))
	add("add_member: an inviter with no people row", "IntegrityError", base, invite(w.g1, w.la, w.missingPerson))
	add("add_member: a role that does not exist", "ValueError", base, invite(w.g1, w.la, w.chu, "owner"))

	membership := func(id string) oracleCall { return read("get_membership", args("membership_id", id)) }
	add("get_membership: named, link, left, erased, unnamed, missing", "", base,
		membership(w.mMoi), membership(w.mXin), membership(w.mRoi), membership(w.mXoa), membership(w.mTrong),
		membership(w.mLaiOld), membership(w.missingMembership))
	add("get_membership: under a Vietnam session TimeZone", "", vietnam, membership(w.mLaiNow), membership(w.mSom))

	accept := func(id string) oracleCall {
		return write("accept_membership", args("membership_id", id, "now", groupsNow), memberships)
	}
	add("accept_membership: a named invitation", "", base, accept(w.mMoi))
	add("accept_membership: a link request", "", base, accept(w.mXin))
	add("accept_membership: joined_at already at that instant", "", base, accept(w.mSom))
	add("accept_membership: a missing membership", "", base, accept(w.missingMembership))
	add("accept_membership: an active membership", "MEMBERSHIP_NOT_INVITED", base, accept(w.mBan))
	add("accept_membership: a membership that ended", "MEMBERSHIP_NOT_INVITED", base, accept(w.mRoi))
	add("flow.add_member_accept: an invitation and its acceptance in one transaction", "", base,
		write("flow.add_member_accept", args("context_id", w.g1, "person_id", w.la, "invited_by_id", w.ban, "now", groupsNow), memberships))

	leave := func(context, person string) oracleCall {
		return write("leave_context", args("context_id", context, "person_id", person, "now", groupsNow), memberships)
	}
	add("leave_context: an active member", "", base, leave(w.g1, w.ban))
	add("leave_context: the rejoined person's open row, not the old one", "", base, leave(w.g1, w.lai))
	add("leave_context: the last admin", "", vietnam, leave(w.g2, w.hai))
	add("leave_context: nothing to leave", "", base,
		leave(w.g1, w.moi), leave(w.g1, w.roi), leave(w.g1, w.la), leave(w.missingContext, w.chu), leave(w.g3, w.missingPerson))
	add("leave_context: leave, then the roster", "", base,
		leave(w.g1, w.trong), read("list_members", ctxArg(w.g1)), read("is_member", args("context_id", w.g1, "person_id", w.trong)))

	role := func(context, person string) oracleCall {
		return read("membership_role", args("context_id", context, "person_id", person))
	}
	add("membership_role: admin, member, invited, left, rejoined, stranger, missing", "", base,
		role(w.g1, w.chu), role(w.g1, w.ban), role(w.g1, w.moi), role(w.g1, w.roi), role(w.g1, w.lai),
		role(w.g2, w.chu), role(w.g2, w.hai), role(w.g1, w.la), role(w.missingContext, w.chu))

	// --- balances ---------------------------------------------------------------
	inputs := func(context string, ids any) oracleCall {
		return read("load_batch_inputs", args("context_id", context, "expense_version_ids", ids))
	}
	add("load_batch_inputs: the balances read model, with its row locks", "", base, inputs(w.g1, nil))
	add("load_batch_inputs: another group, an empty group, a missing group", "", base,
		inputs(w.g2, nil), inputs(w.g3, nil), inputs(w.missingContext, nil))
	add("load_batch_inputs: under a Vietnam session TimeZone", "", vietnam, inputs(w.g1, nil))
	add("load_batch_inputs: the batch path, every reason to be unavailable", "", base,
		inputs(w.g1, list(w.e4v1, w.e1v1, w.e1v2, w.e5v1, w.missingVersion, w.e2v1, w.e2v1, w.e3v1, w.e6v1)))
	add("load_batch_inputs: the batch path, only what is available", "", base, inputs(w.g1, list(w.e6v1, w.e2v1)))
	add("load_batch_inputs: the batch path, an empty tuple", "", base, inputs(w.g1, list()))
	add("load_batch_inputs: the batch path, nothing collectable", "", base, inputs(w.g1, list(w.e4v1)))
	receipts := func(context string) oracleCall { return read("load_confirmed_receipts", ctxArg(context)) }
	add("load_confirmed_receipts: sums across versions, a sender's confirmation left out", "", base,
		receipts(w.g1), receipts(w.g2), receipts(w.g3), receipts(w.missingContext))
	add("load_confirmed_receipts: under a Vietnam session TimeZone", "", vietnam, receipts(w.g1))

	// Receipt totals past int64. Each receipt fits its BIGINT column; their
	// SUM is numeric, and Python's int() of it keeps every digit, which the
	// tag compares by value.
	add("load_confirmed_receipts: two confirmations at the bigint maximum on one pair", "",
		join(base, receiptBatch(1, w.g3, w.la, receiptPair{w.ban, w.la, []int64{largestBigint, largestBigint}})),
		receipts(w.g3))
	add("load_confirmed_receipts: a pair past int64 between normal pairs and one exactly at the maximum", "",
		join(base, receiptBatch(2, w.g3, w.la,
			receiptPair{w.xoa, w.la, []int64{12_000, 3_000}},
			receiptPair{w.ban, w.la, []int64{largestBigint, 1}},
			receiptPair{w.roi, w.la, []int64{largestBigint - 5, 5}},
			receiptPair{w.chu, w.la, []int64{7_000}})),
		receipts(w.g3))
	add("load_confirmed_receipts: one pair past int64 across two batches, under a Vietnam session TimeZone", "",
		join(vietnam,
			receiptBatch(3, w.g3, w.la, receiptPair{w.ban, w.la, []int64{largestBigint}}),
			receiptBatch(4, w.g3, w.la, receiptPair{w.ban, w.la, []int64{largestBigint, largestBigint}},
				receiptPair{w.la, w.ban, []int64{40_000}})),
		receipts(w.g3))
	add("GET /contexts/{id}/balances with a pair total past int64", "",
		join(base,
			[]string{insertSQL("memberships", "id", fid(kindMembership, 0x80), "context_id", w.g3, "person_id", w.la,
				"state", "active", "joined_at", "2030-06-03T00:00:00Z", "created_at", "2030-06-02T00:00:00Z")},
			receiptBatch(5, w.g3, w.la, receiptPair{w.ban, w.la, []int64{largestBigint, largestBigint}})),
		read("is_member", args("context_id", w.g3, "person_id", w.la)), inputs(w.g3, nil), receipts(w.g3))

	// --- places -----------------------------------------------------------------
	place := func(id string) oracleCall { return read("get_place", args("place_id", id)) }
	add("get_place: JSON shapes, case, unicode, empty and missing ids", "", base,
		place("p-b"), place("p-A"), place("p-a"), place("p-ä"), place("p-big"), place("P-B"), place(""), place("khong-co"))
	add("get_place: true in kinds", "TypeError", join(base, []string{`UPDATE places SET kinds = 'true' WHERE id = 'p-c'`}), place("p-c"))

	// --- memories ---------------------------------------------------------------
	photoURL := "/contexts/" + w.g1 + "/photos/" + fid(kindImage, 0x31)
	memory := func(context, author, image string, caption, placeID, placeName any, now string) oracleCall {
		return write("create_memory", args("context_id", context, "author_id", author, "image_url", image,
			"caption", caption, "now", now, "place_id", placeID, "place_name", placeName), memories)
	}
	add("create_memory: a photograph without a place", "", base, memory(w.g1, w.ban, photoURL, nil, nil, nil, groupsNow))
	add("create_memory: at a place, with a caption, at a Vietnam offset", "", base,
		memory(w.g1, w.xoa, photoURL, "Chú thích 🙂 ' (dữ liệu mẫu)", "p-b", "Quán p-b (dữ liệu mẫu)", "2030-07-10T19:00:00.000001+07:00"))
	add("create_memory: a place id without a name", "IntegrityError", base, memory(w.g1, w.ban, photoURL, nil, "p-b", nil, groupsNow))
	add("create_memory: an empty image url", "IntegrityError", base, memory(w.g1, w.ban, "", nil, nil, nil, groupsNow))
	add("create_memory: a missing group", "IntegrityError", base, memory(w.missingContext, w.ban, photoURL, nil, nil, nil, groupsNow))
	add("create_memory: an author with no people row", "IntegrityError", base, memory(w.g1, w.missingPerson, photoURL, nil, nil, nil, groupsNow))
	checkin := func(context, author, placeID, placeName string, lat, lng float64, caption any) oracleCall {
		return write("create_checkin", args("context_id", context, "author_id", author, "place_id", placeID,
			"place_name", placeName, "lat", lat, "lng", lng, "caption", caption, "now", groupsNow), memories)
	}
	add("create_checkin: the catalogue's snapshot", "", base, checkin(w.g1, w.ban, "p-c", "Quán p-c (dữ liệu mẫu)", 10.7702, 106.7, nil))
	add("create_checkin: coordinates at the edges, a caption", "", base,
		checkin(w.g2, w.hai, "p-big", "Quán p-big (dữ liệu mẫu)", -90, 180, "Tới rồi (dữ liệu mẫu)"))
	add("create_checkin: a latitude off the Earth", "IntegrityError", base, checkin(w.g1, w.ban, "p-c", "Quán p-c (dữ liệu mẫu)", 90.5, 106.7, nil))
	add("create_checkin: an empty place name", "IntegrityError", base, checkin(w.g1, w.ban, "p-c", "", 10.7702, 106.7, nil))

	contextMemory := func(context, id string, viewer ...string) oracleCall {
		a := args("context_id", context, "memory_id", id)
		if len(viewer) == 1 {
			a["viewer_id"] = viewer[0]
		}
		return read("get_context_memory", a)
	}
	add("get_context_memory: counts, another group's memory, missing, viewers", "", base,
		contextMemory(w.g1, w.photo1), contextMemory(w.g1, w.photo1, w.chu), contextMemory(w.g1, w.photo1, w.la),
		contextMemory(w.g1, w.photoG2), contextMemory(w.g2, w.photoG2, w.chu), contextMemory(w.g1, w.missingMemory),
		contextMemory(w.g1, w.photoQuiet), contextMemory(w.g1, w.checkin3, w.missingPerson), contextMemory(w.g1, w.photo2))
	add("get_context_memory: under a Vietnam session TimeZone", "", vietnam, contextMemory(w.g1, w.photo2))

	react := func(memoryID, person, now string) oracleCall {
		return write("add_memory_reaction", args("memory_id", memoryID, "person_id", person, "now", now), reactions)
	}
	unreact := func(memoryID, person string) oracleCall {
		return write("remove_memory_reaction", args("memory_id", memoryID, "person_id", person), reactions)
	}
	add("add_memory_reaction: a first heart", "", base, react(w.photo2, w.ban, "2030-07-10T12:00:00.000001Z"))
	add("add_memory_reaction: a heart, then taking it back", "", base, react(w.photoQuiet, w.xoa, groupsNow), unreact(w.photoQuiet, w.xoa))
	add("add_memory_reaction: a second heart by the same person", "ALREADY_REACTED", base, react(w.photo1, w.chu, groupsNow))
	add("add_memory_reaction: a missing memory", "IntegrityError", base, react(w.missingMemory, w.chu, groupsNow))
	add("add_memory_reaction: a person with no people row", "IntegrityError", base, react(w.photo1, w.missingPerson, groupsNow))
	add("remove_memory_reaction: one's own, twice, somebody else's, a missing memory", "", base,
		unreact(w.photo1, w.ban), unreact(w.photo1, w.ban), unreact(w.photoG2, w.hai), unreact(w.missingMemory, w.chu))

	comment := func(memoryID, author, body, now string) oracleCall {
		return write("create_memory_comment", args("memory_id", memoryID, "author_id", author, "body", body, "now", now), comments)
	}
	add("create_memory_comment: by an unnamed author, then an erased one", "", base,
		comment(w.photo2, w.trong, "Không tên (dữ liệu mẫu)", groupsNow),
		comment(w.photo2, w.xoa, "Dòng một\nDòng hai ' \" \\ 🙂 (dữ liệu mẫu)", "2030-07-10T19:00:00+07:00"))
	add("create_memory_comment: an empty body", "IntegrityError", base, comment(w.photo1, w.ban, "", groupsNow))
	add("create_memory_comment: a missing memory", "IntegrityError", base, comment(w.missingMemory, w.ban, "x", groupsNow))
	add("create_memory_comment: an author with no people row", "IntegrityError", base, comment(w.photo1, w.missingPerson, "x", groupsNow))
	thread := func(memoryID string) oracleCall { return read("list_memory_comments", args("memory_id", memoryID)) }
	add("list_memory_comments: a created_at tie, none, a missing memory", "", base,
		thread(w.photo1), thread(w.photo2), thread(w.photoG2), thread(w.missingMemory))
	add("list_memory_comments: under a Vietnam session TimeZone", "", vietnam, thread(w.photo1))

	// --- orders and refusals more than one case must pin ---------------------------
	//
	// unavailable_version_ids is sorted by UUID bytes and decides the order
	// POST /batches reports. Each case below leaves three to six ids in it,
	// seeded or asked for out of order, that tie with their neighbours on every
	// character but the last few, so only a comparison of the whole id orders
	// them; the reasons an id is there (not returned, no allocations, already a
	// source) interleave in the sorted result.
	tied := func(suffix string) string { return "f40000fe-aaaa-4aaa-8aaa-aaaaaaaaa" + suffix }
	nearE3 := func(last string) string { return w.e3v1[:len(w.e3v1)-1] + last }
	tiedLedger := join(base,
		orderedLedger(1, w.g3, w.la, orderedVersion{tied("b00"), nil}),
		orderedLedger(2, w.g3, w.la, orderedVersion{tied("a0c"), nil}),
		orderedLedger(3, w.g3, w.la, orderedVersion{tied("a0b"), []share{{w.ban, 5_000}}}, orderedVersion{tied("a0d"), nil}),
		orderedLedger(4, w.g3, w.la, orderedVersion{tied("a1a"), nil}),
		orderedLedger(5, w.g3, w.la, orderedVersion{tied("a0e"), []share{{w.la, 1_000}, {w.ban, 2_000}}}))
	add("load_batch_inputs: order of unavailable ids tied on all but their last characters", "", tiedLedger,
		inputs(w.g3, nil))
	add("load_batch_inputs: order of requested ids that are not returned", "", base,
		inputs(w.g1, list(tied("a2f"), tied("a0f"), w.missingVersion, w.e5v1, tied("a2f"), tied("a10"))))
	add("load_batch_inputs: order across every reason to be unavailable", "",
		join(tiedLedger, orderedLedger(6, w.g3, w.la, orderedVersion{tied("a1b"), []share{{w.ban, 3_000}}}),
			[]string{insertSQL("collection_obligation_sources", "obligation_id", fid(kindObligation, 2),
				"confirmed_allocation_id", orderedAllocationID(5, 0, 1), "amount_vnd", int64(2_000))}),
		inputs(w.g3, list(tied("b00"), tied("a1b"), tied("a0e"), tied("a09"), tied("a0b"), tied("a0d"), w.e1v2)))
	add("load_batch_inputs: order around a seeded id that differs only in its last character", "",
		join(base, orderedLedger(7, w.g1, w.chu, orderedVersion{nearE3("b"), nil}),
			orderedLedger(8, w.g1, w.chu, orderedVersion{nearE3("0"), nil})),
		inputs(w.g1, nil))

	// ALREADY_REACTED is the one conflict code the heart route branches on.
	add("add_memory_reaction: a second heart on another group's photograph", "ALREADY_REACTED", base, react(w.photoG2, w.chu, groupsNow))
	add("add_memory_reaction: a second heart under a Vietnam session TimeZone", "ALREADY_REACTED", vietnam,
		react(w.photo1, w.ban, "2030-07-10T19:00:00+07:00"))
	add("POST /contexts/{id}/memories/{memory_id}/reactions a second time", "ALREADY_REACTED", base,
		read("is_member", args("context_id", w.g1, "person_id", w.lai)), contextMemory(w.g1, w.photo1),
		react(w.photo1, w.lai, groupsNow))

	// A theme equal to the stored one is not a change.
	add("update_context: the stored theme alone writes nothing", "", base, update(w.g1, [2]string{"theme", "mac-dinh"}))
	add("update_context: a new name with the stored theme", "", base,
		update(w.g2, [2]string{"display_name", "Tên khác (dữ liệu mẫu)"}, [2]string{"theme", "bien-dem"}))
	add("PATCH /contexts/{id} with the theme it already has", "", base,
		read("is_member", args("context_id", w.g2, "person_id", w.chu)), update(w.g2, [2]string{"theme", "bien-dem"}))

	// --- the calls each route makes, in its order ------------------------------------
	member := func(context, person string) oracleCall {
		return read("is_member", args("context_id", context, "person_id", person))
	}
	person := func(id string) oracleCall { return read("get_person", args("person_id", id)) }
	add("POST /contexts", "", base, person(w.la), flow(w.la, "Nhóm của tôi (dữ liệu mẫu)"))
	add("PATCH /contexts/{id} with a theme", "", base, member(w.g1, w.ban), update(w.g1, [2]string{"theme", "hoang-hon"}))
	add("POST /contexts/{id}/members", "", base,
		member(w.g1, w.chu), role(w.g1, w.chu), read("get_context", ctxArg(w.g1)), person(w.la), invite(w.g1, w.la, w.chu))
	add("POST /memberships/{id}/accept for a link request", "", base,
		membership(w.mXin), member(w.g1, w.ban), read("get_context", ctxArg(w.g1)), accept(w.mXin))
	add("DELETE /contexts/{id}/members/{person_id}", "", base,
		member(w.g1, w.ban), read("get_context", ctxArg(w.g1)), leave(w.g1, w.ban))
	add("GET /contexts/{id}/members", "", base, member(w.g1, w.chu), read("list_members", ctxArg(w.g1)))
	add("GET /contexts/{id}/balances", "", base, member(w.g1, w.chu), inputs(w.g1, nil), receipts(w.g1))
	add("GET /contexts/{id} of a pair", "", base,
		member(w.pair, w.chu), read("get_context", ctxArg(w.pair)), read("list_members", ctxArg(w.pair)))
	add("POST /contexts/{id}/memories at a place", "", base,
		member(w.g1, w.ban), place("p-b"), memory(w.g1, w.ban, photoURL, "Ở quán (dữ liệu mẫu)", "p-b", "Quán p-b (dữ liệu mẫu)", groupsNow))
	add("GET /contexts/{id}/memories", "", base, member(w.g1, w.chu),
		read("list_memories", args("context_id", w.g1, "limit", 50, "kind", nil, "place_id", nil, "viewer_id", w.chu)))
	add("POST /contexts/{id}/checkins", "", base,
		member(w.g1, w.lai), place("p-c"), checkin(w.g1, w.lai, "p-c", "Quán p-c (dữ liệu mẫu)", 10.7702, 106.7, "Đến rồi (dữ liệu mẫu)"))
	add("GET /contexts/{id}/widget", "", base,
		member(w.g1, w.ban), read("list_memories", args("context_id", w.g1, "limit", 1, "kind", "photo")), person(w.xoa))
	add("POST /contexts/{id}/memories/{memory_id}/reactions", "", base,
		member(w.g1, w.ban), contextMemory(w.g1, w.photo2), react(w.photo2, w.ban, groupsNow), contextMemory(w.g1, w.photo2))
	add("DELETE /contexts/{id}/memories/{memory_id}/reactions", "", base,
		member(w.g1, w.chu), contextMemory(w.g1, w.photo1), unreact(w.photo1, w.chu))
	add("POST /contexts/{id}/memories/{memory_id}/comments", "", base,
		member(w.g1, w.trong), contextMemory(w.g1, w.photo1), comment(w.photo1, w.trong, "Đẹp quá (dữ liệu mẫu)", groupsNow), person(w.trong))
	add("GET /contexts/{id}/memories/{memory_id}/comments", "", base,
		member(w.g1, w.chu), contextMemory(w.g1, w.photo1), thread(w.photo1), person(w.trong), person(w.ban), person(w.xoa))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

func join(parts ...[]string) []string {
	out := []string{}
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// groupsMethods is every method this port covers; the corpus must reach each
// with at least one normal return.
var groupsMethods = []string{
	"create_context", "update_context", "add_member", "get_membership", "accept_membership", "leave_context",
	"membership_role", "flow.create_context", "flow.add_member_accept", "load_batch_inputs",
	"load_confirmed_receipts", "get_place", "create_memory", "create_checkin", "get_context_memory",
	"add_memory_reaction", "remove_memory_reaction", "create_memory_comment", "list_memory_comments",
}

func TestGroupsRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of groups_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_groups_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	// pgrowlocks is database-wide; public is outside both sides' search_path,
	// so the probe calls it qualified and no table dump can see it.
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "groups_oracle_")

	cases, built := groupsOracleCases()
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_groups_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_groups_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	returned := map[string]int{}
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
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
			if s.Error == nil {
				returned[c.Steps[j].Call]++
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}

		// The fixture must reach what the case is named for.
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
			t.Errorf("case %q: python ended in %q, the case is written for %q", c.Name, end, cases[i].wantEnd)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python refused at step %d of %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}

		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, normalizeGroupsSteps(pySteps, c), normalizeGroupsSteps(runGroupsGoCase(t, pool, c), c))
		})
	}
	for _, method := range groupsMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	t.Logf("groups repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which "+
		"%d table rows, %d generated ids bound, %d mismatches", tally.cases, tally.steps, tally.results, tally.errors,
		tally.statements, tally.probeRows, tally.tableRows, tally.generated, tally.mismatches)
}

// optionalFloat reads a coordinate the oracle may send as null. A check-in at a
// place with no coordinates still happens; it just carries none.
func optionalFloat(args map[string]any, key string) *float64 {
	value, ok := args[key].(float64)
	if !ok {
		return nil
	}
	return &value
}
