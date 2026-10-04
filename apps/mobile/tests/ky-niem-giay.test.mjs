/* Kỷ niệm thành giấy (ADR-0037, kế hoạch UI v3 S6): ba hàm thuần mà màn đọc.
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/ky-niem-giay.test.mjs
 *
 *   1. cauKeAlbumRong (B4): «chưa có kèo» khác «kèo chưa tới ngày».
 *   2. nghiengAnh: ảnh in nghiêng một hai độ, không bao giờ thẳng, cùng id cùng góc.
 *   3. huyHieuMoi (M8): huy hiệu mở từ lần xem trước; kho hỏng thì coi như chưa thấy gì.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { cauKeAlbumRong, huyHieuMoi, nghiengAnh } from "../dist-test/rudi/ky-niem/ky-niem.js";

test("B4: kệ album rỗng nói đúng lý do", () => {
  const khong = cauKeAlbumRong([], "2026-09-25");
  assert.equal(khong.tieuDe, "Chưa có kèo nào");
  const sau = cauKeAlbumRong(
    [
      { title: "Đà Lạt cuối tuần", starts_on: "2026-10-17" },
      { title: "Cà phê sáng", starts_on: "2026-10-02" },
      { title: "Đã qua", starts_on: "2026-09-01" },
    ],
    "2026-09-25",
  );
  assert.equal(sau.tieuDe, "Chưa tới ngày đi");
  assert.match(sau.than, /^«Cà phê sáng» bắt đầu 2\/10\./, "phải gọi kèo GẦN nhất, ngày không số 0 đứng đầu");
  // Only outings already under way or past: the shelf has nothing to wait for.
  assert.equal(cauKeAlbumRong([{ title: "Hôm nay", starts_on: "2026-09-25" }], "2026-09-25").tieuDe, "Chưa có kèo nào");
});

test("ảnh in nghiêng một hai độ, không bao giờ thẳng, tất định theo id", () => {
  const thay = new Set();
  for (let i = 0; i < 400; i += 1) {
    const g = nghiengAnh(`ky-niem-${i}`);
    assert.ok([-2, -1, 1, 2].includes(g), `góc lạ ${g}`);
    assert.equal(nghiengAnh(`ky-niem-${i}`), g);
    thay.add(g);
  }
  assert.equal(thay.size, 4, "bốn góc đều phải xuất hiện: một bức tường cùng một góc là không nghiêng");
});

test("M8: huy hiệu mới là cái MỚI NHẤT chưa thấy; máy chưa từng xem chỉ trình bày huy hiệu vừa mở (QA UI-160)", () => {
  const now = Date.parse("2026-10-01T12:00:00+07:00");
  const a = { id: "a", earned_at: "2026-09-29T09:00:00+07:00" };
  const b = { id: "b", earned_at: "2026-10-01T08:00:00+07:00" };
  assert.equal(huyHieuMoi([a, b], JSON.stringify(["a"]), now), "b");
  assert.equal(huyHieuMoi([a, b], JSON.stringify([]), now), "b", "hai cái chưa thấy: trình bày cái mới nhất, không phải cái cũ nhất");
  assert.equal(huyHieuMoi([a, b], JSON.stringify(["a", "b"]), now), null);
  // A phone that never looked: only a badge opened within 48 hours.
  assert.equal(huyHieuMoi([a, b], null, now), "b");
  assert.equal(huyHieuMoi([a], null, now), null, "máy mới: huy hiệu đạt từ hai ngày trước không diễn lại");
  assert.equal(huyHieuMoi([a], "{hỏng", now), null, "kho hỏng coi như máy chưa từng xem");
  assert.equal(huyHieuMoi([a], JSON.stringify({ a: 1 }), now), null);
  assert.equal(huyHieuMoi([], JSON.stringify(["a"]), now), null);
  // QA UI-160: the account's mark leads; the phone's store no longer decides.
  const sa = { ...a, seen: true };
  const sb = { ...b, seen: false };
  assert.equal(huyHieuMoi([sa, sb], null, now), "b", "máy mới, tài khoản chưa thấy b: trình bày b");
  assert.equal(huyHieuMoi([sa, { ...b, seen: true }], null, now), null, "máy mới, tài khoản đã thấy cả hai: không trình bày gì");
  assert.equal(huyHieuMoi([sa, { ...b, seen: true }], JSON.stringify([]), now), null, "kho máy nói chưa thấy gì, tài khoản nói đã thấy: tài khoản thắng");
  assert.equal(huyHieuMoi([{ ...a, seen: false }, sb], null, now), "b", "hai cái chưa thấy: cái mới nhất");
});
