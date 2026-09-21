package routes

import (
	"context"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"testing"
)

func TestCandidateSlashCannotImplicitlyReadHistory(t *testing.T) {
	for _, body := range []string{"/plan đi ăn", "@Rủ Đi gợi ý", "/chia-bill"} {
		posted := pyjson.NewOrderedMap()
		// No repository or limiter is supplied. Any legacy implicit-history route
		// would dereference these dependencies instead of reaching this guard.
		out, err := actOnMessageIntent(context.Background(), &endpoint.Call{ExplicitChatInvocation: true}, repo.Repository{}, "synthetic", posted, repo.Message{Kind: "text", Body: &body})
		if err != nil {
			t.Fatal(err)
		}
		code, _ := out.Get("intent_error")
		if code != pyjson.String("explicit_invocation_required") {
			t.Fatalf("%q: %v", body, code)
		}
	}
}
