//go:build postgres

package storage

// Differential test of this package against app/media/storage.py, on a real
// filesystem, driven through scripts/render_photo_storage_oracle.py in the
// parity image. Nothing here opens a database: the build tag is `postgres`
// because scripts/go_postgres_tier.sh is the run that hands tests the image
// (CORE_PYTHON_IMAGE) and refuses a skip.
//
// One temporary directory per side: Python's is bind-mounted into the
// container at its own absolute path and written by the image's user under the
// image's umask; Go's is written by this process under the same umask (or the
// scenario's). Every scenario seeds identical files in its own subdirectory
// and runs identical steps, and after each step both sides report the result
// or the exception, the os calls write/read/delete made in order (Python's
// through wrappers around the os module, Go's through the fileSystem seam), and
// the whole file tree with kinds, sizes, permission bits and link targets.
//
// Payloads are drawn from crypto/rand when the test runs, keys from
// NewStorageKey; neither is committed anywhere.
//
// Beyond the comparison the test requires the corpus to reach what it is
// written for: each exception class it names, both delete answers, an fsync
// and a replace in a trace, and a stored 0600 file.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	osexec "os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

type setupOp struct {
	Op     string `json:"op"`
	Path   string `json:"path"`
	Mode   int    `json:"mode"`
	Hex    string `json:"hex"`
	Target string `json:"target,omitempty"`
}

type stepOp struct {
	Op   string             `json:"op"`
	Key  string             `json:"key"`
	Hex  string             `json:"hex"`
	Root *string            `json:"root,omitempty"`
	Env  map[string]*string `json:"env,omitempty"`
	Cwd  string             `json:"cwd,omitempty"`
}

type scenario struct {
	Name  string    `json:"name"`
	Umask *int      `json:"umask"`
	Setup []setupOp `json:"setup"`
	Steps []stepOp  `json:"steps"`
}

type pythonStorageRun struct {
	DefaultUmask int      `json:"default_umask"`
	Keys         []string `json:"keys"`
	Scenarios    []struct {
		Name  string `json:"name"`
		Steps []any  `json:"steps"`
	} `json:"scenarios"`
}

func text(s string) *string { return &s }

func octal(n int) *int { return &n }

func payload(t *testing.T, size int) string {
	t.Helper()
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func freshKey(t *testing.T) string {
	t.Helper()
	key, err := NewStorageKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// storageScenarios is the corpus. Every scenario but the root ones starts by
// opening storage at {base}/media.
func storageScenarios(t *testing.T) []scenario {
	k1, k2, k3, k4, k5 := freshKey(t), freshKey(t), freshKey(t), freshKey(t), freshKey(t)
	sibling := k3[:4] + freshKey(t)[4:]
	// neighbour is another key in key's directory, never key itself.
	neighbour := func(key string) string {
		if strings.HasSuffix(key, "0") {
			return key[:31] + "1"
		}
		return key[:31] + "0"
	}
	media := text("{base}/media")
	open := stepOp{Op: "storage", Root: media}
	write := func(key string, size int) stepOp { return stepOp{Op: "write", Key: key, Hex: payload(t, size)} }
	read := func(key string) stepOp { return stepOp{Op: "read", Key: key} }
	del := func(key string) stepOp { return stepOp{Op: "delete", Key: key} }
	path := func(key string) stepOp { return stepOp{Op: "path", Key: key} }
	keyDir := func(key string) string { return "media/" + key[:2] + "/" + key[2:4] }
	dirs := func(key string, mode int) []setupOp {
		return []setupOp{{Op: "mkdir", Path: "media", Mode: 0o755}, {Op: "mkdir", Path: "media/" + key[:2], Mode: 0o755},
			{Op: "mkdir", Path: keyDir(key), Mode: mode}}
	}
	root := func(value string, env map[string]*string) stepOp {
		return stepOp{Op: "storage", Root: text(value), Env: env, Cwd: "cwd"}
	}
	mediaRoot := func(env map[string]*string) stepOp { return stepOp{Op: "media_root", Env: env, Cwd: "cwd"} }
	env := func(pairs ...any) map[string]*string {
		out := map[string]*string{}
		for i := 0; i < len(pairs); i += 2 {
			value, _ := pairs[i+1].(string)
			if pairs[i+1] == nil {
				out[pairs[i].(string)] = nil
			} else {
				out[pairs[i].(string)] = text(value)
			}
		}
		return out
	}
	rootWorld := []setupOp{{Op: "mkdir", Path: "cwd", Mode: 0o755}, {Op: "mkdir", Path: "deep", Mode: 0o755},
		{Op: "mkdir", Path: "deep/er", Mode: 0o755}, {Op: "symlink", Path: "link", Target: "{base}/deep/er"},
		{Op: "symlink", Path: "loop", Target: "{base}/loop"}, {Op: "symlink", Path: "dangling", Target: "{base}/target/media"},
		{Op: "mkdir", Path: "h", Mode: 0o755}, {Op: "mkdir", Path: "elsewhere", Mode: 0o755},
		{Op: "symlink", Path: "h/.local", Target: "../elsewhere"}, {Op: "symlink", Path: "cwd/relative", Target: "../deep"}}
	upper := strings.ToUpper(k1[:1]) + k1[1:]
	if upper == k1 {
		upper = "A" + k1[1:]
	}
	badKeys := []string{"", upper, k1[:31], k1 + "0", "../../etc/passwd", k1[:31] + "\n", k1[:31] + "g", "0x" + k1[:30],
		strings.Repeat("٠", 32), " " + k1[:31], k1[:16] + "/" + k1[17:]}

	var out []scenario
	add := func(name string, umask *int, setup []setupOp, steps ...stepOp) {
		out = append(out, scenario{Name: name, Umask: umask, Setup: append([]setupOp{}, setup...), Steps: steps})
	}

	add("write, read back, overwrite with a larger payload, delete twice", nil, nil,
		open, path(k1), write(k1, 1024), read(k1), write(k1, 3<<20+17), read(k1), del(k1), del(k1), read(k1))
	add("an empty payload is stored and read back empty", nil, nil, open, write(k2, 0), read(k2), del(k2))
	add("two keys share a directory, a third does not, delete keeps directories", nil, nil,
		open, write(k3, 1), write(sibling, 2), write(k4, 3), del(k3), read(sibling))
	var bad []stepOp
	for _, key := range badKeys {
		bad = append(bad, path(key), write(key, 4), read(key), del(key))
	}
	add("keys that are not 32 lowercase hex characters", nil, nil, append([]stepOp{open}, bad...)...)
	add("an empty directory at the key's path", nil, append(dirs(k1, 0o755), setupOp{Op: "mkdir", Path: keyDir(k1) + "/" + k1, Mode: 0o755}),
		open, write(k1, 8), read(k1), del(k1))
	add("a regular file where a key directory goes", nil, []setupOp{{Op: "mkdir", Path: "media", Mode: 0o755},
		{Op: "file", Path: "media/" + k2[:2], Hex: payload(t, 5), Mode: 0o644}},
		open, write(k2, 8), read(k2), del(k2))
	add("a read-only key directory", nil, append(dirs(k3, 0o755),
		setupOp{Op: "file", Path: keyDir(k3) + "/" + k3, Hex: payload(t, 7), Mode: 0o644},
		setupOp{Op: "chmod", Path: keyDir(k3), Mode: 0o555}),
		open, write(k3, 8), read(k3), del(k3), write(neighbour(k3), 8))
	add("an unreadable stored file is still deleted", nil, append(dirs(k4, 0o755),
		setupOp{Op: "file", Path: keyDir(k4) + "/" + k4, Hex: payload(t, 9), Mode: 0o000}),
		open, read(k4), del(k4), del(k4))
	add("a symlink at the key's path, and one to nothing", nil, append(dirs(k5, 0o755),
		setupOp{Op: "mkdir", Path: "elsewhere", Mode: 0o755},
		setupOp{Op: "file", Path: "elsewhere/target", Hex: payload(t, 11), Mode: 0o644},
		setupOp{Op: "symlink", Path: keyDir(k5) + "/" + k5, Target: "{base}/elsewhere/target"},
		setupOp{Op: "symlink", Path: keyDir(k5) + "/" + neighbour(k5), Target: "{base}/nothing"}),
		open, read(k5), write(k5, 13), read(k5), read(neighbour(k5)), del(neighbour(k5)), del(neighbour(k5)))
	add("a wider old file is replaced by a 0600 one and a stale temporary file stays", nil, append(dirs(k1, 0o755),
		setupOp{Op: "file", Path: keyDir(k1) + "/" + k1, Hex: payload(t, 3), Mode: 0o644},
		setupOp{Op: "file", Path: keyDir(k1) + "/." + k1 + ".cu_mau01.tmp", Hex: payload(t, 2), Mode: 0o600}),
		open, write(k1, 21), read(k1))
	for _, mask := range []int{0o022, 0o077, 0o000, 0o027, 0o277, 0o700} {
		add(fmt.Sprintf("umask %#o: a fresh root, then a second key", mask), octal(mask), nil,
			open, write(k2, 5), write(k3, 6), read(k2))
	}
	add("a root three levels from anything that exists", nil, nil,
		stepOp{Op: "storage", Root: text("{base}/a/b/media")}, write(k4, 4), read(k4))

	add("explicit roots: empty, relative, through a link and .., dangling, a loop, a tilde", nil, rootWorld,
		root("", nil), root("rel/media", nil), root("{base}/link/../media", nil), root("relative/er/../x", nil),
		root("~", nil), root("{base}/loop", nil), root("{base}/loop/media", nil),
		root("//"+"{base}/media", nil), root("{base}/dangling", nil), write(k5, 6), read(k5))

	home := env("HOME", "{base}/h")
	add("media_root from MOBILE_MEDIA_ROOT", nil, rootWorld,
		mediaRoot(env(MediaRootEnv, "{base}/m")), mediaRoot(env(MediaRootEnv, "")), mediaRoot(env(MediaRootEnv, "rel")),
		mediaRoot(env(MediaRootEnv, "~", "HOME", "{base}/h")), mediaRoot(env(MediaRootEnv, "~/x", "HOME", "{base}/h/")),
		mediaRoot(env(MediaRootEnv, "./~/x", "HOME", "{base}/h")), mediaRoot(env(MediaRootEnv, ".//~//x/.", "HOME", "{base}/h")),
		mediaRoot(env(MediaRootEnv, "~root/x")), mediaRoot(env(MediaRootEnv, "~khong-co-nguoi-nay/x")),
		mediaRoot(env(MediaRootEnv, "x/~")), mediaRoot(env(MediaRootEnv, "{base}/link/../m")),
		mediaRoot(env(MediaRootEnv, "{base}/loop/m")), mediaRoot(env(MediaRootEnv, "{base}/h/.local/share")),
		mediaRoot(env(MediaRootEnv, "~", "HOME", "")), mediaRoot(env(MediaRootEnv, "~", "HOME", nil)))
	add("media_root from the home directory", nil, rootWorld,
		mediaRoot(env(MediaRootEnv, nil, "HOME", "{base}/h")), mediaRoot(env(MediaRootEnv, nil, "HOME", "")),
		mediaRoot(env(MediaRootEnv, nil, "HOME", nil)), mediaRoot(env(MediaRootEnv, nil, "HOME", "relhome")),
		mediaRoot(env(MediaRootEnv, nil, "HOME", "./~khong-co-nguoi-nay")), mediaRoot(env(MediaRootEnv, nil, "HOME", "~")),
		mediaRoot(env(MediaRootEnv, nil, "HOME", "/")), mediaRoot(env(MediaRootEnv, nil, "HOME", "{base}/link/..")),
		stepOp{Op: "storage", Env: home, Cwd: "cwd"}, write(k1, 3), read(k1))
	return out
}

// ---------------------------------------------------------------------------
// The Go side
// ---------------------------------------------------------------------------

var temporaryName = regexp.MustCompile(`(\.[0-9a-f]{32}\.)[a-z0-9_]{8}(\.tmp)(/|$)`)

type goSide struct {
	dir  string
	home string
}

func (g *goSide) rel(path string) string {
	switch {
	case path == g.dir:
		path = "{base}"
	case strings.HasPrefix(path, g.dir+"/"):
		path = "{base}" + path[len(g.dir):]
	case g.home != "" && (path == g.home || strings.HasPrefix(path, g.home+"/")):
		path = "{home}" + path[len(g.home):]
	}
	return temporaryName.ReplaceAllString(path, "${1}<random>${2}${3}")
}

type tracingFileSystem struct {
	inner fileSystem
	side  *goSide
	log   []any
}

func (f *tracingFileSystem) Mkdir(path string, mode uint32) error {
	f.log = append(f.log, []any{"mkdir", f.side.rel(path), fmt.Sprintf("0o%o", mode)})
	return f.inner.Mkdir(path, mode)
}

func (f *tracingFileSystem) Stat(path string) (fs.FileInfo, error) {
	f.log = append(f.log, []any{"stat", f.side.rel(path)})
	return f.inner.Stat(path)
}

func (f *tracingFileSystem) Open(path string, flag int, mode uint32) (*os.File, error) {
	f.log = append(f.log, []any{"open", f.side.rel(path), flag, fmt.Sprintf("0o%o", mode)})
	return f.inner.Open(path, flag, mode)
}

func (f *tracingFileSystem) Fsync(file *os.File) error {
	f.log = append(f.log, []any{"fsync", f.side.rel(file.Name())})
	return f.inner.Fsync(file)
}

func (f *tracingFileSystem) Replace(source, target string) error {
	f.log = append(f.log, []any{"replace", f.side.rel(source), f.side.rel(target)})
	return f.inner.Replace(source, target)
}

func (f *tracingFileSystem) Unlink(path string) error {
	f.log = append(f.log, []any{"unlink", f.side.rel(path)})
	return f.inner.Unlink(path)
}

// pythonOSErrorClass is the OSError subclass CPython raises for an errno.
func pythonOSErrorClass(errno syscall.Errno) string {
	switch errno {
	case syscall.EAGAIN, syscall.EALREADY, syscall.EINPROGRESS:
		return "BlockingIOError"
	case syscall.ECHILD:
		return "ChildProcessError"
	case syscall.EPIPE, syscall.ESHUTDOWN:
		return "BrokenPipeError"
	case syscall.ECONNABORTED:
		return "ConnectionAbortedError"
	case syscall.ECONNREFUSED:
		return "ConnectionRefusedError"
	case syscall.ECONNRESET:
		return "ConnectionResetError"
	case syscall.EEXIST:
		return "FileExistsError"
	case syscall.ENOENT:
		return "FileNotFoundError"
	case syscall.EISDIR:
		return "IsADirectoryError"
	case syscall.ENOTDIR:
		return "NotADirectoryError"
	case syscall.EINTR:
		return "InterruptedError"
	case syscall.EACCES, syscall.EPERM:
		return "PermissionError"
	case syscall.ESRCH:
		return "ProcessLookupError"
	case syscall.ETIMEDOUT:
		return "TimeoutError"
	}
	return "OSError"
}

func (g *goSide) pythonError(err error) map[string]any {
	out := map[string]any{"type": nil, "errno": nil, "filename": nil, "filename2": nil, "message": nil}
	var loop *SymlinkLoopError
	var errno syscall.Errno
	var link *os.LinkError
	var pathErr *fs.PathError
	switch {
	case errors.Is(err, ErrInvalidKey), errors.Is(err, ErrNoHomeDirectory):
		out["type"], out["message"] = map[bool]string{true: "ValueError", false: "RuntimeError"}[errors.Is(err, ErrInvalidKey)], err.Error()
	case errors.As(err, &loop):
		out["type"], out["message"] = "RuntimeError", strings.ReplaceAll(err.Error(), g.dir, "{base}")
	case errors.As(err, &errno):
		out["type"], out["errno"] = pythonOSErrorClass(errno), int(errno)
		switch {
		case errors.As(err, &link):
			out["filename"], out["filename2"] = g.rel(link.Old), g.rel(link.New)
		case errors.As(err, &pathErr):
			out["filename"] = g.rel(pathErr.Path)
		}
	default:
		out["type"] = "go error: " + err.Error()
	}
	return out
}

func (g *goSide) tree(t *testing.T) []any {
	t.Helper()
	rows := [][]any{}
	err := filepath.WalkDir(g.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == g.dir {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		mode := info.Sys().(*syscall.Stat_t).Mode
		kind, size, target := "?", any(nil), any(nil)
		switch mode & syscall.S_IFMT {
		case syscall.S_IFDIR:
			kind = "d"
		case syscall.S_IFLNK:
			kind = "l"
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			target = g.rel(link)
		case syscall.S_IFREG:
			kind, size = "f", info.Size()
		}
		rows = append(rows, []any{g.rel(path), kind, size, fmt.Sprintf("0o%o", mode&0o7777), target})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(rows, func(a, b int) bool { return rows[a][0].(string) < rows[b][0].(string) })
	out := make([]any, len(rows))
	for i, row := range rows {
		out[i] = row
	}
	return out
}

func (g *goSide) sub(value string) string { return strings.ReplaceAll(value, "{base}", g.dir) }

func (g *goSide) setup(t *testing.T, op setupOp) {
	t.Helper()
	path := filepath.Join(g.dir, op.Path)
	var err error
	switch op.Op {
	case "mkdir":
		if err = os.Mkdir(path, 0o700); err == nil {
			err = os.Chmod(path, fs.FileMode(op.Mode))
		}
	case "file":
		var content []byte
		if content, err = hex.DecodeString(op.Hex); err == nil {
			if err = os.WriteFile(path, content, 0o600); err == nil {
				err = os.Chmod(path, fs.FileMode(op.Mode))
			}
		}
	case "chmod":
		err = os.Chmod(path, fs.FileMode(op.Mode))
	case "symlink":
		err = os.Symlink(g.sub(op.Target), path)
	default:
		err = errors.New("unknown setup op " + op.Op)
	}
	if err != nil {
		t.Fatalf("setup %+v: %v", op, err)
	}
}

func (g *goSide) step(t *testing.T, op stepOp, storage *PhotoStorage) (map[string]any, *PhotoStorage) {
	t.Helper()
	record := map[string]any{"result": nil, "error": nil, "trace": []any{}, "tree": nil}
	restore := map[string]*string{}
	for name, value := range op.Env {
		if previous, ok := os.LookupEnv(name); ok {
			restore[name] = text(previous)
		} else {
			restore[name] = nil
		}
		if value == nil {
			os.Unsetenv(name)
		} else {
			os.Setenv(name, g.sub(*value))
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if op.Cwd != "" {
		if err := os.Chdir(filepath.Join(g.dir, op.Cwd)); err != nil {
			t.Fatal(err)
		}
	}
	tracer := &tracingFileSystem{inner: system, side: g, log: []any{}}
	var callErr error
	func() {
		if op.Op == "write" || op.Op == "read" || op.Op == "delete" {
			previous := system
			system = tracer
			defer func() { system = previous }()
		}
		switch op.Op {
		case "storage":
			// A refused construction leaves the previous storage in place, as
			// Python's assignment that never happens does.
			var opened *PhotoStorage
			if op.Root == nil {
				opened, callErr = New()
			} else {
				opened, callErr = NewAt(g.sub(*op.Root))
			}
			if callErr == nil {
				storage = opened
				record["result"] = g.rel(storage.Root)
			}
		case "media_root":
			var root string
			if root, callErr = MediaRoot(); callErr == nil {
				record["result"] = g.rel(root)
			}
		case "path":
			var path string
			if path, callErr = storage.PathFor(op.Key); callErr == nil {
				record["result"] = g.rel(path)
			}
		case "write":
			content, err := hex.DecodeString(op.Hex)
			if err != nil {
				t.Fatal(err)
			}
			callErr = storage.Write(op.Key, content)
		case "read":
			var content []byte
			if content, callErr = storage.Read(op.Key); callErr == nil {
				sum := sha256Hex(content)
				record["result"] = map[string]any{"size": len(content), "sha256": sum}
			}
		case "delete":
			var existed bool
			if existed, callErr = storage.Delete(op.Key); callErr == nil {
				record["result"] = existed
			}
		default:
			t.Fatalf("unknown step %s", op.Op)
		}
	}()
	if callErr != nil {
		record["error"] = g.pythonError(callErr)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	for name, value := range restore {
		if value == nil {
			os.Unsetenv(name)
		} else {
			os.Setenv(name, *value)
		}
	}
	record["trace"] = tracer.log
	record["tree"] = g.tree(t)
	return record, storage
}

func runGoScenarios(t *testing.T, base string, defaultUmask int, scenarios []scenario) [][]any {
	t.Helper()
	account, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	out := make([][]any, len(scenarios))
	for i, sc := range scenarios {
		side := &goSide{dir: filepath.Join(base, strconv.Itoa(i)), home: account.HomeDir}
		if err := os.Mkdir(side.dir, 0o777); err != nil {
			t.Fatal(err)
		}
		for _, op := range sc.Setup {
			side.setup(t, op)
		}
		mask := defaultUmask
		if sc.Umask != nil {
			mask = *sc.Umask
		}
		previous := syscall.Umask(mask)
		var storage *PhotoStorage
		steps := []any{}
		for _, op := range sc.Steps {
			var record map[string]any
			record, storage = side.step(t, op, storage)
			steps = append(steps, record)
		}
		syscall.Umask(previous)
		out[i] = steps
	}
	return out
}

// makeRemovable gives the owner rwx on every directory under dir, so a
// read-only or 0500 directory a scenario left does not stop its removal.
func makeRemovable(dir string) {
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			_ = os.Chmod(path, 0o700)
		}
		return nil
	})
}

const pythonCleanup = `import os, shutil, sys
base = sys.argv[1]
for dirpath, dirnames, _ in os.walk(base):
    for name in dirnames:
        path = os.path.join(dirpath, name)
        if not os.path.islink(path):
            os.chmod(path, 0o700)
for name in os.listdir(base):
    path = os.path.join(base, name)
    if os.path.isdir(path) and not os.path.islink(path):
        shutil.rmtree(path)
    else:
        os.unlink(path)
`

func scriptsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "scripts", "render_photo_storage_oracle.py")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(dir, "scripts")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("scripts/render_photo_storage_oracle.py not found above the package")
		}
		dir = parent
	}
}

func generic(t *testing.T, v any) any {
	t.Helper()
	payload, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func clip(v any) string {
	payload, _ := json.Marshal(v)
	if len(payload) > 3000 {
		return string(payload[:3000]) + "..."
	}
	return string(payload)
}

var storageKeyShape = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestPhotoStorageOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of storage_oracle_postgres_test.go")
	}
	scripts := scriptsDir(t)
	base := t.TempDir()
	pyBase, goBase := filepath.Join(base, "py"), filepath.Join(base, "go")
	for _, dir := range []string{pyBase, goBase} {
		if err := os.Mkdir(dir, 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o777); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { makeRemovable(goBase) })
	t.Cleanup(func() {
		cleanup := osexec.Command("docker", "run", "--rm", "-v", pyBase+":"+pyBase, "--entrypoint", "python", image,
			"-c", pythonCleanup, pyBase)
		if out, err := cleanup.CombinedOutput(); err != nil {
			t.Errorf("cleaning the python side: %v\n%s", err, out)
		}
	})

	scenarios := storageScenarios(t)
	spec, err := json.Marshal(map[string]any{"base": pyBase, "scenarios": scenarios})
	if err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "-v", pyBase+":"+pyBase, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_photo_storage_oracle.py")
	driver.Stdin = bytes.NewReader(spec)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_photo_storage_oracle.py: %v\n%s", err, stderr.String())
	}
	var python pythonStorageRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Scenarios) != len(scenarios) {
		t.Fatalf("python answered %d scenarios of %d", len(python.Scenarios), len(scenarios))
	}

	golang := runGoScenarios(t, goBase, python.DefaultUmask, scenarios)

	var steps, results, refusals, traced, rows, mismatches int
	seen := map[string]int{}
	for i, sc := range scenarios {
		if python.Scenarios[i].Name != sc.Name {
			t.Fatalf("scenario %d is %q in python", i, python.Scenarios[i].Name)
		}
		t.Run(sc.Name, func(t *testing.T) {
			py, gs := python.Scenarios[i].Steps, generic(t, golang[i]).([]any)
			if len(py) != len(gs) || len(py) != len(sc.Steps) {
				mismatches++
				t.Fatalf("python ran %d steps, go %d, of %d", len(py), len(gs), len(sc.Steps))
			}
			for j := range py {
				p, g := py[j].(map[string]any), gs[j].(map[string]any)
				steps++
				if p["error"] != nil {
					refusals++
					seen["error "+p["error"].(map[string]any)["type"].(string)]++
				} else {
					results++
				}
				if sc.Steps[j].Op == "delete" && p["error"] == nil {
					seen[fmt.Sprintf("delete %v", p["result"])]++
				}
				trace, _ := p["trace"].([]any)
				traced += len(trace)
				for _, call := range trace {
					seen["trace "+call.([]any)[0].(string)]++
				}
				tree, _ := p["tree"].([]any)
				rows += len(tree)
				for _, row := range tree {
					if cells := row.([]any); cells[1] == "f" && cells[3] == "0o600" {
						seen["file 0o600"]++
					}
				}
				for _, key := range []string{"result", "error", "trace", "tree"} {
					if !reflect.DeepEqual(p[key], g[key]) {
						mismatches++
						t.Errorf("step %d %s %.40q: %s differs\n  python: %s\n  go:     %s", j, sc.Steps[j].Op, sc.Steps[j].Key,
							key, clip(p[key]), clip(g[key]))
					}
				}
			}
		})
	}
	for _, want := range []string{"error ValueError", "error RuntimeError", "error FileNotFoundError",
		"error IsADirectoryError", "error NotADirectoryError", "error PermissionError", "delete true", "delete false",
		"trace mkdir", "trace stat", "trace open", "trace fsync", "trace replace", "trace unlink", "file 0o600"} {
		if seen[want] == 0 {
			t.Errorf("no python step reaches %q", want)
		}
	}
	distinct := map[string]bool{}
	for _, key := range python.Keys {
		distinct[key] = true
		if !storageKeyShape.MatchString(key) {
			t.Errorf("python storage key %q is not 32 lowercase hex", key)
		}
	}
	for i := 0; i < len(python.Keys); i++ {
		key := freshKey(t)
		distinct[key] = true
		if !storageKeyShape.MatchString(key) {
			t.Errorf("go storage key %q is not 32 lowercase hex", key)
		}
	}
	if len(distinct) != 2*len(python.Keys) {
		t.Errorf("%d distinct keys of %d", len(distinct), 2*len(python.Keys))
	}
	t.Logf("photo storage oracle: %d scenarios, %d steps (%d results, %d refusals), %d traced calls, %d tree rows, "+
		"image umask %#o, %d mismatches", len(scenarios), steps, results, refusals, traced, rows, python.DefaultUmask, mismatches)
}
