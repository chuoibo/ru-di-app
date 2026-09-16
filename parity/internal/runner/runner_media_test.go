package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mobile/parity/internal/mediasnap"
	"mobile/parity/internal/scenario"
)

// photoAPI stores every upload under a random key in root, the way
// PhotoStorage.write lays it out, and answers 201 with a photo id. A port that
// drops the file answers the same; reading a photo answers the same whether
// its file is there or not, so only the store can tell the two apart.
func photoAPI(t *testing.T, root string, dropFile bool) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == "POST" && r.URL.Path == "/photos":
			var b [16]byte
			_, _ = rand.Read(b[:])
			key := hex.EncodeToString(b[:])
			dir := filepath.Join(root, key[:2], key[2:4])
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Error(err)
			}
			for _, d := range []string{filepath.Join(root, key[:2]), dir} {
				_ = os.Chmod(d, 0o755)
			}
			path := filepath.Join(dir, key)
			if err := os.WriteFile(path, []byte("jpeg bytes"), 0o600); err != nil {
				t.Error(err)
			}
			if dropFile {
				_ = os.Remove(path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = fmt.Fprintf(w, `{"id":"%s"}`, uuid4())
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/photos/"):
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg bytes"))
		default:
			w.WriteHeader(405)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

const photoScript = `
id: fake/upload-then-read
routes: ["POST /photos", "GET /photos/{photo_id}"]
auth_mode: dev
personas: {owner: {}}
steps:
  - id: upload
    as: owner
    request: {method: POST, path: /photos, body_raw: 'x'}
    bind: {photo: {from: body, pointer: /id, class: uuid}}
  - id: read
    as: owner
    request: {method: GET, path: "/photos/{{photo}}"}
`

func photoRun(t *testing.T, name string, dropFile, lane bool) *Run {
	t.Helper()
	sc, err := scenario.Parse([]byte(photoScript))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	s := stack(t, name, photoAPI(t, root, dropFile))
	if lane {
		s.Media = root
	}
	run, err := Execute(context.Background(), sc, s, "t1")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestAFileDroppedBehindAnEqualAnswerIsAMediaDifference(t *testing.T) {
	reference := photoRun(t, "reference", false, true)
	if diffs := Diff(reference, photoRun(t, "candidate", false, true)); len(diffs) != 0 {
		t.Fatalf("faithful stores differ: %+v", diffs)
	}
	diffs := Diff(reference, photoRun(t, "candidate", true, true))
	if len(diffs) != 1 || diffs[0].StepID != "upload" || len(diffs[0].Differences) != 0 || len(diffs[0].Media) == 0 {
		t.Fatalf("diffs = %+v", diffs)
	}
	// The reference stored a file; the candidate only modified a key
	// directory, which no row or answer ties to a key.
	d := diffs[0].Media[0]
	if d.Kind != mediasnap.KindCreated || len(d.OnlyReference) != 1 || !strings.Contains(d.OnlyReference[0], `"place":"key"`) ||
		len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], `"place":"keydir","type":"dir","path":"<hex2>/<hex2>"`) {
		t.Errorf("media difference reads %q", d.String())
	}
	// Without the lane the same candidate reads as equal: that is the hole.
	if diffs := Diff(photoRun(t, "reference", false, false), photoRun(t, "candidate", true, false)); len(diffs) != 0 {
		t.Fatalf("fixture: the wire alone already sees the dropped file: %+v", diffs)
	}
}

func TestAMediaLaneOnOneSideOnlyIsADifference(t *testing.T) {
	diffs := Diff(photoRun(t, "reference", false, true), photoRun(t, "candidate", false, false))
	if len(diffs) != 2 || diffs[0].Media[0].Kind != MediaLaneMismatch {
		t.Fatalf("diffs = %+v", diffs)
	}
}

// A store the harness cannot read stops the run with an error the canary can
// tell from damage it caught.
func TestAnUnreadableStoreIsAnErrorNotADifference(t *testing.T) {
	sc, err := scenario.Parse([]byte(photoScript))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	s := stack(t, "candidate", photoAPI(t, root, false))
	s.Media = filepath.Join(root, "missing")
	if _, err := Execute(context.Background(), sc, s, "t1"); !errors.Is(err, mediasnap.ErrSnapshot) {
		t.Fatalf("err = %v", err)
	}
}
