//! Protocol drill: a line-oriented wrapper over the C ABI, so a test on the
//! other side of a pipe (services/core/internal/chatv2http, tag `drill`) can
//! play several devices against the real Go server through exactly the entry
//! points the app will call. One JSON request per line in, one JSON answer per
//! line out. Synthetic data only; nothing here is shipped.
//!
//! Request: {"client": name, "fn": f, ...}. `fn` is one of
//!   new {actor, device} | identity | create_group {conversation_id}
//!   | encrypt {conversation_id, logical_send_id, operation}
//!   | receive {envelope, roster|null} | call {method, args}
use std::collections::HashMap;
use std::ffi::{c_char, CStr, CString};
use std::io::{self, BufRead, Write};

use rudi_chat_crypto_ffi::*;

fn take(out: *mut c_char) -> serde_json::Value {
    if out.is_null() {
        return serde_json::json!({"error": "null"});
    }
    let text = unsafe { CStr::from_ptr(out) }
        .to_string_lossy()
        .into_owned();
    unsafe { rudi_chat_crypto_string_free(out) };
    serde_json::from_str(&text).unwrap_or(serde_json::json!({"error": "decode"}))
}

fn c(value: &serde_json::Value) -> CString {
    let text = match value {
        serde_json::Value::String(s) => s.clone(),
        other => other.to_string(),
    };
    CString::new(text).unwrap()
}

fn main() {
    let mut clients: HashMap<String, *mut ClientHandle> = HashMap::new();
    let stdin = io::stdin();
    let mut stdout = io::stdout().lock();
    for line in stdin.lock().lines() {
        let Ok(line) = line else { break };
        let request: serde_json::Value = match serde_json::from_str(&line) {
            Ok(v) => v,
            Err(_) => {
                writeln!(stdout, r#"{{"error":"request"}}"#).unwrap();
                continue;
            }
        };
        let name = request["client"].as_str().unwrap_or_default().to_owned();
        let handle = clients.get(&name).copied().unwrap_or(std::ptr::null_mut());
        let answer = match request["fn"].as_str().unwrap_or_default() {
            "new" => {
                let (a, d) = (c(&request["actor"]), c(&request["device"]));
                let made = unsafe { rudi_chat_crypto_client_new(a.as_ptr(), d.as_ptr()) };
                if made.is_null() {
                    serde_json::json!({"error": "new"})
                } else {
                    clients.insert(name, made);
                    serde_json::json!({"ok": true})
                }
            }
            "identity" => take(unsafe { rudi_chat_crypto_identity(handle) }),
            "create_group" => {
                let conv = c(&request["conversation_id"]);
                take(unsafe { rudi_chat_crypto_create_group(handle, conv.as_ptr()) })
            }
            "encrypt" => {
                let (conv, logical, op) = (
                    c(&request["conversation_id"]),
                    c(&request["logical_send_id"]),
                    c(&request["operation"]),
                );
                take(unsafe {
                    rudi_chat_crypto_encrypt(handle, conv.as_ptr(), logical.as_ptr(), op.as_ptr())
                })
            }
            "receive" => {
                let envelope = c(&request["envelope"]);
                if request["roster"].is_null() {
                    take(unsafe {
                        rudi_chat_crypto_receive(handle, envelope.as_ptr(), std::ptr::null())
                    })
                } else {
                    let roster = c(&request["roster"]);
                    take(unsafe {
                        rudi_chat_crypto_receive(handle, envelope.as_ptr(), roster.as_ptr())
                    })
                }
            }
            "call" => {
                let (method, args) = (c(&request["method"]), c(&request["args"]));
                take(unsafe { rudi_chat_crypto_call(handle, method.as_ptr(), args.as_ptr()) })
            }
            _ => serde_json::json!({"error": "fn"}),
        };
        writeln!(stdout, "{answer}").unwrap();
        stdout.flush().unwrap();
    }
    for (_, handle) in clients {
        unsafe { rudi_chat_crypto_client_free(handle) };
    }
}
