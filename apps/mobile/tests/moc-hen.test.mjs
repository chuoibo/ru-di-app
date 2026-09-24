import assert from "node:assert/strict";
import { test } from "node:test";

import { cauHenTrongChat, cauLanHen, demNgay } from "../dist-test/rudi/to-giay/moc-hen.js";

const to = (id, state, ngay, viec = "Ăn tối") => ({
  id,
  state,
  version: 1,
  versions: [{ version: 1, content: { ngay, chang: [{ gio: "19:00", viec, place_id: null, can_kiem: true }] } }],
  keeps: [],
});

test("đếm ngày tới buổi hẹn; ngày đã qua thì không đếm", () => {
  assert.equal(demNgay(to("a", "chot", "2026-09-26"), "2026-09-24"), "Còn 2 ngày");
  assert.equal(demNgay(to("a", "chot", "2026-09-25"), "2026-09-24"), "Ngày mai");
  assert.equal(demNgay(to("a", "chot", "2026-09-24"), "2026-09-24"), "Hôm nay");
  assert.equal(demNgay(to("a", "chot", "2026-09-20"), "2026-09-24"), null);
});

test("lần hẹn thứ mấy: đếm các tờ đã chốt/đã đi trước ngày này", () => {
  const cu = [to("x", "da_giu", "2026-09-05"), to("y", "da_di", "2026-09-12"), to("z", "huy", "2026-09-19")];
  const nay = to("a", "chot", "2026-09-26");
  assert.equal(cauLanHen(nay, [...cu, nay]), "Lần hẹn thứ 3 của hai bạn");
  assert.equal(cauLanHen(nay, [nay]), "Lần hẹn đầu tiên của hai bạn");
  assert.equal(cauLanHen(to("b", "da_gui", "2026-09-26"), cu), null);
});

test("dòng ghim trong chat khi đã hẹn: ngày, chỗ, đếm ngày", () => {
  assert.equal(
    cauHenTrongChat(to("a", "chot", "2026-09-26"), "2026-09-24", "Lẩu Gà Lá É Tao Ngộ"),
    "Hai bạn hẹn Thứ Bảy 26/09 · Lẩu Gà Lá É Tao Ngộ · còn 2 ngày",
  );
  assert.equal(cauHenTrongChat(to("a", "chot", "2026-09-26", "Ăn lẩu"), "2026-09-26"), "Hai bạn hẹn Thứ Bảy 26/09 · Ăn lẩu · hôm nay");
  assert.equal(cauHenTrongChat(to("a", "da_gui", "2026-09-26"), "2026-09-24"), null);
});
