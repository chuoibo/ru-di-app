// Package mediasnap records what a scenario step did to a stack's photo store,
// so the parity harness compares the files a port writes and removes the way
// dbsnap compares the rows it writes.
//
// Per stack: Snapshot the store root once before the first step and again
// after every step; Delta(previous, next) is that step's Change. The runner
// observes Change.Groups() with that stack's normalize.Binder after the step's
// response and database change, so a storage key an uploaded_images row named
// keeps that row's <hex32#n> and its file is tied to the row. Once every step
// is observed it calls Change.Normalise(binder) and Compare(reference,
// candidate).
//
// # What a snapshot holds
//
// The store is app/media/storage.py's layout: root/k[0:2]/k[2:4]/k, written
// through a temporary ".<k>.<8 characters>.tmp" in the same directory, 0600,
// with directories made by mkdir(parents=True) and never removed. A snapshot
// holds every entry that is not a directory, every empty directory that is not
// a key directory, and the permission bits and modification time of every
// directory. An entry carries its path relative to the root, its type, its
// permission bits, the permission bits of every directory between the root and
// it, and, for a regular file, its size and sha256. Owners and directory sizes
// are left out: the host user and the filesystem decide them.
//
// # Key directories
//
// k[0:2] and k[0:2]/k[2:4] belong to every key with those digits, and which
// random keys share one differs between stacks. A key directory is therefore
// never compared as created, emptied or filled: an upload into a directory an
// earlier key left empty would read, on one stack only, as a directory that
// stopped being empty. What is compared does not depend on the dice:
//
//   - the modes of a file's directories, in the file's entry;
//   - for a deleted entry, the modes its directories have after the step, "-"
//     for one that is gone ("dirs_after");
//   - a k[0:2]/k[2:4] directory created or modified during a step with no
//     created, changed or deleted entry beneath it: something was written
//     there and removed again within the step, as by a port that deletes a
//     photo after a failed insert;
//   - a k[0:2]/k[2:4] directory that was empty and is gone.
//
// The last two are "keydir" entries. Modification times decide "modified", so
// a write and removal inside a directory changed less than one clock tick
// before the previous snapshot is missed.
//
// # How an entry renders
//
// Entry.Text is one JSON object with a fixed member order. What "path" says
// depends on the entry's place:
//
//   - "key": a non-directory at root/k[0:2]/k[2:4]/k, k 32 lowercase hex. The
//     path is spelled "k[0:2]/k[2:4]/k" with the key literal three times, so
//     binding the key binds its two directory names with it.
//   - "tmp": ".k.XXXXXXXX.tmp" in k's directory, the eight tempfile characters
//     written <rand8>; another length or alphabet stays literal.
//   - "keydir": a k[0:2]/k[2:4] directory. Its four hex digits are all it
//     knows of a key, so Normalise ties it to the one key the binder observed
//     with that prefix, and masks it <hex2>/<hex2> when there is none or more
//     than one.
//   - "stray": anything else. Every two-hex path component is masked <hex2>;
//     32-hex runs are left for the binder like any other text.
//
// A content hash renders as uppercase hex. The binder numbers 64 lowercase hex
// as <digest#n>; in uppercase it never matches, so hashes are compared
// literally: a photo must be the same bytes on both stacks, not merely a file
// of the same shape.
package mediasnap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ErrSnapshot wraps every error Snapshot returns. The store could not be read:
// that is the harness failing, never a difference between stacks.
var ErrSnapshot = errors.New("media snapshot")

// Snap is one store at one moment.
type Snap struct {
	Root    string
	Entries []Entry        // sorted by Path
	Dirs    map[string]Dir // every directory below the root, by relative path
}

// Dir is one directory of a snapshot.
type Dir struct {
	Mode  fs.FileMode
	MTime int64 // nanoseconds
	Empty bool
}

// Entry is one compared item of the store.
type Entry struct {
	Path   string      // slash-separated, relative to the root, as on disk
	Type   string      // "file", "dir", "symlink" or "other"
	Mode   fs.FileMode // permission, setuid, setgid and sticky bits
	Dirs   []fs.FileMode
	After  string // deleted entries only: its directories' modes after the step
	Size   int64  // regular files only
	SHA256 string // regular files only, lowercase hex
	Link   string // symlinks only: the target, unresolved
	Place  string // "key", "tmp", "keydir" or "stray"
	Key    string // the storage key of a key or tmp entry
	Prefix string // the four hex digits of a keydir entry
	Rand   string // the random part of a tmp name
	Text   string // what deltas compare
	mask   string // Text with every random name hidden, for ordering
}

// Place values.
const (
	PlaceKey    = "key"
	PlaceTemp   = "tmp"
	PlaceKeyDir = "keydir"
	PlaceStray  = "stray"
)

var (
	storageKey = regexp.MustCompile(`\A[0-9a-f]{32}\z`)
	tempName   = regexp.MustCompile(`\A\.([0-9a-f]{32})\.(.*)\.tmp\z`)
	hexByte    = regexp.MustCompile(`\A[0-9a-f]{2}\z`)
	// tempfile._RandomNameSequence: eight of "abcdefghijklmnopqrstuvwxyz0123456789_".
	tempRandom = regexp.MustCompile(`\A[a-z0-9_]{8}\z`)
	keyRun     = regexp.MustCompile(`[0-9a-f]{32,}`)
)

// Snapshot reads the store under root. The root must be a directory; a missing
// root is an error, never an empty store, or a lane pointed at the wrong path
// would compare two empty trees and read as equal.
func Snapshot(root string) (*Snap, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSnapshot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrSnapshot, root)
	}
	snap := &Snap{Root: root, Dirs: map[string]Dir{}}
	if err := walk(snap, root, "", nil); err != nil {
		return nil, err
	}
	sort.Slice(snap.Entries, func(i, j int) bool { return snap.Entries[i].Path < snap.Entries[j].Path })
	return snap, nil
}

// walk reads the directory rel, whose ancestors between the root and it
// (itself included, unless it is the root) have modes dirs.
func walk(snap *Snap, root, rel string, dirs []fs.FileMode) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	children, err := os.ReadDir(full)
	if err != nil {
		if rel != "" && errors.Is(err, fs.ErrNotExist) {
			return nil // removed while the walk was under way
		}
		return fmt.Errorf("%w: %v", ErrSnapshot, err)
	}
	for _, child := range children {
		childRel := child.Name()
		if rel != "" {
			childRel = rel + "/" + child.Name()
		}
		path := filepath.Join(full, child.Name())
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("%w: %v", ErrSnapshot, err)
		}
		mode := permissions(info.Mode())
		if info.IsDir() {
			empty, err := isEmpty(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return fmt.Errorf("%w: %v", ErrSnapshot, err)
			}
			snap.Dirs[childRel] = Dir{Mode: mode, MTime: info.ModTime().UnixNano(), Empty: empty}
			if empty {
				if !keyDirPosition(childRel) {
					snap.Entries = append(snap.Entries, classify(Entry{Path: childRel, Type: "dir", Mode: mode, Dirs: dirs}))
				}
				continue
			}
			if err := walk(snap, root, childRel, append(append([]fs.FileMode(nil), dirs...), mode)); err != nil {
				return err
			}
			continue
		}
		entry := Entry{Path: childRel, Mode: mode, Dirs: dirs}
		switch {
		case info.Mode().IsRegular():
			entry.Type = "file"
			size, sum, err := hashFile(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return fmt.Errorf("%w: %v (the harness user must be able to read every stored file)", ErrSnapshot, err)
			}
			entry.Size, entry.SHA256 = size, sum
		case info.Mode()&fs.ModeSymlink != 0:
			entry.Type = "symlink"
			if entry.Link, err = os.Readlink(path); err != nil {
				return fmt.Errorf("%w: %v", ErrSnapshot, err)
			}
		default:
			entry.Type = "other"
		}
		snap.Entries = append(snap.Entries, classify(entry))
	}
	return nil
}

// keyDirPosition reports whether rel is where a key directory goes: k[0:2]
// or k[0:2]/k[2:4].
func keyDirPosition(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if !hexByte.MatchString(part) {
			return false
		}
	}
	return len(parts) <= 2
}

// keyDir reports whether rel is a k[0:2]/k[2:4] directory.
func keyDir(rel string) bool {
	return strings.Count(rel, "/") == 1 && keyDirPosition(rel)
}

func isEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.Readdirnames(1); err != nil {
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func hashFile(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

// permissions keeps the bits chmod sets, as fs.FileMode bits.
func permissions(mode fs.FileMode) fs.FileMode {
	return mode & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
}

// octal spells permission bits as chmod does: "0600", "4755".
func octal(mode fs.FileMode) string {
	bits := uint32(mode & fs.ModePerm)
	if mode&fs.ModeSetuid != 0 {
		bits |= 0o4000
	}
	if mode&fs.ModeSetgid != 0 {
		bits |= 0o2000
	}
	if mode&fs.ModeSticky != 0 {
		bits |= 0o1000
	}
	return fmt.Sprintf("%04o", bits)
}

// classify sets Place, Key and Rand from the path, then Text and mask.
func classify(e Entry) Entry {
	parts := strings.Split(e.Path, "/")
	e.Place = PlaceStray
	if e.Type != "dir" && len(parts) == 3 {
		if storageKey.MatchString(parts[2]) && parts[0] == parts[2][0:2] && parts[1] == parts[2][2:4] {
			e.Place, e.Key = PlaceKey, parts[2]
		} else if m := tempName.FindStringSubmatch(parts[2]); m != nil && parts[0] == m[1][0:2] && parts[1] == m[1][2:4] {
			e.Place, e.Key, e.Rand = PlaceTemp, m[1], m[2]
		}
	}
	return e.rendered()
}

// keyDirEntry is the keydir entry for the k[0:2]/k[2:4] directory path of snap.
func keyDirEntry(snap *Snap, path string) Entry {
	parts := strings.Split(path, "/")
	e := Entry{Path: path, Type: "dir", Mode: snap.Dirs[path].Mode, Place: PlaceKeyDir, Prefix: parts[0] + parts[1]}
	if first, ok := snap.Dirs[parts[0]]; ok {
		e.Dirs = []fs.FileMode{first.Mode}
	}
	return e.rendered()
}

// rendered sets Text and mask from the other fields.
func (e Entry) rendered() Entry {
	e.Text = e.render(e.pathText())
	e.mask = e.render(e.maskedPath())
	return e
}

// pathText is how the entry's path is spelled before normalisation.
func (e Entry) pathText() string {
	switch e.Place {
	case PlaceKey:
		return keyPath(e.Key)
	case PlaceTemp:
		return e.Key + "[0:2]/" + e.Key + "[2:4]/." + e.Key + "." + randomText(e.Rand) + ".tmp"
	case PlaceKeyDir:
		return e.Prefix[0:2] + "/" + e.Prefix[2:4]
	}
	parts := strings.Split(e.Path, "/")
	for i, part := range parts {
		if hexByte.MatchString(part) {
			parts[i] = "<hex2>"
		}
	}
	return strings.Join(parts, "/")
}

// maskedPath hides every random name, so ordering by it gives both stacks one
// order.
func (e Entry) maskedPath() string {
	if e.Place == PlaceKeyDir {
		return "<hex2>/<hex2>"
	}
	return keyRun.ReplaceAllStringFunc(e.pathText(), func(run string) string {
		if len(run) == 32 {
			return "<hex32>"
		}
		return run
	})
}

// keyPath spells a stored file's path through its key alone.
func keyPath(key string) string {
	return key + "[0:2]/" + key + "[2:4]/" + key
}

func randomText(random string) string {
	if tempRandom.MatchString(random) {
		return "<rand8>"
	}
	return random
}

// dirsAfter spells the modes the directories above path have in snap, "-" for
// one that is not there.
func dirsAfter(snap *Snap, path string) string {
	parts := strings.Split(path, "/")
	modes := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		if dir, ok := snap.Dirs[strings.Join(parts[:i], "/")]; ok {
			modes = append(modes, octal(dir.Mode))
		} else {
			modes = append(modes, "-")
		}
	}
	return strings.Join(modes, "/")
}

type rendered struct {
	Place  string `json:"place"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	Dirs   string `json:"dirs"`
	After  string `json:"dirs_after,omitempty"`
	Mode   string `json:"mode"`
	Size   *int64 `json:"size,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Link   string `json:"link,omitempty"`
}

func (e Entry) render(path string) string {
	dirs := make([]string, len(e.Dirs))
	for i, mode := range e.Dirs {
		dirs[i] = octal(mode)
	}
	r := rendered{Place: e.Place, Type: e.Type, Path: path, Dirs: strings.Join(dirs, "/"), After: e.After, Mode: octal(e.Mode), Link: e.Link}
	if e.Type == "file" {
		size := e.Size
		r.Size = &size
		r.SHA256 = strings.ToUpper(e.SHA256)
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	// Placeholders such as <rand8> must stay readable, and Apply matches them.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(r); err != nil {
		panic(err) // strings and an int64 always encode
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// StoredFiles lists the relative paths of the stored files under root: entries
// that are not directories at k[0:2]/k[2:4]/k. It reads names only, no content.
func StoredFiles(root string) (map[string]bool, error) {
	out := map[string]bool{}
	firsts, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSnapshot, err)
	}
	for _, first := range firsts {
		if !first.IsDir() || !hexByte.MatchString(first.Name()) {
			continue
		}
		seconds, err := os.ReadDir(filepath.Join(root, first.Name()))
		if err != nil {
			continue
		}
		for _, second := range seconds {
			if !second.IsDir() || !hexByte.MatchString(second.Name()) {
				continue
			}
			files, err := os.ReadDir(filepath.Join(root, first.Name(), second.Name()))
			if err != nil {
				continue
			}
			for _, file := range files {
				name := file.Name()
				if !file.IsDir() && storageKey.MatchString(name) && name[0:2] == first.Name() && name[2:4] == second.Name() {
					out[first.Name()+"/"+second.Name()+"/"+name] = true
				}
			}
		}
	}
	return out, nil
}
