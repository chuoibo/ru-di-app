/**
 * A community post's album (owner's choice, 02/10): one full-width picture per
 * page, with «2/3» over it. The page in view comes from the scroll offset.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { nhanTrang, trangAlbum } from "../dist-test/rudi/community/album.js";

test("trang album: làm tròn theo bề rộng trang, kẹp trong [0, n-1]", () => {
  assert.equal(trangAlbum(0, 350, 3), 0);
  assert.equal(trangAlbum(176, 350, 3), 1);
  assert.equal(trangAlbum(9999, 350, 3), 2);
  assert.equal(trangAlbum(-40, 350, 3), 0);
  assert.equal(trangAlbum(120, 0, 3), 0);
});

test("nhãn trang chỉ có khi album nhiều hơn một tấm", () => {
  assert.equal(nhanTrang(0, 1), null);
  assert.equal(nhanTrang(1, 3), "2/3");
});
