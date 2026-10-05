-- ADR-0057 §5.2: media of the v2 lane, ciphertext only. The device sealed the
-- file under a key that travels inside MLS; the server keeps opaque bytes, their
-- size and digest, and which device uploaded them to which room.
CREATE TABLE chat_v2_media (
 id uuid PRIMARY KEY,
 context_id uuid NOT NULL REFERENCES chat_v2_conversations(context_id),
 uploader_device uuid NOT NULL REFERENCES chat_v2_devices(id),
 storage_key text NOT NULL UNIQUE CHECK (storage_key ~ '^[0-9a-f]{32}$'),
 size bigint NOT NULL CHECK (size BETWEEN 1 AND 26214400),
 sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX chat_v2_media_uploader_day ON chat_v2_media(uploader_device, created_at);
