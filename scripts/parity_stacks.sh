#!/usr/bin/env bash
# Bring up the two isolated stacks the parity harness compares (ADR-0029).
#
#   scripts/parity_stacks.sh up [--auth dev|prod] [--image TAG] [--env FILE]
#   scripts/parity_stacks.sh down --env FILE
#
# reference  Python API image  ->  its own Postgres
# candidate  core (Go, built from this tree)  ->  Python API image  ->  its own Postgres
#
# Both Python processes run the IMAGE, never the host's uvicorn: this machine
# has fastapi 0.135 and pydantic 2.12, the image pins fastapi 0.115 and pydantic
# 2.13, and those libraries write the bytes being compared.
#
# Everything uses host networking and loopback ports. The Docker daemon on this
# machine has run out of subnets for new networks (2026-09-14), so the harness
# must never need one.
#
# The two databases are separate on purpose. Running both sides against one
# database would let a write on one side answer a read on the other, and make
# every stateful scenario look equal.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$PWD"
PG_IMAGE="${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}"

usage() { sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }

free_port() {
  python3 -c "import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()"
}

wait_http() {
  local url="$1" what="$2" i
  for i in $(seq 1 90); do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      echo "  $what sẵn sàng sau ${i}s"
      return 0
    fi
    sleep 1
  done
  echo "$what không trả lời $url" >&2
  return 1
}

cmd_up() {
  local auth="dev" image="" env_file=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --auth) auth="$2"; shift ;;
      --image) image="$2"; shift ;;
      --env) env_file="$2"; shift ;;
      *) usage ;;
    esac
    shift
  done
  case "$auth" in dev|prod) ;; *) echo "--auth phải là dev hoặc prod" >&2; exit 2 ;; esac
  for tool in docker go curl python3; do
    command -v "$tool" >/dev/null 2>&1 || { echo "thiếu $tool" >&2; exit 2; }
  done

  local run work
  run="p$(date +%s)$$"
  work="$(mktemp -d)"
  env_file="${env_file:-$work/stacks.env}"

  if [ -z "$image" ]; then
    # Commit plus a hash of this checkout's path: two worktrees at one commit
    # with different uncommitted Python must not overwrite each other's tag.
    image="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$REPO_ROOT" | cksum | cut -d' ' -f1)"
    echo "--- dựng ảnh Python từ cây này: $image"
    ( cd services/api && docker build -q -t "$image" . ) >/dev/null
  fi
  local image_id
  image_id="$(docker image inspect --format '{{.Id}}' "$image")"

  # One key for both sides of a run: person ids are HMAC-derived, and the two
  # sides must derive the same ones. Random per run, never a literal in git.
  local id_key
  id_key="$(head -c 48 /dev/urandom | base64 | tr -d '/+=' | head -c 44)"

  local containers=() role
  declare -A api_url dsn
  for role in ref cand; do
    local pg_port api_port password pg_name api_name
    pg_port="$(free_port)"; api_port="$(free_port)"
    password="$(head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 24)"
    pg_name="parity-$run-$role-pg"; api_name="parity-$run-$role-api"

    echo "--- $role: Postgres trên 127.0.0.1:$pg_port"
    docker run -d --rm --name "$pg_name" \
      -e POSTGRES_DB=mobile -e POSTGRES_USER=mobile -e POSTGRES_PASSWORD="$password" \
      -p "127.0.0.1:$pg_port:5432" "$PG_IMAGE" \
      -c timezone=UTC -c fsync=off -c synchronous_commit=off -c full_page_writes=off >/dev/null
    containers+=("$pg_name")
    local i
    for i in $(seq 1 60); do
      docker exec "$pg_name" pg_isready -h 127.0.0.1 -U mobile -d mobile >/dev/null 2>&1 && break
      sleep 1
    done
    dsn[$role]="postgresql://mobile:$password@127.0.0.1:$pg_port/mobile"
    local sqlalchemy_url="postgresql+psycopg://mobile:$password@127.0.0.1:$pg_port/mobile"

    echo "--- $role: migrate + seed catalog (cùng lệnh với service migrate của compose)"
    docker run --rm --network host -e MOBILE_DATABASE_URL="$sqlalchemy_url" -e TZ=UTC "$image" \
      sh -c "alembic upgrade head && python -m app.places.seed_catalog" >"$work/$role-migrate.log" 2>&1 || {
        tail -20 "$work/$role-migrate.log" >&2; exit 1; }

    echo "--- $role: API trên 127.0.0.1:$api_port"
    local auth_env=()
    [ "$auth" = dev ] && auth_env=(-e MOBILE_AUTH_MODE=dev)
    docker run -d --rm --name "$api_name" --network host \
      -e MOBILE_DATABASE_URL="$sqlalchemy_url" \
      "${auth_env[@]}" \
      -e MOBILE_PERSON_ID_KEY="$id_key" \
      -e MOBILE_OTP_DEBUG_CODE=000000 -e MOBILE_OTP_LOG_CODES=1 \
      -e MOBILE_MEDIA_ROOT=/tmp/parity-media -e TZ=UTC \
      -e PYTHONUNBUFFERED=1 \
      "$image" uvicorn app.api.main:app --host 127.0.0.1 --port "$api_port" >/dev/null
    containers+=("$api_name")
    wait_http "http://127.0.0.1:$api_port/healthz" "$role API"
    api_url[$role]="http://127.0.0.1:$api_port"
  done

  echo "--- candidate: core trước API của candidate"
  ( cd services/core && go build -o "$work/core" ./cmd/core )
  ( cd parity && go build -o "$work/parity" ./cmd/parity )
  # The tap records every request that reaches the candidate's Python, so a run
  # can show which steps core answered in Go without looking inside core.
  local tap_port tap_control
  tap_port="$(free_port)"; tap_control="$(free_port)"
  nohup "$work/parity" tap --listen "127.0.0.1:$tap_port" --control "127.0.0.1:$tap_control" \
    --upstream "${api_url[cand]}" >"$work/tap.log" 2>&1 &
  local tap_pid=$!
  wait_http "http://127.0.0.1:$tap_port/healthz" "tap"
  local core_port live_port
  core_port="$(free_port)"; live_port="$(free_port)"
  # Go routes authenticate and query for themselves, so core runs in the auth
  # mode and on the database of the Python behind it. Every route whose Go code
  # is merged (manifest PORTED or later) is served from Go: that is what the
  # candidate stack is for.
  MOBILE_CORE_LISTEN="127.0.0.1:$core_port" \
  MOBILE_CORE_LIVENESS_LISTEN="127.0.0.1:$live_port" \
  MOBILE_PYTHON_UPSTREAM="http://127.0.0.1:$tap_port" \
  MOBILE_AUTH_MODE="$auth" \
  MOBILE_DATABASE_URL="${dsn[cand]}" \
  MOBILE_CORE_CANDIDATE_ROUTES="${PARITY_CANDIDATE_ROUTES:-ported}" \
    nohup "$work/core" serve >"$work/core.log" 2>&1 &
  local core_pid=$!
  wait_http "http://127.0.0.1:$core_port/healthz" "core"
  "$work/core" routes --json >"$work/served-routes.json"

  cat >"$env_file" <<ENV
PARITY_RUN=$run
PARITY_AUTH=$auth
PARITY_IMAGE=$image
PARITY_IMAGE_ID=$image_id
PARITY_REF_URL=${api_url[ref]}
PARITY_CAND_URL=http://127.0.0.1:$core_port
PARITY_CAND_PYTHON_URL=${api_url[cand]}
PARITY_CAND_TAP_URL=http://127.0.0.1:$tap_control
PARITY_SERVED_ROUTES=$work/served-routes.json
PARITY_TAP_PID=$tap_pid
PARITY_REF_DSN=${dsn[ref]}
PARITY_CAND_DSN=${dsn[cand]}
PARITY_CONTAINERS="${containers[*]}"
PARITY_CORE_PID=$core_pid
PARITY_WORK=$work
ENV
  echo
  echo "Hai stack đã lên (auth=$auth, ảnh $image)."
  echo "  env:  $env_file"
  echo "  so:   (cd parity && go run ./cmd/parity run --auth $auth --reference ${api_url[ref]} --candidate http://127.0.0.1:$core_port scenarios/)"
  echo "  tắt:  scripts/parity_stacks.sh down --env $env_file"
}

cmd_down() {
  local env_file=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --env) env_file="$2"; shift ;;
      *) usage ;;
    esac
    shift
  done
  [ -f "$env_file" ] || { echo "không thấy env file $env_file" >&2; exit 2; }
  # shellcheck disable=SC1090
  . "$env_file"
  kill "$PARITY_CORE_PID" >/dev/null 2>&1 || true
  kill "${PARITY_TAP_PID:-}" >/dev/null 2>&1 || true
  # shellcheck disable=SC2086
  docker rm -f $PARITY_CONTAINERS >/dev/null 2>&1 || true
  case "$PARITY_WORK" in /tmp/*) rm -rf "$PARITY_WORK" ;; esac
  echo "đã tắt $PARITY_RUN"
}

case "${1:-}" in
  up) shift; cmd_up "$@" ;;
  down) shift; cmd_down "$@" ;;
  *) usage ;;
esac
