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
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatassist"
	"mobile/services/core/internal/chatlegacychange"
	"mobile/services/core/internal/config"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/googleid"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/mw/servererror"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/idem"
	"mobile/services/core/internal/identity"
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/proxy"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/routes"
	"mobile/services/core/internal/sms"
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
		return listRoutes(args[1:], stdout, stderr)
	case "migrate-chat-candidate":
		return migrateChatCandidate(getenv, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

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
	candidates, err := manifest.ParseCandidates(getenv(ownership.EnvCandidateRoutes), force)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	served := append(manifest.GoServed(force), candidates...)
	chatCandidate := getenv(chatlegacychange.CandidateEnv) == "1"
	if raw := getenv(chatlegacychange.CandidateEnv); raw != "" && raw != "0" && raw != "1" {
		logger.Error("refusing to start", "error", "MOBILE_CHAT_CHANGES_CANDIDATE must be 0 or 1")
		return 1
	}
	if chatCandidate {
		if err := validateChatCandidate(cfg.AuthMode, manifest.Routes, served); err != nil {
			logger.Error("refusing to start", "error", err.Error())
			return 1
		}
	}
	candidateCtx, stopCandidate := context.WithCancel(context.Background())
	defer stopCandidate()
	// Every route, Python's included: registration order decides which route a
	// request belongs to, and a Python route declared first must still win.
	table, err := router.New(manifest.Routes)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}

	// Go routes authenticate in the auth mode Python resolved and query the
	// same database. Nothing is opened while Go serves nothing, so a binary
	// with every route forced back to Python needs no database settings.
	sender, debug, err := sms.FromEnv(getenv)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	env := endpoint.Env{
		Mode:         endpoint.Mode(cfg.AuthMode),
		Now:          time.Now,
		NewUnit:      func() *db.Unit { return db.NewUnit(nil) },
		Limits:       limit.NewSet(limit.Monotonic),
		PersonIDKey:  getenv(identity.KeyEnvVar),
		SMS:          sender,
		OTPDebugCode: debug,
		Google:       googleid.FromEnv(getenv),
	}
	var idempotency func(http.Handler) http.Handler
	var pool *pgxpool.Pool
	if len(served) > 0 || chatCandidate {
		pool, err = db.Open(context.Background(), getenv(db.EnvDatabaseURL))
		if err != nil {
			logger.Error("refusing to start", "error", err.Error())
			return 1
		}
		defer pool.Close()
		env.NewUnit = func() *db.Unit { return db.NewUnit(pool) }
		// A store failure is an unhandled exception in Python, answered from
		// the outermost layer; raising keeps it there instead of inside CORS.
		idempotency = idem.New(idem.NewPostgresStore(pool), idem.WithErrorHandler(
			func(w http.ResponseWriter, r *http.Request, err error) { servererror.Raise(err) }))
	}
	if chatCandidate {
		ctx, cancel := context.WithTimeout(candidateCtx, 5*time.Second)
		var installed bool
		err := pool.QueryRow(ctx, `SELECT to_regclass('chat_legacy_changes') IS NOT NULL AND to_regclass('chat_ai_invocations') IS NOT NULL`).Scan(&installed)
		cancel()
		if err != nil || !installed {
			logger.Error("refusing to start", "error", "chat candidate migration is required")
			return 1
		}
		env.BeforeServe = chatlegacychange.BeforeWrite
	}
	contract, err := pyval.Load()
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	handlers, err := routes.Handlers(contract, pyval.NewRegistry(), env)
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	// Unset and empty give the same loopback-only policy in app/api/cors.py.
	origins := getenv(cors.OriginsEnvVar)
	front, err := dispatch.New(dispatch.Options{
		Router:      table,
		Served:      served,
		Handlers:    handlers,
		Python:      proxy.New(cfg.PythonUpstream, logger),
		CORS:        cors.New(origins, origins != ""),
		Logger:      logger,
		Idempotency: idempotency,
	})
	if err != nil {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	if chatCandidate {
		var allowedOrigins []string
		if origins != "" {
			allowedOrigins = strings.Split(origins, ",")
		}
		changes := chatlegacychange.New(chatlegacychange.Store{Pool: pool}, candidateCtx, allowedOrigins)
		assistant := chatassist.New(pool, brain.Configured())
		go changes.Listen()
		go assistant.Run(candidateCtx)
		fallback := front
		feature := cors.New(origins, origins != "").Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if chatlegacychange.Matches(r.URL.Path) {
				changes.ServeHTTP(w, r)
				return
			}
			assistant.ServeHTTP(w, r)
		}))
		front = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if chatlegacychange.Matches(r.URL.Path) || chatassist.Matches(r.URL.Path) {
				feature.ServeHTTP(w, r)
				return
			}
			fallback.ServeHTTP(w, r)
		})
	}
	logger.Info("core starting",
		"listen", cfg.Listen,
		"liveness", cfg.LivenessListen,
		"python_upstream", cfg.PythonUpstream.String(),
		"go_served", len(served),
		"candidates", len(candidates),
		"chat_candidate", chatCandidate,
		"auth_mode", cfg.AuthMode,
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
	stopCandidate()
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

// listRoutes prints the routes this binary implements, in manifest order, so
// the ownership gate can compare them with the manifest.
func listRoutes(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "--json" {
		fmt.Fprintln(stderr, "usage: core routes --json")
		return 2
	}
	manifest, err := ownership.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	implemented := map[string]bool{}
	for _, route := range routes.All() {
		implemented[route.ID] = true
	}
	views := []routeView{}
	for _, r := range manifest.Routes {
		if implemented[r.ID] {
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

// validateChatCandidate prevents the privacy and lock-order hooks from being
// bypassed through Python while the opt-in extension is enabled.
func validateChatCandidate(mode string, all, served []ownership.Route) error {
	if mode != "prod" {
		return errors.New("chat candidate requires real bearer sessions (MOBILE_AUTH_MODE=prod)")
	}
	inGo := map[string]bool{}
	for _, route := range served {
		inGo[route.ID] = true
	}
	for _, route := range all {
		if (route.Group == "messages" || route.Group == "votes" || route.Group == "outings") && !inGo[route.ID] {
			return fmt.Errorf("chat candidate requires the complete Go messages, votes and outings candidates; missing %s", route.ID)
		}
	}
	return nil
}

func migrateChatCandidate(getenv func(string) string, stdout, stderr io.Writer) int {
	if getenv(chatlegacychange.CandidateEnv) != "1" {
		fmt.Fprintln(stderr, "migration requires MOBILE_CHAT_CHANGES_CANDIDATE=1")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, getenv(db.EnvDatabaseURL))
	if err != nil {
		fmt.Fprintln(stderr, "chat migration: invalid database configuration")
		return 1
	}
	defer pool.Close()
	if err = chatlegacychange.Migrate(ctx, pool); err == nil {
		err = chatassist.Migrate(ctx, pool)
	}
	if err != nil {
		fmt.Fprintln(stderr, "chat migration failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Đã áp dụng migration chat candidate; ownership production giữ nguyên.")
	return 0
}
