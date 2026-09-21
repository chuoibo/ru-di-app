/**
 * Audit native 09/09, F44. Android lays the EditText HINT out at the view's
 * width and lets it wrap even in a single-line box, then clips the second line
 * (ảnh 20: «Tìm quán,» on one line, «món…» cut underneath). React Native gives
 * no `ellipsize` for TextInput, so the kit's `Field` draws the placeholder
 * itself: a `Text numberOfLines={1}` over the input, shown while the value is
 * empty, and the native `placeholder` prop is not passed at all.
 *
 * What this file CAN prove, from the markup react-native-web emits: the input
 * carries no `placeholder` attribute, the sentence is a text node of its own,
 * the input's accessible name is still the full sentence, and a typed value
 * hides the node. What it cannot prove is the wrapping itself -- that is what
 * the device screenshots at font scale 1.0/1.3/2.0 in
 * docs/archive/claude/2026-09-10/native-r12 are for.
 */
import assert from "node:assert/strict";
import test from "node:test";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { Field } from "../dist-test/rudi/ui/Field.js";

const CAU = "Tìm quán, món… hoặc hỏi Rủ Đi AI";

function ve(value) {
  return renderToStaticMarkup(
    React.createElement(Field, { onChangeText: () => undefined, placeholder: CAU, returnKeyType: "search", value }),
  );
}

test("ô tìm: placeholder là chữ của nhà vẽ, không phải hint native", () => {
  const html = ve("");
  const input = /<input\b[^>]*>/.exec(html)?.[0];
  assert.ok(input, `không có <input> trong markup:\n${html}`);
  assert.doesNotMatch(input, /\bplaceholder=/, `hint native vẫn được truyền xuống input: ${input}`);
  assert.match(html, new RegExp(`>${CAU.replace(/[.*+?^${}()|[\\]\\\\]/g, "\\\\$&")}<`), `câu placeholder không phải một node chữ riêng:\n${html}`);
});

test("ô tìm: tên trợ năng của input vẫn là trọn câu", () => {
  const input = /<input\b[^>]*>/.exec(ve(""))?.[0] ?? "";
  assert.match(input, new RegExp(`aria-label="${CAU}"`), `input mất tên trợ năng: ${input}`);
});

test("ô tìm: có chữ đã gõ thì placeholder biến mất", () => {
  const html = ve("phở");
  assert.equal(html.includes(`>${CAU}<`), false, `placeholder còn đè lên chữ đã gõ:\n${html}`);
  assert.match(html, /value="phở"/);
});
