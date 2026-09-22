-- A shared draft is the group's own tờ hẹn: a plan card people edit together
-- before anyone commits to it. It is anchored to one `ai_card` message so the
-- existing change feed, snapshot lane and promotion path all work unchanged --
-- the card is the projection, this table is the source of truth.
CREATE TABLE chat_shared_drafts (
 id uuid PRIMARY KEY,
 context_id uuid NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
 -- One anchor per draft and one draft per anchor: the thread never shows the
 -- same sheet twice, and an edit always knows which card to repaint.
 message_id uuid NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
 -- Deliberately not a foreign key. The handler checks in the same
 -- transaction that the poll exists in this room and is closed, under the
 -- same feed lock a promotion takes. A key here would also mean a deleted
 -- poll quietly nulls or removes the sheet, and the sheet is the record of
 -- what the group decided -- losing its provenance is worse than a
 -- dangling id. It also kept the isolated test schema reaching into public.
 source_vote_id uuid,
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 title text NOT NULL CHECK (btrim(title) <> ''),
 starts_on date,
 ends_on date,
 headcount integer CHECK (headcount IS NULL OR headcount > 0),
 budget_per_person_vnd bigint CHECK (budget_per_person_vnd IS NULL OR budget_per_person_vnd >= 0),
 stops jsonb NOT NULL DEFAULT '[]'::jsonb,
 status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','promoted','discarded')),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
-- A decided poll feeds exactly one open sheet. This is what lets the screen
-- answer "which tờ hẹn is receiving this choice" instead of guessing.
CREATE UNIQUE INDEX chat_shared_drafts_one_open_per_vote
 ON chat_shared_drafts(source_vote_id)
 WHERE status = 'open' AND source_vote_id IS NOT NULL;
CREATE INDEX chat_shared_drafts_room ON chat_shared_drafts(context_id, created_at DESC);
-- Append-only, like every other ledger here: the current row is a cache of
-- this list, and "who changed what" survives the next edit.
CREATE TABLE chat_shared_draft_edits (
 draft_id uuid NOT NULL REFERENCES chat_shared_drafts(id) ON DELETE CASCADE,
 revision bigint NOT NULL CHECK (revision > 0),
 editor_id uuid NOT NULL,
 patch jsonb NOT NULL,
 at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (draft_id, revision)
);
