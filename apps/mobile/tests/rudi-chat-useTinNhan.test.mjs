/**
 * F42 (audit native 09/09): a response that arrives after the hook has moved
 * on -- to another group, another person, or off the screen -- must not be
 * written into whatever is on screen now.
 *
 * This runs the REAL hook under the REAL React. The renderer is
 * `react-test-renderer`, pinned to the exact React version; React 19 marks it
 * deprecated and says so once on import (filtered below, with the reason). The
 * alternative, `react-dom/client` under a fake DOM, would flip react-native-web
 * into its browser branches (`canUseDOM`, `AppState.isAvailable`) for every
 * other test in this suite, which is a larger change than the one being proved.
 *
 * The transport is `globalThis.fetch`, answered BY HAND: every request is parked
 * in `daGoi` and the test decides when, and with what, each one comes back. The
 * subject here is WHEN, and a stub of `docTrangTin` could not carry it -- the
 * module's ESM exports are immutable and the hook holds them by name.
 *
 * The reviewer reproduced the first case with a hand-rolled hook scheduler
 * (docs/codex/2026-09-09/native-audit-evidence/probe-late-context.mjs). These
 * are the same scenario under React's own effect ordering, plus the cases that
 * probe did not reach: a late first page, a late FAILURE, unmount, a person
 * change, and the read mark. The retry case is the F32 contract this fix must
 * not break: a retry in the SAME conversation still replays the same key.
 */
import assert from "node:assert/strict";
import test from "node:test";
import React, { act } from "react";

// react-test-renderer announces its own deprecation with console.error, once
// per root it creates. It is the one line this file knowingly accepts; anything
// else on console.error is still an unexpected message and stays visible.
const consoleErrorGoc = console.error;
console.error = (...args) => {
  if (String(args[0]).includes("react-test-renderer is deprecated")) return;
  consoleErrorGoc(...args);
};
test.after(() => {
  console.error = consoleErrorGoc;
});
const { default: TestRenderer } = await import("react-test-renderer");

import { AppState } from "react-native-web";

import { datTokenPhien } from "../dist-test/danh-tinh.js";
import { useBanNhap } from "../dist-test/rudi/chat/useBanNhap.js";
import { useTinNhan } from "../dist-test/rudi/chat/useTinNhan.js";
import { CAU_HINH } from "./stubs/expo-router.mjs";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const A = "3cc00000-cccc-4ccc-8ccc-0000c000000a";
const B = "3cc00000-cccc-4ccc-8ccc-0000c000000b";
const P1 = "4dd00000-dddd-4ddd-8ddd-0000d0000001";
const P2 = "4dd00000-dddd-4ddd-8ddd-0000d0000002";

function tin(ctx, id, at = "2030-09-09T12:00:00Z") {
  return { id: `${ctx.slice(-1)}-${id}`, context_id: ctx, author_id: P1, kind: "sticker", body: "cho-ti", image_url: null, card: null, cursor: `${ctx.slice(-1)}-${id}`, created_at: at };
}
function trang(ctx, ...ids) {
  return { context_id: ctx, messages: ids.map((id) => tin(ctx, id)), next_cursor: null, has_more: false };
}
function traLoi(body, status = 200) {
  return { ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) };
}
const LOI_MAY_CHU = { code: "boom", detail: "Máy chủ đang hỏng." };

/** Park every request; the test answers each one when it chooses. */
function batFetch() {
  const daGoi = [];
  globalThis.fetch = (url, init) =>
    new Promise((resolve) => {
      daGoi.push({ url, method: init?.method ?? "GET", init, resolve: (body, status) => resolve(traLoi(body, status)) });
    });
  return daGoi;
}
const cuaNhom = (daGoi, ctx, method) => daGoi.filter((g) => g.url.includes(`/contexts/${ctx}/`) && (method === undefined || g.method === method));

let ketQua;
function Probe({ contextId, personId }) {
  ketQua = useTinNhan(contextId, personId);
  return null;
}
async function dung(contextId, personId = P1) {
  let root;
  await act(async () => {
    root = TestRenderer.create(React.createElement(Probe, { contextId, personId }));
  });
  return root;
}
async function doi(root, contextId, personId = P1) {
  await act(async () => {
    root.update(React.createElement(Probe, { contextId, personId }));
  });
}
async function tra(g, body, status) {
  await act(async () => {
    g.resolve(body, status);
  });
}
async function bo(root) {
  await act(async () => {
    root.unmount();
  });
}
const ids = () => ketQua.tin.map((t) => t.id);

test.beforeEach(() => {
  datTokenPhien("tok-f42");
  CAU_HINH.focus = false;
});

test("F42: sticker gửi ở A về muộn không ghép vào B, và không kéo thêm lượt đọc A", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  await tra(daGoi[0], trang(A, "cu"));
  assert.deepEqual(ids(), ["a-cu"]);

  let gui;
  await act(async () => {
    gui = ketQua.guiSticker("cho-ti");
  });
  assert.equal(daGoi[1].method, "POST", "sticker phải đi ra ngay khi bấm");

  await doi(root, B);
  await tra(cuaNhom(daGoi, B, "GET")[0], trang(B, "cu"));
  assert.deepEqual(ids(), ["b-cu"]);

  const truoc = daGoi.length;
  await act(async () => {
    daGoi[1].resolve({ ...tin(A, "muon"), intent: null }, 201);
    await gui;
  });
  assert.deepEqual(ids(), ["b-cu"], "tin của A lọt vào danh sách của B");
  assert.equal(daGoi.length, truoc, `phản hồi muộn của A kéo thêm ${daGoi.length - truoc} lượt gọi (napMoi đóng trên A)`);
  await bo(root);
});

test("F42: trang đầu của A về muộn không đè lên trang đầu của B", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  await doi(root, B);
  await tra(cuaNhom(daGoi, B, "GET")[0], trang(B, "cu"));
  assert.deepEqual(ids(), ["b-cu"]);
  await tra(cuaNhom(daGoi, A, "GET")[0], trang(A, "cu"));
  assert.deepEqual(ids(), ["b-cu"], "trang của A đè lên B");
  assert.equal(ketQua.dangNap, false);
  await bo(root);
});

test("F42: gửi ở A hỏng muộn khi đang ở B thì B không có lỗi, không có hàng chờ, và không ném lên màn", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  await tra(daGoi[0], trang(A, "cu"));
  let gui;
  await act(async () => {
    gui = ketQua.guiSticker("cho-ti");
  });
  assert.equal(ketQua.hangCho.length, 1, "hàng chờ của A phải có một dòng khi đang gửi");

  await doi(root, B);
  await tra(cuaNhom(daGoi, B, "GET")[0], trang(B, "cu"));
  assert.equal(ketQua.hangCho.length, 0, "hàng chờ không đi theo sang nhóm khác (R2 09/09)");

  let kq = "chua-tra";
  await act(async () => {
    daGoi[1].resolve(LOI_MAY_CHU, 500);
    await assert.doesNotReject(async () => {
      kq = await gui;
    }, "lỗi muộn của A ném lên màn đang hiện B");
  });
  assert.equal(kq, null, "gửi cho một cuộc hội thoại đã rời màn phải trả null, không phải tin");
  assert.equal(ketQua.loi, null);
  assert.deepEqual(ketQua.hangCho, []);
  assert.deepEqual(ids(), ["b-cu"]);
  await bo(root);
});

test("F42: sau unmount, phản hồi muộn không kéo thêm lượt gọi nào", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  await tra(daGoi[0], trang(A, "cu"));
  let gui;
  await act(async () => {
    gui = ketQua.guiSticker("cho-ti");
  });
  await bo(root);
  const truoc = daGoi.length;
  await act(async () => {
    daGoi[1].resolve({ ...tin(A, "muon"), intent: null }, 201);
    await gui;
  });
  assert.equal(daGoi.length, truoc, "hook đã unmount vẫn poll");
});

test("F42: đổi người trên cùng nhóm, trang của người cũ về muộn không đè lên trang của người mới", async () => {
  const daGoi = batFetch();
  const root = await dung(A, P1);
  await doi(root, A, P2);
  assert.equal(daGoi.length, 2, "mỗi người một lượt đọc");
  await tra(daGoi[1], trang(A, "cua-p2"));
  await tra(daGoi[0], trang(A, "cua-p1"));
  assert.deepEqual(ids(), ["a-cua-p2"]);
  await bo(root);
});

test("F32 giữ nguyên: thử lại trong CÙNG cuộc hội thoại gửi lại đúng Idempotency-Key cũ", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  await tra(daGoi[0], trang(A, "cu"));
  let gui;
  await act(async () => {
    gui = ketQua.guiSticker("cho-ti");
  });
  await act(async () => {
    daGoi[1].resolve(LOI_MAY_CHU, 500);
    await gui.catch(() => undefined);
  });
  assert.equal(ketQua.hangCho[0]?.trangThai, "that-bai");
  assert.equal(ketQua.hangCho[0]?.loi, "Máy chủ đang hỏng.");
  const khoa = ketQua.hangCho[0].attempt.key;

  let lai;
  await act(async () => {
    lai = ketQua.thuLaiMot(khoa);
  });
  assert.equal(daGoi[2].method, "POST");
  assert.equal(daGoi[2].init.headers["Idempotency-Key"], daGoi[1].init.headers["Idempotency-Key"], "thử lại phải mang cùng một chìa");
  await act(async () => {
    daGoi[2].resolve({ ...tin(A, "moi", "2030-09-09T12:00:01Z"), intent: null }, 201);
    await lai;
  });
  assert.deepEqual(ids(), ["a-moi", "a-cu"]);
  assert.deepEqual(ketQua.hangCho, []);
  await bo(root);
});

test("F42: đổi nhóm khi màn đang focus không PUT read-mark của B bằng id tin của A", async () => {
  // The focused path: react-native-web has no DOM here, so `AppState` is told
  // it is in the background (no poll interval) and given a listener handle
  // the hook's cleanup can call `remove()` on.
  const daGoi = batFetch();
  CAU_HINH.focus = true;
  const moTaCu = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const addCu = AppState.addEventListener;
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => "background" });
  AppState.addEventListener = () => ({ remove() {} });
  try {
    const root = await dung(A);
    await tra(cuaNhom(daGoi, A, "GET")[0], trang(A, "cu"));
    const putA = cuaNhom(daGoi, A, "PUT");
    assert.equal(putA.length, 0, "tải tin trong nền không được coi là đã đọc");
    await act(async () => ketQua.danhDauHienThi(["a-cu"]));
    assert.equal(cuaNhom(daGoi, A, "PUT").length, 0, "viewport trong nền không được ghi đã đọc");

    await doi(root, B);
    await tra(cuaNhom(daGoi, B, "GET")[0], trang(B, "cu"));
    for (const g of cuaNhom(daGoi, B, "PUT")) {
      const { message_id } = JSON.parse(g.init.body);
      assert.ok(message_id.startsWith("b-"), `read-mark của B mang id ${message_id} của nhóm khác`);
    }
    await bo(root);
  } finally {
    Object.defineProperty(AppState, "currentState", moTaCu);
    AppState.addEventListener = addCu;
    CAU_HINH.focus = false;
  }
});


async function voiForeground(chay) {
  const moTaCu = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const addCu = AppState.addEventListener;
  CAU_HINH.focus = true;
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => "active" });
  AppState.addEventListener = () => ({ remove() {} });
  const root = await dung(A);
  try { await chay(root); }
  finally {
    await bo(root);
    Object.defineProperty(AppState, "currentState", moTaCu);
    AppState.addEventListener = addCu;
    CAU_HINH.focus = false;
  }
}

test("đã đọc chỉ theo viewport: tải tin mới không đánh dấu, nhìn tin cũ không nhảy tới mới", async () => {
  const daGoi = batFetch();
  await voiForeground(async () => {
    await tra(daGoi[0], { ...trang(A), messages: [tin(A, "moi", "2030-09-09T12:01:00Z"), tin(A, "cu")] });
    assert.equal(cuaNhom(daGoi, A, "PUT").length, 0);
    await act(async () => ketQua.danhDauHienThi(["a-cu", "b-khong-thuoc-nhom"]));
    const req = cuaNhom(daGoi, A, "PUT")[0];
    assert.equal(JSON.parse(req.init.body).message_id, "a-cu");
    await tra(req, null, 204);
    await act(async () => ketQua.danhDauHienThi(["a-cu"]));
    assert.equal(cuaNhom(daGoi, A, "PUT").length, 1);
  });
});

test("đã đọc chờ ACK, gộp viewport mới hơn và không ghi lùi khi cuộn lên", async () => {
  const daGoi = batFetch();
  await voiForeground(async () => {
    await tra(daGoi[0], { ...trang(A), messages: [tin(A, "moi", "2030-09-09T12:01:00Z"), tin(A, "cu")] });
    await act(async () => ketQua.danhDauHienThi(["a-cu"]));
    await act(async () => ketQua.danhDauHienThi(["a-moi"]));
    assert.equal(cuaNhom(daGoi, A, "PUT").length, 1, "không ghi hai ACK cạnh tranh");
    await tra(cuaNhom(daGoi, A, "PUT")[0], null, 204);
    const puts = cuaNhom(daGoi, A, "PUT");
    assert.equal(puts.length, 2);
    assert.equal(JSON.parse(puts[1].init.body).message_id, "a-moi");
    await tra(puts[1], null, 204);
    await act(async () => ketQua.danhDauHienThi(["a-cu"]));
    assert.equal(cuaNhom(daGoi, A, "PUT").length, 2);
  });
});

test("ACK đã đọc thất bại được thử lại cùng watermark", async () => {
  const daGoi = batFetch();
  await voiForeground(async () => {
    await tra(daGoi[0], trang(A, "cu"));
    await act(async () => ketQua.danhDauHienThi(["a-cu"]));
    await tra(cuaNhom(daGoi, A, "PUT")[0], LOI_MAY_CHU, 500);
    await act(async () => ketQua.danhDauHienThi(["a-cu"]));
    const puts = cuaNhom(daGoi, A, "PUT");
    assert.equal(puts.length, 2, "ACK lỗi không được lưu thành đã thành công");
    assert.equal(JSON.parse(puts[1].init.body).message_id, "a-cu");
    await tra(puts[1], null, 204);
  });
});

test("text gửi cạnh tranh giữ từng hàng lỗi và retry đúng nội dung, quote, key", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  try {
    await tra(daGoi[0], trang(A, "cu"));
    let mot, hai;
    const quote = { id: "a-cu", author_id: P1, preview: "Tin cũ" };
    await act(async () => {
      mot = ketQua.gui("Tin thứ nhất", quote).catch(() => undefined);
      hai = ketQua.gui("Tin thứ hai").catch(() => undefined);
    });
    const posts = cuaNhom(daGoi, A, "POST");
    assert.equal(ketQua.hangCho.length, 2);
    await act(async () => { posts[1].resolve(LOI_MAY_CHU, 500); await hai; });
    await act(async () => { posts[0].resolve(LOI_MAY_CHU, 500); await mot; });
    const row = ketQua.hangCho.find((r) => r.than === "Tin thứ nhất");
    assert.equal(row.trangThai, "that-bai");
    let retry;
    await act(async () => { retry = ketQua.thuLaiMot(row.attempt.key); });
    const repeat = cuaNhom(daGoi, A, "POST")[2];
    assert.equal(repeat.init.headers["Idempotency-Key"], posts[0].init.headers["Idempotency-Key"]);
    assert.deepEqual(JSON.parse(repeat.init.body), JSON.parse(posts[0].init.body));
    await act(async () => { repeat.resolve({ ...tin(A, "text"), kind: "text", body: "Tin thứ nhất" }, 201); await retry; });
    assert.deepEqual(ketQua.hangCho.map((r) => r.than), ["Tin thứ hai"]);
  } finally { await bo(root); }
});

test("ACK text đã tới trước trang đầu không bị trang đầu đến muộn làm mất", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  try {
    let sending;
    await act(async () => { sending = ketQua.gui("Tin mới"); });
    await act(async () => { daGoi[1].resolve({ ...tin(A, "moi", "2030-09-09T12:01:00Z"), kind: "text" }, 201); await sending; });
    await tra(daGoi[0], trang(A, "cu"));
    assert.deepEqual(ids(), ["a-moi", "a-cu"]);
  } finally { await bo(root); }
});

test("hai lần chủ động gửi cùng chữ là hai tin, retry khi đang gửi không phát lại", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  try {
    await tra(daGoi[0], trang(A, "cu"));
    let mot, hai;
    await act(async () => { mot = ketQua.gui("Đồng ý").catch(() => undefined); });
    const key = ketQua.hangCho[0].attempt.key;
    await act(async () => { assert.equal(await ketQua.thuLaiMot(key), null); });
    await act(async () => { hai = ketQua.gui("Đồng ý").catch(() => undefined); });
    const posts = cuaNhom(daGoi, A, "POST");
    assert.equal(posts.length, 2);
    assert.notEqual(posts[0].init.headers["Idempotency-Key"], posts[1].init.headers["Idempotency-Key"]);
    await act(async () => { for (const req of posts) req.resolve(LOI_MAY_CHU, 500); await Promise.all([mot, hai]); });
  } finally { await bo(root); }
});


test("callback gửi cũ sau đổi account hoặc unmount không tạo HTTP mới", async () => {
  const daGoi = batFetch();
  const root = await dung(A);
  const guiCu = ketQua.gui;
  await doi(root, A, P2);
  const before = daGoi.length;
  await act(async () => assert.equal(await guiCu("Không được gửi"), null));
  assert.equal(daGoi.length, before);
  const guiDaBo = ketQua.gui;
  await bo(root);
  await act(async () => assert.equal(await guiDaBo("Không được gửi"), null));
  assert.equal(daGoi.length, before);
});

test("ACK ảnh không xoá bản nháp mới, kể cả khi gõ lại cùng chữ", async () => {
  let draft;
  function Composer() { draft = useBanNhap(); return null; }
  let root;
  await act(async () => { root = TestRenderer.create(React.createElement(Composer)); });
  try {
    await act(async () => draft.change("Chú thích"));
    const uploaded = draft.snapshot.current;
    await act(async () => { draft.change(""); draft.change("Chú thích"); });
    await act(async () => draft.clearIfUnchanged(uploaded.revision));
    assert.equal(draft.text, "Chú thích", "cùng chữ nhưng khác lần nhập phải được giữ");
    const unchanged = draft.snapshot.current;
    await act(async () => draft.clearIfUnchanged(unchanged.revision));
    assert.equal(draft.text, "", "caption chưa thay đổi được xoá sau ACK");
  } finally { await bo(root); }
});
