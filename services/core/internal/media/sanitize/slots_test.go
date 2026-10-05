package sanitize

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Decodes run at most cap(slots) at once; a caller whose context ends while
// every slot is busy gives up instead of queueing without end.
func TestDecodesWaitForASlotOnlyAsLongAsTheirContext(t *testing.T) {
	for range cap(slots) {
		slots <- struct{}{}
	}
	defer func() {
		for range cap(slots) {
			<-slots
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := SanitizeContext(ctx, []byte("not an image"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("with every slot busy: %v", err)
	}
}
