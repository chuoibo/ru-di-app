// Taste in a couple's notebook, worded (ADR-0034 §2.1–2.2).
import assert from "node:assert/strict";
import test from "node:test";

import { cauGu, noiDanhSach } from "../dist-test/rudi/to-giay/gu-doi.js";

const gu = (over = {}) => ({ mine_shared: false, theirs_shared: false, theirs: [], common: [], ...over });

test("ngoài «Một đôi» không có câu nào", () => {
  assert.equal(cauGu(null, "Minh"), null);
  assert.equal(cauGu(undefined, "Minh"), null);
});

test("chưa ai bật: không nói gu của ai", () => {
  assert.deepEqual(cauGu(gu(), "Minh"), { chung: null, cuaHo: null, cuaToi: "Gu của bạn đang để riêng." });
});

test("người kia bật thì thấy gu họ bằng nhãn, bỏ id lạ", () => {
  const c = cauGu(gu({ theirs_shared: true, theirs: ["cafe", "outdoor", "tag-la"] }), "Minh");
  assert.equal(c.cuaHo, "Minh thích Cafe và Outdoor.");
  assert.equal(c.chung, null, "gu chung cần cả hai bật");
});

test("cả hai bật: gu chung, kể cả khi chưa trùng", () => {
  assert.equal(cauGu(gu({ mine_shared: true, theirs_shared: true, theirs: ["cafe"], common: ["cafe"] }), "Minh").chung, "Hai bạn cùng thích Cafe.");
  assert.match(cauGu(gu({ mine_shared: true, theirs_shared: true, theirs: ["game"] }), "Minh").chung, /chưa trùng gu/);
  assert.equal(cauGu(gu({ mine_shared: true }), "Minh").cuaToi, "Minh thấy gu của bạn trong sổ này.");
});

test("nối danh sách kiểu tiếng Việt", () => {
  assert.equal(noiDanhSach([]), "");
  assert.equal(noiDanhSach(["A"]), "A");
  assert.equal(noiDanhSach(["A", "B", "C"]), "A, B và C");
});
