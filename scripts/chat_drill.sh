#!/usr/bin/env bash
# The ADR-0057 protocol drill: real MLS devices (the Rust crate through its C
# ABI, packages/chat-crypto-ffi/examples/drill.rs) against the real Go chat v2
# lane on a disposable PostgreSQL. Builds the drill with the pinned toolchain
# (on the host when cargo is there, else in the pinned image), then runs
# services/core/internal/chatv2http/drill_postgres_test.go on the Postgres
# tier with the `drill` tag. A skip is a failure, as on the tier.
set -euo pipefail
cd "$(dirname "$0")/.."
if command -v cargo >/dev/null 2>&1; then
  cargo build --locked --release --manifest-path packages/chat-crypto-ffi/Cargo.toml --example drill
  bin="$(cargo metadata --manifest-path packages/chat-crypto-ffi/Cargo.toml --format-version 1 --no-deps | python3 -c 'import json,sys;print(json.load(sys.stdin)["target_directory"])')/release/examples/drill"
else
  cache="$HOME/.cache/rudi-cargo"
  mkdir -p "$cache/drill-registry" "$cache/drill-target"
  docker run --rm --user "$(id -u):$(id -g)" -e CARGO_HOME=/cargo -e CARGO_TARGET_DIR=/target \
    -v "$cache/drill-registry:/cargo" -v "$cache/drill-target:/target" -v "$PWD/packages":/repo/packages:ro -w /repo \
    rust:1.98.1-bookworm cargo build --locked --release --manifest-path packages/chat-crypto-ffi/Cargo.toml --example drill
  bin="$cache/drill-target/release/examples/drill"
fi
[ -x "$bin" ] || { echo "không dựng được binary diễn tập" >&2; exit 1; }
CHAT_DRILL_BIN="$bin" GO_EXTRA_TAGS=drill scripts/go_postgres_tier.sh ./internal/chatv2http/ ./internal/db/ "$@"
# The app's engine (src/rudi/chat/e2ee) driving the same real MLS devices.
(
  cd apps/mobile
  npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
  CHAT_DRILL_BIN="$bin" node --test tests/drill/*.drill.mjs
)
