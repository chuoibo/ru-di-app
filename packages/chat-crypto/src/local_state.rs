//! Local process restart is separate from backup recovery. An anchor must come
//! from an independent, current device store; deriving it from the imported blob
//! defeats rollback protection. No platform implementation is provided here.

use chacha20poly1305::{
    aead::{Aead, AeadCore, KeyInit, Payload},
    XChaCha20Poly1305, XNonce,
};
use openmls::prelude::{GroupId, MlsGroup};
use openmls_basic_credential::SignatureKeyPair;
use openmls_rust_crypto::OpenMlsRustCrypto;
use openmls_traits::{types::SignatureScheme, OpenMlsProvider};
use rand_core::OsRng;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use zeroize::{Zeroize, Zeroizing};

use crate::{
    check_members, roster, Client, Error, IdentityCard, PendingCommit, Result, Roster, StoredSend,
    MAX_OUTBOX,
};
use std::collections::BTreeMap;

const MAX_LOCAL_STATE: usize = 8 * 1024 * 1024;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct LocalAnchor {
    pub actor_id: String,
    pub device_id: String,
    pub generation: u64,
    pub sealed_digest: [u8; 32],
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SealedLocalState {
    version: u8,
    actor_id: String,
    device_id: String,
    generation: u64,
    nonce: [u8; 24],
    ciphertext: Vec<u8>,
}

impl SealedLocalState {
    fn aad(&self) -> Result<Vec<u8>> {
        serde_json::to_vec(&(
            "rudi-local-mls-state",
            self.version,
            &self.actor_id,
            &self.device_id,
            self.generation,
        ))
        .map_err(|_| Error::Checkpoint)
    }

    /// Store this independently of the blob and exclude both from device backups.
    /// The platform must atomically replace blob + anchor before network I/O or ACK.
    pub fn anchor(&self) -> Result<LocalAnchor> {
        let bytes = serde_json::to_vec(self).map_err(|_| Error::Checkpoint)?;
        Ok(LocalAnchor {
            actor_id: self.actor_id.clone(),
            device_id: self.device_id.clone(),
            generation: self.generation,
            sealed_digest: Sha256::digest(bytes).into(),
        })
    }
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct LocalState {
    version: u8,
    identity: IdentityCard,
    transport_key: [u8; 32],
    group_id: Option<String>,
    entries: Vec<(Vec<u8>, Vec<u8>)>,
    roster: Roster,
    outbox: BTreeMap<String, StoredSend>,
    pending: Option<PendingCommit>,
    generation: u64,
}

impl Drop for LocalState {
    fn drop(&mut self) {
        self.transport_key.zeroize();
        for (key, value) in &mut self.entries {
            key.zeroize();
            value.zeroize();
        }
    }
}

impl Client {
    pub(crate) fn capture_receive_state(&self) -> Result<ReceiveSnapshot> {
        let entries = self
            .provider
            .storage()
            .values
            .read()
            .map_err(|_| Error::Checkpoint)?;
        let size: usize = entries
            .iter()
            .map(|(key, value)| key.len() + value.len())
            .sum();
        if size > MAX_LOCAL_STATE {
            return Err(Error::Capacity);
        }
        Ok(ReceiveSnapshot {
            entries: entries
                .iter()
                .map(|(key, value)| (key.clone(), value.clone()))
                .collect(),
            group_id: self.group.as_ref().map(|group| group.group_id().clone()),
            roster: self.roster.clone(),
        })
    }

    pub(crate) fn rollback_receive(&mut self, mut snapshot: ReceiveSnapshot) -> Result<()> {
        // Processing can consume receive ratchets before application authorization.
        // Rejected commits must not strand the client or consume a valid retransmit.
        self.group = None;
        {
            let mut storage = self
                .provider
                .storage()
                .values
                .write()
                .map_err(|_| Error::Checkpoint)?;
            for value in storage.values_mut() {
                value.zeroize();
            }
            storage.clear();
            storage.extend(snapshot.entries.drain(..));
        }
        self.group = match &snapshot.group_id {
            Some(id) => Some(
                MlsGroup::load(self.provider.storage(), id)
                    .map_err(|_| Error::Checkpoint)?
                    .ok_or(Error::Checkpoint)?,
            ),
            None => None,
        };
        self.roster = std::mem::take(&mut snapshot.roster);
        Ok(())
    }

    /// The caller supplies a device-bound wrapping key held outside this crate.
    /// This is a local restart checkpoint, never a portable backup or recovery file.
    pub fn seal_local_state(&self, wrapping_key: &[u8; 32]) -> Result<SealedLocalState> {
        let entries = self
            .provider
            .storage()
            .values
            .read()
            .map_err(|_| Error::Checkpoint)?
            .iter()
            .map(|(key, value)| (key.clone(), value.clone()))
            .collect();
        let state = LocalState {
            version: 1,
            identity: self.identity.clone(),
            transport_key: self.transport_signer.to_bytes(),
            group_id: self
                .group
                .as_ref()
                .map(|group| String::from_utf8(group.group_id().as_slice().to_vec()))
                .transpose()
                .map_err(|_| Error::Checkpoint)?,
            entries,
            roster: self.roster.clone(),
            outbox: self.outbox.clone(),
            pending: self.pending.clone(),
            generation: self.generation,
        };
        let plaintext = Zeroizing::new(serde_json::to_vec(&state).map_err(|_| Error::Checkpoint)?);
        if plaintext.len() > MAX_LOCAL_STATE {
            return Err(Error::Capacity);
        }
        let nonce = XChaCha20Poly1305::generate_nonce(&mut OsRng);
        let mut sealed = SealedLocalState {
            version: 1,
            actor_id: self.identity.actor_id.clone(),
            device_id: self.identity.device_id.clone(),
            generation: self.generation,
            nonce: nonce.into(),
            ciphertext: Vec::new(),
        };
        sealed.ciphertext = XChaCha20Poly1305::new(wrapping_key.into())
            .encrypt(
                &nonce,
                Payload {
                    msg: &plaintext,
                    aad: &sealed.aad()?,
                },
            )
            .map_err(|_| Error::Checkpoint)?;
        Ok(sealed)
    }

    /// Requires the latest anchor from this device's non-backup trusted store.
    /// Never call this for OS/cloud backup recovery; create a new device instead.
    pub fn resume_local_state(
        sealed: &SealedLocalState,
        wrapping_key: &[u8; 32],
        current_anchor: &LocalAnchor,
    ) -> Result<Self> {
        if sealed.version != 1
            || sealed.ciphertext.len() > MAX_LOCAL_STATE + 16
            || &sealed.anchor()? != current_anchor
        {
            return Err(Error::Checkpoint);
        }
        let plaintext = Zeroizing::new(
            XChaCha20Poly1305::new(wrapping_key.into())
                .decrypt(
                    XNonce::from_slice(&sealed.nonce),
                    Payload {
                        msg: &sealed.ciphertext,
                        aad: &sealed.aad()?,
                    },
                )
                .map_err(|_| Error::Checkpoint)?,
        );
        let mut state: LocalState =
            serde_json::from_slice(&plaintext).map_err(|_| Error::Checkpoint)?;
        if state.version != 1
            || state.generation != sealed.generation
            || state.generation == 0
            || state.identity.actor_id != sealed.actor_id
            || state.identity.device_id != sealed.device_id
            || state.entries.len() > 10000
            || state.outbox.len() > MAX_OUTBOX
        {
            return Err(Error::Checkpoint);
        }
        state.identity.identity().map_err(|_| Error::Checkpoint)?;
        let provider = OpenMlsRustCrypto::default();
        {
            let mut storage = provider
                .storage()
                .values
                .write()
                .map_err(|_| Error::Checkpoint)?;
            for (key, value) in state.entries.drain(..) {
                if storage.insert(key, value).is_some() {
                    return Err(Error::Checkpoint);
                }
            }
        }
        let signer = SignatureKeyPair::read(
            provider.storage(),
            &state.identity.mls_signature_key,
            SignatureScheme::ED25519,
        )
        .ok_or(Error::Checkpoint)?;
        let transport_signer = ed25519_dalek::SigningKey::from_bytes(&state.transport_key);
        if transport_signer.verifying_key().to_bytes() != state.identity.transport_signature_key
            || signer.to_public_vec() != state.identity.mls_signature_key
        {
            return Err(Error::Checkpoint);
        }
        let group = match &state.group_id {
            Some(id) if crate::wire::valid_id(id) => Some(
                MlsGroup::load(provider.storage(), &GroupId::from_slice(id.as_bytes()))
                    .map_err(|_| Error::Checkpoint)?
                    .ok_or(Error::Checkpoint)?,
            ),
            None => None,
            _ => return Err(Error::Checkpoint),
        };
        if let Some(group) = &group {
            let cards: Vec<_> = state.roster.values().cloned().collect();
            if roster(&cards).map_err(|_| Error::Checkpoint)? != state.roster {
                return Err(Error::Checkpoint);
            }
            if group.is_active() {
                check_members(group.members(), &state.roster).map_err(|_| Error::Checkpoint)?;
                if state.roster.get(&state.identity.device_id) != Some(&state.identity) {
                    return Err(Error::Checkpoint);
                }
            }
            if group.pending_commit().is_some() != state.pending.is_some() {
                return Err(Error::Checkpoint);
            }
        } else if !state.roster.is_empty() || state.pending.is_some() || !state.outbox.is_empty() {
            return Err(Error::Checkpoint);
        }
        for (logical_id, sent) in &state.outbox {
            if logical_id != &sent.envelope.logical_send_id
                || sent.envelope.device_id != state.identity.device_id
            {
                return Err(Error::Checkpoint);
            }
            sent.envelope
                .verify(&state.identity.transport_signature_key)
                .map_err(|_| Error::Checkpoint)?;
        }
        if let Some(pending) = &state.pending {
            if !state.outbox.contains_key(&pending.logical_send_id) {
                return Err(Error::Checkpoint);
            }
        }
        Ok(Self {
            provider,
            signer,
            transport_signer,
            identity: state.identity.clone(),
            group,
            roster: std::mem::take(&mut state.roster),
            outbox: std::mem::take(&mut state.outbox),
            pending: state.pending.take(),
            generation: state.generation,
        })
    }
}

pub(crate) struct ReceiveSnapshot {
    entries: Vec<(Vec<u8>, Vec<u8>)>,
    group_id: Option<GroupId>,
    roster: Roster,
}

impl Drop for ReceiveSnapshot {
    fn drop(&mut self) {
        for (key, value) in &mut self.entries {
            key.zeroize();
            value.zeroize();
        }
    }
}

impl Drop for Client {
    fn drop(&mut self) {
        // OpenMLS owns ratchet secret types; the memory provider additionally holds
        // serialized key material that otherwise would not be zeroized on drop.
        self.group.take();
        if let Ok(mut entries) = self.provider.storage().values.write() {
            for value in entries.values_mut() {
                value.zeroize();
            }
            entries.clear();
        }
    }
}
