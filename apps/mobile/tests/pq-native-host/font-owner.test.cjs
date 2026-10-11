// Actual source ownership and metrics checks; host props never prove Android rendering.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const crypto = require('node:crypto');
const { createRequire } = require('node:module');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, React, act, repo, sourceRoot } = require('./owner-host.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const oldError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) oldError(...args); };
after(() => { console.error = oldError; if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const plain = value => JSON.parse(JSON.stringify(value));
async function withHost(options, run) {
  const h = createHost(options);
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
function ownership(h, live = 1, subscriptions = 1, removals = 0) {
  const observed = h.observation();
  assert.equal(observed.live, live, 'live native font observers');
  assert.equal(observed.subscriptions, subscriptions, 'cumulative subscriptions');
  assert.equal(observed.removals, removals, 'cumulative cleanup');
  assert.equal(observed.maxLive, subscriptions === 0 ? 0 : 1, 'never overlap two surface observers');
}
function actual(h, names) {
  for (const name of names) {
    const relative = `apps/mobile/${name}`;
    const file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(file.startsWith(sourceRoot + path.sep), true, 'source stays inside the selected complete graph');
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function inputMetrics(h, size, line, min) {
  const style = flatten(h.input().props.style);
  assert.equal(style.fontSize, size); assert.equal(style.lineHeight, line); assert.equal(style.minHeight, min);
}
function same(h, input, owner, body) {
  assert.equal(h.input() === input, true, 'same native input host within one phase');
  assert.equal(h.owner('CommentsForContext') === owner, true, 'same keyed comment context within one phase');
  assert.equal(h.input().props.value, body);
}
const unavailable = () => ({ status: 'unavailable', reason: 'module-unavailable' });

test('FO01 explicit available or unavailable provider owns fallback without nested native reads', async () => {
  for (const value of [snapshot(2), unavailable(), { status: 'available', fontScale: 2 }]) {
    await withHost({ surface: 'comments', explicitProvider: true, initialSnapshot: value }, async h => {
      ownership(h, 0, 0, 0); assert.equal(h.observation().reads, 0, 'explicit provider never starts an observer');
      if (value.fontMetrics) {
        inputMetrics(h, 28.75, 39.25, 39.5); assert.equal(h.input().props.allowFontScaling, false);
      } else {
        inputMetrics(h, 17, 24, 24); assert.equal(h.input().props.allowFontScaling, undefined, 'RN owns fallback scaling');
      }
      actual(h, ['src/rudi/community/Comments', 'src/rudi/ui/NativeFontMetrics', 'src/rudi/ui/Field']);
    });
  }
});

test('FO02 standalone comments retain one observer input and journal through recovery and cleanup', async () => {
  await withHost({ surface: 'comments' }, async h => {
    ownership(h); assert.equal(h.observation().reads, 2, 'subscribe first then close the read gap');
    await h.change('Synthetic standalone font draft'); await h.focus();
    const input = h.input(), owner = h.owner('CommentsForContext'), saved = plain(h.drafts.at(-1));
    const reads = h.callsFor('readComments').length;
    for (const [value, size, line, min] of [
      [snapshot(2), 28.75, 39.25, 39.5], [unavailable(), 17, 24, 24],
      [{ status: 'available', fontScale: 2 }, 17, 24, 24], [snapshot(1), 17, 24, 24],
    ]) {
      await h.nativeFont(value); same(h, input, owner, saved.body); ownership(h);
      inputMetrics(h, size, line, min); assert.deepEqual(plain(h.drafts.at(-1)), saved);
      assert.equal(h.callsFor('readComments').length, reads, 'font configuration does not read the thread again');
    }
    await h.dispose(); ownership(h, 0, 1, 1);
    const after = h.observation().reads, apiCalls = h.calls.length;
    await h.invokeRetired();
    assert.equal(h.observation().reads, after, 'retired font callback does not even reread native state');
    assert.equal(h.calls.length, apiCalls);
  });
});

test('FO03 detail owns one cumulative observer from loading through real sheet completion', async () => {
  await withHost({ surface: 'detail', deferPost: true }, async h => {
    const content = h.owner('PostDetailContent');
    assert.equal(h.editors().length, 0, 'initial transport is pending'); ownership(h);
    assert.equal(h.callsFor('getPost').length, 1);
    await h.resolvePost(); assert.equal(h.editors().length, 1); ownership(h);
    await h.openComments(); ownership(h); assert.equal(h.commentSheet().props.open, true);
    await h.closeComments(); const closing = h.closeAnimation(); ownership(h);
    assert.equal(h.editors().length, 1, 'closing completion still owns its editor');
    await h.completeClose(closing); ownership(h);
    assert.equal(h.owner('PostDetailContent') === content, true, 'root controller survives every phase');
    assert.equal(h.editors().length, 1);
    actual(h, ['src/rudi/community/PostDetail', 'src/rudi/community/Comments', 'src/rudi/ui/Sheet', 'src/rudi/ui', 'src/rudi/chia-bill/useBillFontScale', 'modules/rudi-font-scale']);
    assert.equal(fs.realpathSync(h.consumed.get('apps/mobile/modules/rudi-font-scale')), fs.realpathSync(path.join(sourceRoot, 'apps/mobile/modules/rudi-font-scale/index.ts')), 'native reader receipt identifies the exact canonical index.ts file');
    await h.dispose(); ownership(h, 0, 1, 1);
  });
});

test('FO04 hot metrics in loading inline sheet and closing keep the current controller and editor', async () => {
  await withHost({ surface: 'detail', deferPost: true }, async h => {
    const content = h.owner('PostDetailContent');
    const values = [snapshot(2), unavailable(), { status: 'available', fontScale: 2 }, snapshot(1)];
    for (const value of values) {
      await h.nativeFont(value); ownership(h);
      assert.equal(h.owner('PostDetailContent') === content, true); assert.equal(h.callsFor('getPost').length, 1);
    }
    await h.resolvePost(); await h.change('Synthetic phase font draft');
    for (const phase of ['inline', 'sheet', 'closing']) {
      if (phase === 'sheet') await h.openComments();
      if (phase === 'closing') await h.closeComments();
      const input = h.input(), owner = h.owner('CommentsForContext'), writer = h.writer();
      const bodyScroll = h.one('RNScrollView', props => props.testID === 'community-post-body');
      const gets = h.callsFor('getPost').length, reads = h.callsFor('readComments').length;
      for (const value of values) {
        await h.nativeFont(value); ownership(h); same(h, input, owner, 'Synthetic phase font draft');
        assert.equal(h.writer() === writer, true, 'metrics never rotate the current writer lease');
        assert.equal(h.owner('PostDetailContent') === content, true);
        assert.equal(h.one('RNScrollView', props => props.testID === 'community-post-body') === bodyScroll, true, 'phase-local reference host stays mounted');
        assert.equal(h.callsFor('getPost').length, gets); assert.equal(h.callsFor('readComments').length, reads);
        const converted = value.fontMetrics !== undefined;
        inputMetrics(h, converted && value.fontScale === 2 ? 28.75 : 17, converted && value.fontScale === 2 ? 39.25 : 24, converted && value.fontScale === 2 ? 39.5 : 24);
        assert.equal(h.input().props.allowFontScaling, converted ? false : undefined);
      }
    }
    await h.completeClose(); ownership(h); assert.equal(h.input().props.value, 'Synthetic phase font draft');
  });
});

test('FO05 feed memo cards and comment sheet reuse one owner and convert supported metadata', async () => {
  await withHost({ surface: 'feed' }, async h => {
    ownership(h); assert.equal(h.cards().length, 3);
    const content = h.owner('CommunityScreenContent'), list = h.one('RNFlatList');
    await h.nativeFont(snapshot(2)); ownership(h);
    assert.equal(h.owner('CommunityScreenContent') === content, true); assert.equal(h.one('RNFlatList') === list, true);
    for (const row of h.state.feedPosts) {
      const bodies = h.text(row.body); assert.equal(bodies.length, 1);
      assert.equal(flatten(bodies[0].props.style).fontSize, 28.75); assert.equal(flatten(bodies[0].props.style).lineHeight, 39.25);
      assert.equal(bodies[0].props.allowFontScaling, false);
    }
    const names = h.text('Synthetic synthetic-B'); assert.equal(names.length, 3);
    for (const name of names) { assert.equal(flatten(name.props.style).fontSize, 28.75); assert.equal(flatten(name.props.style).lineHeight, 37.5); }
    const metadata = h.text(props => Array.isArray(props.children) && props.children.includes('Cộng đồng'));
    assert.equal(metadata.length, 3);
    for (const text of metadata) { assert.equal(flatten(text.props.style).fontSize, 21.5); assert.equal(flatten(text.props.style).lineHeight, 30.25); assert.equal(text.props.allowFontScaling, false); }
    const count = h.text('2 lượt thích · 3 bình luận'); assert.equal(count.length, 3);
    for (const text of count) assert.equal(flatten(text.props.style).fontSize, 21.5);
    const unsupported = h.text('Cộng đồng').find(text => flatten(text.props.style).lineHeight === 39);
    assert.ok(unsupported); assert.equal(unsupported.props.allowFontScaling, undefined, 'unsupported display metrics keep RN fallback');
    await h.openComments(1); ownership(h); await h.change('Synthetic feed comment');
    await h.closeComments(); ownership(h); await h.completeClose(); ownership(h);
    await act(async () => { h.state.feedPosts = h.state.feedPosts.map(row => ({ ...row, body: row.body + ' refreshed' })); await h.one('RNFlatList').props.refreshControl.props.onRefresh(); });
    assert.equal(h.cards().length, 3); ownership(h);
    assert.equal(h.one('RNFlatList') === list, true);
    for (const row of h.state.feedPosts) assert.equal(h.text(row.body)[0].props.allowFontScaling, false);
    await h.pressText('Đang theo dõi'); ownership(h);
    actual(h, ['src/rudi/community/CommunityScreen', 'src/rudi/community/PostCard', 'src/rudi/community/Comments', 'src/rudi/theme', 'src/rudi/community/api', 'src/rudi/ui/NativeFontMetrics']);
  });
});

test('FO06 ownership survives unusable tables and same-scale native metrics recovery', async () => {
  for (const explicitProvider of [true, false]) {
    await withHost({ surface: 'boundary', explicitProvider }, async h => {
      const input = h.input(); await h.change('Synthetic fallback boundary draft');
      const malformed = snapshot(2); malformed.fontMetrics.spToDp.pop();
      const entries = explicitProvider
        ? [[snapshot(2), 28.75, 39.25, 118.5, false], [{ status: 'available', fontScale: 2 }, 17, 24, 72, undefined], [unavailable(), 17, 24, 72, undefined], [snapshot(2, 55), 28.75, 55, 165, false]]
        : [[snapshot(2), 28.75, 39.25, 118.5, false], [malformed, 17, 24, 72, undefined], [unavailable(), 17, 24, 72, undefined], [snapshot(2, 55), 28.75, 55, 165, false]];
      for (const [value, size, line, min, scaled] of entries) {
        await h.nativeFont(value);
        ownership(h, explicitProvider ? 0 : 1, explicitProvider ? 0 : 1, 0);
        assert.equal(h.one('OwnerProbe').props.owned, true, 'provider still owns unavailable or missing metrics');
        assert.equal(h.input() === input, true); assert.equal(h.input().props.value, 'Synthetic fallback boundary draft');
        inputMetrics(h, size, line, min); assert.equal(h.input().props.allowFontScaling, scaled);
      }
      if (explicitProvider) assert.equal(h.observation().reads, 0);
      actual(h, ['src/rudi/ui/NativeFontMetrics', 'src/rudi/community/Comments', 'src/rudi/native-font-metrics']);
    });
  }
});

test('FO07 pending retry ACK and picker completion preserve journals without observer churn', async () => {
  await withHost({ surface: 'detail' }, async h => {
    ownership(h); await h.change('Synthetic pending font draft'); await h.send();
    const sent = plain(h.writes[0].options.body), input = h.input(), owner = h.owner('CommentsForContext');
    await h.nativeFont(snapshot(2)); ownership(h); same(h, input, owner, sent.body);
    assert.equal(h.writes.length, 1); assert.equal(h.input().props.editable, false);
    await h.rejectWrite(0); await h.send();
    assert.deepEqual(plain(h.writes[1].options.body), sent, 'response-loss retry retains exact body and logical id');
    await h.openComments(); await h.change('Synthetic newer after handoff');
    await h.resolveWrite(1); assert.equal(h.input().props.value, 'Synthetic newer after handoff', 'retired ACK cannot clear the newer editor');
    await h.send(); assert.notEqual(h.writes[2].options.body.logical_id, sent.logical_id);
    await h.closeComments(); const closing = h.closeAnimation();
    await h.nativeFont(snapshot(1)); ownership(h);
    await h.resolveWrite(2); await h.completeClose(closing);
    assert.equal(h.input().props.value, '', 'accepted closing writer cannot resurrect its draft'); ownership(h); assert.equal(h.writes.length, 3);
    await h.pressAccessibility('Thêm ảnh'); assert.equal(h.picks.length, 1, 'real picker owns a pending native permission reply');
    await h.nativeFont(snapshot(2)); ownership(h);
    await h.openComments(); const newer = h.input();
    await h.finish(h.picks[0], { canceled: false, assets: [{ uri: 'file:///synthetic-retired.jpg', type: 'image', mimeType: 'image/jpeg', width: 32, height: 32 }] }); ownership(h);
    assert.equal(h.nativeHostCalls.filter(call => call.kind === 'manipulateAsync').length, 0, 'successful retired picker reply must not even process its image');
    assert.equal(h.uploads.length, 0, 'retired selection never reaches upload transport');
    assert.equal(h.input() === newer, true); assert.equal(h.input().props.value, ''); assert.equal(h.writes.length, 3);
    actual(h, ['src/rudi/community/MediaPicker', 'src/rudi/community/api', 'src/rudi/community/PostDetail']);
  });
});

test('FO08 metrics import has no native bridge dependency and nearest explicit provider wins', async () => {
  const ts = dependency('typescript'), Renderer = dependency('react-test-renderer');
  const cache = new Map(), consumed = new Map(), externals = [];
  const Native = { Text: 'PureRNText', TextInput: 'PureRNTextInput', StyleSheet: { flatten } };
  function load(relative) {
    if (cache.has(relative)) return cache.get(relative).exports;
    const file = ['.ts', '.tsx'].map(ext => path.resolve(sourceRoot, relative + ext)).find(fs.existsSync);
    assert.ok(file && fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), 'pure loader stays in frozen graph');
    consumed.set(relative, file);
    const output = ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true }, reportDiagnostics: true });
    assert.deepEqual((output.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error).map(item => ({ code: item.code, start: item.start, message: ts.flattenDiagnosticMessageText(item.messageText, '\n') })), []);
    const module = { exports: {} }; cache.set(relative, module);
    const req = specifier => {
      if (specifier === 'react') { externals.push(specifier); return React; }
      if (specifier === 'react/jsx-runtime') { externals.push(specifier); return dependency(specifier); }
      if (specifier === 'react-native') { externals.push(specifier); return Native; }
      assert.equal(specifier.startsWith('.'), true, 'no optional Expo bridge is allowed by this import boundary');
      return load(path.posix.normalize(path.posix.join(path.posix.dirname(relative), specifier)));
    };
    vm.runInNewContext(`(function(require,module,exports){${output.outputText}\n})`, { console }, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  const metrics = load('apps/mobile/src/rudi/ui/NativeFontMetrics');
  assert.equal(consumed.has('apps/mobile/src/rudi/chia-bill/useBillFontScale'), false);
  assert.equal(consumed.has('apps/mobile/modules/rudi-font-scale/index'), false);
  assert.equal(externals.every(name => ['react', 'react/jsx-runtime', 'react-native'].includes(name)), true);
  let root;
  try {
    await act(async () => { root = Renderer.create(React.createElement(metrics.NativeFontMetricsProvider, { snapshot: snapshot(2) },
      React.createElement(metrics.Text, { testID: 'outer', style: { fontSize: 17, lineHeight: 24 } }, 'Outer'),
      React.createElement(metrics.NativeFontMetricsProvider, { snapshot: snapshot(1) }, React.createElement(metrics.Text, { testID: 'inner', style: { fontSize: 17, lineHeight: 24 } }, 'Inner')))); });
    const text = id => root.root.findAllByType('PureRNText').find(node => node.props.testID === id);
    assert.equal(flatten(text('outer').props.style).fontSize, 28.75); assert.equal(flatten(text('outer').props.style).lineHeight, 39.25);
    assert.equal(flatten(text('inner').props.style).fontSize, 17); assert.equal(flatten(text('inner').props.style).lineHeight, 24);
    assert.equal(text('inner').props.allowFontScaling, false);
  } finally { if (root) await act(async () => root.unmount()); }
});
