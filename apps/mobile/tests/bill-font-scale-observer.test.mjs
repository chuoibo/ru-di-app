/**
 * Exercise the production observer hook and Expo wrapper with real React effects.
 * Only the React Native platform and Expo native-module boundary are replaced.
 * These cases do not mount M or prove Android configuration delivery, focus or IME.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import React, { act, useEffect } from "react";
import TestRenderer from "react-test-renderer";
import ts from "typescript";

const hookUrl = new URL("../src/rudi/chia-bill/useBillFontScale.ts", import.meta.url);
const wrapperUrl = new URL("../modules/rudi-font-scale/index.ts", import.meta.url);
const productionUrls = new Set([hookUrl.href, wrapperUrl.href]);
const caseParameter = "bill-font-scale-test";
const registryKey = Symbol.for("rudi.bill-font-scale-observer.test.boundaries");
const boundaries = new Map();
const consumed = new Map();
let nextCase = 0;
globalThis[registryKey] = boundaries;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

function identify(url) {
  if (!url) return null;
  const parsed = new URL(url);
  const id = parsed.searchParams.get(caseParameter);
  parsed.search = "";
  return id !== null && productionUrls.has(parsed.href) ? { id, base: parsed.href } : null;
}

function forCase(url, id) {
  const versioned = new URL(url);
  versioned.searchParams.set(caseParameter, id);
  return versioned.href;
}

const loader = registerHooks({
  resolve(specifier, context, nextResolve) {
    const parent = identify(context.parentURL);
    if (parent?.base === hookUrl.href && specifier === "../../../modules/rudi-font-scale") {
      return { url: forCase(wrapperUrl, parent.id), shortCircuit: true };
    }
    if (parent?.base === wrapperUrl.href && specifier === "expo-modules-core") {
      return { url: `rudi-font-scale-observer-test:expo/${parent.id}`, shortCircuit: true };
    }
    if (parent?.base === wrapperUrl.href && specifier === "react-native") {
      return { url: `rudi-font-scale-observer-test:rn/${parent.id}`, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    const native = /^rudi-font-scale-observer-test:(expo|rn)\/(\d+)$/.exec(url);
    if (native) {
      assert.ok(boundaries.has(native[2]), "native boundary belongs to a live case");
      const binding = `globalThis[Symbol.for(${JSON.stringify(Symbol.keyFor(registryKey))})].get(${JSON.stringify(native[2])})`;
      const source = native[1] === "expo"
        ? `export const requireOptionalNativeModule = ${binding}.requireOptionalNativeModule;`
        : `export const Platform = ${binding}.Platform;`;
      return { format: "module", source, shortCircuit: true };
    }
    const production = identify(url);
    if (production) {
      consumed.get(production.id).add(production.base);
      const result = ts.transpileModule(readFileSync(fileURLToPath(production.base), "utf8"), {
        fileName: fileURLToPath(production.base),
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        reportDiagnostics: true,
      });
      assert.deepEqual(
        (result.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error),
        [],
        "production observer source must transpile without syntax diagnostics",
      );
      return { format: "module", source: result.outputText, shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});

const originalConsoleError = console.error;
console.error = (...args) => {
  if (String(args[0]).includes("react-test-renderer is deprecated")) return;
  originalConsoleError(...args);
};
after(() => {
  loader.deregister();
  console.error = originalConsoleError;
  delete globalThis[registryKey];
});

function nativeBoundary(options = {}) {
  const state = {
    platform: "android",
    mode: "available",
    current: { status: "available", fontScale: 1 },
    readThrows: false,
    subscriptionMode: "available",
    onSubscribe: null,
    onRemove: null,
    ...options,
    reads: 0,
    removes: 0,
    lookups: [],
    captured: [],
    listeners: new Set(),
    trace: [],
  };
  const native = {
    getFontScale() {
      state.reads += 1;
      state.trace.push("read");
      if (state.readThrows) throw new Error("synthetic read failure");
      return state.current;
    },
    addListener(eventName, callback) {
      state.captured.push({ eventName, callback });
      state.trace.push("subscribe");
      state.onSubscribe?.(callback, state);
      if (state.subscriptionMode === "throws") throw new Error("synthetic subscription failure");
      if (state.subscriptionMode === "invalid") return {};
      state.listeners.add(callback);
      let removed = false;
      return {
        remove() {
          if (removed) return;
          removed = true;
          state.removes += 1;
          state.trace.push("remove");
          state.listeners.delete(callback);
          state.onRemove?.(callback);
        },
      };
    },
  };
  return {
    state,
    Platform: { OS: state.platform },
    requireOptionalNativeModule(name) {
      state.lookups.push(name);
      state.trace.push("lookup");
      if (name !== "RudiFontScale" || state.mode === "missing") return null;
      if (state.mode === "lookup-throws") throw new Error("synthetic lookup failure");
      return native;
    },
  };
}

async function withHook(options, run) {
  const id = String(++nextCase);
  const boundary = nativeBoundary(options);
  boundaries.set(id, boundary);
  consumed.set(id, new Set());
  let root;
  let current;
  let tick = 0;
  const commits = [];
  const harness = {
    native: boundary.state,
    commits,
    get current() { return current; },
    async notify() {
      const queued = [...boundary.state.listeners];
      await act(async () => { for (const callback of queued) callback(); });
    },
    async deliver(callback) { await act(async () => { callback(); }); },
    async rerender() {
      tick += 1;
      await act(async () => { root.update(React.createElement(Probe, { tick })); });
    },
    async unmount() {
      if (!root) return;
      const old = root;
      root = null;
      await act(async () => { old.unmount(); });
    },
  };
  let useBillFontScale;
  function Probe() {
    const snapshot = useBillFontScale();
    current = snapshot;
    useEffect(() => { commits.push(snapshot); }, [snapshot]);
    return null;
  }
  try {
    ({ useBillFontScale } = await import(forCase(hookUrl, id)));
    assert.deepEqual([...consumed.get(id)].sort(), [...productionUrls].sort(), "both production modules were loaded for this case");
    await act(async () => { root = TestRenderer.create(React.createElement(Probe, { tick })); });
    await run(harness);
  } finally {
    await harness.unmount();
    assert.equal(boundary.state.listeners.size, 0, "native listener leaked after unmount");
    boundaries.delete(id);
    consumed.delete(id);
  }
}

function assertUnavailable(actual, reason) {
  assert.deepEqual(actual, { status: "unavailable", reason });
  assert.equal(Object.hasOwn(actual, "fontScale"), false, "unavailability must not invent a fallback scale");
}

test("H01: a fresh read closes a change between initial render and listener registration", async () => {
  await withHook({ onSubscribe: (_callback, native) => { native.current = { status: "available", fontScale: 2 }; } }, async h => {
    assert.deepEqual(h.current, { status: "available", fontScale: 2 }, "the subscription gap must not retain the initial scale");
    const order = h.native.trace.filter(operation => operation === "read" || operation === "subscribe");
    const subscription = order.indexOf("subscribe");
    assert.equal(order[0], "read", "initial render reads before subscribing");
    assert.ok(subscription > 0, "a native subscription follows the initial read");
    assert.ok(order.slice(subscription + 1).includes("read"), "a fresh read follows subscription");
    assert.ok(h.native.lookups.length > 0);
    assert.ok(h.native.lookups.every(name => name === "RudiFontScale"));
    assert.equal(h.native.captured[0].eventName, "onFontScaleChanged");
    assert.equal(h.native.listeners.size, 1);
  });
});

test("H02: queued notifications reread current resources rather than a prior change", async () => {
  await withHook({}, async h => {
    assert.deepEqual(h.current, { status: "available", fontScale: 1 });
    h.native.current = { status: "available", fontScale: 1.3 };
    const queued = h.native.captured[0].callback;
    h.native.current = { status: "available", fontScale: 2 };
    const reads = h.native.reads;
    await h.deliver(queued);
    assert.deepEqual(h.current, { status: "available", fontScale: 2 });
    assert.equal(h.native.reads, reads + 1, "a notification must perform a new resource read");
    h.native.current = { status: "available", fontScale: 1 };
    await h.notify();
    assert.deepEqual(h.current, { status: "available", fontScale: 1 });
  });
});

test("H03: equal snapshots preserve committed effect identity without resubscribing", async () => {
  await withHook({}, async h => {
    const initial = h.current;
    const firstCommits = h.commits.length;
    const reads = h.native.reads;
    await h.notify();
    assert.equal(h.current, initial);
    assert.equal(h.commits.length, firstCommits, "an equal available snapshot retriggered a dependent effect");
    assert.equal(h.native.reads, reads + 1, "deduplication must still read the current native value");
    await h.rerender();
    assert.equal(h.current, initial);
    assert.equal(h.native.captured.length, 1, "an unrelated parent render replaced the native subscription");
    assert.equal(h.native.removes, 0);

    h.native.current = { status: "available", fontScale: 2 };
    await h.notify();
    assert.notEqual(h.current, initial);
    assert.equal(h.commits.length, firstCommits + 1);
    h.native.current = { status: "unavailable", reason: "context-unavailable" };
    await h.notify();
    assertUnavailable(h.current, "context-unavailable");
    const unavailable = h.current;
    const unavailableCommits = h.commits.length;
    await h.notify();
    assert.equal(h.current, unavailable);
    assert.equal(h.commits.length, unavailableCommits, "an equal unavailable snapshot retriggered a dependent effect");
    h.native.current = { status: "unavailable", reason: "module-inactive" };
    await h.notify();
    assertUnavailable(h.current, "module-inactive");
    assert.notEqual(h.current, unavailable, "different unavailability reasons are distinct snapshots");
  });
});

test("H04: unavailable native boundaries expose a typed failure without a fabricated scale", async t => {
  const cases = [
    ["unsupported platform", { platform: "ios" }, "unsupported-platform", 0],
    ["missing module", { mode: "missing" }, "module-unavailable", 1],
    ["lookup throws", { mode: "lookup-throws" }, "module-unavailable", 1],
    ["read throws", { readThrows: true }, "read-failed", 1],
    ["subscription throws", { subscriptionMode: "throws" }, "subscription-failed", 1],
    ["invalid subscription handle", { subscriptionMode: "invalid" }, "subscription-failed", 1],
  ];
  for (const [name, options, reason, lookups] of cases) {
    await t.test(name, async () => {
      await withHook(options, async h => {
        assertUnavailable(h.current, reason);
        assert.equal(h.native.lookups.length, lookups);
        if (lookups === 0 || options.mode) {
          assert.equal(h.native.reads, 0);
          assert.equal(h.native.captured.length, 0);
        }
        if (options.subscriptionMode) {
          assert.equal(h.native.reads, 1, "failed observation must override the successful initial read");
          assert.equal(h.native.listeners.size, 0);
          const captured = h.native.captured[0].callback;
          await h.unmount();
          const reads = h.native.reads;
          await h.deliver(captured);
          assert.equal(h.native.reads, reads, "failed-observation cleanup left a live callback");
        }
      });
    });
  }
});

test("H04 values: malformed reads are rejected and a later valid notification can recover", async t => {
  const invalid = [null, 2, {}, { status: "other" },
    ...[0, -1, NaN, Infinity, "2"].map(fontScale => ({ status: "available", fontScale })),
    { status: "unavailable", reason: "unexpected-reason" }];
  for (let index = 0; index < invalid.length; index += 1) {
    await t.test(`invalid native value ${index + 1}`, async () => {
      await withHook({ current: invalid[index] }, async h => {
        assertUnavailable(h.current, "invalid-native-value");
        h.native.current = { status: "available", fontScale: 1.3 };
        await h.notify();
        assert.deepEqual(h.current, { status: "available", fontScale: 1.3 });
      });
    });
  }
  for (const reason of ["module-inactive", "context-unavailable", "observer-unavailable", "invalid-native-value", "read-failed"]) {
    await t.test(`native reason ${reason}`, async () => {
      await withHook({ current: { status: "unavailable", reason } }, async h => {
        assertUnavailable(h.current, reason);
      });
    });
  }
});

test("H05: cleanup removes once and blocks both reentrant and already queued notifications", async () => {
  await withHook({}, async h => {
    const queued = h.native.captured[0].callback;
    const before = h.current;
    const commits = h.commits.length;
    const reads = h.native.reads;
    let reentrant = 0;
    h.native.current = { status: "available", fontScale: 2 };
    h.native.onRemove = callback => {
      reentrant += 1;
      assert.equal(h.native.listeners.has(callback), false);
      callback();
    };
    await h.unmount();
    assert.equal(reentrant, 1, "the fixture must deliver the reentrant cleanup notification");
    assert.equal(h.native.removes, 1);
    assert.equal(h.native.listeners.size, 0);
    assert.equal(h.native.reads, reads, "cleanup read resources through a reentrant notification");
    await h.deliver(queued);
    assert.equal(h.native.reads, reads, "a queued notification read resources after unmount");
    assert.equal(h.current, before);
    assert.equal(h.commits.length, commits);
    await h.unmount();
    assert.equal(h.native.removes, 1);
  });
});
