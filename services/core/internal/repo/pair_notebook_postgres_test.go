//go:build postgres

package repo

// Go-only tests of the pair-consent reads on real PostgreSQL. They pin the
// shapes the service relies on; exact equality with Python is
// pair_oracle_postgres_test.go's job.

import (
	"reflect"
	"testing"
)

func TestGetContextReadsKindAndPairKey(t *testing.T) {
	w := newPairWorld()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	group, err := repo.GetContext(bg, w.group)
	if err != nil || group == nil || group.Kind != "group" || group.PairKey != nil || group.Theme != "mac-dinh" {
		t.Fatalf("group: %+v %v", group, err)
	}
	pair, err := repo.GetContext(bg, w.ab)
	if err != nil || pair == nil || pair.Kind != "pair" || pair.PairKey == nil || *pair.PairKey != w.a+":"+w.b ||
		pair.DisplayName != "" || pair.CreatedByID != w.a || pair.CreatedAt.Location().String() != "UTC" {
		t.Fatalf("pair: %+v %v", pair, err)
	}
	missing, err := repo.GetContext(bg, w.missingContext)
	if err != nil || missing != nil {
		t.Fatalf("missing: %+v %v", missing, err)
	}
	if len(rec.log) != 3 {
		t.Fatalf("%d statements, want one per call", len(rec.log))
	}
}

func TestListMembersIsTheOpenRosterNamedInOneStatement(t *testing.T) {
	w := newPairWorld()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	rows, err := repo.ListMembers(bg, w.group)
	if err != nil {
		t.Fatal(err)
	}
	var people, names, states []string
	for _, row := range rows {
		people = append(people, row.PersonID)
		names = append(names, row.DisplayName)
		states = append(states, row.State)
		if row.LeftAt != nil {
			t.Fatalf("an ended membership came back: %+v", row)
		}
	}
	// e sorts first on created_at although its id is the largest; the tie on
	// stdCreated falls to the id; d's re-join is newest; d's ended row is gone.
	if want := []string{w.e, w.a, w.b, w.c, w.f, w.d}; !reflect.DeepEqual(people, want) {
		t.Fatalf("order %v, want %v", people, want)
	}
	if want := []string{w.e, "An (dữ liệu mẫu)", "Bình (dữ liệu mẫu)", "Chi (dữ liệu mẫu)", "Phương (dữ liệu mẫu)", "Dũng (dữ liệu mẫu)"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names %v, want %v (an empty name falls back to the id)", names, want)
	}
	if want := []string{"active", "active", "active", "invited", "active", "active"}; !reflect.DeepEqual(states, want) {
		t.Fatalf("states %v", states)
	}
	if rows[1].Role != "admin" || rows[3].Origin != "link" || rows[3].InvitedByID == nil || *rows[3].InvitedByID != w.a {
		t.Fatalf("role/origin/invited_by lost: %+v %+v", rows[1], rows[3])
	}
	if len(rec.log) != 2 {
		t.Fatalf("%d statements, want the roster and one names statement", len(rec.log))
	}
	rec.log = nil
	empty, err := repo.ListMembers(bg, w.missingContext)
	if err != nil || empty == nil || len(empty) != 0 || len(rec.log) != 1 {
		t.Fatalf("missing context: %v %v after %d statements", empty, err, len(rec.log))
	}
}

func TestBudgetBandsByPersonKeepsEmptyBandsAndLeavesOutSilence(t *testing.T) {
	w := newPairWorld()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	bands, err := repo.BudgetBandsByPerson(bg, []string{w.a, w.c, w.e, w.missingPerson, w.a})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, band := range bands {
		got[band.PersonID] = band.Band
	}
	if want := map[string]string{w.a: "vua-phai", w.e: ""}; len(bands) != 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("bands %v", bands)
	}
	rec.log = nil
	none, err := repo.BudgetBandsByPerson(bg, nil)
	if err != nil || none == nil || len(none) != 0 || len(rec.log) != 0 {
		t.Fatalf("no ids: %v %v after %d statements", none, err, len(rec.log))
	}
}

func TestGetPairNotebookReadsOnlyTheLiveCycleInPythonsOrders(t *testing.T) {
	w := newPairWorld()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	notebook, err := repo.GetPairNotebook(bg, w.ab)
	if err != nil || notebook == nil {
		t.Fatalf("%+v %v", notebook, err)
	}
	if notebook.CycleID == nil || *notebook.CycleID != w.abLive || *notebook.CycleState != "active" || notebook.TermsVersion != 2 {
		t.Fatalf("cycle: %+v", notebook)
	}
	if want := []string{w.b, w.a}; !reflect.DeepEqual(notebook.Participants, want) {
		t.Fatalf("participants %v, want created_at order %v", notebook.Participants, want)
	}
	var proposals []string
	for _, p := range notebook.Proposals {
		proposals = append(proposals, p.Purpose)
	}
	if want := []string{"lap_so", "bat_doi", "doc_chat"}; !reflect.DeepEqual(proposals, want) {
		t.Fatalf("proposals %v", proposals)
	}
	var consents []string
	for _, c := range notebook.Consents {
		consents = append(consents, c.Purpose+"/"+c.PersonID)
	}
	want := []string{"lap_so/" + w.a, "lap_so/" + w.b, "doc_chat/" + w.a, "doc_chat/" + w.b, "bat_doi/" + w.a, "bat_doi/" + w.b}
	if !reflect.DeepEqual(consents, want) {
		t.Fatalf("consents %v, want %v", consents, want)
	}
	if notebook.Consents[2].ProposalExpiresAt.Format("2006-01-02T15:04:05.000000Z07:00") != "2030-02-09T00:00:00.500000Z" ||
		notebook.Consents[5].GrantedAt != nil {
		t.Fatalf("consent fields: %+v", notebook.Consents)
	}
	var constraints []string
	for _, c := range notebook.Constraints {
		constraints = append(constraints, c.OwnerID+"/"+c.Kind)
	}
	if want := []string{w.a + "/dung", w.a + "/khong_an_duoc", w.b + "/dung"}; !reflect.DeepEqual(constraints, want) {
		t.Fatalf("constraints %v", constraints)
	}
	if len(rec.log) != 6 {
		t.Fatalf("%d statements, want 6", len(rec.log))
	}

	for name, context := range map[string]string{"closed only": w.bc, "never opened": w.bd} {
		rec.log = nil
		empty, err := repo.GetPairNotebook(bg, context)
		if err != nil || empty == nil || empty.CycleID != nil || empty.CycleState != nil || empty.TermsVersion != 0 ||
			empty.Participants == nil || len(empty.Participants)+len(empty.Consents)+len(empty.Proposals)+len(empty.Constraints) != 0 {
			t.Fatalf("%s: %+v %v", name, empty, err)
		}
		if len(rec.log) != 2 {
			t.Fatalf("%s: %d statements, want the notebook and the cycle", name, len(rec.log))
		}
	}
	rec.log = nil
	for _, context := range []string{w.ac, w.group, w.missingContext} {
		none, err := repo.GetPairNotebook(bg, context)
		if err != nil || none != nil {
			t.Fatalf("%s: %+v %v", context, none, err)
		}
	}
	if len(rec.log) != 3 {
		t.Fatalf("%d statements for three missing notebooks", len(rec.log))
	}
}
