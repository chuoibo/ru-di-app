#!/usr/bin/env bash
# Builds packages/chat-crypto-ffi for the Android ABIs the app ships
# (arm64-v8a, x86_64, armeabi-v7a) into android/src/main/jniLibs, with the
# pinned Rust toolchain and the machine's Android NDK. With cargo on the host it
# builds there; otherwise in the pinned rust image with the NDK mounted. The
# libraries are build output, never committed (see ../.gitignore).
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(cd "$here/../../../.." && pwd)"
ndk="${ANDROID_NDK_ROOT:-${ANDROID_NDK_HOME:-}}"
[ -z "$ndk" ] && ndk="$(ls -d "$HOME"/Android/Sdk/ndk/* 2>/dev/null | sort -V | tail -1)"
[ -n "$ndk" ] && [ -d "$ndk" ] || { echo "không tìm thấy Android NDK (ANDROID_NDK_ROOT)" >&2; exit 1; }
targets="aarch64-linux-android:arm64-v8a x86_64-linux-android:x86_64 armv7-linux-androideabi:armeabi-v7a"
build() {
  local bin="$1/toolchains/llvm/prebuilt/linux-x86_64/bin" pair target abi clang upper
  rustup target add aarch64-linux-android x86_64-linux-android armv7-linux-androideabi >/dev/null
  for pair in $targets; do
    target="${pair%%:*}"
    clang="$bin/${target}26-clang"
    [ "$target" = armv7-linux-androideabi ] && clang="$bin/armv7a-linux-androideabi26-clang"
    upper="$(echo "$target" | tr 'a-z-' 'A-Z_')"
    env "CARGO_TARGET_${upper}_LINKER=$clang" "CC_${target//-/_}=$clang" "AR_${target//-/_}=$bin/llvm-ar" \
      cargo build --locked --release --manifest-path "$2/packages/chat-crypto-ffi/Cargo.toml" --target "$target"
  done
}
if command -v cargo >/dev/null 2>&1; then
  build "$ndk" "$repo"
  out="$(cargo metadata --manifest-path "$repo/packages/chat-crypto-ffi/Cargo.toml" --format-version 1 --no-deps | python3 -c 'import json,sys;print(json.load(sys.stdin)["target_directory"])')"
else
  cache="$HOME/.cache/rudi-cargo"
  mkdir -p "$cache/android-registry" "$cache/android-target"
  docker run -i --rm -e CARGO_TARGET_DIR=/target -e targets="$targets" \
    -v "$cache/android-registry:/usr/local/cargo/registry" -v "$cache/android-target:/target" \
    -v "$repo/packages":/repo/packages:ro -v "$ndk":"$ndk":ro -v "$here/scripts":/scripts:ro \
    rust:1.98.1-bookworm bash -c "set -e; source /dev/stdin" <<INNER
$(declare -f build)
targets="$targets"
build "$ndk" /repo
chown -R $(id -u):$(id -g) /target
INNER
  out="$cache/android-target"
fi
for pair in $targets; do
  target="${pair%%:*}"; abi="${pair##*:}"
  mkdir -p "$here/android/src/main/jniLibs/$abi"
  cp "$out/$target/release/librudi_chat_crypto_ffi.so" "$here/android/src/main/jniLibs/$abi/"
done
ls -la "$here"/android/src/main/jniLibs/*/
