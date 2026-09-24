/* Phiên trên web sống qua lần tải lại trang, mà token không nằm yên ở chỗ
 * script đọc được.
 *
 * `khoPhienWeb` giữ token trong bộ nhớ; thứ sống qua reload là cookie HttpOnly
 * do máy chủ đặt (services/core/internal/websession). Ca ở đây chạy hàm thật
 * với `fetch` giả: đo đúng request nào đi, mang `credentials: "include"` hay
 * không, và rằng Web Storage không bị đụng tới.
 *
 * Không chứng minh: trình duyệt thật có nhận cookie Secure/SameSite=Strict hay
 * không -- việc đó đo bằng Chromium trên stack thật, ghi trong commit.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { khoPhienWeb } from "../dist-test/phien-web.js";

const BASE = "http://api.test";
const PHIEN = {
  token: "tok-A",
  person_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  expires_at: "2026-10-24T00:00:00Z",
  context_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
  contexts: [],
};

/* The module calls the global `fetch` (so the contract gate can read its
 * paths); each case installs its own fake there. */
function mayChu(traLoi) {
  const goi = [];
  globalThis.fetch = async (url, init) => {
    goi.push({ url, ...init });
    const r = traLoi(url, init);
    if (r instanceof Error) throw r;
    return { ok: r.status >= 200 && r.status < 300, status: r.status, json: async () => r.body };
  };
  return { goi };
}

// Any Web Storage write fails the case: the whole point is that none happens.
for (const ten of ["localStorage", "sessionStorage"]) {
  globalThis[ten] = new Proxy({}, { get() { throw new Error(`${ten} bị đụng tới`); } });
}

test("tải lại trang: hỏi /resume kèm cookie, dựng lại phiên tối thiểu, chưa chọn nhóm", async () => {
  const { goi } = mayChu(() => ({
    status: 200,
    body: { token: "tok-A", person_id: PHIEN.person_id, expires_at: PHIEN.expires_at, issued_via: "otp", profile: { display_name: "Minh Anh" } },
  }));
  const kho = khoPhienWeb(BASE);
  const doc = JSON.parse(await kho.doc("rudi.phien"));
  assert.deepEqual(goi.map((g) => [g.url, g.method, g.credentials, g.headers]), [[`${BASE}/sessions/web/resume`, "POST", "include", undefined]]);
  assert.equal(doc.token, "tok-A");
  assert.equal(doc.context_id, null, "nhóm do khoiPhucPhien đọc lại, không đoán");
  assert.equal(doc.profile.display_name, "Minh Anh");
  await kho.doc("rudi.phien");
  assert.equal(goi.length, 1, "đã có trong bộ nhớ thì không hỏi lại");
});

test("không có cookie, cookie chết, mạng hỏng, câu trả lời méo: đều là 'chưa đăng nhập', không ném", async () => {
  for (const traLoi of [
    () => ({ status: 401, body: { code: "authentication_required" } }),
    () => ({ status: 403, body: { code: "origin_forbidden" } }),
    () => new Error("mạng"),
    () => ({ status: 200, body: { token: "", person_id: "x", expires_at: "y" } }),
    () => ({ status: 200, body: "rác" }),
  ]) {
    mayChu(traLoi);
    const kho = khoPhienWeb(BASE);
    assert.equal(await kho.doc("rudi.phien"), null);
  }
});

test("đăng nhập: đặt cookie một lần cho mỗi token, bằng header Bearer; đổi nhóm không tốn request", async () => {
  const { goi } = mayChu(() => ({ status: 204, body: null }));
  const kho = khoPhienWeb(BASE);
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  assert.deepEqual(goi.map((g) => [g.url, g.headers, g.credentials]), [[`${BASE}/sessions/web`, { Authorization: "Bearer tok-A" }, "include"]]);
  await kho.ghi("rudi.phien", JSON.stringify({ ...PHIEN, context_id: null }));
  assert.equal(goi.length, 1, "cùng token: không đặt lại cookie");
  await kho.ghi("rudi.phien", JSON.stringify({ ...PHIEN, token: "tok-B" }));
  assert.equal(goi.length, 2);
  assert.equal(JSON.parse(await kho.doc("rudi.phien")).token, "tok-B", "bộ nhớ là nguồn trong trang");
});

test("máy chủ không đặt được cookie thì vẫn đăng nhập trong trang, và lần ghi sau thử lại", async () => {
  let lan = 0;
  const { goi } = mayChu(() => (++lan === 1 ? new Error("mạng") : { status: 204, body: null }));
  const kho = khoPhienWeb(BASE);
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  assert.equal(JSON.parse(await kho.doc("rudi.phien")).token, "tok-A");
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  assert.equal(goi.length, 2, "lần trước hỏng thì lần này đặt lại");
});

test("đăng xuất: quên trong bộ nhớ và xoá cookie; lần đọc sau phải hỏi lại máy chủ", async () => {
  const { goi } = mayChu((url) => (url.endsWith("/resume") ? { status: 401, body: {} } : { status: 204, body: null }));
  const kho = khoPhienWeb(BASE);
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  await kho.xoa("rudi.phien");
  assert.equal(goi.at(-1).url, `${BASE}/sessions/web/clear`);
  assert.equal(goi.at(-1).credentials, "include");
  assert.equal(await kho.doc("rudi.phien"), null);
  assert.equal(goi.at(-1).url, `${BASE}/sessions/web/resume`);
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  assert.equal(goi.at(-1).url, `${BASE}/sessions/web`, "đăng nhập lại cùng token sau đăng xuất vẫn đặt lại cookie");
});
