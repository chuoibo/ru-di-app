package profilemedia

import "testing"

const (
	testPerson = "00000000-0000-0000-0000-000000000001"
	testKey    = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func TestPersonTokenMatchesLegacyMediaOwnerContract(t *testing.T) {
	got, err := PersonToken(testPerson, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if got != "97d3282911d4ded0" {
		t.Fatalf("token = %q", got)
	}
}

func TestJobIDsArePrivateAndRejectMalformedOrForeignIDs(t *testing.T) {
	jobID, err := NewJobID(testPerson, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if !OwnsJob(jobID, testPerson, testKey) {
		t.Fatalf("owner cannot read own job %q", jobID)
	}
	for _, candidate := range []string{
		jobID[:16] + "-short",
		"97d3282911d4ded0-../sneak",
		"97d3282911d4ded0-AAAAAAAAAAAAAAAAAAAAAA=",
	} {
		if OwnsJob(candidate, testPerson, testKey) {
			t.Fatalf("accepted malformed job ID %q", candidate)
		}
	}
	if OwnsJob(jobID, "00000000-0000-0000-0000-000000000002", testKey) {
		t.Fatal("foreign actor can read job")
	}
}

func TestMediaIdentityRefusesShortServerKey(t *testing.T) {
	if _, err := PersonToken(testPerson, "short"); err == nil {
		t.Fatal("short HMAC key was accepted")
	}
	if _, err := NewJobID(testPerson, "short"); err == nil {
		t.Fatal("short HMAC key was accepted for job")
	}
}
