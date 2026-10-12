// Actual Home + actual shared header/Field/cards/metrics owner, native boundaries only.
// The header callback is a fixture seam; the route JSX is inspected, not executed.
// React hosts and native-table values are not Android Yoga, glyph or caret proof.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./owner-host.cjs');
const richWire = require('./home-fixtures/catalogue-rich-licensed.json');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const plain = value => JSON.parse(JSON.stringify(value));
const ids = ['synthetic-place-1', 'synthetic-place-2', 'synthetic-place-3', 'synthetic-place-4'];
const window100 = { width: 320, height: 640, fontScale: 1, scale: 2 };
const city = richWire.destination.name;
const name = richWire.places[0].name;
const subtitle = 'Cà phê · Xưởng thủ công · 20 phút đi xe';
const reason = richWire.places[0].match.reason;
const price = '180.000đ – 230.000đ mỗi người';
const rating = '4.6 (156) · 1.6 km';
const hours = 'Đang mở, 07:00 - 22:00';
const gu = 'Chưa biết gu bạn nên chưa xếp hạng. Chọn sở thích ở Cá nhân để Rủ Đi xếp giúp.';
const placeholder = 'Một món thèm, một nơi muốn ghé…';
function wire() {
  const result = plain(richWire);
  result.places = ids.map((id, index) => ({ ...plain(richWire.places[0]), id, name: index === 0 ? name : `${name} · ${index + 1}`, photo_url: `/places/${id}/photos/synthetic-photo-${index + 1}` }));
  return result;
}
function text(value) {
  if (value == null || typeof value === 'boolean') return '';
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  if (Array.isArray(value)) return value.map(text).join('');
  return value.props ? text(value.props.children) : '';
}
const semantic = value => text(value).replace(/\u00a0/g, ' ').replace(/\u2060/g, '');
function one(nodes, message) { assert.equal(nodes.length, 1, message); return nodes[0]; }
function leaf(owner, copy) { return one(owner.findAllByType('RNText').filter(node => semantic(node.props.children) === copy), `exact complete copy: ${copy}`); }
function ancestor(node, type) { for (let at = node.parent; at; at = at.parent) if (at.type === type) return at; assert.fail(`actual ancestor ${String(type)}`); }
function within(node, parent) { for (let at = node.parent; at; at = at.parent) if (at === parent) return true; return false; }
function component(h, relative, exported) { return one(h.root.root.findAllByType(h.load(relative)[exported]), `one actual ${exported}`); }
function owner(h) { return component(h, 'apps/mobile/src/rudi/screens/explore/ExploreLive', 'ExploreLiveScreen'); }
function lead(h) { return component(h, 'apps/mobile/src/rudi/screens/explore/HangDiaDiem', 'PlaceLead'); }
function header(h) { return component(h, 'apps/mobile/src/rudi/ui/DauKhamPha', 'DauKhamPha'); }
function compare(h) { return component(h, 'apps/mobile/src/rudi/screens/explore/HangDiaDiem', 'PlaceCompare'); }
function converted(h, node, original, kind = 'text') {
  assert.equal(node.props.allowFontScaling, false, `${kind} uses explicit native-table metrics`);
  ancestor(node, kind === 'input' ? h.metrics.TextInput : h.metrics.Text);
  const provider = ancestor(node, h.metrics.NativeFontMetricsProvider);
  assert.equal(provider.props.snapshot.status, 'available');
  const table = h.state.nativeFont.fontMetrics.spToDp, effective = flatten(node.props.style);
  for (const [key, base] of Object.entries(original)) {
    const row = one(table.filter(item => item.base === base), `one supplied native conversion for ${base}`);
    assert.equal(effective[key], row.dp, `${kind} ${key} comes from exact nonlinear native table`);
  }
  assert.equal(node.props.maxFontSizeMultiplier, undefined, 'no system-font cap');
  assert.notEqual(node.props.adjustsFontSizeToFit, true, 'no shrink-to-fit');
}
function raw(node, size, lineHeight) {
  assert.notEqual(node.props.allowFontScaling, false, 'fallback/default keeps RN scaling');
  const style = flatten(node.props.style); assert.equal(style.fontSize, size); assert.equal(style.lineHeight, lineHeight);
}
function consumed(h) {
  for (const relative of [
    'apps/mobile/src/rudi/screens/explore/ExploreLive', 'apps/mobile/src/rudi/screens/explore/HangDiaDiem',
    'apps/mobile/src/rudi/community/Comments', 'apps/mobile/src/rudi/ui/NativeFontMetrics', 'apps/mobile/src/rudi/ui/Field',
    'apps/mobile/src/rudi/ui/DauKhamPha', 'apps/mobile/src/rudi/ui', 'apps/mobile/src/rudi/chia-bill/useBillFontScale',
    'apps/mobile/modules/rudi-font-scale', 'apps/mobile/src/rudi/kham-pha/dia-diem', 'apps/mobile/src/screens/kham-pha/places',
  ]) {
    const file = h.consumed.get(relative); assert.ok(file, `actual module consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true, 'complete frozen graph only');
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function counters(h) { return { calls: h.calls.length, storage: h.exploreStorage.length, attempts: h.attemptCount(), routes: h.routes.length, nativeCalls: h.nativeHostCalls.length }; }
function noFontIO(h, before) { assert.deepEqual(counters(h), before, 'metrics-only event cannot read/write/navigate/create attempts or imperative native calls'); }
function saved(h, selected = false, placeName = name) { return h.one('RNPressable', props => props.accessibilityLabel === `${selected ? 'Bỏ lưu' : 'Lưu'} ${placeName}`); }
function category(h) { return one(h.hosts('RNPressable').filter(node => node.findAllByType('RNText').some(child => semantic(child.props.children) === 'Cà phê')), 'one actual category action'); }
async function press(node) { await act(async () => node.props.onPress()); }
async function home(options, run) {
  const h = createHost({ surface: 'explore', initialWindow: window100, nativeGeometry: true, exploreIO: { placeIds: ids, destinationIds: ['synthetic-city-1'], saved: [] }, ...options });
  // This is the explicit header slot, not an application component stub.
  // The real route's equivalent opt-in is checked separately in HC09.
  const Header = h.load('apps/mobile/src/rudi/ui/DauKhamPha').DauKhamPha;
  h.state.exploreProps = { ...h.state.exploreProps, dau: phai => h.React.createElement(Header, { nativeMetrics: true, muc: 'explore', onDoiMuc() {}, phai }) };
  try { await h.mount(); await h.finish(h.exploreCatalogueReads[0], wire()); await run(h); } finally { await h.dispose(); }
}

test('HC09 actual Home owner converts header destination cards and search through stale-window 100→200→100 without replacing query or refs', async () => {
  // Source wiring guard only: this test does not execute the Expo route.
  const route = fs.readFileSync(path.join(sourceRoot, 'apps/mobile/app/(tabs)/explore.tsx'), 'utf8');
  assert.match(route, /<DauKhamPha\s+nativeMetrics\s+muc="explore"/);
  assert.match(route, /<ExploreLiveScreen\s+dau=\{dau\}\s+phien=\{phien\}/);
  await home({}, async h => {
    const initialOwner = owner(h), input = h.input(), inputRef = h.native.node(input), initialHeader = header(h), firstLead = lead(h);
    assert.equal(h.root.root.findAllByType(h.load('apps/mobile/src/rudi/community/Comments').CommunityFontMetrics).length, 1);
    assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1);
    assert.equal(within(initialHeader, h.root.root.findByType(h.metrics.NativeFontMetricsProvider)), true, 'route-provided header renders beneath actual owner');
    converted(h, leaf(initialHeader, 'Địa điểm'), { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
    converted(h, input, { fontSize: 17, lineHeight: 24 }, 'input');
    const hint = leaf(h.root.root, placeholder); assert.equal(hint.props.allowFontScaling, false); assert.equal(flatten(hint.props.style).fontSize, 17);
    await h.native.focus(input); await h.change('kiểm thử');
    const firstName = leaf(firstLead, name), firstSub = leaf(firstLead, subtitle), before = counters(h);
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale));
      assert.equal(h.state.window.fontScale, 1, 'window fontScale stays stale at 100%');
      assert.equal(owner(h) === initialOwner, true, "actual Home owner identity remains stable"); assert.equal(header(h) === initialHeader, true, "actual header identity remains stable"); assert.equal(lead(h) === firstLead, true, "actual lead identity remains stable");
      assert.equal(h.input() === input, true, "actual search input identity remains stable"); assert.equal(h.native.node(h.input()) === inputRef, true, "actual input native ref remains stable"); assert.equal(inputRef.isFocused(), true);
      assert.equal(h.input().props.value, 'kiểm thử'); assert.equal(leaf(firstLead, name) === firstName, true, "actual lead name host remains stable"); assert.equal(leaf(firstLead, subtitle) === firstSub, true, "actual lead subtitle host remains stable");
      converted(h, leaf(header(h), 'Địa điểm'), { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
      converted(h, leaf(header(h), 'Cộng đồng'), { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
      converted(h, h.input(), { fontSize: 17, lineHeight: 24 }, 'input');
      converted(h, leaf(h.root.root, `${city} · đổi nơi khác`), { fontSize: 14, lineHeight: 19 });
      converted(h, firstName, { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
      converted(h, firstSub, { fontSize: 17, lineHeight: 24 });
      converted(h, leaf(h.root.root, '4 kết quả'), { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
      converted(h, leaf(h.root.root, gu), { fontSize: 13, lineHeight: 18 });
      noFontIO(h, before); assert.equal(h.observation().subscriptions, 1); assert.equal(h.observation().live, 1); assert.equal(h.observation().maxLive, 1);
    }
    consumed(h);
  });
});

test('HC10 effective metrics preserve local filters and an in-flight actual save lock payload and retry without font I/O', async () => {
  await home({}, async h => {
    await h.change('kiểm thử'); await press(category(h)); assert.equal(category(h).props['aria-pressed'], true);
    const retained = saved(h).props.onPress; await act(async () => retained());
    const pending = h.exploreSavedWrites[0]; assert.ok(pending); assert.equal(pending.route, '/people/me/saved-places/synthetic-place-1');
    assert.deepEqual(plain(pending.options), { method: 'PUT', actorId: 'synthetic-A' }, 'actual save has no invented attempt field');
    const before = counters(h), input = h.input(), content = owner(h);
    await h.nativeFont(snapshot(2)); noFontIO(h, before);
    converted(h, input, { fontSize: 17, lineHeight: 24 }, 'input'); assert.equal(h.input() === input, true, "actual search input identity remains stable"); assert.equal(owner(h) === content, true, "actual Home owner survives the pending save");
    assert.equal(input.props.value, 'kiểm thử'); assert.equal(category(h).props['aria-pressed'], true); assert.equal(saved(h, true).props.disabled, true);
    await act(async () => retained()); assert.equal(h.exploreSavedWrites.length, 1, 'same retained callback still sees the synchronous lock');
    await h.fail(pending, new (h.load('apps/mobile/src/api').ApiError)(503, 'Lỗi lưu kiểm thử metrics'));
    assert.equal(Boolean(saved(h).props.disabled), false); assert.deepEqual(h.state.exploreSaved, []);
    await press(saved(h)); assert.equal(h.exploreSavedWrites.length, 2); assert.deepEqual(plain(h.exploreSavedWrites[1].options), plain(pending.options));
    await h.finish(h.exploreSavedWrites[1], {}); assert.deepEqual(h.state.exploreSaved, ['synthetic-place-1']);
    const settled = counters(h); await h.nativeFont(snapshot()); noFontIO(h, settled); assert.equal(category(h).props['aria-pressed'], true); assert.equal(h.input().props.value, 'kiểm thử');
    assert.equal(h.observation().subscriptions, 1); consumed(h);
  });
});

test('HC11 inherited unavailable and missing native tables retain RN fallback without child observations then recover on the same input', async () => {
  for (const unavailable of [{ status: 'unavailable', reason: 'module-inactive' }, { status: 'available', fontScale: 2 }, { status: 'available', fontScale: 2, fontMetrics: { density: 2, spToDp: [] } }]) {
    await home({ explicitProvider: true, inheritedExploreMetrics: true, initialSnapshot: unavailable }, async h => {
      const input = h.input(); raw(input, 17, 24); raw(leaf(header(h), 'Địa điểm'), 21, 27);
      assert.equal(compare(h).props.largeText, true, 'unusable owner chooses conservative layout without invented scale');
      assert.deepEqual(plain(h.observation()), { live: 0, subscriptions: 0, removals: 0, reads: 0, maxLive: 0 });
      assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1, 'only inherited provider exists');
      await h.change('kiểm thử'); const before = counters(h); await h.nativeFont(snapshot(2)); noFontIO(h, before);
      assert.equal(h.input() === input, true, "actual search input identity remains stable"); assert.equal(input.props.value, 'kiểm thử'); converted(h, input, { fontSize: 17, lineHeight: 24 }, 'input');
      converted(h, leaf(header(h), 'Địa điểm'), { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
      await h.nativeFont(unavailable); assert.equal(h.input() === input, true, "actual search input identity remains stable"); raw(input, 17, 24);
      assert.deepEqual(plain(h.observation()), { live: 0, subscriptions: 0, removals: 0, reads: 0, maxLive: 0 }); consumed(h);
    });
  }
});

test('HC12 shared Hang Dau and Field defaults keep their RN path under an actual metrics owner without child subscriptions', async () => {
  const dd = { id: 'synthetic-place-default', name: 'Địa điểm mặc định kiểm thử', sub: subtitle, facts: [{ icon: 'star', text: '4.6 (156)' }, { icon: 'navigate-outline', text: '1.6 km' }], glyph: 'cafe-outline', loai: 'cafe', anh: null, badge: null };
  const h = createHost({ surface: 'boundary', initialSnapshot: snapshot(2), initialWindow: { ...window100, width: 768, fontScale: 2 }, boundaryChildren: ({ React, load }) => {
    const kit = load('apps/mobile/src/rudi/ui'), hang = load('apps/mobile/src/rudi/screens/explore/HangDiaDiem'), Header = load('apps/mobile/src/rudi/ui/DauKhamPha').DauKhamPha;
    return React.createElement(React.Fragment, null,
      React.createElement(Header, { muc: 'community', onDoiMuc() {} }),
      React.createElement(kit.SearchField, { accessibilityLabel: 'HC default search', value: '', onChangeText() {} }),
      React.createElement(hang.PlaceLead, { dd, daLuu: false, compact: true, onOpen() {}, onSave() {} }),
      React.createElement(hang.PlaceCompare, { items: [dd, { ...dd, id: 'synthetic-place-default-2', name: 'Địa điểm mặc định thứ hai' }], daLuu: () => false, onOpen() {}, onSave() {} }));
  } });
  try {
    h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem'); // Import failures surface before React can retry a partial module.
    await h.mount(); raw(h.one('RNTextInput', props => props.accessibilityLabel === 'HC default search'), 17, 24);
    raw(leaf(header(h), 'Cộng đồng'), 21, 27); raw(leaf(lead(h), dd.name), 21, 27);
    assert.equal(compare(h).findAllByType(h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceRow).length, 2, 'default wide large-window branch remains the original rows');
    const scroll = one(header(h).findAllByType('RNScrollView'), 'actual header scroll');
    await act(async () => scroll.props.onLayout()); assert.equal(h.nativeHostCalls.at(-1).kind, 'scrollToEnd');
    assert.equal(h.observation().subscriptions, 1); assert.equal(h.observation().live, 1); assert.equal(h.calls.length, 0);
    assert.equal(h.exploreStorage.length, 0, 'pure shared card rendering cannot access destination storage');
  } finally { await h.dispose(); }
});

test('HC13 actual rating paragraph uses nonlinear native metrics with a sibling decorative star and preserves facts price and reason', async () => {
  await home({}, async h => {
    await h.nativeFont(snapshot(2)); const card = lead(h), fact = leaf(card, rating);
    converted(h, fact, { fontSize: 13, lineHeight: 18 }); assert.equal(fact.props.accessibilityLabel, '4.6\u00a0(156) · 1.6\u00a0km');
    assert.equal(fact.findAllByType('RNIonicons').length, 0, 'opaque inline icon cannot force factual paragraph to RN fallback');
    const line = ancestor(fact, 'RNView'), star = one(line.findAllByType('RNIonicons').filter(node => node.props.name === 'star'), 'one sibling decorative star');
    assert.equal(within(star, fact), false); assert.equal(star.props.accessible, false); assert.equal(star.props.importantForAccessibility, 'no');
    converted(h, leaf(card, hours), { fontSize: 13, lineHeight: 18 }); converted(h, leaf(card, price), { fontSize: 14, lineHeight: 19 }); converted(h, leaf(card, reason), { fontSize: 14, lineHeight: 19 });
    // The real licensed MediaSlot fallback owns the category artwork label.
    const image = one(card.findAllByType('RNExpoImage').filter(node => typeof node.props.source?.uri === 'string'), 'one real licensed image');
    await act(async () => image.props.onError());
    const artwork = one(card.findAllByType(h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceArtwork), 'actual category fallback');
    converted(h, leaf(artwork, 'Minh hoạ'), { fontSize: 13, lineHeight: 18 }); consumed(h);
  });
  // A distance-only paragraph must convert too; no star is fabricated.
  const h = createHost({ surface: 'boundary', initialSnapshot: snapshot(2), boundaryChildren: ({ React, load }) => {
    const Row = load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceRow;
    return React.createElement(Row, { nativeMetrics: true, largeText: true, dd: { id: 'synthetic-place-distance', name: 'Chỉ có khoảng cách', sub: '', facts: [{ icon: 'navigate-outline', text: '1.6 km' }], glyph: 'cafe-outline', loai: 'cafe', anh: null, badge: null }, daLuu: false, onOpen() {}, onSave() {} });
  } });
  try {
    h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem'); // Keep this native external import separate from the React render boundary.
    await h.mount(); const row = component(h, 'apps/mobile/src/rudi/screens/explore/HangDiaDiem', 'PlaceRow'), distance = leaf(row, '1.6 km');
    converted(h, distance, { fontSize: 13, lineHeight: 18 }); assert.equal(row.findAllByType('RNIonicons').filter(node => node.props.name === 'star').length, 0);
    assert.equal(h.exploreStorage.length, 0, 'distance-only rendering cannot access destination storage');
  } finally { await h.dispose(); }
});

test('HC14 opted-in wide Compare keeps keyed cards images hearts and content through native-only font changes while direction adapts', async () => {
  await home({ initialWindow: { ...window100, width: 768, height: 1024 } }, async h => {
    const pair = compare(h), frame = h.one('RNView', props => props.testID === 'explore-compare');
    assert.equal(flatten(frame.props.style).flexDirection, 'row');
    const rows = h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceRow; assert.equal(pair.findAllByType(rows).length, 0);
    const candidates = pair.props.items.map(item => ({ item, open: h.one('RNPressable', props => props.accessibilityLabel === `Mở ${item.name}`), heart: saved(h, false, item.name), title: leaf(pair, item.name), image: one(pair.findAllByType('RNExpoImage').filter(node => node.props.source?.uri?.includes(`/places/${item.id}/photos/`)), 'actual candidate photo') }));
    const before = counters(h);
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale)); assert.equal(h.state.window.fontScale, 1); assert.equal(compare(h) === pair, true, "actual wide Compare identity remains stable");
      assert.equal(flatten(frame.props.style).flexDirection, scale === 2 ? 'column' : 'row'); assert.equal(pair.findAllByType(rows).length, 0, 'large metrics keep the same wide card branch');
      for (const candidate of candidates) {
        assert.equal(h.one('RNPressable', props => props.accessibilityLabel === `Mở ${candidate.item.name}`) === candidate.open, true, "actual keyed open control remains stable");
        assert.equal(saved(h, false, candidate.item.name) === candidate.heart, true, "actual keyed heart remains stable"); assert.equal(leaf(pair, candidate.item.name) === candidate.title, true, "actual keyed title host remains stable");
        assert.equal(one(pair.findAllByType('RNExpoImage').filter(node => node.props.source?.uri?.includes(`/places/${candidate.item.id}/photos/`)), 'same candidate photo') === candidate.image, true, "actual keyed image host remains stable");
        assert.equal(candidate.title.props.numberOfLines, scale === 2 ? undefined : 2); converted(h, candidate.title, { fontSize: 17, lineHeight: 23, letterSpacing: -0.15 });
      }
      noFontIO(h, before); assert.equal(h.observation().subscriptions, 1);
    }
    consumed(h);
  });
});


test('HC15 actual phone PlaceRow keeps a self-contained right heart frame through native 100→200→100 without replacing controls or input', async () => {
  await home({}, async h => {
    const Row = h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceRow;
    const row = compare(h).findAllByType(Row)[0]; assert.ok(row, 'actual phone comparison row');
    const placeName = row.props.dd.name, heart = saved(h, false, placeName), placement = ancestor(heart, 'RNView');
    const title = leaf(row, placeName), input = h.input(), inputRef = h.native.node(input);
    await h.native.focus(input); await h.change('kiểm thử'); const before = counters(h);
    const check = large => {
      const style = flatten(placement.props.style), target = flatten(heart.props.style);
      assert.equal(style.position, 'absolute', 'heart placement stays outside row flow');
      assert.equal(Object.hasOwn(style, 'width'), false, 'the placement frame has no width competing with its horizontal anchor');
      assert.equal(Object.hasOwn(style, 'left'), !large, 'large placement must not inherit a left constraint, including an undefined override');
      assert.equal(Object.hasOwn(style, 'right'), large);
      assert.equal(style.left, large ? undefined : 36); assert.equal(style.right, large ? 10 : undefined);
      assert.equal(style.top, large ? 24 : 20); assert.equal(target.width, 48); assert.equal(target.height, 48);
      assert.equal(within(heart, row), true); assert.equal(within(heart, h.one('RNPressable', props => props.accessibilityLabel === `Mở ${placeName}`)), false, 'Save remains a sibling of Open');
    };
    check(false);
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale)); assert.equal(h.state.window.fontScale, 1, 'native owner, not stale window scale, chooses the row direction');
      assert.equal(compare(h).findAllByType(Row)[0] === row, true, 'actual row owner remains stable');
      assert.equal(saved(h, false, placeName) === heart, true, 'actual heart remains stable'); assert.equal(ancestor(heart, 'RNView') === placement, true, 'actual placement host remains stable');
      assert.equal(leaf(row, placeName) === title, true, 'actual title host remains stable'); assert.equal(h.input() === input, true); assert.equal(h.native.node(input) === inputRef, true);
      assert.equal(inputRef.isFocused(), true); assert.equal(input.props.value, 'kiểm thử'); check(scale === 2);
      converted(h, title, { fontSize: 17, lineHeight: 23, letterSpacing: -0.15 }); noFontIO(h, before);
    }
    assert.equal(h.observation().subscriptions, 1); consumed(h);
  });
});

test('HC16 actual wide comparison fallback fills its own ratio frame and retained raster hints cannot become parent layout through hot font return', async () => {
  await home({ initialWindow: { ...window100, width: 768, height: 1024 } }, async h => {
    const pair = compare(h), Media = h.load('apps/mobile/src/rudi/ui/MediaSlot').MediaSlot;
    const Artwork = h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceArtwork;
    const Illustration = h.load('apps/mobile/src/rudi/asset-pack/react-native/AssetIllustration').AssetIllustration;
    const slots = pair.findAllByType(Media); assert.equal(slots.length, 2);
    const fixtures = [];
    for (const slot of slots) {
      const licensed = one(slot.findAllByType('RNExpoImage').filter(node => typeof node.props.source?.uri === 'string'), 'actual licensed comparison image');
      await act(async () => licensed.props.onError());
      const artwork = one(slot.findAllByType(Artwork), 'actual failed-media artwork'), illustration = one(artwork.findAllByType(Illustration), 'actual raster illustration');
      const image = one(illustration.findAllByType('RNExpoImage'), 'actual local artwork image'), root = ancestor(image, 'RNView'), label = leaf(artwork, 'Minh hoạ'), labelBox = ancestor(label, 'RNView');
      const frame = one(slot.findAllByType('RNView').filter(node => flatten(node.props.style)?.aspectRatio !== undefined), 'actual MediaSlot ratio frame');
      fixtures.push({ slot, artwork, illustration, image, root, label, labelBox, frame, retainedLayout: root.props.onLayout });
    }
    const before = counters(h);
    const check = (fixture, scale) => {
      const { slot, artwork, illustration, image, root, label, labelBox, frame } = fixture;
      assert.equal(slot.findAllByType(Artwork)[0] === artwork, true, 'failed-media fallback controller survives font changes');
      assert.equal(artwork.findAllByType(Illustration)[0] === illustration, true); assert.equal(illustration.findAllByType('RNExpoImage')[0] === image, true); assert.equal(leaf(artwork, 'Minh hoạ') === label, true);
      assert.equal(slot.findAllByType('RNExpoImage').filter(node => typeof node.props.source?.uri === 'string').length, 0, 'font-only changes cannot reset actual failed-media state');
      const bounds = flatten(root.props.style), picture = flatten(image.props.style), media = flatten(frame.props.style), tag = flatten(labelBox.props.style);
      assert.equal(media.width, '100%'); assert.equal(media.aspectRatio, scale === 2 ? 2 : 16 / 7);
      assert.equal(bounds.position, 'absolute', 'fallback consumes the MediaSlot frame instead of adding an intrinsic fixed-height box');
      for (const edge of ['left', 'right', 'top', 'bottom']) { assert.equal(bounds[edge], 0); assert.equal(picture[edge], 0); }
      assert.equal(Object.hasOwn(bounds, 'width'), false); assert.equal(Object.hasOwn(bounds, 'height'), false);
      assert.equal(picture.position, 'absolute'); assert.equal(picture.width, '100%', 'numeric raster width cannot feed back into the comparison column'); assert.equal(picture.height, '100%', 'numeric raster height cannot leave bands or clip the label');
      assert.equal(tag.position, 'absolute'); assert.equal(tag.bottom, 8); assert.equal(tag.paddingVertical, 2); assert.equal(within(label, root), true); assert.equal(within(root, frame), true);
      converted(h, label, { fontSize: 13, lineHeight: 18 });
      assert.equal(Number.isFinite(illustration.props.width) && illustration.props.width > 0, true); assert.equal(Number.isFinite(illustration.props.height) && illustration.props.height > 0, true);
    };
    for (const scale of [1, 2, 1]) {
      if (scale !== 1 || flatten(fixtures[0].frame.props.style).aspectRatio === 2) await h.nativeFont(snapshot(scale));
      assert.equal(compare(h) === pair, true); assert.equal(h.state.window.fontScale, 1);
      for (const fixture of fixtures) {
        check(fixture, scale);
        // Native layout dimensions are an explicit boundary fixture, not a Yoga calculation.
        const width = scale === 2 ? 616 : 300, height = width / (scale === 2 ? 2 : 16 / 7);
        await act(async () => fixture.root.props.onLayout({ nativeEvent: { layout: { x: 0, y: 0, width, height } } }));
        assert.equal(fixture.illustration.props.width, width); assert.equal(fixture.illustration.props.height, height);
        const tag = flatten(fixture.labelBox.props.style), line = flatten(fixture.label.props.style).lineHeight;
        assert.equal(height - tag.bottom - 2 * tag.paddingVertical - line >= 0, true, 'the supplied native frame has room for one declared label line pitch');
        for (const invalid of [{ width: NaN, height }, { width: Infinity, height }, { width: 0, height }, { width, height: -1 }, { width, height: NaN }]) {
          await act(async () => fixture.root.props.onLayout({ nativeEvent: { layout: invalid } }));
          assert.equal(fixture.illustration.props.width, width); assert.equal(fixture.illustration.props.height, height);
        }
        check(fixture, scale);
      }
      noFontIO(h, before);
    }
    // A prior-render finite callback can change only raster hints, never native layout constraints.
    for (const fixture of fixtures) {
      await act(async () => fixture.retainedLayout({ nativeEvent: { layout: { width: 616, height: 308 } } }));
      assert.equal(fixture.illustration.props.width, 616); assert.equal(fixture.illustration.props.height, 308); check(fixture, 1);
    }
    noFontIO(h, before); assert.equal(h.observation().subscriptions, 1); consumed(h);
  });
  const h = createHost({ surface: 'boundary', initialSnapshot: snapshot(2), boundaryChildren: ({ React, load }) => React.createElement(load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceArtwork, { loai: 'cafe' }) });
  try {
    h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem'); await h.mount();
    const artwork = component(h, 'apps/mobile/src/rudi/screens/explore/HangDiaDiem', 'PlaceArtwork'), image = one(artwork.findAllByType('RNExpoImage'), 'actual default artwork image'), frame = ancestor(image, 'RNView');
    assert.equal(flatten(frame.props.style).width, '100%'); assert.equal(flatten(frame.props.style).height, 180); assert.equal(flatten(frame.props.style).position, undefined);
    await act(async () => frame.props.onLayout({ nativeEvent: { layout: { width: 300, height: 131.25 } } }));
    assert.equal(flatten(image.props.style).width, 300); assert.equal(flatten(image.props.style).height, 180); assert.equal(flatten(image.props.style).position, undefined);
    raw(leaf(artwork, 'Minh hoạ'), 13, 18); assert.equal(h.exploreStorage.length, 0);
  } finally { await h.dispose(); }
});
