// Package receipt reads what a bill reader transcribed into integer đồng.
// It is the Go port of services/api/app/domain/receipt.py (ADR-0051): the
// model copies strings off the paper; this package decides, deterministically,
// what they are worth, and refuses what it cannot read exactly. It performs
// no I/O and never reconciles the bill's arithmetic.
//
// Money law 1: no float touches an amount. The one float here is the model's
// legibility confidence, truncated to a percentage as Python's int(c*100).
// testdata/python_receipt*.json pins every function to the Python oracle.
package receipt

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Confidence gates, in percent.
const (
	ConfidenceFloor  = 50
	ConfidenceReview = 90
)

// MaxAmountVND is the ledger's ceiling (app/domain/contract.py).
const MaxAmountVND int64 = 1_000_000_000_000

// The document types the reader chooses among; only LoaiHoaDon is admitted.
const (
	DocumentTypeReceipt   = "receipt"
	DocumentTypePriceList = "price_list"
	DocumentTypeOther     = "other"
)

// Error is one stable refusal, named as Python's ReceiptError codes.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func refuse(code string) error { return &Error{Code: code} }

func unreadable() error { return refuse("UNREADABLE_AMOUNT") }

// ws is Python's `\s` for str patterns.
const ws = `[\t\n\v\f\r\x1c-\x1f \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

const currencyMarker = `(?:VND|VNĐ|đồng|dong|đ|₫|d)`

var (
	leadingMarker  = regexp.MustCompile(`(?i)^` + ws + `*` + currencyMarker + ws + `*(.*?)` + ws + `*$`)
	trailingMarker = regexp.MustCompile(`(?i)^` + ws + `*(.*?)` + ws + `*` + currencyMarker + ws + `*$`)
	// Bare "trăm" is absent on purpose: "2 trăm" is 200 đồng on a price list
	// and 200000 in conversation.
	suffixPattern = regexp.MustCompile(`(?i)^(.+?)` + ws + `*(trăm` + ws + `*nghìn|trăm` + ws + `*ngàn|nghìn|ngàn|triệu|tr|k)$`)
	fractional    = regexp.MustCompile(`^(\p{Nd}+)[.,](\p{Nd}+)$`)
	// A count printed beside the dish, "X4" or "4x": digits wrapped in one
	// marker and nothing else.
	quantityMarker = regexp.MustCompile(`(?i)^(?:[x×]` + ws + `*(\p{Nd}+)|(\p{Nd}+)` + ws + `*[x×])$`)
)

var suffixMultipliers = map[string]int64{
	"trămnghìn": 100_000,
	"trămngàn":  100_000,
	"nghìn":     1_000,
	"ngàn":      1_000,
	"k":         1_000,
	"triệu":     1_000_000,
	"tr":        1_000_000,
}

func stripCurrencyMarker(value string) string {
	if m := leadingMarker.FindStringSubmatch(value); m != nil {
		value = m[1]
	}
	if m := trailingMarker.FindStringSubmatch(value); m != nil {
		value = m[1]
	}
	return pyStrip(value)
}

func checked(v int64) (int64, error) {
	if v < 0 || v > MaxAmountVND {
		return 0, unreadable()
	}
	return v, nil
}

// maxDigits is len(str(MaxAmountVND)).
var maxDigits = len(strconv.FormatInt(MaxAmountVND, 10))

// parseDigits is _parse_amount_digits: leading ASCII zeros dropped, at most
// as many digits as the ceiling has, then int().
func parseDigits(value string) (int64, error) {
	significant := strings.TrimLeft(value, "0")
	if significant == "" {
		significant = "0"
	}
	if utf8.RuneCountInString(significant) > maxDigits {
		return 0, unreadable()
	}
	var n int64
	for _, r := range significant {
		n = n*10 + int64(digitValue(r))
	}
	return n, nil
}

// grouped is `\d{1,3}([., ])\d{3}(?:\1\d{3})*` in full: its separator, or ok
// false.
func grouped(value string) (rune, bool) {
	runes := []rune(value)
	i := 0
	for i < len(runes) && pyDigit(runes[i]) {
		i++
	}
	if i < 1 || i > 3 || i == len(runes) {
		return 0, false
	}
	sep := runes[i]
	if sep != '.' && sep != ',' && sep != ' ' {
		return 0, false
	}
	for i < len(runes) {
		if runes[i] != sep || i+4 > len(runes) {
			return 0, false
		}
		for _, r := range runes[i+1 : i+4] {
			if !pyDigit(r) {
				return 0, false
			}
		}
		i += 4
	}
	return sep, true
}

func mulAdd(a, m, add int64) (int64, error) {
	if a > (math.MaxInt64-add)/m {
		return 0, unreadable()
	}
	return checked(a*m + add)
}

func parseSuffixed(number string, multiplier int64) (int64, error) {
	if _, ok := grouped(number); ok {
		return 0, unreadable()
	}
	if allDigits(number) {
		n, err := parseDigits(number)
		if err != nil {
			return 0, err
		}
		return mulAdd(n, multiplier, 0)
	}
	m := fractional.FindStringSubmatch(number)
	if m == nil {
		return 0, unreadable()
	}
	whole, fraction := m[1], []rune(m[2])
	// How many digits the fractional part is worth, read off the multiplier.
	scale := len(strconv.FormatInt(multiplier, 10)) - 1
	if len(fraction) > scale {
		for _, r := range fraction[scale:] {
			if r != '0' {
				return 0, unreadable()
			}
		}
		fraction = fraction[:scale]
	}
	var frac int64
	for _, r := range fraction {
		frac = frac*10 + int64(digitValue(r))
	}
	for i := len(fraction); i < scale; i++ {
		frac *= 10
	}
	n, err := parseDigits(whole)
	if err != nil {
		return 0, err
	}
	return mulAdd(n, multiplier, frac)
}

// NormalizeVND reads one unambiguous Vietnamese amount as exact integer
// đồng: "165.000đ", "180k", "1,5tr", "2 trăm nghìn". Anything else is
// UNREADABLE_AMOUNT. text is whatever the model returned; only a string
// can be an amount.
func NormalizeVND(text any) (int64, error) {
	s, ok := text.(string)
	if !ok {
		return 0, unreadable()
	}
	value := collapseSpace(strings.NewReplacer(" ", " ", " ", " ").Replace(s))
	value = stripCurrencyMarker(pyStrip(value))
	if value == "" {
		return 0, unreadable()
	}
	if m := suffixPattern.FindStringSubmatch(value); m != nil {
		multiplier, ok := suffixMultipliers[dropSpace(strings.ToLower(m[2]))]
		if !ok {
			return 0, unreadable()
		}
		return parseSuffixed(pyStrip(m[1]), multiplier)
	}
	if allDigits(value) {
		n, err := parseDigits(value)
		if err != nil {
			return 0, err
		}
		return checked(n)
	}
	if sep, ok := grouped(value); ok {
		n, err := parseDigits(strings.ReplaceAll(value, string(sep), ""))
		if err != nil {
			return 0, err
		}
		return checked(n)
	}
	return 0, unreadable()
}

// parseBig is int() of a run of Nd digits.
func parseBig(digits string) *big.Int {
	n := new(big.Int)
	ten := big.NewInt(10)
	for _, r := range digits {
		n.Mul(n, ten)
		n.Add(n, big.NewInt(int64(digitValue(r))))
	}
	return n
}

// readQuantity treats "no count printed" (no key, null, blank) as one of the
// item, and refuses a printed count it cannot read exactly.
func readQuantity(item map[string]any) (*big.Int, error) {
	raw, ok := item["quantity_text"]
	if !ok || raw == nil {
		return big.NewInt(1), nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil, refuse("INVALID_QUANTITY")
	}
	stripped := pyStrip(s)
	if stripped == "" {
		return big.NewInt(1), nil
	}
	if !allDigits(stripped) {
		m := quantityMarker.FindStringSubmatch(stripped)
		if m == nil {
			return nil, refuse("INVALID_QUANTITY")
		}
		stripped = m[1] + m[2]
	}
	q := parseBig(stripped)
	if q.Sign() <= 0 {
		return nil, refuse("INVALID_QUANTITY")
	}
	return q, nil
}

// jsonNumber is json.Number as the reader's decoder gives it, named by its
// method so this package stays free of encoding/json.
type jsonNumber interface{ Float64() (float64, error) }

// number reads a JSON number as the reader's decoder gives it (json.Number)
// or as a test hands it over (float64, int64).
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case jsonNumber:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

// readConfidence is the model's legibility estimate in 0..1, as a
// percentage truncated toward zero.
func readConfidence(raw map[string]any) (int, error) {
	v, ok := raw["confidence"]
	if !ok {
		return 0, refuse("INVALID_CONFIDENCE")
	}
	f, ok := number(v)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1 {
		return 0, refuse("INVALID_CONFIDENCE")
	}
	return int(f * 100), nil
}

// Item is one line of the bill.
type Item struct {
	Name     string
	Quantity *big.Int
	// UnitPriceVND is nil when the bill printed none and the line total does
	// not divide by the count.
	UnitPriceVND *int64
	LineTotalVND int64
}

// Reading is a normalized bill. Confidence is kept for the gate; the route
// never publishes it.
type Reading struct {
	Items              []Item
	ItemsTotalVND      int64
	TotalVND           *int64
	TotalsAgree        *bool
	TotalDifferenceVND *int64
	Confidence         int
	NeedsReview        bool
	Warnings           []string
}

// ReadScannedDocument admits only a legible receipt, then normalizes it.
// Legibility first: under ConfidenceFloor the type is as untrustworthy as the
// amounts, and "chụp lại" is the true instruction. Then the type, fail-closed:
// only the exact string "receipt" opens the gate.
func ReadScannedDocument(raw any) (Reading, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return Reading{}, refuse("INVALID_RECEIPT")
	}
	confidence, err := readConfidence(m)
	if err != nil {
		return Reading{}, err
	}
	if confidence < ConfidenceFloor {
		return Reading{}, refuse("RECEIPT_TOO_BLURRY")
	}
	switch m["document_type"] {
	case DocumentTypeReceipt:
		return ReadReceipt(m)
	case DocumentTypePriceList:
		return Reading{}, refuse("NOT_A_RECEIPT_PRICE_LIST")
	}
	return Reading{}, refuse("NOT_A_RECEIPT")
}

// ReadReceipt normalizes one reading without reconciling its independent
// amounts: every printed number is kept, a disagreement is a warning.
func ReadReceipt(raw map[string]any) (Reading, error) {
	items, ok := raw["items"].([]any)
	if !ok {
		return Reading{}, refuse("INVALID_RECEIPT")
	}
	confidence, err := readConfidence(raw)
	if err != nil {
		return Reading{}, err
	}
	if confidence < ConfidenceFloor {
		return Reading{}, refuse("RECEIPT_TOO_BLURRY")
	}
	if len(items) == 0 {
		return Reading{}, refuse("NO_ITEMS_READ")
	}
	totalText, ok := raw["total_text"]
	if !ok {
		return Reading{}, refuse("INVALID_RECEIPT")
	}
	out := Reading{Confidence: confidence, Warnings: []string{}}
	if confidence < ConfidenceReview {
		out.Warnings = append(out.Warnings, "Ảnh bill chưa đủ rõ; hãy kiểm tra từng món và số tiền trước khi xác nhận.")
	}
	for _, rawItem := range items {
		it, ok := rawItem.(map[string]any)
		if !ok {
			return Reading{}, refuse("INVALID_RECEIPT_ITEM")
		}
		name, ok := it["name"].(string)
		if !ok || pyStrip(name) == "" {
			return Reading{}, refuse("INVALID_RECEIPT_ITEM")
		}
		lineText, ok := it["line_total_text"]
		if !ok {
			return Reading{}, refuse("INVALID_RECEIPT_ITEM")
		}
		quantity, err := readQuantity(it)
		if err != nil {
			return Reading{}, err
		}
		line, err := NormalizeVND(lineText)
		if err != nil {
			return Reading{}, err
		}
		var unit *int64
		if unitText := it["unit_price_text"]; unitText != nil {
			u, err := NormalizeVND(unitText)
			if err != nil {
				return Reading{}, err
			}
			unit = &u
			if new(big.Int).Mul(big.NewInt(u), quantity).Cmp(big.NewInt(line)) != 0 {
				out.Warnings = append(out.Warnings, `Đơn giá in trên bill của "`+name+`" nhân số lượng không khớp thành tiền; giữ nguyên cả hai số.`)
			}
		} else {
			q, r := new(big.Int).QuoRem(big.NewInt(line), quantity, new(big.Int))
			if r.Sign() == 0 {
				u := q.Int64()
				unit = &u
			}
		}
		out.Items = append(out.Items, Item{Name: name, Quantity: quantity, UnitPriceVND: unit, LineTotalVND: line})
		// A line is at most MaxAmountVND and a reading at most the model's
		// 16 MiB answer, so the sum stays far inside int64.
		out.ItemsTotalVND += line
	}
	if totalText == nil {
		out.Warnings = append(out.Warnings, "Không đọc thấy tổng in trên bill để đối chiếu với tổng các dòng.")
	} else {
		total, err := NormalizeVND(totalText)
		if err != nil {
			return Reading{}, err
		}
		diff := total - out.ItemsTotalVND
		agree := diff == 0
		out.TotalVND, out.TotalDifferenceVND, out.TotalsAgree = &total, &diff, &agree
		if !agree {
			if confidence < ConfidenceReview {
				out.Warnings = append(out.Warnings, fmt.Sprintf("Tổng in trên bill chênh %+d đồng so với tổng các dòng, nhưng ảnh chưa đủ rõ nên chênh lệch này có thể do đọc sai dòng chứ không phải do bill; đối chiếu từng dòng với tờ giấy trước khi xác nhận.", diff))
			} else {
				out.Warnings = append(out.Warnings, fmt.Sprintf("Tổng in trên bill chênh %+d đồng so với tổng các dòng; giữ nguyên cả hai số.", diff))
			}
		}
	}
	// Derived from the warnings: the one field the app branches on to demand
	// per-item confirmation.
	out.NeedsReview = len(out.Warnings) > 0
	return out, nil
}
