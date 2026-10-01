package community

import "testing"

// An empty shared ranking (no public post yet) must come out as an empty
// list, never nil: pgx writes a nil slice as NULL, community_feeds.post_ids is
// NOT NULL, and a reader of an empty community got 503 «community_unavailable»
// — «Cộng đồng chưa kết nối được» — instead of the empty feed (QA stack,
// 2026-10-02, sqlstate 23502).
func TestBanSaoXepHangRongKhongNil(t *testing.T) {
	if got := banSaoXepHang(nil); got == nil || len(got) != 0 {
		t.Fatalf("banSaoXepHang(nil) = %#v, want an empty non-nil slice", got)
	}
	nguon := []string{"a", "b"}
	got := banSaoXepHang(nguon)
	got[0] = "x"
	if nguon[0] != "a" {
		t.Fatal("the copy shares the shared ranking's backing array")
	}
}
