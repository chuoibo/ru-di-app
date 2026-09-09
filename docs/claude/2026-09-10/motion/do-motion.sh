#!/usr/bin/env bash
# Frame-time gate (audit 09/09 §5): one release APK, named sequences, `dumpsys
# gfxinfo` reset before each and read after. Emits one markdown table row per
# sequence: janky %, p50/p90/p95/p99 and the slow/frozen counts the OS keeps.
#
#   docs/claude/2026-09-10/motion/do-motion.sh <out-dir> [reduce]
#
# `reduce` runs the same sequences with the OS animation scales at 0 -- what
# Android's «Remove animations» setting does and what `AccessibilityInfo.
# isReduceMotionEnabled` reads -- and restores 1 afterwards.
#
# What this does NOT prove: anything about a phone. This is an emulator on a
# WSL2 host; the numbers are a regression baseline for this rig, not a verdict
# on smoothness, and the release APK carries the fixture, not live data.
set -u
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
export PATH="$ANDROID_HOME/platform-tools:$HOME/.maestro/bin:$PATH"
export ANDROID_ADB_SERVER_PORT="${ANDROID_ADB_SERVER_PORT:-5038}"
export ANDROID_SERIAL="${ANDROID_SERIAL:-emulator-5554}"
OUT="${1:?thư mục ra}"; MODE="${2:-thuong}"
APP=com.lakiet.rudi
DAY="$(cd "$(dirname "$0")" && pwd)"
# FLOWS_DIR=.maestro-motion-live and OTP_PHONE/OTP_CODE in the environment select
# the release+live variant; the default is the dev-client fixture board.
FLOWS="$(cd "$DAY/../../../../apps/mobile/${FLOWS_DIR:-.maestro-motion}" && pwd)"
THEM=(); [ -n "${OTP_PHONE:-}" ] && THEM=(-e OTP_PHONE="$OTP_PHONE" -e OTP_CODE="${OTP_CODE:-000000}")
mkdir -p "$OUT"
scale() { for k in window_animation_scale transition_animation_scale animator_duration_scale; do adb shell settings put global $k "$1" >/dev/null; done; }
[ "$MODE" = reduce ] && scale 0
adb shell settings get global animator_duration_scale > "$OUT/animator_duration_scale.txt"
echo "| chuỗi | khung | janky | p50 | p90 | p95 | p99 | slow UI | slow draw | missed vsync | maestro |" > "$OUT/bang.md"
echo "|---|---|---|---|---|---|---|---|---|---|---|" >> "$OUT/bang.md"
for f in "$FLOWS"/m*.yaml; do
  ten="$(basename "$f" .yaml)"
  # warm the app once (cold start is not what is being measured), then reset counters
  adb shell am force-stop $APP >/dev/null 2>&1; sleep 1
  adb shell dumpsys gfxinfo $APP reset >/dev/null 2>&1
  rm -rf "$OUT/$ten-maestro"
  timeout 900 maestro --device "$ANDROID_SERIAL" test "${THEM[@]}" --test-output-dir "$OUT/$ten-maestro" "$f" > "$OUT/$ten.maestro.log" 2>&1
  rc=$?
  adb shell dumpsys gfxinfo $APP > "$OUT/$ten.gfxinfo.txt" 2>&1
  g="$OUT/$ten.gfxinfo.txt"
  # Values sit AFTER the colon; the label's own digits («50th») must not be read as the value.
  sau() { grep -m1 "^$1" "$g" | sed 's/^[^:]*: *//' | grep -oE '^[0-9]+(\.[0-9]+)?%?(ms)?' | head -1; }
  khung=$(sau "Total frames rendered"); janky=$(grep -m1 "^Janky frames:" "$g" | grep -oE '\([0-9.]+%\)' | tr -d '()')
  p50=$(sau "50th percentile"); p90=$(sau "90th percentile"); p95=$(sau "95th percentile"); p99=$(sau "99th percentile")
  sui=$(sau "Number Slow UI thread"); sdr=$(sau "Number Slow issue draw commands"); vsync=$(sau "Number Missed Vsync")
  echo "| $ten | ${khung:-?} | ${janky:-?} | ${p50:-?} | ${p90:-?} | ${p95:-?} | ${p99:-?} | ${sui:-?} | ${sdr:-?} | ${vsync:-?} | rc=$rc |" >> "$OUT/bang.md"
done
[ "$MODE" = reduce ] && scale 1
cat "$OUT/bang.md"
