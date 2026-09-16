package routes

import (
	"testing"
	"time"

	"mobile/services/core/internal/domain/peoplesteps"
	"mobile/services/core/internal/pyjson"
)

// The stacks compare the bytes of every people route, but only for the states
// a scenario reaches. These two tests hold what a scenario cannot: the field
// order of the response models with their optional halves both ways, and the
// changes dict as repository keyword arguments, including the shape a cleared
// bio has.

func compact(t *testing.T, value pyjson.Value) string {
	t.Helper()
	encoded, err := pyjson.Compact(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestTheWireCarriesEveryFieldInTheModelsOrder(t *testing.T) {
	instant := time.Date(2026, 9, 16, 4, 5, 6, 700_000_000, time.UTC)
	bio, city, author, band := "Thích cà phê", "Đà Nẵng", "1e1cb2c0-0f8e-4e7a-9b5a-6a2f0c9d1e7b", "vua"
	profile := peoplesteps.Profile{
		ID: "3f2a6b1c-9d4e-4a5b-8c7d-0e1f2a3b4c5d", DisplayName: "Chủ hồ sơ", Bio: &bio, City: &city,
		CreatedAt:    instant,
		Counts:       peoplesteps.ProfileCounts{Friends: 2, Contexts: 1, Outings: 0, PlacesCheckedIn: 0, Memories: 3},
		LoginMethods: []string{}, Interests: []string{"cafe", "outdoor"}, BudgetBand: &band,
		WallCommentPolicy: "readers", DiscoverableByPhone: true,
	}
	want := `{"id":"3f2a6b1c-9d4e-4a5b-8c7d-0e1f2a3b4c5d","display_name":"Chủ hồ sơ","bio":"Thích cà phê",` +
		`"city":"Đà Nẵng","created_at":"2026-09-16T04:05:06.700000Z","counts":{"friends":2,"contexts":1,` +
		`"outings":0,"places_checked_in":0,"memories":3},"login_methods":[],"interests":["cafe","outdoor"],` +
		`"budget_band":"vua","wall_comment_policy":"readers","discoverable_by_phone":true}`
	if got := compact(t, wireProfile(profile)); got != want {
		t.Fatalf("profile\n got %s\nwant %s", got, want)
	}

	empty := peoplesteps.ContextSummary{
		ID: "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d", DisplayName: "Nhóm A", MemberCount: 2, MyRole: "admin",
		MyState: "invited", MembershipID: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e", UnreadCount: 0,
		Theme: "mac-dinh", Kind: "group",
	}
	want = `{"id":"9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d","display_name":"Nhóm A","member_count":2,` +
		`"my_role":"admin","my_state":"invited","membership_id":"2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",` +
		`"joined_at":null,"last_message":null,"unread_count":0,"theme":"mac-dinh","kind":"group",` +
		`"counterpart":null,"unavailable":false}`
	if got := compact(t, wireContextSummary(empty)); got != want {
		t.Fatalf("empty summary\n got %s\nwant %s", got, want)
	}

	full := empty
	full.DisplayName = "Bạn"
	full.Kind = "pair"
	full.MyState = "active"
	full.JoinedAt = &instant
	full.LastMessage = &peoplesteps.LastMessage{
		ID: "7c6d5e4f-3a2b-4c1d-8e9f-0a1b2c3d4e5f", Kind: "text", Preview: "Đi chứ",
		AuthorID: &author, AuthorDisplayName: &full.DisplayName, CreatedAt: instant,
	}
	full.UnreadCount = 1
	full.Counterpart = &peoplesteps.Counterpart{ID: author, DisplayName: "Bạn"}
	full.Unavailable = true
	want = `{"id":"9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d","display_name":"Bạn","member_count":2,` +
		`"my_role":"admin","my_state":"active","membership_id":"2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",` +
		`"joined_at":"2026-09-16T04:05:06.700000Z","last_message":{"id":"7c6d5e4f-3a2b-4c1d-8e9f-0a1b2c3d4e5f",` +
		`"kind":"text","preview":"Đi chứ","author_id":"1e1cb2c0-0f8e-4e7a-9b5a-6a2f0c9d1e7b",` +
		`"author_display_name":"Bạn","created_at":"2026-09-16T04:05:06.700000Z"},"unread_count":1,` +
		`"theme":"mac-dinh","kind":"pair","counterpart":{"id":"1e1cb2c0-0f8e-4e7a-9b5a-6a2f0c9d1e7b",` +
		`"display_name":"Bạn"},"unavailable":true}`
	if got := compact(t, wireContextSummary(full)); got != want {
		t.Fatalf("pair summary\n got %s\nwant %s", got, want)
	}

	person := peoplesteps.PublicPerson{ID: profile.ID, DisplayName: "Chủ hồ sơ", CreatedAt: instant, Relation: "friend"}
	want = `{"id":"3f2a6b1c-9d4e-4a5b-8c7d-0e1f2a3b4c5d","display_name":"Chủ hồ sơ","bio":null,"city":null,` +
		`"created_at":"2026-09-16T04:05:06.700000Z","relation":"friend"}`
	if got := compact(t, wirePublicPerson(person)); got != want {
		t.Fatalf("public person\n got %s\nwant %s", got, want)
	}

	want = `{"place_id":"p-tiem-ca-phe","name":"Tiệm cà phê","category":"cafe",` +
		`"saved_at":"2026-09-16T04:05:06.700000Z"}`
	saved := peoplesteps.SavedPlaceSummary{PlaceID: "p-tiem-ca-phe", Name: "Tiệm cà phê", Category: "cafe", SavedAt: instant}
	if got := compact(t, wireSavedPlace(saved)); got != want {
		t.Fatalf("saved place\n got %s\nwant %s", got, want)
	}

	want = `{"person_id":"1e1cb2c0-0f8e-4e7a-9b5a-6a2f0c9d1e7b","display_name":"Bạn",` +
		`"blocked_at":"2026-09-16T04:05:06.700000Z"}`
	blocked := peoplesteps.BlockedPerson{PersonID: author, DisplayName: "Bạn", BlockedAt: instant}
	if got := compact(t, wireBlockedPerson(blocked)); got != want {
		t.Fatalf("blocked person\n got %s\nwant %s", got, want)
	}

	want = `{"person_id":"1e1cb2c0-0f8e-4e7a-9b5a-6a2f0c9d1e7b","state":"blocked"}`
	if got := compact(t, wireBlockState(peoplesteps.BlockState{PersonID: author, State: "blocked"})); got != want {
		t.Fatalf("block state\n got %s\nwant %s", got, want)
	}
}

func TestTheChangesDictBecomesTheRepositoryArguments(t *testing.T) {
	changes, err := peopleProfileChanges([]peoplesteps.Change{
		{Field: "display_name", Value: "Tên mới"},
		{Field: "bio", Value: nil},
		{Field: "city", Value: "Huế"},
		{Field: "wall_comment_policy", Value: "nobody"},
		{Field: "discoverable_by_phone", Value: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case changes.DisplayName == nil || *changes.DisplayName != "Tên mới":
		t.Fatalf("display_name = %v", changes.DisplayName)
	case !changes.Bio.Set || changes.Bio.Value != nil:
		t.Fatalf("a cleared bio is %+v, not a present key holding None", changes.Bio)
	case !changes.City.Set || changes.City.Value == nil || *changes.City.Value != "Huế":
		t.Fatalf("city = %+v", changes.City)
	case changes.WallCommentPolicy == nil || *changes.WallCommentPolicy != "nobody":
		t.Fatalf("wall_comment_policy = %v", changes.WallCommentPolicy)
	case changes.DiscoverableByPhone == nil || *changes.DiscoverableByPhone:
		t.Fatalf("discoverable_by_phone = %v", changes.DiscoverableByPhone)
	case changes.BudgetBand.Set:
		t.Fatal("a key the dict does not carry became a column")
	}

	// A field nobody sends means the port and the service disagree, which is
	// a failure, not a column quietly dropped.
	if _, err := peopleProfileChanges([]peoplesteps.Change{{Field: "notify_prefs", Value: "{}"}}); err == nil {
		t.Fatal("an unknown change was accepted")
	}
	if _, err := peopleProfileChanges([]peoplesteps.Change{{Field: "display_name", Value: 7}}); err == nil {
		t.Fatal("a display_name that is not a string was accepted")
	}
	if _, err := peopleProfileChanges([]peoplesteps.Change{{Field: "bio", Value: true}}); err == nil {
		t.Fatal("a bio that is not a string or None was accepted")
	}
}
