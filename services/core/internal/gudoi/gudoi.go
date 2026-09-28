// Package gudoi decides whose taste Rủ Đi AI may read in a couple's chat
// (ADR-0048, amending ADR-0034 §2.1/§3 for chat).
//
// A couple is a chat of two whose two people both said yes to «Một đôi»
// (pairnotebook.CanBatDoi). Inside it, a person's taste may reach the model
// only when that person turned `chia_gu` on under the wording that names the
// chat: a consent granted at or after MocChat. A consent granted before it
// promised only «Nếp dùng gu khi phác tờ» and does not cover the chat; the
// person re-consents by turning the switch off and on again, which files a
// new consent row (the notebook's own flows, no new route, no schema change).
//
// Pure functions over values; the clock is a parameter. Nothing here reads a
// name, a budget or the notebook's shared constraints: only consent rows,
// the cycle's participants and the closed taste vocabulary's ids.
package gudoi

import (
	"slices"
	"time"

	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/domain/pairnotebook"
)

// mocChat is ADR-0048 §3's effective instant: midnight starting 29 September 2026 in Vietnam
// (UTC+7). It must not be earlier than the release that shows the new
// wording; moving it later only narrows who is covered (fail closed).
var mocChat = time.Date(2026, 9, 28, 17, 0, 0, 0, time.UTC)

// MocChat is the instant from which a `chia_gu` consent covers the chat.
func MocChat() time.Time { return mocChat }

// ChoChat reports whether a consent granted at grantedAt covers the chat.
func ChoChat(grantedAt *time.Time) bool {
	return grantedAt != nil && !grantedAt.Before(mocChat)
}

// NguoiDuocDung are the participants whose taste the chat may use at now,
// in participants order: none unless the two are a couple; then each person
// with a live `chia_gu` consent granted at or after MocChat. Liveness is the
// notebook's own (pairnotebook.GrantedBy over only the rows that qualify),
// so a revoked, lapsed or older consent never counts.
func NguoiDuocDung(consents []pairnotebook.Consent, participants []string, now time.Time) []string {
	out := []string{}
	if !pairnotebook.CanBatDoi(consents, participants, &now) {
		return out
	}
	moi := make([]pairnotebook.Consent, 0, len(consents))
	for _, c := range consents {
		if c.Purpose == "chia_gu" && ChoChat(c.GrantedAt) {
			moi = append(moi, c)
		}
	}
	for _, person := range participants {
		if slices.Contains(out, person) {
			continue
		}
		if slices.Contains(pairnotebook.GrantedBy(moi, person, &now), "chia_gu") {
			out = append(out, person)
		}
	}
	return out
}

// TrangThai is where one person's own switch stands for the chat.
type TrangThai string

const (
	// Tat: `chia_gu` is off.
	Tat TrangThai = "tat"
	// Bat: on, under the wording that names the chat.
	Bat TrangThai = "bat"
	// CanBatLai: on, but granted before MocChat: it covers the notebook and
	// not the chat until the person turns it on again.
	CanBatLai TrangThai = "can_bat_lai"
)

// TrangThaiCua is person's switch as the app words it (TrangThai).
func TrangThaiCua(consents []pairnotebook.Consent, person string, now time.Time) TrangThai {
	if !slices.Contains(pairnotebook.GrantedBy(consents, person, &now), "chia_gu") {
		return Tat
	}
	moi := make([]pairnotebook.Consent, 0, len(consents))
	for _, c := range consents {
		if c.Purpose == "chia_gu" && ChoChat(c.GrantedAt) {
			moi = append(moi, c)
		}
	}
	if slices.Contains(pairnotebook.GrantedBy(moi, person, &now), "chia_gu") {
		return Bat
	}
	return CanBatLai
}

// Gu is one person's taste as the chat may read it: ids of the closed
// vocabulary, in its order.
type Gu struct {
	NguoiID string
	The     []string
}

// GuChoChat lays out the taste of the people in duocDung: each person's tags
// in vocabulary order, unknown tags left out, a person with no known tag
// left out. Nobody else's rows are looked at, whatever guTheoNguoi holds.
func GuChoChat(duocDung []string, guTheoNguoi map[string][]string) []Gu {
	out := []Gu{}
	for _, person := range duocDung {
		co := map[string]bool{}
		for _, tag := range guTheoNguoi[person] {
			co[tag] = true
		}
		g := Gu{NguoiID: person, The: []string{}}
		for _, id := range interests.InterestIDs() {
			if co[id] {
				g.The = append(g.The, id)
			}
		}
		if len(g.The) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// Chung is what everyone in gs likes, in vocabulary order; empty unless at
// least two people are in gs.
func Chung(gs []Gu) []string {
	out := []string{}
	if len(gs) < 2 {
		return out
	}
	for _, id := range interests.InterestIDs() {
		all := true
		for _, g := range gs {
			all = all && slices.Contains(g.The, id)
		}
		if all {
			out = append(out, id)
		}
	}
	return out
}

// Nhan is the vocabulary label of tag id ("" when unknown).
func Nhan(id string) string {
	for _, t := range interests.InterestTags() {
		if t.ID == id {
			return t.Label
		}
	}
	return ""
}
