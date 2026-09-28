package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestS3FramesSignsAndMapsStatus. The request goes to the path-style object
// URL with the key percent-encoded, carries a SigV4 Authorization for the
// right access key, and a 404 is "not uploaded yet" rather than a failure.
func TestS3FramesSignsAndMapsStatus(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.EscapedPath(), r.Header.Get("Authorization")
		if strings.HasSuffix(r.URL.Path, "missing.jpg") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("jpeg"))
	}))
	defer server.Close()
	frames := S3Frames{Endpoint: server.URL, Bucket: "vnlocal-frames", AccessKey: "rudi-core", SecretKey: "s"}

	data, err := frames.Read(context.Background(), "plc_ab/7684 cover.jpg")
	if err != nil || string(data) != "jpeg" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if gotPath != "/vnlocal-frames/plc_ab/7684%20cover.jpg" {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=rudi-core/") ||
		!strings.Contains(gotAuth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Errorf("authorization = %s", gotAuth)
	}

	if _, err := frames.Read(context.Background(), "plc_ab/missing.jpg"); !errors.Is(err, ErrFrameMissing) {
		t.Errorf("404 = %v, want ErrFrameMissing", err)
	}
	if _, err := frames.Read(context.Background(), "../etc/passwd"); !errors.Is(err, ErrFrameRefused) {
		t.Errorf("climbing key = %v, want ErrFrameRefused", err)
	}
}

// TestDirFramesStaysInsideItsRoot. A key is a name under the frame directory;
// one that climbs out is refused rather than followed.
func TestDirFramesStaysInsideItsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plc_a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plc_a", "f.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	frames := DirFrames{Root: root}
	if data, err := frames.Read(context.Background(), "plc_a/f.jpg"); err != nil || string(data) != "x" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if _, err := frames.Read(context.Background(), "plc_a/none.jpg"); !errors.Is(err, ErrFrameMissing) {
		t.Errorf("missing = %v", err)
	}
	// Clean("/" + key) pins the key under the root, so a climb resolves inside.
	if _, err := frames.Read(context.Background(), "../../etc/passwd"); !errors.Is(err, ErrFrameMissing) {
		t.Errorf("climb = %v, want it resolved inside the root and missing", err)
	}
}
