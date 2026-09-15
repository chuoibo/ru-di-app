//go:build postgres

package repo

// Differential test of the W8 repository methods (the two-person notebook and
// its sheets) against the real SqlAlchemyApiRepository, driven through
// scripts/render_pair_repo_oracle.py. Same design as
// guests_oracle_postgres_test.go, whose case format, value tags, statement
// normalisation, conflict tagging, row lock probe and comparison it reuses,
// with two additions:
//
//   - Route steps. A `route.*` step runs, on the Python side, the real
//     ApiService method behind one of the nineteen routes, and on the Go side
//     the repository calls pair_repo_routes_postgres_test.go says that method
//     makes. Their statements, row locks and table dumps must agree, and a
//     refusal must come at the same statement with the same status and code.
//     This is what pins the call order the Go route port has to keep.
//   - Coverage the corpus must reach in Python, checked after the run: every
//     method and every route answering normally at least once, and every
//     write branch named in pairBranches issuing its statement.
//
// TestPairRowLocksSerialiseRequests is Go only: the notebook lock, the paper
// lock, the consent lock and the close lock each make a second request wait.
//
// Without CORE_PYTHON_IMAGE both tests skip; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

var (
	pairNotebooksDump    = orderedDump("pair_notebooks t", fixtureFirst("t.id")+", t.context_id, t.id")
	pairCyclesDump       = orderedDump("pair_notebook_cycles t", fixtureFirst("t.id")+", t.notebook_id, t.created_at, t.id")
	pairParticipantsDump = orderedDump("pair_cycle_participants t JOIN pair_notebook_cycles c ON c.id = t.cycle_id",
		fixtureFirst("t.cycle_id")+", c.notebook_id, c.created_at, t.created_at, t.person_id")
	pairProposalsDump  = orderedDump("pair_consent_proposals t", fixtureFirst("t.id")+", t.created_at, t.purpose, t.proposed_by_id, t.id")
	pairConsentsDump   = orderedDump("pair_consents t", fixtureFirst("t.id")+", t.created_at, t.person_id, t.id")
	pairCoupleDump     = orderedDump("active_couple_members t", "t.person_id")
	pairConstraintDump = orderedDump("pair_shared_constraints t", fixtureFirst("t.cycle_id")+", t.cycle_id, t.owner_id, t.kind")
	pairPapersDump     = orderedDump("pair_papers t", fixtureFirst("t.id")+", t.context_id, t.created_at, t.id")
	pairVersionsDump   = orderedDump("pair_paper_versions t", fixtureFirst("t.paper_id")+", t.paper_id, t.version")
	pairViewsDump      = orderedDump("pair_paper_views t", "t.paper_id, t.version, t.person_id")
	pairResponsesDump  = orderedDump("pair_paper_responses t", fixtureFirst("t.id")+", t.paper_id, t.version, t.created_at, t.kind, t.person_id, t.id")
	pairKeepsDump      = orderedDump("pair_paper_keeps t", fixtureFirst("t.id")+", t.paper_id, t.created_at, t.person_id, t.id")
	pairLinksDump      = orderedDump("pair_paper_outings t", "t.paper_id")
	pairOutingsDump    = orderedDump("outings t", fixtureFirst("t.id")+", t.context_id, t.created_at, t.id")
)

// probePairRowLocks lists the row locks held on every table a W8 method locks
// FOR UPDATE, updates, deletes or references through a foreign key it writes,
// each row named by its key.
var probePairRowLocks = func() string {
	var parts []string
	for _, t := range []struct{ table, key string }{
		{"people", "t.id::text"}, {"contexts", "t.id::text"}, {"memberships", "t.id::text"}, {"outings", "t.id::text"},
		{"pair_notebooks", "t.id::text"}, {"pair_notebook_cycles", "t.id::text"},
		{"pair_consent_proposals", "t.id::text"}, {"pair_consents", "t.id::text"},
		{"active_couple_members", "t.person_id::text"}, {"pair_papers", "t.id::text"},
		{"pair_paper_versions", "t.paper_id::text || '/' || t.version"},
		{"pair_shared_constraints", "t.cycle_id::text || '/' || t.owner_id::text || '/' || t.kind"},
		{"pair_paper_outings", "t.paper_id::text"},
	} {
		parts = append(parts, `SELECT '`+t.table+`' AS rel, `+t.key+` AS id, array_to_string(r.modes, ',') AS modes
			FROM public.pgrowlocks('`+t.table+`') r LEFT JOIN `+t.table+` t ON t.ctid = r.locked_row`)
	}
	return `SELECT l.rel || ' ' || coalesce(l.id, '(superseded row)') || ' ' || l.modes FROM (` +
		strings.Join(parts, " UNION ALL ") + `) l ORDER BY 1`
}()

// ---------------------------------------------------------------------------
// Tags and the Go side
// ---------------------------------------------------------------------------

func tPairProposal(p PairProposal) any {
	return tRecord("PairProposalRecord", "id", tUUID(p.ID), "cycle_id", tUUID(p.CycleID), "purpose", tStr(p.Purpose),
		"proposed_by_id", tUUID(p.ProposedByID), "terms_version", tInt32(p.TermsVersion),
		"completed_at", optional(p.CompletedAt, tInstant), "created_at", tInstant(p.CreatedAt),
		"expires_at", tInstant(p.ExpiresAt))
}

func tPairConstraint(c PairConstraint) any {
	return tRecord("PairConstraintRecord", "owner_id", tUUID(c.OwnerID), "kind", tStr(c.Kind), "content", tStr(c.Content),
		"version", tInt32(c.Version), "updated_at", tInstant(c.UpdatedAt))
}

func tPairKeep(k PairKeep) any {
	return tRecord("PairKeepRecord", "id", tUUID(k.ID), "person_id", tUUID(k.PersonID), "line", tStr(k.Line),
		"created_at", tInstant(k.CreatedAt))
}

func tPairPaper(p PairPaper) any {
	versions, views, responses, keeps := []any{}, []any{}, []any{}, []any{}
	for _, v := range p.Versions {
		versions = append(versions, tRecord("PairVersionRecord", "version", tInt(v.Version), "content", tJSON(v.Content),
			"ly_do", optional(v.LyDo, tStr), "nguon", tJSON(v.Nguon), "author_type", tStr(v.AuthorType),
			"sent_at", optional(v.SentAt, tInstant), "sent_by", optional(v.SentBy, tUUID)))
	}
	for _, v := range p.Views {
		views = append(views, tRecord("PairViewRecord", "version", tInt(v.Version), "person_id", tUUID(v.PersonID),
			"seen_at", tInstant(v.SeenAt)))
	}
	for _, r := range p.Responses {
		responses = append(responses, tRecord("PairResponseRecord", "version", tInt(r.Version),
			"person_id", tUUID(r.PersonID), "kind", tStr(r.Kind), "created_at", tInstant(r.CreatedAt)))
	}
	for _, k := range p.Keeps {
		keeps = append(keeps, tPairKeep(k))
	}
	return tRecord("PairPaperRecord", "id", tUUID(p.ID), "context_id", tUUID(p.ContextID),
		"cycle_id", optional(p.CycleID, tUUID), "is_temporary", tBool(p.IsTemporary),
		"draft_owner_id", tUUID(p.DraftOwnerID), "state", tStr(p.State), "current_version", tInt(p.CurrentVersion),
		"tuan", tDay(p.Tuan), "expires_at", tInstant(p.ExpiresAt), "created_at", tInstant(p.CreatedAt),
		"done_recorded_by_id", optional(p.DoneRecordedByID, tUUID),
		"done_recorded_at", optional(p.DoneRecordedAt, tInstant), "outing_id", optional(p.OutingID, tUUID),
		"versions", tSeq(versions), "views", tSeq(views), "responses", tSeq(responses), "keeps", tSeq(keeps))
}

func argJSON(v any) json.RawMessage {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}

func argDay(text string) time.Time {
	day, err := time.Parse("2006-01-02", text)
	if err != nil {
		panic(err)
	}
	return day
}

func argOptionalInstant(v any) *time.Time {
	if s, ok := v.(string); ok {
		instant := argInstant(s)
		return &instant
	}
	return nil
}

func argOutingInput(a map[string]any) OutingInput {
	return OutingInput{ContextID: argString(a, "context_id"), CreatedByID: argString(a, "created_by_id"),
		Title: argString(a, "title"), StartsOn: argDay(argString(a, "starts_on")), EndsOn: argDay(argString(a, "ends_on")),
		Headcount: argNumber(a["headcount"]), BudgetPerPersonVND: argNumber(a["budget_per_person_vnd"]),
		Now: argInstant(argString(a, "now"))}
}

func pairRepoGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	now := func() time.Time { return argInstant(s("now")) }
	if strings.HasPrefix(method, "route.") {
		return pairRouteGo(repo, method, a)
	}
	switch method {
	case "create_pair_notebook":
		n, err := repo.CreatePairNotebook(bg, s("context_id"), now())
		return tNotebook(&n), err
	case "lock_pair_notebook":
		n, err := repo.LockPairNotebook(bg, s("context_id"))
		return tNotebook(n), err
	case "open_pair_cycle":
		id, err := repo.OpenPairCycle(bg, s("notebook_id"), argStrings(a["participants"]), argNumber(a["terms_version"]), now())
		return tUUID(id), err
	case "activate_pair_cycle":
		return nil, repo.ActivatePairCycle(bg, s("cycle_id"), now())
	case "close_pair_cycle":
		return nil, repo.ClosePairCycle(bg, s("cycle_id"), now())
	case "create_consent_proposal":
		p, err := repo.CreateConsentProposal(bg, ConsentProposalInput{CycleID: s("cycle_id"), Purpose: s("purpose"),
			ProposedByID: s("proposed_by_id"), TermsVersion: argNumber(a["terms_version"]),
			ExpiresAt: argInstant(s("expires_at")), Now: now()})
		return tPairProposal(p), err
	case "get_consent_proposal":
		p, err := repo.GetConsentProposal(bg, s("proposal_id"))
		return nilOr(p, tPairProposal), err
	case "grant_consent":
		return nil, repo.GrantConsent(bg, s("proposal_id"), s("person_id"), now())
	case "complete_consent_proposal":
		return nil, repo.CompleteConsentProposal(bg, s("proposal_id"), now())
	case "revoke_consents":
		n, err := repo.RevokeConsents(bg, s("cycle_id"), s("purpose"), s("person_id"), now())
		return tInt(int64(n)), err
	case "set_couple_member":
		return nil, repo.SetCoupleMember(bg, s("person_id"), s("cycle_id"), now())
	case "clear_couple_member":
		return nil, repo.ClearCoupleMember(bg, s("person_id"))
	case "set_pair_constraint":
		c, err := repo.SetPairConstraint(bg, PairConstraintInput{CycleID: s("cycle_id"), OwnerID: s("owner_id"),
			Kind: s("kind"), Content: s("content"), Now: now()})
		return tPairConstraint(c), err
	case "delete_pair_constraint":
		ok, err := repo.DeletePairConstraint(bg, s("cycle_id"), s("owner_id"), s("kind"))
		return tBool(ok), err
	case "create_pair_paper":
		p, err := repo.CreatePairPaper(bg, PairPaperInput{ContextID: s("context_id"), CycleID: argText(a["cycle_id"]),
			DraftOwnerID: s("draft_owner_id"), Tuan: argDay(s("tuan")), ExpiresAt: argInstant(s("expires_at")),
			Content: argJSON(a["content"]), LyDo: argText(a["ly_do"]), Nguon: argJSON(a["nguon"]),
			AuthorType: s("author_type"), Now: now()})
		return tPairPaper(p), err
	case "get_pair_paper":
		p, err := repo.GetPairPaper(bg, s("paper_id"))
		return nilOr(p, tPairPaper), err
	case "lock_pair_paper":
		p, err := repo.LockPairPaper(bg, s("paper_id"))
		return nilOr(p, tPairPaper), err
	case "list_pair_papers":
		papers, err := repo.ListPairPapers(bg, s("context_id"))
		items := []any{}
		for _, p := range papers {
			items = append(items, tPairPaper(p))
		}
		return tSeq(items), err
	case "update_pair_draft":
		return nil, repo.UpdatePairDraft(bg, s("paper_id"), argJSON(a["content"]), argText(a["ly_do"]))
	case "add_paper_version":
		return nil, repo.AddPaperVersion(bg, PaperVersionInput{PaperID: s("paper_id"), Version: argNumber(a["version"]),
			Content: argJSON(a["content"]), LyDo: argText(a["ly_do"]), Nguon: argJSON(a["nguon"]),
			AuthorType: s("author_type"), SentAt: argOptionalInstant(a["sent_at"]), SentBy: argText(a["sent_by"]), Now: now()})
	case "mark_version_sent":
		return nil, repo.MarkVersionSent(bg, s("paper_id"), argNumber(a["version"]), argText(a["sent_by"]), now())
	case "set_paper_state":
		in := PaperStateInput{PaperID: s("paper_id"), State: s("state"), Now: now()}
		if v, ok := a["current_version"]; ok {
			n := argNumber(v)
			in.CurrentVersion = &n
		}
		if v, ok := a["recorded_by_id"]; ok {
			in.RecordedByID = argText(v)
		}
		return nil, repo.SetPaperState(bg, in)
	case "mark_paper_viewed":
		seen, err := repo.MarkPaperViewed(bg, s("paper_id"), argNumber(a["version"]), s("person_id"), now())
		return tInstant(seen), err
	case "add_paper_response":
		return nil, repo.AddPaperResponse(bg, PaperResponseInput{PaperID: s("paper_id"), Version: argNumber(a["version"]),
			PersonID: s("person_id"), Kind: s("kind"), Now: now()})
	case "link_paper_outing":
		return nil, repo.LinkPaperOuting(bg, s("paper_id"), argNumber(a["version"]), s("outing_id"), now())
	case "get_paper_outing":
		id, err := repo.GetPaperOuting(bg, s("paper_id"))
		return nilOr(id, tUUID), err
	case "add_paper_keep":
		k, err := repo.AddPaperKeep(bg, s("paper_id"), s("person_id"), s("line"), now())
		return tPairKeep(k), err
	case "close_open_pair_papers":
		c, err := repo.CloseOpenPairPapers(bg, s("context_id"), now())
		return tv("dict", []any{[]any{tStr("bo"), tInt(c.Bo)}, []any{tStr("huy"), tInt(c.Huy)}}), err
	case "create_outing":
		o, err := repo.CreateOuting(bg, argOutingInput(a))
		return tOuting(o), err
	case "flow.open_cycle_propose":
		cycle, err := repo.OpenPairCycle(bg, s("notebook_id"), argStrings(a["participants"]), argNumber(a["terms_version"]), now())
		if err != nil {
			return nil, err
		}
		p, err := repo.CreateConsentProposal(bg, ConsentProposalInput{CycleID: cycle, Purpose: s("purpose"),
			ProposedByID: s("proposed_by_id"), TermsVersion: argNumber(a["terms_version"]),
			ExpiresAt: argInstant(s("expires_at")), Now: now()})
		if err != nil {
			return nil, err
		}
		if err := repo.GrantConsent(bg, p.ID, p.ProposedByID, now()); err != nil {
			return nil, err
		}
		return tSeq([]any{tUUID(cycle), tPairProposal(p)}), nil
	case "flow.create_outing_link":
		o, err := repo.CreateOuting(bg, argOutingInput(a))
		if err != nil {
			return nil, err
		}
		if err := repo.LinkPaperOuting(bg, s("paper_id"), argNumber(a["version"]), o.ID, now()); err != nil {
			return nil, err
		}
		linked, err := repo.GetPaperOuting(bg, s("paper_id"))
		return tSeq([]any{tOuting(o), nilOr(linked, tUUID)}), err
	}
	return pairGoCall(repo, method, a)
}

// pairRepoGoError is socialGoError plus the ValueError a stored JSON shape
// raises and the ApiProblem a route answers.
func pairRepoGoError(err error) map[string]any {
	var refusal *routeRefusal
	if errors.As(err, &refusal) {
		return map[string]any{"type": "ApiProblem", "sqlstate": nil, "constraint": nil, "code": refusal.Error(), "cause": nil}
	}
	out := socialGoError(err)
	if errors.Is(err, ErrPythonValueError) {
		out["type"] = "ValueError"
	}
	return out
}

func runPairRepoGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
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
		value, err := pairRepoGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = pairRepoGoError(err)
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

// normalizePairSteps is normalizeMoneySteps for probePairRowLocks.
func normalizePairSteps(steps []any, c oracleCase) []any {
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
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
			case probePairRowLocks:
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

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func pairRepoOracleCases() ([]socialCase, oracleSpec) {
	w := newPairRepoWorld()
	var cases []socialCase
	add := func(name, wantEnd string, setup []string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps}, wantEnd})
	}
	probes := append(append([]string{probeLocks, probeWrites, probePairRowLocks}, pairNotebooksDump, pairCyclesDump,
		pairParticipantsDump, pairProposalsDump, pairConsentsDump, pairCoupleDump, pairConstraintDump, pairPapersDump,
		pairVersionsDump, pairViewsDump, pairResponsesDump, pairKeepsDump, pairLinksDump, pairOutingsDump), probeNow)
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	step := func(call string, pairs ...any) oracleCall {
		return oracleCall{Call: call, Args: args(pairs...), Before: append([]string{}, writesBaseline...),
			Probes: append([]string{}, probes...)}
	}
	// tweak runs SQL just before the step, ahead of the write counters' baseline.
	tweak := func(sql string, s oracleCall) oracleCall {
		s.Before = append([]string{sql}, s.Before...)
		return s
	}
	later := func(n int) string { return fmt.Sprintf("2030-09-18T05:0%d:00.654321Z", n) }
	base := w.sql
	vietnam := join([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base)
	const now = pairNow
	const inAWeek = "2030-09-25T05:00:00.654321Z"
	const vietnamNow = "2030-09-18T12:00:00.654321+07:00"
	missingCycle, missingProposal := fid(kindCycle, 0xff), fid(kindProposal, 0xff)
	hostile := "Đừng ' \" \\ \n\t <script>alert(1)</script> '; DROP TABLE pair_papers; -- 🙂 ẞ %s %(x)s $1 (dữ liệu mẫu)"
	stop := func(gio, viec string, place any, canKiem any) map[string]any {
		return map[string]any{"gio": gio, "viec": viec, "place_id": place, "can_kiem": canKiem}
	}
	content := func(day string, stops ...map[string]any) map[string]any {
		items := []any{}
		for _, s := range stops {
			items = append(items, s)
		}
		return map[string]any{"ngay": day, "chang": items}
	}
	dinner := stop("19:00", "Ăn tối (dữ liệu mẫu)", nil, true)
	nguon := map[string]any{"scope": "chung", "dung": []any{"routine", "rang_buoc"}, "luc": "2030-09-18T05:00:00.654321+00:00"}

	// --- the notebook and its cycles -------------------------------------------------
	add("create_pair_notebook: a conversation without one, then locked", "", base,
		step("create_pair_notebook", "context_id", w.be, "now", now), step("lock_pair_notebook", "context_id", w.be))
	add("create_pair_notebook: a second notebook for the conversation", "IntegrityError", base,
		step("create_pair_notebook", "context_id", w.ab, "now", now))
	add("create_pair_notebook: a group context", "IntegrityError", base,
		step("create_pair_notebook", "context_id", w.group, "now", now))
	add("lock_pair_notebook: active, pending, closed, never opened, none, group, missing", "", base,
		step("lock_pair_notebook", "context_id", w.ab), step("lock_pair_notebook", "context_id", w.ae),
		step("lock_pair_notebook", "context_id", w.bc), step("lock_pair_notebook", "context_id", w.bg),
		step("lock_pair_notebook", "context_id", w.be), step("lock_pair_notebook", "context_id", w.group),
		step("lock_pair_notebook", "context_id", w.missingContext))
	add("lock_pair_notebook: under a Vietnam session TimeZone", "", vietnam, step("lock_pair_notebook", "context_id", w.ab))
	add("open_pair_cycle: a new cycle beside a closed one, then read under lock", "", base,
		step("open_pair_cycle", "notebook_id", w.nbBC, "participants", []any{w.chi, w.binh}, "terms_version", 1, "now", now),
		step("lock_pair_notebook", "context_id", w.bc))
	add("open_pair_cycle: no participants", "", base,
		step("open_pair_cycle", "notebook_id", w.nbBG, "participants", []any{}, "terms_version", 2, "now", now),
		step("lock_pair_notebook", "context_id", w.bg))
	add("open_pair_cycle: a second live cycle", "IntegrityError", base,
		step("open_pair_cycle", "notebook_id", w.nbAB, "participants", []any{w.an, w.binh}, "terms_version", 1, "now", now))
	add("open_pair_cycle: a participant with no people row", "IntegrityError", base,
		step("open_pair_cycle", "notebook_id", w.nbBG, "participants", []any{w.binh, w.missingPerson}, "terms_version", 1, "now", now))
	add("open_pair_cycle: a terms version past INTEGER", "DataError", base,
		step("open_pair_cycle", "notebook_id", w.nbBG, "participants", []any{w.binh}, "terms_version", int64(1)<<31, "now", now))
	add("activate_pair_cycle: a pending cycle, then again", "", base,
		step("activate_pair_cycle", "cycle_id", w.cyAE, "now", now), step("activate_pair_cycle", "cycle_id", w.cyAE, "now", later(1)),
		step("lock_pair_notebook", "context_id", w.ae))
	add("activate_pair_cycle: active, closed and missing cycles", "", base,
		step("activate_pair_cycle", "cycle_id", w.cyAB, "now", now), step("activate_pair_cycle", "cycle_id", w.cyBC, "now", now),
		step("activate_pair_cycle", "cycle_id", missingCycle, "now", now))
	add("activate_pair_cycle: a pending cycle opened once before", "", base,
		tweak("UPDATE pair_notebook_cycles SET opened_at = '2030-09-17T00:00:00Z' WHERE id = '"+w.cyAE+"'",
			step("activate_pair_cycle", "cycle_id", w.cyAE, "now", now)))
	add("activate_pair_cycle: an active cycle whose opened_at is empty", "", base,
		tweak("UPDATE pair_notebook_cycles SET opened_at = NULL WHERE id = '"+w.cyAB+"'",
			step("activate_pair_cycle", "cycle_id", w.cyAB, "now", now)))
	add("close_pair_cycle: an active cycle, then again", "", base,
		step("close_pair_cycle", "cycle_id", w.cyAB, "now", now), step("close_pair_cycle", "cycle_id", w.cyAB, "now", later(1)),
		step("lock_pair_notebook", "context_id", w.ab))
	add("close_pair_cycle: closed and missing cycles", "", base,
		step("close_pair_cycle", "cycle_id", w.cyBC, "now", now), step("close_pair_cycle", "cycle_id", missingCycle, "now", now))
	add("close_pair_cycle: a pending cycle under a Vietnam session TimeZone", "", vietnam,
		step("close_pair_cycle", "cycle_id", w.cyAE, "now", vietnamNow))

	// --- proposals and grants ------------------------------------------------------------
	propose := func(cycle, purpose, by, expires, at string) oracleCall {
		return step("create_consent_proposal", "cycle_id", cycle, "purpose", purpose, "proposed_by_id", by,
			"terms_version", 1, "expires_at", expires, "now", at)
	}
	add("create_consent_proposal: an offer, then the notebook read back", "", base,
		propose(w.cyAB, "bat_doi", w.binh, inAWeek, now), step("lock_pair_notebook", "context_id", w.ab))
	add("create_consent_proposal: an unknown purpose", "IntegrityError", base, propose(w.cyAB, "doc chat'; --", w.an, inAWeek, now))
	add("create_consent_proposal: a deadline not after its creation", "IntegrityError", base, propose(w.cyAB, "doc_chat", w.an, now, now))
	add("create_consent_proposal: a missing cycle", "IntegrityError", base, propose(missingCycle, "lap_so", w.an, inAWeek, now))
	add("get_consent_proposal: complete, open and missing", "", base,
		step("get_consent_proposal", "proposal_id", w.prLapSoAB), step("get_consent_proposal", "proposal_id", w.prBatDoiAB),
		step("get_consent_proposal", "proposal_id", missingProposal))
	add("flow.open_cycle_propose: a new cycle, its first offer and the asker's grant", "", base,
		step("flow.open_cycle_propose", "notebook_id", w.nbBC, "participants", []any{w.chi, w.binh}, "terms_version", 1,
			"purpose", "lap_so", "proposed_by_id", w.chi, "expires_at", inAWeek, "now", now),
		step("lock_pair_notebook", "context_id", w.bc))
	add("grant_consent: a first answer", "", base, step("grant_consent", "proposal_id", w.prLapSoAE, "person_id", w.em, "now", now))
	add("grant_consent: a row never granted is granted", "", base,
		step("grant_consent", "proposal_id", w.prBatDoiAB, "person_id", w.binh, "now", now))
	add("grant_consent: a standing grant and a revoked grant are left alone", "", base,
		step("grant_consent", "proposal_id", w.prDocChatAB, "person_id", w.an, "now", now),
		step("grant_consent", "proposal_id", w.prOldAB, "person_id", w.binh, "now", now))
	add("grant_consent: twice in one transaction", "", base,
		step("grant_consent", "proposal_id", w.prLapSoAE, "person_id", w.em, "now", now),
		step("grant_consent", "proposal_id", w.prLapSoAE, "person_id", w.em, "now", later(1)))
	add("grant_consent: a missing proposal", "IntegrityError", base,
		step("grant_consent", "proposal_id", missingProposal, "person_id", w.an, "now", now))
	add("complete_consent_proposal: an open offer, again, a complete one and a missing one", "", base,
		step("complete_consent_proposal", "proposal_id", w.prBatDoiAB, "now", now),
		step("complete_consent_proposal", "proposal_id", w.prBatDoiAB, "now", later(1)),
		step("complete_consent_proposal", "proposal_id", w.prLapSoAB, "now", now),
		step("complete_consent_proposal", "proposal_id", missingProposal, "now", now))
	add("revoke_consents: live grants on two offers of one purpose, one of them lapsed", "", base,
		step("revoke_consents", "cycle_id", w.cyAB, "purpose", "doc_chat", "person_id", w.an, "now", now))
	add("revoke_consents: an unanswered row, a grant, the same grant again", "", base,
		step("revoke_consents", "cycle_id", w.cyAB, "purpose", "bat_doi", "person_id", w.binh, "now", now),
		step("revoke_consents", "cycle_id", w.cyAB, "purpose", "lap_so", "person_id", w.an, "now", now),
		step("revoke_consents", "cycle_id", w.cyAB, "purpose", "lap_so", "person_id", w.an, "now", later(1)))
	add("revoke_consents: under a Vietnam session TimeZone", "", vietnam,
		step("revoke_consents", "cycle_id", w.cyCD, "purpose", "bat_doi", "person_id", w.dung, "now", vietnamNow))

	// --- the couple slot and constraints ----------------------------------------------------
	add("set_couple_member: a new row, the same cycle again, a row already there", "", base,
		step("set_couple_member", "person_id", w.an, "cycle_id", w.cyAB, "now", now),
		step("set_couple_member", "person_id", w.an, "cycle_id", w.cyAB, "now", later(1)),
		step("set_couple_member", "person_id", w.chi, "cycle_id", w.cyCD, "now", now))
	add("set_couple_member: somebody already a couple elsewhere", "couple_slot_taken", base,
		step("set_couple_member", "person_id", w.chi, "cycle_id", w.cyCG, "now", now))
	add("set_couple_member: a person with no people row", "IntegrityError", base,
		step("set_couple_member", "person_id", w.missingPerson, "cycle_id", w.cyAB, "now", now))
	add("clear_couple_member: a couple row, then again, then nobody's", "", base,
		step("clear_couple_member", "person_id", w.chi), step("clear_couple_member", "person_id", w.chi),
		step("clear_couple_member", "person_id", w.an))
	constraint := func(cycle, owner, kind, text, at string) oracleCall {
		return step("set_pair_constraint", "cycle_id", cycle, "owner_id", owner, "kind", kind, "content", text, "now", at)
	}
	add("set_pair_constraint: a first line of hostile text", "", base, constraint(w.cyAB, w.an, "dung", hostile, now))
	add("set_pair_constraint: a new content", "", base, constraint(w.cyAB, w.an, "khong_an_duoc", "Tỏi (dữ liệu mẫu)", now))
	add("set_pair_constraint: the same content, then at the same instant", "", base,
		constraint(w.cyAB, w.binh, "dung", "Đừng hát (dữ liệu mẫu)", now), constraint(w.cyAB, w.binh, "dung", "Đừng hát (dữ liệu mẫu)", now))
	add("set_pair_constraint: under a Vietnam session TimeZone", "", vietnam,
		constraint(w.cyAB, w.binh, "dung", "Khác (dữ liệu mẫu)", vietnamNow))
	add("set_pair_constraint: a version at the INTEGER maximum", "DataError", base,
		tweak(fmt.Sprintf("UPDATE pair_shared_constraints SET version = %d WHERE owner_id = '%s'", 1<<31-1, w.binh),
			constraint(w.cyAB, w.binh, "dung", "Khác (dữ liệu mẫu)", now)))
	add("set_pair_constraint: two hundred and one characters", "IntegrityError", base,
		constraint(w.cyAB, w.an, "dung", strings.Repeat("ơ", 201), now))
	add("set_pair_constraint: an unknown kind", "IntegrityError", base, constraint(w.cyAB, w.an, "thich", "x", now))
	add("delete_pair_constraint: a line, again, and a line nobody wrote", "", base,
		step("delete_pair_constraint", "cycle_id", w.cyAB, "owner_id", w.binh, "kind", "dung"),
		step("delete_pair_constraint", "cycle_id", w.cyAB, "owner_id", w.binh, "kind", "dung"),
		step("delete_pair_constraint", "cycle_id", w.cyAB, "owner_id", w.an, "kind", "dung"))

	// --- papers --------------------------------------------------------------------------------
	paper := func(ctx string, cycle any, owner string, body map[string]any, lyDo any, author string) oracleCall {
		return step("create_pair_paper", "context_id", ctx, "cycle_id", cycle, "draft_owner_id", owner, "tuan", "2030-09-16",
			"expires_at", pairWeekEnd, "content", body, "ly_do", lyDo, "nguon", nguon, "author_type", author, "now", now)
	}
	add("create_pair_paper: a temporary sheet of hostile text with three-key provenance", "", base,
		paper(w.be, nil, w.binh, content("2030-09-21", stop("18:30", hostile, nil, true), dinner), nil, "human"))
	add("create_pair_paper: in a cycle that closed, with a reason", "", base,
		paper(w.bc, w.cyBC, w.chi, content("2030-09-21", dinner), "Lý do ' \" (dữ liệu mẫu)", "human"))
	add("create_pair_paper: under a Vietnam session TimeZone", "", vietnam,
		paper(w.bg, nil, w.giang, content("2030-09-21", dinner), nil, "human"))
	add("create_pair_paper: a second open sheet", "IntegrityError", base,
		paper(w.cd, w.cyCD, w.dung, content("2030-09-21", dinner), nil, "human"))
	add("create_pair_paper: a Nếp draft nobody sent", "IntegrityError", base,
		paper(w.be, nil, w.em, content("2030-09-21", dinner), nil, "nep"))
	add("create_pair_paper: a group context", "IntegrityError", base,
		paper(w.group, nil, w.an, content("2030-09-21", dinner), nil, "human"))
	add("create_pair_paper: content with a NUL character", "DataError", base,
		paper(w.be, nil, w.binh, content("2030-09-21", stop("18:30", "a\x00b", nil, true)), nil, "human"))
	add("get_pair_paper: two versions, a view, tied responses; an outing; tied kept lines; missing", "", base,
		step("get_pair_paper", "paper_id", w.pAB1), step("get_pair_paper", "paper_id", w.pAB2),
		step("get_pair_paper", "paper_id", w.pAB3), step("get_pair_paper", "paper_id", w.missingPaper))
	add("get_pair_paper: under a Vietnam session TimeZone", "", vietnam, step("get_pair_paper", "paper_id", w.pAB1))
	add("get_pair_paper: every stored shape dict() accepts", "", base, step("get_pair_paper", "paper_id", w.odd[0]))
	for i, want := range []string{"TypeError", "ValueError", "TypeError", "ValueError", "TypeError", "ValueError",
		"TypeError", "TypeError", "ValueError"} {
		add(fmt.Sprintf("get_pair_paper: a stored shape dict() refuses (%d)", i+2), want, base,
			step("get_pair_paper", "paper_id", w.odd[i+1]))
	}
	add("lock_pair_paper: a sheet and a missing one", "", base,
		step("lock_pair_paper", "paper_id", w.pAB1), step("lock_pair_paper", "paper_id", w.missingPaper))
	add("list_pair_papers: newest first, and a conversation without sheets", "", base,
		step("list_pair_papers", "context_id", w.ab), step("list_pair_papers", "context_id", w.be))
	add("list_pair_papers: a stored shape that refuses stops the list", "TypeError", base,
		step("list_pair_papers", "context_id", w.fg))

	// --- drafts and versions ---------------------------------------------------------------------
	draft := func(paperID string, body map[string]any, lyDo any) oracleCall {
		return step("update_pair_draft", "paper_id", paperID, "content", body, "ly_do", lyDo)
	}
	add("update_pair_draft: new content and a reason", "", base,
		draft(w.pCD1, content("2030-09-20", stop("20:00", hostile, "e0000098-aaaa-4aaa-8aaa-aaaaaaaaaaaa", false)), "Đổi ngày (dữ liệu mẫu)"))
	add("update_pair_draft: equal as Python dicts in another key order, 1.0 for true", "", base,
		draft(w.pCD1, map[string]any{"chang": []any{map[string]any{"can_kiem": json.Number("1.0"), "place_id": nil,
			"viec": "Ăn tối (dữ liệu mẫu)", "gio": "19:00"}}, "ngay": "2030-09-21"}, nil))
	add("update_pair_draft: only the reason", "", base, draft(w.pCD1, content("2030-09-21", dinner), "Chỉ lý do (dữ liệu mẫu)"))
	add("update_pair_draft: a reason taken away", "", base,
		tweak("UPDATE pair_paper_versions SET ly_do = 'cu' WHERE paper_id = '"+w.pCD1+"'", draft(w.pCD1, content("2030-09-21", dinner), nil)))
	add("update_pair_draft: a version already sent", "OperationalError", base, draft(w.pAB1, content("2030-09-19", dinner), nil))
	add("update_pair_draft: a sent version with nothing changed", "", base, draft(w.pAB1, content("2030-09-21", dinner), nil))
	add("update_pair_draft: a missing sheet", "", base, draft(w.missingPaper, content("2030-09-21", dinner), nil))
	version := func(paperID string, v int64, sentAt, sentBy any) oracleCall {
		return step("add_paper_version", "paper_id", paperID, "version", v, "content", content("2030-09-19", dinner),
			"ly_do", "Đổi (dữ liệu mẫu)", "nguon", map[string]any{"scope": "chung", "dung": []any{"nguoi"}, "luc": now},
			"author_type", "human", "sent_at", sentAt, "sent_by", sentBy, "now", now)
	}
	add("add_paper_version: the next version, sent", "", base, version(w.pAB1, 3, now, w.an))
	add("add_paper_version: a version number taken", "IntegrityError", base, version(w.pAB1, 2, now, w.an))
	add("add_paper_version: a human version sent by nobody", "IntegrityError", base, version(w.pAB1, 3, now, nil))
	add("add_paper_version: version zero", "IntegrityError", base, version(w.pAB1, 0, now, w.an))
	add("add_paper_version: a version past INTEGER", "DataError", base, version(w.pAB1, int64(1)<<31, now, w.an))
	add("mark_version_sent: a draft", "", base, step("mark_version_sent", "paper_id", w.pCD1, "version", 1, "sent_by", w.chi, "now", now))
	add("mark_version_sent: a version already sent and a missing one", "", base,
		step("mark_version_sent", "paper_id", w.pAB1, "version", 1, "sent_by", w.binh, "now", now),
		step("mark_version_sent", "paper_id", w.pCD1, "version", 9, "sent_by", w.chi, "now", now))
	add("mark_version_sent: a human draft sent by nobody", "IntegrityError", base,
		step("mark_version_sent", "paper_id", w.pCD1, "version", 1, "sent_by", nil, "now", now))

	// --- state, views, responses, outings, kept lines ----------------------------------------------
	state := func(paperID, to, at string, extra ...any) oracleCall {
		return step("set_paper_state", append([]any{"paper_id", paperID, "state", to, "now", at}, extra...)...)
	}
	add("set_paper_state: a state, then the same state", "", base, state(w.pAB1, "huy", now), state(w.pAB1, "huy", later(1)))
	add("set_paper_state: a new state and a new current version", "", base, state(w.pAB1, "da_xem", now, "current_version", 3))
	add("set_paper_state: the same current version", "", base, state(w.pAB1, "da_xem", now, "current_version", 2))
	add("set_paper_state: da_di with a recorder, then again at the same instant", "", base,
		state(w.pAB2, "da_di", now, "recorded_by_id", w.binh), state(w.pAB2, "da_di", now, "recorded_by_id", w.binh))
	add("set_paper_state: da_di recorded by nobody", "IntegrityError", base, state(w.pAB2, "da_di", now))
	add("set_paper_state: da_di recorded again by somebody else", "", base, state(w.pFA1, "da_di", now, "recorded_by_id", w.phuong))
	add("set_paper_state: a recorder given for another state", "", base, state(w.pAB1, "da_xem", now, "recorded_by_id", w.an))
	add("set_paper_state: an unknown state", "IntegrityError", base, state(w.pAB1, "xong", now))
	add("set_paper_state: a second open sheet", "IntegrityError", base, state(w.pAB2, "nhap", now))
	add("set_paper_state: a missing sheet", "", base, state(w.missingPaper, "huy", now))
	add("set_paper_state: a version past INTEGER", "DataError", base, state(w.pAB1, "da_gui", now, "current_version", int64(1)<<31))
	view := func(paperID string, v int64, person, at string) oracleCall {
		return step("mark_paper_viewed", "paper_id", paperID, "version", v, "person_id", person, "now", at)
	}
	add("mark_paper_viewed: a first look, then again", "", base, view(w.pAB1, 2, w.an, now), view(w.pAB1, 2, w.an, later(1)))
	add("mark_paper_viewed: a stored look, under a Vietnam session TimeZone", "", vietnam, view(w.pAB1, 1, w.binh, vietnamNow))
	add("mark_paper_viewed: a version that does not exist", "IntegrityError", base, view(w.pAB1, 9, w.an, now))
	respond := func(paperID string, v int64, person, kind, at string) oracleCall {
		return step("add_paper_response", "paper_id", paperID, "version", v, "person_id", person, "kind", kind, "now", at)
	}
	add("add_paper_response: an agreement, then the same agreement", "paper_already_agreed", base,
		respond(w.pAB1, 2, w.an, "dong_y", now), respond(w.pAB1, 2, w.an, "dong_y", later(1)))
	add("add_paper_response: two counter-proposals", "", base,
		respond(w.pAB1, 2, w.an, "de_nghi_sua", now), respond(w.pAB1, 2, w.an, "de_nghi_sua", later(1)))
	add("add_paper_response: somebody outside the conversation", "OperationalError", base, respond(w.pAB1, 2, w.la, "de_nghi_sua", now))
	add("add_paper_response: somebody who left", "OperationalError", base, respond(w.pFA1, 1, w.phuong, "dong_y", now))
	add("add_paper_response: an unknown kind", "IntegrityError", base, respond(w.pAB1, 2, w.an, "ok", now))
	add("add_paper_response: an agreement already stored", "paper_already_agreed", base, respond(w.pAB1, 2, w.binh, "dong_y", now))
	link := func(paperID string, v int64, outing string) oracleCall {
		return step("link_paper_outing", "paper_id", paperID, "version", v, "outing_id", outing, "now", now)
	}
	add("link_paper_outing: a sheet without one, then read", "", base,
		link(w.pDE1, 1, w.oFree), step("get_paper_outing", "paper_id", w.pDE1))
	add("link_paper_outing: a sheet already linked", "paper_outing_exists", base, link(w.pAB2, 1, w.oFree))
	add("link_paper_outing: an outing another sheet has", "IntegrityError", base, link(w.pDE1, 1, w.oAB))
	add("link_paper_outing: a version that does not exist", "IntegrityError", base, link(w.pDE1, 7, w.oFree))
	add("get_paper_outing: linked and not", "", base,
		step("get_paper_outing", "paper_id", w.pAB2), step("get_paper_outing", "paper_id", w.pAB1))
	outing := func(title string, headcount int) oracleCall {
		return step("create_outing", "context_id", w.ab, "created_by_id", w.an, "title", title, "starts_on", "2030-09-21",
			"ends_on", "2030-09-21", "headcount", headcount, "budget_per_person_vnd", 0, "now", now)
	}
	add("create_outing: the outing a sheet becomes", "", base, outing("Tờ lời rủ 21/09", 2))
	add("create_outing: a blank title", "IntegrityError", base, outing("", 2))
	add("create_outing: nobody going", "IntegrityError", base, outing("Tờ lời rủ 21/09", 0))
	add("flow.create_outing_link: an outing, its link and the link read back", "", base,
		step("flow.create_outing_link", "context_id", w.de, "created_by_id", w.em, "title", "Tờ lời rủ 21/09",
			"starts_on", "2030-09-21", "ends_on", "2030-09-21", "headcount", 2, "budget_per_person_vnd", 0,
			"paper_id", w.pDE1, "version", 1, "now", now))
	keep := func(paperID, person, line string) oracleCall {
		return step("add_paper_keep", "paper_id", paperID, "person_id", person, "line", line, "now", now)
	}
	add("add_paper_keep: hostile text", "", base, keep(w.pFA1, w.an, hostile))
	add("add_paper_keep: a line of only whitespace", "IntegrityError", base, keep(w.pFA1, w.an, " \n\t "))
	add("add_paper_keep: a missing sheet", "IntegrityError", base, keep(w.missingPaper, w.an, "x"))
	add("close_open_pair_papers: a sent sheet, then again", "", base,
		step("close_open_pair_papers", "context_id", w.ab, "now", now), step("close_open_pair_papers", "context_id", w.ab, "now", later(1)))
	add("close_open_pair_papers: a draft, a lapsed sheet, none", "", base,
		step("close_open_pair_papers", "context_id", w.cd, "now", now), step("close_open_pair_papers", "context_id", w.eg, "now", now),
		step("close_open_pair_papers", "context_id", w.be, "now", now))

	// --- routes ---------------------------------------------------------------------------------------
	route := func(name, actor string, pairs ...any) oracleCall {
		return step("route."+name, append([]any{"actor_id", actor, "now", now}, pairs...)...)
	}
	onCtx := func(name, actor, ctx string, pairs ...any) oracleCall {
		return route(name, actor, append([]any{"context_id", ctx}, pairs...)...)
	}
	onPaper := func(name, actor, paperID string, pairs ...any) oracleCall {
		return route(name, actor, append([]any{"paper_id", paperID}, pairs...)...)
	}
	body := func(pairs ...any) map[string]any { return args(pairs...) }
	type routeCase struct {
		name, want string
		call       oracleCall
	}
	routes := []routeCase{
		{"GET notebook: an active notebook", "", onCtx("pair_notebook", w.an, w.ab)},
		{"GET notebook: somebody else's draft in play", "", onCtx("pair_notebook", w.dung, w.cd)},
		{"GET notebook: none yet", "", onCtx("pair_notebook", w.em, w.be)},
		{"GET notebook: the other person left", "", onCtx("pair_notebook", w.an, w.fa)},
		{"GET notebook: a stranger", "404:notebook_not_found", onCtx("pair_notebook", w.la, w.ab)},
		{"GET notebook: a group", "404:notebook_not_found", onCtx("pair_notebook", w.an, w.group)},
		{"GET notebook: a missing conversation", "404:notebook_not_found", onCtx("pair_notebook", w.an, w.missingContext)},
		{"POST proposals: lap_so where the cycle closed", "", onCtx("propose_pair_consent", w.binh, w.bc, "body", body("purpose", "lap_so"))},
		{"POST proposals: lap_so with no notebook", "", onCtx("propose_pair_consent", w.em, w.be, "body", body("purpose", "lap_so"))},
		{"POST proposals: lap_so again in a pending notebook", "", onCtx("propose_pair_consent", w.em, w.ae, "body", body("purpose", "lap_so"))},
		{"POST proposals: doc_chat in an active notebook", "", onCtx("propose_pair_consent", w.binh, w.ab, "body", body("purpose", "doc_chat"))},
		{"POST proposals: bat_doi in a pending notebook", "409:consent_missing", onCtx("propose_pair_consent", w.an, w.ae, "body", body("purpose", "bat_doi"))},
		{"POST proposals: the other person only invited", "409:cycle_not_active", onCtx("propose_pair_consent", w.an, w.ag, "body", body("purpose", "lap_so"))},
		{"POST proposals: a stranger", "404:notebook_not_found", onCtx("propose_pair_consent", w.la, w.ab, "body", body("purpose", "lap_so"))},
		{"POST grant: the second lap_so opens the notebook", "", onCtx("grant_pair_consent", w.em, w.ae, "proposal_id", w.prLapSoAE)},
		{"POST grant: the second bat_doi makes a couple", "", onCtx("grant_pair_consent", w.binh, w.ab, "proposal_id", w.prBatDoiAB)},
		{"POST grant: bat_doi with one of the two a couple elsewhere", "409:couple_slot_taken", onCtx("grant_pair_consent", w.chi, w.cg, "proposal_id", w.prBatDoiCG)},
		{"POST grant: one's own offer", "403:permission_denied", onCtx("grant_pair_consent", w.an, w.ae, "proposal_id", w.prLapSoAE)},
		{"POST grant: an offer of another notebook", "404:consent_proposal_not_found", onCtx("grant_pair_consent", w.binh, w.ab, "proposal_id", w.prLapSoAE)},
		{"POST grant: an offer that lapsed", "409:consent_proposal_expired", onCtx("grant_pair_consent", w.binh, w.ab, "proposal_id", w.prOldAB)},
		{"POST grant: an offer already complete", "409:consent_proposal_expired", onCtx("grant_pair_consent", w.an, w.ab, "proposal_id", w.prDocChatAB)},
		{"POST grant: no notebook yet", "404:consent_proposal_not_found", onCtx("grant_pair_consent", w.em, w.be, "proposal_id", w.prLapSoAE)},
		{"DELETE consents: bat_doi ends a couple for both", "", onCtx("revoke_pair_consent", w.chi, w.cd, "purpose", "bat_doi")},
		{"DELETE consents: bat_doi with nobody a couple", "", onCtx("revoke_pair_consent", w.an, w.ab, "purpose", "bat_doi")},
		{"DELETE consents: doc_chat drops a Nếp draft", "", onCtx("revoke_pair_consent", w.an, w.ae, "purpose", "doc_chat")},
		{"DELETE consents: doc_chat with sheets but no Nếp draft", "", onCtx("revoke_pair_consent", w.an, w.ab, "purpose", "doc_chat")},
		{"DELETE consents: no notebook yet", "", onCtx("revoke_pair_consent", w.em, w.be, "purpose", "lap_so")},
		{"DELETE consents: an unknown purpose", "404:consent_purpose_unknown", onCtx("revoke_pair_consent", w.an, w.ab, "purpose", "Lap_So")},
		{"DELETE consents: a stranger", "404:notebook_not_found", onCtx("revoke_pair_consent", w.la, w.ab, "purpose", "doc_chat")},
		{"PUT constraints: a first line of hostile text", "", onCtx("put_pair_constraint", w.an, w.ab, "kind", "dung", "body", body("content", "  "+hostile+"\x1f "))},
		{"PUT constraints: a line rewritten", "", onCtx("put_pair_constraint", w.binh, w.ab, "kind", "dung", "body", body("content", "Đừng đi muộn (dữ liệu mẫu)"))},
		{"PUT constraints: no cycle", "409:cycle_not_active", onCtx("put_pair_constraint", w.binh, w.bg, "kind", "dung", "body", body("content", "x"))},
		{"PUT constraints: an unknown kind", "404:constraint_kind_unknown", onCtx("put_pair_constraint", w.an, w.ab, "kind", "thich", "body", body("content", "x"))},
		{"DELETE constraints: a line", "", onCtx("delete_pair_constraint", w.binh, w.ab, "kind", "dung")},
		{"DELETE constraints: a line nobody wrote", "", onCtx("delete_pair_constraint", w.an, w.ab, "kind", "dung")},
		{"DELETE constraints: no notebook yet", "", onCtx("delete_pair_constraint", w.em, w.be, "kind", "dung")},
		{"DELETE constraints: an unknown kind", "404:constraint_kind_unknown", onCtx("delete_pair_constraint", w.an, w.ab, "kind", "Dung")},
		{"POST close/preview: an active notebook", "", onCtx("preview_close_pair_notebook", w.an, w.ab)},
		{"POST close/preview: a stranger", "404:notebook_not_found", onCtx("preview_close_pair_notebook", w.la, w.ab)},
		{"POST close: the current revision", "", onCtx("close_pair_notebook", w.an, w.ab, "revision", "@current")},
		{"POST close: a couple with a draft", "", onCtx("close_pair_notebook", w.chi, w.cd, "revision", "@current")},
		{"POST close: a pending notebook", "", onCtx("close_pair_notebook", w.em, w.ae, "revision", "@current")},
		{"POST close: no notebook yet", "", onCtx("close_pair_notebook", w.em, w.be, "revision", "@current")},
		{"POST close: a stale revision", "409:notebook_revision_stale", onCtx("close_pair_notebook", w.an, w.ab, "revision", "cu-roi")},
		{"GET papers: a notebook with history", "", onCtx("list_pair_papers", w.an, w.ab)},
		{"GET papers: the other person's draft", "", onCtx("list_pair_papers", w.dung, w.cd)},
		{"GET papers: a stranger", "404:notebook_not_found", onCtx("list_pair_papers", w.la, w.ab)},
		{"POST papers/draft: a notebook whose cycle closed", "", onCtx("draft_pair_paper", w.binh, w.bc)},
		{"POST papers/draft: no notebook yet", "", onCtx("draft_pair_paper", w.em, w.be)},
		{"POST papers/draft: a constraint on file", "", onCtx("draft_pair_paper", w.an, w.fa)},
		{"POST papers/draft: a sheet already open", "409:paper_wrong_state", onCtx("draft_pair_paper", w.an, w.ab)},
		{"POST papers/draft: a pending notebook", "409:cycle_not_active", onCtx("draft_pair_paper", w.em, w.ae)},
		{"POST papers/draft: beside a sheet whose week is over", "IntegrityError", onCtx("draft_pair_paper", w.em, w.eg)},
		{"POST papers/draft: a stranger", "404:notebook_not_found", onCtx("draft_pair_paper", w.la, w.ab)},
		{"GET paper: a sheet sent back", "", onPaper("pair_paper", w.binh, w.pAB1)},
		{"GET paper: a plan with its outing", "", onPaper("pair_paper", w.an, w.pAB2)},
		{"GET paper: somebody else's draft", "404:paper_not_found", onPaper("pair_paper", w.dung, w.pCD1)},
		{"GET paper: a stranger", "404:notebook_not_found", onPaper("pair_paper", w.la, w.pAB1)},
		{"GET paper: missing", "404:paper_not_found", onPaper("pair_paper", w.an, w.missingPaper)},
		{"GET paper: content that cannot be read", "409:paper_wrong_state", onPaper("pair_paper", w.phuong, w.odd[0])},
		{"PATCH draft: new content", "", onPaper("edit_pair_draft", w.chi, w.pCD1, "body", body("content",
			content("2030-09-20", stop("20:00", hostile, "e0000098-aaaa-4aaa-8aaa-aaaaaaaaaaaa", false)), "ly_do", "  Đổi giờ  "))},
		{"PATCH draft: the content already stored", "", onPaper("edit_pair_draft", w.chi, w.pCD1, "body", body("content", content("2030-09-21", dinner)))},
		{"PATCH draft: somebody else's draft", "404:paper_not_found", onPaper("edit_pair_draft", w.dung, w.pCD1, "body", body("content", content("2030-09-21", dinner)))},
		{"PATCH draft: a sheet already sent", "409:paper_wrong_state", onPaper("edit_pair_draft", w.an, w.pAB1, "body", body("content", content("2030-09-21", dinner)))},
		{"POST send: a draft", "", onPaper("send_pair_paper", w.chi, w.pCD1, "body", body("version", 1))},
		{"POST send: a stale version", "409:paper_version_stale", onPaper("send_pair_paper", w.chi, w.pCD1, "body", body("version", 2))},
		{"POST send: a sheet already sent", "409:paper_wrong_state", onPaper("send_pair_paper", w.an, w.pAB1, "body", body("version", 2))},
		{"POST viewed: the current version by its recipient", "", onPaper("mark_pair_paper_viewed", w.an, w.pAB1, "version", 2)},
		{"POST viewed: an earlier version already seen", "", onPaper("mark_pair_paper_viewed", w.binh, w.pAB1, "version", 1)},
		{"POST viewed: a sheet already opened", "", onPaper("mark_pair_paper_viewed", w.em, w.pCE1, "version", 1)},
		{"POST viewed: by its sender", "409:paper_self_response", onPaper("mark_pair_paper_viewed", w.binh, w.pAB1, "version", 2)},
		{"POST viewed: a version that does not exist", "404:paper_not_found", onPaper("mark_pair_paper_viewed", w.an, w.pAB1, "version", 9)},
		{"POST viewed: a sheet whose week is over", "409:paper_expired", onPaper("mark_pair_paper_viewed", w.em, w.pEG1, "version", 1)},
		{"POST responses: the second yes makes an outing", "", onPaper("respond_pair_paper", w.an, w.pAB1, "version", 2, "body", body("kind", "dong_y"))},
		{"POST responses: a counter-proposal", "", onPaper("respond_pair_paper", w.an, w.pAB1, "version", 2, "body", body("kind", "de_nghi_sua",
			"content", content("2030-09-19", dinner, stop("21:30", "Kem (dữ liệu mẫu)", nil, true)), "ly_do", "Thứ Sáu nhé"))},
		{"POST responses: a Nếp sheet reaches chot", "", onPaper("respond_pair_paper", w.em, w.pDE1, "version", 1, "body", body("kind", "dong_y"))},
		{"POST responses: a yes already given", "", onPaper("respond_pair_paper", w.dung, w.pDE1, "version", 1, "body", body("kind", "dong_y"))},
		{"POST responses: yes after opening", "", onPaper("respond_pair_paper", w.em, w.pCE1, "version", 1, "body", body("kind", "dong_y"))},
		{"POST responses: by the sender", "409:paper_self_response", onPaper("respond_pair_paper", w.binh, w.pAB1, "version", 2, "body", body("kind", "dong_y"))},
		{"POST responses: a stale version", "409:paper_version_stale", onPaper("respond_pair_paper", w.an, w.pAB1, "version", 1, "body", body("kind", "dong_y"))},
		{"POST responses: a sheet whose week is over", "409:paper_expired", onPaper("respond_pair_paper", w.em, w.pEG1, "version", 1, "body", body("kind", "dong_y"))},
		{"POST responses: a counter-proposal to a plan", "409:paper_frozen", onPaper("respond_pair_paper", w.an, w.pAB2, "version", 1, "body", body("kind", "de_nghi_sua",
			"content", content("2030-09-19", dinner)))},
		{"POST withdraw: the sender before any look", "", onPaper("withdraw_pair_paper", w.binh, w.pAB1, "body", body("version", 2))},
		{"POST withdraw: not the sender", "409:paper_not_withdrawable", onPaper("withdraw_pair_paper", w.an, w.pAB1, "body", body("version", 2))},
		{"POST withdraw: a stale version", "409:paper_version_stale", onPaper("withdraw_pair_paper", w.binh, w.pAB1, "body", body("version", 1))},
		{"POST withdraw: after the other opened it", "409:paper_not_withdrawable", onPaper("withdraw_pair_paper", w.chi, w.pCE1, "body", body("version", 1))},
		{"POST skip: a draft", "", onPaper("skip_pair_week", w.chi, w.pCD1)},
		{"POST skip: a sent sheet", "", onPaper("skip_pair_week", w.an, w.pAB1)},
		{"POST skip: a plan", "409:paper_wrong_state", onPaper("skip_pair_week", w.an, w.pAB2)},
		{"POST skip: a sheet whose week is over", "409:paper_expired", onPaper("skip_pair_week", w.em, w.pEG1)},
		{"POST done: a plan whose day has come", "", onPaper("record_pair_outing_done", w.binh, w.pAB2)},
		{"POST done: a sheet not agreed", "409:paper_wrong_state", onPaper("record_pair_outing_done", w.an, w.pAB1)},
		{"POST keeps: the first line", "", onPaper("keep_pair_paper_line", w.an, w.pFA1, "body", body("line", "  Vui (dữ liệu mẫu)  "))},
		{"POST keeps: another line", "", onPaper("keep_pair_paper_line", w.binh, w.pAB3, "body", body("line", hostile))},
		{"POST keeps: before the outing", "409:paper_wrong_state", onPaper("keep_pair_paper_line", w.an, w.pAB1, "body", body("line", "x"))},
	}
	for _, r := range routes {
		add("route "+r.name, r.want, base, r.call)
	}
	// Routes that need a tweak or another clock.
	add("route POST responses: one yes of two on a Nếp sheet", "", base,
		tweak("DELETE FROM pair_paper_responses WHERE id = '"+fid(kindResponse, 0x21)+"'",
			onPaper("respond_pair_paper", w.em, w.pDE1, "version", 1, "body", body("kind", "dong_y"))))
	add("route POST responses: yes to a plan whose other yes is gone", "409:paper_wrong_state", base,
		tweak("DELETE FROM pair_paper_responses WHERE id = '"+fid(kindResponse, 0x15)+"'",
			onPaper("respond_pair_paper", w.an, w.pAB2, "version", 1, "body", body("kind", "dong_y"))))
	add("route POST send: a draft whose week ended at this instant", "409:paper_expired", base,
		tweak("UPDATE pair_papers SET expires_at = '"+now+"' WHERE id = '"+w.pCD1+"'",
			onPaper("send_pair_paper", w.chi, w.pCD1, "body", body("version", 1))))
	add("route POST done: a plan whose day is ahead", "409:paper_wrong_state", base,
		tweak("UPDATE pair_papers SET state = 'chot' WHERE id = '"+w.pCE1+"'", onPaper("record_pair_outing_done", w.chi, w.pCE1)))
	sunday := step("route.draft_pair_paper", "actor_id", w.binh, "now", "2030-09-22T16:59:59.999999Z", "context_id", w.bc)
	add("route POST papers/draft: the last microsecond of a Sunday", "", base, sunday)
	add("route POST papers/draft: under a Vietnam session TimeZone at another offset", "", vietnam,
		step("route.draft_pair_paper", "actor_id", w.em, "now", vietnamNow, "context_id", w.be))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// pairMethods is every repository method and route this port covers; the
// corpus must reach each with at least one normal return in Python.
var pairMethods = []string{
	"create_pair_notebook", "lock_pair_notebook", "open_pair_cycle", "activate_pair_cycle", "close_pair_cycle",
	"create_consent_proposal", "get_consent_proposal", "grant_consent", "complete_consent_proposal", "revoke_consents",
	"set_couple_member", "clear_couple_member", "set_pair_constraint", "delete_pair_constraint", "create_pair_paper",
	"get_pair_paper", "lock_pair_paper", "list_pair_papers", "update_pair_draft", "add_paper_version",
	"mark_version_sent", "set_paper_state", "mark_paper_viewed", "add_paper_response", "link_paper_outing",
	"get_paper_outing", "add_paper_keep", "close_open_pair_papers", "create_outing",
	"route.pair_notebook", "route.propose_pair_consent", "route.grant_pair_consent", "route.revoke_pair_consent",
	"route.put_pair_constraint", "route.delete_pair_constraint", "route.preview_close_pair_notebook",
	"route.close_pair_notebook", "route.list_pair_papers", "route.draft_pair_paper", "route.pair_paper",
	"route.edit_pair_draft", "route.send_pair_paper", "route.mark_pair_paper_viewed", "route.respond_pair_paper",
	"route.withdraw_pair_paper", "route.skip_pair_week", "route.record_pair_outing_done", "route.keep_pair_paper_line",
}

// pairBranches are the write branches and locks the corpus must reach in
// Python, each named by the call and the start of the normalised statement
// only that branch issues.
var pairBranches = []struct{ call, prefix string }{
	{"lock_pair_notebook", "SELECT pair_notebooks.id, pair_notebooks.context_id, pair_notebooks.context_kind, pair_notebooks.created_at FROM pair_notebooks WHERE pair_notebooks.context_id = ?::UUID FOR UPDATE"},
	{"create_pair_notebook", "INSERT INTO pair_notebooks"},
	{"open_pair_cycle", "INSERT INTO pair_notebook_cycles"},
	{"open_pair_cycle", "INSERT INTO pair_cycle_participants"},
	{"activate_pair_cycle", "UPDATE pair_notebook_cycles SET state=?::VARCHAR, opened_at="},
	{"activate_pair_cycle", "UPDATE pair_notebook_cycles SET state=?::VARCHAR WHERE"},
	{"activate_pair_cycle", "UPDATE pair_notebook_cycles SET opened_at="},
	{"close_pair_cycle", "UPDATE pair_notebook_cycles SET state=?::VARCHAR, closed_at="},
	{"grant_consent", "INSERT INTO pair_consents"},
	{"grant_consent", "UPDATE pair_consents SET granted_at="},
	{"complete_consent_proposal", "UPDATE pair_consent_proposals SET completed_at="},
	{"revoke_consents", "UPDATE pair_consents SET revoked_at="},
	{"set_couple_member", "INSERT INTO active_couple_members"},
	{"clear_couple_member", "DELETE FROM active_couple_members"},
	{"set_pair_constraint", "INSERT INTO pair_shared_constraints"},
	{"set_pair_constraint", "UPDATE pair_shared_constraints SET content="},
	{"set_pair_constraint", "UPDATE pair_shared_constraints SET version="},
	{"delete_pair_constraint", "DELETE FROM pair_shared_constraints"},
	{"create_pair_paper", "INSERT INTO pair_paper_versions"},
	{"lock_pair_paper", "SELECT pair_papers.id AS pair_papers_id"},
	{"update_pair_draft", "UPDATE pair_paper_versions SET content="},
	{"update_pair_draft", "UPDATE pair_paper_versions SET ly_do="},
	{"mark_version_sent", "UPDATE pair_paper_versions SET sent_at="},
	{"set_paper_state", "UPDATE pair_papers SET state=?::VARCHAR, current_version="},
	{"set_paper_state", "UPDATE pair_papers SET state=?::VARCHAR, done_recorded_by_id="},
	{"set_paper_state", "UPDATE pair_papers SET done_recorded_by_id="},
	{"mark_paper_viewed", "INSERT INTO pair_paper_views"},
	{"add_paper_response", "INSERT INTO pair_paper_responses"},
	{"link_paper_outing", "INSERT INTO pair_paper_outings"},
	{"add_paper_keep", "INSERT INTO pair_paper_keeps"},
	{"close_open_pair_papers", "UPDATE pair_papers SET state="},
	{"create_outing", "INSERT INTO outings"},
	{"route.respond_pair_paper", "INSERT INTO outings"},
	{"route.respond_pair_paper", "INSERT INTO pair_paper_versions"},
	{"route.grant_pair_consent", "INSERT INTO active_couple_members"},
	{"route.grant_pair_consent", "UPDATE pair_notebook_cycles SET state="},
	{"route.revoke_pair_consent", "DELETE FROM active_couple_members"},
	{"route.revoke_pair_consent", "UPDATE pair_papers SET state="},
	{"route.close_pair_notebook", "UPDATE pair_notebook_cycles SET state="},
	{"route.propose_pair_consent", "INSERT INTO pair_notebooks"},
	{"route.draft_pair_paper", "INSERT INTO pair_papers"},
	{"route.record_pair_outing_done", "UPDATE pair_papers SET state=?::VARCHAR, done_recorded_by_id="},
}

func TestPairRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of pair_repo_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_pair_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "pair_repo_oracle_")

	cases, built := pairRepoOracleCases()
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
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_pair_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_pair_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
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
	reached := make([]int, len(pairBranches))
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
					for b, branch := range pairBranches {
						if call == branch.call && strings.HasPrefix(statement, branch.prefix) {
							reached[b]++
						}
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
			t.Errorf("case %q: python ended in %q, the case is written for %q (%v)", c.Name, end, cases[i].wantEnd,
				pyCase.Steps[len(pyCase.Steps)-1].Error)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python refused at step %d of %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}

		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, normalizePairSteps(pySteps, c), normalizePairSteps(runPairRepoGoCase(t, pool, c), c))
		})
	}
	for _, method := range pairMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	for b, branch := range pairBranches {
		if reached[b] == 0 {
			t.Errorf("no case makes python issue, from %s: %s...", branch.call, branch.prefix)
		}
	}
	t.Logf("pair repo oracle: %d cases, %d steps (%d route steps; %d results, %d refusals), %d statements, "+
		"%d probe rows of which %d table rows, %d generated ids bound, %d branches reached, %d mismatches",
		tally.cases, tally.steps, routeSteps, tally.results, tally.errors, tally.statements, tally.probeRows,
		tally.tableRows, tally.generated, len(pairBranches), tally.mismatches)
}

// ---------------------------------------------------------------------------
// Two requests at once (Go only)
// ---------------------------------------------------------------------------

func TestPairRowLocksSerialiseRequests(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of pair_repo_oracle_postgres_test.go")
	}
	pool, _ := migratedOracleSchema(t, image, "pair_locks_")
	w := newPairRepoWorld()
	for _, sql := range w.sql {
		if _, err := pool.Exec(bg, sql); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	now := argInstant(pairNow)
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

	t.Run("a locked notebook makes a second lock wait, and a missing one locks nothing", func(t *testing.T) {
		first := begin(t, "")
		if n, err := (Repository{Q: first}).LockPairNotebook(bg, w.ab); err != nil || n == nil {
			t.Fatalf("first lock: %v %v", n, err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).LockPairNotebook(bg, w.ab); !lockNotAvailable(err) {
			t.Fatalf("a second lock did not wait on the first: %v", err)
		}
		if _, err := begin(t, "150ms").Exec(bg, `UPDATE pair_notebook_cycles SET terms_version = 2 WHERE id = $1::UUID`, w.cyAB); err != nil {
			t.Fatalf("the notebook lock must not reach its cycle: %v", err)
		}
	})

	t.Run("a locked paper makes a second lock and a close wait", func(t *testing.T) {
		first := begin(t, "")
		if p, err := (Repository{Q: first}).LockPairPaper(bg, w.pAB1); err != nil || p == nil {
			t.Fatalf("first lock: %v %v", p, err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).LockPairPaper(bg, w.pAB1); !lockNotAvailable(err) {
			t.Fatalf("a second paper lock did not wait: %v", err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).CloseOpenPairPapers(bg, w.ab, now); !lockNotAvailable(err) {
			t.Fatalf("closing the notebook did not wait on the paper: %v", err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).GetPairPaper(bg, w.pAB1); err != nil {
			t.Fatalf("a plain read must not wait: %v", err)
		}
	})

	t.Run("revoking holds the consent rows, not the proposals", func(t *testing.T) {
		first := begin(t, "")
		if n, err := (Repository{Q: first}).RevokeConsents(bg, w.cyAB, "doc_chat", w.an, now); err != nil || n != 2 {
			t.Fatalf("revoke: %d %v", n, err)
		}
		if _, err := (Repository{Q: begin(t, "150ms")}).RevokeConsents(bg, w.cyAB, "doc_chat", w.an, now); !lockNotAvailable(err) {
			t.Fatalf("a second revocation of the same grants did not wait: %v", err)
		}
		if err := (Repository{Q: begin(t, "150ms")}).GrantConsent(bg, w.prDocChatAB, w.an, now); err != nil {
			t.Fatalf("reading a grant being revoked must not wait: %v", err)
		}
		if err := (Repository{Q: begin(t, "150ms")}).CompleteConsentProposal(bg, w.prBatDoiAB, now); err != nil {
			t.Fatalf("the proposals must stay unlocked: %v", err)
		}
	})

	t.Run("two first agreements to one version end as one row", func(t *testing.T) {
		first := begin(t, "")
		repo := Repository{Q: first}
		if _, err := repo.LockPairPaper(bg, w.pCE1); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddPaperResponse(bg, PaperResponseInput{PaperID: w.pCE1, Version: 1, PersonID: w.em, Kind: "dong_y", Now: now}); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		second := begin(t, "")
		go func() {
			repo := Repository{Q: second}
			if _, err := repo.LockPairPaper(bg, w.pCE1); err != nil {
				done <- err
				return
			}
			done <- repo.AddPaperResponse(bg, PaperResponseInput{PaperID: w.pCE1, Version: 1, PersonID: w.em, Kind: "dong_y",
				Now: now.Add(time.Second)})
		}()
		select {
		case err := <-done:
			t.Fatalf("the second agreement did not wait for the first: %v", err)
		case <-time.After(300 * time.Millisecond):
		}
		if err := first.Commit(bg); err != nil {
			t.Fatal(err)
		}
		var conflict *Conflict
		if err := <-done; !errors.As(err, &conflict) || conflict.Code != "paper_already_agreed" {
			t.Fatalf("the second agreement, after the first committed: %v", err)
		}
		var rows int
		if err := pool.QueryRow(bg, `SELECT count(*) FROM pair_paper_responses WHERE paper_id = $1::UUID AND person_id = $2::UUID
			AND kind = 'dong_y'`, w.pCE1, w.em).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 1 {
			t.Fatalf("%d agreements from one person to one version", rows)
		}
	})
}
