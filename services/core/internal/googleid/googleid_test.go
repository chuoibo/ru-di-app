package googleid

import "testing"

func TestClaimsFromRefusesTheWrongAudienceIssuerOrNoSubject(t *testing.T) {
	ids := map[string]bool{"web-client.apps": true}
	good := map[string]any{
		"aud":  "web-client.apps",
		"iss":  issuerAccountsURL,
		"sub":  "s",
		"name": "  An  ",
	}
	claims, err := ClaimsFrom(good, ids)
	if err != nil || claims.Subject != "s" || claims.DisplayName == nil || *claims.DisplayName != "An" {
		t.Fatalf("%+v %v", claims, err)
	}
	if _, err := ClaimsFrom(map[string]any{"aud": "somebody-elses-client", "iss": issuerAccountsURL, "sub": "s"}, ids); err == nil {
		t.Fatal("wrong audience")
	}
	if _, err := ClaimsFrom(map[string]any{"aud": "web-client.apps", "iss": "https://evil.example", "sub": "s"}, ids); err == nil {
		t.Fatal("wrong issuer")
	}
	if _, err := ClaimsFrom(map[string]any{"aud": "web-client.apps", "iss": issuerAccounts, "sub": ""}, ids); err == nil {
		t.Fatal("empty subject")
	}
	if _, err := ClaimsFrom(map[string]any{"aud": "web-client.apps", "iss": issuerAccounts}, ids); err == nil {
		t.Fatal("missing subject")
	}
	claims, err = ClaimsFrom(map[string]any{"aud": "web-client.apps", "iss": issuerAccounts, "sub": "s"}, ids)
	if err != nil || claims.Subject != "s" || claims.DisplayName != nil {
		t.Fatalf("%+v %v", claims, err)
	}
}

func TestFromEnvIsNilWithoutIds(t *testing.T) {
	if FromEnv(func(string) string { return "" }) != nil {
		t.Fatal("empty env")
	}
	if FromEnv(func(string) string { return "  , " }) != nil {
		t.Fatal("blank parts")
	}
	got := FromEnv(func(string) string { return " a.apps , b " })
	v, ok := got.(*LibraryVerifier)
	if !ok || !v.ClientIDs["a.apps"] || !v.ClientIDs["b"] {
		t.Fatalf("%#v", got)
	}
}
