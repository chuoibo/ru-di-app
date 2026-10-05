package profilemedia

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProfileVideoUsesOneCreditAndKeepsFilePrivate(t *testing.T) {
	pool, person := mediaFixture(t)
	playable, err := base64.StdEncoding.DecodeString(playableMP4Base64)
	if err != nil {
		t.Fatal(err)
	}
	imageID, err := NewJobID(person, testKey)
	if err != nil {
		t.Fatal(err)
	}
	proxyPosts := 0
	honorRange, sent := false, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			proxyPosts++
			w.WriteHeader(202)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/media/") && strings.HasSuffix(r.URL.Path, "/file") {
			w.Header().Set("Content-Type", "video/mp4")
			if honorRange {
				counting := &countingWriter{ResponseWriter: w}
				http.ServeContent(counting, r, "v.mp4", time.Time{}, bytes.NewReader(playable))
				sent += counting.n
				return
			}
			_, _ = w.Write(playable)
			return
		}
		_, _ = w.Write([]byte(`{"trang_thai":"xong","so_byte":1547}`))
	}))
	defer server.Close()
	h := New(pool, "dev", Proxy{URL: server.URL, Token: "proxy-secret", PersonKey: testKey, Client: server.Client()})
	request := func(method, path, actor string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Actor-ID", actor)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	input := map[string]any{"kind": "nep_video", "image_job_ids": []string{imageID}, "idempotency_key": "synthetic-click-one"}
	first := request("POST", "/me/profile-videos", person, input)
	if first.Code != 202 {
		t.Fatalf("create: %d %s", first.Code, first.Body.String())
	}
	var created struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil || !OwnsJob(created.JobID, person, testKey) {
		t.Fatalf("create job: %+v err=%v", created, err)
	}
	library := request("GET", "/me/profile-videos", person, nil)
	if library.Code != 200 || !bytes.Contains(library.Body.Bytes(), []byte(created.JobID)) {
		t.Fatalf("owner cannot find saved movie: %d %s", library.Code, library.Body.String())
	}
	foreignLibrary := request("GET", "/me/profile-videos", "00000000-0000-0000-0000-000000000002", nil)
	if foreignLibrary.Code != 200 || bytes.Contains(foreignLibrary.Body.Bytes(), []byte(created.JobID)) {
		t.Fatalf("other person sees movie: %d %s", foreignLibrary.Code, foreignLibrary.Body.String())
	}
	retry := request("POST", "/me/profile-videos", person, input)
	var retried struct {
		JobID string `json:"job_id"`
	}
	_ = json.Unmarshal(retry.Body.Bytes(), &retried)
	if retry.Code != 202 || retried.JobID != created.JobID || proxyPosts != 1 {
		t.Fatalf("retry: code=%d job=%q posts=%d", retry.Code, retried.JobID, proxyPosts)
	}
	foreign := request("GET", "/me/profile-videos/"+created.JobID, "00000000-0000-0000-0000-000000000002", nil)
	if foreign.Code != 404 {
		t.Fatalf("foreign status=%d", foreign.Code)
	}
	status := request("GET", "/me/profile-videos/"+created.JobID, person, nil)
	if status.Code != 200 || !bytes.Contains(status.Body.Bytes(), []byte(`"status":"ready"`)) {
		t.Fatalf("status: %d %s", status.Code, status.Body.String())
	}
	file := request("GET", "/me/profile-videos/"+created.JobID+"/file", person, nil)
	if file.Code != 200 || file.Header().Get("Content-Type") != "video/mp4" || !bytes.Contains(file.Body.Bytes(), []byte("ftyp")) {
		t.Fatalf("file: %d %q", file.Code, file.Body.Bytes())
	}
	rangeRequest := httptest.NewRequest(http.MethodGet, "/me/profile-videos/"+created.JobID+"/file", nil)
	rangeRequest.Header.Set("X-Actor-ID", person)
	rangeRequest.Header.Set("Range", "bytes=4-7")
	rangeReply := httptest.NewRecorder()
	h.ServeHTTP(rangeReply, rangeRequest)
	if rangeReply.Code != http.StatusPartialContent || rangeReply.Body.String() != "ftyp" {
		t.Fatalf("MP4 range: %d %q", rangeReply.Code, rangeReply.Body.String())
	}
	// An upstream that honours Range sends only what was asked for, and a
	// HEAD costs one byte (audit 2026-10-05, PER-PROFILE-01): the file is
	// passed through, never read whole into memory.
	honorRange = true
	sent = 0
	rangeReply = httptest.NewRecorder()
	h.ServeHTTP(rangeReply, rangeRequest)
	if rangeReply.Code != http.StatusPartialContent || rangeReply.Body.String() != "ftyp" || rangeReply.Header().Get("Content-Range") == "" {
		t.Fatalf("passed-through range: %d %q %v", rangeReply.Code, rangeReply.Body.String(), rangeReply.Header())
	}
	sent = 0
	head := httptest.NewRequest(http.MethodHead, "/me/profile-videos/"+created.JobID+"/file", nil)
	head.Header.Set("X-Actor-ID", person)
	headReply := httptest.NewRecorder()
	h.ServeHTTP(headReply, head)
	if headReply.Code != 200 || headReply.Body.Len() != 0 || headReply.Header().Get("Content-Length") != strconv.Itoa(len(playable)) || sent > 1 {
		t.Fatalf("HEAD: %d len=%q upstream sent %d bytes", headReply.Code, headReply.Header().Get("Content-Length"), sent)
	}
	spent := request("POST", "/me/profile-videos", person, map[string]any{"kind": "nep_video", "image_job_ids": []string{imageID}, "idempotency_key": "synthetic-click-two"})
	if spent.Code != 409 || proxyPosts != 1 {
		t.Fatalf("spent credit: %d %s posts=%d", spent.Code, spent.Body.String(), proxyPosts)
	}
}

func TestQueuedReservationCanRecoverAfterLostProxySubmission(t *testing.T) {
	pool, person := mediaFixture(t)
	imageID, _ := NewJobID(person, testKey)
	jobID, _ := NewJobID(person, testKey)
	input := map[string]any{"kind": "nep_video", "image_job_ids": []string{imageID}, "idempotency_key": "recover-click"}
	payload, _ := json.Marshal(createInput{Kind: "nep_video", ImageJobIDs: []string{imageID}, IdempotencyKey: "recover-click"})
	job, err := Reserve(context.Background(), pool, person, "recover-click", jobID, "nep_video", payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(context.Background(), pool, person, job.ID, "queued", "", ""); err != nil {
		t.Fatal(err)
	}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.NotFound(w, r)
			return
		}
		posts++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	h := New(pool, "dev", Proxy{URL: server.URL, Token: "proxy-secret", PersonKey: testKey, Client: server.Client()})
	data, _ := json.Marshal(input)
	r := httptest.NewRequest(http.MethodGet, "/me/profile-videos/"+job.ID, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Actor-ID", person)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || posts != 1 || !bytes.Contains(w.Body.Bytes(), []byte(job.ID)) {
		t.Fatalf("queued job was stranded: status=%d proxy posts=%d body=%s", w.Code, posts, w.Body.String())
	}
}

type countingWriter struct {
	http.ResponseWriter
	n int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += n
	return n, err
}
