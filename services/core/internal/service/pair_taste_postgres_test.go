//go:build postgres

package service

import (
	"slices"
	"testing"
	"time"

	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

// TestAPairSumsOnlyTheTasteOfThoseWhoShare is ADR-0034 §2.1–2.2 on real rows.
//
// In a pair a «sum» is two readings of one person each: subtract your own
// answers and the rest is the other person's. So both having consented to
// chat reading opens nobody's taste; only `chia_gu`, each person's own switch,
// does -- and only theirs.
func TestAPairSumsOnlyTheTasteOfThoseWhoShare(t *testing.T) {
	const granted, far = "2030-01-05T00:00:00Z", "2030-12-01T00:00:00Z"
	w := newConsentWorld()
	tag := 0
	interest := func(person, name string) {
		tag++
		w.insert("person_interests", "id", fixtureID(0x94, tag), "person_id", person, "tag", name,
			"created_at", created)
	}
	interest(w.a, "cafe")
	interest(w.a, "outdoor")
	interest(w.b, "cafe")
	interest(w.b, "an-uong")

	// ab: both granted doc_chat already (newConsentWorld). Add «Một đôi», and
	// `chia_gu` from a alone.
	cycle := fixtureID(0x91, 1) // the live cycle newConsentWorld opened first, for ab
	w.offer(cycle, "bat_doi", far, w.a, granted, nil, w.b, granted, nil)
	w.offer(cycle, "chia_gu", far, w.a, granted, nil)

	tx := testdb.Tx(t)
	seed(t, tx, w.sql)
	store := repo.Repository{Q: tx}
	now := w.deadline.Add(-4 * 24 * time.Hour)

	sharers, err := PairTasteSharers(bg, store, w.ab, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(sharers) != 1 || !sharers[w.a] {
		t.Fatalf("sharers = %v, want only a", sharers)
	}
	if got, err := PairTasteSharers(bg, store, w.group, now); err != nil || got != nil {
		t.Fatalf("a group is not a pair: %v, %v", got, err)
	}

	profile, err := GroupTaste(bg, store, w.ab, now)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Known() || profile.PeopleAnswered != 1 {
		t.Fatalf("profile = %+v, want a's answers alone", profile)
	}
	if !slices.Contains(profile.Interests, "outdoor") || slices.Contains(profile.Interests, "an-uong") {
		t.Errorf("interests = %v: a's tags in, b's out", profile.Interests)
	}

	// The group path is untouched: both members summed, no consent asked.
	groupProfile, err := GroupTaste(bg, store, w.group, now)
	if err != nil {
		t.Fatal(err)
	}
	if groupProfile.PeopleAnswered != 2 {
		t.Errorf("group answered = %d, want 2", groupProfile.PeopleAnswered)
	}

}

// TestDocChatAloneOpensNoPairTaste: both consented to chat reading, nobody
// turned `chia_gu` on -- unknown, not an empty sum.
func TestDocChatAloneOpensNoPairTaste(t *testing.T) {
	w := newConsentWorld()
	tx := testdb.Tx(t)
	seed(t, tx, w.sql)
	now := w.deadline.Add(-4 * 24 * time.Hour)
	profile, err := GroupTaste(bg, repo.Repository{Q: tx}, w.ab, now)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Known() {
		t.Errorf("doc_chat alone opened a pair's taste: %+v", profile)
	}
}
