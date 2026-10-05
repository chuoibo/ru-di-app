package chatv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/media/storage"
)

// MaxMedia is the largest ciphertext the lane stores: 25 MiB of sealed file.
const MaxMedia = 25 << 20

// maxMediaPerDay bounds one device's uploads in 24 hours.
const maxMediaPerDay = 300

// MediaStore keeps the ciphertext bytes; nil means media is off.
type MediaStore interface {
	Write(key string, data []byte) error
	Read(key string) ([]byte, error)
}

// WithMedia gives the store a place for ciphertext.
func (s *Store) WithMedia(m MediaStore) *Store { s.media = m; return s }

// PutMedia stores one sealed file under the id the device chose (it is part of
// what was sealed). The same bytes again are the same upload; other bytes
// under a taken id are a conflict.
func (s *Store) PutMedia(ctx context.Context, actor string, digest []byte, device, conversation, mediaID string, data []byte) ([]byte, error) {
	if s.media == nil {
		return nil, ErrNotReady
	}
	if !ValidID(mediaID) || len(data) < 1 || len(data) > MaxMedia+64 {
		return nil, ErrInvalid
	}
	sum := sha256.Sum256(data)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := authorizeSessionWithLock(ctx, tx, actor, device, conversation, lockShare, digest); err != nil {
		return nil, err
	}
	var existing []byte
	var room string
	err = tx.QueryRow(ctx, `SELECT sha256, context_id::text FROM chat_v2_media WHERE id=$1`, mediaID).Scan(&existing, &room)
	if err == nil {
		if room != conversation || !bytes.Equal(existing, sum[:]) {
			return nil, ErrConflict
		}
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var today int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM chat_v2_media WHERE uploader_device=$1 AND created_at>clock_timestamp()-interval '1 day'`, device).Scan(&today); err != nil {
		return nil, err
	}
	if today >= maxMediaPerDay {
		return nil, ErrCapacity
	}
	key, err := storage.NewStorageKey()
	if err != nil {
		return nil, err
	}
	if err := s.media.Write(key, data); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_v2_media(id,context_id,uploader_device,storage_key,size,sha256) VALUES($1,$2,$3,$4,$5,$6)`,
		mediaID, conversation, device, key, len(data), sum[:]); err != nil {
		return nil, err
	}
	return sum[:], tx.Commit(ctx)
}

// GetMedia answers a sealed file of the room to one of its member devices.
func (s *Store) GetMedia(ctx context.Context, actor string, digest []byte, device, conversation, mediaID string) ([]byte, error) {
	if s.media == nil {
		return nil, ErrNotReady
	}
	if !ValidID(mediaID) {
		return nil, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := authorizeSessionWithLock(ctx, tx, actor, device, conversation, lockShare, digest); err != nil {
		return nil, err
	}
	var key string
	err = tx.QueryRow(ctx, `SELECT storage_key FROM chat_v2_media WHERE id=$1 AND context_id=$2`, mediaID, conversation).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	data, err := s.media.Read(strings.ToLower(key))
	if err != nil {
		return nil, err
	}
	return data, nil
}
