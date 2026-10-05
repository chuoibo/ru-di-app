//go:build postgres

package dieuchinh

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
	plan "mobile/services/core/internal/domain/dieuchinh"
	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

// world is one group: An paid two dinners and published a batch. Bình and
// Chi owe for the first, Dũng for the second; all three hold guest links.
// Texts are sample data (dữ liệu mẫu).
type world struct {
	pool                *pgxpool.Pool
	an, binh, chi, dung string
	group, batch        string
	firstExpense        string
	tokens              map[string]string
	now                 time.Time
}

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	pool := testdb.Pool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	w := &world{pool: pool, tokens: map[string]string{}, now: time.Now().UTC().Truncate(time.Microsecond)}
	person := func(name string) string {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),$1) RETURNING id::text`, name+" (dữ liệu mẫu)").Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	w.an, w.binh, w.chi, w.dung = person("An"), person("Bình"), person("Chi"), person("Dũng")
	r := repo.Repository{Q: pool}
	group, err := r.CreateContext(ctx, "Nhóm điều chỉnh (dữ liệu mẫu)", w.an)
	if err != nil {
		t.Fatal(err)
	}
	w.group = group.ID
	for _, p := range []string{w.an, w.binh, w.chi, w.dung} {
		if _, err := pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin,created_at) VALUES(gen_random_uuid(),$1,$2,'active','member','named',now())`, w.group, p); err != nil {
			t.Fatal(err)
		}
	}
	confirm := func(shares map[string]int64) (string, plan.Source, []repo.AllocationRow) {
		expense, err := r.CreateExpense(ctx, w.group, nil)
		if err != nil {
			t.Fatal(err)
		}
		var total int64
		var allocations []repo.ParticipantAmount
		for _, p := range []string{w.an, w.binh, w.chi, w.dung} {
			if amount, ok := shares[p]; ok {
				allocations = append(allocations, repo.ParticipantAmount{ParticipantID: p, AmountVND: amount})
				total += amount
			}
		}
		record, err := r.SaveExpenseConfirmation(ctx, repo.ExpenseConfirmation{ExpenseID: expense.ID,
			Proposal: repo.ExpenseProposal{RecordedByID: w.an, PaidByID: w.an, VerificationScope: "totals_only", OccurredAt: w.now},
			Rollups:  repo.ExpenseRollups{SubtotalVND: total, TotalVND: total}, Allocations: allocations,
			ConfirmedByID: w.an, PayerAcknowledgement: "acknowledged", Now: w.now})
		if err != nil {
			t.Fatal(err)
		}
		rows, err := pool.Query(ctx, `SELECT id::text, participant_id::text, amount_vnd FROM confirmed_allocations WHERE expense_version_id=$1::uuid`, record.ExpenseVersionID)
		if err != nil {
			t.Fatal(err)
		}
		var stored []repo.AllocationRow
		source := plan.Source{ExpenseVersionID: record.ExpenseVersionID, PaidByID: w.an}
		for rows.Next() {
			var a repo.AllocationRow
			if err := rows.Scan(&a.ID, &a.ParticipantID, &a.AmountVND); err != nil {
				t.Fatal(err)
			}
			stored = append(stored, a)
			source.Allocations = append(source.Allocations, ledger.Allocation{ParticipantID: a.ParticipantID, AmountVND: money.VND(a.AmountVND)})
		}
		rows.Close()
		return expense.ID, source, stored
	}
	first, s1, a1 := confirm(map[string]int64{w.an: 100_000, w.binh: 100_000, w.chi: 100_000})
	_, s2, a2 := confirm(map[string]int64{w.an: 50_000, w.dung: 50_000})
	w.firstExpense = first
	edges, err := plan.Obligations([]plan.Source{s1, s2})
	if err != nil {
		t.Fatal(err)
	}
	var drafts []repo.ObligationDraft
	for _, e := range edges {
		d := repo.ObligationDraft{SenderID: e.SenderID, RecipientID: e.RecipientID, AmountVND: e.AmountVND.Int64()}
		for _, a := range append(append([]repo.AllocationRow{}, a1...), a2...) {
			if a.ParticipantID == e.SenderID {
				d.Sources = append(d.Sources, a)
			}
		}
		drafts = append(drafts, d)
	}
	frozen, err := r.SaveFrozenBatch(ctx, repo.FrozenBatchInput{ContextID: w.group, OwnerID: w.an, DueAt: w.now.Add(48 * time.Hour), Obligations: drafts, Now: w.now})
	if err != nil {
		t.Fatal(err)
	}
	w.batch = frozen.ID
	batch, err := r.LoadBatchForPublish(ctx, w.batch)
	if err != nil {
		t.Fatal(err)
	}
	var links []repo.GuestLinkDraft
	for _, p := range []string{w.binh, w.chi, w.dung} {
		token, err := mintToken()
		if err != nil {
			t.Fatal(err)
		}
		w.tokens[p] = token
		links = append(links, repo.GuestLinkDraft{SenderID: p, TokenDigest: auth.TokenDigest(token), ExpiresAt: w.now.Add(30 * 24 * time.Hour)})
	}
	if _, err := r.SavePublishedBatch(ctx, *batch, "published", links, w.an, w.now); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *world) store(at time.Time) Store {
	return Store{Pool: w.pool, Now: func() time.Time { return at }}
}

func (w *world) linkOf(t *testing.T, token string) (status, sender string, version int) {
	t.Helper()
	err := w.pool.QueryRow(context.Background(), `SELECT gl.status::text, e.sender_id::text, v.version_number
		  FROM guest_links gl JOIN collection_envelopes e ON e.id = gl.envelope_id JOIN collection_batch_versions v ON v.id = e.batch_version_id
		 WHERE gl.token_digest=$1`, auth.TokenDigest(token)).Scan(&status, &sender, &version)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	return
}

func (w *world) obligations(t *testing.T, version int) map[string]int64 {
	t.Helper()
	rows, err := w.pool.Query(context.Background(), `SELECT o.sender_id::text, o.amount_vnd FROM collection_obligations o JOIN collection_batch_versions v ON v.id = o.batch_version_id
		WHERE v.batch_id=$1::uuid AND v.version_number=$2`, w.batch, version)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var sender string
		var amount int64
		if err := rows.Scan(&sender, &amount); err != nil {
			t.Fatal(err)
		}
		out[sender] = amount
	}
	return out
}

func tokenOf(t *testing.T, links []ReviewLink, sender string) string {
	t.Helper()
	for _, l := range links {
		if l.SenderID == sender {
			return strings.TrimSuffix(strings.TrimPrefix(l.Path, "/g/"), "/dieu-chinh")
		}
	}
	t.Fatalf("no review link for %s in %v", sender, links)
	return ""
}

func refusal(err error) string {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	if err == nil {
		return "<nil>"
	}
	return "error: " + err.Error()
}

// Everyone affected accepts: the expense gets version 2, the batch version 2
// with the corrected obligations, each old pair its successor; Chi, freed of
// her share, keeps her URL on an empty envelope; Dũng, untouched, keeps his
// link, moved to the new version; nothing about Dũng needed his answer.
func TestAmendmentAppliesOnceEveryAffectedPersonAccepts(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	s := w.store(w.now.Add(time.Minute))
	got, err := s.Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Chi không ăn, Bình gọi thêm",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 200_000, w.chi: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "proposed" || len(got.Links) != 2 {
		t.Fatalf("proposal: %+v", got)
	}
	binhReview, chiReview := tokenOf(t, got.Links, w.binh), tokenOf(t, got.Links, w.chi)
	if st, _, _ := w.linkOf(t, w.tokens[w.binh]); st != "rotated" {
		t.Fatalf("Bình's old link is %s, want rotated", st)
	}
	if st, _, v := w.linkOf(t, w.tokens[w.dung]); st != "active" || v != 1 {
		t.Fatalf("Dũng's link moved early: %s v%d", st, v)
	}
	var versions int
	_ = w.pool.QueryRow(ctx, `SELECT count(*) FROM expense_versions WHERE expense_id=$1::uuid`, w.firstExpense).Scan(&versions)
	if versions != 1 {
		t.Fatalf("a proposal wrote %d expense versions; balances must not move before everyone agrees", versions)
	}
	if _, err := s.Decide(ctx, got.AmendmentID, w.dung, true, "app"); refusal(err) != "amendment_not_affected" {
		t.Fatalf("Dũng answered an amendment that does not touch him: %v", refusal(err))
	}
	if st, err := s.Decide(ctx, got.AmendmentID, w.binh, true, "guest_link"); err != nil || st != "proposed" {
		t.Fatalf("Bình: %s %v", st, err)
	}
	if _, err := s.Decide(ctx, got.AmendmentID, w.binh, true, "guest_link"); refusal(err) != "amendment_already_answered" {
		t.Fatalf("Bình answered twice: %v", refusal(err))
	}
	if st, err := s.Decide(ctx, got.AmendmentID, w.chi, true, "guest_link"); err != nil || st != "applied" {
		t.Fatalf("Chi: %s %v", st, err)
	}

	if o := w.obligations(t, 2); len(o) != 2 || o[w.binh] != 200_000 || o[w.dung] != 50_000 {
		t.Fatalf("version 2 obligations: %v", o)
	}
	if o := w.obligations(t, 1); o[w.chi] != 100_000 || o[w.binh] != 100_000 {
		t.Fatalf("version 1 changed: %v", o)
	}
	_ = w.pool.QueryRow(ctx, `SELECT count(*) FROM expense_versions WHERE expense_id=$1::uuid`, w.firstExpense).Scan(&versions)
	if versions != 2 {
		t.Fatalf("expense versions after applying: %d", versions)
	}
	var successions int
	_ = w.pool.QueryRow(ctx, `SELECT count(*) FROM collection_obligation_successions WHERE amendment_id=$1::uuid`, got.AmendmentID).Scan(&successions)
	if successions != 2 {
		t.Fatalf("successions: %d, want Bình's and Dũng's pairs", successions)
	}
	if st, sender, v := w.linkOf(t, binhReview); st != "active" || sender != w.binh || v != 2 {
		t.Fatalf("Bình's review URL now opens %s %s v%d", st, sender, v)
	}
	if st, sender, v := w.linkOf(t, chiReview); st != "active" || sender != w.chi || v != 2 {
		t.Fatalf("Chi's review URL now opens %s %s v%d", st, sender, v)
	}
	if o := w.obligations(t, 2); o[w.chi] != 0 {
		t.Fatalf("Chi still owes in version 2: %v", o)
	}
	if st, sender, v := w.linkOf(t, w.tokens[w.dung]); st != "active" || sender != w.dung || v != 2 {
		t.Fatalf("Dũng's link: %s %s v%d", st, sender, v)
	}
	var sources int64
	_ = w.pool.QueryRow(ctx, `SELECT coalesce(sum(s.amount_vnd),0) FROM collection_obligation_sources s JOIN collection_obligations o ON o.id = s.obligation_id
		JOIN collection_batch_versions v ON v.id = o.batch_version_id WHERE v.batch_id=$1::uuid AND v.version_number=2`, w.batch).Scan(&sources)
	if sources != 250_000 {
		t.Fatalf("version 2 sources sum to %d, want the obligations' 250000", sources)
	}
	var events int
	_ = w.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE aggregate_id=$1::uuid`, got.AmendmentID).Scan(&events)
	if events != 4 {
		t.Fatalf("audit events: %d, want proposed + two decisions + applied", events)
	}
}

// One refusal ends it: nothing is written but the answers, and each review
// URL opens again what its rotated link opened.
func TestARefusedAmendmentLeavesTheBatchAsItWas(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	s := w.store(w.now.Add(time.Minute))
	got, err := s.Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Chia lại",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 150_000, w.chi: 50_000}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Lần hai",
		Allocations: map[string]int64{w.an: 300_000}}); refusal(err) != "amendment_already_open" {
		t.Fatalf("a second open amendment: %v", refusal(err))
	}
	if st, err := s.Decide(ctx, got.AmendmentID, w.chi, false, "guest_link"); err != nil || st != "rejected" {
		t.Fatalf("Chi refused: %s %v", st, err)
	}
	if _, err := s.Decide(ctx, got.AmendmentID, w.binh, true, "guest_link"); refusal(err) != "amendment_closed" {
		t.Fatalf("an answer after the end: %v", refusal(err))
	}
	var batchVersions int
	_ = w.pool.QueryRow(ctx, `SELECT count(*) FROM collection_batch_versions WHERE batch_id=$1::uuid`, w.batch).Scan(&batchVersions)
	if batchVersions != 1 {
		t.Fatalf("a refused amendment wrote batch version %d", batchVersions)
	}
	for _, p := range []string{w.binh, w.chi} {
		if st, sender, v := w.linkOf(t, tokenOf(t, got.Links, p)); st != "active" || sender != p || v != 1 {
			t.Fatalf("review URL after refusal: %s %s v%d", st, sender, v)
		}
	}
}

// Seven days without every answer: the amendment expires the next time
// anyone touches the batch, and a new one may be proposed.
func TestAnUnansweredAmendmentExpires(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	got, err := w.store(w.now.Add(time.Minute)).Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Chia lại",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 150_000, w.chi: 50_000}})
	if err != nil {
		t.Fatal(err)
	}
	later := w.store(w.now.Add(Lifetime + 2*time.Minute))
	if _, err := later.Decide(ctx, got.AmendmentID, w.binh, true, "guest_link"); refusal(err) != "amendment_closed" {
		t.Fatalf("an answer after expiry: %v", refusal(err))
	}
	var status string
	_ = w.pool.QueryRow(ctx, `SELECT status FROM collection_amendments WHERE id=$1::uuid`, got.AmendmentID).Scan(&status)
	if status != "expired" {
		t.Fatalf("status %s", status)
	}
	if _, err := later.Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Lần hai",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 150_000, w.chi: 50_000}}); err != nil {
		t.Fatalf("a new proposal after expiry: %v", err)
	}
}

func TestOnlyTheOwnerRecorderOrPayerProposesAndOnlyAChange(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	s := w.store(w.now.Add(time.Minute))
	base := ProposeInput{BatchID: w.batch, ExpenseID: w.firstExpense, Reason: "Chia lại",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 150_000, w.chi: 50_000}}
	in := base
	in.ActorID = w.binh
	if _, err := s.Propose(ctx, in); refusal(err) != "amendment_not_allowed" {
		t.Fatalf("Bình proposed: %v", refusal(err))
	}
	var stranger string
	_ = w.pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Người lạ (dữ liệu mẫu)') RETURNING id::text`).Scan(&stranger)
	in.ActorID = stranger
	if _, err := s.Propose(ctx, in); refusal(err) != "batch_not_found" {
		t.Fatalf("a stranger: %v", refusal(err))
	}
	in.ActorID = w.an
	in.Allocations = map[string]int64{w.an: 100_000, w.binh: 100_000, w.chi: 100_000}
	if _, err := s.Propose(ctx, in); refusal(err) != "amendment_changes_nothing" {
		t.Fatalf("an empty change: %v", refusal(err))
	}
	in.Allocations = map[string]int64{w.an: 100_000, w.binh: 150_000, stranger: 50_000}
	if _, err := s.Propose(ctx, in); refusal(err) != "participant_not_member" {
		t.Fatalf("a stranger as participant: %v", refusal(err))
	}
	in.Allocations = map[string]int64{w.an: 1_000_000_000_000, w.binh: 1}
	if _, err := s.Propose(ctx, in); refusal(err) != "allocation_total_invalid" {
		t.Fatalf("past the ceiling: %v", refusal(err))
	}
	in.Allocations = map[string]int64{w.an: 300_000}
	if got, err := s.Propose(ctx, in); err != nil || got.Status != "proposed" {
		t.Fatalf("An alone keeps the whole bill: %+v %v", got, err)
	}
}

// The proposer shares the review links, so a link must never answer for
// someone with an account (security review 2026-10-05): Bình signed in once,
// gets no link, cannot answer through one, and answers in the app.
func TestAMemberWithAnAccountAnswersOnlyInTheApp(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	session, err := mintToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES(gen_random_uuid(),$1,$2,'genesis',now()+interval '1 hour')`,
		w.binh, auth.TokenDigest(session)); err != nil {
		t.Fatal(err)
	}
	s := w.store(w.now.Add(time.Minute))
	got, err := s.Propose(ctx, ProposeInput{BatchID: w.batch, ActorID: w.an, ExpenseID: w.firstExpense, Reason: "Chia lại",
		Allocations: map[string]int64{w.an: 100_000, w.binh: 150_000, w.chi: 50_000}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 1 || got.Links[0].SenderID != w.chi {
		t.Fatalf("review links: %+v, want Chi's alone", got.Links)
	}
	if st, _, _ := w.linkOf(t, w.tokens[w.binh]); st != "rotated" {
		t.Fatalf("Bình's guest link still shows the old amounts: %s", st)
	}
	if _, err := s.Decide(ctx, got.AmendmentID, w.binh, true, "guest_link"); refusal(err) != "amendment_answer_in_app" {
		t.Fatalf("a link answered for Bình: %v", refusal(err))
	}
	if _, err := s.Decide(ctx, got.AmendmentID, w.binh, true, "proposer"); err == nil {
		t.Fatal("a caller wrote a proposer's answer")
	}
	if st, err := s.Decide(ctx, got.AmendmentID, w.binh, true, "app"); err != nil || st != "proposed" {
		t.Fatalf("Bình in the app: %s %v", st, err)
	}
	if st, err := s.Decide(ctx, got.AmendmentID, w.chi, true, "guest_link"); err != nil || st != "applied" {
		t.Fatalf("Chi: %s %v", st, err)
	}
}
