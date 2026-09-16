// Package canary proves the comparator can fail.
//
// A parity run that reports zero differences means nothing until the same run
// reports differences when the candidate is known to be wrong. The canary is a
// proxy that damages responses in exactly the ways a port tends to damage them
// — a status off by one, a header added or duplicated, a float that lost its
// ".0", a zone written "+00:00", keys in another order, a gzipped body, a
// keyset cursor that kept its base64 padding, a guest token one character
// short, a redirect followed — and every mode it manages to apply must turn the run red.
// The identity mode, which damages nothing, must stay green.
//
// One mode damages the target's photo store instead of its answer: a file the
// target stored while answering is gone before the answer arrives, as when a
// port deletes a photo after a failed insert. Only the media lane can see that
// on the step itself.
package canary

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"mobile/parity/internal/mediasnap"
	"mobile/parity/internal/normalize"
)

// Mode damages one response. It reports whether it changed anything, so a run
// can tell "caught" from "never exercised".
type Mode struct {
	Name   string
	Mutate func(resp *http.Response, body []byte) (newBody []byte, applied bool)
	// Store, when set, damages the target's photo store around a request:
	// Proxy calls it before forwarding, and calls what it returns once the
	// response is back and before the body goes on. That reports whether it
	// damaged anything.
	Store func() (after func() bool)
}

type storeAfterKey struct{}

// MediaFileDropped removes, after each response, one stored file (a file at
// k[0:2]/k[2:4]/k under root) that was not there before the request went to
// the target. Concurrent requests share one lock, so a burst loses at most one
// file per response.
func MediaFileDropped(root string) Mode {
	var mu sync.Mutex
	return Mode{Name: "media-file-dropped", Store: func() func() bool {
		mu.Lock()
		before, err := mediasnap.StoredFiles(root)
		mu.Unlock()
		return func() bool {
			if err != nil {
				return false
			}
			mu.Lock()
			defer mu.Unlock()
			after, err := mediasnap.StoredFiles(root)
			if err != nil {
				return false
			}
			created := make([]string, 0, len(after))
			for path := range after {
				if !before[path] {
					created = append(created, path)
				}
			}
			sort.Strings(created)
			for _, path := range created {
				if os.Remove(filepath.Join(root, filepath.FromSlash(path))) == nil {
					return true
				}
			}
			return false
		}
	}}
}

var (
	floatWithPoint = regexp.MustCompile(`(\d)\.0([,}\]])`)
	zuluTimestamp  = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}(?:\.\d+)?)Z"`)
	quotedRun      = regexp.MustCompile(`"[A-Za-z0-9_-]{60,}"`)
	tokenRun       = regexp.MustCompile(`[A-Za-z0-9_-]{43,}`)
	firstTwoKeys   = regexp.MustCompile(`^\{("[^"]+":(?:"[^"]*"|[^,{}\[\]"]+)),("[^"]+":(?:"[^"]*"|[^,{}\[\]"]+))`)
)

// Modes returns every mode that damages a response, identity first.
// MediaFileDropped is not among them: it needs the target's store.
func Modes() []Mode {
	return []Mode{
		{Name: "identity", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) { return body, true }},
		{Name: "status-off-by-one", Mutate: func(resp *http.Response, body []byte) ([]byte, bool) {
			resp.StatusCode++
			resp.Status = strconv.Itoa(resp.StatusCode)
			return body, true
		}},
		{Name: "header-added", Mutate: func(resp *http.Response, body []byte) ([]byte, bool) {
			resp.Header.Set("X-Powered-By", "canary")
			return body, true
		}},
		{Name: "header-duplicated", Mutate: func(resp *http.Response, body []byte) ([]byte, bool) {
			names := make([]string, 0, len(resp.Header))
			for name := range resp.Header {
				switch strings.ToLower(name) {
				case "date", "server", "content-length":
					continue
				}
				names = append(names, name)
			}
			if len(names) == 0 {
				return body, false
			}
			sort.Strings(names)
			resp.Header[names[0]] = append(resp.Header[names[0]], resp.Header[names[0]]...)
			return body, true
		}},
		{Name: "header-dropped", Mutate: func(resp *http.Response, body []byte) ([]byte, bool) {
			for _, name := range []string{"Vary", "Access-Control-Allow-Origin", "Allow", "Location", "Cache-Control", "Content-Type"} {
				if _, ok := resp.Header[name]; ok {
					delete(resp.Header, name)
					return body, true
				}
			}
			return body, false
		}},
		{Name: "body-extra-byte", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) {
			if len(body) == 0 {
				return body, false
			}
			return append(append([]byte{}, body...), ' '), true
		}},
		{Name: "float-lost-point", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := floatWithPoint.ReplaceAll(body, []byte("$1$2"))
			return changed, !bytes.Equal(changed, body)
		}},
		{Name: "zone-written-as-offset", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := zuluTimestamp.ReplaceAll(body, []byte(`$1+00:00"`))
			return changed, !bytes.Equal(changed, body)
		}},
		{Name: "first-keys-swapped", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := firstTwoKeys.ReplaceAll(body, []byte("{$2,$1"))
			return changed, !bytes.Equal(changed, body)
		}},
		{Name: "body-gzipped", Mutate: func(resp *http.Response, body []byte) ([]byte, bool) {
			if len(body) == 0 {
				return body, false
			}
			var buf bytes.Buffer
			zw := gzip.NewWriter(&buf)
			_, _ = zw.Write(body)
			_ = zw.Close()
			resp.Header.Set("Content-Encoding", "gzip")
			return buf.Bytes(), true
		}},
		{Name: "cursor-padding-kept", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := quotedRun.ReplaceAllFunc(body, func(quoted []byte) []byte {
				run := string(quoted[1 : len(quoted)-1])
				if _, ok := normalize.DecodeCursor(run); !ok {
					return quoted
				}
				return []byte(`"` + run + `="`)
			})
			return changed, !bytes.Equal(changed, body)
		}},
		{Name: "token-shortened", Mutate: func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := tokenRun.ReplaceAllFunc(body, func(run []byte) []byte {
				if len(run) != 43 {
					return run
				}
				return append([]byte{}, run[:42]...)
			})
			return changed, !bytes.Equal(changed, body)
		}},
		{Name: "redirect-followed", Mutate: func(resp *http.Response, body []byte) ([]byte, bool) {
			if resp.StatusCode < 300 || resp.StatusCode >= 400 {
				return body, false
			}
			resp.StatusCode = http.StatusOK
			resp.Status = "200 OK"
			resp.Header.Del("Location")
			return body, true
		}},
	}
}

// Proxy forwards to target and applies mode to every response. applied counts
// the responses the mode actually changed.
func Proxy(target *url.URL, mode Mode, applied *atomic.Int64) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = r.In.Host
			if mode.Store != nil {
				r.Out = r.Out.WithContext(context.WithValue(r.Out.Context(), storeAfterKey{}, mode.Store()))
			}
		},
		Transport: &http.Transport{Proxy: nil, DisableCompression: true},
		ModifyResponse: func(resp *http.Response) error {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			newBody, changed := body, false
			if mode.Mutate != nil {
				newBody, changed = mode.Mutate(resp, body)
			}
			if after, ok := resp.Request.Context().Value(storeAfterKey{}).(func() bool); ok && after() {
				changed = true
			}
			if changed && mode.Name != "identity" {
				applied.Add(1)
			}
			resp.Body = io.NopCloser(bytes.NewReader(newBody))
			// Only a changed body gets a new length. A HEAD answer carries the
			// length of a body it never sends; rewriting it from the empty read
			// would make the canary itself a difference.
			if !bytes.Equal(newBody, body) {
				resp.ContentLength = int64(len(newBody))
				resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
			}
			return nil
		},
	}
}
