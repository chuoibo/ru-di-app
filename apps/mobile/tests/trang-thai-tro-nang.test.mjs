/* State that reaches the DOM, for the controls QA found silent (UI-003, UI-053).
 *
 * `accessibilityState` arrives on the web as nothing (react-native-web 0.21
 * forwards no prop by that name), so the tab bar never said which tab was
 * current and a seat checkbox never said it was ticked. These render through
 * react-native-web -- the library the browser gets -- and read the markup.
 *
 * Not proven here: that a real screen reader announces it (that is a run with
 * one), or that every control in the app uses the helpers (that is the
 * migration, checked by the QA DOM sweep).
 */
import assert from "node:assert/strict";
import test from "node:test";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { Pressable, Text, View } from "react-native-web";

import { TABLIST, expandState, giuState, phimSpace, tabState, toggleState } from "../dist-test/ui/a11y.js";

const ve = (el) => renderToStaticMarkup(el);
const o = (props, chu = "Ô") => React.createElement(Pressable, props, React.createElement(Text, null, chu));

test("ô chọn mang role và aria-checked thật, đổi theo trạng thái", () => {
  const bat = ve(o(toggleState("checkbox", true)));
  const tat = ve(o(toggleState("checkbox", false)));
  assert.match(bat, /role="checkbox"/);
  assert.match(bat, /aria-checked="true"/);
  assert.match(tat, /aria-checked="false"/);
  assert.notEqual(bat, tat, "bật và tắt phải khác nhau trong DOM");
});

test("tab: role tab + aria-selected trong một tablist; không mang aria-checked", () => {
  const html = ve(React.createElement(View, TABLIST, o(tabState(true), "Khám phá"), o(tabState(false), "Lên plan")));
  assert.match(html, /role="tablist"/);
  assert.equal((html.match(/role="tab"/g) ?? []).length, 2);
  assert.match(html, /aria-selected="true"[^>]*>|<[^>]*aria-selected="true"/);
  assert.match(html, /aria-selected="false"/);
  assert.doesNotMatch(html, /aria-checked/);
});

test("nút giữ (chip lọc, ngày đã chọn) mang aria-pressed, không mang aria-selected trên role button", () => {
  const bat = ve(o({ accessibilityRole: "button", ...giuState(true) }));
  assert.match(bat, /aria-pressed="true"/);
  assert.doesNotMatch(bat, /aria-selected/);
  assert.match(ve(o({ accessibilityRole: "button", ...giuState(false) })), /aria-pressed="false"/);
});

test("mở gập mang aria-expanded", () => {
  assert.match(ve(o({ accessibilityRole: "button", ...expandState(true) })), /aria-expanded="true"/);
  assert.match(ve(o({ accessibilityRole: "button", ...expandState(false) })), /aria-expanded="false"/);
});

test("Space đổi ô trên web và không cuộn trang; phím khác không làm gì", () => {
  let lan = 0;
  let chan = 0;
  const { onKeyDown } = phimSpace(() => lan++);
  assert.equal(typeof onKeyDown, "function", "trên web phải có onKeyDown");
  onKeyDown({ key: " ", preventDefault: () => chan++ });
  onKeyDown({ key: "Spacebar", preventDefault: () => chan++ });
  onKeyDown({ key: "a", preventDefault: () => chan++ });
  onKeyDown({ key: "Enter", preventDefault: () => chan++ });
  assert.equal(lan, 2, "Space đổi đúng hai lần; Enter để press responder lo");
  assert.equal(chan, 2);
  assert.equal(typeof toggleState("radio", false, () => undefined).onKeyDown, "function");
  assert.equal(toggleState("radio", false).onKeyDown, undefined, "không truyền onToggle thì không gắn phím");
});
