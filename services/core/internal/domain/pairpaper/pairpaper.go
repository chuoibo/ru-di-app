// Package pairpaper is services/api/app/domain/pair_paper.py: the sheet of
// paper two people pass each other, its thirteen states, the one transition
// function, and the three questions the service may not answer for itself --
// is this sheet still in force, have both people agreed to this version, and
// may the sender still take it back.
//
// Pure functions over values; the clock is always a parameter. Python's dicts
// become small structs holding exactly the keys the functions read, with a nil
// pointer for None. testdata/python_pair_paper*.json is rendered by
// scripts/render_domain_w8_goldens.py from the real module in the parity API
// image, and oracle_test.go replays every case.
//
// # The week's clock
//
// MUI_GIO is ZoneInfo("Asia/Ho_Chi_Minh") read from the image's tzdata. The
// domain may not load zone files, so the zone is written here as the periods
// that file holds: local mean time (+07:06:30) until 1906, Phu Lien mean time
// at the same offset until 1911, the +07, +08 and +09 periods between 1942 and
// 1975, and +07 since 1975-06-13. An instant converts with the offset in force
// at it, as zoneinfo's fromutc does. HanTuan converts a Monday midnight back
// with zoneinfo's fold=0 rule; no Monday midnight of this zone falls in a gap
// or a fold, which the oracle checks across every transition.
//
// Python's datetime stops at years 1 and 9999 and raises OverflowError past
// them, where Go keeps counting. An instant whose local week leaves that range
// (the first and last days of the calendar) is outside what these functions
// answer for; the service's clock never gets there.
package pairpaper

import (
	"cmp"
	"slices"
	"strconv"
	"time"
)

// MuiGio is the key of MUI_GIO.
const MuiGio = "Asia/Ho_Chi_Minh"

// thuBay is _THU_BAY: Saturday as date.weekday() numbers it.
const thuBay = 5

var (
	paperStates   = [...]string{"nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y", "chot", "da_di", "da_giu", "nghi_tuan", "het_han", "rut", "bo", "huy"}
	openStates    = [...]string{"nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"}
	planStates    = [...]string{"chot", "da_di"}
	terminal      = [...]string{"nghi_tuan", "het_han", "rut", "bo", "huy", "da_giu"}
	authorTypes   = [...]string{"human", "nep"}
	responseKinds = [...]string{"dong_y", "de_nghi_sua"}
)

// PaperStates is PAPER_STATES, in the order the week walks them.
func PaperStates() []string { return slices.Clone(paperStates[:]) }

// OpenStates is OPEN_STATES: still being decided, so a deadline can end them.
func OpenStates() []string { return slices.Clone(openStates[:]) }

// PlanStates is PLAN_STATES: a plan exists.
func PlanStates() []string { return slices.Clone(planStates[:]) }

// Terminal is TERMINAL: the week is closed one way or another.
func Terminal() []string { return slices.Clone(terminal[:]) }

// AuthorTypes is AUTHOR_TYPES.
func AuthorTypes() []string { return slices.Clone(authorTypes[:]) }

// ResponseKinds is RESPONSE_KINDS.
func ResponseKinds() []string { return slices.Clone(responseKinds[:]) }

// IsOpen is `state in OPEN_STATES`.
func IsOpen(state string) bool { return slices.Contains(openStates[:], state) }

// IsPlan is `state in PLAN_STATES`.
func IsPlan(state string) bool { return slices.Contains(planStates[:], state) }

// PaperError is PaperError: a refusal carrying the wire code.
type PaperError struct {
	Code string
}

func (e *PaperError) Error() string { return e.Code }

func refuse(code string) error { return &PaperError{Code: code} }

// Date is datetime.date.
type Date struct {
	Year, Month, Day int
}

// ISOFormat is date.isoformat().
func (d Date) ISOFormat() string {
	return pad(d.Year, 4) + "-" + pad(d.Month, 2) + "-" + pad(d.Day, 2)
}

// Compare orders two dates as Python compares them.
func (d Date) Compare(other Date) int {
	if c := cmp.Compare(d.Year, other.Year); c != 0 {
		return c
	}
	if c := cmp.Compare(d.Month, other.Month); c != 0 {
		return c
	}
	return cmp.Compare(d.Day, other.Day)
}

// AddDays is `date + timedelta(days=n)`.
func (d Date) AddDays(n int) Date {
	return DateOf(time.Date(d.Year, time.Month(d.Month), d.Day+n, 0, 0, 0, 0, time.UTC))
}

// Weekday is date.weekday(): Monday is 0 and Sunday 6.
func (d Date) Weekday() int {
	return (int(time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC).Weekday()) + 6) % 7
}

// DateOf is `.date()` of t on its own wall clock.
func DateOf(t time.Time) Date {
	year, month, day := t.Date()
	return Date{Year: year, Month: int(month), Day: day}
}

func pad(value, width int) string {
	text := strconv.Itoa(value)
	for len(text) < width {
		text = "0" + text
	}
	return text
}

// ISOFormat is datetime.isoformat() of an aware datetime: microseconds only
// when non-zero, and the offset as +HH:MM, with :SS when it has seconds.
func ISOFormat(t time.Time) string {
	_, offset := t.Zone()
	text := DateOf(t).ISOFormat() + "T" + pad(t.Hour(), 2) + ":" + pad(t.Minute(), 2) + ":" + pad(t.Second(), 2)
	if micro := t.Nanosecond() / 1000; micro != 0 {
		text += "." + pad(micro, 6)
	}
	sign := "+"
	if offset < 0 {
		sign, offset = "-", -offset
	}
	text += sign + pad(offset/3600, 2) + ":" + pad(offset%3600/60, 2)
	if offset%60 != 0 {
		text += ":" + pad(offset%60, 2)
	}
	return text
}

// period is one entry of the zone: the Unix second an offset starts, and the
// offset in seconds east of UTC.
type period struct {
	from   int64
	offset int
}

// lmt is the offset before the first transition, local mean time.
const lmt = 7*3600 + 6*60 + 30

var zone = func() []period {
	at := func(year, month, day, hour, minute, second int) int64 {
		return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC).Unix()
	}
	return []period{
		{at(1906, 6, 30, 16, 53, 30), lmt},
		{at(1911, 4, 30, 16, 53, 30), 7 * 3600},
		{at(1942, 12, 31, 16, 0, 0), 8 * 3600},
		{at(1945, 3, 14, 15, 0, 0), 9 * 3600},
		{at(1945, 9, 1, 15, 0, 0), 7 * 3600},
		{at(1947, 3, 31, 17, 0, 0), 8 * 3600},
		{at(1955, 6, 30, 17, 0, 0), 7 * 3600},
		{at(1959, 12, 31, 16, 0, 0), 8 * 3600},
		{at(1975, 6, 12, 16, 0, 0), 7 * 3600},
	}
}()

// offsetAt is the offset in force at a Unix second (zoneinfo's fromutc, which
// reads whole seconds).
func offsetAt(unix int64) int {
	offset := lmt
	for _, p := range zone {
		if unix < p.from {
			break
		}
		offset = p.offset
	}
	return offset
}

// wallOffset is utcoffset() of a wall-clock second with fold=0: a transition
// reaches the wall clock at its instant plus the larger of the offsets on
// either side of it.
func wallOffset(wall int64) int {
	offset := lmt
	for _, p := range zone {
		if wall < p.from+int64(max(offset, p.offset)) {
			break
		}
		offset = p.offset
	}
	return offset
}

// Local is now.astimezone(MUI_GIO): the same instant on Vietnam's wall clock.
func Local(now time.Time) time.Time {
	return now.In(time.FixedZone("", offsetAt(now.Unix())))
}

// LocalDate is now.astimezone(MUI_GIO).date().
func LocalDate(now time.Time) Date { return DateOf(Local(now)) }

// TuanCua is tuan_cua: the Monday of the week now falls in, in Vietnam.
func TuanCua(now time.Time) Date {
	here := LocalDate(now)
	return here.AddDays(-here.Weekday())
}

// HanTuan is han_tuan: midnight ending Sunday, local, as a UTC instant.
func HanTuan(now time.Time) time.Time {
	next := TuanCua(now).AddDays(7)
	wall := time.Date(next.Year, time.Month(next.Month), next.Day, 0, 0, 0, 0, time.UTC).Unix()
	return time.Unix(wall-int64(wallOffset(wall)), 0).UTC()
}

// NgayDeXuat is ngay_de_xuat: this week's Saturday, or today once Saturday has
// gone.
func NgayDeXuat(now time.Time) Date {
	today := LocalDate(now)
	saturday := TuanCua(now).AddDays(thuBay)
	if saturday.Compare(today) >= 0 {
		return saturday
	}
	return today
}

// Paper is `_paper_dict`: the keys every function here reads. A nil ExpiresAt
// is an absent or None `expires_at`.
type Paper struct {
	ID             string
	State          string
	CurrentVersion int
	ExpiresAt      *time.Time
}

// HieuLuc is hieu_luc: the state a reader sees, with the deadline applied to
// an undecided sheet only.
func HieuLuc(paper Paper, now time.Time) string {
	if !IsOpen(paper.State) {
		return paper.State
	}
	if paper.ExpiresAt != nil && !now.Before(*paper.ExpiresAt) {
		return "het_han"
	}
	return paper.State
}

// Row is a response or view row: `version`, `person_id` and, for a response,
// `kind` ("" on a view).
type Row struct {
	Version  int
	PersonID string
	Kind     string
}

// DaDuDongY is da_du_dong_y: two different people agreed to this version.
func DaDuDongY(responses []Row, version int) bool {
	agreed := map[string]bool{}
	for _, row := range responses {
		if row.Kind == "dong_y" && row.Version == version {
			agreed[row.PersonID] = true
		}
	}
	return len(agreed) >= 2
}

// Version is one row of `_versions_as_dicts`, the keys co_the_rut reads. A nil
// SentBy is None.
type Version struct {
	Version    int
	AuthorType string
	SentBy     *string
}

// CoTheRut is co_the_rut: only the human sender, only while the stored state is
// da_gui, and only while nobody else has opened or answered the current
// version.
func CoTheRut(paper Paper, versions []Version, views, responses []Row, actorID string) bool {
	if paper.State != "da_gui" {
		return false
	}
	version := paper.CurrentVersion
	var current *Version
	for i := range versions {
		if versions[i].Version == version {
			current = &versions[i]
			break
		}
	}
	if current == nil || current.AuthorType != "human" {
		return false
	}
	sentBy := ""
	if current.SentBy != nil {
		sentBy = *current.SentBy
	}
	if sentBy != actorID {
		return false
	}
	for _, row := range views {
		if row.Version == version && row.PersonID != actorID {
			return false
		}
	}
	for _, row := range responses {
		if row.Version == version && row.PersonID != actorID {
			return false
		}
	}
	return true
}

// events is _TU: each event and the states it may be applied to.
var events = []struct {
	name string
	from []string
}{
	{"gui", []string{"nhap"}},
	{"xem", []string{"da_gui", "da_xem"}},
	{"dong_y", []string{"da_gui", "da_xem", "dong_y"}},
	{"de_nghi_sua", []string{"da_gui", "da_xem", "dong_y"}},
	{"rut", []string{"da_gui"}},
	{"nghi_tuan", openStates[:]},
	{"bo", []string{"nhap"}},
	{"da_di", []string{"chot"}},
	{"giu", []string{"da_di", "da_giu"}},
	{"huy", []string{"chot"}},
}

// Events is the keys of _TU, in order.
func Events() []string {
	out := make([]string, len(events))
	for i, event := range events {
		out[i] = event.name
	}
	return out
}

// EventSources is _TU[event], and whether event is a key of it.
func EventSources(event string) ([]string, bool) {
	for _, entry := range events {
		if entry.name == event {
			return slices.Clone(entry.from), true
		}
	}
	return nil, false
}

// Facts is chuyen's **facts, each as Python's truthiness reads it.
type Facts struct {
	DuDongY  bool
	CoTheRut bool
	NguoiGhi bool
}

// Chuyen is chuyen: the sheet after the event, or a *PaperError naming the
// refusal. Checks run in Python's order: unknown event, lapsed deadline,
// frozen plan, wrong state, then the event's own fact.
func Chuyen(paper Paper, suKien string, now time.Time, facts Facts) (Paper, error) {
	from, known := EventSources(suKien)
	if !known {
		return Paper{}, refuse("paper_event_unknown")
	}
	state := HieuLuc(paper, now)
	if state == "het_han" && IsOpen(paper.State) {
		return Paper{}, refuse("paper_expired")
	}
	if suKien == "de_nghi_sua" && IsPlan(state) {
		return Paper{}, refuse("paper_frozen")
	}
	if !slices.Contains(from, state) {
		return Paper{}, refuse("paper_wrong_state")
	}
	after := paper
	switch suKien {
	case "gui":
		after.State = "da_gui"
	case "xem":
		after.State = "da_xem"
	case "dong_y":
		after.State = "dong_y"
		if facts.DuDongY {
			after.State = "chot"
		}
	case "de_nghi_sua":
		after.State = "da_gui"
		after.CurrentVersion = paper.CurrentVersion + 1
	case "rut":
		if !facts.CoTheRut {
			return Paper{}, refuse("paper_not_withdrawable")
		}
		after.State = "rut"
	case "nghi_tuan":
		after.State = "huy"
		if state == "nhap" {
			after.State = "nghi_tuan"
		}
	case "bo":
		after.State = "bo"
	case "da_di":
		if !facts.NguoiGhi {
			return Paper{}, refuse("paper_needs_recorder")
		}
		after.State = "da_di"
	case "giu":
		after.State = "da_giu"
	default:
		after.State = "huy"
	}
	return after, nil
}

// RoutineStop is a stop of the routine phac_to_giay reads.
type RoutineStop struct {
	Gio, Viec string
}

// Routine is phac_to_giay's routine. A nil Ngay is a `ngay` that is not a
// date; a nil DiTiep is a falsy `di_tiep` (None or an empty dict); LyDo is
// `str(routine.get("ly_do") or "")`.
type Routine struct {
	Ngay   *Date
	Gio    string
	Viec   string
	DiTiep *RoutineStop
	LyDo   string
}

// Stop is one stop of a stored sheet's content. A nil PlaceID is None.
type Stop struct {
	Gio     string
	Viec    string
	PlaceID *string
	CanKiem bool
}

// Content is a sheet's stored content: {"ngay", "chang"}.
type Content struct {
	Ngay  string
	Chang []Stop
}

// Nguon is the provenance ADR-0019 asks for: {"scope", "dung", "luc"}.
type Nguon struct {
	Scope string
	Dung  []string
	Luc   string
}

// Draft is phac_to_giay's dict: {"content", "ly_do", "nguon"}.
type Draft struct {
	Content Content
	LyDo    string
	Nguon   Nguon
}

// PhacToGiay is phac_to_giay: a fixed template, one main stop and an optional
// next one, every stop unplaced and to be checked. coRangBuoc is the
// truthiness of rang_buoc, the only thing read from it.
func PhacToGiay(routine Routine, coRangBuoc bool, now time.Time) (Draft, error) {
	if routine.Ngay == nil {
		return Draft{}, refuse("paper_draft_needs_date")
	}
	chang := []Stop{{Gio: routine.Gio, Viec: routine.Viec, CanKiem: true}}
	if routine.DiTiep != nil {
		chang = append(chang, Stop{Gio: routine.DiTiep.Gio, Viec: routine.DiTiep.Viec, CanKiem: true})
	}
	dung := []string{"routine"}
	if coRangBuoc {
		dung = append(dung, "rang_buoc")
	}
	return Draft{
		Content: Content{Ngay: routine.Ngay.ISOFormat(), Chang: chang},
		LyDo:    routine.LyDo,
		Nguon:   Nguon{Scope: "chung", Dung: dung, Luc: ISOFormat(now)},
	}, nil
}
