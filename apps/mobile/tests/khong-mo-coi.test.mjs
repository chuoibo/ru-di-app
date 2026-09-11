/** Câu cuối không rớt một chữ mồ côi: hai chữ cuối nối bằng NBSP (tái audit 10/09, ảnh 20 «lại.»). */
import assert from "node:assert/strict";
import test from "node:test";

import { NBSP, khongMoCoi } from "../dist-test/rudi/ui/chu.js";

test("nối hai chữ cuối của câu dài bằng NBSP", () => {
  const ra = khongMoCoi("Không nối được máy chủ. Kiểm tra mạng rồi thử lại.");
  assert.ok(ra.endsWith(`thử${NBSP}lại.`), ra);
  assert.equal(ra.replace(NBSP, " "), "Không nối được máy chủ. Kiểm tra mạng rồi thử lại.");
  assert.equal((ra.match(new RegExp(NBSP, "g")) ?? []).length, 1, "chỉ một NBSP");
});

test("câu dưới bốn chữ và chuỗi rỗng giữ nguyên", () => {
  assert.equal(khongMoCoi("Thử lại"), "Thử lại");
  assert.equal(khongMoCoi("Chưa có gì ở đây"), `Chưa có gì ở${NBSP}đây`);
  assert.equal(khongMoCoi(""), "");
});
