// Package journey is services/api/app/domain/journey.py: the bounded itinerary
// search over directed road costs, with no IO and no framework.
//
// Pure functions over values. A Python dict becomes a small struct holding
// exactly the keys the functions read, with a nil pointer for None; a
// `Cost | None` cell becomes a *Cost. The clock never appears: a stop is a
// wall-clock minute of a day, and every answer is that same minute arithmetic.
//
// Errors these functions return stand for the exceptions Python raises, which
// the callers in app/journey/preview.py catch (ValueError, KeyError,
// TypeError) or let end the request:
//
//   - *ValueError is ValueError, with Python's message;
//   - *IndexError is "list index out of range", which Python's preview does
//     NOT catch: a costs list shorter than the stops it describes ends the
//     request. Only Schedule and OrderCosts can raise it, and only when a
//     caller passes a matrix or cost list that does not match its stops.
//
// testdata/python_journey*.json is rendered by
// scripts/render_domain_w7_goldens.py from the real module inside the parity
// API image, and oracle_test.go replays every case.
package journey

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxStops is MAX_STOPS and MaxEvaluations is MAX_EVALUATIONS. Neither is read
// by this module; MAX_STOPS gates the preview and MAX_EVALUATIONS bounds
// SuggestOrder's search.
const (
	MaxStops       = 50
	MaxEvaluations = 2000
)

// unreachableCost is the 10**9 SuggestOrder scores a missing road at. It is
// the same number app/journey/routing.py refuses a cost above, so a real road
// can never score as badly as no road at all.
const unreachableCost = 1_000_000_000

// minutesPerDay is the 1440 every overflow test compares against.
const minutesPerDay = 1440

// Cost is one directed road cost: `tuple[int, int]` of travel seconds and
// distance metres. A nil *Cost is Python's None, the missing road.
type Cost struct {
	Seconds int64
	Meters  int64
}

// Matrix is Matrix: a square of directed costs, matrix[from][to].
type Matrix [][]*Cost

// ValueError is ValueError.
type ValueError struct {
	Message string
}

func (e *ValueError) Error() string { return e.Message }

// IndexError is IndexError("list index out of range").
type IndexError struct{}

func (e *IndexError) Error() string { return "list index out of range" }

// Issue is what issue() builds: a code, the stop it is about (nil for an issue
// about the whole day), and the sentence a person reads.
type Issue struct {
	Code    string
	StopID  *string
	Message string
}

// NewIssue is issue(code, message, stop_id). The argument order is Python's.
func NewIssue(code, message string, stopID *string) Issue {
	return Issue{Code: code, StopID: stopID, Message: message}
}

// Stop is the part of a draft stop these functions read. Every field is a key
// _itinerary_draft always writes, so the `.get` defaults of the Python
// (`time_locked` True, `checked_in` False) are folded into the caller.
type Stop struct {
	ID string
	// At is the "HH:MM" the client sent, read by Minute rather than trusted.
	At string
	// DurationMinutes is `duration_minutes`; nil is None, the dwell nobody
	// has confirmed yet.
	DurationMinutes *int64
	TimeLocked      bool
	CheckedIn       bool
}

// Settings is the part of a day's configuration these functions read.
type Settings struct {
	StartAt       string
	ReturnToStart bool
	// EndStopID is `end_stop_id`; nil is None. SuggestOrder compares it with
	// a stop id, so None can never equal one.
	EndStopID *string
}

// Row is one line of Schedule's answer: when the visit is entered in the day,
// when the traveller arrives, when they leave, and how long they wait.
type Row struct {
	ID string
	// At, ArrivalAt and DepartureAt are clock(): nil where the minute is
	// unknown or outside the day.
	At          *string
	ArrivalAt   *string
	DepartureAt *string
	WaitMinutes int64
}

// Plan is schedule()'s answer.
type Plan struct {
	Stops    []Row
	Feasible bool
	Issues   []Issue
}

// Minute is minute(): "HH:MM" as minutes since midnight. The split is Python's
// str.split(":"), which needs exactly two parts, and each part is read by
// Python's int(), which accepts surrounding whitespace, a sign, underscores
// between digits and any Unicode decimal digit -- so "０７:３０" is 450 and
// " +7:_3_0" is 450 too. The range check is on the parts, not on the total.
func Minute(value string) (int64, error) {
	parts := strings.Split(value, ":")
	// `hour, minutes = map(...)` pulls three items from a lazy map: the two it
	// binds and one more to prove the iterator is spent. int() runs on each as
	// it is pulled, so a bad third part is an int() error rather than "too
	// many values", and a part past the third is never read at all.
	var pulled []int64
	for i := 0; i < len(parts) && i < 3; i++ {
		number, err := pyInt(parts[i])
		if err != nil {
			return 0, err
		}
		pulled = append(pulled, number)
	}
	if len(pulled) < 2 {
		return 0, &ValueError{Message: "not enough values to unpack (expected 2, got " + strconv.Itoa(len(pulled)) + ")"}
	}
	if len(parts) > 2 {
		return 0, &ValueError{Message: "too many values to unpack (expected 2)"}
	}
	hour, minutes := pulled[0], pulled[1]
	if hour < 0 || hour > 23 || minutes < 0 || minutes > 59 {
		return 0, &ValueError{Message: "invalid_time"}
	}
	return hour*60 + minutes, nil
}

// Clock is clock(): the minute written back as "HH:MM", or nil (None) for a
// minute outside the day being planned.
func Clock(value int64) *string {
	if value < 0 || value >= minutesPerDay {
		return nil
	}
	text := twoDigits(value/60) + ":" + twoDigits(value%60)
	return &text
}

// twoDigits is Python's "{:02}" for a value already known to be in 0..59.
func twoDigits(value int64) string {
	if value < 10 {
		return "0" + strconv.FormatInt(value, 10)
	}
	return strconv.FormatInt(value, 10)
}

// ceilMinutes is ceil(seconds / 60). Python divides with true division, so the
// quotient is a float before ceil sees it; this reproduces that, rounding
// included, rather than computing the exact ceiling.
func ceilMinutes(seconds int64) int64 {
	return int64(math.Ceil(float64(seconds) / 60))
}

// Schedule is schedule(): evaluate every appointment of one ordering, and
// never infer a dwell time nobody confirmed.
//
// costs holds the road into each stop after the first, and, when the day
// returns to its start, the road home last. A nil cell is a road the provider
// could not find.
func Schedule(stops []Stop, costs []*Cost, settings Settings) (Plan, error) {
	issues := []Issue{}
	rows := make([]Row, 0, len(stops))
	start, err := Minute(settings.StartAt)
	if err != nil {
		return Plan{}, err
	}
	// now is Python's `now: int | None`: nil once the schedule has lost track
	// of the time of day and cannot make it up.
	now := &start
	for index := range stops {
		stop := stops[index]
		id := stop.ID
		if index > 0 {
			cost, ok := at(costs, index-1)
			if !ok {
				return Plan{}, &IndexError{}
			}
			switch {
			case cost == nil:
				issues = append(issues, NewIssue("unreachable", "Không tìm được đường đến điểm này.", &id))
				now = nil
			case now != nil:
				moved := *now + ceilMinutes(cost.Seconds)
				now = &moved
			}
		}
		arrival := now
		locked := stop.TimeLocked || stop.CheckedIn
		expected, err := Minute(stop.At)
		if err != nil {
			return Plan{}, err
		}
		var wait int64
		if now != nil && locked {
			wait = max(0, expected-*now)
		}
		if locked && now != nil {
			if *now > expected {
				issues = append(issues, NewIssue("late_fixed_stop", "Không kịp giờ đã ghim.", &id))
			}
			held := max(*now, expected)
			now = &held
		}
		visit := now
		if stop.DurationMinutes == nil {
			issues = append(issues, NewIssue("missing_duration", "Xác nhận thời lượng ở lại.", &id))
			now = nil
		} else if now != nil {
			stayed := *now + *stop.DurationMinutes
			now = &stayed
		}
		if overflows(arrival) || overflows(visit) || overflows(now) {
			issues = append(issues, NewIssue("day_overflow", "Chặng này vượt qua ngày đang lập.", &id))
		}
		rows = append(rows, Row{
			ID:          stop.ID,
			At:          clockOf(visit),
			ArrivalAt:   clockOf(arrival),
			DepartureAt: clockOf(now),
			WaitMinutes: wait,
		})
	}
	if settings.ReturnToStart && len(stops) > 1 {
		if len(costs) == 0 {
			return Plan{}, &IndexError{}
		}
		cost := costs[len(costs)-1]
		switch {
		case cost == nil:
			issues = append(issues, NewIssue("unreachable_return", "Không tìm được đường quay về.", nil))
		case now != nil && *now+ceilMinutes(cost.Seconds) >= minutesPerDay:
			issues = append(issues, NewIssue("day_overflow", "Đường quay về vượt qua ngày đang lập.", nil))
		}
	}
	return Plan{Stops: rows, Feasible: len(issues) == 0, Issues: issues}, nil
}

// overflows is `value is not None and value >= 1440`.
func overflows(value *int64) bool { return value != nil && *value >= minutesPerDay }

// clockOf is `clock(v) if v is not None else None`, which is clock(v): the
// guard only saves the call.
func clockOf(value *int64) *string {
	if value == nil {
		return nil
	}
	return Clock(*value)
}

// OrderCosts is order_costs(): the cost of each road an ordering drives, and
// the road home last when the day returns to its start.
func OrderCosts(order []int, matrix Matrix, returning bool) ([]*Cost, error) {
	pairs := make([][2]int, 0, len(order))
	for i := 0; i+1 < len(order); i++ {
		pairs = append(pairs, [2]int{order[i], order[i+1]})
	}
	if returning && len(order) > 1 {
		pairs = append(pairs, [2]int{order[len(order)-1], order[0]})
	}
	out := make([]*Cost, 0, len(pairs))
	for _, pair := range pairs {
		row, ok := at(matrix, pair[0])
		if !ok {
			return nil, &IndexError{}
		}
		cost, ok := at(row, pair[1])
		if !ok {
			return nil, &IndexError{}
		}
		out = append(out, cost)
	}
	return out, nil
}

// at is Python's list[index], which counts a negative index back from the end
// and raises IndexError past either edge.
func at[T any](items []T, index int) (T, bool) {
	var zero T
	if index < 0 {
		index += len(items)
	}
	if index < 0 || index >= len(items) {
		return zero, false
	}
	return items[index], true
}

// score is the 4-tuple suggest_order's inner score() returns, compared
// lexicographically: how many things are wrong, then travel time, then
// distance, then how far the ordering moves the client's own arrangement.
type score struct {
	issues    int
	seconds   int64
	meters    int64
	displaced int
}

func (s score) less(other score) bool {
	switch {
	case s.issues != other.issues:
		return s.issues < other.issues
	case s.seconds != other.seconds:
		return s.seconds < other.seconds
	case s.meters != other.meters:
		return s.meters < other.meters
	}
	return s.displaced < other.displaced
}

// SuggestOrder is suggest_order(): keep the fixed slots and endpoints where
// they are, search the directed costs around them, and never claim the answer
// is optimal. The returned ordering indexes stops.
func SuggestOrder(stops []Stop, matrix Matrix, settings Settings) ([]int, error) {
	current := make([]int, len(stops))
	for i := range current {
		current[i] = i
	}
	if len(stops) < 3 {
		return current, nil
	}
	returning := settings.ReturnToStart
	var movable []int
	for i, s := range stops {
		if i == 0 || s.TimeLocked || s.CheckedIn {
			continue
		}
		if settings.EndStopID != nil && s.ID == *settings.EndStopID {
			continue
		}
		movable = append(movable, i)
	}
	evaluations := 0
	scoreOf := func(order []int) (score, error) {
		evaluations++
		costs, err := OrderCosts(order, matrix, returning)
		if err != nil {
			return score{}, err
		}
		ordered := make([]Stop, len(order))
		for i, index := range order {
			stop, ok := at(stops, index)
			if !ok {
				return score{}, &IndexError{}
			}
			ordered[i] = stop
		}
		result, err := Schedule(ordered, costs, settings)
		if err != nil {
			return score{}, err
		}
		// An infeasible solution is never preferred merely because it is
		// shorter, so the issue count is the first thing compared.
		out := score{issues: len(result.Issues)}
		for _, cost := range costs {
			if cost == nil {
				out.seconds += unreachableCost
				out.meters += unreachableCost
				continue
			}
			out.seconds += cost.Seconds
			out.meters += cost.Meters
		}
		// zip(order, current, strict=True): the two are always the same
		// length here, since every candidate is a permutation of current.
		for i := range order {
			if order[i] != current[i] {
				out.displaced++
			}
		}
		return out, nil
	}

	best := append([]int(nil), current...)
	bestScore, err := scoreOf(best)
	if err != nil {
		return nil, err
	}
	greedy := append([]int(nil), current...)
	remaining := append([]int(nil), movable...)
	for _, slot := range movable {
		previous := greedy[slot-1]
		pick, where, err := nearest(matrix, previous, remaining)
		if err != nil {
			return nil, err
		}
		greedy[slot] = pick
		remaining = append(remaining[:where], remaining[where+1:]...)
	}
	candidateScore, err := scoreOf(greedy)
	if err != nil {
		return nil, err
	}
	if candidateScore.less(bestScore) {
		best, bestScore = greedy, candidateScore
	}
	for round := 0; round < 6; round++ {
		improved := false
		for _, a := range movable {
			for _, b := range movable {
				if a >= b {
					continue
				}
				swapped := append([]int(nil), best...)
				swapped[a], swapped[b] = swapped[b], swapped[a]
				// Move within flexible slots, without displacing fixed
				// appointments: the values of the movable slots between a
				// and b rotate one place left.
				relocated := append([]int(nil), best...)
				var slots []int
				for _, i := range movable {
					if a <= i && i <= b {
						slots = append(slots, i)
					}
				}
				values := make([]int, len(slots))
				for i, slot := range slots {
					values[i] = relocated[slot]
				}
				for i, slot := range slots {
					relocated[slot] = values[(i+1)%len(values)]
				}
				for _, candidate := range [][]int{swapped, relocated} {
					if evaluations >= MaxEvaluations {
						return best, nil
					}
					candidateScore, err := scoreOf(candidate)
					if err != nil {
						return nil, err
					}
					if candidateScore.less(bestScore) {
						best, bestScore = candidate, candidateScore
						improved = true
					}
				}
			}
		}
		if !improved {
			break
		}
	}
	return best, nil
}

// nearest is `min(remaining, key=lambda i: (matrix[previous][i] or (10**9,
// 10**9), i))`: the cheapest road out of previous, ties broken by the stop's
// own index, and a missing road scored as the refusal ceiling. It returns the
// chosen stop and where it sat in remaining.
//
// `matrix[previous][i] or ...` is a truth test on the tuple, and a cost tuple
// is always two items long, so only None ever falls through to the ceiling --
// a road of zero seconds and zero metres is kept as itself.
func nearest(matrix Matrix, previous int, remaining []int) (pick, where int, err error) {
	if len(remaining) == 0 {
		// min() of an empty set. suggest_order cannot reach this: it draws
		// exactly one stop per movable slot.
		return 0, 0, &ValueError{Message: "min() iterable argument is empty"}
	}
	row, ok := at(matrix, previous)
	if !ok {
		return 0, 0, &IndexError{}
	}
	key := func(i int) (int64, int64, error) {
		cost, ok := at(row, i)
		if !ok {
			return 0, 0, &IndexError{}
		}
		if cost != nil {
			return cost.Seconds, cost.Meters, nil
		}
		return unreachableCost, unreachableCost, nil
	}
	bestSeconds, bestMeters, err := key(remaining[0])
	if err != nil {
		return 0, 0, err
	}
	pick, where = remaining[0], 0
	for index := 1; index < len(remaining); index++ {
		i := remaining[index]
		seconds, meters, err := key(i)
		if err != nil {
			return 0, 0, err
		}
		better := seconds < bestSeconds ||
			(seconds == bestSeconds && meters < bestMeters) ||
			(seconds == bestSeconds && meters == bestMeters && i < pick)
		if better {
			bestSeconds, bestMeters, pick, where = seconds, meters, i, index
		}
	}
	return pick, where, nil
}

// PyInt is int(str), exported for the service helper `_minute_of_day`, which
// reads a wall-clock time with the same parser and no range check.
func PyInt(text string) (int64, error) { return pyInt(text) }

// pyInt is int(str): optional Python whitespace on both sides, an optional
// sign, then decimal digits which may be separated by single underscores. A
// digit is any Unicode Nd character, and an empty or malformed string is a
// ValueError carrying CPython's own message.
func pyInt(text string) (int64, error) {
	fail := func() (int64, error) {
		return 0, &ValueError{Message: "invalid literal for int() with base 10: " + pyRepr(text)}
	}
	body := strings.TrimFunc(text, isPySpace)
	if strings.ContainsRune(body, 0) {
		return fail()
	}
	negative := false
	if body != "" && (body[0] == '+' || body[0] == '-') {
		negative = body[0] == '-'
		body = body[1:]
	}
	if body == "" {
		return fail()
	}
	var value int64
	previousWasDigit := false
	for i, r := range body {
		if r == '_' {
			if !previousWasDigit || i+utf8.RuneLen(r) >= len(body) {
				return fail()
			}
			previousWasDigit = false
			continue
		}
		digit := decimalValue(r)
		if digit < 0 {
			return fail()
		}
		// A value this large is far past any clock; saturating keeps the
		// range check below refusing it, exactly as the unbounded Python int
		// would.
		if value > (1<<62)/10 {
			value = 1 << 62
		} else {
			value = value*10 + int64(digit)
		}
		previousWasDigit = true
	}
	if !previousWasDigit {
		return fail()
	}
	if negative {
		value = -value
	}
	return value, nil
}

// pyRepr is repr(str) for the message of a ValueError, which is only ever
// compared, never parsed. It writes Python's preferred quoting: single quotes
// unless the text holds one and no double quote.
func pyRepr(text string) string {
	quote := byte('\'')
	if strings.ContainsRune(text, '\'') && !strings.ContainsRune(text, '"') {
		quote = '"'
	}
	var out strings.Builder
	out.WriteByte(quote)
	for _, r := range text {
		switch {
		case r == rune(quote) || r == '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case r == '\n':
			out.WriteString("\\n")
		case r == '\r':
			out.WriteString("\\r")
		case r == '\t':
			out.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			out.WriteString("\\x")
			const hex = "0123456789abcdef"
			out.WriteByte(hex[(r>>4)&0xf])
			out.WriteByte(hex[r&0xf])
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte(quote)
	return out.String()
}

// isPySpace is str.isspace() for one code point, the set int() strips.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// decimalValue is Py_UNICODE_TODECIMAL: what a Unicode Nd character is worth,
// or -1. Nd characters come in runs of ten from zero.
func decimalValue(r rune) int {
	for _, span := range unicode.Nd.R16 {
		if lo, hi := rune(span.Lo), rune(span.Hi); r >= lo && r <= hi {
			return int(r-lo) % 10
		}
	}
	for _, span := range unicode.Nd.R32 {
		if lo, hi := rune(span.Lo), rune(span.Hi); r >= lo && r <= hi {
			return int(r-lo) % 10
		}
	}
	return -1
}
