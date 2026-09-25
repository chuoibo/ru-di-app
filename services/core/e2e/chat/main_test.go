//go:build e2e

package chate2e

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

// TestMain refuses to run without a stack. A suite that skips itself is worse
// than a suite that does not exist: it reports green and measures nothing.
func TestMain(m *testing.M) {
	if os.Getenv("CHAT_E2E_SESSIONS") == "" {
		fmt.Fprintln(os.Stderr,
			"E2E chat: thiếu CHAT_E2E_SESSIONS. Dựng stack bằng scripts/chat_e2e_go.sh.")
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// TestChatE2EReachesGoFrontDoor is the tier sentinel. The runner greps for it,
// so a run that exercised nothing cannot be reported as a pass.
//
// It also proves *which* process answered. `/contexts/{id}/changes` exists only
// in the Go core: it is not in the route manifest and Python has no such route,
// so a 200 here means the Go front door served the request rather than proxying
// it upstream. Without this check the whole suite could be measuring Python.
func TestChatE2EReachesGoFrontDoor(t *testing.T) {
	stack := Load(t)
	client := stack.As(t, 0)

	response := client.Do("GET", "/contexts/"+stack.GroupID+"/changes?limit=1", nil)
	if response.Status != http.StatusOK {
		t.Fatalf("feed thay đổi trả %d: hoặc core Go không phục vụ route này, "+
			"hoặc yêu cầu đã bị proxy sang Python — %s", response.Status, response.trim())
	}
	if _, ok := response.JSON["next_sequence"]; !ok {
		t.Fatalf("câu trả lời không có hình dạng feed của Go: %s", response.trim())
	}

	// The old automatic turn is deleted in both backends (ADR-0036 §2.1): Go
	// has no route for it, so the front door proxies it and Python answers
	// 404. Anything else means one side still serves it.
	gone := client.Do("POST", "/contexts/"+stack.GroupID+"/ai-turn",
		map[string]any{"prompt": "xin chào"}, Idem(newKey()))
	if gone.Status != http.StatusNotFound {
		t.Fatalf("ai-turn cũ phải không còn (404), nhận %d — %s", gone.Status, gone.trim())
	}
}
