import assert from "node:assert/strict";
import { test } from "node:test";

import {
  LE_DUOI,
  LE_TREN,
  NEP_DIA,
  NEP_MEP_DAY,
  NEP_MEP_HEP,
  beRongMep,
  ghimVaoRay,
  rayDoc,
  tyLeTuY,
  yTuTyLe,
} from "../dist-test/rudi/nep/dock-vi-tri.js";

const KHUNG = { cao: 844, dinh: 59, day: 83 };

test("ray dọc chừa lề trên và dừng trước thanh tab, tính theo mép trên của Nếp", () => {
  const ray = rayDoc(KHUNG);
  assert.equal(ray.tren, KHUNG.dinh + LE_TREN);
  assert.equal(ray.duoi, KHUNG.cao - KHUNG.day - LE_DUOI - NEP_DIA);
});

test("Nếp không bao giờ đè lên thanh tab, kể cả khi bị kéo hết cỡ xuống", () => {
  const ray = rayDoc(KHUNG);
  const dayNep = ghimVaoRay(99_999, ray) + NEP_DIA;
  assert.ok(dayNep <= KHUNG.cao - KHUNG.day, `đáy Nếp ${dayNep} phải nằm trên thanh tab ${KHUNG.cao - KHUNG.day}`);
});

test("ghim vào ray chặn cả hai đầu và giữ nguyên giá trị hợp lệ", () => {
  const ray = rayDoc(KHUNG);
  assert.equal(ghimVaoRay(-500, ray), ray.tren);
  assert.equal(ghimVaoRay(99_999, ray), ray.duoi);
  assert.equal(ghimVaoRay(300, ray), 300);
});

test("màn hình quá thấp thì ray thu về một điểm chứ không lộn ngược", () => {
  const ray = rayDoc({ cao: 200, dinh: 59, day: 83 });
  assert.ok(ray.duoi >= ray.tren, "ray không được lộn ngược");
  assert.equal(ray.tren, ray.duoi);
  assert.equal(ghimVaoRay(500, ray), ray.tren);
});

test("chỗ đứng lưu theo tỉ lệ nên đổi máy hay xoay ngang vẫn về đúng chỗ tương đối", () => {
  const ray = rayDoc(KHUNG);
  assert.equal(tyLeTuY(ray.tren, ray), 0);
  assert.equal(tyLeTuY(ray.duoi, ray), 1);
  const giua = yTuTyLe(0.5, ray);
  assert.ok(Math.abs(tyLeTuY(giua, ray) - 0.5) < 1e-9);
});

test("tỉ lệ đọc từ đĩa có thể hỏng, và một tỉ lệ hỏng không thành toạ độ hỏng", () => {
  const ray = rayDoc(KHUNG);
  assert.equal(yTuTyLe(-3, ray), ray.tren);
  assert.equal(yTuTyLe(9, ray), ray.duoi);
  assert.equal(yTuTyLe(Number.NaN, ray), ray.tren);
  assert.equal(tyLeTuY(Number.NaN, ray), 0);
});

test("ray một điểm không sinh phép chia cho không", () => {
  const ray = rayDoc({ cao: 200, dinh: 59, day: 83 });
  assert.equal(tyLeTuY(ray.tren, ray), 0);
  assert.equal(yTuTyLe(0.7, ray), ray.tren);
});

test("mép giấy dày lên khi có việc, đó là cách báo tin duy nhất lúc đang giấu", () => {
  assert.equal(beRongMep(false), NEP_MEP_HEP);
  assert.equal(beRongMep(true), NEP_MEP_DAY);
  assert.ok(NEP_MEP_DAY > NEP_MEP_HEP);
});
