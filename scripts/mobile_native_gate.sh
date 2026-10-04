#!/usr/bin/env bash
# Build this tree, then drive the product table on isolated HTTP-created accounts.
set -euo pipefail
cd "$(dirname "$0")/.."
serial="${ANDROID_SERIAL:-emulator-5612}"
for tool in adb maestro node npm java docker go python3; do
  command -v "$tool" >/dev/null || { echo "KHÔNG ĐO ĐƯỢC: thiếu $tool" >&2; exit 2; }
done
[ "$(adb -s "$serial" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = 1 ] || {
  echo 'KHÔNG ĐO ĐƯỢC: máy ảo Android được chỉ định chưa boot.' >&2; exit 2;
}
export ANDROID_SERIAL="$serial"
export EXPO_NO_TELEMETRY=1
# Rebuild the native module as well as the JS bundle; a previously built APK
# is not proof of this source tree's Kotlin Credential Manager implementation.
(cd apps/mobile && npx --no-install expo prebuild --platform android --no-install)
# One ABI: the gate drives an emulator, and four ABIs quadruple the C++ build.
(cd apps/mobile/android && ./gradlew --no-daemon :app:assembleDebug -PreactNativeArchitectures="${MOBILE_NATIVE_ABI:-x86_64}")
# The harness refuses a dev client whose package.json/app.json drifted from
# the build; record what this build was made from, then hand it the APK.
(cd apps/mobile && git hash-object package.json app.json | tr '\n' ' ' > android/.rudi-native-fingerprint)
export RUDI_NATIVE_APK="$PWD/apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk"
# The whole product table (apps/mobile/.maestro, flows 00-50) on HTTP-created
# synthetic accounts of a disposable stack, plus its canaries.
exec scripts/e2e_slice.sh --native
