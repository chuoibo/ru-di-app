import assert from "node:assert/strict";
import { test } from "node:test";

import { tiLeKhung } from "../dist-test/rudi/ky-niem/ti-le.js";

test("khung ảnh theo tỉ lệ ảnh, giữ trong 3:4 … 1.91:1", () => {
  assert.equal(tiLeKhung({ width: 3000, height: 4000 }), 0.75, "ảnh dọc điện thoại vừa khít, không dải xám, không cắt");
  assert.equal(tiLeKhung({ width: 4000, height: 3000 }), 4 / 3);
  assert.equal(tiLeKhung({ width: 1000, height: 3000 }), 0.75, "ảnh quá dọc thì dừng ở 3:4");
  assert.equal(tiLeKhung({ width: 4000, height: 1000 }), 1.91);
  assert.equal(tiLeKhung(null), 4 / 3, "chưa biết kích thước thì 4:3 trong lúc tải");
  assert.equal(tiLeKhung({ width: 0, height: 10 }, 1), 1);
});
