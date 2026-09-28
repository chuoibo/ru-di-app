package companion

import (
	"strings"
	"testing"

	"mobile/services/core/internal/domain/tree"
)

// The group ceiling, not the oracle's: a text over MaxText but within
// MaxReplyText is kept whole; one over MaxReplyText is cut at the last
// sentence end inside it, or at the ceiling when there is none.
func TestGroundReplyTextCeiling(t *testing.T) {
	text := func(s string) []tree.Value {
		return []tree.Value{parse(t, `{"kind":"text","payload":{"text":"`+s+`"}}`)}
	}
	chu := func(got *tree.OrderedMap) string {
		payload, _ := got.Get("payload")
		phan, _ := payload.(*tree.OrderedMap).Get("phan")
		body, _ := phan.(tree.List)[0].(*tree.OrderedMap).Get("payload")
		v, _ := body.(*tree.OrderedMap).Get("text")
		return string(v.(tree.String))
	}
	whole := strings.Repeat("ơ", MaxText+50)
	got, err := GroundReply(meta, text(whole), nil)
	if err != nil || chu(got) != whole {
		t.Fatalf("a %d-rune text was not kept whole: %v", MaxText+50, err)
	}
	sentence := strings.Repeat("a", 1000) + ". " + strings.Repeat("b", 800)
	got, err = GroundReply(meta, text(sentence), nil)
	if err != nil || chu(got) != strings.Repeat("a", 1000)+"." {
		t.Fatalf("an over-long text was not cut at its last sentence end: %v %d", err, len([]rune(chu(got))))
	}
	flat := strings.Repeat("c", MaxReplyText+10)
	got, err = GroundReply(meta, text(flat), nil)
	if err != nil || len([]rune(chu(got))) != MaxReplyText {
		t.Fatalf("a text with no sentence end was not cut at the ceiling: %v", err)
	}
	if _, err := GroundReply(meta, text(strings.Repeat("d", MaxReplyText)), nil); err != nil {
		t.Fatalf("a text of exactly the ceiling was refused: %v", err)
	}
}

// expense_draft is a pointer: so_khoan and da_ghi, nothing else. An amount,
// a payer, a count out of range or a bad index drops the part.
func TestGroundReplyExpenseDraft(t *testing.T) {
	ok := `{"kind":"expense_draft","payload":{"so_khoan":3,"da_ghi":[]}}`
	got, err := GroundReply(ReplyMeta{InvocationID: "x", Command: "chia_bill", Read: 4}, []tree.Value{
		parse(t, `{"kind":"text","payload":{"text":"Đề xuất chia bill"}}`), parse(t, ok)}, nil)
	if err != nil || !strings.Contains(dump(t, got), `{"kind": "expense_draft", "payload": {"so_khoan": 3, "da_ghi": []}}`) {
		t.Fatalf("a valid pointer was not kept: %v", err)
	}
	for _, bad := range []string{
		`{"kind":"expense_draft","payload":{"so_khoan":3,"da_ghi":[],"amount_vnd":850000}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":3,"da_ghi":[],"paid_by_id":"p"}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":0,"da_ghi":[]}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":9,"da_ghi":[]}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":2,"da_ghi":[2]}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":2,"da_ghi":[1,1]}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":2}}`,
		`{"kind":"expense_draft","payload":{"so_khoan":"2","da_ghi":[]}}`,
	} {
		if _, err := GroundReply(meta, []tree.Value{parse(t, bad)}, nil); code(err) != "companion_reply_empty" {
			t.Errorf("%s: %v", bad, err)
		}
	}
	// da_ghi a person marked by a real expense line is kept as it is.
	got, err = GroundReply(meta, []tree.Value{parse(t, `{"kind":"expense_draft","payload":{"so_khoan":3,"da_ghi":[2,0]}}`)}, nil)
	if err != nil || !strings.Contains(dump(t, got), `"da_ghi": [2, 0]`) {
		t.Fatalf("da_ghi not kept: %v", err)
	}
}
