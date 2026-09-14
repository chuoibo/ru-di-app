#!/usr/bin/env bash
# Run the Go core's real-PostgreSQL tests against a disposable database (ADR-0029).
#
#   scripts/go_postgres_tier.sh [--image TAG] [-- go test args...]
#
# PostgreSQL 16 on a random loopback port, migrated by Alembic from the API image
# -- Alembic stays the only schema owner, so the Go tests read exactly the schema
# production has -- then `go test -tags postgres` in services/core with
# CORE_TEST_DATABASE_URL set and CORE_REQUIRE_POSTGRES_TESTS=1.
#
# Two ways this tier could read green while measuring nothing, both refused:
#   * no database: the tests skip. CORE_REQUIRE_POSTGRES_TESTS=1 turns that
#     skip into a failure, the rule tests/postgres already follows in Python.
#   * no tests: `go test -tags postgres ./...` passes when no file carries the
#     tag. The run must show the sentinel TestPostgresTierReachesDatabase
#     passing, or the tier fails.
#   * a test that skips itself: the idempotency and repository oracles skip
#     without an image to run Python from. This tier hands them its own API
#     image, and any SKIP line fails the tier.
#
# Host networking and published loopback ports only: the Docker daemon on the
# shared machine has no subnets left for new networks.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$PWD"
PG_IMAGE="${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}"
SENTINEL="TestPostgresTierReachesDatabase"

image=""
while [ $# -gt 0 ]; do
  case "$1" in
    --image) image="$2"; shift 2 ;;
    --) shift; break ;;
    *) break ;;
  esac
done
go_args=("$@")
[ ${#go_args[@]} -eq 0 ] && go_args=(./...)

for tool in docker go python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "thiếu $tool" >&2; exit 2; }
done

container="go-pg-tier-$$"
log="$(mktemp)"
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -f "$log"
}
trap cleanup EXIT INT TERM

if [ -z "$image" ]; then
  # Same tag scheme as scripts/parity_stacks.sh, so one build serves both.
  image="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$REPO_ROOT" | cksum | cut -d' ' -f1)"
  echo "--- dựng ảnh API từ cây này: $image"
  ( cd services/api && docker build -q -t "$image" . ) >/dev/null
fi

port="$(python3 -c "import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()")"
password="$(head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 24)"

echo "--- PostgreSQL dùng một lần trên 127.0.0.1:$port"
docker run -d --rm --name "$container" \
  -e POSTGRES_DB=mobile -e POSTGRES_USER=mobile -e POSTGRES_PASSWORD="$password" \
  -p "127.0.0.1:$port:5432" "$PG_IMAGE" \
  -c timezone=UTC -c fsync=off -c synchronous_commit=off -c full_page_writes=off >/dev/null
for _ in $(seq 1 60); do
  docker exec "$container" pg_isready -h 127.0.0.1 -U mobile -d mobile >/dev/null 2>&1 && break
  sleep 1
done

echo "--- alembic upgrade head (từ ảnh API)"
docker run --rm --network host \
  -e MOBILE_DATABASE_URL="postgresql+psycopg://mobile:$password@127.0.0.1:$port/mobile" \
  "$image" alembic upgrade head >"$log" 2>&1 || { tail -20 "$log" >&2; exit 1; }

echo "--- go test -tags postgres ${go_args[*]}"
set +e
(
  cd services/core &&
    CORE_TEST_DATABASE_URL="postgresql://mobile:$password@127.0.0.1:$port/mobile" \
    CORE_REQUIRE_POSTGRES_TESTS=1 \
    IDEM_ORACLE_IMAGE="$image" \
    CORE_PYTHON_IMAGE="$image" \
    go test -tags postgres -count=1 -v "${go_args[@]}"
) 2>&1 | tee "$log"
rc=${PIPESTATUS[0]}
set -e

if [ "$rc" -ne 0 ]; then
  echo "HỎNG: go test -tags postgres thoát $rc" >&2
  exit "$rc"
fi
if ! grep -qE "^--- PASS: $SENTINEL\b" "$log"; then
  echo "HỎNG: không thấy $SENTINEL PASS -- tầng này không đo được gì (thiếu test mang tag postgres?)" >&2
  exit 1
fi
if grep -qE '^\s*--- SKIP: ' "$log"; then
  echo "HỎNG: có ca bị bỏ qua trong tầng Postgres -- bỏ qua không phải là xanh:" >&2
  grep -E '^\s*--- SKIP: ' "$log" >&2
  exit 1
fi
passed="$(grep -cE '^\s*--- PASS: ' "$log" || true)"
echo "tầng Postgres của Go: $passed ca PASS, sentinel có mặt"
