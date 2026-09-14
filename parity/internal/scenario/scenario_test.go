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
		}, "uuid or token"},
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
