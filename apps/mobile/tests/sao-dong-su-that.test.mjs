/**
 * A place's quiet line of facts starts with its rating, and a bare «4.8 (64)»
 * does not say what it counts: the line wears a star before it (critique
 * 02/10; the lead card lost its star when the price moved to a chip). The star
 * belongs to the rating only, so it shows when the line starts with it.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { moDauBangSao } from "../dist-test/rudi/kham-pha/dia-diem.js";

const sao = { icon: "star", text: "4.8 (64)" };
const gia = { icon: "wallet-outline", text: "200.000đ – 250.000đ mỗi người" };
const km = { icon: "navigate-outline", text: "1.2 km" };

test("dòng sự thật mở đầu bằng điểm đánh giá thì có sao, kể cả khi giá đứng trước nó", () => {
  assert.equal(moDauBangSao([sao, km, gia]), true);
  // The price leaves the line for its own place, so the line still opens on the rating.
  assert.equal(moDauBangSao([gia, sao, km]), true);
});

test("không có điểm đánh giá, hoặc dòng không mở đầu bằng nó, thì không có sao", () => {
  assert.equal(moDauBangSao([km, gia]), false);
  assert.equal(moDauBangSao([km, sao]), false);
  assert.equal(moDauBangSao([]), false);
});
