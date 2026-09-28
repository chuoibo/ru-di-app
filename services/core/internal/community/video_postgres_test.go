//go:build postgres && communitymedia

package community

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPostgresCommunityVideoTranscodeAndAuthorizedRange(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	if output, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1920x1080:d=1", "-c:v", "libx264", "-threads", "2", "-pix_fmt", "yuv420p", path).CombinedOutput(); err != nil {
		t.Fatalf("synthetic video: %v %s", err, output)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/v2/community/media", bytes.NewReader(b))
	r.Header.Set("Content-Type", "video/mp4")
	r.Header.Set("Authorization", "Bearer "+f.tokens[0])
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	requireStatus(t, w, 201)
	var media Media
	if err = json.Unmarshal(w.Body.Bytes(), &media); err != nil {
		t.Fatal(err)
	}
	f.h.processVideo(context.Background())
	var state string
	var width, height int
	if err = f.pool.QueryRow(context.Background(), `SELECT state,width,height FROM community_media WHERE id=$1`, media.ID).Scan(&state, &width, &height); err != nil || state != "ready" || width != 1280 || height != 720 {
		t.Fatalf("state=%s width=%d height=%d err=%v", state, width, height, err)
	}
	r = httptest.NewRequest("GET", "/v2/community/media/"+media.ID, nil)
	r.Header.Set("Authorization", "Bearer "+f.tokens[0])
	r.Header.Set("Range", "bytes=0-15")
	w = httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	requireStatus(t, w, 206)
	if len(w.Body.Bytes()) != 16 || w.Header().Get("Content-Type") != "video/mp4" {
		t.Fatal("invalid ranged video")
	}
	requireStatus(t, f.call("GET", "/v2/community/media/"+media.ID, 1, nil), 404)
}
