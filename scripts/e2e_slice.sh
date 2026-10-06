#!/usr/bin/env bash
#
# Run the vertical slice against an API and a database this script starts and
# destroys itself.
#
# ## Why this exists
#
# `apps/mobile/tests/e2e/vertical-slice.test.mjs` is the only test in this
# repository that proves the client and the server connect. Everything else
# proves a piece with the other side faked: `tests/api/` runs on a fake
# repository, `apps/mobile`'s suite runs on a fake API, and CLAUDE.md's table
# says outright what each cannot see. The slice drives `src/api.ts` -- the same
# module the app imports -- against a real FastAPI on a real PostgreSQL, from
# propose through confirm, batch, publish, the guest page, and receipt.
#
# Nothing ran it. Measured on 2026-08-30 at 1649c16:
#
#   - `npm test` cannot: `scripts.test` prunes `tests/e2e` by construction
#     (`find tests -path tests/e2e -prune -o -name '*.test.mjs' -print`), so
#     the 55 files it runs are exactly the ones that fake the server.
#   - `.github/workflows/test.yml`'s mobile job runs that same `npm test`.
#   - `scripts/gate.sh` had no stage for it, and `grep -rn test:e2e` over every
#     .yml, .sh, .py, .json and .md found one definition in package.json and
#     not one caller. Every other hit is a QA report written by hand.
#
# So the hero path -- the one thing the product is being built to demonstrate
# -- was proven by a file that ran when somebody remembered, and the QA reports
# record the times nobody did: "chua chay", "khong chay trong luot nay".
#
# ## Why it provisions instead of pointing at what is already running
#
# `BASE_URL` in `apps/mobile/src/api.ts` falls back to `http://localhost:8099`,
# and 8099 on this machine is the shared `make up` stack that every worktree
# uses. Measured 2026-08-30 while writing this: that container served 52 routes
# and this tree renders 58. Aiming the slice there does not test this branch,
# it tests whatever was built last and passes or fails for reasons no reader
# can attribute -- the shape CLAUDE.md's "do tai <sha>" rule exists to stop.
#
# The answer is the one `scripts/postgres_tier.sh` already reached for the
# repository tier: have nothing to guess. A PostgreSQL container of this
# script's own on a random loopback port, a uvicorn of its own on another,
# both removed on the way out including on Ctrl-C. It never reads or names a
# compose project, so no lane's stack can be caught by it.
#
# ## A skip is not a pass
#
# The slice skips when it cannot reach a server, on purpose -- a developer with
# no Postgres should still be able to run the rest of the suite. That makes its
# honest summary "1 skipped", which reads exactly like a pass in a summary
# line. `MOBILE_REQUIRE_E2E=1` is what turns the skip into a failure, and this
# script always sets it. Running the slice without it is how the slice sat
# unproven for a week.
#
# ## What this does NOT prove
#
# It drives the client over `node --test`, which is not a browser: `fetch` in
# node does not enforce CORS, so a CORS misconfiguration that kills the web
# build passes here exactly as it passes every other gate in this repository.
# It exercises two flows, not the API's whole surface -- `gate.sh api` and the
# postgres tier remain the breadth. It runs one uvicorn worker on a database
# with no other traffic, so it says nothing about races or query plans. And the
# disposable database runs with `fsync=off`, correct for something deleted
# thirty seconds later and indefensible for anything else.
#
# Usage:
#   scripts/e2e_slice.sh                provision, run the slice, tear down
#   scripts/e2e_slice.sh --keep         leave the stack up, print its URL
#   scripts/e2e_slice.sh --native       run managed-account Android acceptance
#   scripts/e2e_slice.sh -- -t "ten"    extra arguments go to `node --test`
#
# Exit codes: 0 the slice passed, 1 the slice failed,
# 2 it could not be run at all -- no docker, no node, never healthy.

set -uo pipefail

cd "$(dirname "$0")/.." || exit 2
REPO_ROOT="$PWD"

IMAGE="${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}"
KEEP=0
NATIVE=0
TEST_ARGS=()

while [ $# -gt 0 ]; do
  case "$1" in
    --keep) KEEP=1 ;;
    --native) NATIVE=1 ;;
    --) shift; TEST_ARGS=("$@"); break ;;
    -h|--help) sed -n '2,72p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Tham số lạ: $1 (xem scripts/e2e_slice.sh --help)" >&2; exit 2 ;;
  esac
  shift
done

CONTAINER=""
API_PID=""
CORE_PID=""
REDIS_CONTAINER=""
WORK_DIR=""

cleanup() {
  if [ "$KEEP" -ne 1 ] && [ -n "$REDIS_CONTAINER" ]; then docker rm -f "$REDIS_CONTAINER" >/dev/null 2>&1 || true; fi
  # EXIT so Ctrl-C and early returns are covered too. A uvicorn that outlives
  # the run is worse than none: it holds a port and answers /healthz, and the
  # next reader diagnoses a stale answer as somebody else's bug.
  if [ "$KEEP" -eq 1 ]; then
    return
  fi
  if [ -n "$CORE_PID" ]; then
    kill "$CORE_PID" >/dev/null 2>&1 || true
    wait "$CORE_PID" 2>/dev/null || true
  fi
  if [ -n "$API_PID" ]; then
    kill "$API_PID" >/dev/null 2>&1 || true
    wait "$API_PID" 2>/dev/null || true
  fi
  if [ -n "$CONTAINER" ]; then
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  fi
  if [ -n "$WORK_DIR" ]; then
    rm -rf "$WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

# --- the database ---------------------------------------------------------

provision_db() {
  command -v docker >/dev/null 2>&1 || {
    echo "không có docker — không dựng được database dùng một lần" >&2; return 2; }
  docker info >/dev/null 2>&1 || {
    echo "docker daemon không chạy" >&2; return 2; }
  docker image inspect "$IMAGE" >/dev/null 2>&1 || {
    echo "chưa có ảnh $IMAGE tại máy — chạy 'docker pull $IMAGE' một lần" >&2; return 2; }

  local password
  password="$(head -c 18 /dev/urandom | base64 | tr -d '/+=' | head -c 20)"
  CONTAINER="mobile-e2e-pg-$$-$(head -c 4 /dev/urandom | od -An -tx1 | tr -d ' \n')"

  # Random loopback port: a fixed one is the single thing guaranteed to collide
  # on a machine running five worktrees. fsync off is safe exactly here, where
  # the volume dies with the container and there is no crash to survive.
  docker run -d --rm --name "$CONTAINER" \
    -e POSTGRES_DB=mobile \
    -e POSTGRES_USER=mobile \
    -e POSTGRES_PASSWORD="$password" \
    -p 127.0.0.1::5432 \
    "$IMAGE" \
    -c fsync=off -c full_page_writes=off -c synchronous_commit=off >/dev/null || {
      echo "không khởi động được container postgres" >&2; return 2; }

  local hostport
  hostport="$(docker port "$CONTAINER" 5432/tcp 2>/dev/null | head -1)"
  hostport="${hostport##*:}"
  [ -n "$hostport" ] || { echo "không đọc được cổng đã publish" >&2; return 2; }

  # `-h 127.0.0.1` is not decoration. During initdb PostgreSQL runs a temporary
  # server bound to the unix socket only, so a `pg_isready` without -h answers
  # "ready" while the TCP port is still closed, and whatever starts on that
  # answer dies of connection refused. docker-compose.yml carries the same scar.
  local i
  for i in $(seq 1 60); do
    if docker exec "$CONTAINER" pg_isready -h 127.0.0.1 -U mobile -d mobile >/dev/null 2>&1; then
      DATABASE_URL="postgresql+psycopg://mobile:${password}@127.0.0.1:${hostport}/mobile"
      echo "database dùng một lần: 127.0.0.1:${hostport} (container ${CONTAINER}, sẵn sàng sau ${i}s)"
      return 0
    fi
    if [ "$(docker inspect --format '{{.State.Running}}' "$CONTAINER" 2>/dev/null)" != "true" ]; then
      echo "container postgres thoát trước khi sẵn sàng" >&2
      docker logs "$CONTAINER" 2>&1 | tail -20 >&2
      return 2
    fi
    sleep 1
  done
  echo "container postgres không bao giờ sẵn sàng" >&2
  docker logs "$CONTAINER" 2>&1 | tail -20 >&2
  return 2
}

# --- the API --------------------------------------------------------------

start_api() {
  # MOBILE_DATABASE_URL is set for every command below, never left to default.
  # `app/db/session.py` and `app/db/migrations/env.py` both fall back to the
  # shared dev database on localhost:5432 when it is unset, so an unset
  # variable here would migrate a database five other worktrees are using --
  # the accident that produced the orphaned-revision incident on 2026-08-29.
  echo "--- alembic upgrade head (database dùng một lần)"
  ( cd "$REPO_ROOT/services/api" \
      && MOBILE_DATABASE_URL="$DATABASE_URL" python3 -m alembic upgrade head ) || {
    echo "migration hỏng — không dựng được schema để chạy lát cắt" >&2; return 1; }

  # M9: danh mục địa điểm là một bảng, và một database vừa dựng thì bảng ấy
  # rỗng. Không seed thì Khám phá trống rỗng và mọi flow chạm địa điểm đỏ vì
  # thiếu dữ liệu chứ không phải vì app sai.
  echo "--- seed danh mục địa điểm"
  ( cd "$REPO_ROOT/services/api" \
      && MOBILE_DATABASE_URL="$DATABASE_URL" python3 -m app.places.seed_catalog ) || {
    echo "seed danh mục hỏng" >&2; return 1; }

  # `seed_catalog` cố ý chỉ mang HAI thành phố: mười hai hàng mẫu của nó nằm ở
  # hai thành phố ấy, và tầng Postgres đọc chung module đó nên đừng nới nó.
  # Nhưng bảng Maestro lái tới một thành phố KHÔNG có hàng mẫu nào («Hội An»,
  # flow 35) đúng để chứng minh màn Khám phá đọc điểm đến từ máy chủ chứ không
  # in cứng. Thiếu danh sách đầy đủ thì flow ấy đỏ ở bước cuộn, và cái đỏ ấy nói
  # về stack chứ không nói về app (đo 2026-09-07: 2 điểm đến, flow 35 đỏ).
  echo "--- seed 15 điểm đến của danh mục thật"
  ( cd "$REPO_ROOT/services/api" && MOBILE_DATABASE_URL="$DATABASE_URL" python3 - <<'PYDD'
import os

from sqlalchemy import create_engine
from sqlalchemy.orm import Session

from app.db.models import Destination
from app.places.destinations_vn import DESTINATIONS_VN

with Session(create_engine(os.environ["MOBILE_DATABASE_URL"])) as session:
    for row in DESTINATIONS_VN:
        existing = session.get(Destination, row["id"])
        if existing is None:
            session.add(Destination(**row))
            continue
        for key, value in row.items():
            if key != "id" and getattr(existing, key) != value:
                setattr(existing, key, value)
    session.commit()
PYDD
  ) || { echo "seed điểm đến hỏng" >&2; return 1; }

  local port
  port="$(python3 -c "import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()")" || { echo "không tìm được cổng trống" >&2; return 2; }

  WORK_DIR="$(mktemp -d)"
  API_LOG="$WORK_DIR/uvicorn.log"

  # A key of this run's own. The API answers 503 identity_key_missing without
  # one, and a literal in the repository would be the enumeration bug of
  # bug-140342 with extra steps -- see scripts/check_identity_key.sh.
  ID_KEY="$(head -c 48 /dev/urandom | base64 | tr -d '/+=' | head -c 44)"

  # Since the /internal brain door became fail-closed, create_app() refuses to
  # start without a token. It went into docker-compose and eight scripts but not
  # into this one, so the slice could not start an API at all -- the same miss
  # as scripts/parity_stacks.sh. A token of this run's own, like the id key
  # above: the slice never sends it, it only has to exist.
  INTERNAL_TOKEN="$(head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 32)"

  (
    cd "$REPO_ROOT/services/api" || exit 2
    MOBILE_DATABASE_URL="$DATABASE_URL" \
    MOBILE_MEDIA_ROOT="$WORK_DIR/media" \
    MOBILE_PERSON_ID_KEY="$ID_KEY" \
    MOBILE_INTERNAL_TOKEN="$INTERNAL_TOKEN" \
      python3 -m uvicorn app.api.main:app \
        --host 127.0.0.1 --port "$port" --log-level warning
  ) >"$API_LOG" 2>&1 &
  API_PID=$!

  API_URL="http://127.0.0.1:$port"

  # /healthz deliberately does not touch the database, so this loop proves a
  # process is answering and nothing more. The schema is proven by alembic's
  # exit code above, which is the right place for it: restarting the API never
  # fixed Postgres, and CLAUDE.md says so.
  local i
  for i in $(seq 1 60); do
    if curl -fsS --max-time 2 "$API_URL/healthz" >/dev/null 2>&1; then
      echo "API dùng một lần: $API_URL (sẵn sàng sau ${i}s)"
      return 0
    fi
    if ! kill -0 "$API_PID" 2>/dev/null; then
      echo "uvicorn thoát trước khi trả lời /healthz" >&2
      tail -30 "$API_LOG" >&2
      API_PID=""
      return 2
    fi
    sleep 1
  done
  echo "API không bao giờ trả lời /healthz" >&2
  tail -30 "$API_LOG" >&2
  return 2
}

# --- the front door -------------------------------------------------------

# ADR-0029: clients reach the API through the Go front door, so the slice does
# too. Everything below talks to API_URL, which from here on is core; uvicorn
# stays reachable only as core's upstream. Go missing is a failure, not a skip:
# a slice that quietly bypasses the front door proves nothing about it.
start_core() {
  if ! command -v go >/dev/null 2>&1; then
    echo "không có go trên PATH — lát cắt dọc phải đi qua cửa trước Go (ADR-0029)" >&2
    return 2
  fi
  local core_bin="$WORK_DIR/core" core_log="$WORK_DIR/core.log" port liveness
  ( cd "$REPO_ROOT/services/core" && go build -o "$core_bin" ./cmd/core ) || {
    echo "go build ./cmd/core thất bại" >&2
    return 2
  }
  # Profile routes have Go-owned tables and require an explicit migration.
  # Keep the disposable slice database aligned with the core being tested.
  if ! MOBILE_DATABASE_URL="$DATABASE_URL" "$core_bin" migrate-profile >>"$core_log" 2>&1; then
    echo 'không migrate được lược đồ hồ sơ Go:' >&2
    tail -3 "$core_log" >&2
    return 2
  fi
  port="$(python3 -c "import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()")" || return 2
  liveness="$(python3 -c "import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()")" || return 2

  # Same database as the API, and no MOBILE_AUTH_MODE for either, so both run
  # prod. Merged Go routes (manifest PORTED or later) are served from Go.
  #
  # core gets the SAME settings the API got, not fewer. A route moving to Go
  # moves its configuration with it: once W9 landed, core answered the OTP
  # routes and minted person ids, and a core without MOBILE_OTP_DEBUG_CODE or
  # MOBILE_PERSON_ID_KEY failed the slice at its first login. Same for the media
  # root, which the W6 photo routes write through. The rule to keep: whatever
  # the API is started with, core is started with.
  # This stack runs prod, where core serves the chat change feed and the AI
  # engine by default and refuses to start without their schema (serving never
  # runs DDL). So the chat migration always runs, flag or no flag. A caller can
  # still pass MOBILE_CHAT_CHANGES_CANDIDATE=0 below to measure a stack without
  # them.
  if ! MOBILE_DATABASE_URL="$DATABASE_URL" \
      "$core_bin" migrate-chat >>"$core_log" 2>&1; then
    echo 'không migrate được lược đồ chat (feed thay đổi + AI nhóm):' >&2
    tail -3 "$core_log" >&2
    return 2
  fi
  # --native: the Android table writes wall posts through the community
  # composer (flow 33, 42), which core serves only with its schema and flag.
  local community=""
  if [ "$NATIVE" -eq 1 ]; then
    MOBILE_DATABASE_URL="$DATABASE_URL" "$core_bin" migrate-community >>"$core_log" 2>&1 || {
      echo 'không migrate được lược đồ cộng đồng:' >&2; tail -3 "$core_log" >&2; return 2; }
    community=1
  fi
  ACCOUNT_ENCRYPTION_KEY="$(head -c 32 /dev/urandom | base64 | tr -d '\n')"
  ACCOUNT_LOOKUP_KEY="$(head -c 32 /dev/urandom | base64 | tr -d '\n')"
  REDIS_CONTAINER="rudi-e2e-auth-redis-$$"
  docker run -d --rm --name "$REDIS_CONTAINER" -p 127.0.0.1::6379 redis@sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499 redis-server --maxmemory-policy noeviction >/dev/null || return 2
  local redis_port
  redis_port="$(docker port "$REDIS_CONTAINER" 6379/tcp | cut -d: -f2)"
  for i in $(seq 1 30); do docker exec "$REDIS_CONTAINER" redis-cli ping >/dev/null 2>&1 && break; sleep 1; done
  MOBILE_DATABASE_URL="$DATABASE_URL" "$core_bin" migrate-accounts >>"$core_log" 2>&1 || return 2
  MOBILE_DATABASE_URL="$DATABASE_URL" "$core_bin" migrate-amendments >>"$core_log" 2>&1 || return 2
  MOBILE_CORE_LISTEN="127.0.0.1:$port" \
  MOBILE_CORE_LIVENESS_LISTEN="127.0.0.1:$liveness" \
  MOBILE_PYTHON_UPSTREAM="$API_URL" \
  MOBILE_DATABASE_URL="$DATABASE_URL" \
  MOBILE_CORE_CANDIDATE_ROUTES="${MOBILE_CORE_CANDIDATE_ROUTES:-ported}" \
  MOBILE_PERSON_ID_KEY="$ID_KEY" \
  MOBILE_MEDIA_ROOT="$WORK_DIR/media" \
  MOBILE_ACCOUNT_AUTH_ENABLED=1 \
  MOBILE_AMENDMENTS_ENABLED=1 \
  MOBILE_CHAT_V2_ENABLED=1 \
  MOBILE_PUSH_MODE=log \
  MOBILE_AUTH_TRUSTED_PROXY_CIDRS=127.0.0.1/32 \
  MOBILE_COMMUNITY_ENABLED="$community" \
  MOBILE_ACCOUNT_ENCRYPTION_KEY="$ACCOUNT_ENCRYPTION_KEY" \
  MOBILE_ACCOUNT_LOOKUP_KEY="$ACCOUNT_LOOKUP_KEY" \
  MOBILE_AUTH_REDIS_URL="redis://127.0.0.1:$redis_port/0" \
  MOBILE_EMAIL_SMTP_HOST=127.0.0.1 MOBILE_EMAIL_SMTP_PORT=9 \
  MOBILE_EMAIL_SMTP_USER=isolated-fixture MOBILE_EMAIL_SMTP_PASSWORD=isolated-fixture \
  MOBILE_EMAIL_FROM="fixture@${SYNTHETIC_MAIL_DOMAIN:-example.test}" \
  MOBILE_CHAT_CHANGES_CANDIDATE="${MOBILE_CHAT_CHANGES_CANDIDATE:-}" \
  MOBILE_INTERNAL_TOKEN="$INTERNAL_TOKEN" \
    "$core_bin" serve >"$core_log" 2>&1 &
  CORE_PID=$!

  local upstream="$API_URL" i
  API_URL="http://127.0.0.1:$port"
  for i in $(seq 1 30); do
    # Through core, so this proves the whole chain: core answers and reaches
    # the uvicorn above.
    if curl -fsS --max-time 2 "$API_URL/healthz" >/dev/null 2>&1; then
      echo "cửa trước Go: $API_URL -> $upstream (sẵn sàng sau ${i}s)"
      return 0
    fi
    if ! kill -0 "$CORE_PID" 2>/dev/null; then
      echo "core thoát trước khi trả lời /healthz" >&2
      tail -30 "$core_log" >&2
      CORE_PID=""
      return 2
    fi
    sleep 1
  done
  echo "core không bao giờ trả lời /healthz" >&2
  tail -30 "$core_log" >&2
  return 2
}

# --- synthetic accounts through the shipped HTTP auth door ----------------
mint_sessions() {
  SESSION_FILE="$WORK_DIR/sessions.json"
  # --native: the Android table's world (seven people per lap, passwords drawn
  # at runtime, nobody signed in yet), written next to the stack in the system
  # temp directory, never in a worktree.
  local world=""
  if [ "$NATIVE" -eq 1 ]; then world=native; fi
  (cd "$REPO_ROOT/services/core" && \
    CORE_REQUIRE_POSTGRES_TESTS=1 CORE_TEST_DATABASE_URL="${DATABASE_URL/postgresql+psycopg:/postgresql:}" \
    RUDI_TEST_ACCOUNT_URL="$API_URL" RUDI_TEST_ACCOUNT_OUTPUT="$WORK_DIR" \
    RUDI_TEST_ACCOUNT_WORLD="$world" RUDI_TEST_ACCOUNT_LAPS="${MOBILE_NATIVE_ACCOUNT_LAPS:-${MOBILE_NATIVE_LAP:-1}}" \
    MOBILE_ACCOUNT_ENCRYPTION_KEY="$ACCOUNT_ENCRYPTION_KEY" \
    MOBILE_ACCOUNT_LOOKUP_KEY="$ACCOUNT_LOOKUP_KEY" \
    go test -tags=postgres,authfixture -count=1 -run '^TestProvisionSyntheticAccountWorld$' ./internal/accountauth) || return 2
}

# --- run ------------------------------------------------------------------

command -v node >/dev/null 2>&1 || { echo "không có node" >&2; exit 2; }
command -v npm  >/dev/null 2>&1 || { echo "không có npm" >&2; exit 2; }
[ -d "$REPO_ROOT/apps/mobile/node_modules" ] || {
  echo "chưa 'npm ci' trong apps/mobile" >&2; exit 2; }

provision_db || exit $?
start_api || exit $?
start_core || exit $?
mint_sessions || exit $?

if [ "$NATIVE" -eq 1 ]; then
  # The same disposable HTTP-created account world supplies the device: the
  # whole product table (scripts/mobile_native.sh), not one login flow.
  # Real credentials never enter Maestro arguments or this fixture harness;
  # the synthetic ones live in $WORK_DIR (system temp) and die with the stack.
  # MOBILE_DATABASE_URL: two checks (story expiry after 43, the reports row
  # after 45) read the disposable database because no route exposes them.
  if [ "$KEEP" -eq 1 ]; then
    echo "--keep: giữ stack lại sau bảng Android."
    echo "  API:        $API_URL"
    echo "  tài khoản:  $WORK_DIR/credentials.json"
    echo "  dọn bằng:   docker rm -f $CONTAINER $REDIS_CONTAINER; kill $API_PID $CORE_PID; rm -rf $WORK_DIR"
  fi
  RUDI_NATIVE_TEST_ACK=synthetic-only MOBILE_DATABASE_URL="$DATABASE_URL" \
    scripts/mobile_native.sh --account --api-port "${API_URL##*:}" \
      --credentials "$WORK_DIR/credentials.json" --lap "${MOBILE_NATIVE_LAP:-1}" \
      ${ANDROID_SERIAL:+--serial "$ANDROID_SERIAL"} "${TEST_ARGS[@]}"
  exit $?
fi

if [ "$KEEP" -eq 1 ]; then
  echo "--keep: giữ stack lại."
  echo "  API:       $API_URL"
  echo "  fixture:   $WORK_DIR"
  echo "  dọn bằng:  docker rm -f $CONTAINER $REDIS_CONTAINER; kill $API_PID $CORE_PID; rm -rf $WORK_DIR"
fi

# EXPO_PUBLIC_API_URL is pinned, not defaulted. `src/api.ts` falls back to
# localhost:8099 -- the shared stack -- and a slice that silently tested
# another lane's container would be worse than no slice, because it would
# report a colour about code nobody can identify.
#
# MOBILE_REQUIRE_E2E=1 is the whole point: without it an unreachable server is
# `t.skip` and exit 0, and this script would provision a stack, fail to reach
# it, and report success.
echo "--- npm run test:e2e (EXPO_PUBLIC_API_URL=$API_URL, MOBILE_REQUIRE_E2E=1)"
(
  cd "$REPO_ROOT/apps/mobile" || exit 2
  EXPO_PUBLIC_API_URL="$API_URL" \
  MOBILE_REQUIRE_E2E=1 \
  RUDI_TEST_ACCOUNT_WORLD="$WORK_DIR/world.json" \
  MOBILE_E2E_SESSIONS="$SESSION_FILE" \
  MOBILE_E2E_DATABASE_URL="$DATABASE_URL" \
    npm run --silent test:e2e ${TEST_ARGS[0]+-- "${TEST_ARGS[@]}"}
)
rc=$?

if [ "$rc" -ne 0 ] && [ -n "${API_LOG:-}" ] && [ -s "$API_LOG" ]; then
  echo
  echo "---- 30 dòng cuối log uvicorn ----"
  tail -30 "$API_LOG"
fi

exit "$rc"
