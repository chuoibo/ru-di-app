package endpoint

import (
	"context"
	"testing"

	"mobile/services/core/internal/pyjson"
)

// A route answering like Starlette's `Response(status_code=204)` sends the
// status alone: no body and no content headers, whatever Body holds.
func TestAnEmptyReplyIsTheStatusAlone(t *testing.T) {
	valid := `{"target_type":"post","target_id":"` + targetID + `","reason":"other","note":null}`
	body := pyjson.NewOrderedMap()
	body.Set("ignored", pyjson.Bool(true))
	rec := send(front(t, func(context.Context, *Call) (Reply, error) {
		return Reply{Status: 204, Empty: true, Body: body}, nil
	}), valid, signedIn)
	result := rec.Result()
	if result.StatusCode != 204 || rec.Body.Len() != 0 {
		t.Fatalf("got %d %q", result.StatusCode, rec.Body)
	}
	for _, name := range []string{"Content-Type", "Content-Length"} {
		if values, ok := result.Header[name]; ok {
			t.Fatalf("an empty reply carries %s %q", name, values)
		}
	}
}
