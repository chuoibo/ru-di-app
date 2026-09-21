#!/usr/bin/env bash
# Mutation gate for the W9 port. Every mutant in mutants.json is applied to a
# COPY of one tracked file and handed to the Go toolchain through `-overlay`,
# so the working tree is never edited and a run that dies half way leaves
# nothing behind.
#
# Each mutant must:
#   1. land -- `find` occurs exactly once in the tracked file, or the run stops;
#   2. compile -- `go build -overlay` must succeed, or it proves nothing;
#   3. be caught -- `go test -overlay` must FAIL. A mutant that passes is a
#      survivor: a gap in the corpus, reported rather than hidden.
#
# Run from services/core:
#
#	tools/mutants/w9/run.sh            # every mutant
#	tools/mutants/w9/run.sh steps-     # only those whose name has this prefix
set -u
set -o pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
filter=${1:-}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

readarray -t names < <(python3 -c '
import json, sys
with open(sys.argv[1]) as handle:
    for mutant in json.load(handle)["mutants"]:
        print(mutant["name"])
' "$here/mutants.json")

killed=0
survived=0
broken=0
declare -a survivors=()
declare -a unbuilt=()

for name in "${names[@]}"; do
	case "$name" in "$filter"*) ;; *) continue ;; esac

	# Write the mutated copy and the overlay that points at it. The helper
	# asserts the anchor is present exactly the declared number of times.
	if ! read -r overlay pkg gotest < <(python3 - "$here/mutants.json" "$root" "$work" "$name" <<'PY'
import json, os, sys

table, root, work, name = sys.argv[1:5]
with open(table) as handle:
    mutants = {m["name"]: m for m in json.load(handle)["mutants"]}
mutant = mutants[name]
source = os.path.join(root, mutant["file"])
with open(source, encoding="utf-8") as handle:
    text = handle.read()
count = mutant.get("count", 1)
found = text.count(mutant["find"])
if found != count:
    sys.exit(f"{name}: anchor occurs {found} times in {mutant['file']}, want {count}")
mutated = text.replace(mutant["find"], mutant["replace"], count)
if mutated == text:
    sys.exit(f"{name}: the replacement changes nothing")
target = os.path.join(work, name + ".go")
with open(target, "w", encoding="utf-8") as handle:
    handle.write(mutated)
overlay = os.path.join(work, name + ".overlay.json")
with open(overlay, "w", encoding="utf-8") as handle:
    json.dump({"Replace": {source: target}}, handle)
print(overlay, "./" + os.path.dirname(mutant["file"]), mutant["test"])
PY
	); then
		echo "ANCHOR-LOST  $name"
		broken=$((broken + 1))
		continue
	fi

	if ! (cd "$root" && CGO_ENABLED=0 go build -overlay "$overlay" ./... >"$work/$name.build" 2>&1); then
		echo "NO-COMPILE   $name"
		sed -n '1,4p' "$work/$name.build"
		unbuilt+=("$name")
		broken=$((broken + 1))
		continue
	fi

	if (cd "$root" && go test -count=1 -overlay "$overlay" -run "$gotest" "$pkg" >"$work/$name.test" 2>&1); then
		echo "SURVIVED     $name"
		survivors+=("$name")
		survived=$((survived + 1))
	else
		reds=$(grep -c 'Python \|  Python' "$work/$name.test" || true)
		echo "killed       $name (${reds} case(s) red)"
		killed=$((killed + 1))
	fi
done

echo
echo "killed $killed, survived $survived, unusable $broken"
if ((survived)); then
	printf 'survivor: %s\n' "${survivors[@]}"
fi
if ((broken)); then
	printf 'unusable: %s\n' "${unbuilt[@]:-}"
	exit 2
fi
((survived == 0))
