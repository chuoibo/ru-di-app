package routes

import (
	"encoding/json"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// maxHighlightRunes bounds one text of the highlights, as the retrieval side
// bounds one value of the feed's review block (rag/nap maxRuneMuc).
const maxHighlightRunes = 4000

// weekdays is the order a week is shown in, with the label of each.
var weekdays = []struct{ code, label string }{
	{"mon", "Thứ Hai"}, {"tue", "Thứ Ba"}, {"wed", "Thứ Tư"}, {"thu", "Thứ Năm"},
	{"fri", "Thứ Sáu"}, {"sat", "Thứ Bảy"}, {"sun", "Chủ nhật"},
}

var bookAheadLabels = map[string]string{
	"nen": "Nên đặt trước", "bat_buoc": "Cần đặt trước", "khong_can": "Không cần đặt trước",
}

var settingLabels = map[string]string{
	"trong_nha": "Trong nhà", "ngoai_troi": "Ngoài trời", "ca_hai": "Cả trong nhà và ngoài trời",
}

// highlightText trims s and keeps it only when it is non-empty prose that
// passes the same filter every model-bound or card-bound review text passes
// (promptsafety.TextSafe), the one rag/nap.dongReview applies.
func highlightText(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || !promptsafety.TextSafe(s, maxHighlightRunes) {
		return "", false
	}
	return s, true
}

// textsOf reads a review value that is a string or a list of strings into the
// safe, non-empty texts it holds.
func textsOf(raw json.RawMessage) []string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		if t, ok := highlightText(one); ok {
			return []string{t}
		}
		return nil
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var out []string
	for _, item := range list {
		var s string
		if json.Unmarshal(item, &s) != nil {
			continue
		}
		if t, ok := highlightText(s); ok {
			out = append(out, t)
		}
	}
	return out
}

func stringList(items []string) pyjson.List {
	out := pyjson.List{}
	for _, s := range items {
		out = append(out, pyjson.String(s))
	}
	return out
}

// scalarText reads a string, or a number written as text, from a dish field.
func scalarText(raw json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return highlightText(s)
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil && n.String() != "" {
		return highlightText(n.String())
	}
	return "", false
}

// buildHighlights is the structured «what to know» of a place: the feed's
// review block (an object), the live web facts, the ten-way category and the
// search attributes. Every field is optional and absent when there is nothing
// to say (never an empty string, never an empty list). It returns nil when the
// whole object would be empty.
//
// Deliberately never read: bang_chung (people's verbatim words), thieu_gi
// (a note about the record, not the place), di_ung, an_kieng, chen_lenh and
// tin_cay (nothing here may read as an allergen-safety claim).
func buildHighlights(reviews json.RawMessage, extras repo.PlaceExtras) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	var block map[string]json.RawMessage
	if len(reviews) == 0 || json.Unmarshal(reviews, &block) != nil {
		block = nil
	}
	// Single texts: a list the feed wrote where a sentence was expected is
	// joined, so the app always receives one string for these keys.
	for _, key := range []string{"tai_sao_dang_den", "khong_khi", "khung_gio_dep_nhat", "muc_gia"} {
		if raw, ok := block[key]; ok {
			if texts := textsOf(raw); len(texts) > 0 {
				out.Set(key, pyjson.String(strings.Join(texts, "; ")))
			}
		}
	}
	if raw, ok := block["mon_phai_thu"]; ok {
		if dishes := dishesOf(raw); len(dishes) > 0 {
			out.Set("mon_phai_thu", dishes)
		}
	}
	for _, key := range []string{"phu_hop_voi", "khau_vi_phu_hop", "luu_y", "trai_nghiem_phai_thu"} {
		if raw, ok := block[key]; ok {
			if texts := textsOf(raw); len(texts) > 0 {
				out.Set(key, stringList(texts))
			}
		}
	}
	if f := extras.Facts; f != nil {
		if price := priceOf(f); price != nil {
			out.Set("gia", price)
		}
		if hours := hoursOf(f.Hours); len(hours) > 0 {
			out.Set("gio_mo_cua", hours)
		}
		if note, ok := highlightText(deref0(f.HoursNote)); ok {
			out.Set("gio_ghi_chu", pyjson.String(note))
		}
		if v := labelled(deref0(f.BookAhead), bookAheadLabels); v != nil {
			out.Set("can_dat_truoc", v)
		}
		if v := labelled(deref0(f.Setting), settingLabels); v != nil {
			out.Set("trong_nha_ngoai_troi", v)
		}
		if d := durationOf(f); d != nil {
			out.Set("thoi_luong_phut", d)
		}
	}
	if list := vocabulary(extras.Categories, tuvung.DanhMuc); len(list) > 0 {
		out.Set("danh_muc", list)
	}
	if list := vocabulary(extras.Vibes, tuvung.KhiChat); len(list) > 0 {
		out.Set("khi_chat", list)
	}
	var dishes []string
	for _, s := range extras.MainDishes {
		if t, ok := highlightText(s); ok {
			dishes = append(dishes, t)
		}
	}
	if len(dishes) > 0 {
		out.Set("mon_chinh", stringList(dishes))
	}
	if out.Len() == 0 {
		return nil
	}
	return out
}

func deref0(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func dishesOf(raw json.RawMessage) pyjson.List {
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	out := pyjson.List{}
	for _, item := range items {
		name, ok := scalarText(item["ten"])
		if !ok {
			continue
		}
		dish := pyjson.NewOrderedMap()
		dish.Set("ten", pyjson.String(name))
		for _, key := range []string{"mo_ta", "huong_vi", "cach_an", "gia"} {
			if v, ok := item[key]; ok {
				if t, ok := scalarText(v); ok {
					dish.Set(key, pyjson.String(t))
				}
			}
		}
		out = append(out, dish)
	}
	return out
}

// priceOf is the live per-person spend. `uoc` is always present: an estimate
// built from dish prices must be shown as one («khoảng … (ước)»).
func priceOf(f *repo.PlaceFactsLive) *pyjson.OrderedMap {
	if f.PriceMinVND == nil {
		return nil
	}
	out := pyjson.NewOrderedMap()
	out.Set("min_vnd", pyjson.NewInt(*f.PriceMinVND))
	if f.PriceMaxVND != nil && *f.PriceMaxVND > *f.PriceMinVND {
		out.Set("max_vnd", pyjson.NewInt(*f.PriceMaxVND))
	}
	out.Set("uoc", pyjson.Bool(f.PriceEstimated))
	return out
}

// hoursOf lists the opening hours Monday to Sunday, one row per day the feed
// gave (a day it did not give is unknown, not closed).
func hoursOf(raw json.RawMessage) pyjson.List {
	var rows []struct {
		Thu   string `json:"thu"`
		Mo    string `json:"mo"`
		Dong  string `json:"dong"`
		QuaDe bool   `json:"qua_dem"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	byDay := map[string]int{}
	for i, r := range rows {
		if _, seen := byDay[r.Thu]; !seen {
			byDay[r.Thu] = i
		}
	}
	out := pyjson.List{}
	for _, day := range weekdays {
		i, ok := byDay[day.code]
		if !ok {
			continue
		}
		r := rows[i]
		if !clockOK(r.Mo) || !clockOK(r.Dong) {
			continue
		}
		entry := pyjson.NewOrderedMap()
		entry.Set("thu", pyjson.String(day.code))
		entry.Set("nhan", pyjson.String(day.label))
		entry.Set("mo", pyjson.String(r.Mo))
		entry.Set("dong", pyjson.String(r.Dong))
		entry.Set("qua_dem", pyjson.Bool(r.QuaDe))
		out = append(out, entry)
	}
	return out
}

// clockOK accepts «H:MM» or «HH:MM» only.
func clockOK(s string) bool {
	if len(s) < 4 || len(s) > 5 || s[len(s)-3] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:len(s)-3])
	m, err2 := strconv.Atoi(s[len(s)-2:])
	return err1 == nil && err2 == nil && h >= 0 && h <= 24 && m >= 0 && m <= 59
}

func labelled(code string, labels map[string]string) *pyjson.OrderedMap {
	label, ok := labels[code]
	if !ok {
		return nil
	}
	out := pyjson.NewOrderedMap()
	out.Set("ma", pyjson.String(code))
	out.Set("nhan", pyjson.String(label))
	return out
}

func durationOf(f *repo.PlaceFactsLive) *pyjson.OrderedMap {
	if f.DurationMin == nil && f.DurationMax == nil {
		return nil
	}
	out := pyjson.NewOrderedMap()
	if f.DurationMin != nil {
		out.Set("min", pyjson.NewInt(*f.DurationMin))
	}
	if f.DurationMax != nil {
		out.Set("max", pyjson.NewInt(*f.DurationMax))
	}
	return out
}

// vocabulary turns closed-vocabulary ids into {id, nhan}, dropping what the
// vocabulary does not name («khong_ro» included) and repeats.
func vocabulary(ids []string, v *tuvung.TuVung) pyjson.List {
	out := pyjson.List{}
	seen := map[string]bool{}
	for _, id := range ids {
		label := v.Nhan(id)
		if label == "" || id == tuvung.KhongRo || seen[id] {
			continue
		}
		seen[id] = true
		entry := pyjson.NewOrderedMap()
		entry.Set("id", pyjson.String(id))
		entry.Set("nhan", pyjson.String(label))
		out = append(out, entry)
	}
	return out
}
