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
cleanup() {
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
echo "Bắt đầu mô phỏng thật; artifact: $scratch" >&2
/usr/bin/time -v -o "$scratch/resources.txt" "$scratch/chat-load" -lab-binary "$scratch/chat-lab" "$@" > >(tee "$scratch/result.json") 2> >(tee "$scratch/progress.log" >&2)
