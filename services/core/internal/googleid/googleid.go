// Package googleid is app.api.google_identity: a Google ID token as a door
// (ADR-0016). Claims carry the stable sub and a display name. They do not
// carry an e-mail address.
package googleid

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

const (
	ClientIDsEnv      = "MOBILE_GOOGLE_CLIENT_IDS"
	ClockSkewSeconds  = 10
	googleCertsURL    = "https://www.googleapis.com/oauth2/v3/certs"
	issuerAccounts    = "accounts.google.com"
	issuerAccountsURL = "https://accounts.google.com"
)

// TokenInvalid is GoogleTokenInvalid.
type TokenInvalid struct {
	Message string
}

func (e *TokenInvalid) Error() string { return e.Message }

// Claims is GoogleClaims: what a verified token is allowed to say.
type Claims struct {
	Subject     string
	DisplayName *string
}

// Verifier is GoogleTokenVerifier.
type Verifier interface {
	Verify(idToken string) (Claims, error)
}

// ClaimsFrom is claims_from: reduce a verified JWT payload to Claims, or
// refuse it. Pure, so the audience/issuer/subject rules are testable without
// a token signed by Google.
func ClaimsFrom(payload map[string]any, clientIDs map[string]bool) (Claims, error) {
	aud, _ := payload["aud"].(string)
	if aud == "" || !clientIDs[aud] {
		return Claims{}, &TokenInvalid{Message: "audience is not one of this server's client ids"}
	}
	iss := payload["iss"]
	if iss != issuerAccounts && iss != issuerAccountsURL {
		return Claims{}, &TokenInvalid{Message: "issuer is not Google"}
	}
	sub, _ := payload["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return Claims{}, &TokenInvalid{Message: "token carries no subject"}
	}
	var display *string
	if name, ok := payload["name"].(string); ok {
		if stripped := strings.TrimSpace(name); stripped != "" {
			display = &stripped
		}
	}
	return Claims{Subject: strings.TrimSpace(sub), DisplayName: display}, nil
}

// LibraryVerifier is GoogleAuthLibraryVerifier: Google's certificates, then
// claims_from. A host with no client id has no verifier (FromEnv returns nil).
type LibraryVerifier struct {
	ClientIDs map[string]bool
	Now       func() time.Time
	certs     certCache
}

// Verify is GoogleAuthLibraryVerifier.verify.
func (v *LibraryVerifier) Verify(idToken string) (Claims, error) {
	payload, err := v.decode(idToken)
	if err != nil {
		return Claims{}, &TokenInvalid{Message: err.Error()}
	}
	return ClaimsFrom(payload, v.ClientIDs)
}

func (v *LibraryVerifier) decode(idToken string) (map[string]any, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("wrong number of segments in token")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, err
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("id token has invalid algorithm")
	}
	key, err := v.certs.key(header.Kid)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, err
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	skew := time.Duration(ClockSkewSeconds) * time.Second
	if err := checkTimeClaim(payload, "exp", now, skew, true); err != nil {
		return nil, err
	}
	if err := checkTimeClaim(payload, "iat", now, skew, false); err != nil {
		return nil, err
	}
	if err := checkTimeClaim(payload, "nbf", now, skew, false); err != nil {
		return nil, err
	}
	return payload, nil
}

func checkTimeClaim(payload map[string]any, name string, now time.Time, skew time.Duration, expire bool) error {
	raw, ok := payload[name]
	if !ok {
		if expire {
			return fmt.Errorf("token has no %s claim", name)
		}
		return nil
	}
	secs, ok := jsonNumber(raw)
	if !ok {
		return fmt.Errorf("token %s claim is not a number", name)
	}
	at := time.Unix(secs, 0)
	if expire {
		if now.After(at.Add(skew)) {
			return fmt.Errorf("token expired")
		}
		return nil
	}
	if at.After(now.Add(skew)) {
		return fmt.Errorf("token used too early")
	}
	return nil
}

func jsonNumber(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// FromEnv is build_google_verifier: nil when no client id is configured.
func FromEnv(getenv func(string) string) Verifier {
	if getenv == nil {
		return nil
	}
	raw := strings.TrimSpace(getenv(ClientIDsEnv))
	ids := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids[id] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return &LibraryVerifier{ClientIDs: ids}
}

type certCache struct {
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func (c *certCache) key(kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) && c.keys != nil {
		if key := c.keys[kid]; key != nil {
			return key, nil
		}
	}
	keys, maxAge, err := fetchGoogleKeys()
	if err != nil {
		return nil, err
	}
	c.keys = keys
	c.expires = time.Now().Add(maxAge)
	key := keys[kid]
	if key == nil {
		return nil, fmt.Errorf("certificate for key id not found")
	}
	return key, nil
}

func jwkInt(s string) (*big.Int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(raw), nil
}
