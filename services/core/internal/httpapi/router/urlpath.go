package router

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrURLPath is the ValueError (or UnicodeDecodeError) Starlette raises while
// rebuilding request.url. Raised inside an exception handler, it is an
// unhandled exception: a plain 500.
var ErrURLPath = errors.New("router: request.url cannot be parsed")

// URLPath is request.url.path in the pinned image (starlette 0.41.3 on
// CPython 3.12.14), which is not scope["path"]: URL(scope=scope) builds
// "http://" + the Host header decoded as latin-1 + the scope path, appends
// "?" + the query string decoded as UTF-8, and reads the path back through
// urllib.parse.urlsplit. host is the Host header ("" for none), path the
// scope path, rawQuery the raw query string.
//
// A Host-less request builds its URL from the server address instead; that
// URL and "http://" + "" + path both split to the scope path, so "" stands
// for either. The scheme cannot change the path: every scheme uvicorn accepts
// from X-Forwarded-Proto is split off the same way.
//
// testdata/starlette_url_paths.json pins this against the real libraries
// (scripts/render_url_path_goldens.py).
func URLPath(host, path, rawQuery string) (string, error) {
	url := "http://" + latin1(host) + path
	if rawQuery != "" {
		if !utf8.ValidString(rawQuery) {
			return "", ErrURLPath
		}
		url += "?" + rawQuery
	}
	return urlsplitPath(url)
}

// urlsplitPath is urlsplit(url).path for a url that starts "http://".
func urlsplitPath(url string) (string, error) {
	url = strings.TrimLeft(url, whatwgC0ControlOrSpace)
	for _, unsafe := range []string{"\t", "\r", "\n"} {
		url = strings.ReplaceAll(url, unsafe, "")
	}
	if i := strings.IndexByte(url, ':'); i > 0 && isASCIIAlpha(url[0]) && strings.Trim(url[:i], schemeChars) == "" {
		url = url[i+1:]
	}
	if strings.HasPrefix(url, "//") {
		end := len(url)
		if i := strings.IndexAny(url[2:], "/?#"); i >= 0 {
			end = 2 + i
		}
		netloc := url[2:end]
		url = url[end:]
		open, closed := strings.Contains(netloc, "["), strings.Contains(netloc, "]")
		if open != closed {
			return "", ErrURLPath
		}
		if open && !bracketedNetlocOK(netloc) {
			return "", ErrURLPath
		}
		// _checknetloc cannot refuse here: the netloc is latin-1 text (the
		// Host header), and no latin-1 character normalises under NFKC to one
		// of "/?#@:", which the render script asserts.
	}
	if i := strings.IndexByte(url, '#'); i >= 0 {
		url = url[:i]
	}
	if i := strings.IndexByte(url, '?'); i >= 0 {
		url = url[:i]
	}
	return url, nil
}

const (
	whatwgC0ControlOrSpace = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f "
	schemeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+-."
)

func isASCIIAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// bracketedNetlocOK is _check_bracketed_netloc not raising.
func bracketedNetlocOK(netloc string) bool {
	hostAndPort := netloc
	if i := strings.LastIndexByte(netloc, '@'); i >= 0 {
		hostAndPort = netloc[i+1:]
	}
	var hostname string
	if before, bracketed, found := strings.Cut(hostAndPort, "["); found {
		if before != "" {
			return false
		}
		var port string
		hostname, port, _ = strings.Cut(bracketed, "]")
		if port != "" && !strings.HasPrefix(port, ":") {
			return false
		}
	} else {
		hostname, _, _ = strings.Cut(hostAndPort, ":")
	}
	return bracketedHostOK(hostname)
}

// bracketedHostOK is _check_bracketed_host not raising: an IPvFuture literal,
// or text ipaddress.ip_address reads as IPv6. The IPv4 refusal needs no test
// of its own: an IPv4 address has no colon, so it is never IPv6 either.
func bracketedHostOK(hostname string) bool {
	if strings.HasPrefix(hostname, "v") {
		// re.match(r"\Av[a-fA-F0-9]+\..+\Z", hostname); no newline is left.
		rest := hostname[1:]
		n := 0
		for n < len(rest) && isHexDigit(rest[n]) {
			n++
		}
		return n > 0 && n+1 < len(rest) && rest[n] == '.'
	}
	return parseIPv6(hostname)
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// parseIPv4 is IPv4Address(text) not raising.
func parseIPv4(text string) bool {
	if text == "" || strings.Contains(text, "/") {
		return false
	}
	octets := strings.Split(text, ".")
	if len(octets) != 4 {
		return false
	}
	for _, octet := range octets {
		if octet == "" || len(octet) > 3 {
			return false
		}
		value := 0
		for i := 0; i < len(octet); i++ {
			if octet[i] < '0' || octet[i] > '9' {
				return false
			}
			value = value*10 + int(octet[i]-'0')
		}
		if octet != "0" && octet[0] == '0' || value > 255 {
			return false
		}
	}
	return true
}

// parseIPv6 is IPv6Address(text) not raising: an optional non-empty scope id
// after the first "%", then at most 45 characters of hextets and colons with
// an optional IPv4 tail. Lengths are code points in Python; any non-ASCII
// character already makes the text invalid, so bytes give the same answers.
func parseIPv6(text string) bool {
	if strings.Contains(text, "/") {
		return false
	}
	addr, scope, scoped := strings.Cut(text, "%")
	if scoped && (scope == "" || strings.Contains(scope, "%")) {
		return false
	}
	if addr == "" || utf8.RuneCountInString(addr) > 45 {
		return false
	}
	const hextets, maxParts = 8, 9
	parts := strings.SplitN(addr, ":", maxParts+1)
	if len(parts) < 3 {
		return false
	}
	if last := parts[len(parts)-1]; strings.Contains(last, ".") {
		if !parseIPv4(last) {
			return false
		}
		// Two hextets stand for the IPv4 tail; any valid hex does.
		parts = append(parts[:len(parts)-1], "0", "0")
	}
	if len(parts) > maxParts {
		return false
	}
	skip := -1
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			if skip >= 0 {
				return false
			}
			skip = i
		}
	}
	var hi, lo int
	if skip >= 0 {
		hi, lo = skip, len(parts)-skip-1
		if parts[0] == "" {
			if hi--; hi != 0 {
				return false
			}
		}
		if parts[len(parts)-1] == "" {
			if lo--; lo != 0 {
				return false
			}
		}
		if hextets-(hi+lo) < 1 {
			return false
		}
	} else {
		if len(parts) != hextets || parts[0] == "" || parts[len(parts)-1] == "" {
			return false
		}
		hi = len(parts)
	}
	for _, part := range append(append([]string{}, parts[:hi]...), parts[len(parts)-lo:]...) {
		// int("", 16) raises, so an empty hextet fails too.
		if part == "" || len(part) > 4 {
			return false
		}
		for i := 0; i < len(part); i++ {
			if !isHexDigit(part[i]) {
				return false
			}
		}
	}
	return true
}
