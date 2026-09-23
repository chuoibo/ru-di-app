package profilemedia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateVideoSendsOnlyOwnedImageJobsToProxy(t *testing.T) {
	imageID, err := NewJobID(testPerson, testKey)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/media/video" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer proxy-secret" {
			t.Error("proxy token missing")
		}
		var body struct {
			JobID       string   `json:"job_id"`
			Tenant      string   `json:"tenant"`
			ImageJobIDs []string `json:"anh_job_ids"`
			Seconds     *int     `json:"giay_moi_anh"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !OwnsJob(body.JobID, testPerson, testKey) || body.Tenant != body.JobID[:tokenLength] || len(body.ImageJobIDs) != 1 || body.ImageJobIDs[0] != imageID || (seen == 1 && (body.Seconds == nil || *body.Seconds != 3)) {
			t.Errorf("unexpected video payload %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	proxy := Proxy{URL: server.URL, Token: "proxy-secret", PersonKey: testKey, Client: server.Client()}
	seconds := 3
	jobID, err := proxy.GenerateVideo(context.Background(), testPerson, []string{imageID}, &seconds)
	if err != nil || !OwnsJob(jobID, testPerson, testKey) || seen != 1 {
		t.Fatalf("job=%q err=%v calls=%d", jobID, err, seen)
	}
	foreignID, err := NewJobID("00000000-0000-0000-0000-000000000002", testKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proxy.GenerateVideo(context.Background(), testPerson, []string{foreignID}, nil); err == nil || seen != 1 {
		t.Fatalf("foreign image job was sent to proxy: err=%v calls=%d", err, seen)
	}
	if err := proxy.SubmitVideo(context.Background(), testPerson, jobID, []string{imageID}, nil); err != nil || seen != 2 {
		t.Fatalf("durable reserved ID was not used: err=%v calls=%d", err, seen)
	}
}

func TestMediaStatusAndFileStayBoundToOwner(t *testing.T) {
	jobID, err := NewJobID(testPerson, testKey)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		if r.Header.Get("Authorization") != "Bearer proxy-secret" {
			t.Error("proxy token missing")
		}
		if r.URL.Path == "/v1/media/"+jobID {
			_, _ = w.Write([]byte(`{"trang_thai":"xong","so_byte":4,"internal_secret":"never forward"}`))
			return
		}
		if r.URL.Path == "/v1/media/"+jobID+"/file" {
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("mp4!"))
			return
		}
		t.Errorf("unexpected path %s", r.URL.Path)
	}))
	defer server.Close()
	proxy := Proxy{URL: server.URL, Token: "proxy-secret", PersonKey: testKey, Client: server.Client()}
	status, err := proxy.Status(context.Background(), testPerson, jobID)
	if err != nil || status.State != "xong" || status.Bytes == nil || *status.Bytes != 4 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	file, mediaType, err := proxy.File(context.Background(), testPerson, jobID)
	if err != nil || string(file) != "mp4!" || mediaType != "video/mp4" {
		t.Fatalf("file=%q type=%q err=%v", file, mediaType, err)
	}
	if _, err := proxy.Status(context.Background(), "00000000-0000-0000-0000-000000000002", jobID); err == nil || seen != 2 {
		t.Fatalf("foreign actor queried proxy: err=%v calls=%d", err, seen)
	}
}
