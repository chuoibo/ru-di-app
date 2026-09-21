// Bounds check for F41: are «Thử lại» and «Bỏ» drawn, label and all, INSIDE the screen?
//
// Two things a view-tree assertion cannot see, both measured on the audit's own
// dump (docs/archive/codex/2026-09-09/native-audit-evidence/16-failed-sticker-dark.xml):
//
//   1. `uiautomator dump` CLAMPS bounds to the screen. The «Thử lại» button that
//      hung off the left edge is reported as [0,1209]→[195,1335], which looks
//      in frame. So `x0 >= 0` proves nothing.
//   2. A label that is entirely off screen has NO node at all. The «Bỏ» button
//      carries a child TextView «Bỏ»; the clipped «Thử lại» button carries none.
//
// So the gate is: every Button whose content-desc is one of the two labels must
// have a TextView child with the same text, that text must sit inside the
// button with at least `LE` px to spare on both sides (the button's own
// horizontal padding, 14dp ≈ 37px at 420dpi -- a label clipped by the screen
// edge is reported flush against the button's clamped edge, margin 0), and the
// button itself must lie within the screen. Buttons in one row must not overlap.
//
//   node kiem-bounds.mjs <hierarchy.xml> [<width>x<height>]   (default 1080x2400)
//
// Exit 0 = every button and label in frame and ≥ 48dp; 1 = clipped, missing, small or overlapping.
import { readFileSync } from "node:fs";

const [tep, coMan = "1080x2400"] = process.argv.slice(2);
if (!tep) {
  console.error("cần đường dẫn hierarchy.xml");
  process.exit(2);
}
const [W, H] = coMan.split("x").map(Number);
const LE = 30;
// 48dp Material target at 420dpi (2.625 px/dp) = 126px. The finish review of the
// first fix measured «Bỏ» at 124px: content-sized buttons need a width floor too.
const MAT_DO = 2.625;
const SAN_NUT = Math.round(48 * MAT_DO);
const NHAN = ["Thử lại", "Bỏ"];
const xml = readFileSync(tep, "utf8");

/** Every node in document order, flattened: {class, text, desc, bounds}. */
function cacNode() {
  const ra = [];
  const re = /<node\b([^>]*)\/?>/g;
  let m;
  while ((m = re.exec(xml)) !== null) {
    const a = m[1];
    const b = /\bbounds="\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]"/.exec(a);
    if (!b) continue;
    ra.push({
      lop: /\bclass="([^"]*)"/.exec(a)?.[1] ?? "",
      text: /\btext="([^"]*)"/.exec(a)?.[1] ?? "",
      desc: /\bcontent-desc="([^"]*)"/.exec(a)?.[1] ?? "",
      x0: +b[1], y0: +b[2], x1: +b[3], y1: +b[4],
    });
  }
  return ra;
}
const nodes = cacNode();
const trong = (a, b) => a.x0 >= b.x0 && a.y0 >= b.y0 && a.x1 <= b.x1 && a.y1 <= b.y1;
const man = { x0: 0, y0: 0, x1: W, y1: H };
const hop = (n) => `[${n.x0},${n.y0}]→[${n.x1},${n.y1}]`;

const nut = nodes.filter((n) => n.lop.endsWith("Button") && NHAN.includes(n.desc));
const chu = nodes.filter((n) => n.lop.endsWith("TextView") && NHAN.includes(n.text));
const dong = [];
let hong = false;

for (const b of nut) {
  const nhan = chu.filter((t) => t.text === b.desc && trong(t, b));
  const loi = [];
  if (!trong(b, man) || b.x1 <= b.x0) loi.push("nút ngoài màn");
  if (b.x1 - b.x0 < SAN_NUT || b.y1 - b.y0 < SAN_NUT) loi.push(`nút ${b.x1 - b.x0}×${b.y1 - b.y0}px dưới sàn ${SAN_NUT}px (48dp)`);
  if (nhan.length === 0) loi.push("KHÔNG có node chữ trong nút (chữ đã rơi khỏi màn)");
  for (const t of nhan) {
    if (t.x0 - b.x0 < LE || b.x1 - t.x1 < LE) loi.push(`chữ ${hop(t)} sát mép nút (lề ${t.x0 - b.x0}/${b.x1 - t.x1}px < ${LE})`);
  }
  if (loi.length > 0) hong = true;
  dong.push(`${loi.length ? "ĐỎ  " : "ok  "} «${b.desc}» ${hop(b)} rộng ${b.x1 - b.x0}px` + (loi.length ? `  ← ${loi.join("; ")}` : `  chữ ${nhan.map(hop).join(" ")}`));
}
if (!nut.some((b) => b.desc === "Thử lại")) {
  hong = true;
  dong.push("ĐỎ   không có nút «Thử lại» — hàng «hỏng, gửi lại được» chưa lên màn");
}
if (!nut.some((b) => b.desc === "Bỏ")) {
  hong = true;
  dong.push("ĐỎ   không có nút «Bỏ»");
}
for (const t of nut.filter((n) => n.desc === "Thử lại")) {
  for (const b of nut.filter((n) => n.desc === "Bỏ")) {
    if (t.x0 < b.x1 && b.x0 < t.x1 && t.y0 < b.y1 && b.y0 < t.y1) {
      hong = true;
      dong.push(`ĐỒ   «Thử lại» ${hop(t)} chồng «Bỏ» ${hop(b)}`);
    }
  }
}
console.log(`màn ${W}×${H}px · ${nut.length} nút · ${chu.length} node chữ`);
for (const d of dong) console.log("  " + d);
console.log(hong ? "ĐỎ: nút tràn, chữ mất, hoặc chồng nhau" : "XANH: mọi nút và nhãn nằm trọn trong màn");
process.exit(hong ? 1 : 0);
