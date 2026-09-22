#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/../../.." && pwd)"
task_dir="$(mktemp -d "${TMPDIR:-/tmp}/rudi-mls-interop.XXXXXX")"
trap 'rm -rf "$task_dir"' EXIT
export CARGO_BUILD_JOBS=2
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-$task_dir/target}"
case "$(realpath -m "$CARGO_TARGET_DIR")" in
  "$repo_dir"|"$repo_dir"/*) echo 'CARGO_TARGET_DIR must be outside the checkout' >&2; exit 2 ;;
esac

nice -n 19 "${CARGO_BIN:-cargo}" "${RUST_TOOLCHAIN_ARG:-+1.98.1}" run --quiet --locked \
  --manifest-path "$repo_dir/packages/chat-crypto/Cargo.toml" --example emit_interop > "$task_dir/vector.json"

# Compile the actual checked-out Go wire implementation with a standalone driver.
# Only the package name changes; no Go source inside the checkout is modified.
sed 's/^package chatv2$/package main/' "$repo_dir/services/core/internal/chatv2/types.go" > "$task_dir/types.go"
cat > "$task_dir/main.go" <<'GO'
package main

import (
    "bytes"
    "crypto/ed25519"
    "encoding/json"
    "fmt"
    "os"
)

func main() {
    raw, err := os.ReadFile(os.Args[1])
    if err != nil { panic(err) }
    var vector struct {
        PublicKey []byte `json:"public_key"`
        SigningBytes []byte `json:"signing_bytes"`
        Envelope Envelope `json:"envelope"`
    }
    if err = json.Unmarshal(raw, &vector); err != nil { panic(err) }
    preimage, err := SigningBytes(vector.Envelope)
    if err != nil { panic(err) }
    if !bytes.Equal(preimage, vector.SigningBytes) { panic("Rust/Go preimage differs") }
    if !ed25519.Verify(vector.PublicKey, preimage, vector.Envelope.Signature) { panic("Go rejected Rust signature") }
    vector.Envelope.Epoch++
    changed, err := SigningBytes(vector.Envelope)
    if err != nil { panic(err) }
    if ed25519.Verify(vector.PublicKey, changed, vector.Envelope.Signature) { panic("epoch mutation accepted") }
    fmt.Println("PASS: real MLS ciphertext, checked-out Go SigningBytes, Ed25519 verification, epoch tamper rejection")
}
GO
nice -n 19 go run "$task_dir/types.go" "$task_dir/main.go" "$task_dir/vector.json"
