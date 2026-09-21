package routes

import (
	"errors"
	"testing"

	"mobile/services/core/internal/httpapi/endpoint"
)

// Sample numbers are assembled from short pieces so no digit run reaches the
// repository.
func TestMobileFromBodyRefusesInPythonsOrder(t *testing.T) {
	mobile := "03" + "99" + "000" + "001"
	canonical := "84" + "399" + "000" + "001"
	cases := map[string]string{
		``:                                      "invalid_body",
		`nope`:                                  "invalid_body",
		"\xff":                                  "invalid_body",
		`["phone"]`:                             "phone_required",
		`{"phone":7}`:                           "phone_required",
		`{"phone":null}`:                        "phone_required",
		`{"phone":"02` + "99" + "000" + `001"}`: "phone_not_mobile",
		`{"phone":"` + mobile + `"}`:            canonical,
		`{"phone":7,"phone":"` + mobile + `"}`:  canonical,
		`{"phone":"` + mobile + `","phone":7}`:  "phone_required",
		"\ufeff" + `{"phone":"` + mobile + `"}`: canonical,
		`{"phone":" (` + mobile[:4] + `) ` + mobile[4:] + `"}`: canonical,
	}
	for body, want := range cases {
		got, err := mobileFromBody([]byte(body))
		var refusal *endpoint.Refusal
		switch {
		case errors.As(err, &refusal):
			got = refusal.Problem.Code
		case err != nil:
			t.Errorf("%q: %v", body, err)
			continue
		}
		if got != want {
			t.Errorf("%q: %q, want %q", body, got, want)
		}
	}
}
