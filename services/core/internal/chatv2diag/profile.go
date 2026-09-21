// Package chatv2diag exposes opt-in diagnostics only for synthetic lab commands.
package chatv2diag

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"time"
)

// Start never mounts handlers on the product server. Each laboratory process
// gets a separate ephemeral loopback listener; normal runs have no profiler.
func Start(role string, getenv func(string) string, out io.Writer) (func(), error) {
	if getenv("RUDI_CHAT_PROFILE") != "1" {
		return func() {}, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	for _, name := range []string{"heap", "goroutine", "mutex", "block", "allocs"} {
		mux.Handle("GET /debug/pprof/"+name, pprof.Handler(name))
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096}
	oldMutex := runtime.SetMutexProfileFraction(20)
	runtime.SetBlockProfileRate(10000)
	go srv.Serve(listener)
	fmt.Fprintf(out, "profile role=%s pid=%d url=http://%s\n", role, os.Getpid(), listener.Addr())
	return func() { srv.Close(); runtime.SetMutexProfileFraction(oldMutex); runtime.SetBlockProfileRate(0) }, nil
}
