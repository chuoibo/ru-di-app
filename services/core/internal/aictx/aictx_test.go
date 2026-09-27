package aictx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"mobile/services/core/internal/aiharness/trinho"
)

var khoaThu = []byte("0123456789abcdef0123456789abcdef")

func moiThu(t *testing.T) *Kho {
	t.Helper()
	k, err := Moi(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}), "thu", khoaThu)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

const (
	nguoiA = "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01"
	luot1  = "1b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01"
)

// An end-to-end encrypted room has no buffer: no session id can be built for
// it, so nothing downstream can write one.
func TestV2KhongCoPhien(t *testing.T) {
	if _, err := PhienNhom(nguoiA, luot1, LaneV2); !errors.Is(err, ErrE2EE) {
		t.Fatalf("v2 room got a session id: %v", err)
	}
	if _, err := PhienNhom(nguoiA, luot1, Lane("")); !errors.Is(err, ErrE2EE) {
		t.Fatalf("an unnamed lane got a session id: %v", err)
	}
	s, err := PhienNhom(nguoiA, luot1, LaneLegacy)
	if err != nil || s != "grp:"+nguoiA+":"+luot1 {
		t.Fatalf("legacy: %q %v", s, err)
	}
}

func TestPhienLaDongVaKhongThoatKhoa(t *testing.T) {
	for _, bad := range []string{"", "nep:a:b", "v2:" + nguoiA + ":" + luot1, "nep:" + nguoiA + ":" + luot1 + ":x",
		"nep:" + nguoiA + ":*", "nep:" + nguoiA + ":" + luot1 + "\r\nFLUSHALL", "idx:" + nguoiA + ":" + luot1} {
		if _, err := docPhien(bad); !errors.Is(err, ErrPhien) {
			t.Errorf("accepted %q", bad)
		}
	}
	if _, err := PhienNep("không-phải-id", luot1); !errors.Is(err, ErrPhien) {
		t.Fatal("a non-id person was accepted")
	}
	k := moiThu(t)
	p, _ := docPhien("nep:" + nguoiA + ":" + luot1)
	if got := k.khoa(p); got != "rudi:thu:aictx:nep:"+nguoiA+":"+luot1 || !k.Owns(got) {
		t.Fatalf("key %q", got)
	}
	if ttl(p) != TTLNep {
		t.Fatal("Nếp buffer TTL")
	}
	g, _ := docPhien("grp:" + nguoiA + ":" + luot1)
	if ttl(g) != 900*time.Second {
		t.Fatalf("group legacy buffer must be EX 900, got %v", ttl(g))
	}
}

// Sealed values open only under their own key name: a value copied to
// another buffer (another person's) does not open there.
func TestNiemMoTheoTenKhoa(t *testing.T) {
	k := moiThu(t)
	l := trinho.Luot{Vai: trinho.Toi, Chu: "quán cà phê yên tĩnh gần hồ", Luc: time.Date(2026, 9, 25, 7, 0, 0, 0, time.UTC), BangChungIDs: []string{"p1"}}
	v, err := k.niem("a", l)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(v, "yên tĩnh") || strings.Contains(v, "cà phê") {
		t.Fatal("sealed value carries the words")
	}
	got, err := k.mo("a", v)
	if err != nil || got.Chu != l.Chu || got.Vai != l.Vai || !got.Luc.Equal(l.Luc) || got.BangChungIDs[0] != "p1" {
		t.Fatalf("roundtrip %+v %v", got, err)
	}
	if _, err := k.mo("b", v); !errors.Is(err, ErrMoKhoa) {
		t.Fatal("a value opened under another key name")
	}
	other, _ := Moi(k.client, "thu", []byte("fedcba9876543210fedcba9876543210"))
	if _, err := other.mo("a", v); !errors.Is(err, ErrMoKhoa) {
		t.Fatal("a value opened under another sealing key")
	}
	v2, _ := k.niem("a", l)
	if v2 == v {
		t.Fatal("two seals of the same turn are identical: the nonce is not fresh")
	}
}

func TestDocKhoa(t *testing.T) {
	if _, err := DocKhoa("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "short", "MDEyMzQ1Njc4OWFiY2RlZg=="} {
		if _, err := DocKhoa(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

type cauHinhGia map[string]string

func (c cauHinhGia) get(_ context.Context, name string) (string, bool, error) {
	if c["loi"] == name {
		return "", false, errors.New("NOPERM")
	}
	v, ok := c[name]
	return v, ok, nil
}

// The startup rule fails closed in prod on any persistence and on an
// unreadable configuration, and only warns in dev.
func TestKiemCauHinhDongKhiLuu(t *testing.T) {
	sach := cauHinhGia{"save": "", "appendonly": "no", "maxmemory-policy": "volatile-ttl"}
	if _, err := kiem(context.Background(), sach); err != nil {
		t.Fatalf("identity: a clean instance was refused: %v", err)
	}
	for name, c := range map[string]cauHinhGia{
		"rdb":   {"save": "3600 1", "appendonly": "no", "maxmemory-policy": "noeviction"},
		"aof":   {"save": "", "appendonly": "yes", "maxmemory-policy": "volatile-ttl"},
		"acl":   {"save": "", "appendonly": "no", "maxmemory-policy": "volatile-ttl", "loi": "save"},
		"thieu": {"appendonly": "no", "maxmemory-policy": "volatile-ttl"},
	} {
		_, err := kiem(context.Background(), c)
		if !errors.Is(err, ErrCauHinh) {
			t.Errorf("%s: not refused: %v", name, err)
		}
		if tuChoi, _ := LuatKhoiDong("prod", err); tuChoi == nil {
			t.Errorf("%s: prod started anyway", name)
		}
		if tuChoi, canh := LuatKhoiDong("dev", err); tuChoi != nil || canh == "" {
			t.Errorf("%s: dev must warn, not refuse", name)
		}
	}
	if tuChoi, canh := LuatKhoiDong("prod", nil); tuChoi != nil || canh != "" {
		t.Fatal("a clean instance produced a refusal or a warning")
	}
}

func TestThemTuChoiLuotSai(t *testing.T) {
	k := moiThu(t)
	s, _ := PhienNep(nguoiA, luot1)
	for _, l := range []trinho.Luot{
		{Vai: "nep", Chu: "x", Luc: time.Now()},
		{Vai: trinho.Toi, Chu: "", Luc: time.Now()},
		{Vai: trinho.Toi, Chu: "x"},
		{Vai: trinho.Toi, Chu: strings.Repeat("a", MaxChu+1), Luc: time.Now()},
	} {
		if err := k.Them(context.Background(), s, l); err == nil || strings.Contains(err.Error(), "connect") {
			t.Errorf("turn %+v reached Redis: %v", l.Vai, err)
		}
	}
}
