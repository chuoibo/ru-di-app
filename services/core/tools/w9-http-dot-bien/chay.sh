#!/usr/bin/env bash
#
# Bảng đột biến của làn HTTP W9 (auth/sessions). Chạy từ GỐC REPO:
#
#   services/core/tools/w9-http-dot-bien/chay.sh [--khong-chay] [tên-đột-biến ...]
#
# Overlay phải rơi vào dòng mã của handler mới. Phán quyết = số ca đỏ
# (`--- FAIL:`) đối chiếu `cho_doi`, không = mã thoát.
# `do` → ≥1 ca đỏ; `song-sot` / `song-sot-oracle` → 0 ca đỏ.
# Overlay không biên dịch hoặc log rỗng thì không ra phán quyết.
set -euo pipefail

cd "$(dirname "$0")/../../../.."
DIR="services/core/tools/w9-http-dot-bien"

DRY=0
while :; do
  case "${1:-}" in
    --khong-chay) DRY=1; shift ;;
    *) break ;;
  esac
done

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
spec = json.load(open("services/core/tools/w9-http-dot-bien/dot-bien.json"))
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
want = mutant.get("cho_doi")
if not want:
    raise SystemExit(f"HỎNG: {name} thiếu cho_doi")
open(os.path.join(out, "cho_doi"), "w", encoding="utf-8").write(want)
target = os.path.join(out, os.path.basename(mutant["tep"]))
open(target, "w", encoding="utf-8").write(mutated)
overlay = {"Replace": {os.path.abspath(path): os.path.abspath(target)}}
json.dump(overlay, open(os.path.join(out, "overlay.json"), "w"), indent=2)
print("./" + os.path.dirname(mutant["tep"]) + "/")
PY
)"
  if [ "$DRY" = 1 ]; then
    ( cd services/core && CGO_ENABLED=0 GOTOOLCHAIN=local go test -c -o /dev/null -overlay "$out/overlay.json" "$pkg" ) \
      || { echo "HỎNG: $name không biên dịch"; exit 1; }
    echo "    (--khong-chay: dừng sau khi biên dịch) cho_doi=$(cat "$out/cho_doi")"
    continue
  fi
  log="$out/test.log"
  set +e
  ( cd services/core && CGO_ENABLED=0 GOTOOLCHAIN=local go test -count=1 -overlay "$out/overlay.json" "$pkg" ) >"$log" 2>&1
  rc=$?
  set -e
  fails="$(grep -cE '^\s*--- FAIL: ' "$log" || true)"
  want="$(cat "$out/cho_doi")"
  echo "    ca đỏ: $fails · cho_doi=$want · (mã thoát $rc, KHÔNG dùng để phán quyết)"
  if [ ! -s "$log" ]; then
    echo "HỎNG: $name log rỗng — không ra phán quyết"
    exit 1
  fi
  if ! grep -qE 'ok\s|FAIL\s|\?' "$log"; then
    echo "HỎNG: $name overlay có thể không biên dịch"
    tail -40 "$log"
    exit 1
  fi
  case "$want" in
    do)
      if [ "$fails" -lt 1 ]; then
        echo "HỎNG: $name cho_doi=do nhưng 0 ca đỏ"
        tail -40 "$log"
        exit 1
      fi
      ;;
    song-sot|song-sot-oracle)
      if [ "$fails" -ne 0 ]; then
        echo "HỎNG: $name cho_doi=$want nhưng đỏ $fails ca"
        grep -E '^\s*--- FAIL: ' "$log" || true
        exit 1
      fi
      ;;
    *)
      echo "HỎNG: $name cho_doi=$want không phải do|song-sot|song-sot-oracle"
      exit 1
      ;;
  esac
done
