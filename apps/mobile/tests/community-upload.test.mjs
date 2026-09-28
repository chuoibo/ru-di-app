import test from "node:test";
import assert from "node:assert/strict";
import { uploadMedia } from "../dist-test/rudi/community/api.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";

function nativeBlob(bytes) {
  const source = new Blob([bytes]);
  const closed = { source: 0, body: 0 };
  source.close = () => { closed.source++; };
  const slice = source.slice.bind(source);
  source.slice = (...args) => {
    const body = slice(...args);
    body.close = () => { closed.body++; };
    return body;
  };
  return { source, closed };
}

// The native E2E supplies the platform evidence. These cases protect the wire
// contract and native Blob lifetime without claiming to emulate Android.
for (const mime of ["image/jpeg", "video/mp4"]) {
  test(`upload keeps binary bytes and MIME on an untyped local Blob: ${mime}`, async (t) => {
    const original = globalThis.fetch;
    t.after(() => { globalThis.fetch = original; datTokenPhien(null); });
    datTokenPhien("synthetic-upload-session");
    const bytes = Uint8Array.of(0, 255, 17, 128, 42);
    const { source, closed } = nativeBlob(bytes);
    let calls = 0;
    globalThis.fetch = async (url, init) => {
      calls++;
      if (calls === 1) {
        assert.equal(url, "file:///synthetic-media");
        return { blob: async () => source };
      }
      assert.equal(init.method, "POST");
      assert.equal(init.headers.Authorization, "Bearer synthetic-upload-session");
      assert.equal(init.headers["Content-Type"], mime);
      assert.equal(init.body.type, mime);
      assert.deepEqual(new Uint8Array(await init.body.arrayBuffer()), bytes);
      assert.deepEqual(closed, { source: 0, body: 0 }, "native bytes released while request was in flight");
      return { ok: true, json: async () => ({ id: "synthetic-result", state: "ready" }) };
    };
    assert.equal((await uploadMedia("synthetic-actor", "file:///synthetic-media", mime)).id, "synthetic-result");
    assert.equal(calls, 2);
    assert.deepEqual(closed, { source: 1, body: 1 });
  });
}

test("upload failure releases the native Blob and preserves a readable rejection", async (t) => {
  const original = globalThis.fetch;
  t.after(() => { globalThis.fetch = original; });
  const { source, closed } = nativeBlob("synthetic");
  globalThis.fetch = async (url, init) => init
    ? { ok: false, json: async () => ({ code: "unsupported_media" }) }
    : { blob: async () => source };
  await assert.rejects(uploadMedia("synthetic-actor", "file:///synthetic-media", "image/jpeg"), /JPEG.*PNG/);
  assert.deepEqual(closed, { source: 1, body: 1 });
});

test("failure to create a typed Blob still releases the native source", async (t) => {
  const original = globalThis.fetch;
  t.after(() => { globalThis.fetch = original; });
  const { source, closed } = nativeBlob("synthetic");
  source.slice = () => { throw new Error("native allocation failed"); };
  globalThis.fetch = async () => ({ blob: async () => source });
  await assert.rejects(uploadMedia("synthetic-actor", "file:///synthetic-media", "image/jpeg"), /native allocation failed/);
  assert.deepEqual(closed, { source: 1, body: 0 });
});
