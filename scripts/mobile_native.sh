#!/usr/bin/env bash
# Native auth acceptance uses a disposable account stack and its synthetic
# credentials. Personal credentials never go into argv, artifacts or this file.
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
port="${MOBILE_METRO_PORT:-8097}"
api="${RUDI_NATIVE_QA_PORT:-}"
serial="${ANDROID_SERIAL:-emulator-5612}"
keep=0
lap=1
while [ $# -gt 0 ]; do
 case "$1" in
  --account) shift ;;
  --port) port="$2";shift 2 ;;
  --api-port) api="$2";shift 2 ;;
  --serial) serial="$2";shift 2 ;;
  --keep) keep=1;shift ;;
  --lap) lap="$2";shift 2 ;;
  --otp|--otp-phone|--live|--dang-nhap) echo 'Cửa điện thoại/lời mời đã gỡ (ADR-0055). Dùng --account trên stack QA cô lập.' >&2;exit 64 ;;
  -h|--help) echo 'scripts/mobile_native.sh --account --api-port <stack QA cô lập> [--serial emulator-5612] [--port 8097]';exit 0 ;;
  *) echo 'Tham số không được hỗ trợ.' >&2;exit 64 ;;
 esac
done
[ "${RUDI_NATIVE_TEST_ACK:-}" = synthetic-only ] && [ -n "$api" ] || {
 echo 'KHÔNG ĐO ĐƯỢC: cần stack QA cô lập, RUDI_NATIVE_TEST_ACK=synthetic-only và --api-port. Không dùng credentials thật trong harness.' >&2;exit 2; }
for tool in adb maestro node npm setsid;do command -v "$tool" >/dev/null || { echo "KHÔNG ĐO ĐƯỢC: thiếu $tool" >&2;exit 2; };done
[[ "$port" =~ ^[0-9]+$ && "$api" =~ ^[0-9]+$ ]] && ((port>0 && port<65536 && api>0 && api<65536 && port!=api)) || { echo 'Cổng QA không hợp lệ.' >&2;exit 2; }
if ! node -e 'const s=require("net").createServer();s.on("error",()=>process.exit(1));s.listen(Number(process.argv[1]),"127.0.0.1",()=>s.close())' "$port";then
 echo 'KHÔNG ĐO ĐƯỢC: cổng Metro đang có process khác; không được dùng lại một bundle không thuộc lượt này.' >&2;exit 2
fi
[ -n "${RUDI_TEST_USERNAME:-}" ] && [ -n "${RUDI_TEST_PASSWORD:-}" ] || { echo 'KHÔNG ĐO ĐƯỢC: thiếu tài khoản tổng hợp được tạo qua HTTP.' >&2;exit 2; }
[[ "$RUDI_TEST_USERNAME" == fixture_* ]] || { echo 'Chỉ nhận namespace fixture của QA cô lập.' >&2;exit 2; }
[ "$(adb -s "$serial" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = 1 ] || { echo 'KHÔNG ĐO ĐƯỢC: emulator chưa boot.' >&2;exit 2; }
apk="$root/apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk"
[ -f "$apk" ] || { echo 'KHÔNG ĐO ĐƯỢC: cần prebuild và assembleDebug trước.' >&2;exit 2; }
artifacts="$(mktemp -d "${TMPDIR:-/tmp}/rudi-native-account.XXXXXX")"
metro_pid=""
tree_digest="$(git diff --binary HEAD | sha256sum | cut -c1-12)"
fingerprint="account-$tree_digest-$$-$(date +%s)"
cleanup() {
 if [ "$keep" = 0 ] && [ -n "$metro_pid" ];then kill -- "-$metro_pid" 2>/dev/null || true;fi
}
trap cleanup EXIT INT TERM
adb -s "$serial" install -r "$apk" >"$artifacts/install.log" 2>&1
adb -s "$serial" reverse "tcp:$port" "tcp:$port"
adb -s "$serial" reverse "tcp:$api" "tcp:$api"
(cd apps/mobile && exec setsid env CI=1 EXPO_PUBLIC_TREE_FINGERPRINT="$fingerprint" EXPO_PUBLIC_API_URL="http://127.0.0.1:$api" npx --no-install expo start --dev-client --localhost --port "$port") >"$artifacts/metro.log" 2>&1 &
metro_pid=$!
for _ in $(seq 1 60);do curl -fsS "http://127.0.0.1:$port/status" >/dev/null 2>&1 && break;sleep 1;done
kill -0 "$metro_pid" 2>/dev/null && curl -fsS "http://127.0.0.1:$port/status" >/dev/null || { echo 'Metro của lượt này không lên.' >&2;exit 1; }
grep -Fq "Starting project at $root/apps/mobile" "$artifacts/metro.log" || { echo 'Metro không xác nhận đúng cây nguồn.' >&2;exit 1; }
printf 'head=%s\ndiff=%s\nfingerprint=%s\nserial=%s\napi_port=%s\nmetro_port=%s\nmetro_process_group=%s\napk_sha256=%s\n' "$(git rev-parse HEAD)" "$tree_digest" "$fingerprint" "$serial" "$api" "$port" "$metro_pid" "$(sha256sum "$apk" | cut -d' ' -f1)" >"$artifacts/source.txt"
adb -s "$serial" shell am start -a android.intent.action.VIEW -d "rudi://expo-development-client/?url=http%3A%2F%2F127.0.0.1%3A$port" >"$artifacts/launch.log" 2>&1
for ((i=1;i<=lap;i++));do
 maestro --device "$serial" test --test-output-dir "$artifacts/lap-$i" \
  -e TREE_FINGERPRINT="$fingerprint" -e RUDI_TEST_USERNAME="$RUDI_TEST_USERNAME" -e RUDI_TEST_PASSWORD="$RUDI_TEST_PASSWORD" \
  apps/mobile/.maestro/50-account-login.yaml >"$artifacts/maestro-$i.log" 2>&1 || { echo "Native auth đỏ; bằng chứng: $artifacts" >&2;exit 1; }
done
grep -Eq 'Android Bundled|Android .*bundled' "$artifacts/metro.log" || { echo 'Không có bằng chứng Android lấy bundle từ Metro của lượt này.' >&2;exit 1; }
if maestro --device "$serial" test --test-output-dir "$artifacts/canary" \
 -e TREE_FINGERPRINT=NONEXISTENT_ACCOUNT_TREE -e RUDI_TEST_USERNAME="$RUDI_TEST_USERNAME" -e RUDI_TEST_PASSWORD="$RUDI_TEST_PASSWORD" \
 apps/mobile/.maestro/50-account-login.yaml >"$artifacts/canary.log" 2>&1;then
 echo 'Canary dấu vân sai vẫn xanh.' >&2;exit 1
fi
first_failure="$(grep 'FAILED' "$artifacts/canary.log" | head -n 1)"
[[ "$first_failure" == *TREE_FINGERPRINT* ]] || { echo 'Canary đỏ sai bước, cần xem log.' >&2;exit 1; }
echo "Native auth: $lap lượt đạt. Phải mở ảnh ra xem: $artifacts"
