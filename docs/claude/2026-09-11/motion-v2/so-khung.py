#!/usr/bin/env python3
"""Find screen transitions in a screen recording and measure how long each took.

Frames are compared with their predecessor. A frame that differs from the one
before it is «changing»; a run of consecutive changing frames is one transition.
A stack push that slides 300 ms at 30 fps is a run of ~9; an instant cut is a
run of 1 (the one frame in which the new screen appears). The clip must contain
ONLY navigation events (tap, wait, Back, wait): a scroll or a typing caret would
count as a transition too, and the caller keeps them out of the recording.

    so-khung.py <frames-dir> [--nguong 0.02] [--json out.json]

Prints `runs=[9, 8] max=9 gop=[9, 8] max_gop=9 events=2 chuoi=...` and exits 0.
`gop` merges runs separated by a SINGLE unchanged frame: `screenrecord` is
variable-frame-rate and `fps=30` duplicates a frame now and then, which would
split one slide into two short runs and let it pass a max-run gate (finish
review 11/09). The GATE belongs to the caller and reads `max_gop`: with the OS
animation scales at 1 a push must give max_gop >= 4 (positive control: the
gate can see a slide); with Reduce Motion on, max_gop <= 3 and max <= 2 for
every event (a cut is one frame; a content pop-in right after it is one more).
A scale-0 result is only readable beside a same-session scale-1 control.
Frames come out of a video encoder and are trusted only for WHERE a screen is,
never for sharpness or colour.
"""
import argparse, json, sys
from pathlib import Path
from PIL import Image, ImageChops, ImageStat

def nap(p: Path) -> Image.Image:
    # Downscale hard: the question is «did the screen move», not «is this pixel right».
    return Image.open(p).convert("L").resize((54, 120), Image.BILINEAR)

def khac(a: Image.Image, b: Image.Image) -> float:
    return ImageStat.Stat(ImageChops.difference(a, b)).mean[0] / 255.0

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("frames")
    ap.add_argument("--nguong", type=float, default=0.02, help="mean |diff| (0..1) vs previous frame above which the screen is moving")
    ap.add_argument("--json")
    a = ap.parse_args()
    files = sorted(Path(a.frames).glob("*.png"))
    if len(files) < 3:
        print(f"chỉ có {len(files)} khung", file=sys.stderr); return 2
    imgs = [nap(f) for f in files]
    dong = [khac(imgs[i - 1], imgs[i]) >= a.nguong for i in range(1, len(imgs))]
    chuoi = "." + "".join("x" if d else "." for d in dong)
    runs, n = [], 0
    for c in chuoi + ".":
        if c == "x": n += 1
        elif n: runs.append(n); n = 0
    # Merge runs separated by exactly one '.', counting that frame as part of the event.
    gop, n = [], 0
    for c in chuoi.replace("x.x", "xxx").replace("x.x", "xxx") + ".":
        if c == "x": n += 1
        elif n: gop.append(n); n = 0
    ket = {"khung": len(chuoi), "runs": runs, "max": max(runs) if runs else 0, "gop": gop, "max_gop": max(gop) if gop else 0,
           "events": len(gop), "nguong": a.nguong, "chuoi": chuoi}
    print(f"khung={ket['khung']} runs={runs} max={ket['max']} gop={gop} max_gop={ket['max_gop']} events={ket['events']} chuoi={chuoi}")
    if a.json:
        Path(a.json).write_text(json.dumps(ket, ensure_ascii=False, indent=1))
    return 0

if __name__ == "__main__":
    sys.exit(main())
