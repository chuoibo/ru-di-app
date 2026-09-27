package ingest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrFrameMissing means the source does not hold that frame (yet). Not an
// error for the import: the feed uploads frames after the place row, bumps the
// row's `synced_at` when it does, and the next pull brings the place back.
var ErrFrameMissing = errors.New("frame not in source")

// ErrFrameRefused means the key names something outside the source.
var ErrFrameRefused = errors.New("frame key escapes the source")

// maxFrameBytes caps one read. The largest frame the feed has declared is
// 7.7 MB; anything far past that is not a video frame.
const maxFrameBytes = 32 << 20

// FrameSource hands back the bytes of one frame by its feed storage key.
type FrameSource interface {
	Read(ctx context.Context, key string) ([]byte, error)
}

// DirFrames reads frames from the feed's frame directory on disk.
type DirFrames struct{ Root string }

// Read refuses a key that climbs out of the root rather than following it.
func (d DirFrames) Read(_ context.Context, key string) ([]byte, error) {
	path := filepath.Join(d.Root, filepath.Clean("/"+key))
	if !strings.HasPrefix(path, filepath.Clean(d.Root)+string(os.PathSeparator)) {
		return nil, ErrFrameRefused
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrFrameMissing
	}
	return raw, err
}

// S3Frames reads frames from the feed's MinIO bucket (path-style, SigV4).
//
// Only GetObject is needed, so this is a signer and one request rather than an
// SDK: the module carries no AWS dependency, and one read-only call is small
// enough to own.
type S3Frames struct {
	Endpoint  string // http://host:9000
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string // any value, used consistently; MinIO default us-east-1
	Client    *http.Client
	now       func() time.Time
}

// Read fetches one object. 404 is ErrFrameMissing.
func (s S3Frames) Read(ctx context.Context, key string) ([]byte, error) {
	if key == "" || strings.Contains(key, "..") {
		return nil, ErrFrameRefused
	}
	base, err := url.Parse(strings.TrimRight(s.Endpoint, "/"))
	if err != nil {
		return nil, err
	}
	path := "/" + s.Bucket + "/" + encodeKey(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.Scheme+"://"+base.Host+path, nil)
	if err != nil {
		return nil, err
	}
	s.sign(req, path)
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrFrameMissing
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("s3 get %s: %s: %s", key, resp.Status, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFrameBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFrameBytes {
		return nil, fmt.Errorf("s3 get %s: larger than %d bytes", key, maxFrameBytes)
	}
	return data, nil
}

// encodeKey is the SigV4 canonical URI encoding of an object key: every byte
// outside the RFC 3986 unreserved set is percent-encoded, '/' kept.
func encodeKey(key string) string {
	var b strings.Builder
	for i := 0; i < len(key); i++ {
		c := key[i]
		if ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || c == '/' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// sign adds AWS Signature Version 4 headers for a GET with no query string.
func (s S3Frames) sign(req *http.Request, canonicalURI string) {
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	region := s.Region
	if region == "" {
		region = "us-east-1"
	}
	t := now().UTC()
	amzDate := t.Format("20060102T150405Z")
	day := t.Format("20060102")
	const payload = "UNSIGNED-PAYLOAD"
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payload)

	signed := "host;x-amz-content-sha256;x-amz-date"
	canonical := strings.Join([]string{
		http.MethodGet, canonicalURI, "",
		"host:" + req.URL.Host, "x-amz-content-sha256:" + payload, "x-amz-date:" + amzDate, "",
		signed, payload,
	}, "\n")
	scope := day + "/" + region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := hmacSHA256([]byte("AWS4"+s.SecretKey), day)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, "s3")
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.AccessKey+"/"+scope+
		", SignedHeaders="+signed+", Signature="+signature)
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
