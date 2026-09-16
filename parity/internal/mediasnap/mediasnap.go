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
// # Reading only what changed
//
// Hashing every stored file after every step costs the whole store on every
// step, and the store grows all run. A Cache carries what the last snapshot of
// one store saw into the next one: every entry is still stat'd, every
// directory is still listed, and only a file whose identity changed is opened
// and read again. Identity is the device and inode numbers, the link count,
// the full mode, the size, the modification time and the inode change time,
// plus the modes of the directories above it, which the entry's text spells.
//
// The cost of that: a file rewritten in place with the same size, inode and
// mode, inside one timestamp tick, is taken for the file we already hashed. Two
// things stand against it here. The store is content-addressed, and both
// writers — app/media/storage.py and PhotoStorage.Write in
// services/core/internal/media/storage — put a key down through a fresh
// temporary file in the same directory and rename(2) it over, so a rewrite
// arrives on a new inode; and a file whose directory's modification time moved
// since the previous snapshot is re-read whatever its identity says, so that
// rename makes every file in that directory read again. What is left is a write through an already-open descriptor that keeps the
// length, the mode, the inode and the directory's modification time, and lands
// within one tick of the previous snapshot. Nothing in this system writes that
// way. Where the operating system does not tell us a file's inode and change
// time the cache never reuses anything and every snapshot reads every file.
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
	"runtime"
	"sort"
	"strings"
	"sync"
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

// Cache carries what one store's last snapshot saw into its next one, so a
// snapshot opens and reads only the files whose identity changed. A Cache
// belongs to one store; pointed at another root it starts over. The zero value
// is an empty cache, and NewCache is how a caller says it means to keep one
// across snapshots.
type Cache struct {
	mu    sync.Mutex
	root  string
	files map[string]cached // by path relative to the root
	dirs  map[string]int64  // directory path -> modification time; "" is the root
}

// cached is one file as the last snapshot read it.
type cached struct {
	id    identity
	entry Entry
}

// identity is what a stat says about a file. Two stats with the same identity
// are the same bytes, up to the tick the package comment describes.
type identity struct {
	known bool // false: the OS did not say enough, so never reuse
	mode  fs.FileMode
	size  int64
	mtime int64 // nanoseconds
	ctime int64 // nanoseconds
	dev   uint64
	ino   uint64
	nlink uint64
}

func identify(info fs.FileInfo) identity {
	id := identity{mode: info.Mode(), size: info.Size(), mtime: info.ModTime().UnixNano()}
	fillIdentity(info, &id)
	return id
}

// NewCache returns an empty cache for one store.
func NewCache() *Cache { return &Cache{} }

// Snapshot reads the store under root. The root must be a directory; a missing
// root is an error, never an empty store, or a lane pointed at the wrong path
// would compare two empty trees and read as equal. Every snapshot reads every
// store file once; a Cache kept across snapshots of one store spares the reads
// its entries explain.
func Snapshot(root string) (*Snap, error) { return NewCache().Snapshot(root) }

// Snapshot reads the store under root, reusing what the cache still explains.
func (c *Cache) Snapshot(root string) (*Snap, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSnapshot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrSnapshot, root)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != root {
		c.root, c.files, c.dirs = root, nil, nil
	}
	snap := &Snap{Root: root, Dirs: make(map[string]Dir, len(c.dirs))}
	next := &Cache{root: root, files: make(map[string]cached, len(c.files)), dirs: make(map[string]int64, len(c.dirs))}
	rootMTime := info.ModTime().UnixNano()
	next.dirs[""] = rootMTime
	var subtrees []descent
	if _, _, err := c.walk(snap, next, root, "", nil, c.touched("", rootMTime), &subtrees); err != nil {
		return nil, err
	}
	if err := c.fanOut(snap, next, root, subtrees); err != nil {
		return nil, err
	}
	sort.Slice(snap.Entries, func(i, j int) bool { return snap.Entries[i].Path < snap.Entries[j].Path })
	c.files, c.dirs = next.files, next.dirs
	return snap, nil
}

// touched reports whether the directory rel was modified since the snapshot the
// cache holds, a directory the cache has never seen included. Every file
// directly inside such a directory is read again whatever its identity says: a
// key written through a temporary file and renamed over an existing one moves
// the directory's modification time even when the file keeps its length.
func (c *Cache) touched(rel string, mtime int64) bool {
	was, seen := c.dirs[rel]
	return !seen || was != mtime
}

// walk reads the directory rel, whose ancestors between the root and it (itself
// included, unless it is the root) have modes dirs, and whose own modification
// time moved since the cached snapshot if touched. It reports whether the
// directory holds nothing and whether it was gone by the time it was read.
func (c *Cache) walk(snap *Snap, next *Cache, root, rel string, dirs []fs.FileMode, touched bool, subtrees *[]descent) (empty, gone bool, err error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	// One listing: what the directory holds also says whether it is empty.
	children, err := os.ReadDir(full)
	if err != nil {
		if rel != "" && errors.Is(err, fs.ErrNotExist) {
			return false, true, nil // removed while the walk was under way
		}
		return false, false, fmt.Errorf("%w: %v", ErrSnapshot, err)
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
			return false, false, fmt.Errorf("%w: %v", ErrSnapshot, err)
		}
		mode := permissions(info.Mode())
		if info.IsDir() {
			child := descent{rel: childRel, mode: mode, mtime: info.ModTime().UnixNano(), dirs: dirs}
			if subtrees != nil {
				// At the root: left for fanOut, which walks the subtrees
				// side by side.
				*subtrees = append(*subtrees, child)
				continue
			}
			if err := c.descend(snap, next, root, child); err != nil {
				return false, false, err
			}
			continue
		}
		entry := Entry{Path: childRel, Mode: mode, Dirs: dirs}
		switch {
		case info.Mode().IsRegular():
			entry.Type = "file"
			id := identify(info)
			if was, ok := c.files[childRel]; ok && id.known && was.id == id && !touched && sameModes(was.entry.Dirs, dirs) {
				// Same file, same directories: the size, the hash and the
				// rendered text the last snapshot read all still hold.
				entry = was.entry
			} else {
				size, sum, err := hashFile(path)
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					return false, false, fmt.Errorf("%w: %v (the harness user must be able to read every stored file)", ErrSnapshot, err)
				}
				entry.Size, entry.SHA256 = size, sum
				entry = classify(entry)
			}
			next.files[childRel] = cached{id: id, entry: entry}
			snap.Entries = append(snap.Entries, entry)
			continue
		case info.Mode()&fs.ModeSymlink != 0:
			entry.Type = "symlink"
			if entry.Link, err = os.Readlink(path); err != nil {
				return false, false, fmt.Errorf("%w: %v", ErrSnapshot, err)
			}
		default:
			entry.Type = "other"
		}
		snap.Entries = append(snap.Entries, classify(entry))
	}
	return len(children) == 0, false, nil
}

// descent is one directory a walk has yet to read, as its parent saw it.
type descent struct {
	rel   string
	mode  fs.FileMode
	mtime int64
	dirs  []fs.FileMode // the modes above the directory, its own left out
}

// descend reads one directory into snap and next, and records what its parent
// saw of it: an empty directory that is not a key directory is an entry of its
// own, and one that was gone by the time it was read is nothing at all.
func (c *Cache) descend(snap *Snap, next *Cache, root string, d descent) error {
	dirs := append(append([]fs.FileMode(nil), d.dirs...), d.mode)
	empty, gone, err := c.walk(snap, next, root, d.rel, dirs, c.touched(d.rel, d.mtime), nil)
	if err != nil {
		return err
	}
	if gone {
		return nil
	}
	snap.Dirs[d.rel] = Dir{Mode: d.mode, MTime: d.mtime, Empty: empty}
	next.dirs[d.rel] = d.mtime
	if empty && !keyDirPosition(d.rel) {
		snap.Entries = append(snap.Entries, classify(Entry{Path: d.rel, Type: "dir", Mode: d.mode, Dirs: d.dirs}))
	}
	return nil
}

// workers is how many of the root's subtrees are read at once. A snapshot is
// almost nothing but system calls — open, getdents, lstat, and the reads of
// the few files a step changed — so the wall clock falls with the number of
// goroutines waiting on them, well past what the arithmetic would need.
var workers = min(runtime.GOMAXPROCS(0), 8)

// fanOut reads the root's subtrees side by side. Each one fills a snapshot and
// a cache of its own, so no two goroutines write one map; they are merged in
// the order the root listed them, and the entries are sorted by path
// afterwards, so the result does not depend on which subtree finished first.
// The cache being read is not written until every subtree is done.
func (c *Cache) fanOut(snap *Snap, next *Cache, root string, subtrees []descent) error {
	if len(subtrees) == 0 {
		return nil
	}
	subs := make([]*Snap, len(subtrees))
	nexts := make([]*Cache, len(subtrees))
	errs := make([]error, len(subtrees))
	running := min(workers, len(subtrees))
	var wg sync.WaitGroup
	for w := 0; w < running; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < len(subtrees); i += running {
				sub := &Snap{Root: root, Dirs: map[string]Dir{}}
				subNext := &Cache{root: root, files: map[string]cached{}, dirs: map[string]int64{}}
				errs[i] = c.descend(sub, subNext, root, subtrees[i])
				subs[i], nexts[i] = sub, subNext
			}
		}(w)
	}
	wg.Wait()
	for i := range subtrees {
		if errs[i] != nil {
			return errs[i]
		}
		snap.Entries = append(snap.Entries, subs[i].Entries...)
		for path, dir := range subs[i].Dirs {
			snap.Dirs[path] = dir
		}
		for path, file := range nexts[i].files {
			next.files[path] = file
		}
		for path, mtime := range nexts[i].dirs {
			next.dirs[path] = mtime
		}
	}
	return nil
}

// sameModes reports whether two directory-mode lists are equal.
func sameModes(a, b []fs.FileMode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
