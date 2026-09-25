package rag

import (
	"strings"

	"mobile/services/core/internal/rag/xephang"
	"mobile/services/core/internal/repo"
)

// hoSoDat is a profile that passed the hard filters, with its unknowns.
type hoSoDat struct {
	h  HoSo
	co []string
}

// locSong builds and filters live rows: tombstoned ids, rows SafeDeep drops
// and rows that break a hard filter are left out.
func locSong(rows []repo.Place, bia map[string]bool, r rangBuoc) []hoSoDat {
	var out []hoSoDat
	for _, row := range rows {
		if bia[row.ID] {
			continue
		}
		h, report := DungHoSo(row)
		if report.Bo {
			continue
		}
		if ok, co := r.dat(h); ok {
			out = append(out, hoSoDat{h: h, co: co})
		}
	}
	return out
}

// diemMem is the closed-vocabulary list's score: two for each atmosphere the
// place declares that the asker asked for, one for the asked category.
func diemMem(category string, khiChat []string, y YeuCau) float64 {
	s := 0.0
	for _, k := range khiChat {
		for _, want := range y.KhiChat {
			if k == want {
				s += 2
			}
		}
	}
	for _, c := range y.LoaiCho {
		if c == category {
			s++
			break
		}
	}
	return s
}

// memToiDa bounds the closed-vocabulary list, which ties many places.
const memToiDa = 200

// xepSong answers from live rows alone: what Retrieve does with no active
// index version, and the ranking the public search uses without one. The
// full-text list is rag/xephang (BM25) over the same chunks the index would
// hold; there is no trigram list.
func xepSong(rows []repo.Place, bia map[string]bool, y YeuCau) KetQua {
	r := rangBuocCua(y)
	return xepHoSo(locSong(rows, bia, r), y, r)
}

// xepHoSo ranks profiles that already passed the hard filters.
func xepHoSo(pass []hoSoDat, y YeuCau, r rangBuoc) KetQua {
	kq := KetQua{Degraded: true, NLoc: len(pass)}
	byID := map[string]hoSoDat{}
	var chunks []xephang.Doan
	mem := map[string]float64{}
	for _, p := range pass {
		byID[p.h.ID] = p
		for _, d := range p.h.Doan {
			chunks = append(chunks, xephang.Doan{ID: d.ChunkID, Chu: d.Body})
		}
		if s := diemMem(p.h.LoaiCho, p.h.KhiChat, y); s > 0 {
			mem[p.h.ID] = s
		}
	}
	text := map[string]float64{}
	if len(chunks) > 0 {
		for _, hit := range xephang.Dung(chunks).TimThuat(thuatTruyVan(y), 4*danhSachToiDa) {
			doc, _, _ := strings.Cut(hit.ID, "#")
			if hit.Diem > text[doc] {
				text[doc] = hit.Diem
			}
		}
	}
	fused := fuse(xepDiem(text, danhSachToiDa), xepDiem(mem, memToiDa))
	kq.NUngVien = len(fused)
	for i := range fused {
		fused[i].Co = byID[fused[i].ID].co
	}
	fused = r.xepChuaRo(fused)
	if len(fused) > y.k() {
		fused = fused[:y.k()]
	}
	kq.Quan = fused
	kq.Co = append(coTruyVan(y), coHits(fused)...)
	return kq
}
