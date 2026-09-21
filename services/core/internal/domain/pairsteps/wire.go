package pairsteps

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/pyuuid"
)

// Object is a JSON object as json.loads builds a dict: keys in document order,
// each once.
type Object struct {
	Keys   []string
	Values []any
}

// Get is `key in dict` and `dict[key]`.
func (o *Object) Get(key string) (any, bool) {
	for i, name := range o.Keys {
		if name == key {
			return o.Values[i], true
		}
	}
	return nil, false
}

// WireStop is PaperStop.
type WireStop struct {
	Gio     string
	Viec    string
	PlaceID *string
	CanKiem bool
}

// WireContent is PaperContent.
type WireContent struct {
	Ngay  pairpaper.Date
	Chang []WireStop
}

// VersionView is PaperVersionResponse.
type VersionView struct {
	Version             int
	Content             WireContent
	LyDo                *string
	AuthorType          string
	SentAt              *time.Time
	SentBy              *string
	MyResponse          *string
	TheirAgreed         bool
	ViewedByRecipientAt *time.Time
}

// PaperView is PaperResponse.
type PaperView struct {
	ID           string
	State        string
	Version      int
	AuthorType   string
	SentBy       *string
	Tuan         pairpaper.Date
	ExpiresAt    time.Time
	OutingID     *string
	CoTheGhiDaDi bool
	Versions     []VersionView
	Keeps        []Keep
}

func unreadable() error {
	return refusal(409, "paper_wrong_state", "Tờ giấy này không đọc được.")
}

// NoiDungWire is _noi_dung_wire: stored content as PaperContent, or the 409
// every KeyError, TypeError and ValueError on the way becomes.
func NoiDungWire(content any) (WireContent, error) {
	object, ok := content.(*Object)
	if !ok {
		return WireContent{}, unreadable() // content["ngay"] on a non-dict
	}
	raw, found := object.Get("ngay")
	if !found {
		return WireContent{}, unreadable()
	}
	ngay, ok := dateFromISOFormat(pyStr(raw))
	if !ok {
		return WireContent{}, unreadable()
	}
	out := WireContent{Ngay: ngay, Chang: []WireStop{}}
	chang, found := object.Get("chang")
	if !found {
		return out, nil
	}
	var stops []any
	switch value := chang.(type) {
	case []any:
		stops = value
	case *Object:
		// Iterating a dict yields its keys, and a str key is not subscriptable
		// by "gio".
		if len(value.Keys) > 0 {
			return WireContent{}, unreadable()
		}
	case string:
		if value != "" {
			return WireContent{}, unreadable()
		}
	default:
		return WireContent{}, unreadable() // not iterable
	}
	for _, item := range stops {
		stop, ok := item.(*Object)
		if !ok {
			return WireContent{}, unreadable()
		}
		gio, hasGio := stop.Get("gio")
		viec, hasViec := stop.Get("viec")
		if !hasGio || !hasViec {
			return WireContent{}, unreadable()
		}
		var placeID *string
		if place, found := stop.Get("place_id"); found && place != nil && place != "" {
			id, ok := pyuuid.Parse(pyStr(place))
			if !ok {
				return WireContent{}, unreadable()
			}
			placeID = &id
		}
		canKiem := true
		if value, found := stop.Get("can_kiem"); found {
			canKiem = truthy(value)
		}
		out.Chang = append(out.Chang, WireStop{Gio: pyStr(gio), Viec: pyStr(viec), PlaceID: placeID, CanKiem: canKiem})
	}
	return out, nil
}

// NgayCua is _ngay_cua: the day the current version proposes, or nil when it
// cannot be read.
func NgayCua(paper *Paper) *pairpaper.Date {
	current := versionNumbered(paper, int64(paper.CurrentVersion))
	if current == nil {
		return nil
	}
	object, ok := current.Content.(*Object)
	if !ok {
		return nil
	}
	raw, found := object.Get("ngay")
	if !found {
		return nil
	}
	ngay, ok := dateFromISOFormat(pyStr(raw))
	if !ok {
		return nil
	}
	return &ngay
}

// ChangDau is _chang_dau: the first stop of the current version, or nil for
// a row the list cannot read.
func ChangDau(paper *Paper) *SummaryStop {
	current := versionNumbered(paper, int64(paper.CurrentVersion))
	if current == nil {
		return nil
	}
	object, ok := current.Content.(*Object)
	if !ok {
		return nil
	}
	raw, _ := object.Get("chang")
	chang, ok := raw.([]any)
	if !ok || len(chang) == 0 {
		return nil
	}
	first, ok := chang[0].(*Object)
	if !ok {
		return nil
	}
	gio, hasGio := first.Get("gio")
	viec, hasViec := first.Get("viec")
	if !hasGio || !hasViec {
		return nil
	}
	return &SummaryStop{Gio: pyStr(gio), Viec: pyStr(viec)}
}

// CoTheGhiDaDi is _co_the_ghi_da_di: a plan that stands, from its day on
// Vietnam's calendar.
func CoTheGhiDaDi(paper *Paper, now time.Time) bool {
	if pairpaper.HieuLuc(PaperDict(paper), now) != "chot" {
		return false
	}
	ngay := NgayCua(paper)
	return ngay != nil && pairpaper.LocalDate(now).Compare(*ngay) >= 0
}

// WirePaper is _wire_paper: the sheet as actorID may read it.
func WirePaper(paper *Paper, actorID string, now time.Time) (PaperView, error) {
	view := PaperView{
		ID:           paper.ID,
		State:        pairpaper.HieuLuc(PaperDict(paper), now),
		Version:      paper.CurrentVersion,
		AuthorType:   "human",
		Tuan:         paper.Tuan,
		ExpiresAt:    paper.ExpiresAt,
		OutingID:     paper.OutingID,
		CoTheGhiDaDi: CoTheGhiDaDi(paper, now),
		Versions:     []VersionView{},
		Keeps:        append([]Keep{}, paper.Keeps...),
	}
	if current := versionNumbered(paper, int64(paper.CurrentVersion)); current != nil {
		view.AuthorType = current.AuthorType
		view.SentBy = current.SentBy
	}
	for i := range paper.Versions {
		version, err := wireVersion(&paper.Versions[i], paper, actorID)
		if err != nil {
			return PaperView{}, err
		}
		view.Versions = append(view.Versions, version)
	}
	return view, nil
}

// wireVersion is _wire_paper_version: the reader's own last answer, whether
// the other agreed, and when the recipient looked, told only to the sender.
func wireVersion(version *Version, paper *Paper, actorID string) (VersionView, error) {
	view := VersionView{
		Version:    version.Version,
		LyDo:       version.LyDo,
		AuthorType: version.AuthorType,
		SentAt:     version.SentAt,
		SentBy:     version.SentBy,
	}
	for _, row := range paper.Responses {
		if row.Version != version.Version {
			continue
		}
		if row.PersonID == actorID {
			kind := row.Kind
			view.MyResponse = &kind
		} else if row.Kind == "dong_y" {
			view.TheirAgreed = true
		}
	}
	if version.SentBy != nil && *version.SentBy == actorID {
		for _, row := range paper.Views {
			if row.Version == version.Version && row.PersonID != actorID {
				seen := row.SeenAt
				view.ViewedByRecipientAt = &seen
				break
			}
		}
	}
	content, err := NoiDungWire(version.Content)
	if err != nil {
		return VersionView{}, err
	}
	view.Content = content
	return view, nil
}

// truthy is Python's bool() of a stored JSON value.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case *big.Int:
		return v.Sign() != 0
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case *Object:
		return len(v.Keys) > 0
	}
	panic(unsupported(value))
}

func unsupported(value any) string {
	return fmt.Sprintf("pairsteps: %T is not a stored JSON value", value)
}

// pyStr is str() of a stored JSON value.
func pyStr(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return pyRepr(value)
}

// pyRepr is repr() of a stored JSON value.
func pyRepr(value any) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	case string:
		return strRepr(v)
	case *big.Int:
		return v.String()
	case float64:
		return floatRepr(v)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = pyRepr(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Object:
		parts := make([]string, len(v.Keys))
		for i, key := range v.Keys {
			parts[i] = strRepr(key) + ": " + pyRepr(v.Values[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	panic(unsupported(value))
}

// strRepr is repr() of a str in CPython 3.12, for valid UTF-8 (a JSONB string
// never holds a lone surrogate). Printable is str.isprintable, which Go's
// unicode.IsPrint spells the same way over the same Unicode 15.0 tables.
func strRepr(text string) string {
	quote := '\''
	if strings.ContainsRune(text, '\'') && !strings.ContainsRune(text, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteRune(quote)
	for _, r := range text {
		switch {
		case r == quote || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7F:
			b.WriteString(`\x` + hexDigits(int(r), 2))
		case r < 0x7F || unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xFF:
			b.WriteString(`\x` + hexDigits(int(r), 2))
		case r <= 0xFFFF:
			b.WriteString(`\u` + hexDigits(int(r), 4))
		default:
			b.WriteString(`\U` + hexDigits(int(r), 8))
		}
	}
	b.WriteRune(quote)
	return b.String()
}

func hexDigits(value, width int) string {
	text := strconv.FormatInt(int64(value), 16)
	return strings.Repeat("0", max(0, width-len(text))) + text
}

// floatRepr is repr() of a float: the shortest digits that round trip, fixed
// notation when the decimal point falls in (-4, 16], exponent notation with
// at least two digits otherwise. (internal/domain/scoring keeps the same rule
// unexported.)
func floatRepr(value float64) string {
	switch {
	case math.IsNaN(value):
		return "nan"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(value) {
		sign, value = "-", -value
	}
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(value, 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	power, _ := strconv.Atoi(exponent)
	point := power + 1
	if point > -4 && point <= 16 {
		switch {
		case point <= 0:
			return sign + "0." + strings.Repeat("0", -point) + digits
		case point >= len(digits):
			return sign + digits + strings.Repeat("0", point-len(digits)) + ".0"
		default:
			return sign + digits[:point] + "." + digits[point:]
		}
	}
	out := sign + digits[:1]
	if len(digits) > 1 {
		out += "." + digits[1:]
	}
	shown, expSign := point-1, "+"
	if shown < 0 {
		shown, expSign = -shown, "-"
	}
	return fmt.Sprintf("%se%s%02d", out, expSign, shown)
}

// unixEpochOrdinal is date(1970, 1, 1).toordinal().
const unixEpochOrdinal = 719163

func ordinal(year, month, day int) int {
	return int(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()/86400) + unixEpochOrdinal
}

func fromOrdinal(value int) pairpaper.Date {
	return pairpaper.DateOf(time.Unix(int64(value-unixEpochOrdinal)*86400, 0).UTC())
}

func isLeap(year int) bool { return year%4 == 0 && (year%100 != 0 || year%400 == 0) }

// validDate is new_date's range check.
func validDate(year, month, day int) (pairpaper.Date, bool) {
	if year < 1 || year > 9999 || month < 1 || month > 12 || day < 1 {
		return pairpaper.Date{}, false
	}
	if day > time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return pairpaper.Date{}, false
	}
	return pairpaper.Date{Year: year, Month: month, Day: day}, true
}

// isoToYMD is iso_to_ymd followed by new_date.
func isoToYMD(isoYear, week, day int) (pairpaper.Date, bool) {
	if isoYear < 1 || isoYear > 9999 {
		return pairpaper.Date{}, false
	}
	if week <= 0 || week >= 53 {
		outOfRange := true
		if week == 53 {
			firstWeekday := (ordinal(isoYear, 1, 1) + 6) % 7
			if firstWeekday == 3 || (firstWeekday == 2 && isLeap(isoYear)) {
				outOfRange = false
			}
		}
		if outOfRange {
			return pairpaper.Date{}, false
		}
	}
	if day <= 0 || day >= 8 {
		return pairpaper.Date{}, false
	}
	firstDay := ordinal(isoYear, 1, 1)
	firstWeekday := (firstDay + 6) % 7
	weekOneMonday := firstDay - firstWeekday
	if firstWeekday > 3 {
		weekOneMonday += 7
	}
	d := fromOrdinal(weekOneMonday + (week-1)*7 + day - 1)
	return validDate(d.Year, d.Month, d.Day)
}

// dateFromISOFormat is date.fromisoformat of CPython 3.12
// (Modules/_datetimemodule.c) over the UTF-8 bytes: 7, 8 or 10 of them, a
// four-digit year, then an ISO week with an optional day or a month and a
// day, a dash used after the year only if used throughout, and whatever
// follows the parsed fields ignored.
func dateFromISOFormat(text string) (pairpaper.Date, bool) {
	n := len(text)
	if n != 7 && n != 8 && n != 10 {
		return pairpaper.Date{}, false
	}
	at := func(i int) byte {
		if i < n {
			return text[i]
		}
		return 0
	}
	p := 0
	digits := func(count int) (int, bool) {
		value := 0
		for i := 0; i < count; i++ {
			digit := at(p) - '0'
			p++
			if digit > 9 {
				return 0, false
			}
			value = value*10 + int(digit)
		}
		return value, true
	}
	year, ok := digits(4)
	if !ok {
		return pairpaper.Date{}, false
	}
	separated := at(p) == '-'
	if separated {
		p++
	}
	if at(p) == 'W' {
		p++
		week, ok := digits(2)
		if !ok {
			return pairpaper.Date{}, false
		}
		day := 1
		if p < n {
			if separated {
				dash := at(p)
				p++
				if dash != '-' {
					return pairpaper.Date{}, false
				}
			}
			if day, ok = digits(1); !ok {
				return pairpaper.Date{}, false
			}
		}
		return isoToYMD(year, week, day)
	}
	month, ok := digits(2)
	if !ok {
		return pairpaper.Date{}, false
	}
	if separated {
		dash := at(p)
		p++
		if dash != '-' {
			return pairpaper.Date{}, false
		}
	}
	day, ok := digits(2)
	if !ok {
		return pairpaper.Date{}, false
	}
	return validDate(year, month, day)
}
