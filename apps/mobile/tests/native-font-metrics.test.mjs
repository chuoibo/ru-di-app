/** Pure conversion contracts; no Android curve, host layout or paint proof. */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as nodeModule from "node:module";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

const source = readFileSync(new URL("../src/rudi/native-font-metrics.ts", import.meta.url), "utf8");
const { outputText, diagnostics } = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext }, reportDiagnostics: true,
});
assert.deepEqual(diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error), []);
let resolveTextMetrics;
if (typeof nodeModule.registerHooks === "function") {
  const productionUrl = "rudi-native-font-metrics:test";
  const loader = nodeModule.registerHooks({
    resolve(specifier, context, next) {
      return specifier === productionUrl ? { url: productionUrl, shortCircuit: true } : next(specifier, context);
    },
    load(url, context, next) {
      return url === productionUrl ? { format: "module", source: outputText, shortCircuit: true } : next(url, context);
    },
  });
  ({ resolveTextMetrics } = await import(productionUrl));
  loader.deregister();
} else {
  const cjs = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  });
  const mod = { exports: {} };
  vm.runInThisContext(`(function(exports, module) {\n${cjs.outputText}\n})`)(mod.exports, mod);
  ({ resolveTextMetrics } = mod.exports);
}
const metrics = { density: 1.5, spToDp: [
  { base: 14, dp: 22 }, { base: 18, dp: 27 }, { base: 24, dp: 32 }, { base: -0.3, dp: -0.5 },
] };
const style = { fontSize: 14, lineHeight: 18, letterSpacing: -0.3 };

test("uses each sampled SP value rather than multiplying by one scale", () => {
  assert.deepEqual(resolveTextMetrics(metrics, style, {}, "text").style,
    { fontSize: 22, lineHeight: 27, letterSpacing: -0.5 });
});

test("Text caps font size while TextInput caps line height and negative tracking too", () => {
  assert.deepEqual(resolveTextMetrics(metrics, style, { maxFontSizeMultiplier: 1.2 }, "text").style,
    { fontSize: 16.8, lineHeight: 27, letterSpacing: -0.5 });
  assert.deepEqual(resolveTextMetrics(metrics, style, { maxFontSizeMultiplier: 1.2 }, "input").style,
    { fontSize: 16.8, lineHeight: 18 * 1.2, letterSpacing: -0.5 });
});

test("unknown or invalid numeric styles reject the whole conversion", () => {
  for (const extra of [{ fontSize: 16 }, { lineHeight: 20 }, { letterSpacing: 0.2 },
    { fontSize: NaN }, { fontSize: 0 }, { lineHeight: Infinity }]) {
    assert.equal(resolveTextMetrics(metrics, { ...style, ...extra }, {}, "text"), null);
  }
  for (const maxFontSizeMultiplier of [-1, 0.5, NaN, Infinity]) {
    assert.equal(resolveTextMetrics(metrics, style, { maxFontSizeMultiplier }, "text"), null);
  }
});

test("disabled scaling preserves raw metrics, including an unsampled font size", () => {
  assert.deepEqual(resolveTextMetrics(metrics, { ...style, fontSize: 16 }, { allowFontScaling: false }, "text").style,
    { ...style, fontSize: 16 });
});

test("nested text inherits original SP once; explicit cap zero clears inherited cap", () => {
  const parent = resolveTextMetrics(metrics, style, { maxFontSizeMultiplier: 1.2 }, "text");
  const child = resolveTextMetrics(metrics, { fontSize: 24 }, { maxFontSizeMultiplier: 0 }, "text", parent.original);
  assert.deepEqual(child.style, { fontSize: 32, lineHeight: 27, letterSpacing: -0.5 });
  assert.equal(child.original.lineHeight, 18);
  const unscaled = resolveTextMetrics(metrics, style, { allowFontScaling: false }, "text");
  assert.deepEqual(resolveTextMetrics(metrics, {}, { allowFontScaling: true }, "text", unscaled.original).style, style);
});

test("natural line height and zero tracking remain natural and zero", () => {
  assert.deepEqual(resolveTextMetrics(metrics, { fontSize: 14, lineHeight: undefined, letterSpacing: 0 }, {}, "text").style,
    { fontSize: 22, lineHeight: undefined, letterSpacing: 0 });
});
