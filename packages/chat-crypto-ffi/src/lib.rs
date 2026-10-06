//! C ABI over `rudi-chat-crypto`, for calling MLS from the Android and iOS app.
//!
//! Shape: JSON in, JSON out, one owned C string back. Keeping the boundary at
//! JSON means the ABI stays small -- the lifecycle goes through one
//! `rudi_chat_crypto_call(handle, method, args)` -- and every argument that
//! crosses it is already validated by the core crate rather than by
//! hand-written marshalling. Binary values travel as standard base64.
//!
//! What never crosses: a private key, a plaintext export, or a panic. The core
//! crate exposes no key accessor, every entry point here catches unwinding, and
//! errors come back as a JSON object with a stable `error` string so the app
//! can branch on it without parsing prose.

use std::ffi::{c_char, CStr, CString};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::ptr;
use std::sync::{Mutex, MutexGuard};

use base64::{engine::general_purpose::STANDARD, Engine};
use rudi_chat_crypto::{
    Client, Envelope, IdentityCard, LocalAnchor, MediaRef, Operation, SealedLocalState,
};
use serde::Deserialize;
use zeroize::Zeroizing;

/// Opaque handle. The app holds the pointer and gives it back; it never reads
/// through it, and the layout is deliberately not part of the ABI. The mutex
/// makes two threads calling at once wait for each other instead of racing
/// one ratchet; the native module still serializes calls on one queue.
pub struct ClientHandle {
    client: Mutex<Client>,
}

fn reply(value: serde_json::Value) -> *mut c_char {
    // A CString allocation can only fail on an interior NUL, which serde_json
    // never emits. Falling back to a fixed error keeps the ABI total.
    match CString::new(value.to_string()) {
        Ok(s) => s.into_raw(),
        Err(_) => CString::new(r#"{"error":"encoding"}"#)
            .expect("literal has no NUL")
            .into_raw(),
    }
}

fn fail(kind: &str) -> *mut c_char {
    reply(serde_json::json!({ "error": kind }))
}

/// A stable code per error variant.
///
/// `Error` derives its `Display` from `thiserror`, so `to_string()` gives prose
/// meant for a person: "message authentication failed". Sending that across the
/// ABI would make the app branch on a sentence, and any wording change would
/// silently break the branch. These codes are the contract instead.
fn code(error: &rudi_chat_crypto::Error) -> &'static str {
    use rudi_chat_crypto::Error;
    match error {
        Error::Invalid => "invalid",
        Error::Authentication => "authentication",
        Error::Roster => "roster",
        Error::State => "state",
        Error::Conflict => "conflict",
        Error::Capacity => "capacity",
        Error::Mls => "mls",
        Error::Checkpoint => "checkpoint",
    }
}

/// Every entry point funnels through here: a panic must not unwind across the
/// ABI, because unwinding into Java is undefined behaviour rather than a crash
/// anyone can read.
fn guard<F: FnOnce() -> *mut c_char>(body: F) -> *mut c_char {
    match catch_unwind(AssertUnwindSafe(body)) {
        Ok(out) => out,
        Err(_) => fail("panic"),
    }
}

/// # Safety
/// `text` must be a NUL-terminated C string that stays valid for this call.
unsafe fn borrow<'a>(text: *const c_char) -> Option<&'a str> {
    if text.is_null() {
        return None;
    }
    CStr::from_ptr(text).to_str().ok()
}

/// # Safety
/// `handle` must come from `rudi_chat_crypto_client_new` or `_client_resume`
/// and not yet be freed. A poisoned lock (a panic mid-call) refuses further
/// use: the state it guards may be half-changed.
unsafe fn client<'a>(handle: *mut ClientHandle) -> Option<MutexGuard<'a, Client>> {
    handle.as_ref().and_then(|h| h.client.lock().ok())
}

/// Create a client for one account/device pair.
///
/// # Safety
/// `actor_id` and `device_id` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_client_new(
    actor_id: *const c_char,
    device_id: *const c_char,
) -> *mut ClientHandle {
    let made = catch_unwind(AssertUnwindSafe(|| {
        let actor = borrow(actor_id)?;
        let device = borrow(device_id)?;
        Client::new(actor, device).ok()
    }));
    match made {
        Ok(Some(client)) => Box::into_raw(Box::new(ClientHandle {
            client: Mutex::new(client),
        })),
        _ => ptr::null_mut(),
    }
}

/// Reopen a client from its sealed checkpoint. `wrapping_key_b64` is the 32-byte
/// key the platform keeps in its Keystore/Keychain; `anchor_json` is the latest
/// anchor from the device's non-backup store. Null on any mismatch: a stale,
/// tampered or foreign checkpoint never opens.
///
/// # Safety
/// The three arguments must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_client_resume(
    sealed_json: *const c_char,
    wrapping_key_b64: *const c_char,
    anchor_json: *const c_char,
) -> *mut ClientHandle {
    let made = catch_unwind(AssertUnwindSafe(|| {
        let sealed: SealedLocalState = serde_json::from_str(borrow(sealed_json)?).ok()?;
        let key = wrapping_key(borrow(wrapping_key_b64)?)?;
        let anchor: LocalAnchor = serde_json::from_str(borrow(anchor_json)?).ok()?;
        Client::resume_local_state(&sealed, &key, &anchor).ok()
    }));
    match made {
        Ok(Some(client)) => Box::into_raw(Box::new(ClientHandle {
            client: Mutex::new(client),
        })),
        _ => ptr::null_mut(),
    }
}

fn wrapping_key(encoded: &str) -> Option<Zeroizing<[u8; 32]>> {
    let bytes = Zeroizing::new(STANDARD.decode(encoded).ok()?);
    let mut key = Zeroizing::new([0u8; 32]);
    if bytes.len() != 32 {
        return None;
    }
    key.copy_from_slice(&bytes);
    Some(key)
}

/// Release a client. Passing null is allowed and does nothing.
///
/// # Safety
/// `handle` must come from `rudi_chat_crypto_client_new`, and must not be used
/// again afterwards.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_client_free(handle: *mut ClientHandle) {
    if !handle.is_null() {
        drop(Box::from_raw(handle));
    }
}

/// Release a string this library returned. Passing null is allowed.
///
/// # Safety
/// `text` must be a pointer this library returned and not yet freed.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_string_free(text: *mut c_char) {
    if !text.is_null() {
        drop(CString::from_raw(text));
    }
}

/// The device's identity card as JSON. Public keys only.
///
/// # Safety
/// `handle` must be a live client handle.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_identity(handle: *mut ClientHandle) -> *mut c_char {
    guard(|| {
        let Some(client) = client(handle) else {
            return fail("handle");
        };
        let card = client.identity();
        match serde_json::to_value(card) {
            Ok(card) => reply(card),
            Err(_) => fail("encoding"),
        }
    })
}

/// Start a group on this device.
///
/// # Safety
/// `handle` must be live and `conversation_id` a valid C string.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_create_group(
    handle: *mut ClientHandle,
    conversation_id: *const c_char,
) -> *mut c_char {
    guard(|| {
        let (Some(mut client), Some(conversation)) = (client(handle), borrow(conversation_id))
        else {
            return fail("handle");
        };
        match client.create_group(conversation) {
            Ok(()) => reply(serde_json::json!({ "ok": true })),
            Err(e) => fail(code(&e)),
        }
    })
}

/// Encrypt one operation in one conversation. `operation_json` is a
/// `rudi_chat_crypto::Operation`.
///
/// # Safety
/// `handle` must be live; the three strings must be valid C strings.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_encrypt(
    handle: *mut ClientHandle,
    conversation_id: *const c_char,
    logical_send_id: *const c_char,
    operation_json: *const c_char,
) -> *mut c_char {
    guard(|| {
        let (Some(mut client), Some(conversation), Some(logical), Some(raw)) = (
            client(handle),
            borrow(conversation_id),
            borrow(logical_send_id),
            borrow(operation_json),
        ) else {
            return fail("handle");
        };
        let Ok(operation) = serde_json::from_str::<Operation>(raw) else {
            return fail("invalid_operation");
        };
        match client.encrypt(conversation, logical, operation) {
            Ok(envelope) => match serde_json::to_value(envelope) {
                Ok(value) => reply(value),
                Err(_) => fail("encoding"),
            },
            Err(e) => fail(code(&e)),
        }
    })
}

/// Decrypt one envelope.
///
/// `verified_roster_json` is a JSON array of `IdentityCard`, or null. It is not
/// a convenience argument and it is deliberately not defaulted here: the core
/// crate treats a supplied roster as an **authorization assertion by the
/// caller's trusted enrollment layer**, because an MLS BasicCredential does not
/// by itself prove which account a device belongs to.
///
/// Passing null is safe and means "I have verified nothing": an application
/// message still decrypts, and a commit — the message that would change who is
/// in the group — is refused with `{"error":"roster"}` rather than merged. Hiding
/// this argument inside the bridge would turn that refusal into a silent
/// accept, which is the one thing this whole crate exists to prevent.
///
/// # Safety
/// `handle` must be live, `envelope_json` a valid C string, and
/// `verified_roster_json` either null or a valid C string.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_receive(
    handle: *mut ClientHandle,
    envelope_json: *const c_char,
    verified_roster_json: *const c_char,
) -> *mut c_char {
    guard(|| {
        let (Some(mut client), Some(raw)) = (client(handle), borrow(envelope_json)) else {
            return fail("handle");
        };
        let Ok(envelope) = serde_json::from_str::<Envelope>(raw) else {
            return fail("invalid_envelope");
        };
        let roster = match borrow(verified_roster_json) {
            None => None,
            Some(text) => match serde_json::from_str::<Vec<IdentityCard>>(text) {
                Ok(cards) => Some(cards),
                Err(_) => return fail("invalid_roster"),
            },
        };
        match client.receive(&envelope, roster.as_deref()) {
            Ok(received) => reply(describe(received)),
            Err(e) => fail(code(&e)),
        }
    })
}

/// Flatten `Received` into something a Kotlin `when` can branch on without
/// knowing Rust enum encoding.
fn describe(received: rudi_chat_crypto::Received) -> serde_json::Value {
    use rudi_chat_crypto::Received;
    match received {
        Received::Application {
            actor_id,
            device_id,
            logical_send_id,
            operation,
        } => serde_json::json!({
            "kind": "application",
            "actor_id": actor_id,
            "device_id": device_id,
            "logical_send_id": logical_send_id,
            "operation": operation,
        }),
        Received::Commit { epoch } => serde_json::json!({ "kind": "commit", "epoch": epoch }),
        Received::Removed => serde_json::json!({ "kind": "removed" }),
    }
}

/// The rest of the lifecycle (ADR-0057), one method name and one JSON object of
/// arguments per call:
///
/// | method | args | answer |
/// |---|---|---|
/// | `generation` | `{}` | `{"generation"}` |
/// | `enrollment` | `{}` | `{"card", "proof":b64}` for POST /v2/chat/devices |
/// | `conversations` | `{}` | `{"conversations":[id]}` |
/// | `key_package` | `{}` | `{"key_package":b64}` |
/// | `join_group` | `{conversation_id, welcome:b64, roster:[card]}` | `{"ok":true}` |
/// | `epoch` / `roster` | `{conversation_id}` | `{"epoch"}` / `{"roster":[card]}` |
/// | `stage_add` | `{conversation_id, logical_send_id, members:[{card, key_package:b64}]}` | commit |
/// | `stage_remove` | `{conversation_id, logical_send_id, device_id}` | commit |
/// | `stage_rekey` / `pending_commit` | `{conversation_id[, logical_send_id]}` | commit |
/// | `acknowledge_commit` / `acknowledge_sent` / `abandon_send` | `{envelope}` | `{"ok":true}` |
/// | `abandon_commit` / `forget` | `{conversation_id}` | `{"ok":true}` |
/// | `seal` | `{wrapping_key:b64}` | `{"sealed", "anchor"}` |
/// | `seal_media` | `{media_id, mime, plaintext:b64}` | `{"ciphertext":b64, "media"}` |
/// | `open_media` | `{media, ciphertext:b64}` | `{"plaintext":b64}` |
///
/// A commit answer is `{"envelope", "welcome": b64|null}`. Errors are
/// `{"error": code}` with the same codes as every other entry point, plus
/// `unknown_method` and `invalid_arguments`.
///
/// # Safety
/// `handle` must be live; `method` and `args_json` valid C strings.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_call(
    handle: *mut ClientHandle,
    method: *const c_char,
    args_json: *const c_char,
) -> *mut c_char {
    guard(|| {
        let (Some(mut client), Some(method), Some(raw)) =
            (client(handle), borrow(method), borrow(args_json))
        else {
            return fail("handle");
        };
        match call(&mut client, method, raw) {
            Ok(value) => reply(value),
            Err(kind) => fail(kind),
        }
    })
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Args {
    conversation_id: Option<String>,
    logical_send_id: Option<String>,
    device_id: Option<String>,
    welcome: Option<String>,
    roster: Option<Vec<IdentityCard>>,
    members: Option<Vec<Member>>,
    envelope: Option<Envelope>,
    wrapping_key: Option<String>,
    media_id: Option<String>,
    mime: Option<String>,
    plaintext: Option<String>,
    card: Option<String>,
    media: Option<MediaRef>,
    ciphertext: Option<String>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Member {
    card: IdentityCard,
    key_package: String,
}

type Answer = Result<serde_json::Value, &'static str>;

fn need<T>(value: Option<T>) -> Result<T, &'static str> {
    value.ok_or("invalid_arguments")
}

fn bytes(encoded: Option<String>) -> Result<Vec<u8>, &'static str> {
    STANDARD
        .decode(need(encoded)?)
        .map_err(|_| "invalid_arguments")
}

fn ok() -> Answer {
    Ok(serde_json::json!({ "ok": true }))
}

fn commit(bundle: rudi_chat_crypto::CommitBundle) -> Answer {
    Ok(serde_json::json!({
        "envelope": bundle.envelope,
        "welcome": bundle.welcome.map(|w| STANDARD.encode(w)),
    }))
}

fn call(client: &mut Client, method: &str, raw: &str) -> Answer {
    let args: Args = serde_json::from_str(raw).map_err(|_| "invalid_arguments")?;
    let core = |e: rudi_chat_crypto::Error| code(&e);
    match method {
        "generation" => Ok(serde_json::json!({ "generation": client.generation() })),
        "enrollment" => Ok(serde_json::json!({
            "card": client.identity(),
            "proof": STANDARD.encode(client.enrollment_proof()),
        })),
        "conversations" => Ok(serde_json::json!({ "conversations": client.conversations() })),
        "key_package" => Ok(serde_json::json!({
            "key_package": STANDARD.encode(client.key_package().map_err(core)?),
        })),
        "join_group" => {
            let welcome = bytes(args.welcome)?;
            client
                .join_group(&need(args.conversation_id)?, &welcome, &need(args.roster)?)
                .map_err(core)?;
            ok()
        }
        "epoch" => Ok(serde_json::json!({
            "epoch": client.epoch(&need(args.conversation_id)?).map_err(core)?,
        })),
        "roster" => Ok(serde_json::json!({
            "roster": client.roster(&need(args.conversation_id)?),
        })),
        "stage_add" => {
            let mut members = Vec::new();
            for m in need(args.members)? {
                members.push((m.card, bytes(Some(m.key_package))?));
            }
            commit(
                client
                    .stage_add(
                        &need(args.conversation_id)?,
                        &need(args.logical_send_id)?,
                        &members,
                    )
                    .map_err(core)?,
            )
        }
        "stage_remove" => commit(
            client
                .stage_remove(
                    &need(args.conversation_id)?,
                    &need(args.logical_send_id)?,
                    &need(args.device_id)?,
                )
                .map_err(core)?,
        ),
        "stage_rekey" => commit(
            client
                .stage_rekey(&need(args.conversation_id)?, &need(args.logical_send_id)?)
                .map_err(core)?,
        ),
        "pending_commit" => commit(
            client
                .pending_commit(&need(args.conversation_id)?)
                .map_err(core)?,
        ),
        "acknowledge_commit" => {
            client
                .acknowledge_commit(&need(args.envelope)?)
                .map_err(core)?;
            ok()
        }
        "abandon_send" => {
            client.abandon_send(&need(args.envelope)?).map_err(core)?;
            ok()
        }
        "acknowledge_sent" => {
            client
                .acknowledge_sent(&need(args.envelope)?)
                .map_err(core)?;
            ok()
        }
        "abandon_commit" => {
            client
                .abandon_commit(&need(args.conversation_id)?)
                .map_err(core)?;
            ok()
        }
        "forget" => {
            client.forget(&need(args.conversation_id)?).map_err(core)?;
            ok()
        }
        "ai_card_digest" => {
            let card = need(args.card)?;
            let digest = rudi_chat_crypto::ai_card_digest(&card);
            let hex: String = digest.iter().map(|b| format!("{b:02x}")).collect();
            Ok(serde_json::json!({ "digest": hex }))
        }
        "settle_received" => {
            client
                .settle_received(&need(args.conversation_id)?)
                .map_err(core)?;
            ok()
        }
        "seal" => {
            let key = wrapping_key(&need(args.wrapping_key)?).ok_or("invalid_arguments")?;
            let sealed = client.seal_local_state(&key).map_err(core)?;
            let anchor = sealed.anchor().map_err(core)?;
            Ok(serde_json::json!({ "sealed": sealed, "anchor": anchor }))
        }
        "seal_media" => {
            let plaintext = Zeroizing::new(bytes(args.plaintext)?);
            let (ciphertext, media) =
                rudi_chat_crypto::seal_media(&need(args.media_id)?, &need(args.mime)?, &plaintext)
                    .map_err(core)?;
            Ok(serde_json::json!({ "ciphertext": STANDARD.encode(ciphertext), "media": media }))
        }
        "open_media" => {
            let plaintext =
                rudi_chat_crypto::open_media(&need(args.media)?, &bytes(args.ciphertext)?)
                    .map_err(core)?;
            Ok(serde_json::json!({ "plaintext": STANDARD.encode(plaintext.as_slice()) }))
        }
        _ => Err("unknown_method"),
    }
}
