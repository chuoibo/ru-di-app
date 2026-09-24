//! C ABI over `rudi-chat-crypto`, for calling MLS from the Android app.
//!
//! Shape: JSON in, JSON out, one owned C string back. Keeping the boundary at
//! JSON means the ABI has four functions instead of twenty, and every argument
//! that crosses it is already validated by the core crate rather than by
//! hand-written marshalling.
//!
//! What never crosses: a private key, a plaintext export, or a panic. The core
//! crate exposes no key accessor, every entry point here catches unwinding, and
//! errors come back as a JSON object with a stable `error` string so the app
//! can branch on it without parsing prose.

use std::ffi::{c_char, CStr, CString};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::ptr;

use rudi_chat_crypto::{Client, Envelope, IdentityCard, Operation};

/// Opaque handle. The app holds the pointer and gives it back; it never reads
/// through it, and the layout is deliberately not part of the ABI.
pub struct ClientHandle {
    client: Client,
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
/// `handle` must come from `rudi_chat_crypto_client_new` and not yet be freed.
unsafe fn client<'a>(handle: *mut ClientHandle) -> Option<&'a mut Client> {
    handle.as_mut().map(|h| &mut h.client)
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
        Ok(Some(client)) => Box::into_raw(Box::new(ClientHandle { client })),
        _ => ptr::null_mut(),
    }
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
        match serde_json::to_value(client.identity()) {
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
        let (Some(client), Some(conversation)) = (client(handle), borrow(conversation_id)) else {
            return fail("handle");
        };
        match client.create_group(conversation) {
            Ok(()) => reply(serde_json::json!({ "ok": true })),
            Err(e) => fail(code(&e)),
        }
    })
}

/// Encrypt one operation. `operation_json` is a `rudi_chat_crypto::Operation`.
///
/// # Safety
/// `handle` must be live; the two strings must be valid C strings.
#[no_mangle]
pub unsafe extern "C" fn rudi_chat_crypto_encrypt(
    handle: *mut ClientHandle,
    logical_send_id: *const c_char,
    operation_json: *const c_char,
) -> *mut c_char {
    guard(|| {
        let (Some(client), Some(logical), Some(raw)) = (
            client(handle),
            borrow(logical_send_id),
            borrow(operation_json),
        ) else {
            return fail("handle");
        };
        let Ok(operation) = serde_json::from_str::<Operation>(raw) else {
            return fail("invalid_operation");
        };
        match client.encrypt(logical, operation) {
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
        let (Some(client), Some(raw)) = (client(handle), borrow(envelope_json)) else {
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
