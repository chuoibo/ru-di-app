#!/usr/bin/env bash
# Frame-time gate, v2 (audit 09/09 §5; re-audit 10/09 R2/B1–B3).
#
#   docs/claude/2026-09-10/motion/do-motion.sh <out-dir> [thuong|reduce]
#
# One warm-up at the start (the app is walked to «Khám phá» by the entry helper,
# OUTSIDE every window), then per sequence: pid before → reset gfxinfo while the
# process is alive → the sequence, which is ONLY the gesture under test → dump →
# pid after. A row is valid only when Maestro exited 0, the pid did not change
# and frames > 0; the script exits 1 if any row is invalid, so a table of «?»
# can never come out of a run that exited 0 (B1). Animation scales are read
# first and restored to exactly those values by a trap (B3); `thuong` never
# writes them. `khung>150ms` is the sum of the histogram buckets from 150 ms up;
# p50…p99 are the LABELS of the buckets the percentiles fall in, not ceilings
# (B2: the histogram runs to 4950 ms).
#
# FLOWS_DIR=.maestro-motion-live and OTP_PHONE/OTP_CODE select the live variant;
# the default is the dev-client fixture board. The rig is an emulator on WSL2:
# a regression baseline, not a verdict on a phone.
set -u -o pipefail
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
# Tools already on PATH win (the canary puts fakes there); the usual homes are appended, not prepended.
command -v adb >/dev/null 2>&1 || export PATH="$PATH:$ANDROID_HOME/platform-tools"
command -v maestro >/dev/null 2>&1 || export PATH="$PATH:$HOME/.maestro/bin"
export ANDROID_ADB_SERVER_PORT="${ANDROID_ADB_SERVER_PORT:-5038}"
export ANDROID_SERIAL="${ANDROID_SERIAL:-emulator-5554}"
OUT="${1:?thư mục ra}"; MODE="${2:-thuong}"
case "$MODE" in thuong|reduce) ;; *) echo "chế độ «$MODE» không có; dùng thuong|reduce" >&2; exit 2 ;; esac
APP=com.lakiet.rudi
DAY="$(cd "$(dirname "$0")" && pwd)"
FLOWS="$(cd "$DAY/../../../../apps/mobile/${FLOWS_DIR:-.maestro-motion}" && pwd)"
VAO="${VAO_FLOW:-$([ "${FLOWS_DIR:-}" = .maestro-motion-live ] && echo _vao-live.yaml || echo _vao-app-sach.yaml)}"
THEM=()
[ -n "${OTP_PHONE:-}" ] && THEM=(-e OTP_PHONE="$OTP_PHONE" -e OTP_CODE="${OTP_CODE:-000000}")
mkdir -p "$OUT"
KHOA=(window_animation_scale transition_animation_scale animator_duration_scale)

# --- animation scales: read, remember, restore by trap --------------------------
declare -A GOC
that_bai=0
for k in "${KHOA[@]}"; do GOC[$k]="$(adb shell settings get global "$k" | tr -d '\r')"; done
{ for k in "${KHOA[@]}"; do echo "$k=${GOC[$k]}"; done; } > "$OUT/scale-goc.txt"
for k in "${KHOA[@]}"; do
  # An unreadable original is a run we cannot restore from: say so, and never `put ""`.
  [[ "${GOC[$k]}" =~ ^[0-9.]+$ ]] || { echo "không đọc được $k ban đầu («${GOC[$k]}») — lượt không hợp lệ" >&2; that_bai=1; }
done
khoi_phuc() {
  for k in "${KHOA[@]}"; do
    [[ "${GOC[$k]}" =~ ^[0-9.]+$ ]] || continue
    adb shell settings put global "$k" "${GOC[$k]}" >/dev/null 2>&1 || true
  done
  { for k in "${KHOA[@]}"; do echo "$k=$(adb shell settings get global "$k" | tr -d '\r')"; done; } > "$OUT/scale-sau.txt" 2>/dev/null || true
}
# INT/TERM must EXIT after restoring: a handler that returns lets the loop go on
# measuring the remaining rows at the restored scale under the old label.
trap 'khoi_phuc; exit 130' INT TERM
trap khoi_phuc EXIT
if [ "$MODE" = reduce ]; then for k in "${KHOA[@]}"; do adb shell settings put global "$k" 0 >/dev/null; done; fi
{ for k in "${KHOA[@]}"; do echo "$k=$(adb shell settings get global "$k" | tr -d '\r')"; done; } > "$OUT/scale-luc-do.txt"

# --- warm-up, once, outside every window -----------------------------------------
rm -rf "$OUT/warm-maestro"
timeout 900 maestro --device "$ANDROID_SERIAL" test "${THEM[@]}" --test-output-dir "$OUT/warm-maestro" "$FLOWS/$VAO" > "$OUT/warm.maestro.log" 2>&1
warm_rc=$?
if [ "$warm_rc" != 0 ]; then echo "warm-up ($VAO) rc=$warm_rc — không có cửa sổ nào hợp lệ" >&2; that_bai=1; fi

# --- parser: values sit AFTER the colon --------------------------------------------
sau() { grep -m1 "^$1" "$2" | sed 's/^[^:]*: *//' | grep -oE '^[0-9]+(\.[0-9]+)?%?(ms)?' | head -1; }
tren150() {
  # sum of HISTOGRAM buckets whose label is >= 150 ms (B2: 150 is a bucket, not the ceiling)
  grep -m1 '^HISTOGRAM:' "$1" | tr ' ' '\n' | grep -oE '^[0-9]+ms=[0-9]+' | awk -F'[m=]' '$1>=150{s+=$NF} END{print s+0}'
}

bang="$OUT/bang.md"
{
  echo "| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync | rc | pid |"
  echo "|---|---|---|---|---|---|---|---|---|---|---|---|---|"
} > "$bang"

for f in "$FLOWS"/m*.yaml; do
  ten="$(basename "$f" .yaml)"
  if [ "$that_bai" = 1 ] && [ "$warm_rc" != 0 ]; then
    echo "| $ten | KHÔNG HỢP LỆ (warm-up đỏ) | | | | | | | | | | rc=- | - |" >> "$bang"; continue
  fi
  pid0="$(adb shell pidof "$APP" | tr -d '\r')"
  adb exec-out screencap -p > "$OUT/$ten-truoc.png" 2>/dev/null || true
  adb shell dumpsys gfxinfo "$APP" reset > /dev/null 2>&1
  rm -rf "$OUT/$ten-maestro"
  timeout 900 maestro --device "$ANDROID_SERIAL" test "${THEM[@]}" --test-output-dir "$OUT/$ten-maestro" "$f" > "$OUT/$ten.maestro.log" 2>&1
  rc=$?
  g="$OUT/$ten.gfxinfo.txt"
  adb shell dumpsys gfxinfo "$APP" > "$g" 2>&1
  pid1="$(adb shell pidof "$APP" | tr -d '\r')"
  khung=$(sau "Total frames rendered" "$g"); janky=$(grep -m1 "^Janky frames:" "$g" | grep -oE '\([0-9.]+%\)' | tr -d '()')
  p50=$(sau "50th percentile" "$g"); p90=$(sau "90th percentile" "$g"); p95=$(sau "95th percentile" "$g"); p99=$(sau "99th percentile" "$g")
  sui=$(sau "Number Slow UI thread" "$g"); sdr=$(sau "Number Slow issue draw commands" "$g"); vsync=$(sau "Number Missed Vsync" "$g")
  cham=$(tren150 "$g")
  hop_le=1
  [ "$rc" = 0 ] || hop_le=0
  [ -n "$pid0" ] && [ "$pid0" = "$pid1" ] || hop_le=0
  [ -n "${khung:-}" ] && [ "${khung:-0}" -gt 0 ] 2>/dev/null || hop_le=0
  if [ "$hop_le" = 1 ]; then
    echo "| $ten | $khung | $janky | $p50 | $p90 | $p95 | $p99 | $cham | ${sui:-?} | ${sdr:-?} | ${vsync:-?} | rc=$rc | $pid0 |" >> "$bang"
  else
    echo "| $ten | KHÔNG HỢP LỆ | ${janky:-?} | | | | | | | | | rc=$rc | ${pid0:-?}→${pid1:-?} |" >> "$bang"
    that_bai=1
  fi
done

{
  echo
  echo "Chế độ: $MODE · scale lúc đo: $(tr '\n' ' ' < "$OUT/scale-luc-do.txt") · scale gốc: $(tr '\n' ' ' < "$OUT/scale-goc.txt")"
  echo "p50…p99 là nhãn bucket của histogram gfxinfo chứa percentile ấy (histogram tới 4950 ms); khung>150ms là tổng các bucket từ 150 ms."
} >> "$bang"
cat "$bang"
if [ "$that_bai" = 1 ]; then echo "CÓ CHUỖI KHÔNG HỢP LỆ — bảng này không phải phép đo" >&2; exit 1; fi
