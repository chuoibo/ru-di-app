import assert from "node:assert/strict";
import { test } from "node:test";

import { chiaKeo, dauLich, homNay, nhanNhip, nhipKeo } from "../dist-test/rudi/keo/nhip-keo.js";

test("kèo sắp tới đếm ngày tới ngày bắt đầu, không phải ngày kết thúc", () => {
  assert.deepEqual(nhipKeo("2026-10-17", "2026-10-19", "2026-09-06"), { kieu: "sap-toi", conNgay: 41 });
  assert.equal(nhanNhip(nhipKeo("2026-09-07", "2026-09-07", "2026-09-06")), "Ngày mai");
});

test("hôm nay, đang diễn ra và đã qua là ba nhịp khác nhau", () => {
  assert.deepEqual(nhipKeo("2026-09-06", "2026-09-06", "2026-09-06"), { kieu: "hom-nay" });
  assert.deepEqual(nhipKeo("2026-09-05", "2026-09-08", "2026-09-06"), { kieu: "dang-dien-ra", conNgay: 2 });
  assert.deepEqual(nhipKeo("2026-09-04", "2026-09-06", "2026-09-06"), { kieu: "dang-dien-ra", conNgay: 0 });
  assert.deepEqual(nhipKeo("2026-09-01", "2026-09-03", "2026-09-06"), { kieu: "da-qua", truocNgay: 3 });
  assert.equal(nhanNhip({ kieu: "da-qua", truocNgay: 1 }), "Hôm qua");
});

test("ngày không đọc được không thành một con số bịa", () => {
  assert.deepEqual(nhipKeo("2026-02-30", "2026-03-01", "2026-09-06"), { kieu: "khong-ro" });
  assert.equal(nhanNhip({ kieu: "khong-ro" }), "");
  assert.equal(dauLich("2026-13-01"), null);
});

test("chia kèo: sắp tới gần nhất trước, đã qua mới nhất trước, ngày hỏng xếp cuối phần sắp tới", () => {
  const keo = [
    { id: "xa", starts_on: "2026-12-01", ends_on: "2026-12-02" },
    { id: "gan", starts_on: "2026-09-10", ends_on: "2026-09-10" },
    { id: "cu1", starts_on: "2026-08-01", ends_on: "2026-08-02" },
    { id: "cu2", starts_on: "2026-08-20", ends_on: "2026-08-21" },
    { id: "hong", starts_on: "??", ends_on: "??" },
  ];
  const { sapToi, daQua } = chiaKeo(keo, "2026-09-06");
  assert.deepEqual(sapToi.map((k) => k.id), ["gan", "xa", "hong"]);
  assert.deepEqual(daQua.map((k) => k.id), ["cu2", "cu1"]);
});

test("dấu lịch in ngày không có số 0 đứng đầu và tháng bằng chữ", () => {
  assert.deepEqual(dauLich("2026-10-07"), { ngay: "7", thang: "tháng 10" });
  assert.match(homNay(new Date(2026, 8, 6)), /^2026-09-06$/);
});
