import assert from "node:assert/strict";
import test from "node:test";
import { khopTenCuocTroChuyen } from "../dist-test/rudi/nhom/tim-cuoc-tro-chuyen.js";

test("tìm cuộc trò chuyện: tên thật có dấu, không dấu, chữ hoa và nhiều từ", () => {
  assert.equal(khopTenCuocTroChuyen("Hội mình · Đà Lạt", "da hoi"), true);
  assert.equal(khopTenCuocTroChuyen("Đặng Ngọc", "DANG ngoc"), true);
  assert.equal(khopTenCuocTroChuyen("Đặng Ngọc".normalize("NFD"), "đặng"), true);
  assert.equal(khopTenCuocTroChuyen("Hội mình · Tổng hợp", "   "), true);
  assert.equal(khopTenCuocTroChuyen("Hội mình", "minh ngoc"), false);
});
