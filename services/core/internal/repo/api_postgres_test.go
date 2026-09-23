//go:build postgres

package repo

// Go-only tests of the pilot methods on real PostgreSQL. They pin the
// behaviours services/api/tests/postgres pins for the Python adapter
// (test_interests_postgres.py, test_group_recap_postgres.py,
// test_group_memories_postgres.py, test_memory_reactions_postgres.py,
// test_place_catalog_postgres.py, test_profile_postgres.py) plus the lock and
// race cases no Python test has. Exact equality with Python is
// oracle_postgres_test.go's job.

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

var bg = context.Background()

func ptr[T any](v T) *T { return &v }

func pgState(err error) (code, constraint string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code, pg.ConstraintName
	}
	return "", ""
}

func mustInstant(t *testing.T, text string) time.Time {
	t.Helper()
	instant, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		t.Fatal(err)
	}
	return instant
}

func day(t *testing.T, text string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", text)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestIsMemberIsAnActiveMembershipThatHasNotEnded(t *testing.T) {
	w := newStdWorld()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	for _, c := range []struct {
		name, context, person string
		want                  bool
	}{
		{"active", w.group, w.owner, true},
		{"invited", w.group, w.invited, false},
		{"left", w.group, w.left, false},
		{"left then rejoined", w.group, w.rejoined, true},
		{"erased account keeps its row", w.group, w.erased, true},
		{"stranger", w.group, w.stranger, false},
		{"member of another group only", w.other, w.mate, false},
		{"group with no memberships", w.empty, w.mate, false},
		{"no such group", w.missingContext, w.owner, false},
	} {
		got, err := repo.IsMember(bg, c.context, c.person)
		if err != nil || got != c.want {
			t.Errorf("%s: IsMember = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestGetPersonReturnsErasedAccountsAndKeepsNullApartFromEmpty(t *testing.T) {
	w := newStdWorld()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}

	owner, err := repo.GetPerson(bg, w.owner)
	if err != nil || owner == nil {
		t.Fatalf("owner: %+v %v", owner, err)
	}
	if owner.Bio != nil || owner.City == nil || *owner.City != "Phố cổ (dữ liệu mẫu)" || owner.BudgetBand == nil ||
		owner.WallCommentPolicy != "readers" || !owner.DiscoverableByPhone || owner.DeletedAt != nil ||
		!owner.CreatedAt.Equal(mustInstant(t, stdCreated)) || owner.CreatedAt.Nanosecond() != 123456*1000 {
		t.Fatalf("owner = %+v", owner)
	}
	blank, _ := repo.GetPerson(bg, w.blank)
	if blank.Bio == nil || *blank.Bio != "" || blank.City == nil || *blank.City != "" {
		t.Fatalf("an empty bio is not a NULL bio: %+v", blank)
	}
	erased, _ := repo.GetPerson(bg, w.erased)
	if erased == nil || erased.DeletedAt == nil || !erased.DeletedAt.Equal(mustInstant(t, "2030-02-01T00:00:00Z")) {
		t.Fatalf("erased account: %+v", erased)
	}
	if missing, err := repo.GetPerson(bg, w.missingPerson); missing != nil || err != nil {
		t.Fatalf("missing person: %+v %v", missing, err)
	}
}

func xactWrites(t *testing.T, q Querier, table string) (ins, upd, del int64) {
	t.Helper()
	err := q.QueryRow(bg, `SELECT n_tup_ins, n_tup_upd, n_tup_del FROM pg_stat_xact_user_tables
		WHERE relid = $1::regclass`, table).Scan(&ins, &upd, &del)
	if err != nil {
		t.Fatal(err)
	}
	return ins, upd, del
}

func TestUpdatePersonProfileWritesOnlyTheColumnsThatChange(t *testing.T) {
	w := newStdWorld()
	tx, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}

	person, err := repo.UpdatePersonProfile(bg, w.owner, ProfileChanges{
		DisplayName:         ptr("Chủ nhóm (dữ liệu mẫu)"),
		Bio:                 SetText(nil),
		City:                SetText(ptr("Phố mới (dữ liệu mẫu)")),
		DiscoverableByPhone: ptr(false),
	})
	if err != nil || person == nil || *person.City != "Phố mới (dữ liệu mẫu)" || person.DiscoverableByPhone || person.Bio != nil {
		t.Fatalf("person = %+v, %v", person, err)
	}
	if len(rec.log) != 2 || !strings.HasSuffix(strings.TrimSpace(rec.log[0]), "FOR UPDATE") ||
		!strings.Contains(rec.log[1], "SET city=$1::VARCHAR, discoverable_by_phone=$2 WHERE") {
		t.Fatalf("statements = %q", rec.log)
	}
	var city string
	var discoverable bool
	if err := tx.QueryRow(bg, `SELECT city, discoverable_by_phone FROM people WHERE id = $1`, w.owner).Scan(&city, &discoverable); err != nil ||
		city != "Phố mới (dữ liệu mẫu)" || discoverable {
		t.Fatalf("stored %q %v %v", city, discoverable, err)
	}

	// Equal values are not changes: no UPDATE statement, no updated tuple.
	rec.log = nil
	_, updBefore, _ := xactWrites(t, tx, "people")
	same, err := repo.UpdatePersonProfile(bg, w.mate, ProfileChanges{
		DisplayName: ptr("Bạn đồng hành (dữ liệu mẫu)"), WallCommentPolicy: ptr("friends"), DiscoverableByPhone: ptr(false),
	})
	if _, updAfter, _ := xactWrites(t, tx, "people"); err != nil || same == nil || len(rec.log) != 1 || updAfter != updBefore {
		t.Fatalf("no-op update: %+v %v statements=%q updated %d -> %d", same, err, rec.log, updBefore, updAfter)
	}

	// "" and NULL are different values in both directions.
	blank, err := repo.UpdatePersonProfile(bg, w.blank, ProfileChanges{Bio: SetText(nil), City: SetText(ptr(""))})
	if err != nil || blank.Bio != nil || blank.City == nil || *blank.City != "" {
		t.Fatalf("blank = %+v %v", blank, err)
	}

	// An erased account is still written; a missing one answers nil after the locking read.
	if erased, err := repo.UpdatePersonProfile(bg, w.erased, ProfileChanges{Bio: SetText(ptr("x"))}); err != nil || erased == nil {
		t.Fatalf("erased: %+v %v", erased, err)
	}
	rec.log = nil
	if missing, err := repo.UpdatePersonProfile(bg, w.missingPerson, ProfileChanges{City: SetText(ptr("x"))}); missing != nil || err != nil || len(rec.log) != 1 {
		t.Fatalf("missing: %+v %v %q", missing, err, rec.log)
	}

	err = savepoint(t, tx, func(repo Repository) error {
		_, err := repo.UpdatePersonProfile(bg, w.owner, ProfileChanges{WallCommentPolicy: ptr("everyone")})
		return err
	})
	if code, constraint := pgState(err); code != "23514" || !strings.Contains(constraint, "wall_comment_policy_known") {
		t.Fatalf("unknown policy: %v (%s %s)", err, code, constraint)
	}
}

// committedPerson writes a person outside any test transaction, for the race
// tests that need two transactions to see the same row, and removes it after.
func committedPerson(t *testing.T, pool *pgxpool.Pool, tags ...string) string {
	t.Helper()
	id, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(bg, insertSQL("people", "id", id, "display_name", "Khoá (dữ liệu mẫu)")); err != nil {
		t.Fatal(err)
	}
	for _, tag := range tags {
		if _, err := pool.Exec(bg, `INSERT INTO person_interests (id, person_id, tag) VALUES (gen_random_uuid(), $1, $2)`, id, tag); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(bg, `DELETE FROM person_interests WHERE person_id = $1`, id)
		_, _ = pool.Exec(bg, `DELETE FROM people WHERE id = $1`, id)
	})
	return id
}

func begin(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(bg) })
	return tx
}

// waitsOnLock reports whether the backend is blocked on a lock within the
// deadline.
func waitsOnLock(t *testing.T, pool *pgxpool.Pool, pid uint32, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		var waiting *string
		if err := pool.QueryRow(bg, `SELECT wait_event_type FROM pg_stat_activity WHERE pid = $1`, int32(pid)).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting != nil && *waiting == "Lock" {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestUpdatePersonProfileHoldsTheRowLockItReadsUnder(t *testing.T) {
	pool := testdb.Pool(t)

	t.Run("a second writer waits even when the first changed nothing", func(t *testing.T) {
		person := committedPerson(t, pool)
		first, second := begin(t, pool), begin(t, pool)
		if _, err := (Repository{Q: first}).UpdatePersonProfile(bg, person, ProfileChanges{}); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := Repository{Q: second}.UpdatePersonProfile(bg, person, ProfileChanges{})
			done <- err
		}()
		if !waitsOnLock(t, pool, second.Conn().PgConn().PID(), 3*time.Second) {
			t.Fatal("the second SELECT ... FOR UPDATE did not wait for the first transaction's row lock")
		}
		if err := first.Rollback(bg); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})

	t.Run("the waiting writer answers with what the first committed", func(t *testing.T) {
		person := committedPerson(t, pool)
		first, second := begin(t, pool), begin(t, pool)
		if _, err := (Repository{Q: first}).UpdatePersonProfile(bg, person, ProfileChanges{Bio: SetText(ptr("một"))}); err != nil {
			t.Fatal(err)
		}
		type answer struct {
			person *Person
			err    error
		}
		done := make(chan answer, 1)
		go func() {
			p, err := Repository{Q: second}.UpdatePersonProfile(bg, person, ProfileChanges{City: SetText(ptr("hai"))})
			done <- answer{p, err}
		}()
		if !waitsOnLock(t, pool, second.Conn().PgConn().PID(), 3*time.Second) {
			t.Fatal("the second writer did not wait")
		}
		if err := first.Commit(bg); err != nil {
			t.Fatal(err)
		}
		got := <-done
		if got.err != nil || got.person == nil || got.person.Bio == nil || *got.person.Bio != "một" || *got.person.City != "hai" {
			t.Fatalf("second writer answered %+v %v; a read under FOR UPDATE sees the committed bio", got.person, got.err)
		}
	})
}

func interestRows(t *testing.T, q Querier, person string) map[string][2]string {
	t.Helper()
	rows, err := q.Query(bg, `SELECT tag, id::text, to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US')
		FROM person_interests WHERE person_id = $1`, person)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][2]string{}
	for rows.Next() {
		var tag, id, created string
		if err := rows.Scan(&tag, &id, &created); err != nil {
			t.Fatal(err)
		}
		out[tag] = [2]string{id, created}
	}
	return out
}

var v4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestSetPersonInterestsReplacesTheSetAndLeavesSurvivorsAlone(t *testing.T) {
	w := newStdWorld()
	w.tastes()
	tx, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	now := time.Date(2030, 8, 27, 12, 0, 0, 123456*1000+789, time.FixedZone("ICT", 7*3600))

	got, err := repo.SetPersonInterests(bg, w.owner, []string{"nightlife", "cafe", "an-uong", "cafe"}, now)
	if err != nil || !slices.Equal(got, []string{"an-uong", "cafe", "nightlife"}) {
		t.Fatalf("SetPersonInterests = %q, %v", got, err)
	}
	rows := interestRows(t, tx, w.owner)
	if len(rows) != 3 || rows["cafe"] != [2]string{fid(kindInterest, 2), "2030-03-02T00:00:00.500000"} {
		t.Fatalf("the surviving row must keep its id and created_at: %v", rows)
	}
	for _, tag := range []string{"an-uong", "nightlife"} {
		if !v4.MatchString(rows[tag][0]) || rows[tag][1] != "2030-08-27T05:00:00.123456" {
			t.Fatalf("new row %s = %v; want a uuid4 and the caller's clock at microseconds", tag, rows[tag])
		}
	}
	// SELECT, one INSERT for both new rows, DELETE karaoke then game (id order), SELECT.
	if len(rec.log) != 5 || !strings.Contains(rec.log[1], "VALUES ($1::UUID") || !strings.Contains(rec.log[1], "), ($5::UUID") ||
		!strings.HasPrefix(rec.log[2], "DELETE") || !strings.HasPrefix(rec.log[3], "DELETE") {
		t.Fatalf("statements = %q", rec.log)
	}
	if mate := interestRows(t, tx, w.mate); len(mate) != 4 {
		t.Fatalf("another person's rows moved: %v", mate)
	}

	rec.log = nil
	if got, err := repo.SetPersonInterests(bg, w.owner, []string{"cafe", "nightlife", "an-uong"}, now); err != nil || len(got) != 3 || len(rec.log) != 2 {
		t.Fatalf("the same set must write nothing: %q %v %q", got, err, rec.log)
	}
	if got, err := repo.SetPersonInterests(bg, w.mate, nil, now); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("clearing: %q %v", got, err)
	}
	if rows := interestRows(t, tx, w.mate); len(rows) != 0 {
		t.Fatalf("clearing left %v", rows)
	}
}

// savepoint runs one refusal inside the test transaction and rolls back to
// before it, so a second fixture transaction never waits on the first's rows.
func savepoint(t *testing.T, tx pgx.Tx, refuse func(Repository) error) error {
	t.Helper()
	sp, err := tx.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sp.Rollback(bg) }()
	return refuse(Repository{Q: sp})
}

func TestSetPersonInterestsRefusalsComeFromTheSchema(t *testing.T) {
	w := newStdWorld()
	tx, _ := seeded(t, w.sql)
	err := savepoint(t, tx, func(repo Repository) error {
		_, err := repo.SetPersonInterests(bg, w.missingPerson, []string{"cafe"}, time.Now())
		return err
	})
	if code, constraint := pgState(err); code != "23503" || constraint != "fk_person_interests_person" {
		t.Fatalf("a taste needs a person: %v", err)
	}
	err = savepoint(t, tx, func(repo Repository) error {
		_, err := repo.SetPersonInterests(bg, w.owner, []string{"   "}, time.Now())
		return err
	})
	if code, constraint := pgState(err); code != "23514" || !strings.Contains(constraint, "person_interest_tag_not_blank") {
		t.Fatalf("a blank tag: %v", err)
	}
}

func TestSetPersonInterestsConcurrentClaimsOfOneTasteLeaveOneRow(t *testing.T) {
	pool := testdb.Pool(t)
	person := committedPerson(t, pool)
	first, second := begin(t, pool), begin(t, pool)
	if _, err := (Repository{Q: first}).SetPersonInterests(bg, person, []string{"cafe"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Repository{Q: second}.SetPersonInterests(bg, person, []string{"cafe"}, time.Now())
		done <- err
	}()
	if !waitsOnLock(t, pool, second.Conn().PgConn().PID(), 3*time.Second) {
		t.Fatal("the second insert did not wait on the unique index")
	}
	if err := first.Commit(bg); err != nil {
		t.Fatal(err)
	}
	if code, constraint := pgState(<-done); code != "23505" || constraint != "uq_person_interests_person_tag" {
		t.Fatalf("second claim: %s %s", code, constraint)
	}
}

func TestSetPersonInterestsToleratesARowAnotherRequestDeletedFirst(t *testing.T) {
	pool := testdb.Pool(t)
	person := committedPerson(t, pool, "cafe")
	first, second := begin(t, pool), begin(t, pool)
	if _, err := (Repository{Q: first}).SetPersonInterests(bg, person, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	type answer struct {
		tags []string
		err  error
	}
	done := make(chan answer, 1)
	go func() {
		tags, err := Repository{Q: second}.SetPersonInterests(bg, person, nil, time.Now())
		done <- answer{tags, err}
	}()
	if !waitsOnLock(t, pool, second.Conn().PgConn().PID(), 3*time.Second) {
		t.Fatal("the second delete did not wait on the first")
	}
	if err := first.Commit(bg); err != nil {
		t.Fatal(err)
	}
	// SQLAlchemy only warns when a DELETE matches 0 rows; so must Go not fail.
	if got := <-done; got.err != nil || len(got.tags) != 0 {
		t.Fatalf("second request: %q %v", got.tags, got.err)
	}
}

func TestInterestsByPersonGroupsInDatabaseOrderAndLeavesOutPeopleWithNone(t *testing.T) {
	w := newStdWorld()
	w.tastes()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	got, err := repo.InterestsByPerson(bg, []string{w.rejoined, w.owner, w.stranger, w.owner, w.missingPerson, w.mate})
	if err != nil || len(got) != 3 || got[0].PersonID != w.owner || got[1].PersonID != w.mate || got[2].PersonID != w.rejoined {
		t.Fatalf("got %+v %v", got, err)
	}
	if !slices.Equal(got[0].Tags, []string{"cafe", "game", "karaoke"}) || len(got[1].Tags) != 4 || !slices.Equal(got[2].Tags, []string{"cafe"}) {
		t.Fatalf("tags %+v", got)
	}
	rec.log = nil
	if empty, err := repo.InterestsByPerson(bg, nil); err != nil || empty == nil || len(empty) != 0 || len(rec.log) != 0 {
		t.Fatalf("no ids must answer without a statement: %+v %v %q", empty, err, rec.log)
	}
}

func TestListPlacesFiltersAtTheDatabaseAndReadsJSONThePythonWay(t *testing.T) {
	var w world
	w.catalogue()
	tx, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}

	all, err := repo.ListPlaces(bg, PlaceFilter{})
	if err != nil || len(all) != 6 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	byID := map[string]Place{}
	var ascii []string
	for _, p := range all {
		byID[p.ID] = p
		if p.ID == "p-b" || p.ID == "p-big" || p.ID == "p-c" {
			ascii = append(ascii, p.ID)
		}
	}
	if !slices.Equal(ascii, []string{"p-b", "p-big", "p-c"}) {
		t.Fatalf("ORDER BY id: %q", ascii)
	}
	b := byID["p-b"]
	if !slices.Equal(b.Kinds, []string{"cà phê", "yên tĩnh"}) || len(b.Traits) != 0 || b.Traits == nil ||
		*b.Rating != 4.5 || *b.RatingCount != 12 || *b.PriceMinVND != 20000 || !*b.OpenNow ||
		!strings.HasPrefix(string(b.GroupFit), `{"a": [1, 2.0, 0, 100, `+hugeJSONInt+`]`) || b.Reviews == nil || b.Activities == nil {
		t.Fatalf("p-b = %+v (group_fit %s)", b, b.GroupFit)
	}
	if a := byID["p-A"]; len(a.Kinds) != 0 || !slices.Equal(a.Traits, []string{"a", "b"}) || a.GroupFit != nil || a.Reviews != nil || a.Rating != nil {
		t.Fatalf("p-A = %+v", a)
	}
	if a := byID["p-a"]; !slices.Equal(a.Kinds, []string{"z", "aa"}) || len(a.Traits) != 0 || *a.DistanceKM != 1e-7 || a.Activities != nil || *a.OpenNow {
		t.Fatalf("p-a = %+v", a)
	}
	if big := byID["p-big"]; *big.PriceMinVND != int64(1)<<53 || *big.PriceMaxVND != int64(1)<<53+1 || big.Lng == nil || *big.Lng != 179.87654 {
		t.Fatalf("p-big = %+v", big)
	}

	check := func(filter PlaceFilter, want ...string) {
		t.Helper()
		got, err := repo.ListPlaces(bg, filter)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, p := range got {
			ids = append(ids, p.ID)
		}
		slices.Sort(ids)
		slices.Sort(want)
		if !slices.Equal(ids, want) {
			t.Fatalf("filter %+v: %q, want %q", filter, ids, want)
		}
	}
	check(PlaceFilter{DestinationID: ptr("d-bien")}, "p-b", "p-a", "p-c")
	check(PlaceFilter{Category: ptr("cafe")}, "p-b", "p-ä")
	check(PlaceFilter{DestinationID: ptr("d-bien"), Category: ptr("cafe")}, "p-b")
	check(PlaceFilter{Category: ptr("")})
	check(PlaceFilter{DestinationID: ptr("khong-co")})

	for raw, want := range map[string]error{`true`: ErrPythonTypeError, `[1]`: ErrUnrepresentable} {
		err := savepoint(t, tx, func(repo Repository) error {
			if _, err := repo.Q.Exec(bg, `UPDATE places SET kinds = '`+raw+`' WHERE id = 'p-c'`); err != nil {
				t.Fatal(err)
			}
			_, err := repo.ListPlaces(bg, PlaceFilter{})
			return err
		})
		if !errors.Is(err, want) {
			t.Fatalf("kinds %s: %v, want %v", raw, err, want)
		}
	}
}

func memoryIDs(page MemoryPage) []string {
	ids := []string{}
	for _, m := range page.Memories {
		ids = append(ids, m.ID)
	}
	return ids
}

func memories(ns ...int) []string {
	ids := []string{}
	for _, n := range ns {
		ids = append(ids, fid(kindMemory, n))
	}
	return ids
}

func TestListMemoriesIsNewestFirstAndPagesBackwardsThroughTies(t *testing.T) {
	w := newStdWorld()
	w.wall()
	_, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	const tie = "2030-09-02T08:30:00.5Z"

	page, err := repo.ListMemories(bg, w.group, MemoryQuery{Limit: 10, ViewerID: ptr(w.owner)})
	if err != nil || page.HasMore || !slices.Equal(memoryIDs(page), memories(7, 6, 5, 4, 3, 2, 1)) {
		t.Fatalf("wall: %q has_more=%v %v", memoryIDs(page), page.HasMore, err)
	}
	three, seven := page.Memories[4], page.Memories[0]
	if three.ReactionCount != 3 || three.CommentCount != 2 || !three.ViewerHasReacted {
		t.Fatalf("hearts and comments must not multiply: %+v", three)
	}
	if seven.ReactionCount != 1 || seven.CommentCount != 0 || seven.ViewerHasReacted || *seven.Caption != "Ảnh 📸 (dữ liệu mẫu)" || seven.Kind != "photo" {
		t.Fatalf("m7 = %+v", seven)
	}
	if first := page.Memories[6]; first.PlaceID != nil || first.Lat != nil || first.ReactionCount != 0 {
		t.Fatalf("a photo without a place: %+v", first)
	}
	if c := page.Memories[1]; c.Kind != "checkin" || *c.Lat != 10.7702 || *c.PlaceID != "p-cho" {
		t.Fatalf("check-in: %+v", c)
	}

	for _, c := range []struct {
		name    string
		query   MemoryQuery
		want    []string
		hasMore bool
	}{
		{"first page", MemoryQuery{Limit: 3}, memories(7, 6, 5), true},
		{"before the middle of a tie", MemoryQuery{Limit: 10, Before: &MemoryCursor{mustInstant(t, tie), fid(kindMemory, 3)}}, memories(2, 1), false},
		{"before the top of a tie", MemoryQuery{Limit: 2, Before: &MemoryCursor{mustInstant(t, tie), fid(kindMemory, 4)}}, memories(3, 2), true},
		{"photos", MemoryQuery{Limit: 10, Kind: ptr("photo")}, memories(7, 5, 1), false},
		{"check-ins", MemoryQuery{Limit: 10, Kind: ptr("checkin")}, memories(6, 4, 3, 2), false},
		{"one place", MemoryQuery{Limit: 10, PlaceID: ptr("p-cho")}, memories(6, 5), false},
		{"the empty place id is a filter, not None", MemoryQuery{Limit: 10, PlaceID: ptr("")}, memories(), false},
		{"limit equal to the count", MemoryQuery{Limit: 7}, memories(7, 6, 5, 4, 3, 2, 1), false},
		{"limit one below the count", MemoryQuery{Limit: 6}, memories(7, 6, 5, 4, 3, 2), true},
		{"limit zero", MemoryQuery{Limit: 0}, memories(), true},
		{"limit minus one", MemoryQuery{Limit: -1}, memories(), true},
	} {
		got, err := repo.ListMemories(bg, w.group, c.query)
		if err != nil || !slices.Equal(memoryIDs(got), c.want) || got.HasMore != c.hasMore {
			t.Errorf("%s: %q has_more=%v %v; want %q %v", c.name, memoryIDs(got), got.HasMore, err, c.want, c.hasMore)
		}
	}

	rec.log = nil
	other, err := repo.ListMemories(bg, w.other, MemoryQuery{Limit: 10})
	if err != nil || !slices.Equal(memoryIDs(other), memories(8)) || other.Memories[0].ReactionCount != 1 || len(rec.log) != 3 {
		t.Fatalf("another group's wall: %+v %v; without a viewer there is no third read: %q", other, err, rec.log)
	}
	rec.log = nil
	if _, err := repo.ListMemories(bg, w.group, MemoryQuery{Limit: 10, Kind: ptr("video")}); !errors.Is(err, ErrUnknownMemoryKind) || len(rec.log) != 0 {
		t.Fatalf("unknown kind: %v %q", err, rec.log)
	}
	if _, err := repo.ListMemories(bg, w.empty, MemoryQuery{Limit: -2}); err == nil {
		t.Fatal("a limit below -1 reaches PostgreSQL as a negative LIMIT")
	}
}

type recapRow struct {
	outing           int
	inProgress       bool
	total, exp, mems int64
}

func recapRows(got []RecapOuting) []recapRow {
	out := []recapRow{}
	for _, r := range got {
		n := 0
		for i := 1; i < 16; i++ {
			if fid(kindOuting, i) == r.Outing.ID {
				n = i
			}
		}
		out = append(out, recapRow{n, r.InProgress, r.SplitTotalVND, r.ExpenseCount, r.MemoryCount})
	}
	return out
}

func TestGroupRecapClaimsMoneyAndMemoriesOnVietnamsDays(t *testing.T) {
	for _, zone := range []string{"", "Asia/Ho_Chi_Minh", "Etc/GMT+10"} {
		t.Run("session TimeZone "+zone, func(t *testing.T) {
			w := newStdWorld()
			w.trips()
			sql := w.sql
			if zone != "" {
				sql = append([]string{"SET LOCAL TimeZone = '" + zone + "'"}, sql...)
			}
			_, rec := seeded(t, sql)
			repo := Repository{Q: rec}

			got, err := repo.GroupRecap(bg, w.group, day(t, "2030-08-27"))
			if err != nil {
				t.Fatal(err)
			}
			want := []recapRow{
				{6, true, 340000, 1, 1},
				{5, true, 340000, 1, 1},
				{4, false, 70000, 1, 1},
				{1, false, 90000, 1, 1},
				{2, false, 690000, 2, 1},
				{3, false, 701000, 3, 2},
			}
			if !slices.Equal(recapRows(got), want) {
				t.Fatalf("recap = %+v\nwant   %+v", recapRows(got), want)
			}
			long := got[5].Outing
			if len(long.Stops) != 3 || long.Stops[0].Position != 0 || long.Stops[1].MeetingLat == nil || *long.Stops[1].MeetingLat != 10.5 ||
				long.Stops[0].Day == nil || long.Stops[0].Day.Format("2006-01-02") != "2030-08-21" || long.Stops[0].TimeLocked ||
				!long.Stops[2].TimeLocked || long.Stops[2].DurationMinutes != nil {
				t.Fatalf("stops = %+v", long.Stops)
			}
			if len(long.ItineraryDays) != 2 || string(long.ItineraryDays[1]) != `{"ngay": 2}` || long.ItineraryVersion != 2 ||
				long.TimelineRevision != 2 || long.BudgetPerPersonVND != 1500000 || long.StartsOn.Format("2006-01-02") != "2030-08-21" {
				t.Fatalf("outing = %+v", long)
			}
			if got[0].Outing.Stops == nil || len(got[0].Outing.ItineraryDays) != 0 {
				t.Fatalf("a trip with no stops and no itinerary: %+v", got[0].Outing)
			}
			// Statements: outings, money, memories, one stops read per outing.
			if len(rec.log) != 3+6 {
				t.Fatalf("%d statements", len(rec.log))
			}

			onTheLastDay, err := repo.GroupRecap(bg, w.group, day(t, "2030-08-23"))
			if err != nil || !slices.Equal(recapRows(onTheLastDay), []recapRow{{1, true, 90000, 1, 1}, {2, true, 690000, 2, 1}, {3, true, 701000, 3, 2}}) {
				t.Fatalf("on the last day: %+v %v", recapRows(onTheLastDay), err)
			}
			rec.log = nil
			if before, err := repo.GroupRecap(bg, w.group, day(t, "2030-08-20")); err != nil || before == nil || len(before) != 0 || len(rec.log) != 1 {
				t.Fatalf("before every trip: %+v %v %q", before, err, rec.log)
			}
			if empty, err := repo.GroupRecap(bg, w.empty, day(t, "2030-08-27")); err != nil || len(empty) != 0 {
				t.Fatalf("empty group: %+v %v", empty, err)
			}
			if other, err := repo.GroupRecap(bg, w.other, day(t, "2030-08-27")); err != nil || !slices.Equal(recapRows(other), []recapRow{{8, false, 999000, 1, 1}}) {
				t.Fatalf("other group: %+v %v", recapRows(other), err)
			}
		})
	}
}

func TestCreateReportStoresTheCallersClockAndRefusesAReporterWithNoRow(t *testing.T) {
	w := newStdWorld()
	tx, rec := seeded(t, w.sql)
	repo := Repository{Q: rec}
	now := time.Date(2030, 8, 27, 12, 0, 0, 70000999, time.UTC)

	report, err := repo.CreateReport(bg, ReportInput{ReporterID: w.owner, TargetType: "person", TargetID: w.missingPerson, Reason: "spam", Now: now})
	if err != nil || !v4.MatchString(report.ID) || !report.CreatedAt.Equal(now.Truncate(time.Microsecond)) {
		t.Fatalf("report = %+v %v", report, err)
	}
	var reporter, target, reason string
	var note *string
	var created time.Time
	if err := tx.QueryRow(bg, `SELECT reporter_id::text, target_id::text, reason, note, created_at FROM reports WHERE id = $1`, report.ID).
		Scan(&reporter, &target, &reason, &note, &created); err != nil || reporter != w.owner || target != w.missingPerson ||
		reason != "spam" || note != nil || !created.Equal(report.CreatedAt) {
		t.Fatalf("stored %s %s %s %v %v %v", reporter, target, reason, note, created, err)
	}
	if _, err := repo.CreateReport(bg, ReportInput{ReporterID: w.erased, TargetType: "story", TargetID: w.owner, Reason: "other",
		Note: ptr(strings.Repeat("😀", 500)), Now: now}); err != nil {
		t.Fatalf("an erased reporter with 500 characters: %v", err)
	}

	err = savepoint(t, tx, func(repo Repository) error {
		_, err := repo.CreateReport(bg, ReportInput{ReporterID: w.missingPerson, TargetType: "person", TargetID: w.owner, Reason: "spam", Now: now})
		return err
	})
	if code, constraint := pgState(err); code != "23503" || constraint != "fk_reports_reporter" {
		t.Fatalf("reporter without a people row: %v", err)
	}
	err = savepoint(t, tx, func(repo Repository) error {
		_, err := repo.CreateReport(bg, ReportInput{ReporterID: w.owner, TargetType: "person", TargetID: w.owner, Reason: "spam",
			Note: ptr(strings.Repeat("😀", 501)), Now: now})
		return err
	})
	if code, constraint := pgState(err); code != "23514" || !strings.Contains(constraint, "report_note_length") {
		t.Fatalf("501 characters: %v", err)
	}
}
