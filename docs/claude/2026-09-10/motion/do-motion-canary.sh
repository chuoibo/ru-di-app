#!/usr/bin/env bash
# Self-test of do-motion.sh's exit codes with a fake `adb` and a fake `maestro`
# on PATH; no device is touched (re-audit 12/09 C1, runner v4).
#
#   docs/claude/2026-09-10/motion/do-motion-canary.sh
#
# Every red branch must make the runner exit with the documented non-zero code
# AT THE END of the run, and the two green controls (a real gfxinfo capture from
# the repo, originals 0.5 / 1.5 / 2) must exit 0 with four numeric rows and the
# originals written back. The fake adb keeps the scales in a state file so the
# canary can also count `settings put` calls and read what was left behind.
# The table below has 31 branches: two green controls plus malformed dumps,
# command failures, scale drift, restore failures and interruption.
# A canary that cannot make the runner red proves the gate does not bite; a
# canary that only tests red cannot see a gate that blocks the happy path — so
# a real run on the emulator is still required before the table is trusted.
set -u -o pipefail
DAY="$(cd "$(dirname "$0")" && pwd)"
RUNNER="${MOTION_RUNNER:-$DAY/do-motion.sh}"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
mkdir -p "$T/bin"
DUMP="$DAY/dev-client-v2/thuong/m1-doi-tab.gfxinfo.txt"
[ -f "$DUMP" ] || { echo "thiếu dump mẫu $DUMP" >&2; exit 2; }
# The fake pidof answers with the pid in the dump's own header, so the green
# control proves the pid-in-dump check passes on real data.
PID_DUMP="$(grep -m1 -oE 'for pid [0-9]+' "$DUMP" | grep -oE '[0-9]+')"
[ -n "$PID_DUMP" ] || { echo "dump mẫu không có header pid" >&2; exit 2; }

cat > "$T/bin/adb" <<'ADB'
#!/usr/bin/env bash
# Fake adb. State (scales, counters) lives in $CANARY_STATE as key=value lines,
# last line wins; every call is appended to $CANARY_CALLS.
st="$CANARY_STATE"; echo "adb $*" >> "$CANARY_CALLS"
get() { grep "^$1=" "$st" | tail -1 | cut -d= -f2-; }
put() { printf '%s=%s\n' "$1" "$2" >> "$st"; }
case "${1:-} ${2:-} ${3:-} ${4:-}" in
  "shell settings get global")
    if [ "$CANARY_MODE" = goc-rong ] && [ "$5" = animator_duration_scale ] && ! grep -q '^da_put=' "$st"; then echo; exit 0; fi
    if [ "$CANARY_MODE" = goc-rc ] && ! grep -q '^da_put=' "$st"; then get "$5"; exit 1; fi
    if [ "$CANARY_MODE" = doclai-rc ] && grep -q '^da_put=' "$st" && ! grep -q '^tra=' "$st"; then get "$5"; exit 1; fi
    if [ "$CANARY_MODE" = tra-doc-rc ] && grep -q '^tra=' "$st"; then get "$5"; exit 1; fi
    if [ "$CANARY_MODE" = cua-so-doc-rc ] && grep -q '^warm=' "$st" && ! grep -q '^tra=' "$st"; then get "$5"; exit 1; fi
    get "$5"; exit 0 ;;
  "shell settings put global")
    put da_put 1
    [ "$6" != 0 ] && put tra 1
    [ "$CANARY_MODE" = put-hong ] && [ "$6" = 0 ] && { echo "permission denied"; exit 1; }
    [ "$CANARY_MODE" = tra-hong ] && [ "$6" != 0 ] && { echo "permission denied"; exit 1; }
    put "$5" "$6"; exit 0 ;;
esac
case "${1:-} ${2:-}" in
  "shell pidof")
    n=$(( $(get pid_calls) + 1 )); put pid_calls "$n"
    case "$CANARY_MODE" in
      pid-rong) echo ;; pid-sai) echo pid-not-known ;; pid-doi) echo $((CANARY_PID + n)) ;; *) echo "$CANARY_PID" ;;
    esac; exit 0 ;;
  "shell dumpsys")
    if [ "${5:-}" = reset ]; then [ "$CANARY_MODE" = reset-hong ] && exit 1; exit 0; fi
    case "$CANARY_MODE" in
      khung-0)       sed 's/^Total frames rendered: .*/Total frames rendered: 0/' "$CANARY_DUMP" ;;
      dump-rong)     : ;;
      dump-thieu)    echo "Total frames rendered: 12" ;;
      dump-pid-khac) sed "s/for pid $CANARY_PID /for pid $((CANARY_PID + 1)) /" "$CANARY_DUMP" ;;
      hist-lech)     sed 's/^HISTOGRAM: 5ms=0/HISTOGRAM: 5ms=1/' "$CANARY_DUMP" ;;
      hist-*)
        frames="$(awk '/^Total frames rendered:/ {print $4; exit}' "$CANARY_DUMP")"
        case "$CANARY_MODE" in
          hist-chu) line="nonsense=$frames" ;;
          hist-thieu-ms) line="150=$frames" ;;
          hist-dem-am) line="5ms=-1 150ms=$((frames + 1))" ;;
          hist-dem-le) line="5ms=0.5 150ms=$((frames - 1)).5" ;;
          hist-trung) line="150ms=0 150ms=$frames" ;;
          hist-dao) line="150ms=$frames 5ms=0" ;;
          hist-rac) line="5ms=0 rác 150ms=$frames" ;;
          hist-hai-dong) line="5ms=$frames" ;;
          hist-xanh-cham) line="5ms=$((frames - 3)) 150ms=1 200ms=2" ;;
        esac
        sed "s/^HISTOGRAM:.*/HISTOGRAM: $line/" "$CANARY_DUMP"
        [ "$CANARY_MODE" = hist-hai-dong ] && echo "HISTOGRAM: 5ms=0"
        ;;
      *)             cat "$CANARY_DUMP" ;;
    esac; exit 0 ;;
esac
exit 0
ADB
cat > "$T/bin/maestro" <<'MAE'
#!/usr/bin/env bash
echo "maestro $*" >> "$CANARY_CALLS"
n="$(grep -c '^maestro ' "$CANARY_CALLS")"
[ "$n" = 1 ] && echo warm=1 >> "$CANARY_STATE"
if { [ "$CANARY_MODE" = scale-truoc ] && [ "$n" = 1 ]; } || { [ "$CANARY_MODE" = scale-sau ] && [ "$n" = 2 ]; }; then
  echo animator_duration_scale=1 >> "$CANARY_STATE"
fi
# `ngat`: interrupt the runner (grandparent: fake maestro ← timeout ← do-motion.sh) during the second flow.
if [ "$CANARY_MODE" = ngat ] && [ "$n" = 2 ]; then kill -INT "$(ps -o ppid= -p "$PPID" | tr -d ' ')"; sleep 0.3; fi
[ "$CANARY_MODE" = maestro-42 ] && exit 42
exit 0
MAE
chmod +x "$T/bin/adb" "$T/bin/maestro"
export CANARY_DUMP="$DUMP" CANARY_PID="$PID_DUMP" ANDROID_HOME="$T/no-sdk"

GOC_W=0.5; GOC_T=1.5; GOC_A=2
chay() { # <mode> <thuong|reduce> → sets rc, hop_le, maestro, puts, scale
  local mode="$1" che="$2"
  export CANARY_MODE="$mode" CANARY_STATE="$T/$mode.state" CANARY_CALLS="$T/$mode.calls"
  printf 'window_animation_scale=%s\ntransition_animation_scale=%s\nanimator_duration_scale=%s\npid_calls=0\n' "$GOC_W" "$GOC_T" "$GOC_A" > "$CANARY_STATE"
  : > "$CANARY_CALLS"
  PATH="$T/bin:/usr/bin:/bin" bash "$RUNNER" "$T/ra-$mode" "$che" > "$T/$mode.out" 2>&1
  rc=$?
  hop_le="$(grep -E '^\| m' "$T/$mode.out" | grep -vc 'KHÔNG HỢP LỆ' || true)"
  hoi="$(grep -E '^\| m' "$T/$mode.out" | grep -v 'KHÔNG HỢP LỆ' | grep -c '?' || true)"
  maestro="$(grep -c '^maestro ' "$CANARY_CALLS" || true)"
  puts="$(grep -c '^adb shell settings put ' "$CANARY_CALLS" || true)"
  local w t a
  w="$(grep '^window_animation_scale=' "$CANARY_STATE" | tail -1 | cut -d= -f2)"
  t="$(grep '^transition_animation_scale=' "$CANARY_STATE" | tail -1 | cut -d= -f2)"
  a="$(grep '^animator_duration_scale=' "$CANARY_STATE" | tail -1 | cut -d= -f2)"
  if [ "$w/$t/$a" = "$GOC_W/$GOC_T/$GOC_A" ]; then scale=goc; elif [ "$w/$t/$a" = "0/0/0" ]; then scale=0; else scale="$w/$t/$a"; fi
}

# mode · run mode · expected: rc · valid rows · maestro calls · put calls · scales left (goc|0|*)
BANG='
xanh          reduce 0   4 5 * goc
xanh-thuong   thuong 0   4 5 0 goc
maestro-42    reduce 1   0 1 * goc
pid-doi       reduce 1   0 5 * goc
pid-rong      reduce 1   0 1 * goc
pid-sai       reduce 1   0 1 * goc
khung-0       reduce 1   0 5 * goc
dump-rong     reduce 1   0 5 * goc
dump-thieu    reduce 1   0 5 * goc
dump-pid-khac reduce 1   0 5 * goc
hist-lech     reduce 1   0 5 * goc
hist-chu      reduce 1   0 5 * goc
hist-thieu-ms reduce 1   0 5 * goc
hist-dem-am   reduce 1   0 5 * goc
hist-dem-le   reduce 1   0 5 * goc
hist-trung    reduce 1   0 5 * goc
hist-dao      reduce 1   0 5 * goc
hist-rac      reduce 1   0 5 * goc
hist-hai-dong reduce 1   0 5 * goc
hist-xanh-cham reduce 0  4 5 * goc
reset-hong    reduce 1   0 1 * goc
goc-rong      reduce 3   0 0 0 goc
goc-rc        reduce 3   0 0 0 goc
doclai-rc     reduce 4   0 0 * goc
tra-doc-rc    reduce 5   4 5 * goc
cua-so-doc-rc reduce 1   0 1 * goc
scale-truoc   reduce 1   0 1 * goc
scale-sau     reduce 1   0 2 * goc
put-hong      reduce 4   0 0 * goc
tra-hong      reduce 5   4 5 * 0
ngat          reduce 130 0 2 * goc
'
loi=0; so_ca=0; LY_DO=''
printf '%-14s %-7s | %-9s %-7s %-10s %-6s %-9s | %s\n' nhánh chếđộ 'exit' 'hợp lệ' maestro put 'scale sau' 'kết'
while read -r mode che e_rc e_hl e_ma e_put e_sc; do
  [ -n "$mode" ] || continue
  so_ca=$((so_ca + 1))
  chay "$mode" "$che"
  ket=đúng
  [ "$rc" = "$e_rc" ] || ket="SAI exit"
  [ "$hop_le" = "$e_hl" ] || ket="SAI hợp lệ"
  [ "$maestro" = "$e_ma" ] || ket="SAI maestro"
  [ "$e_put" = '*' ] || [ "$puts" = "$e_put" ] || ket="SAI put"
  [ "$scale" = "$e_sc" ] || ket="SAI scale"
  [ "$hoi" = 0 ] || ket="SAI: hàng hợp lệ có «?»"
  if [ "$mode" = hist-xanh-cham ]; then
    [ "$(awk -F'|' '/^\| m/ {gsub(/ /, "", $9); if ($9 == 3) n++} END {print n+0}' "$T/$mode.out")" = 4 ] || ket="SAI khung chậm"
  fi
  [ "$ket" = đúng ] || loi=1
  printf '%-14s %-7s | %-3s→%-5s %-1s→%-5s %-1s→%-8s %-6s %-9s | %s\n' "$mode" "$che" "$e_rc" "$rc" "$e_hl" "$hop_le" "$e_ma" "$maestro" "$puts" "$scale" "$ket"
  # what the runner SAID about the failure — the reason must be legible in the table, not only in the exit code
  ly_do="$(grep -m1 -oE 'KHÔNG HỢP LỆ \([^)]*\)' "$T/$mode.out" || grep -m1 -E 'không|KHÔNG' "$T/$mode.out" || true)"
  [ "$e_rc" = 0 ] || LY_DO+="$(printf '%-14s %s\n' "$mode" "${ly_do:-(không in lý do — kết thúc bằng mã $rc)}")"$'\n'
done <<< "$BANG"
echo; echo "lý do runner in ra ở từng nhánh đỏ:"; printf '%s' "$LY_DO"
echo; echo "hàng xanh mẫu:"; grep -E '^\| m1' "$T/xanh.out"
[ "$loi" = 0 ] && echo "CANARY XANH: $so_ca nhánh đúng kỳ vọng" || { echo "CANARY ĐỎ: có nhánh sai kỳ vọng" >&2; exit 1; }
