#!/usr/bin/env bash
# Start an isolated synthetic chat stack. No production secrets or databases.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
umask 077
if [[ "${1:-}" == restart-core ]]; then
  state="${2:?state directory required}"
  [[ "$state" =~ ^/tmp/rudi-chat-e2e\.[[:alnum:]]{6}$ && -f "$state/connection.json" ]] || exit 2
  container="$(node -e 'const c=require(process.argv[1]);if(!/^chat-e2e-[0-9]+-[0-9]+-core$/.test(c.coreContainer))process.exit(2);process.stdout.write(c.coreContainer)' "$state/connection.json")"
  docker inspect "$container" > "$state/core-inspect.json"
  node -e 'const fs=require("fs"),c=JSON.parse(fs.readFileSync(process.argv[1]))[0];if(c.Config.Env.some(v=>/[\n\r]/.test(v)))process.exit(2);fs.writeFileSync(process.argv[2],c.Config.Env.join("\n")+"\n",{mode:0o600})' "$state/core-inspect.json" "$state/restart-core.env"
  image="$(node -e 'const c=require(process.argv[1]);if(!/^rudi-chat-e2e-api:chat-e2e-[0-9]+-[0-9]+$/.test(c.image))process.exit(2);process.stdout.write(c.image)' "$state/connection.json")"
  core_url="$(node -e 'const c=require(process.argv[1]);if(!/^http:\/\/127\.0\.0\.1:[0-9]+$/.test(c.apiUrl))process.exit(2);process.stdout.write(c.apiUrl)' "$state/connection.json")"
  (cd "$ROOT/services/core" && go build -o "$state/core.next" ./cmd/core)
  mv "$state/core.next" "$state/core"
  docker rm -f "$container" >/dev/null
  docker run -d --rm --name "$container" --network host --user "$(id -u):$(id -g)" -v "$state:$state" --env-file "$state/restart-core.env" "$image" "$state/core" serve >/dev/null
  for _ in $(seq 1 60); do curl -fsS "$core_url/healthz" >/dev/null 2>&1 && break; sleep 1; done
  curl -fsS "$core_url/healthz" >/dev/null
  printf 'CORE READY: %s/connection.json\n' "$state"
  exit
fi
if [[ "${1:-}" == down ]]; then
  state="${2:?state directory required}"
  [[ "$state" =~ ^/tmp/rudi-chat-e2e\.[[:alnum:]]{6}$ ]] || exit 2
  while read -r container; do
    [[ "$container" == chat-e2e-*-pg || "$container" == chat-e2e-*-api || "$container" == chat-e2e-*-core ]] || exit 2
    docker rm -f "$container" >/dev/null
  done < "$state/containers"
  exit
fi
[[ "${1:-up}" == up ]] || exit 2
umask 077
work="$(mktemp -d /tmp/rudi-chat-e2e.XXXXXX)"
printf '%s\n' "$work"
touch "$work/containers"
cleanup_failed_start() {
  while read -r container; do docker rm -f "$container" >/dev/null 2>&1 || true; done < "$work/containers"
  printf 'Khởi tạo thất bại; log còn ở %s\n' "$work" >&2
}
trap cleanup_failed_start ERR
run="chat-e2e-$(date +%s)-$$"
# Optional deterministic inference seam for the E2E tier; empty in normal use.
brain_env=""
if [ -n "${CHAT_E2E_BRAIN_URL:-}" ]; then
  case "$CHAT_E2E_BRAIN_URL" in http://127.0.0.1:*) brain_env=1 ;; *) echo "CHAT_E2E_BRAIN_URL phải là loopback" >&2; exit 2 ;; esac
fi
free_port() { node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }
pg_port="$(free_port)"; api_port="$(free_port)"; core_port="$(free_port)"; live_port="$(free_port)"
password="$(openssl rand -hex 24)"; identity="$(openssl rand -hex 32)"
# The E2E tier needs the same internal token as its inference stub, so it may
# supply one. Anything else gets a fresh random secret, as before.
internal="${CHAT_E2E_INTERNAL_TOKEN:-$(openssl rand -hex 32)}"
case "$internal" in *[!0-9a-fA-F]*|"") echo "internal token phải là hex" >&2; exit 2 ;; esac
image="rudi-chat-e2e-api:$run"
docker build -q -t "$image" "$ROOT/services/api" >"$work/build.log" 2>&1
docker run -d --rm --name "$run-pg" -e POSTGRES_DB=chat_e2e_test -e POSTGRES_USER=chat_e2e -e POSTGRES_PASSWORD="$password" -p "127.0.0.1:$pg_port:5432" postgres:16-alpine -c timezone=UTC > /dev/null
printf '%s\n' "$run-pg" > "$work/containers"
for _ in $(seq 1 60); do docker exec "$run-pg" pg_isready -U chat_e2e -d chat_e2e_test >/dev/null 2>&1 && break; sleep 1; done
dsn="postgresql://chat_e2e:$password@127.0.0.1:$pg_port/chat_e2e_test"
sqlalchemy="postgresql+psycopg://chat_e2e:$password@127.0.0.1:$pg_port/chat_e2e_test"
docker run --rm --network host -e MOBILE_DATABASE_URL="$sqlalchemy" "$image" sh -c 'alembic upgrade head && python -m app.places.seed_catalog' >"$work/migrate.log" 2>&1
mkdir "$work/media"
docker run -d --rm --name "$run-api" --health-cmd "python -c \"import urllib.request; urllib.request.urlopen('http://127.0.0.1:$api_port/healthz', timeout=2)\"" --network host --user "$(id -u):$(id -g)" -e HOME=/tmp -v "$work/media:$work/media" -e MOBILE_DATABASE_URL="$sqlalchemy" -e MOBILE_AUTH_MODE=prod -e MOBILE_PERSON_ID_KEY="$identity" -e MOBILE_INTERNAL_TOKEN="$internal" -e MOBILE_OTP_DEBUG_CODE=000000 -e MOBILE_OTP_LOG_CODES=1 -e MOBILE_MEDIA_ROOT="$work/media" "$image" uvicorn app.api.main:app --host 127.0.0.1 --port "$api_port" >/dev/null
printf '%s\n' "$run-api" >> "$work/containers"
(cd "$ROOT/services/core" && go build -o "$work/core" ./cmd/core)
# No chat flag anywhere below: this prod stack is the proof that core serves
# the change feed and the AI engine by default. It only needs the schema.
MOBILE_DATABASE_URL="$dsn" "$work/core" migrate-chat >>"$work/migrate.log" 2>&1
docker run -d --rm --name "$run-core" --health-cmd "python -c \"import urllib.request; urllib.request.urlopen('http://127.0.0.1:$core_port/healthz', timeout=2)\"" --network host --user "$(id -u):$(id -g)" -v "$work:$work" -e MOBILE_CORE_LISTEN="127.0.0.1:$core_port" -e MOBILE_CORE_LIVENESS_LISTEN="127.0.0.1:$live_port" -e MOBILE_PYTHON_UPSTREAM="http://127.0.0.1:$api_port" -e MOBILE_AUTH_MODE=prod -e MOBILE_DATABASE_URL="$dsn" -e MOBILE_PERSON_ID_KEY="$identity" -e MOBILE_INTERNAL_TOKEN="$internal" -e MOBILE_OTP_DEBUG_CODE=000000 -e MOBILE_OTP_LOG_CODES=1 -e MOBILE_MEDIA_ROOT="$work/media" -e MOBILE_CORE_CANDIDATE_ROUTES=ported ${brain_env:+-e MOBILE_BRAIN_URL="$CHAT_E2E_BRAIN_URL"} "$image" "$work/core" serve >/dev/null
printf '%s\n' "$run-core" >> "$work/containers"
for _ in $(seq 1 60); do curl -fsS "http://127.0.0.1:$core_port/healthz" >/dev/null 2>&1 && break; sleep 1; done
curl -fsS "http://127.0.0.1:$core_port/healthz" >/dev/null
cat > "$work/connection.json" <<EOF
{"root":"$work","apiUrl":"http://127.0.0.1:$core_port","pythonUrl":"http://127.0.0.1:$api_port","databaseUrl":"$dsn","pgContainer":"$run-pg","apiContainer":"$run-api","coreContainer":"$run-core","image":"$image","sourceCommit":"$(git -C "$ROOT" rev-parse HEAD)","chatCandidate":true}
EOF
docker exec "$run-pg" psql -U chat_e2e -d chat_e2e_test -Atc 'show fsync; show synchronous_commit; show full_page_writes' > "$work/durability.txt"
echo "READY: $work/connection.json"
trap - ERR
