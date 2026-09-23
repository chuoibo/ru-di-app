//go:build postgres

package repo

// Differential test against the real SqlAlchemyApiRepository. From services/core:
//
//	CORE_PYTHON_IMAGE=mobile-parity-api:7bf58e3d CORE_TEST_DATABASE_URL=postgresql://... \
//	  GOTOOLCHAIN=local go test -tags postgres -run TestRepositoryOracle -v ./internal/repo/
//
// or scripts/go_postgres_tier.sh with CORE_PYTHON_IMAGE exported. Without
// CORE_PYTHON_IMAGE the test skips.
//
// Design: one database, one private schema, each side in its own transaction.
// The test creates a fresh schema in the tier database and migrates it with
// Alembic from the image, so both implementations read the one schema
// production has, under the same collation, server settings and TimeZone, and
// no other package's committed rows can enter a full-table dump. Every case
// then runs twice on that schema: once through render_repo_oracle.py (the real
// method, the service's sessionmaker settings) and once through this package,
// each inside a transaction that seeds identical literal SQL and is rolled
// back. Nothing a case writes survives it, so the Python run cannot leak into
// the Go run. Two databases would add nothing a rolled-back transaction does
// not already give, and would let a collation or TimeZone difference between
// them pass for a port difference.
//
// After every call both sides record, and the test compares:
//   - the returned value, tagged so JSON loses nothing (ints versus Decimals,
//     float bits, UTC datetimes at microseconds, dataclass field names and
//     order, dict insertion order), or the exception class, SQLSTATE and
//     constraint name;
//   - every statement issued, placeholders and whitespace normalised, one
//     entry per executed statement (a true executemany expands);
//   - probes read in the same transaction: the relation locks it holds, the
//     per-transaction insert/update/delete counters of every table, and for
//     write methods a row_to_json dump of each touched table in full.
//
// Ids a call generates (uuid4 on both sides) cannot be equal; they are bound
// by order of appearance to <generated-N> on each side, so the same generated
// id must appear in the same places (the returned id is the dumped row's id),
// and each must be a version 4 UUID. No generated timestamp is ignored: the
// methods write only the clock the caller passes, and a server-side now()
// would surface as a mismatch.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

const (
	probeLocks = `SELECT c.relname || ' ' || l.mode FROM pg_locks l JOIN pg_class c ON c.oid = l.relation
WHERE l.pid = pg_backend_pid() AND l.locktype = 'relation' AND c.relkind = 'r'
AND c.relnamespace = current_schema()::text::regnamespace ORDER BY 1`
	// pg_stat_xact_user_tables also carries counts of earlier transactions on
	// the same backend that have not been flushed yet, and when that happens
	// depends on timing and on connection reuse. A backend never flushes inside
	// a transaction, so the counters are compared as the difference from a
	// snapshot taken just before the call (writesBaseline).
	probeWrites = `SELECT s.relname || ' ins=' || (s.n_tup_ins - coalesce(b.n_tup_ins, 0))
|| ' upd=' || (s.n_tup_upd - coalesce(b.n_tup_upd, 0)) || ' del=' || (s.n_tup_del - coalesce(b.n_tup_del, 0))
FROM pg_stat_xact_user_tables s LEFT JOIN oracle_writes b ON b.relid = s.relid
WHERE s.schemaname = current_schema()
AND (s.n_tup_ins - coalesce(b.n_tup_ins, 0)) + (s.n_tup_upd - coalesce(b.n_tup_upd, 0)) + (s.n_tup_del - coalesce(b.n_tup_del, 0)) > 0
ORDER BY 1`
	dumpPrefix = `SELECT row_to_json(t)::text FROM `
)

// writesBaseline runs before a call whose probes include probeWrites. The temp
// table lives in pg_temp, outside every probe's schema filter.
var writesBaseline = []string{
	`DROP TABLE IF EXISTS pg_temp.oracle_writes`,
	`CREATE TEMP TABLE oracle_writes AS SELECT relid, n_tup_ins, n_tup_upd, n_tup_del FROM pg_stat_xact_user_tables`,
}

func dumpProbe(table, order string) string { return dumpPrefix + table + ` t ORDER BY ` + order }

type oracleSpec struct {
	Clock []string     `json:"clock"`
	Cases []oracleCase `json:"cases"`
}

type oracleCase struct {
	Name  string       `json:"name"`
	Setup []string     `json:"setup"`
	Steps []oracleCall `json:"steps"`
}

type oracleCall struct {
	Call   string         `json:"call"`
	Args   map[string]any `json:"args"`
	Before []string       `json:"before"`
	Probes []string       `json:"probes"`
}

type pythonRun struct {
	Clock []string `json:"clock"`
	Cases []struct {
		Name  string `json:"name"`
		Steps []struct {
			Result     any     `json:"result"`
			Error      any     `json:"error"`
			Warnings   []any   `json:"warnings"`
			Statements [][]any `json:"statements"`
			Probes     any     `json:"probes"`
		} `json:"steps"`
	} `json:"cases"`
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func oracleCases() oracleSpec {
	var cases []oracleCase
	add := func(name string, setup []string, steps ...oracleCall) {
		cases = append(cases, oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps})
	}
	call := func(method string, args map[string]any, probes ...string) oracleCall {
		c := oracleCall{Call: method, Args: args, Before: []string{}, Probes: append([]string{}, probes...)}
		for _, probe := range probes {
			if probe == probeWrites {
				c.Before = append([]string{}, writesBaseline...)
			}
		}
		return c
	}
	join := func(parts ...[]string) []string {
		out := []string{}
		for _, part := range parts {
			out = append(out, part...)
		}
		return out
	}
	vietnam := []string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}
	people := dumpProbe("people", "t.id")
	interests := dumpProbe("person_interests", "t.person_id, t.tag")
	reports := dumpProbe("reports", "t.created_at, t.id")
	w := newStdWorld()
	base := w.sql

	// --- is_member ---------------------------------------------------------
	var membership []oracleCall
	for _, pair := range [][2]string{
		{w.group, w.owner}, {w.group, w.mate}, {w.group, w.invited}, {w.group, w.left}, {w.group, w.rejoined},
		{w.group, w.erased}, {w.group, w.stranger}, {w.other, w.owner}, {w.other, w.mate}, {w.empty, w.mate},
		{w.missingContext, w.owner}, {w.group, w.missingPerson},
	} {
		membership = append(membership, call("is_member", map[string]any{"context_id": pair[0], "person_id": pair[1]}))
	}
	add("is_member: every membership state", base, membership...)
	add("is_member: a group with no rows at all", nil, call("is_member", map[string]any{"context_id": w.group, "person_id": w.owner}))

	// --- get_person --------------------------------------------------------
	var persons []oracleCall
	for _, id := range []string{w.owner, w.mate, w.erased, w.blank, w.stranger, w.missingPerson} {
		persons = append(persons, call("get_person", map[string]any{"person_id": id}))
	}
	add("get_person: NULL, empty, erased and missing", base, persons...)
	add("get_person: under a Vietnam session TimeZone", join(vietnam, base), persons[0], persons[2])

	// --- update_person_profile ---------------------------------------------
	profile := func(person string, changes ...[2]any) map[string]any {
		pairs := []any{}
		for _, change := range changes {
			pairs = append(pairs, []any{change[0], change[1]})
		}
		return map[string]any{"person_id": person, "changes": pairs}
	}
	probesPeople := []string{probeLocks, probeWrites, people}
	add("update_person_profile: only changed columns, NULL and empty apart", base,
		call("update_person_profile", profile(w.owner,
			[2]any{"wall_comment_policy", "nobody"}, [2]any{"city", "Phố mới (dữ liệu mẫu)"},
			[2]any{"display_name", "Chủ nhóm (dữ liệu mẫu)"}, [2]any{"bio", nil}, [2]any{"discoverable_by_phone", false}),
			probesPeople...),
		call("update_person_profile", profile(w.owner, [2]any{"bio", ""}, [2]any{"budget_band", nil}), probesPeople...),
		call("update_person_profile", profile(w.owner, [2]any{"bio", nil}, [2]any{"city", "Phố mới (dữ liệu mẫu)"}), probesPeople...),
		call("update_person_profile", profile(w.blank, [2]any{"bio", nil}, [2]any{"city", ""}), probesPeople...),
	)
	add("update_person_profile: nothing to change still locks", base,
		call("update_person_profile", profile(w.owner), probesPeople...))
	add("update_person_profile: equal values write nothing", base,
		call("update_person_profile", profile(w.mate, [2]any{"display_name", "Bạn đồng hành (dữ liệu mẫu)"},
			[2]any{"wall_comment_policy", "friends"}, [2]any{"discoverable_by_phone", false},
			[2]any{"bio", "Thích cà phê ☕ (dữ liệu mẫu)"}), probesPeople...))
	add("update_person_profile: a missing person", base,
		call("update_person_profile", profile(w.missingPerson, [2]any{"city", "x"}), probesPeople...))
	add("update_person_profile: an erased account is still written", base,
		call("update_person_profile", profile(w.erased, [2]any{"bio", "Vẫn ghi (dữ liệu mẫu)"}, [2]any{"budget_band", "tiet-kiem"}),
			probesPeople...))
	add("update_person_profile: unicode and every field", base,
		call("update_person_profile", profile(w.stranger, [2]any{"display_name", "Tên mới 🌿 ẞ (dữ liệu mẫu)"},
			[2]any{"bio", "Dòng một\nDòng hai ' \" \\ (dữ liệu mẫu)"}, [2]any{"city", "Nơi ở (dữ liệu mẫu)"},
			[2]any{"budget_band", "thoai-mai"}, [2]any{"wall_comment_policy", "friends"},
			[2]any{"discoverable_by_phone", false}), probesPeople...))
	add("update_person_profile: the check constraint refuses", base,
		call("update_person_profile", profile(w.owner, [2]any{"wall_comment_policy", "everyone"}), probesPeople...))
	add("update_person_profile: longer than varchar(8)", base,
		call("update_person_profile", profile(w.owner, [2]any{"wall_comment_policy", "everybody"}), probesPeople...))

	// --- person interests --------------------------------------------------
	tasted := newStdWorld()
	tasted.tastes()
	withTastes := tasted.sql
	probesInterests := []string{probeLocks, probeWrites, interests}
	setTags := func(person string, now string, tags ...string) map[string]any {
		list := []any{}
		for _, tag := range tags {
			list = append(list, tag)
		}
		return map[string]any{"person_id": person, "tags": list, "now": now}
	}
	add("list_person_interests: database order, nobody, unicode", withTastes,
		call("list_person_interests", map[string]any{"person_id": w.owner}),
		call("list_person_interests", map[string]any{"person_id": w.mate}),
		call("list_person_interests", map[string]any{"person_id": w.stranger}),
		call("list_person_interests", map[string]any{"person_id": w.missingPerson}))
	add("set_person_interests: replace keeps survivors, then replace again", withTastes,
		call("set_person_interests", setTags(w.owner, "2030-08-27T12:00:00.123456+00:00", "nightlife", "cafe", "an-uong", "cafe"), probesInterests...),
		call("set_person_interests", setTags(w.owner, "2030-08-28T19:00:00.5+07:00", "cafe", "outdoor"), probesInterests...),
		call("list_person_interests", map[string]any{"person_id": w.owner}))
	add("set_person_interests: clearing deletes every row", withTastes,
		call("set_person_interests", setTags(w.mate, "2030-08-27T12:00:00+00:00"), probesInterests...))
	add("set_person_interests: the same set writes nothing", withTastes,
		call("set_person_interests", setTags(w.owner, "2030-08-27T12:00:00+00:00", "game", "karaoke", "cafe"), probesInterests...))
	add("set_person_interests: unicode and case in new tags", withTastes,
		call("set_person_interests", setTags(w.stranger, "2030-08-27T12:00:00+00:00", "ăn vặt", "Zeta", "alpha", "Ăn", "zeta"), probesInterests...))
	add("set_person_interests: a person with no row", withTastes,
		call("set_person_interests", setTags(w.missingPerson, "2030-08-27T12:00:00+00:00", "cafe"), probesInterests...))
	add("set_person_interests: a blank tag", withTastes,
		call("set_person_interests", setTags(w.owner, "2030-08-27T12:00:00+00:00", "   "), probesInterests...))
	for _, count := range []int{1000, 1001} {
		many := []string{}
		for i := 0; i < count; i++ {
			many = append(many, fmt.Sprintf("t%04d", count-i))
		}
		add(fmt.Sprintf("set_person_interests: %d new rows", count), base,
			call("set_person_interests", setTags(w.stranger, "2030-08-27T12:00:00+00:00", many...), probeWrites, interests))
	}
	add("interests_by_person: order, absent people, duplicates, none", withTastes,
		call("interests_by_person", map[string]any{"person_ids": []any{w.rejoined, w.owner, w.stranger, w.owner, w.missingPerson, w.mate}}),
		call("interests_by_person", map[string]any{"person_ids": []any{}}),
		call("interests_by_person", map[string]any{"person_ids": []any{w.stranger}}))

	// --- list_places -------------------------------------------------------
	var shops world
	shops.catalogue()
	add("list_places: filter shapes and JSON shapes", shops.sql,
		call("list_places", map[string]any{}),
		call("list_places", map[string]any{"destination_id": "d-bien"}),
		call("list_places", map[string]any{"category": "cafe"}),
		call("list_places", map[string]any{"destination_id": "d-nui", "category": "food"}),
		call("list_places", map[string]any{"category": ""}),
		call("list_places", map[string]any{"destination_id": nil, "category": "bar"}),
		call("list_places", map[string]any{"destination_id": "khong-co"}))
	add("list_places: an empty catalogue", nil, call("list_places", map[string]any{}))
	for _, odd := range []struct{ name, column, value string }{
		{"true in kinds", "kinds", "true"},
		{"a non-zero number in traits", "traits", "7"},
	} {
		add("list_places: "+odd.name, join(shops.sql, []string{`UPDATE places SET ` + odd.column + ` = '` + odd.value + `' WHERE id = 'p-c'`}),
			call("list_places", map[string]any{}))
	}

	// --- list_memories -----------------------------------------------------
	walled := newStdWorld()
	walled.wall()
	const tie = "2030-09-02T08:30:00.5+00:00"
	wall := func(extra map[string]any) map[string]any {
		args := map[string]any{"context_id": w.group, "limit": 10}
		for k, v := range extra {
			args[k] = v
		}
		return args
	}
	add("list_memories: newest first, ties, pages, filters, counts", walled.sql,
		call("list_memories", wall(map[string]any{"viewer_id": w.owner})),
		call("list_memories", wall(map[string]any{"viewer_id": w.mate, "limit": 3})),
		call("list_memories", wall(map[string]any{"before": []any{tie, fid(kindMemory, 3)}})),
		call("list_memories", wall(map[string]any{"before": []any{tie, fid(kindMemory, 4)}, "limit": 2, "viewer_id": w.rejoined})),
		call("list_memories", wall(map[string]any{"before": []any{"2030-09-02T15:30:00.5+07:00", fid(kindMemory, 2)}})),
		call("list_memories", wall(map[string]any{"kind": "photo"})),
		call("list_memories", wall(map[string]any{"kind": "checkin", "viewer_id": w.owner})),
		call("list_memories", wall(map[string]any{"place_id": "p-cho"})),
		call("list_memories", wall(map[string]any{"place_id": ""})),
		call("list_memories", wall(map[string]any{"kind": nil, "place_id": nil, "viewer_id": nil})),
		call("list_memories", wall(map[string]any{"limit": 7})),
		call("list_memories", wall(map[string]any{"limit": 6})),
		call("list_memories", wall(map[string]any{"limit": 0})),
		call("list_memories", wall(map[string]any{"limit": -1})),
		call("list_memories", wall(map[string]any{"context_id": w.other, "viewer_id": w.owner})),
		call("list_memories", wall(map[string]any{"context_id": w.empty})),
		call("list_memories", wall(map[string]any{"viewer_id": w.stranger})))
	add("list_memories: an unknown kind", walled.sql, call("list_memories", wall(map[string]any{"kind": "video"})))
	add("list_memories: a limit below -1", walled.sql, call("list_memories", wall(map[string]any{"limit": -2})))

	// --- group_recap -------------------------------------------------------
	travelled := newStdWorld()
	travelled.trips()
	recap := func(context, today string) oracleCall {
		return call("group_recap", map[string]any{"context_id": context, "today": today})
	}
	add("group_recap: trips, money and memories around today", travelled.sql,
		recap(w.group, "2030-08-27"), recap(w.group, "2030-08-23"), recap(w.group, "2030-08-24"),
		recap(w.group, "2030-08-20"), recap(w.group, "2031-01-01"), recap(w.other, "2030-08-27"), recap(w.empty, "2030-08-27"))
	add("group_recap: a Vietnam session TimeZone moves nothing", join(vietnam, travelled.sql), recap(w.group, "2030-08-27"))
	add("group_recap: a session TimeZone west of UTC moves nothing",
		join([]string{"SET LOCAL TimeZone = 'Etc/GMT+10'"}, travelled.sql), recap(w.group, "2030-08-27"))

	// --- create_report -----------------------------------------------------
	probesReports := []string{probeLocks, probeWrites, reports}
	report := func(reporter, targetType, target, reason string, note any, now string) oracleCall {
		return call("create_report", map[string]any{"reporter_id": reporter, "target_type": targetType, "target_id": target,
			"reason": reason, "note": note, "now": now}, probesReports...)
	}
	add("create_report: without and with a note", base,
		report(w.owner, "person", w.mate, "spam", nil, "2030-08-27T12:00:00.070000+00:00"),
		report(w.mate, "post", fid(kindReport, 1), "other", "Nội dung 🙂 ' (dữ liệu mẫu)", "2030-08-27T19:00:01+07:00"),
		report(w.owner, "comment", w.missingPerson, "harassment", "", "2030-08-27T12:00:02.123456"+"789+00:00"))
	add("create_report: 500 characters", base,
		report(w.erased, "story", w.owner, "impersonation", strings.Repeat("😀", 500), "2030-08-27T12:00:00+00:00"))
	add("create_report: 501 characters", base,
		report(w.owner, "message", w.owner, "inappropriate", strings.Repeat("😀", 501), "2030-08-27T12:00:00+00:00"))
	add("create_report: a reporter with no people row", base,
		report(w.missingPerson, "person", w.owner, "spam", nil, "2030-08-27T12:00:00+00:00"))
	add("create_report: a target type the check refuses", base,
		report(w.owner, "user", w.owner, "spam", nil, "2030-08-27T12:00:00+00:00"))

	return oracleSpec{
		Clock: []string{"2030-08-27T16:59:59.999999+00:00", "2030-08-27T17:00:00+00:00", "2030-08-28T00:30:00+07:00",
			"2030-12-31T23:59:59-10:00", "1970-06-01T16:30:00+00:00", "1975-06-13T16:30:00+00:00"},
		Cases: cases,
	}
}

// ---------------------------------------------------------------------------
// The Go side
// ---------------------------------------------------------------------------

func tv(kind string, value any) any { return map[string]any{kind: value} }
func tStr(s string) any             { return tv("str", s) }
func tUUID(s string) any            { return tv("uuid", s) }
func tInt(n int64) any              { return tv("int", strconv.FormatInt(n, 10)) }
func tBool(b bool) any              { return tv("bool", b) }
func tFloat(f float64) any          { return tv("float", fmt.Sprintf("%016x", math.Float64bits(f))) }
func tInstant(x time.Time) any {
	return tv("datetime", x.UTC().Format("2006-01-02T15:04:05.000000+00:00"))
}
func tDay(x time.Time) any { return tv("date", x.Format("2006-01-02")) }
func tSeq(items []any) any { return tv("seq", append([]any{}, items...)) }

func optional[T any](v *T, tag func(T) any) any {
	if v == nil {
		return nil
	}
	return tag(*v)
}

func tStrings(values []string) any {
	items := []any{}
	for _, v := range values {
		items = append(items, tStr(v))
	}
	return tSeq(items)
}

func tRecord(name string, fields ...any) any {
	pairs := []any{}
	for i := 0; i < len(fields); i += 2 {
		pairs = append(pairs, []any{fields[i], fields[i+1]})
	}
	return map[string]any{"record": name, "fields": pairs}
}

// tJSON tags JSON text the way json.loads then tag() would see it: key order
// kept, a number with a fraction or exponent a float, any other an int.
func tJSON(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func() any
	value = func() any {
		token, err := decoder.Token()
		if err != nil {
			panic(err)
		}
		switch x := token.(type) {
		case nil:
			return nil
		case bool:
			return tBool(x)
		case string:
			return tStr(x)
		case json.Number:
			literal := x.String()
			if strings.ContainsAny(literal, ".eE") {
				f, _ := strconv.ParseFloat(literal, 64)
				return tFloat(f)
			}
			n, _ := new(big.Int).SetString(literal, 10)
			return tv("int", n.String())
		case json.Delim:
			items := []any{}
			for decoder.More() {
				if x == '{' {
					key, _ := decoder.Token()
					items = append(items, []any{tStr(key.(string)), value()})
				} else {
					items = append(items, value())
				}
			}
			if _, err := decoder.Token(); err != nil {
				panic(err)
			}
			if x == '{' {
				return tv("dict", items)
			}
			return tv("seq", items)
		}
		panic(fmt.Sprintf("json token %T", token))
	}
	return value()
}

func tPerson(p *Person) any {
	if p == nil {
		return nil
	}
	return tRecord("PersonRecord", "id", tUUID(p.ID), "display_name", tStr(p.DisplayName), "created_at", tInstant(p.CreatedAt),
		"bio", optional(p.Bio, tStr), "city", optional(p.City, tStr), "budget_band", optional(p.BudgetBand, tStr),
		"wall_comment_policy", tStr(p.WallCommentPolicy), "discoverable_by_phone", tBool(p.DiscoverableByPhone),
		"deleted_at", optional(p.DeletedAt, tInstant))
}

func tPlace(p Place) any {
	return tRecord("PlaceRecord", "id", tStr(p.ID), "destination_id", tStr(p.DestinationID), "name", tStr(p.Name),
		"category", tStr(p.Category), "kinds", tStrings(p.Kinds), "address", optional(p.Address, tStr),
		"lat", optional(p.Lat, tFloat), "lng", optional(p.Lng, tFloat), "rating", optional(p.Rating, tFloat),
		"rating_count", optional(p.RatingCount, tInt), "price_min_vnd", optional(p.PriceMinVND, tInt),
		"price_max_vnd", optional(p.PriceMaxVND, tInt), "open_hours", optional(p.OpenHours, tStr),
		"open_now", optional(p.OpenNow, tBool), "travel_minutes", optional(p.TravelMinutes, tInt),
		"distance_km", optional(p.DistanceKM, tFloat), "photo_count", tInt(p.PhotoCount), "traits", tStrings(p.Traits),
		"group_fit", tJSON(p.GroupFit), "flag", optional(p.Flag, tStr), "description", optional(p.Description, tStr),
		"reviews", tJSON(p.Reviews), "source", tStr(p.Source), "source_ref", optional(p.SourceRef, tStr),
		"license", optional(p.License, tStr), "activities", tJSON(p.Activities),
		"geo_precision", optional(p.GeoPrecision, tStr))
}

func tMemory(m Memory) any {
	return tRecord("MemoryRecord", "id", tUUID(m.ID), "context_id", tUUID(m.ContextID), "author_id", tUUID(m.AuthorID),
		"kind", tStr(m.Kind), "image_url", optional(m.ImageURL, tStr), "caption", optional(m.Caption, tStr),
		"place_id", optional(m.PlaceID, tStr), "place_name", optional(m.PlaceName, tStr),
		"lat", optional(m.Lat, tFloat), "lng", optional(m.Lng, tFloat), "created_at", tInstant(m.CreatedAt),
		"reaction_count", tInt(m.ReactionCount), "comment_count", tInt(m.CommentCount),
		"viewer_has_reacted", tBool(m.ViewerHasReacted))
}

func tRecap(r RecapOuting) any {
	o := r.Outing
	stops := []any{}
	for _, s := range o.Stops {
		stops = append(stops, tRecord("OutingStopRecord", "id", tUUID(s.ID), "position", tInt(s.Position),
			"minute_of_day", tInt(s.MinuteOfDay), "label", tStr(s.Label), "place_name", optional(s.PlaceName, tStr),
			"place_id", optional(s.PlaceID, tStr), "day", optional(s.Day, tDay),
			"duration_minutes", optional(s.DurationMinutes, tInt), "time_locked", tBool(s.TimeLocked),
			"meeting_lat", optional(s.MeetingLat, tFloat), "meeting_lng", optional(s.MeetingLng, tFloat),
			"meeting_label", optional(s.MeetingLabel, tStr)))
	}
	days := []any{}
	for _, d := range o.ItineraryDays {
		days = append(days, tJSON(d))
	}
	outing := tRecord("OutingRecord", "id", tUUID(o.ID), "context_id", tUUID(o.ContextID), "created_by_id", tUUID(o.CreatedByID),
		"title", tStr(o.Title), "starts_on", tDay(o.StartsOn), "ends_on", tDay(o.EndsOn), "headcount", tInt(o.Headcount),
		"budget_per_person_vnd", tInt(o.BudgetPerPersonVND), "created_at", tInstant(o.CreatedAt), "stops", tSeq(stops),
		"timeline_revision", tInt(o.TimelineRevision), "itinerary_version", tInt(o.ItineraryVersion),
		"itinerary_days", tSeq(days))
	return tRecord("RecapOutingRecord", "outing", outing, "in_progress", tBool(r.InProgress),
		"split_total_vnd", tInt(r.SplitTotalVND), "expense_count", tInt(r.ExpenseCount), "memory_count", tInt(r.MemoryCount))
}

func argString(a map[string]any, key string) string { s, _ := a[key].(string); return s }

func argOptional(a map[string]any, key string) *string {
	if s, ok := a[key].(string); ok {
		return &s
	}
	return nil
}

func argStrings(v any) []string {
	out := []string{}
	for _, item := range v.([]any) {
		out = append(out, item.(string))
	}
	return out
}

func argInstant(text string) time.Time {
	instant, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		panic(err)
	}
	return instant
}

func argChanges(v any) ProfileChanges {
	var changes ProfileChanges
	for _, item := range v.([]any) {
		pair := item.([]any)
		var text *string
		if s, ok := pair[1].(string); ok {
			text = &s
		}
		switch pair[0].(string) {
		case "display_name":
			changes.DisplayName = text
		case "bio":
			changes.Bio = SetText(text)
		case "city":
			changes.City = SetText(text)
		case "budget_band":
			changes.BudgetBand = SetText(text)
		case "wall_comment_policy":
			changes.WallCommentPolicy = text
		case "discoverable_by_phone":
			flag := pair[1].(bool)
			changes.DiscoverableByPhone = &flag
		default:
			panic(pair[0])
		}
	}
	return changes
}

func goCall(repo Repository, method string, a map[string]any) (any, error) {
	switch method {
	case "is_member":
		ok, err := repo.IsMember(bg, argString(a, "context_id"), argString(a, "person_id"))
		return tBool(ok), err
	case "get_person":
		p, err := repo.GetPerson(bg, argString(a, "person_id"))
		return tPerson(p), err
	case "update_person_profile":
		p, err := repo.UpdatePersonProfile(bg, argString(a, "person_id"), argChanges(a["changes"]))
		return tPerson(p), err
	case "list_person_interests":
		tags, err := repo.ListPersonInterests(bg, argString(a, "person_id"))
		return tStrings(tags), err
	case "set_person_interests":
		tags, err := repo.SetPersonInterests(bg, argString(a, "person_id"), argStrings(a["tags"]), argInstant(argString(a, "now")))
		return tStrings(tags), err
	case "interests_by_person":
		entries, err := repo.InterestsByPerson(bg, argStrings(a["person_ids"]))
		pairs := []any{}
		for _, e := range entries {
			pairs = append(pairs, []any{tUUID(e.PersonID), tStrings(e.Tags)})
		}
		return tv("dict", pairs), err
	case "list_places":
		places, err := repo.ListPlaces(bg, PlaceFilter{DestinationID: argOptional(a, "destination_id"), Category: argOptional(a, "category")})
		items := []any{}
		for _, p := range places {
			items = append(items, tPlace(p))
		}
		return tSeq(items), err
	case "list_memories":
		query := MemoryQuery{Limit: int(a["limit"].(float64)), Kind: argOptional(a, "kind"),
			PlaceID: argOptional(a, "place_id"), ViewerID: argOptional(a, "viewer_id")}
		if before, ok := a["before"].([]any); ok {
			query.Before = &MemoryCursor{CreatedAt: argInstant(before[0].(string)), ID: before[1].(string)}
		}
		page, err := repo.ListMemories(bg, argString(a, "context_id"), query)
		items := []any{}
		for _, m := range page.Memories {
			items = append(items, tMemory(m))
		}
		return tRecord("MemoryPage", "memories", tSeq(items), "has_more", tBool(page.HasMore)), err
	case "group_recap":
		today, err := time.Parse("2006-01-02", argString(a, "today"))
		if err != nil {
			panic(err)
		}
		records, err := repo.GroupRecap(bg, argString(a, "context_id"), today)
		items := []any{}
		for _, r := range records {
			items = append(items, tRecap(r))
		}
		return tSeq(items), err
	case "create_report":
		report, err := repo.CreateReport(bg, ReportInput{ReporterID: argString(a, "reporter_id"),
			TargetType: argString(a, "target_type"), TargetID: argString(a, "target_id"), Reason: argString(a, "reason"),
			Note: argOptional(a, "note"), Now: argInstant(argString(a, "now"))})
		return tRecord("ReportRecord", "id", tUUID(report.ID), "created_at", tInstant(report.CreatedAt)), err
	}
	panic("unknown method " + method)
}

// pythonErrorClass is the DB-API class psycopg (and SQLAlchemy's wrapper of
// the same name) raises for a SQLSTATE class.
func pythonErrorClass(code string) string {
	switch code[:2] {
	case "22":
		return "DataError"
	case "23":
		return "IntegrityError"
	case "42":
		return "ProgrammingError"
	case "0A":
		return "NotSupportedError"
	case "08", "40", "53", "54", "55", "57", "58":
		return "OperationalError"
	}
	return "InternalError"
}

func goError(err error) map[string]any {
	out := map[string]any{"type": nil, "sqlstate": nil, "constraint": nil}
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &pg):
		out["type"], out["sqlstate"] = pythonErrorClass(pg.Code), pg.Code
		if pg.ConstraintName != "" {
			out["constraint"] = pg.ConstraintName
		}
	case errors.Is(err, ErrUnknownMemoryKind):
		out["type"] = "ValueError"
	case errors.Is(err, ErrPythonTypeError):
		out["type"] = "TypeError"
	default:
		out["type"] = "go error: " + err.Error()
	}
	return out
}

func generic(t *testing.T, v any) any {
	t.Helper()
	payload, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

var (
	pythonBind = regexp.MustCompile(`%\(\w+\)s`)
	goBind     = regexp.MustCompile(`\$\d+`)
)

func normalizeSQL(sql string) string {
	sql = pythonBind.ReplaceAllString(sql, "?")
	sql = goBind.ReplaceAllString(sql, "?")
	sql = strings.Join(strings.Fields(sql), " ")
	sql = strings.ReplaceAll(sql, "( ", "(")
	return strings.ReplaceAll(sql, " )", ")")
}

func runGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	for _, sql := range c.Setup {
		if _, err := tx.Exec(bg, sql); err != nil {
			t.Fatalf("setup: %v\n%s", err, sql)
		}
	}
	steps := []any{}
	for _, step := range c.Steps {
		for _, sql := range step.Before {
			if _, err := tx.Exec(bg, sql); err != nil {
				t.Fatalf("before: %v\n%s", err, sql)
			}
		}
		rec := &recorder{Querier: tx}
		value, err := goCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = goError(err)
			steps = append(steps, generic(t, out))
			break
		}
		out["result"] = value
		probes := []any{}
		for _, sql := range step.Probes {
			rows, err := tx.Query(bg, sql)
			if err != nil {
				t.Fatalf("probe: %v\n%s", err, sql)
			}
			texts, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				t.Fatal(err)
			}
			probes = append(probes, append([]string{}, texts...))
		}
		out["probes"] = probes
		steps = append(steps, generic(t, out))
	}
	return steps
}

// ---------------------------------------------------------------------------
// Comparing
// ---------------------------------------------------------------------------

var uuidText = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// binder names the ids a side generated, in order of appearance.
type binder struct {
	known map[string]bool
	names map[string]string
	notV4 []string
}

func (b *binder) walk(v any) any {
	switch x := v.(type) {
	case string:
		return uuidText.ReplaceAllStringFunc(x, func(id string) string {
			if b.known[id] {
				return id
			}
			name, seen := b.names[id]
			if !seen {
				name = fmt.Sprintf("<generated-%d>", len(b.names)+1)
				b.names[id] = name
				if !v4.MatchString(id) {
					b.notV4 = append(b.notV4, id)
				}
			}
			return name
		})
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = b.walk(x[i])
		}
		return out
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := map[string]any{}
		for _, k := range keys {
			out[k] = b.walk(x[k])
		}
		return out
	}
	return v
}

type repoTally struct {
	cases, steps, results, errors, statements, probeRows, tableRows, generated, mismatches int
}

func (tally *repoTally) mismatch(t *testing.T, format string, args ...any) {
	t.Helper()
	tally.mismatches++
	t.Errorf(format, args...)
}

func clipJSON(v any) string {
	payload, _ := json.Marshal(v)
	text := []rune(string(payload))
	if len(text) > 2500 {
		return string(text[:2500]) + "..."
	}
	return string(text)
}

func compareCase(t *testing.T, tally *repoTally, c oracleCase, python []any, golang []any) {
	t.Helper()
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
	}
	pb := &binder{known: known, names: map[string]string{}}
	gb := &binder{known: known, names: map[string]string{}}
	if len(python) != len(golang) {
		tally.mismatch(t, "python ran %d steps, go ran %d", len(python), len(golang))
	}
	for i := 0; i < len(python) && i < len(golang); i++ {
		p := pb.walk(python[i]).(map[string]any)
		g := gb.walk(golang[i]).(map[string]any)
		tally.steps++
		if p["error"] != nil {
			tally.errors++
		} else {
			tally.results++
		}
		statements, _ := p["statements"].([]any)
		tally.statements += len(statements)
		probes, _ := p["probes"].([]any)
		for j, rows := range probes {
			n := len(rows.([]any))
			tally.probeRows += n
			if strings.HasPrefix(c.Steps[i].Probes[j], dumpPrefix) {
				tally.tableRows += n
			}
		}
		for _, key := range []string{"result", "error", "warnings", "statements", "probes"} {
			if !reflect.DeepEqual(p[key], g[key]) {
				tally.mismatch(t, "step %d %s: %s differs\n  python: %s\n  go:     %s", i, c.Steps[i].Call, key, clipJSON(p[key]), clipJSON(g[key]))
			}
		}
	}
	if len(pb.names) != len(gb.names) {
		tally.mismatch(t, "python generated %d ids, go %d", len(pb.names), len(gb.names))
	}
	tally.generated += len(pb.names)
	for _, id := range append(pb.notV4, gb.notV4...) {
		tally.mismatch(t, "generated id %s is not a version 4 UUID", id)
	}
}

func pythonURL(raw, schema string) string {
	url := raw
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(url, prefix) {
			url = "postgresql+psycopg://" + strings.TrimPrefix(url, prefix)
			break
		}
	}
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return url + separator + "options=-csearch_path=" + schema
}

func tail(out []byte) string {
	if len(out) > 3000 {
		return string(out[len(out)-3000:])
	}
	return string(out)
}

func TestRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of oracle_postgres_test.go")
	}
	base := testdb.Pool(t)
	raw := strings.TrimSpace(os.Getenv("CORE_TEST_DATABASE_URL"))
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}

	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "repo_oracle_" + hex.EncodeToString(suffix[:])
	if _, err := base.Exec(bg, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(bg, "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	migrate := osexec.Command("docker", "run", "--rm", "--network", "host",
		"-e", "MOBILE_DATABASE_URL="+pythonURL(raw, schema), image, "alembic", "upgrade", "head")
	if out, err := migrate.CombinedOutput(); err != nil {
		t.Fatalf("alembic upgrade head into %s: %v\n%s", schema, err, tail(out))
	}
	config, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(bg, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	payload, err := json.Marshal(oracleCases())
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+pythonURL(raw, schema), "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for _, s := range python.Cases[i].Steps {
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statements = append(statements, normalizeSQL(entry[0].(string)))
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}
		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, pySteps, runGoCase(t, pool, c))
		})
	}

	if len(python.Clock) != len(spec.Clock) {
		t.Fatalf("python answered %d clock instants of %d", len(python.Clock), len(spec.Clock))
	}
	for i, instant := range spec.Clock {
		if got := WallClockDate(argInstant(instant)).Format("2006-01-02"); got != python.Clock[i] {
			tally.mismatch(t, "WallClockDate(%s) = %s, python %s", instant, got, python.Clock[i])
		}
	}

	t.Logf("repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which %d table rows, "+
		"%d generated ids bound, %d clock instants, %d mismatches",
		tally.cases, tally.steps, tally.results, tally.errors, tally.statements, tally.probeRows, tally.tableRows,
		tally.generated, len(spec.Clock), tally.mismatches)
}
