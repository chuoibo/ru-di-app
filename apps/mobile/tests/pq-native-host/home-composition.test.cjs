// Actual ExploreLive/PlaceLead graph with native and I/O boundaries only.
// React ancestry and props are not Android Yoga, glyph, clipping or fold evidence.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, act, sourceRoot, flatten } = require('./owner-host.cjs');
const emptyWire = require('./home-fixtures/catalogue-empty-facts.json');
const richWire = require('./home-fixtures/catalogue-rich-licensed.json');
const unlicensedWire = require('./home-fixtures/catalogue-unlicensed.json');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const plain = value => JSON.parse(JSON.stringify(value));
const placeId = 'synthetic-place-1';
const cityId = 'synthetic-city-1';
const fullName = 'Địa điểm tổng hợp kiểm thử — góc cà phê và xưởng thủ công có tên dài để kiểm cách xuống dòng';
const fullSub = 'Cà phê · Xưởng thủ công · 20 phút đi xe';
const cityName = 'Thành phố kiểm thử — khu phố sáng tạo có tên rất dài';
const fullReason = 'Lý do tổng hợp kiểm thử: có xưởng thủ công và mức chi trong dữ liệu kiểm thử.';
const fullCredit = 'Ảnh quanh đây: Tác giả tổng hợp kiểm thử · CC BY-SA 4.0';
const guUnknown = 'Chưa biết gu bạn nên chưa xếp hạng. Chọn sở thích ở Cá nhân để Rủ Đi xếp giúp.';
const longWindow = { width: 320, height: 640, fontScale: 1, scale: 2 };
function exactly(values, message) { assert.equal(values.length, 1, message); return values[0]; }
function inside(node, parent) { for (let at = node.parent; at; at = at.parent) if (at === parent) return true; return false; }
function nearest(node, type) { for (let at = node.parent; at; at = at.parent) if (at.type === type) return at; assert.fail(`Expected native ancestor ${type}`); }
function textValue(value) {
  if (value == null || typeof value === 'boolean') return '';
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  if (Array.isArray(value)) return value.map(textValue).join('');
  if (typeof value === 'object' && value.props) return textValue(value.props.children);
  return '';
}
const semantic = value => textValue(value).replace(/\u00a0/g, ' ').replace(/\u2060/g, '');
// Owner decision 2026-10-20: the «Minh hoạ» label is gone from every surface.
function noMinhHoa(node) { assert.deepEqual(node.findAllByType('RNText').filter(leaf => /minh\s*ho[aạ]/i.test(String(semantic(leaf.props.children)))).length, 0, 'no «Minh hoạ» label'); }
function textWithin(node, value) { return exactly(node.findAllByType('RNText').filter(leaf => semantic(leaf.props.children) === value), `one full native text: ${value}`); }
function uncapped(node, message, h) {
  assert.equal(node.props.numberOfLines, undefined, `${message}: no line cap`);
  assert.equal(node.props.maxFontSizeMultiplier, undefined, `${message}: no font cap`);
  if (node.props.allowFontScaling === false) {
    const wrapper = nearest(node, h.metrics.Text);
    const provider = nearest(node, h.metrics.NativeFontMetricsProvider);
    const snapshot = provider.props.snapshot;
    assert.equal(snapshot.status, 'available', `${message}: manual text has an actual usable owner`);
    assert.ok(snapshot.fontMetrics?.spToDp?.length, `${message}: no invented native conversion`);
    const original = flatten(wrapper.props.style), effective = flatten(node.props.style);
    for (const key of ['fontSize', 'lineHeight', 'letterSpacing']) {
      if (typeof original[key] !== 'number') continue;
      const entry = snapshot.fontMetrics.spToDp.find(item => item.base === original[key]);
      assert.ok(entry, `${message}: native table contains original ${key}`);
      assert.equal(effective[key], entry.dp, `${message}: manual ${key} is the exact native table value`);
    }
  }
  assert.notEqual(node.props.adjustsFontSizeToFit, true, `${message}: no shrink to fit`);
}
function lead(h) { return exactly(h.root.root.findAllByType(h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceLead), 'one actual PlaceLead owner'); }
function controller(h) { return exactly(h.root.root.findAllByType(h.load('apps/mobile/src/rudi/screens/explore/ExploreLive').ExploreLiveScreen), 'one actual ExploreLiveScreen owner'); }
function save(h, selected) {
  const action = h.one('RNPressable', props => props.accessibilityLabel === `${selected ? 'Bỏ lưu' : 'Lưu'} ${fullName}`);
  assert.equal(action.props.accessibilityState?.selected, selected, 'actual bookmark selection remains accessible');
  return action;
}
function composition(h, compact) {
  const owner = lead(h), open = exactly(owner.findAllByType('RNPressable').filter(node => node.props.accessibilityLabel === `Mở ${fullName}`), 'actual independent open-card action');
  assert.equal(owner.props.compact, compact, 'viewport selects the intended composition');
  const name = textWithin(owner, fullName), sub = textWithin(owner, fullSub);
  const content = exactly(open.findAllByType('RNView').filter(node => nearest(node, 'RNPressable') === open && !open.findAllByType('RNView').some(other => other !== node && inside(node, other))), 'one direct native card content');
  const views = content.findAllByType('RNView').filter(node => node !== content && nearest(node, 'RNView') === content);
  const mediaType = owner.props.dd.anh === null ? h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceArtwork : h.load('apps/mobile/src/rudi/ui/MediaSlot').MediaSlot;
  const media = exactly(owner.findAllByType(mediaType), 'actual card media owner');
  const mediaWrap = nearest(media, 'RNView');
  if (compact) {
    uncapped(name, 'compact title', h); uncapped(sub, 'compact subtitle', h);
    const titleRow = nearest(name, 'RNView'), header = nearest(sub, 'RNView');
    assert.notEqual(titleRow, header, 'subtitle receives the full header width beneath the title/heart row');
    assert.equal(nearest(titleRow, 'RNView'), header, 'name and subtitle share the same identity header');
    assert.equal(views[0], header, 'identity is before the media in the actual native tree');
    assert.equal(views[1], mediaWrap, 'media follows the complete identity group');
    return { owner, open, content, views, name, sub, header, titleRow, mediaWrap };
  }
  assert.equal(name.props.numberOfLines, 2, 'larger viewport retains the baseline title cap');
  const metadata = nearest(name, 'RNView');
  assert.equal(nearest(sub, 'RNView'), metadata, 'baseline title and subtitle share metadata');
  assert.equal(views[0], mediaWrap, 'baseline media remains first');
  assert.equal(views[1], metadata, 'baseline identity remains after media');
  return { owner, open, content, views, name, sub, metadata, mediaWrap };
}
function actual(h) {
  controller(h); lead(h);
  for (const name of [
    'screens/explore/ExploreLive', 'screens/explore/HangDiaDiem', 'kham-pha/dia-diem',
    'kham-pha/diem-den', 'ui/MediaSlot', 'ui/ghi-cong',
  ]) {
    const relative = `apps/mobile/src/rudi/${name}`, file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true, 'one strict complete source graph');
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
  const ui = 'apps/mobile/src/rudi/ui';
  assert.ok(h.consumed.get(ui), 'actual SearchField/IconButton/Chip module is consumed');
  assert.equal(h.consumedHashes.get(ui), crypto.createHash('sha256').update(fs.readFileSync(h.consumed.get(ui))).digest('hex'));
  const parser = 'apps/mobile/src/screens/kham-pha/places';
  assert.ok(h.consumed.get(parser), 'actual wire parser is consumed, not replaced');
  assert.equal(h.consumedHashes.get(parser), crypto.createHash('sha256').update(fs.readFileSync(h.consumed.get(parser))).digest('hex'));
  assert.equal(h.state.explicitProvider, false, 'default Home seam adds no external metrics provider');
  const actualOwner = h.load('apps/mobile/src/rudi/community/Comments').CommunityFontMetrics;
  assert.equal(h.root.root.findAllByType(actualOwner).length, 1, 'the actual Home surface owns native metrics');
  assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1, 'the provider comes from the actual surface owner');
  assert.equal(h.observation().subscriptions, 1, 'one native subscription across the complete actual Home graph');
}
async function withHome(options, run) {
  const h = createHost({ surface: 'explore', initialWindow: longWindow, exploreIO: { placeIds: [placeId], destinationIds: [cityId], saved: [] }, ...options });
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
async function loaded(h, wire = emptyWire) {
  assert.equal(h.exploreCatalogueReads.length, 1, 'initial catalogue read is deferred');
  await h.finish(h.exploreCatalogueReads[0], plain(wire));
  assert.equal(h.exploreSavedReads.length, 1); actual(h);
}
function callShape(h, call, method, route) {
  assert.equal(call.route, route); assert.deepEqual(plain(call.options), { method, actorId: h.state.actor.person_id });
}
async function press(node) { await act(async () => { node.props.onPress(); }); }
function allNativeText(owner) { return owner.findAllByType('RNText').map(node => semantic(node.props.children)).join('\n'); }
function noUnplanned(h) {
  assert.equal(h.calls.filter(call => !['exploreCatalogueRead', 'exploreSavedRead', 'exploreSavedWrite', 'exploreReasonRead'].includes(call.kind)).length, 0);
  assert.equal(h.calls.some(call => call.options.method === 'POST'), false, 'composition, local filter and bookmark actions cannot infer or post');
  assert.equal(h.uploads.length, 0); assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0);
}

test('HC01 compact Home identity survives same-viewport window font 100→200→100 uncapped', async () => {
  await withHome({}, async h => {
    await loaded(h);
    const first = composition(h, true), originalController = controller(h), reads = h.calls.length;
    assert.equal(first.views.length, 2, 'empty compact card has identity and media only');
    for (const fontScale of [2, 1]) {
      await h.update(state => { state.window = { ...state.window, fontScale }; });
      const next = composition(h, true);
      for (const key of ['owner', 'open', 'content', 'name', 'sub', 'header', 'titleRow', 'mediaWrap']) assert.equal(next[key], first[key], `same mounted ${key} through window font change`);
      assert.equal(controller(h), originalController);
      assert.equal(h.state.window.width, 320); assert.equal(h.state.window.height, 640);
      assert.equal(h.calls.length, reads, 'font-only update neither reloads nor calls inference');
    }
    actual(h); noUnplanned(h);
  });
});

test('HC02 larger Home viewports retain baseline below-media identity and real resize may switch composition', async () => {
  for (const window of [{ width: 320, height: 720, fontScale: 1, scale: 2 }, { width: 768, height: 1024, fontScale: 2, scale: 2 }]) {
    await withHome({ initialWindow: window }, async h => {
      await loaded(h); composition(h, false); noUnplanned(h);
    });
  }
  await withHome({}, async h => {
    await loaded(h); composition(h, true);
    await h.update(state => { state.window = { ...state.window, height: 720 }; });
    composition(h, false);
    await h.update(state => { state.window = { ...state.window, height: 640 }; });
    composition(h, true); noUnplanned(h);
  });
});

test('HC03 actual catalogue failure has no invented lead and its retry reads then parses real wire', async () => {
  await withHome({}, async h => {
    const PlaceLead = h.load('apps/mobile/src/rudi/screens/explore/HangDiaDiem').PlaceLead;
    assert.equal(h.root.root.findAllByType(PlaceLead).length, 0, 'loading has no fake lead');
    callShape(h, h.exploreCatalogueReads[0], 'GET', '/places?limit=30');
    const error = 'Lỗi đọc tổng hợp kiểm thử';
    await h.fail(h.exploreCatalogueReads[0], new (h.load('apps/mobile/src/api').ApiError)(503, error));
    assert.equal(h.root.root.findAllByType(PlaceLead).length, 0, 'failure has no fallback catalogue fixture');
    textWithin(h.root.root, 'Những chỗ hay chưa hiện lên'); textWithin(h.root.root, error);
    await h.pressText('Thử lại');
    assert.equal(h.exploreCatalogueReads.length, 2, 'actual ErrorState action starts another read');
    callShape(h, h.exploreCatalogueReads[1], 'GET', '/places?limit=30');
    await h.finish(h.exploreCatalogueReads[1], plain(emptyWire));
    composition(h, true); actual(h); noUnplanned(h);
  });
});

test('HC04 actual bookmark PUT locks a retained callback, rolls back refusal and accepts a later retry', async () => {
  await withHome({}, async h => {
    await loaded(h);
    const retainedSave = save(h, false).props.onPress;
    await act(async () => { retainedSave(); });
    assert.equal(h.exploreSavedWrites.length, 1);
    callShape(h, h.exploreSavedWrites[0], 'PUT', `/people/me/saved-places/${placeId}`);
    const busy = save(h, true);
    assert.equal(busy.props.disabled, true); assert.equal(busy.props['aria-busy'], true);
    assert.equal(busy.findAllByType('RNActivityIndicator').length, 1);
    await act(async () => { retainedSave(); });
    assert.equal(h.exploreSavedWrites.length, 1, 'actual lock blocks the old callback while pending');
    const error = 'Lỗi lưu tổng hợp kiểm thử';
    await h.fail(h.exploreSavedWrites[0], new (h.load('apps/mobile/src/api').ApiError)(503, error));
    assert.equal(Boolean(save(h, false).props.disabled), false); assert.deepEqual(h.state.exploreSaved, []);
    assert.equal(textWithin(h.root.root, error).props.accessibilityLiveRegion, 'polite');
    await press(save(h, false)); assert.equal(h.exploreSavedWrites.length, 2);
    callShape(h, h.exploreSavedWrites[1], 'PUT', `/people/me/saved-places/${placeId}`);
    await h.finish(h.exploreSavedWrites[1], {});
    assert.equal(Boolean(save(h, true).props.disabled), false); assert.deepEqual(h.state.exploreSaved, [placeId]);
    assert.deepEqual(h.routes, []); composition(h, true); noUnplanned(h);
  });
});

test('HC05 actual bookmark DELETE is independent from card open and failure retains saved state', async () => {
  await withHome({ exploreIO: { placeIds: [placeId], destinationIds: [cityId], saved: [placeId] } }, async h => {
    await loaded(h);
    const first = composition(h, true), selected = save(h, true), retainedDelete = selected.props.onPress;
    assert.equal(inside(selected, first.open), false, 'save is a sibling control, not inside open-card');
    await act(async () => { retainedDelete(); });
    assert.equal(h.exploreSavedWrites.length, 1);
    callShape(h, h.exploreSavedWrites[0], 'DELETE', `/people/me/saved-places/${placeId}`);
    assert.equal(save(h, false).props.disabled, true);
    await act(async () => { retainedDelete(); }); assert.equal(h.exploreSavedWrites.length, 1);
    await press(composition(h, true).open);
    assert.deepEqual(h.routes, [`/places/${placeId}`], 'actual open remains independent while bookmark is pending');
    await h.fail(h.exploreSavedWrites[0], new (h.load('apps/mobile/src/api').ApiError)(503, 'Lỗi bỏ lưu tổng hợp kiểm thử'));
    assert.equal(Boolean(save(h, true).props.disabled), false); assert.deepEqual(h.state.exploreSaved, [placeId]);
    await press(save(h, true)); assert.equal(h.exploreSavedWrites.length, 2);
    callShape(h, h.exploreSavedWrites[1], 'DELETE', `/people/me/saved-places/${placeId}`);
    await h.finish(h.exploreSavedWrites[1], {});
    assert.equal(Boolean(save(h, false).props.disabled), false); assert.deepEqual(h.state.exploreSaved, []); noUnplanned(h);
  });
});

test('HC06 actual licensed media keeps complete facts and credit through native error and window font changes', async () => {
  await withHome({}, async h => {
    await loaded(h, richWire);
    const parts = composition(h, true);
    assert.equal(parts.views.length, 3, 'rich compact facts retain their own metadata after the media');
    const facts = allNativeText(parts.owner);
    for (const text of ['4.6 (156) · 1.6 km', 'Đang mở, 07:00 - 22:00', '180.000đ – 230.000đ mỗi người', fullReason]) assert.equal(facts.includes(text), true, `complete semantic fact: ${text}`);
    const reason = textWithin(parts.owner, fullReason), price = textWithin(parts.owner, '180.000đ – 230.000đ mỗi người');
    uncapped(reason, 'grounded reason', h); uncapped(price, 'whole price with unit', h);
    for (const node of parts.owner.findAllByType('RNText').filter(node => semantic(node.props.children).includes('4.6 (156)') || semantic(node.props.children).includes('07:00 - 22:00'))) uncapped(node, 'complete fact line', h);
    const MediaSlot = h.load('apps/mobile/src/rudi/ui/MediaSlot').MediaSlot;
    const media = exactly(parts.owner.findAllByType(MediaSlot), 'actual licensed MediaSlot');
    const expectedUri = 'https://synthetic.invalid/places/synthetic-place-1/photos/synthetic-photo-1';
    assert.deepEqual(plain(media.props.nguon.anh.ve()), { source: { uri: expectedUri }, ghiCong: fullCredit });
    const image = exactly(media.findAllByType('RNExpoImage'), 'actual venue ExpoImage host');
    assert.deepEqual(plain(image.props.source), { uri: expectedUri }); textWithin(media, fullCredit);
    await act(async () => { image.props.onError(); });
    noMinhHoa(media); textWithin(media, 'Chưa tải được ảnh'); textWithin(media, fullCredit);
    assert.equal(media.findAllByType('RNExpoImage').filter(node => node.props.source?.uri === expectedUri).length, 0);
    for (const fontScale of [2, 1]) {
      await h.update(state => { state.window = { ...state.window, fontScale }; });
      const current = exactly(lead(h).findAllByType(MediaSlot), 'same actual MediaSlot after font rerender');
      assert.equal(current, media, 'unchanged URL preserves the failed media owner');
      noMinhHoa(current); textWithin(current, 'Chưa tải được ảnh'); textWithin(current, fullCredit);
      assert.equal(current.findAllByType('RNExpoImage').filter(node => node.props.source?.uri === expectedUri).length, 0, 'a rebuilt source object cannot erase the native error');
      composition(h, true);
    }
    actual(h); noUnplanned(h);
  });
});

test('HC07 compact empty facts leave no metadata slot while actual city search categories and taste controls remain', async () => {
  await withHome({}, async h => {
    await loaded(h);
    const parts = composition(h, true), copy = allNativeText(parts.owner);
    assert.equal(parts.views.length, 2, 'empty card has only complete identity and media; no empty metadata frame');
    for (const absent of ['Chưa có giá', 'Chưa có giờ mở cửa', 'AI MATCH', 'Lý do tổng hợp', '4.6 (156)', '180.000đ']) assert.equal(copy.includes(absent), false, `no fabricated card fact: ${absent}`);
    const AssetIllustration = h.load('apps/mobile/src/rudi/asset-pack/react-native/AssetIllustration').AssetIllustration;
    const cityStage = () => h.root.root.findAllByType(AssetIllustration).filter(node => node.props.name === 'scene-hills');
    assert.equal(cityStage().length, 1); assert.equal(cityStage()[0].props.height, 80, 'compact artwork prop only; no pixel/fold claim');
    const location = h.one('RNPressable', props => props.accessibilityLabel === 'Đổi điểm đến');
    assert.equal(location.findAllByType('RNText').some(node => semantic(node.props.children) === `${cityName} · đổi nơi khác`), true, 'full response destination remains present');
    assert.equal(h.input().props.value, ''); h.one('RNPressable', props => props.accessibilityLabel === 'Hỏi Rủ Đi AI');
    textWithin(h.root.root, `Chỗ hay ở ${cityName}`); textWithin(h.root.root, guUnknown);
    await h.change('kiểm thử'); assert.equal(cityStage().length, 0); lead(h);
    await h.change(''); assert.equal(cityStage().length, 1);
    await h.pressText('Cà phê'); assert.equal(cityStage().length, 0); lead(h);
    await h.pressText('Cà phê'); assert.equal(cityStage().length, 1);
    assert.equal(h.exploreCatalogueReads.length, 1, 'local filters do not reload the catalogue');
    await h.pressAccessibility('Đổi điểm đến'); await h.pressText(guUnknown);
    assert.deepEqual(h.routes, ['/destinations', '/personalization']); noUnplanned(h); actual(h);
  });
});

test('HC08 missing license chooses actual fallback and the Explore boundary refuses unplanned transport and storage', async () => {
  await withHome({}, async h => {
    await loaded(h, unlicensedWire);
    const owner = lead(h);
    assert.equal(owner.props.dd.anh, null, 'actual catalogue adapter refuses unlicensed image');
    noMinhHoa(owner);
    assert.equal(owner.findAllByType('RNExpoImage').some(node => typeof node.props.source?.uri === 'string' && node.props.source.uri.includes('/photos/')), false);
    const api = h.load('apps/mobile/src/api'), actorId = h.state.actor.person_id;
    const before = h.calls.length;
    noUnplanned(h);
    for (const [route, options, expected] of [
      ['/places?unplanned=1', { method: 'GET', actorId }, /Unexpected synthetic Explore transport/],
      ['/places', { method: 'GET', actorId }, /Unexpected synthetic Explore transport/], // an unpaged read is no longer planned
      ['/places?limit=30', { method: 'POST', actorId }, /Unexpected synthetic Explore transport/],
      ['/people/me/saved-places/synthetic-place-unknown', { method: 'PUT', actorId }, /Unexpected synthetic Explore transport/],
      ['/places?limit=30', { method: 'GET', actorId: 'synthetic-other' }, /actual supplied session actor/],
      ['/places?limit=30', { method: 'GET', actorId, body: {} }, /No unplanned query context/],
    ]) await assert.rejects(api.translatedAsActor({}, route, options), expected);
    await assert.rejects(api.translatedAnonymous({}, '/places', { method: 'GET' }), /Unexpected anonymous transport/);
    assert.equal(h.calls.length, before + 7, 'only the intentional negative-boundary calls are added');
    assert.equal(h.exploreCatalogueReads.length, 1); assert.equal(h.exploreSavedWrites.length, 0);
    const storage = h.exploreStorageBoundary;
    assert.ok(storage);
    await assert.rejects(storage.getItem('synthetic-unknown-key'));
    await assert.rejects(storage.removeItem('synthetic-unknown-key'));
    await assert.rejects(storage.setItem('rudi.diem-den.v1', cityId), /Unexpected destination write/);
    assert.equal(h.exploreStorage.some(call => call.method !== 'getItem' || call.key !== 'rudi.diem-den.v1'), false, 'negative storage probes cannot mutate the fixture');
    assert.equal(h.state.exploreStoredDestination, null);
    await assert.rejects(h.exploreFetchBoundary('https://synthetic.invalid/v2/community/media', {}), /Unexpected synthetic Explore fetch/);
    assert.equal(h.uploads.length, 0); assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0); assert.deepEqual(h.routes, []);
    actual(h);
  });
});
