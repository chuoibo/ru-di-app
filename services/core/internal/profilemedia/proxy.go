package profilemedia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Proxy preserves the legacy Nếp media wire contract while Go owns authorization.
type Proxy struct {
	URL       string
	Token     string
	PersonKey string
	Client    *http.Client
}

type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return e.Code }

type MediaStatus struct {
	State string `json:"trang_thai"`
	Bytes *int64 `json:"so_byte,omitempty"`
	Error string `json:"loi,omitempty"`
}

func (p Proxy) configured() error {
	if strings.TrimSpace(p.URL) == "" || strings.TrimSpace(p.Token) == "" {
		return &Error{http.StatusServiceUnavailable, "nep_media_chua_cau_hinh"}
	}
	if len(p.PersonKey) < 32 {
		return &Error{http.StatusServiceUnavailable, "nep_media_thieu_khoa"}
	}
	return nil
}

func (p Proxy) request(ctx context.Context, method, path string, payload any) (*http.Response, error) {
	if err := p.configured(); err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.URL, "/")+path, body)
	if err != nil {
		return nil, &Error{http.StatusBadGateway, "nep_media_khong_goi_duoc"}
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &Error{http.StatusBadGateway, "nep_media_khong_goi_duoc"}
	}
	return resp, nil
}

// GenerateVideo accepts only the actor's generated image jobs.
func (p Proxy) GenerateVideo(ctx context.Context, personID string, imageJobIDs []string, secondsPerImage *int) (string, error) {
	if err := p.configured(); err != nil {
		return "", err
	}
	jobID, err := NewJobID(personID, p.PersonKey)
	if err != nil {
		return "", &Error{http.StatusServiceUnavailable, "nep_media_thieu_khoa"}
	}
	if err := p.SubmitVideo(ctx, personID, jobID, imageJobIDs, secondsPerImage); err != nil {
		return "", err
	}
	return jobID, nil
}

// SubmitVideo uses an already reserved ID so a retried request keeps one job.
func (p Proxy) SubmitVideo(ctx context.Context, personID, jobID string, imageJobIDs []string, secondsPerImage *int) error {
	if err := p.owner(personID, jobID); err != nil {
		return err
	}
	if len(imageJobIDs) == 0 {
		return &Error{http.StatusUnprocessableEntity, "can_anh_job_ids"}
	}
	for _, id := range imageJobIDs {
		if !OwnsJob(id, personID, p.PersonKey) {
			return &Error{http.StatusNotFound, "khong_thay_job"}
		}
	}
	resp, err := p.request(ctx, http.MethodPost, "/v1/media/video", map[string]any{
		"job_id": jobID, "tenant": jobID[:tokenLength], "anh_job_ids": imageJobIDs, "giay_moi_anh": secondsPerImage,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		if resp.StatusCode == 422 || resp.StatusCode == 429 {
			return &Error{resp.StatusCode, "nep_media_proxy_tu_choi"}
		}
		return &Error{http.StatusBadGateway, "nep_media_proxy_tu_choi"}
	}
	return nil
}

func (p Proxy) owner(personID, jobID string) error {
	if err := p.configured(); err != nil {
		return err
	}
	if !OwnsJob(jobID, personID, p.PersonKey) {
		return &Error{http.StatusNotFound, "khong_thay_job"}
	}
	return nil
}

// Status returns only public status fields, never arbitrary proxy JSON.
func (p Proxy) Status(ctx context.Context, personID, jobID string) (MediaStatus, error) {
	if err := p.owner(personID, jobID); err != nil {
		return MediaStatus{}, err
	}
	resp, err := p.request(ctx, http.MethodGet, "/v1/media/"+jobID, nil)
	if err != nil {
		return MediaStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return MediaStatus{}, &Error{http.StatusNotFound, "khong_thay_job"}
	}
	if resp.StatusCode >= 400 {
		return MediaStatus{}, &Error{http.StatusBadGateway, "nep_media_proxy_tu_choi"}
	}
	var raw struct {
		State string `json:"trang_thai"`
		Bytes *int64 `json:"so_byte"`
		Error string `json:"loi"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&raw); err != nil {
		return MediaStatus{}, &Error{http.StatusBadGateway, "nep_media_trang_thai_la"}
	}
	switch raw.State {
	case "dang-cho", "dang-chay", "xong", "hong":
	default:
		return MediaStatus{}, &Error{http.StatusBadGateway, "nep_media_trang_thai_la"}
	}
	if len(raw.Error) > 400 {
		raw.Error = raw.Error[:400]
	}
	return MediaStatus{State: raw.State, Bytes: raw.Bytes, Error: raw.Error}, nil
}

// File returns bytes only after verifying the actor's opaque job ownership.
func (p Proxy) File(ctx context.Context, personID, jobID string) ([]byte, string, error) {
	if err := p.owner(personID, jobID); err != nil {
		return nil, "", err
	}
	resp, err := p.request(ctx, http.MethodGet, "/v1/media/"+jobID+"/file", nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, "", &Error{http.StatusNotFound, "chua_co_media"}
	}
	if resp.StatusCode >= 400 {
		return nil, "", &Error{http.StatusBadGateway, "nep_media_proxy_tu_choi"}
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20+1))
	if err != nil || len(content) > 128<<20 {
		return nil, "", &Error{http.StatusBadGateway, "nep_media_proxy_tu_choi"}
	}
	mediaType := resp.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return content, mediaType, nil
}

// streamClient waits at most 10 s for the upstream's headers but never cuts
// a body mid-copy: the client's own request bounds how long that takes.
var streamClient = &http.Client{Transport: &http.Transport{
	Proxy:                 nil,
	ResponseHeaderTimeout: 10 * time.Second,
	IdleConnTimeout:       90 * time.Second,
}}

// Stream is File without the buffer: the upstream's answer for the job's
// file, a Range header passed through, for the caller to copy as it reads
// (audit 2026-10-05, PER-PROFILE-01). File read the whole MP4, up to 128 MiB,
// into memory for every request, a HEAD or a one-byte seek included.
func (p Proxy) Stream(ctx context.Context, personID, jobID, rangeHeader string) (*http.Response, error) {
	if err := p.owner(personID, jobID); err != nil {
		return nil, err
	}
	if err := p.configured(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.URL, "/")+"/v1/media/"+jobID+"/file", nil)
	if err != nil {
		return nil, &Error{http.StatusBadGateway, "nep_media_khong_goi_duoc"}
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	client := p.Client
	if client == nil {
		client = streamClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &Error{http.StatusServiceUnavailable, "nep_media_unavailable"}
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		resp.Body.Close()
		return nil, &Error{http.StatusNotFound, "chua_co_media"}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
	case resp.StatusCode >= 400:
		resp.Body.Close()
		return nil, &Error{http.StatusBadGateway, "nep_media_proxy_tu_choi"}
	}
	return resp, nil
}

func asMediaError(err error) *Error {
	var mediaErr *Error
	if errors.As(err, &mediaErr) {
		return mediaErr
	}
	return &Error{http.StatusServiceUnavailable, "nep_media_unavailable"}
}
