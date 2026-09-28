// The chat tray's words for a pair versus a group (QA couple 23/09, §11).
// Owner decision 2026-09-28: a two-person chat has a group's tools, «Tờ hẹn»
// included; what still differs is wording, and a room of two is never «hội».
import assert from "node:assert/strict";
import test from "node:test";

import { CHU_CONG_CU_RONG_NHAT, ICON_CONG_CU_NHO, KHOANG_CONG_CU, MO_DAU_HOI_AI, boCucKhay, chuKhay, tranKhay } from "../dist-test/rudi/chat/khay-cong-cu.js";

test("nhắn riêng hai người: khay không nói «hội» hay «nhóm»", () => {
  const c = chuKhay(true);
  // Every word the pair's tray can show.
  const cacCau = Object.values(c);
  assert.ok(cacCau.length >= 4, "khay cặp phải có đủ chữ");
  for (const cau of cacCau) assert.doesNotMatch(cau, /hội|nhóm/i, cau);
  // Its AI entry asks for a plan, like a group's.
  assert.equal(MO_DAU_HOI_AI, "/plan ");
});

test("nhóm: giữ chữ của hội", () => {
  const c = chuKhay(false);
  assert.match(c.nhanPlan, /hội/);
  assert.match(c.tieuDePoll, /Hội/);
});

// Bug 2026-09-28 (lab screenshot 11): at 360 dp the couple's five tools wrapped
// 4 + 1, the fifth stretched across the row, and the tray's height cap cut its
// label. `boCucKhay` decides the grid; `tranKhay` keeps the cap above it.
test("khay năm công cụ: một hàng ở 360 và 320, không ô nào hẹp hơn chữ dài nhất", () => {
  for (const rong of [328, 288]) {
    const b = boCucKhay(rong, 5, 1);
    assert.equal(b.hang, 1, `${rong}: phải một hàng`);
    assert.equal(b.cot, 5);
    assert.ok(b.oRong >= CHU_CONG_CU_RONG_NHAT, `${rong}: ô ${b.oRong} hẹp hơn chữ`);
    assert.ok(5 * b.oRong + 4 * KHOANG_CONG_CU <= rong, `${rong}: năm ô tràn hàng`);
    assert.ok(b.icon <= b.oRong && b.icon >= ICON_CONG_CU_NHO, `${rong}: ô vuông ${b.icon}`);
  }
  // Measured on web at 360 (328 usable): 59.2 each, square 56.
  assert.deepEqual(boCucKhay(328, 5, 1), { cot: 5, hang: 1, oRong: 59.2, icon: 56, caoNoiDung: 119 });
});

test("khay bốn công cụ (đám bạn) giữ nguyên: một hàng, mỗi ô một phần tư", () => {
  // Before the fix: flex 1, gap 8, so (358 - 24) / 4 = 83.5 at 390 and 76 at 360.
  assert.deepEqual(boCucKhay(358, 4, 1), { cot: 4, hang: 1, oRong: 83.5, icon: 56, caoNoiDung: 119 });
  assert.equal(boCucKhay(328, 4, 1).oRong, 76);
});

test("chữ lớn: xuống hàng cân (3 + 2), không bao giờ 4 + 1, và trần khay cao đủ", () => {
  for (const scale of [1.3, 1.5, 2]) {
    for (const rong of [288, 328, 358]) {
      const b = boCucKhay(rong, 5, scale);
      assert.ok(b.oRong >= Math.ceil(CHU_CONG_CU_RONG_NHAT * scale) || b.cot === 1, `${rong}@${scale}: ô ${b.oRong} hẹp hơn chữ`);
      const cuoi = 5 - (b.hang - 1) * b.cot;
      assert.ok(b.hang === 1 || cuoi >= b.cot - 1, `${rong}@${scale}: hàng cuối ${cuoi} ô lẻ loi (cột ${b.cot})`);
      assert.ok(b.cot * b.oRong + (b.cot - 1) * KHOANG_CONG_CU <= rong, `${rong}@${scale}: tràn hàng`);
    }
  }
  const b = boCucKhay(328, 5, 1.3);
  assert.deepEqual([b.hang, b.cot], [2, 3]);
  // The cap grows to the grid: at 780 the old cap was 195, the grid needs more.
  assert.ok(b.caoNoiDung > 195);
  assert.equal(tranKhay(780, b.caoNoiDung), b.caoNoiDung);
  // A form keeps the old cap; a grid taller than 60% of the window scrolls.
  assert.equal(tranKhay(780, null), 195);
  assert.equal(tranKhay(844, null), 211);
  assert.equal(tranKhay(640, 1000), 384);
  assert.equal(tranKhay(780, 119), 195);
});

test("chữ trên khay và hàng tờ hẹn: hai người không đọc «hội» / «người giữ sổ»", () => {
  assert.equal(chuKhay(true).suaChung, "Sửa cùng nhau");
  assert.equal(chuKhay(false).suaChung, "Sửa cùng hội", "Maestro 48 (nhóm) vẫn tìm «Sửa cùng hội»");
  assert.doesNotMatch(chuKhay(true).ghiChuAnh, /bạn đồng hành|người giữ sổ|hội|nhóm/);
  assert.match(chuKhay(false).ghiChuAnh, /người giữ sổ/);
});
