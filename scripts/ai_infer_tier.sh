#!/usr/bin/env bash
# The inference sidecar's test tiers (services/ai-infer, ADR-0031).
#
#   scripts/ai_infer_tier.sh            offline: pure fakes + a loopback Gemini fake
#   scripts/ai_infer_tier.sh --milvus   live: a real Milvus (AI_INFER_TEST_MILVUS_URI,
#                                       AI_INFER_TEST_MILVUS_TOKEN), or a disposable
#                                       digest-pinned container when docker is there
#
# Python: $AI_INFER_PYTHON, else services/ai-infer/.venv/bin/python, else
# python3 -- whichever it is must have requirements-dev.txt installed.
#
# What green means (offline): the sparse wire contract and its stub hold; mem0
# builds with no PostHog client and no SQLite connection even with
# MEM0_TELEMETRY=true in the environment (and the canaries show both checks
# can see what they forbid); memory text never reaches a log; one person never
# reads, lists or deletes another's memories; every delete is followed by a
# count of zero; the Gemini request body carries the task-in-text format, 1536
# dims, one content per text, no taskType. What it does NOT mean: that the
# model extracts well (no real model call happens anywhere here), or that
# MILCO works (its weights are not here and not licensed yet).
#
# Refused ways to read green while testing nothing:
#   * a skipped test fails the tier (the live half fails at collection when
#     its Milvus is missing: AI_INFER_REQUIRE_MILVUS=1);
#   * each sentinel must be seen PASSED by name;
#   * a real network is unreachable: every proxy variable points at a closed
#     loopback port and no GEMINI_API_KEY is passed.
set -euo pipefail

cd "$(dirname "$0")/.."
SVC=services/ai-infer
MODE=offline
[ "${1:-}" = "--milvus" ] && MODE=milvus

PY="${AI_INFER_PYTHON:-}"
if [ -z "$PY" ]; then
  if [ -x "$SVC/.venv/bin/python" ]; then PY="$SVC/.venv/bin/python"; else PY=python3; fi
fi
command -v "$PY" >/dev/null 2>&1 || { echo "thiếu python ($PY)" >&2; exit 2; }
if ! "$PY" -c "import fastapi, mem0, pymilvus, google.genai, pytest" >/dev/null 2>&1; then
  echo "thiếu phụ thuộc: $PY chưa cài $SVC/requirements-dev.txt (đặt AI_INFER_PYTHON)" >&2
  exit 2
fi
[ -d "$SVC/tests" ] || { echo "HỎNG: không có $SVC/tests" >&2; exit 1; }

work="$(mktemp -d)"
MILVUS_CTR=""
cleanup() {
  [ -n "$MILVUS_CTR" ] && docker rm -f "$MILVUS_CTR" >/dev/null 2>&1
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

# A disposable Milvus when none is handed over: the pinned v3.0.2 image,
# standalone, embedded etcd, local storage, woodpecker WAL on local disk set
# explicitly (the image default is minio), auth on with a random root
# password, published on loopback only. NOT exercised on the machine that
# wrote it (no Docker there); the local run used a Milvus handed over by URI.
MILVUS_IMAGE="milvusdb/milvus:v3.0.2@sha256:5f13bf88e110a517911c3e6dd8172454e90042c21e606a868084615a4302c8a0"
start_milvus() {
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 ||
    { echo "HỎNG: --milvus cần AI_INFER_TEST_MILVUS_URI hoặc docker" >&2; exit 1; }
  local pw port health
  pw="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
  mkdir -p "$work/milvus"
  cat >"$work/milvus/embedEtcd.yaml" <<'YAML'
listen-client-urls: http://0.0.0.0:2379
advertise-client-urls: http://0.0.0.0:2379
auto-compaction-mode: revision
auto-compaction-retention: '1000'
YAML
  (umask 077; cat >"$work/milvus/user.yaml" <<YAML
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
  chmod 644 "$work/milvus/user.yaml" "$work/milvus/embedEtcd.yaml"
  MILVUS_CTR="ai-infer-milvus-$$"
  docker run -d --name "$MILVUS_CTR" --security-opt seccomp:unconfined \
    -e ETCD_USE_EMBED=true -e ETCD_DATA_DIR=/var/lib/milvus/etcd \
    -e ETCD_CONFIG_PATH=/milvus/configs/embedEtcd.yaml -e COMMON_STORAGETYPE=local -e DEPLOY_MODE=STANDALONE \
    -v "$work/milvus/embedEtcd.yaml:/milvus/configs/embedEtcd.yaml:ro" \
    -v "$work/milvus/user.yaml:/milvus/configs/user.yaml:ro" \
    -p 127.0.0.1::19530 -p 127.0.0.1::9091 "$MILVUS_IMAGE" milvus run standalone >/dev/null
  port="$(docker port "$MILVUS_CTR" 19530/tcp | head -1 | sed 's/.*://')"
  health="$(docker port "$MILVUS_CTR" 9091/tcp | head -1 | sed 's/.*://')"
  for _ in $(seq 1 120); do
    curl -fsS "http://127.0.0.1:$health/healthz" >/dev/null 2>&1 && break
    sleep 1
  done
  curl -fsS "http://127.0.0.1:$health/healthz" >/dev/null 2>&1 ||
    { echo "HỎNG: Milvus không healthy sau 120 s" >&2; docker logs --tail 50 "$MILVUS_CTR" >&2; exit 1; }
  export AI_INFER_TEST_MILVUS_URI="http://127.0.0.1:$port"
  export AI_INFER_TEST_MILVUS_TOKEN="root:$pw"
  echo "Milvus tạm $MILVUS_CTR trên 127.0.0.1:$port"
}

if [ "$MODE" = milvus ]; then
  [ -n "${AI_INFER_TEST_MILVUS_URI:-}" ] || start_milvus
  SENTINELS=(test_live_memory_lifecycle test_live_owner_value_is_a_parameter_not_code test_live_analyzer_folds_vietnamese)
  MARK="milvus"
else
  SENTINELS=(test_posthog_never_constructed test_posthog_canary_is_seen test_no_sqlite_ever test_sqlite_canary_is_seen
    test_memory_text_never_logged test_log_canary_is_seen test_cross_user_isolation test_purge_leaves_zero_rows
    test_delete_all_leaves_zero_rows test_delete_all_repeats_until_zero test_sparse_stub_deterministic test_embed_wire_contract
    test_sparse_path_never_loads_memory test_boundary_canary_is_seen test_embedding_is_the_owner_decision_and_search_has_no_hyde)
  MARK="not milvus"
fi

echo "--- pytest $SVC ($MODE) với $PY"
set +e
(
  cd "$SVC" &&
    env -u GEMINI_API_KEY -u GOOGLE_API_KEY \
      HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 \
      https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 all_proxy=http://127.0.0.1:9 \
      NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost \
      AI_INFER_REQUIRE_MILVUS="$([ "$MODE" = milvus ] && echo 1 || echo 0)" \
      "$PY" -m pytest -p no:cacheprovider -m "$MARK" -rA -s -q
) >"$work/test.log" 2>&1
rc=$?
set -e
cat "$work/test.log"
if [ "$rc" -ne 0 ]; then
  echo "HỎNG: pytest thoát $rc" >&2
  exit "$rc"
fi
if grep -qE '^SKIPPED|^XFAIL|^XPASS' "$work/test.log"; then
  echo "HỎNG: có ca bị bỏ qua hoặc xfail -- bỏ qua không phải là xanh:" >&2
  grep -E '^SKIPPED|^XFAIL|^XPASS' "$work/test.log" >&2
  exit 1
fi
for s in "${SENTINELS[@]}"; do
  if ! grep -qE "^PASSED tests/[a-z_]+\.py::$s\$" "$work/test.log"; then
    echo "HỎNG: không thấy $s PASS" >&2
    exit 1
  fi
done
n=$(grep -cE '^PASSED ' "$work/test.log" || true)
echo "ai-infer ($MODE): $n ca PASS, ${#SENTINELS[@]}/${#SENTINELS[@]} sentinel, 0 SKIP"
