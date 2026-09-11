#!/usr/bin/env bash
# Self-test of do-motion.sh's exit code, with a fake `adb` and a fake `maestro`
# on PATH and no device touched (re-audit 10/09, B1: the v1 runner exited 0 with
# every Maestro flow red and every frame count «?»).
#
#   docs/claude/2026-09-10/motion/do-motion-canary.sh
#
# Case ĐỎ: maestro exits 42 → do-motion.sh MUST exit non-zero.
# Case XANH: maestro exits 0 and the dump is a real gfxinfo capture from the repo
#            → do-motion.sh MUST exit 0 and print a numeric row.
# A canary that cannot make the runner red proves the gate does not bite.
set -u -o pipefail
DAY="$(cd "$(dirname "$0")" && pwd)"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
mkdir -p "$T/bin"
DUMP="$DAY/dev-client/thuong/m1-doi-tab.gfxinfo.txt"
[ -f "$DUMP" ] || { echo "thiếu dump mẫu $DUMP" >&2; exit 2; }

cat > "$T/bin/adb" <<'ADB'
#!/usr/bin/env bash
# Fake adb: settings get → 1; pidof → a constant (or a new value every call when
# CANARY_PID_DOI=1); gfxinfo → the sample dump (or one with 0 frames when
# CANARY_KHUNG_0=1); everything else → nothing.
case "$*" in
  *"settings get"*) echo 1 ;;
  *"pidof"*) if [ "${CANARY_PID_DOI:-0}" = 1 ]; then echo $((4242 + RANDOM)); else echo 4242; fi ;;
  *"dumpsys gfxinfo"*" reset"*) : ;;
  *"dumpsys gfxinfo"*) if [ "${CANARY_KHUNG_0:-0}" = 1 ]; then sed 's/^Total frames rendered: .*/Total frames rendered: 0/' "$CANARY_DUMP"; else cat "$CANARY_DUMP"; fi ;;
  *) : ;;
esac
ADB
cat > "$T/bin/maestro" <<'MAE'
#!/usr/bin/env bash
exit "${CANARY_MAESTRO_RC:-0}"
MAE
chmod +x "$T/bin/adb" "$T/bin/maestro"
export CANARY_DUMP="$DUMP"
export ANDROID_HOME="$T/no-sdk"   # keep the real platform-tools off PATH's front

chay() { # <label> <maestro rc> ; prints runner rc
  local nhan="$1" rc="$2" ra="$T/ra-$1"
  PATH="$T/bin:$PATH" CANARY_MAESTRO_RC="$rc" bash "$DAY/do-motion.sh" "$ra" thuong > "$T/$nhan.out" 2>&1
  echo $?
}
do_rc="$(chay do 42)"; xanh_rc="$(chay xanh 0)"
pid_rc="$(CANARY_PID_DOI=1 chay pid 0)"; khung_rc="$(CANARY_KHUNG_0=1 chay khung 0)"
doc() { [ "$1" != 0 ] && echo '→ đúng, cổng chặn' || echo '→ SAI: cổng cho qua'; }
echo "canary ĐỎ  (maestro exit 42):        do-motion.sh exit $do_rc  $(doc "$do_rc")"
echo "canary ĐỎ  (pid đổi giữa chuỗi):      do-motion.sh exit $pid_rc  $(doc "$pid_rc")"
echo "canary ĐỎ  (Total frames rendered 0): do-motion.sh exit $khung_rc  $(doc "$khung_rc")"
echo "canary XANH (maestro exit 0, dump thật): do-motion.sh exit $xanh_rc  $( [ "$xanh_rc" = 0 ] && echo '→ đúng' || echo '→ SAI: cổng chặn nhầm')"
grep -E '^\| m' "$T/xanh.out" | head -2
[ "$do_rc" != 0 ] && [ "$pid_rc" != 0 ] && [ "$khung_rc" != 0 ] && [ "$xanh_rc" = 0 ]
