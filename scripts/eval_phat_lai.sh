#!/usr/bin/env bash
# Tier T2 of the AI eval (design 06 §6): replay a recorded run's cassette on
# the CURRENT tree, with no key and every proxy pointed at a closed port. It
# proves that code after the model (grounding, verifier reads, the scorer)
# changed without changing a grade; it does not prove that the model would
# answer the same today. A request the cassette has no recording for is
# bang_lech: red, never a network call.
#
#   scripts/eval_phat_lai.sh <run_dir> [--bo <corpus>] [--out <dir>]
#
# <run_dir> is a `that` (or `ghi`) run directory under
# ~/.cache/rudi-bang-chung/eval/. The corpus defaults to the path the run
# recorded; it must have the same sha256. Exit 0: every grade line
# identical and 0 provider calls; 1: it ran and differs (or bang_lech);
# 2: it could not run.
set -euo pipefail

cd "$(dirname "$0")/.."
CORE=services/core
[ $# -ge 1 ] || { sed -n '2,17p' "$0" >&2; exit 2; }
dir="$1"; shift
[ -f "$dir/manifest.json" ] && [ -f "$dir/bang-ghi.json" ] || { echo "eval_phat_lai: $dir không phải thư mục lượt có cassette" >&2; exit 2; }
extra=()
while [ $# -gt 0 ]; do
  case "$1" in
    --bo|--out) extra+=("$1" "${2:-}"); shift 2 ;;
    *) echo "eval_phat_lai: tham số lạ: $1" >&2; exit 2 ;;
  esac
done
command -v go >/dev/null 2>&1 || { echo "thiếu go" >&2; exit 2; }
sha="$(git rev-parse HEAD)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM
( cd "$CORE" && go build -tags eval -o "$work/rudi-eval" ./cmd/rudi-eval )
set +e
( cd "$CORE" && env -u GEMINI_API_KEY -u MOBILE_GEMINI_BASE_URL -u MOBILE_RERANK_URL \
    HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 \
    https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 all_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy= \
    "$work/rudi-eval" --mo-hinh phat-lai --bang "$dir" --git-sha "$sha" "${extra[@]}" )
rc=$?
set -e
exit "$rc"
