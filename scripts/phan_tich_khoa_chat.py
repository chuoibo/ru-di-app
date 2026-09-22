"""Summarise the lock sampler by minute so incidents can be read off the file.

The sampler writes a heartbeat line per tick and, when anything is waiting, the
waiting locks and the queries of whoever blocks them. Counting those per minute
is what turns "there were waits" into "the waits were here, and the holder was
doing this".
"""

import collections
import re
import sys

path = sys.argv[1]
minute = collections.defaultdict(
    lambda: {"ticks": 0, "waiting": 0, "tuple": 0, "holder": collections.Counter()}
)
current = None
clock = re.compile(
    r"^(\d\d):(\d\d):(\d\d)\.\d+ active=(\d+) idletx=(\d+) waiting=(\d+)"
)
for line in open(path, encoding="utf-8", errors="replace"):
    line = line.rstrip("\n")
    m = clock.match(line)
    if m:
        current = f"{m.group(1)}:{m.group(2)}"
        bucket = minute[current]
        bucket["ticks"] += 1
        bucket["waiting"] += int(m.group(6))
        continue
    if current is None:
        continue
    if line.startswith("  WAIT tuple"):
        minute[current]["tuple"] += 1
    elif line.startswith("  HOLDER "):
        parts = line.split()
        event = parts[2] if len(parts) > 2 else "?"
        wait = parts[3] if len(parts) > 3 else "?"
        minute[current]["holder"][f"{event}/{wait}"] += 1

print(f"{'phút':6} {'tick':>5} {'chờ':>6} {'tuple':>6}  kẻ giữ (top 2)")
for key in sorted(minute):
    b = minute[key]
    if not (b["waiting"] or b["tuple"] or b["holder"]):
        continue
    top = ", ".join(f"{k}x{v}" for k, v in b["holder"].most_common(2))
    print(f"{key:6} {b['ticks']:5d} {b['waiting']:6d} {b['tuple']:6d}  {top}")
print()
total = collections.Counter()
for b in minute.values():
    total.update(b["holder"])
print("Tổng kẻ giữ theo wait event:")
for k, v in total.most_common():
    print(f"  {v:5d}  {k}")
