import assert from "node:assert/strict";
import { test } from "node:test";

import { chuanGio, loiGio, loiThuTu, ngayChonDuoc, ngayNgan } from "../dist-test/rudi/to-giay/sua-to.js";
import { khacGi } from "../dist-test/rudi/to-giay/to-giay.js";

test("ngày chọn được: từ hôm nay tới hết tuần sau, không có ngày đã qua", () => {
  const ngay = ngayChonDuoc("2026-09-21", "2026-09-24", "2026-09-26");
  assert.equal(ngay[0], "2026-09-24");
  assert.equal(ngay.at(-1), "2026-10-04");
  assert.equal(ngay.length, 11);
  assert.ok(ngay.includes("2026-09-26"));
});

test("ngày đang có trên tờ luôn còn trong danh sách, kể cả khi đã qua", () => {
  const ngay = ngayChonDuoc("2026-09-21", "2026-09-24", "2026-09-22");
  assert.equal(ngay[0], "2026-09-22");
  assert.deepEqual(ngayChonDuoc("x", "2026-09-24", "y"), []);
});

test("tên ngắn của chip", () => {
  assert.equal(ngayNgan("2026-09-26"), "T7 26/09");
  assert.equal(ngayNgan("2026-09-27"), "CN 27/09");
  assert.equal(ngayNgan("2026-09-21"), "T2 21/09");
  assert.equal(ngayNgan("lạ"), "lạ");
});

test("giờ gõ kiểu gì cũng thành hh:mm của wire", () => {
  for (const [tho, ra] of [["1930", "19:30"], ["19h30", "19:30"], ["19.30", "19:30"], ["7:05", "07:05"], ["7h", "07:00"], [" 19:30 ", "19:30"]]) {
    assert.equal(chuanGio(tho), ra, tho);
  }
  for (const tho of ["2430", "19:75", "tối", ""]) assert.equal(chuanGio(tho), tho.trim(), tho);
});

test("giờ sai thì nói trước khi gửi; giờ trống chỉ sai ở chặng chính", () => {
  assert.equal(loiGio("19:30", true), null);
  assert.match(loiGio("25:00", false), /hh:mm/);
  assert.match(loiGio("", true), /cần một giờ/);
  assert.equal(loiGio("", false), null);
});

test("đi tiếp phải sau chặng chính; qua nửa đêm thì được", () => {
  assert.match(loiThuTu("19:00", "18:00"), /sau 19:00/);
  assert.match(loiThuTu("19:00", "19:00"), /sau 19:00/);
  assert.equal(loiThuTu("19:00", "21:00"), null);
  assert.equal(loiThuTu("22:30", "00:30"), null);
  assert.equal(loiThuTu("x", "18:00"), null);
});

test("đổi chỗ là một thay đổi người kia phải thấy, kể cả khi dòng việc giữ nguyên", () => {
  const v = (place_id) => ({ version: 1, content: { ngay: "2026-09-26", chang: [{ gio: "19:00", viec: "Ăn tối", place_id, can_kiem: true }] } });
  const ten = (id) => ({ "p-a": "Lẩu gà", "p-b": "Tiệm nướng" })[id];
  assert.deepEqual(khacGi(v("p-b"), v("p-a"), ten), ["Chỗ chính: Lẩu gà → Tiệm nướng"]);
  assert.deepEqual(khacGi(v(null), v("p-a"), ten), ["Chỗ chính: Lẩu gà → bỏ chỗ"]);
  assert.deepEqual(khacGi(v("p-x"), v(null)), ["Chỗ chính: chưa chọn chỗ → một chỗ trong danh mục"]);
  assert.deepEqual(khacGi(v("p-a"), v("p-a"), ten), []);
});

test("chặng có giờ mà không có việc thì không gửi được", async () => {
  const { loiViec } = await import("../dist-test/rudi/to-giay/sua-to.js");
  assert.match(loiViec("", "18:00"), /dòng việc/);
  assert.equal(loiViec("", ""), null);
  assert.equal(loiViec("Dạo hồ", "21:00"), null);
});

test("dòng việc theo loại quán, cùng bảng với máy chủ", async () => {
  const { viecTheoLoai } = await import("../dist-test/rudi/to-giay/sua-to.js");
  assert.equal(viecTheoLoai("cafe", "Ăn tối"), "Cà phê");
  assert.equal(viecTheoLoai("vui-choi", "Ăn tối"), "Đi chơi");
  assert.equal(viecTheoLoai("quan-an-local", "Ăn tối"), "Ăn tối");
  assert.equal(viecTheoLoai(null, "Ăn tối"), "Ăn tối");
});
