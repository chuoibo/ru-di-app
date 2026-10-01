/* «Quay lại» always goes somewhere (QA UI-018).
 *
 * A screen opened cold from a link has nothing behind it, and `router.back()`
 * did nothing at all. `luiVe` goes back through history when there is any,
 * else to the tab the route belongs to.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { duongCha, luiVe } from "../dist-test/rudi/lui-ve.js";

const router = (coLichSu) => {
  const goi = [];
  return { goi, canGoBack: () => coLichSu, back: () => goi.push(["back"]), replace: (h) => goi.push(["replace", h]) };
};

test("có lịch sử thì lùi; không có thì về tab của route", () => {
  const a = router(true);
  luiVe(a, "/outings/abc");
  assert.deepEqual(a.goi, [["back"]]);
  const b = router(false);
  luiVe(b, "/outings/abc");
  assert.deepEqual(b.goi, [["replace", "/plan"]]);
});

test("mỗi họ route về đúng tab; route lạ về cửa của app", () => {
  assert.equal(duongCha("/community/posts/p1"), "/community");
  assert.equal(duongCha("/places/banh-can-le"), "/explore");
  assert.equal(duongCha("/groups/g1/chat?x=1"), "/messages");
  assert.equal(duongCha("/settings/blocked"), "/profile");
  assert.equal(duongCha("/login"), "/");
  assert.equal(duongCha("/"), "/");
});

test("màn tự chọn nơi về thì dùng nơi đó", () => {
  const r = router(false);
  luiVe(r, "/outings/abc", "/messages");
  assert.deepEqual(r.goi, [["replace", "/messages"]]);
});
