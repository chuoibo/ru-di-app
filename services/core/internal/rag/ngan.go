package rag

import (
	"context"
	"sort"

	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// ToiDaNgan is the most catalogue rows the public search ever hands a model
// (design 04 §7).
const ToiDaNgan = 30

// Ngan is the public search's shortlist and how it was made, in ids, enums
// and counts only.
type Ngan struct {
	Rows []repo.Place
	// DiemDen is what the words resolved to; "" searched every destination.
	DiemDen DiemDenGiai
	// PhienBan is the index version that ranked the hits; 0 for live rows.
	PhienBan int64
	// Trung are how many rows the lexical ranking chose; the rest are the
	// best of the destination for the searcher's taste.
	Trung int
}

// DanhSachNgan is what POST /places/search sends the model instead of the
// whole catalogue: at most ToiDaNgan live rows, of the destination the words
// name when they name one. The rows the words rank come first -- ranked by
// the active index version when there is one, by rag/xephang over live rows
// when there is none -- and the rest of the thirty are the destination's
// best for the searcher's taste (service.ScoreOrZero, the ranking the
// routes' catalogue uses). Every row passed the hard filters the words set
// (an allergy named after «dị ứng», a diet) and none is tombstoned or
// dropped by SafeDeep. It never falls back to the whole catalogue: an empty
// shortlist is an empty shortlist.
func (k Kho) DanhSachNgan(ctx context.Context, cau string, group taste.Profile) (Ngan, error) {
	dests, err := repo.Repository{Q: k.Q}.ListDestinations(ctx)
	if err != nil {
		return Ngan{}, err
	}
	y, dd := DocCau(cau, DiemDenTuRepo(dests))
	y.K = ToiDaNgan
	installed, err := Installed(ctx, k.Q)
	if err != nil {
		return Ngan{}, err
	}
	rows, bia, err := k.hangSong(ctx, installed, y.DiemDen)
	if err != nil {
		return Ngan{}, err
	}
	r := rangBuocCua(y)
	out := Ngan{DiemDen: dd}
	var kq KetQua
	ranked := false
	if installed {
		v, err := k.phienBanActive(ctx)
		if err != nil {
			return Ngan{}, err
		}
		if v != 0 {
			if got, err := k.trongPhienBan(ctx, v, y); err == nil {
				kq, ranked = got, true
			}
		}
	}
	// passed says whether a live row may be shortlisted. Without an index
	// the live ranking already profiled every row; with one, rows are
	// profiled only as the taste order reaches them, so a large destination
	// costs thirty-odd profiles, not all of them.
	var passed func(repo.Place) bool
	if !ranked {
		pass := locSong(rows, bia, r)
		kq = xepHoSo(pass, y, r)
		ok := map[string]bool{}
		for _, p := range pass {
			ok[p.h.ID] = true
		}
		passed = func(row repo.Place) bool { return ok[row.ID] }
	} else {
		passed = func(row repo.Place) bool {
			if bia[row.ID] {
				return false
			}
			h, report := DungHoSo(row)
			if report.Bo {
				return false
			}
			ok, _ := r.dat(h)
			return ok
		}
	}
	out.PhienBan = kq.PhienBan
	byID := map[string]repo.Place{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	taken := map[string]bool{}
	for _, h := range kq.Quan {
		row, ok := byID[h.ID]
		if !ok || taken[h.ID] || len(out.Rows) == ToiDaNgan {
			continue
		}
		taken[h.ID] = true
		out.Rows = append(out.Rows, row)
	}
	out.Trung = len(out.Rows)
	type scored struct {
		row   repo.Place
		score int64
	}
	rest := make([]scored, 0, len(rows))
	for _, row := range rows {
		if !taken[row.ID] {
			rest = append(rest, scored{row, service.ScoreOrZero(service.PlaceRow(row), group)})
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].score != rest[j].score {
			return rest[i].score > rest[j].score
		}
		return rest[i].row.ID < rest[j].row.ID
	})
	for _, s := range rest {
		if len(out.Rows) == ToiDaNgan {
			break
		}
		if passed(s.row) {
			out.Rows = append(out.Rows, s.row)
		}
	}
	return out, nil
}
