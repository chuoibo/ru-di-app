package normalize

import (
	"strings"
	"testing"
)

func run(t *testing.T, named map[string]string, texts ...string) []string {
	t.Helper()
	b := NewBinder()
	for literal, name := range named {
		if err := b.Name(literal, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, text := range texts {
		if err := b.Observe(text); err != nil {
			t.Fatal(err)
		}
	}
	out := make([]string, len(texts))
	for i, text := range texts {
		out[i] = b.Apply(text)
	}
	return out
}

func TestSameStructureDifferentValuesNormaliseEqual(t *testing.T) {
	python := run(t, nil,
		`{"id":"3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b","created_at":"2026-09-14T10:00:00.123456Z"}`,
		`{"context_id":"3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b","member_id":"cafebabe-dead-4bee-8f00-abcdefabcdef","joined_at":"2026-09-14T10:00:01Z"}`,
	)
	golang := run(t, nil,
		`{"id":"aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee","created_at":"2026-09-14T11:30:00.654321Z"}`,
		`{"context_id":"aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee","member_id":"facefeed-beef-4a11-b222-fedcbafedcba","joined_at":"2026-09-14T11:30:05Z"}`,
	)
	for i := range python {
		if python[i] != golang[i] {
			t.Fatalf("step %d:\npython %s\ngo     %s", i, python[i], golang[i])
		}
	}
	if !strings.Contains(python[1], `"context_id":"<uuid#1>"`) || !strings.Contains(python[1], `"member_id":"<uuid#2>"`) {
		t.Fatalf("numbering by first appearance: %s", python[1])
	}
}

func TestFormatDifferencesSurviveNormalisation(t *testing.T) {
	cases := map[string][2]string{
		"Z vs +00:00":          {`"2026-09-14T10:00:00.123456Z"`, `"2026-09-14T10:00:00.123456+00:00"`},
		"micro vs milli":       {`"2026-09-14T10:00:00.123456Z"`, `"2026-09-14T10:00:00.123Z"`},
		"no fraction vs zeros": {`"2026-09-14T10:00:00Z"`, `"2026-09-14T10:00:00.000000Z"`},
		"uppercase uuid":       {`"3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b"`, `"3F2B8C1E-9A4D-4E2F-8B1A-7C6D5E4F3A2B"`},
		"float point":          {`{"score":1.0}`, `{"score":1}`},
		"key order":            {`{"a":1,"b":2}`, `{"b":2,"a":1}`},
		"space separator":      {`"2026-09-14T10:00:00Z"`, `"2026-09-14 10:00:00Z"`}, // repo-guard: allow=long-number reason=timestamp-fixture-not-an-account
		"escaped vs raw utf8":  {`"Đà Lạt"`, `"\u0110\u00e0 L\u1ea1t"`},
		"reused id vs two ids": {`"3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b","3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b"`, `"aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee","cafebabe-dead-4bee-8f00-abcdefabcdef"`},
	}
	for name, pair := range cases {
		t.Run(name, func(t *testing.T) {
			python := run(t, nil, pair[0])[0]
			golang := run(t, nil, pair[1])[0]
			if python == golang {
				t.Fatalf("difference hidden: both normalised to %s", python)
			}
		})
	}
}

func TestTimestampRanksKeepOrderAndEquality(t *testing.T) {
	out := run(t, nil,
		`"2026-09-14T10:00:05Z"`,
		`"2026-09-14T10:00:01Z"`,
		`"2026-09-14T10:00:01.000000+00:00"`,
	)
	if out[0] != `"<ts#2|f0|Z>"` || out[1] != `"<ts#1|f0|Z>"` || out[2] != `"<ts#1|f6|+00:00>"` {
		t.Fatalf("ranks/shapes = %q", out)
	}
}

func TestClockSourceChangeCollapsesRanks(t *testing.T) {
	// Python stamps two columns at different moments; a port that stamps both
	// with one clock read must not normalise to the same text.
	python := run(t, nil, `{"created_at":"2026-09-14T10:00:00.100000Z","accepted_at":"2026-09-14T10:00:00.200000Z"}`)[0]
	golang := run(t, nil, `{"created_at":"2026-09-14T10:00:00.100000Z","accepted_at":"2026-09-14T10:00:00.100000Z"}`)[0]
	if python == golang {
		t.Fatalf("clock collapse hidden: %s", python)
	}
}

func TestNamedLiteralsAndTokens(t *testing.T) {
	persona := "0b6c1d2e-3f40-4a5b-8c6d-7e8f90a1b2c3"
	token := "Zm9vYmFyYmF6cXV4cXV1eHF1dXhxdXV4cXV1eHF1dXg"
	out := run(t, map[string]string{persona: "persona:owner", token: "token:invite"},
		`{"created_by_id":"`+persona+`","link":"/g/`+token+`","other":"3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b"}`,
	)[0]
	want := `{"created_by_id":"<persona:owner>","link":"/g/<token:invite>","other":"<uuid#1>"}`
	if out != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
}

func TestUnobservedValuesStayLiteral(t *testing.T) {
	b := NewBinder()
	_ = b.Observe(`"3f2b8c1e-9a4d-4e2f-8b1a-7c6d5e4f3a2b"`)
	got := b.Apply(`"aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee"`)
	if got != `"aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee"` {
		t.Fatalf("unobserved id was numbered: %s", got)
	}
	if err := b.Observe("late"); err == nil {
		t.Fatal("Observe after Apply accepted")
	}
}

func TestNameRejectsRebinding(t *testing.T) {
	b := NewBinder()
	if err := b.Name("abc", "token:a"); err != nil {
		t.Fatal(err)
	}
	if err := b.Name("abc", "token:b"); err == nil {
		t.Fatal("rebinding a literal accepted")
	}
	if err := b.Name("xyz", "token:a"); err == nil {
		t.Fatal("two literals under one name accepted")
	}
}

// A storage key is random per upload, so two stacks share only where it is
// reused. Reuse, case and length must still show.
func TestStorageKeysAreBoundByFirstAppearanceNotByValue(t *testing.T) {
	key := func(c string) string { return strings.Repeat(c, 32) }
	rows := func(first, second string) []string {
		return []string{
			`{"storage_key":"` + first + `"}`,
			`{"storage_key":"` + second + `","replaces":"` + first + `"}`,
		}
	}
	apply := func(texts []string) []string {
		b := NewBinder()
		for _, text := range texts {
			if err := b.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
		out := make([]string, len(texts))
		for i, text := range texts {
			out[i] = b.Apply(text)
		}
		return out
	}
	ref := apply(rows(key("a"), key("b")))
	cand := apply(rows(key("c"), key("d")))
	if strings.Join(ref, "\n") != strings.Join(cand, "\n") {
		t.Fatalf("same key pattern differs:\n%v\n%v", ref, cand)
	}
	if ref[1] != `{"storage_key":"<hex32#2>","replaces":"<hex32#1>"}` {
		t.Fatalf("not bound: %v", ref)
	}

	// The candidate reused one key where the reference wrote two.
	if reused := apply(rows(key("c"), key("c"))); reused[1] == ref[1] {
		t.Fatalf("key reuse hidden: %v", reused)
	}

	for _, literal := range []string{strings.Repeat("a", 31), strings.Repeat("a", 33), strings.Repeat("A", 32), strings.Repeat("a", 63)} {
		if got := apply([]string{literal})[0]; got != literal {
			t.Fatalf("%d-character run %q bound as %q", len(literal), literal[:4], got)
		}
	}
	// Two keys written side by side are one 64-character run: a digest.
	if got := apply([]string{key("a") + key("b")})[0]; got != "<digest#1>" {
		t.Fatalf("64-character run bound as %q", got)
	}

	masks := map[string]string{
		key("a"):                "<hex32>",
		strings.Repeat("a", 33): strings.Repeat("a", 33),
		strings.Repeat("a", 64): "<digest>",
		strings.Repeat("a", 65): "<digest>",
	}
	for literal, want := range masks {
		if got := Mask(literal); got != want {
			t.Errorf("Mask(%d-character run) = %q, want %q", len(literal), got, want)
		}
	}
}

func TestShape(t *testing.T) {
	cases := map[string]string{
		"2026-09-14T10:00:00Z":             "f0|Z",
		"2026-09-14T10:00:00.123456Z":      "f6|Z",
		"2026-09-14T10:00:00.123+07:00":    "f3|+07:00",
		"2026-09-14 10:00:00":              "space|f0|naive", // repo-guard: allow=long-number reason=timestamp-fixture-not-an-account
		"2026-09-14T10:00:00.123456789Z":   "f9|Z",
		"2026-09-14T10:00:00.000001-05:00": "f6|-05:00", // repo-guard: allow=long-number reason=timestamp-fixture-not-an-account
	}
	for literal, want := range cases {
		if got := Shape(literal); got != want {
			t.Errorf("Shape(%q) = %q, want %q", literal, got, want)
		}
	}
}

func TestDigestsAreBoundByFirstAppearanceNotByValue(t *testing.T) {
	digest := func(c string) string { return strings.Repeat(c, 64) }
	transcript := func(first, second string) []string {
		return []string{
			`{"fingerprint":"` + first + `"}`,
			`{"fingerprint":"` + first + `","token_digest":"\\x` + second + `"}`,
		}
	}
	apply := func(texts []string) []string {
		b := NewBinder()
		for _, text := range texts {
			if err := b.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
		out := make([]string, len(texts))
		for i, text := range texts {
			out[i] = b.Apply(text)
		}
		return out
	}
	ref := apply(transcript(digest("a"), digest("b")))
	cand := apply(transcript(digest("c"), digest("d")))
	if strings.Join(ref, "\n") != strings.Join(cand, "\n") {
		t.Fatalf("same reuse pattern differs:\n%v\n%v", ref, cand)
	}
	if !strings.Contains(ref[1], `"<digest#1>"`) || !strings.Contains(ref[1], `\\x<digest#2>`) {
		t.Fatalf("not bound: %v", ref)
	}

	// A different reuse pattern still shows: the candidate stored a new
	// digest where the reference reused the first one.
	broken := apply([]string{`{"fingerprint":"` + digest("c") + `"}`, `{"fingerprint":"` + digest("e") + `"}`})
	if ref[0] != broken[0] || strings.Contains(broken[1], "<digest#1>") {
		t.Fatalf("reuse pattern difference hidden: %v vs %v", ref, broken)
	}

	for _, literal := range []string{strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("A", 64)} {
		if got := apply([]string{literal})[0]; got != literal {
			t.Fatalf("%d-character run %q bound as %q", len(literal), literal[:4], got)
		}
	}
}

func TestTieInstantsSinceSharesOneRank(t *testing.T) {
	before, first, second := "2026-09-16T10:00:00.000001Z", "2026-09-16T10:00:00.000003Z", "2026-09-16T10:00:00.000002Z"
	ranked := func(tie bool) string {
		b := NewBinder()
		if err := b.Observe(before); err != nil {
			t.Fatal(err)
		}
		mark := b.InstantMark()
		if err := b.Observe(first + " " + second); err != nil {
			t.Fatal(err)
		}
		if tie {
			if err := b.TieInstantsSince(mark); err != nil {
				t.Fatal(err)
			}
		}
		return b.Apply(before + " " + first + " " + second)
	}
	if got, want := ranked(false), "<ts#1|f6|Z> <ts#3|f6|Z> <ts#2|f6|Z>"; got != want {
		t.Fatalf("untied = %q, want %q", got, want)
	}
	if got, want := ranked(true), "<ts#1|f6|Z> <ts#2|f6|Z> <ts#2|f6|Z>"; got != want {
		t.Fatalf("tied = %q, want %q", got, want)
	}
	b := NewBinder()
	_ = b.Apply("")
	if err := b.TieInstantsSince(0); err == nil {
		t.Fatal("tying after Apply succeeded")
	}
}
