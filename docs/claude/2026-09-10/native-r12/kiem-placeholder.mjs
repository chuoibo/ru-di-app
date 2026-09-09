// One-line check for F44: every search placeholder node in a hierarchy dump must
// be ONE line tall at the font scale it was captured at.
//
// Android's native hint wrapped onto a second line and was clipped (audit ảnh
// 20). The kit now draws the placeholder as a `Text numberOfLines={1}`, so the
// visible node's height has to be a single line. Measured on this emulator, RN
// on Android does NOT scale the kit's 24dp line-height with the font scale: the
// one-line node is 63px at 1.0, 70px at 1.3 and 95px at 2.0 -- the line box is
// max(line-height, glyph height), with body at 17sp. So the ceiling is
// max(24dp, 17sp × fs) × 1.35: a wrapped node is at least two lines (≈ 2×) and
// never clears it, and the margin does not shrink to 1px at 2.0 the way a
// linear 24dp × fs × 1.5 ceiling did (finish review 10/09, non-material #3).
//
//   node kiem-placeholder.mjs <hierarchy.xml> <fontScale> [<width>x<height>]
//
// Also refuses any text node that carries `http` -- the F43 half of the same
// board. Exit 0 = clean, 1 = a placeholder is taller than one line, missing,
// or a URL is on screen.
import { readFileSync } from "node:fs";

const [tep, fsRaw, coMan = "1080x2400"] = process.argv.slice(2);
if (!tep || !fsRaw) {
  console.error("cần <hierarchy.xml> <fontScale>");
  process.exit(2);
}
const fs = Number(fsRaw);
const MAT_DO = 2.625;
const GLYPH = 17 * fs * MAT_DO;
const DONG = Math.max(24 * MAT_DO, GLYPH);
const TRAN = Math.round(DONG * 1.35);
const [W] = coMan.split("x").map(Number);
const xml = readFileSync(tep, "utf8");

const nodes = [];
for (const m of xml.matchAll(/<node\b([^>]*)\/?>/g)) {
  const a = m[1];
  const b = /\bbounds="\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]"/.exec(a);
  if (!b) continue;
  nodes.push({
    lop: /\bclass="([^"]*)"/.exec(a)?.[1] ?? "",
    text: /\btext="([^"]*)"/.exec(a)?.[1] ?? "",
    x0: +b[1], y0: +b[2], x1: +b[3], y1: +b[4],
  });
}
const placeholder = nodes.filter((n) => n.lop.endsWith("TextView") && /^Tìm /.test(n.text));
const url = nodes.filter((n) => /https?:\/\/|localhost|\d+\.\d+\.\d+\.\d+:\d+/.test(n.text));
let hong = false;
console.log(`cỡ chữ ${fs} · một dòng ≈ ${Math.round(DONG)}px · trần ${TRAN}px · ${placeholder.length} placeholder`);
if (placeholder.length === 0) {
  hong = true;
  console.log("  ĐỎ   không có node placeholder nào bắt đầu bằng «Tìm » trên màn");
}
for (const n of placeholder) {
  const cao = n.y1 - n.y0;
  const ok = cao <= TRAN && n.x0 >= 0 && n.x1 <= W;
  if (!ok) hong = true;
  console.log(`  ${ok ? "ok " : "ĐỎ "}  «${n.text.slice(0, 40)}» cao ${cao}px rộng ${n.x1 - n.x0}px [${n.x0},${n.y0}]→[${n.x1},${n.y1}]`);
}
for (const n of url) {
  hong = true;
  console.log(`  ĐỎ   node chữ mang địa chỉ máy chủ: «${n.text.slice(0, 80)}»`);
}
console.log(hong ? "ĐỎ" : "XANH: mọi placeholder một dòng, không URL trên màn");
process.exit(hong ? 1 : 0);
