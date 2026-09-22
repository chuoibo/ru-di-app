use rudi_chat_crypto::{
    Client, Envelope, Error, IdentityCard, Operation, Received, SealedLocalState,
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

fn group() -> (Client, Client, Client) {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    let mut carol = client(3, 33);
    alice.create_group(&id(100)).unwrap();
    assert_eq!(alice.epoch().unwrap(), 1);
    let bundle = alice
        .stage_add(
            &id(200),
            &[
                (bob.identity(), bob.key_package().unwrap()),
                (carol.identity(), carol.key_package().unwrap()),
            ],
        )
        .unwrap();
    assert_eq!(
        alice.epoch().unwrap(),
        1,
        "no epoch advance before service ACK"
    );
    let verified = vec![alice.identity(), bob.identity(), carol.identity()];
    bob.join_group(&id(100), bundle.welcome.as_ref().unwrap(), &verified)
        .unwrap();
    carol
        .join_group(&id(100), bundle.welcome.as_ref().unwrap(), &verified)
        .unwrap();
    alice.acknowledge_commit(&bundle.envelope).unwrap();
    assert_eq!(alice.epoch().unwrap(), 2);
    assert_eq!(bob.epoch().unwrap(), 2);
    (alice, bob, carol)
}

fn application(value: Received, expected: &Operation, sender: &IdentityCard) {
    let Received::Application {
        actor_id,
        device_id,
        operation,
        ..
    } = value
    else {
        panic!("not an application message");
    };
    assert_eq!(actor_id, sender.actor_id);
    assert_eq!(device_id, sender.device_id);
    assert_eq!(&operation, expected);
}

#[test]
fn three_real_mls_peers_exchange_typed_operations_and_go_json() {
    let (mut alice, mut bob, mut carol) = group();
    let operations = [
        text("Synthetic canary: ngày đi cùng nhau"),
        Operation::Reaction {
            message_id: id(900),
            emoji: "❤".into(),
        },
        Operation::Vote {
            poll_id: id(901),
            option_id: id(902),
        },
        Operation::Delete {
            message_id: id(900),
        },
    ];
    for (index, operation) in operations.iter().enumerate() {
        let envelope = alice
            .encrypt(&id(300 + index as u64), operation.clone())
            .unwrap();
        envelope
            .verify(&alice.identity().transport_signature_key)
            .unwrap();
        let json = serde_json::to_value(&envelope).unwrap();
        assert!(
            json["ciphertext"].is_string(),
            "Go []byte JSON uses base64 strings"
        );
        assert!(json["signature"].is_string());
        let decoded: Envelope = serde_json::from_value(json).unwrap();
        application(
            bob.receive(&decoded, None).unwrap(),
            operation,
            &alice.identity(),
        );
        application(
            carol.receive(&decoded, None).unwrap(),
            operation,
            &alice.identity(),
        );
        assert!(!envelope
            .ciphertext
            .windows(16)
            .any(|part| part == b"Synthetic canary"));
    }
    let reply = bob
        .encrypt(&id(310), text("Reply from a different MLS leaf"))
        .unwrap();
    application(
        alice.receive(&reply, None).unwrap(),
        &text("Reply from a different MLS leaf"),
        &bob.identity(),
    );
    application(
        carol.receive(&reply, None).unwrap(),
        &text("Reply from a different MLS leaf"),
        &bob.identity(),
    );
}

#[test]
fn retry_reuses_ciphertext_and_changed_payload_conflicts() {
    let (mut alice, mut bob, _) = group();
    let envelope = alice.encrypt(&id(300), text("one logical send")).unwrap();
    let generation = alice.generation();
    assert_eq!(
        alice.encrypt(&id(300), text("one logical send")).unwrap(),
        envelope
    );
    assert_eq!(alice.generation(), generation);
    assert_eq!(
        alice.encrypt(&id(300), text("changed payload")),
        Err(Error::Conflict)
    );
    bob.receive(&envelope, None).unwrap();
    assert!(
        bob.receive(&envelope, None).is_err(),
        "MLS rejects a replayed generation"
    );
}

#[test]
fn bounded_out_of_order_delivery_works_and_replay_does_not() {
    let (mut alice, mut bob, _) = group();
    let first = alice.encrypt(&id(301), text("first")).unwrap();
    let second = alice.encrypt(&id(302), text("second")).unwrap();
    application(
        bob.receive(&second, None).unwrap(),
        &text("second"),
        &alice.identity(),
    );
    application(
        bob.receive(&first, None).unwrap(),
        &text("first"),
        &alice.identity(),
    );
    assert!(bob.receive(&second, None).is_err());
}

#[test]
fn rekey_requires_ack_and_drops_prior_epoch_messages() {
    let (mut alice, mut bob, mut carol) = group();
    let delayed = alice
        .encrypt(&id(301), text("old epoch, intentionally delayed"))
        .unwrap();
    let rekey = alice.stage_rekey(&id(400)).unwrap();
    assert_eq!(alice.epoch().unwrap(), 2);
    assert_eq!(
        alice.encrypt(&id(302), text("while pending")),
        Err(Error::State)
    );
    assert_eq!(alice.pending_commit().unwrap().envelope, rekey.envelope);
    let mut wrong_ack = rekey.envelope.clone();
    wrong_ack.logical_send_id = id(999);
    assert_eq!(alice.acknowledge_commit(&wrong_ack), Err(Error::Conflict));
    let verified = alice.roster();
    assert_eq!(
        bob.receive(&rekey.envelope, Some(&verified)).unwrap(),
        Received::Commit { epoch: 3 }
    );
    carol.receive(&rekey.envelope, Some(&verified)).unwrap();
    alice.acknowledge_commit(&rekey.envelope).unwrap();
    assert_eq!(bob.receive(&delayed, None), Err(Error::Authentication));
    let next = carol.encrypt(&id(303), text("new epoch")).unwrap();
    application(
        alice.receive(&next, None).unwrap(),
        &text("new epoch"),
        &carol.identity(),
    );
    application(
        bob.receive(&next, None).unwrap(),
        &text("new epoch"),
        &carol.identity(),
    );
}

#[test]
fn removal_excludes_online_and_offline_devices_from_future_messages() {
    let (mut alice, mut bob, mut carol) = group();
    let previous_send = carol.encrypt(&id(310), text("before removal")).unwrap();
    let removal = alice
        .stage_remove(&id(400), &carol.identity().device_id)
        .unwrap();
    let verified = vec![alice.identity(), bob.identity()];
    bob.receive(&removal.envelope, Some(&verified)).unwrap();
    alice.acknowledge_commit(&removal.envelope).unwrap();
    let future = alice.encrypt(&id(301), text("after removal")).unwrap();
    assert!(
        carol.receive(&future, None).is_err(),
        "offline removed leaf cannot decrypt new epoch"
    );
    assert_eq!(
        carol.receive(&removal.envelope, Some(&verified)).unwrap(),
        Received::Removed
    );
    assert_eq!(carol.encrypt(&id(302), text("removed")), Err(Error::State));
    assert_eq!(
        carol.encrypt(&id(310), text("before removal")),
        Err(Error::State)
    );
    assert!(alice.receive(&previous_send, None).is_err());
    application(
        bob.receive(&future, None).unwrap(),
        &text("after removal"),
        &alice.identity(),
    );
}

#[test]
fn unauthorized_roster_commit_does_not_consume_receive_ratchet() {
    let (mut alice, mut bob, mut carol) = group();
    let removal = alice
        .stage_remove(&id(400), &carol.identity().device_id)
        .unwrap();
    assert_eq!(
        bob.receive(&removal.envelope, Some(&bob.roster())),
        Err(Error::Roster)
    );
    assert_eq!(bob.epoch().unwrap(), 2);
    let expected = vec![alice.identity(), bob.identity()];
    assert_eq!(
        bob.receive(&removal.envelope, Some(&expected)).unwrap(),
        Received::Commit { epoch: 3 }
    );
    assert_eq!(carol.receive(&removal.envelope, None), Err(Error::Roster));
    assert_eq!(
        carol.receive(&removal.envelope, Some(&expected)).unwrap(),
        Received::Removed
    );
}

#[test]
fn add_member_validates_key_package_and_welcome_roster() {
    let (mut alice, mut bob, mut carol) = group();
    let mut dave = client(4, 44);
    let package = dave.key_package().unwrap();
    let mut substituted = dave.identity();
    substituted.actor_id = id(999);
    assert!(matches!(
        alice.stage_add(&id(400), &[(substituted, package.clone())]),
        Err(Error::Authentication)
    ));
    let mut trailing = package.clone();
    trailing.push(0);
    assert!(alice
        .stage_add(&id(400), &[(dave.identity(), trailing)])
        .is_err());
    let addition = alice
        .stage_add(&id(400), &[(dave.identity(), package)])
        .unwrap();
    let mut expected = alice.roster();
    expected.push(dave.identity());
    bob.receive(&addition.envelope, Some(&expected)).unwrap();
    carol.receive(&addition.envelope, Some(&expected)).unwrap();
    alice.acknowledge_commit(&addition.envelope).unwrap();
    dave.join_group(&id(100), addition.welcome.as_ref().unwrap(), &expected)
        .unwrap();
    let sent = dave.encrypt(&id(300), text("newly joined")).unwrap();
    application(
        bob.receive(&sent, None).unwrap(),
        &text("newly joined"),
        &dave.identity(),
    );
}

#[test]
fn tampered_outer_fields_ciphertext_signature_and_cross_group_fail_closed() {
    let (mut alice, mut bob, _) = group();
    let envelope = alice.encrypt(&id(300), text("authenticated")).unwrap();
    let mut mutations = vec![envelope.clone(); 7];
    mutations[0].device_id = bob.identity().device_id;
    mutations[1].logical_send_id = id(999);
    mutations[2].epoch += 1;
    mutations[3].conversation_id = id(999);
    mutations[4].ciphertext[10] ^= 1;
    mutations[5].signature[10] ^= 1;
    mutations[6].protocol = "plaintext-fallback".into();
    for mutation in mutations {
        assert!(bob.receive(&mutation, None).is_err());
    }
    application(
        bob.receive(&envelope, None).unwrap(),
        &text("authenticated"),
        &alice.identity(),
    );
}

#[test]
fn local_restart_preserves_replay_rejection_and_identical_outbox() {
    let (mut alice, mut bob, _) = group();
    let operation = text("persisted retry");
    let envelope = alice.encrypt(&id(300), operation.clone()).unwrap();
    bob.receive(&envelope, None).unwrap();
    let key = [7; 32];
    let alice_state = alice.seal_local_state(&key).unwrap();
    let bob_state = bob.seal_local_state(&key).unwrap();
    let alice_anchor = alice_state.anchor().unwrap();
    let bob_anchor = bob_state.anchor().unwrap();
    // Drop the original objects before re-opening the same device session.
    drop(alice);
    drop(bob);
    let mut alice = Client::resume_local_state(&alice_state, &key, &alice_anchor).unwrap();
    let mut bob = Client::resume_local_state(&bob_state, &key, &bob_anchor).unwrap();
    assert_eq!(alice.encrypt(&id(300), operation).unwrap(), envelope);
    assert!(bob.receive(&envelope, None).is_err());
    let next = alice.encrypt(&id(301), text("after restart")).unwrap();
    application(
        bob.receive(&next, None).unwrap(),
        &text("after restart"),
        &alice.identity(),
    );
}

#[test]
fn pending_commit_survives_local_restart_without_epoch_jump() {
    let (mut alice, mut bob, _) = group();
    let commit = alice.stage_rekey(&id(400)).unwrap();
    let sealed = alice.seal_local_state(&[7; 32]).unwrap();
    let anchor = sealed.anchor().unwrap();
    drop(alice);
    let mut alice = Client::resume_local_state(&sealed, &[7; 32], &anchor).unwrap();
    assert_eq!(alice.epoch().unwrap(), 2);
    assert_eq!(alice.pending_commit().unwrap().envelope, commit.envelope);
    bob.receive(&commit.envelope, Some(&alice.roster()))
        .unwrap();
    alice.acknowledge_commit(&commit.envelope).unwrap();
    assert_eq!(alice.epoch().unwrap(), 3);
}

#[test]
fn stale_checkpoint_wrong_key_tamper_and_other_device_anchor_are_rejected() {
    let (mut alice, _, _) = group();
    let key = [7; 32];
    let old = alice.seal_local_state(&key).unwrap();
    alice.encrypt(&id(300), text("advances ratchet")).unwrap();
    let latest = alice.seal_local_state(&key).unwrap();
    let anchor = latest.anchor().unwrap();
    assert!(matches!(
        Client::resume_local_state(&old, &key, &anchor),
        Err(Error::Checkpoint)
    ));
    assert!(matches!(
        Client::resume_local_state(&latest, &[8; 32], &anchor),
        Err(Error::Checkpoint)
    ));
    let mut other_device = anchor.clone();
    other_device.device_id = id(999);
    assert!(matches!(
        Client::resume_local_state(&latest, &key, &other_device),
        Err(Error::Checkpoint)
    ));
    let mut json = serde_json::to_value(&latest).unwrap();
    json["ciphertext"][0] = serde_json::json!(json["ciphertext"][0].as_u64().unwrap() ^ 1);
    let tampered: SealedLocalState = serde_json::from_value(json).unwrap();
    assert!(matches!(
        Client::resume_local_state(&tampered, &key, &anchor),
        Err(Error::Checkpoint)
    ));
    // Even replacing the untrusted blob's hash cannot bypass AEAD integrity.
    assert!(matches!(
        Client::resume_local_state(&tampered, &key, &tampered.anchor().unwrap()),
        Err(Error::Checkpoint)
    ));
}

#[test]
fn backup_recovery_issues_new_identity_and_needs_fresh_membership() {
    let (mut alice, bob, _) = group();
    let old_identity = bob.identity();
    assert!(matches!(
        Client::recover_as_new_device(&old_identity, &old_identity.device_id),
        Err(Error::Invalid)
    ));
    let mut recovered = Client::recover_as_new_device(&old_identity, &id(99)).unwrap();
    assert_eq!(recovered.identity().actor_id, old_identity.actor_id);
    assert_ne!(
        recovered.identity().mls_signature_key,
        old_identity.mls_signature_key
    );
    assert_ne!(
        recovered.identity().transport_signature_key,
        old_identity.transport_signature_key
    );
    assert!(recovered.roster().is_empty());
    let envelope = alice
        .encrypt(&id(300), text("old membership is not restored"))
        .unwrap();
    assert_eq!(recovered.receive(&envelope, None), Err(Error::State));
}

#[test]
fn invalid_payloads_do_not_advance_send_ratchet() {
    let (mut alice, _, _) = group();
    let generation = alice.generation();
    for invalid in [
        text(" "),
        text(&"a".repeat(16385)),
        Operation::Delete {
            message_id: "not-a-uuid".into(),
        },
        Operation::Reaction {
            message_id: id(900),
            emoji: "".into(),
        },
    ] {
        assert_eq!(alice.encrypt(&id(300), invalid), Err(Error::Invalid));
    }
    assert_eq!(alice.generation(), generation);
}

#[test]
fn welcome_key_substitution_and_wrong_context_do_not_destroy_join_state() {
    let mut alice = client(1, 11);
    let mut bob = client(2, 22);
    alice.create_group(&id(100)).unwrap();
    let invitation = alice
        .stage_add(&id(200), &[(bob.identity(), bob.key_package().unwrap())])
        .unwrap();
    let welcome = invitation.welcome.unwrap();
    let correct = vec![alice.identity(), bob.identity()];
    let mut wrong = correct.clone();
    wrong[0].mls_signature_key[0] ^= 1;
    assert_eq!(
        bob.join_group(&id(100), &welcome, &wrong),
        Err(Error::Roster)
    );
    assert_eq!(
        bob.join_group(&id(999), &welcome, &correct),
        Err(Error::Authentication)
    );
    bob.join_group(&id(100), &welcome, &correct).unwrap();
}

#[test]
fn existing_device_transport_key_cannot_be_substituted_during_rekey() {
    let (mut alice, mut bob, _) = group();
    let commit = alice.stage_rekey(&id(400)).unwrap();
    let mut wrong = alice.roster();
    wrong[0].transport_signature_key[0] ^= 1;
    assert_eq!(
        bob.receive(&commit.envelope, Some(&wrong)),
        Err(Error::Roster)
    );
    assert!(bob.receive(&commit.envelope, Some(&alice.roster())).is_ok());
}

#[test]
fn sixth_device_for_one_account_is_rejected_before_group_mutation() {
    let mut alice = client(1, 11);
    alice.create_group(&id(100)).unwrap();
    let mut members = Vec::new();
    for device in 21..27 {
        let mut candidate = client(2, device);
        members.push((candidate.identity(), candidate.key_package().unwrap()));
    }
    let generation = alice.generation();
    assert!(matches!(
        alice.stage_add(&id(200), &members),
        Err(Error::Roster)
    ));
    assert_eq!(alice.generation(), generation);
    assert_eq!(alice.roster().len(), 1);
}

#[test]
fn full_outbox_fails_closed_without_discarding_retry_ciphertext() {
    let (mut alice, _, _) = group();
    let first = alice.encrypt(&id(300), text("bounded outbox")).unwrap();
    for logical in 301..427 {
        alice.encrypt(&id(logical), text("bounded outbox")).unwrap();
    }
    let generation = alice.generation();
    assert_eq!(
        alice.encrypt(&id(427), text("must not discard old sends")),
        Err(Error::Capacity)
    );
    assert_eq!(alice.generation(), generation);
    assert_eq!(
        alice.encrypt(&id(300), text("bounded outbox")).unwrap(),
        first
    );
}
