use base64::{engine::general_purpose::STANDARD, Engine};
use ed25519_dalek::{Signature, Signer, SigningKey, VerifyingKey};
use serde::{Deserialize, Serialize};

use crate::{Error, Result};

pub const PROTOCOL: &str = "rudi-chat-v2-mls";
pub const MAX_CIPHERTEXT: usize = 256 * 1024;

/// Opaque transport format shared with services/core/internal/chatv2/types.go.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Envelope {
    pub conversation_id: String,
    pub device_id: String,
    pub logical_send_id: String,
    pub protocol: String,
    pub epoch: i64,
    #[serde(with = "base64_bytes")]
    pub ciphertext: Vec<u8>,
    #[serde(with = "base64_bytes")]
    pub signature: Vec<u8>,
}

impl Envelope {
    pub(crate) fn unsigned(context: &str, device: &str, logical: &str, epoch: i64) -> Result<Self> {
        if !valid_id(context) || !valid_id(device) || !valid_id(logical) || epoch < 1 {
            return Err(Error::Invalid);
        }
        Ok(Self {
            conversation_id: context.into(),
            device_id: device.into(),
            logical_send_id: logical.into(),
            protocol: PROTOCOL.into(),
            epoch,
            ciphertext: Vec::new(),
            signature: Vec::new(),
        })
    }

    fn metadata(&self, prefix: &[u8]) -> Result<Vec<u8>> {
        if !valid_id(&self.conversation_id)
            || !valid_id(&self.device_id)
            || !valid_id(&self.logical_send_id)
            || self.protocol != PROTOCOL
            || self.epoch < 1
        {
            return Err(Error::Invalid);
        }
        let mut bytes = prefix.to_vec();
        for value in [
            &self.conversation_id,
            &self.device_id,
            &self.logical_send_id,
            &self.protocol,
        ] {
            bytes.extend_from_slice(&(value.len() as u32).to_be_bytes());
            bytes.extend_from_slice(value.as_bytes());
        }
        bytes.extend_from_slice(&self.epoch.to_be_bytes());
        Ok(bytes)
    }

    /// Byte-for-byte Go SigningBytes compatibility; JSON is never signed.
    pub fn signing_bytes(&self) -> Result<Vec<u8>> {
        if self.ciphertext.is_empty() || self.ciphertext.len() > MAX_CIPHERTEXT {
            return Err(Error::Invalid);
        }
        let mut bytes = self.metadata(b"RUDI-CHAT-ENVELOPE\0v2\0")?;
        bytes.extend_from_slice(&(self.ciphertext.len() as u32).to_be_bytes());
        bytes.extend_from_slice(&self.ciphertext);
        Ok(bytes)
    }

    pub(crate) fn aad(&self) -> Result<Vec<u8>> {
        self.metadata(b"RUDI-CHAT-MLS-AAD\0v1\0")
    }

    pub(crate) fn sign(&mut self, key: &SigningKey) -> Result<()> {
        self.signature = key.sign(&self.signing_bytes()?).to_bytes().to_vec();
        Ok(())
    }

    pub fn verify(&self, key: &[u8; 32]) -> Result<()> {
        let verifier = VerifyingKey::from_bytes(key).map_err(|_| Error::Authentication)?;
        let signature =
            Signature::from_slice(&self.signature).map_err(|_| Error::Authentication)?;
        verifier
            .verify_strict(&self.signing_bytes()?, &signature)
            .map_err(|_| Error::Authentication)
    }
}

/// A file sealed on the device before upload (ADR-0057 §5.2): the store holds
/// the ciphertext under `media_id`; only members of the conversation, who
/// receive this reference inside MLS, hold the key.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MediaRef {
    pub media_id: String,
    #[serde(with = "base64_bytes")]
    pub key: Vec<u8>,
    #[serde(with = "base64_bytes")]
    pub nonce: Vec<u8>,
    #[serde(with = "base64_bytes")]
    pub sha256: Vec<u8>,
    pub mime: String,
    pub size: u64,
}

impl MediaRef {
    fn validate(&self, mimes: &[&str], max_size: u64) -> bool {
        valid_id(&self.media_id)
            && self.key.len() == 32
            && self.nonce.len() == 24
            && self.sha256.len() == 32
            && mimes.contains(&self.mime.as_str())
            && self.size > 0
            && self.size <= max_size
    }
}

const IMAGE_MIMES: &[&str] = &["image/jpeg", "image/png", "image/webp"];
const VOICE_MIMES: &[&str] = &["audio/aac", "audio/mp4", "audio/ogg"];
/// The largest media a message may reference: 25 MiB of ciphertext.
pub const MAX_MEDIA: u64 = 25 * 1024 * 1024;

/// The application reducer must separately authorize operations on prior objects
/// (only the author edits or deletes; a reply's target must exist).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum Operation {
    Text {
        body: String,
    },
    Reaction {
        message_id: String,
        emoji: String,
    },
    Delete {
        message_id: String,
    },
    Vote {
        poll_id: String,
        option_id: String,
    },
    Reply {
        reply_to: String,
        body: String,
    },
    Edit {
        message_id: String,
        body: String,
    },
    Image {
        media: Box<MediaRef>,
        caption: Option<String>,
        width: u32,
        height: u32,
    },
    Sticker {
        pack_id: String,
        sticker_id: String,
    },
    Voice {
        media: Box<MediaRef>,
        duration_ms: u32,
    },
    /// Rủ Đi AI's answer to an `@Rủ Đi` message, sealed by the device of the
    /// person who asked (ADR-0057 §6): `card` is the server's result byte for
    /// byte, so every member can check it against the digest the server keeps
    /// for `invocation_id`. A card that does not match is drawn as the
    /// sender's own words, never as the assistant's.
    AiCard {
        invocation_id: String,
        reply_to: String,
        card: String,
    },
}

/// The largest AI card a message carries (the server refuses to make a larger
/// one). Below the payload ceiling with room for the JSON escaping of the
/// card inside the operation.
pub const MAX_AI_CARD: usize = 12 * 1024;

fn valid_body(body: &str) -> bool {
    !body.trim().is_empty() && body.len() <= 16 * 1024
}

fn valid_slug(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 64
        && value
            .bytes()
            .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-' || c == b'_')
}

impl Operation {
    pub(crate) fn validate(&self) -> Result<()> {
        let valid = match self {
            Self::Text { body } => valid_body(body),
            Self::Reaction { message_id, emoji } => {
                valid_id(message_id) && !emoji.trim().is_empty() && emoji.len() <= 64
            }
            Self::Delete { message_id } => valid_id(message_id),
            Self::Vote { poll_id, option_id } => valid_id(poll_id) && valid_id(option_id),
            Self::Reply { reply_to, body } => valid_id(reply_to) && valid_body(body),
            Self::Edit { message_id, body } => valid_id(message_id) && valid_body(body),
            Self::Image {
                media,
                caption,
                width,
                height,
            } => {
                media.validate(IMAGE_MIMES, MAX_MEDIA)
                    && caption.as_deref().is_none_or(|c| c.len() <= 2 * 1024)
                    && (1..=16_384).contains(width)
                    && (1..=16_384).contains(height)
            }
            Self::Sticker {
                pack_id,
                sticker_id,
            } => valid_slug(pack_id) && valid_slug(sticker_id),
            Self::Voice { media, duration_ms } => {
                media.validate(VOICE_MIMES, MAX_MEDIA) && (1..=15 * 60 * 1000).contains(duration_ms)
            }
            Self::AiCard {
                invocation_id,
                reply_to,
                card,
            } => {
                valid_id(invocation_id)
                    && valid_id(reply_to)
                    && !card.is_empty()
                    && card.len() <= MAX_AI_CARD
                    && serde_json::from_str::<serde_json::Value>(card).is_ok_and(|v| v.is_object())
            }
        };
        if valid {
            Ok(())
        } else {
            Err(Error::Invalid)
        }
    }
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct Payload {
    pub version: u8,
    pub operation: Operation,
}

/// What a device signs with its transport key to enroll (ADR-0057 §1.2),
/// byte for byte services/core/internal/chatv2.EnrollmentBytes.
pub fn enrollment_bytes(actor_id: &str, device_id: &str, mls_signature_key: &[u8; 32]) -> Vec<u8> {
    let mut bytes = b"RUDI-CHAT-DEVICE\0v1\0".to_vec();
    for value in [actor_id, device_id] {
        bytes.extend_from_slice(&(value.len() as u32).to_be_bytes());
        bytes.extend_from_slice(value.as_bytes());
    }
    bytes.extend_from_slice(mls_signature_key);
    bytes
}

pub(crate) fn valid_id(value: &str) -> bool {
    value.len() == 36
        && value.bytes().enumerate().all(|(i, c)| {
            if [8, 13, 18, 23].contains(&i) {
                c == b'-'
            } else {
                c.is_ascii_digit() || (b'a'..=b'f').contains(&c)
            }
        })
        && value.bytes().any(|c| c != b'0' && c != b'-')
}

pub(crate) mod base64_bytes {
    use super::*;
    pub fn serialize<S: serde::Serializer>(
        value: &[u8],
        serializer: S,
    ) -> std::result::Result<S::Ok, S::Error> {
        serializer.serialize_str(&STANDARD.encode(value))
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(
        deserializer: D,
    ) -> std::result::Result<Vec<u8>, D::Error> {
        let encoded = String::deserialize(deserializer)?;
        if encoded.len() > MAX_CIPHERTEXT * 2 {
            return Err(serde::de::Error::custom("wire limit"));
        }
        STANDARD
            .decode(encoded)
            .map_err(|_| serde::de::Error::custom("invalid base64"))
    }
}
