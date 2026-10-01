import assert from "node:assert/strict";
import test from "node:test";
import { apDungTuyen, changChuaXep, doiViTri, ngayMacDinh, nhapTuKeo, noiDungGui, suaChang, xepVaoNgay, xoaChang } from "../dist-test/rudi/hanh-trinh/ke-hoach.js";

const day = "2030-10-17";
const other = "2030-10-18";
function draft() {
  return { expected_revision: 4, days: [{ ...ngayMacDinh(day), start_stop_id: "a" }], stops: [
    { id: "a", position: 0, day, at: "08:00", label: "Chợ", time_locked: true, duration_minutes: 30 },
    { id: "x", position: 1, day: other, at: "09:00", label: "Ngày sau", time_locked: true, duration_minutes: null },
    { id: "b", position: 2, day, at: "10:00", label: "Hồ", time_locked: false, duration_minutes: 45 },
    { id: "c", position: 3, day, at: "11:00", label: "Cà phê", time_locked: false, duration_minutes: 90 },
  ] };
}
test("chia ngày tạo cấu hình, bỏ neo cũ, không sửa nguồn", () => {
  const before = draft(); const after = suaChang(before, "a", { day: other });
  assert.equal(after.days[0].start_stop_id, null);
  assert.ok(after.days.some((d) => d.day === other));
  assert.equal(before.stops[0].day, day);
});
test("đổi thứ tự trong ngày giữ ID, ngày khác và thời lượng", () => {
  const after = doiViTri(draft(), "c", "first");
  assert.deepEqual(after.stops.map((s) => s.id), ["c", "x", "a", "b"]);
  assert.equal(after.days[0].start_stop_id, null);
  assert.equal(after.stops[0].duration_minutes, 90);
});
test("áp dụng chỉ sửa ngày đã xem và từ chối đề xuất thiếu/trùng ID", () => {
  const route = { stops: [{ id: "a", at: "08:00" }, { id: "c", at: "08:40" }, { id: "b", at: "10:20" }] };
  const after = apDungTuyen(draft(), day, route);
  assert.deepEqual(after.stops.map((s) => [s.id, s.at]), [["a", "08:00"], ["x", "09:00"], ["c", "08:40"], ["b", "10:20"]]);
  assert.throws(() => apDungTuyen(draft(), day, { stops: route.stops.slice(1) }));
  assert.throws(() => apDungTuyen(draft(), day, { stops: [route.stops[0], route.stops[0], route.stops[1]] }));
  assert.equal(noiDungGui(after).stops[0].position, undefined);
});
test("bỏ chặng bỏ cả neo; lịch cũ nhiều ngày không đoán ngày hoặc thời lượng", () => {
  assert.equal(xoaChang(draft(), "a").days[0].start_stop_id, null);
  const legacy = nhapTuKeo({ starts_on: day, ends_on: other, stops: [{ id: "old", at: "12:00" }] });
  assert.equal(legacy.stops[0].day, null);
  assert.equal(legacy.stops[0].duration_minutes, null);
  assert.equal(legacy.stops[0].time_locked, true);
});

test("chặng chưa xếp ngày được nêu ra, và chỉ vào ngày khi người dùng xếp (UI-032)", () => {
  const third = "2030-10-19";
  const before = { ...draft(), stops: [...draft().stops, { id: "n1", position: 4, day: null, at: "12:00", label: "Ăn trưa", time_locked: true, duration_minutes: null }, { id: "n2", position: 5, day: "2030-11-01", at: "19:00", label: "Ngoài chuyến", time_locked: true, duration_minutes: null }] };
  assert.deepEqual(changChuaXep(before, [day, other, third]).map((s) => s.id), ["n1", "n2"]);
  // A day that has stops: the newcomers come after them, in their own order.
  const after = xepVaoNgay(before, ["n1", "n2"], day);
  assert.deepEqual(after.stops.filter((s) => s.day === day).map((s) => s.id), ["a", "b", "c", "n1", "n2"]);
  assert.deepEqual(changChuaXep(after, [day, other, third]), []);
  assert.equal(after.stops.find((s) => s.id === "x").day, other);
  assert.equal(before.stops.find((s) => s.id === "n1").day, null);
  // A day with no settings yet gets the defaults, as choosing it in the editor does.
  const empty = xepVaoNgay(before, ["n1"], third);
  assert.ok(empty.days.some((d) => d.day === third));
  assert.equal(xepVaoNgay(before, [], third), before);
});
