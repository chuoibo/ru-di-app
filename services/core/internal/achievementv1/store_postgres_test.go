//go:build postgres

package achievementv1

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/domain/achievement"
	"mobile/services/core/internal/testdb"
)

func TestFactsAndPermanentRouteCreditInPostgres(t *testing.T) {
	// A wrong join that counts the planned headcount as actual companions, or
	// counts one outing twice, must fail this real PostgreSQL scenario.
	ctx := context.Background()
	tx := testdb.Tx(t)
	schema := fmt.Sprintf("achievement_test_%d", time.Now().UnixNano())
	if _, err := tx.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	ddl := []string{
		`CREATE TABLE people(id uuid PRIMARY KEY)`,
		`CREATE TABLE places(id text PRIMARY KEY,destination_id text NOT NULL)`,
		`CREATE TABLE outings(id uuid PRIMARY KEY,context_id uuid NOT NULL)`,
		`CREATE TABLE outing_stops(id uuid PRIMARY KEY,outing_id uuid NOT NULL,place_id text)`,
		`CREATE TABLE outing_stop_checkins(id uuid PRIMARY KEY,stop_id uuid NOT NULL,person_id uuid NOT NULL,created_at timestamptz NOT NULL)`,
		`CREATE TABLE memories(id uuid PRIMARY KEY,context_id uuid NOT NULL,author_id uuid NOT NULL,kind text NOT NULL,place_id text,created_at timestamptz NOT NULL)`,
		`CREATE TABLE posts(id uuid PRIMARY KEY,author_id uuid NOT NULL,body text NOT NULL,created_at timestamptz NOT NULL)`,
	}
	for _, query := range ddl {
		if _, err := tx.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := MigrateTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE profile_media_jobs(id text PRIMARY KEY,person_id uuid NOT NULL,credit_source text NOT NULL,status text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	const a = "11111111-1111-4111-8111-111111111111"
	const b = "22222222-2222-4222-8222-222222222222"
	const c = "33333333-3333-4333-8333-333333333333"
	const group = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const trip1 = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const trip2 = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	const stop1 = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	const stop2 = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	const stop3 = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	for _, q := range []string{
		`INSERT INTO people VALUES('` + a + `'),('` + b + `'),('` + c + `')`,
		`INSERT INTO places VALUES('p1','d-a'),('p2','d-a'),('p3','d-b')`,
		`INSERT INTO outings VALUES('` + trip1 + `','` + group + `'),('` + trip2 + `','` + group + `')`,
		`INSERT INTO outing_stops VALUES('` + stop1 + `','` + trip1 + `','p1'),('` + stop2 + `','` + trip1 + `','p2'),('` + stop3 + `','` + trip2 + `','p3')`,
		`INSERT INTO outing_stop_checkins VALUES
		 ('11111111-2222-4111-8111-111111111111','` + stop1 + `','` + a + `','2026-09-01'),
		 ('11111111-3333-4111-8111-111111111111','` + stop2 + `','` + a + `','2026-09-01'),
		 ('11111111-4444-4111-8111-111111111111','` + stop3 + `','` + a + `','2026-09-02'),
		 ('22222222-1111-4222-8222-222222222222','` + stop1 + `','` + b + `','2026-09-01'),
		 ('22222222-3333-4222-8222-222222222222','` + stop3 + `','` + b + `','2026-09-02'),
		 ('33333333-1111-4333-8333-333333333333','` + stop1 + `','` + c + `','2026-09-01')`,
		`INSERT INTO memories VALUES('44444444-1111-4444-8444-444444444444','` + group + `','` + a + `','photo','p1','2026-09-01')`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	store := Store{Q: tx}
	facts, err := store.Facts(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if facts.DistinctPlacesInOneOuting != 2 || facts.DistinctDestinations != 2 || facts.OutingsWithDestinations != 2 || facts.SharedOutings != 2 || facts.LargestSharedParty != 3 || facts.RepeatedCompanionOutings != 2 || !facts.PhotoAtCheckedPlace || !facts.PhotoInSharedGroup {
		t.Fatalf("facts from two trips = %+v", facts)
	}
	if !containsID(achievement.OpeningBadges(facts), "first_together") {
		t.Fatal("earlier actor did not earn shared check-in")
	}
	for i := 0; i < 2; i++ {
		if err := store.GrantCredit(ctx, a, "route:dau_chan"); err != nil {
			t.Fatal(err)
		}
	}
	credits, err := store.Credits(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if credits != 1 {
		t.Fatalf("credit grants = %d, want one", credits)
	}
	if err := store.GrantCredit(ctx, a, "true_end"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO profile_media_jobs(id,person_id,credit_source,status) VALUES('synthetic-ready',$1,'route:dau_chan','ready')`, a); err != nil {
		t.Fatal(err)
	}
	summary, err := store.CreditSummary(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Granted != 2 || summary.Used != 1 || summary.Available != 1 {
		t.Fatalf("credit summary = %+v", summary)
	}
	empty, err := store.CreditSummary(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Granted != 0 || empty.Used != 0 || empty.Available != 0 {
		t.Fatalf("empty credit summary = %+v", empty)
	}
	firstRun, err := store.SelectRun(ctx, a, "dau_chan", "many_turns")
	if err != nil {
		t.Fatal(err)
	}
	secondRun, err := store.SelectRun(ctx, a, "ky_niem", "photos_remain")
	if err != nil {
		t.Fatal(err)
	}
	if firstRun.ID == secondRun.ID {
		t.Fatal("replayed choice reused prior run ID")
	}
	active, err := store.ActiveRun(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != secondRun.ID {
		t.Fatalf("active run = %+v", active)
	}
	history, err := store.RouteHistory(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0] != "ky_niem" || history[1] != "dau_chan" {
		t.Fatalf("choice history = %v", history)
	}
	for _, id := range []string{"many_turns", "photos_remain", "again_together"} {
		if err := store.GrantBadge(ctx, a, id); err != nil {
			t.Fatal(err)
		}
	}
	firstChapter, err := snapshot(ctx, store, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstChapter.Chapters) != 1 || firstChapter.Chapters[0].TargetEndingID != "map_becomes_page" {
		t.Fatalf("map chapter = %+v", firstChapter.Chapters)
	}
	if _, ok := choiceByID(firstChapter.Candidates, "shared_memory"); ok {
		t.Fatal("companion branch appeared before its ordered choice")
	}
	if _, err := store.SelectRun(ctx, a, "dong_hanh", "again_together"); err != nil {
		t.Fatal(err)
	}
	secondChapter, err := snapshot(ctx, store, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondChapter.Chapters) != 1 || secondChapter.Chapters[0].TargetEndingID != "shared_memory" {
		t.Fatalf("companion chapter = %+v", secondChapter.Chapters)
	}
	if _, ok := choiceByID(secondChapter.Candidates, "map_becomes_page"); ok {
		t.Fatal("old unearned map branch stayed open after changing the choice sequence")
	}
}

func TestPublicBadgeEndpointRespectsRelationAndBlockInPostgres(t *testing.T) {
	ctx := context.Background()
	tx := testdb.Tx(t)
	schema := fmt.Sprintf("achievement_acl_%d", time.Now().UnixNano())
	if _, err := tx.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE TABLE people(id uuid PRIMARY KEY,deleted_at timestamptz)`,
		`CREATE TABLE friend_requests(id uuid PRIMARY KEY,requester_id uuid NOT NULL,addressee_id uuid NOT NULL,state text NOT NULL)`,
		`CREATE TABLE memberships(id uuid PRIMARY KEY,context_id uuid NOT NULL,person_id uuid NOT NULL,state text NOT NULL)`,
	} {
		if _, err := tx.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := MigrateTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	const a = "11111111-1111-4111-8111-111111111111"
	const b = "22222222-2222-4222-8222-222222222222"
	const c = "33333333-3333-4333-8333-333333333333"
	const group = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for _, query := range []string{
		`INSERT INTO people(id) VALUES('` + a + `'),('` + b + `'),('` + c + `')`,
		`INSERT INTO memberships VALUES('aaaaaaaa-1111-4111-8111-111111111111','` + group + `','` + a + `','active'),('aaaaaaaa-2222-4222-8222-222222222222','` + group + `','` + b + `','active')`,
	} {
		if _, err := tx.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	store := Store{Q: tx}
	if err := store.GrantBadge(ctx, b, "first_checkin"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDisplay(ctx, b, []string{"first_checkin"}); err != nil {
		t.Fatal(err)
	}
	h := New(nil, "dev")
	request := func(actor string) (int, any, error) {
		r, err := http.NewRequest(http.MethodGet, "/people/"+b+"/achievements", nil)
		if err != nil {
			t.Fatal(err)
		}
		return h.handle(ctx, r, store, actor)
	}
	status, body, err := request(a)
	if err != nil || status != 200 || len(body.(map[string]any)["badges"].([]EarnedBadge)) != 1 {
		t.Fatalf("shared group display status=%d body=%v err=%v", status, body, err)
	}
	_, _, err = request(c)
	if denied, ok := err.(*routeError); !ok || denied.Code != "person_not_visible" {
		t.Fatalf("unrelated actor saw badges: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO friend_requests VALUES('bbbbbbbb-1111-4111-8111-111111111111',$1::uuid,$2::uuid,'blocked')`, b, a); err != nil {
		t.Fatal(err)
	}
	_, _, err = request(a)
	if denied, ok := err.(*routeError); !ok || denied.Code != "person_not_visible" {
		t.Fatalf("blocked groupmate saw badges: %v", err)
	}
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
