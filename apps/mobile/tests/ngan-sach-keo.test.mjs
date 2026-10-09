import assert from "node:assert/strict";
import test from "node:test";
import { docNganSach, dichNganSach, nganSachTuViTri } from "../dist-test/rudi/keo/ngan-sach-keo.js";

test("thanh ngân sách: vuốt hai mép, giữa và ngoài track vẫn ra số đồng nguyên", () => {
  assert.equal(nganSachTuViTri(0, 288), 0);
  assert.equal(nganSachTuViTri(144, 288), 1000000);
  assert.equal(nganSachTuViTri(288, 288), 2000000);
  assert.equal(nganSachTuViTri(-30, 288), 0);
  assert.equal(nganSachTuViTri(400, 288), 2000000);
  assert.equal(nganSachTuViTri(35, 288), 250000);
  for (let x = 0; x < 289; x += .7) assert.equal(nganSachTuViTri(x, 288) % 25000, 0);
  for (const [x, width] of [[0, 0], [NaN, 20], [5, Infinity], [5, -2]]) assert.equal(nganSachTuViTri(x, width), null);
});

test("nhập chính xác không bị thanh giới hạn 2 triệu làm tròn hoặc cắt", () => {
  assert.equal(docNganSach("3250123"), 3250123);
  assert.equal(docNganSach("0"), 0);
  for (const text of ["", "-1", "1.5", "1e6", String(Number.MAX_SAFE_INTEGER + 1)]) assert.equal(docNganSach(text), null);
  assert.equal(dichNganSach(3250123, 1), 3275123);
  assert.equal(dichNganSach(3250123, -1), 3225123);
  assert.equal(dichNganSach(0, -1), 0);
  assert.equal(dichNganSach(2000000, 1), 2000000);
});
