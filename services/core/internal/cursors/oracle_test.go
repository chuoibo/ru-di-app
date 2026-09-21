package cursors

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.api.cursors in the parity API image. Every decode case is
// replayed through DecodeCursor, and every accepted one is re-encoded and
// turned into an instant; every encode case goes through EncodeDateTime and,
// when its offset is whole seconds, through EncodeCursor.

type goldenFile struct {
	Mode      string          `json:"mode"`
	Constants json.RawMessage `json:"constants"`
	Fuzz      *struct {
		Shard  int `json:"shard"`
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Fn     string         `json:"fn"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
}

func loadGoldens(t *testing.T) []goldenFile {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	sort.Strings(paths)
	files := make([]goldenFile, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		var file goldenFile
		if err := decoder.Decode(&file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		files = append(files, file)
	}
	return files
}

// text reads a str of the oracle encoding: plain, or "$sp:" groups of six
// code points joined by "|".
func text(raw any) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("want a str, got %T", raw)
	}
	if !strings.HasPrefix(s, "$") {
		return s, nil
	}
	body, found := strings.CutPrefix(s, "$sp:")
	if !found {
		return "", fmt.Errorf("not a str: %q", s)
	}
	var out strings.Builder
	count := 0
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRuneInString(body[i:])
		i += size
		if count == 6 {
			if r != '|' {
				return "", fmt.Errorf("bad group separator in %q", s)
			}
			count = 0
			continue
		}
		out.WriteRune(r)
		count++
	}
	return out.String(), nil
}

func integer(raw any) (int64, error) {
	switch v := raw.(type) {
	case json.Number:
		return strconv.ParseInt(string(v), 10, 64)
	case string:
		if body, found := strings.CutPrefix(v, "$i:"); found {
			return strconv.ParseInt(body, 0, 64)
		}
	}
	return 0, fmt.Errorf("not an int: %#v", raw)
}

// dateTime reads "$dt:<wall>|<offset>" into a DateTime, the offset being
// ±HH:MM, ±HH:MM:SS or ±HH:MM:SS.ffffff.
func dateTime(raw any) (DateTime, error) {
	s, _ := raw.(string)
	body, found := strings.CutPrefix(s, "$dt:")
	wall, offset, cut := strings.Cut(body, "|")
	if !found || !cut || len(offset) < 6 {
		return DateTime{}, fmt.Errorf("not a datetime: %v", raw)
	}
	clock, err := time.Parse("2006-01-02T15:04:05.000000", wall)
	if err != nil {
		return DateTime{}, err
	}
	whole, fraction, _ := strings.Cut(offset[1:], ".")
	var micros int64
	for i, part := range strings.Split(whole, ":") {
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || i > 2 {
			return DateTime{}, fmt.Errorf("bad offset %q", offset)
		}
		micros += n * []int64{3600, 60, 1}[i] * 1_000_000
	}
	if fraction != "" {
		n, err := strconv.ParseInt(fraction, 10, 64)
		if err != nil || len(fraction) != 6 {
			return DateTime{}, fmt.Errorf("bad offset %q", offset)
		}
		micros += n
	}
	if offset[0] == '-' {
		micros = -micros
	}
	return DateTime{
		Year: clock.Year(), Month: int(clock.Month()), Day: clock.Day(),
		Hour: clock.Hour(), Minute: clock.Minute(), Second: clock.Second(),
		Microsecond: clock.Nanosecond() / 1000, OffsetMicros: micros,
	}, nil
}

// pythonDecoded is decode_cursor's result as the oracle shapes it.
type pythonDecoded struct {
	fields    [9]int64
	messageID string
	reencoded string
	utcMicros int64
}

func readDecoded(body any) (pythonDecoded, error) {
	m, ok := body.(map[string]any)
	if !ok || len(m) != 4 {
		return pythonDecoded{}, fmt.Errorf("decoded result %#v", body)
	}
	var out pythonDecoded
	items, ok := m["created_at"].([]any)
	if !ok || len(items) != 9 {
		return out, fmt.Errorf("created_at %#v", m["created_at"])
	}
	for i, item := range items {
		n, err := integer(item)
		if err != nil {
			return out, err
		}
		out.fields[i] = n
	}
	var err error
	if out.messageID, err = text(m["message_id"]); err != nil {
		return out, err
	}
	if out.reencoded, err = text(m["reencoded"]); err != nil {
		return out, err
	}
	out.utcMicros, err = integer(m["utc_microseconds"])
	return out, err
}

func floorDiv(a, b int64) (int64, int64) {
	q, r := a/b, a%b
	if r < 0 {
		q, r = q-1, r+b
	}
	return q, r
}

func goDecoded(position Position) pythonDecoded {
	d := position.CreatedAt
	seconds, micros := floorDiv(d.OffsetMicros, 1_000_000)
	return pythonDecoded{
		fields: [9]int64{int64(d.Year), int64(d.Month), int64(d.Day), int64(d.Hour), int64(d.Minute),
			int64(d.Second), int64(d.Microsecond), seconds, micros},
		messageID: position.MessageID,
		reencoded: EncodeDateTime(d, position.MessageID),
		utcMicros: d.Time().UnixMicro(),
	}
}

type tally struct {
	cases, accepted, refused, mismatches int
	fuzz                                 map[string]int
}

func (tl *tally) mismatch(t *testing.T, format string, args ...any) {
	t.Helper()
	tl.mismatches++
	if tl.mismatches <= 20 {
		t.Errorf(format, args...)
	}
}

func replayDecode(t *testing.T, tl *tally, c goldenCase) {
	raw, err := text(c.Args["raw"])
	if err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	position, goErr := DecodeCursor(raw)
	if raised, ok := c.Result["raised"].(map[string]any); ok {
		message, err := text(raised["message"])
		if err != nil {
			t.Fatal(err)
		}
		tl.refused++
		var refusal *CursorError
		if !errors.As(goErr, &refusal) || raised["type"] != "CursorError" || message != goErr.Error() ||
			raised["code"] != nil {
			tl.mismatch(t, "%s decode_cursor(%q): Python raised %v %q, Go %+v %v", c.Name, raw, raised["type"], message, position, goErr)
		}
		return
	}
	want, err := readDecoded(c.Result["ok"])
	if err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	tl.accepted++
	if goErr != nil {
		tl.mismatch(t, "%s decode_cursor(%q): Python %+v, Go %v", c.Name, raw, want, goErr)
		return
	}
	if got := goDecoded(position); got != want {
		tl.mismatch(t, "%s decode_cursor(%q):\n  Python %+v\n  Go     %+v", c.Name, raw, want, got)
	}
}

func replayEncode(t *testing.T, tl *tally, c goldenCase) {
	createdAt, err := dateTime(c.Args["created_at"])
	if err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	messageID, err := text(c.Args["message_id"])
	if err != nil {
		t.Fatal(err)
	}
	want, err := text(c.Result["ok"])
	if err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	tl.accepted++
	if got := EncodeDateTime(createdAt, messageID); got != want {
		tl.mismatch(t, "%s EncodeDateTime(%+v): Python %q, Go %q", c.Name, createdAt, want, got)
	}
	if createdAt.OffsetMicros%1_000_000 == 0 {
		if got := EncodeCursor(createdAt.Time(), messageID); got != want {
			tl.mismatch(t, "%s EncodeCursor(%v): Python %q, Go %q", c.Name, createdAt.Time(), want, got)
		}
	}
}

func TestCursorsMatchPython(t *testing.T) {
	tl := &tally{fuzz: map[string]int{}}
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			tl.cases++
			if file.Fuzz != nil {
				tl.fuzz[c.Fn]++
			}
			switch c.Fn {
			case "decode_cursor":
				replayDecode(t, tl, c)
			case "encode_cursor":
				replayEncode(t, tl, c)
			default:
				t.Fatalf("unknown function %q", c.Fn)
			}
		}
	}
	if tl.mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", tl.mismatches, tl.cases)
	}
	if tl.fuzz["decode_cursor"] < 12000 || tl.fuzz["encode_cursor"] < 900 || tl.accepted < 2000 || tl.refused < 2000 {
		t.Errorf("corpus too thin: %+v", tl)
	}
	t.Logf("%d Python cases agree, 0 mismatches (%d returned, %d refused)", tl.cases, tl.accepted, tl.refused)
}

func TestFuzzShardsAreComplete(t *testing.T) {
	seen := map[int]bool{}
	declared, shards, rendered := -1, -1, 0
	for _, file := range loadGoldens(t) {
		if file.Fuzz == nil {
			continue
		}
		if declared >= 0 && (declared != file.Fuzz.Total || shards != file.Fuzz.Shards) {
			t.Fatal("fuzz shards disagree on their totals")
		}
		declared, shards = file.Fuzz.Total, file.Fuzz.Shards
		if seen[file.Fuzz.Shard] {
			t.Fatalf("shard %d appears twice", file.Fuzz.Shard)
		}
		seen[file.Fuzz.Shard] = true
		rendered += len(file.Cases)
	}
	if shards < 1 || len(seen) != shards || rendered != declared {
		t.Fatalf("%d of %d shards holding %d of %d fuzz cases", len(seen), shards, rendered, declared)
	}
}

var goNames = map[string]string{
	"CursorError":   "CursorError",
	"decode_cursor": "DecodeCursor",
	"encode_cursor": "EncodeCursor",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		IsSpace [][2]rune `json:"isspace"`
		Decimal [][3]int  `json:"decimal"`
		Names   []string  `json:"names"`
	}
	found := 0
	for _, file := range loadGoldens(t) {
		if file.Constants == nil {
			continue
		}
		found++
		if err := json.Unmarshal(file.Constants, &constants); err != nil {
			t.Fatal(err)
		}
	}
	if found != 1 {
		t.Fatalf("%d files carry constants, want 1", found)
	}
	if len(constants.Names) != len(goNames) || !reflect.DeepEqual(sortedKeys(goNames), constants.Names) {
		t.Errorf("Python public names %v, goNames %v", constants.Names, sortedKeys(goNames))
	}
	if len(constants.IsSpace) == 0 || len(constants.Decimal) < 60 {
		t.Fatalf("tables missing: %d space ranges, %d decimal runs", len(constants.IsSpace), len(constants.Decimal))
	}
	space, digit := 0, 0
	for r := rune(0); r <= utf8.MaxRune; r++ {
		for space < len(constants.IsSpace) && constants.IsSpace[space][1] < r {
			space++
		}
		wantSpace := space < len(constants.IsSpace) && constants.IsSpace[space][0] <= r
		if isPySpace(r) != wantSpace {
			t.Errorf("U+%04X: str.isspace() is %v, Go %v", r, wantSpace, isPySpace(r))
		}
		for digit < len(constants.Decimal) && rune(constants.Decimal[digit][1]) < r {
			digit++
		}
		want := -1
		if digit < len(constants.Decimal) && rune(constants.Decimal[digit][0]) <= r {
			run := constants.Decimal[digit]
			want = (run[2] + int(r) - run[0]) % 10
		}
		if got := decimalValue(r); got != want {
			t.Errorf("U+%04X: unicodedata.decimal is %d, Go %d", r, want, got)
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
