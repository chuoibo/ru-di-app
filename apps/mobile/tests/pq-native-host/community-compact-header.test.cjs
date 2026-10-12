// Actual Community route, primitives and owner; native events are explicit boundaries.
// Effective styles and callback targets do not establish Android Yoga or glyph fit.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./owner-host.cjs');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const unavailable = () => ({ status: 'unavailable', reason: 'module-inactive' });
const window100 = { width: 320, height: 640, fontScale: 1, scale: 2 };
const notification = { id: 'synthetic-notification-1', post_id: 'synthetic-post-1', kind: 'mention', created_at: '2026-10-02T00:00:00Z' };
const emptyPage = () => ({ posts: [], next_cursor: null, personalized: false });
function one(nodes, message) { assert.equal(nodes.length, 1, message); return nodes[0]; }
function inside(node, parent) { for (let at = node.parent; at; at = at.parent) if (at === parent) return true; return false; }
function ancestor(node, type) { for (let at = node.parent; at; at = at.parent) if (at.type === type) return at; assert.fail('actual ancestor ' + String(type)); }
function style(node) { return flatten(typeof node.props.style === 'function' ? node.props.style({ pressed: false }) : node.props.style); }
function copy(parent, value) { return one(parent.findAllByType('RNText').filter(node => node.props.children === value), 'one complete actual copy: ' + value); }
function input(h) { return h.one('RNTextInput', props => props.accessibilityLabel === 'Tìm chủ đề'); }
function component(h, relative, name) { return one(h.root.root.findAllByType(h.load(relative)[name]), 'one actual ' + name); }
function settings(h) { return one(h.root.root.findAllByType(h.load('apps/mobile/src/rudi/ui/Sheet').Sheet).filter(node => node.props.accessibilityLabel === 'Bảng tin của bạn'), 'actual settings Sheet'); }
function options(h) { return one(h.hosts('RNPressable', props => /^(Tuỳ chọn Cộng đồng(, có thông báo mới)?|Cài đặt bảng tin)$/.test(props.accessibilityLabel)), 'actual options action in either header generation'); }
function action(parent, label) { return one(parent.findAllByType('RNPressable').filter(node => node.props.accessibilityLabel === label || node.findAllByType('RNText').some(leaf => leaf.props.children === label)), 'actual semantic action: ' + label); }
async function press(node) { assert.notEqual(node.props.disabled, true); await act(async () => { node.props.onPress(); }); }
function observation(h, removed = 0) {
  const value = h.observation();
  assert.deepEqual({ live: value.live, subscriptions: value.subscriptions, removals: value.removals, maxLive: value.maxLive }, { live: 1 - removed, subscriptions: 1, removals: removed, maxLive: 1 });
  if (!removed) assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1, 'one existing Community metrics owner');
}
function metrics(h, node, base) {
  const usable = h.state.nativeFont.status === 'available';
  assert.equal(node.props.allowFontScaling, usable ? false : undefined);
  const effective = style(node);
  for (const [key, value] of Object.entries(base)) {
    const expected = usable ? one(h.state.nativeFont.fontMetrics.spToDp.filter(row => row.base === value), 'native metric row').dp : value;
    assert.equal(effective[key], expected, 'actual native-table ' + key);
  }
  assert.equal(node.props.maxFontSizeMultiplier, undefined); assert.notEqual(node.props.adjustsFontSizeToFit, true);
}
function raw(node, base) { assert.equal(node.props.allowFontScaling, undefined); for (const [key, value] of Object.entries(base)) assert.equal(style(node)[key], value); }
function actual(h, route = true) {
  const names = ['apps/mobile/src/rudi/community/CommunityScreen', 'apps/mobile/src/rudi/community/api', 'apps/mobile/src/rudi/community/bang-tin', 'apps/mobile/src/rudi/community/useCommunityStream', 'apps/mobile/src/rudi/community/useViewSignal', 'apps/mobile/src/rudi/community/Comments', 'apps/mobile/src/rudi/ui', 'apps/mobile/src/rudi/ui/DauKhamPha', 'apps/mobile/src/rudi/ui/HangChuTab', 'apps/mobile/src/rudi/ui/Field', 'apps/mobile/src/rudi/ui/Sheet', 'apps/mobile/src/rudi/ui/NativeFontMetrics', 'apps/mobile/src/rudi/session'];
  if (route) names.push('apps/mobile/app/(tabs)/community');
  for (const relative of names) {
    const file = h.consumed.get(relative); assert.ok(file, 'actual module consumed: ' + relative);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function counters(h) { return { calls: h.calls.length, storage: h.community.storageCalls.length, attempts: h.attemptCount(), routes: h.routes.length }; }
async function community(view, run) {
  const h = createHost({ surface: 'community', communityView: view, nativeGeometry: true, initialWindow: window100, communityIO: { notifications: [notification], preferences: { asked: true, personalized: false }, seen: null } });
  try { await h.mount(); if (view !== 'defaults') { assert.equal(h.community.feedReads.length, 1); await h.finish(h.community.feedReads[0], emptyPage()); } await run(h); }
  finally { await h.dispose(); h.community.assertClean(); assert.equal(Object.values(h.community.listeners).every(entries => entries.size === 0), true, 'actual listeners clean up'); assert.equal(h.sockets.every(socket => socket.closed), true); observation(h, 1); }
}

test('CH01 actual tab keeps one inline overflow and selected section callbacks on retained search through hot metrics', async () => {
  await community('tab', async h => {
    assert.equal(h.sockets.length, 1, 'actual stream creates one synthetic native socket');
    assert.equal(h.sockets[0].url, 'wss://synthetic.invalid/v2/community/stream');
    await act(async () => h.sockets[0].onopen());
    assert.equal(h.nativeHostCalls.filter(call => typeof call.socket === 'string').length, 1, 'actual stream authenticates in its frame through the closed boundary');
    const root = h.one('RNView', props => props.testID === 'dau-kham-pha');
    assert.equal(style(root).paddingTop, undefined, 'CH01 section words share the row instead of reserving a separate 48dp action row');
    assert.equal(style(root).flexDirection, 'row'); assert.equal(style(root).gap, 8);
    const header = component(h, 'apps/mobile/src/rudi/ui/DauKhamPha', 'DauKhamPha');
    assert.equal(header.props.inlineActions, true); assert.equal(header.props.nativeMetrics, true);
    const overflow = options(h), overflowRef = h.native.node(overflow), right = ancestor(overflow, 'RNView');
    assert.equal(style(overflow).width, 48); assert.equal(style(overflow).height, 48);
    assert.equal(style(right).position, undefined); assert.equal(style(right).flexShrink, 0);
    assert.equal(root.findAllByType('RNPressable').length, 3, 'two real section tabs and exactly one overflow target');
    const scroll = one(root.findAllByType('RNScrollView'), 'actual section scroll'), scrollRef = h.native.node(scroll);
    assert.equal(style(scroll).flexBasis, 0); assert.equal(style(scroll).minWidth, 0);
    const active = action(root, 'Cộng đồng'), inactive = action(root, 'Địa điểm');
    assert.equal(active.props.accessibilityRole, 'tab'); assert.equal(inactive.props.accessibilityRole, 'tab');
    assert.equal(active.props['aria-selected'], true); assert.equal(inactive.props['aria-selected'], false);
    const activeText = copy(active, 'Cộng đồng'), inactiveText = copy(inactive, 'Địa điểm');
    const tabs = component(h, 'apps/mobile/src/rudi/ui/HangChuTab', 'HangChuTab');
    assert.equal(tabs.props.nativeMetrics, true);
    const feedText = copy(tabs, 'Dành cho bạn'), search = input(h), searchRef = h.native.node(search);
    const owner = h.owner('CommunityScreenContent'), fontOwner = h.owner('CommunityFontMetrics');
    const hint = copy(h.root.root, 'Đi đâu, ăn gì, trải nghiệm gì?'); metrics(h, hint, { fontSize: 17, lineHeight: 24 });
    await h.native.focus(search); await act(async () => search.props.onChangeText('QC community query'));
    const before = counters(h);
    for (const value of [snapshot(), snapshot(2), unavailable(), snapshot()]) {
      await h.nativeFont(value); observation(h);
      assert.equal(h.state.window.width, 320); assert.equal(h.state.window.fontScale, 1, 'native metrics change while window remains stale');
      assert.equal(h.owner('CommunityScreenContent') === owner, true); assert.equal(h.owner('CommunityFontMetrics') === fontOwner, true);
      assert.equal(component(h, 'apps/mobile/src/rudi/ui/DauKhamPha', 'DauKhamPha') === header, true);
      assert.equal(component(h, 'apps/mobile/src/rudi/ui/HangChuTab', 'HangChuTab') === tabs, true);
      assert.equal(input(h) === search, true); assert.equal(h.native.node(search) === searchRef, true); assert.equal(searchRef.isFocused(), true);
      assert.equal(search.props.value, 'QC community query'); metrics(h, search, { fontSize: 17, lineHeight: 24 });
      assert.equal(copy(active, 'Cộng đồng') === activeText, true); assert.equal(copy(inactive, 'Địa điểm') === inactiveText, true); assert.equal(copy(tabs, 'Dành cho bạn') === feedText, true);
      metrics(h, activeText, { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 }); metrics(h, inactiveText, { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 }); metrics(h, feedText, { fontSize: 14, lineHeight: 19 });
      assert.equal(h.native.node(options(h)) === overflowRef, true); assert.deepEqual(counters(h), before, 'font-only change cannot dispatch I/O, attempts or navigation');
      const offset = h.nativeHostCalls.length;
      await act(async () => { scroll.props.onLayout({ nativeEvent: { layout: { width: 232, height: 64 } } }); scroll.props.onContentSizeChange(400, 64); });
      const calls = h.nativeHostCalls.slice(offset);
      assert.equal(calls.length, 2, 'both actual selected-section callbacks issue their own request');
      for (const call of calls) { assert.equal(call.kind, 'scrollToEnd'); assert.equal(call.target, scrollRef.id); assert.equal(call.options.animated, false); assert.deepEqual(Object.keys(call.options), ['animated']); }
    }
    await press(active); assert.equal(h.routes.length, 0, 'active section does not navigate');
    await press(inactive); assert.equal(h.community.navigationCalls.at(-1).method, 'navigate'); assert.equal(h.routes.at(-1), '/explore');
    await act(async () => search.props.onSubmitEditing());
    assert.deepEqual(JSON.parse(JSON.stringify(h.routes.at(-1))), { pathname: '/community/search', params: { q: 'QC community query' } });
    assert.equal(h.load('apps/mobile/src/rudi/ui/thanh-tab').dichCuaCot('explore'), 'community', 'actual route focus remembers the Community section');
    actual(h);
  });
});

test('CH02 actual settings Sheet exposes full notification action and clears the header marker on its retained route handler', async () => {
  await community('tab', async h => {
    const toggle = options(h), owner = h.owner('CommunityScreenContent'), search = input(h);
    assert.equal(toggle.props.accessibilityLabel.includes('có thông báo mới'), true);
    const dot = one(toggle.findAllByType('RNView').filter(node => style(node)?.width === 10 && style(node)?.height === 10), 'actual fresh-notification dot');
    assert.equal(style(dot).position, 'absolute');
    await press(toggle); const panel = settings(h); assert.equal(panel.props.open, true);
    const rows = panel.findAllByType('RNPressable').filter(node => node.props.accessibilityLabel === 'Thông báo · có điều mới');
    assert.equal(rows.length, 1, 'CH02 the moved notification action exists inside the actual settings task');
    const row = rows[0], label = copy(row, 'Thông báo · có điều mới');
    assert.equal(row.props.accessibilityRole, 'button'); assert.ok(style(row).minHeight >= 48);
    assert.equal(label.props.numberOfLines, undefined); assert.equal(label.props.ellipsizeMode, undefined);
    const retryBefore = counters(h);
    for (const value of [snapshot(2), snapshot()]) {
      await h.nativeFont(value); metrics(h, label, { fontSize: 14, lineHeight: 19 });
      assert.equal(settings(h) === panel, true); assert.equal(copy(row, 'Thông báo · có điều mới') === label, true);
      assert.equal(input(h) === search, true); assert.equal(h.owner('CommunityScreenContent') === owner, true);
      assert.deepEqual(counters(h), retryBefore); observation(h);
    }
    const retained = row.props.onPress; await act(async () => { retained(); });
    assert.equal(panel.props.open, false); assert.equal(h.routes.at(-1), '/community/notifications');
    const cleared = options(h); assert.equal(cleared === toggle, true); assert.equal(cleared.props.accessibilityLabel, 'Tuỳ chọn Cộng đồng');
    assert.equal(cleared.findAllByType('RNView').filter(node => style(node)?.width === 10 && style(node)?.height === 10).length, 0);
    assert.equal(h.community.storageCalls.every(call => call.method !== 'ghiGiaoDienAsync'), true, 'the route owns the later seen timestamp; header only resets RAM marker');
    await press(cleared); assert.equal(settings(h).props.open, true); assert.equal(action(settings(h), 'Thông báo').props.accessibilityRole, 'button');
    await act(async () => settings(h).props.onClose()); assert.equal(settings(h).props.open, false);
    actual(h);
  });
});

test('CH03 standalone actions and shared defaults retain their contracts while actual preferences history and feed-load guards remain explicit', async () => {
  await community('standalone', async h => {
    assert.equal(h.hosts('RNView', props => props.testID === 'dau-kham-pha').length, 0);
    const bell = h.one('RNPressable', props => props.accessibilityLabel === 'Thông báo, có điều mới');
    const gear = options(h), compose = h.one('RNPressable', props => props.accessibilityLabel === 'Đăng khoảnh khắc');
    assert.equal(gear.props.accessibilityLabel, 'Cài đặt bảng tin');
    for (const node of [bell, gear, compose]) { assert.equal(style(node).width, 48); assert.equal(style(node).height, 48); }
    await press(bell); assert.equal(h.routes.at(-1), '/community/notifications');
    await press(gear); const panel = settings(h); assert.equal(panel.props.open, true);
    assert.equal(panel.findAllByType('RNPressable').some(node => /Thông báo/.test(node.props.accessibilityLabel)), false, 'standalone retains its direct bell instead of duplicating it in the menu');
    const toggle = action(panel, 'Bật cá nhân hóa'); assert.equal(toggle.props.accessibilityRole, 'switch'); assert.equal(toggle.props.accessibilityState.checked, false);
    await press(toggle); assert.equal(h.community.preferenceWrites.length, 1);
    const write = h.community.preferenceWrites[0]; assert.equal(write.options.body.personalized, true);
    const before = counters(h); await h.nativeFont(snapshot(2)); assert.deepEqual(counters(h), before, 'hot metrics alone do not resubmit a pending preference mutation');
    await h.finish(write, { asked: true, personalized: true });
    assert.equal(h.community.feedReads.length, 2, 'accepted preference response starts one actual reload');
    const pendingFeed = h.community.feedReads.at(-1), count = h.community.feedReads.length;
    const list = h.one('RNFlatList');
    await act(async () => { list.props.refreshControl.props.onRefresh(); list.props.refreshControl.props.onRefresh(); });
    assert.equal(h.community.feedReads.length, count, 'actual fetching guard refuses concurrent reloads');
    await h.finish(pendingFeed, emptyPage());
    assert.equal(action(panel, 'Tắt cá nhân hóa').props.accessibilityState.checked, true);
    await press(action(panel, 'Xóa lịch sử đề xuất')); assert.equal(h.community.historyWrites.length, 0, 'disclosure alone cannot delete history');
    const confirmation = h.one('RNView', props => props.testID === 'cong-dong-xac-nhan-xoa');
    copy(confirmation, 'Xoá những gì bảng tin đã học từ lượt xem và tương tác của bạn? Bài đã lưu, bài đã ẩn và người bạn theo dõi vẫn giữ nguyên.');
    await press(action(confirmation, 'Giữ lại')); assert.equal(h.community.historyWrites.length, 0);
    await press(action(panel, 'Xóa lịch sử đề xuất')); await press(action(panel, 'Xoá lịch sử'));
    assert.equal(h.community.historyWrites.length, 1); await h.finish(h.community.historyWrites[0], {});
    assert.equal(h.community.feedReads.length, 3, 'accepted deletion starts one actual reload'); await h.finish(h.community.feedReads.at(-1), emptyPage());
    copy(panel, 'Đã xoá lịch sử đề xuất. Bảng tin bắt đầu lại từ những gì mới.');
    await press(action(panel, 'Bài đã ẩn')); assert.equal(panel.props.open, false);
    assert.equal(h.community.feedReads.length, 4, 'explicit hidden-list choice changes the actual feed request');
    assert.equal(h.community.feedReads.at(-1).route, '/v2/community/feed?mode=hidden');
    await h.finish(h.community.feedReads.at(-1), emptyPage());
    copy(h.root.root, 'Chưa ẩn bài nào');
    assert.equal(h.community.historyWrites.length, 1); assert.equal(h.community.preferenceWrites.length, 1);
    await press(compose); assert.equal(h.routes.at(-1), '/community/new'); actual(h, false);
  });
  await community('defaults', async h => {
    const header = component(h, 'apps/mobile/src/rudi/ui/DauKhamPha', 'DauKhamPha');
    const tabs = component(h, 'apps/mobile/src/rudi/ui/HangChuTab', 'HangChuTab');
    assert.equal(header.props.nativeMetrics, undefined); assert.equal(header.props.inlineActions, undefined); assert.equal(tabs.props.nativeMetrics, undefined);
    const label = copy(header, 'Cộng đồng'), tab = copy(tabs, 'Một');
    for (const value of [snapshot(), snapshot(2), unavailable(), snapshot()]) {
      await h.nativeFont(value); raw(label, { fontSize: 21, lineHeight: 27 }); raw(tab, { fontSize: 14, lineHeight: 19 });
      assert.equal(copy(header, 'Cộng đồng') === label, true); assert.equal(copy(tabs, 'Một') === tab, true); observation(h);
    }
    assert.equal(h.calls.length, 0, 'default shared probes have no application transport');
  });
});
