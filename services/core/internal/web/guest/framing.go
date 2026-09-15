package guest

import (
	"strconv"
	"strings"

	"mobile/services/core/internal/httpapi/problem"
)

// Response is what a Starlette response puts on the wire before the
// middleware and the server add theirs: the status, raw_headers in order
// with lower-case names, and the body. The guest privacy headers are not
// here for a route's answer; GuestPrivacyHeadersMiddleware adds them in
// Python and package mw/guest in Go.
type Response struct {
	Status  int
	Headers [][2]string
	Body    []byte
}

// PathPrefix is GUEST_PATH_PREFIX.
const PathPrefix = "/g"

// newResponse is Response.__init__ with headers=None: content-length unless
// the status carries no body, then content-type with the charset for text/*.
func newResponse(status int, body []byte, mediaType string) Response {
	var headers [][2]string
	if !(status < 200 || status == 204 || status == 304) {
		headers = append(headers, [2]string{"content-length", strconv.Itoa(len(body))})
	}
	if mediaType != "" {
		if strings.HasPrefix(mediaType, "text/") && !strings.Contains(strings.ToLower(mediaType), "charset=") {
			mediaType += "; charset=utf-8"
		}
		headers = append(headers, [2]string{"content-type", mediaType})
	}
	return Response{Status: status, Headers: headers, Body: body}
}

// TemplateResponse is Jinja2Templates.TemplateResponse(request, name,
// context, status_code): the rendered template as text/html. The request
// Starlette adds to the context is not read by these templates.
func TemplateResponse(name string, context map[string]any, status int) (Response, error) {
	t, err := Lookup(name)
	if err != nil {
		return Response{}, err
	}
	body, err := t.Render(context)
	if err != nil {
		return Response{}, err
	}
	return newResponse(status, []byte(body), "text/html"), nil
}

func page(name string, view map[string]any, token string) (Response, error) {
	return TemplateResponse(name, map[string]any{"view": view, "preview": NeutralPreview(), "token": token}, 200)
}

// GuestPage is guest_page's answer for a view GuestView built.
func GuestPage(view map[string]any, token string) (Response, error) {
	return page("guest.html", view, token)
}

// NotMePage is _page(request, "guest_not_me.html", view, token), the answer
// of both not_me_page and not_me_submit.
func NotMePage(view map[string]any, token string) (Response, error) {
	return page("guest_not_me.html", view, token)
}

// WrongAmountPage is wrong_amount_page's answer.
func WrongAmountPage(view map[string]any, token string) (Response, error) {
	return page("guest_wrong_amount.html", view, token)
}

// LinkBrokenPage is guest_link_broken_page: 404 with only the preview in the
// context, so the token is never echoed.
func LinkBrokenPage() (Response, error) {
	return TemplateResponse("guest_link_broken.html", map[string]any{"preview": NeutralPreview()}, 404)
}

// LinkBrokenApplies is api_problem_handler's branch: the page answers only
// guest_link_not_found, and only when request.url.path is a guest path.
// urlPath must be request.url.path, which Starlette parses back out of
// "scheme://<Host header><scope path>", not scope["path"].
func LinkBrokenApplies(code, urlPath string) bool {
	return code == problem.GuestLinkNotFound && problem.IsGuestPath(urlPath)
}

// SeeOther is RedirectResponse(url=url, status_code=303): an empty body with
// content-length 0, then location quoted as Starlette quotes it.
func SeeOther(url string) Response {
	r := newResponse(303, []byte{}, "")
	r.Headers = append(r.Headers, [2]string{"location", quoteLocation(url)})
	return r
}

// GuestPageURL is f"{GUEST_PATH_PREFIX}/{token}", where report_payment and
// wrong_amount_submit send a browser.
func GuestPageURL(token string) string { return PathPrefix + "/" + token }

// EvidenceRequestedURL is request_evidence's redirect: the form's
// obligation_id exactly as posted, not its canonical uuid.
func EvidenceRequestedURL(token, obligationID string) string {
	return PathPrefix + "/" + token + "/doi-so-tien?obligation_id=" + obligationID
}

// AcceptsHTML is `"text/html" in request.headers.get("accept", "")`: values
// are the request's Accept header values in order, and only the first counts.
func AcceptsHTML(values []string) bool {
	return len(values) > 0 && strings.Contains(values[0], "text/html")
}

// ServerError is guest_aware_server_error_response: Starlette's plain 500,
// plus the privacy headers, by assignment and in their declared order, when
// scopePath is under /g.
func ServerError(scopePath string) Response {
	r := newResponse(500, []byte("Internal Server Error"), "text/plain")
	if problem.IsGuestPath(scopePath) {
		for _, pair := range problem.GuestPrivacyHeaders {
			r.Headers = append(r.Headers, [2]string{strings.ToLower(pair[0]), pair[1]})
		}
	}
	return r
}

// locationSafe is RedirectResponse's safe set; letters, digits and "_.-~" are
// always safe in urllib.parse.quote.
const locationSafe = "_.-~:/%#?=@[]!$&'()*+,;"

// quoteLocation is urllib.parse.quote(url, safe=locationSafe) on the UTF-8
// bytes of url. It repeats the unexported quote in httpapi/router.
func quoteLocation(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 && (c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || strings.IndexByte(locationSafe, c) >= 0) {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hexDigits[c>>4])
		out.WriteByte(hexDigits[c&0x0f])
	}
	return out.String()
}
