#!/usr/bin/env bash
# Run the Go core's Milvus tier -- the one retrieval + ingestion system on
# real services: the Milvus vector index and its one schema
# (internal/vectordb), the ingestion pipeline writing it through that schema
# (internal/rag/nap over internal/vectordb/napkho: filter parity with Go,
# the golden set, the lifecycle through the alias, attribute propagation),
# the hybrid retriever end to end over Milvus and the PostgreSQL rows the
# ingest writes (internal/hybrid), with the reranker wiring checked against a
# loopback stand-in inside the tests (no model, no paid call: ADR-0049 §4).
#
#   scripts/go_milvus_tier.sh [--image TAG] [-- go test args...]
#
# Each service comes from the environment when the caller has one, from a
# local no-Docker install when one is named, and from a disposable container
# otherwise:
#   Milvus      MOBILE_TEST_MILVUS_ADDR (+ MOBILE_TEST_MILVUS_USER, default
#               root, and MOBILE_TEST_MILVUS_PASSWORD, or
#               MOBILE_TEST_MILVUS_PASSWORD_FILE naming a file that holds it);
#               or MOBILE_MILVUS_LOCAL_DIR, a directory holding the extracted
#               v3.0.2 server's start.sh and .secret (infra bring-up);
#               or the pinned image below, auth on, embedded etcd, local
#               storage, woodpecker on local disk.
#   PostgreSQL  CORE_TEST_DATABASE_URL (at `alembic head`); or a disposable
#               postgres:16 migrated by Alembic from the API image.
#
# The ways this tier could read green while measuring nothing, all refused:
#   * no service: the tests skip. CORE_REQUIRE_MILVUS_TESTS=1 and
#     CORE_REQUIRE_POSTGRES_TESTS=1 turn those skips into failures, and any
#     SKIP line left in the log fails the tier anyway.
#   * no tests: `go test -tags milvus` passes on a package with no tagged
#     file. Every sentinel below must be seen passing.
#   * a package forgotten: the package list is every directory holding a
#     `//go:build milvus` test file, found here, and an empty list fails.
set -euo pipefail

cd "$(dirname "$0")/.."
PG_IMAGE="${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}"
# milvusdb/milvus:v3.0.2, pinned by its index digest (infra bring-up
# 2026-09-25, research milvus.md §7).
MILVUS_IMAGE="${MOBILE_TEST_MILVUS_IMAGE:-milvusdb/milvus:v3.0.2@sha256:5f13bf88e110a517911c3e6dd8172454e90042c21e606a868084615a4302c8a0}"
SENTINELS=(
  # vectordb: the service, the hard-constraint expression, alias moves,
  # compacted deletes, telemetry off, the memory store's owner filter.
  TestMilvusTierReachesMilvus
  TestKhongViPhamRangBuocCung
  TestAliasNangCapQuayLai
  TestXoaVaNenBienMat
  TestTelemetryTatTrenMayChu
  TestTriNhoChiCuaChuSoHuu
  TestMoRongCapNhatRieng
  # hybrid: fusion beats one leg, end to end with the re-check, the real reranker.
  TestHybridHonDenseChiMot
  TestHybridDauCuoiKhongViPham
  TestHybridQuaRerankThat
  TestHybridRerankTheoLuot
  # ingestion (rag/nap over napkho): the one folded BM25 field, filter parity with Go
  # on rows with unknown allergens/price/hours, the golden set with no
  # violation, the lifecycle through the alias, a takedown surviving a
  # rollback, attributes reaching a collection of another configuration.
  TestBM25GapDauMilvus
  TestKhoaVaCapNhatMotPhanMilvus
  TestLocMilvusKhopGo
  TestHybridLocCungKhongViPham
  TestBuildDoiSoatPromoteRollbackQuaAlias
  TestTombstoneKhongSongLaiSauRollback
  TestThuocTinhDenBanKhacCauHinhMilvus
)

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
  done < <(grep -rlE '^//go:build .*\bmilvus\b' services/core --include='*_test.go' |
    xargs -r -n1 dirname | sort -u)
  if [ ${#go_args[@]} -eq 0 ]; then
    echo "HỎNG: không có file test nào mang tag milvus -- tầng này không có gì để đo" >&2
    exit 1
  fi
fi

command -v go >/dev/null 2>&1 || { echo "thiếu go" >&2; exit 2; }

containers=()
log="$(mktemp)"
work="$(mktemp -d)"
cleanup() {
  for c in "${containers[@]}"; do docker rm -f "$c" >/dev/null 2>&1 || true; done
  rm -rf "$log" "$work"
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

# wait_http NAME TRIES URL: a service that never answers fails the tier.
wait_http() {
  local name="$1" tries="$2" url="$3"
  for _ in $(seq 1 "$tries"); do
    curl -fsS -m 2 "$url" >/dev/null 2>&1 && return 0
    sleep 1
  done
  echo "HỎNG: $name không lên sau ${tries}s ($url)" >&2
  exit 1
}

# --- Milvus -----------------------------------------------------------------
if [ -n "${MOBILE_TEST_MILVUS_ADDR:-}" ]; then
  echo "--- Milvus có sẵn từ MOBILE_TEST_MILVUS_ADDR"
  if [ -z "${MOBILE_TEST_MILVUS_PASSWORD:-}" ] && [ -n "${MOBILE_TEST_MILVUS_PASSWORD_FILE:-}" ]; then
    [ -r "$MOBILE_TEST_MILVUS_PASSWORD_FILE" ] || { echo "HỎNG: không đọc được MOBILE_TEST_MILVUS_PASSWORD_FILE" >&2; exit 2; }
    MOBILE_TEST_MILVUS_PASSWORD="$(tr -d '\n' < "$MOBILE_TEST_MILVUS_PASSWORD_FILE")"
    export MOBILE_TEST_MILVUS_PASSWORD
  fi
  [ -n "${MOBILE_TEST_MILVUS_PASSWORD:-}" ] || { echo "HỎNG: Milvus bật auth: cần MOBILE_TEST_MILVUS_PASSWORD hoặc MOBILE_TEST_MILVUS_PASSWORD_FILE" >&2; exit 2; }
elif [ -n "${MOBILE_MILVUS_LOCAL_DIR:-}" ]; then
  d="$MOBILE_MILVUS_LOCAL_DIR"
  [ -x "$d/start.sh" ] || { echo "HỎNG: $d/start.sh không chạy được" >&2; exit 2; }
  echo "--- Milvus cài tại chỗ (không Docker) từ $d"
  "$d/start.sh"
  export MOBILE_TEST_MILVUS_ADDR="127.0.0.1:${MILVUS_GRPC_PORT:-19530}"
  export MOBILE_TEST_MILVUS_PASSWORD="$(tr -d '\n' < "$d/.secret")"
  wait_http milvus 120 "http://127.0.0.1:${MILVUS_HTTP_PORT:-9091}/healthz"
else
  need_docker MOBILE_TEST_MILVUS_ADDR
  name="go-milvus-$$" port="$(free_port)" hport="$(free_port)"
  pw="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  mkdir -p "$work/conf"
  (umask 077; cat >"$work/conf/user.yaml" <<YAML
etcd:
  use:
    embed: true
  data:
    dir: /var/lib/milvus/etcd
  config:
    path: /milvus/configs/embedEtcd.yaml
common:
  storageType: local
  security:
    authorizationEnabled: true
    defaultRootPassword: "$pw"
mq:
  type: woodpecker
woodpecker:
  storage:
    type: local
YAML
  )
  cat >"$work/conf/embedEtcd.yaml" <<YAML
listen-client-urls: http://0.0.0.0:2379
advertise-client-urls: http://0.0.0.0:2379
quota-backend-bytes: $((4 << 30))
auto-compaction-mode: revision
auto-compaction-retention: '1000'
YAML
  chmod 644 "$work/conf/user.yaml"
  echo "--- Milvus dùng một lần trên 127.0.0.1:$port ($MILVUS_IMAGE)"
  docker run -d --rm --name "$name" \
    -e ETCD_USE_EMBED=true -e ETCD_DATA_DIR=/var/lib/milvus/etcd -e ETCD_CONFIG_PATH=/milvus/configs/embedEtcd.yaml \
    -e COMMON_STORAGETYPE=local -e DEPLOY_MODE=STANDALONE \
    -v "$work/conf/user.yaml:/milvus/configs/user.yaml:ro" \
    -v "$work/conf/embedEtcd.yaml:/milvus/configs/embedEtcd.yaml:ro" \
    -p "127.0.0.1:$port:19530" -p "127.0.0.1:$hport:9091" \
    "$MILVUS_IMAGE" milvus run standalone >/dev/null
  containers+=("$name")
  wait_http milvus 180 "http://127.0.0.1:$hport/healthz"
  export MOBILE_TEST_MILVUS_ADDR="127.0.0.1:$port" MOBILE_TEST_MILVUS_PASSWORD="$pw"
fi
export MOBILE_TEST_MILVUS_USER="${MOBILE_TEST_MILVUS_USER:-root}"

# --- PostgreSQL ---------------------------------------------------------------
if [ -z "${CORE_TEST_DATABASE_URL:-}" ]; then
  need_docker CORE_TEST_DATABASE_URL
  name="go-milvus-pg-$$" port="$(free_port)" password="$(secret)"
  echo "--- PostgreSQL dùng một lần trên 127.0.0.1:$port"
  docker run -d --rm --name "$name" \
    -e POSTGRES_DB=mobile -e POSTGRES_USER=mobile -e POSTGRES_PASSWORD="$password" \
    -p "127.0.0.1:$port:5432" "$PG_IMAGE" -c timezone=UTC -c fsync=off >/dev/null
  containers+=("$name")
  for _ in $(seq 1 60); do
    docker exec "$name" pg_isready -h 127.0.0.1 -U mobile -d mobile >/dev/null 2>&1 && break
    sleep 1
  done
  export CORE_TEST_DATABASE_URL="postgresql://mobile:$password@127.0.0.1:$port/mobile"
  if [ -z "$image" ]; then
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

echo "--- go test -tags milvus ${go_args[*]}"
set +e
(
  cd services/core &&
    CORE_REQUIRE_MILVUS_TESTS=1 \
    CORE_REQUIRE_POSTGRES_TESTS=1 \
    go test -tags milvus -count=1 -p 1 -timeout 45m -v "${go_args[@]}"
) 2>&1 | tee "$log"
rc=${PIPESTATUS[0]}
set -e

if [ "$rc" -ne 0 ]; then
  echo "HỎNG: go test -tags milvus thoát $rc" >&2
  exit "$rc"
fi
for sentinel in "${SENTINELS[@]}"; do
  if ! grep -qE "^--- PASS: $sentinel\b" "$log"; then
    echo "HỎNG: không thấy $sentinel PASS -- tầng này không đo được điều nó hứa" >&2
    exit 1
  fi
done
if grep -qE '^\s*--- SKIP: ' "$log"; then
  echo "HỎNG: có ca bị bỏ qua trong tầng milvus -- bỏ qua không phải là xanh:" >&2
  grep -E '^\s*--- SKIP: ' "$log" >&2
  exit 1
fi
passed="$(grep -cE '^\s*--- PASS: ' "$log" || true)"
echo "tầng Milvus (truy hồi + nạp + reranker) của Go: $passed ca PASS trên ${#go_args[@]} gói, ${#SENTINELS[@]} sentinel có mặt"
