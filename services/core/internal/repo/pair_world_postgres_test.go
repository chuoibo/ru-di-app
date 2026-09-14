//go:build postgres

package repo

// Fixtures for the pair-consent reads, written as literal SQL through
// seed_postgres_test.go's world so the same statements seed the Go tests and
// the Python side of the oracle. Every order the reads impose has a tie or an
// inversion in it: memberships whose created_at and id disagree, cycle
// participants and consents tied on created_at, proposals tied on created_at.

import "strings"

const (
	kindNotebook = 0x90
	kindCycle    = 0x91
	kindProposal = 0x92
	kindConsent  = 0x93
)

type pairWorld struct {
	world
	a, b, c, d, e, f                       string
	group, ab, ac, bc, ad, fb, ae, bd      string
	missingContext, missingPerson          string
	abLive, abDeadline, adPending, fbLive  string
	abLapSo, abBatDoi, abDocChat, bcClosed string
}

func (w *pairWorld) pairContext(n int, x, y string) string {
	id := fid(kindContext, n)
	lo, hi := x, y
	if hi < lo {
		lo, hi = hi, lo
	}
	w.insert("contexts", "id", id, "display_name", "", "created_by_id", x, "created_at", stdCreated,
		"kind", "pair", "pair_key", lo+":"+hi)
	return id
}

func (w *pairWorld) member(n int, contextID, personID, state, createdAt string, extra ...any) {
	pairs := []any{"id", fid(kindMembership, n), "context_id", contextID, "person_id", personID,
		"state", state, "created_at", createdAt}
	switch state {
	case "active":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z")
	case "left":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z", "left_at", "2030-01-03T00:00:00.25Z")
	}
	w.insert("memberships", append(pairs, extra...)...)
}

func (w *pairWorld) notebook(n int, contextID string) string {
	id := fid(kindNotebook, n)
	w.insert("pair_notebooks", "id", id, "context_id", contextID, "context_kind", "pair", "created_at", stdCreated)
	return id
}

func (w *pairWorld) cycle(n int, notebook, state, createdAt string, termsVersion int) string {
	id := fid(kindCycle, n)
	pairs := []any{"id", id, "notebook_id", notebook, "state", state, "terms_version", termsVersion, "created_at", createdAt}
	if state != "pending" {
		pairs = append(pairs, "opened_at", createdAt)
	}
	if state == "closed" {
		pairs = append(pairs, "closed_at", "2030-06-01T00:00:00Z")
	}
	w.insert("pair_notebook_cycles", pairs...)
	return id
}

func (w *pairWorld) participant(cycle, person, createdAt string) {
	w.insert("pair_cycle_participants", "cycle_id", cycle, "person_id", person, "created_at", createdAt)
}

func (w *pairWorld) proposal(n int, cycle, purpose, by, createdAt, expiresAt string, completedAt any) string {
	id := fid(kindProposal, n)
	w.insert("pair_consent_proposals", "id", id, "cycle_id", cycle, "purpose", purpose, "proposed_by_id", by,
		"terms_version", 1+n%2, "completed_at", completedAt, "created_at", createdAt, "expires_at", expiresAt)
	return id
}

func (w *pairWorld) consent(n int, proposal, person string, grantedAt, revokedAt any, createdAt string) {
	w.insert("pair_consents", "id", fid(kindConsent, n), "proposal_id", proposal, "person_id", person,
		"granted_at", grantedAt, "revoked_at", revokedAt, "created_at", createdAt)
}

func (w *pairWorld) constraint(cycle, owner, kind, content string, version int, updatedAt string) {
	w.insert("pair_shared_constraints", "cycle_id", cycle, "owner_id", owner, "kind", kind, "content", content,
		"version", version, "updated_at", updatedAt)
}

func newPairWorld() *pairWorld {
	w := &pairWorld{}
	w.a = w.person(0x11, "An (dữ liệu mẫu)", "budget_band", "vua-phai")
	w.b = w.person(0x12, "Bình (dữ liệu mẫu)", "budget_band", "tiet-kiem")
	w.c = w.person(0x13, "Chi (dữ liệu mẫu)")
	w.d = w.person(0x14, "Dũng (dữ liệu mẫu)", "budget_band", "thoai-mai")
	w.e = w.person(0x15, "", "budget_band", "")
	w.f = w.person(0x16, "Phương (dữ liệu mẫu)", "budget_band", "vua-phai", "deleted_at", "2030-02-01T00:00:00Z")
	w.missingContext = fid(kindContext, 0xff)
	w.missingPerson = fid(kindPerson, 0xff)

	// A group: every membership state, a re-join, an erased account, and a
	// row whose created_at sorts before rows with smaller ids.
	w.group = w.context(0x31, w.a)
	w.member(0x41, w.group, w.a, "active", stdCreated, "role", "admin")
	w.member(0x42, w.group, w.b, "active", stdCreated)
	w.member(0x43, w.group, w.c, "invited", stdCreated, "invited_by_id", w.a, "origin", "link")
	w.member(0x44, w.group, w.d, "left", stdCreated)
	w.member(0x46, w.group, w.d, "active", "2030-01-05T00:00:00Z")
	w.member(0x47, w.group, w.f, "active", stdCreated)
	w.member(0x4f, w.group, w.e, "active", "2029-12-31T23:59:59.999999Z")

	// a+b: a closed cycle whose grants must not count, then the live one.
	w.ab = w.pairContext(0x32, w.a, w.b)
	w.member(0x51, w.ab, w.a, "active", stdCreated)
	w.member(0x52, w.ab, w.b, "active", stdCreated)
	nb := w.notebook(1, w.ab)
	old := w.cycle(1, nb, "closed", "2030-01-10T00:00:00Z", 1)
	w.participant(old, w.a, "2030-01-10T00:00:00Z")
	w.participant(old, w.c, "2030-01-10T00:00:00Z")
	oldChat := w.proposal(1, old, "doc_chat", w.a, "2030-01-10T00:00:00Z", "2030-01-17T00:00:00Z", "2030-01-11T00:00:00Z")
	w.consent(1, oldChat, w.a, "2030-01-10T00:00:00Z", nil, "2030-01-10T00:00:00Z")
	w.consent(2, oldChat, w.c, "2030-01-11T00:00:00Z", nil, "2030-01-11T00:00:00Z")
	w.abLive = w.cycle(2, nb, "active", "2030-02-01T00:00:00Z", 2)
	w.participant(w.abLive, w.b, "2030-02-01T00:00:00Z")
	w.participant(w.abLive, w.a, "2030-02-01T00:00:00.000001Z")
	w.abDeadline = "2030-02-09T00:00:00.5Z"
	w.abDocChat = w.proposal(4, w.abLive, "doc_chat", w.b, "2030-02-02T00:00:00Z", w.abDeadline, "2030-02-02T01:00:00Z")
	w.abBatDoi = w.proposal(3, w.abLive, "bat_doi", w.a, "2030-02-02T00:00:00Z", "2030-02-09T00:00:00Z", nil)
	w.abLapSo = w.proposal(2, w.abLive, "lap_so", w.a, "2030-02-01T00:00:00Z", "2030-02-08T00:00:00Z", "2030-02-01T00:10:00Z")
	w.consent(8, w.abLapSo, w.a, "2030-02-01T00:05:00Z", nil, "2030-02-01T00:05:00Z")
	w.consent(7, w.abLapSo, w.b, "2030-02-01T00:10:00Z", nil, "2030-02-01T00:10:00Z")
	w.consent(6, w.abDocChat, w.b, "2030-02-02T00:00:00Z", nil, "2030-02-02T00:00:00Z")
	w.consent(5, w.abDocChat, w.a, "2030-02-02T01:00:00.123456Z", nil, "2030-02-02T00:00:00Z")
	w.consent(9, w.abBatDoi, w.a, "2030-02-02T00:30:00Z", nil, "2030-02-02T00:30:00Z")
	w.consent(10, w.abBatDoi, w.b, nil, nil, "2030-02-02T00:30:00Z")
	w.constraint(w.abLive, w.b, "dung", "Đừng hát (dữ liệu mẫu)", 1, "2030-02-03T00:00:00Z")
	w.constraint(w.abLive, w.a, "khong_an_duoc", "Hành 🧅 (dữ liệu mẫu)", 3, "2030-02-03T00:00:00Z")
	w.constraint(w.abLive, w.a, "dung", "", 2, "2030-02-04T00:00:00.000001Z")

	// a+c: no notebook.
	w.ac = w.pairContext(0x33, w.a, w.c)
	w.member(0x53, w.ac, w.a, "active", stdCreated)
	w.member(0x54, w.ac, w.c, "active", stdCreated)

	// b+c: a notebook whose only cycle is closed, with grants from both.
	w.bc = w.pairContext(0x34, w.b, w.c)
	w.member(0x55, w.bc, w.b, "active", stdCreated)
	w.member(0x56, w.bc, w.c, "active", stdCreated)
	nb = w.notebook(2, w.bc)
	w.bcClosed = w.cycle(3, nb, "closed", "2030-01-20T00:00:00Z", 1)
	w.participant(w.bcClosed, w.b, "2030-01-20T00:00:00Z")
	w.participant(w.bcClosed, w.c, "2030-01-20T00:00:00Z")
	closedChat := w.proposal(5, w.bcClosed, "doc_chat", w.b, "2030-01-20T00:00:00Z", "2030-12-01T00:00:00Z", "2030-01-20T01:00:00Z")
	w.consent(11, closedChat, w.b, "2030-01-20T00:00:00Z", nil, "2030-01-20T00:00:00Z")
	w.consent(12, closedChat, w.c, "2030-01-20T01:00:00Z", nil, "2030-01-20T01:00:00Z")

	// a+d: a pending cycle after a closed one. d has since left and e is an
	// active member, so the cycle's list and the members' list disagree.
	w.ad = w.pairContext(0x35, w.a, w.d)
	w.member(0x57, w.ad, w.a, "active", stdCreated)
	w.member(0x58, w.ad, w.d, "left", stdCreated)
	w.member(0x59, w.ad, w.e, "active", stdCreated)
	nb = w.notebook(3, w.ad)
	w.participant(w.cycle(5, nb, "closed", "2030-02-15T00:00:00Z", 1), w.a, "2030-02-15T00:00:00Z")
	w.adPending = w.cycle(4, nb, "pending", "2030-03-01T00:00:00Z", 3)
	w.participant(w.adPending, w.d, "2030-03-01T00:00:00Z")
	w.participant(w.adPending, w.a, "2030-03-01T00:00:00Z")
	adChat := w.proposal(6, w.adPending, "doc_chat", w.a, "2030-01-01T00:00:00Z", "2030-12-08T00:00:00Z", "2030-03-01T02:00:00Z")
	w.consent(13, adChat, w.a, "2030-03-01T00:00:00Z", nil, "2030-03-01T00:00:00Z")
	w.consent(14, adChat, w.d, "2030-03-01T02:00:00Z", nil, "2030-03-01T02:00:00Z")

	// f+b: both granted, b revoked.
	w.fb = w.pairContext(0x36, w.f, w.b)
	w.member(0x5a, w.fb, w.f, "active", stdCreated)
	w.member(0x5b, w.fb, w.b, "active", stdCreated)
	nb = w.notebook(4, w.fb)
	w.fbLive = w.cycle(6, nb, "active", "2030-01-25T00:00:00Z", 1)
	w.participant(w.fbLive, w.f, "2030-01-25T00:00:00Z")
	w.participant(w.fbLive, w.b, "2030-01-25T00:00:00Z")
	fbChat := w.proposal(7, w.fbLive, "doc_chat", w.f, "2030-01-25T00:00:00Z", "2030-12-01T00:00:00Z", "2030-01-25T01:00:00Z")
	w.consent(15, fbChat, w.f, "2030-01-25T00:00:00Z", nil, "2030-01-25T00:00:00Z")
	w.consent(16, fbChat, w.b, "2030-01-25T01:00:00Z", "2030-01-30T00:00:00Z", "2030-01-25T01:00:00Z")

	// a+e: each side granted a different offer of the same purpose, and the
	// other side never answered either.
	w.ae = w.pairContext(0x37, w.a, w.e)
	w.member(0x5c, w.ae, w.a, "active", stdCreated)
	w.member(0x5d, w.ae, w.e, "active", stdCreated)
	nb = w.notebook(5, w.ae)
	aeLive := w.cycle(7, nb, "active", "2030-01-26T00:00:00Z", 1)
	w.participant(aeLive, w.a, "2030-01-26T00:00:00Z")
	w.participant(aeLive, w.e, "2030-01-26T00:00:00Z")
	first := w.proposal(8, aeLive, "doc_chat", w.a, "2030-01-26T00:00:00Z", "2030-12-01T00:00:00Z", nil)
	w.consent(17, first, w.a, "2030-01-26T00:00:00Z", nil, "2030-01-26T00:00:00Z")
	w.consent(18, first, w.e, nil, nil, "2030-01-26T00:00:00Z")
	second := w.proposal(9, aeLive, "doc_chat", w.e, "2030-01-27T00:00:00Z", "2030-12-01T00:00:00Z", nil)
	w.consent(19, second, w.e, "2030-01-27T00:00:00Z", nil, "2030-01-27T00:00:00Z")

	// b+d: a notebook that never opened a cycle; d is only invited.
	w.bd = w.pairContext(0x38, w.b, w.d)
	w.member(0x5e, w.bd, w.b, "active", stdCreated)
	w.member(0x5f, w.bd, w.d, "invited", stdCreated)
	w.notebook(6, w.bd)
	return w
}

// sqlText is the whole fixture, for a failure message.
func (w *pairWorld) sqlText() string { return strings.Join(w.sql, ";\n") }
