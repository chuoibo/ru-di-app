// Package servererror answers a failure inside a Go-served route the way the
// API's outermost layers do: Starlette's ServerErrorMiddleware with the app's
// Exception handler (app/api/guest_privacy.py guest_aware_server_error_response),
// run by uvicorn 0.34.
//
//   - Before the response started: 500 "Internal Server Error" as
//     text/plain; charset=utf-8, with only its own headers plus, under /g, the
//     guest privacy headers. Headers the route had set are dropped; they
//     belonged to a response that was never sent.
//   - After the response started: nothing more is written.
//
// Either way the connection is closed afterwards. ServerErrorMiddleware
// re-raises once it has answered, and uvicorn's run_asgi closes the transport
// for an exception raised after a response started, which the 500 itself
// counts as. A keep-alive client sees EOF on its next request, with no
// Connection: close header to warn it.
//
// The middleware sits outside CORS, as ServerErrorMiddleware sits outside
// install_cors, so a crash answer carries no Access-Control-Allow-Origin.
package servererror

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"mobile/services/core/internal/httpapi/problem"
)

// crash carries an error from Raise through layers that only speak
// http.Handler.
type crash struct{ err error }

// Raise ends the request as an unhandled exception ends it in Python. Like
// panic, it must run on the goroutine serving the request.
func Raise(err error) {
	panic(crash{err: err})
}

// Middleware recovers failures in next. scopePath returns the request's
// decoded path (scope["path"]), the string the guest boundary is matched on.
func Middleware(logger *slog.Logger, scopePath func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &startWriter{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			err, ok := v.(crash)
			failure := fmt.Errorf("panic: %v", v)
			if ok {
				failure = err.err
			}
			// No path in the log: under /g the path is the guest's credential.
			logger.Error("exception in Go route", "method", r.Method, "error", failure.Error(),
				"stack", string(debug.Stack()))
			if !tracked.started {
				header := w.Header()
				for name := range header {
					delete(header, name)
				}
				problem.WriteServerError(w, scopePath(r))
				_ = http.NewResponseController(w).Flush()
			}
			// net/http closes the connection without logging for this value.
			panic(http.ErrAbortHandler)
		}()
		next.ServeHTTP(tracked, r)
	})
}

// startWriter records whether the response has started.
type startWriter struct {
	http.ResponseWriter
	started bool
}

func (s *startWriter) WriteHeader(code int) {
	s.started = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *startWriter) Write(b []byte) (int, error) {
	s.started = true
	return s.ResponseWriter.Write(b)
}

func (s *startWriter) Flush() {
	s.started = true
	_ = http.NewResponseController(s.ResponseWriter).Flush()
}

// Unwrap lets http.ResponseController reach the connection's other controls.
func (s *startWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
