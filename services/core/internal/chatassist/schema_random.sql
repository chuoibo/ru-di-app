-- chatassist version 8: an explicit source search for random game choices.
-- The same invocation queue, authority, digest and delivery apply. Personal
-- Nếp invocations still accept only hoi. Applied SQL stays unchanged.
SET LOCAL lock_timeout = '5s';
ALTER TABLE chat_ai_invocations DROP CONSTRAINT chat_ai_command_scope;
ALTER TABLE chat_ai_invocations
    ADD CONSTRAINT chat_ai_command_scope CHECK (
        (scope = 'group' AND command IN ('plan','chia_bill','hoi','random'))
     OR (scope = 'me'    AND command = 'hoi'));
