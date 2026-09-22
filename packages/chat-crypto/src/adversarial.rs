//! Attacker canaries intentionally access private signing keys inside the test
//! module to simulate malicious enrolled devices, not just corrupted transport.
use super::*;

fn id(value: u64) -> String {
    format!("aaaaaaaa-bbbb-4ccc-8ddd-{value:012x}")
}

fn pair() -> (Client, Client) {
    let mut alice = Client::new(&id(1), &id(11)).unwrap();
    let mut bob = Client::new(&id(2), &id(22)).unwrap();
    alice.create_group(&id(100)).unwrap();
    let invitation = alice
        .stage_add(&id(200), &[(bob.identity(), bob.key_package().unwrap())])
        .unwrap();
    bob.join_group(
        &id(100),
        invitation.welcome.as_ref().unwrap(),
        &[alice.identity(), bob.identity()],
    )
    .unwrap();
    alice.acknowledge_commit(&invitation.envelope).unwrap();
    (alice, bob)
}

#[test]
fn valid_outer_signature_cannot_rebind_mls_logical_id() {
    let (mut alice, mut bob) = pair();
    let original = alice
        .encrypt(
            &id(300),
            Operation::Text {
                body: "Synthetic bound message".into(),
            },
        )
        .unwrap();
    let mut resigned = original.clone();
    resigned.logical_send_id = id(301);
    resigned.sign(&alice.transport_signer).unwrap();
    resigned
        .verify(&alice.identity.transport_signature_key)
        .unwrap();
    assert_eq!(bob.receive(&resigned, None), Err(Error::Authentication));
    assert!(matches!(
        bob.receive(&original, None),
        Ok(Received::Application { .. })
    ));
}

#[test]
fn valid_outer_signature_cannot_impersonate_another_mls_leaf() {
    let (mut alice, mut bob) = pair();
    let original = alice
        .encrypt(
            &id(300),
            Operation::Text {
                body: "Synthetic sender binding".into(),
            },
        )
        .unwrap();
    let mut forged = original.clone();
    forged.device_id = bob.identity.device_id.clone();
    forged.sign(&bob.transport_signer).unwrap();
    forged
        .verify(&bob.identity.transport_signature_key)
        .unwrap();
    assert_eq!(bob.receive(&forged, None), Err(Error::Authentication));
    assert!(bob.receive(&original, None).is_ok());
}

#[test]
fn enrolled_sender_cannot_make_corrupt_mls_ciphertext_acceptable() {
    let (mut alice, mut bob) = pair();
    let original = alice
        .encrypt(
            &id(300),
            Operation::Text {
                body: "Synthetic ciphertext integrity".into(),
            },
        )
        .unwrap();
    for limit in [
        1,
        3,
        original.ciphertext.len() / 2,
        original.ciphertext.len() - 1,
    ] {
        let mut truncated = original.clone();
        truncated.ciphertext.truncate(limit);
        truncated.sign(&alice.transport_signer).unwrap();
        assert!(bob.receive(&truncated, None).is_err());
    }
    let mut trailing = original.clone();
    trailing.ciphertext.push(0);
    trailing.sign(&alice.transport_signer).unwrap();
    assert!(bob.receive(&trailing, None).is_err());
    assert!(bob.receive(&original, None).is_ok());
}

#[test]
fn authenticated_but_unknown_payload_version_is_rejected() {
    let (mut alice, mut bob) = pair();
    let envelope = alice.unsigned(&id(300)).unwrap();
    let group = alice.group.as_mut().unwrap();
    group.set_aad(envelope.aad().unwrap());
    let message = group
        .create_message(
            &alice.provider,
            &alice.signer,
            br#"{"version":2,"operation":{"type":"text","body":"Synthetic unknown version"}}"#,
        )
        .unwrap();
    let envelope = alice.finish(envelope, message, [0; 32]).unwrap();
    assert_eq!(bob.receive(&envelope, None), Err(Error::Invalid));
    assert_eq!(bob.receive(&envelope, None), Err(Error::Invalid));
}
