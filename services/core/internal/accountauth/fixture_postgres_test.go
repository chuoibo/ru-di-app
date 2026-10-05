//go:build postgres && authfixture

package accountauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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
	// Each synthetic person signs up from their own documentation-range
	// address, as real people would: one address may ask for only so many
	// codes an hour. The isolated stack trusts loopback as its proxy.
	clientIP := "192.0.2.1"
	request := func(path string, input any, want int) map[string]any {
		b, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", target+path, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", clientIP)
		response, err := client.Do(req)
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
	credentials := map[string]map[string]string{}
	slugs := []string{"minh", "trang", "hai", "ngoc", "duc", "linh", "quan"}
	// The native world is the Android table's: seven people per lap (A-G), each
	// password drawn at runtime, and nobody signed in yet, so every person's
	// first session is the one the device opens (is_new_person, as a real
	// first sign-in is).
	native := os.Getenv("RUDI_TEST_ACCOUNT_WORLD") == "native"
	switch os.Getenv("RUDI_TEST_ACCOUNT_WORLD") {
	case "chat":
		slugs = nil
		for i := 0; i < 22; i++ {
			slugs = append(slugs, "chat"+strconv.Itoa(i))
		}
	case "native":
		laps, err := strconv.Atoi(os.Getenv("RUDI_TEST_ACCOUNT_LAPS"))
		if err != nil || laps < 1 || laps > 9 {
			laps = 1
		}
		slugs = nil
		for lap := 1; lap <= laps; lap++ {
			for _, who := range "abcdefg" {
				slugs = append(slugs, "n"+strconv.Itoa(lap)+string(who))
			}
		}
	}
	for i, slug := range slugs {
		clientIP = "192.0.2." + strconv.Itoa(i+1)
		username := "fixture_" + slug
		password := "isolated synthetic credential for " + slug
		if native {
			raw := make([]byte, 18)
			if _, err := rand.Read(raw); err != nil {
				t.Fatal(err)
			}
			password = "qa-" + base64.RawURLEncoding.EncodeToString(raw)
		}
		c := request("/auth/register", map[string]string{"username": username, "email": username + "@example.test", "password": password}, 202)
		p := proof{c["challenge_id"].(string), c["challenge_secret"].(string), mailCode(t, h, c["challenge_id"].(string))}
		v := request("/auth/register/verify", p, 201)
		person, _ := v["person_id"].(string)
		if !native {
			s := request("/auth/login", map[string]string{"username": username, "password": password}, 201)
			person = s["person_id"].(string)
			sessions[person] = s["token"].(string)
		} else {
			credentials[slug] = map[string]string{"person_id": person, "username": username, "password": password}
		}
		if person == "" {
			t.Fatal("fixture account has no person id")
		}
		world[slug] = map[string]string{"person_id": person, "username": username}
	}
	files := map[string]any{"world.json": world, "sessions.json": sessions}
	if native {
		files["credentials.json"] = credentials
	}
	for name, value := range files {
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
