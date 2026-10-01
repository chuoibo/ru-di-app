/**
 * Khám phá's header (owner's mockup, 01/10): its two sections «Địa điểm» and
 * «Cộng đồng» as one tablist, the open one selected; the control on the
 * right (feed settings, or the demo door) sits outside the list, because a
 * tablist may own tabs only.
 *
 * Does not prove: the words fit at 320dp × 1.3 (that is the emulator shot) or
 * that the coral tape reads as «open» (that is the screenshot).
 */
import assert from "node:assert/strict";
import test from "node:test";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { Text } from "react-native-web";

import { DauKhamPha } from "../dist-test/rudi/ui/DauKhamPha.js";

const ve = (props) => renderToStaticMarkup(React.createElement(DauKhamPha, { onDoiMuc: () => {}, ...props }));
const cacTab = (html) =>
  [...html.matchAll(/<[^>]*role="tab"[^>]*>/g)].map(([the]) => ({
    nhan: the.match(/aria-label="([^"]*)"/)?.[1],
    chon: the.match(/aria-selected="([^"]*)"/)?.[1],
  }));

test("đầu Khám phá: một tablist, hai tab theo thứ tự Địa điểm, Cộng đồng", () => {
  const html = ve({ muc: "explore" });
  assert.equal((html.match(/role="tablist"/g) ?? []).length, 1);
  assert.deepEqual(cacTab(html), [
    { nhan: "Địa điểm", chon: "true" },
    { nhan: "Cộng đồng", chon: "false" },
  ]);
  assert.doesNotMatch(html, /aria-pressed/);
});

test("mục đang mở là Cộng đồng thì chỉ Cộng đồng được chọn", () => {
  assert.deepEqual(
    cacTab(ve({ muc: "community" })).map((t) => t.chon),
    ["false", "true"],
  );
});

test("ô phải nằm sau danh sách tab, không phải một tab", () => {
  const html = ve({ muc: "community", phai: React.createElement(Text, { testID: "o-phai" }, "x") });
  const viTriPhai = html.indexOf('data-testid="o-phai"');
  assert.ok(viTriPhai > html.lastIndexOf('role="tab"'));
  assert.equal(cacTab(html).length, 2);
});
