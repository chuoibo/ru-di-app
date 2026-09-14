package dbsnap

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestTopLevelMembersKeepValueBytes(t *testing.T) {
	text := `{"id":"x","meta":{"b": 1, "aa": [1, 2]},"n":-1.5e3,"s":"q\"}{,:","z":null}`
	members, err := topLevelMembers(text)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"id":   `"x"`,
		"meta": `{"b": 1, "aa": [1, 2]}`,
		"n":    `-1.5e3`,
		"s":    `"q\"}{,:"`,
		"z":    `null`,
	}
	if len(members) != len(want) {
		t.Fatalf("got %d members, want %d", len(members), len(want))
	}
	for _, m := range members {
		if got := text[m.start:m.end]; got != want[m.name] {
			t.Errorf("%s = %s, want %s", m.name, got, want[m.name])
		}
	}
	if _, err := topLevelMembers(`["not", "an", "object"]`); err == nil {
		t.Error("an array was accepted as a row")
	}
}

func TestCanonicalPadsOnlyTimestampColumns(t *testing.T) {
	rel := &Relation{Name: "things", Columns: []Column{
		{Name: "at", Type: "timestamptz"},
		{Name: "naive", Type: "timestamp"},
		{Name: "forever", Type: "timestamptz"},
		{Name: "gone", Type: "timestamptz"},
		{Name: "label", Type: "text"},
	}}
	raw := `{"at":"2026-09-14T10:00:05.12+00:00","naive":"2026-09-14T10:00:05","forever":"infinity","gone":null,"label":"2026-09-14T10:00:05.12+00:00"}`
	got, err := canonicalText(raw, planRewrites(rel, nil))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"at":"2026-09-14T10:00:05.120000+00:00","naive":"2026-09-14T10:00:05.000000","forever":"infinity","gone":null,"label":"2026-09-14T10:00:05.12+00:00"}`
	if got != want {
		t.Errorf("canonical text\n got %s\nwant %s", got, want)
	}
}

func TestCanonicalLeavesRowsWithoutRewritesIdentical(t *testing.T) {
	raw := `{"id":"x","doc":{"b": 1}}`
	rel := &Relation{Name: "plain", Columns: []Column{{Name: "id", Type: "uuid"}, {Name: "doc", Type: "jsonb"}}}
	got, err := canonicalText(raw, planRewrites(rel, nil))
	if err != nil || got != raw {
		t.Errorf("got %q, %v; want the raw text unchanged", got, err)
	}
}

func TestCanonicalDecodesListedByteaOnly(t *testing.T) {
	body := `{"id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","at":"2026-09-14T10:00:05.123456+00:00"}`
	digest := hex.EncodeToString([]byte{0xde, 0xad, 0xbe, 0xef})
	rel := &Relation{Name: "idempotency_keys", Columns: []Column{
		{Name: "response_body", Type: "bytea"},
		{Name: "digest", Type: "bytea"},
	}}
	plan := planRewrites(rel, map[string]bool{"idempotency_keys.response_body": true})

	raw := `{"response_body":"\\x` + hex.EncodeToString([]byte(body)) + `","digest":"\\x` + digest + `"}`
	got, err := canonicalText(raw, plan)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"response_body":{"$bytea_json":` + body + `},"digest":"\\x` + digest + `"}`
	if got != want {
		t.Errorf("JSON body\n got %s\nwant %s", got, want)
	}

	raw = `{"response_body":"\\x` + hex.EncodeToString([]byte("plain <text>")) + `","digest":null}`
	if got, _ := canonicalText(raw, plan); got != `{"response_body":{"$bytea_utf8":"plain <text>"},"digest":null}` {
		t.Errorf("UTF-8 body: got %s", got)
	}

	raw = `{"response_body":"\\xfffe","digest":null}`
	if got, _ := canonicalText(raw, plan); got != raw {
		t.Errorf("binary body was rewritten: %s", got)
	}
}

func TestStoredResponsesDecodeBody(t *testing.T) {
	body := `{"ok": true}`
	rel := &Relation{
		Name:       "idempotency_keys",
		PrimaryKey: []string{"id"},
		Rows: []Row{
			row(`["k1"]`, `{"id":"k1","scope":"person:an","idempotency_key":"b","response_status":201,"response_body":"\\x`+hex.EncodeToString([]byte(body))+`","response_media_type":"application/json"}`),
			row(`["k2"]`, `{"id":"k2","scope":"person:an","idempotency_key":"a","response_status":null,"response_body":null,"response_media_type":null}`),
		},
	}
	s := &Snap{Relations: []*Relation{rel}}

	all := s.StoredResponses()
	if len(all) != 2 || all[0].IdempotencyKey != "a" || all[1].IdempotencyKey != "b" {
		t.Fatalf("snapshot responses not ordered by key: %+v", all)
	}
	if all[0].Body != nil || all[0].Status != 0 || all[0].JSON {
		t.Errorf("incomplete record decoded as %+v", all[0])
	}
	done := all[1]
	if string(done.Body) != body || !done.JSON || done.Status != 201 || done.MediaType != "application/json" {
		t.Errorf("completed record decoded as %+v", done)
	}

	fromChange := Delta(nil, s).StoredResponses()
	if len(fromChange) != 2 {
		t.Errorf("change exposed %d stored responses, want 2", len(fromChange))
	}
	if !strings.Contains(Delta(nil, s).Texts()[0], "idempotency_key") {
		t.Error("texts do not carry the record")
	}
}
