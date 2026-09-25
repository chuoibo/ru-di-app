package rag

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
)

// Golden content hashes of the chunker (Chunker = "place.v1"): any change to
// what a chunk holds -- a field added, a separator, the review cap, the
// quarantine -- changes a hash here, and is a new chunker name and a manual
// promotion, not a silent edit.
var ghimChunk = map[string]string{
	"dl-tiem-banh-may-xanh#ho_so":    "3f8201ca85409c6ebe4e508f22e33e45a4f0e30340d0e5ec528585dd4e6543ad",
	"dl-tiem-banh-may-xanh#danh_gia": "01499d6fd345ad324be7fc14b85efa726a854a8e39826b010e10e02e16776bdd",
	"dl-tiem-tra-gac-mai#ho_so":      "eb729e002596de7c3fec494a1b21cdc644b5f2efb84441432d507a8ac0569373",
	"dl-tiem-tra-gac-mai#danh_gia":   "22b0d6b9abda188fb9daddd800af3b791420fb83122fe14906e644be461409af",
	"dl-bun-bo-goc-pho#ho_so":        "15a81e1a09cb8b5fa11b083a451851cb0b620f645e247b4e5ba7a6e97539e8f6",
	"dl-bun-bo-goc-pho#danh_gia":     "7d69e427c42f097aa3e3ac8fa6be15cb207c6cb16fe679debdf462d0ceb55844",
	"sg-hai-san-bo-ke#ho_so":         "261b43775f1c911d76c72410ba143ea6ed0be637dd780cf9529a118e1745420b",
	"g-dl-000#ho_so":                 "1d163fad7de09b8b1298aee1dcb864d8b37eb7a6d9a6563b7f1cf2ea29d4bb43",
	"g-dl-000#danh_gia":              "de152904fcf04e0372abf8b361df4a50abac234b16b607990fdea0b4ed5c69d6",
}

func TestChunkerGoldenHashes(t *testing.T) {
	v := docVang(t)
	byID := map[string]int{}
	for i, q := range v.Quan {
		byID[q.ID] = i
	}
	got := map[string]string{}
	for _, id := range []string{"dl-tiem-banh-may-xanh", "dl-tiem-tra-gac-mai", "dl-bun-bo-goc-pho", "sg-hai-san-bo-ke", "g-dl-000"} {
		i := byID[id]
		h, rep := DungHoSo(v.hang(i, v.Quan[i]))
		if rep.Bo {
			t.Fatalf("%s dropped", id)
		}
		for _, d := range h.Doan {
			got[d.ChunkID] = hex.EncodeToString(d.Hash[:])
		}
	}
	if !reflect.DeepEqual(got, ghimChunk) {
		for k, h := range got {
			t.Logf("%q: %q,", k, h)
		}
		t.Fatalf("chunk hashes moved; %d got, %d pinned", len(got), len(ghimChunk))
	}
}

func TestChunkBodiesAndTags(t *testing.T) {
	v := docVang(t)
	for i, q := range v.Quan {
		switch q.ID {
		case "dl-tiem-banh-may-xanh":
			h, _ := DungHoSo(v.hang(i, q))
			want := "Tiệm Bánh Mây Xanh\nCafe · bánh ngọt, cà phê\nDốc Mây, Đà Lạt\nyên tĩnh, view đồi\nTiệm bánh nhỏ trên dốc, cửa kính nhìn ra đồi thông, hợp ngồi làm việc với laptop."
			if h.Doan[0].Body != want {
				t.Fatalf("ho_so body %q", h.Doan[0].Body)
			}
			if h.Doan[0].ChunkID != "dl-tiem-banh-may-xanh#ho_so" || h.Doan[1].Facet != FacetDanhGia {
				t.Fatalf("chunk ids %+v", h.Doan)
			}
			if !strings.Contains(h.Doan[0].ChuTim, "tiem_banh") || !strings.HasPrefix(h.Doan[0].ChuTim, "tiem banh may xanh") {
				t.Fatalf("search text %q", h.Doan[0].ChuTim)
			}
			if !reflect.DeepEqual(h.KhiChat, []string{"yen_tinh", "view_dep", "lam_viec"}) || h.TenGap != "tiem banh may xanh" || h.Lich == nil {
				t.Fatalf("profile %+v", h)
			}
		case "dl-tiem-tra-gac-mai":
			h, rep := DungHoSo(v.hang(i, q))
			if !reflect.DeepEqual(rep.CachLy, []string{"reviews[1]"}) || len(h.Doan) != 2 || h.Doan[1].Body != "Trà thơm, chủ quán nhiệt tình." {
				t.Fatalf("quarantine: %+v %+v", rep, h.Doan)
			}
		case "dl-bun-bo-goc-pho":
			h, rep := DungHoSo(v.hang(i, q))
			if !reflect.DeepEqual(rep.CachLy, []string{"description"}) || strings.Contains(h.Doan[0].Body, "travel bot") {
				t.Fatalf("quarantine: %+v %q", rep, h.Doan[0].Body)
			}
		case "dl-quan-ngon-inj", "sg-quan-nhau-inj":
			if _, rep := DungHoSo(v.hang(i, q)); !rep.Bo {
				t.Fatalf("%s kept", q.ID)
			}
		case "dl-quan-an-khuya-khong-ten":
			if h, _ := DungHoSo(v.hang(i, q)); h.Lich != nil {
				t.Fatal("unknown hours read as a schedule")
			}
		}
	}
}

// A review list longer than five keeps five in the chunk, and a profile
// longer than 2000 characters is cut at 2000 (the column's CHECK).
func TestChunkBounds(t *testing.T) {
	v := docVang(t)
	q := quanMau{ID: "x", DiemDen: "d-da-lat", Ten: "Quán Dài", Loai: "cafe", MoTa: str(strings.Repeat("dài ", 370))}
	for i := 0; i < 20; i++ {
		q.Traits = append(q.Traits, strings.Repeat("rộng", 12))
		q.HoatDong = append(q.HoatDong, strings.Repeat("ngồi", 12))
	}
	for i := 0; i < 7; i++ {
		q.DanhGia = append(q.DanhGia, strings.Repeat("ngon ", 20))
	}
	h, rep := DungHoSo(v.hang(0, q))
	if rep.Bo || len(rep.CachLy) != 0 || len(h.Doan) != 2 {
		t.Fatalf("%+v, %d chunks", rep, len(h.Doan))
	}
	if n := len([]rune(h.Doan[0].Body)); n != 2000 {
		t.Fatalf("ho_so %d runes, want exactly 2000", n)
	}
	if reviews := strings.Split(h.Doan[1].Body, "\n"); len(reviews) != 5 {
		t.Fatalf("%d reviews in the chunk, want 5", len(reviews))
	}
}
