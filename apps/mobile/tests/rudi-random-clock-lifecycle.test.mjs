/** Real React effects; device clock, AppState and HTTP completion are controlled, not native delivery. */
import assert from "node:assert/strict";
import test from "node:test";
import React, { act } from "react";
import TestRenderer from "react-test-renderer";
import { AppState } from "react-native-web";
import { CAU_HINH } from "./stubs/expo-router.mjs";
import { useRandomRoomClock } from "../dist-test/rudi/chat/e2ee/useRandomLive.js";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

test("room clock có tối đa ba probe, bỏ thế hệ background/phòng cũ và lấy mới khi resume", async (t) => {
  const originalState = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const originalListener = AppState.addEventListener;
  const originalError = console.error;
  const listeners = new Set();
  const pending = [];
  let state = "active", mono = 100, current, root;
  const sample = () => new Promise((resolve) => pending.push({ room: "A", resolve }));
  const otherSample = () => new Promise((resolve) => pending.push({ room: "B", resolve }));
  console.error = (...args) => { if (!String(args[0]).includes("react-test-renderer is deprecated")) originalError(...args); };
  // Keep React's real User Timing API; only the device-time seam is controlled.
  t.mock.method(globalThis.performance, "now", () => mono);
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => state });
  AppState.addEventListener = (_name, callback) => { listeners.add(callback); return { remove: () => listeners.delete(callback) }; };
  CAU_HINH.focus = true;
  function Probe({ reader }) { current = useRandomRoomClock(reader, true); return null; }
  const respond = async (index) => act(async () => {
    mono += 20;
    pending[index].resolve({ server_now: new Date(Date.parse("2026-10-14T10:00:00Z") + mono).toISOString() });
  });
  const app = async (value) => act(async () => { state = value; for (const listener of listeners) listener(value); });
  try {
    await act(async () => { root = TestRenderer.create(React.createElement(Probe, { reader: sample })); });
    assert.equal(current.status, "syncing");
    assert.equal(pending.length, 1);
    await respond(0);
    assert.equal(pending.length, 2);
    await app("background");
    assert.equal(current.anchor, null);
    assert.equal(current.status, "unavailable");
    await respond(1);
    assert.equal(pending.length, 2, "background completion cannot start another probe");
    assert.equal(current.anchor, null);
    await app("active");
    for (const index of [2, 3, 4]) await respond(index);
    assert.equal(pending.length, 5, "resume performs only its three bounded probes");
    assert.equal(current.status, "ready");
    assert.ok(current.anchor.uncertaintyMs <= 250);
    await app("background");
    await app("active");
    assert.equal(pending.length, 6);
    await act(async () => root.update(React.createElement(Probe, { reader: otherSample })));
    assert.equal(pending.at(-1).room, "B");
    assert.equal(current.anchor, null);
    await respond(5);
    assert.equal(current.anchor, null, "late previous-room sample cannot establish readiness");
    const count = pending.length;
    await act(async () => root.unmount());
    root = null;
    await respond(count - 1);
    assert.equal(pending.length, count, "unmount cancels further probes");
    assert.equal(listeners.size, 0);
  } finally {
    if (root) await act(async () => root.unmount());
    if (originalState) Object.defineProperty(AppState, "currentState", originalState);
    else delete AppState.currentState;
    AppState.addEventListener = originalListener;
    console.error = originalError;
    CAU_HINH.focus = false;
  }
});
