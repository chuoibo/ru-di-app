package scenario

import (
	"strings"
	"testing"
)

const valid = `
id: contexts/create-then-get
routes: ["POST /contexts", "GET /contexts/{context_id}"]
auth_mode: dev
personas:
  owner: {roles: [member]}
steps:
  - id: create
    as: owner
    request:
      method: POST
      path: /contexts
      headers: {content-type: application/json, status: kept-as-a-header-name}
      body_raw: '{"display_name":"Team Đà Lạt"}'
    bind:
      context_id: {from: body, pointer: /id, class: uuid}
  - id: read
    as: owner
    request:
      method: GET
      path: /contexts/{{context_id}}
  - id: anonymous_read
    as: anonymous
    request: {method: GET, path: "/contexts/{{context_id}}?by={{persona.owner}}"}
`

func TestValidScenarioLoads(t *testing.T) {
	sc, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Steps) != 3 || sc.Steps[0].Bind["context_id"].Pointer != "/id" {
		t.Fatalf("parsed = %+v", sc)
	}
}

func TestRefusals(t *testing.T) {
	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"expected status": {func(s string) string {
			return strings.Replace(s, "  - id: read\n", "  - id: read\n    expect: {status: 200}\n", 1)
		}, "not allowed"},
		"body oracle under step": {func(s string) string {
			return strings.Replace(s, "  - id: read\n", "  - id: read\n    body: '{}'\n", 1)
		}, "not allowed"},
		"unknown field": {func(s string) string {
			return strings.Replace(s, "  - id: read\n", "  - id: read\n    sleep: 5\n", 1)
		}, "not found"},
		"via other than python": {func(s string) string {
			return strings.Replace(s, "  - id: read\n    as: owner\n", "  - id: read\n    as: owner\n    via: core\n", 1)
		}, "via"},
		"unknown persona": {func(s string) string {
			return strings.Replace(s, "  - id: read\n    as: owner", "  - id: read\n    as: stranger", 1)
		}, "neither"},
		"unbound template": {func(s string) string {
			return strings.Replace(s, "/contexts/{{context_id}}\n", "/contexts/{{nope}}\n", 1)
		}, "not bound"},
		"duplicate step": {func(s string) string {
			return strings.Replace(s, "  - id: anonymous_read", "  - id: read", 1)
		}, "unique"},
		"bad auth mode": {func(s string) string {
			return strings.Replace(s, "auth_mode: dev", "auth_mode: maybe", 1)
		}, "dev or prod"},
		"bad bind class": {func(s string) string {
			return strings.Replace(s, "class: uuid", "class: number", 1)
		}, "uuid, token or echo"},
		"two documents": {func(s string) string { return s + "\n---\nid: other\n" }, "one scenario"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.mutate(valid)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	got, err := Render("/contexts/{{ context_id }}?by={{persona.owner}}", map[string]string{
		"context_id": "abc", "persona.owner": "p1",
	})
	if err != nil || got != "/contexts/abc?by=p1" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := Render("{{missing}}", map[string]string{}); err == nil {
		t.Fatal("missing variable accepted")
	}
}

func TestProdScenariosRefuseRolesAndKnowTokens(t *testing.T) {
	withRoles := `
id: t/prod-roles
routes: ["GET /people/me"]
auth_mode: prod
personas: {owner: {roles: [member]}}
steps:
  - {id: me, as: owner, request: {method: GET, path: /people/me}}
`
	if _, err := Parse([]byte(withRoles)); err == nil || !strings.Contains(err.Error(), "derives roles") {
		t.Fatalf("roles in prod accepted: %v", err)
	}
	devToken := `
id: t/dev-token
routes: ["GET /people/me"]
auth_mode: dev
personas: {owner: {}}
steps:
  - {id: me, as: anonymous, request: {method: GET, path: /people/me, headers: {authorization: "Bearer {{token.owner}}"}}}
`
	if _, err := Parse([]byte(devToken)); err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("token variable accepted in dev mode: %v", err)
	}
}

func TestSeededSessionsDefaultByModeAndBindTheirTokens(t *testing.T) {
	devSeeded := `
id: t/dev-sessions
routes: ["GET /sessions"]
auth_mode: dev
personas: {owner: {}, friend: {sessions: 2}}
steps:
  - {id: one, as: friend, request: {method: GET, path: /sessions, headers: {authorization: "Bearer {{token.friend}}"}}}
  - {id: two, as: friend, request: {method: GET, path: /sessions, headers: {authorization: "Bearer {{token.friend.2}}"}}}
`
	sc, err := Parse([]byte(devSeeded))
	if err != nil {
		t.Fatal(err)
	}
	if got := sc.Personas["owner"].SeededSessions("dev"); got != 0 {
		t.Fatalf("dev persona without sessions seeds %d, want 0", got)
	}
	if got := sc.Personas["friend"].SeededSessions("dev"); got != 2 {
		t.Fatalf("dev persona with sessions: 2 seeds %d", got)
	}
	if got := (Persona{}).SeededSessions("prod"); got != 1 {
		t.Fatalf("prod persona seeds %d by default, want 1", got)
	}
	for name, text := range map[string]string{
		"dev token of an unseeded persona": `
id: t/dev-unseeded
routes: ["GET /sessions"]
auth_mode: dev
personas: {owner: {}, friend: {sessions: 1}}
steps:
  - {id: one, as: anonymous, request: {method: GET, path: /sessions, headers: {authorization: "Bearer {{token.owner}}"}}}
`,
		"a token beyond the seeded count": `
id: t/dev-beyond
routes: ["GET /sessions"]
auth_mode: dev
personas: {friend: {sessions: 1}}
steps:
  - {id: one, as: friend, request: {method: GET, path: /sessions, headers: {authorization: "Bearer {{token.friend.2}}"}}}
`,
	} {
		if _, err := Parse([]byte(text)); err == nil || !strings.Contains(err.Error(), "not bound") {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	for name, text := range map[string]string{
		"prod persona with no session": `
id: t/prod-zero
routes: ["GET /sessions"]
auth_mode: prod
personas: {owner: {sessions: 0}}
steps:
  - {id: one, as: owner, request: {method: GET, path: /sessions}}
`,
		"more than the maximum": `
id: t/dev-many
routes: ["GET /sessions"]
auth_mode: dev
personas: {owner: {sessions: 5}}
steps:
  - {id: one, as: owner, request: {method: GET, path: /sessions}}
`,
	} {
		if _, err := Parse([]byte(text)); err == nil || !strings.Contains(err.Error(), "sessions") {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
}

func TestViaPythonLoads(t *testing.T) {
	sc, err := Parse([]byte(strings.Replace(valid, "  - id: read\n    as: owner\n", "  - id: read\n    as: owner\n    via: python\n", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if sc.Steps[1].Via != ViaPython || sc.Steps[0].Via != "" {
		t.Fatalf("via = %q, %q", sc.Steps[0].Via, sc.Steps[1].Via)
	}
}

func TestConcurrentSteps(t *testing.T) {
	burst := strings.Replace(valid, "  - id: read\n    as: owner\n", "  - id: read\n    as: owner\n    concurrent: 4\n", 1)
	burst = strings.Replace(burst, "path: /contexts/{{context_id}}\n", "path: /contexts/{{context_id}}?copy={{burst}}\n", 1)
	sc, err := Parse([]byte(burst))
	if err != nil {
		t.Fatal(err)
	}
	if sc.Steps[1].Concurrent != 4 || !sc.HasBursts() {
		t.Fatalf("concurrent = %d, HasBursts = %v", sc.Steps[1].Concurrent, sc.HasBursts())
	}
	plain, err := Parse([]byte(valid))
	if err != nil || plain.HasBursts() {
		t.Fatalf("a scenario without concurrent steps has bursts (err %v)", err)
	}
	refused := map[string]string{
		"one copy":                       strings.Replace(burst, "concurrent: 4", "concurrent: 1", 1),
		"too many copies":                strings.Replace(burst, "concurrent: 4", "concurrent: 17", 1),
		"burst in a step not concurrent": strings.Replace(burst, "    concurrent: 4\n", "", 1),
		"concurrent step binds":          strings.Replace(burst, "    concurrent: 4\n", "    concurrent: 4\n    bind: {again: {from: body, pointer: /id, class: uuid}}\n", 1),
		"bind named burst":               strings.Replace(burst, "context_id: {from: body", "burst: {from: body", 1),
	}
	for name, text := range refused {
		if text == burst {
			t.Fatalf("%s: the mutation did not apply", name)
		}
		if _, err := Parse([]byte(text)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}
