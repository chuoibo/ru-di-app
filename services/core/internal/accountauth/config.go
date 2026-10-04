package accountauth

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/api/idtoken"
)

type GoogleClaims struct {
	Issuer, Subject, Email, Name, Nonce string
	EmailVerified                       bool
}
type GoogleVerifier interface {
	Verify(context.Context, string) (GoogleClaims, error)
}
type officialGoogle struct {
	validator *idtoken.Validator
	audiences map[string]bool
}

func (g officialGoogle) Verify(ctx context.Context, token string) (GoogleClaims, error) {
	// Validate signature and lifetime first, then explicitly check the configured audiences.
	p, err := g.validator.Validate(ctx, token, "")
	if err != nil {
		return GoogleClaims{}, problem(401, "google_token_invalid")
	}
	if !g.audiences[p.Audience] || (p.Issuer != "accounts.google.com" && p.Issuer != "https://accounts.google.com") || p.Subject == "" {
		return GoogleClaims{}, problem(401, "google_token_invalid")
	}
	stringClaim := func(k string) string { s, _ := p.Claims[k].(string); return s }
	verified, _ := p.Claims["email_verified"].(bool)
	// Google is authoritative for Gmail and hosted Workspace email only.
	// Third-party email must complete our own OTP before it can recover access.
	email := strings.ToLower(stringClaim("email"))
	verified = verified && (strings.HasSuffix(email, "@gmail.com") || stringClaim("hd") != "")
	return GoogleClaims{"https://accounts.google.com", p.Subject, stringClaim("email"), stringClaim("name"), stringClaim("nonce"), verified}, nil
}

type Limiter interface {
	Allow(context.Context, string, int, time.Duration) (bool, error)
	// Count reads a counter without spending from it; a missing key is zero.
	Count(context.Context, string) (int, error)
	Clear(context.Context, string) error
}
type redisLimiter struct {
	client    *redis.Client
	namespace string
}

// Increment and expiry are one operation shared by all replicas. No local fallback.
var limitScript = redis.NewScript(`local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('PEXPIRE',KEYS[1],ARGV[1]) end; return n`)

func (l redisLimiter) Allow(ctx context.Context, key string, max int, window time.Duration) (bool, error) {
	n, err := limitScript.Run(ctx, l.client, []string{l.namespace + key}, window.Milliseconds()).Int64()
	return n <= int64(max), err
}
func (l redisLimiter) Count(ctx context.Context, key string) (int, error) {
	n, err := l.client.Get(ctx, l.namespace+key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return n, err
}
func (l redisLimiter) Clear(ctx context.Context, key string) error {
	return l.client.Del(ctx, l.namespace+key).Err()
}

type Config struct {
	Logger         *slog.Logger
	Vault          vault
	Limits         Limiter
	Google         GoogleVerifier
	Sender         Sender
	HashSlots      int
	TrustedProxies []*net.IPNet
	// MailPerDay is the provider's daily sending allowance (Brevo Free: 300).
	MailPerDay int
}

// FromEnv returns no handler only when explicitly disabled. Enabling refuses incomplete configuration.
func FromEnv(ctx context.Context, getenv func(string) string) (Config, func(), error) {
	v, err := newVault(getenv("MOBILE_ACCOUNT_ENCRYPTION_KEY"), getenv("MOBILE_ACCOUNT_LOOKUP_KEY"))
	if err != nil {
		return Config{}, nil, err
	}
	opts, err := redis.ParseURL(getenv("MOBILE_AUTH_REDIS_URL"))
	if err != nil {
		return Config{}, nil, fmt.Errorf("managed auth requires MOBILE_AUTH_REDIS_URL")
	}
	opts.DialTimeout = 2 * time.Second
	opts.ReadTimeout = 2 * time.Second
	opts.WriteTimeout = 2 * time.Second
	opts.PoolSize = 16
	client := redis.NewClient(opts)
	cleanup := func() { _ = client.Close() }
	if err = client.Ping(ctx).Err(); err != nil {
		cleanup()
		return Config{}, nil, fmt.Errorf("managed auth Redis unavailable")
	}
	policy, e := client.ConfigGet(ctx, "maxmemory-policy").Result()
	if e != nil || policy["maxmemory-policy"] != "noeviction" {
		cleanup()
		return Config{}, nil, fmt.Errorf("auth Redis must use noeviction; rate limits must not be evicted")
	}
	sender, err := smtpFromEnv(getenv)
	if err != nil {
		cleanup()
		return Config{}, nil, err
	}
	c := Config{Vault: v, Limits: redisLimiter{client, "rudi:accounts:v1:"}, Sender: sender, HashSlots: 4, MailPerDay: 300}
	if raw := strings.TrimSpace(getenv("MOBILE_EMAIL_DAILY_LIMIT")); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 {
			cleanup()
			return Config{}, nil, fmt.Errorf("MOBILE_EMAIL_DAILY_LIMIT must be a positive integer")
		}
		c.MailPerDay = n
	}
	for _, raw := range strings.Split(getenv("MOBILE_AUTH_TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		_, network, e := net.ParseCIDR(strings.TrimSpace(raw))
		if e != nil {
			cleanup()
			return Config{}, nil, fmt.Errorf("invalid auth trusted proxy CIDR")
		}
		c.TrustedProxies = append(c.TrustedProxies, network)
	}
	audiences := map[string]bool{}
	for _, a := range strings.Split(getenv("MOBILE_GOOGLE_CLIENT_IDS"), ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			audiences[a] = true
		}
	}
	if len(audiences) > 0 {
		validator, e := idtoken.NewValidator(ctx)
		if e != nil {
			cleanup()
			return Config{}, nil, e
		}
		c.Google = officialGoogle{validator, audiences}
	}
	return c, cleanup, nil
}
func (h *Handler) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	trusted := false
	for _, network := range h.cfg.TrustedProxies {
		if network.Contains(ip) {
			trusted = true
			break
		}
	}
	if trusted {
		// Only a single address supplied by a configured sanitizing proxy is accepted.
		candidate := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
		if forwarded := net.ParseIP(candidate); forwarded != nil {
			return forwarded.String()
		}
	}
	if ip != nil {
		return ip.String()
	}
	return "unknown"
}
func (h *Handler) rateKey(scope, value string) string {
	return scope + ":" + hex.EncodeToString(h.cfg.Vault.mac("rate:"+scope, value))
}
func (h *Handler) limit(ctx context.Context, scope, value string, max int, window time.Duration) error {
	if h.cfg.Limits == nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	ok, err := h.cfg.Limits.Allow(ctx, h.rateKey(scope, value), max, window)
	if err != nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	if !ok {
		return problem(429, "auth_rate_limited")
	}
	return nil
}

// A budget counts failures only, so a stranger cannot lock someone out by
// spending the attempts of a person who then types the right password.
type budget struct {
	scope, value string
	max          int
	window       time.Duration
}

// spent refuses once any budget is used up, before the expensive work.
func (h *Handler) spent(ctx context.Context, budgets ...budget) error {
	if h.cfg.Limits == nil {
		return problem(503, "auth_temporarily_unavailable")
	}
	for _, b := range budgets {
		n, err := h.cfg.Limits.Count(ctx, h.rateKey(b.scope, b.value))
		if err != nil {
			return problem(503, "auth_temporarily_unavailable")
		}
		if n >= b.max {
			return problem(429, "auth_rate_limited")
		}
	}
	return nil
}

// fail records one failure against every budget; a lost write closes auth.
func (h *Handler) fail(ctx context.Context, budgets ...budget) error {
	for _, b := range budgets {
		if _, err := h.cfg.Limits.Allow(ctx, h.rateKey(b.scope, b.value), b.max, b.window); err != nil {
			return problem(503, "auth_temporarily_unavailable")
		}
	}
	return nil
}

// forgive drops failures a proven owner has cleared; best effort, the
// counter expires by itself anyway.
func (h *Handler) forgive(ctx context.Context, budgets ...budget) {
	for _, b := range budgets {
		_ = h.cfg.Limits.Clear(ctx, h.rateKey(b.scope, b.value))
	}
}
