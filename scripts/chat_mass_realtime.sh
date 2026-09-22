#!/usr/bin/env bash
# Drive actual chat-lab processes against a private disposable PostgreSQL.
set -euo pipefail
cd "$(dirname "$0")/.."
# Only the tools this script actually calls. `rg` was listed here and never
# used, which blocked every non-interactive runner: it is a shell function on
# this machine, not an installed binary.
for tool in docker go; do command -v "$tool" >/dev/null || { echo "Thiếu $tool" >&2; exit 2; }; done
scratch="$(mktemp -d /tmp/rudi-chat-mass.XXXXXXXX)"
{
  git rev-parse HEAD
  sha256sum services/core/internal/chatv2/store.go services/core/internal/chatv2http/handler.go services/core/cmd/chat-load/*.go
  go version
  uname -sr
  nproc
  free -m
} >"$scratch/provenance.txt"
container=""
sampler=""
cleanup() {
  if [ -n "$sampler" ]; then kill "$sampler" >/dev/null 2>&1 || true; fi
  if [ -n "$container" ]; then docker rm -f "$container" >/dev/null 2>&1 || true; fi
  echo "Bằng chứng tổng hợp: $scratch" >&2
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
# Source and migrations come from this checkout. No shared service is used.
docker build -q services/api >"$scratch/image"
api_image="$(tail -1 "$scratch/image")"
container="$(docker run -d --rm -e POSTGRES_DB=chat_mass_test -e POSTGRES_USER=chat_test -e POSTGRES_PASSWORD=synthetic-chat-test-only -p 127.0.0.1::5432 postgres:16-alpine)"
port="$(docker port "$container" 5432/tcp)"; port="${port##*:}"
ready=false
for ((attempt=0; attempt<60; attempt++)); do
 if docker exec "$container" pg_isready -U chat_test -d chat_mass_test >/dev/null 2>&1; then ready=true; break; fi
 sleep 1
done
if [ "$ready" != true ]; then echo 'PostgreSQL không sẵn sàng' >&2; exit 1; fi
docker run --rm --network host -e MOBILE_DATABASE_URL="postgresql+psycopg://chat_test:synthetic-chat-test-only@127.0.0.1:$port/chat_mass_test" "$api_image" alembic upgrade head >"$scratch/migration.log" 2>&1
(cd services/core && go build -o "$scratch/chat-lab" ./cmd/chat-lab && go build -o "$scratch/chat-load" ./cmd/chat-load)
export RUDI_CHAT_LOAD_DATABASE_URL="postgresql://chat_test:synthetic-chat-test-only@127.0.0.1:$port/chat_mass_test"
# Lock sampling is opt-in because it costs one psql round trip per tick. The
# 30-minute baseline counted waits by `mode` alone, which cannot tell a queue on
# the conversation counter from a queue anywhere else: both look like
# `Lock/transactionid`. These three queries carry the relation name, the page
# and the tuple, so the answer is readable instead of inferred.
if [ -n "${RUDI_CHAT_LOAD_LOCK_SAMPLE:-}" ]; then
 docker exec -e TICK="${RUDI_CHAT_LOAD_LOCK_TICK:-0.25}" "$container" sh -c '
  while :; do
   psql -At -U chat_test -d chat_mass_test \
    -c "SELECT to_char(clock_timestamp(),'"'"'HH24:MI:SS.MS'"'"')||'"'"' active='"'"'||(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='"'"'active'"'"')||'"'"' idletx='"'"'||(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='"'"'idle in transaction'"'"')||'"'"' waiting='"'"'||(SELECT count(*) FROM pg_locks WHERE NOT granted)" \
    -c "SELECT '"'"'  WAIT '"'"'||locktype||'"'"' '"'"'||mode||'"'"' rel='"'"'||rel||'"'"' page='"'"'||pg||'"'"' tuple='"'"'||tp||'"'"' waiters='"'"'||n FROM (SELECT l.locktype,l.mode,coalesce(c.relname,'"'"'-'"'"') AS rel,coalesce(l.page::text,'"'"'-'"'"') AS pg,coalesce(l.tuple::text,'"'"'-'"'"') AS tp,count(*)::text AS n FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid LEFT JOIN pg_class c ON c.oid=l.relation WHERE NOT l.granted AND a.datname=current_database() GROUP BY 1,2,3,4,5) s" \
    -c "SELECT '"'"'  HOLDER '"'"'||b.state||'"'"' '"'"'||coalesce(b.wait_event_type,'"'"'-'"'"')||'"'"'/'"'"'||coalesce(b.wait_event,'"'"'-'"'"')||'"'"' q='"'"'||left(regexp_replace(b.query,'"'"'\s+'"'"','"'"' '"'"','"'"'g'"'"'),120) FROM pg_stat_activity b WHERE b.pid IN (SELECT unnest(pg_blocking_pids(w.pid)) FROM pg_stat_activity w WHERE w.datname=current_database())" \ \
    -c "SELECT '"'"'  CKPT '"'"'||checkpoints_timed||'"'"' timed '"'"'||checkpoints_req||'"'"' requested write_ms='"'"'||checkpoint_write_time||'"'"' sync_ms='"'"'||checkpoint_sync_time||'"'"' buffers='"'"'||buffers_checkpoint FROM pg_stat_bgwriter" \
    -c "SELECT '"'"'  WAL '"'"'||wal_records||'"'"' rec '"'"'||wal_fpi||'"'"' fpi bytes='"'"'||wal_bytes||'"'"' sync='"'"'||wal_sync||'"'"' syncms='"'"'||wal_sync_time FROM pg_stat_wal"
    2>/dev/null
   sleep "$TICK"
  done' >"$scratch/pg-locks.txt" 2>&1 &
 sampler=$!
 echo "Lấy mẫu khoá vào $scratch/pg-locks.txt" >&2
fi
echo "Bắt đầu mô phỏng thật; artifact: $scratch" >&2
/usr/bin/time -v -o "$scratch/resources.txt" "$scratch/chat-load" -lab-binary "$scratch/chat-lab" "$@" > >(tee "$scratch/result.json") 2> >(tee "$scratch/progress.log" >&2)
