/**
 * Đổi cỡ chữ hệ thống không được đưa người dùng về màn đầu (QA 23/09: đang ở
 * nhắn riêng thì nhảy về Khám phá). Plugin thêm `fontScale|density` vào
 * `android:configChanges` của activity chính. Test chứng minh: plugin được khai
 * trong app.json, giữ nguyên các mục đã có, thêm đúng hai mục, chạy hai lần
 * không nhân đôi.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { THEM, giuManKhiDoiCoChu } from "../plugins/giu-man-khi-doi-co-chu.js";

const app = JSON.parse(readFileSync(fileURLToPath(new URL("../app.json", import.meta.url)), "utf8"));

function manifestGia(configChanges) {
  const activity = { $: { "android:name": ".MainActivity" }, "intent-filter": [{ action: [{ $: { "android:name": "android.intent.action.MAIN" } }], category: [{ $: { "android:name": "android.intent.category.LAUNCHER" } }] }] };
  if (configChanges !== undefined) activity.$["android:configChanges"] = configChanges;
  return { manifest: { $: {}, application: [{ $: { "android:name": ".MainApplication" }, activity: [activity] }] } };
}

const doi = (m) => m.manifest.application[0].activity[0].$["android:configChanges"].split("|");

test("app.json khai plugin", () => {
  assert.ok(app.expo.plugins.includes("./plugins/giu-man-khi-doi-co-chu"));
});

test("thêm fontScale và density, giữ nguyên các mục Expo đã có", () => {
  const m = giuManKhiDoiCoChu(manifestGia("keyboard|keyboardHidden|orientation|screenSize|uiMode"));
  assert.deepEqual(doi(m), ["keyboard", "keyboardHidden", "orientation", "screenSize", "uiMode", ...THEM]);
});

test("chạy hai lần không nhân đôi; manifest chưa có mục nào vẫn được thêm", () => {
  const m = giuManKhiDoiCoChu(giuManKhiDoiCoChu(manifestGia("uiMode|fontScale")));
  assert.deepEqual(doi(m), ["uiMode", "fontScale", "density"]);
  assert.deepEqual(doi(giuManKhiDoiCoChu(manifestGia(undefined))), ["fontScale", "density"]);
});
