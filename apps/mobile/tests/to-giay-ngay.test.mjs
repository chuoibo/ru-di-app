// Ngày trên tờ giấy: wire mang ISO, màn phải đọc ra tiếng Việt. Trên máy thật
// bản đầu in thẳng «2026-09-19» ra giữa tờ giấy (đo 14/09).
import assert from "node:assert/strict";
import test from "node:test";

import { ngayDocDuoc } from "../dist-test/rudi/to-giay/ngay.js";

test("ISO thành thứ và ngày tháng", () => {
  assert.equal(ngayDocDuoc("2026-09-19"), "Thứ Bảy 19/09");
  assert.equal(ngayDocDuoc("2026-09-14"), "Thứ Hai 14/09");
  assert.equal(ngayDocDuoc("2026-09-13"), "Chủ nhật 13/09");
  assert.equal(ngayDocDuoc("2026-01-01"), "Thứ Năm 01/01");
});

test("đọc theo UTC, không theo giờ máy", () => {
  // `new Date("2026-09-19")` ở một số máy đọc theo giờ địa phương và lùi một
  // ngày cho múi giờ âm; hai người cùng nhìn một buổi tối sẽ thấy hai ngày.
  const truoc = process.env.TZ;
  process.env.TZ = "America/Los_Angeles";
  assert.equal(ngayDocDuoc("2026-09-19"), "Thứ Bảy 19/09");
  process.env.TZ = truoc ?? "UTC";
});

test("chuỗi không đọc được trả lại nguyên văn, không thành rỗng", () => {
  // Một tờ giấy mất ngày là một tờ giấy không dùng được; một ngày lạ mắt vẫn
  // đọc được, và vẫn nói cho người đọc rằng có gì đó sai.
  assert.equal(ngayDocDuoc("Thứ Bảy 20/09"), "Thứ Bảy 20/09");
  assert.equal(ngayDocDuoc(""), "");
  assert.equal(ngayDocDuoc("2026-13-45"), "2026-13-45");
});
