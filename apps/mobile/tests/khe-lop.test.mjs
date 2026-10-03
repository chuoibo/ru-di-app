/* The screen's overlay slot (QA UI-041).
 *
 * A sheet drawn inside a screen's body covered only the body: the header stayed
 * lit and live above its scrim. `LenLop` hands its children to the slot that
 * `RudiScreen` keeps beside its `overlay`, so they cover the whole screen while
 * keeping the state and callbacks of the component that drew them.
 *
 * Run under the real React with `react-test-renderer` (see
 * rudi-chat-useTinNhan.test.mjs for why that renderer).
 *
 * Not proven here: that the scrim covers the header on a real page (the B3
 * probe measures the hit point over «Quay lại»), or that an input inside the
 * slot never lags a keystroke in a browser (the browser test that typed into
 * the day editor drove the demo trip, and went with it on 2026-10-03).
 */
import assert from "node:assert/strict";
import test from "node:test";
import React, { act, useState } from "react";

const consoleErrorGoc = console.error;
console.error = (...args) => {
  if (String(args[0]).includes("react-test-renderer is deprecated")) return;
  consoleErrorGoc(...args);
};
test.after(() => {
  console.error = consoleErrorGoc;
});
const { default: TestRenderer } = await import("react-test-renderer");

import { KheLopProvider, LenLop, useKheLop } from "../dist-test/rudi/ui/KheLop.js";

const h = React.createElement;

/** A screen: its body, then its slot, as RudiScreen draws them. */
function Man({ children }) {
  const { khe, lop } = useKheLop();
  return h("man", null, h(KheLopProvider, { khe }, h("than", null, children)), h("khe", null, lop));
}

let datGiaTri = null;
let soLanVeCha = 0;
/** A component deep in the body that owns state and draws a layer. */
function BanDo({ mo = true }) {
  const [giaTri, setGiaTri] = useState("a");
  datGiaTri = setGiaTri;
  soLanVeCha += 1;
  return h("bando", null, mo ? h(LenLop, null, h("sheet", { giaTri })) : null);
}

const tim = (root, loai) => root.findAll((n) => n.type === loai);

test("không có khe: con vẽ tại chỗ", () => {
  let r;
  act(() => { r = TestRenderer.create(h(BanDo)); });
  assert.equal(tim(r.root, "sheet").length, 1);
  assert.equal(tim(r.root, "bando")[0].findAll((n) => n.type === "sheet").length, 1, "không khe thì vẽ trong chính component");
  act(() => r.unmount());
});

test("có khe: con lên khe của màn, giữ state của component đã vẽ nó, và rời khe khi gỡ", () => {
  let r;
  soLanVeCha = 0;
  act(() => { r = TestRenderer.create(h(Man, null, h(BanDo))); });
  const khe = tim(r.root, "khe")[0];
  const than = tim(r.root, "than")[0];
  assert.equal(khe.findAll((n) => n.type === "sheet").length, 1, "sheet phải nằm trong khe");
  assert.equal(than.findAll((n) => n.type === "sheet").length, 0, "sheet không còn trong thân màn");
  assert.equal(khe.findAll((n) => n.type === "sheet")[0].props.giaTri, "a");

  const truoc = soLanVeCha;
  act(() => datGiaTri("b"));
  assert.equal(tim(r.root, "khe")[0].findAll((n) => n.type === "sheet")[0].props.giaTri, "b", "state mới phải tới khe");
  // The slot updating re-renders the screen, not the component that drew the
  // layer: one render of BanDo for one state change, no loop.
  assert.equal(soLanVeCha - truoc, 1, "đổi khe không được vẽ lại component đã gửi lớp");

  act(() => r.update(h(Man, null, h(BanDo, { mo: false }))));
  assert.equal(tim(r.root, "khe")[0].findAll((n) => n.type === "sheet").length, 0, "gỡ LenLop thì khe trống");
  act(() => r.unmount());
});

test("hai lớp giữ thứ tự mở, mỗi lớp một chỗ", () => {
  const Hai = () => h(React.Fragment, null, h(LenLop, null, h("sheet", { ten: "mot" })), h(LenLop, null, h("sheet", { ten: "hai" })));
  let r;
  act(() => { r = TestRenderer.create(h(Man, null, h(Hai))); });
  assert.deepEqual(tim(r.root, "khe")[0].findAll((n) => n.type === "sheet").map((n) => n.props.ten), ["mot", "hai"]);
  act(() => r.unmount());
});
