/* A revoked session is noticed (ADR-0055): a password or email change, a
 * Google link or unlink, a reset or «đăng xuất tất cả» revokes every session
 * on the server, and a phone that kept the token would open signed in and
 * fail on every screen.
 *
 *   - any bearer call answered `401 authentication_required` forgets that
 *     bearer and tells the session provider, once;
 *   - an anonymous 401, or a 401 to a token that was already replaced, does not;
 *   - launch asks the server once and drops a revoked record, but an offline
 *     launch keeps the person signed in.
 *
 * Does not prove the provider routes to the sign-in door (that is a render);
 * it proves the signal the provider listens to, and what is left on disk.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien, tokenPhienHienTai, translatedAnonymous, translatedAsActor } from "../dist-test/api.js";
import { ngheThuHoiPhien } from "../dist-test/danh-tinh.js";
import { dangXuat, ghiNho, khoTrongBoNho, khoiPhucPhien, quenPhienDaThuHoi } from "../dist-test/phien.js";

const NGUOI = "2bb00000-bbbb-4bbb-8bbb-0000b0000002";
const NHOM = "1aa00000-aaaa-4aaa-8aaa-aaaaaaaaaaab";
const PHIEN = {
  token: "tok-song",
  person_id: NGUOI,
  context_id: null,
  membership_state: null,
  membership_id: null,
  expires_at: new Date(Date.now() + 86_400_000).toISOString(),
  issued_via: "password",
  contexts: [],
};

function json(body, status = 200) {
  return new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}

async function voiFetch(impl, fn) {
  const truoc = globalThis.fetch;
  globalThis.fetch = impl;
  try {
    return await fn();
  } finally {
    globalThis.fetch = truoc;
  }
}

function ngheThu() {
  const nghe = [];
  const bo = ngheThuHoiPhien((token) => nghe.push(token));
  return { nghe, bo };
}

test.afterEach(() => datTokenPhien(null));

test("401 authentication_required cho một lời gọi mang bearer: quên bearer và báo đúng một lần", async () => {
  const { nghe, bo } = ngheThu();
  datTokenPhien("tok-song");
  try {
    await voiFetch(
      async () => json({ code: "authentication_required" }, 401),
      () => assert.rejects(translatedAsActor({}, "/people/me", { method: "GET", actorId: NGUOI })),
    );
    assert.equal(tokenPhienHienTai(), null);
    assert.deepEqual(nghe, ["tok-song"]);
    // The next call carries no bearer, so a second 401 cannot loop back here.
    await voiFetch(
      async () => json({ code: "authentication_required" }, 401),
      () => assert.rejects(translatedAsActor({}, "/people/me", { method: "GET", actorId: NGUOI })),
    );
    assert.deepEqual(nghe, ["tok-song"]);
  } finally {
    bo();
  }
});

test("401 của lời gọi ẩn danh hoặc mã khác không đăng xuất ai", async () => {
  const { nghe, bo } = ngheThu();
  datTokenPhien("tok-song");
  try {
    await voiFetch(
      async () => json({ code: "authentication_required" }, 401),
      () => assert.rejects(translatedAnonymous({}, "/auth/login", { method: "POST", body: {} })),
    );
    // A wrong current password on reauth is the person's typo, not a dead session.
    await voiFetch(
      async () => json({ code: "credentials_invalid" }, 401),
      () => assert.rejects(translatedAsActor({}, "/people/me/account/reauth", { method: "POST", actorId: NGUOI, body: {} })),
    );
    await voiFetch(
      async () => json({ code: "reauthentication_required" }, 403),
      () => assert.rejects(translatedAsActor({}, "/people/me/account/password", { method: "PUT", actorId: NGUOI, body: {} })),
    );
    assert.equal(tokenPhienHienTai(), "tok-song");
    assert.deepEqual(nghe, []);
  } finally {
    bo();
  }
});

test("401 trả về muộn cho token cũ không xoá phiên mới đã xoay", async () => {
  const { nghe, bo } = ngheThu();
  datTokenPhien("tok-cu");
  let traLoi;
  try {
    const dangGoi = voiFetch(
      () => new Promise((r) => { traLoi = r; }),
      () => translatedAsActor({}, "/people/me", { method: "GET", actorId: NGUOI }),
    );
    datTokenPhien("tok-moi");
    traLoi(json({ code: "authentication_required" }, 401));
    await assert.rejects(dangGoi);
    assert.equal(tokenPhienHienTai(), "tok-moi");
    assert.deepEqual(nghe, []);
  } finally {
    bo();
  }
});

test("quenPhienDaThuHoi xoá bản ghi giữ đúng token đó, để yên bản ghi của phiên mới hơn", async () => {
  const kho = khoTrongBoNho();
  await ghiNho(PHIEN, kho);
  await quenPhienDaThuHoi("tok-khac", kho);
  assert.notEqual(await kho.doc("rudi.phien"), null, "phiên mới hơn bị xoá theo token cũ");
  await quenPhienDaThuHoi("tok-song", kho);
  assert.equal(await kho.doc("rudi.phien"), null);
});

test("mở app: máy chủ nói phiên đã bị thu hồi thì bỏ bản ghi, về cửa đăng nhập", async () => {
  const kho = khoTrongBoNho();
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  const goi = [];
  const phien = await voiFetch(async (url, init) => {
    goi.push({ url: String(url), init });
    return json({ code: "authentication_required" }, 401);
  }, () => khoiPhucPhien(kho));
  assert.equal(phien, null);
  assert.equal(tokenPhienHienTai(), null);
  assert.equal(await kho.doc("rudi.phien"), null);
  assert.equal(goi.length, 1);
  assert.match(goi[0].url, /\/people\/me\/contexts$/);
  assert.equal(goi[0].init.headers["Authorization"], "Bearer tok-song");
});

test("mở app khi mất mạng: vẫn đăng nhập, bản ghi giữ nguyên", async () => {
  const kho = khoTrongBoNho();
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  const phien = await voiFetch(async () => {
    throw new TypeError("Network request failed");
  }, () => khoiPhucPhien(kho));
  assert.equal(phien?.token, "tok-song");
  assert.equal(tokenPhienHienTai(), "tok-song");
  assert.equal(JSON.parse(await kho.doc("rudi.phien")).token, "tok-song");
});

test("mở app khi máy chủ lỗi 5xx: vẫn đăng nhập, không coi là bị thu hồi", async () => {
  const kho = khoTrongBoNho();
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  const phien = await voiFetch(async () => json({ code: "internal_error" }, 503), () => khoiPhucPhien(kho));
  assert.equal(phien?.token, "tok-song");
  assert.equal(tokenPhienHienTai(), "tok-song");
});

test("mở app khi phiên còn sống: danh sách nhóm được làm mới từ máy chủ", async () => {
  const kho = khoTrongBoNho();
  await kho.ghi("rudi.phien", JSON.stringify(PHIEN));
  const nhom = { id: NHOM, display_name: "Nhóm", my_state: "active", membership_id: "4dd00000-dddd-4ddd-8ddd-ddddddddddde", member_count: 2, unread_count: 0 };
  const phien = await voiFetch(async () => json({ contexts: [nhom] }), () => khoiPhucPhien(kho));
  assert.equal(phien?.context_id, NHOM);
  assert.equal(JSON.parse(await kho.doc("rudi.phien")).context_id, NHOM);
});

test("đăng xuất trả lời muộn không xoá phiên vừa đăng nhập lại", async () => {
  const kho = khoTrongBoNho();
  await ghiNho(PHIEN, kho);
  let traLoi;
  const dangXuatXong = voiFetch(() => new Promise((r) => { traLoi = r; }), () => dangXuat(NGUOI, kho));
  // Signed in again before the server answered the sign-out.
  await ghiNho({ ...PHIEN, token: "tok-moi" }, kho);
  traLoi(json(null, 204));
  await dangXuatXong;
  assert.equal(tokenPhienHienTai(), "tok-moi");
  assert.equal(JSON.parse(await kho.doc("rudi.phien")).token, "tok-moi");
});
