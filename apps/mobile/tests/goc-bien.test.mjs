/**
 * `gocBien`: a pixel transform origin that React Native's own parser accepts.
 *
 * The parser is the real one from node_modules, so a future React Native that
 * changes its grammar is measured too. Before the fix the stage behind a place
 * passed `${width / 2}px`, and a fractional width became four tokens.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { gocBien } from "../dist-test/rudi/ui/goc-bien.js";

// processTransformOrigin's token grammar, read from React Native itself.
const NGUON = readFileSync(new URL("../node_modules/react-native/Libraries/StyleSheet/processTransformOrigin.js", import.meta.url), "utf8");
const MAU = new RegExp(/TRANSFORM_ORIGIN_REGEX = \/(.+)\/gi;/.exec(NGUON)[1], "gi");
const soToken = (s) => [...s.matchAll(MAU)].length;

test("điểm xoay phân số làm parser của React Native thấy hơn ba token", () => {
  assert.ok(soToken(`${400.101 / 2}px ${30.0303}px`) > 3, "đối chứng: chuỗi cũ phải hỏng");
});

test("gocBien luôn cho đúng hai token px mà parser đọc được", () => {
  for (const [x, y] of [[196.36, 41.5], [205.09, 0.4], [0, 0], [180, 33]]) {
    const s = gocBien(x, y);
    assert.match(s, /^\d+px \d+px$/);
    assert.equal(soToken(s), 2, s);
  }
});
