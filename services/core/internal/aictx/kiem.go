package aictx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CauHinh is what KiemCauHinh read from the instance.
type CauHinh struct {
	Save, AppendOnly, MaxMemoryPolicy string
}

// ViPham lists what makes an instance unfit to hold chat words: any
// persistence (research stm §2: persistence is per instance, never per
// key). An eviction policy other than volatile-ttl is advice, not a
// violation, and is not listed.
func (c CauHinh) ViPham() []string {
	var out []string
	if strings.TrimSpace(c.Save) != "" {
		out = append(out, fmt.Sprintf("save is %q, want \"\" (RDB snapshots would write chat words to disk)", c.Save))
	}
	if c.AppendOnly != "no" {
		out = append(out, fmt.Sprintf("appendonly is %q, want \"no\"", c.AppendOnly))
	}
	return out
}

// ErrCauHinh: the instance persists, or its configuration cannot be read.
var ErrCauHinh = errors.New("aictx: the redis-ai instance is not configured to hold chat words")

// configGetter is the one Redis call KiemCauHinh needs, so a unit test can
// hand it any configuration.
type configGetter interface {
	get(ctx context.Context, name string) (string, bool, error)
}

type redisConfig struct{ k *Kho }

func (r redisConfig) get(ctx context.Context, name string) (string, bool, error) {
	m, err := r.k.client.ConfigGet(ctx, name).Result()
	if err != nil {
		return "", false, err
	}
	v, ok := m[name]
	return v, ok, nil
}

// KiemCauHinh reads save, appendonly and maxmemory-policy from the
// instance. It returns the configuration and ErrCauHinh when the instance
// persists or cannot be read (CONFIG refused by an ACL counts as unread).
// The caller decides: `serve`/`work` refuse to start in prod, and warn in
// dev (LuatKhoiDong).
func (k *Kho) KiemCauHinh(ctx context.Context) (CauHinh, error) {
	return kiem(ctx, redisConfig{k})
}

func kiem(ctx context.Context, g configGetter) (CauHinh, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var c CauHinh
	for _, f := range []struct {
		name string
		dst  *string
	}{{"save", &c.Save}, {"appendonly", &c.AppendOnly}, {"maxmemory-policy", &c.MaxMemoryPolicy}} {
		v, ok, err := g.get(ctx, f.name)
		if err != nil {
			return c, fmt.Errorf("%w: CONFIG GET %s failed", ErrCauHinh, f.name)
		}
		if !ok {
			return c, fmt.Errorf("%w: CONFIG GET %s answered nothing", ErrCauHinh, f.name)
		}
		*f.dst = v
	}
	if v := c.ViPham(); len(v) > 0 {
		return c, fmt.Errorf("%w: %s", ErrCauHinh, strings.Join(v, "; "))
	}
	return c, nil
}

// LuatKhoiDong applies the startup rule: in prod a persisting or unreadable
// instance is a refusal (fail closed); in dev it is a warning the caller
// logs, and startup goes on.
func LuatKhoiDong(authMode string, err error) (tuChoi error, canhBao string) {
	if err == nil {
		return nil, ""
	}
	if authMode == "prod" {
		return err, ""
	}
	return nil, err.Error()
}
