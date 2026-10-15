// Actual Keeps/session/metrics/primitives and deferred I/O; no Android Yoga or glyph simulation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./owner-host.cjs');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const font = scale => snapshot(scale, scale === 2 ? 36 : 24);
const unavailable = () => ({ status: 'unavailable', reason: 'module-inactive' });
const rows = [
  { id: 'synthetic-keep-1', body: 'KS fixture Aà😀\n\nKept second paragraph', ai_generated: true, created_at: '2026-10-01T00:00:00Z' },
  { id: 'synthetic-keep-2', body: 'KS fixture second note with its own complete body', ai_generated: false, created_at: '2026-10-02T00:00:00Z' },
];
const intro = 'Những ghi chép riêng của bạn. Chỉ mình bạn đọc được.';
const confirmation = 'Xoá ghi chép này? Không lấy lại được.';
const deleteRefusal = 'Chưa xoá được ghi chép này.';
const readRefusal = 'Chưa mở được ghi chép.';
const emptyTitle = 'Chưa có ghi chép nào';
const emptyBody = 'Nhờ Nếp giữ một điều từ câu chuyện bạn chọn. Ghi chép chỉ mình bạn đọc được.';
const emptyBodyText = emptyBody.replace('đọc được.', 'đọc\u00a0được.');
function exactly(nodes, message) { assert.equal(nodes.length, 1, message); return nodes[0]; }
function inside(node, parent) { for (let at = node.parent; at; at = at.parent) if (at === parent) return true; return false; }
function ancestor(node, match, message) { for (let at = node.parent; at; at = at.parent) if (match(at)) return at; assert.fail(message); }
function text(node) {
  const join = value => Array.isArray(value) ? value.map(join).join('') : typeof value === 'string' || typeof value === 'number' ? String(value) : '';
  return join(node.props.children);
}
function copy(h, value) { return exactly(h.hosts('RNText').filter(node => text(node) === value), 'one exact synthetic/native copy'); }
function title(h) { return exactly(h.hosts('RNText', props => props.accessibilityRole === 'header').filter(node => text(node) === 'Điều mình muốn giữ'), 'visible full Keeps header'); }
function card(h, row = rows[0]) {
  const Card = h.load('apps/mobile/src/rudi/ui').Card;
  return exactly(h.root.root.findAllByType(Card).filter(node => node.findAllByType('RNText').some(leaf => text(leaf) === row.body)), 'one actual reading card for the synthetic item');
}
function metadata(h, row = rows[0]) { return card(h, row).findAllByType('RNText')[0]; }
function actionWithin(node, label) { return exactly(node.findAllByType('RNPressable').filter(button => button.findAllByType('RNText').some(leaf => text(leaf) === label)), 'one actual action in its semantic region'); }
function action(h, label) { return actionWithin(h.root.root, label); }
function trash(h, row = rows[0]) { return h.one('RNPressable', props => props.accessibilityLabel === `Xoá ghi chép: «${row.body.slice(0, 40)}»`); }
async function press(node) { const callback = node.props.onPress; await act(async () => callback()); }
async function refresh(h) {
  const refreshControl = h.one('RNScrollView').props.refreshControl;
  assert.equal(typeof refreshControl?.props.onRefresh, 'function', 'actual RudiScreen refresh boundary');
  await act(async () => refreshControl.props.onRefresh());
}
function ownership(h, count, removed = 0) {
  const value = h.observation();
  assert.deepEqual({ live: value.live, subscriptions: value.subscriptions, removals: value.removals, maxLive: value.maxLive },
    { live: count - removed, subscriptions: count, removals: removed, maxLive: count }, 'cumulative native owner subscriptions, not just active count');
  if (!removed) assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1, 'one actual owner above Keeps including unavailable fallback');
}
function actual(h) {
  for (const name of ['community/Keeps', 'community/Comments', 'ui', 'ui/HoiTaiHang', 'ui/EmptyState', 'ui/CauTaiCho', 'ui/NativeFontMetrics', 'native-font-metrics', 'chia-bill/useBillFontScale', 'session']) {
    const relative = 'apps/mobile/src/rudi/' + name, file = h.consumed.get(relative);
    assert.ok(file, 'actual Keeps graph consumed: ' + relative);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
  const reader = h.consumed.get('apps/mobile/modules/rudi-font-scale');
  assert.equal(fs.realpathSync(reader), fs.realpathSync(path.join(sourceRoot, 'apps/mobile/modules/rudi-font-scale/index.ts')), 'directory receipt is bound to canonical actual native reader');
}
function metrics(node, base, value, original = false) {
  const usable = !original && value.status === 'available' && value.fontMetrics !== undefined;
  const dp = number => number === undefined ? undefined : usable ? value.fontMetrics.spToDp.find(row => row.base === number)?.dp : number;
  const style = flatten(node.props.style);
  assert.equal(style.fontSize, dp(base.fontSize), 'native table owns font size rather than stale window scale');
  assert.equal(style.lineHeight, dp(base.lineHeight), 'native table owns line pitch');
  assert.equal(style.letterSpacing, dp(base.letterSpacing));
  assert.equal(node.props.allowFontScaling, usable ? false : undefined, 'manual DP conversion or original RN fallback');
  assert.equal(node.props.maxFontSizeMultiplier, undefined, 'no cap introduced');
}
function refusal(h, value) {
  const leaf = copy(h, value);
  const error = ancestor(leaf, node => node.type === h.load('apps/mobile/src/rudi/ui/CauTaiCho').CauTaiCho, 'actual alert component');
  const region = ancestor(error, node => typeof node.type === 'function' && node.type.name === 'KeepsFailure', 'actual local recovery region');
  const retry = actionWithin(region, 'Thử lại');
  assert.equal(inside(retry, error), false, 'recovery must not divide the alert paragraph into a narrow shared row');
  assert.equal(error.findAllByType('RNPressable').length, 0, 'the sentence has its full reading region');
  const stack = ancestor(error, node => node.type === 'RNView' && inside(retry, node), 'common native stack contains the separate recovery row');
  assert.equal(flatten(stack.props.style).alignSelf, 'stretch', 'stack fills the reading column');
  assert.equal(error.props.hanhDong, undefined);
  assert.equal(leaf.props.numberOfLines, undefined); assert.equal(leaf.props.ellipsizeMode, undefined);
  assert.ok(flatten(retry.props.style).minHeight >= 48, 'retry remains a real target');
  return { leaf, error, retry, stack };
}
function transport(h, call, method, row) {
  assert.equal(call.route, method === 'GET' ? '/v2/community/keeps' : '/v2/community/keeps/' + row.id);
  assert.equal(call.options.method, method); assert.equal(call.options.actorId, 'synthetic-A');
  assert.equal(call.options.body, undefined);
  if (method === 'DELETE') {
    assert.deepEqual(Object.keys(call.options.attempt).sort(), ['at', 'key'], 'actual Attempt shape');
    assert.match(call.options.attempt.key, /^synthetic-attempt-[1-9][0-9]*$/);
    assert.equal(Number.isSafeInteger(call.options.attempt.at), true);
    assert.equal(call.options.attempt.at, Date.parse('2026-10-01T00:00:00Z') + Number(call.options.attempt.key.slice('synthetic-attempt-'.length)));
  }
}
async function withKeeps(options, run) {
  const h = createHost({ ...options, surface: 'keeps' });
  try {
    await h.mount(); await run(h);
    assert.equal(h.calls.every(call => ['readKeeps', 'deleteKeep'].includes(call.kind)), true, 'an unexpected request cannot hide behind a caught UI refusal');
  } finally { await h.dispose(); }
}
async function loaded(h) {
  assert.equal(h.keepReads.length, 1); transport(h, h.keepReads[0], 'GET');
  await h.finish(h.keepReads[0], { keeps: rows });
  assert.equal(h.hosts('RNTextInput').length, 0, 'Keeps has no editor input to fabricate');
}

test('KS01 actual Keeps owns loading, notes, alert and confirmation through hot native metrics without remounts', async () => {
  await withKeeps({}, async h => {
    ownership(h, 1); await loaded(h); actual(h);
    const type = h.load('apps/mobile/src/rudi/theme').typography;
    const body = copy(h, rows[0].body), other = copy(h, rows[1].body), meta = metadata(h), secondMeta = metadata(h, rows[1]);
    metrics(body, { ...type.body, lineHeight: 28 }, font(1));
    const controller = h.owner('KeepsContent'), readingCard = card(h), heading = title(h), introduction = copy(h, intro);
    assert.equal(flatten(trash(h).props.style).width, 48); assert.equal(flatten(trash(h).props.style).height, 48);
    await press(trash(h)); assert.equal(h.keepDeletes.length, 0, 'opening confirmation is not a write');
    const confirm = copy(h, confirmation), confirmAction = action(h, 'Xoá');
    await refresh(h); assert.equal(h.keepReads.length, 2); await h.fail(h.keepReads[1], 'Synthetic read refusal');
    const alert = copy(h, readRefusal), retry = action(h, 'Thử lại');
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value); ownership(h, 1);
      assert.equal(h.state.window.fontScale, 1, 'native event arrives with stale window scale');
      assert.equal(copy(h, rows[0].body) === body, true); assert.equal(copy(h, rows[1].body) === other, true);
      assert.equal(metadata(h) === meta, true); assert.equal(metadata(h, rows[1]) === secondMeta, true);
      assert.equal(card(h) === readingCard, true); assert.equal(h.owner('KeepsContent') === controller, true);
      assert.equal(title(h) === heading, true); assert.equal(copy(h, intro) === introduction, true);
      assert.equal(copy(h, confirmation) === confirm, true); assert.equal(action(h, 'Xoá') === confirmAction, true);
      assert.equal(copy(h, readRefusal) === alert, true); assert.equal(action(h, 'Thử lại') === retry, true);
      metrics(body, { ...type.body, lineHeight: 28 }, value); metrics(other, { ...type.body, lineHeight: 28 }, value);
      metrics(meta, type.caption, value); metrics(secondMeta, type.caption, value);
      metrics(heading, type.title, value); metrics(introduction, type.body, value); metrics(confirm, type.note, value);
      metrics(alert, type.body, value); metrics(exactly(confirmAction.findAllByType('RNText'), 'confirmation action copy'), type.label, value);
      assert.equal(h.keepReads.length, 2, 'metrics never reload notes'); assert.equal(h.keepDeletes.length, 0);
    }
    assert.deepEqual(h.routes, []); assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0);
    await h.dispose(); ownership(h, 1, 1);
    const reads = h.observation().reads, calls = h.calls.length;
    await h.invokeRetired(); assert.equal(h.observation().reads, reads); assert.equal(h.calls.length, calls);
  });
});

test('KS02 cancel is write-free and refused deletion retains the note with a separate retry of the same operation', async () => {
  await withKeeps({}, async h => {
    await loaded(h); const type = h.load('apps/mobile/src/rudi/theme').typography;
    const body = copy(h, rows[0].body), other = copy(h, rows[1].body);
    metrics(body, { ...type.body, lineHeight: 28 }, font(1));
    await press(trash(h)); await press(action(h, 'Thôi'));
    assert.equal(h.keepDeletes.length, 0); assert.equal(copy(h, rows[0].body) === body, true);
    assert.equal(h.text(confirmation).length, 0);
    await press(trash(h)); const confirm = action(h, 'Xoá'), retainedConfirm = confirm.props.onPress;
    await press(confirm); assert.equal(h.keepDeletes.length, 1); transport(h, h.keepDeletes[0], 'DELETE', rows[0]);
    await act(async () => retainedConfirm()); assert.equal(h.keepDeletes.length, 1, 'deleting guard blocks retained duplicate pending callback');
    assert.equal(action(h, 'Xoá').props.disabled, true);
    await h.nativeFont(font(2)); ownership(h, 1); metrics(copy(h, confirmation), type.note, font(2));
    await h.fail(h.keepDeletes[0], 'Synthetic deletion refusal');
    const failed = refusal(h, deleteRefusal);
    assert.equal(copy(h, rows[0].body) === body, true); assert.equal(copy(h, rows[1].body) === other, true);
    metrics(failed.leaf, type.body, font(2)); assert.equal(h.keepReads.length, 1, 'a failed delete does not pretend a refresh ACK');
    const retainedRetry = failed.retry.props.onPress;
    await press(failed.retry); assert.equal(h.keepDeletes.length, 2); transport(h, h.keepDeletes[1], 'DELETE', rows[0]);
    assert.notEqual(h.keepDeletes[1].options.attempt.key, h.keepDeletes[0].options.attempt.key, 'retry keeps ID and rotates only the transport attempt');
    await act(async () => retainedRetry()); assert.equal(h.keepDeletes.length, 2);
    await h.nativeFont(font(1)); assert.equal(copy(h, rows[0].body) === body, true); ownership(h, 1);
    await h.finish(h.keepDeletes[1]); assert.equal(h.keepReads.length, 2, 'successful delete awaits actual GET acknowledgement');
    assert.equal(copy(h, rows[0].body) === body, true, 'old row is retained until reload answers');
    transport(h, h.keepReads[1], 'GET'); await h.finish(h.keepReads[1], { keeps: [rows[1]] });
    assert.equal(h.text(rows[0].body).length, 0); assert.equal(copy(h, rows[1].body) === other, true, 'ID-keyed surviving row is retained');
    assert.equal(h.text(deleteRefusal).length, 0); assert.equal(h.keepDeletes.length, 2);
    assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0); assert.deepEqual(h.routes, []);
  });
});

test('KS03 initial loading and refusal differ from retained refresh data and the compact empty way in', async () => {
  await withKeeps({}, async h => {
    ownership(h, 1);
    const skeleton = h.load('apps/mobile/src/rudi/ui/Skeleton');
    assert.equal(h.root.root.findAllByType(skeleton.SkeletonGroup).length, 1);
    assert.equal(h.root.root.findAllByType(skeleton.SkeletonRow).length, 2);
    assert.equal(h.text(emptyTitle).length, 0);
    await h.fail(h.keepReads[0], 'Synthetic initial refusal');
    refusal(h, readRefusal); assert.equal(h.root.root.findAllByType(skeleton.SkeletonGroup).length, 0); assert.equal(h.text(emptyTitle).length, 0);
    await press(action(h, 'Thử lại')); assert.equal(h.keepReads.length, 2); await h.finish(h.keepReads[1], { keeps: rows });
    const body = copy(h, rows[0].body), type = h.load('apps/mobile/src/rudi/theme').typography;
    metrics(body, { ...type.body, lineHeight: 28 }, font(1));
    await refresh(h); assert.equal(h.keepReads.length, 3); await h.fail(h.keepReads[2], 'Synthetic reload refusal');
    const failed = refusal(h, readRefusal); assert.equal(copy(h, rows[0].body) === body, true);
    await h.nativeFont(font(2)); ownership(h, 1); metrics(failed.leaf, type.body, font(2));
    await press(failed.retry); assert.equal(h.keepReads.length, 4); await h.finish(h.keepReads[3], { keeps: [] });
    assert.equal(h.text(rows[0].body).length, 0); assert.equal(h.text(readRefusal).length, 0);
    const Empty = h.load('apps/mobile/src/rudi/ui/EmptyState').EmptyState;
    const empty = exactly(h.root.root.findAllByType(Empty), 'actual compact EmptyState');
    assert.equal(empty.props.layout, 'inline'); assert.equal(empty.props.nativeMetrics, true);
    assert.equal(empty.props.body, emptyBody, 'original explanation retains the existing orphan-joining typography helper');
    assert.equal(empty.props.illustration, undefined, 'the way in does not add a decorative picture');
    const heading = copy(h, emptyTitle), explanation = copy(h, emptyBodyText);
    metrics(heading, type.h2, font(2)); metrics(explanation, type.body, font(2));
    assert.equal(flatten(heading.props.style).textAlign, 'left'); assert.equal(flatten(explanation.props.style).textAlign, 'left');
    const enter = action(h, 'Tới cộng đồng'); assert.ok(flatten(enter.props.style).minHeight >= 48);
    await h.nativeFont(font(1)); ownership(h, 1);
    assert.equal(copy(h, emptyTitle) === heading, true); assert.equal(copy(h, emptyBodyText) === explanation, true);
    metrics(heading, type.h2, font(1)); metrics(explanation, type.body, font(1));
    await press(enter); assert.deepEqual(h.routes, ['/(tabs)/community']);
    assert.equal(h.keepDeletes.length, 0); assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0);
  });
});

test('KS04 inherited and unavailable owners stay distinct from an absent native module and child opt-ins preserve defaults', async () => {
  for (const options of [
    { explicitProvider: true, initialSnapshot: font(2) },
    { explicitProvider: true, initialSnapshot: unavailable() },
    { initialSnapshot: unavailable() },
    { initialSnapshot: font(2), nativeFontModulePresent: false },
  ]) {
    await withKeeps(options, async h => {
      await loaded(h);
      const count = options.explicitProvider || options.nativeFontModulePresent === false ? 0 : 1;
      const initial = options.nativeFontModulePresent === false ? unavailable() : options.initialSnapshot;
      const body = copy(h, rows[0].body), type = h.load('apps/mobile/src/rudi/theme').typography;
      metrics(body, { ...type.body, lineHeight: 28 }, initial); ownership(h, count);
      const controller = h.owner('KeepsContent');
      if (!count) assert.equal(h.observation().reads, 0, 'inherited/absent module never starts a child native read');
      for (const value of [font(2), unavailable(), { status: 'available', fontScale: 2 }, font(1)]) {
        await h.nativeFont(value); ownership(h, count);
        metrics(body, { ...type.body, lineHeight: 28 }, options.nativeFontModulePresent === false ? unavailable() : value);
        assert.equal(copy(h, rows[0].body) === body, true); assert.equal(h.owner('KeepsContent') === controller, true);
        assert.equal(h.keepReads.length, 1); assert.equal(h.keepDeletes.length, 0);
      }
      actual(h);
    });
  }
  let confirmed = 0, cancelled = 0, entered = 0;
  const h = createHost({ surface: 'boundary', explicitProvider: true, initialSnapshot: font(2), boundaryChildren: ({ React, load }) => {
    const Hoi = load('apps/mobile/src/rudi/ui/HoiTaiHang').HoiTaiHang;
    const Empty = load('apps/mobile/src/rudi/ui/EmptyState').EmptyState;
    return React.createElement(React.Fragment, null,
      React.createElement(Hoi, { testID: 'ks-default-confirm', cau: 'Synthetic default confirmation', nhan: 'Synthetic confirm default', onDongY: () => confirmed++, onThoi: () => cancelled++ }),
      React.createElement(Hoi, { testID: 'ks-opted-confirm', nativeMetrics: true, cau: 'Synthetic opted confirmation', nhan: 'Synthetic confirm opted', onDongY() {}, onThoi() {} }),
      React.createElement(Empty, { testID: 'ks-default-empty', kind: 'first-use', layout: 'inline', title: 'Synthetic default empty', body: 'Synthetic default explanation', action: { label: 'Synthetic enter default', onPress: () => entered++ } }),
      React.createElement(Empty, { testID: 'ks-opted-empty', kind: 'first-use', nativeMetrics: true, layout: 'inline', title: 'Synthetic opted empty', body: 'Synthetic opted explanation' }));
  } });
  try {
    await h.mount(); ownership(h, 0); assert.equal(h.observation().reads, 0);
    const type = h.load('apps/mobile/src/rudi/theme').typography;
    const defaultConfirm = copy(h, 'Synthetic default confirmation'), optedConfirm = copy(h, 'Synthetic opted confirmation');
    const defaultEmpty = copy(h, 'Synthetic default empty'), optedEmpty = copy(h, 'Synthetic opted empty');
    const defaultExplanation = copy(h, 'Synthetic default explanation'), optedExplanation = copy(h, 'Synthetic opted explanation');
    for (const value of [font(2), unavailable(), font(1), font(2)]) {
      await h.nativeFont(value); ownership(h, 0);
      assert.equal(copy(h, 'Synthetic default confirmation') === defaultConfirm, true); assert.equal(copy(h, 'Synthetic opted confirmation') === optedConfirm, true);
      assert.equal(copy(h, 'Synthetic default empty') === defaultEmpty, true); assert.equal(copy(h, 'Synthetic opted empty') === optedEmpty, true);
      metrics(defaultConfirm, type.note, value, true); metrics(optedConfirm, type.note, value);
      metrics(defaultEmpty, type.h2, value, true); metrics(optedEmpty, type.h2, value);
      metrics(defaultExplanation, type.body, value, true); metrics(optedExplanation, type.body, value);
    }
    await press(action(h, 'Synthetic confirm default')); await press(actionWithin(h.one('RNView', props => props.testID === 'ks-default-confirm'), 'Thôi'));
    await press(action(h, 'Synthetic enter default'));
    assert.deepEqual({ confirmed, cancelled, entered }, { confirmed: 1, cancelled: 1, entered: 1 });
    assert.equal(h.calls.length, 0, 'compatibility witnesses have no I/O');
  } finally { await h.dispose(); }
});
