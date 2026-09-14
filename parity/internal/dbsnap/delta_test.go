package dbsnap

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"mobile/parity/internal/normalize"
)

func row(key, text string) Row { return Row{Key: key, Raw: text, Text: text} }

// newUUID returns a random canonical version-4 UUID.
func newUUID(t *testing.T) string {
	t.Helper()
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		t.Fatal(err)
	}
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// keyed builds a relation keyed by its "id" member.
func keyed(t *testing.T, name string, texts ...string) *Relation {
	t.Helper()
	rel := &Relation{Name: name, Kind: "table", PrimaryKey: []string{"id"}}
	for _, text := range texts {
		var fields struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(text), &fields); err != nil || fields.ID == "" {
			t.Fatalf("fixture row without id: %s", text)
		}
		rel.Rows = append(rel.Rows, row(`["`+fields.ID+`"]`, text))
	}
	return rel
}

func keyless(name string, texts ...string) *Relation {
	rel := &Relation{Name: name, Kind: "view"}
	for _, text := range texts {
		rel.Rows = append(rel.Rows, row("", text))
	}
	return rel
}

func snapOf(rels ...*Relation) *Snap { return &Snap{Schema: "public", Relations: rels} }

func texts(rows []Row) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, r.Text)
	}
	return out
}

func instant(minute, second, micro int) string {
	return fmt.Sprintf("2026-09-14T10:%02d:%02d.%06d+00:00", minute, second, micro)
}

func TestDeltaKeyedInsertUpdateDelete(t *testing.T) {
	g1, g2, g3 := newUUID(t), newUUID(t), newUUID(t)
	prev := snapOf(
		keyed(t, "groups", `{"id":"`+g1+`","name":"a"}`, `{"id":"`+g2+`","name":"b"}`),
		keyed(t, "untouched", `{"id":"`+g1+`"}`),
	)
	next := snapOf(
		keyed(t, "groups", `{"id":"`+g3+`","name":"c"}`, `{"id":"`+g1+`","name":"a2"}`),
		keyed(t, "untouched", `{"id":"`+g1+`"}`),
	)
	change := Delta(prev, next)
	if len(change.Changed) != 1 || change.Changed[0].Relation != "groups" || !change.Changed[0].Keyed {
		t.Fatalf("changed relations: %+v", change.Changed)
	}
	rc := change.Changed[0]
	if got := texts(rc.Inserted); !reflect.DeepEqual(got, []string{`{"id":"` + g3 + `","name":"c"}`}) {
		t.Errorf("inserted %v", got)
	}
	if got := texts(rc.Deleted); !reflect.DeepEqual(got, []string{`{"id":"` + g2 + `","name":"b"}`}) {
		t.Errorf("deleted %v", got)
	}
	if len(rc.Updated) != 1 || rc.Updated[0].Before.Text != `{"id":"`+g1+`","name":"a"}` || rc.Updated[0].After.Text != `{"id":"`+g1+`","name":"a2"}` {
		t.Errorf("updated %+v", rc.Updated)
	}
	if !reflect.DeepEqual(change.Relations, []string{"groups", "untouched"}) {
		t.Errorf("relations %v", change.Relations)
	}
	if Delta(next, next).Empty() != true {
		t.Error("a snapshot differs from itself")
	}
}

func TestTextsIgnorePhysicalRowOrder(t *testing.T) {
	var rows []string
	for _, name := range []string{"an", "binh", "chi", "dung", "em", "giang"} {
		rows = append(rows, `{"id":"`+newUUID(t)+`","person":"`+name+`"}`)
	}
	shuffled := []string{rows[3], rows[0], rows[5], rows[1], rows[4], rows[2]}
	tags := []string{`{"label":"x"}`, `{"label":"y"}`, `{"label":"x"}`}

	a := Delta(nil, snapOf(keyed(t, "members", rows...), keyless("tags", tags...))).Texts()
	b := Delta(nil, snapOf(keyed(t, "members", shuffled...), keyless("tags", tags[2], tags[1], tags[0]))).Texts()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("texts depend on physical order:\n%v\n%v", a, b)
	}
}

// stackFixture writes the same two steps a stack would, with its own ids and
// instants. extraColumn makes the second step also change a note.
func stackFixture(t *testing.T, minute int, extraColumn bool) (s0, s1, s2 *Snap) {
	t.Helper()
	group := newUUID(t)
	people := []string{"an", "binh", "chi"}
	ids := map[string]string{}
	var members []string
	for i, person := range people {
		ids[person] = newUUID(t)
		members = append(members, fmt.Sprintf(`{"id":"%s","group_id":"%s","person":"%s","share":%d,"note":null,"updated_at":"%s"}`,
			ids[person], group, person, 1000*(i+1), instant(minute, 1, 120000)))
	}
	groups := fmt.Sprintf(`{"id":"%s","name":"Da Lat","created_at":"%s"}`, group, instant(minute, 0, 500000))

	s0 = snapOf(keyed(t, "groups"), keyed(t, "members"))
	// Physical order differs per stack on purpose.
	reversed := []string{members[2], members[1], members[0]}
	if minute%2 == 0 {
		reversed = members
	}
	s1 = snapOf(keyed(t, "groups", groups), keyed(t, "members", reversed...))

	note := "null"
	if extraColumn {
		note = `"moved"`
	}
	updated := fmt.Sprintf(`{"id":"%s","group_id":"%s","person":"binh","share":2500,"note":%s,"updated_at":"%s"}`,
		ids["binh"], group, note, instant(minute, 9, 0))
	s2 = snapOf(keyed(t, "groups", groups), keyed(t, "members", updated, members[0]))
	return s0, s1, s2
}

func normalisedSteps(t *testing.T, snaps ...*Snap) []*Change {
	t.Helper()
	binder := normalize.NewBinder()
	var deltas []*Change
	for i := 1; i < len(snaps); i++ {
		delta := Delta(snaps[i-1], snaps[i])
		for _, text := range delta.Texts() {
			if err := binder.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
		deltas = append(deltas, delta)
	}
	out := make([]*Change, len(deltas))
	for i, delta := range deltas {
		out[i] = delta.Normalise(binder.Apply)
	}
	return out
}

func TestNormalisedEqualWhenOnlyIdsAndInstantsDiffer(t *testing.T) {
	r0, r1, r2 := stackFixture(t, 3, false)
	c0, c1, c2 := stackFixture(t, 44, false)
	if Delta(r0, r1).Texts()[0] == Delta(c0, c1).Texts()[0] {
		t.Fatal("fixture stacks share ids; the test would prove nothing")
	}
	ref := normalisedSteps(t, r0, r1, r2)
	cand := normalisedSteps(t, c0, c1, c2)
	for step := range ref {
		if diffs := Compare(ref[step], cand[step]); len(diffs) > 0 {
			t.Errorf("step %d differs:\n%v", step+1, diffs)
		}
	}
	joined := strings.Join(ref[0].Texts(), "\n")
	if !strings.Contains(joined, "<uuid#") || !strings.Contains(joined, "<ts#") {
		t.Errorf("normalised texts carry no placeholders:\n%s", joined)
	}
	if got := len(ref[1].Changed[0].Deleted) + len(ref[1].Changed[0].Updated); got != 2 {
		t.Errorf("step 2 should update one member and delete one, got %+v", ref[1].Changed[0])
	}
}

func TestCompareReportsExtraUpdatedColumn(t *testing.T) {
	r0, r1, r2 := stackFixture(t, 3, false)
	c0, c1, c2 := stackFixture(t, 44, true)
	ref := normalisedSteps(t, r0, r1, r2)
	cand := normalisedSteps(t, c0, c1, c2)
	if diffs := Compare(ref[0], cand[0]); len(diffs) > 0 {
		t.Fatalf("step 1 should be equal: %v", diffs)
	}
	diffs := Compare(ref[1], cand[1])
	if len(diffs) != 1 {
		t.Fatalf("want one difference, got %v", diffs)
	}
	d := diffs[0]
	if d.Relation != "members" || d.Kind != KindUpdated || d.Reference != 1 || d.Candidate != 1 {
		t.Errorf("difference %+v", d)
	}
	if len(d.OnlyCandidate) != 1 || !strings.HasPrefix(d.OnlyCandidate[0], `set {"share":2500,"note":"moved",`) {
		t.Errorf("candidate side should lead with the changed columns: %v", d.OnlyCandidate)
	}
	if len(d.OnlyReference) != 1 || strings.Contains(d.OnlyReference[0], "moved") {
		t.Errorf("reference side: %v", d.OnlyReference)
	}
	if !strings.Contains(d.String(), "db members updated: reference 1, candidate 1") {
		t.Errorf("String() = %s", d)
	}
}

func TestKeylessRelationIsMultiset(t *testing.T) {
	prev := snapOf(keyless("tags", `{"label":"x"}`, `{"label":"x"}`, `{"label":"y"}`))
	next := snapOf(keyless("tags", `{"label":"y"}`, `{"label":"x"}`, `{"label":"z"}`, `{"label":"y"}`))
	change := Delta(prev, next)
	if len(change.Changed) != 1 || change.Changed[0].Keyed {
		t.Fatalf("changed %+v", change.Changed)
	}
	rc := change.Changed[0]
	if got := texts(rc.Inserted); !reflect.DeepEqual(got, []string{`{"label":"y"}`, `{"label":"z"}`}) {
		t.Errorf("inserted %v", got)
	}
	if got := texts(rc.Deleted); !reflect.DeepEqual(got, []string{`{"label":"x"}`}) {
		t.Errorf("deleted %v", got)
	}
	same := snapOf(keyless("tags", `{"label":"y"}`, `{"label":"x"}`, `{"label":"x"}`))
	if !Delta(prev, same).Empty() {
		t.Error("reordering a key-less relation looked like a change")
	}
}

func TestDuplicateKeysFallBackToMultiset(t *testing.T) {
	rel := &Relation{Name: "odd", PrimaryKey: []string{"id"}, Rows: []Row{row(`["a"]`, `{"id":"a","v":1}`), row(`["a"]`, `{"id":"a","v":2}`)}}
	change := Delta(nil, snapOf(rel))
	if change.Changed[0].Keyed || len(change.Changed[0].Inserted) != 2 {
		t.Errorf("duplicate keys: %+v", change.Changed[0])
	}
}

func TestCompareCountsMultiplicityAndRelations(t *testing.T) {
	ref := Delta(nil, snapOf(keyless("tags", `{"label":"x"}`, `{"label":"x"}`), keyless("extra")))
	cand := Delta(nil, snapOf(keyless("tags", `{"label":"x"}`)))
	diffs := Compare(ref, cand)
	if len(diffs) != 2 {
		t.Fatalf("want relations and tags differences, got %v", diffs)
	}
	if diffs[0].Kind != KindRelations || !reflect.DeepEqual(diffs[0].OnlyReference, []string{"extra"}) {
		t.Errorf("relations difference %+v", diffs[0])
	}
	if diffs[1].Relation != "tags" || diffs[1].Reference != 2 || diffs[1].Candidate != 1 ||
		!reflect.DeepEqual(diffs[1].OnlyReference, []string{`{"label":"x"}`}) || diffs[1].OnlyCandidate != nil {
		t.Errorf("multiplicity difference %+v", diffs[1])
	}
}

func TestDisplayTruncatesAndCapsLists(t *testing.T) {
	long := `{"v":"` + strings.Repeat("ắ", 300) + `"}`
	got := truncate(long)
	if len(got) > maxTextBytes+32 || !strings.Contains(got, "bytes)") || !strings.HasPrefix(long, strings.SplitN(got, "…(+", 2)[0]) {
		t.Errorf("truncate produced %q", got)
	}
	many := make([]string, 25)
	for i := range many {
		many[i] = fmt.Sprintf(`{"n":%d}`, i)
	}
	if listedRows := listed(many); len(listedRows) != maxListedRows+1 || listedRows[maxListedRows] != "… and 15 more" {
		t.Errorf("listed %v", listedRows)
	}
}

// TestOrderMaskMatchesBinder keeps the ordering masks in step with what
// normalize binds: a value the binder numbers but the mask leaves visible
// would let random values decide the observation order.
func TestOrderMaskMatchesBinder(t *testing.T) {
	spaced := "2026-09-14" + " " + "10:00:05"
	text := `{"a":"` + newUUID(t) + `","b":["` + newUUID(t) + `","` + instant(0, 5, 120000) + `"],"c":"` +
		spaced + `","d":"2026-09-14T10:00:05Z","e":"AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE","f":"2026-09-14"}`
	binder := normalize.NewBinder()
	if err := binder.Observe(text); err != nil {
		t.Fatal(err)
	}
	fromBinder := regexp.MustCompile(`<ts#\d+\|[^>]*>`).ReplaceAllString(
		regexp.MustCompile(`<uuid#\d+>`).ReplaceAllString(binder.Apply(text), "<uuid>"), "<ts>")
	if got := orderMask(text); got != fromBinder {
		t.Errorf("mask and binder disagree\n mask   %s\n binder %s", got, fromBinder)
	}
}
