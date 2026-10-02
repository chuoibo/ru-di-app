/**
 * The «Tạo» word under the stamp is part of the button (critique 02/10: it sat
 * outside the stamp's touch target, so a tap on the word did nothing while
 * the four neighbouring columns open on their word too).
 *
 * Read from the JSX: the stamp needs the router and Reanimated and is not
 * rendered here. The browser probe `kiem-ux/cham-lai-tab.mjs` taps the word.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";

const nguon = readFileSync(new URL("../src/rudi/ui/ConDauTao.tsx", import.meta.url), "utf8");
const cay = ts.createSourceFile("ConDauTao.tsx", nguon, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

function tim(nut, thu) {
  if (thu(nut)) return nut;
  return ts.forEachChild(nut, (con) => tim(con, thu));
}
const laNutDau = (n) =>
  ts.isJsxElement(n) &&
  n.openingElement.tagName.getText() === "PressScale" &&
  n.openingElement.attributes.properties.some((a) => ts.isJsxAttribute(a) && a.name.getText() === "testID" && a.initializer?.getText() === '"con-dau-tao"');

test("chữ «Tạo» nằm trong nút con dấu, nên chạm vào chữ cũng mở bàn Tạo", () => {
  const nut = tim(cay, laNutDau);
  assert.ok(nut, "không thấy PressScale testID=con-dau-tao");
  const chu = tim(nut, (n) => ts.isJsxText(n) && n.getText().trim() === "Tạo");
  assert.ok(chu, "chữ «Tạo» nằm ngoài nút con dấu");
  // The button still names itself in full for a screen reader.
  assert.match(nut.openingElement.getText(), /accessibilityLabel="Tạo mới"/);
});
