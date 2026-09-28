package profilemedia

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoCredit = errors.New("no video credit available")
var ErrKeyReuse = errors.New("idempotency key reused for different media request")

type Job struct {
	ID             string
	PersonID       string
	CreditSource   string
	Kind           string
	Status         string
	IdempotencyKey string
	CreatedAt      time.Time
}

type Balance struct {
	Granted   int `json:"granted"`
	Used      int `json:"used"`
	Available int `json:"available"`
}

// Reserve locks one grant row before inserting a durable, idempotent job.
func Reserve(ctx context.Context, pool *pgxpool.Pool, personID, idempotencyKey, jobID, kind string, payload []byte) (Job, error) {
	if idempotencyKey == "" || len(idempotencyKey) > 128 || jobID == "" || len(payload) == 0 || (kind != "nep_video" && kind != "album_video") {
		return Job{}, fmt.Errorf("invalid media reservation")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	// A stable transaction lock serializes the same click before either caller
	// can observe an absent row. Other clicks can still reserve other grants.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, personID+":"+idempotencyKey); err != nil {
		return Job{}, err
	}
	var old Job
	var samePayload bool
	err = tx.QueryRow(ctx, `SELECT id,person_id::text,credit_source,kind,status,idempotency_key,created_at,payload=$3::jsonb
	 FROM profile_media_jobs WHERE person_id=$1::uuid AND idempotency_key=$2 FOR UPDATE`, personID, idempotencyKey, string(payload)).
		Scan(&old.ID, &old.PersonID, &old.CreditSource, &old.Kind, &old.Status, &old.IdempotencyKey, &old.CreatedAt, &samePayload)
	if err == nil {
		if old.Kind != kind || !samePayload {
			return Job{}, ErrKeyReuse
		}
		return old, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, err
	}
	var source string
	err = tx.QueryRow(ctx, `SELECT c.source FROM achievement_mp4_credits c
	 WHERE c.person_id=$1::uuid AND NOT EXISTS (
	   SELECT 1 FROM profile_media_jobs j WHERE j.person_id=c.person_id AND j.credit_source=c.source
	    AND j.status IN ('reserved','queued','running','ready'))
	 ORDER BY c.granted_at,c.source LIMIT 1 FOR UPDATE OF c SKIP LOCKED`, personID).Scan(&source)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNoCredit
	}
	if err != nil {
		return Job{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO profile_media_jobs(id,person_id,credit_source,kind,status,idempotency_key,payload)
	 VALUES($1,$2::uuid,$3,$4,'reserved',$5,$6::jsonb)
	 RETURNING id,person_id::text,credit_source,kind,status,idempotency_key,created_at`, jobID, personID, source, kind, idempotencyKey, string(payload)).
		Scan(&old.ID, &old.PersonID, &old.CreditSource, &old.Kind, &old.Status, &old.IdempotencyKey, &old.CreatedAt)
	if err != nil {
		return Job{}, err
	}
	return old, tx.Commit(ctx)
}

// SetStatus makes a terminal state immutable; failed jobs release the credit.
func SetStatus(ctx context.Context, pool *pgxpool.Pool, personID, jobID, status, errorCode, resultKey string) error {
	switch status {
	case "queued", "running", "ready", "failed":
	default:
		return fmt.Errorf("invalid media state")
	}
	command, err := pool.Exec(ctx, `UPDATE profile_media_jobs SET status=$3,error_code=NULLIF($4,''),result_key=NULLIF($5,''),updated_at=clock_timestamp()
	 WHERE person_id=$1::uuid AND id=$2 AND (
	   (status='reserved' AND $3 IN ('queued','running','ready','failed')) OR
	   (status='queued' AND $3 IN ('running','ready','failed')) OR
	   (status='running' AND $3 IN ('running','ready','failed'))
	 )`, personID, jobID, status, errorCode, resultKey)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func CreditBalance(ctx context.Context, pool *pgxpool.Pool, personID string) (Balance, error) {
	var b Balance
	err := pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM achievement_mp4_credits WHERE person_id=$1::uuid),
	 (SELECT count(*) FROM profile_media_jobs WHERE person_id=$1::uuid AND status='ready'),
	 (SELECT count(*) FROM achievement_mp4_credits c WHERE c.person_id=$1::uuid AND NOT EXISTS
	   (SELECT 1 FROM profile_media_jobs j WHERE j.person_id=c.person_id AND j.credit_source=c.source
	     AND j.status IN ('reserved','queued','running','ready')))`, personID).
		Scan(&b.Granted, &b.Used, &b.Available)
	return b, err
}
