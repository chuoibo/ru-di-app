-- ADR-0056: amending an expense after its collection batch was frozen or
-- published. Every table but collection_amendments is append-only; an
-- amendment's own row only moves once, from proposed to where it ended.
CREATE TABLE IF NOT EXISTS collection_amendments (
  id uuid PRIMARY KEY,
  batch_id uuid NOT NULL REFERENCES collection_batches(id),
  base_batch_version_id uuid NOT NULL REFERENCES collection_batch_versions(id),
  expense_id uuid NOT NULL REFERENCES expenses(id),
  replaced_expense_version_id uuid NOT NULL REFERENCES expense_versions(id),
  proposed_by_id uuid NOT NULL REFERENCES people(id),
  reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
  -- The confirmation the amendment applies: the expense input and the
  -- allocator's allocations, exactly as POST /expenses/{id}/confirm takes them.
  confirmation jsonb NOT NULL,
  status text NOT NULL CHECK (status IN ('proposed','applied','rejected','expired')),
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
  resolved_at timestamptz,
  applied_batch_version_id uuid REFERENCES collection_batch_versions(id),
  CHECK ((status = 'proposed') = (resolved_at IS NULL)),
  CHECK ((status = 'applied') = (applied_batch_version_id IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS collection_amendments_one_open
  ON collection_amendments (batch_id) WHERE status = 'proposed';

-- Every pair whose amount changes: old 0 is a new edge, new 0 a removed one.
CREATE TABLE IF NOT EXISTS collection_amendment_lines (
  amendment_id uuid NOT NULL REFERENCES collection_amendments(id),
  sender_id uuid NOT NULL REFERENCES people(id),
  recipient_id uuid NOT NULL REFERENCES people(id),
  old_amount_vnd bigint NOT NULL CHECK (old_amount_vnd >= 0),
  new_amount_vnd bigint NOT NULL CHECK (new_amount_vnd >= 0),
  PRIMARY KEY (amendment_id, sender_id, recipient_id),
  CHECK (old_amount_vnd <> new_amount_vnd),
  CHECK (sender_id <> recipient_id)
);

-- The people who must accept: both ends of every line.
CREATE TABLE IF NOT EXISTS collection_amendment_parties (
  amendment_id uuid NOT NULL REFERENCES collection_amendments(id),
  person_id uuid NOT NULL REFERENCES people(id),
  PRIMARY KEY (amendment_id, person_id)
);

CREATE TABLE IF NOT EXISTS collection_amendment_decisions (
  amendment_id uuid NOT NULL,
  person_id uuid NOT NULL,
  accept boolean NOT NULL,
  via text NOT NULL CHECK (via IN ('app','guest_link','proposer')),
  decided_at timestamptz NOT NULL,
  PRIMARY KEY (amendment_id, person_id),
  FOREIGN KEY (amendment_id, person_id) REFERENCES collection_amendment_parties (amendment_id, person_id)
);

-- A sender's review link on a published batch: their live link was rotated
-- when the amendment was proposed, and this token answers
-- /g/{token}/dieu-chinh until it resolves; then the same digest becomes a
-- guest_links row, to the new envelope if applied, else back to the old one
-- (or nowhere, for a sender the amendment would have added).
CREATE TABLE IF NOT EXISTS collection_amendment_links (
  token_digest bytea PRIMARY KEY CHECK (octet_length(token_digest) = 32),
  amendment_id uuid NOT NULL REFERENCES collection_amendments(id),
  sender_id uuid NOT NULL REFERENCES people(id),
  -- The live link it replaced; none for a sender the amendment adds, who
  -- had nothing to open before.
  old_link_id uuid UNIQUE REFERENCES guest_links(id),
  created_at timestamptz NOT NULL,
  -- What the link this token becomes expires at: the replaced link's expiry,
  -- or the batch's latest one for an added sender.
  expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
  UNIQUE (amendment_id, sender_id)
);

-- An obligation of an applied amendment's batch version and the obligation of
-- the same pair it replaces: receipts on the old one count toward the new.
CREATE TABLE IF NOT EXISTS collection_obligation_successions (
  old_obligation_id uuid PRIMARY KEY REFERENCES collection_obligations(id),
  new_obligation_id uuid NOT NULL UNIQUE REFERENCES collection_obligations(id),
  amendment_id uuid NOT NULL REFERENCES collection_amendments(id),
  CHECK (old_obligation_id <> new_obligation_id)
);

CREATE OR REPLACE FUNCTION collection_amendment_append_only() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'append_only' USING ERRCODE = 'restrict_violation';
END
$$ LANGUAGE plpgsql;

-- The only change an amendment row takes: proposed -> applied, rejected or
-- expired, once, with its resolution fields; nothing else may move.
CREATE OR REPLACE FUNCTION collection_amendment_resolve_only() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' OR OLD.status <> 'proposed' OR NEW.status = 'proposed'
     OR (NEW.id, NEW.batch_id, NEW.base_batch_version_id, NEW.expense_id, NEW.replaced_expense_version_id,
         NEW.proposed_by_id, NEW.reason, NEW.confirmation, NEW.created_at, NEW.expires_at)
        IS DISTINCT FROM
        (OLD.id, OLD.batch_id, OLD.base_batch_version_id, OLD.expense_id, OLD.replaced_expense_version_id,
         OLD.proposed_by_id, OLD.reason, OLD.confirmation, OLD.created_at, OLD.expires_at) THEN
    RAISE EXCEPTION 'amendment_resolved_once' USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS collection_amendments_resolve_only ON collection_amendments;
CREATE TRIGGER collection_amendments_resolve_only BEFORE UPDATE OR DELETE ON collection_amendments
  FOR EACH ROW EXECUTE FUNCTION collection_amendment_resolve_only();
DROP TRIGGER IF EXISTS collection_amendment_lines_append_only ON collection_amendment_lines;
CREATE TRIGGER collection_amendment_lines_append_only BEFORE UPDATE OR DELETE ON collection_amendment_lines
  FOR EACH ROW EXECUTE FUNCTION collection_amendment_append_only();
DROP TRIGGER IF EXISTS collection_amendment_parties_append_only ON collection_amendment_parties;
CREATE TRIGGER collection_amendment_parties_append_only BEFORE UPDATE OR DELETE ON collection_amendment_parties
  FOR EACH ROW EXECUTE FUNCTION collection_amendment_append_only();
DROP TRIGGER IF EXISTS collection_amendment_decisions_append_only ON collection_amendment_decisions;
CREATE TRIGGER collection_amendment_decisions_append_only BEFORE UPDATE OR DELETE ON collection_amendment_decisions
  FOR EACH ROW EXECUTE FUNCTION collection_amendment_append_only();
DROP TRIGGER IF EXISTS collection_amendment_links_append_only ON collection_amendment_links;
CREATE TRIGGER collection_amendment_links_append_only BEFORE UPDATE OR DELETE ON collection_amendment_links
  FOR EACH ROW EXECUTE FUNCTION collection_amendment_append_only();
DROP TRIGGER IF EXISTS collection_obligation_successions_append_only ON collection_obligation_successions;
CREATE TRIGGER collection_obligation_successions_append_only BEFORE UPDATE OR DELETE ON collection_obligation_successions
  FOR EACH ROW EXECUTE FUNCTION collection_amendment_append_only();
