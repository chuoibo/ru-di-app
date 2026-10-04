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
    [[ "$container" == chat-e2e-*-pg || "$container" == chat-e2e-*-api || "$container" == chat-e2e-*-core || "$container" == chat-e2e-*-redis ]] || exit 2
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
# Optional deterministic Gemini stand-in for the E2E tier (loopback only, so
# the synthetic key can never reach a real provider); empty in normal use, and
# the stack then runs keyless.
gemini_env=""
if [ -n "${CHAT_E2E_GEMINI_URL:-}" ]; then
  case "$CHAT_E2E_GEMINI_URL" in http://127.0.0.1:*) gemini_env=1 ;; *) echo "CHAT_E2E_GEMINI_URL phải là loopback" >&2; exit 2 ;; esac
fi
free_port() { node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }
pg_port="$(free_port)"; api_port="$(free_port)"; core_port="$(free_port)"; live_port="$(free_port)"; redis_port="$(free_port)"
password="$(openssl rand -hex 24)"
account_encryption="$(openssl rand -base64 32)"; account_lookup="$(openssl rand -base64 32)"
internal="$(openssl rand -hex 32)"
image="rudi-chat-e2e-api:$run"
docker build -q -t "$image" "$ROOT/services/api" >"$work/build.log" 2>&1
docker run -d --rm --name "$run-pg" -e POSTGRES_DB=chat_e2e_test -e POSTGRES_USER=chat_e2e -e POSTGRES_PASSWORD="$password" -p "127.0.0.1:$pg_port:5432" postgres:16-alpine -c timezone=UTC > /dev/null
printf '%s\n' "$run-pg" > "$work/containers"
docker run -d --rm --name "$run-redis" -p "127.0.0.1:$redis_port:6379" redis:7.4-alpine@sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499 redis-server --maxmemory-policy noeviction --appendonly yes --appendfsync always >/dev/null
printf '%s\n' "$run-redis" >> "$work/containers"
for _ in $(seq 1 30);do docker exec "$run-redis" redis-cli ping >/dev/null 2>&1 && break; sleep 1;done
for _ in $(seq 1 60); do docker exec "$run-pg" pg_isready -U chat_e2e -d chat_e2e_test >/dev/null 2>&1 && break; sleep 1; done
dsn="postgresql://chat_e2e:$password@127.0.0.1:$pg_port/chat_e2e_test"
sqlalchemy="postgresql+psycopg://chat_e2e:$password@127.0.0.1:$pg_port/chat_e2e_test"
docker run --rm --network host -e MOBILE_DATABASE_URL="$sqlalchemy" "$image" sh -c 'alembic upgrade head && python -m app.places.seed_catalog' >"$work/migrate.log" 2>&1
mkdir "$work/media"
docker run -d --rm --name "$run-api" --health-cmd "python -c \"import urllib.request; urllib.request.urlopen('http://127.0.0.1:$api_port/healthz', timeout=2)\"" --network host --user "$(id -u):$(id -g)" -e HOME=/tmp -v "$work/media:$work/media" -e MOBILE_DATABASE_URL="$sqlalchemy" -e MOBILE_AUTH_MODE=prod -e MOBILE_INTERNAL_TOKEN="$internal" -e MOBILE_MEDIA_ROOT="$work/media" "$image" uvicorn app.api.main:app --host 127.0.0.1 --port "$api_port" >/dev/null
printf '%s\n' "$run-api" >> "$work/containers"
(cd "$ROOT/services/core" && go build -o "$work/core" ./cmd/core)
# No chat flag anywhere below: this prod stack is the proof that core serves
# the change feed and the AI engine by default. It only needs the schema.
MOBILE_DATABASE_URL="$dsn" "$work/core" migrate-chat >>"$work/migrate.log" 2>&1
MOBILE_DATABASE_URL="$dsn" "$work/core" migrate-profile >>"$work/migrate.log" 2>&1
MOBILE_DATABASE_URL="$dsn" "$work/core" migrate-accounts >>"$work/migrate.log" 2>&1
# The Go engine's catalogue retrieval reads the retrieval index, not the
# places table: build it over the seed catalogue, evaluate it, promote it.
MOBILE_DATABASE_URL="$dsn" "$work/core" migrate-rag >>"$work/migrate.log" 2>&1
rag_version="$(MOBILE_DATABASE_URL="$dsn" "$work/core" rag build 2>>"$work/migrate.log" | python3 -c 'import json,sys; print(json.load(sys.stdin)["phien_ban"])')"
MOBILE_DATABASE_URL="$dsn" "$work/core" rag eval "$rag_version" >>"$work/migrate.log" 2>&1
MOBILE_DATABASE_URL="$dsn" "$work/core" rag promote "$rag_version" >>"$work/migrate.log" 2>&1
# repo-guard: allow=email reason=synthetic-reserved-test-domain
docker run -d --rm --name "$run-core" --health-cmd "python -c \"import urllib.request; urllib.request.urlopen('http://127.0.0.1:$core_port/healthz', timeout=2)\"" --network host --user "$(id -u):$(id -g)" -v "$work:$work" -e MOBILE_CORE_LISTEN="127.0.0.1:$core_port" -e MOBILE_CORE_LIVENESS_LISTEN="127.0.0.1:$live_port" -e MOBILE_PYTHON_UPSTREAM="http://127.0.0.1:$api_port" -e MOBILE_AUTH_MODE=prod -e MOBILE_DATABASE_URL="$dsn" -e MOBILE_ACCOUNT_AUTH_ENABLED=1 -e MOBILE_ACCOUNT_ENCRYPTION_KEY="$account_encryption" -e MOBILE_ACCOUNT_LOOKUP_KEY="$account_lookup" -e MOBILE_AUTH_REDIS_URL="redis://127.0.0.1:$redis_port/0" -e MOBILE_EMAIL_SMTP_HOST=127.0.0.1 -e MOBILE_EMAIL_SMTP_PORT=9 -e MOBILE_EMAIL_SMTP_USER=isolated-fixture -e MOBILE_EMAIL_SMTP_PASSWORD=isolated-fixture -e MOBILE_EMAIL_FROM=fixture@example.test -e MOBILE_INTERNAL_TOKEN="$internal" -e MOBILE_MEDIA_ROOT="$work/media" -e MOBILE_CORE_CANDIDATE_ROUTES=ported ${gemini_env:+-e GEMINI_API_KEY=e2e-stub-key -e MOBILE_GEMINI_BASE_URL="$CHAT_E2E_GEMINI_URL"} "$image" "$work/core" serve >/dev/null
printf '%s\n' "$run-core" >> "$work/containers"
for _ in $(seq 1 60); do curl -fsS "http://127.0.0.1:$core_port/healthz" >/dev/null 2>&1 && break; sleep 1; done
curl -fsS "http://127.0.0.1:$core_port/healthz" >/dev/null
cat > "$work/connection.json" <<EOF
{"root":"$work","apiUrl":"http://127.0.0.1:$core_port","pythonUrl":"http://127.0.0.1:$api_port","databaseUrl":"$dsn","pgContainer":"$run-pg","apiContainer":"$run-api","coreContainer":"$run-core","image":"$image","sourceCommit":"$(git -C "$ROOT" rev-parse HEAD)","chatCandidate":true}
EOF
docker exec "$run-pg" psql -U chat_e2e -d chat_e2e_test -Atc 'show fsync; show synchronous_commit; show full_page_writes' > "$work/durability.txt"
(cd "$ROOT/services/core" && CORE_REQUIRE_POSTGRES_TESTS=1 CORE_TEST_DATABASE_URL="$dsn" \
 RUDI_TEST_ACCOUNT_URL="http://127.0.0.1:$core_port" RUDI_TEST_ACCOUNT_OUTPUT="$work" RUDI_TEST_ACCOUNT_WORLD=chat \
 MOBILE_ACCOUNT_ENCRYPTION_KEY="$account_encryption" MOBILE_ACCOUNT_LOOKUP_KEY="$account_lookup" \
 go test -tags=postgres,authfixture -count=1 -run '^TestProvisionSyntheticAccountWorld$' ./internal/accountauth) >"$work/accounts.log" 2>&1
echo "READY: $work/connection.json"
trap - ERR
