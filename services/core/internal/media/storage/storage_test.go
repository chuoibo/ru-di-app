package storage

// Checks that need neither Docker nor Python. The differential test against
// app/media/storage.py is storage_oracle_postgres_test.go.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func testStorage(t *testing.T) *PhotoStorage {
	t.Helper()
	s, err := NewAt(filepath.Join(t.TempDir(), "media"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testKey(t *testing.T) string {
	t.Helper()
	key, err := NewStorageKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestAKeyThatIsNotLowercaseHexIsTheValueError(t *testing.T) {
	s := testStorage(t)
	key := testKey(t)
	for _, bad := range []string{"", strings.ToUpper(key[:1]) + "F" + key[2:], key[:31], key + "a", "../" + key[3:], key[:31] + "\n"} {
		if _, err := s.PathFor(bad); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("PathFor(%q) = %v", bad, err)
		}
		if err := s.Write(bad, []byte("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Write(%q) = %v", bad, err)
		}
	}
	if ErrInvalidKey.Error() != "Storage keys must be exactly 32 lowercase hex characters." {
		t.Errorf("ValueError text drifted: %q", ErrInvalidKey)
	}
}

func TestTheLayoutIsTwoLevelsOfTheKeyItself(t *testing.T) {
	s := testStorage(t)
	key := testKey(t)
	path, err := s.PathFor(key)
	if err != nil {
		t.Fatal(err)
	}
	if want := s.Root + "/" + key[:2] + "/" + key[2:4] + "/" + key; path != want {
		t.Fatalf("PathFor = %s, want %s", path, want)
	}
}

func TestWriteStoresA0600FileAndLeavesNoTemporaryName(t *testing.T) {
	previous := syscall.Umask(0o022)
	defer syscall.Umask(previous)
	s := testStorage(t)
	key := testKey(t)
	if err := s.Write(key, []byte("anh mau")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read(key)
	if err != nil || string(got) != "anh mau" {
		t.Fatalf("Read = %q, %v", got, err)
	}
	path, _ := s.PathFor(key)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("stored file mode %v, %v", info.Mode(), err)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil || dir.Mode().Perm() != 0o755 {
		t.Fatalf("key directory mode %v, %v", dir.Mode(), err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != key {
		t.Fatalf("key directory holds %v, %v", entries, err)
	}
}

func TestAFailedReplaceUnlinksItsTemporaryFile(t *testing.T) {
	s := testStorage(t)
	key := testKey(t)
	path, _ := s.PathFor(key)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err := s.Write(key, []byte("x"))
	var link *os.LinkError
	if !errors.As(err, &link) || !errors.Is(err, syscall.EISDIR) || link.New != path {
		t.Fatalf("Write over a directory = %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("a temporary file was left: %v", entries)
	}
}

func TestDeleteAnswersWhetherAFileWasThereAndNeverRemovesADirectory(t *testing.T) {
	s := testStorage(t)
	key := testKey(t)
	if existed, err := s.Delete(key); existed || err != nil {
		t.Fatalf("Delete of nothing = %v, %v", existed, err)
	}
	if err := s.Write(key, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if existed, err := s.Delete(key); !existed || err != nil {
		t.Fatalf("Delete = %v, %v", existed, err)
	}
	path, _ := s.PathFor(key)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(key); !errors.Is(err, syscall.EISDIR) {
		t.Fatalf("Delete of an empty directory = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the directory went: %v", err)
	}
}

func TestReadOfAMissingKeyIsNotExist(t *testing.T) {
	s := testStorage(t)
	if _, err := s.Read(testKey(t)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Read = %v", err)
	}
}

type recordingFileSystem struct {
	fileSystem
	calls []string
}

func (r *recordingFileSystem) Mkdir(path string, mode uint32) error {
	r.calls = append(r.calls, "mkdir")
	return r.fileSystem.Mkdir(path, mode)
}
func (r *recordingFileSystem) Open(path string, flag int, mode uint32) (*os.File, error) {
	r.calls = append(r.calls, "open")
	return r.fileSystem.Open(path, flag, mode)
}
func (r *recordingFileSystem) Fsync(file *os.File) error {
	r.calls = append(r.calls, "fsync")
	return r.fileSystem.Fsync(file)
}
func (r *recordingFileSystem) Replace(source, target string) error {
	r.calls = append(r.calls, "replace")
	return r.fileSystem.Replace(source, target)
}
func (r *recordingFileSystem) Unlink(path string) error {
	r.calls = append(r.calls, "unlink")
	return r.fileSystem.Unlink(path)
}

func TestWriteSyncsBeforeItReplacesAndCleansUpAfter(t *testing.T) {
	s := testStorage(t)
	key := testKey(t)
	path, _ := s.PathFor(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	recording := &recordingFileSystem{fileSystem: system}
	previous := system
	system = recording
	defer func() { system = previous }()
	if err := s.Write(key, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(recording.calls, " "); got != "mkdir open fsync replace unlink" {
		t.Fatalf("calls: %s", got)
	}
}

func TestMediaRootReadsTheEnvironmentAndResolvesLinksBeforeDotDot(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "deep", "er"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "deep", "er"), filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	t.Setenv(MediaRootEnv, base+"/link/../m")
	if got, err := MediaRoot(); err != nil || got != base+"/deep/m" {
		t.Fatalf("MediaRoot = %q, %v", got, err)
	}
	t.Setenv(MediaRootEnv, "~/x")
	t.Setenv("HOME", base+"/")
	if got, err := MediaRoot(); err != nil || got != base+"/x" {
		t.Fatalf("MediaRoot with ~ = %q, %v", got, err)
	}
	os.Unsetenv(MediaRootEnv)
	if got, err := MediaRoot(); err != nil || got != base+"/.local/share/rudi/media" {
		t.Fatalf("default MediaRoot = %q, %v", got, err)
	}
	t.Setenv("HOME", "")
	if got, err := MediaRoot(); err != nil || got != "/.local/share/rudi/media" {
		t.Fatalf("MediaRoot with an empty HOME = %q, %v", got, err)
	}
}

func TestPosixpathHelpersMatchCPython(t *testing.T) {
	for in, want := range map[string]string{"": ".", "//a/../b": "//b", "///a//b/": "/a/b", "a/../../b": "../b",
		"/../a": "/a", "a/./b/.": "a/b"} {
		if got := normpath(in); got != want {
			t.Errorf("normpath(%q) = %q, want %q", in, got, want)
		}
	}
	if head, tail := pySplit("//x"); head != "//" || tail != "x" {
		t.Errorf("split(//x) = %q %q", head, tail)
	}
	if head, tail := pySplit("a/b//"); head != "a/b" || tail != "" {
		t.Errorf("split(a/b//) = %q %q", head, tail)
	}
	if got := pyJoin("a", "", "/b", "c"); got != "/b/c" {
		t.Errorf("join = %q", got)
	}
	for in, want := range map[string]string{"a'b": `"a'b"`, `a'"b`: `'a\'"b'`, "x\ty\x00": `'x\ty\x00'`,
		"á ": `'á\xa0'`, "\xff": `'\udcff'`} {
		if got := pyRepr(in); got != want {
			t.Errorf("pyRepr(%q) = %s, want %s", in, got, want)
		}
	}
}
