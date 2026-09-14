// Command core is the Go front door of the API. It serves the routes the
// ownership manifest gives to Go and forwards everything else to Python.
//
//	core serve         run the front door
//	core healthcheck   exit 0 if this process answers on its liveness port
//	core routes --json list the routes this binary serves itself
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mobile/services/core/internal/config"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/proxy"
	"mobile/services/core/ownership"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: core serve | healthcheck | routes --json")
		return 2
	}
	switch args[0] {
	case "serve":
		return serve(getenv, stderr)
	case "healthcheck":
		return healthcheck(getenv, stderr)
	case "routes":
		return routes(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

// handlers maps a manifest route id to the Go handler that serves it. It is
// empty until the first router group is migrated; the manifest may not give
// Go a route that has no handler here.
var handlers = map[string]http.Handler{}

func serve(getenv func(string) string, stderr io.Writer) int {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	cfg, err := config.Load(getenv)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	manifest, err := ownership.Load()
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	force, err := manifest.ParseForce(cfg.ForcePython)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	served := manifest.GoServed(force)
	// Every route, Python's included: registration order decides which route a
	// request belongs to, and a Python route declared first must still win.
	routes, err := router.New(manifest.Routes)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	// Unset and empty give the same loopback-only policy in app/api/cors.py.
	origins := getenv(cors.OriginsEnvVar)
	front, err := dispatch.New(dispatch.Options{
		Router:   routes,
		Served:   served,
		Handlers: handlers,
		Python:   proxy.New(cfg.PythonUpstream, logger),
		CORS:     cors.New(origins, origins != ""),
		Logger:   logger,
	})
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	logger.Info("core starting",
		"listen", cfg.Listen,
		"liveness", cfg.LivenessListen,
		"python_upstream", cfg.PythonUpstream.String(),
		"go_served", len(served),
		"manifest_routes", len(manifest.Routes),
		"force_python", force.Tokens,
	)

	public := &http.Server{
		Addr:              cfg.Listen,
		Handler:           front,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	liveness := &http.Server{
		Addr:              cfg.LivenessListen,
		Handler:           http.HandlerFunc(livez),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errs := make(chan error, 2)
	for _, srv := range []*http.Server{public, liveness} {
		srv := srv
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- fmt.Errorf("%s: %w", srv.Addr, err)
			}
		}()
	}

	exit := 0
	select {
	case <-ctx.Done():
		logger.Info("core stopping")
	case err := <-errs:
		logger.Error("listener failed", "error", err.Error())
		exit = 1
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	_ = public.Shutdown(shutdown)
	_ = liveness.Shutdown(shutdown)
	return exit
}

func livez(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/livez" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

func healthcheck(getenv func(string) string, stderr io.Writer) int {
	address := getenv(config.EnvLivenessListen)
	if address == "" {
		address = "127.0.0.1:8001"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: %v\n", err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/livez")
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: %v\n", err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

type routeView struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Group  string `json:"group"`
}

// routes prints the routes this binary has handlers for, in manifest order,
// so the ownership gate can compare them with the manifest's Go-owned rows.
func routes(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "--json" {
		fmt.Fprintln(stderr, "usage: core routes --json")
		return 2
	}
	manifest, err := ownership.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	views := []routeView{}
	for _, r := range manifest.Routes {
		if handlers[r.ID] != nil {
			views = append(views, routeView{ID: r.ID, Method: r.Method, Path: r.Path, Group: r.Group})
		}
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(views); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
