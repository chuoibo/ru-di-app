import assert from "node:assert/strict";
import test from "node:test";
import React, { act } from "react";
import TestRenderer from "react-test-renderer";
import { AppState } from "react-native-web";
import { CAU_HINH } from "./stubs/expo-router.mjs";
import { datTokenPhien } from "../dist-test/danh-tinh.js";
import { useChatChanges } from "../dist-test/rudi/chat/useChatChanges.js";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const context = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const actor = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const empty = (watermark = 10, contextId = context) => ({ context_id: contextId, watermark, messages: [], votes: [] });
const page = (sequence, changes = []) => ({ context_id: context, changes, next_sequence: sequence, watermark: sequence, has_more: false });

test("Feed hydrates before ACK, keeps snapshot watermark out of cursor and drops late-room work", async () => {
  const originalFetch = globalThis.fetch;
  const originalSocket = globalThis.WebSocket;
  const originalState = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const originalListener = AppState.addEventListener;
  const originalError = console.error;
  const pending = [];
  const sockets = [];
  const snapshots = [];
  let root;
  console.error = (...args) => { if (!String(args[0]).includes("react-test-renderer is deprecated")) originalError(...args); };
  class Socket {
    static OPEN = 1;
    readyState = 0;
    sent = [];
    constructor(url) { this.url = url; sockets.push(this); }
    send(value) { this.sent.push(JSON.parse(value)); }
    close() { this.readyState = 3; this.onclose?.(); }
  }
  globalThis.WebSocket = Socket;
  globalThis.fetch = (url, init) => new Promise((resolve) => pending.push({ url, init, resolve }));
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => "active" });
  AppState.addEventListener = () => ({ remove() {} });
  CAU_HINH.focus = true;
  datTokenPhien("synthetic-feed-token");
  const apply = (snapshot) => snapshots.push(snapshot);
  function Probe({ room }) { useChatChanges(room, actor, apply); return null; }
  const respond = async (index, data) => act(async () => pending[index].resolve({ ok: true, status: 200, json: async () => data, text: async () => JSON.stringify(data) }));
  try {
    await act(async () => { root = TestRenderer.create(React.createElement(Probe, { room: context })); });
    await respond(0, empty());
    assert.ok(pending[1].url.includes("after=10"));
    await respond(1, page(11, [{ sequence: 11, type: "message", entity_id: "m", revision: 11 }]));
    assert.equal(sockets.length, 0, "No stream until the HTTP page is applied");
    await respond(2, empty(14));
    assert.ok(sockets[0].url.endsWith("after=11"), "Hydration watermark must not skip unrelated changes");
    assert.equal(sockets[0].url.includes("token"), false);
    await act(async () => { sockets[0].readyState = 1; sockets[0].onopen(); });
    assert.deepEqual(sockets[0].sent, [{ type: "authenticate", token: "synthetic-feed-token" }]);
    await act(async () => { sockets[0].onmessage({ data: JSON.stringify(page(11)) }); });
    assert.deepEqual(sockets[0].sent.at(-1), { type: "ack", sequence: 11 });
    await act(async () => { sockets[0].onmessage({ data: JSON.stringify(page(12, [{ sequence: 12, type: "message", entity_id: "m", revision: 12 }])) }); });
    assert.equal(sockets[0].sent.length, 2, "ACK waits on hydrated snapshot");
    const appliedBeforeSwitch = snapshots.length;
    await act(async () => root.update(React.createElement(Probe, { room: "other-room" })));
    await respond(3, empty(12));
    assert.equal(snapshots.length, appliedBeforeSwitch, "Late previous-room snapshot is discarded");
    assert.equal(sockets[0].sent.length, 2, "No ACK on an abandoned socket");
  } finally {
    await act(async () => root?.unmount());
    globalThis.fetch = originalFetch;
    globalThis.WebSocket = originalSocket;
    if (originalState) Object.defineProperty(AppState, "currentState", originalState);
    else delete AppState.currentState;
    AppState.addEventListener = originalListener;
    console.error = originalError;
    CAU_HINH.focus = false;
    datTokenPhien(null);
  }
});

/* Slice 12: the room's `ai` frames on the same socket (design 02 §5.4). */
test("AI frames: opt-in on authenticate, routed before the busy lane, never ACKed, fresh room on every socket", async () => {
  const originalFetch = globalThis.fetch;
  const originalSocket = globalThis.WebSocket;
  const originalState = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const originalListener = AppState.addEventListener;
  const originalError = console.error;
  const pending = [];
  const sockets = [];
  const frames = [];
  let fresh = 0;
  let root;
  console.error = (...args) => { if (!String(args[0]).includes("react-test-renderer is deprecated")) originalError(...args); };
  class Socket {
    static OPEN = 1;
    readyState = 0;
    sent = [];
    closed = false;
    constructor(url) { this.url = url; sockets.push(this); }
    send(value) { this.sent.push(JSON.parse(value)); }
    close() { this.closed = true; this.readyState = 3; this.onclose?.(); }
  }
  globalThis.WebSocket = Socket;
  globalThis.fetch = (url, init) => new Promise((resolve) => pending.push({ url, init, resolve }));
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => "active" });
  AppState.addEventListener = () => ({ remove() {} });
  CAU_HINH.focus = true;
  datTokenPhien("synthetic-feed-token");
  const ai = { khung: (k) => frames.push(k), moi: () => { fresh += 1; } };
  function Probe() { useChatChanges(context, actor, () => {}, ai); return null; }
  const respond = async (index, data) => act(async () => pending[index].resolve({ ok: true, status: 200, json: async () => data, text: async () => JSON.stringify(data) }));
  const inv = "0b8f1c9e-aaaa-4bbb-8ccc-00000000aa01";
  const frame = (id, e, d) => JSON.stringify({ type: "ai", inv, tin: "0b8f1c9e-aaaa-4bbb-8ccc-00000000bb01", so_tin: 3, id, e, d });
  try {
    await act(async () => { root = TestRenderer.create(React.createElement(Probe)); });
    await respond(0, empty());
    await respond(1, page(10));
    const s = sockets[0];
    await act(async () => { s.readyState = 1; s.onopen(); });
    assert.deepEqual(s.sent, [{ type: "authenticate", token: "synthetic-feed-token", ai: true }], "the socket asks for the room's frames");
    assert.equal(fresh, 1, "a new socket starts the room's answers over");
    await act(async () => { s.onmessage({ data: JSON.stringify(page(10)) }); });
    assert.deepEqual(s.sent.at(-1), { type: "ack", sequence: 10 });
    // A page that hydrates holds the apply lane (busy)...
    await act(async () => { s.onmessage({ data: JSON.stringify(page(11, [{ sequence: 11, type: "message", entity_id: "m", revision: 11 }])) }); });
    const hydrate = pending.length - 1;
    // ...and an AI frame arriving meanwhile goes straight to the room, does not
    // close the socket (as a second page would), and is not acknowledged.
    await act(async () => { s.onmessage({ data: frame("1-0", "trang_thai", { cau: "dang_doc" }) }); });
    await act(async () => { s.onmessage({ data: frame("1-1", "delta", { p: 0, text: "Tối nay " }) }); });
    assert.equal(s.closed, false, "an AI frame during hydration must not close the socket");
    assert.deepEqual(frames.map((k) => `${k.e}:${k.id}`), ["trang_thai:1-0", "delta:1-1"]);
    assert.equal(frames[1].tin, "0b8f1c9e-aaaa-4bbb-8ccc-00000000bb01");
    assert.equal(frames[1].soTin, 3);
    // Outside the room's vocabulary: dropped, and still no close.
    await act(async () => { s.onmessage({ data: frame("1-2", "thu_hoi", {}) }); });
    await act(async () => { s.onmessage({ data: JSON.stringify({ type: "ai", inv: "bad", id: "1-3", e: "delta", d: {} }) }); });
    assert.equal(frames.length, 2);
    assert.equal(s.closed, false);
    assert.equal(s.sent.length, 2, "no ACK for any AI frame");
    await respond(hydrate, empty(11));
    assert.deepEqual(s.sent.at(-1), { type: "ack", sequence: 11 }, "the page lane carries on after the frames");
    // The socket drops; the next one starts the room over again.
    await act(async () => { s.close(); });
    assert.equal(sockets.length, 1);
  } finally {
    await act(async () => root?.unmount());
    globalThis.fetch = originalFetch;
    globalThis.WebSocket = originalSocket;
    if (originalState) Object.defineProperty(AppState, "currentState", originalState);
    else delete AppState.currentState;
    AppState.addEventListener = originalListener;
    console.error = originalError;
    CAU_HINH.focus = false;
    datTokenPhien(null);
  }
});
