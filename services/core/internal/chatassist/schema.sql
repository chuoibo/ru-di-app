CREATE TABLE chat_ai_invocations (
    id uuid PRIMARY KEY,
    context_id uuid NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    membership_id uuid NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    session_digest bytea NOT NULL CHECK (octet_length(session_digest) = 32),
    logical_id uuid NOT NULL,
    input_digest bytea NOT NULL CHECK (octet_length(input_digest) = 32),
    command text NOT NULL CHECK (command = 'plan'),
    prompt text CHECK (length(prompt) BETWEEN 1 AND 4000),
    share_expires_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 3),
    lease_id uuid,
    lease_until timestamptz,
    code text,
    message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (context_id, person_id, logical_id),
    CHECK ((status = 'running') = (lease_id IS NOT NULL AND lease_until IS NOT NULL))
);
CREATE INDEX chat_ai_jobs_pending ON chat_ai_invocations(created_at)
    WHERE status IN ('queued','running');
CREATE INDEX chat_ai_jobs_person ON chat_ai_invocations(context_id,person_id,created_at DESC);

CREATE TABLE chat_plan_promotions (
    context_id uuid NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    source_message_id uuid NOT NULL REFERENCES messages(id),
    outing_id uuid NOT NULL REFERENCES outings(id),
    input_digest bytea NOT NULL CHECK (octet_length(input_digest)=32),
    created_by_id uuid NOT NULL REFERENCES people(id),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(context_id,source_message_id),
    UNIQUE(outing_id)
);

-- Leaving and rejoining the same legacy membership cannot revive sharing.
CREATE FUNCTION chat_ai_membership_revoked() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state = 'active' AND NEW.state <> 'active' THEN
        UPDATE chat_ai_invocations SET status='cancelled',code='sharing_revoked',
            prompt=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp()
        WHERE membership_id=OLD.id AND status IN ('queued','running','failed');
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_ai_membership_revoked AFTER UPDATE OF state ON memberships
    FOR EACH ROW EXECUTE FUNCTION chat_ai_membership_revoked();
