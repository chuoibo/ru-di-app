//! Media is sealed on the device before upload (ADR-0057 §5.2): a fresh key and
//! nonce per file, XChaCha20-Poly1305 with the media id as associated data, so
//! a ciphertext cannot be swapped under another message's reference. The
//! caller strips metadata (EXIF) before sealing; this crate never sees a path.

use chacha20poly1305::{
    aead::{Aead, AeadCore, KeyInit, Payload},
    XChaCha20Poly1305, XNonce,
};
use rand_core::OsRng;
use sha2::{Digest, Sha256};
use zeroize::Zeroizing;

use crate::{wire, Error, MediaRef, Result};

fn aad(media_id: &str, mime: &str) -> Vec<u8> {
    let mut bytes = b"RUDI-CHAT-MEDIA\0v1\0".to_vec();
    for value in [media_id, mime] {
        bytes.extend_from_slice(&(value.len() as u32).to_be_bytes());
        bytes.extend_from_slice(value.as_bytes());
    }
    bytes
}

/// Seals `plaintext` for upload under `media_id`; returns the ciphertext to
/// upload and the reference to send inside MLS.
pub fn seal_media(media_id: &str, mime: &str, plaintext: &[u8]) -> Result<(Vec<u8>, MediaRef)> {
    if !wire::valid_id(media_id) || plaintext.is_empty() {
        return Err(Error::Invalid);
    }
    let key = Zeroizing::new(XChaCha20Poly1305::generate_key(&mut OsRng));
    let nonce = XChaCha20Poly1305::generate_nonce(&mut OsRng);
    let ciphertext = XChaCha20Poly1305::new(&key)
        .encrypt(
            &nonce,
            Payload {
                msg: plaintext,
                aad: &aad(media_id, mime),
            },
        )
        .map_err(|_| Error::Invalid)?;
    if ciphertext.len() as u64 > wire::MAX_MEDIA {
        return Err(Error::Capacity);
    }
    let reference = MediaRef {
        media_id: media_id.into(),
        key: key.to_vec(),
        nonce: nonce.to_vec(),
        sha256: Sha256::digest(&ciphertext).to_vec(),
        mime: mime.into(),
        size: ciphertext.len() as u64,
    };
    Ok((ciphertext, reference))
}

/// Opens a downloaded ciphertext against the reference a member sent. The
/// digest and size are checked before decryption is attempted.
pub fn open_media(reference: &MediaRef, ciphertext: &[u8]) -> Result<Zeroizing<Vec<u8>>> {
    if ciphertext.len() as u64 != reference.size
        || Sha256::digest(ciphertext).as_slice() != reference.sha256.as_slice()
        || reference.key.len() != 32
        || reference.nonce.len() != 24
    {
        return Err(Error::Authentication);
    }
    let plaintext = XChaCha20Poly1305::new_from_slice(&reference.key)
        .map_err(|_| Error::Authentication)?
        .decrypt(
            XNonce::from_slice(&reference.nonce),
            Payload {
                msg: ciphertext,
                aad: &aad(&reference.media_id, &reference.mime),
            },
        )
        .map_err(|_| Error::Authentication)?;
    Ok(Zeroizing::new(plaintext))
}
