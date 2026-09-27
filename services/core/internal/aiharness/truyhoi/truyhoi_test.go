package truyhoi

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/dong"
)

func TestHopChatChatHonThang(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	l := time.Date(2026, 9, 26, 19, 0, 0, 0, time.UTC)
	l2 := l.Add(time.Hour)
	router := Cung{DiemDenID: "da-lat", DiUng: []string{"tom", "cua"}, NganSachVND: n(200000), MoLuc: &l}

	// The model's tool call drops the allergens and raises the budget: the
	// router's constraints survive, the lower budget wins.
	got, err := router.HopChat(Cung{DiUng: []string{"sua"}, AnKieng: []string{"chay"}, NganSachVND: n(300000)})
	if err != nil {
		t.Fatal(err)
	}
	want := Cung{DiemDenID: "da-lat", DiUng: []string{"tom", "cua", "sua"}, AnKieng: []string{"chay"}, NganSachVND: n(200000), MoLuc: &l}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	// A tighter budget from the tool wins.
	if got, _ := router.HopChat(Cung{NganSachVND: n(100000)}); *got.NganSachVND != 100000 {
		t.Fatalf("budget %d", *got.NganSachVND)
	}
	// With no router constraint the tool's stand.
	if got, _ := (Cung{}).HopChat(Cung{DiemDenID: "sai-gon", MoLuc: &l2}); got.DiemDenID != "sai-gon" || !got.MoLuc.Equal(l2) {
		t.Fatalf("%+v", got)
	}
	if _, err := router.HopChat(Cung{DiemDenID: "sai-gon"}); !errors.Is(err, ErrXungDot) {
		t.Fatal("two destinations merged")
	}
	if _, err := router.HopChat(Cung{MoLuc: &l2}); !errors.Is(err, ErrXungDot) {
		t.Fatal("two open instants merged")
	}
	if got, _ := (Cung{}).HopChat(Cung{}); got.DiUng != nil || got.AnKieng != nil {
		t.Fatal("empty lists must stay nil")
	}
}

func TestYeuCauKiem(t *testing.T) {
	neg := int64(-1)
	for name, y := range map[string]YeuCau{
		"unknown source":  {Nguon: "web"},
		"negative k":      {Nguon: Places, K: -1},
		"k over max":      {Nguon: Places, K: MaxK + 1},
		"negative budget": {Nguon: Places, Cung: Cung{NganSachVND: &neg}},
	} {
		if err := y.Kiem(); !errors.Is(err, ErrYeuCau) {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := (YeuCau{Nguon: Manual, K: MaxK}).Kiem(); err != nil {
		t.Fatal(err)
	}
	if _, err := (Mem{}).Bo(RBNganSach); !errors.Is(err, dong.ErrLa) {
		t.Fatal("a hard constraint relaxed")
	}
	for _, r := range RangBuocCungs.Values() {
		if RangBuocMems.Co(RangBuoc(r)) {
			t.Fatalf("%s is both hard and soft", r)
		}
	}
}
