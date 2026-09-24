// Package static serves MOUNT /static: the three files the guest pages ask for.
//
// The guest pages are Go's and their stylesheet was not, so a guest was served
// Go HTML that linked a Python stylesheet. Deleting Python without this package
// leaves those pages unstyled.
//
// Every status, header and body below was measured against the pinned Python
// image on 2026-09-22 and is recorded in docs/migration/routes/static. Two
// details are easy to "tidy" into a divergence and are deliberate here:
// a 304 carries the ETag ALONE (no Last-Modified), and a 405 carries NO Allow
// header -- unlike the API router's 405, because Starlette answers this mount
// from StaticFiles and not from the router.
//
// WHAT DIFFERS FROM PYTHON, ON PURPOSE: the ETag VALUE. Starlette derives it
// from the file's mtime and size, never from the bytes:
//
//	etag = md5(f"{st_mtime}-{st_size}")
//
// Measured across two builds of identical content: mtime 21/09 14:47:00,898309 gave
// 643656382d0b4df1c31428b69735d3ec and mtime 21/09 17:46:18,253247 gave
// a31f7470416e1f039dddfaf64f1362c5, while md5(content) stayed
// 397aab04ee043894e776a8a6a2605f7e in both. So the value already changes on
// every image rebuild and no client can depend on it. Embedded files carry no
// mtime at all, so this package hashes the CONTENT, which is what a cache
// validator is supposed to track. See ADR-0029 §2.4 STATIC-VALIDATOR-VALUE.
package static

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"fmt"
	"net/http"
	"path"

	"mobile/services/core/internal/httpapi/router"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed files/*
var files embed.FS

// Names are the embedded files, copied from services/api/app/web/static.
var Names = []string{"design_system.css", "guest.css", "guest.js"}

// Prefix is the mount point. A path outside it is not ours.
const Prefix = "/static"

// lastModified is stamped once per process. Python's value is the mtime of the
// file inside the image, which is the moment that image was built; a process
// start is the same thing one layer up, and it is stable for as long as the
// answers it validates are.
var lastModified = time.Now().UTC().Truncate(time.Second)

// Response is one answer, framed byte for byte: the headers go out in this
// order, which is the order Starlette emits them in.
type Response struct {
	Status  int
	Headers [][2]string
	Body    []byte
}

type entry struct {
	body        []byte
	etag        string
	contentType string
}

var entries = mustLoad()

func mustLoad() map[string]entry {
	out := map[string]entry{}
	for _, name := range Names {
		body, err := files.ReadFile("files/" + name)
		if err != nil {
			panic(err)
		}
		sum := md5.Sum(body)
		out[name] = entry{
			body:        body,
			etag:        `"` + hex.EncodeToString(sum[:]) + `"`,
			contentType: contentTypeFor(name),
		}
	}
	return out
}

// contentTypeFor is mimetypes.guess_type as the pinned image answers it for the
// extensions this mount actually serves. It is a closed list on purpose: a new
// extension must be measured against Python before it can be served, since
// guessing wrong is a silent wire change.
func contentTypeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	}
	return "application/octet-stream"
}

var (
	notFound     = []byte(`{"detail":"Not Found"}`)
	notAllowed   = []byte(`{"detail":"Method Not Allowed"}`)
	httpDateForm = "Mon, 02 Jan 2006 15:04:05 GMT"
)

// Handles reports whether this mount answers the path. The bare prefix is not
// ours: the router redirects it before anything reaches a handler.
func Handles(urlPath string) bool {
	return strings.HasPrefix(urlPath, Prefix+"/")
}

// Serve answers one request beneath the mount.
//
// The bare "/static" 307 is NOT here. That redirect is Starlette's
// redirect_slashes, which belongs to the router and answers the same way for
// every route in the manifest; httpapi/router already emits it as KindRedirect.
// Reproducing it here would put one route's copy of a global rule in a second
// place, where the two could drift.
func Serve(method, urlPath string, header http.Header) Response {
	if method != http.MethodGet && method != http.MethodHead {
		// No Allow header: measured. The API router's 405 carries one; this
		// mount's does not, and reproducing "the tidy version" is a divergence.
		return json(http.StatusMethodNotAllowed, notAllowed)
	}

	name := strings.TrimPrefix(urlPath, Prefix+"/")
	file, ok := entries[name]
	// A name with a slash, a dot segment or an empty name is not a file we
	// serve. Starlette answers all of them 404, so there is no traversal branch
	// to get wrong: the map lookup is the whole of the decision.
	if !ok || name == "" || strings.Contains(name, "/") {
		return json(http.StatusNotFound, notFound)
	}

	if notModified(header, file.etag) {
		// The ETag alone. Measured: no Last-Modified on a 304.
		return Response{Status: http.StatusNotModified, Headers: [][2]string{{"etag", file.etag}}}
	}

	size := len(file.body)
	start, end, satisfiable, partial := rangeOf(header, file.etag, size)
	if partial && !satisfiable {
		return Response{Status: http.StatusRequestedRangeNotSatisfiable, Headers: [][2]string{
			{"content-range", "*/" + strconv.Itoa(size)},
			{"content-length", "0"},
			{"content-type", "text/plain; charset=utf-8"},
		}}
	}

	body := file.body
	status := http.StatusOK
	headers := [][2]string{
		{"content-type", file.contentType},
		{"accept-ranges", "bytes"},
		{"content-length", strconv.Itoa(size)},
		{"last-modified", lastModified.Format(httpDateForm)},
		{"etag", file.etag},
	}
	if partial {
		body = file.body[start : end+1]
		status = http.StatusPartialContent
		headers[2] = [2]string{"content-length", strconv.Itoa(len(body))}
		headers = append(headers, [2]string{
			"content-range", fmt.Sprintf("bytes %d-%d/%d", start, end, size),
		})
	}
	if method == http.MethodHead {
		body = nil
	}
	return Response{Status: status, Headers: headers, Body: body}
}

func json(status int, body []byte) Response {
	return Response{Status: status, Headers: [][2]string{
		{"content-length", strconv.Itoa(len(body))},
		{"content-type", "application/json"},
	}, Body: body}
}

// notModified is RFC 9110 precedence: If-None-Match decides alone when present,
// and only then does If-Modified-Since get a say.
func notModified(header http.Header, etag string) bool {
	if match := header.Get("If-None-Match"); match != "" {
		return etagMatches(match, etag)
	}
	since := header.Get("If-Modified-Since")
	if since == "" {
		return false
	}
	at, err := http.ParseTime(since)
	return err == nil && !lastModified.After(at)
}

func etagMatches(candidates, etag string) bool {
	for _, part := range strings.Split(candidates, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == etag || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}

// rangeOf reads a single byte range. A multi-range request is served whole:
// its boundary would be random on both sides, so it can never be compared byte
// for byte, and answering 200 is a legal answer to any Range request.
func rangeOf(header http.Header, etag string, size int) (start, end int, satisfiable, partial bool) {
	spec := header.Get("Range")
	if spec == "" {
		return 0, 0, false, false
	}
	if cond := header.Get("If-Range"); cond != "" && !etagMatches(cond, etag) {
		return 0, 0, false, false
	}
	if !strings.HasPrefix(spec, "bytes=") {
		return 0, 0, false, false
	}
	spec = strings.TrimPrefix(spec, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false, false
	}
	from, to, found := strings.Cut(spec, "-")
	if !found {
		return 0, 0, false, false
	}
	if from == "" {
		length, err := strconv.Atoi(to)
		if err != nil || length <= 0 {
			return 0, 0, false, true
		}
		if length > size {
			length = size
		}
		return size - length, size - 1, true, true
	}
	start, err := strconv.Atoi(from)
	if err != nil || start >= size {
		return 0, 0, false, true
	}
	end = size - 1
	if to != "" {
		if end, err = strconv.Atoi(to); err != nil {
			return 0, 0, false, true
		}
		if end > size-1 {
			end = size - 1
		}
	}
	if end < start {
		return 0, 0, false, true
	}
	return start, end, true, true
}

// Files lists the embedded names sorted, for the ownership gate.
func Files() []string {
	out := append([]string(nil), Names...)
	sort.Strings(out)
	return out
}

// RouteID is the manifest row this mount answers.
const RouteID = "MOUNT /static"

// Handler adapts Serve to net/http. It recomputes the request path the same way
// dispatch does, so the handler sees exactly the path the router decided on.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := router.SplitTarget(r.RequestURI)
		response := Serve(r.Method, router.ScopePath(raw), r.Header)
		header := w.Header()
		for _, pair := range response.Headers {
			header.Set(pair[0], pair[1])
		}
		w.WriteHeader(response.Status)
		// A 304 and a HEAD carry no body, and Serve already emptied them.
		if len(response.Body) > 0 {
			_, _ = w.Write(response.Body)
		}
	})
}
