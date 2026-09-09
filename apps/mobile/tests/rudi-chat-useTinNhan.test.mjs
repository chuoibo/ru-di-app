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
    assert.equal(putA.length, 1, "đang focus, tin mới nhất của A phải được đánh dấu đã đọc");
    assert.equal(JSON.parse(putA[0].init.body).message_id, "a-cu");
    await tra(putA[0], null, 204);

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
