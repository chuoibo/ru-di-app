// Command chat-lab runs the isolated transport laboratory, never the public
// core router. Device enrollment and MLS readiness cannot be bypassed through
// any HTTP route. Only synthetic test fixtures may provision those records.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mobile/services/core/internal/chatbus"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/chatv2diag"
	"mobile/services/core/internal/chatv2http"
	"mobile/services/core/internal/db"
)

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func settings(getenv func(string) string) (string, string, error) {
	if getenv("RUDI_CHAT_LAB") != "1" {
		return "", "", errors.New("RUDI_CHAT_LAB=1 is required; production enrollment is not implemented")
	}
	address := getenv("RUDI_CHAT_LAB_LISTEN")
	if address == "" {
		address = "127.0.0.1:8198"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "0" || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return "", "", errors.New("chat laboratory must listen on a numeric loopback address and a nonzero port")
	}
	raw := getenv("RUDI_CHAT_LAB_DATABASE_URL")
	cfg, err := db.PoolConfig(raw)
	if err != nil {
		return "", "", errors.New("invalid RUDI_CHAT_LAB_DATABASE_URL")
	}
	if !strings.HasSuffix(cfg.ConnConfig.Database, "_test") && !strings.HasSuffix(cfg.ConnConfig.Database, "_lab") {
		return "", "", errors.New("laboratory database name must end in _test or _lab")
	}
	return address, raw, nil
}

func run(args []string, getenv func(string) string) error {
	if len(args) != 1 || (args[0] != "serve" && args[0] != "migrate") {
		return errors.New("usage: chat-lab serve | migrate")
	}
	address, raw, err := settings(getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := db.Open(ctx, raw)
	if err != nil {
		return errors.New("cannot initialize laboratory database")
	}
	defer pool.Close()
	if args[0] == "migrate" {
		if err := chatv2.Migrate(ctx, pool); err != nil {
			return errors.New("laboratory migration failed; verify legacy schema and migration checksum")
		}
		return nil
	}
	stopProfile, err := chatv2diag.Start("server", getenv, os.Stderr)
	if err != nil {
		return errors.New("cannot initialize laboratory diagnostics")
	}
	defer stopProfile()
	if getenv("RUDI_CHAT_PROFILE") == "1" {
		// Pool counters contain no identifiers or payloads. They distinguish
		// database work from queued acquisition during diagnostic load runs.
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s := pool.Stat()
					fmt.Fprintf(os.Stderr, "profile role=pool pid=%d acquired=%d constructing=%d total=%d max=%d empty_acquires=%d cancelled_acquires=%d acquire_milliseconds=%d\n", os.Getpid(), s.AcquiredConns(), s.ConstructingConns(), s.TotalConns(), s.MaxConns(), s.EmptyAcquireCount(), s.CanceledAcquireCount(), s.AcquireDuration().Milliseconds())
				}
			}
		}()
	}
	h := chatv2http.New(chatv2http.Options{Store: chatv2.NewStore(pool), BatchSessions: true, Authenticate: chatv2http.Sessions(pool), Experimental: true, Context: ctx})
	go h.Listen(ctx, pool)
	bus := chatbus.Postgres()
	if rawRedis := getenv("RUDI_CHAT_REDIS_URL"); rawRedis != "" {
		bus, err = chatbus.New(rawRedis, getenv("RUDI_CHAT_REDIS_NAMESPACE"))
		if err != nil {
			return errors.New("invalid laboratory Redis configuration")
		}
		go bus.Listen(ctx, h.Wake, nil)
	}
	defer bus.Close()
	go bus.Relay(ctx, pool)
	srv := &http.Server{Addr: address, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	fmt.Fprintln(os.Stderr, "chat transport laboratory started; native MLS and production gates remain closed")
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("laboratory listener failed")
		}
		return nil
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
