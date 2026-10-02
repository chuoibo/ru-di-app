//go:build postgres && communitymedia

package community

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/congdong"
	"mobile/services/core/internal/media/storage"
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

// reviewCut uploads a synthetic video of the given seconds, processes it and
// answers its review pieces' frame counts and heights.
func reviewCut(t *testing.T, f world, seconds int) (frames, heights []int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	if output, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=1280x720:d=%d", seconds), "-f", "lavfi", "-i", fmt.Sprintf("sine=d=%d", seconds), "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", path).CombinedOutput(); err != nil {
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
	var keys []string
	if err = f.pool.QueryRow(context.Background(), `SELECT review_keys FROM community_media WHERE id=$1 AND state='ready'`, media.ID).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	st, err := storage.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		piece, err := st.PathFor(key)
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("ffprobe", "-v", "error", "-count_frames", "-select_streams", "v:0", "-show_entries", "stream=nb_read_frames,height", "-of", "csv=p=0", piece).Output()
		if err != nil {
			t.Fatal(err)
		}
		var n, h int
		if _, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%d,%d", &h, &n); err != nil {
			t.Fatalf("%q: %v", out, err)
		}
		frames, heights = append(frames, n), append(heights, h)
	}
	return frames, heights
}

// A processed video carries its review cut: a piece per congdong.GiayDoan
// seconds at one frame a second, 360 pixels high, which is what a reading
// sends the model.
func TestPostgresCommunityVideoReviewCut(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	frames, heights := reviewCut(t, f, 50)
	if len(frames) != 2 || frames[0] != congdong.GiayDoan || frames[1] != 5 || heights[0] != 360 || heights[1] != 360 {
		t.Fatalf("50 s video cut into frames %v at heights %v", frames, heights)
	}
}
