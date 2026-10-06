package chatv2

import (
	"encoding/hex"
	"testing"
)

// The bytes a device signs to enroll are the Rust crate's, byte for byte: the
// same vector is pinned in packages/chat-crypto/tests/multi_conversation.rs.
func TestEnrollmentBytesMatchTheRustVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	// repo-guard: allow=long-number reason=synthetic-uuid-test-vector
	got := hex.EncodeToString(EnrollmentBytes("aaaaaaaa-bbbb-4ccc-8ddd-000000000001", "aaaaaaaa-bbbb-4ccc-8ddd-00000000000b", key))
	if got != "525544492d434841542d444556494345007631000000002461616161616161612d626262622d346363632d386464642d3030303030303030303030310000002461616161616161612d626262622d346363632d386464642d303030303030303030303062000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" {
		t.Fatalf("enrollment bytes drifted from the Rust crate: %s", got)
	}
}
