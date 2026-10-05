package chatv2

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"mobile/services/core/internal/media/storage"
)

// ADR-0057 §5.2 on a real PostgreSQL: the lane keeps sealed bytes under the
// id the device chose; the same bytes again are the same upload, others a
// conflict; only a member device of the room reads them back.
func TestMediaIsCiphertextUnderTheChosenIDForMembersOnly(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	disk, err := storage.NewAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.PutMedia(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, id(), []byte("x")); !errors.Is(err, ErrNotReady) {
		t.Fatalf("media without a store: %v", err)
	}
	l.store.WithMedia(disk)
	if _, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room); err != nil {
		t.Fatal(err)
	}
	sealed := bytes.Repeat([]byte{0xA5}, 4096)
	media := id()
	if _, err := l.store.PutMedia(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, media, sealed); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.PutMedia(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, media, sealed); err != nil {
		t.Fatalf("the same upload again: %v", err)
	}
	if _, err := l.store.PutMedia(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, media, []byte("other")); !errors.Is(err, ErrConflict) {
		t.Fatalf("other bytes under a taken id: %v", err)
	}
	got, err := l.store.GetMedia(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, media)
	if err != nil || !bytes.Equal(got, sealed) {
		t.Fatalf("read back: %d bytes %v", len(got), err)
	}
	// Bình is in the room on the server but his device is not an MLS member yet.
	if _, err := l.store.GetMedia(ctx, l.other, l.tokenB, l.deviceB.id, l.room, media); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a non-member device read the media: %v", err)
	}
	if _, err := l.store.PutMedia(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, id(), make([]byte, MaxMedia+65)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an oversize upload: %v", err)
	}
}
