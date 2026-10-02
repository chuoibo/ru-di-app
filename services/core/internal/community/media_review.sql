-- A video's review cut: the pieces a reading sends the model in place of the
-- video (congdong.GiayDoan seconds each, one frame a second, low resolution,
-- with sound), cut by the media runtime when it processes the upload. Empty
-- for an image, and for a video processed before the cut existed: such a
-- video is never sent, and its post goes to a human.
ALTER TABLE community_media ADD COLUMN review_keys text[] NOT NULL DEFAULT '{}';
CREATE OR REPLACE FUNCTION community_queue_media_gc() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO community_media_gc SELECT k FROM unnest(array_prepend(OLD.storage_key, OLD.review_keys)) k ON CONFLICT DO NOTHING;
 RETURN OLD;
END $$;
