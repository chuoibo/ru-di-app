package achievementv1

import (
	"context"
	"net/http"
	"testing"

	"mobile/services/core/internal/domain/achievement"
)

func TestDevModeAuthenticatesSyntheticActorHeaders(t *testing.T) {
	h := New(nil, "dev")
	header := http.Header{}
	header.Set("X-Actor-ID", "11111111-1111-4111-8111-111111111111")
	header.Set("X-Actor-Roles", "member")
	id, err := h.actor(context.Background(), nil, header)
	if err != nil || id != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("dev actor = %q, %v", id, err)
	}
}

func TestMatchesOnlyAchievementExtensionPaths(t *testing.T) {
	for _, path := range []string{"/me/achievement-routes", "/me/achievement-runs", "/me/achievement-runs/11111111-1111-4111-8111-111111111111/finish", "/me/achievement-suggestions", "/me/achievement-display", "/people/11111111-1111-4111-8111-111111111111/achievements"} {
		if !Matches(path) {
			t.Fatalf("extension did not claim %s", path)
		}
	}
	// QA UI-160: the "seen" mark is the extension's too.
	if !Matches("/me/achievement-seen") {
		t.Fatal("extension did not claim /me/achievement-seen")
	}
	for _, path := range []string{"/me/achievement-routes/other", "/people/me/posts", "/people/11111111-1111-4111-8111-111111111111/posts"} {
		if Matches(path) {
			t.Fatalf("extension swallowed existing route %s", path)
		}
	}
}

func TestAISuggestionIDsMustBeRealCandidates(t *testing.T) {
	choices := achievement.Candidates(achievement.Facts{}, map[string]bool{})
	got := validatedSuggestionIDs([]string{"whole_journey", "open_map", "open_map", "invented", "photos_remain"}, choices)
	if len(got) != 2 || got[0] != "open_map" || got[1] != "photos_remain" {
		t.Fatalf("grounded suggestion IDs = %v", got)
	}
}

func TestAINarrationRejectsLinksAndLongModelOutput(t *testing.T) {
	if safeNarration("Xem https://outside.invalid") != "" {
		t.Fatal("AI narration leaked a link")
	}
	if safeNarration("Đường bạn chọn đã mở thêm một trang.") == "" {
		t.Fatal("safe short line was lost")
	}
}

func TestSuggestionSummaryExcludesUndisclosedDetail(t *testing.T) {
	facts := achievement.Facts{Checkins: 3, DistinctDestinations: 2, PhotoDays: 1, StoryDays: 1, SharedOutings: 2, LargestSharedParty: 8, PhotoAtCheckedPlace: true}
	summary := minimalSuggestionSummary(facts)
	if len(summary) != 5 || summary["checkins"] != 3 || summary["shared_outings"] != 2 {
		t.Fatalf("summary = %v", summary)
	}
	if _, ok := summary["largest_shared_party"]; ok {
		t.Fatal("undisclosed party size sent to AI")
	}
}
