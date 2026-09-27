package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aictx"
	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/config"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/nepnho"
)

// Nếp's memory (internal/nepnho, ADR-0043 draft) and the engine's short-term
// store (internal/aictx), configured from the environment. Each is off until
// its variables are set, and the engine then keeps its in-process defaults:
// no personalization, the memory tools answer loi_nguon, the device's turns
// stay in the worker's memory.
const (
	// EnvNepMemoryKey is the HMAC key of forgotten facts' tombstones and of
	// the keyed receipts an account deletion leaves: base64 of 32 bytes.
	// Without it the memory routes answer 503 and nothing is remembered.
	EnvNepMemoryKey = "MOBILE_NEP_MEMORY_KEY"
	// EnvAIInferURL is the inference sidecar (services/ai-infer) whose
	// /v1/memory endpoints hold the words; EnvAIInferToken its bearer, the
	// same secret the sidecar reads. Without the URL the toggle, the events
	// and deletions of what Postgres holds still work; nothing is recalled.
	EnvAIInferURL   = "MOBILE_AI_INFER_URL"
	EnvAIInferToken = "AI_INFER_TOKEN"
	// EnvNepMemoryThreshold is the similarity floor sent with a recall.
	EnvNepMemoryThreshold = "MOBILE_NEP_MEMORY_THRESHOLD"
)

// nepMem is what a process built: nil fields are off.
type nepMem struct {
	kho  *nepnho.Kho
	ngan *aictx.Kho
}

// openNepMem builds the memory adapter and the short-term store over pool.
// It refuses a half configuration (a sidecar URL without a key or token, a
// Redis URL without its sealing key) and, in prod, a redis-ai instance that
// persists or cannot be checked; in dev that is a warning.
func openNepMem(ctx context.Context, getenv func(string) string, logger *slog.Logger, pool *pgxpool.Pool) (nepMem, error) {
	var out nepMem
	mode := getenv(config.EnvAuthMode)
	if mode == "" {
		mode = "prod"
	}
	if raw := getenv(aictx.EnvRedisURL); raw != "" {
		khoa, err := aictx.DocKhoa(getenv(aictx.EnvKhoa))
		if err != nil {
			return out, err
		}
		namespace := getenv(EnvRedisNamespace)
		if namespace == "" {
			namespace = "main"
		}
		ngan, err := aictx.Open(raw, namespace, khoa)
		if err != nil {
			return out, err
		}
		if pool == nil {
			// Configuration only (work, before any connection opens; Open
			// dials nothing): the instance is checked when built for real.
			_ = ngan.Close()
			return out, checkNepMemKey(getenv)
		}
		_, kiem := ngan.KiemCauHinh(ctx)
		tuChoi, canhBao := aictx.LuatKhoiDong(mode, kiem)
		if tuChoi != nil {
			_ = ngan.Close()
			return out, tuChoi
		}
		if canhBao != "" {
			logger.Warn("redis-ai is not fit to hold chat words; accepted in dev only", "reason", canhBao)
		}
		out.ngan = ngan
	} else if getenv(aictx.EnvKhoa) != "" {
		return out, fmt.Errorf("%s is set without %s", aictx.EnvKhoa, aictx.EnvRedisURL)
	}
	if pool == nil {
		return out, checkNepMemKey(getenv)
	}
	key, kho, err := nepMemConfig(getenv)
	if err != nil {
		out.close()
		return nepMem{}, err
	}
	if key == nil {
		// Long-term memory off; the short-term store stands on its own.
		return out, nil
	}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if ok, err := nepnho.SchemaCurrent(check, pool); err != nil || !ok {
		out.close()
		return nepMem{}, errors.New("Nếp's memory schema is missing or older than this binary: run `core migrate-chat` (compose: service migrate-chat) first")
	}
	k, err := nepnho.Moi(pool, kho, key)
	if err != nil {
		out.close()
		return nepMem{}, err
	}
	if out.ngan != nil {
		k.VoiNganHan(out.ngan)
	}
	out.kho = k
	return out, nil
}

// checkNepMemKey validates the memory's configuration without building it.
func checkNepMemKey(getenv func(string) string) error {
	_, _, err := nepMemConfig(getenv)
	return err
}

// nepMemConfig reads the tombstone key and the sidecar client. A nil key
// with no error means memory is off on this host.
func nepMemConfig(getenv func(string) string) ([]byte, nepnho.KhoNho, error) {
	rawKey := getenv(EnvNepMemoryKey)
	url := getenv(EnvAIInferURL)
	if rawKey == "" {
		if url != "" {
			return nil, nil, fmt.Errorf("%s needs %s", EnvAIInferURL, EnvNepMemoryKey)
		}
		return nil, nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawKey))
	if err != nil || len(key) != 32 {
		return nil, nil, fmt.Errorf("%s must be base64 of 32 bytes", EnvNepMemoryKey)
	}
	if url == "" {
		return key, nil, nil
	}
	nguong := 0.3
	if raw := getenv(EnvNepMemoryThreshold); raw != "" {
		if nguong, err = strconv.ParseFloat(raw, 64); err != nil {
			return nil, nil, fmt.Errorf("%s must be a number", EnvNepMemoryThreshold)
		}
	}
	c, err := nepnho.MoiKhachHTTP(url, getenv(EnvAIInferToken), nguong)
	if err != nil {
		return nil, nil, err
	}
	return key, c, nil
}

// engineOptions are what the memory adds to Nếp's engine.
func (m nepMem) engineOptions() []aiharness.Option {
	var opts []aiharness.Option
	if m.kho != nil {
		opts = append(opts, aiharness.WithHoSo(m.kho))
	}
	if m.ngan != nil {
		opts = append(opts, aiharness.WithNganHan(m.ngan))
	}
	return opts
}

// periodic is the memory's periodic tasks: the deletion safety net and the
// event and tombstone windows.
func (m nepMem) periodic() []jobs.DinhKy {
	if m.kho == nil {
		return nil
	}
	return []jobs.DinhKy{m.kho.DinhKyXoa(), m.kho.DinhKyDon()}
}

func (m nepMem) close() {
	if m.ngan != nil {
		_ = m.ngan.Close()
	}
}
