/**
 * The violet surface, and the one mistake it invites.
 *
 * `aiInk` is the ink for text sitting ON the solid `ai` fill -- white in the
 * light scheme, near-black in the dark one. `aiSoft` is the pale wash used as a
 * BACKGROUND. Put `aiInk` on `aiSoft` and you get white on near-white: 1.17:1
 * in light, 1.33:1 in dark. The label is still in the DOM, the snapshot still
 * renders, every test still passes, and nobody can read it.
 *
 * That is exactly what happened: `NepBang` shipped its «Mình đang thấy» label
 * that way, and the chat tray copied the pairing from it. It was found by
 * opening a screenshot, because nothing else in this repository looks at a
 * colour pair -- `rudi-khong-hex` only checks that colours come from tokens,
 * and the .tsx contrast gate reads the other app's idiom.
 *
 * So this file gates the pair rather than the pixels: the token that MAY sit on
 * `aiSoft` must clear 4.5:1, and the token that may not must be provably unable
 * to, so a future reader can see why the rule exists rather than being asked to
 * trust it.
 */
import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const tokens = JSON.parse(readFileSync(new URL("../../../packages/shared/tokens.json", import.meta.url), "utf8"));

const kenh = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
function doSang(hex) {
  const h = hex.replace("#", "");
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255);
  return 0.2126 * kenh(r) + 0.7152 * kenh(g) + 0.0722 * kenh(b);
}
function tuongPhan(a, b) {
  const [x, y] = [doSang(a), doSang(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

test("chữ trên nền aiSoft dùng token `ai`, và nó đọc được ở cả hai chủ đề", () => {
  for (const scheme of ["light", "dark"]) {
    const c = tokens.color[scheme];
    const ti = tuongPhan(c.ai, c.aiSoft);
    assert.ok(ti >= 4.5, `${scheme}: ai trên aiSoft chỉ ${ti.toFixed(2)}:1, cần ít nhất 4.5:1`);
  }
});

test("aiInk trên aiSoft là cặp KHÔNG đọc được, nên luật cấm nó có cơ sở đo được", () => {
  for (const scheme of ["light", "dark"]) {
    const c = tokens.color[scheme];
    const ti = tuongPhan(c.aiInk, c.aiSoft);
    assert.ok(ti < 3, `${scheme}: aiInk trên aiSoft là ${ti.toFixed(2)}:1; nếu cặp này đã đọc được thì bài test dưới không còn lý do tồn tại`);
  }
});

test("không file nào vừa tô nền aiSoft vừa viết chữ bằng aiInk", () => {
  const goc = new URL("../src/", import.meta.url).pathname;
  const viPham = [];
  let daDoc = 0;
  const di = (thu) => {
    for (const ten of readdirSync(thu)) {
      const duong = join(thu, ten);
      if (statSync(duong).isDirectory()) { di(duong); continue; }
      if (!/\.tsx?$/.test(ten)) continue;
      const raw = readFileSync(duong, "utf8");
      daDoc += 1;
      if (raw.includes("colors.aiSoft") && raw.includes("colors.aiInk")) viPham.push(duong.slice(goc.length));
    }
  };
  di(goc);
  // A scan that read nothing is indistinguishable from a scan that found
  // nothing, and this one has to be able to fail.
  assert.ok(daDoc > 100, `chỉ đọc được ${daDoc} file nguồn; cổng đang nhìn vào chỗ trống`);
  assert.deepEqual(viPham, [], `đặt aiInk lên nền aiSoft là chữ trắng trên nền trắng:\n${viPham.join("\n")}`);
});
