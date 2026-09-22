//! Emits only public keys and an opaque MLS envelope made from synthetic data.
use base64::{engine::general_purpose::STANDARD, Engine};
use rudi_chat_crypto::{Client, Operation};

fn id(value: u64) -> String {
    format!("aaaaaaaa-bbbb-4ccc-8ddd-{value:012x}")
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut sender = Client::new(&id(1), &id(11))?;
    let mut peer = Client::new(&id(2), &id(22))?;
    sender.create_group(&id(100))?;
    let invitation = sender.stage_add(&id(200), &[(peer.identity(), peer.key_package()?)])?;
    peer.join_group(
        &id(100),
        invitation.welcome.as_ref().ok_or("missing welcome")?,
        &[sender.identity(), peer.identity()],
    )?;
    sender.acknowledge_commit(&invitation.envelope)?;
    let envelope = sender.encrypt(
        &id(300),
        Operation::Text {
            body: "Synthetic Go/MLS interoperability canary".into(),
        },
    )?;
    peer.receive(&envelope, None)?;
    println!(
        "{}",
        serde_json::json!({
            "public_key": STANDARD.encode(sender.identity().transport_signature_key),
            "signing_bytes": STANDARD.encode(envelope.signing_bytes()?),
            "envelope": envelope,
        })
    );
    Ok(())
}
