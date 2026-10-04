//go:build postgres && authfixture

package accountauth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/testdb"
)

// Synthetic fixture setup uses the shipped HTTP doors. Only the test reads
// its isolated encrypted outbox; production has no code-retrieval endpoint.
func TestProvisionSyntheticAccountWorld(t *testing.T) {
	target := os.Getenv("RUDI_TEST_ACCOUNT_URL")
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || os.Getenv("CORE_REQUIRE_POSTGRES_TESTS") != "1" {
		t.Fatal("isolated loopback fixture required")
	}
	output := os.Getenv("RUDI_TEST_ACCOUNT_OUTPUT")
	if !filepath.IsAbs(output) || !strings.HasPrefix(output, os.TempDir()+string(os.PathSeparator)) {
		t.Fatal("fixture output must be outside worktrees in the system temp directory")
	}
	v, err := newVault(os.Getenv("MOBILE_ACCOUNT_ENCRYPTION_KEY"), os.Getenv("MOBILE_ACCOUNT_LOOKUP_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	h := New(testdb.Pool(t), Config{Vault: v})
	client := &http.Client{Timeout: 15 * time.Second}
	request := func(path string, input any, want int) map[string]any {
		b, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Post(target+path, "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal("fixture API unavailable")
		}
		defer response.Body.Close()
		var body map[string]any
		if err = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&body); err != nil {
			t.Fatal("fixture response malformed")
		}
		requireCode(t, response.StatusCode, want, body)
		return body
	}
	world := map[string]map[string]string{}
	sessions := map[string]string{}
	slugs := []string{"minh", "trang", "hai", "ngoc", "duc", "linh", "quan"}
	if os.Getenv("RUDI_TEST_ACCOUNT_WORLD") == "chat" {
		slugs = nil
		for i := 0; i < 22; i++ {
			slugs = append(slugs, "chat"+strconv.Itoa(i))
		}
	}
	for _, slug := range slugs {
		username := "fixture_" + slug
		password := "isolated synthetic credential for " + slug
		c := request("/auth/register", map[string]string{"username": username, "email": username + "@example.test", "password": password}, 202)
		p := proof{c["challenge_id"].(string), c["challenge_secret"].(string), mailCode(t, h, c["challenge_id"].(string))}
		request("/auth/register/verify", p, 201)
		s := request("/auth/login", map[string]string{"username": username, "password": password}, 201)
		person := s["person_id"].(string)
		sessions[person] = s["token"].(string)
		world[slug] = map[string]string{"person_id": person, "username": username}
	}
	for name, value := range map[string]any{"world.json": world, "sessions.json": sessions} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(output, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var phones int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM account_identities WHERE provider='phone'`).Scan(&phones); err != nil || phones != 0 {
		t.Fatal("fixture created phone identities")
	}
	t.Logf("%d synthetic accounts created and signed in through public HTTP", len(slugs))
}
