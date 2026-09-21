package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

type goldens struct {
	UUID []struct {
		Input string `json:"input"`
		UUID  string `json:"uuid"`
		Error bool   `json:"error"`
	} `json:"uuid"`
	Actor []struct {
		Input struct {
			ID       *string `json:"id"`
			Roles    *string `json:"roles"`
			Contexts *string `json:"contexts"`
		} `json:"input"`
		Actor *struct {
			ID       string   `json:"id"`
			Roles    []string `json:"roles"`
			Contexts []string `json:"contexts"`
		} `json:"actor"`
		Problem *Problem `json:"problem"`
	} `json:"actor"`
	Bearer []struct {
		Input   *string  `json:"input"`
		Token   string   `json:"token"`
		Problem *Problem `json:"problem"`
	} `json:"bearer"`
}

// latin1 turns a golden string (runes <= 0xff) back into header bytes.
func latin1(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		out = append(out, byte(r))
	}
	return string(out)
}

func load(t *testing.T) goldens {
	t.Helper()
	data, err := os.ReadFile("testdata/python_actor.json")
	if err != nil {
		t.Fatalf("goldens missing — render them with scripts/render_actor_goldens.py: %v", err)
	}
	var g goldens
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.UUID) < 30 || len(g.Actor) < 15 || len(g.Bearer) < 10 {
		t.Fatalf("golden file looks truncated: %d/%d/%d", len(g.UUID), len(g.Actor), len(g.Bearer))
	}
	return g
}

func (p *Problem) UnmarshalJSON(data []byte) error {
	var raw struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = Problem{raw.Status, raw.Code, raw.Detail}
	return nil
}

func TestParsePythonUUIDMatchesPython(t *testing.T) {
	for _, c := range load(t).UUID {
		got, err := ParsePythonUUID(latin1(c.Input))
		if c.Error {
			if err == nil {
				t.Errorf("UUID(%q): Python refuses, Go accepted %s", c.Input, got)
			}
			continue
		}
		if err != nil || got != c.UUID {
			t.Errorf("UUID(%q) = %q, %v; Python gives %q", c.Input, got, err, c.UUID)
		}
	}
}

func TestDevActorMatchesPython(t *testing.T) {
	for _, c := range load(t).Actor {
		h := http.Header{}
		if c.Input.ID != nil {
			h["X-Actor-Id"] = []string{latin1(*c.Input.ID)}
		}
		if c.Input.Roles != nil {
			h["X-Actor-Roles"] = []string{latin1(*c.Input.Roles)}
		}
		if c.Input.Contexts != nil {
			h["X-Actor-Contexts"] = []string{latin1(*c.Input.Contexts)}
		}
		actor, problem := DevActor(h)
		name := describe(c.Input.ID, c.Input.Roles, c.Input.Contexts)
		if c.Problem != nil {
			if problem == nil || *problem != *c.Problem {
				t.Errorf("%s: got %+v %+v, Python refuses with %+v", name, actor, problem, *c.Problem)
			}
			continue
		}
		if problem != nil {
			t.Errorf("%s: Go refused %+v, Python accepted %+v", name, *problem, *c.Actor)
			continue
		}
		if actor.ID != c.Actor.ID || strings.Join(actor.Roles, ",") != strings.Join(c.Actor.Roles, ",") ||
			strings.Join(actor.Contexts, ",") != strings.Join(c.Actor.Contexts, ",") {
			t.Errorf("%s: got %+v, Python %+v", name, *actor, *c.Actor)
		}
	}
}

func TestBearerTokenMatchesPython(t *testing.T) {
	for _, c := range load(t).Bearer {
		h := http.Header{}
		if c.Input != nil {
			h["Authorization"] = []string{latin1(*c.Input)}
		}
		token, problem := BearerToken(h)
		if c.Problem != nil {
			if problem == nil || *problem != *c.Problem {
				t.Errorf("bearer %v: got %q %+v, Python refuses %+v", c.Input, token, problem, *c.Problem)
			}
			continue
		}
		if problem != nil || token != latin1(c.Token) {
			t.Errorf("bearer %v: got %q %+v, Python token %q", c.Input, token, problem, c.Token)
		}
	}
}

func describe(values ...*string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			parts[i] = "<absent>"
		} else {
			parts[i] = "\"" + *v + "\""
		}
	}
	return strings.Join(parts, " | ")
}

func TestFirstHeaderValueWins(t *testing.T) {
	h := http.Header{"X-Actor-Id": {"khong-phai-uuid", "abcdefab-cdef-abcd-efab-cdefabcdefab"}}
	if _, problem := DevActor(h); problem == nil || problem.Code != "invalid_actor_id" {
		t.Fatalf("second X-Actor-ID value was used: %+v", problem)
	}
}
