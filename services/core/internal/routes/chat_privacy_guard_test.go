package routes

import (
	"context"
	"testing"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// TestOldAICommandsAreOrdinaryText pins ADR-0036 §2.1: `/plan`, `@Rủ Đi` and
// `/chia-bill` posted as a message reach no model, no limiter and no history.
// No repository, limiter or actor is supplied, so any branch that still tried
// to take a companion turn or read the room would dereference one of them.
func TestOldAICommandsAreOrdinaryText(t *testing.T) {
	for _, body := range []string{"/plan đi ăn", "@Rủ Đi gợi ý", "/chia-bill", "/chiabill 3 người"} {
		posted := pyjson.NewOrderedMap()
		out, err := actOnMessageIntent(context.Background(), &endpoint.Call{}, repo.Repository{}, "synthetic", posted, repo.Message{Kind: "text", Body: &body})
		if err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		keys := []string{}
		for key, value := range out.All() {
			keys = append(keys, key)
			if _, null := value.(pyjson.Null); !null {
				t.Fatalf("%q: %s = %v, want null", body, key, value)
			}
		}
		if len(keys) != 3 || keys[0] != "intent" || keys[1] != "vote" || keys[2] != "intent_error" {
			t.Fatalf("%q: posted-message fields %v, want [intent vote intent_error]", body, keys)
		}
	}
}
