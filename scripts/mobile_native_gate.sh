#!/usr/bin/env bash
# Build this tree, then provision isolated HTTP-created accounts for Android.
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
(cd apps/mobile/android && ./gradlew --no-daemon :app:assembleDebug)
exec scripts/e2e_slice.sh --native
