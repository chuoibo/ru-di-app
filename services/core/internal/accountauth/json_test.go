package accountauth

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountJSONRejectsAmbiguousProofs(t *testing.T) {
	for _, body := range []string{`{"username":"first","username":"second"}`, `{"username":"first","user\u006eame":"second"}`, `{"google":{"id_token":"first","id_token":"second"}}`, `{"password":"\ud800"}`, `{"password":"\udc00"}`, `{"username":"x"} {}`, strings.Repeat("[", 17) + strings.Repeat("]", 17)} {
		if unambiguousJSON([]byte(body)) {
			t.Fatal("ambiguous input accepted")
		}
	}
	for _, body := range []string{`{"password":"\ud83d\ude00"}`, `{"password":"\\ud800"}`, `{"password":"literal replacement �"}`, `{"google":{"id_token":"value"}}`} {
		if !unambiguousJSON([]byte(body)) {
			t.Fatal("valid Unicode rejected")
		}
	}
	var input struct {
		Username string `json:"username"`
	}
	for _, body := range []string{`{"username":"valid","unexpected":true}`, strings.Repeat(" ", 17<<10) + `{}`} {
		r := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if decode(httptest.NewRecorder(), r, &input) == nil {
			t.Fatal("unbounded or unknown input accepted")
		}
	}
}
