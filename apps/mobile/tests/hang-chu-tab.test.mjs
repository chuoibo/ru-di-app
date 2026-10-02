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
