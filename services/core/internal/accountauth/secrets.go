// Package accountauth owns self-managed accounts. Secrets never enter the
// generic request replay store, log attributes, or error responses.
package accountauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._]{3,32}$`)

func Username(s string) (string, error) {
	s = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "@"))
	if !usernamePattern.MatchString(s) {
		return "", problem(422, "username_invalid")
	}
	return s, nil
}
func Email(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || len(s) > 254 || !strings.Contains(s[strings.LastIndex(s, "@")+1:], ".") || strings.ContainsAny(s, "\r\n") {
		return "", problem(422, "email_invalid")
	}
	return s, nil
}

//go:embed common-passwords.sha256
var commonPasswordDigests string
var blocked = func() map[string]bool {
	m := map[string]bool{}
	for _, line := range strings.Fields(commonPasswordDigests) {
		m[strings.ReplaceAll(line, ":", "")] = true
	}
	return m
}()

func commonlyGuessed(s string) bool {
	lower := strings.ToLower(s)
	has := func(value string) bool { return blocked[hex.EncodeToString(digest(value))] }
	if has(lower) {
		return true
	}
	runes := []rune(lower)
	for n := 1; n <= len(runes)/2; n++ {
		if len(runes)%n == 0 && strings.Repeat(string(runes[:n]), len(runes)/n) == lower && has(string(runes[:n])) {
			return true
		}
	}
	// repo-guard: allow=long-number reason=public-common-password-policy
	for _, v := range []string{"123456789012345", "1234567890123456", "qwertyuiopasdfgh", "correct horse battery staple"} {
		if lower == v {
			return true
		}
	}
	return false
}
func Password(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", problem(422, "password_invalid")
	}
	s = norm.NFC.String(s)
	n := utf8.RuneCountInString(s)
	if n < 8 || n > 128 || commonlyGuessed(s) {
		return "", problem(422, "password_weak")
	}
	same := true
	var first rune
	for i, r := range s {
		if i == 0 {
			first = r
		} else if r != first {
			same = false
		}
	}
	if same {
		return "", problem(422, "password_weak")
	}
	return s, nil
}
func hashPassword(s string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(s), salt, 2, 19*1024, 1, 32)
	return "$argon2id$v=19$m=19456,t=2,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}
func verifyPassword(s, encoded string) bool {
	fields := strings.Split(encoded, "$")
	if len(fields) != 6 || fields[1] != "argon2id" || fields[2] != "v=19" || fields[3] != "m=19456,t=2,p=1" {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(fields[4])
	expected, e2 := base64.RawStdEncoding.DecodeString(fields[5])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}
	key := argon2.IDKey([]byte(norm.NFC.String(s)), salt, 2, 19*1024, 1, 32)
	return subtle.ConstantTimeCompare(key, expected) == 1
}
func randomSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}
func digest(s string) []byte { d := sha256.Sum256([]byte(s)); return d[:] }
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

type vault struct {
	aead cipher.AEAD
	key  []byte
}

func newVault(enc, lookup string) (vault, error) {
	a, e := base64.StdEncoding.DecodeString(enc)
	b, f := base64.StdEncoding.DecodeString(lookup)
	if e != nil || f != nil || len(a) != 32 || len(b) != 32 || subtle.ConstantTimeCompare(a, b) == 1 {
		return vault{}, fmt.Errorf("managed auth requires two distinct base64 32-byte keys")
	}
	block, err := aes.NewCipher(a)
	if err != nil {
		return vault{}, err
	}
	gcm, err := cipher.NewGCM(block)
	return vault{gcm, b}, err
}
func (v vault) mac(domain, value string) []byte {
	m := hmac.New(sha256.New, v.key)
	m.Write([]byte(domain + "\x00" + value))
	return m.Sum(nil)
}
func (v vault) seal(scope string, value any) ([]byte, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	n := make([]byte, v.aead.NonceSize())
	if _, err = rand.Read(n); err != nil {
		return nil, err
	}
	return v.aead.Seal(n, n, b, []byte(scope)), nil
}
func (v vault) open(scope string, b []byte, value any) error {
	n := v.aead.NonceSize()
	if len(b) < n {
		return fmt.Errorf("invalid encrypted payload")
	}
	plain, err := v.aead.Open(nil, b[:n], b[n:], []byte(scope))
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, value)
}
