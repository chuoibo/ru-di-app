package storage

// media_root() and pathlib's resolve(), ported from CPython 3.12 as the parity
// image ships it (pathlib.Path.expanduser, posixpath.expanduser,
// posixpath.realpath with strict=False, posixpath.abspath and normpath), so
// the root a Go process stores under is the root the Python process would.
//
// Go's own helpers differ where it shows: os.Getwd prefers $PWD, which may run
// through symlinks, where os.getcwd asks the kernel; filepath.EvalSymlinks
// fails on a missing component, where realpath keeps the rest as written; and
// neither knows pathlib's rule that a relative path whose first component
// starts with "~" is expanded after "." components are dropped.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// ErrNoHomeDirectory is the RuntimeError pathlib raises when ~ or ~user does
// not expand.
var ErrNoHomeDirectory = errors.New("Could not determine home directory.")

// SymlinkLoopError is the RuntimeError resolve() raises for a symlink loop.
type SymlinkLoopError struct{ Path string }

func (e *SymlinkLoopError) Error() string { return "Symlink loop from " + pyRepr(e.Path) }

// MediaRoot is media_root(): MOBILE_MEDIA_ROOT when the variable is set (an
// empty value is the working directory), otherwise ~/.local/share/rudi/media;
// then expanduser() and resolve().
func MediaRoot() (string, error) {
	var path pathParts
	if configured, ok := os.LookupEnv(MediaRootEnv); ok {
		path = parsePath(configured)
	} else {
		home, err := expanduser(parsePath("~"))
		if err != nil {
			return "", err
		}
		path = home.join(".local", "share", "rudi", "media")
	}
	expanded, err := expanduser(path)
	if err != nil {
		return "", err
	}
	return resolve(expanded)
}

// pathParts is a parsed PurePosixPath: its root ("", "/" or "//") and its
// components, with empty and "." components dropped.
type pathParts struct {
	root string
	tail []string
}

func parsePath(text string) pathParts {
	root := ""
	switch {
	case strings.HasPrefix(text, "//") && !strings.HasPrefix(text, "///"):
		root = "//"
	case strings.HasPrefix(text, "/"):
		root = "/"
	}
	var tail []string
	for _, part := range strings.Split(text, "/") {
		if part != "" && part != "." {
			tail = append(tail, part)
		}
	}
	return pathParts{root: root, tail: tail}
}

func (p pathParts) String() string {
	text := p.root + strings.Join(p.tail, "/")
	if text == "" {
		return "."
	}
	return text
}

func (p pathParts) join(parts ...string) pathParts {
	out := pathParts{root: p.root, tail: append(append([]string{}, p.tail...), parts...)}
	return parsePath(out.String())
}

// expanduser is pathlib.Path.expanduser.
func expanduser(p pathParts) (pathParts, error) {
	if p.root != "" || len(p.tail) == 0 || !strings.HasPrefix(p.tail[0], "~") {
		return p, nil
	}
	home := posixExpanduser(p.tail[0])
	if strings.HasPrefix(home, "~") {
		return pathParts{}, ErrNoHomeDirectory
	}
	parsed := parsePath(home)
	return pathParts{root: parsed.root, tail: append(parsed.tail, p.tail[1:]...)}, nil
}

// posixExpanduser is posixpath.expanduser for one component ("~" or
// "~name"): the component unchanged when the home cannot be found.
func posixExpanduser(component string) string {
	var home string
	if component == "~" {
		if value, ok := os.LookupEnv("HOME"); ok {
			home = value
		} else {
			account, err := user.LookupId(strconv.Itoa(os.Getuid()))
			if err != nil {
				return component
			}
			home = account.HomeDir
		}
	} else {
		account, err := user.Lookup(component[1:])
		if err != nil {
			return component
		}
		home = account.HomeDir
	}
	home = strings.TrimRight(home, "/")
	if home == "" {
		return "/"
	}
	return home
}

// resolve is pathlib.Path.resolve(strict=False): realpath, then a stat that
// turns ELOOP into SymlinkLoopError and ignores every other error.
func resolve(p pathParts) (string, error) {
	resolved, err := realpath(p.String())
	if err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno == syscall.ELOOP {
			return "", &SymlinkLoopError{Path: pathOf(err)}
		}
		return "", err
	}
	if _, err := os.Stat(resolved); err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno == syscall.ELOOP {
			return "", &SymlinkLoopError{Path: resolved}
		}
	}
	return resolved, nil
}

func pathOf(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Path
	}
	return ""
}

// realpath is posixpath.realpath(filename, strict=False).
func realpath(filename string) (string, error) {
	path, _, err := joinRealpath("", filename, map[string]*string{})
	if err != nil {
		return "", err
	}
	return abspath(path)
}

// joinRealpath is posixpath._joinrealpath with strict=False: lstat errors are
// "not a link", a loop returns what is resolved so far plus the rest as
// written, and os.readlink errors propagate.
func joinRealpath(path, rest string, seen map[string]*string) (string, bool, error) {
	if strings.HasPrefix(rest, "/") {
		rest = rest[1:]
		path = "/"
	}
	for rest != "" {
		var name string
		name, rest, _ = strings.Cut(rest, "/")
		if name == "" || name == "." {
			continue
		}
		if name == ".." {
			if path != "" {
				path, name = pySplit(path)
				if name == ".." {
					path = pyJoin(path, "..", "..")
				}
			} else {
				path = ".."
			}
			continue
		}
		newpath := pyJoin(path, name)
		info, err := os.Lstat(newpath)
		if err != nil || info.Mode()&fs.ModeSymlink == 0 {
			path = newpath
			continue
		}
		if cached, ok := seen[newpath]; ok {
			if cached != nil {
				path = *cached
				continue
			}
			return pyJoin(newpath, rest), false, nil
		}
		seen[newpath] = nil
		target, err := os.Readlink(newpath)
		if err != nil {
			return "", false, err
		}
		var resolved bool
		path, resolved, err = joinRealpath(path, target, seen)
		if err != nil {
			return "", false, err
		}
		if !resolved {
			return pyJoin(path, rest), false, nil
		}
		done := path
		seen[newpath] = &done
	}
	return path, true, nil
}

// abspath is posixpath.abspath, with the kernel's working directory.
func abspath(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		cwd, err := syscall.Getwd()
		if err != nil {
			return "", &fs.PathError{Op: "getcwd", Path: "", Err: err}
		}
		path = pyJoin(cwd, path)
	}
	return normpath(path), nil
}

// normpath is posixpath.normpath.
func normpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	var comps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == "..") {
			comps = append(comps, comp)
		} else if len(comps) > 0 {
			comps = comps[:len(comps)-1]
		}
	}
	out := strings.Repeat("/", initial) + strings.Join(comps, "/")
	if out == "" {
		return "."
	}
	return out
}

// pyJoin is posixpath.join.
func pyJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// pySplit is posixpath.split.
func pySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// pyRepr is repr() of a str decoded with the filesystem's surrogateescape:
// single quotes unless the text holds a single quote and no double quote,
// backslash escapes for the quote, backslash, \t \n \r, other controls and
// non-printable characters, and \udcXX for a byte that is not UTF-8.
func pyRepr(text string) string {
	quote := byte('\'')
	if strings.Contains(text, "'") && !strings.Contains(text, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, "\\udc%02x", text[i])
			i++
			continue
		}
		i += size
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < ' ' || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r < 0x7f || unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r <= 0xffff:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			fmt.Fprintf(&b, "\\U%08x", r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
