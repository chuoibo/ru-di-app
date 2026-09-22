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
