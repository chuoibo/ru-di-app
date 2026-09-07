import assert from "node:assert/strict";
import { test } from "node:test";
import { aiCoGi, cauMonCuaNguoi } from "../dist-test/rudi/chia-bill/ai-co-gi.js";

const LINES = [
  { id: "l1", name: "Bún bò" },
  { id: "l2", name: "   " },
  { id: "l3", name: "Trà đá" },
];
const NGUOI = [
  { id: "a", name: "An" },
  { id: "b", name: "Bình" },
];

test("đọc ngược bản gán: mỗi người thấy món của mình theo thứ tự bill, món trống tên thành «Món N»", () => {
  const bang = aiCoGi(LINES, NGUOI, { l1: ["a", "b"], l2: ["b"] });
  assert.deepEqual(bang.nguoi, [
    { id: "a", ten: "An", mon: ["Bún bò"] },
    { id: "b", ten: "Bình", mon: ["Bún bò", "Món 2"] },
  ]);
  assert.deepEqual(bang.chuaChon, ["Trà đá"]);
});

test("không ai chọn gì thì mọi món đều chờ, không người nào có món", () => {
  const bang = aiCoGi(LINES, NGUOI, {});
  assert.deepEqual(bang.chuaChon, ["Bún bò", "Món 2", "Trà đá"]);
  assert.ok(bang.nguoi.every((n) => n.mon.length === 0));
  assert.equal(cauMonCuaNguoi(bang.nguoi[0]), "Chưa có món nào");
});

test("câu dưới tên đếm món rồi liệt kê, không có tiền", () => {
  assert.equal(cauMonCuaNguoi({ id: "a", ten: "An", mon: ["Bún bò", "Trà đá"] }), "2 món · Bún bò, Trà đá");
});
