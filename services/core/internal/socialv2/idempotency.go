package socialv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// beginAttempt serializes concurrent requests carrying the same key inside
// PostgreSQL's unique index. The key and result commit in the same transaction.
func beginAttempt(ctx context.Context, tx pgx.Tx, header http.Header, actor, kind, request string) (*string, error) {
	key := header.Get("Idempotency-Key")
	if key == "" {
		return nil, nil
	}
	if len(key) > 128 {
		return nil, bad(422, "invalid_idempotency_key")
	}
	digest := sha256.Sum256([]byte(request))
	var inserted string
	err := tx.QueryRow(ctx, `INSERT INTO social_mutation_keys(actor_id,request_key,kind,request_hash)
 VALUES($1::uuid,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING request_key`, actor, key, kind, digest[:]).Scan(&inserted)
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var oldKind string
	var oldDigest []byte
	var result *string
	err = tx.QueryRow(ctx, `SELECT kind,request_hash,result_id FROM social_mutation_keys WHERE actor_id=$1::uuid AND request_key=$2`, actor, key).Scan(&oldKind, &oldDigest, &result)
	if err != nil {
		return nil, err
	}
	if oldKind != kind || !bytes.Equal(oldDigest, digest[:]) {
		return nil, bad(422, "idempotency_key_reuse")
	}
	if result == nil {
		return nil, bad(409, "idempotency_request_in_flight")
	}
	return result, nil
}

func finishAttempt(ctx context.Context, tx pgx.Tx, header http.Header, actor, resultID string) error {
	key := header.Get("Idempotency-Key")
	if key == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE social_mutation_keys SET result_id=$3::uuid WHERE actor_id=$1::uuid AND request_key=$2`, actor, key, resultID)
	return err
}
