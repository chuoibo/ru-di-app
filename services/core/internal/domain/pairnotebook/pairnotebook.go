// Package pairnotebook is services/api/app/domain/pair_notebook.py: the
// two-person notebook's consent ladder, its cycle vocabulary, and what closing
// it costs.
//
// Pure functions over values. The clock is always a parameter. Python's dicts
// become small structs holding exactly the keys the Python functions read, with
// a nil pointer for a key whose value is None (the service never passes a key
// that is absent, and never a value of another type).
//
// One dependency is not pure in the allowlist's sense: the close preview's
// revision is a SHA-256 digest (hashlib in Python), and crypto/sha256 is not a
// domain import. XemTruocDongSo therefore takes the digest function as a
// parameter; callers pass sha256.Sum256.
//
// The preview reads pair_paper.hieu_luc, OPEN_STATES and PLAN_STATES, as the
// Python module imports them: from the pairpaper package.
package pairnotebook

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mobile/services/core/internal/domain/pairpaper"
)

// OfferWindow is HAN_DE_NGHI: how long an unanswered offer stands.
const OfferWindow = 7 * 24 * time.Hour

// CycleStates is CYCLE_STATES. Each call returns a fresh slice.
func CycleStates() []string { return []string{"pending", "active", "closed"} }

// ConsentPurposes is CONSENT_PURPOSES, the ladder from tier 2 upward, in order.
func ConsentPurposes() []string { return []string{"lap_so", "bat_doi", "doc_chat"} }

// ConstraintKinds is CONSTRAINT_KINDS.
func ConstraintKinds() []string { return []string{"khong_an_duoc", "dung"} }

// ladder is CONSENT_PURPOSES as the functions below read it.
var ladder = [...]string{"lap_so", "bat_doi", "doc_chat"}

// NotebookError is NotebookError: a refusal carrying the wire code.
type NotebookError struct {
	Code string
}

func (e *NotebookError) Error() string { return e.Code }

// HanDeNghi is han_de_nghi: when an offer made at now lapses.
//
// Python adds a timedelta to an aware datetime in wall-clock terms of its own
// tzinfo; AddDate does the same in now's Location, which for UTC and fixed
// offsets (all the service ever passes) is also the absolute instant plus a
// week.
func HanDeNghi(now time.Time) time.Time { return now.AddDate(0, 0, 7) }

// Consent is one row of `_consents_as_dicts`: the keys `_live` and
// `granted_by` read.
type Consent struct {
	PersonID          string
	Purpose           string
	GrantedAt         *time.Time
	RevokedAt         *time.Time
	ProposalExpiresAt *time.Time
}

// live is _live: granted, not revoked, and the PROPOSAL not lapsed. A nil now
// is Python's `now=None`, which skips the expiry test entirely.
func live(consent Consent, now *time.Time) bool {
	if consent.GrantedAt == nil {
		return false
	}
	if consent.RevokedAt != nil {
		return false
	}
	if consent.ProposalExpiresAt != nil && now != nil && !now.Before(*consent.ProposalExpiresAt) {
		return false
	}
	return true
}

func onLadder(purpose string) bool {
	for _, rung := range ladder {
		if rung == purpose {
			return true
		}
	}
	return false
}

// GrantedBy is granted_by: what one person has granted and not taken back.
//
// Python returns a frozenset; Go returns its members in ladder order, never
// nil, so equal sets are equal slices.
func GrantedBy(consents []Consent, personID string, now *time.Time) []string {
	held := map[string]bool{}
	for _, row := range consents {
		if row.PersonID == personID && onLadder(row.Purpose) && live(row, now) {
			held[row.Purpose] = true
		}
	}
	out := []string{}
	for _, rung := range ladder {
		if held[rung] {
			out = append(out, rung)
		}
	}
	return out
}

// GrantedPurposes is granted_purposes: the purposes every one of the distinct
// participants has granted. Fewer than two distinct people unlock nothing, and
// duplicates in participants count once, as Python's set does.
func GrantedPurposes(consents []Consent, participants []string, now *time.Time) []string {
	people := map[string]bool{}
	for _, person := range participants {
		people[person] = true
	}
	if len(people) < 2 {
		return []string{}
	}
	count := map[string]int{}
	for person := range people {
		for _, purpose := range GrantedBy(consents, person, now) {
			count[purpose]++
		}
	}
	out := []string{}
	for _, rung := range ladder {
		if count[rung] == len(people) {
			out = append(out, rung)
		}
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// CanBatDoi is can_bat_doi: both people have said yes to «Một đôi».
func CanBatDoi(consents []Consent, participants []string, now *time.Time) bool {
	return contains(GrantedPurposes(consents, participants, now), "bat_doi")
}

// ChatConsentActive is chat_consent_active: may Nếp read this pair's messages,
// at now. Python requires the keyword; the service always passes its clock.
func ChatConsentActive(consents []Consent, participants []string, now time.Time) bool {
	return contains(GrantedPurposes(consents, participants, &now), "doc_chat")
}

// Proposal is one row of `_proposals_as_dicts`.
type Proposal struct {
	ID          string
	CompletedAt *time.Time
	ExpiresAt   *time.Time
}

// DangCho is dang_cho: nobody completed the offer and it has not lapsed.
func DangCho(proposal Proposal, now time.Time) bool {
	if proposal.CompletedAt != nil {
		return false
	}
	if proposal.ExpiresAt != nil && !now.Before(*proposal.ExpiresAt) {
		return false
	}
	return true
}

// Paper is `_paper_dict`, as pair_paper reads it; the preview reads its id,
// state and deadline.
type Paper = pairpaper.Paper

// ClosePreview is xem_truoc_dong_so's dict, keys in the same order.
type ClosePreview struct {
	Revision    string
	SoNhapBo    int
	SoToHuy     int
	SoToKhoa    int
	SoDeNghiHuy int
}

// revision is _revision: the first 16 hex digits of the SHA-256 of the sorted
// material joined by newlines. Python sorts str by code point; for the valid
// UTF-8 every Python str arrives as, byte order is the same order.
func revision(material []string, sum256 func([]byte) [32]byte) string {
	rows := append([]string(nil), material...)
	sort.Strings(rows)
	digest := sum256([]byte(strings.Join(rows, "\n")))
	return fmt.Sprintf("%x", digest[:8])
}

// XemTruocDongSo is xem_truoc_dong_so: what closing would do, counted, with a
// revision that pins exactly the rows counted. sum256 is sha256.Sum256.
func XemTruocDongSo(papers []Paper, proposals []Proposal, now time.Time, sum256 func([]byte) [32]byte) ClosePreview {
	var out ClosePreview
	material := make([]string, 0, len(papers)+len(proposals))
	for _, paper := range papers {
		state := pairpaper.HieuLuc(paper, now)
		material = append(material, paper.ID+":"+state)
		switch {
		case state == "nhap":
			out.SoNhapBo++
		case pairpaper.IsOpen(state):
			out.SoToHuy++
		case pairpaper.IsPlan(state):
			out.SoToKhoa++
		}
	}
	for _, proposal := range proposals {
		if DangCho(proposal, now) {
			material = append(material, "dn:"+proposal.ID)
			out.SoDeNghiHuy++
		}
	}
	out.Revision = revision(material, sum256)
	return out
}
