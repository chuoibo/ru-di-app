package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"mobile/services/core/contract"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/media/sanitize"
	"mobile/services/core/internal/media/storage"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/ownership"
)

// The photo routes need a database for every answer that reaches a row, and
// the parity scenarios compare those on real stacks. These tests hold what can
// be shown without one: the order of refusals against the file and the
// transaction, the file store the request builds, and the answer's shape.

const (
	photoActorID = "5c0a1b2c-3d4e-4f5a-8b6c-7d8e9f0a1b2c"
	photoOtherID = "6d1b2c3d-4e5f-4a6b-9c7d-8e9f0a1b2c3d"
)

// photoRequest sends method target as photoActorID with roles; a non-nil
// content is one multipart `file` part.
func photoRequest(t *testing.T, h http.Handler, method, target, roles string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	contentType := ""
	if content != nil {
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("file", "mau.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		contentType = form.FormDataContentType()
	}
	req := httptest.NewRequest(method, target, &body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Actor-ID", photoActorID)
	req.Header.Set("X-Actor-Roles", roles)
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if v := recover(); v != nil && v != http.ErrAbortHandler {
				panic(v)
			}
		}()
		h.ServeHTTP(rec, req)
	}()
	return rec
}

func storedFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) (code, detail string) {
	t.Helper()
	var body struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%d %q: %v", rec.Code, rec.Body, err)
	}
	return body.Code, body.Detail
}

// A small PNG with varied alpha, generated here so no image bytes are
// committed.
func smallPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(37 * i)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPhotoRoutesAreRegisteredInManifestOrder(t *testing.T) {
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	group := map[string]bool{}
	for _, row := range manifest.Routes {
		if row.Group == "photos" {
			want = append(want, row.ID)
			group[row.ID] = true
		}
	}
	var got []string
	for _, route := range All() {
		if group[route.ID] {
			got = append(got, route.ID)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || len(want) != 6 {
		t.Fatalf("registered %v, manifest %v", got, want)
	}
}

// Every dependency in a Go route's tree must be one the endpoint stands in
// for; otherwise the route would fail on its first request, not at start.
func TestEveryRouteDependsOnlyOnWhatTheEndpointStandsIn(t *testing.T) {
	type node struct {
		Call         string `json:"call"`
		Dependencies []node `json:"dependencies"`
	}
	names, err := fs.Glob(contract.IR, "ir/*.json")
	if err != nil || len(names) == 0 {
		t.Fatalf("no IR files: %v", err)
	}
	trees := map[string]node{}
	for _, name := range names {
		raw, err := fs.ReadFile(contract.IR, name)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Routes []struct {
				ID        string `json:"id"`
				Dependant node   `json:"dependant"`
			} `json:"routes"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, route := range doc.Routes {
			trees[route.ID] = route.Dependant
		}
	}
	var walk func(id string, n node)
	walk = func(id string, n node) {
		for _, dependency := range n.Dependencies {
			if !endpoint.SupportedDependencies[dependency.Call] {
				t.Errorf("%s depends on %s, which has no Go stand-in", id, dependency.Call)
			}
			walk(id, dependency)
		}
	}
	for _, route := range All() {
		tree, ok := trees[route.ID]
		if !ok {
			t.Fatalf("%s is not in the contract IR", route.ID)
		}
		walk(route.ID, tree)
	}
}

// The permission is decided before the sanitizer, and every refusal of an
// upload comes before the file is written and before any statement.
func TestAnUploadRefusalWritesNoFileAndBeginsNoTransaction(t *testing.T) {
	root := t.TempDir()
	t.Setenv(storage.MediaRootEnv, root)
	h, units := groupCore(t, "photos")
	garbage := []byte("not a picture")
	over := make([]byte, sanitize.MaxUploadBytes+1)
	for _, tc := range []struct {
		name, target, roles string
		content             []byte
		status              int
		code, detail        string
	}{
		{"avatar of somebody else, garbage", "/people/" + photoOtherID + "/avatar", "member", garbage, 403, "permission_denied", "is_self"},
		{"own avatar without a permitted role", "/people/" + photoActorID + "/avatar", "guest", garbage, 403, "permission_denied", "role_not_permitted"},
		{"personal without a permitted role", "/people/me/photos", "guest", over, 403, "permission_denied", "role_not_permitted"},
		{"personal garbage", "/people/me/photos", "member", garbage, 415, "not_an_image", notAnImageDetail},
		{"personal empty file", "/people/me/photos", "member", []byte{}, 415, "not_an_image", notAnImageDetail},
		{"own avatar one byte over the cap", "/people/" + photoActorID + "/avatar", "member", over, 413, "image_too_large", "Image exceeds the 10485760-byte upload limit."},
	} {
		before := len(*units)
		rec := photoRequest(t, h, "POST", tc.target, tc.roles, tc.content)
		code, detail := problemOf(t, rec)
		if rec.Code != tc.status || code != tc.code || detail != tc.detail {
			t.Fatalf("%s: %d %s %q, want %d %s %q", tc.name, rec.Code, code, detail, tc.status, tc.code, tc.detail)
		}
		if files := storedFiles(t, root); len(files) != 0 {
			t.Fatalf("%s: a refusal left %v", tc.name, files)
		}
		for _, unit := range (*units)[before:] {
			if unit.Begun() {
				t.Fatalf("%s: a refusal began a transaction", tc.name)
			}
		}
	}
}

// Reading one's own photo or avatar asks no visibility question: the role
// refusal comes first, with no statement, and for a personal photo it is the
// gate's 404, never a 403.
func TestOwnReadsAreDecidedWithoutAStatement(t *testing.T) {
	t.Setenv(storage.MediaRootEnv, t.TempDir())
	h, units := groupCore(t, "photos")
	for _, tc := range []struct {
		target       string
		status       int
		code, detail string
	}{
		{"/people/" + photoActorID + "/photos/" + photoOtherID, 404, "photo_not_found", "Photo does not exist"},
		{"/people/" + photoActorID + "/avatar", 403, "permission_denied", "role_not_permitted"},
	} {
		before := len(*units)
		rec := photoRequest(t, h, "GET", tc.target, "guest", nil)
		code, detail := problemOf(t, rec)
		if rec.Code != tc.status || code != tc.code || detail != tc.detail {
			t.Fatalf("%s: %d %s %q, want %d %s %q", tc.target, rec.Code, code, detail, tc.status, tc.code, tc.detail)
		}
		for _, unit := range (*units)[before:] {
			if unit.Begun() {
				t.Fatalf("%s: began a transaction", tc.target)
			}
		}
	}
}

// The file is written before the row: with no database the INSERT fails and
// the file stays, at <root>/<k0k1>/<k2k3>/<key>, 0600, under the media root
// read for that request.
func TestAFailedInsertLeavesTheWrittenFileUnderThatRequestsRoot(t *testing.T) {
	h, _ := groupCore(t, "photos")
	for _, root := range []string{t.TempDir(), t.TempDir()} {
		t.Setenv(storage.MediaRootEnv, root)
		rec := photoRequest(t, h, "POST", "/people/me/photos", "member", smallPNG(t))
		if rec.Code != 500 {
			t.Fatalf("%d %q, want the INSERT's 500", rec.Code, rec.Body)
		}
		files := storedFiles(t, root)
		if len(files) != 1 {
			t.Fatalf("files under the request's root: %v", files)
		}
		rel, err := filepath.Rel(root, files[0])
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 3 || len(parts[2]) != 32 || parts[0] != parts[2][:2] || parts[1] != parts[2][2:4] {
			t.Fatalf("stored at %s", rel)
		}
		info, err := os.Stat(files[0])
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", info.Mode())
		}
	}
}

// get_photo_storage runs as a dependency: a media root that cannot be resolved
// fails the request after the actor and before the path, the role or the file
// is looked at.
func TestAnUnresolvableMediaRootFailsBeforeAnyRefusalOfTheRoute(t *testing.T) {
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	t.Setenv(storage.MediaRootEnv, loop)
	h, _ := groupCore(t, "photos")
	for _, target := range []string{"/people/me/photos", "/people/me/avatar"} {
		rec := photoRequest(t, h, "POST", target, "guest", []byte("x"))
		if rec.Code != 500 || rec.Body.String() != "Internal Server Error" {
			t.Fatalf("%s: %d %q", target, rec.Code, rec.Body)
		}
	}
}

func refusalOf(t *testing.T, err error) problem.Problem {
	t.Helper()
	var refusal *endpoint.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("%v is not a refusal", err)
	}
	return refusal.Problem
}

// _stored_image_bytes: missing and empty are the record's own 404, other
// failures are unhandled, and bytes arrive untouched with exactly three headers.
func TestAStoredImageIsItsBytesOrTheRecordsOwn404(t *testing.T) {
	photos, err := storage.NewAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	call := &endpoint.Call{Photos: photos}
	key := "ab12cd34ef56ab78cd90ef12ab34cd56"
	record := repo.UploadedImage{StorageKey: key, ContentType: "image/png"}
	notFound := problem.Problem{Status: 404, Code: "avatar_not_found", Detail: "Avatar does not exist"}

	_, err = storedImageReply(call, record, notFound.Code, notFound.Detail)
	if got := refusalOf(t, err); got != notFound {
		t.Fatalf("missing file: %+v", got)
	}
	if err := photos.Write(key, nil); err != nil {
		t.Fatal(err)
	}
	_, err = storedImageReply(call, record, notFound.Code, notFound.Detail)
	if got := refusalOf(t, err); got != notFound {
		t.Fatalf("empty file: %+v", got)
	}
	stored := []byte{0x89, 'P', 'N', 'G', 0}
	if err := photos.Write(key, stored); err != nil {
		t.Fatal(err)
	}
	reply, err := storedImageReply(call, record, notFound.Code, notFound.Detail)
	if err != nil || reply.Raw == nil {
		t.Fatalf("%+v %v", reply, err)
	}
	wantHeaders := [][2]string{{"cache-control", "private, max-age=300"}, {"content-length", "5"}, {"content-type", "image/png"}}
	if reply.Raw.Status != 200 || !bytes.Equal(reply.Raw.Body, stored) || !reflect.DeepEqual(reply.Raw.Headers, wantHeaders) {
		t.Fatalf("%d %v %q", reply.Raw.Status, reply.Raw.Headers, reply.Raw.Body)
	}

	var refusal *endpoint.Refusal
	if _, err := storedImageReply(call, repo.UploadedImage{StorageKey: "AB12CD34EF56AB78CD90EF12AB34CD56"}, "c", "d"); err == nil || errors.As(err, &refusal) {
		t.Fatalf("a malformed key is %v, want an unhandled failure", err)
	}
	directory := "cd34ef56ab78cd90ef12ab34cd56ab12"
	path, err := photos.PathFor(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := storedImageReply(call, repo.UploadedImage{StorageKey: directory}, "c", "d"); err == nil || errors.As(err, &refusal) {
		t.Fatalf("a directory is %v, want an unhandled failure", err)
	}
}

// _read_upload keeps MAX_UPLOAD_BYTES + 1 bytes: enough for the sanitizer to
// refuse, never the whole of a larger part.
func TestReadUploadKeepsOneByteMoreThanTheCap(t *testing.T) {
	limit := sanitize.MaxUploadBytes + 1
	for _, n := range []int{0, 7, sanitize.MaxUploadBytes, limit, limit + 1, 2 * sanitize.MaxUploadBytes} {
		want := n
		if want > limit {
			want = limit
		}
		if got := len(capUpload(make([]byte, n))); got != want {
			t.Fatalf("%d bytes kept %d, want %d", n, got, want)
		}
	}
}

// _image_rejection_problem, and what is not a refusal at all.
func TestSanitizerOutcomesAnswerAsPythonDoes(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want problem.Problem
	}{
		{&sanitize.Rejected{Code: "image_too_large", Detail: "a"}, problem.Problem{Status: 413, Code: "image_too_large", Detail: "a"}},
		{&sanitize.Rejected{Code: "image_dimensions_too_large", Detail: "b"}, problem.Problem{Status: 413, Code: "image_dimensions_too_large", Detail: "b"}},
		{&sanitize.Rejected{Code: "not_an_image", Detail: "c"}, problem.Problem{Status: 415, Code: "not_an_image", Detail: "c"}},
		{&sanitize.UnsupportedError{Format: "AVIF", Reason: "not ported"}, problem.Problem{Status: 415, Code: "not_an_image", Detail: notAnImageDetail}},
	} {
		_, err := sanitizeOutcome(sanitize.Sanitized{}, tc.err)
		if got := refusalOf(t, err); got != tc.want {
			t.Fatalf("%v: %+v, want %+v", tc.err, got, tc.want)
		}
	}
	var refusal *endpoint.Refusal
	for _, unhandled := range []error{
		&sanitize.EncodeError{Err: errors.New("save")},
		&sanitize.InternalError{Panic: "boom"},
		&sanitize.Rejected{Code: "image_unknown", Detail: "d"},
	} {
		if _, err := sanitizeOutcome(sanitize.Sanitized{}, unhandled); err == nil || errors.As(err, &refusal) {
			t.Fatalf("%v answered %v, want an unhandled failure", unhandled, err)
		}
	}
	sanitized := sanitize.Sanitized{Data: []byte{1}, ContentType: "image/jpeg", Width: 1, Height: 1}
	if got, err := sanitizeOutcome(sanitized, nil); err != nil || !reflect.DeepEqual(got, sanitized) {
		t.Fatalf("%+v %v", got, err)
	}
}

// UploadedImageResponse: keys in declaration order, null context_id, created_at
// as pydantic writes an aware UTC datetime.
func TestAnUploadAnswersUploadedImageResponse(t *testing.T) {
	contextID := photoOtherID
	created := time.Date(2026, 9, 16, 1, 2, 3, 4000, time.UTC)
	for _, tc := range []struct {
		record repo.UploadedImage
		url    string
		want   string
	}{
		{
			repo.UploadedImage{ID: photoActorID, ContextID: &contextID, ContentType: "image/jpeg", ByteSize: 1600, Width: 48, Height: 64, CreatedAt: created},
			"/contexts/" + contextID + "/photos/" + photoActorID,
			`{"id":"` + photoActorID + `","context_id":"` + contextID + `","url":"/contexts/` + contextID + `/photos/` + photoActorID +
				`","content_type":"image/jpeg","byte_size":1600,"width":48,"height":64,"created_at":"2026-09-16T01:02:03.000004Z"}`,
		},
		{
			repo.UploadedImage{ID: photoActorID, ContentType: "image/png", ByteSize: 77, Width: 3, Height: 2, CreatedAt: created.Truncate(time.Second)},
			"/people/" + photoOtherID + "/avatar",
			`{"id":"` + photoActorID + `","context_id":null,"url":"/people/` + photoOtherID +
				`/avatar","content_type":"image/png","byte_size":77,"width":3,"height":2,"created_at":"2026-09-16T01:02:03Z"}`,
		},
	} {
		got, err := pyjson.Compact(wireUploadedImage(tc.record, tc.url))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Fatalf("got  %s\nwant %s", got, tc.want)
		}
	}
}
