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

	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/domain/pairpaper"
)

// OfferWindow is HAN_DE_NGHI: how long an unanswered offer stands.
const OfferWindow = 7 * 24 * time.Hour

// CycleStates is CYCLE_STATES. Each call returns a fresh slice.
func CycleStates() []string { return []string{"pending", "active", "closed"} }

// ConsentPurposes is CONSENT_PURPOSES, the ladder from tier 2 upward, in order.
// `chia_gu` (ADR-0034) is each person's own switch, not a rung both climb.
func ConsentPurposes() []string { return []string{"lap_so", "bat_doi", "doc_chat", "chia_gu"} }

// PerPersonPurposes is PER_PERSON_PURPOSES: what one person decides alone.
func PerPersonPurposes() []string { return []string{"chia_gu"} }

// ConstraintKinds is CONSTRAINT_KINDS.
func ConstraintKinds() []string { return []string{"khong_an_duoc", "dung"} }

// ladder is CONSENT_PURPOSES as the functions below read it.
var ladder = [...]string{"lap_so", "bat_doi", "doc_chat", "chia_gu"}

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

// Consent is one row of `_consents_as_dicts`: the keys `_live`, `granted_by`
// and `granted_purposes` read. ProposalID "" is Python's absent key: every such
// row belongs to one shared group, the old per-purpose reading.
type Consent struct {
	PersonID            string
	Purpose             string
	GrantedAt           *time.Time
	RevokedAt           *time.Time
	ProposalExpiresAt   *time.Time
	ProposalID          string
	ProposalCompletedAt *time.Time
}

// live is _live: granted, not revoked, and the PROPOSAL not lapsed -- unless it
// was completed: an agreed rung stands until revoked, it does not switch itself
// off when its offer window ends (QA 23/09). A nil now is Python's `now=None`,
// which skips the expiry test entirely.
func live(consent Consent, now *time.Time) bool {
	if consent.GrantedAt == nil {
		return false
	}
	if consent.RevokedAt != nil {
		return false
	}
	if consent.ProposalCompletedAt != nil {
		return true
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
// participants has granted ON ONE PROPOSAL (ADR-0027: «cả hai chấp nhận cùng đề
// nghị»). Two people who each filed their own proposal have each agreed only
// with themselves; counted per purpose that read as «both» with nothing
// completed, and the same count gates Nếp reading the chat (QA 23/09). Fewer
// than two distinct people unlock nothing, and duplicates in participants count
// once, as Python's set does.
func GrantedPurposes(consents []Consent, participants []string, now *time.Time) []string {
	people := map[string]bool{}
	for _, person := range participants {
		people[person] = true
	}
	if len(people) < 2 {
		return []string{}
	}
	type key struct{ purpose, proposal string }
	agreed := map[key]map[string]bool{}
	for _, row := range consents {
		if !onLadder(row.Purpose) || !live(row, now) || !people[row.PersonID] {
			continue
		}
		k := key{row.Purpose, row.ProposalID}
		if agreed[k] == nil {
			agreed[k] = map[string]bool{}
		}
		agreed[k][row.PersonID] = true
	}
	held := map[string]bool{}
	for k, who := range agreed {
		if len(who) == len(people) {
			held[k.purpose] = true
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

// Taste is gu_hai_nguoi's dict (ADR-0034 §2.1–2.2).
type Taste struct {
	MineShared   bool
	TheirsShared bool
	Theirs       []string
	Common       []string
}

// GuHaiNguoi is gu_hai_nguoi: what of the two tastes `me` may see. Nil outside
// «Một đôi»; the other's tags only if they turned `chia_gu` on; the common tags
// only if both did. Tags in vocabulary order, unknown ones left out.
func GuHaiNguoi(consents []Consent, participants []string, me string, guTheoNguoi map[string][]string, now time.Time) *Taste {
	if !CanBatDoi(consents, participants, &now) {
		return nil
	}
	var other *string
	for _, person := range participants {
		if person != me {
			other = &person
			break
		}
	}
	mineShared := contains(GrantedBy(consents, me, &now), "chia_gu")
	theirsShared := other != nil && contains(GrantedBy(consents, *other, &now), "chia_gu")
	theirTags := map[string]bool{}
	if theirsShared {
		for _, tag := range guTheoNguoi[*other] {
			theirTags[tag] = true
		}
	}
	myTags := map[string]bool{}
	for _, tag := range guTheoNguoi[me] {
		myTags[tag] = true
	}
	taste := &Taste{MineShared: mineShared, TheirsShared: theirsShared, Theirs: []string{}, Common: []string{}}
	for _, tag := range interests.InterestIDs() {
		if !theirTags[tag] {
			continue
		}
		taste.Theirs = append(taste.Theirs, tag)
		if mineShared && myTags[tag] {
			taste.Common = append(taste.Common, tag)
		}
	}
	return taste
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

// ToTinHieu is `_paper_signals`: what nguoi_lo_suy reads of one sheet. A nil
// CycleID is Python's None; a Version with a nil SentBy was never sent.
type ToTinHieu struct {
	CycleID   *string
	Versions  []PhienBanTinHieu
	Responses []TraLoiTinHieu
}

// PhienBanTinHieu is one version as nguoi_lo_suy reads it.
type PhienBanTinHieu struct {
	Version    int
	AuthorType string
	SentBy     *string
}

// TraLoiTinHieu is one response as nguoi_lo_suy reads it.
type TraLoiTinHieu struct {
	PersonID string
	Kind     string
}

// Diem is one participant's score.
type Diem struct {
	PersonID string
	Score    int
}

// NguoiLo is nguoi_lo_suy's and vai_tuan's dict: NguoiLo lists who leads,
// Cach is "suy" or "chon" ("" from nguoi_lo_suy alone).
type NguoiLo struct {
	NguoiLo []string
	Cach    string
	Diem    []Diem
}

// NguoiLoSuy is nguoi_lo_suy (ADR-0034 §2.3–2.4): who tends to take the lead,
// read only from this cycle's sheets; a tie or nothing yet goes to whoever
// opened the notebook, then to the first participant.
func NguoiLoSuy(participants []string, toGiay []ToTinHieu, cycleID string, nguoiLapSo *string) NguoiLo {
	people := []string{}
	seen := map[string]bool{}
	for _, p := range participants {
		if !seen[p] {
			seen[p] = true
			people = append(people, p)
		}
	}
	diem := map[string]int{}
	for _, to := range toGiay {
		if to.CycleID == nil || *to.CycleID != cycleID {
			continue
		}
		for _, v := range to.Versions {
			if v.Version != 1 {
				continue
			}
			if v.AuthorType == "human" && v.SentBy != nil && seen[*v.SentBy] {
				diem[*v.SentBy] += 2
			}
			break
		}
		for _, tl := range to.Responses {
			if tl.Kind == "de_nghi_sua" && seen[tl.PersonID] {
				diem[tl.PersonID]++
			}
		}
	}
	if len(people) == 0 {
		return NguoiLo{NguoiLo: []string{}, Diem: []Diem{}}
	}
	cao := diem[people[0]]
	for _, p := range people {
		if diem[p] > cao {
			cao = diem[p]
		}
	}
	dauBang := []string{}
	for _, p := range people {
		if diem[p] == cao {
			dauBang = append(dauBang, p)
		}
	}
	lo := dauBang[0]
	if len(dauBang) > 1 && nguoiLapSo != nil && contains(dauBang, *nguoiLapSo) {
		lo = *nguoiLapSo
	}
	out := NguoiLo{NguoiLo: []string{lo}, Diem: []Diem{}}
	for _, p := range people {
		out.Diem = append(out.Diem, Diem{PersonID: p, Score: diem[p]})
	}
	return out
}

// VaiTuan is vai_tuan: the week's stored choice, or the inference. chon nil
// is Python's None (nothing chosen); chon pointing at nil is «cả hai».
func VaiTuan(suy NguoiLo, chon **string, participants []string) NguoiLo {
	if chon == nil {
		return NguoiLo{NguoiLo: append([]string{}, suy.NguoiLo...), Cach: "suy", Diem: suy.Diem}
	}
	if *chon == nil {
		people := []string{}
		seen := map[string]bool{}
		for _, p := range participants {
			if !seen[p] {
				seen[p] = true
				people = append(people, p)
			}
		}
		return NguoiLo{NguoiLo: people, Cach: "chon", Diem: suy.Diem}
	}
	return NguoiLo{NguoiLo: []string{**chon}, Cach: "chon", Diem: suy.Diem}
}
