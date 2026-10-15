/**
 * Render the production metric wrappers, Stepper and ruled input with real React.
 * Only the React Native boundary is substituted. This does not emulate Android
 * text measurement, Fabric spans, configuration delivery, native focus or IME.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as nodeModule from "node:module";
import { dirname } from "node:path";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import React, { act, useCallback, useState } from "react";
import TestRenderer from "react-test-renderer";
import ts from "typescript";

const require = nodeModule.createRequire(import.meta.url);
const hasRegisterHooks = typeof nodeModule.registerHooks === "function";

// This proposed tracked test belongs in apps/mobile/tests/ after root review.
const urls = Object.fromEntries(Object.entries({
  wrapper: "../src/rudi/ui/NativeFontMetrics.tsx",
  helper: "../src/rudi/native-font-metrics.ts",
  stepper: "../src/rudi/ui/Stepper.tsx",
  input: "../src/rudi/ui/ONhapMuc.tsx",
  theme: "../src/rudi/theme.ts",
  appearance: "../src/rudi/giao-dien.ts",
  chatColors: "../src/rudi/mau-chat.ts",
  stageTokens: "../src/rudi/san-khau/token.ts",
  personColors: "../src/rudi/nguoi/muc-nguoi.ts",
  outline: "../src/rudi/ui/khong-vien-web.ts",
  tokens: "../../../packages/shared/tokens.json",
}).map(([name, path]) => [name, new URL(path, import.meta.url)]));
const productionUrls = new Set(Object.values(urls).map(url => url.href));
const parameter = "bill-native-metrics-wrapper-test";
const registryKey = Symbol.for("rudi.bill-native-metrics-wrapper.test.boundaries");
const boundaries = new Map();
const consumed = new Map();
const oldActEnvironment = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis[registryKey] = boundaries;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
let nextCase = 0;

function identify(url) {
  if (!url || !url.startsWith("file:")) return null;
  const parsed = new URL(url);
  const id = parsed.searchParams.get(parameter);
  parsed.search = "";
  return id !== null && productionUrls.has(parsed.href) ? { id, base: parsed.href } : null;
}
function forCase(url, id) {
  const versioned = new URL(url);
  versioned.searchParams.set(parameter, id);
  return versioned.href;
}
const loader = hasRegisterHooks ? nodeModule.registerHooks({
  resolve(specifier, context, nextResolve) {
    const parent = identify(context.parentURL);
    if (parent && specifier === "react-native") {
      return { url: `rudi-native-metrics-wrapper-test:rn/${parent.id}`, shortCircuit: true };
    }
    if (parent && specifier.startsWith(".")) {
      const relative = new URL(specifier, parent.base).href;
      const target = [relative, `${relative}.ts`, `${relative}.tsx`].find(url => productionUrls.has(url));
      assert.ok(target, `unhandled real production dependency: ${specifier}`);
      return { url: forCase(target, parent.id), shortCircuit: true };
    }
    if (parent) assert.ok(specifier === "react" || specifier === "react/jsx-runtime", `unexpected native dependency: ${specifier}`);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    const native = /^rudi-native-metrics-wrapper-test:rn\/(\d+)$/.exec(url);
    if (native) {
      assert.ok(boundaries.has(native[1]), "RN boundary belongs to a live case");
      const key = JSON.stringify(Symbol.keyFor(registryKey));
      const id = JSON.stringify(native[1]);
      return { format: "module", shortCircuit: true, source:
        `const b = globalThis[Symbol.for(${key})].get(${id});\n` +
        "export const Text=b.Text, TextInput=b.TextInput, View=b.View, StyleSheet=b.StyleSheet;\n" +
        "export const Platform=b.Platform, Appearance=b.Appearance, useWindowDimensions=b.useWindowDimensions;" };
    }
    const production = identify(url);
    if (!production) return nextLoad(url, context);
    consumed.get(production.id).add(production.base);
    const source = readFileSync(fileURLToPath(production.base), "utf8");
    if (production.base === urls.tokens.href) {
      // Load the actual JSON bytes; this is a module-format adapter, not token fixtures.
      return { format: "module", source: `export default ${source};`, shortCircuit: true };
    }
    const result = ts.transpileModule(source, {
      fileName: fileURLToPath(production.base),
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
      reportDiagnostics: true,
    });
    assert.deepEqual((result.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error), [], "production source must transpile without syntax diagnostics");
    return { format: "module", source: result.outputText, shortCircuit: true };
  },
}) : null;

function loadFallback(targetHref, id, cache) {
  if (cache.has(targetHref)) return cache.get(targetHref).exports;
  consumed.get(id).add(targetHref);
  const filePath = fileURLToPath(targetHref);
  const source = readFileSync(filePath, "utf8");
  if (targetHref === urls.tokens.href) {
    const parsed = JSON.parse(source);
    const mod = { exports: { default: parsed, ...parsed } };
    cache.set(targetHref, mod);
    return mod.exports;
  }
  const esmResult = ts.transpileModule(source, {
    fileName: filePath,
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
    reportDiagnostics: true,
  });
  assert.deepEqual((esmResult.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error), [], "production source must transpile without syntax diagnostics");
  const cjsResult = ts.transpileModule(source, {
    fileName: filePath,
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true, jsx: ts.JsxEmit.ReactJSX },
  });
  const mod = { exports: {} };
  cache.set(targetHref, mod);
  const b = boundaries.get(id);
  const customRequire = (specifier) => {
    if (specifier === "react-native") {
      return {
        Text: b.Text, TextInput: b.TextInput, View: b.View, StyleSheet: b.StyleSheet,
        Platform: b.Platform, Appearance: b.Appearance, useWindowDimensions: b.useWindowDimensions,
      };
    }
    if (specifier.startsWith(".")) {
      const relative = new URL(specifier, targetHref).href;
      const target = [relative, `${relative}.ts`, `${relative}.tsx`].find(url => productionUrls.has(url));
      assert.ok(target, `unhandled real production dependency: ${specifier}`);
      return loadFallback(target, id, cache);
    }
    assert.ok(specifier === "react" || specifier === "react/jsx-runtime", `unexpected native dependency: ${specifier}`);
    return require(specifier);
  };
  const fn = vm.runInThisContext(
    `(function(exports, require, module, __filename, __dirname) {\n${cjsResult.outputText}\n})`,
    { filename: filePath },
  );
  fn(mod.exports, customRequire, mod, filePath, dirname(filePath));
  return mod.exports;
}

const oldConsoleError = console.error;
console.error = (...args) => {
  if (String(args[0]).includes("react-test-renderer is deprecated")) return;
  oldConsoleError(...args);
};
after(() => {
  loader?.deregister();
  console.error = oldConsoleError;
  delete globalThis[registryKey];
  if (oldActEnvironment === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT;
  else globalThis.IS_REACT_ACT_ENVIRONMENT = oldActEnvironment;
});

// Match RN's final-write style semantics, including an explicit undefined value.
function flatten(style) {
  if (style === null || typeof style !== "object") return undefined;
  if (!Array.isArray(style)) return style;
  const result = {};
  for (const entry of style) {
    const value = flatten(entry);
    if (value) Object.assign(result, value);
  }
  return result;
}
function rnBoundary() {
  const host = name => React.forwardRef(function NativeHost(props, ref) {
    return React.createElement(name, { ...props, ref });
  });
  return {
    Text: host("RNText"), TextInput: host("RNTextInput"), View: host("RNView"),
    StyleSheet: { flatten, create: styles => styles, hairlineWidth: 0.5,
      absoluteFill: { position: "absolute", left: 0, right: 0, top: 0, bottom: 0 } },
    Platform: { OS: "android", select: choices => choices.android ?? choices.default },
    Appearance: { getColorScheme: () => "light" },
    useWindowDimensions: () => ({ width: 320, height: 640, scale: 2, fontScale: 2 }),
  };
}

// Independent, deliberately nonproportional data. These are not Android samples.
const fontMetrics = {
  density: 2,
  spToDp: [
    [-0.65, -1.1], [-0.3, -0.49], [-0.15, -0.24], [0.4, 0.62], [0.8, 1.2], [1.6, 2.3],
    [11, 18.5], [12, 20.25], [13, 21.5], [14, 23.25], [15, 25], [17, 28.75],
    [18, 30.25], [19, 31.75], [21, 34.25], [22, 36], [23, 37.5], [24, 39.25],
    [27, 43], [28, 44.25], [34, 52],
  ].map(([base, dp]) => ({ base, dp })),
};
const active = { status: "available", fontScale: 2, fontMetrics };
const absent = { status: "available", fontScale: 2 };
const unavailable = { status: "unavailable", reason: "module-unavailable" };
const OUTSIDE = Symbol("outside provider");

async function withCase(run) {
  const id = String(++nextCase);
  const boundary = rnBoundary();
  boundaries.set(id, boundary);
  consumed.set(id, new Set());
  let root;
  let nextHost = 0;
  let api, stepper, input;
  if (hasRegisterHooks) {
    [api, stepper, input] = await Promise.all([
      import(forCase(urls.wrapper, id)), import(forCase(urls.stepper, id)), import(forCase(urls.input, id)),
    ]);
  } else {
    const cache = new Map();
    api = loadFallback(urls.wrapper.href, id, cache);
    stepper = loadFallback(urls.stepper.href, id, cache);
    input = loadFallback(urls.input.href, id, cache);
  }
  const h = {
    api, boundary, Stepper: stepper.Stepper, ONhapMuc: input.ONhapMuc,
    get root() { return root; },
    async render(children, snapshot = OUTSIDE) {
      const element = snapshot === OUTSIDE ? children : React.createElement(api.NativeFontMetricsProvider, { snapshot }, children);
      await act(async () => {
        if (root) root.update(element);
        else root = TestRenderer.create(element, { createNodeMock: element => ({ kind: element.type, id: ++nextHost }) });
      });
    },
    host(kind, testID) {
      const matches = root.root.findAllByType(kind).filter(node => node.props.testID === testID);
      assert.equal(matches.length, 1, `one ${kind} host named ${testID}`);
      return matches[0];
    },
  };
  try {
    assert.deepEqual([...consumed.get(id)].sort(), [...productionUrls].sort(), "wrapper/helper/components and real dependency sources were loaded");
    await run(h);
  } finally {
    if (root) await act(async () => { root.unmount(); });
    boundaries.delete(id);
    consumed.delete(id);
  }
}
function withoutRef(props) {
  const { ref: _ref, ...rest } = props;
  return rest;
}

test("W01: outside, unavailable and absent-table snapshots preserve original Text/Input props", async () => {
  await withCase(async h => {
    const style = [{ fontSize: 17, lineHeight: 24 }, [null, { lineHeight: undefined, color: "fixture-ink" }]];
    const textProps = { testID: "plain", style, allowFontScaling: true, numberOfLines: 2, children: "Nội dung" };
    const inputProps = { testID: "plain-input", style, value: "17.", selection: { start: 2, end: 2 }, placeholder: "Nhập" };
    for (const snapshot of [OUTSIDE, absent, unavailable]) {
      await h.render(React.createElement("Block", null,
        React.createElement(h.api.Text, textProps), React.createElement(h.api.TextInput, inputProps)), snapshot);
      assert.deepEqual(withoutRef(h.host("RNText", "plain").props), textProps);
      assert.deepEqual(withoutRef(h.host("RNTextInput", "plain-input").props), inputProps);
      assert.equal(h.host("RNText", "plain").props.style, style);
      assert.equal(Object.hasOwn(flatten(style), "lineHeight"), true);
      assert.equal(flatten(style).lineHeight, undefined);
    }
  });
});

test("W02: final array styles use independent mappings and Text caps only fontSize", async () => {
  await withCase(async h => {
    const style = [{ fontSize: 13, lineHeight: 18, letterSpacing: 0.8 }, false,
      [{ fontSize: 21, lineHeight: 27, letterSpacing: -0.3, color: "fixture-ink" }]];
    await h.render(React.createElement(h.api.Text, { testID: "mapped", style, numberOfLines: 2 }, "Tiêu đề"), active);
    const node = h.host("RNText", "mapped");
    assert.equal(node.props.allowFontScaling, false);
    assert.deepEqual(flatten(node.props.style), { fontSize: 34.25, lineHeight: 43, letterSpacing: -0.49, color: "fixture-ink" });
    assert.equal(node.props.numberOfLines, 2);
    await h.render(React.createElement(h.api.Text, { testID: "mapped", style, maxFontSizeMultiplier: 1.25 }, "Tiêu đề"), active);
    assert.deepEqual(flatten(h.host("RNText", "mapped").props.style), { fontSize: 26.25, lineHeight: 43, letterSpacing: -0.49, color: "fixture-ink" });
  });
});

test("W03: actual Stepper accepts scalar-array progress children without falling back", async () => {
  await withCase(async h => {
    await h.render(React.createElement(h.Stepper, { steps: ["Ảnh", "Xem lại", "Gán món", "Kết quả", "Đã ghi"], current: 1 }), active);
    const progress = h.root.root.findAllByType("RNText").find(node => node.children.join("") === "Bước 2/5");
    assert.ok(progress, "production Stepper must retain the complete progress text");
    assert.equal(progress.props.allowFontScaling, false, "numeric JSX children must not force native fallback");
    assert.equal(flatten(progress.props.style).fontSize, 21.5);
    assert.equal(flatten(progress.props.style).lineHeight, 30.25);
  });
});

test("W04: nested originals inherit once, including multiplier override and root-only scaling", async () => {
  await withCase(async h => {
    await h.render(React.createElement(h.api.Text, { testID: "parent", style: { fontSize: 17, lineHeight: 24, letterSpacing: 0.8 }, maxFontSizeMultiplier: 1.25 },
      React.createElement(React.Fragment, null, "Trước ", React.createElement(h.api.Text,
        { testID: "child", style: { fontWeight: "700" }, maxFontSizeMultiplier: 0, allowFontScaling: false },
        React.createElement(h.api.Text, { testID: "grandchild", style: { fontSize: 13, lineHeight: undefined } }, "Sau")))), active);
    assert.equal(flatten(h.host("RNText", "parent").props.style).fontSize, 21.25);
    assert.deepEqual(flatten(h.host("RNText", "child").props.style), { fontWeight: "700", fontSize: 28.75, lineHeight: 39.25, letterSpacing: 1.2 });
    assert.deepEqual(flatten(h.host("RNText", "grandchild").props.style), { fontSize: 21.5, lineHeight: 39.25, letterSpacing: 1.2 });
    for (const node of h.root.root.findAllByType("RNText")) assert.equal(node.props.allowFontScaling, false);
  });
});

test("W05: opaque/raw/unknown descendants keep the whole paragraph on its original native path", async () => {
  await withCase(async h => {
    const parentStyle = { fontSize: 17, lineHeight: 24 };
    const childStyle = { fontSize: 13, lineHeight: 18 };
    function Opaque() { return React.createElement(h.api.Text, { testID: "descendant", style: childStyle }, "Ẩn"); }
    const children = [
      React.createElement(Opaque),
      React.createElement(h.boundary.Text, null, React.createElement(h.api.Text, { testID: "descendant", style: childStyle }, "Raw")),
      React.createElement(h.api.Text, { testID: "descendant", style: { fontSize: 16, lineHeight: 18 } }, "Không có mẫu"),
    ];
    for (const child of children) {
      await h.render(React.createElement(h.api.Text, { testID: "paragraph", style: parentStyle }, "Trước ", child), active);
      const parent = h.host("RNText", "paragraph");
      const descendant = h.host("RNText", "descendant");
      assert.equal(parent.props.style, parentStyle);
      assert.equal(Object.hasOwn(parent.props, "allowFontScaling"), false);
      assert.equal(Object.hasOwn(descendant.props, "allowFontScaling"), false);
      assert.ok(flatten(descendant.props.style).fontSize === 13 || flatten(descendant.props.style).fontSize === 16);
    }
  });
});

test("W06: snapshot activation/fallback retains React hosts, refs, raw value, selection and handlers", async () => {
  await withCase(async h => {
    const textRefs = [], inputRefs = [];
    const textRef = value => textRefs.push(value);
    const inputRef = value => inputRefs.push(value);
    function Draft() {
      const [value, setValue] = useState("17");
      const [selection, setSelection] = useState({ start: 1, end: 1 });
      const onChangeText = useCallback(next => setValue(next), []);
      const onSelectionChange = useCallback(event => setSelection(event.nativeEvent.selection), []);
      return React.createElement("Block", null,
        React.createElement(h.api.Text, { ref: textRef, testID: "title", style: { fontSize: 17, lineHeight: 24 } }, "Nháp"),
        React.createElement(h.api.TextInput, { ref: inputRef, testID: "draft", style: { fontSize: 17, lineHeight: 24 }, value, selection, onChangeText, onSelectionChange }));
    }
    await h.render(React.createElement(Draft), absent);
    const initialTextHost = h.host("RNText", "title");
    const initialInputHost = h.host("RNTextInput", "draft");
    assert.equal(textRefs.length, 1);
    assert.equal(inputRefs.length, 1);
    assert.ok(textRefs[0] && inputRefs[0]);
    const change = initialInputHost.props.onChangeText;
    const select = initialInputHost.props.onSelectionChange;
    await act(async () => { change("17."); select({ nativeEvent: { selection: { start: 2, end: 2 } } }); });
    for (const snapshot of [active, unavailable, absent, active]) {
      await h.render(React.createElement(Draft), snapshot);
      const current = h.host("RNTextInput", "draft");
      assert.equal(h.host("RNText", "title"), initialTextHost);
      assert.equal(current, initialInputHost);
      assert.equal(current.props.value, "17.");
      assert.deepEqual(current.props.selection, { start: 2, end: 2 });
      assert.equal(current.props.onChangeText, change);
      assert.equal(current.props.onSelectionChange, select);
      assert.equal(textRefs.length, 1, "Text ref detached during a metric transition");
      assert.equal(inputRefs.length, 1, "Input ref detached during a metric transition");
      assert.equal(current.props.allowFontScaling, snapshot === active ? false : undefined);
    }
  });
});

test("W07: actual ONhap uses final input metrics for placeholder and pixel-rounded multiline area", async () => {
  await withCase(async h => {
    const face = { fontSize: 17, lineHeight: 24, letterSpacing: 0.8, fontFamily: "FixtureFace", fontWeight: "700", fontStyle: "italic", fontVariant: ["tabular-nums"], textTransform: "uppercase" };
    await h.render(React.createElement(h.ONhapMuc, { testID: "field", style: [{ fontSize: 13 }, face], placeholder: "Ví dụ", value: "", fontScaleOverride: 2 }), active);
    const field = h.host("RNTextInput", "field");
    const hint = h.root.root.findAllByType("RNText").find(node => node.children.join("") === "Ví dụ");
    assert.ok(hint, "production ruled input renders its own placeholder");
    assert.equal(field.props.allowFontScaling, false);
    assert.equal(hint.props.allowFontScaling, false, "converted placeholder must not scale twice");
    for (const [key, expected] of Object.entries({ ...face, fontSize: 28.75, lineHeight: 39.25, letterSpacing: 1.2 })) {
      assert.deepEqual(flatten(field.props.style)[key], expected, `input ${key}`);
      assert.deepEqual(flatten(hint.props.style)[key], expected, `placeholder ${key}`);
    }
    await h.render(React.createElement(h.ONhapMuc, { testID: "field", style: face, multiline: true, numberOfLines: 2, value: "Hai dòng", fontScaleOverride: 2 }), active);
    const multiline = flatten(h.host("RNTextInput", "field").props.style);
    assert.equal(multiline.lineHeight, 39.25);
    assert.equal(multiline.minHeight, 79, "two lines reserve 2 × ceil(39.25 × 2) / 2 dp");
    assert.equal(multiline.maxHeight, 316, "eight-line ceiling uses the same pixel-rounded pitch");
  });
});

test("W08: capped, natural-height and unknown Input styles preserve the native path", async () => {
  await withCase(async h => {
    const cases = [
      { style: { fontSize: 17, lineHeight: 24 }, maxFontSizeMultiplier: 1.25 },
      { style: [{ fontSize: 17, lineHeight: 24 }, { lineHeight: undefined }] },
      { style: { fontSize: 16, lineHeight: 24 } },
    ];
    for (const props of cases) {
      await h.render(React.createElement(h.api.TextInput, { ...props, testID: "fallback", value: "Gõ" }), active);
      assert.deepEqual(withoutRef(h.host("RNTextInput", "fallback").props), { ...props, testID: "fallback", value: "Gõ" });
      assert.equal(h.host("RNTextInput", "fallback").props.style, props.style);
    }
  });
});
