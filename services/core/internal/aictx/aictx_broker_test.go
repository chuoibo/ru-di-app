//go:build broker

package aictx

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"mobile/services/core/internal/aiharness/trinho"
)

// CORE_TEST_REDIS_URL names a disposable Redis (scripts/go_broker_tier.sh).
// With CORE_REQUIRE_BROKER_TESTS=1 a missing URL fails: a skip is not a pass.
func mo(t *testing.T) *Kho {
	t.Helper()
	url := os.Getenv("CORE_TEST_REDIS_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_REDIS_URL is required")
		}
		t.Skip("CORE_TEST_REDIS_URL not set")
	}
	k, err := Open(url, fmt.Sprintf("t%d", time.Now().UnixNano()), khoaThu)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		keys, _ := k.client.Keys(ctx, k.prefix+"*").Result()
		if len(keys) > 0 {
			k.client.Unlink(ctx, keys...)
		}
		_ = k.Close()
	})
	return k
}

// Sentinel of the tier: this package reached the Redis it tests.
func TestAictxTierReachesRedis(t *testing.T) {
	k := mo(t)
	if err := k.client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
}

// every key of the namespace, with its TTL.
func khoaVaTTL(t *testing.T, k *Kho) map[string]time.Duration {
	t.Helper()
	ctx := context.Background()
	keys, err := k.client.Keys(ctx, k.prefix+"*").Result()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]time.Duration{}
	for _, key := range keys {
		d, err := k.client.PTTL(ctx, key).Result()
		if err != nil {
			t.Fatal(err)
		}
		out[key] = d
	}
	return out
}

func luot(i int) trinho.Luot {
	vai := trinho.Toi
	if i%2 == 1 {
		vai = trinho.TroLy
	}
	return trinho.Luot{Vai: vai, Chu: fmt.Sprintf("lượt số %d quán yên tĩnh", i), Luc: time.Date(2026, 9, 25, 7, 0, i, 0, time.UTC)}
}

// Every key the package writes has a TTL from the write that created it,
// within its bound; the words never reach Redis in the clear; Doc gives the
// newest turns oldest first.
func TestThemCoTTLNguyenTuVaMaHoa(t *testing.T) {
	k := mo(t)
	ctx := context.Background()
	s, _ := PhienNep(nguoiA, luot1)
	for i := 0; i < 15; i++ {
		if err := k.Them(ctx, s, luot(i)); err != nil {
			t.Fatal(err)
		}
	}
	all := khoaVaTTL(t, k)
	if len(all) != 2 {
		t.Fatalf("want the buffer and the person's index, got %v", all)
	}
	for key, d := range all {
		if d <= 0 || d > TTLNep {
			t.Errorf("%s has TTL %v, want within (0, %v]", key, d, TTLNep)
		}
	}
	raw, err := k.client.LRange(ctx, k.prefix+"nep:"+nguoiA+":"+luot1, 0, -1).Result()
	if err != nil || len(raw) != GiuToiDa {
		t.Fatalf("buffer holds %d turns (%v), want %d", len(raw), err, GiuToiDa)
	}
	for _, v := range raw {
		if strings.Contains(v, "quán") || strings.Contains(v, "lượt") {
			t.Fatal("a turn is stored in the clear")
		}
	}
	got, err := k.Doc(ctx, s)
	if err != nil || len(got) != trinho.MaxLuotNganHan {
		t.Fatalf("Doc %d %v", len(got), err)
	}
	for i, l := range got {
		want := luot(15 - trinho.MaxLuotNganHan + i)
		if l.Chu != want.Chu || l.Vai != want.Vai || !l.Luc.Equal(want.Luc) {
			t.Fatalf("turn %d = %+v, want %+v", i, l, want)
		}
	}
}

// A legacy-lane group buffer lives at most 900 s from its first write: a
// later write does not extend it, and it is never entered in a person index.
func TestNhomLegacyEX900KhongKeoDai(t *testing.T) {
	k := mo(t)
	ctx := context.Background()
	s, _ := PhienNhom(nguoiA, luot1, LaneLegacy)
	if err := k.Them(ctx, s, luot(0)); err != nil {
		t.Fatal(err)
	}
	key := k.prefix + "grp:" + nguoiA + ":" + luot1
	// EX 900 from the write itself, written here as the number: the
	// sharing window (ADR-0045 §6) is the rule, TTLNhom only its spelling.
	const cuaSo = 900 * time.Second
	if d := k.client.PTTL(ctx, key).Val(); d <= cuaSo-5*time.Second || d > cuaSo {
		t.Fatalf("group buffer TTL %v, want EX 900", d)
	}
	if err := k.client.PExpire(ctx, key, 700*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	if err := k.Them(ctx, s, luot(1)); err != nil {
		t.Fatal(err)
	}
	d := k.client.PTTL(ctx, key).Val()
	if d <= 0 || d > 700*time.Second {
		t.Fatalf("second write moved the TTL to %v; the window must not slide", d)
	}
	all := khoaVaTTL(t, k)
	if len(all) != 1 {
		t.Fatalf("a group buffer created other keys: %v", all)
	}
}

// Many writers at once: no key is ever seen without a TTL.
func TestKhongKhoaNaoThieuTTLKhiGhiDongThoi(t *testing.T) {
	k := mo(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			s, _ := PhienNep(nguoiA, fmt.Sprintf("2b8f1c9e-aaaa-4bbb-8ccc-dddddddddd%02d", w))
			for i := 0; i < 20; i++ {
				_ = k.Them(ctx, s, luot(i))
			}
		}(w)
	}
	stop := make(chan struct{})
	bad := make(chan string, 1)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			keys, _ := k.client.Keys(ctx, k.prefix+"*").Result()
			for _, key := range keys {
				// go-redis reports "no TTL" as -1 and "gone" as -2, raw.
				if d := k.client.PTTL(ctx, key).Val(); d == -1 {
					select {
					case bad <- key:
					default:
					}
				}
			}
		}
	}()
	wg.Wait()
	close(stop)
	select {
	case key := <-bad:
		t.Fatalf("%s existed without a TTL", key)
	default:
	}
	for key, d := range khoaVaTTL(t, k) {
		if d <= 0 {
			t.Fatalf("%s has no TTL (%v)", key, d)
		}
	}
}

// Xoa unlinks at once; XoaNguoi unlinks every buffer of one person and
// leaves another person's.
func TestXoaVaXoaNguoi(t *testing.T) {
	k := mo(t)
	ctx := context.Background()
	nguoiB := "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd02"
	var cuaA []string
	for i := 0; i < 3; i++ {
		s, _ := PhienNep(nguoiA, fmt.Sprintf("3b8f1c9e-aaaa-4bbb-8ccc-dddddddddd%02d", i))
		cuaA = append(cuaA, s)
		if err := k.Them(ctx, s, luot(i)); err != nil {
			t.Fatal(err)
		}
	}
	sB, _ := PhienNep(nguoiB, luot1)
	if err := k.Them(ctx, sB, luot(0)); err != nil {
		t.Fatal(err)
	}
	if err := k.Xoa(ctx, cuaA[0]); err != nil {
		t.Fatal(err)
	}
	if got, _ := k.Doc(ctx, cuaA[0]); len(got) != 0 {
		t.Fatal("Xoa left the buffer")
	}
	n, err := k.XoaNguoi(ctx, nguoiA)
	if err != nil || n != 2 {
		t.Fatalf("XoaNguoi = %d %v, want 2", n, err)
	}
	for key := range khoaVaTTL(t, k) {
		if strings.Contains(key, nguoiA) {
			t.Fatalf("%s survived the person's purge", key)
		}
	}
	if got, _ := k.Doc(ctx, sB); len(got) != 1 {
		t.Fatal("another person's buffer went with the purge")
	}
}

// The tier's own instance is configured to hold chat words (the tier starts
// it with --save "" --appendonly no), read the way serve reads it.
func TestKiemCauHinhTrenRedisThat(t *testing.T) {
	k := mo(t)
	ctx := context.Background()
	c, err := k.KiemCauHinh(ctx)
	if err != nil {
		t.Fatalf("the tier's Redis is refused: %v (%+v)", err, c)
	}
	raw, err := k.client.ConfigGet(ctx, "save").Result()
	if err != nil || raw["save"] != c.Save {
		t.Fatalf("KiemCauHinh read save %q, CONFIG GET says %v", c.Save, raw)
	}
}

var _ = redis.Nil
