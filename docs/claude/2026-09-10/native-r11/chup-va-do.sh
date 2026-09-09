#!/usr/bin/env bash
# F41 evidence loop: the failed-sticker row at three text sizes × two themes,
# each one screenshotted AND measured from the view hierarchy.
#
#   docs/claude/2026-09-10/native-r11/chup-va-do.sh <metro-port> <out-dir>
#
# Run AFTER `scripts/mobile_native.sh --flows .maestro-bs-r11 --keep`: `--keep`
# leaves Metro on <metro-port> but removes the adb reverse, so this re-adds it.
# The dev client reopens the last bundle on launch, which is what every flow's
# `stopApp`/`launchApp` relies on already.
#
# Why a loop outside Maestro: a flow can change neither font scale nor theme,
# and `assertVisible` cannot see a button hanging off the screen (audit 09/09,
# ảnh 16). So each pass sets the device, runs the one flow for the screenshot,
# dumps the hierarchy, and lets kiem-bounds.mjs say whether both labels are in
# frame. Exit 1 if any pass is red.
set -u
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
export PATH="$ANDROID_HOME/platform-tools:$HOME/.maestro/bin:$PATH"
export ANDROID_ADB_SERVER_PORT="${ANDROID_ADB_SERVER_PORT:-5038}"
export ANDROID_SERIAL="${ANDROID_SERIAL:-emulator-5554}"

PORT="${1:?cổng Metro}"; OUT="${2:?thư mục ra}"
DAY="$(cd "$(dirname "$0")" && pwd)"
MOBILE="$(cd "$DAY/../../../../apps/mobile" && pwd)"
FLOW="$MOBILE/.maestro-bs-r11/76-hang-cho-chu-lon.yaml"
mkdir -p "$OUT"
timeout 20 adb reverse "tcp:$PORT" "tcp:$PORT" >/dev/null || { echo "adb reverse hỏng"; exit 2; }

do_=0
for fs in 1.0 1.3 2.0; do
  for night in no yes; do
    ten="fs$fs-$([ "$night" = yes ] && echo toi || echo sang)"
    adb shell settings put system font_scale "$fs" >/dev/null
    adb shell cmd uimode night "$night" >/dev/null
    sleep 3
    # Screenshots land under --test-output-dir (the harness does the same), NOT
    # in the cwd: the first run of this loop looked in the cwd and kept nothing.
    # --device is explicit because Maestro otherwise drives the FIRST device
    # `adb devices` lists, which on this machine can be another lane's.
    rm -rf "$OUT/$ten-maestro"
    timeout 600 maestro --device "$ANDROID_SERIAL" test --test-output-dir "$OUT/$ten-maestro" "$FLOW" > "$OUT/$ten.maestro.log" 2>&1
    echo "maestro exit=$?" >> "$OUT/$ten.maestro.log"
    anh="$(find "$OUT/$ten-maestro" -name 'r11-76-hang-cho.png' | head -1)"
    [ -n "$anh" ] && mv "$anh" "$OUT/$ten.png"
    # uiautomator writes the XML then one trailing status line; keep the XML.
    timeout 30 adb exec-out uiautomator dump /dev/tty 2>/dev/null | sed -n '/<?xml/,/<\/hierarchy>/p' > "$OUT/$ten.xml"
    echo "== $ten =="
    tail -1 "$OUT/$ten.maestro.log"
    if node "$DAY/kiem-bounds.mjs" "$OUT/$ten.xml" | tee "$OUT/$ten.bounds.txt"; then :; else do_=1; fi
  done
done
adb shell settings put system font_scale 1.0 >/dev/null
adb shell cmd uimode night no >/dev/null
[ "$do_" = 0 ] && echo "XANH: sáu cấu hình đều có hai nhãn trong khung" || echo "ĐỎ: có cấu hình tràn/thiếu nhãn"
exit $do_
