#!/usr/bin/env bash
# Tier T3 of the AI eval (design 06 §5, §8.1): ONE command for the product
# owner to get measured numbers from the real model, then the replay that
# proves them (§8.2).
#
#   scripts/eval_that.sh --bo <corpus> --tran-goi N [--lap k] [--chi-buoc hieu] [--out <dir>]
#
#   --bo        the corpus: a whole-turn corpus (testdata/corpus/*.json) or,
#               with --chi-buoc hieu, a router set (testdata/hieu/*.json).
#               A path, or a name under services/core/internal/aieval/testdata
#               («corpus/nep-kich-ban.json», «hieu/tien_v3.json»).
#   --tran-goi  the number of provider calls (model + embedding + rerank) the
#               Lead approved for this run. Required; no default. The run is
#               refused when its exact upper-bound estimate exceeds it, and a
#               watchdog stops it at N (then it is «chưa xong»: no trailer).
#
# The model goes where production's goes (ADR-0051): through agy-proxy when
# AGY_PROXY_URL and AGY_PROXY_KEY are set, else the Gemini API directly.
# Embeddings always go to the Gemini API (agy serves none), so GEMINI_API_KEY
# is needed either way. The scoreboard names which door answered.
#
# What it does, in order:
#   1. refuses without GEMINI_API_KEY (it never prints the value) or without
#      --tran-goi, or with AGY_PROXY_URL but no AGY_PROXY_KEY;
#   2. builds cmd/rudi-eval at the checked-out SHA;
#   3. prints the estimate; runs `rudi-eval --mo-hinh that`, which records
#      a cassette of every real call into the evidence store
#      (~/.cache/rudi-bang-chung/eval/<run_id>/ unless --out);
#   4. replays that run at the same SHA with `--mo-hinh phat-lai`, without the
#      key and with every proxy pointed at a closed port: it must make 0
#      provider calls and reproduce every grade line;
#   5. prints the Vietnamese commit-trailer block (only for a valid run on a
#      clean tree whose replay matched), and where bang-diem.md is.
#
# Cassettes and traces are model output: they stay in the evidence store,
# never in Git (design 06 §14.1). Runbook: docs/claude/2026-09-25/chay-model-that.md.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$(pwd)"
CORE=services/core
TESTDATA="$CORE/internal/aieval/testdata"

bo="" tran="" lap="1" chi_buoc="" out=""
while [ $# -gt 0 ]; do
  case "$1" in
    --bo) bo="${2:-}"; shift 2 ;;
    --tran-goi) tran="${2:-}"; shift 2 ;;
    --lap) lap="${2:-}"; shift 2 ;;
    --chi-buoc) chi_buoc="${2:-}"; shift 2 ;;
    --out) out="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,32p' "$0"; exit 0 ;;
    *) echo "eval_that: tham số lạ: $1" >&2; exit 2 ;;
  esac
done

if [ -z "${GEMINI_API_KEY:-}" ]; then
  cat >&2 <<'MSG'
eval_that: TỪ CHỐI — chưa có GEMINI_API_KEY trong môi trường.
  Thêm khoá trong cài đặt môi trường (environment settings → Edit → biến môi trường
  GEMINI_API_KEY), rồi mở một phiên MỚI để biến có hiệu lực. Không dán khoá vào lệnh,
  vào file hay vào repo. Hướng dẫn: docs/claude/2026-09-25/chay-model-that.md
MSG
  exit 2
fi
if [ -z "$tran" ]; then
  echo "eval_that: TỪ CHỐI — thiếu --tran-goi N: số lời gọi Lead đã duyệt cho lượt này (không có mặc định)." >&2
  exit 2
fi
case "$tran" in ''|*[!0-9]*) echo "eval_that: --tran-goi phải là số nguyên dương" >&2; exit 2 ;; esac
case "$lap" in ''|*[!0-9]*) echo "eval_that: --lap phải là số nguyên dương" >&2; exit 2 ;; esac
[ -n "$bo" ] || { echo "eval_that: thiếu --bo <corpus>" >&2; exit 2; }
if [ ! -f "$bo" ] && [ -f "$TESTDATA/$bo" ]; then bo="$TESTDATA/$bo"; fi
[ -f "$bo" ] || { echo "eval_that: không thấy corpus $bo" >&2; exit 2; }
bo="$(cd "$(dirname "$bo")" && pwd)/$(basename "$bo")"
if [ -n "${MOBILE_GEMINI_BASE_URL:-}" ]; then
  echo "eval_that: MOBILE_GEMINI_BASE_URL đang đặt — lượt thật phải tới Gemini API thật; bỏ biến đó (bản giả loopback là --mo-hinh ghi)." >&2
  exit 2
fi
if [ -n "${AGY_PROXY_URL:-}" ]; then
  if [ -z "${AGY_PROXY_KEY:-}" ]; then
    echo "eval_that: TỪ CHỐI — AGY_PROXY_URL đã đặt mà thiếu AGY_PROXY_KEY (không dán khoá vào lệnh; đặt trong môi trường)." >&2
    exit 2
  fi
  echo "--- model qua agy-proxy (cửa production dùng); embedding gọi Gemini API thẳng"
else
  echo "--- model gọi Gemini API thẳng (không có AGY_PROXY_URL)"
fi
command -v go >/dev/null 2>&1 || { echo "thiếu go" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "thiếu python3" >&2; exit 2; }

sha="$(git rev-parse HEAD)"
cay=sach
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  cay=ban
  echo "eval_that: CẢNH BÁO — cây có thay đổi chưa commit: số vẫn đo, nhưng KHÔNG in trailer (trailer chỉ cho cây sạch đúng SHA)." >&2
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM
echo "--- build cmd/rudi-eval tại ${sha:0:12}"
( cd "$CORE" && go build -tags eval -o "$work/rudi-eval" ./cmd/rudi-eval )

args=(--bo "$bo" --tran-goi "$tran" --lap "$lap" --git-sha "$sha" --cay "$cay")
[ -n "$chi_buoc" ] && args+=(--chi-buoc "$chi_buoc")
[ -n "$out" ] && args+=(--out "$out")

echo "--- dự toán"
"$work/rudi-eval" --mo-hinh that "${args[@]}" --du-toan

echo "--- lượt thật (--mo-hinh that, trần $tran lời gọi)"
set +e
( cd "$CORE" && "$work/rudi-eval" --mo-hinh that "${args[@]}" ) >"$work/that.out"
rc_that=$?
set -e
dir="$(python3 -c 'import json,sys
lines=[l for l in open(sys.argv[1]) if l.strip()]
print(json.loads(lines[-1]).get("thu_muc","") if lines else "")' "$work/that.out" 2>/dev/null || true)"
if [ -z "$dir" ]; then
  echo "HỎNG: lượt thật không ra thư mục bằng chứng (thoát $rc_that)" >&2
  exit 2
fi
echo "thư mục lượt thật: $dir (thoát $rc_that)"

echo "--- phát lại cùng SHA (--mo-hinh phat-lai, không khoá, proxy đóng)"
set +e
( cd "$CORE" && env -u GEMINI_API_KEY -u MOBILE_GEMINI_BASE_URL -u MOBILE_RERANK_URL -u AGY_PROXY_URL -u AGY_PROXY_KEY \
    HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 \
    https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 all_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy= \
    "$work/rudi-eval" --mo-hinh phat-lai --bang "$dir" --git-sha "$sha" ${out:+--out "$out"} ) >"$work/lai.out"
rc_lai=$?
set -e
lai="$(python3 -c 'import json,sys
lines=[l for l in open(sys.argv[1]) if l.strip()]
print(json.loads(lines[-1]).get("thu_muc","") if lines else "")' "$work/lai.out" 2>/dev/null || true)"
[ -n "$lai" ] || { echo "HỎNG: phát lại không ra thư mục (thoát $rc_lai)" >&2; exit 2; }

python3 - "$dir" "$lai" <<'PY'
import json, os, sys
goc, lai = sys.argv[1], sys.argv[2]
mg = json.load(open(os.path.join(goc, "manifest.json"), encoding="utf-8"))
ml = json.load(open(os.path.join(lai, "manifest.json"), encoding="utf-8"))
bad = []
if ml["goi"]["da_dung"] != 0:
    bad.append(f"phát lại gọi nhà cung cấp {ml['goi']['da_dung']} lần (phải 0)")
sv = ml.get("so_voi_nguon") or {}
if not sv.get("trung"):
    bad.append("điểm phát lại KHÁC lượt thật: " + "; ".join(sv.get("khac") or []))
print(f"lượt thật {mg['run_id']}: {mg['trang_thai']}; gọi {mg['goi']['da_dung']}/{mg['goi']['tran_duyet']} "
      f"(model {mg['goi']['mo_hinh']}, nhúng {mg['goi']['nhung']}, xếp lại {mg['goi']['xep_lai']}); dự toán {mg['goi']['du_toan']['tran']}")
print(f"phát lại {ml['run_id']}: {ml['trang_thai']}; gọi {ml['goi']['da_dung']}; trả từ cassette {ml['goi']['tu_bang']}; điểm {'trùng' if sv.get('trung') else 'KHÁC'}")
print(f"bảng điểm: {os.path.join(goc, 'bang-diem.md')}")
tr = os.path.join(lai, "trailer.txt")
if os.path.exists(tr):
    print("\n--- trailer cho commit message (dán nguyên khối):\n")
    print(open(tr, encoding="utf-8").read().rstrip())
else:
    why = mg.get("ly_do", []) + ml.get("ly_do", [])
    print("\nKHÔNG có trailer: " + ("; ".join(why) if why else "lượt không hợp lệ hoặc cây không sạch"))
if bad:
    sys.exit("HỎNG: " + "; ".join(bad))
PY
if [ "$rc_that" -ne 0 ] || [ "$rc_lai" -ne 0 ]; then
  echo "eval_that: lượt thật thoát $rc_that, phát lại thoát $rc_lai — đọc bang-diem.md trước khi dùng số nào." >&2
  exit 1
fi
