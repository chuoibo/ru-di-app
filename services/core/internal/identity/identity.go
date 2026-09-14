// Package identity ports app.api.person_identity: a telephone number becomes
// the person id the database stores, an HMAC-SHA256 under a server key, and
// the OTP tables store digests instead of numbers.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image.
//
// Two Python rules shape the port. `_MOBILE` is `^[35789]\d{8}$` on a str
// pattern, and `\d` there is every Unicode decimal digit, so CanonicalMobile
// accepts "0" followed by "9" and eight Arabic-Indic digits and returns them
// unchanged after "84". The derivations then call `canonical.encode("ascii")`,
// which raises UnicodeEncodeError for exactly those digits: a number the
// canonical form admits is one no id can be derived from. Both halves are
// kept. Strings are Python str values: valid UTF-8.
package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// KeyEnvVar is KEY_ENV_VAR.
const KeyEnvVar = "MOBILE_PERSON_ID_KEY"

// MinKeyLength is MIN_KEY_LENGTH, in code points of the stripped key.
const MinKeyLength = 32

const (
	domain         = "ru-di:nguoi:v2:"
	otpPhoneDomain = "ru-di:otp-phone:v1:"
	otpCodeDomain  = "ru-di:otp-code:v1:"
)

// Domain returns DOMAIN.
func Domain() []byte { return []byte(domain) }

// OTPPhoneDomain returns OTP_PHONE_DOMAIN.
func OTPPhoneDomain() []byte { return []byte(otpPhoneDomain) }

// OTPCodeDomain returns OTP_CODE_DOMAIN.
func OTPCodeDomain() []byte { return []byte(otpCodeDomain) }

// PersonIDKeyMissing is PersonIdKeyMissing. The message never holds the key.
type PersonIDKeyMissing struct {
	Message string
}

func (e *PersonIDKeyMissing) Error() string { return e.Message }

// UnicodeEncodeError is the UnicodeEncodeError str.encode raises: the codec,
// the first run of characters it could not encode (code point positions,
// End exclusive) and the reason. Error() is Python's str(exc).
type UnicodeEncodeError struct {
	Encoding string
	Object   []rune
	Start    int
	End      int
	Reason   string
}

func (e *UnicodeEncodeError) Error() string {
	if e.End == e.Start+1 {
		char := e.Object[e.Start]
		var escaped string
		switch {
		case char <= 0xFF:
			escaped = `\x` + padHex(char, 2)
		case char <= 0xFFFF:
			escaped = `\u` + padHex(char, 4)
		default:
			escaped = `\U` + padHex(char, 8)
		}
		return "'" + e.Encoding + "' codec can't encode character '" + escaped + "' in position " +
			strconv.Itoa(e.Start) + ": " + e.Reason
	}
	return "'" + e.Encoding + "' codec can't encode characters in position " + strconv.Itoa(e.Start) + "-" +
		strconv.Itoa(e.End-1) + ": " + e.Reason
}

func padHex(r rune, width int) string {
	digits := strconv.FormatInt(int64(r), 16)
	return strings.Repeat("0", max(0, width-len(digits))) + digits
}

// ReadKey is read_key for the raw bytes of the environment variable, "" when
// it is unset. os.environ decodes those bytes as UTF-8 with surrogateescape:
// every byte outside a well-formed sequence becomes U+DC80..U+DCFF, is not
// whitespace, counts as one character, and makes the final
// `.encode("utf-8")` fail.
func ReadKey(raw string) ([]byte, error) {
	points := surrogateEscape(raw)
	start, end := 0, len(points)
	for start < end && isPySpace(points[start]) {
		start++
	}
	for end > start && isPySpace(points[end-1]) {
		end--
	}
	stripped := points[start:end]
	if len(stripped) == 0 {
		return nil, &PersonIDKeyMissing{Message: KeyEnvVar + " is not set; person ids cannot be derived"}
	}
	if len(stripped) < MinKeyLength {
		return nil, &PersonIDKeyMissing{
			Message: KeyEnvVar + " is shorter than " + strconv.Itoa(MinKeyLength) + " characters",
		}
	}
	for i, r := range stripped {
		if !isSurrogate(r) {
			continue
		}
		run := i + 1
		for run < len(stripped) && isSurrogate(stripped[run]) {
			run++
		}
		return nil, &UnicodeEncodeError{
			Encoding: "utf-8", Object: stripped, Start: i, End: run, Reason: "surrogates not allowed",
		}
	}
	return []byte(string(stripped)), nil
}

func surrogateEscape(raw string) []rune {
	points := make([]rune, 0, len(raw))
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRuneInString(raw[i:])
		if r == utf8.RuneError && size == 1 {
			r = 0xDC00 + rune(raw[i])
		}
		points = append(points, r)
		i += size
	}
	return points
}

func isSurrogate(r rune) bool { return r >= 0xD800 && r <= 0xDFFF }

// CanonicalMobile is canonical_mobile: "84" and nine digits, or ok false.
// Whitespace, ".", "-", "(" and ")" are removed anywhere; then one of "+84",
// "84" or "0" comes off the front.
func CanonicalMobile(raw string) (string, bool) {
	var packed strings.Builder
	for _, r := range raw {
		if isPySpace(r) || r == '.' || r == '-' || r == '(' || r == ')' {
			continue
		}
		packed.WriteRune(r)
	}
	rest := packed.String()
	if rest == "" {
		return "", false
	}
	switch {
	case strings.HasPrefix(rest, "+84"):
		rest = rest[3:]
	case strings.HasPrefix(rest, "84"):
		rest = rest[2:]
	case strings.HasPrefix(rest, "0"):
		rest = rest[1:]
	}
	if !isMobile(rest) {
		return "", false
	}
	return "84" + rest, true
}

// isMobile is `_MOBILE.fullmatch`: one of 3 5 7 8 9, then exactly eight
// Unicode decimal digits.
func isMobile(s string) bool {
	count := 0
	for i, r := range s {
		if i == 0 {
			if !strings.ContainsRune("35789", r) {
				return false
			}
		} else if !unicode.Is(unicode.Nd, r) {
			return false
		}
		count++
	}
	return count == 9
}

// asciiBytes is text.encode("ascii").
func asciiBytes(text string) ([]byte, error) {
	points := []rune(text)
	for i, r := range points {
		if r < 0x80 {
			continue
		}
		run := i + 1
		for run < len(points) && points[run] >= 0x80 {
			run++
		}
		return nil, &UnicodeEncodeError{
			Encoding: "ascii", Object: points, Start: i, End: run, Reason: "ordinal not in range(128)",
		}
	}
	return []byte(text), nil
}

func sign(key []byte, parts ...[]byte) []byte {
	mac := hmac.New(sha256.New, key)
	for _, part := range parts {
		mac.Write(part)
	}
	return mac.Sum(nil)
}

// DerivePhoneDigest is derive_phone_digest.
func DerivePhoneDigest(canonical string, key []byte) ([]byte, error) {
	encoded, err := asciiBytes(canonical)
	if err != nil {
		return nil, err
	}
	return sign(key, []byte(otpPhoneDomain), encoded), nil
}

// DeriveCodeDigest is derive_code_digest; challengeID is UUID.bytes.
func DeriveCodeDigest(challengeID [16]byte, code string, key []byte) ([]byte, error) {
	encoded, err := asciiBytes(code)
	if err != nil {
		return nil, err
	}
	return sign(key, []byte(otpCodeDomain), challengeID[:], encoded), nil
}

// DerivePersonID is derive_person_id, returned as str(UUID): the first 16
// bytes of the digest with version 8 and the RFC 9562 variant forced.
func DerivePersonID(canonical string, key []byte) (string, error) {
	encoded, err := asciiBytes(canonical)
	if err != nil {
		return "", err
	}
	raw := sign(key, []byte(domain), encoded)[:16]
	raw[6] = raw[6]&0x0F | 0x80
	raw[8] = raw[8]&0x3F | 0x80
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, b := range raw {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out), nil
}

// isPySpace is str.isspace() for one code point, which is also what `\s`
// matches in a str pattern. oracle_test.go checks both against CPython over
// every code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}
