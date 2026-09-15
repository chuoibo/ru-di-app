package pyval

import (
	"errors"
	"sort"
	"strings"
)

// Starlette dispatches a form body on the content type, and python-multipart
// names each part, through python_multipart.multipart.parse_options_header,
// which hands any value holding ";" to email.message.Message.get_params.
// This file ports that path as the pinned image has it:
//
//   - python_multipart 0.0.20, multipart.py:167-207 (parse_options_header);
//   - CPython 3.12.14 email/message.py:73-96 (_parseparam), 99-107
//     (_unquotevalue), 666-707 (_get_params_preserve, get_params);
//   - email/utils.py:355-362 (unquote), 367-372 (decode_rfc2231),
//     390-439 (decode_params);
//   - urllib/parse.py:647-712 (unquote with encoding="latin-1").
//
// Every function works on latin-1 text held one byte per code point, which
// is how both callers decode the header (Starlette's Headers, and
// parse_options_header's own bytes.decode("latin-1")).

// errHeaderParams is an exception raised while decoding RFC 2231
// parameters: the TypeError list.sort raises when numbered and unnumbered
// continuations of one name meet, or the ValueError int() raises on a
// continuation number longer than 4300 digits. FastAPI answers either with
// its generic 400.
var errHeaderParams = errors.New("pyval: header parameters raised an exception")

// parseOptionsHeader is parse_options_header(value) for a header value
// (latin-1, one byte per code point); an absent header is "".
func parseOptionsHeader(value string) (string, map[string]string, error) {
	options := map[string]string{}
	if value == "" {
		return "", options, nil
	}
	if strings.IndexByte(value, ';') < 0 {
		return latin1Strip(latin1Lower(value)), options, nil
	}
	params, err := messageParams(value)
	if err != nil {
		return "", nil, err
	}
	for _, p := range params[1:] {
		v := p.value
		// The IE6 fix: a full Windows path keeps its last component.
		if p.name == "filename" && (pySlice(v, 1, 3) == `:\` || pySlice(v, 0, 2) == `\\`) {
			v = v[strings.LastIndexByte(v, '\\')+1:]
		}
		options[p.name] = v
	}
	// The content type is the first parameter's name, neither lowercased
	// nor unquoted: "Multipart/Form-Data; boundary=x" is not multipart.
	return params[0].name, options, nil
}

// pySlice is s[i:j] for non-negative i and j, clamped as Python clamps.
func pySlice(s string, i, j int) string {
	i, j = min(i, len(s)), min(j, len(s))
	if i >= j {
		return ""
	}
	return s[i:j]
}

type emailParam struct {
	name string
	// value is the unquoted value; for an RFC 2231 parameter, the third
	// item of its (charset, language, value) tuple.
	value string
}

// messageParams is Message.get_params() after message["content-type"] =
// value, with compat32's header_store_parse and header_fetch_parse leaving
// a latin-1 str unchanged.
func messageParams(value string) ([]emailParam, error) {
	var raw [][2]string
	for _, p := range parseParam(value) {
		if i := strings.IndexByte(p, '='); i >= 0 {
			raw = append(raw, [2]string{latin1Strip(p[:i]), latin1Strip(p[i+1:])})
		} else {
			raw = append(raw, [2]string{latin1Strip(p), ""})
		}
	}
	decoded, err := decodeParams(raw)
	if err != nil {
		return nil, err
	}
	out := make([]emailParam, len(decoded))
	for i, d := range decoded {
		out[i] = emailParam{name: d.name, value: emailUnquote(d.value)}
	}
	return out, nil
}

// parseParam is email.message._parseparam, index arithmetic included: the
// quote count walks segments between semicolons with Python's find and
// count, whose indices clamp and wrap as CPython's ADJUST_INDICES does.
func parseParam(value string) []string {
	s := ";" + value
	var plist []string
	start := 0
	for pyFind(s, ";", start, len(s)) == start {
		start++
		end := pyFind(s, ";", start, len(s))
		ind, diff := start, 0
		for end > 0 {
			diff += pyCount(s, `"`, ind, end) - pyCount(s, `\"`, ind, end)
			if diff%2 == 0 {
				break
			}
			end, ind = ind, pyFind(s, ";", end+1, len(s))
		}
		if end < 0 {
			end = len(s)
		}
		var f string
		if i := pyFind(s, "=", start, end); i == -1 {
			f = s[start:end]
		} else {
			f = latin1Lower(latin1RStrip(s[start:i])) + "=" + latin1LStrip(s[i+1:end])
		}
		plist = append(plist, latin1Strip(f))
		start = end
	}
	return plist
}

// adjustIndices is CPython's ADJUST_INDICES for str.find and str.count.
func adjustIndices(start, end, n int) (int, int) {
	if end > n {
		end = n
	} else if end < 0 {
		end = max(end+n, 0)
	}
	if start < 0 {
		start = max(start+n, 0)
	}
	return start, end
}

// pyFind is s.find(sub, start, end) for a non-empty sub.
func pyFind(s, sub string, start, end int) int {
	start, end = adjustIndices(start, end, len(s))
	if end-start < len(sub) {
		return -1
	}
	if i := strings.Index(s[start:end], sub); i >= 0 {
		return start + i
	}
	return -1
}

// pyCount is s.count(sub, start, end) for a non-empty sub.
func pyCount(s, sub string, start, end int) int {
	start, end = adjustIndices(start, end, len(s))
	if end-start < len(sub) {
		return 0
	}
	return strings.Count(s[start:end], sub)
}

type decodedParam struct {
	name  string
	value string
}

type continuation struct {
	hasNum  bool
	num     string // decimal digits without leading zeros
	value   string
	encoded bool
}

// decodeParams is email.utils.decode_params. A tuple value is represented
// by its third item, the only one parse_options_header keeps.
func decodeParams(params [][2]string) ([]decodedParam, error) {
	out := []decodedParam{{name: params[0][0], value: params[0][1]}}
	var order []string
	groups := map[string][]continuation{}
	for _, p := range params[1:] {
		name, value := p[0], p[1]
		encoded := strings.HasSuffix(name, "*")
		value = emailUnquote(value)
		base, num, hasNum, ok := rfc2231Continuation(name)
		if !ok {
			out = append(out, decodedParam{name: name, value: `"` + emailQuote(value) + `"`})
			continue
		}
		c := continuation{value: value, encoded: encoded, hasNum: hasNum}
		if hasNum {
			// int(num) refuses more than sys.get_int_max_str_digits() digits.
			if len(num) > maxIntStrDigits {
				return nil, errHeaderParams
			}
			c.num = strings.TrimLeft(num, "0")
		}
		if _, seen := groups[base]; !seen {
			order = append(order, base)
		}
		groups[base] = append(groups[base], c)
	}
	for _, name := range order {
		conts := groups[name]
		if len(conts) > 1 {
			numbered := 0
			for _, c := range conts {
				if c.hasNum {
					numbered++
				}
			}
			// Comparing an int with None raises TypeError, and sorting a
			// list that holds both has to compare the two kinds somewhere.
			if numbered != 0 && numbered != len(conts) {
				return nil, errHeaderParams
			}
		}
		sort.SliceStable(conts, func(i, j int) bool { return lessContinuation(conts[i], conts[j]) })
		var joined strings.Builder
		extended := false
		for _, c := range conts {
			s := c.value
			if c.encoded {
				// urllib.parse.unquote(s, encoding="latin-1") keeps one byte
				// per code point, so the unquoted bytes are the text.
				if strings.IndexByte(s, '%') >= 0 {
					s = string(unquoteToBytes(s))
				}
				extended = true
			}
			joined.WriteString(s)
		}
		value := emailQuote(joined.String())
		if extended {
			// decode_rfc2231: s.split("'", 2), the value is the last part
			// when there are three.
			if parts := strings.SplitN(value, "'", 3); len(parts) == 3 {
				value = parts[2]
			}
		}
		out = append(out, decodedParam{name: name, value: `"` + value + `"`})
	}
	return out, nil
}

// lessContinuation orders (num, value, encoded) tuples as Python does.
func lessContinuation(a, b continuation) bool {
	if a.hasNum && b.hasNum && a.num != b.num {
		if len(a.num) != len(b.num) {
			return len(a.num) < len(b.num)
		}
		return a.num < b.num
	}
	if a.value != b.value {
		return a.value < b.value
	}
	return !a.encoded && b.encoded
}

// rfc2231Continuation matches ^(?P<name>\w+)\*((?P<num>[0-9]+)\*?)?$ under
// re.ASCII. A name never ends in a newline here: it was stripped.
func rfc2231Continuation(name string) (base, num string, hasNum, ok bool) {
	star := strings.IndexByte(name, '*')
	if star <= 0 {
		return "", "", false, false
	}
	for i := 0; i < star; i++ {
		if !isASCIIWord(name[i]) {
			return "", "", false, false
		}
	}
	rest := name[star+1:]
	if rest == "" {
		return name[:star], "", false, true
	}
	digits := strings.TrimSuffix(rest, "*")
	if digits == "" {
		return "", "", false, false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return "", "", false, false
		}
	}
	return name[:star], digits, true, true
}

func isASCIIWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// emailUnquote is email.utils.unquote.
func emailUnquote(s string) string {
	if len(s) > 1 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return strings.ReplaceAll(strings.ReplaceAll(s[1:len(s)-1], `\\`, `\`), `\"`, `"`)
		}
		if s[0] == '<' && s[len(s)-1] == '>' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// emailQuote is email.utils.quote.
func emailQuote(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

// latin1IsSpace is str.isspace() for a latin-1 code point.
func latin1IsSpace(c byte) bool {
	return c >= 0x09 && c <= 0x0D || c >= 0x1C && c <= 0x20 || c == 0x85 || c == 0xA0
}

func latin1LStrip(s string) string {
	i := 0
	for i < len(s) && latin1IsSpace(s[i]) {
		i++
	}
	return s[i:]
}

func latin1RStrip(s string) string {
	j := len(s)
	for j > 0 && latin1IsSpace(s[j-1]) {
		j--
	}
	return s[:j]
}

func latin1Strip(s string) string { return latin1RStrip(latin1LStrip(s)) }

// latin1Lower is str.lower() for latin-1 text: A-Z and U+00C0..U+00DE
// except U+00D7 move to their lower case, which is also latin-1.
func latin1Lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' || c >= 0xC0 && c <= 0xDE && c != 0xD7 {
			b[i] = c + 0x20
		}
	}
	return string(b)
}
