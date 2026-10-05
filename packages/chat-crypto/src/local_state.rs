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
    check_members, roster, Client, Conversation, Error, IdentityCard, PendingCommit, Received,
    Result, Roster, StoredSend, MAX_CONVERSATIONS, MAX_DELIVERED, MAX_KEY_PACKAGES, MAX_OUTBOX,
    MAX_RECEIVED,
};
use std::collections::{BTreeMap, BTreeSet, VecDeque};

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
struct StoredConversation {
    roster: Roster,
    outbox: BTreeMap<String, StoredSend>,
    delivered: BTreeSet<String>,
    pending: Option<PendingCommit>,
    #[serde(default)]
    received: VecDeque<([u8; 32], Received)>,
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct LocalState {
    version: u8,
    identity: IdentityCard,
    transport_key: [u8; 32],
    entries: Vec<(Vec<u8>, Vec<u8>)>,
    conversations: BTreeMap<String, StoredConversation>,
    outstanding_key_packages: u32,
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
    /// Everything a rejected receive or join may have touched: the provider's
    /// store and this conversation's bookkeeping. Other conversations' groups
    /// do not change during the operation, so restoring the store leaves them
    /// as they are.
    pub(crate) fn capture_receive_state(&self, conversation_id: &str) -> Result<ReceiveSnapshot> {
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
        let conversation = self.conversations.get(conversation_id);
        Ok(ReceiveSnapshot {
            entries: entries
                .iter()
                .map(|(key, value)| (key.clone(), value.clone()))
                .collect(),
            conversation_id: conversation_id.into(),
            present: conversation.is_some(),
            roster: conversation.map(|c| c.roster.clone()).unwrap_or_default(),
            outstanding_key_packages: self.outstanding_key_packages,
        })
    }

    pub(crate) fn rollback_receive(&mut self, mut snapshot: ReceiveSnapshot) -> Result<()> {
        // Processing can consume receive ratchets before application authorization.
        // Rejected commits must not strand the client or consume a valid retransmit.
        let previous = self.conversations.remove(&snapshot.conversation_id);
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
        if snapshot.present {
            let mut conversation = previous.ok_or(Error::Checkpoint)?;
            conversation.group = MlsGroup::load(
                self.provider.storage(),
                &GroupId::from_slice(snapshot.conversation_id.as_bytes()),
            )
            .map_err(|_| Error::Checkpoint)?
            .ok_or(Error::Checkpoint)?;
            conversation.roster = std::mem::take(&mut snapshot.roster);
            self.conversations
                .insert(snapshot.conversation_id.clone(), conversation);
        }
        self.outstanding_key_packages = snapshot.outstanding_key_packages;
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
            version: 2,
            identity: self.identity.clone(),
            transport_key: self.transport_signer.to_bytes(),
            entries,
            conversations: self
                .conversations
                .iter()
                .map(|(id, c)| {
                    (
                        id.clone(),
                        StoredConversation {
                            roster: c.roster.clone(),
                            outbox: c.outbox.clone(),
                            delivered: c.delivered.clone(),
                            pending: c.pending.clone(),
                            received: c.received.clone(),
                        },
                    )
                })
                .collect(),
            outstanding_key_packages: self.outstanding_key_packages,
            generation: self.generation,
        };
        let plaintext = Zeroizing::new(serde_json::to_vec(&state).map_err(|_| Error::Checkpoint)?);
        if plaintext.len() > MAX_LOCAL_STATE {
            return Err(Error::Capacity);
        }
        let nonce = XChaCha20Poly1305::generate_nonce(&mut OsRng);
        let mut sealed = SealedLocalState {
            version: 2,
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
        if sealed.version != 2
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
        if state.version != 2
            || state.generation != sealed.generation
            || state.generation == 0
            || state.identity.actor_id != sealed.actor_id
            || state.identity.device_id != sealed.device_id
            || state.entries.len() > 200_000
            || state.conversations.len() > MAX_CONVERSATIONS
            || state.outstanding_key_packages > MAX_KEY_PACKAGES
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
        let mut conversations = BTreeMap::new();
        for (id, stored) in std::mem::take(&mut state.conversations) {
            if !crate::wire::valid_id(&id)
                || stored.outbox.len() > MAX_OUTBOX
                || stored.delivered.len() > MAX_DELIVERED
                || stored.received.len() > MAX_RECEIVED
            {
                return Err(Error::Checkpoint);
            }
            let group = MlsGroup::load(provider.storage(), &GroupId::from_slice(id.as_bytes()))
                .map_err(|_| Error::Checkpoint)?
                .ok_or(Error::Checkpoint)?;
            let cards: Vec<_> = stored.roster.values().cloned().collect();
            if roster(&cards).map_err(|_| Error::Checkpoint)? != stored.roster {
                return Err(Error::Checkpoint);
            }
            if group.is_active() {
                check_members(group.members(), &stored.roster).map_err(|_| Error::Checkpoint)?;
                if stored.roster.get(&state.identity.device_id) != Some(&state.identity) {
                    return Err(Error::Checkpoint);
                }
            }
            if group.pending_commit().is_some() != stored.pending.is_some() {
                return Err(Error::Checkpoint);
            }
            for (logical_id, sent) in &stored.outbox {
                if logical_id != &sent.envelope.logical_send_id
                    || sent.envelope.device_id != state.identity.device_id
                    || sent.envelope.conversation_id != id
                {
                    return Err(Error::Checkpoint);
                }
                sent.envelope
                    .verify(&state.identity.transport_signature_key)
                    .map_err(|_| Error::Checkpoint)?;
            }
            if let Some(pending) = &stored.pending {
                if !stored.outbox.contains_key(&pending.logical_send_id) {
                    return Err(Error::Checkpoint);
                }
            }
            conversations.insert(
                id,
                Conversation {
                    group,
                    roster: stored.roster,
                    outbox: stored.outbox,
                    delivered: stored.delivered,
                    pending: stored.pending,
                    received: stored.received,
                },
            );
        }
        Ok(Self {
            provider,
            signer,
            transport_signer,
            identity: state.identity.clone(),
            conversations,
            outstanding_key_packages: state.outstanding_key_packages,
            generation: state.generation,
        })
    }
}

pub(crate) struct ReceiveSnapshot {
    entries: Vec<(Vec<u8>, Vec<u8>)>,
    conversation_id: String,
    present: bool,
    roster: Roster,
    outstanding_key_packages: u32,
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
        self.conversations.clear();
        if let Ok(mut entries) = self.provider.storage().values.write() {
            for value in entries.values_mut() {
                value.zeroize();
            }
            entries.clear();
        }
    }
}
