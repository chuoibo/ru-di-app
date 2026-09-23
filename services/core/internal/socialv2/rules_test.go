package socialv2

import (
	"testing"
	"time"
)

func TestWallCursorRoundTripAndRejectsTampering(t *testing.T) {
	created := time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	cursor := encodeWallCursor(created, id)
	got, err := decodeWallCursor(cursor)
	if err != nil || !got.CreatedAt.Equal(created) || got.ID != id {
		t.Fatalf("roundtrip = %+v, %v", got, err)
	}
	for _, bad := range []string{"", cursor + "!", "eyJpZCI6ImZha2UifQ", "../../etc/passwd"} {
		if _, err := decodeWallCursor(bad); err == nil {
			t.Errorf("accepted malformed cursor %q", bad)
		}
	}
}

func TestReplyMustAttachOnlyToTopLevelCommentOnSamePost(t *testing.T) {
	post := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	parent := commentParent{PostID: post}
	if !replyParentAllowed(post, parent) {
		t.Fatal("top-level comment in the post should accept a reply")
	}
	parent.ParentID = "ffffffff-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	if replyParentAllowed(post, parent) {
		t.Fatal("reply to a reply created a second nesting level")
	}
	parent.ParentID = ""
	if replyParentAllowed("bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee", parent) {
		t.Fatal("cross-post reply was accepted")
	}
}

func TestWallWakeHintTargetsOnlyMatchingOwner(t *testing.T) {
	h := New(nil, "dev")
	a, leaveA := h.subscribe("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	defer leaveA()
	b, leaveB := h.subscribe("bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	defer leaveB()
	h.wake("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	select {
	case <-a:
	default:
		t.Fatal("owner subscriber was not woken")
	}
	select {
	case <-b:
		t.Fatal("unrelated wall subscriber was woken")
	default:
	}
}
