package profilemedia

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"
)

const tokenLength = 16

var errInvalidIdentity = errors.New("invalid media identity")

// PersonToken is the legacy media proxy's opaque, stable tenant identifier.
func PersonToken(personID, key string) (string, error) {
	personID = strings.TrimSpace(personID)
	if personID == "" || utf8.RuneCountInString(key) < 32 {
		return "", errInvalidIdentity
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(personID))
	return hex.EncodeToString(mac.Sum(nil))[:tokenLength], nil
}

// NewJobID preserves the Python oracle's HMAC prefix and 128-bit random suffix.
func NewJobID(personID, key string) (string, error) {
	token, err := PersonToken(personID, key)
	if err != nil {
		return "", err
	}
	var secret [16]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", err
	}
	return token + "-" + base64.RawURLEncoding.EncodeToString(secret[:]), nil
}

// OwnsJob rejects malformed IDs before comparing the opaque tenant token.
func OwnsJob(jobID, personID, key string) bool {
	if len(jobID) < tokenLength+1+8 || jobID[tokenLength] != '-' {
		return false
	}
	for i := 0; i < tokenLength; i++ {
		c := jobID[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	for i := tokenLength + 1; i < len(jobID); i++ {
		c := jobID[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	want, err := PersonToken(personID, key)
	return err == nil && hmac.Equal([]byte(jobID[:tokenLength]), []byte(want))
}
