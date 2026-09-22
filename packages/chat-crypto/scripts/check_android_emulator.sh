#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/../../.." && pwd)"
ndk_dir="${ANDROID_NDK_ROOT:?Set ANDROID_NDK_ROOT to an installed Android NDK}"
adb_bin="${ADB_BIN:-adb}"
adb_port="${CHAT_ADB_PORT:?Set the authorized ADB server port}"
adb_serial="${CHAT_ADB_SERIAL:?Set the authorized emulator serial}"
toolchain="$ndk_dir/toolchains/llvm/prebuilt/linux-x86_64/bin"
export CARGO_BUILD_JOBS=2
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:?Set CARGO_TARGET_DIR outside every checkout}"
case "$(realpath -m "$CARGO_TARGET_DIR")" in
  "$repo_dir"|"$repo_dir"/*) echo 'CARGO_TARGET_DIR must be outside the checkout' >&2; exit 2 ;;
esac
abi="$("$adb_bin" -P "$adb_port" -s "$adb_serial" shell getprop ro.product.cpu.abi | tr -d '\r')"
test "$abi" = x86_64
export CARGO_TARGET_X86_64_LINUX_ANDROID_LINKER="$toolchain/x86_64-linux-android26-clang"
export CC_x86_64_linux_android="$CARGO_TARGET_X86_64_LINUX_ANDROID_LINKER"
export AR_x86_64_linux_android="$toolchain/llvm-ar"
nice -n 19 "${CARGO_BIN:-cargo}" "${RUST_TOOLCHAIN_ARG:-+1.98.1}" test --locked --release --no-run \
  --manifest-path "$repo_dir/packages/chat-crypto/Cargo.toml" --target x86_64-linux-android

# Own only this fresh directory; never install an APK, alter UI, or restart ADB.
remote_dir="$("$adb_bin" -P "$adb_port" -s "$adb_serial" shell mktemp -d /data/local/tmp/rudi-chat-crypto.XXXXXX | tr -d '\r')"
if [[ ! "$remote_dir" =~ ^/data/local/tmp/rudi-chat-crypto\.[[:alnum:]]{6}$ ]]; then
  echo 'Unexpected temporary directory from Android' >&2
  exit 2
fi
trap '"$adb_bin" -P "$adb_port" -s "$adb_serial" shell rm -rf "$remote_dir" >/dev/null' EXIT
count=0
for binary in "$CARGO_TARGET_DIR"/x86_64-linux-android/release/deps/{rudi_chat_crypto,mls_canaries}-*; do
  test -x "$binary" || continue
  name="$(basename "$binary")"
  [[ "$name" =~ ^(rudi_chat_crypto|mls_canaries)-[[:xdigit:]]+$ ]] || continue
  "$adb_bin" -P "$adb_port" -s "$adb_serial" push "$binary" "$remote_dir/$name"
  "$adb_bin" -P "$adb_port" -s "$adb_serial" shell chmod 700 "$remote_dir/$name"
  "$adb_bin" -P "$adb_port" -s "$adb_serial" shell "$remote_dir/$name" --test-threads=1
  count=$((count + 1))
done
test "$count" -ge 2
