#!/usr/bin/env bash
# Record ONE navigation flow on the dev client and count its transition frames.
#
#   quay-chuyen-canh.sh <out-dir> <flow.yaml> [scale 0|1]
#
# The app must already be on «Khám phá» (fixture) with Metro serving this tree.
# If a scale is given, the three Android animation scales are set to it for the
# recording and restored to their previous values by a trap. The clip is pulled,
# split at 30 fps, and so-khung.py prints the run lengths; the caller reads
# `max`: scale 1 must show a slide (max >= 3, positive control), Reduce Motion
# after the R1 fix must show a cut (max <= 2 for every event).
set -u -o pipefail
export ANDROID_ADB_SERVER_PORT="${ANDROID_ADB_SERVER_PORT:-5038}" ANDROID_SERIAL="${ANDROID_SERIAL:-emulator-5554}"
export PATH="$HOME/Android/Sdk/platform-tools:$HOME/.maestro/bin:$PATH"
OUT="${1:?thư mục ra}"; FLOW="${2:?flow}"; SCALE="${3:-}"
DAY="$(cd "$(dirname "$0")" && pwd)"
ten="$(basename "$FLOW" .yaml)"; mkdir -p "$OUT/$ten-khung"
KHOA=(window_animation_scale transition_animation_scale animator_duration_scale)
declare -A GOC; for k in "${KHOA[@]}"; do GOC[$k]="$(adb shell settings get global "$k" | tr -d '\r')"; done
khoi_phuc() { for k in "${KHOA[@]}"; do adb shell settings put global "$k" "${GOC[$k]}" >/dev/null 2>&1 || true; done; }
trap 'khoi_phuc; exit 130' INT TERM
trap khoi_phuc EXIT
if [ -n "$SCALE" ]; then for k in "${KHOA[@]}"; do adb shell settings put global "$k" "$SCALE" >/dev/null; done; sleep 1; fi
APP=com.lakiet.rudi
pid0="$(adb shell pidof $APP | tr -d '\r')"
{ for k in "${KHOA[@]}"; do echo "$k=$(adb shell settings get global "$k" | tr -d '\r')"; done; echo "pid_truoc=$pid0"; } > "$OUT/$ten.scale.txt"
adb shell rm -f "/sdcard/$ten.mp4" >/dev/null 2>&1
adb shell screenrecord --time-limit 25 --bit-rate 6000000 "/sdcard/$ten.mp4" > /dev/null 2>&1 &
REC=$!
sleep 1.5
maestro --device "$ANDROID_SERIAL" test --test-output-dir "$OUT/$ten-maestro" "$FLOW" > "$OUT/$ten.maestro.log" 2>&1
rc=$?
sleep 1
adb shell pkill -INT screenrecord >/dev/null 2>&1 || true
wait $REC 2>/dev/null || true
sleep 1
adb pull "/sdcard/$ten.mp4" "$OUT/$ten.mp4" > /dev/null
pid1="$(adb shell pidof $APP | tr -d '\r')"
echo "pid_sau=$pid1" >> "$OUT/$ten.scale.txt"
[ -n "$pid0" ] && [ "$pid0" = "$pid1" ] || { echo "pid đổi ($pid0 → $pid1): app đã khởi động lại giữa clip" >&2; rc=1; }
rm -f "$OUT/$ten-khung"/*.png
ffmpeg -v error -y -i "$OUT/$ten.mp4" -vf fps=30 "$OUT/$ten-khung/%03d.png"
echo "flow=$ten maestro_rc=$rc $(tr '\n' ' ' < "$OUT/$ten.scale.txt")"
python3 "$DAY/so-khung.py" "$OUT/$ten-khung" --json "$OUT/$ten.khung.json" | cut -c1-160
[ "$rc" = 0 ]
