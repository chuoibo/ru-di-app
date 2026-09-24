import assert from "node:assert/strict";
import { test } from "node:test";

import { cauSoPhan, goiYTien, hienO, loiO } from "../dist-test/rudi/chia-bill/o-so.js";

test("ô tiền rời đi thì hiện số có dấu chấm, không hiện số thô", () => {
  assert.equal(hienO("tien", 350000), "350.000");
  assert.equal(hienO("tien", 0), "");
  assert.equal(hienO("so-luong", 12), "12");
});

test("ô số lượng xoá trống được lúc đang gõ; rỗng chỉ là lỗi, không bị chặn gõ", () => {
  assert.match(loiO("so-luong", ""), /Ít nhất 1/);
  assert.match(loiO("so-luong", "0"), /Ít nhất 1/);
  assert.equal(loiO("so-luong", "2"), null);
  assert.match(loiO("so-luong", "150"), /99/);
  assert.match(loiO("so-luong", "hai"), /Chỉ gõ số/);
});

test("ô tiền: rỗng là 0 đồng (món được mời), số quá lớn thì nói", () => {
  assert.equal(loiO("tien", ""), null);
  assert.equal(loiO("tien", "350.000"), null);
  assert.match(loiO("tien", "9".repeat(20)), /lớn quá/);
  assert.match(loiO("tien", "35k"), /Chỉ gõ số/);
});

test("gợi ý nói rõ thành tiền là tổng cả dòng; giá mỗi phần chỉ khi chia chẵn", () => {
  assert.equal(goiYTien(2, 70000), "Tổng cả dòng · 35.000đ mỗi phần.");
  assert.equal(goiYTien(3, 100000), "Tổng cả dòng, không phải giá một phần.");
  assert.equal(goiYTien(1, 70000), "Tổng cả dòng, không phải giá một phần.");
  assert.equal(cauSoPhan(12), "12 phần");
  assert.equal(cauSoPhan(1), null);
});
