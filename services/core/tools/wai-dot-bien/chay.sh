#!/usr/bin/env bash
#
# Bảng đột biến của làn WAI domain (ADR-0029 T2). Chạy từ GỐC REPO:
#
#   services/core/tools/wai-dot-bien/chay.sh [tên-đột-biến ...]
#
# Với mỗi đột biến: neo đúng một lần, dòng đổi là dòng mã, `go test -overlay`
# trên đúng package, ĐẾM SỐ CA ĐỎ (`--- FAIL:`). Phán quyết đọc từ số ca,
# không từ mã thoát.
set -euo pipefail

cd "$(dirname "$0")/../../../.."
REPO_ROOT="$PWD"
DIR="services/core/tools/wai-dot-bien"

names="$(python3 -c "
import json
spec=json.load(open('$DIR/dot-bien.json'))
print(' '.join(m['ten'] for m in spec['dot_bien']))
")"
WANT=("$@")
[ ${#WANT[@]} -eq 0 ] && read -r -a WANT <<< "$names"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

for name in "${WANT[@]}"; do
  echo "=== đột biến $name"
  out="$work/$name"
  mkdir -p "$out"
  pkg="$(python3 - "$name" "$out" <<'PY'
import json, os, sys

name, out = sys.argv[1], sys.argv[2]
spec = json.load(open("services/core/tools/wai-dot-bien/dot-bien.json"))
mutant = next(m for m in spec["dot_bien"] if m["ten"] == name)
path = os.path.join("services/core", mutant["tep"])
source = open(path, encoding="utf-8").read()
hits = source.count(mutant["cu"])
if hits != 1:
    raise SystemExit(f"HỎNG: neo của {name} khớp {hits} lần trong {path}, phải đúng 1")
mutated = source.replace(mutant["cu"], mutant["moi"], 1)
before, after = source.splitlines(), mutated.splitlines()
head = min(len(before), len(after))
first = next((i for i in range(head) if before[i] != after[i]), None)
if first is None:
    raise SystemExit(f"HỎNG: {name} không đổi dòng nào")
line = before[first].strip()
if line.startswith("//") or line.startswith("*") or line.startswith("/*"):
    raise SystemExit(f"HỎNG: {name} rơi vào dòng CHÚ THÍCH: {line!r}")
print(f"    dòng mã đổi: {line[:96]}", file=sys.stderr)
target = os.path.join(out, os.path.basename(mutant["tep"]))
open(target, "w", encoding="utf-8").write(mutated)
overlay = {"Replace": {os.path.abspath(path): os.path.abspath(target)}}
json.dump(overlay, open(os.path.join(out, "overlay.json"), "w"), indent=2)
print("./" + os.path.dirname(mutant["tep"]) + "/")
PY
)"
  log="$out/test.log"
  set +e
  ( cd services/core && CGO_ENABLED=0 GOTOOLCHAIN=local go test -count=1 -overlay "$out/overlay.json" "$pkg" ) >"$log" 2>&1
  rc=$?
  set -e
  fails="$(grep -cE '^\s*--- FAIL: ' "$log" || true)"
  echo "    ca đỏ: $fails · (mã thoát $rc, KHÔNG dùng để phán quyết)"
  if [ "$fails" -lt 1 ]; then
    echo "HỎNG: $name không làm đỏ ca nào"
    tail -40 "$log"
    exit 1
  fi
done
