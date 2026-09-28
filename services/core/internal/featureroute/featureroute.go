// Package featureroute is the mux the Go-only feature handlers register on
// (the chat change feed, the group AI engine, the avatar feed, the web session
// cookie). Those routes sit in front of the manifest router, so the Python-
// rendered route rows never list them. The mux remembers every pattern it is
// given, and the ownership manifest's `features` block is checked against that
// memory: a route added here without a manifest row, or a row left behind after
// a route is deleted, turns `go test ./cmd/core` and the ownership gate red.
//
// The registrations stay literal `h.mux.HandleFunc("METHOD /path", ...)` calls
// on purpose: scripts/check_api_contract.py reads them out of the source.
package featureroute

import "net/http"

// Mux is an http.ServeMux that records its patterns.
type Mux struct {
	*http.ServeMux
	patterns []string
}

func NewMux() *Mux { return &Mux{ServeMux: http.NewServeMux()} }

// HandleFunc registers like http.ServeMux.HandleFunc and remembers the pattern.
func (m *Mux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.patterns = append(m.patterns, pattern)
	m.ServeMux.HandleFunc(pattern, handler)
}

// Patterns returns the registered patterns in registration order.
func (m *Mux) Patterns() []string { return append([]string(nil), m.patterns...) }
