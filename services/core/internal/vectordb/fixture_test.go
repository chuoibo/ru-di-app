package vectordb

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
)

// The synthetic catalogue the unit and live tests share: invented places
// with attributes drawn from a seeded generator, so every run sees the same
// rows. No real place, person or text.

var (
	fxDiemDen = []string{"da-lat", "sai-gon", "ha-noi"}
	fxAmTiet  = []string{"quán", "cà", "phê", "lẩu", "nướng", "chay", "bún", "phở", "đồi", "thông", "sân", "vườn", "yên", "tĩnh", "nhạc", "sống", "hải", "sản", "bánh", "mì", "trà", "sữa", "view", "hồ"}
)

// fxDiaDiem builds n place rows from seed.
func fxDiaDiem(seed uint64, n int) []HangDiaDiem {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	allergens := tuvung.DiUng.IDs()
	diets := tuvung.AnKieng.IDs()
	out := make([]HangDiaDiem, n)
	for i := range out {
		var words []string
		for j := 0; j < 4+r.IntN(6); j++ {
			words = append(words, fxAmTiet[r.IntN(len(fxAmTiet))])
		}
		text := strings.Join(words, " ")
		t := ThuocTinh{DiemDen: fxDiemDen[r.IntN(len(fxDiemDen))], GiaMinVND: GiaKhongRo}
		switch r.IntN(6) {
		case 0:
			t.DiUng = []string{KhongRo}
		case 1, 2:
			// none established: an empty list
		default:
			for _, a := range allergens {
				if r.IntN(5) == 0 {
					t.DiUng = append(t.DiUng, a)
				}
			}
		}
		for _, d := range diets {
			if r.IntN(3) == 0 {
				t.AnKieng = append(t.AnKieng, d)
			}
		}
		t.AnKieng = tuvung.DoiKieng(t.AnKieng)
		if r.IntN(5) != 0 {
			for day := 0; day < 7; day++ {
				if r.IntN(7) == 0 {
					continue
				}
				open, close := 12+r.IntN(8), 36+r.IntN(12)
				for s := open; s < close; s++ {
					t.OSlots = append(t.OSlots, int16(day*48+s))
				}
			}
		}
		if r.IntN(5) != 0 {
			t.GiaMinVND = int64(20+r.IntN(40)) * 5000
		}
		t.GoBo = r.IntN(25) == 0
		// Categories from the index alone, so the seeded stream above (and
		// every count pinned on it) is unchanged: every eleventh place's
		// categories are unknown, the rest hold one or two ids.
		t.DanhMuc = fxDanhMuc(i)
		dense, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: text}})
		out[i] = HangDiaDiem{
			ID: fmt.Sprintf("p%04d", i), Dense: dense[0], Text: text, ThuocTinh: t,
			GiaMaxVND: max(t.GiaMinVND, t.GiaMinVND*2), PhienBan: 1,
		}
	}
	return out
}

// fxDanhMuc are place i's categories.
func fxDanhMuc(i int) []string {
	if i%11 == 10 {
		return []string{KhongRo}
	}
	ids := tuvung.DanhMuc.IDs()
	a := ids[i%len(ids)]
	if i%3 == 0 {
		if b := ids[(i/3+1)%len(ids)]; b != a {
			return []string{a, b}
		}
	}
	return []string{a}
}

// fxLoc draws a random hard-constraint set: any subset of the five.
func fxLoc(r *rand.Rand) (truyhoi.Cung, string) {
	var c truyhoi.Cung
	var q []string
	for j := 0; j < 2+r.IntN(3); j++ {
		q = append(q, fxAmTiet[r.IntN(len(fxAmTiet))])
	}
	if r.IntN(3) != 0 {
		c.DiemDenID = fxDiemDen[r.IntN(len(fxDiemDen))]
	}
	if r.IntN(2) == 0 {
		ids := tuvung.DiUng.IDs()
		c.DiUng = []string{ids[r.IntN(len(ids))]}
	}
	if r.IntN(3) == 0 {
		ids := tuvung.AnKieng.IDs()
		c.AnKieng = []string{ids[r.IntN(len(ids))]}
	}
	if r.IntN(2) == 0 {
		t := time.Date(2026, 9, 21+r.IntN(7), r.IntN(24), r.IntN(60), 0, 0, ViTri)
		c.MoLuc = &t
	}
	if r.IntN(2) == 0 {
		n := int64(20+r.IntN(40)) * 5000
		c.NganSachVND = &n
	}
	// A price floor (below any ceiling drawn above) and categories.
	if r.IntN(3) == 0 {
		f := int64(10+r.IntN(30)) * 5000
		if c.NganSachVND != nil && f > *c.NganSachVND {
			f = *c.NganSachVND
		}
		c.GiaTuVND = &f
	}
	if r.IntN(3) == 0 {
		ids := tuvung.DanhMuc.IDs()
		c.DanhMuc = []string{ids[r.IntN(len(ids))]}
		if r.IntN(2) == 0 {
			c.DanhMuc = append(c.DanhMuc, ids[r.IntN(len(ids))])
		}
	}
	return c, strings.Join(q, " ")
}

// fxDat counts the rows of rows satisfying l.
func fxDat(rows []HangDiaDiem, l LocCung) []string {
	var ids []string
	for _, r := range rows {
		if ok, _ := l.Dat(r.thuocTinh()); ok {
			ids = append(ids, r.ID)
		}
	}
	slices.Sort(ids)
	return ids
}
