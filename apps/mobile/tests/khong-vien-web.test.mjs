/* B7 (QC 24/09): mọi ô nhập trên web tắt viền của trình duyệt, vì ô nào cũng
 * tự vẽ viền focus của nó.
 *
 * Chạy từ apps/mobile:
 *     node --test tests/khong-vien-web.test.mjs
 *
 * Quét mọi `<TextInput` trong src/rudi: prop `style` của nó phải mang
 * KHONG_VIEN_WEB. Không chứng minh trình duyệt vẽ đúng -- đó là việc của ảnh
 * chụp khi một ô đang focus.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const GOC = new URL("../src/rudi", import.meta.url).pathname;

function tep(dir) {
  const ra = [];
  for (const ten of readdirSync(dir)) {
    const p = join(dir, ten);
    if (statSync(p).isDirectory()) ra.push(...tep(p));
    else if (ten.endsWith(".tsx")) ra.push(p);
  }
  return ra;
}

/** Each `<TextInput …>` element's own text, up to its first `/>` or `>` at depth 0 of braces. */
function theTextInput(nguon) {
  const ra = [];
  let i = nguon.indexOf("<TextInput");
  while (i >= 0) {
    let sau = 0;
    let j = i + 10;
    for (; j < nguon.length; j += 1) {
      const c = nguon[j];
      if (c === "{") sau += 1;
      else if (c === "}") sau -= 1;
      else if (c === ">" && sau === 0) break;
    }
    ra.push(nguon.slice(i, j + 1));
    i = nguon.indexOf("<TextInput", j);
  }
  return ra;
}

test("mọi TextInput trong vỏ RuDi tắt viền trình duyệt trên web", () => {
  let dem = 0;
  const thieu = [];
  for (const f of tep(GOC)) {
    for (const the of theTextInput(readFileSync(f, "utf8"))) {
      dem += 1;
      const style = the.match(/\bstyle=\{([\s\S]*?)\}\s*(?:\w+=|\/?>)/);
      if (!style || !style[1].includes("KHONG_VIEN_WEB")) thieu.push(`${f.slice(GOC.length + 1)}: ${the.slice(0, 60).replace(/\s+/g, " ")}`);
    }
  }
  assert.ok(dem >= 6, `chỉ thấy ${dem} TextInput: bộ quét đã hỏng`);
  assert.deepEqual(thieu, [], "TextInput còn viền trình duyệt trên web");
});
