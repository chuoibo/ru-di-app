// Package canary proves the comparator can fail.
//
// A parity run that reports zero differences means nothing until the same run
// reports differences when the candidate is known to be wrong. The canary is a
// proxy that damages responses in exactly the ways a port tends to damage them
// — a status off by one, a header added or duplicated, a float that lost its
// ".0", a zone written "+00:00", keys in another order, a gzipped body, a
// redirect followed — and every mode it manages to apply must turn the run red.
// The identity mode, which damages nothing, must stay green.
package canary

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

// Mode damages one response. It reports whether it changed anything, so a run
// can tell "caught" from "never exercised".
type Mode struct {
	Name   string
	Mutate func(resp *http.Response, body []byte) (newBody []byte, applied bool)
}

var (
	floatWithPoint = regexp.MustCompile(`(\d)\.0([,}\]])`)
	zuluTimestamp  = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}(?:\.\d+)?)Z"`)
	firstTwoKeys   = regexp.MustCompile(`^\{("[^"]+":(?:"[^"]*"|[^,{}\[\]"]+)),("[^"]+":(?:"[^"]*"|[^,{}\[\]"]+))`)
)

// Modes returns every damage mode, identity first.
func Modes() []Mode {
	return []Mode{
		{"identity", func(_ *http.Response, body []byte) ([]byte, bool) { return body, true }},
		{"status-off-by-one", func(resp *http.Response, body []byte) ([]byte, bool) {
			resp.StatusCode++
			resp.Status = strconv.Itoa(resp.StatusCode)
			return body, true
		}},
		{"header-added", func(resp *http.Response, body []byte) ([]byte, bool) {
			resp.Header.Set("X-Powered-By", "canary")
			return body, true
		}},
		{"header-duplicated", func(resp *http.Response, body []byte) ([]byte, bool) {
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
		{"header-dropped", func(resp *http.Response, body []byte) ([]byte, bool) {
			for _, name := range []string{"Vary", "Access-Control-Allow-Origin", "Allow", "Location", "Cache-Control", "Content-Type"} {
				if _, ok := resp.Header[name]; ok {
					delete(resp.Header, name)
					return body, true
				}
			}
			return body, false
		}},
		{"body-extra-byte", func(_ *http.Response, body []byte) ([]byte, bool) {
			if len(body) == 0 {
				return body, false
			}
			return append(append([]byte{}, body...), ' '), true
		}},
		{"float-lost-point", func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := floatWithPoint.ReplaceAll(body, []byte("$1$2"))
			return changed, !bytes.Equal(changed, body)
		}},
		{"zone-written-as-offset", func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := zuluTimestamp.ReplaceAll(body, []byte(`$1+00:00"`))
			return changed, !bytes.Equal(changed, body)
		}},
		{"first-keys-swapped", func(_ *http.Response, body []byte) ([]byte, bool) {
			changed := firstTwoKeys.ReplaceAll(body, []byte("{$2,$1"))
			return changed, !bytes.Equal(changed, body)
		}},
		{"body-gzipped", func(resp *http.Response, body []byte) ([]byte, bool) {
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
		{"redirect-followed", func(resp *http.Response, body []byte) ([]byte, bool) {
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
		},
		Transport: &http.Transport{Proxy: nil, DisableCompression: true},
		ModifyResponse: func(resp *http.Response) error {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			newBody, changed := mode.Mutate(resp, body)
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
