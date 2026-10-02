/**
 * The tab strip's columns (`src/rudi/ui/thanh-tab.ts`), owner's call 01/10:
 * four columns and the «Tạo» stamp make five slots, so the stamp has a middle;
 * Cộng đồng is a tab route with no column, a section of Khám phá, and while it
 * is open the Khám phá column is the lit one. The last test holds the route's
 * `href: null` in app/(tabs)/_layout.tsx to the map that names its host.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { MUC_TRONG_TAB, dichCuaCot, ghiMucKhamPha, oCuaCot, xepThanh } from "../dist-test/rudi/ui/thanh-tab.js";

const TAB = ["explore", "community", "plan", "messages", "profile"];

test("cột Khám phá mở Địa điểm khi chưa xem mục nào (lần đầu chạy)", () => {
  assert.equal(dichCuaCot("explore"), "explore");
});

test("thanh: bốn cột + con dấu = năm ô, con dấu ở ô giữa", () => {
  const t = xepThanh(TAB, "plan", false);
  assert.deepEqual(t.cot, ["explore", "plan", "messages", "profile"]);
  assert.equal(t.soCot, 5);
  assert.equal(t.viTriDau, 2);
});

test("thanh: đang ở Cộng đồng thì cột Khám phá sáng", () => {
  assert.equal(xepThanh(TAB, "community", false).cotChon, 0);
  assert.equal(xepThanh(TAB, "explore", false).cotChon, 0);
  assert.equal(xepThanh(TAB, "messages", false).cotChon, 2);
  assert.equal(xepThanh(TAB, "khong-co", false).cotChon, -1);
});

test("thanh: ô của từng cột nhảy qua ô con dấu", () => {
  const t = xepThanh(TAB, "plan", false);
  assert.deepEqual([0, 1, 2, 3].map((i) => oCuaCot(t, i)), [0, 1, 3, 4]);
});

test("thanh dọc: con dấu ở đầu", () => {
  assert.equal(xepThanh(TAB, "plan", true).viTriDau, 0);
});

test("cột Khám phá mở lại mục xem sau cùng; cột khác mở chính nó", () => {
  ghiMucKhamPha("community");
  assert.equal(dichCuaCot("explore"), "community");
  assert.equal(dichCuaCot("plan"), "plan");
  ghiMucKhamPha("explore");
  assert.equal(dichCuaCot("explore"), "explore");
});

test("route href: null của _layout.tsx đúng bằng MUC_TRONG_TAB, cột chủ là một tab hiện", () => {
  const layout = readFileSync(new URL("../app/(tabs)/_layout.tsx", import.meta.url), "utf8");
  const tat = [...layout.matchAll(/<Tabs\.Screen\s+name="([^"]+)"\s+options=\{\{([^}]*)\}\}/g)];
  const an = tat.filter(([, , o]) => /href:\s*null/.test(o)).map(([, ten]) => ten).sort();
  assert.deepEqual(an, Object.keys(MUC_TRONG_TAB).sort());
  const hien = tat.filter(([, , o]) => !/href:\s*null/.test(o)).map(([, ten]) => ten);
  for (const chu of Object.values(MUC_TRONG_TAB)) assert.ok(hien.includes(chu), `${chu} không phải tab hiện`);
});

// Deferred minor of 02/10: the strip's indicator worklet recomputed the slot
// of a column by hand instead of using oCuaCot, and animated the column index:
// a move from Lên plan to Tin nhắn slid to the stamp's slot and then jumped
// one slot at the end. The slot comes from oCuaCot and the indicator moves by
// slot. Read from the source: RudiTabBar needs Reanimated and is not rendered.
test("vạch chỉ báo của thanh đi theo ô do oCuaCot tính, không tự tính lại trong worklet", () => {
  const thanhTab = readFileSync(new URL("../src/rudi/ui/RudiTabBar.tsx", import.meta.url), "utf8");
  assert.match(thanhTab, /oCuaCot\(thanh, viTriSang\)/);
  assert.doesNotMatch(thanhTab, />= viTriDau \? indicator\.value \+ 1/);
});
