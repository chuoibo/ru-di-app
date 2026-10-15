// Actual seal source on the selected complete graph; native events are explicit inputs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { sourceRoot } = require('./repository.cjs');
assert.equal(process.env.RUDI_TEST_SOURCE_ROOT, undefined, 'no partial overlay or fallback source tree');
if (process.env.RUDI_TEST_REPO !== undefined) assert.equal(fs.realpathSync(process.env.RUDI_TEST_REPO), sourceRoot);
const nativeHost = import(pathToFileURL(path.join(sourceRoot, 'apps/mobile/tests/n-behavior-host.mjs')).href);
const BASES = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6, 11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34];
function snapshot(scale = 1, density = 1) {
  const actual200 = new Map([[12, 24], [14, 26], [17, 29], [22, 35]]);
  return { status: 'available', fontScale: scale, fontMetrics: { density, spToDp: BASES.map(base => ({ base, dp: scale === 1 ? base : actual200.get(base) ?? base * 2 })) } };
}
const flatten = value => !value || typeof value !== 'object' ? {} : Array.isArray(value) ? Object.assign({}, ...value.map(flatten)) : value;
async function withSealHost({ kind = 'status', density = 1, initialSnapshot = snapshot(1, density), props = {} } = {}, run) {
  const { React, act, withHost } = await nativeHost;
  return withHost(async h => {
    // This is an explicit native-boundary density fixture, never a layout simulation.
    h.rn.PixelRatio.get = () => density;
    h.rn.useWindowDimensions = () => ({ width: 320, height: 800, fontScale: 1, scale: density });
    const metrics = await h.load('apps/mobile/src/rudi/ui/NativeFontMetrics.tsx');
    const { Stamp } = await h.load('apps/mobile/src/rudi/ui/Stamp.tsx');
    const { StampButton } = await h.load('apps/mobile/src/rudi/ui/StampButton.tsx');
    const { MeasuredSealColumn } = await h.load('apps/mobile/src/rudi/ui/MeasuredSealColumn.tsx');
    const { PressScale } = await h.load('apps/mobile/src/rudi/ui/PressScale.tsx');
    const Component = kind === 'status' ? Stamp : StampButton;
    const state = { snapshot: initialSnapshot, props: { label: kind === 'status' ? 'đã về' : 'Gửi đề xuất', ...props } };
    function Fixture({ current }) {
      const [draft, setDraft] = React.useState('Synthetic adjacent draft');
      return React.createElement(metrics.NativeFontMetricsProvider, { snapshot: current.snapshot },
        React.createElement(h.rn.View, { testID: 'sf-parent' },
          React.createElement(metrics.TextInput, { testID: 'sf-draft', value: draft, onChangeText: setDraft, style: { fontSize: 17, lineHeight: 22, letterSpacing: 0.4 } }),
          React.createElement(Component, { label: current.props.label, measureLabel: true, size: 'vua', testID: 'sf-seal', onPress() {}, ...current.props })));
    }
    const hidden = () => h.all('RNText').filter(node => node.props.importantForAccessibility === 'no-hide-descendants' && typeof node.props.onTextLayout === 'function');
    const api = {
      state, React, act, raw: h, Component, Stamp, StampButton, PressScale, MeasuredSealColumn, sourceHashes: new Map(),
      get root() { return h.root; },
      visible() { const rows = h.all('RNText').filter(node => node.props.importantForAccessibility !== 'no-hide-descendants' && node.children.filter(value => typeof value === 'string').join('') === state.props.label); assert.equal(rows.length, 1, 'one visible actual seal label'); return rows[0]; },
      input() { const rows = h.byId('sf-draft'); assert.equal(rows.length, 1); return rows[0]; },
      seal() { const rows = h.byId('sf-seal'); assert.equal(rows.length, 1); return rows[0]; },
      wide() { assert.ok(hidden().length >= 1, 'actual wide probe exists'); return hidden()[0]; },
      fit() { const rows = hidden(); assert.ok(rows.length <= 2, 'at most wide and fitted probes'); return rows[1] ?? null; },
      width() { return flatten(api.visible().props.style).width; },
      async layout(width) { const rows = h.root.findByType(MeasuredSealColumn).findAllByType('RNView').filter(node => typeof node.props.onLayout === 'function' && flatten(node.props.style).minWidth === 0); assert.equal(rows.length, 1, 'one actual finite measurement column'); await act(async () => rows[0].props.onLayout({ nativeEvent: { layout: { width, height: 0 } } })); },
      async lines(nodeOrCallback, lines) { const callback = typeof nodeOrCallback === 'function' ? nodeOrCallback : nodeOrCallback.props.onTextLayout; await act(async () => callback({ nativeEvent: { lines: lines.map(value => typeof value === 'number' ? { width: value } : value) } })); },
      async invoke(callback, event) { await act(async () => callback(event)); },
      async change(value) { await act(async () => api.input().props.onChangeText(value)); },
      async update(nextSnapshot = state.snapshot, nextProps = state.props) { state.snapshot = nextSnapshot; state.props = nextProps; await h.render(React.createElement(Fixture, { current: state })); },
      async unmountSurface() { await h.render(React.createElement(h.rn.View, { testID: 'sf-retired-surface' })); },
      actual() {
        for (const logical of ['apps/mobile/src/rudi/ui/Stamp.tsx', 'apps/mobile/src/rudi/ui/StampButton.tsx', 'apps/mobile/src/rudi/ui/MeasuredSealColumn.tsx', 'apps/mobile/src/rudi/ui/NativeFontMetrics.tsx', 'apps/mobile/src/rudi/native-font-metrics.ts', 'apps/mobile/src/rudi/theme.ts']) {
          const file = h.consumed.get(logical); assert.ok(file, 'actual source consumed: ' + logical);
          assert.equal(fs.realpathSync(file).startsWith(sourceRoot + path.sep), true, 'source stays in selected complete graph');
          assert.equal(fs.realpathSync(file), fs.realpathSync(path.join(sourceRoot, logical)), 'canonical source has no fallback tree');
          api.sourceHashes.set(logical, crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
        }
      },
    };
    await api.update(); api.actual(); await run(api);
  });
}
async function closeSealHost() { const h = await nativeHost; h.closeHost(); }
module.exports = { withSealHost, closeSealHost, snapshot, flatten, sourceRoot };
