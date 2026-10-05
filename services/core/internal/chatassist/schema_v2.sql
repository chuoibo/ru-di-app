-- Version 7: the group AI in an end-to-end room (ADR-0057 §6).
--
-- The server holds no key to a v2 room, so it never posts there. The caller's
-- device asked, with the turns it chose to share in clear (the caller-attached
-- bundle, as in a legacy room); the server keeps the answer for that caller
-- only, and the caller's device seals it into the room as an `ai_card`. The
-- other members check what arrived against `result_digest`.

ALTER TABLE chat_ai_invocations
    -- The `@Rủ Đi` message on the v2 lane: its logical send id. A v2 room has
    -- no row in `messages`, so trigger_message_id stays NULL there.
    ADD COLUMN trigger_v2 uuid,
    -- The answer, byte for byte, until the caller's device delivered it into
    -- the room (or it went stale): text, not jsonb, so its digest is the
    -- digest of what the device seals.
    ADD COLUMN v2_result text CHECK (v2_result IS NULL OR octet_length(v2_result) <= 12288),
    -- SHA-256 of v2_result, kept after the answer itself is dropped.
    ADD COLUMN result_digest bytea CHECK (result_digest IS NULL OR octet_length(result_digest) = 32),
    -- Where the caller's device posted it on the lane.
    ADD COLUMN delivered_sequence bigint CHECK (delivered_sequence IS NULL OR delivered_sequence > 0),
    ADD CONSTRAINT chat_ai_v2_trigger_lane CHECK (trigger_v2 IS NULL OR (lane = 'v2' AND trigger_message_id IS NULL AND scope = 'group')),
    ADD CONSTRAINT chat_ai_v2_result_lane CHECK (v2_result IS NULL OR (lane = 'v2' AND result_digest IS NOT NULL));

-- One message, one answer, as on the legacy lane.
CREATE UNIQUE INDEX chat_ai_one_answer_per_v2_trigger ON chat_ai_invocations(context_id, trigger_v2)
    WHERE trigger_v2 IS NOT NULL AND status IN ('queued','running','succeeded');
