package canary

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func storeFile(t *testing.T, root string) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(b[:])
	dir := filepath.Join(root, key[:2], key[2:4])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, key)
	if err := os.WriteFile(path, []byte("jpeg bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// The store mode removes the file the request stored, keeps every file stored
// before it, and leaves the answer alone.
func TestMediaFileDroppedRemovesOnlyWhatTheRequestStored(t *testing.T) {
	root := t.TempDir()
	older := storeFile(t, root)
	var written string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			written = storeFile(t, root)
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(`{"id":"a"}`))
	}))
	t.Cleanup(target.Close)
	targetURL, _ := url.Parse(target.URL)
	var applied atomic.Int64
	front := httptest.NewServer(Proxy(targetURL, MediaFileDropped(root), &applied))
	t.Cleanup(front.Close)

	if resp, _ := fetch(t, front.URL, "/photos/a"); resp.StatusCode != 200 || applied.Load() != 0 || !exists(older) {
		t.Fatalf("a read was damaged: status %d, applied %d", resp.StatusCode, applied.Load())
	}
	resp, err := http.Post(front.URL+"/photos", "image/jpeg", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || string(body) != `{"id":"a"}` {
		t.Errorf("the answer changed: %d %q", resp.StatusCode, body)
	}
	if applied.Load() != 1 || written == "" || exists(written) || !exists(older) {
		t.Errorf("applied %d, new file kept %v, older file kept %v", applied.Load(), exists(written), exists(older))
	}
}

// The response modes stay what they were; the store mode joins only when a
// run compares stores.
func TestModesAreResponseModesOnly(t *testing.T) {
	for _, mode := range Modes() {
		if mode.Store != nil || mode.Mutate == nil {
			t.Errorf("%s is not a response mode", mode.Name)
		}
	}
}
