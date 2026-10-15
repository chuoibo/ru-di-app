/** Real React effects; device clock, AppState and HTTP completion are controlled, not native delivery. */
import assert from "node:assert/strict";
import test from "node:test";
import React, { act } from "react";
import TestRenderer from "react-test-renderer";
import { AppState } from "react-native-web";
import { CAU_HINH } from "./stubs/expo-router.mjs";
import { useRandomRoomClock } from "../dist-test/rudi/chat/e2ee/useRandomLive.js";
import { createRandomRound } from "../dist-test/rudi/chat/e2ee/random-v2.js";
import { createRandomLiveRound, randomLiveView, randomServerNow } from "../dist-test/rudi/chat/e2ee/random-live.js";

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

test("renewal giữ clock còn hạn trong cuộc đua, không kéo dài anchor lỗi và bỏ response sau blur", async (t) => {
  const originalState = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const originalListener = AppState.addEventListener;
  const originalError = console.error;
  const listeners = new Set(), pending = [], timers = new Map();
  const base = Date.parse("2026-10-14T10:00:00Z");
  const live = createRandomLiveRound(createRandomRound({ id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", request: "Ăn gì?", mode: "ducks",
    options: [{ label: "Phở" }, { label: "Bún" }], seed: "00000001a0000002b0000003c0000004" }));
  let state = "active", mono = 100, current, root, nextTimer = 0;
  const sample = () => new Promise((resolve, reject) => pending.push({ before: mono, resolve, reject }));
  console.error = (...args) => { if (!String(args[0]).includes("react-test-renderer is deprecated")) originalError(...args); };
  t.mock.method(globalThis.performance, "now", () => mono);
  t.mock.method(globalThis, "setTimeout", (callback, delay = 0, ...args) => {
    const id = ++nextTimer;
    timers.set(id, { at: mono + Number(delay), callback: () => callback(...args) });
    return id;
  });
  t.mock.method(globalThis, "clearTimeout", (id) => timers.delete(id));
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => state });
  AppState.addEventListener = (_name, callback) => { listeners.add(callback); return { remove: () => listeners.delete(callback) }; };
  CAU_HINH.focus = true;
  function Probe({ enabled = true }) { current = useRandomRoomClock(sample, enabled); return null; }
  const respond = async (index, fails = false) => act(async () => {
    mono += 20;
    if (fails) pending[index].reject(new Error("synthetic clock unavailable"));
    else pending[index].resolve({ server_now: new Date(base + (pending[index].before + mono) / 2).toISOString() });
  });
  const advanceTo = async (target) => {
    assert.ok(target >= mono);
    for (;;) {
      const next = [...timers.entries()].filter(([, timer]) => timer.at <= target).sort((a, b) => a[1].at - b[1].at)[0];
      if (!next) break;
      mono = next[1].at;
      timers.delete(next[0]);
      await act(async () => next[1].callback());
    }
    mono = target;
  };
  try {
    await act(async () => { root = TestRenderer.create(React.createElement(Probe)); });
    for (const index of [0, 1, 2]) await respond(index);
    const first = current.anchor;
    assert.equal(current.status, "ready");
    await advanceTo(mono + 20_000);
    assert.equal(pending.length, 4, "early renewal starts one of only three sequential probes");
    assert.equal(current.status, "ready");
    assert.equal(current.anchor, first, "an in-flight renewal preserves the usable anchor");
    const acceptedAt = new Date(base + 10_000).toISOString();
    assert.equal(randomLiveView(live, acceptedAt, current.anchor, mono).phase, "racing");
    for (const index of [3, 4, 5]) {
      await respond(index);
      assert.equal(current.status, "ready");
      assert.equal(randomLiveView(live, acceptedAt, current.anchor, mono).phase, "racing");
    }
    const renewed = current.anchor;
    assert.notEqual(renewed, first);
    assert.equal(randomServerNow(renewed, mono), base + mono, "renewal keeps the same shared elapsed time");
    await advanceTo(mono + 20_000);
    assert.equal(pending.length, 7);
    const nextRoundAt = new Date(base + 32_000).toISOString();
    assert.equal(randomLiveView(live, nextRoundAt, current.anchor, mono).phase, "racing");
    for (const index of [6, 7, 8]) await respond(index, true);
    assert.equal(pending.length, 9);
    assert.equal(current.status, "ready");
    assert.equal(current.anchor, renewed, "failed renewal retains only the original usable anchor");
    await advanceTo(renewed.monotonicMs + 30_001);
    assert.equal(current.status, "unavailable", "expiry is measured from the original sample, never renewal failure");
    assert.equal(current.anchor, null);
    assert.equal(randomLiveView(live, nextRoundAt, current.anchor, mono).reveal, false);
    await act(async () => { for (const listener of listeners) listener("active"); });
    assert.equal(pending.length, 10);
    // The focus stub reruns only on effect identity: disabling the reader also
    // triggers the exact focus cleanup while its configured route is blurred.
    CAU_HINH.focus = false;
    await act(async () => root.update(React.createElement(Probe, { enabled: false })));
    assert.equal(current.anchor, null);
    assert.equal(listeners.size, 0);
    assert.equal(timers.size, 0);
    await respond(9);
    assert.equal(pending.length, 10, "a blurred generation cannot continue its probes");
    assert.equal(current.anchor, null);
    CAU_HINH.focus = true;
    await act(async () => root.update(React.createElement(Probe)));
    for (const index of [10, 11, 12]) await respond(index);
    assert.equal(current.status, "ready");
    assert.notEqual(current.anchor, renewed, "focus return requires a fresh anchor");
    await act(async () => root.unmount());
    root = null;
    assert.equal(timers.size, 0);
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
