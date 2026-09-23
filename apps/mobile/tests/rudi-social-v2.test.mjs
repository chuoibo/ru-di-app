import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien, newAttempt } from "../dist-test/api.js";
import { docTrangTuong, docDoiTuong, ghepTrangTuong, guiTraLoi, thichBinhLuan, dangLaiBai, nhanBaiChoChatV2 } from "../dist-test/rudi/tuong/social-v2.js";

const actor = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
const post = "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee";
const parent = "cccccccc-bbbb-4ccc-8ddd-eeeeeeeeeeee";

test("wall pages dedupe overlapping posts without reordering old entries", () => {
  const a = { id: "a" }, b = { id: "b" }, c = { id: "c" };
  assert.deepEqual(ghepTrangTuong([a, b], [b, c]).map((item) => item.id), ["a", "b", "c"]);
});

test("social v2 sends bearer, cursor, parent and idempotency without plaintext chat", async () => {
  datTokenPhien("synthetic-session");
  const previous = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    return new Response(JSON.stringify(calls.length === 1 ? { person_id: actor, posts: [], next_cursor: null, has_more: false } : calls.length === 2 ? { events: [], next_cursor: null, has_more: false } : {}), { status: 200, headers: { "content-type": "application/json" } });
  };
  try {
    await docTrangTuong(actor, actor, "cursor+one");
    await docDoiTuong(actor, actor, "opaque-cursor", 20);
    await guiTraLoi(post, parent, "  Hay quá  ", actor, newAttempt());
    await thichBinhLuan(parent, actor, true);
    await dangLaiBai(post, "friends", actor, newAttempt());
  } finally {
    globalThis.fetch = previous;
    datTokenPhien(null);
  }
  assert.match(calls[0].url, /\/social\/v2\/people\/.*\/posts\?limit=20&cursor=cursor%2Bone$/);
  assert.match(calls[1].url, /\/social\/v2\/people\/.*\/changes\?after=opaque-cursor&wait=20$/);
  assert.deepEqual(JSON.parse(calls[2].init.body), { body: "Hay quá", parent_id: parent });
  assert.equal(calls[3].init.method, "PUT");
  assert.deepEqual(JSON.parse(calls[4].init.body), { audience: "friends" });
  for (const call of calls) assert.equal(call.init.headers.Authorization, "Bearer synthetic-session");
  assert.ok(calls[2].init.headers["Idempotency-Key"]);
  assert.ok(calls[4].init.headers["Idempotency-Key"]);
});

test("chat v2 adapter creates an opaque post reference and never a plaintext message", () => {
  assert.deepEqual(nhanBaiChoChatV2(post), { kind: "post_reference", post_id: post });
});
