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

/// The application reducer must separately authorize operations on prior objects.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum Operation {
    Text { body: String },
    Reaction { message_id: String, emoji: String },
    Delete { message_id: String },
    Vote { poll_id: String, option_id: String },
}

impl Operation {
    pub(crate) fn validate(&self) -> Result<()> {
        let valid = match self {
            Self::Text { body } => !body.trim().is_empty() && body.len() <= 16 * 1024,
            Self::Reaction { message_id, emoji } => {
                valid_id(message_id) && !emoji.trim().is_empty() && emoji.len() <= 64
            }
            Self::Delete { message_id } => valid_id(message_id),
            Self::Vote { poll_id, option_id } => valid_id(poll_id) && valid_id(option_id),
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
