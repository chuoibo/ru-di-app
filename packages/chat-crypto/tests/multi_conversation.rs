//! ADR-0057 canaries: one device identity across many conversations, the key
//! package lifecycle, delivered-send GC, the epoch race, forgetting a
//! conversation, multi-conversation checkpoints, the v2 operations and media.
//! Synthetic data only.
use rudi_chat_crypto::{
    enrollment_bytes, open_media, seal_media, Client, Error, IdentityCard, MediaRef, Operation,
    Received,
};

fn id(value: u64) -> String {
    format!("aaaaaaaa-bbbb-4ccc-8ddd-{value:012x}")
}

fn client(actor: u64, device: u64) -> Client {
    Client::new(&id(actor), &id(device)).unwrap()
}

fn text(body: &str) -> Operation {
    Operation::Text { body: body.into() }
}

/// `creator` opens `conversation` and adds `joiners`, each with a fresh key package.
fn open(creator: &mut Client, conversation: u64, joiners: &mut [&mut Client]) {
    creator.create_group(&id(conversation)).unwrap();
    let members: Vec<(IdentityCard, Vec<u8>)> = joiners
        .iter_mut()
        .map(|j| (j.identity(), j.key_package().unwrap()))
        .collect();
    let bundle = creator
        .stage_add(&id(conversation), &id(conversation + 1000), &members)
        .unwrap();
    let mut roster = vec![creator.identity()];
    roster.extend(joiners.iter().map(|j| j.identity()));
    for joiner in joiners.iter_mut() {
        joiner
            .join_group(&id(conversation), bundle.welcome.as_ref().unwrap(), &roster)
            .unwrap();
    }
    creator.acknowledge_commit(&bundle.envelope).unwrap();
}

fn body(received: Received) -> Operation {
    match received {
        Received::Application { operation, .. } => operation,
        other => panic!("not an application message: {other:?}"),
    }
}

#[test]
fn one_device_identity_serves_two_conversations_independently() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    let mut carol = client(3, 33);
    open(&mut alice, 100, &mut [&mut bob]);
    open(&mut alice, 200, &mut [&mut bob, &mut carol]);
    assert_eq!(alice.conversations(), vec![id(100), id(200)]);
    assert_eq!(bob.conversations(), vec![id(100), id(200)]);
    assert_eq!(alice.roster(&id(100)).len(), 2);
    assert_eq!(alice.roster(&id(200)).len(), 3);

    let to_pair = alice
        .encrypt(&id(100), &id(300), text("cho phòng hai người"))
        .unwrap();
    let to_three = alice
        .encrypt(&id(200), &id(300), text("cho phòng ba người"))
        .unwrap();
    assert_eq!(
        body(bob.receive(&to_pair, None).unwrap()),
        text("cho phòng hai người")
    );
    assert_eq!(
        body(bob.receive(&to_three, None).unwrap()),
        text("cho phòng ba người")
    );
    assert_eq!(
        carol.receive(&to_pair, None),
        Err(Error::State),
        "carol is not in the pair room"
    );

    // A ciphertext of one room relabelled for the other fails closed.
    let mut moved = alice
        .encrypt(&id(100), &id(301), text("đúng phòng"))
        .unwrap();
    moved.conversation_id = id(200);
    assert!(bob.receive(&moved, None).is_err());

    // Rekeying one room moves its epoch alone.
    let rekey = alice.stage_rekey(&id(200), &id(302)).unwrap();
    alice.acknowledge_commit(&rekey.envelope).unwrap();
    assert_eq!(alice.epoch(&id(200)).unwrap(), 3);
    assert_eq!(alice.epoch(&id(100)).unwrap(), 2);
}

#[test]
fn many_key_packages_are_outstanding_and_each_welcome_consumes_one() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    let mut dave = client(4, 44);
    let first = bob.key_package().unwrap();
    let second = bob.key_package().unwrap();
    assert_ne!(first, second);
    alice.create_group(&id(100)).unwrap();
    dave.create_group(&id(200)).unwrap();
    let a = alice
        .stage_add(&id(100), &id(1100), &[(bob.identity(), first)])
        .unwrap();
    let d = dave
        .stage_add(&id(200), &id(1200), &[(bob.identity(), second)])
        .unwrap();
    bob.join_group(
        &id(100),
        a.welcome.as_ref().unwrap(),
        &[alice.identity(), bob.identity()],
    )
    .unwrap();
    bob.join_group(
        &id(200),
        d.welcome.as_ref().unwrap(),
        &[dave.identity(), bob.identity()],
    )
    .unwrap();
    assert_eq!(bob.conversations().len(), 2);

    let mut eve = client(5, 55);
    for _ in 0..64 {
        eve.key_package().unwrap();
    }
    let generation = eve.generation();
    assert_eq!(eve.key_package(), Err(Error::Capacity));
    assert_eq!(
        eve.generation(),
        generation,
        "a refused key package changes nothing"
    );
}

#[test]
fn a_delivered_send_leaves_the_outbox_and_cannot_be_reencrypted() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    let sent = alice.encrypt(&id(100), &id(300), text("đã giao")).unwrap();
    assert_eq!(
        alice.encrypt(&id(100), &id(300), text("đã giao")).unwrap(),
        sent,
        "before the ACK a retry is the same ciphertext"
    );
    alice.acknowledge_sent(&sent).unwrap();
    assert_eq!(
        alice.encrypt(&id(100), &id(300), text("đã giao")),
        Err(Error::Conflict),
        "after the ACK the logical id is spent"
    );
    assert_eq!(alice.acknowledge_sent(&sent), Err(Error::State));
    // The pending commit is acknowledged by acknowledge_commit, never as a send.
    let rekey = alice.stage_rekey(&id(100), &id(301)).unwrap();
    assert_eq!(
        alice.acknowledge_sent(&rekey.envelope),
        Err(Error::Conflict)
    );
    // The sender's GC does not touch the receiver: bob still reads the send,
    // then follows the rekey.
    assert_eq!(body(bob.receive(&sent, None).unwrap()), text("đã giao"));
    alice.acknowledge_commit(&rekey.envelope).unwrap();
    let roster = alice.roster(&id(100));
    assert_eq!(
        bob.receive(&rekey.envelope, Some(&roster)).unwrap(),
        Received::Commit { epoch: 3 }
    );
}

#[test]
fn the_loser_of_an_epoch_race_abandons_its_commit_and_follows_the_winner() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    let roster = alice.roster(&id(100));
    let winner = alice.stage_rekey(&id(100), &id(300)).unwrap();
    let loser = bob.stage_rekey(&id(100), &id(301)).unwrap();
    // The delivery service accepts alice's at epoch 2 and answers bob 409 epoch_moved.
    alice.acknowledge_commit(&winner.envelope).unwrap();
    assert_eq!(
        bob.receive(&winner.envelope, Some(&roster)),
        Err(Error::State)
    );
    bob.abandon_commit(&id(100)).unwrap();
    assert_eq!(bob.pending_commit(&id(100)).err(), Some(Error::State));
    assert_eq!(
        bob.receive(&winner.envelope, Some(&roster)).unwrap(),
        Received::Commit { epoch: 3 }
    );
    // Bob's abandoned commit never applies anywhere.
    assert!(alice.receive(&loser.envelope, Some(&roster)).is_err());
    let after = bob
        .encrypt(&id(100), &id(302), text("sau cuộc đua"))
        .unwrap();
    assert_eq!(
        body(alice.receive(&after, None).unwrap()),
        text("sau cuộc đua")
    );
}

#[test]
fn a_removed_device_forgets_the_conversation_and_keeps_the_others() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    open(&mut alice, 200, &mut [&mut bob]);
    let removal = alice.stage_remove(&id(100), &id(300), &id(22)).unwrap();
    alice.acknowledge_commit(&removal.envelope).unwrap();
    assert_eq!(
        bob.receive(&removal.envelope, Some(&[alice.identity()]))
            .unwrap(),
        Received::Removed
    );
    assert_eq!(bob.conversations(), vec![id(200)]);
    bob.forget(&id(100)).unwrap();
    assert_eq!(bob.forget(&id(100)), Err(Error::State));
    let still = alice
        .encrypt(&id(200), &id(301), text("phòng còn lại"))
        .unwrap();
    assert_eq!(
        body(bob.receive(&still, None).unwrap()),
        text("phòng còn lại")
    );
}

#[test]
fn a_checkpoint_carries_every_conversation_and_the_spent_logical_ids() {
    let key = [9u8; 32];
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    open(&mut alice, 200, &mut [&mut bob]);
    let delivered = alice.encrypt(&id(100), &id(300), text("đã giao")).unwrap();
    alice.acknowledge_sent(&delivered).unwrap();
    let queued = alice.encrypt(&id(200), &id(301), text("còn chờ")).unwrap();
    let sealed = alice.seal_local_state(&key).unwrap();
    let anchor = sealed.anchor().unwrap();
    drop(alice);
    let mut alice = Client::resume_local_state(&sealed, &key, &anchor).unwrap();
    assert_eq!(alice.conversations(), vec![id(100), id(200)]);
    assert_eq!(
        alice.encrypt(&id(200), &id(301), text("còn chờ")).unwrap(),
        queued
    );
    assert_eq!(
        alice.encrypt(&id(100), &id(300), text("đã giao")),
        Err(Error::Conflict)
    );
    assert_eq!(body(bob.receive(&queued, None).unwrap()), text("còn chờ"));
}

fn image(media: MediaRef) -> Operation {
    Operation::Image {
        media: Box::new(media),
        caption: Some("Ảnh mẫu".into()),
        width: 1200,
        height: 900,
    }
}

#[test]
fn v2_operations_and_sealed_media_round_trip_and_validate() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    let photo = b"synthetic jpeg bytes, exif already stripped".to_vec();
    let (uploaded, reference) = seal_media(&id(700), "image/jpeg", &photo).unwrap();
    assert_ne!(uploaded, photo);
    let operations = [
        Operation::Reply {
            reply_to: id(900),
            body: "trả lời".into(),
        },
        Operation::Edit {
            message_id: id(900),
            body: "đã sửa".into(),
        },
        Operation::Sticker {
            pack_id: "nep".into(),
            sticker_id: "cuoi-1".into(),
        },
        image(reference.clone()),
    ];
    for (i, operation) in operations.iter().enumerate() {
        let sent = alice
            .encrypt(&id(100), &id(310 + i as u64), operation.clone())
            .unwrap();
        assert_eq!(&body(bob.receive(&sent, None).unwrap()), operation);
    }
    assert_eq!(
        open_media(&reference, &uploaded).unwrap().as_slice(),
        photo.as_slice()
    );

    // The store cannot swap the bytes or move them under another reference.
    let mut tampered = uploaded.clone();
    tampered[0] ^= 1;
    assert_eq!(
        open_media(&reference, &tampered).err(),
        Some(Error::Authentication)
    );
    let mut moved = reference.clone();
    moved.media_id = id(701);
    assert_eq!(
        open_media(&moved, &uploaded).err(),
        Some(Error::Authentication)
    );

    // Invalid v2 operations never reach the ratchet.
    let generation = alice.generation();
    let mut wrong_mime = reference.clone();
    wrong_mime.mime = "application/pdf".into();
    for invalid in [
        Operation::Edit {
            message_id: id(900),
            body: " ".into(),
        },
        Operation::Reply {
            reply_to: "not-an-id".into(),
            body: "x".into(),
        },
        Operation::Sticker {
            pack_id: "Nep!".into(),
            sticker_id: "a".into(),
        },
        image(wrong_mime),
        Operation::Voice {
            media: Box::new(reference),
            duration_ms: 1000,
        },
    ] {
        assert_eq!(
            alice.encrypt(&id(100), &id(399), invalid),
            Err(Error::Invalid)
        );
    }
    assert_eq!(alice.generation(), generation);
}

/// Security review 05/10: a rejoin under the same conversation id must not
/// erase the group it just joined, and the device must keep working.
#[test]
fn a_removed_device_added_back_rejoins_and_reads_new_messages() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    let removal = alice.stage_remove(&id(100), &id(300), &id(22)).unwrap();
    alice.acknowledge_commit(&removal.envelope).unwrap();
    assert_eq!(
        bob.receive(&removal.envelope, Some(&[alice.identity()]))
            .unwrap(),
        Received::Removed
    );
    let back = alice
        .stage_add(
            &id(100),
            &id(301),
            &[(bob.identity(), bob.key_package().unwrap())],
        )
        .unwrap();
    alice.acknowledge_commit(&back.envelope).unwrap();
    bob.join_group(
        &id(100),
        back.welcome.as_ref().unwrap(),
        &[alice.identity(), bob.identity()],
    )
    .unwrap();
    let hello = alice.encrypt(&id(100), &id(302), text("chào lại")).unwrap();
    assert_eq!(body(bob.receive(&hello, None).unwrap()), text("chào lại"));
    let key = [7u8; 32];
    let sealed = bob.seal_local_state(&key).unwrap();
    let anchor = sealed.anchor().unwrap();
    let mut bob = Client::resume_local_state(&sealed, &key, &anchor).unwrap();
    let again = alice
        .encrypt(&id(100), &id(303), text("sau khởi động lại"))
        .unwrap();
    assert_eq!(
        body(bob.receive(&again, None).unwrap()),
        text("sau khởi động lại")
    );
}

/// The bytes a device signs to enroll are the Go server's, byte for byte: the
/// same vector is pinned in services/core/internal/chatv2 (enrollment_test.go).
#[test]
fn enrollment_bytes_match_the_go_vector_and_the_proof_verifies() {
    let key: [u8; 32] = core::array::from_fn(|i| i as u8);
    let bytes = enrollment_bytes(&id(1), &id(11), &key);
    let hex: String = bytes.iter().map(|b| format!("{b:02x}")).collect();
    assert_eq!(hex, "525544492d434841542d444556494345007631000000002461616161616161612d626262622d346363632d386464642d3030303030303030303030310000002461616161616161612d626262622d346363632d386464642d303030303030303030303062000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f");
    let alice = client(1, 11);
    let card = alice.identity();
    let proof = alice.enrollment_proof();
    let verifier = ed25519_dalek::VerifyingKey::from_bytes(&card.transport_signature_key).unwrap();
    let signature = ed25519_dalek::Signature::from_bytes(&proof);
    verifier
        .verify_strict(
            &enrollment_bytes(&card.actor_id, &card.device_id, &card.mls_signature_key),
            &signature,
        )
        .unwrap();
}

/// A send refused for a moved epoch is abandoned and the same logical ID is
/// encrypted again under the new epoch; a current send cannot be abandoned.
#[test]
fn a_send_of_a_past_epoch_is_abandoned_and_reencrypted() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    open(&mut alice, 100, &mut [&mut bob]);
    let stale = alice
        .encrypt(&id(100), &id(300), text("trễ epoch"))
        .unwrap();
    assert_eq!(
        alice.abandon_send(&stale),
        Err(Error::Conflict),
        "still the current epoch"
    );
    let rekey = bob.stage_rekey(&id(100), &id(301)).unwrap();
    bob.acknowledge_commit(&rekey.envelope).unwrap();
    let roster = bob.roster(&id(100));
    alice.receive(&rekey.envelope, Some(&roster)).unwrap();
    alice.abandon_send(&stale).unwrap();
    let fresh = alice
        .encrypt(&id(100), &id(300), text("trễ epoch"))
        .unwrap();
    assert_ne!(fresh, stale);
    assert_eq!(fresh.epoch, stale.epoch + 1);
    assert_eq!(body(bob.receive(&fresh, None).unwrap()), text("trễ epoch"));
}
