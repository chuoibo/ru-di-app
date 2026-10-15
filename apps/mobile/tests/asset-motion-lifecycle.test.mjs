/**
 * Run the production hooks, live OS-setting subscription and event registry
 * with real React effects. Only device APIs are replaced: the platform clock
 * is hand-driven, because bare Node has no Reanimated UI thread or navigation
 * container. This proves effect ordering, never native animation delivery.
 */
import assert from "node:assert/strict";
import test from "node:test";
import * as nodeModule from "node:module";
import React, { act } from "react";
import TestRenderer from "react-test-renderer";
import { AccessibilityInfo, AppState } from "react-native-web";
import { CLOCK, DEVICE, NavigationContext, advance, finish, isAnimating, resetClock } from "./stubs/motion-platform.mjs";

const platformUrl = new URL("./stubs/motion-platform.mjs", import.meta.url).href;
const redirectedSpecifiers = ["react-native-reanimated", "expo-haptics", "expo-router/build/react-navigation/core/NavigationContext"];
if (typeof nodeModule.registerHooks === "function") {
  nodeModule.registerHooks({
    resolve(specifier, context, nextResolve) {
      if (redirectedSpecifiers.includes(specifier)) {
        return { url: platformUrl, shortCircuit: true };
      }
      return nextResolve(specifier, context);
    },
  });
} else {
  nodeModule.register("./stubs/motion-platform.mjs", import.meta.url);
}
const { useConfirmedProgress } = await import("../dist-test/rudi/asset-pack/react-native/useConfirmedProgress.js");
const { useKhoanhKhac } = await import("../dist-test/rudi/ui/useKhoanhKhac.js");
const { daDienRoi, quenKhoanhKhac } = await import("../dist-test/rudi/khoanh-khac.js");
const { MOTION_MS } = await import("../dist-test/rudi/motion.js");
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const originalConsoleError = console.error;
console.error = (...args) => {
  if (String(args[0]).includes("react-test-renderer is deprecated")) return;
  originalConsoleError(...args);
};

async function withHook(mode, initial, run, { reduced = false, foreground = "active", routeFocused = true } = {}) {
  resetClock();
  quenKhoanhKhac();
  DEVICE.initialReduced = reduced;
  const originalAppState = Object.getOwnPropertyDescriptor(AppState, "currentState");
  const originalAppListener = AppState.addEventListener;
  const originalReduceRead = AccessibilityInfo.isReduceMotionEnabled;
  const originalReduceListener = AccessibilityInfo.addEventListener;
  const appListeners = new Set();
  const reduceListeners = new Set();
  const navigationListeners = new Map([["focus", new Set()], ["blur", new Set()]]);
  Object.defineProperty(AppState, "currentState", { configurable: true, get: () => foreground });
  AppState.addEventListener = (_name, callback) => { appListeners.add(callback); return { remove: () => appListeners.delete(callback) }; };
  AccessibilityInfo.isReduceMotionEnabled = async () => reduced;
  AccessibilityInfo.addEventListener = (_name, callback) => { reduceListeners.add(callback); return { remove: () => reduceListeners.delete(callback) }; };
  const navigation = {
    isFocused: () => routeFocused,
    addListener(name, callback) { navigationListeners.get(name).add(callback); return () => navigationListeners.get(name).delete(callback); },
  };
  let props = { ...initial };
  let root;
  let current;
  function Probe(next) {
    current = mode === "progress"
      ? useConfirmedProgress(next.eventKey ?? null, next.timing, next.kind ?? "envelope", next.focused ?? true, next.withoutEvent ?? 0)
      : useKhoanhKhac(next.id ?? "M7", next.eventKey ?? null, { focused: next.focused, hopLe: next.hopLe, coLoi: next.coLoi });
    return null;
  }
  const element = () => React.createElement(NavigationContext.Provider, { value: navigation }, React.createElement(Probe, props));
  const harness = {
    get current() { return current; },
    get shared() { return mode === "progress" ? current : current.t; },
    async mount() { await act(async () => { root = TestRenderer.create(element()); }); },
    async update(next) { props = { ...props, ...next }; await act(async () => root.update(element())); },
    async unmount() { if (root) { const old = root; root = null; await act(async () => old.unmount()); } },
    async reduce(value) { reduced = value; await act(async () => { for (const listener of reduceListeners) listener(value); }); },
    async app(state) { foreground = state; await act(async () => { for (const listener of appListeners) listener(state); }); },
    async route(focused) {
      routeFocused = focused;
      await act(async () => { for (const listener of navigationListeners.get(focused ? "focus" : "blur")) listener(); });
    },
    async advance(value) { await act(async () => advance(harness.shared, value)); },
    async finish() { await act(async () => finish(harness.shared)); },
    async skip() { await act(async () => current.boQua()); },
  };
  try {
    await harness.mount();
    await run(harness);
  } finally {
    await harness.unmount();
    assert.equal(appListeners.size, 0, "AppState listener leaked after unmount");
    assert.equal(reduceListeners.size, 0, "OS reduce-motion listener leaked after unmount");
    assert.ok([...navigationListeners.values()].every(listeners => listeners.size === 0), "route listener leaked after unmount");
    if (originalAppState) Object.defineProperty(AppState, "currentState", originalAppState);
    AppState.addEventListener = originalAppListener;
    AccessibilityInfo.isReduceMotionEnabled = originalReduceRead;
    AccessibilityInfo.addEventListener = originalReduceListener;
  }
}

test("an envelope stays open without a confirmed event, then closes once after a response", async () => {
  await withHook("progress", { eventKey: null }, async h => {
    assert.equal(h.shared.value, 0);
    assert.equal(CLOCK.starts.length, 0, "mount invented a successful response");
    await h.update({ eventKey: "invitation-synthetic-1" });
    assert.equal(CLOCK.starts.length, 1);
    assert.equal(CLOCK.starts[0].config.duration, MOTION_MS.standard);
    assert.equal(CLOCK.starts[0].config.reduceMotion, "never");
    assert.equal(h.shared.value, 0);
    assert.equal(daDienRoi("asset:envelope:invitation-synthetic-1"), true);
    await h.advance(.4);
    assert.equal(h.shared.value, .4);
    await h.finish();
    assert.equal(h.shared.value, 1);
    await h.update({ eventKey: "invitation-synthetic-1" });
    assert.equal(CLOCK.starts.length, 1, "an unrelated render replayed success");
  });
});

test("a remounted confirmed envelope is already sealed; a different response can close anew", async () => {
  await withHook("progress", { eventKey: "invitation-synthetic-2" }, async h => {
    const oldClock = h.shared;
    await h.advance(.2);
    await h.unmount();
    assert.equal(isAnimating(oldClock), false);
    assert.equal(oldClock.value, 1, "unmount left a half-folded envelope");
    await h.mount();
    assert.equal(h.shared.value, 1);
    assert.equal(CLOCK.starts.length, 1, "back navigation replayed an acknowledged invitation");
    await h.update({ eventKey: "invitation-synthetic-3" });
    assert.equal(CLOCK.starts.length, 2);
    assert.equal(h.shared.value, 0);
  });
});

test("asset event namespaces do not suppress distinct objects with the same response id", async () => {
  await withHook("progress", { eventKey: "response-synthetic", kind: "envelope" }, async h => {
    await h.finish();
    await h.update({ kind: "arrival" });
    assert.equal(CLOCK.starts.length, 2);
    assert.equal(daDienRoi("asset:arrival:response-synthetic"), true);
    assert.equal(daDienRoi("asset:envelope:response-synthetic"), true);
    await h.update({ eventKey: null });
    assert.equal(h.shared.value, 0, "starting another draft must show the open object");
    assert.equal(isAnimating(h.shared), false);
  });
});

const interruptions = {
  "live OS reduced motion": async h => { await h.reduce(true); },
  "route blur": async h => { await h.route(false); },
  "app background": async h => { await h.app("background"); },
  "explicit hidden surface": async h => { await h.update({ focused: false }); },
};

for (const [name, interrupt] of Object.entries(interruptions)) {
  test(`confirmed object settles on ${name} and never replays on return`, async () => {
    await withHook("progress", { eventKey: `progress-${name}` }, async h => {
      await h.advance(.3);
      await interrupt(h);
      assert.equal(isAnimating(h.shared), false);
      assert.equal(h.shared.value, 1);
      assert.equal(CLOCK.cancels.length, 1);
      await h.reduce(false);
      await h.route(true);
      await h.app("active");
      await h.update({ focused: true });
      assert.equal(CLOCK.starts.length, 1);
      assert.equal(h.shared.value, 1);
    });
  });

  test(`Nếp cancels on ${name}, settles without a false haptic, and stays still on return`, async () => {
    await withHook("moment", { eventKey: `moment-${name}`, id: "M7" }, async h => {
      assert.equal(h.current.cach, "dien");
      assert.equal(h.current.dangChay, true);
      const still = h.current.tm.khoa[h.current.tm.khungTinh].t;
      await interrupt(h);
      assert.equal(isAnimating(h.shared), false);
      assert.equal(h.current.dangChay, false);
      assert.equal(h.current.cach, "tinh");
      assert.equal(h.shared.value, still);
      assert.deepEqual(CLOCK.haptics, [], "a cancellation jump rang a completion haptic");
      await h.reduce(false);
      await h.route(true);
      await h.app("active");
      await h.update({ focused: true });
      assert.equal(CLOCK.starts.length, 1);
      assert.equal(h.current.cach, "tinh");
      assert.equal(h.current.dangChay, false);
    });
  });
}

for (const mode of ["progress", "moment"]) {
  test(`${mode}: reduced motion at confirmation consumes the event without animation or haptics`, async () => {
    await withHook(mode, { eventKey: `initial-reduced-${mode}`, id: "M6" }, async h => {
      assert.equal(CLOCK.starts.length, 0);
      assert.equal(isAnimating(h.shared), false);
      if (mode === "progress") assert.equal(h.shared.value, 1);
      else { assert.equal(h.current.cach, "tinh"); assert.equal(h.current.dangChay, false); }
      await h.reduce(false);
      assert.equal(CLOCK.starts.length, 0, "turning motion back on replayed an old event");
      assert.deepEqual(CLOCK.haptics, []);
    }, { reduced: true });
  });

  test(`${mode}: an event received offscreen stays final when its route becomes active`, async () => {
    await withHook(mode, { eventKey: `offscreen-${mode}`, id: "M6" }, async h => {
      assert.equal(CLOCK.starts.length, 0);
      await h.app("active");
      await h.route(true);
      assert.equal(CLOCK.starts.length, 0);
      assert.equal(isAnimating(h.shared), false);
      assert.deepEqual(CLOCK.haptics, []);
    }, { foreground: "background", routeFocused: false });
  });
}

test("Nếp does not appear or consume a failed or unconfirmed event; real confirmation can still play", async () => {
  await withHook("moment", { eventKey: null, id: "M6" }, async h => {
    assert.equal(h.current.tm, null);
    assert.equal(CLOCK.starts.length, 0);
    await h.update({ eventKey: "notebook-synthetic", hopLe: false });
    assert.equal(h.current.tm, null);
    assert.equal(h.current.cach, "khong");
    assert.equal(daDienRoi("M6:notebook-synthetic"), false);
    await h.update({ hopLe: true, coLoi: true });
    assert.equal(h.current.tm, null);
    assert.equal(daDienRoi("M6:notebook-synthetic"), false);
    await h.update({ coLoi: false });
    assert.equal(h.current.cach, "dien");
    assert.equal(CLOCK.starts.length, 1);
    assert.equal(daDienRoi("M6:notebook-synthetic"), true);
  });
});

test("Nếp skip and remount show the canonical still frame without a completion haptic", async () => {
  await withHook("moment", { eventKey: "skip-synthetic", id: "M6" }, async h => {
    const still = h.current.tm.khoa[h.current.tm.khungTinh].t;
    await h.skip();
    assert.equal(h.shared.value, still);
    assert.equal(h.current.dangChay, false);
    assert.equal(isAnimating(h.shared), false);
    assert.deepEqual(CLOCK.haptics, []);
    const oldClock = h.shared;
    await h.unmount();
    assert.equal(isAnimating(oldClock), false);
    await h.mount();
    assert.equal(h.current.cach, "tinh");
    assert.equal(h.shared.value, still);
    assert.equal(CLOCK.starts.length, 1);
    assert.deepEqual(CLOCK.haptics, []);
  });
});

test("an error arriving during a confirmed performance hides Nếp and cancels its clock without a false finish", async () => {
  await withHook("moment", { eventKey: "late-conflict-synthetic", id: "M6" }, async h => {
    assert.equal(h.current.dangChay, true);
    await h.update({ coLoi: true });
    assert.equal(h.current.tm, null, "a conflict retained the celebratory figure");
    assert.equal(h.current.cach, "khong");
    assert.equal(h.current.dangChay, false);
    assert.equal(isAnimating(h.shared), false);
    assert.deepEqual(CLOCK.haptics, []);
    await h.update({ coLoi: false });
    assert.equal(CLOCK.starts.length, 1, "clearing a conflict replayed the same confirmed event");
    assert.equal(h.current.cach, "tinh");
  });
});

test("Nếp's genuine clock crossing fires each authored haptic once, then ends its running state", async () => {
  await withHook("moment", { eventKey: "complete-synthetic", id: "M6" }, async h => {
    const performance = h.current.tm;
    assert.ok(performance.nhip.length > 0, "case must cross an actual authored beat");
    assert.equal(CLOCK.starts[0].config.duration, performance.ms);
    for (const beat of performance.nhip) await h.advance(beat.t);
    await h.finish();
    assert.deepEqual(CLOCK.haptics, performance.nhip.map(beat => beat.kieu === "cham" ? "impact" : beat.kieu === "xong" ? "success" : "selection"));
    assert.equal(h.current.dangChay, false);
    assert.equal(isAnimating(h.shared), false);
    const count = CLOCK.starts.length;
    await h.update({ eventKey: "complete-synthetic" });
    assert.equal(CLOCK.starts.length, count);
    await h.update({ eventKey: "second-synthetic" });
    assert.equal(CLOCK.starts.length, count + 1, "a new real event was wrongly suppressed");
  });
});
