package accountauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
)

func testVault(t *testing.T) vault {
	t.Helper()
	v, e := newVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestPasswordNormalizationAndPolicy(t *testing.T) {
	// repo-guard: allow=long-number reason=synthetic-password-policy-case
	for _, s := range []string{"short", "123456789012345", "passwordpassword", "aaaaaaaaaaaaaaa"} {
		if _, e := Password(s); e == nil {
			t.Fatalf("accepted blocked synthetic password %q", s)
		}
	}
	a, e := Password(" café au lait with space ")
	if e != nil {
		t.Fatal(e)
	}
	hashed, e := hashPassword(a)
	if e != nil {
		t.Fatal(e)
	}
	if !verifyPassword(" cafe\u0301 au lait with space ", hashed) || verifyPassword("café au lait with space", hashed) {
		t.Fatal("NFC must agree; spaces must remain significant")
	}
	second, _ := hashPassword(a)
	if second == hashed {
		t.Fatal("salts repeated")
	}
	// repo-guard: allow=long-number reason=synthetic-unbounded-argon-parameters
	if verifyPassword(a, "$argon2id$v=19$m=999999999,t=2,p=1$x$x") {
		t.Fatal("unbounded encoded parameters accepted")
	}
}
func TestPasswordLengthBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{
		{"seven", "m7Z!q2R", false},
		{"eight", "m7Z!q2Rp", true},
		{"fourteen", "m7Z!q2Rp_4D%v8", true},
		{"unicode_seven", "é猫🌿ßø水Ж", false},
		{"unicode_eight", "é猫🌿ßø水Жλ", true},
		{"nfc_seven", "e\u0301猫🌿ßø水Ж", false},
		{"nfc_eight", "e\u0301猫🌿ßø水Жλ", true},
		{"maximum", strings.Repeat("xY!4", 32), true},
		{"over_maximum", strings.Repeat("xY!4", 32) + "z", false},
		{"blocked_eight", "password", false},
		{"same_eight", "zzzzzzzz", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Password(tc.value)
			if (err == nil) != tc.valid {
				t.Fatalf("password validity = %v, want %v", err == nil, tc.valid)
			}
		})
	}
}
func TestEmailUsernameAndVault(t *testing.T) {
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	a, _ := Email(" Person.Name+tag@EXAMPLE.test ")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	b, _ := Email("personname@example.test")
	if a == b {
		t.Fatal("Gmail-style collapsing changed identity")
	}
	if s, e := Username("@Example.user"); e != nil || s != "example.user" {
		t.Fatal(s, e)
	}
	for _, s := range []string{"ab", "éname", "long space name", "name/route"} {
		if _, e := Username(s); e == nil {
			t.Fatal("accepted invalid username")
		}
	}
	v := testVault(t)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	sealed, e := v.seal("first", "synthetic@example.test")
	if e != nil {
		t.Fatal(e)
	}
	var out string
	if v.open("second", sealed, &out) == nil {
		t.Fatal("ciphertext moved between purposes")
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	if e = v.open("first", sealed, &out); e != nil || out != "synthetic@example.test" {
		t.Fatal("roundtrip failed")
	}
	sealed[len(sealed)-1] ^= 1
	if v.open("first", sealed, &out) == nil {
		t.Fatal("tamper accepted")
	}
}
func TestProxyAndRetiredDoors(t *testing.T) {
	h := New(nil, Config{})
	r := httptest.NewRequest("POST", "/auth/login", nil)
	r.RemoteAddr = "192.0.2.1:443"
	// repo-guard: allow=long-number reason=synthetic-reserved-test-address
	r.Header.Set("X-Forwarded-For", "198.51.100.1")
	if h.clientIP(r) != "192.0.2.1" {
		t.Fatal("untrusted forwarded address accepted")
	}
	for _, path := range []string{"/auth/otp/request", "/auth/otp/verify", "/identity/person-id", "/sessions", "/moi/token"} {
		r := httptest.NewRequest("POST", path, nil)
		if !Retired(r) {
			t.Fatal("retired path accepted", path)
		}
	}
	if Retired(httptest.NewRequest("GET", "/sessions", nil)) || Retired(httptest.NewRequest("POST", "/contexts/group/invites", nil)) {
		t.Fatal("session management/group invitation retired")
	}
	_, e := h.hash(context.Background(), "synthetic test credential long")
	if e != nil {
		t.Fatal(e)
	}
}
