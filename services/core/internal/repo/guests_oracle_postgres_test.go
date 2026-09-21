//go:build postgres

package repo

// Differential test of the W5 repository methods (the guest routes) against
// the real SqlAlchemyApiRepository, driven through
// scripts/render_guest_repo_oracle.py. Same design as
// money_oracle_postgres_test.go, whose world, dumps, value tags, statement
// normalisation, conflict tagging, row lock probe and comparison it reuses.
//
// A case's steps share one Session on the Python side and one transaction on
// the Go side, so the route cases below run the calls each guest route makes,
// in its order, against the identity map that request would hold: the not-me
// page reads the envelope twice before it objects.
//
// Beyond the comparison, TestGuestRepositoryOracle requires the corpus to
// reach every write branch in Python (first open, expiry flip with and
// without it, revocation, a moved revoked_at, a report and its replay), so a
// fixture that drifts cannot turn a branch into a case that compares nothing.
//
// TestGuestLinkLocksSerialiseRequests is Go only: it holds the locks one
// request takes and shows a second request waits on them, and that two
// concurrent reports under one key end as one row. The Python side of the
// same guarantee is the FOR UPDATE statement text the oracle already pins.
//
// Without CORE_PYTHON_IMAGE both tests skip; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/domain/capability"
	"mobile/services/core/internal/testdb"
)

var paymentReportsDump = orderedDump("payment_reports t", fixtureFirst("t.id")+", t.reported_at, t.idempotency_key, t.id")

// ---------------------------------------------------------------------------
// The world
// ---------------------------------------------------------------------------

// guestWorld is the money world plus a published round (bt6) built for the
// guest pages: one sender owing two recipients, each obligation with sources
// whose descriptions repeat, are empty, NULL, or sort differently by code
// point than by eye; receipts that confirm, over-confirm, part-confirm or are
// absent; links in every state around the expiry boundary; reports that spend
// the budget, one of them with no link; and objection events with every shape
// of obligation_id get_guest_envelope has to read.
type guestWorld struct {
	*moneyWorld
	digest map[string]string

	v6                                             string
	oChiAn, oChiBinh, oDungAn, oErasedAn, oGhostAn string

	linkActive, linkOpened, linkAtExpiry, linkBeforeExpiry, linkLapsed, linkOpenedLapsed string
	linkStoredExpired, linkRevoked, linkRevokedNow, linkRotated, linkRevokedLapsed       string
	linkDung, linkErased, linkGhost, linkNoObligations                                   string

	keyOpened, keyNoLink string
}

// guestNow is moneyNow: linkAtExpiry expires at exactly this instant.
const guestNow = moneyNow

func newGuestWorld() *guestWorld {
	w := &guestWorld{moneyWorld: newMoneyWorld(), digest: map[string]string{}}
	m := w.moneyWorld
	for n, id := range []string{m.linkOld, m.linkBinh, m.linkChi} {
		w.digest[id] = strings.Repeat(fmt.Sprintf("%02x", n+1), 32)
	}

	alloc := map[string]string{}
	expense := func(n int, payer string, description any, shares ...share) string {
		sql, ids := expenseSQL(n, m.g, payer, alloc, moneyVersion{"acknowledged", description,
			fmt.Sprintf("2030-05-1%dT12:00:00Z", n-0x60), shares})
		m.sql = append(m.sql, sql...)
		return ids[0]
	}
	grill := "Nướng 🍢 (dữ liệu mẫu)"
	ex1 := expense(0x61, m.an, grill, share{m.chi, 30_000}, share{m.dung, 10_000}, share{m.an, 20_000})
	ex2 := expense(0x62, m.binh, "Xe ' \" (dữ liệu mẫu)", share{m.chi, 25_000}, share{m.binh, 25_000})
	ex3 := expense(0x63, m.an, "", share{m.chi, 5_000}, share{m.an, 5_000})
	ex4 := expense(0x64, m.an, nil, share{m.chi, 1_000}, share{m.an, 1_000})
	ex5 := expense(0x65, m.an, grill, share{m.chi, 2_000}, share{m.an, 2_000})
	ex6 := expense(0x66, m.an, "Ăn vặt (dữ liệu mẫu)", share{m.chi, 3_000}, share{m.an, 3_000})
	ex7 := expense(0x67, m.an, "bánh (dữ liệu mẫu)", share{m.chi, 4_000}, share{m.an, 4_000})

	m.sql = append(m.sql, extraBatchSQL(6, m.g, m.an, "2030-03-01T00:00:00Z",
		extraObligation{m.chi, m.an, 45_000, []int64{20_000, 25_000}},
		extraObligation{m.chi, m.binh, 25_000, []int64{30_000}},
		extraObligation{m.dung, m.an, 10_000, []int64{5_000}},
		extraObligation{m.erased, m.an, 7_000, nil},
		extraObligation{m.missingPerson, m.an, 4_000, nil})...)
	w.v6 = fid(kindBatchVersion, 0x66)
	w.oChiAn, w.oChiBinh, w.oDungAn = extraObligationID(6, 0), extraObligationID(6, 1), extraObligationID(6, 2)
	w.oErasedAn, w.oGhostAn = extraObligationID(6, 3), extraObligationID(6, 4)
	source := func(obligation, version, participant string, amount int64) {
		m.insert("collection_obligation_sources", "obligation_id", obligation,
			"confirmed_allocation_id", alloc[allocKey(version, participant)], "amount_vnd", amount, "created_at", stdCreated)
	}
	source(w.oChiAn, ex1, m.chi, 30_000)
	source(w.oChiAn, ex3, m.chi, 5_000)
	source(w.oChiAn, ex4, m.chi, 1_000)
	source(w.oChiAn, ex5, m.chi, 2_000)
	source(w.oChiAn, ex6, m.chi, 3_000)
	source(w.oChiAn, ex7, m.chi, 4_000)
	source(w.oChiBinh, ex2, m.chi, 25_000)
	source(w.oDungAn, ex1, m.dung, 10_000)

	envelope := func(k int, sender string) string {
		id := fid(kindEnvelope, 0x60+k)
		m.insert("collection_envelopes", "id", id, "batch_version_id", w.v6, "sender_id", sender, "created_at", stdCreated)
		return id
	}
	envChi, envDung, envErased := envelope(1, m.chi), envelope(2, m.dung), envelope(3, m.erased)
	envGhost, envStranger := envelope(4, m.missingPerson), envelope(5, m.stranger)
	link := func(k int, envelopeID, status, expires string, extra ...any) string {
		id := fid(kindGuestLink, 0x60+k)
		m.insert("guest_links", append([]any{"id", id, "envelope_id", envelopeID,
			"token_digest", raw(fmt.Sprintf("decode(repeat('%02x', 32), 'hex')", 0x60+k)), "status", status,
			"expires_at", expires, "created_at", stdCreated}, extra...)...)
		w.digest[id] = strings.Repeat(fmt.Sprintf("%02x", 0x60+k), 32)
		return id
	}
	const far, past = "2031-01-01T00:00:00Z", "2030-06-01T00:00:00Z"
	w.linkActive = link(1, envChi, "active", far)
	w.linkOpened = link(2, envChi, "active", far, "first_opened_at", "2030-03-02T08:00:00.5Z")
	w.linkAtExpiry = link(3, envChi, "active", guestNow)
	w.linkBeforeExpiry = link(4, envChi, "active", "2030-07-10T12:00:00.654322Z")
	w.linkStoredExpired = link(5, envChi, "expired", past)
	w.linkRevoked = link(6, envChi, "revoked", far, "revoked_at", "2030-06-15T00:00:00Z")
	w.linkRevokedNow = link(7, envChi, "revoked", far, "revoked_at", "2030-07-10T19:00:00.654321+07:00")
	w.linkRotated = link(8, envChi, "rotated", far)
	w.linkRevokedLapsed = link(9, envChi, "revoked", past)
	w.linkDung = link(10, envDung, "active", far)
	w.linkErased = link(11, envErased, "active", far)
	w.linkGhost = link(12, envGhost, "active", far)
	w.linkNoObligations = link(13, envStranger, "active", far)
	w.linkLapsed = link(14, envChi, "active", past)
	w.linkOpenedLapsed = link(15, envChi, "active", past, "first_opened_at", "2030-05-02T00:00:00Z")

	report := func(n int, obligation string, linkID any, amount int64, at string, extra ...any) string {
		key := fid(kindReceiptKey, 0x960+n)
		m.insert("payment_reports", append([]any{"id", fid(kindPaymentReport, 0x60+n), "obligation_id", obligation,
			"guest_link_id", linkID, "amount_vnd", amount, "idempotency_key", key, "reported_at", at}, extra...)...)
		return key
	}
	w.keyOpened = report(1, w.oChiAn, w.linkOpened, 45_000, "2030-06-05T00:00:00Z")
	report(2, w.oChiAn, w.linkOpened, 45_000, "2030-06-06T00:00:00Z")
	report(3, w.oChiBinh, w.linkOpened, 25_000, "2030-06-07T00:00:00Z")
	w.keyNoLink = report(4, w.oChiBinh, nil, 25_000, "2030-06-08T00:00:00Z", "reported_by_id", m.chi)

	event := func(n int, linkID, eventType, data string, extra ...any) {
		m.insert("audit_events", append([]any{"id", fid(kindAuditEvent, 0x60+n), "event_type", eventType,
			"aggregate_type", "guest_link", "aggregate_id", linkID, "event_data", data,
			"occurred_at", fmt.Sprintf("2030-06-1%dT00:00:00Z", n%10)}, extra...)...)
	}
	wrong, notMe, evidence := "guest_objection.wrong_amount", "guest_objection.not_me", "guest_objection.evidence_request"
	event(1, w.linkOpened, wrong, `{"kind": "wrong_amount", "obligation_id": "`+w.oChiAn+`", "reason": "so_tien_sai"}`)
	event(2, w.linkOpened, wrong, `{"kind": "wrong_amount", "obligation_id": "`+w.oChiAn+`", "reason": "chua_nhan"}`)
	event(3, w.linkOpened, notMe, `{"kind": "not_me", "obligation_id": "`+w.oChiAn+`", "reason": null}`)
	event(4, w.linkOpened, notMe, `{"kind": "not_me", "obligation_id": null, "reason": null}`)
	event(5, w.linkOpened, evidence, `{"kind": "evidence_request", "obligation_id": "`+w.oChiBinh+`"}`)
	event(6, w.linkOpened, evidence, `{"obligation_id": ""}`)
	event(7, w.linkOpened, wrong, `{"obligation_id": 7}`)
	event(8, w.linkOpened, wrong, `{"obligation_id": "`+strings.ToUpper(w.oChiBinh)+`"}`)
	event(9, w.linkOpened, wrong, `{"obligation_id": true}`)
	event(10, w.linkOpened, "guest_objection.other", `{"obligation_id": "`+w.oChiBinh+`"}`)
	event(11, w.linkOpened, wrong, `{"obligation_id": "`+w.oChiBinh+`"}`, "aggregate_type", "collection_obligation")
	event(12, w.linkOpened, evidence, `{"obligation_id": "`+w.oChiAn+`", "extra": {"nested": [1, 2.5]}}`)
	event(13, w.linkActive, wrong, `{"obligation_id": "`+w.oChiBinh+`", "reason": "khac_lien_ket"}`)
	return w
}

// badEvent is one guest_link event whose data get_guest_envelope cannot read.
func (w *guestWorld) badEvent(linkID, eventType, data string) []string {
	return []string{insertSQL("audit_events", "id", fid(kindAuditEvent, 0x7f), "event_type", eventType,
		"aggregate_type", "guest_link", "aggregate_id", linkID, "event_data", data, "occurred_at", "2030-06-20T00:00:00Z")}
}

// ---------------------------------------------------------------------------
// Tags and the Go side
// ---------------------------------------------------------------------------

func tPair(key string, value any) any { return []any{tStr(key), value} }

func tGuestEnvelope(r GuestEnvelopeRecord) any {
	blocks := []any{}
	for _, b := range r.Envelope.Obligations {
		blocks = append(blocks, tv("dict", []any{
			tPair("obligation_id", tStr(b.ObligationID)),
			tPair("occasion_label", tStr(b.OccasionLabel)),
			tPair("amount_vnd", tInt(b.AmountVND)),
			tPair("recipient_display_name", tStr(b.RecipientDisplayName)),
			tPair("already_reported", tBool(b.AlreadyReported)),
			tPair("evidence_requested", tBool(b.EvidenceRequested)),
			tPair("disputed", tBool(b.Disputed)),
			tPair("objections_used", tInt(b.ObjectionsUsed)),
			tPair("objections_allowed", tInt(b.ObjectionsAllowed)),
			tPair("receiver_confirmed", tBool(b.ReceiverConfirmed)),
		}))
	}
	e := r.Envelope
	return tRecord("GuestEnvelopeRecord", "link_id", tUUID(r.LinkID), "envelope", tv("dict", []any{
		tPair("recorded_by_display_name", tStr(e.RecordedByDisplayName)),
		tPair("claimed_person_display_name", tStr(e.ClaimedPersonDisplayName)),
		tPair("link_state", tStr(e.LinkState)),
		tPair("obligations", tSeq(blocks)),
		tPair("reports_used", tInt(e.ReportsUsed)),
		tPair("reports_allowed", tInt(e.ReportsAllowed)),
		tPair("objections_used", tInt(e.ObjectionsUsed)),
		tPair("objections_allowed", tInt(e.ObjectionsAllowed)),
	}))
}

func tPaymentReportTarget(p PaymentReportTarget) any {
	return tRecord("PaymentReportTarget", "link_id", tUUID(p.LinkID), "obligation_id", tUUID(p.ObligationID),
		"amount_vnd", tInt(p.AmountVND), "active_capability", tBool(p.ActiveCapability), "reports_used", tInt(p.ReportsUsed))
}

func tPaymentReportRecord(p PaymentReportRecord) any {
	amounts := []any{}
	for _, a := range p.ReceiptAmountsVND {
		amounts = append(amounts, tInt(a))
	}
	return tRecord("PaymentReportRecord", "id", tUUID(p.ID), "obligation_id", tUUID(p.ObligationID),
		"amount_vnd", tInt(p.AmountVND), "receipt_amounts_vnd", tSeq(amounts))
}

func argDigest(a map[string]any) []byte {
	digest, err := hex.DecodeString(argString(a, "token_digest"))
	if err != nil {
		panic(err)
	}
	return digest
}

func guestGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	switch method {
	case "get_guest_envelope":
		record, err := repo.GetGuestEnvelope(bg, argDigest(a), argInstant(s("now")))
		return nilOr(record, tGuestEnvelope), err
	case "get_payment_report_target":
		target, err := repo.GetPaymentReportTarget(bg, argDigest(a), s("obligation_id"), argInstant(s("now")))
		return nilOr(target, tPaymentReportTarget), err
	case "save_payment_report":
		record, err := repo.SavePaymentReport(bg, PaymentReportInput{Target: PaymentReportTarget{LinkID: s("link_id"),
			ObligationID: s("obligation_id"), AmountVND: argNumber(a["amount_vnd"]),
			ActiveCapability: a["active_capability"].(bool), ReportsUsed: argNumber(a["reports_used"])},
			IdempotencyKey: s("idempotency_key"), Now: argInstant(s("now"))})
		return tPaymentReportRecord(record), err
	case "flow.report_payment":
		target, err := repo.GetPaymentReportTarget(bg, argDigest(a), s("obligation_id"), argInstant(s("now")))
		if err != nil {
			return nil, err
		}
		if target == nil {
			return tSeq([]any{nil, nil}), nil
		}
		record, err := repo.SavePaymentReport(bg, PaymentReportInput{Target: *target,
			IdempotencyKey: s("idempotency_key"), Now: argInstant(s("now"))})
		return tSeq([]any{tPaymentReportTarget(*target), tPaymentReportRecord(record)}), err
	case "save_guest_objection":
		return nil, repo.SaveGuestObjection(bg, GuestObjectionInput{TokenDigest: argDigest(a), Kind: s("kind"),
			ObligationID: argText(a["obligation_id"]), Reason: argText(a["reason"]), Now: argInstant(s("now"))})
	}
	return moneyGoCall(repo, method, a)
}

// guestGoError is moneyGoError plus the domain refusal get_guest_envelope
// lets propagate.
func guestGoError(err error) map[string]any {
	out := moneyGoError(err)
	var scope *capability.CapabilityScopeError
	if errors.As(err, &scope) {
		out["type"] = "CapabilityScopeError"
	}
	return out
}

func runGuestGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
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
		value, err := guestGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = guestGoError(err)
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

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func guestOracleCases() ([]socialCase, oracleSpec) {
	w := newGuestWorld()
	m := w.moneyWorld
	var cases []socialCase
	add := func(name, wantEnd string, setup []string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps}, wantEnd})
	}
	probes := func(dumps ...string) []string {
		return append(append([]string{probeLocks, probeWrites, probeMoneyRowLocks}, dumps...), probeNow)
	}
	step := func(method string, a map[string]any, dumps ...string) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...), Probes: probes(dumps...)}
	}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	later := func(n int) string { return fmt.Sprintf("2030-07-1%dT12:00:00.654321Z", n) }
	base := m.sql
	vietnam := join([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base)
	nobody := strings.Repeat("ee", 32)
	dg := func(linkID string) string {
		digest, ok := w.digest[linkID]
		if !ok {
			panic("no digest for " + linkID)
		}
		return digest
	}
	newKey := func(n int) string { return fid(kindReceiptKey, 0x980+n) }

	open := func(digest, now string) oracleCall {
		return step("get_guest_envelope", args("token_digest", digest, "now", now), linksDump)
	}
	target := func(digest, obligation, now string) oracleCall {
		return step("get_payment_report_target", args("token_digest", digest, "obligation_id", obligation, "now", now), linksDump)
	}
	report := func(linkID, obligation string, amount int64, active bool, used int, key, now string) oracleCall {
		return step("save_payment_report", args("link_id", linkID, "obligation_id", obligation, "amount_vnd", amount,
			"active_capability", active, "reports_used", used, "idempotency_key", key, "now", now), paymentReportsDump, auditDump)
	}
	reportThrough := func(digest, obligation, key, now string) oracleCall {
		return step("flow.report_payment", args("token_digest", digest, "obligation_id", obligation,
			"idempotency_key", key, "now", now), paymentReportsDump, auditDump)
	}
	object := func(digest, kind string, obligation, reason any, now string) oracleCall {
		return step("save_guest_objection", args("token_digest", digest, "kind", kind, "obligation_id", obligation,
			"reason", reason, "now", now), auditDump, linksDump)
	}

	// --- get_guest_envelope ------------------------------------------------------
	add("get_guest_envelope: a first open records it, a later open writes nothing", "", base,
		open(dg(w.linkActive), guestNow), open(dg(w.linkActive), later(1)))
	add("get_guest_envelope: two obligations, disputes, evidence, quota and reports read back", "", base,
		open(dg(w.linkOpened), guestNow))
	add("get_guest_envelope: an expiry at exactly now flips the link on its first open", "", base,
		open(dg(w.linkAtExpiry), guestNow), open(dg(w.linkAtExpiry), later(1)))
	add("get_guest_envelope: an expiry one microsecond after now stays active", "", base,
		open(dg(w.linkBeforeExpiry), guestNow))
	add("get_guest_envelope: a link opened before whose expiry passed flips alone", "", base,
		open(dg(w.linkOpenedLapsed), guestNow))
	add("get_guest_envelope: clocks past microseconds, at another offset, on both sides of the expiry", "", base,
		open(dg(w.linkBeforeExpiry), "2030-07-10T19:00:00.654321"+"99+07:00"),
		open(dg(w.linkAtExpiry), "2030-07-10T19:00:00.654321"+"01+07:00"))
	add("get_guest_envelope: stored expired, revoked, rotated and revoked past expiry keep their state", "", base,
		open(dg(w.linkStoredExpired), guestNow), open(dg(w.linkRevoked), guestNow), open(dg(w.linkRotated), guestNow),
		open(dg(w.linkRevokedLapsed), guestNow))
	add("get_guest_envelope: an unnamed sender, an erased one with no sources, one with no people row", "", base,
		open(dg(w.linkDung), guestNow), open(dg(w.linkErased), guestNow), open(dg(w.linkGhost), guestNow))
	add("get_guest_envelope: digests no link carries", "", base,
		open(nobody, guestNow), open("", guestNow), open(dg(w.linkActive)+"61", guestNow), open(dg(w.linkActive)[2:], guestNow))
	add("get_guest_envelope: links of the money world, hostile event data included", "", base,
		open(dg(m.linkBinh), guestNow), open(dg(m.linkChi), guestNow), open(dg(m.linkOld), guestNow))
	add("get_guest_envelope: an envelope whose sender owes nothing in its version", "CapabilityScopeError", base,
		open(dg(w.linkNoObligations), guestNow))
	add("get_guest_envelope: evidence event data that is a JSON array", "AttributeError",
		join(base, w.badEvent(w.linkActive, "guest_objection.evidence_request", `["khong", "phai"]`)), open(dg(w.linkActive), guestNow))
	add("get_guest_envelope: dispute event data that is JSON null", "AttributeError",
		join(base, w.badEvent(w.linkActive, "guest_objection.wrong_amount", `null`)), open(dg(w.linkActive), guestNow))
	add("get_guest_envelope: objection event data that is a JSON string", "AttributeError",
		join(base, w.badEvent(w.linkOpened, "guest_objection.not_me", `"chuoi"`)), open(dg(w.linkOpened), guestNow))
	add("get_guest_envelope: under a Vietnam session TimeZone", "", vietnam,
		open(dg(w.linkOpened), guestNow), open(dg(w.linkAtExpiry), guestNow), open(dg(w.linkLapsed), guestNow))

	// --- get_payment_report_target -----------------------------------------------
	add("get_payment_report_target: own obligations, another sender's, another version's, missing", "", base,
		target(dg(w.linkOpened), w.oChiAn, guestNow), target(dg(w.linkOpened), w.oChiBinh, guestNow),
		target(dg(w.linkOpened), w.oDungAn, guestNow), target(dg(w.linkOpened), m.obOld, guestNow),
		target(dg(w.linkOpened), m.missingObligation, guestNow))
	add("get_payment_report_target: expiry boundaries and stored states write nothing", "", base,
		target(dg(w.linkAtExpiry), w.oChiAn, guestNow), target(dg(w.linkBeforeExpiry), w.oChiAn, guestNow),
		target(dg(w.linkLapsed), w.oChiAn, guestNow), target(dg(w.linkStoredExpired), w.oChiAn, guestNow),
		target(dg(w.linkRevoked), w.oChiBinh, guestNow), target(dg(w.linkRotated), w.oChiBinh, guestNow))
	add("get_payment_report_target: a digest no link carries", "", base, target(nobody, w.oChiAn, guestNow))
	add("get_payment_report_target: under a Vietnam session TimeZone", "", vietnam,
		target(dg(w.linkActive), w.oChiBinh, guestNow), target(dg(m.linkBinh), m.obBinhAn, guestNow))

	// --- save_payment_report -----------------------------------------------------
	add("save_payment_report: a first report", "", base, report(w.linkActive, w.oChiAn, 45_000, true, 0, newKey(1), guestNow))
	add("save_payment_report: two reports, then the first key again, under a Vietnam session TimeZone", "", vietnam,
		report(w.linkDung, w.oDungAn, 10_000, true, 0, newKey(2), guestNow),
		report(w.linkDung, w.oDungAn, 10_000, true, 1, newKey(3), later(1)),
		report(w.linkDung, w.oDungAn, 10_000, true, 2, newKey(2), later(2)))
	add("save_payment_report: a stored key answers its stored report", "", base,
		report(w.linkOpened, w.oChiAn, 45_000, true, 3, w.keyOpened, guestNow))
	add("save_payment_report: a stored key for another obligation", "IDEMPOTENCY_KEY_REUSED", base,
		report(w.linkOpened, w.oChiBinh, 25_000, true, 3, w.keyOpened, guestNow))
	add("save_payment_report: a stored key through another link", "IDEMPOTENCY_KEY_REUSED", base,
		report(w.linkActive, w.oChiAn, 45_000, true, 0, w.keyOpened, guestNow))
	add("save_payment_report: a stored key with another amount", "IDEMPOTENCY_KEY_REUSED", base,
		report(w.linkOpened, w.oChiAn, 44_999, true, 3, w.keyOpened, guestNow))
	add("save_payment_report: a stored key whose report has no link", "IDEMPOTENCY_KEY_REUSED", base,
		report(w.linkOpened, w.oChiBinh, 25_000, true, 3, w.keyNoLink, guestNow))
	add("save_payment_report: an obligation with no row", "IntegrityError", base,
		report(w.linkActive, m.missingObligation, 1_000, true, 0, newKey(4), guestNow))
	add("save_payment_report: an amount of zero", "IntegrityError", base,
		report(w.linkActive, w.oChiAn, 0, true, 0, newKey(5), guestNow))
	add("save_payment_report: a link with no row", "IntegrityError", base,
		report(fid(kindGuestLink, 0x5f), w.oChiAn, 45_000, true, 0, newKey(6), guestNow))
	add("save_payment_report: an amount at the bigint maximum", "", base,
		report(w.linkActive, w.oChiAn, largestBigint, false, 0, newKey(7), guestNow))

	// --- flow.report_payment -----------------------------------------------------
	add("flow.report_payment: a report through the link", "", base, reportThrough(dg(w.linkActive), w.oChiBinh, newKey(8), guestNow))
	add("flow.report_payment: an obligation outside the link", "", base, reportThrough(dg(w.linkActive), w.oDungAn, newKey(9), guestNow))
	add("flow.report_payment: a digest no link carries", "", base, reportThrough(nobody, w.oChiAn, newKey(9), guestNow))
	add("flow.report_payment: replayed with its key, under a Vietnam session TimeZone", "", vietnam,
		reportThrough(dg(w.linkActive), w.oChiAn, newKey(10), guestNow), reportThrough(dg(w.linkActive), w.oChiAn, newKey(10), later(1)))
	add("flow.report_payment: the key of a report on another obligation", "IDEMPOTENCY_KEY_REUSED", base,
		reportThrough(dg(w.linkOpened), w.oChiBinh, w.keyOpened, guestNow))

	// --- save_guest_objection ----------------------------------------------------
	hostile := "Sai ' \" \\ \n\t <script>alert(1)</script> '; DROP TABLE guest_links; -- 🙂   ẞ %s %(x)s $1 (dữ liệu mẫu)"
	add("save_guest_objection: wrong_amount with hostile text", "", base,
		object(dg(w.linkActive), "wrong_amount", w.oChiAn, hostile, guestNow))
	add("save_guest_objection: not_me revokes an active link", "", base, object(dg(w.linkActive), "not_me", nil, nil, guestNow))
	add("save_guest_objection: not_me on a link revoked at that instant writes only the event", "", base,
		object(dg(w.linkRevokedNow), "not_me", nil, nil, guestNow))
	add("save_guest_objection: not_me on a revoked link moves revoked_at alone", "", base,
		object(dg(w.linkRevoked), "not_me", nil, nil, guestNow))
	add("save_guest_objection: not_me on a stored expired link, then on a rotated one", "", base,
		object(dg(w.linkStoredExpired), "not_me", nil, nil, guestNow), object(dg(w.linkRotated), "not_me", w.oChiAn, "x", later(1)))
	add("save_guest_objection: evidence_request, a kind in another case and an unknown kind never revoke", "", base,
		object(dg(w.linkActive), "evidence_request", w.oChiBinh, nil, guestNow),
		object(dg(w.linkActive), "Not_Me", nil, nil, later(1)), object(dg(w.linkActive), "khac", w.oChiAn, "ly_do", later(2)))
	add("save_guest_objection: not_me twice in one transaction", "", base,
		object(dg(w.linkActive), "not_me", nil, nil, guestNow), object(dg(w.linkActive), "not_me", nil, nil, later(1)))
	add("save_guest_objection: a digest no link carries writes nothing", "", base, object(nobody, "not_me", nil, nil, guestNow))
	add("save_guest_objection: an obligation outside the link is written as given", "", base,
		object(dg(w.linkActive), "wrong_amount", m.obOld, "so_tien_sai", guestNow))
	add("save_guest_objection: a reason with a NUL character", "DataError", base,
		object(dg(w.linkActive), "wrong_amount", w.oChiAn, "a\x00b", guestNow))
	add("save_guest_objection: a kind past the event type column", "DataError", base,
		object(dg(w.linkActive), strings.Repeat("k", 85), nil, nil, guestNow))
	add("save_guest_objection: a reason of ten thousand characters", "", base,
		object(dg(w.linkDung), "wrong_amount", w.oDungAn, strings.Repeat("Ơ", 10_000), guestNow))
	add("save_guest_objection: under a Vietnam session TimeZone", "", vietnam,
		object(dg(w.linkDung), "not_me", nil, nil, "2030-07-10T19:00:00.654321+07:00"))

	// --- the calls each guest route makes, in its order ------------------------------
	add("GET /g/{token}", "", base, open(dg(w.linkOpened), guestNow))
	add("GET /g/{token} of a link nobody holds", "", base, open(nobody, guestNow))
	add("GET /g/{token}/khong-phai-toi", "", base, open(dg(w.linkDung), guestNow))
	add("POST /g/{token}/khong-phai-toi", "", base,
		open(dg(w.linkActive), guestNow), open(dg(w.linkActive), later(1)), object(dg(w.linkActive), "not_me", nil, nil, later(1)))
	add("GET /g/{token} after objecting that the link is not mine", "", base,
		open(dg(w.linkActive), guestNow), open(dg(w.linkActive), guestNow), object(dg(w.linkActive), "not_me", nil, nil, guestNow),
		open(dg(w.linkActive), later(2)))
	add("GET /g/{token}/doi-so-tien without an obligation id", "", base, open(dg(w.linkOpened), guestNow), open(dg(w.linkOpened), guestNow))
	add("GET /g/{token}/doi-so-tien with an obligation id", "", base, open(dg(w.linkActive), guestNow))
	add("POST /g/{token}/doi-so-tien", "", base,
		open(dg(w.linkActive), guestNow), object(dg(w.linkActive), "wrong_amount", w.oChiAn, "so_tien_sai", later(1)),
		open(dg(w.linkActive), later(2)))
	add("POST /g/{token}/doi-so-tien on a link that has just expired", "", base, open(dg(w.linkAtExpiry), guestNow))
	add("POST /g/{token}/xin-cach-tinh", "", base,
		open(dg(w.linkActive), guestNow), object(dg(w.linkActive), "evidence_request", w.oChiAn, nil, later(1)),
		open(dg(w.linkActive), later(2)))
	add("POST /g/{token}/da-chuyen", "", base, reportThrough(dg(w.linkActive), w.oChiAn, newKey(11), guestNow))
	add("POST /g/{token}/da-chuyen replayed with its key", "", base,
		reportThrough(dg(w.linkDung), w.oDungAn, newKey(12), guestNow), reportThrough(dg(w.linkDung), w.oDungAn, newKey(12), later(1)))
	add("POST /g/{token}/da-chuyen on a revoked link", "", base, target(dg(w.linkRevoked), w.oChiAn, guestNow))
	add("POST /g/{token}/da-chuyen at the instant its link expires", "", base,
		target(dg(w.linkAtExpiry), w.oChiAn, guestNow), target(dg(w.linkBeforeExpiry), w.oChiBinh, guestNow))
	add("POST /g/{token}/da-chuyen with the report budget spent", "", base, target(dg(w.linkOpened), w.oChiBinh, guestNow))
	add("POST /g/{token}/da-chuyen for an obligation outside the link", "", base, target(dg(w.linkActive), m.obOld, guestNow))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// guestMethods is every method this port covers; the corpus must reach each
// with at least one normal return.
var guestMethods = []string{
	"get_guest_envelope", "get_payment_report_target", "save_payment_report", "flow.report_payment", "save_guest_objection",
}

// guestBranches are the write branches the corpus must reach in Python, each
// named by the call and the normalised statement only that branch issues.
var guestBranches = []struct{ call, statement string }{
	{"get_guest_envelope", "UPDATE guest_links SET first_opened_at=?::TIMESTAMP WITH TIME ZONE WHERE guest_links.id = ?::UUID"},
	{"get_guest_envelope", "UPDATE guest_links SET status=?, first_opened_at=?::TIMESTAMP WITH TIME ZONE WHERE guest_links.id = ?::UUID"},
	{"get_guest_envelope", "UPDATE guest_links SET status=? WHERE guest_links.id = ?::UUID"},
	{"save_guest_objection", "UPDATE guest_links SET status=?, revoked_at=?::TIMESTAMP WITH TIME ZONE WHERE guest_links.id = ?::UUID"},
	{"save_guest_objection", "UPDATE guest_links SET revoked_at=?::TIMESTAMP WITH TIME ZONE WHERE guest_links.id = ?::UUID"},
	{"save_payment_report", "INSERT INTO payment_reports (id, obligation_id, guest_link_id, reported_by_id, amount_vnd, idempotency_key, reported_at) VALUES (?::UUID, ?::UUID, ?::UUID, ?::UUID, ?::BIGINT, ?::UUID, ?::TIMESTAMP WITH TIME ZONE)"},
}

func TestGuestRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of guests_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_guest_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "guest_oracle_")

	cases, built := guestOracleCases()
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
		"--entrypoint", "python", image, "/oracle/render_guest_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_guest_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
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
	reached := make([]int, len(guestBranches))
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
					statement := normalizeSQL(entry[0].(string))
					statements = append(statements, statement)
					call := c.Steps[j].Call
					if call == "flow.report_payment" {
						// The flow's only write is save_payment_report's.
						call = "save_payment_report"
					}
					for b, branch := range guestBranches {
						if call == branch.call && statement == branch.statement {
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
			compareCase(t, tally, c, normalizeMoneySteps(pySteps, c), normalizeMoneySteps(runGuestGoCase(t, pool, c), c))
		})
	}
	for _, method := range guestMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	for b, branch := range guestBranches {
		if reached[b] == 0 {
			t.Errorf("no case makes python issue, from %s: %s", branch.call, branch.statement)
		}
	}
	t.Logf("guest repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which "+
		"%d table rows, %d generated ids bound, %d write branches reached, %d mismatches", tally.cases, tally.steps,
		tally.results, tally.errors, tally.statements, tally.probeRows, tally.tableRows, tally.generated,
		len(guestBranches), tally.mismatches)
}

// ---------------------------------------------------------------------------
// Two requests at once (Go only)
// ---------------------------------------------------------------------------

// lockNotAvailable reports whether err is PostgreSQL's 55P03, what a
// statement gets when lock_timeout runs out waiting for a row lock.
func lockNotAvailable(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "55P03"
}

func TestGuestLinkLocksSerialiseRequests(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of guests_oracle_postgres_test.go")
	}
	pool, _ := migratedOracleSchema(t, image, "guest_locks_")
	w := newGuestWorld()
	for _, sql := range w.sql {
		if _, err := pool.Exec(bg, sql); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	digest, err := hex.DecodeString(w.digest[w.linkActive])
	if err != nil {
		t.Fatal(err)
	}
	now := argInstant(guestNow)

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

	t.Run("an open envelope holds its link, envelope, version and batch", func(t *testing.T) {
		first := begin(t, "")
		if record, err := (Repository{Q: first}).GetGuestEnvelope(bg, digest, now); err != nil || record == nil {
			t.Fatalf("first open: %v %v", record, err)
		}
		waiting := begin(t, "150ms")
		if _, err := (Repository{Q: waiting}).GetGuestEnvelope(bg, digest, now); !lockNotAvailable(err) {
			t.Fatalf("a second open did not wait on the first: %v", err)
		}
		for _, row := range []struct{ table, id string }{
			{"guest_links", w.linkActive}, {"collection_envelopes", fid(kindEnvelope, 0x61)},
			{"collection_batch_versions", w.v6}, {"collection_batches", fid(kindBatch, 0x66)},
		} {
			other := begin(t, "150ms")
			_, err := other.Exec(bg, `SELECT 1 FROM `+row.table+` WHERE id = $1::UUID FOR UPDATE`, row.id)
			if !lockNotAvailable(err) {
				t.Errorf("%s row %s is not locked by the open: %v", row.table, row.id, err)
			}
		}
		// A report target on the same link waits too.
		other := begin(t, "150ms")
		if _, err := (Repository{Q: other}).GetPaymentReportTarget(bg, digest, w.oChiAn, now); !lockNotAvailable(err) {
			t.Fatalf("a report target did not wait on the open: %v", err)
		}
	})

	t.Run("two reports under one key end as one row", func(t *testing.T) {
		key := fid(kindReceiptKey, 0x9f0)
		first := begin(t, "")
		target, err := (Repository{Q: first}).GetPaymentReportTarget(bg, digest, w.oChiAn, now)
		if err != nil || target == nil {
			t.Fatalf("first target: %v %v", target, err)
		}
		stored, err := (Repository{Q: first}).SavePaymentReport(bg, PaymentReportInput{Target: *target, IdempotencyKey: key, Now: now})
		if err != nil {
			t.Fatal(err)
		}

		type answer struct {
			record PaymentReportRecord
			err    error
		}
		second := begin(t, "")
		done := make(chan answer, 1)
		go func() {
			repo := Repository{Q: second}
			target, err := repo.GetPaymentReportTarget(bg, digest, w.oChiAn, now.Add(time.Second))
			if err != nil || target == nil {
				done <- answer{err: fmt.Errorf("second target: %v %v", target, err)}
				return
			}
			record, err := repo.SavePaymentReport(bg, PaymentReportInput{Target: *target, IdempotencyKey: key, Now: now.Add(time.Second)})
			done <- answer{record, err}
		}()
		select {
		case got := <-done:
			t.Fatalf("the second report did not wait for the first: %+v", got)
		case <-time.After(300 * time.Millisecond):
		}
		if err := first.Commit(bg); err != nil {
			t.Fatal(err)
		}
		got := <-done
		if got.err != nil {
			t.Fatalf("the second report, after the first committed: %v", got.err)
		}
		if got.record.ID != stored.ID {
			t.Fatalf("the second report wrote %s instead of answering %s", got.record.ID, stored.ID)
		}
		if err := second.Commit(bg); err != nil {
			t.Fatal(err)
		}
		var rows int
		if err := pool.QueryRow(bg, `SELECT count(*) FROM payment_reports WHERE idempotency_key = $1::UUID`, key).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 1 {
			t.Fatalf("%d payment reports under one key", rows)
		}
	})
}
