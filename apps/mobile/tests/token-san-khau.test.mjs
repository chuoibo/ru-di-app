/**
 * Ngân sách và độ cao giấy của «Sân khấu giấy» (ADR-0037 D2, D3, D13).
 *
 * Ba điều test này giữ:
 *   1. Ngân sách ghép không phải bậc thứ năm: MOTION_MS vẫn đúng bốn bậc, một cú
 *      bật dựng là `shared` cộng so le và không bao giờ quá `batToiDa`, và mọi
 *      ngân sách về 0 khi giảm chuyển động.
 *   2. Độ cao giấy tăng thì bóng tăng (dy, blur, alpha), mức 0 không có bóng, và
 *      tầng đứng dựng mất bóng khi nằm phẳng.
 *   3. Màu sân khấu nằm NGOÀI color.*: 25 khoá của color.light/dark không đổi, vì
 *      guest.css soi gương đúng 25 khoá đó và tầng Python legacy không được đổi.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { MOTION_MS, NGAN_SACH_SAN_KHAU, batToiDa, treTang } from "../dist-test/rudi/motion.js";
import { bongCao, mauSanKhau } from "../dist-test/rudi/san-khau/token.js";

const tokens = JSON.parse(readFileSync(new URL("../../../packages/shared/tokens.json", import.meta.url), "utf8"));

test("ngân sách ghép đứng trên bốn bậc, không thành bậc thứ năm", () => {
  assert.deepEqual(Object.keys(MOTION_MS), ["instant", "standard", "shared", "celebrate"]);
  assert.equal(NGAN_SACH_SAN_KHAU.lat, MOTION_MS.shared, "lật trang là shared");
  assert.equal(NGAN_SACH_SAN_KHAU.batToiDa, MOTION_MS.shared + 3 * NGAN_SACH_SAN_KHAU.batTang, "bật dựng = shared + so le của 4 tầng");
  assert.ok(NGAN_SACH_SAN_KHAU.batToiDa <= 420);
  assert.ok(NGAN_SACH_SAN_KHAU.dien <= 1400, "một tiết mục của Nếp quá 1,4 giây là bắt người ta xem kịch");
});

test("so le dừng ở tầng thứ tư và cú bật dựng không vượt trần", () => {
  assert.equal(treTang(0, false), 0);
  assert.equal(treTang(1, false), NGAN_SACH_SAN_KHAU.batTang);
  assert.equal(treTang(3, false), 3 * NGAN_SACH_SAN_KHAU.batTang);
  assert.equal(treTang(9, false), 3 * NGAN_SACH_SAN_KHAU.batTang, "tầng thứ năm trở đi bắt đầu cùng tầng thứ tư");
  assert.equal(batToiDa(1, false), MOTION_MS.shared);
  for (const n of [1, 2, 3, 4, 5, 12]) assert.ok(batToiDa(n, false) <= NGAN_SACH_SAN_KHAU.batToiDa, `${n} tầng`);
});

test("giảm chuyển động đưa mọi ngân sách sân khấu về 0", () => {
  for (const n of [0, 1, 4, 9]) assert.equal(treTang(n, true), 0);
  for (const n of [1, 4, 9]) assert.equal(batToiDa(n, true), 0);
});

test("độ cao giấy tăng thì bóng tăng; mức 0 không có bóng", () => {
  for (const dark of [false, true]) {
    assert.equal(bongCao(0, dark), null, "cái in trên trang không đổ bóng");
    const [b1, b2, b3] = [1, 2, 3].map((cao) => bongCao(cao, dark));
    for (const k of ["dy", "blur", "alpha"]) {
      assert.ok(b1[k] < b2[k] && b2[k] < b3[k], `${dark ? "tối" : "sáng"}: ${k} phải tăng theo độ cao`);
    }
    assert.ok(b3.alpha <= 0.5, "bóng quá đậm thành vết mực");
    assert.equal(b1.mau, mauSanKhau(dark).bong);
  }
});

test("tầng đứng dựng mất bóng khi nằm phẳng", () => {
  const dung = bongCao(2, false, 0);
  const nghieng = bongCao(2, false, 60);
  const phang = bongCao(2, false, 90);
  assert.ok(dung.alpha > nghieng.alpha && nghieng.alpha > phang.alpha);
  assert.ok(phang.alpha < 1e-9 && Math.abs(phang.dy) < 1e-9, "nằm phẳng trên trang thì không đổ bóng");
  assert.deepEqual(bongCao(1, false, 90), bongCao(1, false, 0), "mức 1 không phụ thuộc góc");
});

test("màu sân khấu và mực người nằm ngoài color.*: 25 khoá guest.css soi gương không đổi", () => {
  for (const scheme of ["light", "dark"]) {
    assert.equal(Object.keys(tokens.color[scheme]).length, 25, `color.${scheme}`);
    assert.ok(!("bong" in tokens.color[scheme]) && !("mucNguoi" in tokens.color[scheme]));
    for (const k of ["bong", "anhSang", "mepGiay"]) {
      assert.match(tokens.sanKhau[scheme][k], /^#[0-9a-f]{6}$/, `sanKhau.${scheme}.${k}`);
    }
  }
});
