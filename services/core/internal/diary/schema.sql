CREATE TABLE outing_endings (
 outing_id uuid PRIMARY KEY REFERENCES outings(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK (kind IN ('moment','trip')),
 ended_by uuid NOT NULL REFERENCES people(id),
 ended_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE outing_diaries (
 id uuid PRIMARY KEY,
 outing_id uuid NOT NULL REFERENCES outing_endings(outing_id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 audience text NOT NULL DEFAULT 'private' CHECK (audience IN ('private','public')),
 revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
 document jsonb NOT NULL CHECK (jsonb_typeof(document)='object'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(outing_id,owner_id)
);
CREATE TABLE outing_diary_versions (
 diary_id uuid NOT NULL REFERENCES outing_diaries(id) ON DELETE CASCADE,
 revision integer NOT NULL,
 document jsonb NOT NULL,
 PRIMARY KEY(diary_id,revision)
);
-- Only the current edition grants access to these image bytes. History is private.
CREATE TABLE outing_diary_photos (
 diary_id uuid NOT NULL REFERENCES outing_diaries(id) ON DELETE CASCADE,
 photo_id uuid NOT NULL REFERENCES uploaded_images(id) ON DELETE CASCADE,
 PRIMARY KEY(diary_id,photo_id)
);
CREATE TABLE outing_diary_jobs (
 id uuid PRIMARY KEY,
 outing_id uuid NOT NULL REFERENCES outing_endings(outing_id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 logical_id uuid NOT NULL,
 session_digest bytea NOT NULL CHECK(octet_length(session_digest)=32),
 digest text NOT NULL,
 source jsonb,
 status text NOT NULL CHECK(status IN ('queued','running','succeeded','failed')),
 result jsonb,
 code text,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 lease_id uuid,
 lease_until timestamptz,
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '1 hour',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(owner_id,logical_id)
);
CREATE INDEX outing_diary_jobs_pending ON outing_diary_jobs(created_at) WHERE status IN ('queued','running');
CREATE INDEX outing_diaries_wall ON outing_diaries(owner_id,created_at DESC,id);
