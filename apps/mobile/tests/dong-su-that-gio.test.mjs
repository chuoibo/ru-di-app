/**
 * A place's facts in two quiet lines (finish review 03/10): the rating and the
 * distance, then whether it is open and its hours. The lines never end or
 * start on «·»: inside a fact nothing breaks (no-break spaces, a word joiner
 * after a dash), between the rating and the distance only at « · », and the
 * open line breaks only after its comma, between the state and the hours.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { chiaDongSuThat } from "../dist-test/rudi/kham-pha/dia-diem.js";

const giu = (s) => s.replace(/ /g, " ").replace(/([–-])/g, "$1⁠");
const sao = { icon: "star", text: "4.6 (118)" };
const km = { icon: "navigate-outline", text: "2.3 km" };
const gia = { icon: "wallet-outline", text: "220.000đ – 320.000đ mỗi người" };

test("điểm và khoảng cách một dòng, giờ mở một dòng riêng; giá không ở dòng nào", () => {
  const r = chiaDongSuThat([sao, km, gia, { icon: "time-outline", text: "Đang mở · 08:00 – 21:00" }]);
  assert.equal(r.dau, `${giu("4.6 (118)")} · ${giu("2.3 km")}`);
  assert.equal(r.gio, `${giu("Đang mở")}, ${giu("08:00 – 21:00")}`);
});

test("dòng giờ chỉ gãy sau dấu phẩy, không bao giờ ở cạnh «·»", () => {
  const { gio } = chiaDongSuThat([sao, { icon: "time-outline", text: "Đã đóng · mở 16:00" }]);
  assert.equal(gio, `${giu("Đã đóng")}, ${giu("mở 16:00")}`);
  assert.doesNotMatch(gio, /·/);
  // The only ordinary space left is the one after the comma.
  assert.equal(gio.split(" ").length, 2);
});

test("không có giờ thì không có dòng giờ; không có gì thì dòng đầu rỗng", () => {
  assert.deepEqual(chiaDongSuThat([sao, km, gia]), { dau: `${giu("4.6 (118)")} · ${giu("2.3 km")}`, gio: null });
  assert.deepEqual(chiaDongSuThat([]), { dau: "", gio: null });
});
