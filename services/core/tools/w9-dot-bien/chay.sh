#!/usr/bin/env bash
#
# Bảng đột biến của làn W9 (ADR-0029). Chạy từ GỐC REPO:
#
#   services/core/tools/w9-dot-bien/chay.sh [--lock <tệp>] [tên-đột-biến ...]
#
# Với mỗi đột biến trong dot-bien.json, script:
#
#   1. dựng bản sao đã sửa của tệp nguồn — thay ĐÚNG MỘT lần xuất hiện của `cu`
#      bằng `moi`; nếu neo không có, hoặc có nhiều hơn một, script DỪNG (một neo
#      trượt làm cả bảng xanh giả);
#   2. kiểm dòng bị đổi là DÒNG MÃ, không phải dòng chú thích — đột biến rơi vào
#      comment không chứng minh gì;
#   3. `go build -overlay` để chắc bản đột biến biên dịch được;
#   4. chạy tầng Postgres với overlay ấy, LUÔN giữ sentinel trong bộ lọc `-run`
#      (thiếu sentinel thì tầng từ chối cả lượt và con số không có nghĩa);
#   5. ĐẾM SỐ CA ĐỎ (`--- FAIL:`) — phán quyết đọc từ số ca, KHÔNG BAO GIỜ từ mã
#      thoát: một đột biến làm tầng chết sớm cũng cho mã thoát khác 0.
#
# Không sửa tệp nào trong cây: `-overlay` trỏ sang bản sao trong thư mục tạm.
set -euo pipefail

cd "$(dirname "$0")/../../../.."
REPO_ROOT="$PWD"
DIR="services/core/tools/w9-dot-bien"
IMAGE="${W9_PARITY_IMAGE:-mobile-parity-api:7bf58e3d}"
SENTINEL=TestPostgresTierReachesDatabase
RUN="$SENTINEL|TestAuthRepositoryOracle|TestAuthRepoBindsWhatPythonBinds|TestNamedInviteSecretSerialisesRequests"

LOCK=""
DRY=0
while :; do
  case "${1:-}" in
    --lock) LOCK="$2"; shift 2 ;;
    # --khong-chay: chỉ dựng bản đột biến, kiểm neo, kiểm dòng mã và biên dịch.
    # Không chạm tầng Postgres, nên chạy được khi khoá máy đang bận.
    --khong-chay) DRY=1; shift ;;
    *) break ;;
  esac
done
WANT=("$@")

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

names="$(python3 -c "
import json,sys
spec=json.load(open('$DIR/dot-bien.json'))
print(' '.join(m['ten'] for m in spec['dot_bien']))
")"
[ ${#WANT[@]} -eq 0 ] && read -r -a WANT <<< "$names"

for name in "${WANT[@]}"; do
  echo "=== đột biến $name"
  out="$work/$name"
  mkdir -p "$out"
  # Bước 1 + 2: dựng bản sao, assert neo, và assert dòng đổi là dòng mã.
  python3 - "$name" "$out" <<'PY'
import json, os, sys

name, out = sys.argv[1], sys.argv[2]
spec = json.load(open("services/core/tools/w9-dot-bien/dot-bien.json"))
mutant = next(m for m in spec["dot_bien"] if m["ten"] == name)
path = os.path.join("services/core", mutant["tep"])
source = open(path, encoding="utf-8").read()
hits = source.count(mutant["cu"])
if hits != 1:
    raise SystemExit(f"HỎNG: neo của {name} khớp {hits} lần trong {path}, phải đúng 1")
mutated = source.replace(mutant["cu"], mutant["moi"], 1)

# Dòng nào đổi, và dòng ấy có phải chú thích không.
before, after = source.splitlines(), mutated.splitlines()
changed = [
    (a, b)
    for a, b in zip(before, after)
    if a != b
] or [(x, y) for x, y in zip(before, after[: len(before)]) if x != y]
head = min(len(before), len(after))
first = next((i for i in range(head) if before[i] != after[i]), None)
if first is None and len(before) == len(after):
    raise SystemExit(f"HỎNG: {name} không đổi dòng nào")
line = (before[first] if first is not None else "").strip()
if line.startswith("//") or line.startswith("*") or line.startswith("/*"):
    raise SystemExit(f"HỎNG: {name} rơi vào dòng CHÚ THÍCH: {line!r}")
print(f"    dòng mã đổi: {line[:96]}")

target = os.path.join(out, os.path.basename(mutant["tep"]))
open(target, "w", encoding="utf-8").write(mutated)
overlay = {"Replace": {os.path.abspath(path): os.path.abspath(target)}}
json.dump(overlay, open(os.path.join(out, "overlay.json"), "w"), indent=2)
print("    overlay:", os.path.join(out, "overlay.json"))
PY

  # Bước 3: bản đột biến phải biên dịch được.
  ( cd services/core && CGO_ENABLED=0 GOTOOLCHAIN=local go build -overlay "$out/overlay.json" ./... ) \
    || { echo "HỎNG: $name không biên dịch"; exit 1; }

  if [ "$DRY" = 1 ]; then
    echo "    (--khong-chay: dừng sau khi biên dịch)"
    continue
  fi

  # Bước 4 + 5.
  log="$out/tier.log"
  set +e
  if [ -n "$LOCK" ]; then
    flock -w 5400 "$LOCK" scripts/go_postgres_tier.sh --image "$IMAGE" -- \
      -overlay "$out/overlay.json" -run "$RUN" ./internal/repo/ >"$log" 2>&1
  else
    scripts/go_postgres_tier.sh --image "$IMAGE" -- \
      -overlay "$out/overlay.json" -run "$RUN" ./internal/repo/ >"$log" 2>&1
  fi
  rc=$?
  set -e
  fails="$(grep -cE '^\s*--- FAIL: ' "$log" || true)"
  sentinel="$(grep -cE '^--- PASS: '"$SENTINEL"'\b' "$log" || true)"
  echo "    ca đỏ: $fails · sentinel PASS: $sentinel · (mã thoát $rc, KHÔNG dùng để phán quyết)"
  cp "$log" "${W9_LOG_DIR:-$REPO_ROOT/..}/dot-bien-$name.log" 2>/dev/null || true
done
