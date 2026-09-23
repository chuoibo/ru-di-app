import assert from "node:assert/strict";
import { test } from "node:test";

import { KHOA_DOCK, giaiMaDock, maHoaDock } from "../dist-test/rudi/nep/luu-dock.js";

test("khoá đĩa có phiên bản, để đổi định dạng sau này không đọc nhầm bản cũ", () => {
  assert.match(KHOA_DOCK, /^rudi\.nep\.dock\.v\d+$/);
});

test("ghi rồi đọc lại ra đúng chỗ Nếp đang đứng", () => {
  assert.deepEqual(giaiMaDock(maHoaDock({ tyLe: 0.25, ra: false })), { tyLe: 0.25, ra: false });
  assert.deepEqual(giaiMaDock(maHoaDock({ tyLe: 1, ra: true })), { tyLe: 1, ra: true });
});

test("đĩa hỏng thì coi như chưa chọn gì, không được ném", () => {
  for (const rac of [null, "", "{", "[]", '"chuoi"', "null", "{}", '{"tyLe":"x"}', '{"ra":true}']) {
    assert.doesNotThrow(() => giaiMaDock(rac), `rác: ${rac}`);
    assert.equal(giaiMaDock(rac), null, `rác: ${rac}`);
  }
});

test("tỉ lệ ngoài khoảng bị kéo về khoảng, không thành toạ độ hoang", () => {
  assert.equal(giaiMaDock('{"tyLe":9,"ra":false}').tyLe, 1);
  assert.equal(giaiMaDock('{"tyLe":-4,"ra":false}').tyLe, 0);
});

test("tỉ lệ không phải số thì cả bản ghi bỏ đi, chứ không đoán một nửa", () => {
  assert.equal(giaiMaDock('{"tyLe":null,"ra":true}'), null);
  assert.equal(giaiMaDock('{"tyLe":"0.5","ra":true}'), null);
});

test("chỉ hai trường này được lưu: chỗ đứng và người dùng có kéo Nếp ra không", () => {
  const raw = maHoaDock({ tyLe: 0.5, ra: true });
  assert.deepEqual(Object.keys(JSON.parse(raw)).sort(), ["ra", "tyLe"]);
});

test("trạng thái do máy chủ hay do route quyết định thì KHÔNG được lưu xuống đĩa", () => {
  const raw = maHoaDock({ tyLe: 0.5, ra: false, coViec: true, luiLai: true, trangThai: "mo" });
  const doc = JSON.parse(raw);
  assert.equal(doc.coViec, undefined, "có việc là sự thật của máy chủ, không phải của đĩa");
  assert.equal(doc.luiLai, undefined, "lui hay không do màn đang đứng quyết định");
  assert.equal(doc.trangThai, undefined, "mở bảng không được sống qua lần khởi động sau");
});

test("bản v1 không được đọc: `an: false` ở đó là mặc định cũ, không phải lựa chọn", () => {
  assert.equal(KHOA_DOCK, "rudi.nep.dock.v2");
  // A v1 record carries `an`, not `ra`, so the v2 reader cannot mistake it for
  // «pulled out» and put the old disc back over the page.
  assert.equal(giaiMaDock('{"tyLe":0.5,"an":false}'), null);
});
