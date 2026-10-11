// Compatibility adapter for old tests only. New behavior is tested by exact-host.cjs.
// The wrapper, metric resolver, native reader and observation hook remain actual source.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
function flatten(value) {
  if (value === null || typeof value !== 'object') return undefined;
  if (!Array.isArray(value)) return value;
  const output = {};
  for (const child of value) { const next = flatten(child); if (next) Object.assign(output, next); }
  return output;
}
function nativeMetricsBoundary({ dependency, React, native, repo, sourceRoot }) {
  const ts = dependency('typescript');
  const cache = new Map();
  native.Platform ??= { OS: 'android' };
  native.StyleSheet ??= { create: value => value };
  native.StyleSheet.flatten ??= flatten;
  function load(relative) {
    if (cache.has(relative)) return cache.get(relative).exports;
    const file = [sourceRoot, repo].flatMap(base => ['.ts', '.tsx', '/index.ts'].map(ext => path.join(base, relative + ext))).find(fs.existsSync);
    assert.ok(file, `Actual metric dependency ${relative}`);
    const output = ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true }, reportDiagnostics: true });
    assert.deepEqual((output.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error).map(item => ({ code: item.code, start: item.start, message: ts.flattenDiagnosticMessageText(item.messageText, '\n') })), [], `Syntax ${relative}`);
    const module = { exports: {} }; cache.set(relative, module);
    const req = specifier => {
      if (specifier === 'react') return React;
      if (specifier === 'react/jsx-runtime') return dependency(specifier);
      if (specifier === 'react-native') return native;
      if (specifier === 'expo-modules-core') return { requireOptionalNativeModule: () => null };
      assert.ok(specifier.startsWith('.'), `Explicit metric boundary ${specifier}`);
      return load(path.posix.normalize(path.posix.join(path.posix.dirname(relative), specifier)));
    };
    vm.runInThisContext(`(function(require,module,exports){${output.outputText}\n})`, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  return {
    '../ui/NativeFontMetrics': load('apps/mobile/src/rudi/ui/NativeFontMetrics'),
    '../chia-bill/useBillFontScale': load('apps/mobile/src/rudi/chia-bill/useBillFontScale'),
  };
}
module.exports = { flatten, nativeMetricsBoundary };
