package nap

import (
	"encoding/json"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/ingest"
)

// The three facets of a place (owner, 2026-09-28): what the place is, what
// being there is like, what to eat. Each is embedded on its own so a query
// about the mood meets the experience text and a query about a dish meets
// the food text, neither diluted by the other.
const (
	FacetHoSo       = "ho_so"
	FacetTraiNghiem = "trai_nghiem"
	FacetMonAn      = "mon_an"
	FacetMuc        = "muc"
)

// FacetsQuan are every facet a place may have.
var FacetsQuan = []string{FacetHoSo, FacetTraiNghiem, FacetMonAn}

// maxRuneMuc bounds one value of the feed's review block. The feed's longest
// is ~1,200 runes; a value past this is not prose a person wrote for a card.
const maxRuneMuc = 4000

// mucReview is one key of the feed's review block (vnlocal place.v1
// `review`), with the label its value is written under and its facet. The
// order is the text's order. bang_chung (people's verbatim words) is never
// read: it stays with the feed (vnlocal HANDOFF-KET-NOI.md §7).
type mucReview struct {
	key, nhan, facet string
}

var mucReviews = []mucReview{
	{"tai_sao_dang_den", "Vì sao đáng đến", FacetHoSo},
	{"diem_manh", "Điểm mạnh", FacetHoSo},
	{"khong_khi", "Không khí", FacetTraiNghiem},
	{"phu_hop_voi", "Phù hợp với", FacetTraiNghiem},
	{"khung_gio_dep_nhat", "Giờ đẹp nhất", FacetTraiNghiem},
	{"trai_nghiem_phai_thu", "Trải nghiệm phải thử", FacetTraiNghiem},
	{"luu_y", "Lưu ý", FacetTraiNghiem},
	{"thieu_gi", "Còn thiếu", FacetTraiNghiem},
	{"mon_phai_thu", "Món phải thử", FacetMonAn},
	{"huong_vi_chu_dao", "Hương vị chủ đạo", FacetMonAn},
	{"khau_vi_phu_hop", "Khẩu vị phù hợp", FacetMonAn},
	{"muc_gia", "Mức giá", FacetMonAn},
}

// monFields is the order a dish object of mon_phai_thu is written in.
var monFields = []string{"ten", "mo_ta", "huong_vi", "cach_an", "gia"}

// dongReview reads the feed's review block into labelled lines per facet.
// Every value passes promptsafety.TextSafe; an unsafe one is left out and
// counted. A block that is not an object (the seed catalogue's list of
// review bodies) yields nothing here: danhGia reads that shape.
func dongReview(raw json.RawMessage) (map[string][]string, int) {
	out := map[string][]string{}
	var block map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &block) != nil {
		return out, 0
	}
	cachLy := 0
	safe := func(s string) (string, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return "", false
		}
		if !promptsafety.TextSafe(s, maxRuneMuc) {
			cachLy++
			return "", false
		}
		return s, true
	}
	for _, m := range mucReviews {
		v, ok := block[m.key]
		if !ok {
			continue
		}
		var parts []string
		var s string
		var list []json.RawMessage
		switch {
		case json.Unmarshal(v, &s) == nil:
			if t, ok := safe(s); ok {
				parts = append(parts, t)
			}
		case json.Unmarshal(v, &list) == nil:
			for _, item := range list {
				var one string
				var obj map[string]json.RawMessage
				switch {
				case json.Unmarshal(item, &one) == nil:
					if t, ok := safe(one); ok {
						parts = append(parts, t)
					}
				case json.Unmarshal(item, &obj) == nil:
					var fs []string
					for _, f := range monFields {
						var fv string
						if json.Unmarshal(obj[f], &fv) == nil {
							if t, ok := safe(fv); ok {
								fs = append(fs, t)
							}
						}
					}
					if len(fs) > 0 {
						parts = append(parts, strings.Join(fs, " — "))
					}
				}
			}
		}
		if len(parts) == 0 {
			continue
		}
		sep := "; "
		if m.key == "mon_phai_thu" {
			sep = "\n"
		}
		out[m.facet] = append(out[m.facet], m.nhan+": "+strings.Join(parts, sep))
	}
	return out, cachLy
}

// tenTinh is the province a destination id of the feed names
// («d-tinh-79» → «Thành phố Hồ Chí Minh»), or "" for any other id.
func tenTinh(diemDen string) string {
	rest, ok := strings.CutPrefix(diemDen, "d-tinh-")
	if !ok {
		return ""
	}
	code, err := strconv.Atoi(rest)
	if err != nil {
		return ""
	}
	for _, p := range ingest.Provinces {
		if int(p.Code) == code {
			return p.Name
		}
	}
	return ""
}
