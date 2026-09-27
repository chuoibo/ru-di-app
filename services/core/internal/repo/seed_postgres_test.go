//go:build postgres

package repo

// Fixtures written as literal SQL, so the very same statements seed the Go
// tests and, through render_repo_oracle.py, the Python side of the oracle.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mobile/services/core/internal/testdb"
)

// raw is a SQL fragment written into a statement as is.
type raw string

func lit(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case raw:
		return string(x)
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return "'" + strconv.FormatFloat(x, 'g', -1, 64) + "'::float8"
	}
	panic(fmt.Sprintf("lit: %T", v))
}

// insertSQL renders one INSERT; a column named again later overrides the
// earlier value, so helpers can take defaults first and extras after.
func insertSQL(table string, pairs ...any) string {
	cols := make([]string, 0, len(pairs)/2)
	vals := make([]string, 0, len(pairs)/2)
	at := map[string]int{}
	for i := 0; i < len(pairs); i += 2 {
		column := pairs[i].(string)
		if j, seen := at[column]; seen {
			vals[j] = lit(pairs[i+1])
			continue
		}
		at[column] = len(cols)
		cols = append(cols, column)
		vals = append(vals, lit(pairs[i+1]))
	}
	return "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(vals, ", ") + ")"
}

// Fixture id kinds. fid(kind, n) sorts by (kind, n), so a test can seed rows
// in one order and know the order their ids sort in.
const (
	kindPerson     = 0xa0
	kindReport     = 0xab
	kindMembership = 0xb0
	kindInterest   = 0xba
	kindContext    = 0xc0
	kindMemory     = 0xd0
	kindReaction   = 0xda
	kindComment    = 0xdc
	kindOuting     = 0xe0
	kindStop       = 0xe8
	kindExpense    = 0xf0
	kindVersion    = 0xf4
	kindAllocation = 0xf8
)

func fid(kind, n int) string {
	return fmt.Sprintf("%02x%06x-aaaa-4aaa-8aaa-aaaaaaaaaaaa", kind, n)
}

const stdCreated = "2030-01-01T00:00:00.123456Z"

type world struct{ sql []string }

func (w *world) insert(table string, pairs ...any) {
	w.sql = append(w.sql, insertSQL(table, pairs...))
}

func (w *world) person(n int, name string, extra ...any) string {
	id := fid(kindPerson, n)
	w.insert("people", append([]any{"id", id, "display_name", name, "created_at", stdCreated}, extra...)...)
	return id
}

func (w *world) context(n int, creator string) string {
	id := fid(kindContext, n)
	w.insert("contexts", "id", id, "display_name", fmt.Sprintf("Nhóm %d (dữ liệu mẫu)", n),
		"created_by_id", creator, "created_at", stdCreated)
	return id
}

func (w *world) membership(n int, contextID, personID, state string) {
	pairs := []any{"id", fid(kindMembership, n), "context_id", contextID, "person_id", personID,
		"state", state, "created_at", stdCreated}
	switch state {
	case "active":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z")
	case "left":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z", "left_at", "2030-01-03T00:00:00Z")
	}
	w.insert("memberships", pairs...)
}

func (w *world) photo(n int, contextID, author, createdAt string, extra ...any) string {
	id := fid(kindMemory, n)
	w.insert("memories", append([]any{"id", id, "context_id", contextID, "author_id", author, "kind", "photo",
		"image_url", fmt.Sprintf("/static/mau/anh-%d.jpg", n), "created_at", createdAt}, extra...)...)
	return id
}

func (w *world) checkin(n int, contextID, author, createdAt, placeID, placeName string, lat, lng float64) string {
	id := fid(kindMemory, n)
	w.insert("memories", "id", id, "context_id", contextID, "author_id", author, "kind", "checkin",
		"place_id", placeID, "place_name", placeName, "lat", lat, "lng", lng, "created_at", createdAt)
	return id
}

type expenseVersion struct {
	total      int64
	occurredAt string
	shares     []int64
}

func (w *world) expense(n int, contextID, payer string, versions ...expenseVersion) {
	id := fid(kindExpense, n)
	w.insert("expenses", "id", id, "context_id", contextID, "created_at", stdCreated)
	for v, version := range versions {
		versionID := fid(kindVersion, n*16+v)
		pairs := []any{"id", versionID, "expense_id", id, "version_number", v + 1,
			"recorded_by_id", payer, "paid_by_id", payer, "verification_scope", "totals_only",
			"subtotal_amount_vnd", version.total, "total_amount_vnd", version.total,
			"occurred_at", version.occurredAt, "created_at", stdCreated}
		if v > 0 {
			pairs = append(pairs, "previous_version_number", v)
		}
		w.insert("expense_versions", pairs...)
		for s, amount := range version.shares {
			w.insert("confirmed_allocations", "id", fid(kindAllocation, (n*16+v)*16+s),
				"expense_version_id", versionID, "participant_id", fid(kindPerson, 0x40+s),
				"amount_vnd", amount, "confirmed_by_id", payer, "confirmed_at", stdCreated)
		}
	}
}

// stdWorld is the roster every scenario starts from: one group with a member
// in each membership state, a second group, an empty group, and people whose
// optional columns are NULL, empty or set.
type stdWorld struct {
	world
	owner, mate, invited, left, rejoined, erased, stranger, blank string
	group, other, empty                                           string
	missingPerson, missingContext                                 string
}

func newStdWorld() *stdWorld {
	w := &stdWorld{}
	w.owner = w.person(1, "Chủ nhóm (dữ liệu mẫu)", "city", "Phố cổ (dữ liệu mẫu)", "budget_band", "vua-phai")
	w.mate = w.person(2, "Bạn đồng hành (dữ liệu mẫu)", "bio", "Thích cà phê ☕ (dữ liệu mẫu)",
		"wall_comment_policy", "friends", "discoverable_by_phone", false)
	w.invited = w.person(3, "Người được mời (dữ liệu mẫu)")
	w.left = w.person(4, "Người đã rời (dữ liệu mẫu)")
	w.rejoined = w.person(5, "Người quay lại (dữ liệu mẫu)")
	w.erased = w.person(6, "Tài khoản đã xoá (dữ liệu mẫu)", "deleted_at", "2030-02-01T00:00:00Z")
	w.stranger = w.person(7, "Người lạ (dữ liệu mẫu)")
	w.blank = w.person(8, "Trống (dữ liệu mẫu)", "bio", "", "city", "")
	w.group = w.context(1, w.owner)
	w.other = w.context(2, w.owner)
	w.empty = w.context(3, w.mate)
	w.membership(1, w.group, w.owner, "active")
	w.membership(2, w.group, w.mate, "active")
	w.membership(3, w.group, w.invited, "invited")
	w.membership(4, w.group, w.left, "left")
	w.membership(5, w.group, w.rejoined, "left")
	w.membership(6, w.group, w.rejoined, "active")
	w.membership(7, w.group, w.erased, "active")
	w.membership(8, w.other, w.owner, "active")
	w.missingPerson = fid(kindPerson, 0xff)
	w.missingContext = fid(kindContext, 0xff)
	return w
}

// tastes seeds person_interests with explicit ids and created_at.
func (w *stdWorld) tastes() {
	add := func(n int, person, tag string) {
		w.insert("person_interests", "id", fid(kindInterest, n), "person_id", person, "tag", tag,
			"created_at", fmt.Sprintf("2030-03-0%dT00:00:00.5Z", n))
	}
	add(1, w.owner, "karaoke")
	add(2, w.owner, "cafe")
	add(3, w.owner, "game")
	add(4, w.mate, "nightlife")
	add(5, w.mate, "Zeta")
	add(6, w.mate, "ăn vặt")
	add(7, w.mate, "alpha")
	add(8, w.rejoined, "cafe")
}

// hugeJSONInt is an integer literal past int64, which json.loads keeps exact.
var hugeJSONInt = strings.Repeat("7", 23)

// catalogue seeds destinations and places whose JSONB columns take every
// shape `list(value or [])` and json.loads distinguish.
func (w *world) catalogue() {
	for _, d := range []string{"d-bien", "d-nui"} {
		w.insert("destinations", "id", d, "name", "Nơi "+d+" (dữ liệu mẫu)", "lat", 12.0, "lng", 109.0,
			"bbox_south", 11.0, "bbox_west", 108.0, "bbox_north", 13.0, "bbox_east", 110.0,
			"created_at", stdCreated, "updated_at", stdCreated)
	}
	place := func(id, destination, category string, extra ...any) {
		w.insert("places", append([]any{"id", id, "destination_id", destination, "name", "Quán " + id + " (dữ liệu mẫu)",
			"category", category, "lat", 10.7702, "lng", 106.7, "geo_precision", "rooftop", "source", "seed",
			"created_at", stdCreated, "updated_at", stdCreated}, extra...)...)
	}
	place("p-b", "d-bien", "cafe", "kinds", `["cà phê", "yên tĩnh"]`, "traits", `[]`, "rating", 4.5,
		"rating_count", 12, "price_min_vnd", int64(20000), "price_max_vnd", int64(60000), "open_now", true,
		"group_fit", `{"ti_le": 0.5, "nhom": 4, "a": [1, 2.0, -0, 1e2, `+hugeJSONInt+`]}`,
		"reviews", `[{"sao": 5, "loi": "Ngon (dữ liệu mẫu)"}]`, "activities", `["ngồi lâu"]`,
		"open_hours", "07:00-22:00", "address", "Số 1 (dữ liệu mẫu)")
	place("p-A", "d-nui", "food", "kinds", `null`, "traits", `"ab"`, "group_fit", `null`, "reviews", `null`)
	place("p-a", "d-bien", "food", "kinds", `{"z": 1, "aa": 2}`, "traits", `0`, "distance_km", 1e-7,
		"travel_minutes", 15, "open_now", false, "activities", `null`)
	place("p-ä", "d-nui", "cafe", "kinds", `""`, "traits", `false`, "source", "osm", "source_ref", "node/mau",
		"license", "ODbL", "flag", "hot", "description", "Mô tả dài (dữ liệu mẫu)")
	place("p-c", "d-bien", "bar", "kinds", `{}`, "traits", `0.0`, "photo_count", 3, "rating", 0.1)
	place("p-big", "d-nui", "food", "kinds", `[]`, "traits", `[]`, "price_min_vnd", int64(1)<<53,
		"price_max_vnd", int64(1)<<53+1, "lat", -89.999999, "lng", 179.87654, "rating_count", 0)
}

// wall seeds a memory wall with a three-way tie on created_at, hearts and
// comments, and a memory of another group.
func (w *stdWorld) wall() {
	const tie = "2030-09-02T08:30:00.5Z"
	w.photo(1, w.group, w.owner, "2030-09-01T10:00:00Z")
	w.checkin(2, w.group, w.mate, tie, "p-b", "Quán B (dữ liệu mẫu)", 10.77, 106.7)
	w.checkin(3, w.group, w.owner, tie, "p-b", "Quán B (dữ liệu mẫu)", 10.77, 106.7)
	w.checkin(4, w.group, w.rejoined, tie, "p-b", "Quán B (dữ liệu mẫu)", 10.77, 106.7)
	w.photo(5, w.group, w.mate, "2030-09-03T00:00:00Z", "place_id", "p-cho", "place_name", "Chợ (dữ liệu mẫu)")
	w.checkin(6, w.group, w.owner, "2030-09-04T00:00:00Z", "p-cho", "Chợ (dữ liệu mẫu)", 10.7702, 106.7)
	w.photo(7, w.group, w.erased, "2030-09-05T00:00:00Z", "caption", "Ảnh 📸 (dữ liệu mẫu)")
	w.photo(8, w.other, w.owner, "2030-09-06T00:00:00Z")
	heart := func(n, memory int, person string) {
		w.insert("memory_reactions", "id", fid(kindReaction, n), "memory_id", fid(kindMemory, memory),
			"person_id", person, "created_at", stdCreated)
	}
	heart(1, 3, w.owner)
	heart(2, 3, w.mate)
	heart(3, 3, w.rejoined)
	heart(4, 7, w.mate)
	heart(5, 8, w.owner)
	for n, author := range []string{w.owner, w.mate} {
		w.insert("memory_comments", "id", fid(kindComment, n+1), "memory_id", fid(kindMemory, 3),
			"author_id", author, "body", "Lời nhắn (dữ liệu mẫu)", "created_at", stdCreated)
	}
}

// trips seeds outings around today = 2030-08-27 in Vietnam: three trips tied
// on ends_on and inserted in descending id order, trips that ended yesterday,
// end today, start today and start tomorrow, and money and memories on both
// sides of Vietnam's midnight (17:00 UTC).
func (w *stdWorld) trips() {
	outing := func(n int, contextID, starts, ends string, extra ...any) {
		w.insert("outings", append([]any{"id", fid(kindOuting, n), "context_id", contextID, "created_by_id", w.owner,
			"title", fmt.Sprintf("Chuyến %d (dữ liệu mẫu)", n), "starts_on", starts, "ends_on", ends,
			"headcount", 4, "budget_per_person_vnd", int64(1500000), "created_at", stdCreated}, extra...)...)
	}
	outing(3, w.group, "2030-08-21", "2030-08-23", "itinerary_days", `[{"ngay": 1, "diem": [1, 2.5, "chợ"]}, {"ngay": 2}]`,
		"timeline_revision", 2, "itinerary_version", 2)
	outing(2, w.group, "2030-08-22", "2030-08-23")
	outing(1, w.group, "2030-08-23", "2030-08-23")
	outing(4, w.group, "2030-08-24", "2030-08-26")
	outing(5, w.group, "2030-08-25", "2030-08-27")
	outing(6, w.group, "2030-08-27", "2030-08-29")
	outing(7, w.group, "2030-08-28", "2030-08-30")
	outing(8, w.other, "2030-08-21", "2030-08-23")
	w.insert("outing_stops", "id", fid(kindStop, 3), "outing_id", fid(kindOuting, 3), "position", 2,
		"minute_of_day", 1439, "label", "Về (dữ liệu mẫu)")
	w.insert("outing_stops", "id", fid(kindStop, 1), "outing_id", fid(kindOuting, 3), "position", 0,
		"minute_of_day", 480, "label", "Ăn sáng (dữ liệu mẫu)", "place_name", "Quán (dữ liệu mẫu)", "place_id", "p-a",
		"day", "2030-08-21", "duration_minutes", 60, "time_locked", false)
	w.insert("outing_stops", "id", fid(kindStop, 2), "outing_id", fid(kindOuting, 3), "position", 1,
		"minute_of_day", 600, "label", "Gặp nhau (dữ liệu mẫu)", "duration_minutes", 0,
		"meeting_lat", 10.5, "meeting_lng", 106.25, "meeting_label", "Cổng (dữ liệu mẫu)")

	w.expense(1, w.group, w.owner,
		expenseVersion{520000, "2030-08-22T12:00:00Z", []int64{260000, 260000}},
		expenseVersion{600000, "2030-08-22T12:00:00Z", []int64{300000, 300000}})
	w.expense(2, w.group, w.mate, expenseVersion{90000, "2030-08-23T16:59:59.999999Z", []int64{45000, 45000}})
	w.expense(3, w.group, w.owner, expenseVersion{70000, "2030-08-23T17:00:00Z", []int64{70000}})
	w.expense(4, w.group, w.owner, expenseVersion{11000, "2030-08-20T17:00:00Z", []int64{11000}})
	w.expense(5, w.group, w.owner, expenseVersion{13000, "2030-08-20T16:59:59.999999Z", []int64{13000}})
	w.expense(6, w.group, w.owner, expenseVersion{340000, "2030-08-27T03:00:00Z", []int64{340000}})
	w.expense(7, w.group, w.owner, expenseVersion{50000, "2030-08-22T12:00:00Z", nil})
	w.expense(8, w.other, w.owner, expenseVersion{999000, "2030-08-22T12:00:00Z", []int64{999000}})

	w.photo(0x21, w.group, w.owner, "2030-08-23T16:59:59.999999Z")
	w.checkin(0x22, w.group, w.mate, "2030-08-23T17:00:00Z", "p-b", "Quán B (dữ liệu mẫu)", 10.77, 106.7)
	w.photo(0x23, w.group, w.owner, "2030-08-27T01:00:00Z")
	w.photo(0x24, w.other, w.owner, "2030-08-22T00:00:00Z")
	w.checkin(0x25, w.group, w.owner, "2030-08-21T00:00:00Z", "p-b", "Quán B (dữ liệu mẫu)", 10.77, 106.7)
	w.photo(0x26, w.group, w.owner, "2030-08-20T16:59:59Z")
}

// recorder notes every statement a Repository method issues.
type recorder struct {
	Querier
	log []string
}

func (r *recorder) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.log = append(r.log, sql)
	return r.Querier.Exec(ctx, sql, args...)
}

func (r *recorder) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.log = append(r.log, sql)
	return r.Querier.Query(ctx, sql, args...)
}

func (r *recorder) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.log = append(r.log, sql)
	return r.Querier.QueryRow(ctx, sql, args...)
}

// seeded opens a rolled-back transaction on the tier database and runs the
// fixture SQL in it.
func seeded(t *testing.T, sql []string) (pgx.Tx, *recorder) {
	t.Helper()
	tx := testdb.Tx(t)
	for _, statement := range sql {
		if _, err := tx.Exec(context.Background(), statement); err != nil {
			t.Fatalf("seed: %v\n%s", err, statement)
		}
	}
	return tx, &recorder{Querier: tx}
}
