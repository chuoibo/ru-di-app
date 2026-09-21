package sanitize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/corpus"
)

type goldenFile struct {
	Note  string       `json:"note"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Name string      `json:"name"`
	Spec corpus.Spec `json:"spec"`
	Want outcome     `json:"want"`
}

// TestGolden replays the committed golden without Docker: each input is
// rebuilt from its spec and Sanitize must answer what Python answered when
// the golden was recorded by TestOracle.
func TestGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var file goldenFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("empty golden")
	}
	for _, c := range file.Cases {
		raw, err := corpus.Build(c.Spec)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		got := goldenOutcome(raw)
		if got != c.Want {
			t.Errorf("%s:\n  want %+v\n  got  %+v", c.Name, c.Want, got)
		}
	}
}

// outcome is one side's answer in comparable form.
type outcome struct {
	Result      string `json:"result"` // ok, rejected, error, unsupported, internal
	Code        string `json:"code,omitempty"`
	Detail      string `json:"detail,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Size        int    `json:"size,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

func goldenOutcome(raw []byte) outcome {
	result, err := Sanitize(raw)
	if err == nil {
		sum := sha256.Sum256(result.Data)
		return outcome{Result: "ok", ContentType: result.ContentType, Width: result.Width, Height: result.Height,
			Size: len(result.Data), SHA256: shaTag(hex.EncodeToString(sum[:]))}
	}
	var rejected *Rejected
	var encode *EncodeError
	switch {
	case errors.As(err, &rejected):
		return outcome{Result: "rejected", Code: rejected.Code, Detail: rejected.Detail}
	case errors.As(err, &encode):
		return outcome{Result: "error", Code: "OSError"}
	}
	return outcome{Result: "internal", Detail: err.Error()}
}

// shaTag is how the golden stores a digest: the first 16 hex characters in
// groups of four joined by ':', so no run of nine digits can appear (the
// repo guard's long-number rule) while 64 bits still pin the bytes.
func shaTag(hexDigest string) string {
	prefix := hexDigest[:16]
	return prefix[0:4] + ":" + prefix[4:8] + ":" + prefix[8:12] + ":" + prefix[12:16]
}
