/**
 * A community post's album (owner's choice, 02/10): one full-width picture per
 * page, with «2/3» over it. The page in view comes from the scroll offset.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { readFileSync } from "node:fs";

import { TI_LE_ALBUM, kichTrangAlbum, nhanTrang, trangAlbum } from "../dist-test/rudi/community/album.js";

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

test("trang album 4:3 rộng đúng cột", () => {
  assert.equal(TI_LE_ALBUM, 4 / 3);
  assert.deepEqual(kichTrangAlbum(358), { width: 358, height: 269 });
});

// Review toàn nhánh 02/10: a card drew no album until onLayout measured it, so
// a card the FlatList remounted above the viewport was ~270 dp short for a
// frame and the native feed jumped. The wrapper holds the 4:3 room from the
// first render.
test("khung album giữ chỗ 4:3 từ lần vẽ đầu, trước khi đo", () => {
  const nguon = readFileSync(new URL("../src/rudi/community/PostCard.tsx", import.meta.url), "utf8");
  assert.match(nguon, /khungAlbum:\s*\{[^}]*aspectRatio:\s*TI_LE_ALBUM/);
  assert.match(nguon, /style=\{styles\.khungAlbum\}/);
});
