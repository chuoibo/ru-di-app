#!/usr/bin/env bash
# Frame-time gate, v4 (review 12/09 C1: strict buckets and verified scale windows).
#
#   scripts/do/do-motion.sh <out-dir> [thuong|reduce]
#
# The order is fixed and every step is verified before the next one runs:
#   1. read the three animation scales; any value that is not a plain number
#      ends the run with exit 3 BEFORE any `settings put` and before Maestro —
#      a scale we could not read is a scale we could not restore (B12);
#   2. `reduce` writes 0 to each scale and READS IT BACK; a refused write or a
#      read-back that is not 0 restores and exits 4, still before Maestro;
#   3. one warm-up outside every window (the entry helper walks to «Khám phá»);
#   4. per sequence: pid before → `gfxinfo reset`, whose exit code is checked
#      (a failed reset means the window was never cleared: the sequence is NOT
#      run and the row is invalid, B11) → Maestro, ONLY the gesture under test
#      → dump, exit code checked → pid after;
#   5. the trap writes the originals back, reads them back and compares; a
#      restore that did not land makes the script exit 5 (130 on INT only when
#      the restore landed).
#
# A row is valid only when ALL of these hold — the table never prints a `?`
# in a valid row, and a missing histogram is never printed as 0 slow frames:
#   Maestro rc 0 · dump rc 0 · pid before is a number · pid after equals it ·
#   the pid in the dump header equals it · frames > 0 · janky %, p50/p90/p95/
#   p99, slow UI / slow draw / missed vsync all present and numeric · a
#   HISTOGRAM line exists · the histogram buckets sum to the frame count.
# The script exits 1 if any row is invalid, so a table of «?» can never come
# out of a run that exited 0.
#
# `khung>150ms` is the sum of the histogram buckets from 150 ms up; p50…p99 are
# the LABELS of the buckets the percentiles fall in (the histogram runs to
# 4950 ms). FLOWS_DIR=.maestro-motion-live and OTP_PHONE/OTP_CODE select the
# live variant; default is the dev-client fixture board. The rig is an emulator
# on WSL2: a regression baseline, not a verdict on a phone.
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
FLOWS="$(cd "$DAY/../../apps/mobile/${FLOWS_DIR:-.maestro-motion}" && pwd)"
VAO="${VAO_FLOW:-$([ "${FLOWS_DIR:-}" = .maestro-motion-live ] && echo _vao-live.yaml || echo _vao-app-sach.yaml)}"
THEM=()
[ -n "${OTP_PHONE:-}" ] && THEM=(-e OTP_PHONE="$OTP_PHONE" -e OTP_CODE="${OTP_CODE:-000000}")
mkdir -p "$OUT"
KHOA=(window_animation_scale transition_animation_scale animator_duration_scale)
SO='^[0-9]+(\.[0-9]+)?$'

doc_scale() { adb shell settings get global "$1" 2>/dev/null | tr -d '\r'; }
# Numeric equality of two plain decimals ("0" == "0.0"); non-numbers never compare equal.
bang_so() { [[ "$1" =~ $SO && "$2" =~ $SO ]] && awk -v a="$1" -v b="$2" 'BEGIN{exit !(a+0==b+0)}'; }

# --- 1. originals: read and validate BEFORE any mutation (B12) ---------------------
declare -A GOC
for k in "${KHOA[@]}"; do
  GOC[$k]="$(doc_scale "$k")" || { echo "lệnh đọc $k ban đầu thất bại — không đo" >&2; exit 3; }
done
{ for k in "${KHOA[@]}"; do echo "$k=${GOC[$k]}"; done; } > "$OUT/scale-goc.txt"
for k in "${KHOA[@]}"; do
  [[ "${GOC[$k]}" =~ $SO ]] || { echo "không đọc được $k ban đầu («${GOC[$k]}») — dừng trước mọi thay đổi, không đo" >&2; exit 3; }
done

# --- restore: write back, read back, compare; a miss is exit 5 -----------------------
DA_TRA=""   # set once the restore ran: "0" landed, "1" did not
khoi_phuc() {
  [ -n "$DA_TRA" ] && return "$DA_TRA"
  local loi=0 k muon doc khop
  {
    echo "khoa muon doc khop"
    for k in "${KHOA[@]}"; do
      muon="${GOC[$k]}"; khop=1
      # `thuong` never wrote, so it only verifies; `reduce` writes the original back.
      if [ "$MODE" = reduce ]; then adb shell settings put global "$k" "$muon" >/dev/null 2>&1 || khop=0; fi
      doc="$(doc_scale "$k")" || khop=0
      bang_so "$doc" "$muon" || khop=0
      [ "$khop" = 1 ] || { loi=1; echo "KHÔNG TRẢ ĐƯỢC $k: muốn $muon, đọc «$doc»" >&2; }
      echo "$k $muon $doc $khop"
    done
  } > "$OUT/scale-sau.txt"
  DA_TRA=$loi
  return "$loi"
}
ket_thuc() { # <exit code the run wants>; a restore that did not land overrides it with 5
  local ma="$1"
  khoi_phuc || ma=5
  exit "$ma"
}
# INT/TERM must EXIT after restoring: a handler that returns lets the loop go on
# measuring the remaining rows at the restored scale under the old label.
trap 'ket_thuc 130' INT TERM
trap 'ket_thuc $?' EXIT

# --- 2. set the mode and read it back (B12) --------------------------------------------
if [ "$MODE" = reduce ]; then
  for k in "${KHOA[@]}"; do
    adb shell settings put global "$k" 0 >/dev/null 2>&1 || { echo "không ghi được $k = 0 — không đo" >&2; exit 4; }
    doc="$(doc_scale "$k")" || { echo "lệnh đọc lại $k thất bại — không đo" >&2; exit 4; }
    bang_so "$doc" 0 || { echo "đã ghi $k = 0 nhưng đọc lại «$doc» — không đo" >&2; exit 4; }
  done
else
  for k in "${KHOA[@]}"; do
    doc="$(doc_scale "$k")" || { echo "lệnh đọc lại $k thất bại — không đo" >&2; exit 4; }
    bang_so "$doc" "${GOC[$k]}" || { echo "$k đổi giữa lúc đọc («${GOC[$k]}») và lúc đo («$doc») — không đo" >&2; exit 4; }
  done
fi
# Read command status AND values. Save the observed configuration at the
# boundaries, so a shared emulator changing mode cannot produce a green row
# under a stale label. This cannot detect a change and reversal inside a flow.
kiem_scale() { # <trace path>; true only when every scale still matches this mode
  local k doc muon loi=0
  for k in "${KHOA[@]}"; do
    muon="${GOC[$k]}"; [ "$MODE" = reduce ] && muon=0
    if ! doc="$(doc_scale "$k")"; then
      echo "$k: lệnh đọc thất bại" >&2; loi=1
    elif ! bang_so "$doc" "$muon"; then
      echo "$k: muốn $muon, đọc «$doc»" >&2; loi=1
    fi
    echo "$k=$doc"
  done > "$1"
  return "$loi"
}
kiem_scale "$OUT/scale-luc-do.txt" || exit 4

# --- 3. warm-up, once, outside every window -----------------------------------------
that_bai=0
rm -rf "$OUT/warm-maestro"
timeout 900 maestro --device "$ANDROID_SERIAL" test "${THEM[@]}" --test-output-dir "$OUT/warm-maestro" "$FLOWS/$VAO" > "$OUT/warm.maestro.log" 2>&1
warm_rc=$?
if [ "$warm_rc" != 0 ]; then echo "warm-up ($VAO) rc=$warm_rc — không có cửa sổ nào hợp lệ" >&2; that_bai=1; fi

# --- parser: every field or nothing; the caller decides ----------------------------------
# Prints ONE line, fields separated by `|` (never whitespace: bash `read` collapses runs of blanks, so an empty
# field would shift every later one into the wrong variable): pid khung janky p50 p90 p95 p99 slowUI slowDraw vsync tongHist cham coHist
doc_dump() {
  awk 'BEGIN { OFS = "|"; co = 0 }
    /^\*\* Graphics info for pid [0-9]+ \[/ && pid == "" { match($0, /pid [0-9]+/); pid = substr($0, RSTART + 4, RLENGTH - 4) }
    /^Total frames rendered:/ && khung == "" { khung = $4 }
    /^Janky frames:/ && janky == "" { if (match($0, /\([0-9.]+%\)/)) janky = substr($0, RSTART + 1, RLENGTH - 2) }
    /^50th percentile:/ && p50 == "" { p50 = $3; sub(/ms$/, "", p50) }
    /^90th percentile:/ && p90 == "" { p90 = $3; sub(/ms$/, "", p90) }
    /^95th percentile:/ && p95 == "" { p95 = $3; sub(/ms$/, "", p95) }
    /^99th percentile:/ && p99 == "" { p99 = $3; sub(/ms$/, "", p99) }
    /^Number Slow UI thread:/ && sui == "" { sui = $5 }
    /^Number Slow issue draw commands:/ && sdr == "" { sdr = $6 }
    /^Number Missed Vsync:/ && vsync == "" { vsync = $4 }
    /^HISTOGRAM:/ {
      if (co) bad = 1
      co++; n = split($0, a, " "); if (n < 2) bad = 1
      last = -1
      for (i = 2; i <= n; i++) {
        if (a[i] !~ /^[0-9]+ms=[0-9]+$/) { bad = 1; continue }
        split(a[i], kv, "="); ms = kv[1]; sub(/ms$/, "", ms)
        if (ms + 0 <= last) bad = 1
        last = ms + 0
        tong += kv[2]; if (ms + 0 >= 150) cham += kv[2]
      }
    }
    END { print pid, khung, janky, p50, p90, p95, p99, sui, sdr, vsync, (co ? tong + 0 : ""), (co ? cham + 0 : ""), (co == 1 && !bad ? 1 : 0) }' "$1"
}
la_so() { [[ "$1" =~ ^[0-9]+$ ]]; }

bang="$OUT/bang.md"
{
  echo "| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync | rc | pid |"
  echo "|---|---|---|---|---|---|---|---|---|---|---|---|---|"
} > "$bang"
hang_hong() { echo "| $1 | KHÔNG HỢP LỆ ($2) | | | | | | | | | | rc=${3:--} | ${4:--} |" >> "$bang"; that_bai=1; }

# --- 4. the sequences ----------------------------------------------------------------------
for f in "$FLOWS"/m*.yaml; do
  ten="$(basename "$f" .yaml)"
  if [ "$warm_rc" != 0 ]; then hang_hong "$ten" "warm-up đỏ"; continue; fi
  kiem_scale "$OUT/$ten.scale-truoc.txt" || { hang_hong "$ten" "scale trước cửa sổ không khớp hoặc đọc lỗi"; continue; }
  pid0="$(adb shell pidof "$APP" 2>/dev/null | tr -d '\r')" || { hang_hong "$ten" "lệnh đọc pid trước thất bại"; continue; }
  la_so "$pid0" || { hang_hong "$ten" "pid trước «$pid0» không phải số"; continue; }
  adb exec-out screencap -p > "$OUT/$ten-truoc.png" 2>/dev/null || true
  if ! adb shell dumpsys gfxinfo "$APP" reset > /dev/null 2>&1; then
    # The window was never cleared: do not run the gesture under this label (B11).
    hang_hong "$ten" "reset gfxinfo thất bại" "-" "$pid0"; continue
  fi
  rm -rf "$OUT/$ten-maestro"
  timeout 900 maestro --device "$ANDROID_SERIAL" test "${THEM[@]}" --test-output-dir "$OUT/$ten-maestro" "$f" > "$OUT/$ten.maestro.log" 2>&1
  rc=$?
  g="$OUT/$ten.gfxinfo.txt"
  adb shell dumpsys gfxinfo "$APP" > "$g" 2>&1
  dump_rc=$?
  pid1="$(adb shell pidof "$APP" 2>/dev/null | tr -d '\r')"; pid_rc=$?
  kiem_scale "$OUT/$ten.scale-sau.txt"; scale_rc=$?
  IFS='|' read -r pid_dump khung janky p50 p90 p95 p99 sui sdr vsync tong cham co_hist < <(doc_dump "$g")
  ly_do=()
  [ "$rc" = 0 ] || ly_do+=("maestro rc=$rc")
  [ "$dump_rc" = 0 ] || ly_do+=("dump rc=$dump_rc")
  [ "$pid_rc" = 0 ] || ly_do+=("lệnh đọc pid sau thất bại")
  [ "$scale_rc" = 0 ] || ly_do+=("scale sau cửa sổ không khớp hoặc đọc lỗi")
  [ "$pid1" = "$pid0" ] || ly_do+=("pid $pid0→${pid1:-rỗng}")
  [ "$pid_dump" = "$pid0" ] || ly_do+=("dump của pid «${pid_dump:-?}» ≠ $pid0")
  la_so "$khung" && [ "$khung" -gt 0 ] || ly_do+=("frames «${khung:-?}»")
  [[ "$janky" =~ ^[0-9]+(\.[0-9]+)?%$ ]] || ly_do+=("thiếu janky")
  for c in p50 p90 p95 p99 sui sdr vsync; do la_so "${!c}" || ly_do+=("thiếu $c"); done
  if [ "$co_hist" = 1 ]; then
    [ "$tong" = "$khung" ] || ly_do+=("histogram $tong ≠ frames $khung")
  else
    ly_do+=("HISTOGRAM thiếu hoặc bucket sai dạng/thứ tự")
  fi
  if [ "${#ly_do[@]}" = 0 ]; then
    echo "| $ten | $khung | $janky | $p50 | $p90 | $p95 | $p99 | $cham | $sui | $sdr | $vsync | rc=$rc | $pid0 |" >> "$bang"
  else
    hang_hong "$ten" "$(printf '%s; ' "${ly_do[@]}" | sed 's/; $//')" "$rc" "$pid0→${pid1:-?}"
  fi
done

{
  echo
  echo "Chế độ: $MODE · scale lúc đo: $(tr '\n' ' ' < "$OUT/scale-luc-do.txt") · scale gốc: $(tr '\n' ' ' < "$OUT/scale-goc.txt")"
  echo "p50…p99 là nhãn bucket của histogram gfxinfo chứa percentile ấy (histogram tới 4950 ms); khung>150ms là tổng các bucket từ 150 ms."
  echo "Hàng hợp lệ = Maestro rc 0, dump rc 0, một pid trước/sau/trong dump, frames > 0, đủ percentile và bộ đếm, histogram có mặt và cộng đúng bằng frames."
} >> "$bang"
cat "$bang"
if [ "$that_bai" = 1 ]; then echo "CÓ CHUỖI KHÔNG HỢP LỆ — bảng này không phải phép đo" >&2; exit 1; fi
