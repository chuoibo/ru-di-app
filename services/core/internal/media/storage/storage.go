// Package storage is app/media/storage.py: sanitized photos stored under
// opaque keys outside the repository (ADR-0029, W6).
//
// Python is the reference, down to the system calls. PhotoStorage.write is
// pathlib's mkdir(parents=True, exist_ok=True) on the key's directory, a
// tempfile.NamedTemporaryFile(dir=that directory, prefix=".<key>.",
// suffix=".tmp", delete=False) -- O_RDWR|O_CREAT|O_EXCL|O_NOFOLLOW, mode 0600,
// eight random characters from tempfile's alphabet -- the bytes, fsync, close,
// os.replace onto the key's path, and in every case an unlink of the temporary
// name that ignores ENOENT. read is the whole file; delete is one unlink(2)
// that answers whether a file was there.
//
// Consequences a caller must know, all Python's:
//   - a stored file is 0600 whatever the umask allows, because the temporary
//     file is created 0600 and rename keeps its mode; directories are
//     0777 &^ umask;
//   - the unlink in the cleanup runs after a successful replace too, and an
//     error from it other than ENOENT replaces whatever the write returned;
//   - Delete never removes a directory: unlink(2) of one is EISDIR;
//   - nothing removes empty key directories.
//
// Errors are the ones the Go standard library builds around the errno Python
// would raise with: *fs.PathError for a call that names one path (mkdir, open,
// unlink, read), *os.LinkError for the replace (Python's filename and
// filename2), a bare syscall.Errno for write, fsync and close (Python's
// OSError carries no filename there). errors.Is(err, fs.ErrNotExist) is
// exactly Python's FileNotFoundError (ENOENT). A key that is not 32 lowercase
// hex characters is ErrInvalidKey, Python's ValueError, before any call.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	mathrand "math/rand/v2"
	"os"
	"strings"
	"syscall"
)

// MediaRootEnv is MEDIA_ROOT_ENV.
const MediaRootEnv = "MOBILE_MEDIA_ROOT"

// ErrInvalidKey is the ValueError _path_for raises, with its text.
var ErrInvalidKey = errors.New("Storage keys must be exactly 32 lowercase hex characters.")

// validKey is `_STORAGE_KEY.fullmatch(key)` for `[0-9a-f]{32}`.
func validKey(key string) bool {
	if len(key) != 32 {
		return false
	}
	for i := 0; i < len(key); i++ {
		if c := key[i]; !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

// NewStorageKey is new_storage_key: secrets.token_hex(16).
func NewStorageKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// PhotoStorage is PhotoStorage. Root is absolute and already resolved.
type PhotoStorage struct {
	Root string
}

// New is PhotoStorage(): the root is media_root(), read from the environment,
// the home directory and the working directory at the time of the call.
func New() (*PhotoStorage, error) {
	root, err := MediaRoot()
	if err != nil {
		return nil, err
	}
	return &PhotoStorage{Root: root}, nil
}

// NewAt is PhotoStorage(root): pathlib.Path(root).resolve(), with no ~
// expansion. An empty root is the working directory, as Path("") is ".".
func NewAt(root string) (*PhotoStorage, error) {
	resolved, err := resolve(parsePath(root))
	if err != nil {
		return nil, err
	}
	return &PhotoStorage{Root: resolved}, nil
}

// PathFor is _path_for: root/key[:2]/key[2:4]/key, or ErrInvalidKey.
func (s *PhotoStorage) PathFor(key string) (string, error) {
	if !validKey(key) {
		return "", ErrInvalidKey
	}
	return joinUnder(s.Root, key[:2], key[2:4], key), nil
}

// joinUnder is pathlib's `/` on an absolute, normalised base: one separator
// between parts, none doubled after the filesystem root.
func joinUnder(base string, parts ...string) string {
	path := base
	for _, part := range parts {
		if strings.HasSuffix(path, "/") {
			path += part
		} else {
			path += "/" + part
		}
	}
	return path
}

// Write is write: see the package comment for the exact sequence.
func (s *PhotoStorage) Write(key string, data []byte) (err error) {
	path, err := s.PathFor(key)
	if err != nil {
		return err
	}
	if err := mkdir(parentOf(path), true, true); err != nil {
		return err
	}

	temporary := ""
	defer func() {
		if temporary == "" {
			return
		}
		// `temporary_path.unlink(missing_ok=True)` in a finally: an error
		// raised there is the one that propagates.
		if unlinkErr := system.Unlink(temporary); unlinkErr != nil && !errors.Is(unlinkErr, fs.ErrNotExist) {
			err = unlinkErr
		}
	}()

	file, name, err := createTemp(parentOf(path), "."+key+".", ".tmp")
	if err != nil {
		return err
	}
	temporary = name
	if len(data) > 0 {
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			return bareErrno(err)
		}
	}
	if err := system.Fsync(file); err != nil {
		_ = file.Close()
		return bareErrno(err)
	}
	if err := file.Close(); err != nil {
		return bareErrno(err)
	}
	return system.Replace(temporary, path)
}

// Read is read: the file's bytes. A directory at the key's path is EISDIR at
// open, as io.FileIO reports it.
func (s *PhotoStorage) Read(key string) ([]byte, error) {
	path, err := s.PathFor(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, bareErrno(err)
	}
	if info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: path, Err: syscall.EISDIR}
	}
	data, err := io.ReadAll(file)
	if err != nil {
		// FileIO.read raises OSError with no filename.
		return nil, bareErrno(err)
	}
	return data, nil
}

// Delete is delete: true when a file (or symlink) was unlinked, false for
// ENOENT, any other error as is.
func (s *PhotoStorage) Delete(key string) (bool, error) {
	path, err := s.PathFor(key)
	if err != nil {
		return false, err
	}
	if err := system.Unlink(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// mkdir is pathlib.Path.mkdir(mode=0o777, parents, exist_ok).
func mkdir(path string, parents, existOK bool) error {
	err := system.Mkdir(path, 0o777)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		parent := parentOf(path)
		if !parents || parent == path {
			return err
		}
		if err := mkdir(parent, true, true); err != nil {
			return err
		}
		return mkdir(path, false, existOK)
	}
	if !existOK {
		return err
	}
	isDir, statErr := isDirectory(path)
	if statErr != nil {
		return statErr
	}
	if !isDir {
		return err
	}
	return nil
}

// isDirectory is pathlib.Path.is_dir: stat following links; ENOENT, ENOTDIR,
// EBADF and ELOOP answer false, any other error is raised.
func isDirectory(path string) (bool, error) {
	info, err := system.Stat(path)
	if err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && (errno == syscall.ENOENT || errno == syscall.ENOTDIR ||
			errno == syscall.EBADF || errno == syscall.ELOOP) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// parentOf is pathlib's `.parent` of an absolute, normalised path.
func parentOf(path string) string {
	i := strings.LastIndex(path, "/")
	switch {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	}
	return path[:i]
}

// tmpMax is os.TMP_MAX on Linux, measured in the parity image: how many names
// _mkstemp_inner tries before giving up.
const tmpMax = 238328

// tempAlphabet is tempfile._RandomNameSequence.characters.
const tempAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789_"

// tempOpenFlags is tempfile._bin_openflags on Linux.
const tempOpenFlags = syscall.O_RDWR | syscall.O_CREAT | syscall.O_EXCL | syscall.O_NOFOLLOW

// ErrNoTemporaryName is the FileExistsError _mkstemp_inner raises after
// tmpMax names that all existed.
var ErrNoTemporaryName = &fs.PathError{Op: "mkstemp", Path: "", Err: syscall.EEXIST}

func randomName() string {
	var b [8]byte
	for i := range b {
		b[i] = tempAlphabet[mathrand.IntN(len(tempAlphabet))]
	}
	return string(b[:])
}

// createTemp is _mkstemp_inner(dir, prefix, suffix, _bin_openflags): the
// first of up to tmpMax candidate names that O_EXCL creates, 0600. EEXIST
// tries the next name; any other error is returned.
func createTemp(dir, prefix, suffix string) (*os.File, string, error) {
	dir = normpath(dir)
	for seq := 0; seq < tmpMax; seq++ {
		name := pyJoin(dir, prefix+randomName()+suffix)
		file, err := system.Open(name, tempOpenFlags, 0o600)
		if err == nil {
			return file, name, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return nil, "", err
	}
	return nil, "", ErrNoTemporaryName
}

// bareErrno drops the path Go attaches to a write, fsync or close error.
func bareErrno(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno
	}
	return err
}

// fileSystem is the handful of calls PhotoStorage makes that Python makes
// through the os module, one method per Python function, so the oracle can
// watch their order.
type fileSystem interface {
	Mkdir(path string, mode uint32) error
	Stat(path string) (fs.FileInfo, error)
	Open(path string, flag int, mode uint32) (*os.File, error)
	Fsync(file *os.File) error
	Replace(source, target string) error
	Unlink(path string) error
}

type osFileSystem struct{}

func (osFileSystem) Mkdir(path string, mode uint32) error {
	if err := syscall.Mkdir(path, mode); err != nil {
		return &fs.PathError{Op: "mkdir", Path: path, Err: err}
	}
	return nil
}

func (osFileSystem) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

func (osFileSystem) Open(path string, flag int, mode uint32) (*os.File, error) {
	return os.OpenFile(path, flag, fs.FileMode(mode))
}

func (osFileSystem) Fsync(file *os.File) error { return file.Sync() }

// Replace is os.replace: rename(2) itself. os.Rename would refuse a directory
// target with EEXIST before asking the kernel, which answers EISDIR.
func (osFileSystem) Replace(source, target string) error {
	if err := syscall.Rename(source, target); err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: err}
	}
	return nil
}

// Unlink is os.unlink: unlink(2) itself. os.Remove would fall back to rmdir and
// delete an empty directory Python refuses with EISDIR.
func (osFileSystem) Unlink(path string) error {
	if err := syscall.Unlink(path); err != nil {
		return &fs.PathError{Op: "unlink", Path: path, Err: err}
	}
	return nil
}

var system fileSystem = osFileSystem{}
