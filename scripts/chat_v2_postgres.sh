#!/usr/bin/env bash
# Exercise the isolated chat foundation against migrated, disposable PostgreSQL.
set -euo pipefail
cd "$(dirname "$0")/.."

for tool in docker go rg; do
  command -v "$tool" >/dev/null || { echo "Thiếu $tool" >&2; exit 2; }
done
scratch="$(mktemp -d /tmp/rudi-chat-pg.XXXXXXXX)"
container=""
cleanup() {
  if [ -n "$container" ]; then docker rm -f "$container" >/dev/null 2>&1 || true; fi
  rm -rf "$scratch"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Build from this checkout; Docker's layer cache cannot substitute stale source.
docker build -q services/api >"$scratch/image"
api_image="$(tail -1 "$scratch/image")"
container="$(docker run -d --rm \
  -e POSTGRES_DB=chat_test -e POSTGRES_USER=chat_test \
  -e POSTGRES_PASSWORD=synthetic-chat-test-only \
  -p 127.0.0.1::5432 postgres:16-alpine)"
port="$(docker port "$container" 5432/tcp)"
port="${port##*:}"
ready=false
for ((attempt=0; attempt<60; attempt++)); do
  if docker exec "$container" pg_isready -U chat_test -d chat_test >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [ "$ready" != true ]; then echo 'PostgreSQL không sẵn sàng' >&2; exit 1; fi

# Existing migrations remain the schema baseline; new chat migrations are Go/SQL.
docker run --rm --network host \
  -e MOBILE_DATABASE_URL="postgresql+psycopg://chat_test:synthetic-chat-test-only@127.0.0.1:$port/chat_test" \
  "$api_image" alembic upgrade head
(
  cd services/core
  CORE_TEST_DATABASE_URL="postgresql://chat_test:synthetic-chat-test-only@127.0.0.1:$port/chat_test" \
  CORE_REQUIRE_POSTGRES_TESTS=1 \
    go test -race -tags postgres -count=1 -timeout 5m -v \
    ./internal/chatv2 ./internal/chatv2http ./cmd/chat-lab
) 2>&1 | tee "$scratch/tests.log"

# A successful command with missing or skipped database cases is not evidence.
for sentinel in TestSendConcurrentReplayConflictAndCatchup \
  TestCatchupBoundsBytesWithoutSkippingLargeEnvelopes \
  TestPostgresTwoReplicasThreePeopleAndRestart \
  TestPostgresSessionRevocationClosesQuietSocket; do
  if ! rg -q "^--- PASS: $sentinel " "$scratch/tests.log"; then
    echo "Thiếu bằng chứng PASS: $sentinel" >&2
    exit 1
  fi
done
if rg -q '^\s*--- SKIP:' "$scratch/tests.log"; then
  echo 'Có test bị bỏ qua; cổng không đạt' >&2
  exit 1
fi
echo 'Đạt cổng PostgreSQL/transport chat thử nghiệm; chưa chứng minh MLS hoặc native.'
