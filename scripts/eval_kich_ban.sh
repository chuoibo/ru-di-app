#!/usr/bin/env bash
# Tier T1 of the AI eval (design 06 §6): the engine run through its own seam,
# aiharness.Engine.Run, on the scripted stub, over the hand-written Nếp corpus.
#
#   scripts/eval_kich_ban.sh
#
# What green means: the pipeline holds -- every invariant of design 06 §6.1
# that applies to Nếp at S1 on every request and every Sink event, every
# script expectation met, every wrong script red at the check it names, the
# canary case red exactly where predicted and the identity case green, the same
# bytes on a second run, and not one connection out of the process with a key
# in the environment. What it does NOT mean: that answers are any good. The
# answers are the script author's words; quality is T3, real calls, slice 9.
#
# The ways this tier could read green while measuring nothing, all refused:
#   * the eval kit in the production binary: `go list -deps ./cmd/core` must
#     not name internal/aieval, and the same listing of the eval binary must
#     (the canary that the listing can see it at all);
#   * a test that skips: any SKIP line in the go test log fails the tier, and
#     both sentinels must be seen passing;
#   * a corpus that lost its canary or its identity case, or a canary that went
#     green, or went red somewhere else: the verdict line is read here, not
#     taken from the binary's exit code alone, and the canary's predicted check
#     is pinned below;
#   * a run that is not repeatable: the corpus runs twice and the two reports
#     must be identical byte for byte;
#   * a real model: GEMINI_API_KEY is set to a dummy and every proxy variable
#     points at a closed loopback port, so a client built by mistake has
#     nowhere to go; TestKichBanKhongMoKetNoi counts requests in-process.
set -euo pipefail

cd "$(dirname "$0")/.."
CORE=services/core
BO=internal/aieval/testdata/corpus/nep-kich-ban.json
AIEVAL=mobile/services/core/internal/aieval
SENTINELS=(TestBoNepKichBan TestKichBanKhongMoKetNoi)
CANARY_CHECK=khong_bia_dia_diem

command -v go >/dev/null 2>&1 || { echo "thiếu go" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "thiếu python3" >&2; exit 2; }
[ -f "$CORE/$BO" ] || { echo "HỎNG: không có corpus $CORE/$BO" >&2; exit 1; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM

echo "--- cmd/core không liên kết internal/aieval"
( cd "$CORE" && go list -deps ./cmd/core ) >"$work/deps-core"
( cd "$CORE" && go list -deps -tags eval ./cmd/rudi-eval ) >"$work/deps-eval"
if grep -qx "$AIEVAL" "$work/deps-core"; then
  echo "HỎNG: go list -deps ./cmd/core có $AIEVAL -- bộ đo lọt vào binary production" >&2
  exit 1
fi
if ! grep -qx "$AIEVAL" "$work/deps-eval"; then
  echo "HỎNG: go list -deps -tags eval ./cmd/rudi-eval không có $AIEVAL -- phép kiểm trên không thấy được gì" >&2
  exit 1
fi
echo "cmd/core: $(wc -l <"$work/deps-core") gói, không có aieval; cmd/rudi-eval: có aieval"

echo "--- go vet -tags eval, go test -tags eval (aieval, cmd/rudi-eval)"
( cd "$CORE" && go vet -tags eval ./internal/aieval/... ./cmd/rudi-eval )
set +e
( cd "$CORE" && go test -tags eval -count=1 -v ./internal/aieval/... ./cmd/rudi-eval ) >"$work/test.log" 2>&1
rc=$?
set -e
cat "$work/test.log"
if [ "$rc" -ne 0 ]; then
  echo "HỎNG: go test -tags eval thoát $rc" >&2
  exit "$rc"
fi
if grep -qE '^\s*--- SKIP: ' "$work/test.log"; then
  echo "HỎNG: có ca bị bỏ qua -- bỏ qua không phải là xanh:" >&2
  grep -E '^\s*--- SKIP: ' "$work/test.log" >&2
  exit 1
fi
for sentinel in "${SENTINELS[@]}"; do
  if ! grep -qE "^--- PASS: $sentinel\b" "$work/test.log"; then
    echo "HỎNG: không thấy $sentinel PASS" >&2
    exit 1
  fi
done

echo "--- rudi-eval --mo-hinh kich-ban trên $BO (hai lần)"
( cd "$CORE" && go build -tags eval -o "$work/rudi-eval" ./cmd/rudi-eval )
chay_bo() {
  (
    cd "$CORE" &&
      env GEMINI_API_KEY=khoa-gia-t1-khong-duoc-dung \
        HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 \
        https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 all_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy= \
        "$work/rudi-eval" --mo-hinh kich-ban --bo "$BO"
  )
}
set +e
chay_bo >"$work/lan1.jsonl" 2>"$work/lan1.err"
rc=$?
set -e
cat "$work/lan1.err"
if [ "$rc" -ne 0 ]; then
  echo "HỎNG: rudi-eval thoát $rc" >&2
  exit 1
fi
chay_bo >"$work/lan2.jsonl" 2>/dev/null
if ! cmp -s "$work/lan1.jsonl" "$work/lan2.jsonl"; then
  echo "HỎNG: hai lần chạy cùng corpus ra hai báo cáo khác nhau" >&2
  diff "$work/lan1.jsonl" "$work/lan2.jsonl" | head -20 >&2
  exit 1
fi

# The verdict, read from the report itself.
python3 - "$work/lan1.jsonl" "$CANARY_CHECK" <<'PY'
import json, sys
path, canary_check = sys.argv[1], sys.argv[2]
lines = [json.loads(l) for l in open(path, encoding="utf-8") if l.strip()]
if not lines or "tong_ket" not in lines[-1]:
    sys.exit("HỎNG: báo cáo không kết thúc bằng dòng tong_ket")
tk, runs = lines[-1]["tong_ket"], lines[:-1]
bad = []
if len(runs) != tk["so_luot"] or tk["so_luot"] == 0:
    bad.append(f"{len(runs)} dòng chạy, tong_ket nói {tk['so_luot']}")
not_ok = [f"{r['case_id']} [{r['vai']}, {r['kich_ban']}]" for r in runs if not r["dat"]]
if not_ok:
    bad.append("lượt không đạt: " + ", ".join(not_ok))
c, d = tk["canary"], tk["dong_nhat"]
if c["case_id"] != "00-canary-phai-do" or not c["co_mat"]:
    bad.append("thiếu ca canary 00-canary-phai-do")
elif not c["dat"] or c["truot"] != [canary_check]:
    bad.append(f"canary phải đỏ đúng ở {canary_check}, đỏ ở {c['truot']}")
if d["case_id"] != "00-dong-nhat" or not d["co_mat"]:
    bad.append("thiếu ca đồng nhất 00-dong-nhat")
elif not d["dat"] or d["truot"]:
    bad.append(f"ca đồng nhất không xanh: {d['truot']}")
canary_runs = [r for r in runs if r["case_id"] == "00-canary-phai-do" and r["vai"] == "canary"]
if len(canary_runs) != 1 or sorted(t["kiem"] for t in canary_runs[0]["truot"]) != [canary_check]:
    bad.append("dòng chạy của canary không đỏ đúng chỗ")
if tk["so_sai"] == 0 or tk["sai_dat"] != tk["so_sai"]:
    bad.append(f"kịch bản sai trượt đúng chỗ {tk['sai_dat']}/{tk['so_sai']}")
if not tk["xanh"]:
    bad.append("tong_ket không xanh")
if bad:
    sys.exit("HỎNG: " + "; ".join(bad))
print(f"T1 Nếp: {tk['so_ca']} ca, {tk['so_luot']} lượt chạy, {tk['dat']} đạt; kịch bản sai {tk['sai_dat']}/{tk['so_sai']} trượt đúng chỗ; "
      f"canary đỏ đúng ở {canary_check}; đồng nhất xanh; không SKIP; hai lần chạy trùng byte; "
      f"prompt {tk['prompt_version_nep']}, corpus {tk['sha_bo'][:12]}")
PY
