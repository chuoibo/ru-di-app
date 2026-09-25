#!/usr/bin/env bash
#
# Run the gates that .github/workflows/*.yml run, on this machine, in one
# command.
#
# ## Why this exists
#
# GitHub Actions stopped starting jobs at 07:45Z on 2026-08-29: every run since
# ends in three to five seconds with "The job was not started because recent
# account payments have failed or your spending limit needs to be increased".
# Measured on 2026-08-29T12:0xZ: the last 100 runs are 100 failures and 0
# successes. Nothing is wrong with the code. `gh run view --log-failed` answers
# "log not found" and exits 0, so from the outside a dead account and a broken
# build look identical -- and nine pull requests merged that day under a red X
# that meant billing.
#
# `tests/test_workflow_gates_have_local_callers.py` (#148) already closed half
# of this: every gate that is a *script* must have a caller outside the
# workflows. It states the half it cannot close, in its own words:
#
#   "It also says nothing about workflow steps written inline rather than as a
#    script. Those cannot be detected this way."
#
# That is this file. The inline steps -- the offline DDL render, the image
# running as non-root, the container actually answering /healthz, the native
# bundle, and the one environment variable that turns three accessibility
# checks from decoration into a gate -- exist only inside YAML that currently
# cannot execute. Here they are, callable.
#
# ## The rule this file is built around
#
# A skip is not a pass. CLAUDE.md says it outright ("skip khong phai la xanh"),
# and the repository has been bitten by the other kind: a detector with no
# browser returning `[]` and exit 0, a postgres tier reporting 254 skips that
# read as green. So every stage here ends in exactly one of PASS, FAIL or SKIP;
# a SKIP must carry a reason; the summary prints the counts; and `--strict`
# turns every SKIP into a FAIL for use before a merge.
#
# The workflows' own distinction is kept too, because it is the right one:
# an absent directory is an absence and skipping is honest, but a directory
# that is present and has lost the file the stage runs is a defect, and
# skipping that would turn the stage green for the one reason it must not.
#
# ## What this does NOT prove
#
# It runs the same commands on a different machine. It does not prove the
# workflow YAML is well-formed, that the runner image still has what the jobs
# assume, or that the two will not drift apart -- nobody can check that while
# Actions is down. `tests/test_gate_covers_every_workflow_job.py` holds the
# narrower line that every job in the workflows is at least named here.
#
# Usage:
#   scripts/gate.sh                 every stage whose prerequisites are present
#   scripts/gate.sh api mobile      only those stages
#   scripts/gate.sh --strict        a SKIP is a failure (use before merging)
#   scripts/gate.sh --list          stage names and what each one runs
#
# Exit codes: 0 every stage that ran passed, 1 a stage failed,
# 2 the gate could not do its job -- bad arguments, or nothing ran at all.

set -uo pipefail

cd "$(dirname "$0")/.." || exit 2
REPO_ROOT="$PWD"

# MOBILE_GATE_IMAGE / MOBILE_GATE_CONTAINER: names no other worktree on this
# daemon can also address. Sourced here rather than inside `do_docker` so the
# id is one value for the whole run, and exported from there so
# `check_pinned_import.sh` builds the same tag instead of a second one.
# shellcheck source=scripts/gate_docker_names.sh
. "$REPO_ROOT/scripts/gate_docker_names.sh"

# Every stage, in run order: cheapest and most likely to fail first, so a
# broken tree is reported in seconds rather than after a docker build.
STAGES=(guard guard-range ruff contract client-routes server-routes screens cors ownership python-touch go-vet go-test api migration pinned-import demo-watch hero-walk shared mobile mobile-native docker parity postgres go-postgres go-broker e2e chat-e2e crypto)

stage_help() {
  case "$1" in
    guard)     echo "repo_guard.py tree HEAD (repo-guard.yml)" ;;
    guard-range) echo "repo_guard.py range on every commit this branch adds (repo-guard.yml)" ;;
    ruff)      echo "ruff on the files this branch changes, uncommitted ones included (test.yml: lint)" ;;
    contract)  echo "every route wanting X-Actor-ID is called with it (test.yml: contract)" ;;
    client-routes) echo "every route apps/mobile calls exists in the API (test.yml: api, inline)" ;;
    server-routes) echo "every route the API declares is called by some screen -- the other direction" ;;
    screens)   echo "every screen under apps/mobile/src/screens is rendered by something the entry point reaches" ;;
    cors)      echo "every header and method apps/mobile sends survives the CORS preflight (test.yml: contract)" ;;
    ownership) echo "route manifest matches the Python app and the Go binary; shared limiters have one owner (ADR-0029)" ;;
    python-touch) echo "no Python change on this branch reaches a route Go already serves (ADR-0029 §2.9)" ;;
    go-vet)    echo "gofmt -l and go vet on services/core, the Go front door (ADR-0029)" ;;
    go-test)   echo "go test ./... on services/core: config, transparent proxy, route manifest (ADR-0029)" ;;
    api)       echo "pytest services/api/tests tests (test.yml: api)" ;;
    migration) echo "alembic upgrade head --sql, no database (test.yml: api, inline)" ;;
    pinned-import) echo "app imports under the fastapi version pinned in requirements-dev.txt, not the machine's (test.yml: docker, cheap half)" ;;
    demo-watch) echo "the demo box is still being watched, and its last verdict was about main (máy này thôi)" ;;
    hero-walk) echo "somebody walked ảnh->món->chia->trang khách on the demo box recently, and it worked (máy này thôi)" ;;
    shared)    echo "node packages/shared/money.test.mjs (test.yml: shared)" ;;
    mobile)    echo "tsc, npm test with MOBILE_REQUIRE_WEB_A11Y=1, expo export --platform all (test.yml: mobile)" ;;
    mobile-native) echo "lái .maestro trên máy ảo Android thật qua Expo Go -- target sẽ ship, không phải react-native-web (test.yml: mobile-native)" ;;
    docker)    echo "api and core images pinned, build, non-root, api has no dev tooling, core no shell, both healthy (test.yml: docker)" ;;
    parity)    echo "harness unit tests; two isolated stacks from the API image; canary catches every exercised damage; W0 scenarios equal through core (ADR-0029)" ;;
    postgres)  echo "every live case -- tests/postgres AND tests/qa -- against a real PostgreSQL it provisions itself (postgres-repository.yml)" ;;
    go-postgres) echo "Go core tests on a disposable PostgreSQL migrated by Alembic; a skip or a missing sentinel is a failure (ADR-0029)" ;;
    go-broker) echo "Go tests tagged broker -- Redis Streams, RabbitMQ outbox relay -- on real services, disposable or CORE_TEST_*_URL; a skip or a missing sentinel is a failure" ;;
    e2e)       echo "the vertical slice through src/api.ts against an API and database it provisions itself (test.yml: e2e)" ;;
    chat-e2e)  echo "chat qua HTTP và WebSocket thật vào cửa trước Go, trên stack nó tự dựng (test.yml: chat-e2e)" ;;
    crypto)    echo "crate MLS dựng được, clippy sạch, 21 canary vẫn cắn, và cầu C ABI xuất đủ ký hiệu (test.yml: crypto)" ;;
  esac
}

STRICT=0
SELECTED=()

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    --list)
      echo "Các chặng của cổng (thứ tự chạy):"
      for s in "${STAGES[@]}"; do printf '  %-14s %s\n' "$s" "$(stage_help "$s")"; done
      exit 0
      ;;
    -h|--help) sed -n '2,60p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "Tham số lạ: $1 (xem scripts/gate.sh --help)" >&2; exit 2 ;;
    *)
      # An unknown stage name must not quietly select nothing and exit 0 --
      # that is the "green because it ran nothing" failure this file exists
      # to prevent, reproduced in its own argument parser.
      found=0
      for s in "${STAGES[@]}"; do [ "$s" = "$1" ] && found=1; done
      if [ "$found" -eq 0 ]; then
        echo "Không có chặng tên '$1'. Có: ${STAGES[*]}" >&2
        exit 2
      fi
      SELECTED+=("$1")
      ;;
  esac
  shift
done

[ ${#SELECTED[@]} -eq 0 ] && SELECTED=("${STAGES[@]}")

PASSED=(); FAILED=(); SKIPPED=(); SKIP_WHY=()
# Stages that failed WITHOUT running, and why. They write no log, so the
# failure report at the end has nothing to print for them -- and printing
# nothing under a header promising evidence is the exact shape this file was
# written to stamp out. Recorded here so the report can say what happened.
NORAN=(); NORAN_WHY=()
LOG_DIR="$(mktemp -d)"

banner() { printf '\n\033[1m=== %s ===\033[0m %s\n' "$1" "$(stage_help "$1")"; }
pass() { PASSED+=("$1"); printf '\033[32mĐẠT\033[0m     %s (%ss)\n' "$1" "$2"; }
fail() { FAILED+=("$1"); printf '\033[31mHỎNG\033[0m    %s (%ss)\n' "$1" "$2"; }
skip() { SKIPPED+=("$1"); SKIP_WHY+=("$1: $2"); printf '\033[33mBỎ QUA\033[0m  %s -- %s\n' "$1" "$2"; }

# A failure that never started. The reason goes to stderr for the reader
# watching the run, and into NORAN_WHY for the report at the end -- once stdout
# and stderr are separated, the stderr copy is the one that goes missing.
fail_noran() {
  echo "$2" >&2
  NORAN+=("$1"); NORAN_WHY+=("$2")
  fail "$1" 0
}

# Empty for a stage that ran. Callers use that to choose between printing a log
# and explaining its absence.
noran_why() {
  local i=0
  while [ "$i" -lt "${#NORAN[@]}" ]; do
    [ "${NORAN[$i]}" = "$1" ] && { printf '%s' "${NORAN_WHY[$i]}"; return 0; }
    i=$((i + 1))
  done
}

# Stage output is teed to a file so a failure can be re-printed at the end: on
# an eight-stage run the thing that broke has otherwise scrolled off.
have() { command -v "$1" >/dev/null 2>&1; }

# A per-run tag is a new image every run, and this gate runs dozens of times a
# day on a shared machine -- the fixed name it replaced at least got reused.
# So the run that generated the name takes it back off, once, after its last
# stage. Untagging leaves the layers, so the next run's build is still a cache
# hit; what goes away is the dangling entry in `docker images`.
gate_docker_cleanup() {
  [ "${MOBILE_GATE_NAMES_INHERITED:-0}" = "1" ] && return 0
  have docker || return 0
  docker rm -f "$MOBILE_GATE_CONTAINER" >/dev/null 2>&1 || true
  docker rm -f "$MOBILE_GATE_CORE_CONTAINER" >/dev/null 2>&1 || true
  docker image rm -f "$MOBILE_GATE_IMAGE" >/dev/null 2>&1 || true
  docker image rm -f "$MOBILE_GATE_CORE_IMAGE" >/dev/null 2>&1 || true
}
trap gate_docker_cleanup EXIT

# --- stage bodies ---------------------------------------------------------

do_guard() { python3 scripts/repo_guard.py tree HEAD; }

# The merge base with origin/main, or empty. Shared with check_prereq so the
# "nothing to scan" case can be reported as a skip rather than discovered
# halfway through the stage body.
guard_range_base() {
  local base
  base="$(git merge-base origin/main HEAD 2>/dev/null)" || base=""
  if [ -z "$base" ]; then
    git fetch --no-tags --quiet origin main 2>/dev/null || true
    base="$(git merge-base origin/main HEAD 2>/dev/null)" || base=""
  fi
  printf '%s' "$base"
}

do_guard-range() {
  # `guard` scans the tree at HEAD. That is not the same question, and the gap
  # between them is the whole reason repo-guard.yml runs both.
  #
  # A secret committed and then deleted in a later commit on the same branch is
  # absent from the tree at HEAD and present forever in history. Measured on
  # 2026-08-29 against a branch built exactly that way: `tree HEAD` passed with
  # 632 file scans and exit 0, `scripts/gate.sh guard` printed DAT, and
  # `range` found it -- one finding across 1265 file scans, exit 1.
  #
  # Three things could catch that, and while Actions is down none of them do.
  # The pre-commit hook is bypassed by `--no-verify` and absent entirely until
  # somebody runs scripts/setup-hooks.sh, which CLAUDE.md already says is
  # discipline rather than enforcement. The workflow step cannot start. So the
  # range form ran nowhere, on a repository whose stated rule is that bill
  # photos, account numbers and real participant names never enter Git at all
  # -- and where .gitignore is explicitly not a safe place to put them.
  #
  # `history` is deliberately not what this runs. It fails on main today
  # (findings predating the guard, from the 12,629-file era scripts/setup-hooks.sh
  # describes), so wiring it here would produce a stage that is red for
  # everyone forever and gets switched off within a day.
  local base count
  base="$(guard_range_base)"
  if [ -z "$base" ]; then
    # Same choice `do_ruff` makes, for the same reason: without a base there is
    # no answer, and a stage that cannot answer must not report that it did.
    echo "không tìm được merge base với origin/main" >&2
    return 1
  fi
  count="$(git rev-list --count "$base"..HEAD)"
  echo "quét $count commit nhánh này thêm vào, so với merge base $base"
  python3 scripts/repo_guard.py range "$base" HEAD
}

# The `ruff` stage's scopeless half: an assertion that is true or false no
# matter which files this branch changed. test.yml's lint job does not install
# whatever ruff it finds -- it reads this pin and fails with "::error::no ruff==
# pin" when there is none, so CI always lints with the version everybody agreed
# on.
#
# It is a function of its own because `check_prereq ruff` has to consult it
# before it is allowed to turn this stage into a skip. It did not, and that is
# the hole the two changes opened between them: the prereq decides from the
# changed-Python-file list alone, and deleting the pin edits a .txt file. No
# Python moves, so the scope is empty, so the stage skipped -- so the assertion
# written to catch exactly that deletion never ran. Measured on main, the same
# edit both times:
#
#   @ 23455e7 (pin check in, empty-scope skip not yet)
#     không có dòng ruff== ...        HỎNG    ruff (0s)          exit 1
#   @ ae45575 (both in)
#     BỎ QUA  ruff -- nhánh không đổi file Python nào ...        exit 2
#
# Anything added to do_ruff that does not depend on the changed-file list
# belongs on this side of the line too, or the skip will swallow it the same
# way. tests/test_gate_ruff_empty_scope.py holds both halves.
ruff_pin() {
  grep -E '^ruff==' services/api/requirements-dev.txt 2>/dev/null || true
}

do_ruff() {
  local pin
  pin="$(ruff_pin)"
  if [ -z "$pin" ]; then
    echo "không có dòng ruff== trong services/api/requirements-dev.txt" >&2
    echo "CI cài ruff từ pin đó; mất pin thì mỗi máy lint bằng một bản khác nhau." >&2
    return 1
  fi

  # This used to stop at a CHÚ Ý when the machine's ruff differed from the pin,
  # and pass the stage anyway. The reasoning was that hard-failing on a
  # mismatch makes the stage red on every machine with a newer ruff -- true,
  # and still true. What it missed is that a warning on line three of a
  # thirteen-stage run, under a summary that ends "ĐẠT ruff", is a warning
  # nobody reads, and while Actions is down this gate is the only one there is.
  #
  # Measured 2026-08-30 at c811254 over the 320 tracked Python files: the pin
  # (0.9.2) reports 31 findings, this machine's ruff (0.15.15) reports 30. The
  # missing one is UP038 on services/api/app/domain/place_search.py:105 -- a
  # rule later ruff REMOVED, so the newer binary cannot report it at all.
  # Editing that file got ĐẠT here and HỎNG in CI.
  #
  # So `ruff_changed.sh` now resolves the pin through scripts/ruff_pinned.sh
  # and provisions it when this machine lacks it, the same way
  # scripts/postgres_tier.sh stopped being a permanent BỎ QUA by building its
  # own database instead of demanding one. Nobody has to downgrade their
  # editor's ruff, and the verdict is CI's verdict.
  echo "pin: $pin"

  # The one-argument form compares <base> against the WORKING TREE, so this
  # covers changes not yet committed. CI can only ever see pushed commits;
  # locally the useful moment is before the commit exists.
  #
  # Shared with check_prereq deliberately. The two have to agree on which base
  # they mean: the prereq check decides whether this stage runs at all from the
  # file list at that base, and if it computed a different one it could skip a
  # stage that had work to do.
  local base
  base="$(guard_range_base)"
  if [ -z "$base" ]; then
    echo "không tìm được merge base với origin/main" >&2
    return 1
  fi
  echo "so với merge base $base"
  scripts/ruff_changed.sh "$base"
}

do_contract() {
  # Runs before `api` on purpose. It is seconds, and it answers a question no
  # other stage here asks: the two sides of one HTTP contract are checked by
  # two suites that each mock the other, so a route that starts demanding a
  # header leaves both suites green and the screen dead. See the file header.
  echo "--- self-test: the checker has to be able to be red"
  python3 scripts/check_actor_headers.py --selftest || return 1
  echo "--- client vs OpenAPI"
  python3 scripts/check_actor_headers.py
}

# Deliberately NOT called `do_contract`. It was, until 2026-08-29: this check
# was written on a branch cut before the actor-header stage reached main, and
# both stages picked the name `contract` independently. Merging the two left
# two `do_contract()` bodies in this file, neither inside a conflict marker,
# `bash -n` clean, and bash keeps the last one -- so the actor-header gate
# stopped running while `gate.sh contract` still printed its description and
# exited 0. tests/test_gate_stage_bodies_are_unique.py now refuses that shape.
do_client-routes() {
  # Reads the rendered OpenAPI and the client source. No database, no server,
  # no npm -- the two halves of a request compared where nothing else compares
  # them. Proven on 2026-08-29: with `/batches/current/publish` back in
  # api.ts, `tsc --noEmit` exited 0 and `npm test` passed 493 of 493.
  #
  # The self-test runs first for the same reason `do_contract`'s does, and the
  # reason got sharper on 2026-08-30: this checker used to drop, in silence,
  # every call whose URL it could not follow. `call<void>(path, ...)` with the
  # path handed in as a parameter named a route that has never existed and
  # exited 0. Six canaries now hold both halves -- that it reddens on a missing
  # route, and that it reddens when it cannot read the path at all.
  echo "--- self-test: the checker has to be able to be red"
  python3 scripts/check_api_contract.py --selftest || return 1
  echo "--- client vs OpenAPI"
  python3 scripts/check_api_contract.py
}

do_server-routes() {
  # `client-routes` asks whether every path the app calls exists. This asks the
  # other direction, which nothing in this repository asked until 2026-08-30: a
  # route the API declares that no screen calls does not exist for a user. It
  # ships, it is tested, it is merged, and it is unreachable.
  #
  # Measured on main at 8b6f847: 70 routes declared, 48 called, 5 belonging to
  # the guest page, and 17 with no caller at all. It earned its place the first
  # time it ran on a main newer than the tree it was written against: #319
  # merged /contexts/{id}/widget that morning and no screen calls it.
  #
  # The self-test runs first for the reason the two stages above it do, and
  # with a sharper edge here: this question was attempted twice by hand the
  # same day and got a different wrong answer each time -- substring matching
  # called four dead routes alive, whole-string matching called 32 live routes
  # dead. Six canaries hold both sides, three that must be red and three that
  # must stay green.
  echo "--- self-test: the checker has to be able to be red, and to be green"
  python3 scripts/check_server_routes_called.py --selftest || return 1
  echo "--- OpenAPI vs client"
  python3 scripts/check_server_routes_called.py
}

do_screens() {
  # The third link in the same chain, and the one nothing asked until
  # 2026-08-31. `client-routes` asks whether a path the app calls exists;
  # `server-routes` asks whether a declared route has a screen calling it.
  # Neither asks whether that screen is itself rendered by anything -- so a
  # screen can call its routes correctly, typecheck, pass both stages above,
  # and be openable by nobody.
  #
  # It earned its place on the run that introduced it. A work item claimed
  # `ChiaSe` and `MaCuaToi` had no way in, from a count of how often each name
  # appears under `src/`. Both were wired -- `ChiaSe` behind two real buttons,
  # `MaCuaToi` inside `MaKetBan` on Cá nhân -- and the count had missed the one
  # screen that really is dead, `TheDeXuat`, because it never named it. 48/49
  # reachable, 1 pinned with a reason.
  #
  # The self-test runs first, and here it carries a bug of its own making. The
  # first draft let an entry file's plain imports carry the chain, which marked
  # every screen `App.tsx` merely imports as reachable; deleting the real
  # `<ChiaSe />` render left the gate GREEN. Three canaries now hold it -- a
  # dead screen that must be red, an imported-but-never-rendered screen that
  # must be red, and a live tree that must stay green.
  echo "--- self-test: the checker has to be able to be red, and to be green"
  python3 scripts/check_screens_reachable.py --selftest || return 1
  echo "--- every screen against the render graph from the entry point"
  python3 scripts/check_screens_reachable.py
}

do_cors() {
  # The third question about one request, and the one no suite here can ask.
  # `contract` asks whether a call sends X-Actor-ID; `client-routes` asks
  # whether the path exists. Both assume the request is delivered. In a
  # browser it is not: a header the allowlist does not name is cancelled at
  # the preflight, and nothing in this repository is a browser -- TestClient
  # speaks in-process, node's fetch does not enforce CORS, and expo export
  # never issues a request.
  #
  # Measured on 2026-08-30 by adding "X-Client-Version" to the Khám phá search
  # headers: tsc 0, npm test 645/646 (identical to the clean baseline),
  # test_cors.py 13 passed, check_actor_headers 0, check_api_contract 0 --
  # and this stage exit 1, naming tim-kiem.ts:223.
  echo "--- self-test: the checker has to be able to be red"
  python3 scripts/check_cors_contract.py --selftest || return 1
  echo "--- client headers vs the CORS allowlist"
  python3 scripts/check_cors_contract.py
}

do_ownership() {
  # The selftest first: a gate only ever seen green has not been told apart
  # from one that cannot fail.
  python3 scripts/check_route_ownership.py --selftest || return 1
  python3 scripts/check_route_ownership.py
}

do_python-touch() {
  python3 scripts/check_go_owned_python_touch.py --selftest || return 1
  local base
  base="$(guard_range_base)"
  python3 scripts/check_go_owned_python_touch.py --base "${base:-origin/main}"
}

do_go-vet() {
  local unformatted
  unformatted="$(cd services/core && gofmt -l .)" || return 1
  if [ -n "$unformatted" ]; then
    echo "gofmt muốn sửa:" >&2
    echo "$unformatted" >&2
    return 1
  fi
  ( cd services/core && go vet ./... )
}

do_go-test() { ( cd services/core && go test -count=1 ./... ); }

do_go-postgres() { scripts/go_postgres_tier.sh; }

do_go-broker() { scripts/go_broker_tier.sh; }

# One pair of stacks per auth mode: a scenario means something only against
# stacks started in the mode it was written for. The raw-socket probe runs in
# dev, where the ADR-0029 exception list was measured.
parity_phase() {
  local mode="$1" env_file rc=0
  env_file="$(mktemp)"
  scripts/parity_stacks.sh up --auth "$mode" --env "$env_file" || { rm -f "$env_file"; return 1; }
  # shellcheck disable=SC1090
  . "$env_file"
  # `&&`, not separate lines: inside a subshell on the left of `||`, errexit
  # is off, and a failed canary followed by a passing run would read green.
  # The run goes before the canary. A damaged response can break a later
  # step's bind, and the canary then stops that scenario on the target only, so
  # afterwards the two databases hold different rows; a scenario that reads
  # other scenarios' rows (the public feed of GET /posts) would differ for
  # that reason alone.
  # The limiter lane goes last, in dev only: each of its scenarios waits for a
  # fresh limiter window and spends it, so nothing may run on the stacks after.
  # Every run and the canary also compare the two photo stores after each step
  # (media lane): a file dropped or left behind shows even when the wire and the
  # rows agree. The canary's target is the candidate's Python, which writes the
  # candidate's store.
  (
    cd parity &&
      go run ./cmd/parity run --auth "$PARITY_AUTH" --reference "$PARITY_REF_URL" --candidate "$PARITY_CAND_URL" \
        --reference-dsn "$PARITY_REF_DSN" --candidate-dsn "$PARITY_CAND_DSN" \
        --reference-media "$PARITY_REF_MEDIA" --candidate-media "$PARITY_CAND_MEDIA" \
        --candidate-tap "$PARITY_CAND_TAP_URL" --served-routes "$PARITY_SERVED_ROUTES" --candidate-python "$PARITY_CAND_PYTHON_TAP_URL" scenarios &&
      go run ./cmd/parity canary --auth "$PARITY_AUTH" --reference "$PARITY_REF_URL" --target "$PARITY_CAND_PYTHON_URL" \
        --reference-dsn "$PARITY_REF_DSN" --target-dsn "$PARITY_CAND_DSN" \
        --reference-media "$PARITY_REF_MEDIA" --target-media "$PARITY_CAND_MEDIA" scenarios &&
      { [ "$PARITY_AUTH" != dev ] || go run ./cmd/parity probe --reference "$PARITY_REF_URL" --candidate "$PARITY_CAND_URL"; } &&
      { [ "$PARITY_AUTH" != dev ] || go run ./cmd/parity run --lane limiter --auth "$PARITY_AUTH" --reference "$PARITY_REF_URL" --candidate "$PARITY_CAND_URL" \
          --reference-dsn "$PARITY_REF_DSN" --candidate-dsn "$PARITY_CAND_DSN" \
          --reference-media "$PARITY_REF_MEDIA" --candidate-media "$PARITY_CAND_MEDIA" \
          --candidate-tap "$PARITY_CAND_TAP_URL" --served-routes "$PARITY_SERVED_ROUTES" --candidate-python "$PARITY_CAND_PYTHON_TAP_URL" scenarios; }
  ) || rc=1
  if [ "$rc" -ne 0 ]; then
    # The containers are removed on teardown; keep their last words.
    local container
    for container in $PARITY_CONTAINERS; do
      case "$container" in
        *-api) echo "--- log cuối của $container"; docker logs --tail 30 "$container" 2>&1 ;;
      esac
    done
  fi
  scripts/parity_stacks.sh down --env "$env_file" >/dev/null || true
  rm -f "$env_file"
  return "$rc"
}

do_parity() {
  ( cd parity && go test -count=1 ./... && go run ./cmd/parity lint scenarios ) || return 1
  parity_phase dev || return 1
  parity_phase prod
}

do_api() { python3 -m pytest services/api/tests tests -q; }

do_migration() {
  # The inline step from test.yml's api job. No database: this is the check
  # that was missing when five foreign-key names ran past PostgreSQL's
  # 63-character limit and the migration could not compile at all.
  ( cd services/api && python3 - <<'PY'
import contextlib, io
from alembic import command
from alembic.config import Config
config = Config("alembic.ini")
config.set_main_option("sqlalchemy.url", "postgresql+psycopg://offline/offline")
with contextlib.redirect_stdout(io.StringIO()):
    command.upgrade(config, "head", sql=True)
print("migration renders")
PY
  )
}

# The demo box on 8099 is what the leader opens to decide whether the product
# runs. Twice now it has served an older main than the one it claims to:
# 58 routes against 62 for sixteen commits, then 65 against 69 for the four
# album and contextual-suggestion routes. Neither was a gate failing.
# `check_demo_matches_main.py` answered correctly both times -- it was simply
# never asked, because its only caller was `make demo-check`, which nobody
# types until they already suspect the answer.
#
# So this stage is the caller, and it is deliberately in the DEFAULT list.
# `make gate` is the one thing on this machine that gets run dozens of times a
# day; a check wired anywhere else is decoration with extra steps. It reads the
# recorded verdict rather than measuring live -- `run` builds a worktree and
# renders main's OpenAPI, which is far too slow to sit in every gate run, and
# duplicating it here would just be a second unscheduled call site.
#
# What it does NOT prove: nothing here calls a product route, so a demo serving
# every path of main and answering 500 to all of them passes this stage. It
# says nothing about the mobile bundle, which is built separately and can be
# older than the API on the same box. And `status` proves a check RAN, not that
# the box was reachable between two runs.
do_demo-watch() {
  # --expect-ref is the default, spelled out because this is the assertion the
  # stage exists to make: a verdict about somebody's open branch is not a
  # verdict about main, however fresh it is.
  python3 scripts/demo_watch.py status --expect-ref origin/main
}

# The scan seam -- `POST /receipts/scan` -> `readingFromWire()` -> `POST /bills`
# -- is the one joint of the hero path no other stage crosses. `e2e` runs
# `duong-bill.test.mjs`, which begins at a `reading` written by hand; the client
# unit tests replay a wire body frozen on 2026-08-29; the live model tier is
# opt-in behind a variable nothing sets. Two green halves, no path.
#
# Like `demo-watch`, this reads a RECORDED verdict instead of measuring live: a
# real walk costs a Gemini call, and a paid nondeterministic step in the list
# that runs dozens of times a day would be removed within the week. The live
# walk is `make hero-walk`; this asserts somebody ran it, recently, against this
# box, and that it worked.
#
# What it does NOT prove: nothing here is measured now. A demo that broke five
# minutes ago passes this stage until the verdict ages out. Nor does it prove
# the commits added since the walk still cross the seam -- the verdict binds to
# an ancestor of HEAD, which rules out evidence borrowed from another branch,
# not staleness within this one.
do_hero-walk() {
  # --url spelled out for the same reason demo-watch spells out --expect-ref:
  # a verdict about another box is the failure that looks most like a pass.
  # The runner separately refuses a verdict about another BRANCH; one shared
  # verdict dir serves every worktree here, so both halves are load-bearing.
  scripts/hero_walk.sh --status --url http://127.0.0.1:8099
}

do_shared() { node packages/shared/money.test.mjs; }

do_mobile() {
  cd apps/mobile || return 1
  echo "--- tsc --noEmit"
  npx --no-install tsc --noEmit || return 1
  echo "--- npm test (MOBILE_REQUIRE_WEB_A11Y=1)"
  # The flag is the whole point of running this here. Without it a machine
  # with no browser skips the three render checks in tests/vo-tab-web.test.mjs
  # and `npm test` still exits 0 -- which is the exact shape of the hole they
  # were added to close.
  MOBILE_REQUIRE_WEB_A11Y=1 npm test || return 1
  echo "--- expo export --platform all"
  # Lives only in the workflow otherwise, on purpose (so it cannot be removed
  # by editing one line of package.json). Web is about a third of the native
  # module graph, so a react-native module with no web counterpart leaves
  # `npm test` perfectly green. Export outside the checkout so no artifact
  # lands in the tree.
  EXPO_NO_TELEMETRY=1 npx --no-install expo export --platform all \
    --output-dir "$LOG_DIR/expo-export" >/dev/null || return 1
  echo "bundled for web, ios and android"
}

# Cùng một script mà job `mobile-native` trong test.yml gọi, không phải một bản
# chép lại. Ba cái neo (Metro đúng cây, thiết bị thật nạp bundle, canary phải đỏ)
# nằm trong script chứ không nằm ở đây, nên chạy tay và chạy trên CI hỏi đúng một
# câu hỏi.
do_mobile-native() {
  scripts/mobile_native.sh
  local rc=$?
  # Mã 2 nghĩa là KHÔNG ĐO ĐƯỢC, và check_prereq ở trên đã lọc hết các lý do
  # thường gặp. Tới được đây với mã 2 là hạ tầng rụng GIỮA lượt đo -- máy ảo
  # dùng chung bị lane khác tắt. Đó là hỏng, không phải đạt: bảng vừa chạy
  # không kết luận được gì.
  [ "$rc" -eq 0 ] || return 1
}

do_pinned-import() {
  # The cheap half of `docker`. That stage is the only one that has ever loaded
  # the app with the pinned fastapi, but it builds an image, starts a container
  # and waits on a HEALTHCHECK, so it got skipped on exactly the PRs that
  # needed it -- including the one that shipped an app which could not be
  # imported at all. Same proof, about two seconds, so there is no longer a
  # reason to skip it. `docker` still runs everything else it checks.
  scripts/check_pinned_import.sh
}

do_docker() {
  echo "--- base image pinned by digest"
  scripts/check_dockerfile_pinning.sh services/api/Dockerfile || return 1
  echo "--- build"
  ( cd services/api && docker build -t "$MOBILE_GATE_IMAGE" . ) || return 1
  echo "--- runs as a non-root user"
  local uid
  uid="$(docker run --rm --entrypoint id "$MOBILE_GATE_IMAGE" -u)"
  echo "container uid = $uid"
  [ "$uid" = "0" ] && { echo "ảnh chạy bằng root" >&2; return 1; }
  echo "--- no test tooling in the runtime image"
  if docker run --rm --entrypoint sh "$MOBILE_GATE_IMAGE" -c "ls /venv/bin" | grep -qE '^(pytest|ruff)$'; then
    echo "pytest hoặc ruff lọt vào ảnh chạy thật" >&2; return 1
  fi
  echo "--- the container actually serves /healthz"
  # No published host port, for the reason the workflow gives: curling the
  # host passes whenever *anything* answers on that port. Polling the
  # container's own HEALTHCHECK cannot be satisfied by a stranger.
  docker rm -f "$MOBILE_GATE_CONTAINER" >/dev/null 2>&1 || true
  # Cửa brain fail-closed: create_app() từ chối khởi động khi thiếu token, nên
  # một container dựng không token sẽ không bao giờ healthy và chặng này báo
  # "ảnh không phục vụ được" trong khi ảnh không có lỗi gì. Token của riêng lượt
  # chạy; ảnh không gửi nó đi đâu cả.
  docker run -d --name "$MOBILE_GATE_CONTAINER" \
    -e MOBILE_INTERNAL_TOKEN="$(head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 32)" \
    "$MOBILE_GATE_IMAGE" >/dev/null || return 1
  wait_container_healthy "$MOBILE_GATE_CONTAINER" || return 1

  # ADR-0029: the Go front door ships as its own image, held to the same bar.
  local core_image="$MOBILE_GATE_CORE_IMAGE" core_container="$MOBILE_GATE_CORE_CONTAINER"
  echo "--- core: base images pinned by digest"
  scripts/check_dockerfile_pinning.sh services/core/Dockerfile || return 1
  echo "--- core: build"
  ( cd services/core && docker build -t "$core_image" . ) || return 1
  echo "--- core: runs as a non-root user and ships no shell"
  local user
  user="$(docker image inspect --format '{{.Config.User}}' "$core_image")"
  echo "image user = ${user:-<unset>}"
  case "${user%%:*}" in
    ""|0|root) echo "ảnh core chạy bằng root" >&2; return 1 ;;
  esac
  if docker run --rm --entrypoint sh "$core_image" -c true >/dev/null 2>&1; then
    echo "ảnh core có shell -- nền distroless đã bị thay" >&2; return 1
  fi
  echo "--- core: the container reports healthy"
  docker rm -f "$core_container" >/dev/null 2>&1 || true
  # Any upstream will do: core's healthcheck asks whether core itself serves
  # and deliberately never goes through Python. Same env as test.yml: an
  # unreachable database (the pool is lazy), and the chat feed and group AI
  # turned off by the documented flag, because with them on (the prod
  # default) core checks the chat schema at startup and rightly refuses to
  # start without a database.
  docker run -d --name "$core_container" --health-interval 2s \
    -e MOBILE_PYTHON_UPSTREAM=http://127.0.0.1:9 \
    -e MOBILE_DATABASE_URL=postgresql://none@127.0.0.1:9/none \
    -e MOBILE_CHAT_CHANGES_CANDIDATE=0 \
    "$core_image" >/dev/null || return 1
  wait_container_healthy "$core_container"
}

# Poll a container's own HEALTHCHECK. Removes the container on every outcome.
wait_container_healthy() {
  local name="$1" i status
  for i in $(seq 1 60); do
    status="$(docker inspect --format '{{.State.Health.Status}}' "$name" 2>/dev/null)"
    case "$status" in
      healthy) echo "$name healthy sau ${i}s"; docker rm -f "$name" >/dev/null; return 0 ;;
      unhealthy) echo "$name unhealthy" >&2; docker logs "$name"; docker rm -f "$name" >/dev/null; return 1 ;;
    esac
    if [ "$(docker inspect --format '{{.State.Running}}' "$name" 2>/dev/null)" != "true" ]; then
      echo "$name thoát trước khi healthy" >&2; docker logs "$name"; docker rm -f "$name" >/dev/null; return 1
    fi
    sleep 1
  done
  echo "$name không bao giờ healthy" >&2; docker logs "$name"; docker rm -f "$name" >/dev/null; return 1
}

do_postgres() {
  # Delegates so the provisioning has a caller outside this file and can be
  # tested on its own (tests/test_postgres_tier_runner.py). The script builds
  # its own throwaway database when no URL is given, which is what stops this
  # stage from being the permanent BO QUA it had been: 147 skips and 0 runs in
  # CI, and a skip locally every time nobody exported a connection string.
  #
  # The runner covers `tests/qa` as well as `tests/postgres` since bug-082455.
  # The two trees looked like one because the stage reported a single number:
  # 306 passing cases, which is exactly what `tests/postgres` collects alone,
  # so the sixteen live QA cases were absent from a green nobody could read as
  # incomplete. The runner now prints a verdict per tree.
  scripts/postgres_tier.sh -q
}

do_crypto() {
  # The OpenMLS spike had no gate at all: it could stop compiling, or lose every
  # canary, and nothing in the repository would notice. Counting the canaries is
  # the point -- `cargo test` passes just as happily with none left.
  local log; log="$(mktemp)"
  cargo fmt --manifest-path packages/chat-crypto/Cargo.toml --check || return 1
  cargo clippy --manifest-path packages/chat-crypto/Cargo.toml --all-targets -- -D warnings || return 1
  cargo test --manifest-path packages/chat-crypto/Cargo.toml --all-targets 2>&1 | tee "$log" || return 1
  local passed
  passed="$(grep -oE '^test result: ok\. [0-9]+ passed' "$log" | awk '{s+=$4} END {print s+0}')"
  echo "canary MLS: $passed ca"
  [ "$passed" -ge 20 ] || { echo "chỉ $passed canary chạy; crate này có 21 — bộ test teo lại không phải bộ test xanh" >&2; return 1; }
  ! grep -qE '^test result: .*[1-9][0-9]* (failed|ignored)' "$log" || return 1

  # The C ABI lives in its own crate so the audited core keeps
  # `#![forbid(unsafe_code)]`. It is the only `unsafe` in this repository, so it
  # gets clippy at deny level, and it is built for the Android target the app
  # will dlopen it from -- building only for the host would prove nothing about
  # the thing that actually has to load.
  [ -d packages/chat-crypto-ffi ] || return 0
  cargo fmt --manifest-path packages/chat-crypto-ffi/Cargo.toml --check || return 1
  cargo clippy --manifest-path packages/chat-crypto-ffi/Cargo.toml --all-targets -- -D warnings || return 1
  cargo build --manifest-path packages/chat-crypto-ffi/Cargo.toml --release || return 1
  # `rustup target add` cho std của target và KHÔNG cho gì khác: linker và
  # sysroot đến từ NDK. Thiếu chúng thì link hỏng ở `-llog`, `-lunwind`.
  # `packages/chat-crypto/scripts/check_android.sh` đã ghi đúng ba biến này.
  local ndk="${ANDROID_NDK_ROOT:-${ANDROID_NDK_LATEST_HOME:-}}"
  if rustup target list --installed 2>/dev/null | grep -q x86_64-linux-android \
     && [ -n "$ndk" ] && [ -d "$ndk" ]; then
    local bin="$ndk/toolchains/llvm/prebuilt/linux-x86_64/bin"
    CARGO_TARGET_X86_64_LINUX_ANDROID_LINKER="$bin/x86_64-linux-android26-clang" \
    CC_x86_64_linux_android="$bin/x86_64-linux-android26-clang" \
    AR_x86_64_linux_android="$bin/llvm-ar" \
      cargo build --manifest-path packages/chat-crypto-ffi/Cargo.toml --release --target x86_64-linux-android || return 1
  else
    echo "thiếu target x86_64-linux-android hoặc ANDROID_NDK_ROOT; bỏ qua bước dựng cho Android (CI vẫn dựng)" >&2
  fi
  # A cdylib that exports nothing is a file, not a bridge.
  local so; so="$(find packages/chat-crypto-ffi/target -name 'librudi_chat_crypto_ffi.so' 2>/dev/null | head -1)"
  [ -n "$so" ] || { echo "không sinh ra thư viện dùng chung nào" >&2; return 1; }
  local sym missing=0
  for sym in rudi_chat_crypto_client_new rudi_chat_crypto_client_free \
             rudi_chat_crypto_string_free rudi_chat_crypto_identity \
             rudi_chat_crypto_create_group rudi_chat_crypto_encrypt \
             rudi_chat_crypto_receive; do
    nm -D --defined-only "$so" | grep -q " $sym\$" || { echo "$sym không được xuất" >&2; missing=1; }
  done
  [ "$missing" -eq 0 ]
}

do_chat-e2e() {
  # The only stage that drives the chat surface the way a phone does: real
  # HTTP, a real WebSocket, a real session, against the Go front door. The
  # runner provisions its own PostgreSQL, Python API and core, and refuses a
  # run with no sentinel, any SKIP, or a stack that never came up.
  scripts/chat_e2e_go.sh
}

do_e2e() {
  # Delegates for the same reason `do_postgres` does: the provisioning gets a
  # caller outside this file and can be tested on its own
  # (tests/test_e2e_slice_runner.py).
  #
  # This is the only stage where the client and the server are both real. Every
  # other one holds one side fixed and fakes the other: `api` runs on a fake
  # repository, `mobile` runs on a fake API, `client-routes` and `contract`
  # compare two files without executing either. A defect that lives in the seam
  # -- a body key the client spells differently from the server, a field the
  # server stopped returning -- is invisible to all of them and green in all of
  # them.
  #
  # Measured 2026-08-30 at 1649c16: nothing ran it. `npm test` prunes
  # `tests/e2e` by construction, the mobile job runs that same `npm test`, and
  # `grep -rn test:e2e` over every .yml, .sh, .py, .json and .md found one
  # definition in package.json and no caller at all.
  scripts/e2e_slice.sh
}

# --- prerequisites --------------------------------------------------------
#
# Answers exactly one of: run it, skip it with a reason, or fail because the
# thing is present but broken. The third case is the workflows' rule and it
# matters most: `apps/mobile` with no lockfile is a defect, not an absence.

check_prereq() {
  case "$1" in
    ruff)
      git rev-parse --git-dir >/dev/null 2>&1 || { echo "không phải git repo"; return 1; }
      # `ruff_changed.sh` is a ratchet, so an empty scope makes it print
      # "nothing for ruff to check" and exit 0 -- correct for the script, and a
      # lie once this file renders it as ĐẠT. `guard-range` refuses the
      # identical empty-range condition one stage above; this is the same
      # refusal for the same reason.
      #
      # It hid a real defect. Measured 2026-08-30 on main at 15b0e5c: standing
      # on origin/main the scope is empty, so this stage said ĐẠT, while the
      # same commit in a clone whose merge base was one commit back said HỎNG
      # over three files `ruff format` rejects. The moment you most want to ask
      # "is main clean?" was the moment the stage could not answer.
      #
      # Only a confident empty answer skips. No base, or the script erroring,
      # falls through and runs the body -- which reports the real problem far
      # more loudly than a skip line. A prereq check may turn a run into a skip
      # only when it is certain there is nothing to do.
      local rbase rlist
      rbase="$(guard_range_base)"
      [ -n "$rbase" ] || return 0
      rlist="$(scripts/ruff_changed.sh --list "$rbase" 2>/dev/null)" || return 0
      [ -n "$rlist" ] && return 0
      # An empty scope means ruff itself has nothing to lint. It does NOT mean
      # the stage has nothing to do: `do_ruff` opens with an assertion that has
      # no file list in it at all, and skipping here hid it for a whole day --
      # see ruff_pin() above for the measurement. So the last question before
      # skipping is the scopeless one, and a missing pin sends the run into the
      # body, where it fails with the name of the file that is actually wrong.
      #
      # Deliberately not answered here with `return 1`: a prereq failure is a
      # skip line and no log. The pin deserves a HỎNG with its reason printed,
      # which only the stage body can produce.
      [ -n "$(ruff_pin)" ] || return 0
      echo "nhánh không đổi file Python nào so với origin/main -- ruff không kiểm được gì"
      return 1 ;;
    guard-range)
      git rev-parse --git-dir >/dev/null 2>&1 || { echo "không phải git repo"; return 1; }
      # No base: let the body fail loudly rather than skipping quietly here.
      # An unanswerable question is a failure, not an absence.
      local base; base="$(guard_range_base)"
      [ -n "$base" ] || return 0
      # An empty range scans zero files and exits 0 -- "passed commit range: 0
      # file scan(s)". That is a pass this file must never hand out: it is the
      # green-because-nothing-ran shape, reproduced inside the stage meant to
      # stop it. On `main` itself there genuinely is nothing to scan, so the
      # honest answer is a skip with a reason, and --strict makes it loud.
      [ "$(git rev-list --count "$base"..HEAD)" -gt 0 ] || {
        echo "nhánh không thêm commit nào trên origin/main -- không có gì để quét"; return 1; } ;;
    go-postgres)
      [ -d services/core ] || { echo "services/core không có trên nhánh này"; return 1; }
      [ -f services/core/go.mod ] || return 2
      have docker && have go || { echo "cần docker và go"; return 1; }
      docker info >/dev/null 2>&1 || { echo "docker daemon không trả lời"; return 1; } ;;
    go-broker)
      # Docker is needed only for a service the caller did not hand over as a
      # URL; with all three set, a Docker-less machine runs this stage too.
      [ -d services/core ] || { echo "services/core không có trên nhánh này"; return 1; }
      [ -f services/core/go.mod ] || return 2
      [ -x scripts/go_broker_tier.sh ] || return 2
      have go || { echo "cần go"; return 1; }
      if [ -z "${CORE_TEST_DATABASE_URL:-}" ] || [ -z "${CORE_TEST_REDIS_URL:-}" ] || [ -z "${CORE_TEST_AMQP_URL:-}" ]; then
        have docker || { echo "cần docker, hoặc đặt sẵn CORE_TEST_DATABASE_URL, CORE_TEST_REDIS_URL, CORE_TEST_AMQP_URL"; return 1; }
        docker info >/dev/null 2>&1 || { echo "docker daemon không trả lời, và chưa đặt đủ ba CORE_TEST_*_URL"; return 1; }
      fi ;;
    parity)
      # ADR-0029. The harness needs docker for the stacks and go for itself;
      # missing either is a skip, and --strict makes it a failure.
      [ -d parity ] || { echo "parity/ không có trên nhánh này"; return 1; }
      [ -f parity/go.mod ] || return 2
      have docker && have go && have curl || { echo "cần docker, go và curl"; return 1; }
      docker info >/dev/null 2>&1 || { echo "docker daemon không trả lời"; return 1; } ;;
    python-touch)
      [ -d services/core ] || { echo "services/core không có trên nhánh này"; return 1; }
      [ -f services/core/ownership/routes.json ] || return 2
      git rev-parse --verify origin/main >/dev/null 2>&1 || { echo "không có origin/main để so"; return 1; } ;;
    ownership|go-vet|go-test)
      # ADR-0029. Absent services/core means the branch predates the Go front
      # door, and skipping says so. Present without go.mod is a defect: the
      # stage would find nothing to compile and report green.
      [ -d services/core ] || { echo "services/core không có trên nhánh này"; return 1; }
      [ -f services/core/go.mod ] || return 2
      command -v go >/dev/null 2>&1 || { echo "không có go trên PATH (toolchain ghim trong services/core/go.mod)"; return 1; }
      if [ "$1" = ownership ]; then
        python3 -c "import fastapi" 2>/dev/null || {
          echo "chưa cài fastapi (pip install -r services/api/requirements-dev.txt)"; return 1; }
      fi ;;
    contract)
      # Needs both sides of the contract. Without `apps/mobile` there is no
      # client to check and skipping is the honest answer; with it present but
      # no `src`, the checker itself would find nothing and report green, so
      # that case is a defect and refuses to skip.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -d apps/mobile/src ] || return 2
      python3 -c "import fastapi" 2>/dev/null || {
        echo "chưa cài fastapi (pip install -r services/api/requirements-dev.txt)"; return 1; } ;;
    client-routes)
      # Same two halves as `contract`, same reasoning for each outcome. The
      # question asked of them is different: `contract` asks whether a call
      # sends X-Actor-ID, this one asks whether the path it calls exists.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -d apps/mobile/src ] || return 2
      python3 -c "import fastapi" 2>/dev/null || {
        echo "chưa cài fastapi (pip install -r services/api/requirements-dev.txt)"; return 1; } ;;
    server-routes)
      # Same two halves as `client-routes`, asked the other way round. The
      # "present but no src" case is a defect here for a sharper reason than
      # elsewhere: with no client source to read, EVERY route on the server
      # looks uncalled, so a reader that skipped would be replaced by one that
      # reports 69 findings. The checker refuses that itself by exiting 2 when
      # it can read no path at all, and this refuses to skip past it.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -d apps/mobile/src ] || return 2
      python3 -c "import fastapi" 2>/dev/null || {
        echo "chưa cài fastapi (pip install -r services/api/requirements-dev.txt)"; return 1; } ;;
    screens)
      # Same two halves as the three stages above, and no fastapi: this one
      # reads client source only, so the API side is irrelevant to it.
      #
      # "Present but no screens" is a defect rather than a skip, and for the
      # sharpest version of the reason given above: with no screen file to
      # read, every screen is vacuously reachable and the run prints 0/0 and
      # exits 0 -- the green-because-nothing-ran shape, inside the stage that
      # exists to catch unreachable code. The checker refuses it itself by
      # exiting 2 on an empty read; this refuses to skip past it.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -d apps/mobile/src/screens ] || return 2 ;;
    cors)
      # Same two halves again, third question. `apps/mobile` absent is an
      # absence; present with no `src` is a defect, because a reader that
      # finds no header at all is the shape this gate exists to refuse -- and
      # the checker says so itself by exiting 2 rather than 0.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -d apps/mobile/src ] || return 2
      python3 -c "import fastapi" 2>/dev/null || {
        echo "chưa cài fastapi (pip install -r services/api/requirements-dev.txt)"; return 1; } ;;
    demo-watch)
      # Only this machine hosts the demo. On a CI runner or a fresh clone there
      # is no box on 8099 and no crontab of ours, so the question is meaningless
      # and the stage says so out loud instead of being red for everyone forever
      # -- which is how the `guard history` variant would have died.
      #
      # Two signals, either one enough, because they fail in opposite
      # directions. The crontab block says "this machine took on the job of
      # watching"; that alone must keep the stage running even while the box is
      # down, since a demo that stopped answering is exactly what wants
      # reporting. The live port says "there is a demo here"; that alone keeps
      # the stage running on a host that has one but never installed the
      # schedule -- the state this repo was in when 8099 drifted twice.
      #
      # The hole left: kill the container AND clear the crontab and this skips.
      # It is a skip with a printed reason, and --strict turns it into a
      # failure, which is the most this file can honestly claim.
      [ -f scripts/demo_watch.py ] || return 2
      if ! crontab -l 2>/dev/null | grep -q 'mobile-demo-watch'; then
        (exec 3<>/dev/tcp/127.0.0.1/8099) 2>/dev/null || {
          echo "máy này không dựng demo: không có khối cron canh gác, và 8099 không trả lời"
          return 1
        }
      fi ;;
    hero-walk)
      # Only this machine hosts the demo, so on a CI runner or a fresh clone the
      # question is meaningless and the stage says so rather than being red for
      # everyone forever. Deleting the runner is a different matter: that is the
      # one edit that must not turn this green.
      [ -f scripts/hero_walk.sh ] || return 2
      (exec 3<>/dev/tcp/127.0.0.1/8099) 2>/dev/null || {
        echo "máy này không dựng demo: 8099 không trả lời"
        return 1
      } ;;
    shared)
      have node || { echo "không có node"; return 1; }
      [ -d packages/shared ] || { echo "packages/shared không có trên nhánh này"; return 1; }
      [ -f packages/shared/money.test.mjs ] || return 2 ;;
    mobile)
      have npx || { echo "không có npx"; return 1; }
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -f apps/mobile/package-lock.json ] || return 2
      [ -d apps/mobile/node_modules ] || { echo "chưa 'npm ci' trong apps/mobile"; return 1; } ;;
    mobile-native)
      # Chặng duy nhất cần PHẦN CỨNG. Thiếu máy ảo hay thiếu maestro là bỏ qua
      # có lý do, không phải đỏ -- và `--strict` biến nó thành đỏ, đúng như mọi
      # chặng khác, để một lượt trước merge không đọc "thiếu công cụ" thành ĐẠT.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      [ -d apps/mobile/.maestro ] || return 2
      [ -d apps/mobile/node_modules ] || { echo "chưa 'npm ci' trong apps/mobile"; return 1; }
      have maestro || { echo "không có maestro trên PATH"; return 1; }
      command -v adb >/dev/null 2>&1 || [ -x "${ANDROID_HOME:-$HOME/Android/Sdk}/platform-tools/adb" ] \
        || { echo "không có adb (đặt ANDROID_HOME)"; return 1; }
      [ -n "$(ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}" PATH="${ANDROID_HOME:-$HOME/Android/Sdk}/platform-tools:$PATH" \
              timeout 30 adb devices 2>/dev/null | awk '$2=="device"{print $1; exit}')" ] \
        || { echo "không có máy ảo Android nào đang chạy"; return 1; } ;;
    pinned-import|docker)
      have docker || { echo "không có docker"; return 1; }
      docker info >/dev/null 2>&1 || { echo "docker daemon không chạy"; return 1; } ;;
    postgres)
      # An explicitly given URL still wins: somebody who aimed this at a
      # database on purpose is not to be second-guessed.
      #
      # Otherwise `scripts/postgres_tier.sh` makes one. The old rule here was
      # "only an explicitly given URL", and its reason was right -- guessing a
      # connection string would have pointed the tier at the shared
      # `mobile-local` database that every worktree on this machine uses. But
      # the conclusion cost more than it saved: the stage skipped on every run,
      # so 224 cases that are the only proof of any SQL, index, view or trigger
      # in this repository never executed. Provisioning has nothing to guess --
      # a container of its own, on a random loopback port, deleted on the way
      # out -- so the collision that rule protected against cannot happen.
      [ -n "${MOBILE_TEST_DATABASE_URL:-}" ] && return 0
      have docker || {
        echo "không có docker và chưa đặt MOBILE_TEST_DATABASE_URL"; return 1; }
      docker info >/dev/null 2>&1 || {
        echo "docker daemon không chạy và chưa đặt MOBILE_TEST_DATABASE_URL"; return 1; }
      docker image inspect "${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}" >/dev/null 2>&1 || {
        echo "chưa có ảnh postgres tại máy (docker pull postgres:16-alpine)"; return 1; } ;;
    crypto)
      # An absence skips, a defect fails: the crate missing is an absence, the
      # crate present without its canaries is not.
      [ -d packages/chat-crypto ] || { echo "packages/chat-crypto không có trên nhánh này"; return 1; }
      [ -f packages/chat-crypto/tests/mls_canaries.rs ] || return 2
      have cargo || { echo "không có cargo"; return 1; } ;;
    chat-e2e)
      # Same rule as e2e: an absence skips, a defect fails. Deleting the cases
      # must never be the thing that turns this stage green.
      [ -d services/core ] || { echo "services/core không có trên nhánh này"; return 1; }
      [ -d services/core/e2e/chat ] || { echo "chưa có tầng E2E chat trên nhánh này"; return 1; }
      [ -x scripts/chat_e2e_go.sh ] || return 2
      have go || { echo "không có go"; return 1; }
      have node || { echo "không có node"; return 1; }
      have docker || { echo "không có docker"; return 1; }
      docker info >/dev/null 2>&1 || { echo "docker daemon không chạy"; return 1; }
      docker image inspect "${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}" >/dev/null 2>&1 || {
        echo "chưa có ảnh postgres tại máy (docker pull postgres:16-alpine)"; return 1; } ;;
    e2e)
      # Needs both sides for real, so it asks for more than any other stage:
      # the client to drive, a Python that can serve the API, and Docker for
      # the database. Each missing piece is an absence and skips honestly.
      [ -d apps/mobile ] || { echo "apps/mobile không có trên nhánh này"; return 1; }
      # Present but missing the slice itself is a defect, not an absence.
      # Deleting the one file that proves client and server connect must never
      # be the thing that turns this stage green.
      [ -f apps/mobile/tests/e2e/vertical-slice.test.mjs ] || return 2
      have npm || { echo "không có npm"; return 1; }
      [ -d apps/mobile/node_modules ] || { echo "chưa 'npm ci' trong apps/mobile"; return 1; }
      python3 -c "import fastapi, uvicorn, alembic" 2>/dev/null || {
        echo "chưa cài fastapi/uvicorn/alembic (pip install -r services/api/requirements-dev.txt)"; return 1; }
      have docker || { echo "không có docker"; return 1; }
      docker info >/dev/null 2>&1 || { echo "docker daemon không chạy"; return 1; }
      docker image inspect "${MOBILE_TEST_POSTGRES_IMAGE:-postgres:16-alpine}" >/dev/null 2>&1 || {
        echo "chưa có ảnh postgres tại máy (docker pull postgres:16-alpine)"; return 1; } ;;
  esac
  return 0
}

# The "present but broken" message, kept next to the rule it enforces.
broken_why() {
  case "$1" in
    contract|client-routes|server-routes) echo "apps/mobile có mặt nhưng thiếu src/ -- từ chối bỏ qua" ;;
    screens) echo "apps/mobile có mặt nhưng thiếu src/screens -- 0/0 màn không phải ĐẠT" ;;
    shared) echo "packages/shared có mặt nhưng thiếu money.test.mjs -- từ chối bỏ qua" ;;
    mobile) echo "apps/mobile có mặt nhưng thiếu package-lock.json -- từ chối bỏ qua" ;;
    mobile-native) echo "apps/mobile có mặt nhưng thiếu .maestro -- xoá bảng flow không được biến chặng này thành xanh" ;;
    e2e) echo "apps/mobile có mặt nhưng thiếu tests/e2e/vertical-slice.test.mjs -- từ chối bỏ qua" ;;
    chat-e2e) echo "có services/core/e2e/chat nhưng thiếu scripts/chat_e2e_go.sh -- từ chối bỏ qua" ;;
    crypto) echo "có packages/chat-crypto nhưng thiếu tests/mls_canaries.rs -- từ chối bỏ qua" ;;
    ownership|go-vet|go-test) echo "services/core có mặt nhưng thiếu go.mod -- từ chối bỏ qua" ;;
    python-touch) echo "services/core có mặt nhưng thiếu ownership/routes.json -- từ chối bỏ qua" ;;
    parity) echo "parity/ có mặt nhưng thiếu go.mod -- từ chối bỏ qua" ;;
    go-postgres) echo "services/core có mặt nhưng thiếu go.mod -- từ chối bỏ qua" ;;
    go-broker) echo "services/core có mặt nhưng thiếu go.mod hoặc scripts/go_broker_tier.sh -- từ chối bỏ qua" ;;
    demo-watch) echo "thiếu scripts/demo_watch.py -- xoá canh gác không được biến chặng này thành xanh" ;;
    hero-walk) echo "thiếu scripts/hero_walk.sh -- xoá bài đi bộ không được biến chặng này thành xanh" ;;
    *) echo "thiếu file mà chặng này cần -- từ chối bỏ qua" ;;
  esac
}

# --- run ------------------------------------------------------------------

echo "Cổng chạy tại máy — cùng các lệnh mà .github/workflows chạy."
echo "HEAD $(git rev-parse --short HEAD 2>/dev/null || echo '?')  chặng: ${SELECTED[*]}$([ "$STRICT" -eq 1 ] && echo '  [strict]')"

for stage in "${SELECTED[@]}"; do
  banner "$stage"
  why="$(check_prereq "$stage")"; prereq=$?
  if [ "$prereq" -eq 2 ]; then
    # Present but broken. Never a skip.
    fail_noran "$stage" "$(broken_why "$stage")"
    continue
  fi
  if [ "$prereq" -ne 0 ]; then
    if [ "$STRICT" -eq 1 ]; then
      fail_noran "$stage" "strict: bỏ qua bị tính là hỏng -- $why"
    else
      skip "$stage" "$why"
    fi
    continue
  fi
  # The subshell isolates cwd (`do_mobile` cd's into apps/mobile). PIPESTATUS
  # rather than `$?`: `$?` after a pipe is tee's status, which is 0 even when
  # the stage failed -- a gate that reads the wrong exit code is worse than no
  # gate, because it reports green with evidence scrolling past saying red.
  start=$SECONDS
  ( cd "$REPO_ROOT" && "do_$stage" ) 2>&1 | tee "$LOG_DIR/$stage.log"
  rc=${PIPESTATUS[0]}
  if [ "$rc" -eq 0 ]; then
    pass "$stage" "$((SECONDS - start))"
  else
    fail "$stage" "$((SECONDS - start))"
  fi
done

# --- summary --------------------------------------------------------------

echo
echo "================ TỔNG KẾT ================"
printf 'ĐẠT %d   HỎNG %d   BỎ QUA %d\n' "${#PASSED[@]}" "${#FAILED[@]}" "${#SKIPPED[@]}"
[ ${#PASSED[@]}  -gt 0 ] && echo "  đạt:     ${PASSED[*]}"
[ ${#FAILED[@]}  -gt 0 ] && echo "  hỏng:    ${FAILED[*]}"
if [ ${#SKIPPED[@]} -gt 0 ]; then
  echo "  bỏ qua:"
  for w in "${SKIP_WHY[@]}"; do echo "    $w"; done
  echo
  echo "BỎ QUA KHÔNG PHẢI ĐẠT. Trước khi merge chạy lại với --strict."
fi

# --- verdict: did this run test what actually ships? ----------------------
#
# Every stage above answers "did the thing I ran work". None of them answers
# "was the thing I ran the thing that ships", and on 2026-08-30 the difference
# cost the team a morning: 2305 pytest cases green on fastapi 0.135.3 while the
# image, on the pinned 0.115.6, could not import the app at all. The demo
# machine stayed dead for hours.
#
# The lead's response was a rule held in a person's head -- "a PR that changes a
# route declaration does not merge until the docker stage is green". That is the
# right rule and the wrong enforcement: it had already been skipped on nearly
# every backend PR for two days, by the person who wrote it, because `pytest`
# and the mutation table were green and there was no reason on screen to run
# anything more. This block is that rule with the person taken out of it.
#
# The shape it refuses: a green that was earned on different software. It fires
# only when the run actually claims something about the application code --
# `guard` alone says nothing about libraries and must stay green without docker.
#
# It is deliberately NOT a stage. A stage can be deselected, and the hole being
# closed here IS deselection: `scripts/gate.sh api` printed "ĐẠT 1 HỎNG 0" and
# exited 0 while the tree could not boot. So the check runs on every invocation
# that reaches a verdict, costs milliseconds, and needs nothing but python3.
in_list() {
  local needle="$1" x
  shift
  for x in "$@"; do [ "$x" = "$needle" ] && return 0; done
  return 1
}

# Stages whose green is read as "the application code works".
DRIFT_CODE_TIERS=(api migration postgres e2e)
# Stages that load the app under the versions the image installs, and are
# therefore the only ones whose green survives a drifted machine.
DRIFT_SHIPPING_PROOF=(pinned-import docker)

drift_ran_code_tier=0
drift_proved_shipping=0
for s in ${PASSED[@]+"${PASSED[@]}"}; do
  in_list "$s" "${DRIFT_CODE_TIERS[@]}" && drift_ran_code_tier=1
  in_list "$s" "${DRIFT_SHIPPING_PROOF[@]}" && drift_proved_shipping=1
done

DRIFT_STATE="not-applicable"
DRIFT_NAMES=""
if [ "$drift_ran_code_tier" -eq 1 ]; then
  DRIFT_NAMES="$(python3 scripts/check_pin_drift.py --names-only 2>/dev/null)"
  case $? in
    0) DRIFT_STATE="clean" ;;
    1) DRIFT_STATE="drift" ;;
    # Could not measure. Never a silent pass -- an unreadable requirements file
    # or a python3 that cannot import its own metadata is a broken gate, and a
    # broken gate reporting green is the thing this whole file exists against.
    *) DRIFT_STATE="unknown" ;;
  esac
fi

DRIFT_BLOCKS=0
if [ "$DRIFT_STATE" = "drift" ] && [ "$drift_proved_shipping" -eq 0 ]; then
  DRIFT_BLOCKS=1
fi
[ "$DRIFT_STATE" = "unknown" ] && DRIFT_BLOCKS=1

# The escape hatch exists because the alternative is worse. A machine with no
# docker cannot run `pinned-import` at all, and a gate that is red with no way
# out on such a machine gets deleted within a day -- `do_guard-range` says the
# same thing about `repo_guard.py history` a few hundred lines up. So the way
# past is explicit, printed, and recorded in the summary a merge reads. It is
# not silent, which is the only property that matters.
if [ "$DRIFT_BLOCKS" -eq 1 ] && [ "${MOBILE_GATE_ALLOW_DRIFT:-0}" = "1" ]; then
  DRIFT_BLOCKS=0
  DRIFT_STATE="drift-waived"
fi

# The same counts, for a program rather than a person.
#
# `scripts/gate_merge.sh` has to tell "every stage ran and passed" apart from
# "the stages that mattered most never ran", and until now the only channel it
# had was the block above -- coloured, localised, written for a reader. It did
# not parse it, so it could not tell, so it printed an unconditional green over
# the top of the "BỎ QUA KHÔNG PHẢI ĐẠT" line three lines above its own verdict.
# Measured 2026-08-30 at ef2f5e8: `gate_merge.sh -- guard postgres e2e` with the
# postgres image unresolvable ran one stage, skipped `e2e` and `postgres`, and
# ended "ĐẠT gộp ... cho cây xanh", exit 0.
#
# Making the caller grep this banner would have made a merge decision depend on
# the wording of a heading. So the counts get a second, stable channel, and the
# reader is required to treat an absent or unparseable file as "cannot tell"
# rather than as "nothing was skipped" -- a caller that reads silence as good
# news rebuilds the exact bug this closes.
#
# Written before the exits below so every path reports: the failure path, the
# all-passed path, and the "nothing ran at all" path, which is the one whose
# count of zero is most easily misread as calm.
if [ -n "${GATE_SUMMARY_FILE:-}" ]; then
  {
    printf 'passed=%d\n' "${#PASSED[@]}"
    printf 'failed=%d\n' "${#FAILED[@]}"
    printf 'skipped=%d\n' "${#SKIPPED[@]}"
    for s in ${PASSED[@]+"${PASSED[@]}"}; do printf 'passed-stage=%s\n' "$s"; done
    for s in ${FAILED[@]+"${FAILED[@]}"}; do printf 'failed-stage=%s\n' "$s"; done
    # The reason travels with the name. A caller that can only print "2 chặng
    # bỏ qua" sends the reader back here to find out which and why.
    for w in ${SKIP_WHY[@]+"${SKIP_WHY[@]}"}; do printf 'skipped-stage=%s\n' "$w"; done
    # A merge decision needs to know the run tested what ships, not only that
    # it was green. Absent key = old gate.sh; the reader must treat that as
    # "cannot tell" rather than "clean", the same rule as the counts above.
    printf 'pin-drift=%s\n' "$DRIFT_STATE"
    for n in $DRIFT_NAMES; do printf 'pin-drift-name=%s\n' "$n"; done
  } > "$GATE_SUMMARY_FILE"
fi

# Guard the guard. If every stage was filtered away, the run proved nothing,
# and exiting 0 here would be this file committing the sin it was written for.
if [ ${#PASSED[@]} -eq 0 ] && [ ${#FAILED[@]} -eq 0 ]; then
  echo "Không chặng nào CHẠY -- cổng này không chứng minh gì cả." >&2
  exit 2
fi

if [ ${#FAILED[@]} -gt 0 ]; then
  ran_and_logged=0
  for f in "${FAILED[@]}"; do
    echo
    why="$(noran_why "$f")"
    if [ -n "$why" ]; then
      # Never started, so there is no log and never will be. Say that, rather
      # than printing a header promising thirty lines and then nothing --
      # silence under a promise of evidence reads as "ran, said nothing", which
      # is the opposite of what happened.
      echo "---- chặng hỏng: $f -- KHÔNG CHẠY, nên không có log ----"
      echo "$why"
    elif [ -f "$LOG_DIR/$f.log" ]; then
      echo "---- 30 dòng cuối của chặng hỏng: $f ----"
      tail -30 "$LOG_DIR/$f.log"
      ran_and_logged=1
    else
      # Neither ran-with-a-log nor recorded as never-run: the bookkeeping above
      # missed a path. Better to admit the gap than to print an empty block.
      echo "---- chặng hỏng: $f -- KHÔNG CHẠY hay mất log, cổng không biết ----"
      echo "Không có $LOG_DIR/$f.log và cũng không ghi được lý do. Đây là lỗi của chính scripts/gate.sh."
    fi
  done
  echo
  # Only point at the directory when something is actually in it.
  if [ "$ran_and_logged" -eq 1 ]; then
    echo "Log đầy đủ: $LOG_DIR"
  else
    echo "Không chặng hỏng nào chạy tới mức ghi log, nên $LOG_DIR rỗng."
  fi
  exit 1
fi

if [ "$DRIFT_BLOCKS" -eq 1 ]; then
  echo
  echo "================================================================"
  if [ "$DRIFT_STATE" = "unknown" ]; then
    echo "KHÔNG ĐO ĐƯỢC bản thư viện đang chạy."
    echo
    echo "scripts/check_pin_drift.py không trả lời được, nên cổng này không biết"
    echo "bộ test vừa chạy trên bản nào. Không biết thì không được báo xanh."
    python3 scripts/check_pin_drift.py >&2 || true
  else
    echo "MỌI CHẶNG ĐẠT — NHƯNG KHÔNG PHẢI TRÊN BẢN SẼ SHIP."
    echo
    echo "Các chặng vừa xanh chạy bằng thư viện của MÁY NÀY. Những pin quan"
    echo "trọng dưới đây khác bản mà ảnh cài, và chúng quyết định hành vi ngay"
    echo "lúc import — trước khi một assertion nào kịp chạy:"
    echo
    for n in $DRIFT_NAMES; do echo "    $n"; done
    echo
    echo "Đây đúng là hình dạng đã giết máy demo ngày 30/08: 2305 ca xanh tại"
    echo "chỗ, container không import nổi app. Chặng chứng minh được điều còn"
    echo "thiếu mất khoảng 2 giây:"
    echo
    echo "    scripts/gate.sh ${SELECTED[*]} pinned-import"
    echo
    echo "Máy không có docker thì nói ra chứ đừng lờ đi:"
    echo "    MOBILE_GATE_ALLOW_DRIFT=1 scripts/gate.sh ${SELECTED[*]}"
  fi
  echo "================================================================"
  echo "Log đầy đủ: $LOG_DIR"
  exit 1
fi

rm -rf "$LOG_DIR"
echo "Tất cả chặng đã chạy đều ĐẠT."
if [ "$DRIFT_STATE" = "drift-waived" ]; then
  echo
  echo "LƯU Ý: MOBILE_GATE_ALLOW_DRIFT=1 — pin quan trọng đang lệch và lượt này"
  echo "KHÔNG chứng minh được ảnh sẽ ship chạy được. Đã bỏ qua theo yêu cầu:"
  for n in $DRIFT_NAMES; do echo "    $n"; done
elif [ "$DRIFT_STATE" = "clean" ]; then
  echo "Và đã chạy đúng bản thư viện mà ảnh sẽ cài."
fi
exit 0
