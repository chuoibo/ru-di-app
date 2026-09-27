-- Reuse an unchanged ranking for the same reader instead of copying 500 IDs
-- on every refresh. ACLs and hidden feedback are still checked on every read.
ALTER TABLE community_feeds ADD COLUMN rank_key text;
CREATE UNIQUE INDEX community_feeds_same_ranking ON community_feeds(person_id,mode,rank_key) WHERE rank_key IS NOT NULL;
