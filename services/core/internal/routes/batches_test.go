package routes

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

// secrets.token_urlsafe(32) is 32 bytes as base64url without padding, and
// token_digest is sha256 over the token's UTF-8 bytes.
func TestGuestTokenIsTokenURLSafe32(t *testing.T) {
	alphabet := regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		token, err := mintGuestToken()
		if err != nil {
			t.Fatal(err)
		}
		if !alphabet.MatchString(token) {
			t.Fatalf("token %q is not 43 base64url characters", token)
		}
		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil || len(raw) != 32 {
			t.Fatalf("token %q decodes to %d bytes (%v)", token, len(raw), err)
		}
		want := sha256.Sum256([]byte(token))
		if string(auth.TokenDigest(token)) != string(want[:]) {
			t.Fatalf("digest of %q is not sha256 of its bytes", token)
		}
		if seen[token] {
			t.Fatalf("token %q repeated", token)
		}
		seen[token] = true
	}
}

// An empty expense_version_ids is an empty tuple, which selects nothing; only
// None reads every confirmed version.
func TestBatchVersionIDsKeepsAnEmptyListApartFromNone(t *testing.T) {
	model := func(v pyval.Value) *pyval.Model {
		return &pyval.Model{Class: "BatchCreateRequest", Fields: []pyval.Field{{Name: "expense_version_ids", Value: v}}}
	}
	ids, named, err := batchVersionIDsField(model(pyval.List{}), "expense_version_ids")
	if err != nil || !named || ids == nil || len(ids) != 0 {
		t.Fatalf("[] read as ids=%v named=%v err=%v", ids, named, err)
	}
	ids, named, err = batchVersionIDsField(model(pyjson.Null{}), "expense_version_ids")
	if err != nil || named || ids != nil {
		t.Fatalf("None read as ids=%v named=%v err=%v", ids, named, err)
	}
	id := pyval.UUID{0xab, 0x1c, 0x2d, 0x3e, 0x4f, 0x5a, 0x6b, 0x7c, 0x8d, 0x9e, 0xaf, 0xba, 0xcb, 0xdc, 0xed, 0xfe}
	ids, named, err = batchVersionIDsField(model(pyval.List{id, id}), "expense_version_ids")
	if err != nil || !named || len(ids) != 2 || ids[0] != "ab1c2d3e-4f5a-6b7c-8d9e-afbacbdcedfe" || ids[1] != ids[0] {
		t.Fatalf("[id, id] read as ids=%v named=%v err=%v", ids, named, err)
	}
}
