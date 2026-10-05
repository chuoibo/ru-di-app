//! The C ABI end to end, as the native module will drive it: two devices,
//! a group, a message, a commit, a checkpoint reopened through the ABI, and
//! the refusals. Synthetic data only.
use std::ffi::{c_char, CStr, CString};

use rudi_chat_crypto_ffi::*;

fn id(value: u64) -> String {
    format!("aaaaaaaa-bbbb-4ccc-8ddd-{value:012x}")
}

fn c(text: &str) -> CString {
    CString::new(text).unwrap()
}

/// Takes ownership of a returned string and parses it.
fn take(out: *mut c_char) -> serde_json::Value {
    assert!(!out.is_null());
    let text = unsafe { CStr::from_ptr(out) }.to_str().unwrap().to_owned();
    unsafe { rudi_chat_crypto_string_free(out) };
    serde_json::from_str(&text).unwrap()
}

fn call(handle: *mut ClientHandle, method: &str, args: serde_json::Value) -> serde_json::Value {
    let (m, a) = (c(method), c(&args.to_string()));
    take(unsafe { rudi_chat_crypto_call(handle, m.as_ptr(), a.as_ptr()) })
}

fn new(actor: u64, device: u64) -> *mut ClientHandle {
    let (a, d) = (c(&id(actor)), c(&id(device)));
    let handle = unsafe { rudi_chat_crypto_client_new(a.as_ptr(), d.as_ptr()) };
    assert!(!handle.is_null());
    handle
}

#[test]
fn the_whole_lifecycle_crosses_the_abi() {
    let alice = new(1, 11);
    let bob = new(2, 22);
    let room = id(100);
    let alice_card = take(unsafe { rudi_chat_crypto_identity(alice) });
    let bob_card = take(unsafe { rudi_chat_crypto_identity(bob) });
    let r = c(&room);
    assert_eq!(
        take(unsafe { rudi_chat_crypto_create_group(alice, r.as_ptr()) }),
        serde_json::json!({"ok": true})
    );
    let kp = call(bob, "key_package", serde_json::json!({}))["key_package"].clone();
    let add = call(
        alice,
        "stage_add",
        serde_json::json!({"conversation_id": room, "logical_send_id": id(200),
            "members": [{"card": bob_card, "key_package": kp}]}),
    );
    assert!(add["welcome"].is_string(), "{add}");
    let roster = serde_json::json!([alice_card, bob_card]);
    assert_eq!(
        call(
            bob,
            "join_group",
            serde_json::json!({"conversation_id": room, "welcome": add["welcome"], "roster": roster})
        ),
        serde_json::json!({"ok": true})
    );
    assert_eq!(
        call(
            alice,
            "acknowledge_commit",
            serde_json::json!({"envelope": add["envelope"]})
        ),
        serde_json::json!({"ok": true})
    );
    assert_eq!(
        call(alice, "epoch", serde_json::json!({"conversation_id": room}))["epoch"],
        2
    );

    let (logical, op) = (c(&id(300)), c(r#"{"type":"text","body":"qua ABI"}"#));
    let envelope =
        take(unsafe { rudi_chat_crypto_encrypt(alice, r.as_ptr(), logical.as_ptr(), op.as_ptr()) });
    let env = c(&envelope.to_string());
    let got = take(unsafe { rudi_chat_crypto_receive(bob, env.as_ptr(), std::ptr::null()) });
    assert_eq!(got["kind"], "application");
    assert_eq!(got["operation"]["body"], "qua ABI");
    assert_eq!(
        call(
            alice,
            "acknowledge_sent",
            serde_json::json!({"envelope": envelope})
        ),
        serde_json::json!({"ok": true})
    );

    // A checkpoint sealed through the ABI reopens through it, and nowhere else.
    let key = "CQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQk=";
    let sealed = call(bob, "seal", serde_json::json!({"wrapping_key": key}));
    let (s, k, a) = (
        c(&sealed["sealed"].to_string()),
        c(key),
        c(&sealed["anchor"].to_string()),
    );
    let reopened = unsafe { rudi_chat_crypto_client_resume(s.as_ptr(), k.as_ptr(), a.as_ptr()) };
    assert!(!reopened.is_null());
    let wrong = c("BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc=");
    assert!(
        unsafe { rudi_chat_crypto_client_resume(s.as_ptr(), wrong.as_ptr(), a.as_ptr()) }.is_null()
    );
    assert_eq!(
        call(reopened, "conversations", serde_json::json!({}))["conversations"],
        serde_json::json!([room])
    );

    // Media sealed and opened through the ABI.
    let sealed_media = call(
        alice,
        "seal_media",
        serde_json::json!({"media_id": id(700), "mime": "image/png", "plaintext": "c3ludGhldGlj"}),
    );
    let opened = call(
        bob,
        "open_media",
        serde_json::json!({"media": sealed_media["media"], "ciphertext": sealed_media["ciphertext"]}),
    );
    assert_eq!(opened["plaintext"], "c3ludGhldGlj");

    // Refusals are stable codes, never prose and never a crash.
    assert_eq!(
        call(alice, "nope", serde_json::json!({}))["error"],
        "unknown_method"
    );
    assert_eq!(
        call(alice, "epoch", serde_json::json!({"surprise": 1}))["error"],
        "invalid_arguments"
    );
    assert_eq!(
        call(
            alice,
            "epoch",
            serde_json::json!({"conversation_id": id(999)})
        )["error"],
        "state"
    );
    assert_eq!(
        take(unsafe {
            rudi_chat_crypto_call(std::ptr::null_mut(), c("epoch").as_ptr(), c("{}").as_ptr())
        })["error"],
        "handle"
    );
    for h in [alice, bob, reopened] {
        unsafe { rudi_chat_crypto_client_free(h) };
    }
}
