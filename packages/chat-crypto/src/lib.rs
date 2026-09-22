//! Experimental native MLS adapter. See README.md for the unimplemented release gates.
#![forbid(unsafe_code)]

#[cfg(test)]
mod adversarial;
mod local_state;
mod wire;

pub use local_state::{LocalAnchor, SealedLocalState};
pub use wire::{Envelope, Operation, MAX_CIPHERTEXT, PROTOCOL};

use std::collections::{BTreeMap, BTreeSet};

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

#[derive(Debug, PartialEq, Eq)]
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

/// Single-device, single-group prototype. Mutable access serializes ratchet changes.
/// No private key, plaintext export, server client, or logging API is exposed.
pub struct Client {
    provider: OpenMlsRustCrypto,
    signer: SignatureKeyPair,
    transport_signer: SigningKey,
    identity: IdentityCard,
    group: Option<MlsGroup>,
    roster: Roster,
    outbox: BTreeMap<String, StoredSend>,
    pending: Option<PendingCommit>,
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
            group: None,
            roster: BTreeMap::new(),
            outbox: BTreeMap::new(),
            pending: None,
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

    pub fn epoch(&self) -> Result<i64> {
        let epoch = self.active_group()?.epoch().as_u64();
        i64::try_from(epoch)
            .ok()
            .and_then(|epoch| epoch.checked_add(1))
            .ok_or(Error::State)
    }

    pub fn roster(&self) -> Vec<IdentityCard> {
        self.roster.values().cloned().collect()
    }

    fn mutate(&mut self) -> Result<()> {
        self.generation = self.generation.checked_add(1).ok_or(Error::Capacity)?;
        Ok(())
    }

    fn active_group(&self) -> Result<&MlsGroup> {
        self.group
            .as_ref()
            .filter(|group| group.is_active())
            .ok_or(Error::State)
    }

    fn credential(&self) -> Result<CredentialWithKey> {
        Ok(CredentialWithKey {
            credential: BasicCredential::new(self.identity.identity()?).into(),
            signature_key: self.signer.to_public_vec().into(),
        })
    }

    pub fn key_package(&mut self) -> Result<Vec<u8>> {
        // This prototype has no key package lifecycle service. Bound unjoined state.
        if self.group.is_some() {
            return Err(Error::State);
        }
        if self.generation >= 16 {
            return Err(Error::Capacity);
        }
        self.mutate()?;
        KeyPackage::builder()
            .build(SUITE, &self.provider, &self.signer, self.credential()?)
            .map_err(mls)?
            .key_package()
            .tls_serialize_detached()
            .map_err(mls)
    }

    pub fn create_group(&mut self, conversation_id: &str) -> Result<()> {
        if self.group.is_some() || !wire::valid_id(conversation_id) {
            return Err(Error::State);
        }
        self.mutate()?;
        self.group = Some(
            MlsGroup::new_with_group_id(
                &self.provider,
                &self.signer,
                &config(),
                GroupId::from_slice(conversation_id.as_bytes()),
                self.credential()?,
            )
            .map_err(mls)?,
        );
        self.roster = roster(&[self.identity()])?;
        Ok(())
    }

    pub fn join_group(
        &mut self,
        conversation_id: &str,
        welcome: &[u8],
        verified_roster: &[IdentityCard],
    ) -> Result<()> {
        if self.group.is_some() || welcome.len() > MAX_CIPHERTEXT {
            return Err(Error::State);
        }
        let snapshot = self.capture_receive_state()?;
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
        if self.group.is_some()
            || !wire::valid_id(conversation_id)
            || welcome.len() > MAX_CIPHERTEXT
        {
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
        let staged =
            StagedWelcome::new_from_welcome(&self.provider, config().join_config(), welcome, None)
                .map_err(mls)?;
        if staged.group_context().group_id().as_slice() != conversation_id.as_bytes()
            || staged.group_context().ciphersuite() != SUITE
        {
            return Err(Error::Authentication);
        }
        check_members(staged.members(), &expected)?;
        self.group = Some(staged.into_group(&self.provider).map_err(mls)?);
        self.roster = expected;
        Ok(())
    }

    fn unsigned(&self, logical: &str) -> Result<Envelope> {
        let context = std::str::from_utf8(self.active_group()?.group_id().as_slice())
            .map_err(|_| Error::State)?;
        Envelope::unsigned(context, &self.identity.device_id, logical, self.epoch()?)
    }

    fn previous(&self, logical: &str, digest: &[u8; 32]) -> Result<Option<Envelope>> {
        if let Some(stored) = self.outbox.get(logical) {
            if stored.digest != *digest {
                return Err(Error::Conflict);
            }
            return Ok(Some(stored.envelope.clone()));
        }
        if self.outbox.len() >= MAX_OUTBOX {
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
        self.outbox.insert(
            envelope.logical_send_id.clone(),
            StoredSend {
                digest,
                envelope: envelope.clone(),
            },
        );
        Ok(envelope)
    }

    /// A repeated logical ID returns identical ciphertext; it never advances a ratchet twice.
    pub fn encrypt(&mut self, logical_send_id: &str, operation: Operation) -> Result<Envelope> {
        operation.validate()?;
        let payload = Zeroizing::new(
            serde_json::to_vec(&wire::Payload {
                version: 1,
                operation,
            })
            .map_err(|_| Error::Invalid)?,
        );
        let digest = Sha256::digest(payload.as_slice()).into();
        // An inactive or removed device cannot use the retained outbox to send again.
        self.active_group()?;
        if let Some(previous) = self.previous(logical_send_id, &digest)? {
            return Ok(previous);
        }
        if self.pending.is_some() {
            return Err(Error::State);
        }
        let envelope = self.unsigned(logical_send_id)?;
        self.mutate()?;
        let group = self.group.as_mut().ok_or(Error::State)?;
        group.set_aad(envelope.aad()?);
        let message = group
            .create_message(&self.provider, &self.signer, &payload)
            .map_err(mls)?;
        self.finish(envelope, message, digest)
    }

    pub fn stage_add(
        &mut self,
        logical_send_id: &str,
        members: &[(IdentityCard, Vec<u8>)],
    ) -> Result<CommitBundle> {
        if members.is_empty() || self.pending.is_some() {
            return Err(Error::State);
        }
        if members.len() > MAX_LEAVES.saturating_sub(self.roster.len()) {
            return Err(Error::Roster);
        }
        let mut next = self.roster.clone();
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
        if self.previous(logical_send_id, &digest)?.is_some() {
            return Err(Error::Conflict);
        }
        let envelope = self.unsigned(logical_send_id)?;
        self.mutate()?;
        let group = self.group.as_mut().ok_or(Error::State)?;
        group.set_aad(envelope.aad()?);
        let (message, welcome, _) = group
            .add_members(&self.provider, &self.signer, &packages)
            .map_err(mls)?;
        let welcome = welcome.to_bytes().map_err(mls)?;
        self.finish_commit(envelope, message, digest, next, Some(welcome))
    }

    pub fn stage_remove(&mut self, logical_send_id: &str, device_id: &str) -> Result<CommitBundle> {
        if self.pending.is_some() || device_id == self.identity.device_id {
            return Err(Error::State);
        }
        let removed = self.roster.get(device_id).ok_or(Error::Roster)?;
        let index = self
            .active_group()?
            .members()
            .find(|member| removed.matches(&member.credential, &member.signature_key))
            .ok_or(Error::Roster)?
            .index;
        let mut next = self.roster.clone();
        next.remove(device_id);
        let digest = commit_digest("remove", &next)?;
        if self.previous(logical_send_id, &digest)?.is_some() {
            return Err(Error::Conflict);
        }
        let envelope = self.unsigned(logical_send_id)?;
        self.mutate()?;
        let group = self.group.as_mut().ok_or(Error::State)?;
        group.set_aad(envelope.aad()?);
        let (message, _, _) = group
            .remove_members(&self.provider, &self.signer, &[index])
            .map_err(mls)?;
        self.finish_commit(envelope, message, digest, next, None)
    }

    pub fn stage_rekey(&mut self, logical_send_id: &str) -> Result<CommitBundle> {
        if self.pending.is_some() {
            return Err(Error::State);
        }
        let next = self.roster.clone();
        let digest = commit_digest("rekey", &next)?;
        if self.previous(logical_send_id, &digest)?.is_some() {
            return Err(Error::Conflict);
        }
        let envelope = self.unsigned(logical_send_id)?;
        self.mutate()?;
        let group = self.group.as_mut().ok_or(Error::State)?;
        group.set_aad(envelope.aad()?);
        let message = group
            .self_update(&self.provider, &self.signer, LeafNodeParameters::default())
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
        self.pending = Some(PendingCommit {
            logical_send_id: envelope.logical_send_id.clone(),
            next_roster,
            welcome: welcome.clone(),
        });
        Ok(CommitBundle { envelope, welcome })
    }

    /// Retry the exact pending commit after a timeout or local process restart.
    pub fn pending_commit(&self) -> Result<CommitBundle> {
        let pending = self.pending.as_ref().ok_or(Error::State)?;
        let envelope = self
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

    /// Call only after durable delivery-service acceptance of this exact envelope.
    /// Production must atomically persist this state transition with the ACK cursor.
    pub fn acknowledge_commit(&mut self, accepted: &Envelope) -> Result<()> {
        let pending = self.pending.as_ref().ok_or(Error::State)?;
        let sent = self
            .outbox
            .get(&pending.logical_send_id)
            .ok_or(Error::State)?;
        if &sent.envelope != accepted {
            return Err(Error::Conflict);
        }
        let next = pending.next_roster.clone();
        self.mutate()?;
        let group = self.group.as_mut().ok_or(Error::State)?;
        group.merge_pending_commit(&self.provider).map_err(mls)?;
        check_members(group.members(), &next)?;
        self.roster = next;
        self.pending = None;
        Ok(())
    }

    /// A supplied next roster is an authorization assertion by the caller's trusted
    /// enrollment layer. This wrapper verifies cryptographic correspondence, not roles.
    pub fn receive(
        &mut self,
        envelope: &Envelope,
        verified_next_roster: Option<&[IdentityCard]>,
    ) -> Result<Received> {
        // Reject unauthenticated traffic before allocating a rollback snapshot.
        let group = self.active_group()?;
        if envelope.conversation_id.as_bytes() != group.group_id().as_slice()
            || envelope.epoch != self.epoch()?
        {
            return Err(Error::Authentication);
        }
        let sender = self
            .roster
            .get(&envelope.device_id)
            .ok_or(Error::Authentication)?;
        envelope.verify(&sender.transport_signature_key)?;
        let snapshot = self.capture_receive_state()?;
        let result = self.receive_inner(envelope, verified_next_roster);
        if result.is_err() {
            self.rollback_receive(snapshot)?;
        }
        result
    }

    fn receive_inner(
        &mut self,
        envelope: &Envelope,
        verified_next_roster: Option<&[IdentityCard]>,
    ) -> Result<Received> {
        let group = self.active_group()?;
        if self.pending.is_some() {
            return Err(Error::State);
        }
        if envelope.conversation_id.as_bytes() != group.group_id().as_slice()
            || envelope.epoch != self.epoch()?
        {
            return Err(Error::Authentication);
        }
        let sender = self
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
        if protocol.epoch() != group.epoch() {
            return Err(Error::Authentication);
        }
        let sender_index = group
            .members()
            .find(|member| sender.matches(&member.credential, &member.signature_key))
            .ok_or(Error::Authentication)?
            .index;
        self.mutate()?;
        let group = self.group.as_mut().ok_or(Error::State)?;
        let processed = group
            .process_message(&self.provider, protocol)
            .map_err(mls)?;
        if processed.aad() != envelope.aad()?
            || processed.sender() != &Sender::Member(sender_index)
            || processed.credential().serialized_content() != sender.identity()?
        {
            return Err(Error::Authentication);
        }
        match processed.into_content() {
            ProcessedMessageContent::ApplicationMessage(application) => {
                let bytes = Zeroizing::new(application.into_bytes());
                if bytes.len() > 20 * 1024 {
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
                check_commit(group, &staged, &self.roster, &expected)?;
                let removed = !expected.contains_key(&self.identity.device_id);
                group
                    .merge_staged_commit(&self.provider, *staged)
                    .map_err(mls)?;
                if !removed {
                    check_members(group.members(), &expected)?;
                }
                self.roster = expected;
                if removed {
                    Ok(Received::Removed)
                } else {
                    Ok(Received::Commit {
                        epoch: self.epoch()?,
                    })
                }
            }
            _ => Err(Error::State),
        }
    }
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
