package tools

import (
	"context"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The metrics row's closed tool list is the registry, name for name.
func TestCongCuObsKhopTens(t *testing.T) {
	var a, b []string
	a = append(a, Tens.Values()...)
	for _, c := range obs.CongCus {
		b = append(b, string(c))
	}
	sort.Strings(a)
	sort.Strings(b)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Fatalf("obs.CongCus lệch tools.Tens:\n%v\n%v", b, a)
	}
}

// The manual retriever answers the manual only, with the same evidence the
// tool returns, and DaChay names each tool the ledger counted once, in
// registry order.
func TestSoTayVaDaChay(t *testing.T) {
	if _, err := (SoTay{}).Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "x"}); err == nil {
		t.Fatal("SoTay trả lời nguồn không phải sổ tay")
	}
	kq, err := (SoTay{Man: "outings/[id]"}).Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Manual, Cau: "thêm chặng vào kèo", K: 3})
	if err != nil || len(kq.BangChung) == 0 || len(kq.BangChung) > 3 {
		t.Fatalf("sổ tay: %d mục, lỗi %v", len(kq.BangChung), err)
	}
	for _, b := range kq.BangChung {
		if b.Nguon != truyhoi.Manual || b.ID == "" || b.PhienBanChiMuc == "" {
			t.Fatalf("bằng chứng sổ tay thiếu trường: %+v", b)
		}
	}
	sc := MoiSoCai(obs.BotNep)
	for _, c := range []Ten{GetPlace, SearchPlaces, GetPlace} {
		if err := sc.Giu(c); err != nil {
			t.Fatal(err)
		}
	}
	if got := sc.DaChay(); len(got) != 2 || got[0] != SearchPlaces || got[1] != GetPlace {
		t.Fatalf("DaChay %v", got)
	}
}
