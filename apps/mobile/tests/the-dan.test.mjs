/**
 * The lead card of Khám phá (owner's mockup, 01/10): the price as its own chip,
 * whole — money is never cut with «…» — and every other fact as one quiet line,
 * so the lead never says less than the compare cards under it.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { tachTheDan } from "../dist-test/rudi/kham-pha/the-dan.js";

test("giá tách ra chip riêng, nguyên văn, không cắt", () => {
  const gia = "200.000đ – 250.000đ mỗi người";
  const r = tachTheDan([
    { icon: "star", text: "4,8" },
    { icon: "navigate-outline", text: "1,2 km" },
    { icon: "wallet-outline", text: gia },
    { icon: "time-outline", text: "Mở tới 22:00" },
  ]);
  assert.equal(r.gia, gia);
  assert.equal(r.phu, "4,8 · 1,2 km · Mở tới 22:00");
});

test("không có giá thì không có chip giá; không có gì thì dòng phụ rỗng", () => {
  assert.deepEqual(tachTheDan([{ icon: "star", text: "4,8" }]), { gia: null, phu: "4,8" });
  assert.deepEqual(tachTheDan([]), { gia: null, phu: "" });
});
