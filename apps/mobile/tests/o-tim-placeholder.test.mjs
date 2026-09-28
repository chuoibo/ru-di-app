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

// QA 23/09: every multiline box started its text in the middle of the box.
test("ô nhiều dòng: chữ bắt đầu ở góc trên, padding trên dưới bằng nhau", async () => {
  const { kieuO } = await import("../dist-test/rudi/ui/Field.js");
  const k = kieuO({ multiline: true });
  assert.equal(k.nhap.textAlignVertical, "top");
  assert.equal(k.boc.justifyContent, "flex-start");
  assert.equal(k.boc.alignSelf, "stretch");
  assert.equal(k.khung.alignItems, "flex-start");
  assert.equal(k.khung.paddingTop, undefined, "không còn padding chỉ ở trên");
  assert.equal(typeof k.khung.paddingVertical, "number");
});

test("ô nhiều dòng: cao theo số dòng, có trần rồi cuộn", async () => {
  const { kieuO } = await import("../dist-test/rudi/ui/Field.js");
  const ba = kieuO({ multiline: true }).nhap;
  const nam = kieuO({ multiline: true, numberOfLines: 5 }).nhap;
  const hai_muoi = kieuO({ multiline: true, numberOfLines: 20 }).nhap;
  assert.ok(nam.minHeight > ba.minHeight);
  assert.equal(hai_muoi.minHeight, hai_muoi.maxHeight, "quá trần thì dừng ở trần");
  assert.ok(ba.maxHeight > ba.minHeight);
});

test("viền đang nhập dày 2dp mà chữ không xê dịch", async () => {
  const { kieuO } = await import("../dist-test/rudi/ui/Field.js");
  for (const multiline of [false, true]) {
    const thuong = kieuO({ multiline, vien: 1 }).khung;
    const dang = kieuO({ multiline, vien: 2 }).khung;
    assert.equal(dang.borderWidth, 2);
    assert.equal(thuong.paddingHorizontal + thuong.borderWidth, dang.paddingHorizontal + dang.borderWidth);
    if (multiline) assert.equal(thuong.paddingVertical + thuong.borderWidth, dang.paddingVertical + dang.borderWidth);
  }
});

test("lỗi thay dòng gợi ý và được đọc lên", () => {
  const html = renderToStaticMarkup(
    React.createElement(Field, { label: "Giờ", helper: "Dạng hh:mm", error: "Giờ phải dạng hh:mm", value: "25:00", onChangeText: () => undefined }),
  );
  assert.match(html, /Giờ phải dạng hh:mm/);
  assert.equal(html.includes("Dạng hh:mm<"), false, "gợi ý không hiện cùng lỗi");
  assert.match(html, /aria-live="polite"/);
});
