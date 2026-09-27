#!/usr/bin/env bash
# Reproduce community HTTP/WS checks with two Go replicas and disposable PostgreSQL.
# No inherited application environment, production database, or model is used.
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

for tool in docker go node curl; do
  command -v "$tool" >/dev/null || { echo "Thiếu $tool" >&2; exit 2; }
done
node -e 'if (typeof WebSocket !== "function") throw new Error("Node 22+ required")'

work="$(mktemp -d /tmp/rudi-community-e2e.XXXXXX)"
run="community-e2e-$(date +%s)-$$"
image="rudi-community-e2e-api:$run"
pids=()
cleanup() {
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
  docker rm -f "$run-pg" >/dev/null 2>&1 || true
  docker image rm "$image" >/dev/null 2>&1 || true
  printf 'Log và fixture tổng hợp: %s\n' "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Hold all chosen sockets until the entire distinct port set has been allocated.
read -r pg_port api_one live_one api_two live_two < <(node --input-type=module -e '
  import net from "node:net";
  const servers = await Promise.all(Array.from({length: 5}, () => new Promise(resolve => {
    const server = net.createServer();
    server.listen(0, "127.0.0.1", () => resolve(server));
  })));
  console.log(servers.map(s => s.address().port).join(" "));
  servers.forEach(s => s.close());
')
password="$(node -e 'process.stdout.write(require("node:crypto").randomBytes(24).toString("hex"))')"
dsn="postgresql://community:$password@127.0.0.1:$pg_port/community_synthetic_e2e"

echo "Dựng migration image từ cây hiện tại."
docker build -q -t "$image" services/api >"$work/build.log" 2>&1
docker run -d --rm --name "$run-pg" \
  -e POSTGRES_DB=community_synthetic_e2e -e POSTGRES_USER=community \
  -e POSTGRES_PASSWORD="$password" -p "127.0.0.1:$pg_port:5432" \
  postgres:16-alpine -c timezone=UTC >/dev/null
for _ in {1..60}; do
  docker exec "$run-pg" pg_isready -U community -d community_synthetic_e2e >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$run-pg" pg_isready -U community -d community_synthetic_e2e >/dev/null
docker run --rm --network host \
  -e MOBILE_DATABASE_URL="postgresql+psycopg://community:$password@127.0.0.1:$pg_port/community_synthetic_e2e" \
  "$image" alembic upgrade head >"$work/migrate.log" 2>&1

go -C services/core build -o "$work/core" ./cmd/core
go -C services/core build -o "$work/fixture" ./cmd/community-fixture
env -i PATH="$PATH" MOBILE_DATABASE_URL="$dsn" \
  "$work/fixture" -actors 4 -out "$work/actors.json"
mkdir "$work/media"
start_replica() {
  env -i PATH="$PATH" MOBILE_DATABASE_URL="$dsn" \
    MOBILE_AUTH_MODE=prod MOBILE_COMMUNITY_ENABLED=1 MOBILE_CHAT_CHANGES_CANDIDATE=0 \
    MOBILE_PYTHON_UPSTREAM=http://127.0.0.1:1 MOBILE_MEDIA_ROOT="$work/media" \
    MOBILE_CORE_LISTEN="127.0.0.1:$1" MOBILE_CORE_LIVENESS_LISTEN="127.0.0.1:$2" \
    "$work/core" serve >"$work/core-$1.log" 2>&1 &
  pids+=("$!")
}
start_replica "$api_one" "$live_one"
start_replica "$api_two" "$live_two"
for port in "$api_one" "$api_two"; do
  ready=0
  for _ in {1..60}; do
    # This native endpoint returns 401 without a session. A generic health page
    # could pass while the feature is disabled or served by another process.
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 "http://127.0.0.1:$port/v2/community/preferences" || true)"
    if [ "$code" = 401 ]; then ready=1; break; fi
    sleep 1
  done
  [ "$ready" = 1 ] || { echo "Replica $port chưa phục vụ community" >&2; exit 1; }
done

node scripts/community_e2e.mjs "http://127.0.0.1:$api_one" \
  "$work/actors.json" "http://127.0.0.1:$api_two" | tee "$work/result.json"
node -e '
  const r = require(process.argv[1]);
  if (r.result !== "PASS" || !r.real_http || !r.real_websocket || !r.distinct_replica || !r.publication_realtime || !r.comment_realtime || r.requests < 33) process.exit(1);
' "$work/result.json"
echo "E2E cộng đồng PASS: HTTP thật, hai replica WebSocket, không có SKIP. Chưa đo model/native/tải."
