CREATE TABLE profile_media_jobs (
 id text PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 credit_source text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('nep_video','album_video')),
 status text NOT NULL CHECK(status IN ('reserved','queued','running','ready','failed')),
 idempotency_key text NOT NULL,
 payload jsonb NOT NULL,
 result_key text,
 error_code text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(person_id,credit_source) REFERENCES achievement_mp4_credits(person_id,source) ON DELETE CASCADE,
 UNIQUE(person_id,idempotency_key)
);
CREATE UNIQUE INDEX profile_media_one_spend_per_credit ON profile_media_jobs(person_id,credit_source)
 WHERE status IN ('reserved','queued','running','ready');
CREATE INDEX profile_media_jobs_worker ON profile_media_jobs(status,created_at) WHERE status IN ('reserved','queued','running');
