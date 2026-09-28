package achievement

import "testing"

func TestRouteChoicesUseEvidenceAndEarnedPrerequisites(t *testing.T) {
	// A wrong fixed-order unlock or a city-name comparison must fail here.
	facts := Facts{
		Checkins: 3, DistinctPlacesInOneOuting: 2, DistinctDestinations: 2,
		OutingsWithDestinations: 2, PhotoDays: 3, StoryDays: 3,
		SharedOutings: 2, LargestSharedParty: 3, RepeatedCompanionOutings: 2,
		PhotoAtCheckedPlace: true, PhotoInSharedGroup: true,
	}
	choices := Candidates(facts, map[string]bool{})
	if len(choices) != 6 {
		t.Fatalf("initial candidates = %d, want 6 first-route endings", len(choices))
	}
	for _, choice := range choices {
		if !choice.Eligible {
			t.Fatalf("%s should be eligible from evidence", choice.ID)
		}
		if choice.RouteID == "nga_re" || choice.ID == "whole_journey" {
			t.Fatalf("advanced ending offered before prerequisite: %s", choice.ID)
		}
	}
	earned := map[string]bool{"many_turns": true, "photos_remain": true, "again_together": true}
	mapPath := Candidates(facts, earned, "ky_niem", "dau_chan")
	sharedPath := Candidates(facts, earned, "dong_hanh", "ky_niem")
	if !containsEligible(mapPath, "map_becomes_page") || containsEligible(mapPath, "shared_memory") {
		t.Fatal("footsteps then memories should open only the map branch")
	}
	if !containsEligible(sharedPath, "shared_memory") || containsEligible(sharedPath, "map_becomes_page") {
		t.Fatal("memories then companions should open only the shared branch")
	}
	if containsEligible(choices, "whole_journey") {
		t.Fatal("true ending opened before branch ending")
	}
	earned["map_becomes_page"] = true
	if !containsEligible(Candidates(facts, earned, "ky_niem", "dau_chan"), "whole_journey") {
		t.Fatal("true ending should open with one ending per first route and a branch ending")
	}
}

func TestChapterChangesWithChosenSequenceAndCanBeReplayed(t *testing.T) {
	earned := map[string]bool{"many_turns": true, "photos_remain": true, "again_together": true}
	facts := Facts{PhotoAtCheckedPlace: true, PhotoInSharedGroup: true}
	mapPath := Chapters(facts, earned, "ky_niem", "dau_chan")
	sharedPath := Chapters(facts, earned, "dong_hanh", "ky_niem")
	if len(mapPath) == 0 || len(sharedPath) == 0 || mapPath[0].ID == sharedPath[0].ID {
		t.Fatalf("choice sequence did not change next chapter: %v / %v", mapPath, sharedPath)
	}
	if mapPath[0].TargetEndingID != "map_becomes_page" || sharedPath[0].TargetEndingID != "shared_memory" {
		t.Fatalf("unexpected branch chapters: %v / %v", mapPath, sharedPath)
	}
	// Selecting a branch and then replaying the first two routes must recover
	// the other branch, without permanently locking a personal ending.
	replayed := Candidates(facts, earned, "dong_hanh", "ky_niem", "nga_re", "ky_niem", "dau_chan")
	if !containsEligible(replayed, "shared_memory") {
		t.Fatal("replaying another route sequence did not open its branch")
	}
	if containsEligible(Candidates(facts, earned, "dau_chan", "dong_hanh"), "map_becomes_page") {
		t.Fatal("unordered route collection opened an ordered branch")
	}
}

func TestRouteProgressUsesDistinctOutingsAndDays(t *testing.T) {
	// Counting repeated taps on one day or one outing would falsely unlock.
	facts := Facts{Checkins: 2, DistinctDestinations: 2, OutingsWithDestinations: 1, PhotoDays: 1, SharedOutings: 1, LargestSharedParty: 3}
	for _, id := range []string{"open_map", "photos_remain", "full_house"} {
		if containsEligible(Candidates(facts, map[string]bool{}), id) {
			t.Fatalf("%s unlocked from insufficient distinct evidence", id)
		}
	}
}

func TestFirstStoryBadgeNeedsOneValidStoryNotThreeDays(t *testing.T) {
	// The opening stamp must not wait for the three-day ending.
	got := OpeningBadges(Facts{StoryCount: 1, StoryDays: 1})
	if len(got) != 1 || got[0] != "first_story" {
		t.Fatalf("opening badges = %v", got)
	}
}

func TestCreativeFeatureIDsFollowEarnedEndings(t *testing.T) {
	choices := Candidates(Facts{}, map[string]bool{})
	for _, choice := range choices {
		if choice.ID == "open_map" && choice.FeatureID != "map_album" {
			t.Fatalf("open_map feature = %q", choice.FeatureID)
		}
	}
}

func TestPriorChoicesPersonalizeTheNextRouteWithoutForcingIt(t *testing.T) {
	facts := Facts{}
	fromFootsteps := SuggestedChoices(facts, map[string]bool{}, "", "dau_chan", "dau_chan")
	fromMemories := SuggestedChoices(facts, map[string]bool{}, "", "ky_niem", "ky_niem")
	if fromFootsteps[0].RouteID == fromMemories[0].RouteID {
		t.Fatalf("different prior choices gave the same next route: %s", fromFootsteps[0].RouteID)
	}
	if len(fromFootsteps) != 6 || len(fromMemories) != 6 {
		t.Fatal("a prior choice must never erase another path")
	}
}

func containsEligible(choices []Choice, id string) bool {
	for _, choice := range choices {
		if choice.ID == id {
			return choice.Eligible
		}
	}
	return false
}
