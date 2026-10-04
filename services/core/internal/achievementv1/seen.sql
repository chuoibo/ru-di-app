-- QA UI-160: «MỚI MỞ» is remembered by the account, not by the phone. A new
-- phone or browser presented the oldest badge, earned days before, as just
-- opened. seen_at is when this person's own book first presented the badge;
-- NULL until then. Badges earned more than 48 hours before this migration are
-- taken as seen: the app already declined to present those on a new phone.
ALTER TABLE achievement_earned ADD COLUMN seen_at timestamptz;
UPDATE achievement_earned SET seen_at = earned_at WHERE earned_at < clock_timestamp() - interval '48 hours';
