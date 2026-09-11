#!/usr/bin/env bash
# One flow at two text sizes × two themes: screenshot + hierarchy dump per pass,
# optional gate script on the dump.
#   chup.sh <metro-port> <out-dir> <flow.yaml> <shot-name> [gate.mjs]
set -u
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
export PATH="$ANDROID_HOME/platform-tools:$HOME/.maestro/bin:$PATH"
export ANDROID_ADB_SERVER_PORT="${ANDROID_ADB_SERVER_PORT:-5038}"
export ANDROID_SERIAL="${ANDROID_SERIAL:-emulator-5554}"
PORT="${1:?cổng Metro}"; OUT="${2:?thư mục ra}"; FLOW="${3:?flow}"; SHOT="${4:?tên ảnh}"; GATE="${5:-}"
mkdir -p "$OUT"
timeout 20 adb reverse "tcp:$PORT" "tcp:$PORT" >/dev/null || { echo "adb reverse hỏng"; exit 2; }
do_=0
for fs in 1.0 2.0; do
  for night in no yes; do
    ten="$SHOT-fs$fs-$([ "$night" = yes ] && echo toi || echo sang)"
    adb shell settings put system font_scale "$fs" >/dev/null
    adb shell cmd uimode night "$night" >/dev/null
    sleep 3
    rm -rf "$OUT/$ten-maestro"
    timeout 600 maestro --device "$ANDROID_SERIAL" test --test-output-dir "$OUT/$ten-maestro" "$FLOW" > "$OUT/$ten.maestro.log" 2>&1
    rc=$?; echo "maestro exit=$rc" >> "$OUT/$ten.maestro.log"
    anh="$(find "$OUT/$ten-maestro" -name "$SHOT.png" | head -1)"
    [ -n "$anh" ] && mv "$anh" "$OUT/$ten.png"
    timeout 30 adb exec-out uiautomator dump /dev/tty 2>/dev/null | sed -n '/<?xml/,/<\/hierarchy>/p' > "$OUT/$ten.xml"
    echo "== $ten == maestro exit=$rc"
    [ "$rc" = 0 ] || do_=1
    if [ -n "$GATE" ]; then node "$GATE" "$OUT/$ten.xml" | tee "$OUT/$ten.kiem.txt" || do_=1; fi
  done
done
adb shell settings put system font_scale 1.0 >/dev/null
adb shell cmd uimode night no >/dev/null
[ "$do_" = 0 ] && echo "XANH: bốn cấu hình" || echo "ĐỎ: có cấu hình hỏng"
exit $do_
