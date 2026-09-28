//go:build postgres

package repo

// Differential test of the WAI repository methods (messages, destinations,
// place photos, outing memories, group photos at a place, SetMembershipRole)
// against the real SqlAlchemyApiRepository, driven through
// scripts/render_wai_repo_oracle.py. Same design as
// groups_oracle_postgres_test.go: a private schema, tagged values, normalised
// SQL, probes.
//
// Without CORE_PYTHON_IMAGE the test skips; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"encoding/json"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

const (
	kindWAIMessage    = 0x51
	kindWAIPlacePhoto = 0x52
	kindWAIOuting     = 0x53
)

func tDestination(d Destination) any {
	return tRecord("DestinationRecord", "id", tStr(d.ID), "name", tStr(d.Name),
		"province", optional(d.Province, tStr), "lat", tFloat(d.Lat), "lng", tFloat(d.Lng),
		"bbox_south", tFloat(d.BBoxSouth), "bbox_west", tFloat(d.BBoxWest),
		"bbox_north", tFloat(d.BBoxNorth), "bbox_east", tFloat(d.BBoxEast),
		"blurb", optional(d.Blurb, tStr), "sort_order", tInt(d.SortOrder))
}

func tPlacePhoto(p PlacePhoto) any {
	return tRecord("PlacePhotoRecord", "id", tUUID(p.ID), "place_id", tStr(p.PlaceID),
		"storage_key", tStr(p.StorageKey), "content_type", tStr(p.ContentType),
		"byte_size", tInt(p.ByteSize), "width", tInt(p.Width), "height", tInt(p.Height),
		"author", optional(p.Author, tStr), "license", optional(p.License, tStr), "source_url", tStr(p.SourceURL),
		"title", optional(p.Title, tStr), "sort_order", tInt(p.SortOrder))
}

func tMessage(m Message) any {
	return tRecord("MessageRecord", "id", tUUID(m.ID), "context_id", tUUID(m.ContextID),
		"author_id", optional(m.AuthorID, tUUID), "kind", tStr(m.Kind),
		"body", optional(m.Body, tStr), "image_url", optional(m.ImageURL, tStr),
		"card", tJSON(m.Card), "created_at", tInstant(m.CreatedAt),
		"reply_to_id", optional(m.ReplyToID, tUUID), "deleted_at", optional(m.DeletedAt, tInstant))
}

func tMessagePage(p MessagePage) any {
	items := []any{}
	for _, m := range p.Messages {
		items = append(items, tMessage(m))
	}
	return tRecord("MessagePage", "messages", tSeq(items), "has_more", tBool(p.HasMore))
}

func waiGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	switch method {
	case "create_message":
		m, err := repo.CreateMessage(bg, MessageInput{
			ContextID: s("context_id"), AuthorID: argOptional(a, "author_id"), Kind: s("kind"),
			Body: argOptional(a, "body"), ImageURL: argOptional(a, "image_url"),
			Card: argCard(a), Now: argInstant(s("now")), ReplyToID: argOptional(a, "reply_to_id"),
		})
		return tMessage(m), err
	case "list_messages":
		page, err := repo.ListMessages(bg, s("context_id"), argInt(a, "limit"),
			argMessageCursor(a, "before"), argMessageCursor(a, "after"))
		return tMessagePage(page), err
	case "list_destinations":
		rows, err := repo.ListDestinations(bg)
		items := []any{}
		for _, row := range rows {
			items = append(items, tDestination(row))
		}
		return tSeq(items), err
	case "list_place_photos":
		rows, err := repo.ListPlacePhotos(bg, s("place_id"))
		items := []any{}
		for _, row := range rows {
			items = append(items, tPlacePhoto(row))
		}
		return tSeq(items), err
	case "list_outing_memories":
		rows, err := repo.ListOutingMemories(bg, s("outing_id"), argInt(a, "limit"), argOptional(a, "viewer_id"))
		items := []any{}
		for _, row := range rows {
			items = append(items, tMemory(row))
		}
		return tSeq(items), err
	case "group_photos_at_place":
		rows, err := repo.GroupPhotosAtPlace(bg, s("place_id"), s("viewer_id"), argInt(a, "limit"))
		items := []any{}
		for _, row := range rows {
			items = append(items, tMemory(row))
		}
		return tSeq(items), err
	case "set_membership_role":
		m, err := repo.SetMembershipRole(bg, s("context_id"), s("person_id"), s("role"))
		return nilOr(m, tMembership), err
	}
	return nil, errUnknownWAICall(method)
}

type unknownWAICall string

func (e unknownWAICall) Error() string { return "unknown call " + string(e) }

func errUnknownWAICall(method string) error { return unknownWAICall(method) }

func argCard(a map[string]any) json.RawMessage {
	v, ok := a["card"]
	if !ok || v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

func argMessageCursor(a map[string]any, key string) *MessageCursor {
	v, ok := a[key]
	if !ok || v == nil {
		return nil
	}
	pair := v.([]any)
	return &MessageCursor{CreatedAt: argInstant(pair[0].(string)), ID: pair[1].(string)}
}

func runWAIGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
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
		value, err := waiGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = groupsGoError(err)
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

func waiRepoOracleCases() ([]socialCase, oracleSpec) {
	w := newStdWorld()
	w.catalogue()
	const now = "2030-09-01T12:00:00.123456Z"
	oid := fid(kindOuting, kindWAIOuting)
	w.insert("outings", "id", oid, "context_id", w.group, "created_by_id", w.owner,
		"title", "Đà Lạt (dữ liệu mẫu)", "starts_on", "2030-09-01", "ends_on", "2030-09-03",
		"headcount", 4, "budget_per_person_vnd", int64(100000), "created_at", stdCreated)
	w.photo(0x21, w.group, w.owner, "2030-09-02T08:00:00Z", "place_id", "p-b", "place_name", "Quán p-b (dữ liệu mẫu)")
	w.photo(0x22, w.group, w.mate, "2030-08-01T08:00:00Z", "place_id", "p-b", "place_name", "Quán p-b (dữ liệu mẫu)")
	m1 := fid(kindWAIMessage, 1)
	m2 := fid(kindWAIMessage, 2)
	m3 := fid(kindWAIMessage, 3)
	w.insert("messages", "id", m1, "context_id", w.group, "author_id", w.owner, "kind", "text",
		"body", "Một (dữ liệu mẫu)", "created_at", "2030-09-01T10:00:00Z")
	w.insert("messages", "id", m2, "context_id", w.group, "author_id", w.mate, "kind", "text",
		"body", "Hai (dữ liệu mẫu)", "created_at", "2030-09-01T10:05:00Z")
	w.insert("messages", "id", m3, "context_id", w.group, "author_id", w.owner, "kind", "sticker",
		"body", "ok-chot", "created_at", "2030-09-01T10:10:00Z")
	mOther := fid(kindWAIMessage, 4)
	w.insert("messages", "id", mOther, "context_id", w.other, "author_id", w.owner, "kind", "text",
		"body", "Nhóm kia (dữ liệu mẫu)", "created_at", "2030-09-01T10:00:00Z")
	title := "Ảnh quán (dữ liệu mẫu)"
	w.insert("place_photos", "id", fid(kindWAIPlacePhoto, 1), "place_id", "p-b",
		"storage_key", "place/p-b/mau-1.jpg", "content_type", "image/jpeg",
		"byte_size", 12, "width", 8, "height", 8, "author", "Tác giả mẫu",
		"license", "CC BY", "source_url", "https://example.invalid/p-b-1",
		"title", title, "sort_order", 2, "created_at", stdCreated)
	w.insert("place_photos", "id", fid(kindWAIPlacePhoto, 2), "place_id", "p-b",
		"storage_key", "place/p-b/mau-2.jpg", "content_type", "image/png",
		"byte_size", 16, "width", 4, "height", 4, "author", "Tác giả mẫu",
		"license", "CC BY", "source_url", "https://example.invalid/p-b-2",
		"sort_order", 1, "created_at", stdCreated)

	var cases []socialCase
	add := func(name, wantEnd string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, w.sql...),
			Steps: steps}, wantEnd})
	}
	messagesDump := dumpProbe("messages", "t.created_at, t.id")
	membershipsDump := dumpProbe("memberships", "t.context_id, t.person_id, t.id")
	destinationsDump := dumpProbe("destinations", "t.sort_order, t.id")
	photosDump := dumpProbe("place_photos", "t.sort_order, t.id")
	memoriesDump := dumpProbe("memories", "t.created_at, t.id")
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	read := func(method string, a map[string]any, dumps ...string) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...),
			Probes: append([]string{probeLocks, probeWrites}, dumps...)}
	}
	write := func(method string, a map[string]any, dumps ...string) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...),
			Probes: append([]string{probeLocks, probeWrites}, dumps...)}
	}

	add("create_message: text", "",
		write("create_message", args("context_id", w.group, "author_id", w.owner, "kind", "text",
			"body", "Xin chào (dữ liệu mẫu)", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", nil), messagesDump))
	add("create_message: reply in the same group", "",
		write("create_message", args("context_id", w.group, "author_id", w.mate, "kind", "text",
			"body", "Trả lời (dữ liệu mẫu)", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", m1), messagesDump))
	add("create_message: sticker", "",
		write("create_message", args("context_id", w.group, "author_id", w.owner, "kind", "sticker",
			"body", "ok-chot", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", nil), messagesDump))
	add("create_message: text without a body", "IntegrityError",
		write("create_message", args("context_id", w.group, "author_id", w.owner, "kind", "text",
			"body", nil, "image_url", nil, "card", nil, "now", now, "reply_to_id", nil), messagesDump))
	add("create_message: text with an image", "IntegrityError",
		write("create_message", args("context_id", w.group, "author_id", w.owner, "kind", "text",
			"body", "Xin chào (dữ liệu mẫu)", "image_url", "/static/mau/anh-1.jpg", "card", nil, "now", now,
			"reply_to_id", nil), messagesDump))
	add("create_message: a sticker id the check refuses", "IntegrityError",
		write("create_message", args("context_id", w.group, "author_id", w.owner, "kind", "sticker",
			"body", "KHÔNG", "image_url", nil, "card", nil, "now", now, "reply_to_id", nil), messagesDump))
	add("create_message: a human kind with no author", "IntegrityError",
		write("create_message", args("context_id", w.group, "author_id", nil, "kind", "text",
			"body", "Xin chào (dữ liệu mẫu)", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", nil), messagesDump))
	add("create_message: a missing group", "IntegrityError",
		write("create_message", args("context_id", w.missingContext, "author_id", w.owner, "kind", "text",
			"body", "Xin chào (dữ liệu mẫu)", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", nil), messagesDump))
	add("create_message: an author with no people row", "IntegrityError",
		write("create_message", args("context_id", w.group, "author_id", w.missingPerson, "kind", "text",
			"body", "Xin chào (dữ liệu mẫu)", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", nil), messagesDump))
	add("create_message: a reply into another group", "IntegrityError",
		write("create_message", args("context_id", w.group, "author_id", w.owner, "kind", "text",
			"body", "Trích nhóm kia (dữ liệu mẫu)", "image_url", nil, "card", nil, "now", now,
			"reply_to_id", mOther), messagesDump))
	add("list_messages: newest page", "",
		read("list_messages", args("context_id", w.group, "limit", 2, "before", nil, "after", nil), messagesDump))
	add("list_messages: before cursor", "",
		read("list_messages", args("context_id", w.group, "limit", 10, "before", []any{"2030-09-01T10:10:00Z", m3},
			"after", nil), messagesDump))
	add("list_messages: after cursor", "",
		read("list_messages", args("context_id", w.group, "limit", 10, "before", nil,
			"after", []any{"2030-09-01T10:00:00Z", m1}), messagesDump))
	add("list_destinations", "",
		read("list_destinations", args(), destinationsDump))
	add("list_place_photos: p-b", "",
		read("list_place_photos", args("place_id", "p-b"), photosDump))
	add("list_place_photos: empty", "",
		read("list_place_photos", args("place_id", "p-A"), photosDump))
	add("list_outing_memories: days of the trip", "",
		read("list_outing_memories", args("outing_id", oid, "limit", 10, "viewer_id", w.owner), memoriesDump))
	add("list_outing_memories: days of the trip without a viewer", "",
		read("list_outing_memories", args("outing_id", oid, "limit", 10, "viewer_id", nil), memoriesDump))
	add("list_outing_memories: unknown outing", "",
		read("list_outing_memories", args("outing_id", fid(kindOuting, 0xfe), "limit", 10, "viewer_id", nil), memoriesDump))
	add("group_photos_at_place", "",
		read("group_photos_at_place", args("place_id", "p-b", "viewer_id", w.owner, "limit", 10), memoriesDump))
	add("group_photos_at_place: a stranger", "",
		read("group_photos_at_place", args("place_id", "p-b", "viewer_id", w.stranger, "limit", 10), memoriesDump))
	add("set_membership_role: member to admin", "",
		write("set_membership_role", args("context_id", w.group, "person_id", w.mate, "role", "admin"), membershipsDump))
	add("set_membership_role: the role they already have", "",
		write("set_membership_role", args("context_id", w.group, "person_id", w.mate, "role", "member"), membershipsDump))
	add("set_membership_role: missing member", "",
		write("set_membership_role", args("context_id", w.group, "person_id", w.stranger, "role", "admin"), membershipsDump))
	add("set_membership_role: a role that does not exist", "ValueError",
		write("set_membership_role", args("context_id", w.group, "person_id", w.mate, "role", "owner"), membershipsDump))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

var waiMethods = []string{
	"create_message", "list_messages", "list_destinations", "list_place_photos",
	"list_outing_memories", "group_photos_at_place", "set_membership_role",
}

func TestWAIRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of wai_repo_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_wai_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "wai_repo_oracle_")

	cases, built := waiRepoOracleCases()
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_wai_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_wai_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	returned := map[string]int{}
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
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
			if s.Error == nil {
				returned[c.Steps[j].Call]++
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}
		pyCase := python.Cases[i]
		end := ""
		if len(pyCase.Steps) > 0 {
			if e, ok := pyCase.Steps[len(pyCase.Steps)-1].Error.(map[string]any); ok {
				end, _ = e["type"].(string)
				if code, ok := e["code"].(string); ok {
					end = code
				}
			}
		}
		if end != cases[i].wantEnd {
			t.Errorf("case %q: python ended in %q, the case is written for %q", c.Name, end, cases[i].wantEnd)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python stopped after %d steps, the case has %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}
		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, pySteps, runWAIGoCase(t, pool, c))
		})
	}
	for _, method := range waiMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	t.Logf("wai repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which "+
		"%d table rows, %d generated ids bound, %d mismatches", tally.cases, tally.steps, tally.results, tally.errors,
		tally.statements, tally.probeRows, tally.tableRows, tally.generated, tally.mismatches)
}
