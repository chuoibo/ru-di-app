import assert from "node:assert/strict";
import test from "node:test";
import { googleConfigured, googleSession } from "../dist-test/rudi/google.js";

const client = "synthetic-test.apps.googleusercontent.com";
test("Google chỉ hiện khi native có cấu hình đúng nền tảng", () => {
  assert.equal(googleConfigured("web", client, client), false);
  assert.equal(googleConfigured("android"), false);
  assert.equal(googleConfigured("android", client), true);
  assert.equal(googleConfigured("ios", client), false);
  assert.equal(googleConfigured("ios", client, client), true);
  assert.equal(googleConfigured("android", " "), false);
});
test("hủy chọn và thiếu id token không gọi API", async () => {
  let called = 0;
  const exchange = async () => { called++; };
  assert.equal(await googleSession(async () => ({ type: "cancelled" }), exchange), null);
  await assert.rejects(googleSession(async () => ({ type: "success", data: { idToken: null } }), exchange));
  assert.equal(called, 0);
});
test("chỉ đổi id token, không gửi email hoặc profile", async () => {
  let sent;
  // repo-guard: allow=email reason=synthetic-unused-google-profile
  const result = await googleSession(async () => ({ type: "success", data: { idToken: "synthetic-provider-proof", email: "unused@example.invalid" } }), async token => {
    sent = token;
    return { issued_via: "google" };
  });
  assert.equal(sent, "synthetic-provider-proof");
  assert.deepEqual(result, { issued_via: "google" });
});
