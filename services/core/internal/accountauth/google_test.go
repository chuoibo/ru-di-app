package accountauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

type certTransport struct{ body string }

func (c certTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		panic("unexpected network in Google verifier test")
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": []string{"public, max-age=300"}}, Body: io.NopCloser(strings.NewReader(c.body)), Request: r}, nil
}
func TestOfficialGoogleVerifierChecksSignedClaims(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	certs, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kid": "synthetic", "kty": "RSA", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	validator, err := idtoken.NewValidator(context.Background(), option.WithHTTPClient(&http.Client{Transport: certTransport{string(certs)}}))
	if err != nil {
		t.Fatal(err)
	}
	verifier := officialGoogle{validator: validator, audiences: map[string]bool{"synthetic-client": true}}
	sign := func(claims map[string]any) string {
		header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "synthetic"})
		body, _ := json.Marshal(claims)
		data := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
		sum := sha256.Sum256([]byte(data))
		sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if err != nil {
			t.Fatal(err)
		}
		return data + "." + base64.RawURLEncoding.EncodeToString(sig)
	}
	good := map[string]any{"iss": "https://accounts.google.com", "aud": "synthetic-client", "sub": "synthetic-subject", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": "synthetic-nonce", "email_verified": true, "email": "synthetic" + "@" + "gmail.com"}
	claims, err := verifier.Verify(context.Background(), sign(good))
	if err != nil || claims.Nonce != "synthetic-nonce" || !claims.EmailVerified {
		t.Fatal("valid signed proof refused", err)
	}
	thirdParty := map[string]any{}
	for k, v := range good {
		thirdParty[k] = v
	}
	thirdParty["email"] = "synthetic" + "@" + "example.test"
	thirdClaims, thirdErr := verifier.Verify(context.Background(), sign(thirdParty))
	if thirdErr != nil || thirdClaims.EmailVerified {
		t.Fatal("third-party Google email became recovery evidence", thirdErr)
	}
	thirdParty["hd"] = "example.test"
	hosted, hostedErr := verifier.Verify(context.Background(), sign(thirdParty))
	if hostedErr != nil || !hosted.EmailVerified {
		t.Fatal("hosted Google email refused", hostedErr)
	}

	for _, change := range []struct {
		key   string
		value any
	}{{"iss", "https://untrusted.example"}, {"aud", "different-client"}, {"sub", ""}, {"exp", time.Now().Add(-time.Minute).Unix()}} {
		bad := map[string]any{}
		for k, v := range good {
			bad[k] = v
		}
		bad[change.key] = change.value
		if _, err = verifier.Verify(context.Background(), sign(bad)); err == nil {
			t.Errorf("accepted invalid %s", change.key)
		}
	}
	forged := sign(good)
	parts := strings.Split(forged, ".")
	parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 256))
	if _, err = verifier.Verify(context.Background(), strings.Join(parts, ".")); err == nil {
		t.Fatal("accepted forged signature")
	}
	if _, err = verifier.Verify(context.Background(), "not-a-token"); err == nil {
		t.Fatal("accepted malformed token")
	}
}
