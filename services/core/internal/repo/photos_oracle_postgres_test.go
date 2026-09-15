//go:build postgres

package repo

// Differential test of the W6 repository methods (the photo routes) against
// the real SqlAlchemyApiRepository, driven through
// scripts/render_photo_repo_oracle.py. Same design as
// guests_oracle_postgres_test.go, whose case format, value tags, statement
// normalisation, error tagging, probes and comparison it reuses, on one
// migrated schema: every read here answers one row, none or a boolean, so no
// plan order can reach a result.
//
// The world is the social world (groups with members who left or were only
// invited, a friend graph with pending, declined and blocked edges both ways,
// an erased account) plus what the photo routes read: group photos in two
// groups; avatars whose created_at ties with ids against insertion order, or
// differs by a microsecond against id order; and personal photos each shown by
// one kind of post or story (every audience, a story live, at its deadline and
// expired, an only_me post together with a live story, nothing, a URL in
// another case, a post and a story by the reader showing somebody else's
// photo).
//
// The route cases run the calls each route makes, in its order, from the
// permission read to the row the route answers with. The file is the
// service's business and is proved by internal/media/storage.
//
// Beyond the comparison, TestPhotoRepositoryOracle requires Python to reach
// the INSERT, the post and the story statements, both answers of each boolean
// method (and the self answer that issues nothing), and both a record and None
// from each getter.
//
// Without CORE_PYTHON_IMAGE the test skips; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

const (
	kindPhoto      = 0x6b
	kindPhotoStory = 0x6d
)

// photoNow is the instant the visibility cases stand on: one story's deadline
// is exactly this.
const photoNow = "2030-06-10T12:00:00Z"

var uploadedImagesDump = orderedDump("uploaded_images t", fixtureFirst("t.id")+", t.created_at, t.storage_key, t.id")

// probePhotoRowLocks lists the row locks held on every table these methods
// read or reference through a foreign key.
var probePhotoRowLocks = func() string {
	var parts []string
	for _, table := range []string{"people", "contexts", "memberships", "uploaded_images", "posts", "stories", "friend_requests"} {
		parts = append(parts, `SELECT '`+table+`' AS rel, t.id::text AS id, array_to_string(r.modes, ',') AS modes
			FROM public.pgrowlocks('`+table+`') r LEFT JOIN `+table+` t ON t.ctid = r.locked_row`)
	}
	return `SELECT l.rel || ' ' || coalesce(l.id, '(superseded row)') || ' ' || l.modes FROM (` +
		strings.Join(parts, " UNION ALL ") + `) l ORDER BY 1`
}()

// ---------------------------------------------------------------------------
// The world
// ---------------------------------------------------------------------------

type photoWorld struct {
	*socialWorld

	gpG1, gpG2                                                             string
	avBinhOld, avBinhTieHigh, avBinhTieLow, avHoaLater, avHoaEarlier, avEm string
	avAnByBinh                                                             string

	ppFriends, ppPublic, ppOnlyMe, ppGroup, ppLiveStory, ppDeadline, ppExpired, ppPostAndStory  string
	ppUnshown, ppUpperURL, ppAuthoredByReader, ppStoryByReader, ppAnPublic, ppEmFriends, ppKhoa string

	postFriends, storyLive, missingPhoto string
}

// photoKey is a storage key shaped like new_storage_key's, built here rather
// than written out.
func photoKey(n int) string { return strings.Repeat(fmt.Sprintf("%02x", n), 16) }

func personURL(person, image string) string { return "/people/" + person + "/photos/" + image }

func (w *photoWorld) photo(n int, owner, context any, uploader, purpose, contentType string, size, width, height int64,
	createdAt string) string {
	id := fid(kindPhoto, n)
	w.insert("uploaded_images", "id", id, "storage_key", photoKey(n), "context_id", context, "owner_person_id", owner,
		"uploaded_by_id", uploader, "purpose", purpose, "content_type", contentType, "byte_size", size, "width", width,
		"height", height, "created_at", createdAt)
	return id
}

func (w *photoWorld) photoStory(n int, author, imageURL, createdAt, expiresAt string) string {
	id := fid(kindPhotoStory, n)
	w.insert("stories", "id", id, "author_id", author, "image_url", imageURL, "caption", nil, "audience", "friends",
		"created_at", createdAt, "expires_at", expiresAt)
	return id
}

func newPhotoWorld() *photoWorld {
	w := &photoWorld{socialWorld: newSocialWorld()}
	s := w.socialWorld

	w.gpG1 = w.photo(0x10, nil, s.g1, s.binh, "group", "image/png", 1, 1, 1, "2030-06-01T08:00:00.123456Z")
	w.gpG2 = w.photo(0x11, nil, s.g2, s.hoa, "group", "image/jpeg", math.MaxInt32, 8000, 6000, "2030-06-01T09:00:00Z")

	// binh: an old avatar, the social world's, and a created_at tie whose
	// larger id is inserted first. hoa: a microsecond apart, the later one
	// with the smaller id.
	w.avBinhOld = w.photo(0x20, s.binh, nil, s.binh, "avatar", "image/jpeg", 2048, 256, 256, "2030-02-01T00:00:00Z")
	w.avBinhTieHigh = w.photo(0x22, s.binh, nil, s.binh, "avatar", "image/png", 3072, 512, 512, "2030-03-05T08:00:00.25Z")
	w.avBinhTieLow = w.photo(0x21, s.binh, nil, s.binh, "avatar", "image/jpeg", 1024, 128, 128, "2030-03-05T08:00:00.25Z")
	w.avHoaLater = w.photo(0x30, s.hoa, nil, s.hoa, "avatar", "image/jpeg", 777, 64, 48, "2030-04-01T00:00:00.000001Z")
	w.avHoaEarlier = w.photo(0x31, s.hoa, nil, s.hoa, "avatar", "image/jpeg", 778, 64, 48, "2030-04-01T00:00:00Z")
	w.avEm = w.photo(0x32, s.em, nil, s.em, "avatar", "image/png", 999, 32, 32, "2030-03-10T00:00:00Z")
	w.avAnByBinh = w.photo(0x33, s.an, nil, s.binh, "avatar", "image/jpeg", 1500, 100, 100, "2030-03-11T00:00:00+07:00")

	personal := func(n int, owner string) string {
		return w.photo(n, owner, nil, owner, "personal", "image/jpeg", 4096, 1200, 900, fmt.Sprintf("2030-06-02T00:00:%02dZ", n-0x40))
	}
	w.ppFriends, w.ppPublic, w.ppOnlyMe = personal(0x40, s.binh), personal(0x41, s.binh), personal(0x42, s.binh)
	w.ppGroup, w.ppLiveStory, w.ppDeadline = personal(0x43, s.an), personal(0x44, s.binh), personal(0x45, s.binh)
	w.ppExpired, w.ppPostAndStory, w.ppUnshown = personal(0x46, s.binh), personal(0x47, s.binh), personal(0x48, s.binh)
	w.ppUpperURL, w.ppAuthoredByReader = personal(0x49, s.binh), personal(0x4a, s.binh)
	w.ppStoryByReader, w.ppAnPublic = personal(0x4b, s.binh), personal(0x4c, s.an)
	w.ppEmFriends, w.ppKhoa = personal(0x4d, s.em), personal(0x4e, s.khoa)
	w.missingPhoto = fid(kindPhoto, 0xfe)

	body := "Ảnh (dữ liệu mẫu)"
	at := func(n int) string { return fmt.Sprintf("2030-06-03T00:00:%02dZ", n-0x40) }
	w.postFriends = s.post(0x40, s.binh, "friends", nil, body, at(0x40), personURL(s.binh, w.ppFriends))
	s.post(0x41, s.binh, "public", nil, body, at(0x41), personURL(s.binh, w.ppPublic))
	s.post(0x42, s.binh, "only_me", nil, body, at(0x42), personURL(s.binh, w.ppOnlyMe))
	s.post(0x43, s.an, "group", s.g1, body, at(0x43), personURL(s.an, w.ppGroup))
	s.post(0x47, s.binh, "only_me", nil, body, at(0x47), personURL(s.binh, w.ppPostAndStory))
	s.post(0x49, s.binh, "public", nil, body, at(0x49), personURL(s.binh, strings.ToUpper(w.ppUpperURL)))
	s.post(0x4a, s.an, "only_me", nil, body, at(0x4a), personURL(s.binh, w.ppAuthoredByReader))
	s.post(0x4c, s.an, "public", nil, body, at(0x4c), personURL(s.an, w.ppAnPublic))
	s.post(0x4d, s.em, "friends", nil, body, at(0x4d), personURL(s.em, w.ppEmFriends))
	s.post(0x4e, s.binh, "public", nil, body, at(0x4e), personURL(s.binh, w.avBinhTieHigh))

	w.storyLive = w.photoStory(0x44, s.binh, personURL(s.binh, w.ppLiveStory), "2030-06-10T06:00:00Z", "2030-06-11T06:00:00Z")
	w.photoStory(0x45, s.binh, personURL(s.binh, w.ppDeadline), "2030-06-09T12:00:00Z", photoNow)
	w.photoStory(0x46, s.binh, personURL(s.binh, w.ppExpired), "2030-06-08T00:00:00Z", "2030-06-09T00:00:00Z")
	w.photoStory(0x47, s.binh, personURL(s.binh, w.ppPostAndStory), "2030-06-10T07:00:00Z", "2030-06-11T07:00:00Z")
	w.photoStory(0x4b, s.an, personURL(s.binh, w.ppStoryByReader), "2030-06-08T00:00:00Z", "2030-06-09T00:00:00Z")
	return w
}

// ---------------------------------------------------------------------------
// Tags and the Go side
// ---------------------------------------------------------------------------

func tUploadedImage(m UploadedImage) any {
	return tRecord("UploadedImageRecord", "id", tUUID(m.ID), "storage_key", tStr(m.StorageKey),
		"context_id", optional(m.ContextID, tUUID), "owner_person_id", optional(m.OwnerPersonID, tUUID),
		"uploaded_by_id", tUUID(m.UploadedByID), "content_type", tStr(m.ContentType), "byte_size", tInt(m.ByteSize),
		"width", tInt(m.Width), "height", tInt(m.Height), "created_at", tInstant(m.CreatedAt), "purpose", tStr(m.Purpose))
}

// argUploadedImage is create_uploaded_image's arguments, with Python's default
// purpose when the case leaves it out.
func argUploadedImage(a map[string]any) UploadedImageInput {
	purpose := "group"
	if given, ok := a["purpose"].(string); ok {
		purpose = given
	}
	return UploadedImageInput{StorageKey: argString(a, "storage_key"), ContextID: argText(a["context_id"]),
		OwnerPersonID: argText(a["owner_person_id"]), UploadedByID: argString(a, "uploaded_by_id"),
		ContentType: argString(a, "content_type"), ByteSize: argNumber(a["byte_size"]), Width: argNumber(a["width"]),
		Height: argNumber(a["height"]), Now: argInstant(argString(a, "now")), Purpose: purpose}
}

func photoGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	switch method {
	case "shares_active_context":
		shared, err := repo.SharesActiveContext(bg, s("viewer_id"), s("subject_id"))
		return tBool(shared), err
	case "create_uploaded_image":
		m, err := repo.CreateUploadedImage(bg, argUploadedImage(a))
		return tUploadedImage(m), err
	case "get_context_image":
		m, err := repo.GetContextImage(bg, s("context_id"), s("image_id"))
		return nilOr(m, tUploadedImage), err
	case "get_latest_avatar":
		m, err := repo.GetLatestAvatar(bg, s("person_id"))
		return nilOr(m, tUploadedImage), err
	case "person_image_visible_to":
		visible, err := repo.PersonImageVisibleTo(bg, s("person_id"), s("image_id"), s("reader_id"), argInstant(s("now")))
		return tBool(visible), err
	case "flow.upload_and_read_context_photo":
		m, err := repo.CreateUploadedImage(bg, argUploadedImage(a))
		if err != nil {
			return nil, err
		}
		read, err := repo.GetContextImage(bg, *m.ContextID, m.ID)
		return tSeq([]any{tUploadedImage(m), nilOr(read, tUploadedImage)}), err
	case "flow.upload_and_read_person_photo":
		m, err := repo.CreateUploadedImage(bg, argUploadedImage(a))
		if err != nil {
			return nil, err
		}
		read, err := repo.GetPersonImage(bg, *m.OwnerPersonID, m.ID)
		return tSeq([]any{tUploadedImage(m), nilOr(read, tUploadedImage)}), err
	case "get_person_image":
		// guestGoCall's chain (money, groups, pair, pilot) does not pass
		// through the W2 calls.
		return socialGoCall(repo, method, a)
	}
	return guestGoCall(repo, method, a)
}

func runPhotoGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
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
		value, err := photoGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = guestGoError(err)
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
			texts := []string{}
			for rows.Next() {
				var text string
				if err := rows.Scan(&text); err != nil {
					t.Fatal(err)
				}
				texts = append(texts, text)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			probes = append(probes, texts)
		}
		out["probes"] = probes
		steps = append(steps, generic(t, out))
	}
	return steps
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func photoOracleCases() ([]socialCase, oracleSpec) {
	w := newPhotoWorld()
	s := w.socialWorld
	var cases []socialCase
	add := func(name, wantEnd string, setup []string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps}, wantEnd})
	}
	probes := func(dumps ...string) []string {
		return append(append([]string{probeLocks, probeWrites, probePhotoRowLocks}, dumps...), probeNow)
	}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	read := func(method string, a map[string]any) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...), Probes: probes()}
	}
	write := func(method string, a map[string]any) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...), Probes: probes(uploadedImagesDump)}
	}
	// after runs sql ahead of a call, before the write counters' snapshot.
	after := func(call oracleCall, sql ...string) oracleCall {
		call.Before = append(append([]string{}, sql...), call.Before...)
		return call
	}
	later := func(n int) string { return fmt.Sprintf("2030-06-1%dT12:00:00.654321Z", n) }
	base := s.sql
	vietnam := join([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base)
	g := s.g1

	member := func(context, person string) oracleCall {
		return read("is_member", args("context_id", context, "person_id", person))
	}
	shares := func(viewer, subject string) oracleCall {
		return read("shares_active_context", args("viewer_id", viewer, "subject_id", subject))
	}
	contextImage := func(context, image string) oracleCall {
		return read("get_context_image", args("context_id", context, "image_id", image))
	}
	avatar := func(person string) oracleCall { return read("get_latest_avatar", args("person_id", person)) }
	personImage := func(person, image string) oracleCall {
		return read("get_person_image", args("person_id", person, "image_id", image))
	}
	visible := func(person, image, reader, now string) oracleCall {
		return read("person_image_visible_to", args("person_id", person, "image_id", image, "reader_id", reader, "now", now))
	}
	upload := func(n int, pairs ...any) map[string]any {
		out := args("storage_key", photoKey(0x80+n), "context_id", nil, "owner_person_id", nil, "content_type", "image/jpeg",
			"byte_size", 51234, "width", 1280, "height", 960, "now", photoNow)
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	create := func(n int, pairs ...any) oracleCall { return write("create_uploaded_image", upload(n, pairs...)) }
	deleteImage := func(id string) string { return "DELETE FROM uploaded_images WHERE id = '" + id + "'" }

	// --- shares_active_context ------------------------------------------------------
	add("shares_active_context: members of one group both ways, of the other group, blocked but together", "", base,
		shares(s.an, s.binh), shares(s.binh, s.an), shares(s.an, s.hoa), shares(s.phuong, s.an))
	add("shares_active_context: no active group in common", "", base,
		shares(s.binh, s.hoa), shares(s.dung, s.an), shares(s.dung, s.hoa), shares(s.hoa, s.dung), shares(s.em, s.an),
		shares(s.chi, s.binh))
	add("shares_active_context: the same person issues nothing, with or without a row", "", base,
		shares(s.an, s.an), shares(s.missingPerson, s.missingPerson), shares(s.missingPerson, s.an))

	// --- get_context_image ----------------------------------------------------------
	add("get_context_image: each group's photo, the social world's included", "", base,
		contextImage(g, w.gpG1), contextImage(s.g2, w.gpG2), contextImage(g, s.imgG1))
	add("get_context_image: another group's photo, an avatar, a personal photo, missing ids", "", base,
		contextImage(s.g2, w.gpG1), contextImage(g, w.avBinhTieHigh), contextImage(g, w.ppFriends),
		contextImage(g, w.missingPhoto), contextImage(s.missingContext, w.gpG1))
	add("get_context_image: a deleted row", "", base, after(contextImage(g, w.gpG1), deleteImage(w.gpG1)))
	add("get_context_image: under a Vietnam session TimeZone", "", vietnam, contextImage(g, w.gpG1), contextImage(s.g2, w.gpG2))

	// --- get_latest_avatar ----------------------------------------------------------
	add("get_latest_avatar: a created_at tie goes to the larger id, older avatars and newer personal photos lose", "", base,
		avatar(s.binh))
	add("get_latest_avatar: a microsecond later beats a larger id", "", base, avatar(s.hoa))
	add("get_latest_avatar: an erased person's, one uploaded by somebody else, none, no person", "", base,
		avatar(s.em), avatar(s.an), avatar(s.khoa), avatar(s.missingPerson))
	add("get_latest_avatar: the newest deleted leaves the other row of the tie", "", base,
		after(avatar(s.binh), deleteImage(w.avBinhTieHigh)))
	add("get_latest_avatar: under a Vietnam session TimeZone", "", vietnam, avatar(s.binh), avatar(s.an))

	// --- person_image_visible_to ----------------------------------------------------
	add("person_image_visible_to: a friends post shows the photo to the author's friends only", "", base,
		visible(s.binh, w.ppFriends, s.an, photoNow), visible(s.binh, w.ppFriends, s.giang, photoNow),
		visible(s.binh, w.ppFriends, s.khoa, photoNow), visible(s.binh, w.ppFriends, s.chi, photoNow),
		visible(s.binh, w.ppFriends, s.phuong, photoNow), visible(s.binh, w.ppFriends, s.hoa, photoNow),
		visible(s.binh, w.ppFriends, s.em, photoNow), visible(s.binh, w.ppFriends, s.missingPerson, photoNow))
	add("person_image_visible_to: a public post shows it to everybody but a reader blocked either way", "", base,
		visible(s.binh, w.ppPublic, s.chi, photoNow), visible(s.binh, w.ppPublic, s.phuong, photoNow),
		visible(s.binh, w.ppPublic, s.missingPerson, photoNow), visible(s.an, w.ppAnPublic, s.phuong, photoNow),
		visible(s.an, w.ppAnPublic, s.binh, photoNow))
	add("person_image_visible_to: an only_me post shows it to its author alone", "", base,
		visible(s.binh, w.ppOnlyMe, s.an, photoNow), visible(s.binh, w.ppOnlyMe, s.binh, photoNow))
	add("person_image_visible_to: a group post shows it to active members, blocked or not", "", base,
		visible(s.an, w.ppGroup, s.binh, photoNow), visible(s.an, w.ppGroup, s.phuong, photoNow),
		visible(s.an, w.ppGroup, s.dung, photoNow), visible(s.an, w.ppGroup, s.hoa, photoNow))
	add("person_image_visible_to: a live story shows it to friends who are not blocked", "", base,
		visible(s.binh, w.ppLiveStory, s.an, photoNow), visible(s.binh, w.ppLiveStory, s.phuong, photoNow),
		visible(s.binh, w.ppLiveStory, s.chi, photoNow))
	add("person_image_visible_to: a story at its deadline is gone, a microsecond before it is not", "", base,
		visible(s.binh, w.ppDeadline, s.an, photoNow), visible(s.binh, w.ppDeadline, s.an, "2030-06-10T11:59:59.999999Z"),
		visible(s.binh, w.ppDeadline, s.an, "2030-06-10T18:59:59.999999"+"99+07:00"))
	add("person_image_visible_to: an expired story, to a friend and to its author", "", base,
		visible(s.binh, w.ppExpired, s.an, photoNow), visible(s.binh, w.ppExpired, s.binh, photoNow))
	add("person_image_visible_to: an only_me post and a live story, the story decides", "", base,
		visible(s.binh, w.ppPostAndStory, s.an, photoNow), visible(s.binh, w.ppPostAndStory, s.chi, photoNow))
	add("person_image_visible_to: shown by nothing, a URL in another case, missing ids, another owner's path", "", base,
		visible(s.binh, w.ppUnshown, s.an, photoNow), visible(s.binh, w.ppUpperURL, s.chi, photoNow),
		visible(s.missingPerson, w.missingPhoto, s.an, photoNow), visible(s.an, w.ppFriends, s.an, photoNow))
	add("person_image_visible_to: a post and an expired story by the reader showing somebody else's photo", "", base,
		visible(s.binh, w.ppAuthoredByReader, s.an, photoNow), visible(s.binh, w.ppAuthoredByReader, s.chi, photoNow),
		visible(s.binh, w.ppStoryByReader, s.an, photoNow))
	add("person_image_visible_to: an erased author's friends post, to that author's friend", "", base,
		visible(s.em, w.ppEmFriends, s.an, photoNow))
	add("person_image_visible_to: the post and then the story deleted", "", base,
		after(visible(s.binh, w.ppFriends, s.an, photoNow), "DELETE FROM posts WHERE id = '"+w.postFriends+"'"),
		after(visible(s.binh, w.ppLiveStory, s.an, photoNow), "DELETE FROM stories WHERE id = '"+w.storyLive+"'"))
	add("person_image_visible_to: under a Vietnam session TimeZone", "", vietnam,
		visible(s.binh, w.ppDeadline, s.an, "2030-06-10T18:59:59.999999+07:00"), visible(s.binh, w.ppLiveStory, s.an, photoNow))

	// --- create_uploaded_image ------------------------------------------------------
	add("create_uploaded_image: a group photo with purpose left to its default", "", base,
		create(1, "context_id", g, "uploaded_by_id", s.an))
	add("create_uploaded_image: a group photo with purpose given", "", base,
		create(2, "context_id", g, "uploaded_by_id", s.binh, "purpose", "group", "content_type", "image/png"))
	add("create_uploaded_image: an avatar, then a personal photo", "", base,
		create(3, "owner_person_id", s.binh, "uploaded_by_id", s.binh, "purpose", "avatar"),
		create(4, "owner_person_id", s.an, "uploaded_by_id", s.an, "purpose", "personal", "now", later(1)))
	add("create_uploaded_image: the largest sizes an INTEGER holds, an owner uploaded by somebody else", "", base,
		create(5, "owner_person_id", s.khoa, "uploaded_by_id", s.hoa, "purpose", "avatar", "byte_size", math.MaxInt32,
			"width", math.MaxInt32, "height", 1))
	add("create_uploaded_image: a clock past microseconds at another offset, under a Vietnam session TimeZone", "", vietnam,
		create(6, "context_id", s.g2, "uploaded_by_id", s.hoa, "now", "2030-06-10T19:00:00.654321"+"99+07:00"))
	add("create_uploaded_image: for an erased person", "", base,
		create(7, "owner_person_id", s.em, "uploaded_by_id", s.em, "purpose", "personal"))
	add("create_uploaded_image: a content type the CHECK refuses", "IntegrityError", base,
		create(8, "context_id", g, "uploaded_by_id", s.an, "content_type", "image/gif"))
	add("create_uploaded_image: a byte size of zero", "IntegrityError", base,
		create(9, "context_id", g, "uploaded_by_id", s.an, "byte_size", 0))
	add("create_uploaded_image: a width of zero", "IntegrityError", base,
		create(10, "context_id", g, "uploaded_by_id", s.an, "width", 0))
	add("create_uploaded_image: both a group and an owner", "IntegrityError", base,
		create(11, "context_id", g, "owner_person_id", s.an, "uploaded_by_id", s.an, "purpose", "avatar"))
	add("create_uploaded_image: neither a group nor an owner", "IntegrityError", base,
		create(12, "uploaded_by_id", s.an))
	add("create_uploaded_image: an empty purpose", "IntegrityError", base,
		create(13, "context_id", g, "uploaded_by_id", s.an, "purpose", ""))
	add("create_uploaded_image: an avatar in a group", "IntegrityError", base,
		create(14, "context_id", g, "uploaded_by_id", s.an, "purpose", "avatar"))
	add("create_uploaded_image: a group photo with an owner and no group", "IntegrityError", base,
		create(15, "owner_person_id", s.an, "uploaded_by_id", s.an, "purpose", "group"))
	add("create_uploaded_image: a purpose past its column", "DataError", base,
		create(16, "owner_person_id", s.an, "uploaded_by_id", s.an, "purpose", "personals"))
	add("create_uploaded_image: an owner and uploader with no people row", "IntegrityError", base,
		create(17, "owner_person_id", s.missingPerson, "uploaded_by_id", s.missingPerson, "purpose", "avatar"))
	add("create_uploaded_image: an uploader with no people row", "IntegrityError", base,
		create(18, "owner_person_id", s.an, "uploaded_by_id", s.missingPerson, "purpose", "personal"))
	add("create_uploaded_image: a group with no row", "IntegrityError", base,
		create(19, "context_id", s.missingContext, "uploaded_by_id", s.an))
	add("create_uploaded_image: a storage key already stored", "IntegrityError", base,
		create(20, "context_id", g, "uploaded_by_id", s.an, "storage_key", photoKey(0x10)))

	// --- sequences ------------------------------------------------------------------
	add("flow.upload_and_read_context_photo: the row reads back in the same transaction", "", base,
		write("flow.upload_and_read_context_photo", upload(22, "context_id", g, "uploaded_by_id", s.phuong)))
	add("flow.upload_and_read_person_photo: a personal photo reads back, an avatar does not", "", base,
		write("flow.upload_and_read_person_photo", upload(23, "owner_person_id", s.binh, "uploaded_by_id", s.binh, "purpose", "personal")),
		write("flow.upload_and_read_person_photo", upload(24, "owner_person_id", s.binh, "uploaded_by_id", s.binh, "purpose", "avatar",
			"now", later(1))))
	add("an avatar set twice replaces, one set with an earlier clock does not", "", base,
		create(25, "owner_person_id", s.binh, "uploaded_by_id", s.binh, "purpose", "avatar", "now", later(1)), avatar(s.binh),
		create(26, "owner_person_id", s.binh, "uploaded_by_id", s.binh, "purpose", "avatar", "now", later(2)), avatar(s.binh),
		create(27, "owner_person_id", s.hoa, "uploaded_by_id", s.hoa, "purpose", "avatar", "now", "2030-01-01T00:00:00Z"),
		avatar(s.hoa))

	// --- the calls each photo route makes, in its order ---------------------------------
	add("POST /contexts/{context_id}/photos by a member", "", base,
		member(g, s.an), create(30, "context_id", g, "uploaded_by_id", s.an))
	add("POST /contexts/{context_id}/photos by a member who left, and by one only invited", "", base,
		member(g, s.dung), member(s.g2, s.dung))
	add("GET /contexts/{context_id}/photos/{photo_id} by a member", "", base, member(g, s.binh), contextImage(g, w.gpG1))
	add("GET /contexts/{context_id}/photos/{photo_id} of another group's photo", "", base,
		member(g, s.an), contextImage(g, w.gpG2))
	add("GET /contexts/{context_id}/photos/{photo_id} by a member of another group", "", base, member(g, s.hoa))
	add("GET /contexts/{context_id}/photos/{photo_id} of an unknown photo", "", base,
		member(g, s.an), contextImage(g, w.missingPhoto))
	add("POST /people/{person_id}/avatar", "", base,
		create(31, "owner_person_id", s.binh, "uploaded_by_id", s.binh, "purpose", "avatar"))
	add("POST /people/{person_id}/avatar by an actor with no people row", "IntegrityError", base,
		create(32, "owner_person_id", s.missingPerson, "uploaded_by_id", s.missingPerson, "purpose", "avatar"))
	add("GET /people/{person_id}/avatar by a groupmate", "", base, shares(s.an, s.binh), avatar(s.binh))
	add("GET /people/{person_id}/avatar by oneself", "", base, shares(s.binh, s.binh), avatar(s.binh))
	add("GET /people/{person_id}/avatar by a stranger", "", base, shares(s.chi, s.binh))
	add("GET /people/{person_id}/avatar of a groupmate who has none", "", base, shares(s.an, s.phuong), avatar(s.phuong))
	add("POST /people/me/photos", "", base,
		create(33, "owner_person_id", s.an, "uploaded_by_id", s.an, "purpose", "personal"))
	add("POST /people/me/photos by an actor with no people row", "IntegrityError", base,
		create(34, "owner_person_id", s.missingPerson, "uploaded_by_id", s.missingPerson, "purpose", "personal"))
	add("GET /people/{person_id}/photos/{photo_id} by the owner", "", base, personImage(s.binh, w.ppFriends))
	add("GET /people/{person_id}/photos/{photo_id} by a friend a friends post shows it to", "", base,
		visible(s.binh, w.ppFriends, s.an, photoNow), personImage(s.binh, w.ppFriends))
	add("GET /people/{person_id}/photos/{photo_id} by a stranger", "", base, visible(s.binh, w.ppFriends, s.chi, photoNow))
	add("GET /people/{person_id}/photos/{photo_id} through a live story", "", base,
		visible(s.binh, w.ppLiveStory, s.an, photoNow), personImage(s.binh, w.ppLiveStory))
	add("GET /people/{person_id}/photos/{photo_id} of an avatar a public post shows", "", base,
		visible(s.binh, w.avBinhTieHigh, s.chi, photoNow), personImage(s.binh, w.avBinhTieHigh))
	add("GET /people/{person_id}/photos/{photo_id} of a group photo under a person's path", "", base,
		visible(s.binh, w.gpG1, s.an, photoNow))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// photoMethods is every method this port covers or the routes reach; the
// corpus must reach each with at least one normal return.
var photoMethods = []string{
	"is_member", "shares_active_context", "create_uploaded_image", "get_context_image", "get_latest_avatar",
	"get_person_image", "person_image_visible_to", "flow.upload_and_read_context_photo", "flow.upload_and_read_person_photo",
}

// photoStatements are statement prefixes the corpus must make Python issue.
var photoStatements = []struct{ call, prefix string }{
	{"create_uploaded_image", "INSERT INTO uploaded_images "},
	{"person_image_visible_to", "SELECT posts.id FROM posts "},
	{"person_image_visible_to", "SELECT stories.id FROM stories "},
	{"shares_active_context", "SELECT EXISTS (SELECT memberships_1.id "},
	{"get_latest_avatar", "SELECT uploaded_images.id, "},
	{"get_context_image", "SELECT uploaded_images.id, "},
}

// photoAnswers are answers the corpus must make Python give: a call and the
// JSON of its tagged result, "record" for any record, with the number of
// statements the step issued (-1 for any).
var photoAnswers = []struct {
	call, result string
	statements   int
}{
	{"shares_active_context", `{"bool":true}`, 0},
	{"shares_active_context", `{"bool":true}`, 1},
	{"shares_active_context", `{"bool":false}`, 1},
	{"person_image_visible_to", `{"bool":true}`, 1},
	{"person_image_visible_to", `{"bool":true}`, 2},
	{"person_image_visible_to", `{"bool":false}`, 2},
	{"get_context_image", "record", -1},
	{"get_context_image", "null", -1},
	{"get_latest_avatar", "record", -1},
	{"get_latest_avatar", "null", -1},
	{"get_person_image", "record", -1},
	{"get_person_image", "null", -1},
}

func TestPhotoRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of photos_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_photo_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "photo_oracle_")

	cases, built := photoOracleCases()
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_photo_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_photo_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
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
	issued := make([]int, len(photoStatements))
	answered := make([]int, len(photoAnswers))
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
			call := c.Steps[j].Call
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statement := normalizeSQL(entry[0].(string))
					statements = append(statements, statement)
					for b, want := range photoStatements {
						if call == want.call && strings.HasPrefix(statement, want.prefix) {
							issued[b]++
						}
					}
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			if s.Error == nil {
				returned[call]++
				result, _ := json.Marshal(s.Result)
				for a, want := range photoAnswers {
					got := string(result)
					if record, ok := s.Result.(map[string]any); ok && record["record"] != nil {
						got = "record"
					}
					if call == want.call && got == want.result && (want.statements < 0 || want.statements == len(statements)) {
						answered[a]++
					}
				}
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}

		// The fixture must reach what the case is named for.
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
		if len(pyCase.Steps) != len(c.Steps) && end == "" {
			t.Errorf("case %q: python ran %d of %d steps", c.Name, len(pyCase.Steps), len(c.Steps))
		}
		if end != cases[i].wantEnd {
			t.Errorf("case %q: python ended in %q, the case is written for %q", c.Name, end, cases[i].wantEnd)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python refused at step %d of %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}

		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, normalizeMoneySteps(pySteps, c), normalizeMoneySteps(runPhotoGoCase(t, pool, c), c))
		})
	}
	for _, method := range photoMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	for b, want := range photoStatements {
		if issued[b] == 0 {
			t.Errorf("no case makes python issue, from %s: %s...", want.call, want.prefix)
		}
	}
	for a, want := range photoAnswers {
		if answered[a] == 0 {
			t.Errorf("no case makes python answer %s from %s with %d statements", want.result, want.call, want.statements)
		}
	}
	t.Logf("photo repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which "+
		"%d table rows, %d generated ids bound, %d required answers reached, %d mismatches", tally.cases, tally.steps,
		tally.results, tally.errors, tally.statements, tally.probeRows, tally.tableRows, tally.generated,
		len(photoAnswers), tally.mismatches)
}
