package nhungcache

import (
	"context"
	"errors"
	"testing"

	"mobile/services/core/internal/aiharness/nhung"
)

func TestKhoaSeparatesEveryPart(t *testing.T) {
	base := Khoa(nhung.Model, 1536, "prefix-v1", "title: none | text: a")
	for name, k := range map[string][32]byte{
		"model":   Khoa("another-embedding-model", 1536, "prefix-v1", "title: none | text: a"),
		"dims":    Khoa(nhung.Model, 3072, "prefix-v1", "title: none | text: a"),
		"version": Khoa(nhung.Model, 1536, "prefix-v2", "title: none | text: a"),
		"prompt":  Khoa(nhung.Model, 1536, "prefix-v1", "title: none | text: b"),
		// Shifting a byte between two parts must not collide.
		"shift": Khoa(nhung.Model, 1536, "prefix-v1t", "itle: none | text: a"),
	} {
		if k == base {
			t.Errorf("%s does not change the key", name)
		}
	}
	// The prompt is NFC by construction (DinhDang), so NFD input keys alike.
	p1, _ := nhung.DinhDang(nhung.TaiLieu, "", "Đà Lạt")
	p2, _ := nhung.DinhDang(nhung.TaiLieu, "", "Đà Lạt")
	if Khoa(nhung.Model, 1536, "prefix-v1", p1) != Khoa(nhung.Model, 1536, "prefix-v1", p2) {
		t.Fatal("NFC and NFD of the same text key differently")
	}
}

func TestPrivateCorporaAreRefusedBeforeAnyRead(t *testing.T) {
	c := &CoCache{Inner: nhung.Stub{}} // no DB: a refused call never reaches it
	for _, kho := range []Kho{"memories", "nep_memories", ""} {
		if _, err := c.NhungCongKhai(context.Background(), kho, []nhung.TaiLieuVao{{NoiDung: "x"}}); !errors.Is(err, ErrKhoRieng) {
			t.Fatalf("%q: %v", kho, err)
		}
	}
}

func TestVectorCodecRoundTrips(t *testing.T) {
	v := []float32{0, -1.5, 3.25e-7, 1}
	got := giaiMa(maHoa(v))
	for i := range v {
		if got[i] != v[i] {
			t.Fatalf("%v != %v", got, v)
		}
	}
}
