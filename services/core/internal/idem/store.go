package idem

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// TxStarter is anything that can open a transaction: a *pgxpool.Pool in
// production, a pgx.Tx (which nests as a savepoint) in tests.
type TxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PostgresStore is SqlAlchemyIdempotencyStore behind main.py's
// sqlalchemy_store_factory: every call is one short transaction of its own.
// Reservation must be visible to other processes before the handler runs, so
// it can never ride inside the request's unit of work, and polling in a single
// long transaction would pin a snapshot the winner's commit never reaches.
type PostgresStore struct {
	db TxStarter
}

// NewPostgresStore returns a store over db.
func NewPostgresStore(db TxStarter) *PostgresStore {
	return &PostgresStore{db: db}
}

// The statements SQLAlchemy emits for the Python store. The id is generated
// client-side (Column default uuid.uuid4); created_at is the server default.
const (
	sqlReserve = `INSERT INTO idempotency_keys (id, scope, idempotency_key, request_fingerprint)
VALUES ($1, $2, $3, $4)
ON CONFLICT (scope, idempotency_key) DO NOTHING
RETURNING id`
	sqlExisting = `SELECT request_fingerprint, response_status, response_body, response_media_type
FROM idempotency_keys
WHERE scope = $1 AND idempotency_key = $2`
	sqlAdoptFingerprint = `UPDATE idempotency_keys SET request_fingerprint = $3
WHERE scope = $1 AND idempotency_key = $2`
	sqlComplete = `UPDATE idempotency_keys
SET response_status = $3, response_body = $4, response_media_type = $5, completed_at = now()
WHERE scope = $1 AND idempotency_key = $2`
	sqlRelease = `DELETE FROM idempotency_keys WHERE scope = $1 AND idempotency_key = $2`
)

// Reserve decides the race with a single INSERT ... ON CONFLICT DO NOTHING;
// SELECT-then-INSERT would let two callers both believe they were first.
func (s *PostgresStore) Reserve(ctx context.Context, scope, key, fingerprint, legacyFingerprint string) (Outcome, error) {
	id, err := newUUID()
	if err != nil {
		return Outcome{}, err
	}
	var outcome Outcome
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var inserted pgtype.UUID
		switch err := tx.QueryRow(ctx, sqlReserve, id, scope, key, fingerprint).Scan(&inserted); {
		case err == nil:
			outcome = Outcome{Kind: Reserved}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		var (
			stored    string
			status    *int32
			body      []byte
			mediaType *string
		)
		err := tx.QueryRow(ctx, sqlExisting, scope, key).Scan(&stored, &status, &body, &mediaType)
		if errors.Is(err, pgx.ErrNoRows) {
			// Released between the insert and the read. Refusing is the safe
			// direction; the client retries.
			outcome = Outcome{Kind: InFlight}
			return nil
		}
		if err != nil {
			return err
		}
		if stored != fingerprint {
			if legacyFingerprint == "" || stored != legacyFingerprint {
				outcome = Outcome{Kind: Conflict}
				return nil
			}
			// Written by a server that hashed these very bytes verbatim: same
			// request, older spelling. Adopt the canonical digest in place.
			if _, err := tx.Exec(ctx, sqlAdoptFingerprint, scope, key, fingerprint); err != nil {
				return err
			}
		}
		if status == nil {
			outcome = Outcome{Kind: InFlight}
			return nil
		}
		if body == nil {
			body = []byte{}
		}
		outcome = Outcome{Kind: Replay, Response: StoredResponse{
			Status:    int(*status),
			Body:      body,
			MediaType: mediaType,
		}}
		return nil
	})
	if err != nil {
		return Outcome{}, err
	}
	return outcome, nil
}

// Complete records the answer that is about to be sent.
func (s *PostgresStore) Complete(ctx context.Context, scope, key string, response StoredResponse) error {
	body := response.Body
	if body == nil {
		body = []byte{} // b"" in Python, never NULL
	}
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sqlComplete, scope, key, int32(response.Status), body, response.MediaType)
		return err
	})
}

// Release frees the key so a retry is a genuine second attempt.
func (s *PostgresStore) Release(ctx context.Context, scope, key string) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sqlRelease, scope, key)
		return err
	})
}

// newUUID is uuid.uuid4().
func newUUID() (pgtype.UUID, error) {
	var id pgtype.UUID
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		return id, err
	}
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x40
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id, nil
}
