package dbsnap

import (
	"regexp"
	"strings"
	"testing"

	"mobile/parity/internal/normalize"
)

// tiedShares is one step on one stack: two bill items and one person's share
// of each. The two share rows are equal once ids are masked; only the item id
// they point to tells them apart. Which share's own random id sorts first is
// what differs between stacks.
func tiedShares(itemPho, itemTra, sharePho, shareTra string) *Snap {
	return snapOf(
		&Relation{Name: "bill_items", Kind: "table", PrimaryKey: []string{"id"}, Rows: []Row{
			row(`["`+itemPho+`"]`, `{"id":"`+itemPho+`","name":"pho","amount_vnd":100}`),
			row(`["`+itemTra+`"]`, `{"id":"`+itemTra+`","name":"tra","amount_vnd":100}`),
		}},
		&Relation{Name: "bill_item_shares", Kind: "table", PrimaryKey: []string{"id"}, Rows: []Row{
			row(`["`+sharePho+`"]`, `{"id":"`+sharePho+`","bill_item_id":"`+itemPho+`","participant":"an","amount_vnd":100}`),
			row(`["`+shareTra+`"]`, `{"id":"`+shareTra+`","bill_item_id":"`+itemTra+`","participant":"an","amount_vnd":100}`),
		}},
	)
}

// The reference's pho share sorts first by its random id, the candidate's tra
// share does. Letter-rich ids keep the repo guard's digit-run rule quiet.
func tiedStacks() (reference, candidate *Snap) {
	reference = tiedShares(
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb2",
		"cccccccc-cccc-4ccc-8ccc-cccccccccc01", "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee02")
	candidate = tiedShares(
		"dddddddd-dddd-4ddd-8ddd-dddddddddd03", "abcdabcd-abcd-4abc-8abc-abcdabcdab04",
		"fefefefe-fefe-4efe-8efe-fefefefefe05", "cafecafe-cafe-4afe-8afe-cafecafeca06")
	return reference, candidate
}

func normalisedWith(t *testing.T, snap *Snap, observe func(*normalize.Binder, *Change) error) *Change {
	t.Helper()
	binder := normalize.NewBinder()
	delta := Delta(nil, snap)
	if err := observe(binder, delta); err != nil {
		t.Fatal(err)
	}
	return delta.Normalise(binder.Apply)
}

func observeTexts(binder *normalize.Binder, delta *Change) error {
	for _, text := range delta.Texts() {
		if err := binder.Observe(text); err != nil {
			return err
		}
	}
	return nil
}

func observeGroups(binder *normalize.Binder, delta *Change) error {
	return binder.ObserveGroups(delta.Groups())
}

func TestRowsTiedOnceMaskedAreToldApartByTheValuesTheyName(t *testing.T) {
	reference, candidate := tiedStacks()
	// Without the rule the fixture must differ, or it proves nothing.
	if diffs := Compare(normalisedWith(t, reference, observeTexts), normalisedWith(t, candidate, observeTexts)); len(diffs) == 0 {
		t.Fatal("fixture: observing in text order already agrees; the stacks do not exercise a tie")
	}
	ref := normalisedWith(t, reference, observeGroups)
	cand := normalisedWith(t, candidate, observeGroups)
	if diffs := Compare(ref, cand); len(diffs) > 0 {
		t.Errorf("tied shares numbered by their random ids:\n%v", diffs)
	}
	joined := strings.Join(ref.Texts(), "\n")
	if strings.Contains(joined, `"<uuid>"`) || regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-4`).MatchString(joined) {
		t.Errorf("an id was left unbound:\n%s", joined)
	}
}

// A share that points to the wrong item is still a difference once ties are
// broken by the item id.
func TestBrokenTiesStillShowAShareOnTheWrongItem(t *testing.T) {
	reference, _ := tiedStacks()
	wrong := tiedShares(
		"dddddddd-dddd-4ddd-8ddd-dddddddddd03", "abcdabcd-abcd-4abc-8abc-abcdabcdab04",
		"fefefefe-fefe-4efe-8efe-fefefefefe05", "cafecafe-cafe-4afe-8afe-cafecafeca06")
	shares := wrong.Relations[1].Rows
	shares[1].Text = strings.Replace(shares[1].Text, "abcdabcd-abcd-4abc-8abc-abcdabcdab04", "dddddddd-dddd-4ddd-8ddd-dddddddddd03", 1)
	if diffs := Compare(normalisedWith(t, reference, observeGroups), normalisedWith(t, wrong, observeGroups)); len(diffs) == 0 {
		t.Error("both shares on one item compared equal")
	}
}

// Rows nothing tells apart are still observed, so every id is bound.
func TestRowsNothingTellsApartAreStillBound(t *testing.T) {
	snap := snapOf(keyed(t, "tags",
		`{"id":"`+newUUID(t)+`","label":"x"}`,
		`{"id":"`+newUUID(t)+`","label":"x"}`,
		`{"id":"`+newUUID(t)+`","label":"x"}`))
	joined := strings.Join(normalisedWith(t, snap, observeGroups).Texts(), "\n")
	if regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-4`).MatchString(joined) {
		t.Errorf("tied rows left unbound:\n%s", joined)
	}
}

// OrderKey on a binder that knows nothing masks exactly what orderMask does;
// otherwise a random value could decide the order through one of them.
func TestOrderKeyOfAFreshBinderIsTheOrderMask(t *testing.T) {
	spaced := "2026-09-14" + " " + "10:00:05"
	text := `{"a":"` + newUUID(t) + `","b":["` + newUUID(t) + `","` + instant(0, 5, 120000) + `"],"c":"` +
		spaced + `","d":"\\x` + strings.Repeat("ab", 32) + `","e":"` + strings.Repeat("c", 32) + `","f":"` +
		strings.Repeat("d", 64) + `","g":"/g/` + strings.Repeat("Ab3_", 11)[:43] + `"}`
	if got, want := normalize.NewBinder().OrderKey(text), orderMask(text); got != want {
		t.Errorf("order key and order mask disagree\n key  %s\n mask %s", got, want)
	}
}

// A value the binder knows keeps its number in the key; one it does not know
// is masked.
func TestOrderKeyNumbersKnownValuesOnly(t *testing.T) {
	known, unknown := newUUID(t), newUUID(t)
	token := strings.Repeat("Zx9-", 11)[:43]
	binder := normalize.NewBinder()
	if err := binder.Observe(`{"id":"` + known + `","path":"/g/` + token + `"}`); err != nil {
		t.Fatal(err)
	}
	got := binder.OrderKey(`{"a":"` + known + `","b":"` + unknown + `","c":"/g/` + token + `"}`)
	if want := `{"a":"<uuid#1>","b":"<uuid>","c":"/g/<token43#1>"}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
