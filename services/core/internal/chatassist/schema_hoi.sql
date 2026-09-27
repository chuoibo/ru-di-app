-- chatassist version 6: the group asks the Go engine in the thread with
-- `hoi` (slice 9, design 03 §4.3): the router decides what is asked. The
-- handler takes it only on the Go engine (MOBILE_AI_ENGINE_GROUP=go) and only
-- with a trigger message; the table only widens, so a replica of the previous
-- binary still inserts what it always did. No IF EXISTS, for the reason of
-- version 3: a missing constraint is a database edited by hand.
ALTER TABLE chat_ai_invocations DROP CONSTRAINT chat_ai_command_scope;
ALTER TABLE chat_ai_invocations
    ADD CONSTRAINT chat_ai_command_scope CHECK (
        (scope = 'group' AND command IN ('plan','chia_bill','hoi'))
     OR (scope = 'me'    AND command = 'hoi'));
