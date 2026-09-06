/* Chat bubble themes (L1, ADR-0021 §2.4): five slugs, two schemes, one floor.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-mau-chat.test.mjs
 *
 * The server tier proves tokens ↔ domain ↔ migration agree; this file proves
 * the client reads those tokens without inventing a colour, that the default
 * theme IS the brand accent (so every existing screenshot holds), that ink on
 * bubble clears 4.5:1 in both schemes, and that an unknown slug falls back to
 * the default rather than to `undefined`.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { THEME_CHAT, bangMauChat, laThemeChat, nhanTheme } from "../dist-test/rudi/mau-chat.js";

const tokens = JSON.parse(readFileSync(new URL("../../../packages/shared/tokens.json", import.meta.url), "utf8"));
const HEX = /^#[0-9a-f]{6}$/;

function doSang(hex) {
  const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const lin = c.map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2];
}
function tuongPhan(a, b) {
  const [la, lb] = [doSang(a), doSang(b)];
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

test("năm slug đúng thứ tự tokens.json; mỗi slug × scheme đủ ba khoá hex", () => {
  assert.deepEqual([...THEME_CHAT], Object.keys(tokens.chatTheme));
  for (const slug of THEME_CHAT) {
    for (const dark of [false, true]) {
      const bang = bangMauChat(slug, dark);
      assert.deepEqual(Object.keys(bang), ["bubble", "bubbleInk", "accent"], slug);
      for (const v of Object.values(bang)) assert.match(v, HEX, `${slug}/${dark}`);
      assert.deepEqual(bang, tokens.chatTheme[slug][dark ? "dark" : "light"]);
    }
  }
});

test("mac-dinh là accent/accentInk của từng scheme -- ảnh flow cũ không đổi", () => {
  for (const dark of [false, true]) {
    const scheme = dark ? "dark" : "light";
    const bang = bangMauChat("mac-dinh", dark);
    assert.equal(bang.bubble, tokens.color[scheme].accent);
    assert.equal(bang.bubbleInk, tokens.color[scheme].accentInk);
    assert.equal(bang.accent, tokens.color[scheme].accent);
  }
});

test("bubbleInk trên bubble ≥ 4.5:1 ở cả mười bảng", () => {
  for (const slug of THEME_CHAT) {
    for (const dark of [false, true]) {
      const bang = bangMauChat(slug, dark);
      const c = tuongPhan(bang.bubbleInk, bang.bubble);
      assert.ok(c >= 4.5, `${slug}/${dark ? "dark" : "light"}: ${c.toFixed(2)}`);
    }
  }
});

test("slug lạ, null, undefined đều rơi về mac-dinh; nhãn có tiếng Việt", () => {
  for (const dark of [false, true]) {
    const macDinh = bangMauChat("mac-dinh", dark);
    assert.deepEqual(bangMauChat("hong", dark), macDinh);
    assert.deepEqual(bangMauChat(null, dark), macDinh);
    assert.deepEqual(bangMauChat(undefined, dark), macDinh);
    assert.deepEqual(bangMauChat("#c93900", dark), macDinh);
  }
  assert.equal(laThemeChat("bien-dem"), true);
  assert.equal(laThemeChat("BIEN-DEM"), false);
  assert.equal(nhanTheme("hoang-hon"), "Hoàng hôn");
  assert.equal(nhanTheme("khong-co"), "Mặc định");
  for (const slug of THEME_CHAT) assert.ok(!nhanTheme(slug).includes("—"));
});

test("bảng trả về là bản sao: sửa nó không sửa tokens", () => {
  const bang = bangMauChat("ruc-ro", false);
  bang.bubble = "x";
  assert.equal(bangMauChat("ruc-ro", false).bubble, tokens.chatTheme["ruc-ro"].light.bubble);
});
