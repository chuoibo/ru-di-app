// Package guest stamps the guest privacy headers on every answer a Go-served
// route gives under /g, as app/api/guest_privacy.py GuestPrivacyHeadersMiddleware
// does: by assignment, so a value the route set is replaced rather than kept.
//
// A crash answer never passes through here; package servererror stamps it,
// as guest_aware_server_error_response does in Python.
package guest

import (
	"net/http"

	"mobile/services/core/internal/httpapi/problem"
)

// Middleware wraps next. scopePath returns the request's decoded path
// (scope["path"]).
func Middleware(scopePath func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !problem.IsGuestPath(scopePath(r)) {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(&stampWriter{ResponseWriter: w}, r)
	})
}

// stampWriter sets the headers when the response starts, which is when
// http.response.start passes through the Python middleware.
type stampWriter struct {
	http.ResponseWriter
	stamped bool
}

func (s *stampWriter) stamp() {
	if s.stamped {
		return
	}
	s.stamped = true
	for _, pair := range problem.GuestPrivacyHeaders {
		s.Header().Set(pair[0], pair[1])
	}
}

func (s *stampWriter) WriteHeader(code int) {
	s.stamp()
	s.ResponseWriter.WriteHeader(code)
}

func (s *stampWriter) Write(b []byte) (int, error) {
	s.stamp()
	return s.ResponseWriter.Write(b)
}

func (s *stampWriter) Flush() {
	s.stamp()
	_ = http.NewResponseController(s.ResponseWriter).Flush()
}

// Unwrap lets http.ResponseController reach the connection's other controls.
func (s *stampWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
