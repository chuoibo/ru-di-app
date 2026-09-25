package companion

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/treejson"
)

func parse(t *testing.T, raw string) tree.Value {
	v, err := pyjson.Loads([]byte(raw))
	if err != nil {
		if t == nil {
			panic(err)
		}
		t.Fatal(err)
	}
	return treejson.To(v)
}

// dump is the stored bytes with the ASCII escapes of json.dumps read back,
// so an expectation can be written in the words a person reads.
func dump(t *testing.T, v tree.Value) string {
	t.Helper()
	raw, err := pyjson.Dumps(treejson.From(v))
	if err != nil {
		t.Fatal(err)
	}
	return escape.ReplaceAllStringFunc(string(raw), func(m string) string {
		n, _ := strconv.ParseUint(m[2:], 16, 32)
		return string(rune(n))
	})
}

var escape = regexp.MustCompile(`\\u[0-9a-f]{4}`)

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

var catalogue = []*tree.OrderedMap{
	parse(nil, `{"id":"p1","name":"Quán Một"}`).(*tree.OrderedMap),
	parse(nil, `{"id":"p2","name":"Quán Hai"}`).(*tree.OrderedMap),
}

var meta = ReplyMeta{InvocationID: "0b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d", Command: "plan", Read: 20}

// Each part is exactly what GroundCard makes of it: the reply adds an envelope,
// never a second opinion about a part.
func TestGroundReplyPartIsGroundCardOutput(t *testing.T) {
	for _, raw := range []string{
		`{"kind":"text","payload":{"text":"Tối nay ăn lẩu nhé"}}`,
		`{"kind":"places","payload":{"intro":"Hai chỗ","place_ids":["p2","p1","p2"]}}`,
		`{"kind":"itinerary","payload":{"title":"Tối thứ Sáu","stops":[{"place_id":"p1","time_text":"19:00","note":"ăn"}]}}`,
		`{"kind":"text","payload":{"text":"` + strings.Repeat("ơ", MaxText+50) + `"}}`,
	} {
		want, err := GroundCard(parse(t, raw), catalogue)
		if err != nil {
			t.Fatal(err)
		}
		got, err := GroundReply(meta, []tree.Value{parse(t, raw)}, catalogue)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		payload, _ := got.Get("payload")
		phan, _ := payload.(*tree.OrderedMap).Get("phan")
		if list := phan.(tree.List); len(list) != 1 || dump(t, list[0]) != dump(t, want) {
			t.Fatalf("part is not GroundCard's output:\n got %s\nwant %s", dump(t, phan), dump(t, want))
		}
	}
}

func TestGroundReplyEnvelope(t *testing.T) {
	got, err := GroundReply(meta, []tree.Value{parse(t, `{"kind":"text","payload":{"text":"Chào cả hội"}}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind": "tra_loi", "payload": {"ban": 1, "tac_gia": "rudi-ai", "invocation_id": "0b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d", "lenh": "plan", "doc": {"so_tin": 20, "chi_loi_nho": false}, "phan": [{"kind": "text", "payload": {"text": "Chào cả hội"}}]}}`
	if s := dump(t, got); s != want {
		t.Fatalf("envelope\n got %s\nwant %s", s, want)
	}
	only := meta
	only.Read = 0
	got, err = GroundReply(only, []tree.Value{parse(t, `{"kind":"text","payload":{"text":"Chào"}}`)}, nil)
	if err != nil || !strings.Contains(dump(t, got), `"doc": {"so_tin": 0, "chi_loi_nho": true}`) {
		t.Fatalf("a reply that read no turn must say it read only the request: %v %s", err, dump(t, got))
	}
}

// A part that fails its check is dropped, not published and not fatal while
// another part stands; one kind at most once; three parts at most.
func TestGroundReplyDropsWhatFailsAndBoundsParts(t *testing.T) {
	parts := []tree.Value{
		parse(t, `{"kind":"places","payload":{"intro":"Bịa","place_ids":["p9"]}}`),
		parse(t, `{"kind":"text","payload":{"text":"Một"}}`),
		parse(t, `{"kind":"text","payload":{"text":"Hai"}}`),
		parse(t, `{"kind":"poll","payload":{"vote_id":"x"}}`),
		parse(t, `{"kind":"places","payload":{"intro":"Thật","place_ids":["p1"]}}`),
		parse(t, `{"kind":"itinerary","payload":{"title":"T","stops":[{"place_id":"p2","time_text":"20:00","note":""}]}}`),
		parse(t, `{"kind":"text","payload":{"text":"Bốn"}}`),
	}
	got, err := GroundReply(meta, parts, catalogue)
	if err != nil {
		t.Fatal(err)
	}
	s := dump(t, got)
	for _, want := range []string{`"text": "Một"`, `"intro": "Thật"`, `"kind": "itinerary"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	for _, never := range []string{"Bịa", "p9", `"Hai"`, "poll", "Bốn"} {
		if strings.Contains(s, never) {
			t.Errorf("kept %s: %s", never, s)
		}
	}
}

func TestGroundReplyRefusals(t *testing.T) {
	text := []tree.Value{parse(t, `{"kind":"text","payload":{"text":"Chào"}}`)}
	for name, m := range map[string]ReplyMeta{
		"no invocation":   {Command: "plan", Read: 1},
		"unknown command": {InvocationID: "x", Command: "vote", Read: 1},
		"negative read":   {InvocationID: "x", Command: "plan", Read: -1},
		"read above cap":  {InvocationID: "x", Command: "plan", Read: MaxReplyRead + 1},
	} {
		if _, err := GroundReply(m, text, nil); code(err) != "companion_reply_malformed" {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, parts := range map[string][]tree.Value{
		"nothing":         nil,
		"all ungrounded":  {parse(t, `{"kind":"places","payload":{"place_ids":["p9"]}}`)},
		"only blank text": {parse(t, `{"kind":"text","payload":{"text":"   "}}`)},
		"nested reply":    {parse(t, `{"kind":"tra_loi","payload":{"phan":[]}}`)},
	} {
		if _, err := GroundReply(meta, parts, catalogue); code(err) != "companion_reply_empty" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// POST /messages grounds with GroundCard alone. That is what keeps a person
// from posting a card that claims to be the AI's answer: the oracle does not
// know the kind, so it refuses it, and GroundReply did not teach it otherwise.
func TestGroundCardStillRefusesAReply(t *testing.T) {
	reply, err := GroundReply(meta, []tree.Value{parse(t, `{"kind":"text","payload":{"text":"Chào"}}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GroundCard(reply, catalogue); code(err) != "companion_card_kind_unknown" {
		t.Fatalf("GroundCard accepted a tra_loi card: %v", err)
	}
}
