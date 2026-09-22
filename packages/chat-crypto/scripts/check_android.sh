#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/../../.." && pwd)"
ndk_dir="${ANDROID_NDK_ROOT:?Set ANDROID_NDK_ROOT to an installed Android NDK}"
toolchain="$ndk_dir/toolchains/llvm/prebuilt/linux-x86_64/bin"
export CARGO_BUILD_JOBS=2
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:?Set CARGO_TARGET_DIR outside every checkout}"
case "$(realpath -m "$CARGO_TARGET_DIR")" in
  "$repo_dir"|"$repo_dir"/*) echo 'CARGO_TARGET_DIR must be outside the checkout' >&2; exit 2 ;;
esac
export CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER="$toolchain/aarch64-linux-android26-clang"
export CC_aarch64_linux_android="$CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER"
export AR_aarch64_linux_android="$toolchain/llvm-ar"
test -x "$CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER"
nice -n 19 "${CARGO_BIN:-cargo}" "${RUST_TOOLCHAIN_ARG:-+1.98.1}" build --locked --release \
  --manifest-path "$repo_dir/packages/chat-crypto/Cargo.toml" --target aarch64-linux-android \
  --lib --example emit_interop
"$toolchain/llvm-readelf" --file-header "$CARGO_TARGET_DIR/aarch64-linux-android/release/examples/emit_interop"
