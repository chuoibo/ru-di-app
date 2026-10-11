// Actual tab and stamp graph; native boundaries do not emulate Yoga or glyph paint.
const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./bottom-tabs-host.cjs');
global.IS_REACT_ACT_ENVIRONMENT = true;

const destinations = ['Khám phá', 'Lên plan', 'Tin nhắn', 'Cá nhân'];
const labels = [...destinations, 'Tạo'];
const unavailable = () => ({ status: 'unavailable', reason: 'context-unavailable' });
const noTable = scale => ({ status: 'available', fontScale: scale });
function pitchSnapshot(pitch) {
  const value = snapshot(2);
  value.fontMetrics.spToDp.find(row => row.base === 14).dp = pitch;
  return value;
}
function content(value) {
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  if (Array.isArray(value)) return value.map(content).join('');
  return '';
}
function label(h, value) {
  return h.one('RNText', props => content(props.children) === value);
}
function tab(h, value) {
  return h.one('RNPressable', props => props.accessibilityRole === 'tab' && props.accessibilityLabel === value);
}
function nearest(node, kind) {
  for (let current = node.parent; current; current = current.parent) if (current.type === kind) return current;
  throw new Error('Expected actual ancestor host');
}
function bar(h) {
  return nearest(h.one('RNView', props => props.role === 'tablist'), 'RNView');
}
function stamp(h) {
  return h.one('RNPressable', props => props.testID === 'con-dau-tao');
}
function remember(h) {
  return { owner: h.owner('RudiTabBar'), texts: labels.map(value => label(h, value)), tabs: destinations.map(value => tab(h, value)), stamp: stamp(h), bar: bar(h) };
}
function sameHosts(h, before) {
  assert.equal(h.owner('RudiTabBar') === before.owner, true, 'same mounted exported bar owner');
  assert.equal(bar(h) === before.bar, true, 'same actual bar parent');
  assert.equal(stamp(h) === before.stamp, true, 'same stamp native button');
  labels.forEach((value, index) => assert.equal(label(h, value) === before.texts[index], true, 'same five native label hosts'));
  destinations.forEach((value, index) => assert.equal(tab(h, value) === before.tabs[index], true, 'same four native destination hosts'));
}
function textMetrics(h, size, pitch, manual) {
  labels.forEach((value, index) => {
    const native = label(h, value), style = flatten(native.props.style);
    assert.equal(content(native.props.children), value, 'full destination and stamp labels');
    assert.equal(style.fontSize, size, 'actual native label font size');
    assert.equal(style.lineHeight, pitch, 'actual native label line pitch');
    assert.equal(native.props.allowFontScaling, manual ? false : undefined, 'converted metrics or original RN fallback');
    assert.equal(native.props.maxFontSizeMultiplier, undefined, 'no font size cap added');
    assert.equal(native.props.numberOfLines, index < 4 ? 2 : 1, 'existing full label line contract');
  });
}
function ownership(h, subscriptions, reads, removals = 0) {
  const observed = h.observation();
  assert.equal(observed.subscriptions, subscriptions, 'cumulative native subscriptions');
  assert.equal(observed.removals, removals, 'cumulative native cleanup');
  assert.equal(observed.live, subscriptions - removals, 'live native observers');
  assert.equal(observed.maxLive, subscriptions === 0 ? 0 : 1, 'no additional descendant observer');
  assert.equal(observed.nativeReads, reads, 'native reads follow actual observer notifications');
}
function actual(h) {
  const expected = [
    'apps/mobile/src/rudi/ui/RudiTabBar', 'apps/mobile/src/rudi/ui/ConDauTao',
    'apps/mobile/src/rudi/ui/NativeFontMetrics', 'apps/mobile/src/rudi/chia-bill/useBillFontScale',
    'apps/mobile/modules/rudi-font-scale', 'apps/mobile/src/rudi/native-font-metrics',
    'apps/mobile/src/rudi/session', 'apps/mobile/src/rudi/ui/thanh-tab',
    'apps/mobile/src/rudi/ui/PressScale', 'apps/mobile/src/rudi/screens/Create',
    'apps/mobile/src/rudi/nep/NepProvider',
  ];
  for (const relative of expected) {
    const file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(sourceRoot + path.sep), true, 'source remains in one bound graph');
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'), 'consumed receipt matches exact source bytes');
  }
  assert.equal(path.basename(h.consumed.get('apps/mobile/modules/rudi-font-scale')), 'index.ts', 'native module directory receipt resolves its actual index');
}
function noUnexpectedEffects(h) {
  assert.equal(h.calls.length, 0, 'font and navigation checks never call product transport');
  assert.equal(h.picks.length, 0, 'font and navigation checks never open media picker');
}
async function withHost(options, callback) {
  const h = createHost(options);
  try { await h.mount(); await callback(h); } finally { await h.dispose(); }
}
async function cleanedObserver(h, reads) {
  await h.dispose(); ownership(h, 1, reads, 1);
  await h.notifyRetired(); ownership(h, 1, reads, 1);
  noUnexpectedEffects(h);
}

// Native notifications alone must propagate; useWindowDimensions deliberately stays 1.
test('TB01 local tab owner preserves full label hosts through hot native100 200 100 and same-scale table changes', async () => {
  await withHost({}, async h => {
    ownership(h, 1, 2);
    const before = remember(h);
    textMetrics(h, 12, 14, true);
    assert.equal(flatten(bar(h).props.style).height, 76, '64dp phone policy plus 12dp safe area');
    await h.nativeFont(snapshot(2));
    assert.equal(h.state.window.fontScale, 1, 'window remains stale during actual native event');
    assert.equal(flatten(bar(h).props.style).height, 113, 'observed native scale reserves the two-line phone bar');
    textMetrics(h, 20.25, 23.25, true); sameHosts(h, before); ownership(h, 1, 3);
    await h.nativeFont(pitchSnapshot(37.25));
    assert.equal(flatten(bar(h).props.style).height, 121, 'actual two-line pitch can exceed the scalar policy');
    textMetrics(h, 20.25, 37.25, true); sameHosts(h, before); ownership(h, 1, 4);
    await h.nativeFont(snapshot());
    assert.equal(flatten(bar(h).props.style).height, 76, 'usable font100 restores its smaller reserve');
    textMetrics(h, 12, 14, true); sameHosts(h, before); ownership(h, 1, 5);
    assert.deepEqual(h.routes, []); assert.deepEqual(h.navigations, []);
    actual(h); await cleanedObserver(h, 5);
  });
});

test('TB02 inherited available and unavailable ownership never installs a child observer', async () => {
  // This actual ancestor hook owns the sole native observer; hot events do not root.update.
  await withHost({ inheritMetrics: true }, async h => {
    ownership(h, 1, 2);
    const before = remember(h);
    textMetrics(h, 12, 14, true);
    await h.nativeFont(snapshot(2));
    assert.equal(flatten(bar(h).props.style).height, 93, 'inherited effective pitch supplies a two-line minimum without inventing ancestor scalar');
    textMetrics(h, 20.25, 23.25, true); sameHosts(h, before); ownership(h, 1, 3);
    await h.nativeFont(unavailable());
    assert.equal(flatten(bar(h).props.style).height, 93, 'owned unavailable fallback retains committed geometry');
    textMetrics(h, 12, 14, false); sameHosts(h, before); ownership(h, 1, 4);
    await h.nativeFont(snapshot());
    assert.equal(flatten(bar(h).props.style).height, 76);
    textMetrics(h, 12, 14, true); sameHosts(h, before); ownership(h, 1, 5);
    actual(h); await cleanedObserver(h, 5);
  });
  // A provider without an observer still owns fallback. Updating its props is explicit.
  for (const initialSnapshot of [unavailable(), noTable(2)]) {
    await withHost({ inheritMetrics: true, observeInheritedMetrics: false, initialSnapshot }, async h => {
      ownership(h, 0, 0);
      const before = remember(h);
      textMetrics(h, 12, 14, false);
      assert.equal(flatten(bar(h).props.style).height, 76, 'unusable inherited snapshot does not fabricate available native scale');
      await h.update(() => { h.state.nativeFont = snapshot(2); });
      ownership(h, 0, 0); textMetrics(h, 20.25, 23.25, true); sameHosts(h, before);
      assert.equal(flatten(bar(h).props.style).height, 93);
      noUnexpectedEffects(h);
    });
  }
});

test('TB03 missing native module and unusable snapshots preserve honest RN fallback and committed reserve', async () => {
  await withHost({ nativeModulePresent: false, initialWindow: { width: 320, height: 640, fontScale: 1.3, scale: 2 } }, async h => {
    ownership(h, 0, 0);
    textMetrics(h, 12, 14, false);
    assert.equal(flatten(bar(h).props.style).height, 83, 'missing module keeps actual window policy instead of pretending native100');
    noUnexpectedEffects(h);
  });
  await withHost({ initialSnapshot: pitchSnapshot(37.25) }, async h => {
    const before = remember(h);
    assert.equal(flatten(bar(h).props.style).height, 121, 'known effective two-line reserve is committed');
    textMetrics(h, 20.25, 37.25, true); ownership(h, 1, 2);
    let reads = 2;
    for (const value of [unavailable(), noTable(1), noTable(2)]) {
      await h.nativeFont(value); reads += 1;
      assert.equal(flatten(bar(h).props.style).height, 121, 'unusable snapshots cannot shrink committed geometry');
      textMetrics(h, 12, 14, false); sameHosts(h, before); ownership(h, 1, reads);
    }
    await h.nativeFont(snapshot()); reads += 1;
    assert.equal(flatten(bar(h).props.style).height, 76, 'usable100 permits reserve recovery');
    textMetrics(h, 12, 14, true); sameHosts(h, before); ownership(h, 1, reads);
    actual(h); await cleanedObserver(h, reads);
  });
});

test('TB04 rail indicator follows actual label rows while real tab and create actions retain semantics', async () => {
  await withHost({ initialWindow: { width: 768, height: 1024, fontScale: 1, scale: 2 } }, async h => {
    const before = remember(h);
    const indicator = () => h.one('RNAnimatedView', props => props.testID === 'vach-thanh');
    const checkRail = (row, y) => {
      assert.equal(flatten(bar(h).props.style).width, 104);
      destinations.forEach(value => assert.equal(flatten(tab(h, value).props.style).height, row, 'actual rail destination row fits its two-line metrics'));
      const style = flatten(indicator().props.style);
      assert.equal(style.height, row, 'indicator height matches actual rows');
      assert.equal(style.transform.find(item => Object.hasOwn(item, 'translateY')).translateY, y, 'selected profile indicator follows the same actual row step');
      assert.equal(flatten(stamp(h).props.style).height, 96, 'existing stamp rail slot preserved');
    };
    checkRail(72, 404);
    await h.nativeFont(snapshot(2));
    checkRail(73, 407); textMetrics(h, 20.25, 23.25, true); sameHosts(h, before); ownership(h, 1, 3);
    // Independent same-scale pitch remains within the preserved 96dp stamp rail slot.
    await h.nativeFont(pitchSnapshot(32));
    checkRail(90, 458); textMetrics(h, 20.25, 32, true); sameHosts(h, before); ownership(h, 1, 4);
    await h.nativeFont(snapshot());
    checkRail(72, 404); textMetrics(h, 12, 14, true); sameHosts(h, before); ownership(h, 1, 5);
    await h.pressAccessibility('Cá nhân');
    assert.equal(h.tabEvents.at(-1).target, 'route-profile'); assert.deepEqual(h.navigations, [], 'focused tab emits without navigating');
    h.load('apps/mobile/src/rudi/ui/thanh-tab').ghiMucKhamPha('community');
    await h.pressAccessibility('Khám phá');
    assert.equal(h.tabEvents.at(-1).target, 'route-community'); assert.deepEqual(h.navigations, ['community']);
    h.state.prevented = true;
    await h.pressAccessibility('Lên plan');
    assert.equal(h.tabEvents.at(-1).target, 'route-plan'); assert.deepEqual(h.navigations, ['community'], 'prevented real tab handler does not navigate');
    for (const event of h.tabEvents) { assert.equal(event.type, 'tabPress'); assert.equal(event.canPreventDefault, true); }
    await act(async () => stamp(h).props.onPress({ nativeEvent: {} }));
    assert.equal(h.routes.at(-1), '/create?tu=profile');
    await act(async () => stamp(h).props.onLongPress()); assert.equal(h.routes.at(-1), '/moments/new');
    await act(async () => stamp(h).props.onAccessibilityAction({ nativeEvent: { actionName: 'longpress' } }));
    assert.deepEqual(h.routes, ['/create?tu=profile', '/moments/new', '/moments/new']);
    assert.equal(stamp(h).props.accessibilityActions[0].name, 'longpress');
    actual(h); await cleanedObserver(h, 5);
  });
});
