package llm

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
)

// GioiHan decides whether one model call may go out now. It is shared by every
// worker process, so it bounds the calls a key makes, not the calls one
// process makes (design 02 §6).
type GioiHan interface {
	// Xin asks for one call to model. false with a nil error is a refusal; an
	// error means the limiter could not answer, and the caller lets the call
	// through: losing Redis must not stop the assistant.
	Xin(ctx context.Context, model string) (bool, error)
}

// ErrGioiHan: the rate limiter refused the call before it left the process.
// PhanLoai reads it as a 429, which is what it stands in for.
var ErrGioiHan = errors.New("llm: the per-model call rate limit refused this call")

// EnvModelRPM is the calls a minute every worker together may make to one
// model. Unset or empty: no limiter.
const EnvModelRPM = "MOBILE_MODEL_RPM"

// gcra is the generic cell rate algorithm in one atomic step, on Redis's own
// clock so that workers on machines with different clocks agree. The key
// holds one number, the theoretical arrival time in microseconds, and expires
// as soon as it no longer constrains anything.
var gcra = redis.NewScript(`
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000000 + tonumber(t[2])
local interval = tonumber(ARGV[1])
local tolerance = tonumber(ARGV[2])
local tat = tonumber(redis.call('GET', KEYS[1]) or now)
if tat < now then tat = now end
local next_tat = tat + interval
if next_tat - now > tolerance + interval then
  return 0
end
redis.call('SET', KEYS[1], string.format('%d', next_tat), 'PX', math.ceil((next_tat - now) / 1000) + 1)
return 1
`)

// GioiHanRedis is a GCRA limiter in Redis under the key
// rudi:{namespace}:rl:model:{model}: at most rpm calls a minute per model,
// with a burst of five seconds' worth. It stores a timestamp and nothing else.
type GioiHanRedis struct {
	rdb       redis.UniversalClient
	prefix    string
	interval  time.Duration
	tolerance time.Duration
	// han bounds one decision; past it the call goes through.
	han time.Duration
}

var (
	namespacePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)
	modelPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)
)

// burstWindow is how many seconds' worth of calls may go at once.
const burstWindow = 5 * time.Second

// NewGioiHanRedis builds the limiter for rpm calls a minute (1..100000).
func NewGioiHanRedis(rdb redis.UniversalClient, namespace string, rpm int) (*GioiHanRedis, error) {
	if !namespacePattern.MatchString(namespace) {
		return nil, errors.New("llm: invalid Redis namespace")
	}
	if rpm < 1 || rpm > 100000 {
		return nil, errors.New("llm: " + EnvModelRPM + " must be from 1 to 100000")
	}
	interval := time.Minute / time.Duration(rpm)
	tolerance := time.Duration(int64(burstWindow) / int64(interval) * int64(interval))
	return &GioiHanRedis{rdb: rdb, prefix: "rudi:" + namespace + ":rl:model:", interval: interval, tolerance: tolerance, han: 150 * time.Millisecond}, nil
}

// Key is the Redis key for model.
func (g *GioiHanRedis) Key(model string) (string, error) {
	if !modelPattern.MatchString(model) {
		return "", errors.New("llm: invalid model name for a rate limit key")
	}
	return g.prefix + model, nil
}

// Xin takes one cell for model, or refuses.
func (g *GioiHanRedis) Xin(ctx context.Context, model string) (bool, error) {
	key, err := g.Key(model)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, g.han)
	defer cancel()
	n, err := gcra.Run(ctx, g.rdb, []string{key}, g.interval.Microseconds(), g.tolerance.Microseconds()).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// RedisOptions are the client settings a limiter needs: short timeouts and no
// retries, so a Redis that is down costs a call at most 150 ms, then the call
// goes through.
func RedisOptions(rawURL string) (*redis.Options, error) {
	o, err := redis.ParseURL(rawURL)
	if err != nil {
		// go-redis may echo the URL, password included.
		return nil, errors.New("llm: MOBILE_REDIS_URL is not a valid redis:// URL")
	}
	o.DialTimeout = 150 * time.Millisecond
	o.ReadTimeout = 150 * time.Millisecond
	o.WriteTimeout = 150 * time.Millisecond
	o.ContextTimeoutEnabled = true
	o.MaxRetries = -1
	o.PoolSize = 8
	o.DisableIdentity = true
	return o, nil
}
