/**
 * The community feed's five modes as small text tabs (owner's choice, 02/10):
 * one tablist, the modes in order, exactly one selected, none when the feed
 * shows a mode that has no tab (the hidden posts, opened from the settings
 * sheet); and the row scrolls only as far as it takes to bring the selected
 * tab into view.
 *
 * Does not prove: the ink underline reads as «selected» next to the filter
 * chips of Địa điểm (that is the screenshot), or how the row scrolls on a
 * device (that is the probe).
 */
import assert from "node:assert/strict";
import test from "node:test";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { HangChuTab, cuonDeThay } from "../dist-test/rudi/ui/HangChuTab.js";

const MUC = [
  { id: "for_you", nhan: "Dành cho bạn" },
  { id: "following", nhan: "Đang theo dõi" },
  { id: "trending", nhan: "Thịnh hành" },
  { id: "saved", nhan: "Đã lưu" },
  { id: "mine", nhan: "Bài của tôi" },
];
const ve = (chon) => renderToStaticMarkup(React.createElement(HangChuTab, { muc: MUC, chon, onChon: () => {} }));
const cacTab = (html) =>
  [...html.matchAll(/<[^>]*role="tab"[^>]*>/g)].map(([the]) => ({
    nhan: the.match(/aria-label="([^"]*)"/)?.[1],
    chon: the.match(/aria-selected="([^"]*)"/)?.[1],
  }));

test("hàng chữ-tab: một tablist, năm tab theo thứ tự, đúng một tab được chọn", () => {
  const html = ve("saved");
  assert.equal((html.match(/role="tablist"/g) ?? []).length, 1);
  assert.deepEqual(
    cacTab(html).map((t) => t.nhan),
    ["Dành cho bạn", "Đang theo dõi", "Thịnh hành", "Đã lưu", "Bài của tôi"],
  );
  assert.deepEqual(
    cacTab(html).map((t) => t.chon),
    ["false", "false", "false", "true", "false"],
  );
  // A tab, not a toggle button: no pressed state beside the selected one.
  assert.doesNotMatch(html, /aria-pressed/);
});

test("chế độ không có tab (bài đã ẩn) thì không tab nào được chọn", () => {
  assert.deepEqual(
    cacTab(ve(null)).map((t) => t.chon),
    ["false", "false", "false", "false", "false"],
  );
});

test("cuộn tới tab được chọn: không cuộn khi nó đã thấy trọn, cuộn vừa đủ khi nó khuất", () => {
  const khung = { x: 0, w: 358 };
  // In view with the gutter to spare: leave the row where it is.
  assert.equal(cuonDeThay({ x: 120, w: 90 }, khung, 16), null);
  // Past the right edge: scroll until it ends one gutter inside.
  assert.equal(cuonDeThay({ x: 400, w: 80 }, khung, 16), 400 + 80 + 16 - 358);
  // Scrolled past on the left: bring it back one gutter from the edge.
  assert.equal(cuonDeThay({ x: 40, w: 90 }, { x: 100, w: 358 }, 16), 24);
  // Never before the start of the row.
  assert.equal(cuonDeThay({ x: 8, w: 90 }, { x: 60, w: 358 }, 16), 0);
  // Not measured yet: nothing to do.
  assert.equal(cuonDeThay({ x: 400, w: 80 }, { x: 0, w: 0 }, 16), null);
});

import { khoangCachLo } from "../dist-test/rudi/ui/HangChuTab.js";

// Where the row ends inside the window, given a gap: which tab straddles the edge.
const catNgang = (rong, gap, khung, le) => {
  let x = le;
  for (const w of rong) {
    if (x <= khung - 16 && x + w >= khung + 10) return true;
    x += w + gap;
  }
  return false;
};

test("hàng tràn thì luôn có một chữ bị mép cắt ngang, để người đọc biết còn nữa (finish review 03/10)", () => {
  // Tab widths in dp as measured (03/10): Android 411 at 1.0 and at 1.3
  // (uiautomator bounds ÷ 2.625), the web at 390.
  const coChu = [
    [86, 88, 71, 41, 72],
    [117, 118, 96, 54, 94],
    [92, 93, 75, 44, 78],
  ];
  for (const rong of coChu) {
    // From the narrowest phone the app is laid out for (320) to past a large one.
    for (let khung = 320; khung <= 540; khung++) {
      const { khoang: gap, demCuoi } = khoangCachLo(rong, khung, 16);
      const tong = rong.reduce((a, b) => a + b, 0) + gap * (rong.length - 1) + 16 + demCuoi;
      if (tong <= khung) continue;
      assert.ok(gap >= 12 && gap <= 32, `khoảng ${gap} ngoài 12…32 ở ${khung}`);
      assert.ok(demCuoi >= 16 && demCuoi <= 24, `đệm cuối ${demCuoi} ngoài 16…24 ở ${khung}`);
      assert.ok(catNgang(rong, gap, khung, 16), `không chữ nào bị cắt ngang ở ${khung} dp (khoảng ${gap})`);
    }
  }
});

test("khoảng mặc định 22 giữ nguyên khi nó đã cắt ngang một chữ, hay khi cả hàng vừa", () => {
  // 411 at 1.0: «Bài» already peeks past «Đã lưu».
  assert.equal(khoangCachLo([86, 88, 71, 41, 72], 411, 16).khoang, 22);
  // A tablet column: everything fits, nothing to hint.
  assert.deepEqual(khoangCachLo([86, 88, 71, 41, 72], 528, 0), { khoang: 22, demCuoi: 0 });
  // Not measured yet.
  assert.deepEqual(khoangCachLo([], 0, 16), { khoang: 22, demCuoi: 16 });
});

test("cuộn tới một tab giữa hàng thì chừa chỗ cho tab bên cạnh ló ra, để hàng vẫn nói còn nữa", () => {
  // «Đã lưu» picked with «Bài của tôi» after it: its right edge stops a gap
  // and a peek (22 + 16) inside the window, not the gutter.
  assert.equal(cuonDeThay({ x: 400, w: 44 }, { x: 0, w: 358 }, 16, 38), 400 + 44 + 38 - 358);
  // Same on the left, for a tab with one before it.
  assert.equal(cuonDeThay({ x: 140, w: 90 }, { x: 200, w: 358 }, 38, 16), 102);
});

// The window at full scroll (the last tab picked): which tab straddles the left edge.
const catNgangTrai = (rong, gap, khung, le, demCuoi) => {
  const tong = rong.reduce((a, b) => a + b, 0) + gap * (rong.length - 1) + le + demCuoi;
  const trai = tong - khung;
  let x = le;
  for (const w of rong) {
    if (x <= trai - 10 && x + w >= trai + 16) return true;
    x += w + gap;
  }
  return false;
};

test("cuộn hết sang phải (chọn tab cuối) thì mép trái cũng cắt ngang một chữ (verdict 03/10)", () => {
  const coChu = [
    [86, 88, 71, 41, 72],
    [117, 118, 96, 54, 94],
    [92, 93, 75, 44, 78],
  ];
  for (const rong of coChu) {
    for (let khung = 320; khung <= 540; khung++) {
      const { khoang: gap, demCuoi } = khoangCachLo(rong, khung, 16);
      const tong = rong.reduce((a, b) => a + b, 0) + gap * (rong.length - 1) + 16 + demCuoi;
      if (tong <= khung) continue;
      assert.ok(catNgangTrai(rong, gap, khung, 16, demCuoi), `mép trái không cắt chữ nào ở ${khung} dp (khoảng ${gap}, đệm ${demCuoi})`);
      assert.ok(catNgang(rong, gap, khung, 16), `mép phải không cắt chữ nào ở ${khung} dp (khoảng ${gap})`);
    }
  }
});
