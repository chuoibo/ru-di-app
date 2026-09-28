#!/usr/bin/env bash
# Run the Go core's broker tests: Redis Streams (internal/aistream) and the
# RabbitMQ outbox relay (internal/jobs), against real services.
#
#   scripts/go_broker_tier.sh [-- go test args...]
#
# Each service comes from the environment when the caller already has one --
# CORE_TEST_DATABASE_URL, CORE_TEST_REDIS_URL, CORE_TEST_AMQP_URL -- and from a
# disposable container on a random loopback port otherwise. The first form is
# how the tier runs on a machine without Docker; the second is the default.
#
# The database is migrated by Alembic, as in go_postgres_tier.sh: the job
# queue's end-to-end tests (internal/chatassist) copy the public tables the AI
# engine joins -- people, contexts, memberships, messages -- into a schema of
# their own, then install internal/jobs and chatassist there. A disposable
# database is migrated here from the API image (`--image TAG`, or built from
# this tree under the tag go_postgres_tier.sh and the parity stacks use); a
# database given by CORE_TEST_DATABASE_URL must already be at `alembic head`.
#
# The ways this tier could read green while measuring nothing, all refused:
#   * no service: the tests skip. CORE_REQUIRE_BROKER_TESTS=1 and
#     CORE_REQUIRE_POSTGRES_TESTS=1 turn those skips into failures, and any
#     SKIP line left in the log fails the tier anyway.
#   * no tests: `go test -tags broker` passes on a package with no tagged
#     file. Both sentinels must be seen passing, one per service family.
#   * a package forgotten: the package list is not written here. It is every
#     directory holding a `//go:build broker` test file, so a new one runs
#     the day it is added, and an empty list is a failure.
set -euo pipefail

cd "$(dirname "$0")/.."
PG_IMAGE="${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}"
REDIS_IMAGE="${MOBILE_TEST_REDIS_IMAGE:-redis:7-alpine}"
RABBIT_IMAGE="${MOBILE_TEST_RABBITMQ_IMAGE:-rabbitmq:3.13-alpine}"
# One per service family, the queue end to end through the AI engine, and
# the answer stream end to end to a real SSE client (slice 11), and to
# another member of the room over the change feed's WebSocket (slice 12).
SENTINELS=(TestBrokerTierReachesRedis TestBrokerTierReachesRabbitAndPostgres TestHangDoiDauCuoiQuaBroker TestStreamNepDauCuoiQuaSSE TestPhongThanhVienKhacThayChuSauKhiTheDang)

image=""
while [ $# -gt 0 ]; do
  case "$1" in
    --image) image="$2"; shift 2 ;;
    --) shift; break ;;
    *) break ;;
  esac
done
go_args=("$@")
if [ ${#go_args[@]} -eq 0 ]; then
  while IFS= read -r dir; do
    go_args+=("./${dir#services/core/}")
  done < <(grep -rlE '^//go:build .*\bbroker\b' services/core --include='*_test.go' |
    xargs -r -n1 dirname | sort -u)
  if [ ${#go_args[@]} -eq 0 ]; then
    echo "HỎNG: không có file test nào mang tag broker -- tầng này không có gì để đo" >&2
    exit 1
  fi
fi

command -v go >/dev/null 2>&1 || { echo "thiếu go" >&2; exit 2; }

containers=()
log="$(mktemp)"
cleanup() {
  for c in "${containers[@]}"; do docker rm -f "$c" >/dev/null 2>&1 || true; done
  rm -f "$log"
}
trap cleanup EXIT INT TERM

free_port() {
  python3 -c "import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()"
}
secret() { head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 24; }

need_docker() {
  for tool in docker python3; do
    command -v "$tool" >/dev/null 2>&1 || { echo "thiếu $tool (hoặc đặt sẵn $1)" >&2; exit 2; }
  done
  docker info >/dev/null 2>&1 || { echo "docker daemon không trả lời (hoặc đặt sẵn $1)" >&2; exit 2; }
}

# wait_for NAME TRIES CMD... retries CMD once a second; a service that never
# answers fails the tier with its own log, not a timeout inside go test.
#
# The containers are started WITHOUT --rm (the EXIT trap removes them). With
# --rm a container that crashed on boot was gone before this could read its
# log: CI printed «không lên sau 90s» and then «No such container», which says
# nothing about why. A container that has exited is also reported at once,
# with its exit code, instead of after the full wait.
wait_for() {
  local name="$1" tries="$2"
  shift 2
  for _ in $(seq 1 "$tries"); do
    "$@" >/dev/null 2>&1 && return 0
    if [ "$(docker inspect -f '{{.State.Running}}' "$name" 2>/dev/null)" != "true" ]; then
      echo "HỎNG: $name đã dừng trước khi lên ($(docker inspect -f 'exit={{.State.ExitCode}} oom={{.State.OOMKilled}} error={{.State.Error}}' "$name" 2>&1))" >&2
      docker logs --tail 80 "$name" >&2 2>&1 || true
      exit 1
    fi
    sleep 1
  done
  echo "HỎNG: $name không lên sau ${tries}s" >&2
  docker logs --tail 80 "$name" >&2 2>&1 || true
  exit 1
}

if [ -z "${CORE_TEST_DATABASE_URL:-}" ]; then
  need_docker CORE_TEST_DATABASE_URL
  name="go-broker-pg-$$" port="$(free_port)" password="$(secret)"
  echo "--- PostgreSQL dùng một lần trên 127.0.0.1:$port"
  docker run -d --name "$name" \
    -e POSTGRES_DB=mobile -e POSTGRES_USER=mobile -e POSTGRES_PASSWORD="$password" \
    -p "127.0.0.1:$port:5432" "$PG_IMAGE" -c timezone=UTC -c fsync=off >/dev/null
  containers+=("$name")
  wait_for "$name" 60 docker exec "$name" pg_isready -h 127.0.0.1 -U mobile -d mobile
  export CORE_TEST_DATABASE_URL="postgresql://mobile:$password@127.0.0.1:$port/mobile"
  if [ -z "$image" ]; then
    # Same tag scheme as go_postgres_tier.sh, so one build serves both.
    image="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" | cksum | cut -d' ' -f1)"
    echo "--- dựng ảnh API từ cây này: $image"
    ( cd services/api && docker build -q -t "$image" . ) >/dev/null
  fi
  echo "--- alembic upgrade head (từ ảnh API)"
  docker run --rm --network host \
    -e MOBILE_DATABASE_URL="postgresql+psycopg://mobile:$password@127.0.0.1:$port/mobile" \
    "$image" alembic upgrade head >"$log" 2>&1 || { tail -20 "$log" >&2; exit 1; }
else
  echo "--- PostgreSQL có sẵn từ CORE_TEST_DATABASE_URL (phải đã ở alembic head)"
fi

if [ -z "${CORE_TEST_REDIS_URL:-}" ]; then
  need_docker CORE_TEST_REDIS_URL
  name="go-broker-redis-$$" port="$(free_port)"
  echo "--- Redis dùng một lần trên 127.0.0.1:$port"
  docker run -d --name "$name" -p "127.0.0.1:$port:6379" "$REDIS_IMAGE" \
    redis-server --save "" --appendonly no >/dev/null
  containers+=("$name")
  wait_for "$name" 30 docker exec "$name" redis-cli ping
  export CORE_TEST_REDIS_URL="redis://127.0.0.1:$port/0"
else
  echo "--- Redis có sẵn từ CORE_TEST_REDIS_URL"
fi

if [ -z "${CORE_TEST_AMQP_URL:-}" ]; then
  need_docker CORE_TEST_AMQP_URL
  name="go-broker-rabbit-$$" port="$(free_port)" password="$(secret)"
  echo "--- RabbitMQ dùng một lần trên 127.0.0.1:$port"
  # --ulimit nofile: the Erlang VM sizes its port table from the open-file
  # limit, and a Docker whose containers inherit LimitNOFILE=infinity hands
  # it a limit around 2^30. A precaution; 65536 is the value RabbitMQ's
  # production checklist gives.
  docker run -d --name "$name" --ulimit nofile=65536:65536 \
    -e RABBITMQ_DEFAULT_USER=tier -e RABBITMQ_DEFAULT_PASS="$password" \
    -p "127.0.0.1:$port:5672" "$RABBIT_IMAGE" >/dev/null
  containers+=("$name")
  # The node answers `ping` before its listener is up; ask for the port.
  # -u rabbitmq: root's ~/.erlang.cookie in this image is a symlink to the
  # node's own cookie. A probe run as root that reaches Erlang before the
  # node does creates that file root-owned with mode 0400, and the node then
  # dies on boot with "Error when reading .erlang.cookie: eacces" -- the log
  # GitHub printed on 2026-09-28 once the container was no longer --rm.
  wait_for "$name" 90 docker exec -u rabbitmq "$name" rabbitmq-diagnostics -q check_port_connectivity
  export CORE_TEST_AMQP_URL="amqp://tier:$password@127.0.0.1:$port/"
else
  echo "--- RabbitMQ có sẵn từ CORE_TEST_AMQP_URL"
fi

echo "--- go test -tags broker ${go_args[*]}"
set +e
(
  cd services/core &&
    CORE_REQUIRE_BROKER_TESTS=1 \
    CORE_REQUIRE_POSTGRES_TESTS=1 \
    go test -tags broker -count=1 -race -timeout 10m -v "${go_args[@]}"
) 2>&1 | tee "$log"
rc=${PIPESTATUS[0]}
set -e

if [ "$rc" -ne 0 ]; then
  echo "HỎNG: go test -tags broker thoát $rc" >&2
  exit "$rc"
fi
for sentinel in "${SENTINELS[@]}"; do
  if ! grep -qE "^--- PASS: $sentinel\b" "$log"; then
    echo "HỎNG: không thấy $sentinel PASS -- tầng này không đo được dịch vụ đó" >&2
    exit 1
  fi
done
if grep -qE '^\s*--- SKIP: ' "$log"; then
  echo "HỎNG: có ca bị bỏ qua trong tầng broker -- bỏ qua không phải là xanh:" >&2
  grep -E '^\s*--- SKIP: ' "$log" >&2
  exit 1
fi
passed="$(grep -cE '^\s*--- PASS: ' "$log" || true)"
echo "tầng broker của Go: $passed ca PASS trên ${#go_args[@]} gói, ${#SENTINELS[@]} sentinel có mặt"
