-- Who made a notification, and the comment it came from when it came from one,
-- so the list can say «Lan nhắc bạn trong một bình luận» and quote it (QA
-- UI-147). Rows written before this migration keep both NULL and read as
-- «Bạn được nhắc…»: their source is not guessed after the fact.
ALTER TABLE community_notifications
 ADD COLUMN actor_id uuid REFERENCES people(id) ON DELETE SET NULL,
 ADD COLUMN comment_id uuid REFERENCES post_comments(id) ON DELETE CASCADE;
