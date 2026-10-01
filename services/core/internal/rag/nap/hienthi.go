package nap

import (
	"context"
	"strconv"
	"strings"

	"mobile/services/core/internal/repo"
)

// The evidence fields of a place: what an answer may quote about it (ADR-0051).
// The index stores them with the row (vectordb FMoRong «hien_thi»), so a
// search answers from Milvus alone; the lexical fallback and the live-row
// reader build them with the same function, so every path quotes a place
// the same way.
//
// Keys: ten, loai, diem_den, dia_chi, gio, gia_min_vnd, gia_max_vnd, gia (the
// price as an answer writes it, the value its «gia» citation must equal),
// chua_ro (gio_chua_ro, gia_chua_ro: unknown and not asked about).

// NguonHienThi is what the evidence fields are made of. GioRo and GiaRo say
// whether the hours and the price are known (the filters' reading: hours
// parsed, a minimum price).
type NguonHienThi struct {
	Ten, Loai, DiemDen, DiaChi, Gio string
	GiaMin, GiaMax                  *int64
	GioRo, GiaRo, GiaUoc            bool
}

// Truong are the evidence fields.
func (s NguonHienThi) Truong() map[string]string {
	f := map[string]string{"ten": s.Ten, "loai": s.Loai, "diem_den": s.DiemDen}
	if s.DiaChi != "" {
		f["dia_chi"] = s.DiaChi
	}
	if s.GiaMin != nil {
		f["gia_min_vnd"] = strconv.FormatInt(*s.GiaMin, 10)
	}
	if s.GiaMax != nil {
		f["gia_max_vnd"] = strconv.FormatInt(*s.GiaMax, 10)
	}
	if s.GiaRo {
		if g := DinhDangGia(s.GiaMin, s.GiaMax, s.GiaUoc); g != "" {
			f["gia"] = g
		}
	}
	if s.Gio != "" {
		f["gio"] = s.Gio
	}
	var chuaRo []string
	if !s.GioRo {
		chuaRo = append(chuaRo, "gio_chua_ro")
	}
	if !s.GiaRo {
		chuaRo = append(chuaRo, "gia_chua_ro")
	}
	if len(chuaRo) > 0 {
		f["chua_ro"] = strings.Join(chuaRo, ",")
	}
	return f
}

// TruongHienThi are p's evidence fields. h is p's profile (DungHoSo: the
// hours it could read); giaUoc says the price is estimated from dishes
// (place_facts.gia_uoc, while the fact is live).
func TruongHienThi(p repo.Place, h HoSoQuan, giaUoc bool) map[string]string {
	s := NguonHienThi{Ten: p.Name, Loai: p.Category, DiemDen: p.DestinationID, GiaMin: p.PriceMinVND, GiaMax: p.PriceMaxVND,
		GioRo: h.Lich != nil, GiaRo: p.PriceMinVND != nil, GiaUoc: giaUoc}
	if p.Address != nil {
		s.DiaChi = *p.Address
	}
	if p.OpenHours != nil {
		s.Gio = *p.OpenHours
	}
	return s.Truong()
}

// DinhDangGia writes a per-person price the way an answer states it:
// «35.000 đ/người», «35.000–60.000 đ/người», and «khoảng … (ước)» when it is
// estimated from dish prices. No minimum is no price (""), whatever the
// maximum: the filters read a price as known only with its minimum.
func DinhDangGia(lo, hi *int64, uoc bool) string {
	if lo == nil {
		return ""
	}
	s := soVND(*lo)
	if hi != nil && *hi > *lo {
		s += "–" + soVND(*hi)
	}
	s += " đ/người"
	if uoc {
		s = "khoảng " + s + " (ước)"
	}
	return s
}

// soVND writes whole đồng with a dot every three digits.
func soVND(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	d := strconv.FormatInt(v, 10)
	var b strings.Builder
	for i, c := range d {
		if i > 0 && (len(d)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// DocGiaUoc reads which of ids carry a live estimated price
// (place_facts.gia_uoc, unexpired; ingest's table, absent before its
// schema v3).
func DocGiaUoc(ctx context.Context, q Querier, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	var co bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('place_facts') IS NOT NULL`).Scan(&co); err != nil || !co {
		return out, err
	}
	rows, err := q.Query(ctx, `SELECT place_id FROM place_facts
		WHERE gia_uoc AND het_han_at > clock_timestamp() AND place_id = ANY($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
