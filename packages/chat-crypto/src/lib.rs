//! Native MLS adapter for chat v2 (ADR-0031, ADR-0057). See README.md for the release gates.
#![forbid(unsafe_code)]

#[cfg(test)]
mod adversarial;
mod local_state;
mod media;
mod wire;

pub use local_state::{LocalAnchor, SealedLocalState};
pub use media::{open_media, seal_media};

/// The digest the server keeps for an AI card it made (ADR-0057 §6), so a
/// member can check a received `ai_card` against it: SHA-256 of the card's
/// exact bytes.
pub fn ai_card_digest(card: &str) -> [u8; 32] {
    Sha256::digest(card.as_bytes()).into()
}
pub use wire::{
    enrollment_bytes, Envelope, MediaRef, Operation, MAX_AI_CARD, MAX_CIPHERTEXT, MAX_MEDIA,
    PROTOCOL,
};

use std::collections::{BTreeMap, BTreeSet, VecDeque};

use ed25519_dalek::SigningKey;
use openmls::prelude::*;
use openmls_basic_credential::SignatureKeyPair;
use openmls_rust_crypto::OpenMlsRustCrypto;
use openmls_traits::OpenMlsProvider;
use rand_core::OsRng;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use tls_codec::{Deserialize as TlsDeserialize, Serialize as TlsSerialize};
use zeroize::Zeroizing;

const SUITE: Ciphersuite = Ciphersuite::MLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519;
const MAX_LEAVES: usize = 500;
const MAX_OUTBOX: usize = 128;
/// Delivered logical IDs remembered per conversation, oldest dropped first.
const MAX_DELIVERED: usize = 4096;
/// Conversations one device takes part in.
const MAX_CONVERSATIONS: usize = 1024;
/// Key packages outstanding at once (ADR-0057 §2: ten plus refills).
const MAX_KEY_PACKAGES: u32 = 64;
/// Largest decrypted application payload.
const MAX_PAYLOAD: usize = 20 * 1024;
/// Processed envelopes remembered until the app says it stored their results.
const MAX_RECEIVED: usize = 128;

#[derive(Debug, thiserror::Error, PartialEq, Eq)]
pub enum Error {
    #[error("invalid chat input")]
    Invalid,
    #[error("message authentication failed")]
    Authentication,
    #[error("verified roster does not match MLS state")]
    Roster,
    #[error("group is unavailable or has a pending commit")]
    State,
    #[error("logical send id conflicts with an earlier operation")]
    Conflict,
    #[error("experimental bounded storage is full")]
    Capacity,
    #[error("MLS operation rejected")]
    Mls,
    #[error("local checkpoint is unavailable, corrupt, or stale")]
    Checkpoint,
}

pub type Result<T> = std::result::Result<T, Error>;

fn mls<E>(_: E) -> Error {
    Error::Mls
}

/// Public keys must be verified through a trusted enrollment channel before use.
/// MLS BasicCredential alone does not authenticate an account or device.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct IdentityCard {
    pub actor_id: String,
    pub device_id: String,
    pub mls_signature_key: [u8; 32],
    pub transport_signature_key: [u8; 32],
}

impl IdentityCard {
    fn identity(&self) -> Result<Vec<u8>> {
        if !wire::valid_id(&self.actor_id) || !wire::valid_id(&self.device_id) {
            return Err(Error::Invalid);
        }
        let mut identity = b"RUDI-MLS-IDENTITY\0v1\0".to_vec();
        identity.extend_from_slice(self.actor_id.as_bytes());
        identity.extend_from_slice(self.device_id.as_bytes());
        Ok(identity)
    }

    fn matches(&self, credential: &Credential, signature_key: &[u8]) -> bool {
        credential.credential_type() == CredentialType::Basic
            && self
                .identity()
                .is_ok_and(|identity| identity == credential.serialized_content())
            && self.mls_signature_key.as_slice() == signature_key
    }
}

type Roster = BTreeMap<String, IdentityCard>;

fn roster(cards: &[IdentityCard]) -> Result<Roster> {
    if cards.is_empty() || cards.len() > MAX_LEAVES {
        return Err(Error::Roster);
    }
    let mut result = BTreeMap::new();
    let mut accounts: BTreeMap<&str, usize> = BTreeMap::new();
    let mut keys = BTreeSet::new();
    for card in cards {
        card.identity()?;
        // No device may borrow another leaf's signing identity.
        if !keys.insert(card.mls_signature_key)
            || !keys.insert(card.transport_signature_key)
            || result
                .insert(card.device_id.clone(), card.clone())
                .is_some()
        {
            return Err(Error::Roster);
        }
        let count = accounts.entry(&card.actor_id).or_default();
        *count += 1;
        if *count > 5 || accounts.len() > 100 {
            return Err(Error::Roster);
        }
    }
    Ok(result)
}

fn check_members(members: impl Iterator<Item = Member>, expected: &Roster) -> Result<()> {
    let mut found = BTreeSet::new();
    for member in members {
        let card = expected
            .values()
            .find(|c| c.matches(&member.credential, &member.signature_key))
            .ok_or(Error::Roster)?;
        if !found.insert(card.device_id.clone()) {
            return Err(Error::Roster);
        }
    }
    if found.len() != expected.len() {
        return Err(Error::Roster);
    }
    Ok(())
}

fn config() -> MlsGroupCreateConfig {
    MlsGroupCreateConfig::builder()
        .ciphersuite(SUITE)
        .padding_size(128)
        .max_past_epochs(0)
        .sender_ratchet_configuration(SenderRatchetConfiguration::new(32, 2000))
        .wire_format_policy(PURE_CIPHERTEXT_WIRE_FORMAT_POLICY)
        .use_ratchet_tree_extension(true)
        .build()
}

#[derive(Clone, Serialize, Deserialize)]
struct StoredSend {
    digest: [u8; 32],
    envelope: Envelope,
}

#[derive(Clone, Serialize, Deserialize)]
struct PendingCommit {
    logical_send_id: String,
    next_roster: Roster,
    welcome: Option<Vec<u8>>,
}

#[derive(Clone, Debug)]
pub struct CommitBundle {
    pub envelope: Envelope,
    pub welcome: Option<Vec<u8>>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case", deny_unknown_fields)]
pub enum Received {
    Application {
        actor_id: String,
        device_id: String,
        logical_send_id: String,
        operation: Operation,
    },
    Commit {
        epoch: i64,
    },
    Removed,
}

/// One conversation's MLS group and the device's own bookkeeping for it.
pub(crate) struct Conversation {
    group: MlsGroup,
    roster: Roster,
    outbox: BTreeMap<String, StoredSend>,
    /// Logical IDs whose send the delivery service accepted and whose
    /// ciphertext was dropped from the outbox: a repeat is refused, never
    /// re-encrypted under a new ratchet step.
    delivered: BTreeSet<String>,
    pending: Option<PendingCommit>,
    /// Envelopes processed since the app last said it stored what they
    /// produced (`settle_received`), with that result. The same bytes again --
    /// the app died between this answer and its own write -- answer the same
    /// instead of failing on a ratchet step that is gone, which would wedge
    /// the room for good.
    received: VecDeque<([u8; 32], Received)>,
}

/// One device identity in many conversations (ADR-0057 §1.1). Mutable access
/// serializes ratchet changes. No private key, plaintext export, server
/// client, or logging API is exposed.
pub struct Client {
    provider: OpenMlsRustCrypto,
    signer: SignatureKeyPair,
    transport_signer: SigningKey,
    identity: IdentityCard,
    conversations: BTreeMap<String, Conversation>,
    /// Key packages handed out and not yet consumed by a Welcome.
    outstanding_key_packages: u32,
    generation: u64,
}

impl Client {
    pub fn new(actor_id: &str, device_id: &str) -> Result<Self> {
        if !wire::valid_id(actor_id) || !wire::valid_id(device_id) {
            return Err(Error::Invalid);
        }
        let provider = OpenMlsRustCrypto::default();
        let signer = SignatureKeyPair::new(SUITE.signature_algorithm()).map_err(mls)?;
        signer.store(provider.storage()).map_err(mls)?;
        let transport_signer = SigningKey::generate(&mut OsRng);
        let identity = IdentityCard {
            actor_id: actor_id.into(),
            device_id: device_id.into(),
            mls_signature_key: signer.to_public_vec().try_into().map_err(|_| Error::Mls)?,
            transport_signature_key: transport_signer.verifying_key().to_bytes(),
        };
        Ok(Self {
            provider,
            signer,
            transport_signer,
            identity,
            conversations: BTreeMap::new(),
            outstanding_key_packages: 0,
            generation: 1,
        })
    }

    /// Backup recovery creates a fresh device. Old group state is never imported.
    pub fn recover_as_new_device(previous: &IdentityCard, new_device_id: &str) -> Result<Self> {
        if previous.device_id == new_device_id {
            return Err(Error::Invalid);
        }
        Self::new(&previous.actor_id, new_device_id)
    }

    pub fn identity(&self) -> IdentityCard {
        self.identity.clone()
    }

    pub fn generation(&self) -> u64 {
        self.generation
    }

    /// The enrollment proof: this device's transport key signing
    /// `enrollment_bytes` for its own identity card.
    pub fn enrollment_proof(&self) -> [u8; 64] {
        use ed25519_dalek::Signer;
        self.transport_signer
            .sign(&wire::enrollment_bytes(
                &self.identity.actor_id,
                &self.identity.device_id,
                &self.identity.mls_signature_key,
            ))
            .to_bytes()
    }

    /// The conversations this device is an active member of.
    pub fn conversations(&self) -> Vec<String> {
        self.conversations
            .iter()
            .filter(|(_, c)| c.group.is_active())
            .map(|(id, _)| id.clone())
            .collect()
    }

    pub fn epoch(&self, conversation_id: &str) -> Result<i64> {
        go_epoch(self.active(conversation_id)?)
    }

    pub fn roster(&self, conversation_id: &str) -> Vec<IdentityCard> {
        self.conversations
            .get(conversation_id)
            .map(|c| c.roster.values().cloned().collect())
            .unwrap_or_default()
    }

    fn mutate(&mut self) -> Result<()> {
        self.generation = self.generation.checked_add(1).ok_or(Error::Capacity)?;
        Ok(())
    }

    fn active(&self, conversation_id: &str) -> Result<&MlsGroup> {
        self.conversations
            .get(conversation_id)
            .map(|c| &c.group)
            .filter(|group| group.is_active())
            .ok_or(Error::State)
    }

    fn conversation_ref(&self, conversation_id: &str) -> Result<&Conversation> {
        self.conversations
            .get(conversation_id)
            .filter(|c| c.group.is_active())
            .ok_or(Error::State)
    }

    fn conversation(&mut self, conversation_id: &str) -> Result<&mut Conversation> {
        self.conversations
            .get_mut(conversation_id)
            .filter(|c| c.group.is_active())
            .ok_or(Error::State)
    }

    fn credential(&self) -> Result<CredentialWithKey> {
        Ok(CredentialWithKey {
            credential: BasicCredential::new(self.identity.identity()?).into(),
            signature_key: self.signer.to_public_vec().into(),
        })
    }

    /// A one-time key package for the enrollment service (ADR-0057 §2). Its
    /// private half stays in this client until a Welcome consumes it; at most
    /// MAX_KEY_PACKAGES are outstanding at once.
    pub fn key_package(&mut self) -> Result<Vec<u8>> {
        if self.outstanding_key_packages >= MAX_KEY_PACKAGES {
            return Err(Error::Capacity);
        }
        self.mutate()?;
        let encoded = KeyPackage::builder()
            .build(SUITE, &self.provider, &self.signer, self.credential()?)
            .map_err(mls)?
            .key_package()
            .tls_serialize_detached()
            .map_err(mls)?;
        self.outstanding_key_packages += 1;
        Ok(encoded)
    }

    pub fn create_group(&mut self, conversation_id: &str) -> Result<()> {
        if self.conversations.contains_key(conversation_id) || !wire::valid_id(conversation_id) {
            return Err(Error::State);
        }
        if self.conversations.len() >= MAX_CONVERSATIONS {
            return Err(Error::Capacity);
        }
        self.mutate()?;
        let group = MlsGroup::new_with_group_id(
            &self.provider,
            &self.signer,
            &config(),
            GroupId::from_slice(conversation_id.as_bytes()),
            self.credential()?,
        )
        .map_err(mls)?;
        self.conversations.insert(
            conversation_id.into(),
            Conversation {
                group,
                roster: roster(&[self.identity()])?,
                outbox: BTreeMap::new(),
                delivered: BTreeSet::new(),
                received: VecDeque::new(),
                pending: None,
            },
        );
        Ok(())
    }

    pub fn join_group(
        &mut self,
        conversation_id: &str,
        welcome: &[u8],
        verified_roster: &[IdentityCard],
    ) -> Result<()> {
        if self
            .conversations
            .get(conversation_id)
            .is_some_and(|c| c.group.is_active())
            || welcome.len() > MAX_CIPHERTEXT
        {
            return Err(Error::State);
        }
        if self.conversations.len() >= MAX_CONVERSATIONS {
            return Err(Error::Capacity);
        }
        let snapshot = self.capture_receive_state(conversation_id)?;
        let result = self.join_group_inner(conversation_id, welcome, verified_roster);
        if result.is_err() {
            self.rollback_receive(snapshot)?;
        }
        result
    }

    fn join_group_inner(
        &mut self,
        conversation_id: &str,
        welcome: &[u8],
        verified_roster: &[IdentityCard],
    ) -> Result<()> {
        if !wire::valid_id(conversation_id) || welcome.len() > MAX_CIPHERTEXT {
            return Err(Error::State);
        }
        let expected = roster(verified_roster)?;
        if expected.get(&self.identity.device_id) != Some(&self.identity) {
            return Err(Error::Roster);
        }
        let message = MlsMessageIn::tls_deserialize_exact(welcome).map_err(mls)?;
        let MlsMessageBodyIn::Welcome(welcome) = message.extract() else {
            return Err(Error::Invalid);
        };
        self.mutate()?;
        // A rejoin after removal: the inactive group of the same id is erased
        // from storage BEFORE the new one is written under the same GroupId
        // (security review 05/10: deleting it afterwards erased the new group).
        // A failure from here on is rolled back to the snapshot, old group included.
        if let Some(old) = self.conversations.get_mut(conversation_id) {
            old.group.delete(self.provider.storage()).map_err(mls)?;
        }
        let staged =
            StagedWelcome::new_from_welcome(&self.provider, config().join_config(), welcome, None)
                .map_err(mls)?;
        if staged.group_context().group_id().as_slice() != conversation_id.as_bytes()
            || staged.group_context().ciphersuite() != SUITE
        {
            return Err(Error::Authentication);
        }
        check_members(staged.members(), &expected)?;
        let group = staged.into_group(&self.provider).map_err(mls)?;
        self.conversations.insert(
            conversation_id.into(),
            Conversation {
                group,
                roster: expected,
                outbox: BTreeMap::new(),
                delivered: BTreeSet::new(),
                received: VecDeque::new(),
                pending: None,
            },
        );
        self.outstanding_key_packages = self.outstanding_key_packages.saturating_sub(1);
        Ok(())
    }

    /// Drops a conversation this device was removed from, or left: its group
    /// state and outbox are erased from memory and from the next checkpoint.
    pub fn forget(&mut self, conversation_id: &str) -> Result<()> {
        let provider = &self.provider;
        let conversation = self
            .conversations
            .get_mut(conversation_id)
            .ok_or(Error::State)?;
        conversation.group.delete(provider.storage()).map_err(mls)?;
        self.conversations.remove(conversation_id);
        self.mutate()
    }

    fn unsigned(&self, conversation_id: &str, logical: &str) -> Result<Envelope> {
        Envelope::unsigned(
            conversation_id,
            &self.identity.device_id,
            logical,
            self.epoch(conversation_id)?,
        )
    }

    fn previous(
        &self,
        conversation_id: &str,
        logical: &str,
        digest: &[u8; 32],
    ) -> Result<Option<Envelope>> {
        let conversation = self
            .conversations
            .get(conversation_id)
            .ok_or(Error::State)?;
        if let Some(stored) = conversation.outbox.get(logical) {
            if stored.digest != *digest {
                return Err(Error::Conflict);
            }
            return Ok(Some(stored.envelope.clone()));
        }
        if conversation.delivered.contains(logical) {
            return Err(Error::Conflict);
        }
        if conversation.outbox.len() >= MAX_OUTBOX {
            return Err(Error::Capacity);
        }
        Ok(None)
    }

    fn finish(
        &mut self,
        mut envelope: Envelope,
        message: MlsMessageOut,
        digest: [u8; 32],
    ) -> Result<Envelope> {
        envelope.ciphertext = message.to_bytes().map_err(mls)?;
        envelope.sign(&self.transport_signer)?;
        self.conversation(&envelope.conversation_id.clone())?
            .outbox
            .insert(
                envelope.logical_send_id.clone(),
                StoredSend {
                    digest,
                    envelope: envelope.clone(),
                },
            );
        Ok(envelope)
    }

    /// A repeated logical ID returns identical ciphertext; it never advances a ratchet twice.
    pub fn encrypt(
        &mut self,
        conversation_id: &str,
        logical_send_id: &str,
        operation: Operation,
    ) -> Result<Envelope> {
        operation.validate()?;
        let payload = Zeroizing::new(
            serde_json::to_vec(&wire::Payload {
                version: 1,
                operation,
            })
            .map_err(|_| Error::Invalid)?,
        );
        if payload.len() > MAX_PAYLOAD {
            return Err(Error::Invalid);
        }
        let digest = Sha256::digest(payload.as_slice()).into();
        // An inactive or removed device cannot use the retained outbox to send again.
        self.active(conversation_id)?;
        if let Some(previous) = self.previous(conversation_id, logical_send_id, &digest)? {
            return Ok(previous);
        }
        if self.conversation(conversation_id)?.pending.is_some() {
            return Err(Error::State);
        }
        let envelope = self.unsigned(conversation_id, logical_send_id)?;
        self.mutate()?;
        let provider = &self.provider;
        let signer = &self.signer;
        let group = &mut self
            .conversations
            .get_mut(conversation_id)
            .ok_or(Error::State)?
            .group;
        group.set_aad(envelope.aad()?);
        let message = group
            .create_message(provider, signer, &payload)
            .map_err(mls)?;
        self.finish(envelope, message, digest)
    }

    /// The delivery service durably accepted this application send: its
    /// ciphertext leaves the outbox (ADR-0057 §5.3), and the logical ID is
    /// remembered so a late retry is refused instead of re-encrypted.
    pub fn acknowledge_sent(&mut self, accepted: &Envelope) -> Result<()> {
        let conversation = self
            .conversations
            .get_mut(&accepted.conversation_id)
            .ok_or(Error::State)?;
        let stored = conversation
            .outbox
            .get(&accepted.logical_send_id)
            .ok_or(Error::State)?;
        if &stored.envelope != accepted
            || conversation
                .pending
                .as_ref()
                .is_some_and(|p| p.logical_send_id == accepted.logical_send_id)
        {
            return Err(Error::Conflict);
        }
        conversation.outbox.remove(&accepted.logical_send_id);
        if conversation.delivered.len() >= MAX_DELIVERED {
            let oldest = conversation.delivered.iter().next().cloned();
            if let Some(oldest) = oldest {
                conversation.delivered.remove(&oldest);
            }
        }
        conversation
            .delivered
            .insert(accepted.logical_send_id.clone());
        self.mutate()
    }

    /// The delivery service refused this send because the epoch moved
    /// (409 stale_epoch): its ciphertext leaves the outbox so the same logical
    /// ID can be encrypted again under the current epoch. Only a send of a
    /// past epoch can be abandoned; the service never accepted it.
    pub fn abandon_send(&mut self, refused: &Envelope) -> Result<()> {
        let epoch = self.epoch(&refused.conversation_id)?;
        let conversation = self.conversation(&refused.conversation_id)?;
        let stored = conversation
            .outbox
            .get(&refused.logical_send_id)
            .ok_or(Error::State)?;
        if &stored.envelope != refused
            || refused.epoch >= epoch
            || conversation
                .pending
                .as_ref()
                .is_some_and(|p| p.logical_send_id == refused.logical_send_id)
        {
            return Err(Error::Conflict);
        }
        conversation.outbox.remove(&refused.logical_send_id);
        self.mutate()
    }

    pub fn stage_add(
        &mut self,
        conversation_id: &str,
        logical_send_id: &str,
        members: &[(IdentityCard, Vec<u8>)],
    ) -> Result<CommitBundle> {
        let current = self.conversation_ref(conversation_id)?;
        if members.is_empty() || current.pending.is_some() {
            return Err(Error::State);
        }
        if members.len() > MAX_LEAVES.saturating_sub(current.roster.len()) {
            return Err(Error::Roster);
        }
        let mut next = current.roster.clone();
        let mut packages = Vec::new();
        for (card, encoded) in members {
            if encoded.len() > MAX_CIPHERTEXT
                || next.insert(card.device_id.clone(), card.clone()).is_some()
            {
                return Err(Error::Roster);
            }
            let package = KeyPackageIn::tls_deserialize_exact(encoded)
                .map_err(mls)?
                .validate(self.provider.crypto(), ProtocolVersion::Mls10)
                .map_err(mls)?;
            if package.ciphersuite() != SUITE
                || !card.matches(
                    package.leaf_node().credential(),
                    package.leaf_node().signature_key().as_slice(),
                )
            {
                return Err(Error::Authentication);
            }
            packages.push(package);
        }
        let cards: Vec<_> = next.values().cloned().collect();
        roster(&cards)?;
        let digest = commit_digest("add", &next)?;
        if self
            .previous(conversation_id, logical_send_id, &digest)?
            .is_some()
        {
            return Err(Error::Conflict);
        }
        let envelope = self.unsigned(conversation_id, logical_send_id)?;
        self.mutate()?;
        let provider = &self.provider;
        let signer = &self.signer;
        let group = &mut self
            .conversations
            .get_mut(conversation_id)
            .ok_or(Error::State)?
            .group;
        group.set_aad(envelope.aad()?);
        let (message, welcome, _) = group
            .add_members(provider, signer, &packages)
            .map_err(mls)?;
        let welcome = welcome.to_bytes().map_err(mls)?;
        self.finish_commit(envelope, message, digest, next, Some(welcome))
    }

    pub fn stage_remove(
        &mut self,
        conversation_id: &str,
        logical_send_id: &str,
        device_id: &str,
    ) -> Result<CommitBundle> {
        let current = self.conversation_ref(conversation_id)?;
        if current.pending.is_some() || device_id == self.identity.device_id {
            return Err(Error::State);
        }
        let removed = current.roster.get(device_id).ok_or(Error::Roster)?;
        let index = current
            .group
            .members()
            .find(|member| removed.matches(&member.credential, &member.signature_key))
            .ok_or(Error::Roster)?
            .index;
        let mut next = current.roster.clone();
        next.remove(device_id);
        let digest = commit_digest("remove", &next)?;
        if self
            .previous(conversation_id, logical_send_id, &digest)?
            .is_some()
        {
            return Err(Error::Conflict);
        }
        let envelope = self.unsigned(conversation_id, logical_send_id)?;
        self.mutate()?;
        let provider = &self.provider;
        let signer = &self.signer;
        let group = &mut self
            .conversations
            .get_mut(conversation_id)
            .ok_or(Error::State)?
            .group;
        group.set_aad(envelope.aad()?);
        let (message, _, _) = group
            .remove_members(provider, signer, &[index])
            .map_err(mls)?;
        self.finish_commit(envelope, message, digest, next, None)
    }

    pub fn stage_rekey(
        &mut self,
        conversation_id: &str,
        logical_send_id: &str,
    ) -> Result<CommitBundle> {
        let current = self.conversation_ref(conversation_id)?;
        if current.pending.is_some() {
            return Err(Error::State);
        }
        let next = current.roster.clone();
        let digest = commit_digest("rekey", &next)?;
        if self
            .previous(conversation_id, logical_send_id, &digest)?
            .is_some()
        {
            return Err(Error::Conflict);
        }
        let envelope = self.unsigned(conversation_id, logical_send_id)?;
        self.mutate()?;
        let provider = &self.provider;
        let signer = &self.signer;
        let group = &mut self
            .conversations
            .get_mut(conversation_id)
            .ok_or(Error::State)?
            .group;
        group.set_aad(envelope.aad()?);
        let message = group
            .self_update(provider, signer, LeafNodeParameters::default())
            .map_err(mls)?
            .into_commit();
        self.finish_commit(envelope, message, digest, next, None)
    }

    fn finish_commit(
        &mut self,
        envelope: Envelope,
        message: MlsMessageOut,
        digest: [u8; 32],
        next_roster: Roster,
        welcome: Option<Vec<u8>>,
    ) -> Result<CommitBundle> {
        let envelope = self.finish(envelope, message, digest)?;
        self.conversation(&envelope.conversation_id.clone())?
            .pending = Some(PendingCommit {
            logical_send_id: envelope.logical_send_id.clone(),
            next_roster,
            welcome: welcome.clone(),
        });
        Ok(CommitBundle { envelope, welcome })
    }

    /// Retry the exact pending commit after a timeout or local process restart.
    pub fn pending_commit(&self, conversation_id: &str) -> Result<CommitBundle> {
        let conversation = self
            .conversations
            .get(conversation_id)
            .ok_or(Error::State)?;
        let pending = conversation.pending.as_ref().ok_or(Error::State)?;
        let envelope = conversation
            .outbox
            .get(&pending.logical_send_id)
            .ok_or(Error::State)?
            .envelope
            .clone();
        Ok(CommitBundle {
            envelope,
            welcome: pending.welcome.clone(),
        })
    }

    /// The delivery service rejected the pending commit because another one
    /// won the epoch (409 epoch_moved, ADR-0057 §3.2): it is discarded so the
    /// winner can be received, and the caller stages its change again.
    pub fn abandon_commit(&mut self, conversation_id: &str) -> Result<()> {
        let provider = &self.provider;
        let conversation = self
            .conversations
            .get_mut(conversation_id)
            .filter(|c| c.group.is_active())
            .ok_or(Error::State)?;
        if conversation.pending.is_none() {
            return Err(Error::State);
        }
        // Storage first: if clearing fails, the bookkeeping still matches the
        // group and the checkpoint stays openable (security review 05/10).
        conversation
            .group
            .clear_pending_commit(provider.storage())
            .map_err(mls)?;
        if let Some(pending) = conversation.pending.take() {
            conversation.outbox.remove(&pending.logical_send_id);
        }
        self.mutate()
    }

    /// Call only after durable delivery-service acceptance of this exact envelope.
    /// Production must atomically persist this state transition with the ACK cursor.
    pub fn acknowledge_commit(&mut self, accepted: &Envelope) -> Result<()> {
        let conversation = self.conversation(&accepted.conversation_id)?;
        let pending = conversation.pending.as_ref().ok_or(Error::State)?;
        let sent = conversation
            .outbox
            .get(&pending.logical_send_id)
            .ok_or(Error::State)?;
        if &sent.envelope != accepted {
            return Err(Error::Conflict);
        }
        let next = pending.next_roster.clone();
        let logical = pending.logical_send_id.clone();
        self.mutate()?;
        let provider = &self.provider;
        let conversation = self
            .conversations
            .get_mut(&accepted.conversation_id)
            .ok_or(Error::State)?;
        // Checked before anything changes, so a refusal leaves the pending
        // commit exactly as it was (security review 05/10).
        let staged = conversation.group.pending_commit().ok_or(Error::State)?;
        check_commit(&conversation.group, staged, &conversation.roster, &next)?;
        conversation
            .group
            .merge_pending_commit(provider)
            .map_err(mls)?;
        // Merged: the bookkeeping follows the group whatever comes next.
        conversation.roster = next;
        conversation.pending = None;
        conversation.outbox.remove(&logical);
        conversation.delivered.insert(logical);
        check_members(conversation.group.members(), &conversation.roster)
    }

    /// A supplied next roster is an authorization assertion by the caller's trusted
    /// enrollment layer. This wrapper verifies cryptographic correspondence, not roles.
    pub fn receive(
        &mut self,
        envelope: &Envelope,
        verified_next_roster: Option<&[IdentityCard]>,
    ) -> Result<Received> {
        // The exact bytes of an envelope already processed (and so already
        // authenticated), with the exact roster it was verified against,
        // answer what they produced then. The roster is in the digest: a
        // replayed commit carrying another roster is not the same input and
        // goes through every check (security review 05/10).
        let conversation_id = envelope.conversation_id.as_str();
        let mut hasher = Sha256::new();
        hasher.update(serde_json::to_vec(envelope).map_err(|_| Error::Invalid)?);
        match verified_next_roster {
            None => hasher.update([0u8]),
            Some(cards) => {
                hasher.update([1u8]);
                hasher.update(serde_json::to_vec(cards).map_err(|_| Error::Invalid)?);
            }
        }
        let digest: [u8; 32] = hasher.finalize().into();
        if let Some((_, earlier)) = self
            .conversations
            .get(conversation_id)
            .and_then(|c| c.received.iter().find(|(d, _)| *d == digest))
        {
            return Ok(earlier.clone());
        }
        // Reject unauthenticated traffic before allocating a rollback snapshot.
        if envelope.epoch != self.epoch(conversation_id)? {
            return Err(Error::Authentication);
        }
        let sender = self
            .conversations
            .get(conversation_id)
            .and_then(|c| c.roster.get(&envelope.device_id))
            .ok_or(Error::Authentication)?;
        envelope.verify(&sender.transport_signature_key)?;
        let snapshot = self.capture_receive_state(conversation_id)?;
        let result = self.receive_inner(envelope, verified_next_roster);
        match &result {
            Err(_) => self.rollback_receive(snapshot)?,
            Ok(received) => {
                if let Some(conversation) = self.conversations.get_mut(conversation_id) {
                    if conversation.received.len() >= MAX_RECEIVED {
                        conversation.received.pop_front();
                    }
                    conversation.received.push_back((digest, received.clone()));
                }
            }
        }
        result
    }

    /// The app stored the results of every envelope processed so far in this
    /// conversation: they need not be answered again.
    pub fn settle_received(&mut self, conversation_id: &str) -> Result<()> {
        let conversation = self
            .conversations
            .get_mut(conversation_id)
            .ok_or(Error::State)?;
        if conversation.received.is_empty() {
            return Ok(());
        }
        conversation.received.clear();
        self.mutate()
    }

    fn receive_inner(
        &mut self,
        envelope: &Envelope,
        verified_next_roster: Option<&[IdentityCard]>,
    ) -> Result<Received> {
        let conversation_id = envelope.conversation_id.clone();
        let own_device = self.identity.device_id.clone();
        let epoch = self.epoch(&conversation_id)?;
        let conversation = self.conversation(&conversation_id)?;
        if conversation.pending.is_some() {
            return Err(Error::State);
        }
        if envelope.conversation_id.as_bytes() != conversation.group.group_id().as_slice()
            || envelope.epoch != epoch
        {
            return Err(Error::Authentication);
        }
        let sender = conversation
            .roster
            .get(&envelope.device_id)
            .ok_or(Error::Authentication)?
            .clone();
        envelope.verify(&sender.transport_signature_key)?;
        let message = MlsMessageIn::tls_deserialize_exact(&envelope.ciphertext).map_err(mls)?;
        if message.wire_format() != WireFormat::PrivateMessage {
            return Err(Error::Authentication);
        }
        let protocol = message.try_into_protocol_message().map_err(mls)?;
        if protocol.epoch() != conversation.group.epoch() {
            return Err(Error::Authentication);
        }
        let sender_index = conversation
            .group
            .members()
            .find(|member| sender.matches(&member.credential, &member.signature_key))
            .ok_or(Error::Authentication)?
            .index;
        self.mutate()?;
        let provider = &self.provider;
        let conversation = self
            .conversations
            .get_mut(&conversation_id)
            .ok_or(Error::State)?;
        let group = &mut conversation.group;
        let processed = group.process_message(provider, protocol).map_err(mls)?;
        if processed.aad() != envelope.aad()?
            || processed.sender() != &Sender::Member(sender_index)
            || processed.credential().serialized_content() != sender.identity()?
        {
            return Err(Error::Authentication);
        }
        match processed.into_content() {
            ProcessedMessageContent::ApplicationMessage(application) => {
                let bytes = Zeroizing::new(application.into_bytes());
                if bytes.len() > MAX_PAYLOAD {
                    return Err(Error::Invalid);
                }
                let payload: wire::Payload =
                    serde_json::from_slice(&bytes).map_err(|_| Error::Invalid)?;
                if payload.version != 1 {
                    return Err(Error::Invalid);
                }
                payload.operation.validate()?;
                Ok(Received::Application {
                    actor_id: sender.actor_id,
                    device_id: sender.device_id,
                    logical_send_id: envelope.logical_send_id.clone(),
                    operation: payload.operation,
                })
            }
            ProcessedMessageContent::StagedCommitMessage(staged) => {
                let expected = roster(verified_next_roster.ok_or(Error::Roster)?)?;
                check_commit(group, &staged, &conversation.roster, &expected)?;
                let removed = !expected.contains_key(&own_device);
                group.merge_staged_commit(provider, *staged).map_err(mls)?;
                if !removed {
                    check_members(group.members(), &expected)?;
                }
                conversation.roster = expected;
                if removed {
                    Ok(Received::Removed)
                } else {
                    Ok(Received::Commit {
                        epoch: go_epoch(&conversation.group)?,
                    })
                }
            }
            _ => Err(Error::State),
        }
    }
}

/// Go epoch = MLS epoch + 1 (the delivery service's 1-based epochs).
fn go_epoch(group: &MlsGroup) -> Result<i64> {
    i64::try_from(group.epoch().as_u64())
        .ok()
        .and_then(|epoch| epoch.checked_add(1))
        .ok_or(Error::State)
}

fn commit_digest(kind: &str, roster: &Roster) -> Result<[u8; 32]> {
    Ok(Sha256::digest(serde_json::to_vec(&(kind, roster)).map_err(|_| Error::Invalid)?).into())
}

fn check_commit(
    group: &MlsGroup,
    staged: &StagedCommit,
    current: &Roster,
    expected: &Roster,
) -> Result<()> {
    // Changing an existing device's identity keys requires a new device enrollment.
    for (id, card) in expected {
        if current.get(id).is_some_and(|old| old != card) {
            return Err(Error::Roster);
        }
    }
    let removed: BTreeSet<_> = staged
        .remove_proposals()
        .map(|p| p.remove_proposal().removed())
        .collect();
    let mut predicted: Vec<Member> = group
        .members()
        .filter(|member| !removed.contains(&member.index))
        .collect();
    for proposal in staged.queued_proposals() {
        match proposal.proposal() {
            Proposal::Add(add) => {
                let leaf = add.key_package().leaf_node();
                predicted.push(Member::new(
                    LeafNodeIndex::new(0),
                    Vec::new(),
                    leaf.signature_key().as_slice().to_vec(),
                    leaf.credential().clone(),
                ));
            }
            Proposal::Update(update) => {
                let leaf = update.leaf_node();
                if !expected
                    .values()
                    .any(|card| card.matches(leaf.credential(), leaf.signature_key().as_slice()))
                {
                    return Err(Error::Roster);
                }
            }
            Proposal::Remove(_) => {}
            _ => return Err(Error::Roster),
        }
    }
    if let Some(leaf) = staged.update_path_leaf_node() {
        if !expected
            .values()
            .any(|card| card.matches(leaf.credential(), leaf.signature_key().as_slice()))
        {
            return Err(Error::Roster);
        }
    }
    check_members(predicted.into_iter(), expected)
}
