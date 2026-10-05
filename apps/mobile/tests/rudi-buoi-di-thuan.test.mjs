/* The pure outing rules App B's `buoi-di.test.mjs` pinned, re-pinned on the
 * module itself now that the screens that rendered them are gone. These rules
 * still run under the RuDi shell (`src/rudi/keo/keo.ts` and the outing screens
 * import this module), so the claims move here rather than dying.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { kiemTraChang, kiemTraTaoBuoiDi, nhanKhoangNgay, nhanNganSach, sapXepChang, tongDuKien } from "../dist-test/screens/len-plan/buoi-di.js";

const FORM = { title: "Đà Lạt cuối tuần", starts_on: "2026-10-17", ends_on: "2026-10-19", headcount: "8", nganSach: "2500000" };

test("kiemTraTaoBuoiDi: form hợp lệ ra thân đúng kiểu; từng lỗi một câu tiếng Việt, không em-dash", () => {
  const ok = kiemTraTaoBuoiDi(FORM);
  assert.equal(ok.ok, true);
  assert.deepEqual(ok.body, { title: "Đà Lạt cuối tuần", starts_on: "2026-10-17", ends_on: "2026-10-19", headcount: 8, budget_per_person_vnd: 2_500_000 });
  // Each refusal names its field, so the form says it under that field (QA UI-034).
  for (const [hong, o] of [
    [{ ...FORM, title: "  " }, "ten"],
    [{ ...FORM, starts_on: "17/10/2026" }, "ngay"],
    [{ ...FORM, ends_on: "2026-10-16" }, "ngay"],
    [{ ...FORM, headcount: "0" }, "so-nguoi"],
    [{ ...FORM, nganSach: "2.5 triệu" }, "ngan-sach"],
    [{ ...FORM, nganSach: "" }, "ngan-sach"],
  ]) {
    const kq = kiemTraTaoBuoiDi(hong);
    assert.equal(kq.ok, false);
    assert.ok(kq.loi.length > 0);
    assert.doesNotMatch(kq.loi, /—/);
    assert.equal(kq.o, o);
  }
  // Empty is a choice not yet made, said as such; a bad number keeps the format rule.
  assert.equal(kiemTraTaoBuoiDi({ ...FORM, nganSach: " " }).loi, "Chọn một mức hoặc gõ số tiền mỗi người.");
  assert.match(kiemTraTaoBuoiDi({ ...FORM, nganSach: "2.5 triệu" }).loi, /chữ số/);
});

test("sapXepChang: theo giờ, ổn định khi trùng giờ, không đổi mảng vào", () => {
  const vao = [{ at: "12:00", label: "b" }, { at: "08:30", label: "a" }, { at: "12:00", label: "c" }];
  const ra = sapXepChang(vao);
  assert.deepEqual(ra.map((c) => c.label), ["a", "b", "c"]);
  assert.deepEqual(vao.map((c) => c.label), ["b", "a", "c"]);
});

test("kiemTraChang: giờ phải là HH:MM và nhãn không rỗng", () => {
  assert.equal(kiemTraChang("08:30", "Cà phê").ok, true);
  assert.equal(kiemTraChang("8h30", "Cà phê").ok, false);
  assert.equal(kiemTraChang("08:30", "  ").ok, false);
});

test("nhanKhoangNgay và nhanNganSach: chữ đọc được, không em-dash, tiền dạng Việt", () => {
  const khoang = nhanKhoangNgay("2026-10-17", "2026-10-19");
  assert.doesNotMatch(khoang, /—/);
  assert.match(khoang, /17/);
  assert.match(nhanNganSach(2_500_000), /2\.500\.000/);
});

test("tongDuKien: số người nhân ngân sách mỗi người, số nguyên đồng", () => {
  assert.equal(tongDuKien(2_500_000, 8), 20_000_000);
  assert.ok(Number.isInteger(tongDuKien(333_333, 3)));
});

test("tongDuKien: tích vượt 2^53 bị từ chối thay vì lệch đồng (audit 2026-10-05)", () => {
  // Trần máy chủ: ngân sách ≤ 10^12, số người ≤ 1000 → luôn an toàn.
  assert.equal(tongDuKien(1_000_000_000_000, 1000), 1_000_000_000_000_000);
  assert.throws(() => tongDuKien(Number.MAX_SAFE_INTEGER, 3), RangeError);
});
