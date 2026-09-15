//go:build oracle

// Package pyoracle asks the real Pillow in the pinned parity API image about
// upload sanitizing, through scripts/render_media_sanitize_oracle.py. It is
// test-only: every caller is a test behind the oracle build tag.
//
// Environment:
//
//	MEDIA_ORACLE_IMAGE  image (default mobile-parity-api:7bf58e3d)
package pyoracle

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const script = "scripts/render_media_sanitize_oracle.py"

// Request is one line to the oracle. Byte slices travel as base64.
type Request struct {
	ID       string         `json:"id"`
	Op       string         `json:"op"`
	Input    []byte         `json:"b64"`
	Other    []byte         `json:"b64_other,omitempty"`
	Gen      map[string]any `json:"gen,omitempty"`
	W        int            `json:"w"`
	H        int            `json:"h"`
	Want     []string       `json:"want,omitempty"`
	Metadata map[string]any `json:"-"`
}

// Response is the oracle's answer to one Request.
type Response struct {
	ID          string          `json:"id"`
	Result      string          `json:"result"`
	Code        string          `json:"code"`
	Detail      string          `json:"detail"`
	Type        string          `json:"type"`
	Message     string          `json:"message"`
	SHA256      string          `json:"sha256"`
	Size        int             `json:"size"`
	ContentType string          `json:"content_type"`
	Width       int             `json:"width"`
	Height      int             `json:"height"`
	InputSHA256 string          `json:"input_sha256"`
	Output      []byte          `json:"b64_output"`
	InputBytes  []byte          `json:"b64_input"`
	Pixels      []byte          `json:"b64_pixels"`
	PixelsSHA   string          `json:"pixels_sha256"`
	PixelsMode  string          `json:"pixels_mode"`
	PixelsW     int             `json:"pixels_w"`
	PixelsH     int             `json:"pixels_h"`
	PixelsError string          `json:"pixels_error"`
	OpenedMode  string          `json:"opened_mode"`
	Format      string          `json:"format"`
	Mode        string          `json:"mode"`
	LoadedMode  string          `json:"loaded_mode"`
	LoadedW     int             `json:"loaded_width"`
	LoadedH     int             `json:"loaded_height"`
	PaletteMode string          `json:"palette_mode"`
	Palette     []byte          `json:"b64_palette"`
	Info        json.RawMessage `json:"info"`
	MAE         []float64       `json:"mae"`
	SSIM        float64         `json:"ssim"`
}

// Image is the image the oracle runs in.
func Image() string {
	if image := os.Getenv("MEDIA_ORACLE_IMAGE"); image != "" {
		return image
	}
	return "mobile-parity-api:7bf58e3d"
}

func findScript(t testing.TB) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, script)
		if data, err := os.ReadFile(candidate); err == nil {
			return data
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the test directory", script)
		}
		dir = parent
	}
}

// Run sends every request to one container run, in the foreground, and
// returns the answers in request order.
func Run(t testing.TB, requests []Request) []Response {
	t.Helper()
	if len(requests) == 0 {
		return nil
	}
	source := findScript(t)
	var stdin bytes.Buffer
	encoder := json.NewEncoder(&stdin)
	for i := range requests {
		if requests[i].ID == "" {
			requests[i].ID = fmt.Sprint(i)
		}
		if err := encoder.Encode(requests[i]); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none",
		"--entrypoint", "python", Image(), "-c", string(source))
	cmd.Stdin = &stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	responses := make([]Response, 0, len(requests))
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 1<<30)
	for scanner.Scan() {
		var response Response
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatalf("oracle answer %d: %v", len(responses), err)
		}
		responses = append(responses, response)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading oracle: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("oracle in %s: %v\n%s", Image(), err, stderr.String())
	}
	if len(responses) != len(requests) {
		t.Fatalf("oracle answered %d of %d requests\n%s", len(responses), len(requests), stderr.String())
	}
	for i, response := range responses {
		if response.ID != requests[i].ID {
			t.Fatalf("oracle answer %d is for %q, want %q", i, response.ID, requests[i].ID)
		}
		if response.Type == "OracleCrash" {
			t.Fatalf("oracle crashed on %q: %s", response.ID, response.Message)
		}
	}
	return responses
}
