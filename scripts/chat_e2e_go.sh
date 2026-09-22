#!/usr/bin/env bash
# Chat end-to-end tier: drive the Go front door over real HTTP and WebSocket.
#
#   scripts/chat_e2e_go.sh [-- go test args...]
#
# Every other Go test in this repository stops at a package boundary. This tier
# starts a real stack -- PostgreSQL, the Python API for the routes Go still
# proxies, and the Go core with the chat candidate on -- seeds synthetic
# accounts through the real OTP flow, and then talks to it the way a phone
# would. A case that passes here passes for a client.
#
# Four ways a tier like this reads green while measuring nothing. All refused:
#
#   * no stack: the suite would skip. TestMain exits 2 instead, and this script
#     fails if the stack never came up.
#   * no cases: `go test` passes when a build tag excludes every file. The run
#     must show the sentinel TestChatE2EReachesGoFrontDoor, or the tier fails.
#   * wrong process: the cases could be measuring Python through the proxy.
#     The sentinel asserts a Go-only route answers, so the tier knows whose
#     behaviour it recorded.
#   * a case that skips itself: any `--- SKIP:` line fails the tier. The AI
#     cases get a deterministic brain stub rather than an excuse to skip.
#
# Artifacts stay outside the repository. Synthetic data only: the seeded
# accounts, the stub's itinerary and every message here are invented.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$PWD"
SENTINEL="TestChatE2EReachesGoFrontDoor"

while [ $# -gt 0 ]; do
  case "$1" in
    --) shift; break ;;
    *) break ;;
  esac
done
go_args=("$@")

for tool in docker go node python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "thiếu $tool" >&2; exit 2; }
done

state=""
stub_pid=""
stub_bin=""
cleanup() {
  if [ -n "$stub_pid" ]; then kill "$stub_pid" 2>/dev/null || true; wait "$stub_pid" 2>/dev/null || true; fi
  [ -n "${stub_bin:-}" ] && rm -f "$stub_bin"
  if [ -n "$state" ] && [ -d "$state" ]; then
    scripts/chat_e2e_stack.sh down "$state" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

# The brain stub answers the inference seam deterministically. It must be
# listening before the core starts, because the core reads MOBILE_BRAIN_URL once.
brain_port="$(node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})')"
export CHAT_E2E_BRAIN_URL="http://127.0.0.1:$brain_port"
# One secret shared by the stub and the stack. Generated here because the stub
# must already be listening when the core reads MOBILE_BRAIN_URL at startup.
export CHAT_E2E_INTERNAL_TOKEN="$(openssl rand -hex 32)"

echo "--- dựng brain stub tất định trên $CHAT_E2E_BRAIN_URL"
stub_log="$(mktemp)"
stub_bin="$(mktemp -u)"
( cd services/core && go build -tags e2e -o "$stub_bin" ./e2e/brainstub )
BRAIN_STUB_LISTEN="127.0.0.1:$brain_port" \
MOBILE_INTERNAL_TOKEN="$CHAT_E2E_INTERNAL_TOKEN" \
  "$stub_bin" >"$stub_log" 2>&1 &
stub_pid=$!

for _ in $(seq 1 60); do
  if curl -fsS --max-time 2 "$CHAT_E2E_BRAIN_URL/healthz" >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -fsS --max-time 2 "$CHAT_E2E_BRAIN_URL/healthz" >/dev/null 2>&1 || {
  echo "brain stub không lên; log:" >&2; cat "$stub_log" >&2; exit 1; }

echo "--- dựng stack chat (PostgreSQL + API Python + core Go, candidate bật)"
ready="$(scripts/chat_e2e_stack.sh up | tail -1)"
state="$(dirname "${ready#READY: }")"
[ -d "$state" ] || { echo "chat_e2e_stack.sh không trả thư mục state" >&2; exit 1; }
echo "--- state: $state"

echo "--- seed 22 tài khoản tổng hợp qua OTP thật"
node scripts/chat_e2e_seed.mjs "$state/connection.json"

log="$(mktemp)"
set +e
( cd services/core && \
  CHAT_E2E_SESSIONS="$state/sessions.json" \
  CHAT_E2E_CONNECTION="$state/connection.json" \
  go test -tags e2e -count=1 -timeout 25m -v ./e2e/chat "${go_args[@]}" ) 2>&1 | tee "$log"
status="${PIPESTATUS[0]}"
set -e

if grep -q -- "--- SKIP:" "$log"; then
  echo "tầng E2E chat: có ca tự bỏ qua — tầng này không chấp nhận SKIP" >&2
  grep -- "--- SKIP:" "$log" >&2
  exit 1
fi
if ! grep -qE -- "^(--- )?PASS: $SENTINEL|^--- PASS: $SENTINEL" "$log"; then
  echo "tầng E2E chat: thiếu sentinel $SENTINEL — không có bằng chứng đã chạm core Go" >&2
  exit 1
fi
if [ "$status" -ne 0 ]; then
  echo "tầng E2E chat: có ca ĐỎ" >&2
  grep -E -- "^(    )*--- FAIL:" "$log" >&2 || true
  exit "$status"
fi

passed="$(grep -cE -- "^(    )*--- PASS:" "$log" || true)"
echo "tầng E2E chat: $passed ca PASS, sentinel có mặt, không có SKIP"
