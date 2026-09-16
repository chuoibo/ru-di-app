package mediasnap

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mobile/parity/internal/normalize"
)

// randomKey returns a storage key that starts with prefix and is random after
// it. Prefixes are chosen per test, so which keys share a key directory is
// the test's decision, not chance.
func randomKey(t *testing.T, prefix string) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return prefix + hex.EncodeToString(b[:])[len(prefix):]
}

// side is one stack's store and binder, driven the way runner.Execute drives
// them: after each step the database rows are observed, then the store.
type side struct {
	t      *testing.T
	root   string
	binder *normalize.Binder
	prev   *Snap
	steps  []*Change
}

func newSide(t *testing.T) *side {
	t.Helper()
	s := &side{t: t, root: t.TempDir(), binder: normalize.NewBinder()}
	s.prev = s.snapshot()
	return s
}

func (s *side) snapshot() *Snap {
	s.t.Helper()
	snap, err := Snapshot(s.root)
	if err != nil {
		s.t.Fatal(err)
	}
	return snap
}

// settle moves every directory's modification time into the past, as the
// time between two steps of a real run does. A test writes and snapshots
// within one clock tick, where a later write would not move it.
func (s *side) settle() {
	s.t.Helper()
	past := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != s.root {
			return os.Chtimes(path, past, past)
		}
		return nil
	})
	if err != nil {
		s.t.Fatal(err)
	}
}

// step ends a step whose database change was rows.
func (s *side) step(rows ...string) {
	s.t.Helper()
	if len(rows) > 0 {
		if err := s.binder.ObserveGroups([][]string{rows}); err != nil {
			s.t.Fatal(err)
		}
	}
	next := s.snapshot()
	change := Delta(s.prev, next)
	if err := s.binder.ObserveGroups(change.Groups()); err != nil {
		s.t.Fatal(err)
	}
	s.steps = append(s.steps, change)
	s.settle()
	s.prev = s.snapshot()
}

// seed stores a file before the scenario, as an earlier scenario on the same
// store would: no step saw it and no row of this scenario names it.
func (s *side) seed(key, data string) {
	s.t.Helper()
	s.put(key, data)
	s.settle()
	s.prev = s.snapshot()
}

// upload is an uploaded_images row: the photo id is the same on both stacks
// here, as a numbered id from the response would be.
func upload(photo, key string) string {
	return `{"id":"` + photo + `","storage_key":"` + key + `","content_type":"image/jpeg"}`
}

func (s *side) mkdir(rel string, mode fs.FileMode) {
	s.t.Helper()
	path := filepath.Join(s.root, filepath.FromSlash(rel))
	if err := os.Mkdir(path, mode); err != nil && !errors.Is(err, fs.ErrExist) {
		s.t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		s.t.Fatal(err)
	}
}

func (s *side) write(rel, data string, mode fs.FileMode) {
	s.t.Helper()
	path := filepath.Join(s.root, filepath.FromSlash(rel))
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		s.t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		s.t.Fatal(err)
	}
}

// put leaves what PhotoStorage.write leaves: directories 0755, the file 0600.
func (s *side) put(key, data string) { s.putWith(key, data, 0o600, 0o755) }

func (s *side) putWith(key, data string, file, dirs fs.FileMode) {
	s.t.Helper()
	s.mkdir(key[:2], dirs)
	s.mkdir(key[:2]+"/"+key[2:4], dirs)
	s.write(stored(key), data, file)
}

func (s *side) remove(rel string) {
	s.t.Helper()
	if err := os.Remove(filepath.Join(s.root, filepath.FromSlash(rel))); err != nil {
		s.t.Fatal(err)
	}
}

func stored(key string) string { return key[:2] + "/" + key[2:4] + "/" + key }

func normalised(s *side) []*Change {
	out := make([]*Change, len(s.steps))
	for i, change := range s.steps {
		out[i] = change.Normalise(s.binder)
	}
	return out
}

func compareSides(t *testing.T, reference, candidate *side) [][]Difference {
	t.Helper()
	ref, cand := normalised(reference), normalised(candidate)
	if len(ref) != len(cand) {
		t.Fatalf("fixture: %d steps against %d", len(ref), len(cand))
	}
	out := make([][]Difference, len(ref))
	for i := range ref {
		out[i] = Compare(ref[i], cand[i])
	}
	return out
}

func requireEqual(t *testing.T, reference, candidate *side) {
	t.Helper()
	for i, diffs := range compareSides(t, reference, candidate) {
		if len(diffs) > 0 {
			t.Fatalf("step %d differs:\n%v", i, diffs)
		}
	}
}

// requireDifference returns the difference of kind in step, failing when that
// step has none of that kind.
func requireDifference(t *testing.T, reference, candidate *side, step int, kind string) Difference {
	t.Helper()
	all := compareSides(t, reference, candidate)
	for _, d := range all[step] {
		if d.Kind == kind {
			return d
		}
	}
	t.Fatalf("step %d has no %s difference: %v", step, kind, all)
	return Difference{}
}

func joined(changes []*Change) string {
	var texts []string
	for _, c := range changes {
		for _, group := range c.Groups() {
			texts = append(texts, group...)
		}
	}
	return strings.Join(texts, "\n")
}

// Two stores that did the same thing under different random keys compare
// equal at every step. The reference's keys share directories the way a
// store full of earlier runs makes them: k1 lands in the directory k0's
// deletion left empty, k3 beside k1's file, k2 under k0's k[0:2] alone, and
// deleting k1 leaves k3 in its directory. The candidate's keys share nothing.
func TestStoresThatDidTheSameThingCompareEqualWhateverTheKeys(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	keys := map[*side][4]string{
		reference: {randomKey(t, "ab01"), randomKey(t, "ab01"), randomKey(t, "ab77"), randomKey(t, "ab01")},
		candidate: {randomKey(t, "e5f6"), randomKey(t, "7a8b"), randomKey(t, "9cad"), randomKey(t, "3d4e")},
	}
	for _, s := range []*side{reference, candidate} {
		k := keys[s]
		s.put(k[0], "jpeg bytes")
		s.step(upload("photo-a", k[0]))
		s.remove(stored(k[0]))
		s.step()
		s.put(k[1], "second")
		s.put(k[2], "third")
		s.step(upload("photo-b", k[1]), upload("photo-c", k[2]))
		s.put(k[3], "fourth")
		s.step(upload("photo-d", k[3]))
		s.remove(stored(k[1]))
		s.step()
		s.step()
	}
	requireEqual(t, reference, candidate)

	texts := joined(normalised(reference))
	for _, want := range []string{
		`{"place":"key","type":"file","path":"<hex32#1>[0:2]/<hex32#1>[2:4]/<hex32#1>","dirs":"0755/0755","mode":"0600","size":10,`,
		`{"place":"key","type":"file","path":"<hex32#1>[0:2]/<hex32#1>[2:4]/<hex32#1>","dirs":"0755/0755","dirs_after":"0755/0755","mode":"0600"`,
		`"path":"<hex32#3>[0:2]/<hex32#3>[2:4]/<hex32#3>"`,
	} {
		if !strings.Contains(texts, want) {
			t.Errorf("normalised texts lack %s:\n%s", want, texts)
		}
	}
	if strings.Contains(texts, PlaceKeyDir) {
		t.Errorf("a key directory was compared on its own:\n%s", texts)
	}
	sum := sha256.Sum256([]byte("jpeg bytes"))
	if !strings.Contains(texts, strings.ToUpper(hex.EncodeToString(sum[:]))) || strings.Contains(texts, "<digest#") {
		t.Errorf("the content hash is not literal:\n%s", texts)
	}
	if strings.Contains(texts, keys[reference][0][8:24]) {
		t.Errorf("a key was left unbound:\n%s", texts)
	}
}

// A file one stack never wrote is a difference although both wrote the row.
func TestAFileOneStackNeverWroteIsADifference(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	key := randomKey(t, "a1b2")
	reference.put(key, "jpeg bytes")
	reference.step(upload("photo-a", key))
	candidate.step(upload("photo-a", randomKey(t, "c3d4")))
	d := requireDifference(t, reference, candidate, 0, KindCreated)
	if !strings.HasPrefix(d.String(), "created reference 1, candidate 0") || len(d.OnlyReference) != 1 {
		t.Errorf("difference reads %q", d.String())
	}
}

// A file stored under another key than the row names is a difference: the
// key is bound through the row, not merely counted.
func TestAFileUnderAKeyNoRowNamesIsADifference(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
	reference.put(refKey, "jpeg bytes")
	reference.step(upload("photo-a", refKey))
	candidate.put(randomKey(t, "e5f6"), "jpeg bytes")
	candidate.step(upload("photo-a", candKey))
	d := requireDifference(t, reference, candidate, 0, KindCreated)
	if len(d.OnlyReference) != 1 || !strings.Contains(d.OnlyReference[0], "<hex32#1>") ||
		len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], "<hex32#2>") {
		t.Errorf("difference reads %q", d.String())
	}
}

func TestModeBitsOfFilesAndTheirDirectoriesAreCompared(t *testing.T) {
	for name, modes := range map[string][2]fs.FileMode{
		"file 0644":        {0o644, 0o755},
		"directories 0775": {0o600, 0o775},
	} {
		t.Run(name, func(t *testing.T) {
			reference, candidate := newSide(t), newSide(t)
			refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
			reference.put(refKey, "jpeg bytes")
			reference.step(upload("photo-a", refKey))
			candidate.putWith(candKey, "jpeg bytes", modes[0], modes[1])
			candidate.step(upload("photo-a", candKey))
			requireDifference(t, reference, candidate, 0, KindCreated)
		})
	}
	// A mode that changes later is a change of that file.
	reference, candidate := newSide(t), newSide(t)
	keys := map[*side]string{reference: randomKey(t, "a1b2"), candidate: randomKey(t, "c3d4")}
	for s, key := range keys {
		s.put(key, "jpeg bytes")
		s.step(upload("photo-a", key))
	}
	if err := os.Chmod(filepath.Join(candidate.root, filepath.FromSlash(stored(keys[candidate]))), 0o640); err != nil {
		t.Fatal(err)
	}
	reference.step()
	candidate.step()
	if d := requireDifference(t, reference, candidate, 1, KindChanged); d.Candidate != 1 || d.Reference != 0 {
		t.Errorf("difference reads %q", d.String())
	}
}

// A temporary file left beside the stored one is a difference; its eight
// random characters are not.
func TestALeftoverTemporaryFileIsADifferenceItsRandomNameIsNot(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
	reference.put(refKey, "jpeg bytes")
	reference.step(upload("photo-a", refKey))
	candidate.put(candKey, "jpeg bytes")
	candidate.write(candKey[:2]+"/"+candKey[2:4]+"/."+candKey+".x7_kqp9z.tmp", "jpeg", 0o600)
	candidate.step(upload("photo-a", candKey))
	d := requireDifference(t, reference, candidate, 0, KindCreated)
	if len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], `"place":"tmp"`) ||
		!strings.Contains(d.OnlyCandidate[0], `/.<hex32#1>.<rand8>.tmp"`) {
		t.Errorf("difference reads %q", d.String())
	}

	reference, candidate = newSide(t), newSide(t)
	for s, name := range map[*side]string{reference: "m3n_q8ra", candidate: "zz9_bcd1"} {
		key := randomKey(t, map[*side]string{reference: "a1b2", candidate: "c3d4"}[s])
		s.put(key, "jpeg bytes")
		s.write(key[:2]+"/"+key[2:4]+"/."+key+"."+name+".tmp", "jpeg", 0o600)
		s.step(upload("photo-a", key))
	}
	requireEqual(t, reference, candidate)
}

func TestContentIsComparedByteForByte(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
	reference.put(refKey, "jpeg bytes")
	reference.step(upload("photo-a", refKey))
	candidate.put(candKey, "jpeg bytez")
	candidate.step(upload("photo-a", candKey))
	requireDifference(t, reference, candidate, 0, KindCreated)
}

// Two files written in one step are tied to their rows by the photo ids the
// rows carry, not by the order of their random keys: the reference's keys sort
// one way and the candidate's the other. The same two contents swapped
// between the rows are a difference.
func TestFilesAreTiedToTheRowsThatNameThem(t *testing.T) {
	build := func(swap bool) (*side, *side) {
		reference, candidate := newSide(t), newSide(t)
		refA, refB := randomKey(t, "1a2b"), randomKey(t, "2b3c")
		candA, candB := randomKey(t, "fa9b"), randomKey(t, "0b1c")
		reference.put(refA, "photo A")
		reference.put(refB, "photo B")
		reference.step(upload("photo-a", refA), upload("photo-b", refB))
		contentA, contentB := "photo A", "photo B"
		if swap {
			contentA, contentB = contentB, contentA
		}
		candidate.put(candA, contentA)
		candidate.put(candB, contentB)
		candidate.step(upload("photo-a", candA), upload("photo-b", candB))
		return reference, candidate
	}
	reference, candidate := build(false)
	requireEqual(t, reference, candidate)
	reference, candidate = build(true)
	requireDifference(t, reference, candidate, 0, KindCreated)
}

// A deletion on one stack only is a deletion difference, and it records the
// directories the file leaves behind.
func TestDeletionsAreCompared(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	keys := map[*side]string{reference: randomKey(t, "a1b2"), candidate: randomKey(t, "c3d4")}
	for s, key := range keys {
		s.put(key, "jpeg bytes")
		s.step(upload("photo-a", key))
	}
	reference.remove(stored(keys[reference]))
	reference.step()
	candidate.step()
	d := requireDifference(t, reference, candidate, 1, KindDeleted)
	if d.Reference != 1 || d.Candidate != 0 || !strings.Contains(d.OnlyReference[0], `"dirs_after":"0755/0755"`) {
		t.Errorf("difference reads %q", d.String())
	}
}

// Python never removes a key directory. A port that removes it, with the file
// or later once it is empty, differs in that step.
func TestRemovedKeyDirectoriesAreADifference(t *testing.T) {
	removeDirs := func(s *side, key string) {
		s.remove(key[:2] + "/" + key[2:4])
		s.remove(key[:2])
	}

	reference, candidate := newSide(t), newSide(t)
	keys := map[*side]string{reference: randomKey(t, "a1b2"), candidate: randomKey(t, "c3d4")}
	for s, key := range keys {
		s.put(key, "jpeg bytes")
		s.step(upload("photo-a", key))
		s.remove(stored(key))
	}
	removeDirs(candidate, keys[candidate])
	reference.step()
	candidate.step()
	d := requireDifference(t, reference, candidate, 1, KindDeleted)
	if len(d.OnlyReference) != 1 || !strings.Contains(d.OnlyReference[0], `"dirs_after":"0755/0755"`) ||
		len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], `"dirs_after":"-/-"`) {
		t.Errorf("with the file: difference reads %q", d.String())
	}

	reference, candidate = newSide(t), newSide(t)
	keys = map[*side]string{reference: randomKey(t, "a1b2"), candidate: randomKey(t, "c3d4")}
	for s, key := range keys {
		s.put(key, "jpeg bytes")
		s.step(upload("photo-a", key))
		s.remove(stored(key))
		s.step()
	}
	removeDirs(candidate, keys[candidate])
	reference.step()
	candidate.step()
	d = requireDifference(t, reference, candidate, 2, KindDeleted)
	if d.Reference != 0 || d.Candidate != 1 ||
		!strings.Contains(d.OnlyCandidate[0], `{"place":"keydir","type":"dir","path":"<hex32#1>[0:2]/<hex32#1>[2:4]","dirs":"0755","mode":"0755"}`) {
		t.Errorf("once empty: difference reads %q", d.String())
	}
}

// A photo written and removed again within one step leaves nothing but a
// modified key directory, and that is a difference from a stack that wrote
// nothing. Whether the directory was new or already held another key's file
// is not.
func TestAFileWrittenAndRemovedWithinAStepIsADifference(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	orphan := randomKey(t, "a1b2")
	reference.put(orphan, "jpeg bytes")
	reference.remove(stored(orphan))
	reference.step()
	candidate.step()
	d := requireDifference(t, reference, candidate, 0, KindCreated)
	if d.Reference != 1 || d.Candidate != 0 ||
		!strings.Contains(d.OnlyReference[0], `{"place":"keydir","type":"dir","path":"<hex2>/<hex2>","dirs":"0755","mode":"0755"}`) {
		t.Errorf("difference reads %q", d.String())
	}

	reference, candidate = newSide(t), newSide(t)
	reference.seed(randomKey(t, "c3d4"), "an earlier scenario's photo")
	for s, key := range map[*side]string{reference: randomKey(t, "c3d4"), candidate: randomKey(t, "e5f6")} {
		s.put(key, "jpeg bytes")
		s.remove(stored(key))
		s.step()
	}
	requireEqual(t, reference, candidate)
}

// A file outside its key's directories is a stray, whatever its name.
func TestAFileInTheWrongDirectoriesIsAStray(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
	reference.put(refKey, "jpeg bytes")
	reference.step(upload("photo-a", refKey))
	candidate.mkdir(candKey[2:4], 0o755)
	candidate.mkdir(candKey[2:4]+"/"+candKey[:2], 0o755)
	candidate.write(candKey[2:4]+"/"+candKey[:2]+"/"+candKey, "jpeg bytes", 0o600)
	candidate.step(upload("photo-a", candKey))
	d := requireDifference(t, reference, candidate, 0, KindCreated)
	if len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], `"place":"stray","type":"file","path":"<hex2>/<hex2>/<hex32#1>"`) {
		t.Errorf("difference reads %q", d.String())
	}
}

// symlinkAt puts a symlink where key's file would go, with key's directories
// around it.
func (s *side) symlinkAt(key, target string) {
	s.t.Helper()
	s.mkdir(key[:2], 0o755)
	s.mkdir(key[:2]+"/"+key[2:4], 0o755)
	if err := os.Symlink(target, filepath.Join(s.root, filepath.FromSlash(stored(key)))); err != nil {
		s.t.Fatal(err)
	}
}

// outsideFile writes a file beside the store and returns how a stored file's
// symlink spells it: three levels up from k[0:2]/k[2:4]/k. It lives outside
// the store, so the file itself is never an entry of either snapshot.
func (s *side) outsideFile(name, data string) string {
	s.t.Helper()
	path := filepath.Join(filepath.Dir(s.root), name)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		s.t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		s.t.Fatal(err)
	}
	return "../../../" + name
}

// A symlink is compared as a symlink, by what it says, never by what it points
// at: a link whose target holds the other stack's bytes, in the other stack's
// mode, is still a difference. A snapshot that read through its links would
// call these equal.
func TestSymlinksAreComparedAsSymlinksNotAsWhatTheyPointAt(t *testing.T) {
	t.Run("a symlink where the other stack has the file", func(t *testing.T) {
		reference, candidate := newSide(t), newSide(t)
		refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
		reference.put(refKey, "jpeg bytes")
		reference.step(upload("photo-a", refKey))
		candidate.symlinkAt(candKey, candidate.outsideFile("target.bin", "jpeg bytes"))
		candidate.step(upload("photo-a", candKey))
		d := requireDifference(t, reference, candidate, 0, KindCreated)
		if len(d.OnlyReference) != 1 || !strings.Contains(d.OnlyReference[0], `"type":"file"`) ||
			len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], `"type":"symlink"`) ||
			!strings.Contains(d.OnlyCandidate[0], `"link":"../../../target.bin"`) ||
			strings.Contains(d.OnlyCandidate[0], `"sha256"`) {
			t.Errorf("difference reads %q", d.String())
		}
	})

	t.Run("two symlinks that name different targets", func(t *testing.T) {
		reference, candidate := newSide(t), newSide(t)
		refKey, candKey := randomKey(t, "a1b2"), randomKey(t, "c3d4")
		reference.symlinkAt(refKey, reference.outsideFile("target.bin", "jpeg bytes"))
		reference.step(upload("photo-a", refKey))
		candidate.symlinkAt(candKey, candidate.outsideFile("other.bin", "jpeg bytes"))
		candidate.step(upload("photo-a", candKey))
		d := requireDifference(t, reference, candidate, 0, KindCreated)
		if len(d.OnlyReference) != 1 || !strings.Contains(d.OnlyReference[0], `"link":"../../../target.bin"`) ||
			len(d.OnlyCandidate) != 1 || !strings.Contains(d.OnlyCandidate[0], `"link":"../../../other.bin"`) {
			t.Errorf("difference reads %q", d.String())
		}
	})

	t.Run("two symlinks that say the same thing", func(t *testing.T) {
		reference, candidate := newSide(t), newSide(t)
		for s, key := range map[*side]string{reference: randomKey(t, "a1b2"), candidate: randomKey(t, "c3d4")} {
			s.symlinkAt(key, s.outsideFile("target.bin", "jpeg bytes"))
			s.step(upload("photo-a", key))
		}
		requireEqual(t, reference, candidate)
		if texts := joined(normalised(reference)); !strings.Contains(texts, `"type":"symlink"`) {
			t.Errorf("no symlink was compared:\n%s", texts)
		}
	})
}

// A key directory whose four hex digits two known keys share is tied to
// neither: it is masked. Both stacks do the same thing here, and the key a
// guess would reach for is photo-a's on the reference and photo-b's on the
// candidate, so guessing shows as a difference between them.
func TestATouchedKeyDirectoryWithAnAmbiguousPrefixIsMasked(t *testing.T) {
	reference, candidate := newSide(t), newSide(t)
	keys := map[*side][3]string{
		reference: {randomKey(t, "ab010a"), randomKey(t, "ab01f0"), randomKey(t, "ab01cc")},
		candidate: {randomKey(t, "cd02f0"), randomKey(t, "cd020a"), randomKey(t, "cd02cc")},
	}
	for _, s := range []*side{reference, candidate} {
		k := keys[s]
		s.put(k[0], "photo A")
		s.put(k[1], "photo B")
		s.step(upload("photo-a", k[0]), upload("photo-b", k[1]))
		// A third photo written and removed within the step, in the directory
		// the two named keys already share.
		s.put(k[2], "photo C")
		s.remove(stored(k[2]))
		s.step()
	}
	requireEqual(t, reference, candidate)
	texts := joined(normalised(reference))
	if !strings.Contains(texts, `{"place":"keydir","type":"dir","path":"<hex2>/<hex2>","dirs":"0755","mode":"0755"}`) {
		t.Errorf("an ambiguous key directory was tied to one of the keys:\n%s", texts)
	}
}

func TestSnapshotNeedsAReadableDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := Snapshot(filepath.Join(root, "missing")); !errors.Is(err, ErrSnapshot) {
		t.Errorf("missing root: %v", err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Snapshot(file); !errors.Is(err, ErrSnapshot) {
		t.Errorf("file as root: %v", err)
	}
	if os.Geteuid() == 0 {
		return // root reads a 0000 file
	}
	if err := os.Chmod(file, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Snapshot(root); !errors.Is(err, ErrSnapshot) {
		t.Errorf("unreadable file: %v", err)
	}
}

func TestStoredFilesListsStoredFilesOnly(t *testing.T) {
	s := newSide(t)
	key := randomKey(t, "a1b2")
	s.put(key, "jpeg bytes")
	s.write(key[:2]+"/"+key[2:4]+"/."+key+".abcdefgh.tmp", "", 0o600)
	s.write("stray", "", 0o600)
	files, err := StoredFiles(s.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !files[stored(key)] {
		t.Errorf("stored files = %v", files)
	}
}
