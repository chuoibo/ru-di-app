-- aiharness version 6: the two classes of a room (decision 2026-09-28,
-- ADR-0046 §8.4). A couple's turn (a chat of two whose two people both
-- turned on «Một đôi») runs the group's path and is recorded as bot 'doi',
-- with the couple's prompt version, so its rows are told from a room of
-- friends' (a group, or a chat of two without «Một đôi», still 'nhom').
-- Still a closed list: no free text can be stored. Version 1 is left as it
-- was applied.
ALTER TABLE ai_turn_metrics
  DROP CONSTRAINT IF EXISTS ai_turn_metrics_bot_check,
  ADD CONSTRAINT ai_turn_metrics_bot_check
    CHECK (bot IN ('nep','nhom','doi'));
