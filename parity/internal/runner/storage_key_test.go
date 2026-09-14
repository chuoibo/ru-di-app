package runner

import (
	"strings"
	"testing"

	"mobile/parity/internal/normalize"
)

// uploads takes rows through the path Run takes: observe the change's texts,
// then normalise the change with the same binder.
func uploads(t *testing.T, keys ...string) *Run {
	t.Helper()
	texts := make([]string, len(keys))
	for i, key := range keys {
		texts[i] = `{"storage_key":"` + key + `","content_type":"image/jpeg"}`
	}
	change := inserted(texts...)
	binder := normalize.NewBinder()
	for _, text := range change.Texts() {
		if err := binder.Observe(text); err != nil {
			t.Fatal(err)
		}
	}
	return transcript(StepResult{StepID: "upload", Norm: wire, NormChange: change.Normalise(binder.Apply)})
}

// A storage key is random on each stack and only the database lane sees it.
// Different random keys are equal; a key that is reused, uppercase, of another
// length or a uuid is still a difference.
func TestStorageKeysCompareByReuseAndSpellingNotValue(t *testing.T) {
	key := func(c string) string { return strings.Repeat(c, 32) }
	ref := uploads(t, key("a"), key("b"))
	if diffs := Diff(ref, uploads(t, key("c"), key("d"))); len(diffs) != 0 {
		t.Fatalf("random keys differ: %+v", diffs)
	}
	for name, cand := range map[string]*Run{
		"reused":    uploads(t, key("c"), key("c")),
		"uppercase": uploads(t, key("C"), key("D")),
		"31 hex":    uploads(t, strings.Repeat("c", 31), key("d")),
		"uuid":      uploads(t, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", key("d")),
	} {
		if diffs := Diff(ref, cand); len(diffs) != 1 || len(diffs[0].Database) == 0 {
			t.Errorf("%s key is not a database difference: %+v", name, diffs)
		}
	}
}
