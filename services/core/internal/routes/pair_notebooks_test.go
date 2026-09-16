package routes

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/pyjson"
)

// The parity harness binds every non-empty close revision by name, because
// it hashes ids that differ between the two stacks, so a wrong digest would
// pass there. This replays the preview goldens of the domain
// (scripts/render_pair_notebook_goldens.py, the real xem_truoc_dong_so in the
// pinned image) through the digest and the wire shape the two close routes
// use: the ClosePreviewResponse body must be Python's, byte for byte.
func TestClosePreviewAnswersPythonGoldens(t *testing.T) {
	raw, err := os.ReadFile("../domain/pairnotebook/testdata/python_pair_notebook.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	instant := func(value any) *time.Time {
		if value == nil {
			return nil
		}
		// isoformat(timespec="microseconds") with the renderer's `|` before
		// the offset.
		at, err := time.Parse("2006-01-02T15:04:05.000000|-07:00", strings.TrimPrefix(value.(string), "$dt:"))
		if err != nil {
			t.Fatal(err)
		}
		return &at
	}
	replayed, nonEmpty := 0, 0
	for _, c := range golden.Cases {
		if c["fn"] != "preview" {
			continue
		}
		var papers []pairpaper.Paper
		for _, item := range c["papers"].([]any) {
			row := item.(map[string]any)
			papers = append(papers, pairpaper.Paper{ID: row["id"].(string), State: row["state"].(string), ExpiresAt: instant(row["expires_at"])})
		}
		var proposals []pairnotebook.Proposal
		for _, item := range c["proposals"].([]any) {
			row := item.(map[string]any)
			proposals = append(proposals, pairnotebook.Proposal{ID: row["id"].(string), CompletedAt: instant(row["completed_at"]), ExpiresAt: instant(row["expires_at"])})
		}
		got, err := pyjson.Compact(wirePreview(pairnotebook.XemTruocDongSo(papers, proposals, *instant(c["now"]), pairDigest)))
		if err != nil {
			t.Fatal(err)
		}
		python := c["result"].(map[string]any)["preview"].(map[string]any)
		revision := strings.ReplaceAll(strings.TrimPrefix(python["revision"].(string), "$hex:"), "_", "")
		want := pyjson.NewOrderedMap()
		want.Set("revision", pyjson.String(revision))
		for _, key := range []string{"so_nhap_bo", "so_to_huy", "so_to_khoa", "so_de_nghi_huy"} {
			want.Set(key, pyjson.NewInt(int64(python[key].(float64))))
		}
		wantBody, err := pyjson.Compact(want)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(wantBody) {
			t.Fatalf("%s: Go %s, Python %s", c["name"], got, wantBody)
		}
		if len(papers) == 0 && len(proposals) == 0 && revision != "e3b0c44298fc1c14" {
			t.Fatalf("%s: an empty notebook's revision is %s", c["name"], revision)
		}
		if revision != "e3b0c44298fc1c14" {
			nonEmpty++
		}
		replayed++
	}
	if replayed < 9 || nonEmpty < 7 {
		t.Fatalf("replayed %d preview goldens (%d with a non-empty revision); the corpus has 9", replayed, nonEmpty)
	}
}

// A path or body version past int64 saturates, so it compares with an
// INTEGER version as the Python int does.
func TestSaturatedVersionComparesAsPython(t *testing.T) {
	maxText := strconv.FormatInt(math.MaxInt64, 10)
	minText := strconv.FormatInt(math.MinInt64, 10)
	past := new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)).String()
	below := new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)).String()
	for _, tc := range []struct {
		text string
		want int64
	}{{"9", 9}, {"-9", -9}, {maxText, math.MaxInt64}, {minText, math.MinInt64}, {past, math.MaxInt64}, {below, math.MinInt64}, {"1" + strings.Repeat("0", 40), math.MaxInt64}, {"-1" + strings.Repeat("0", 40), math.MinInt64}} {
		n, ok := pyjson.ParseInt(tc.text)
		if !ok {
			t.Fatalf("ParseInt(%q)", tc.text)
		}
		if got := saturated(n); got != tc.want {
			t.Fatalf("saturated(%s) = %d, want %d", tc.text, got, tc.want)
		}
	}
}
