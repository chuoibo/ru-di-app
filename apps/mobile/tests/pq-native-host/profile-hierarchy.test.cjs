// Actual profile graph; these hosts do not emulate Yoga, glyphs or Android IME.
const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { createProfileHost, snapshot, flatten, act, sourceRoot } = require('./profile-host.cjs');
global.IS_REACT_ACT_ENVIRONMENT = true;

const unavailable = () => ({ status: 'unavailable', reason: 'module-unavailable' });
const labels = ['Ô tên hiển thị', 'Ô giới thiệu', 'Ô thành phố'];
const syntheticDraft = ['Synthetic edited name', 'Synthetic edited bio', 'Synthetic edited city'];
async function withHost(options, callback) {
  const h = createProfileHost(options);
  try { await h.mount(); await callback(h); } finally { await h.dispose(); }
}
function actual(h) {
  for (const relative of ['apps/mobile/src/rudi/screens/Profile', 'apps/mobile/src/rudi/screens/profile/HoSoSong', 'apps/mobile/src/rudi/ui/Field', 'apps/mobile/src/rudi/ui/NativeFontMetrics', 'apps/mobile/src/rudi/nep/NepProvider']) {
    const file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(sourceRoot + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function ownership(h, subscriptions, removals = 0) {
  const observed = h.observation();
  assert.equal(observed.subscriptions, subscriptions, 'cumulative native subscriptions');
  assert.equal(observed.removals, removals, 'cumulative observer cleanup');
  assert.equal(observed.live, subscriptions - removals, 'live observers');
  assert.equal(observed.maxLive, subscriptions === 0 ? 0 : 1, 'no nested surface observers');
}
function text(h, content) {
  const found = h.text(props => props.children === content && props.pointerEvents !== 'none');
  assert.equal(found.length, 1, 'one visible text host');
  return found[0];
}
function button(h, label) {
  const found = h.hosts('RNPressable').filter(node => node.findAllByType('RNText').some(child => child.props.children === label));
  assert.equal(found.length, 1, 'one labeled button'); return found[0];
}
function sameInputs(h, inputs, owner) {
  assert.equal(h.owner('HoSoSong') === owner, true, 'same mounted draft owner');
  for (let index = 0; index < labels.length; index += 1) {
    assert.equal(h.profileInput(labels[index]) === inputs[index], true, 'same native input host through metrics changes');
    assert.equal(h.profileInput(labels[index]).props.value, syntheticDraft[index], 'controlled draft retained');
  }
}
function inputMetrics(h, fontSize, lineHeight, manual) {
  for (const label of labels) {
    const input = h.profileInput(label), style = flatten(input.props.style);
    assert.equal(style.fontSize, fontSize); assert.equal(style.lineHeight, lineHeight);
    assert.equal(input.props.allowFontScaling, manual ? false : undefined);
  }
}
function nearest(node, kind, match) {
  for (let parent = node.parent; parent; parent = parent.parent) if (parent.type === kind && match(parent.props)) return parent;
  throw new Error('Expected actual ancestor host');
}
function mirror(h) {
  return h.one('RNText', props => props.pointerEvents === 'none' && typeof props.onTextLayout === 'function' && props.children === 'Chỉnh hồ sơ');
}

// This is one case with three trials, so cumulative navigation churn is observable.
test('PH01 profile reuses available/unavailable owner and owns one absent-observer through every panel', async () => {
  for (const provider of [null, snapshot(2), unavailable()]) {
    const h = createProfileHost({ explicitProvider: provider !== null, initialSnapshot: provider ?? snapshot(), deferProfile: true });
    try {
      await h.mount(); ownership(h, provider === null ? 1 : 0);
      if (provider !== null) assert.equal(h.observation().reads, 0, 'inherited provider never reads native getter');
      assert.equal(h.profileReads.length, 1, 'real profile read is parked in loading');
      assert.equal(h.hosts('RNView', props => props.testID === 'profile-counts').length, 0);
      await h.finish(h.profileReads[0], h.state.profile);
      const content = h.owner('ProfileContent');
      for (const step of ['Trang đã giữ', 'Hồ sơ', 'Đã lưu', 'Quay lại', 'Tài khoản', 'Quay lại']) {
        if (step === 'Quay lại') await h.pressAccessibility(step); else await h.pressText(step);
        ownership(h, provider === null ? 1 : 0);
        assert.equal(h.owner('ProfileContent') === content, true, 'same content owner through panels');
      }
      await h.nativeFont(snapshot(2)); ownership(h, provider === null ? 1 : 0);
      await h.nativeFont(unavailable()); ownership(h, provider === null ? 1 : 0);
      await h.nativeFont(snapshot()); ownership(h, provider === null ? 1 : 0);
      actual(h);
      await h.dispose(); ownership(h, provider === null ? 1 : 0, provider === null ? 1 : 0);
    } finally { if (h.root) await h.dispose(); }
  }
});

test('PH02 real editor retains input hosts and draft through hot metrics with guarded save/back/cancel', async () => {
  await withHost({}, async h => {
    await h.pressAccessibility('Chỉnh hồ sơ');
    const owner = h.owner('HoSoSong'), inputs = labels.map(label => h.profileInput(label));
    assert.equal(h.profileInputs().length, 3);
    for (let index = 0; index < labels.length; index += 1) await h.changeProfile(labels[index], syntheticDraft[index]);
    sameInputs(h, inputs, owner); inputMetrics(h, 17, 24, true);
    const fieldLabelHosts = new Map(['Tên hiển thị', 'Giới thiệu', 'Thành phố'].map(label => [label, text(h, label)]));
    for (const value of [snapshot(2), unavailable(), snapshot()]) {
      await h.nativeFont(value); sameInputs(h, inputs, owner); ownership(h, 1);
      inputMetrics(h, value.fontMetrics ? value.fontScale === 2 ? 28.75 : 17 : 17, value.fontMetrics ? value.fontScale === 2 ? 39.25 : 24 : 24, Boolean(value.fontMetrics));
      for (const label of ['Tên hiển thị', 'Giới thiệu', 'Thành phố']) {
        const currentLabel = text(h, label);
        assert.equal(currentLabel === fieldLabelHosts.get(label), true, 'opted-in Field label host stays mounted');
        assert.equal(currentLabel.props.allowFontScaling, value.fontMetrics ? false : undefined, 'label follows the existing owner or RN fallback');
        assert.equal(flatten(currentLabel.props.style).fontSize, value.fontMetrics && value.fontScale === 2 ? 23.25 : 14);
        assert.equal(flatten(currentLabel.props.style).lineHeight, value.fontMetrics && value.fontScale === 2 ? 31.75 : 19);
      }
    }
    assert.equal(h.profileWrites.length, 0, 'draft/font changes do not save');
    const saveBefore = button(h, 'Lưu hồ sơ').props.onPress;
    const cancelBefore = button(h, 'Huỷ').props.onPress;
    const backBefore = h.one('RNPressable', props => props.accessibilityLabel === 'Quay lại').props.onPress;
    await h.pressText('Lưu hồ sơ'); assert.equal(h.profileWrites.length, 1);
    assert.equal(button(h, 'Lưu hồ sơ').props.disabled, true);
    assert.equal(button(h, 'Huỷ').props.disabled, true);
    assert.equal(h.profileInputs().every(input => input.props.editable === false), true);
    await act(async () => { saveBefore(); cancelBefore(); backBefore(); });
    assert.equal(h.profileWrites.length, 1, 'retained actions cannot duplicate save while busy');
    sameInputs(h, inputs, owner);
    const write = h.profileWrites[0];
    assert.deepEqual(JSON.parse(JSON.stringify(write.body)), { display_name: syntheticDraft[0], bio: syntheticDraft[1], city: syntheticDraft[2] });
    await h.fail(write, h.apiError('Synthetic save refusal'));
    assert.equal(text(h, 'Synthetic save refusal').props.accessibilityRole, 'alert');
    assert.equal(text(h, 'Synthetic save refusal').props.allowFontScaling, false, 'opted-in Field error uses the still-available owner at font100');
    assert.equal(flatten(text(h, 'Synthetic save refusal').props.style).fontSize, 13);
    assert.equal(flatten(text(h, 'Synthetic save refusal').props.style).lineHeight, 18);
    sameInputs(h, inputs, owner);
    await h.pressAccessibility('Quay lại');
    assert.equal(h.profileInputs().length, 0, 'closing editor intentionally unmounts input hosts');
    await h.pressAccessibility('Sửa tiếp hồ sơ');
    for (let index = 0; index < labels.length; index += 1) assert.equal(h.profileInput(labels[index]).props.value, syntheticDraft[index]);
    await h.pressText('Huỷ'); await h.pressAccessibility('Chỉnh hồ sơ');
    assert.equal(h.profileInput(labels[0]).props.value, h.state.profile.display_name, 'Huỷ discards retained draft');
    for (let index = 0; index < labels.length; index += 1) await h.changeProfile(labels[index], syntheticDraft[index]);
    await h.pressText('Lưu hồ sơ'); assert.equal(h.profileWrites.length, 2);
    const accepted = { ...h.state.profile, display_name: 'Synthetic server name', bio: 'Synthetic server bio', city: 'Synthetic server city' };
    await h.finish(h.profileWrites[1], accepted);
    assert.equal(h.profileInputs().length, 0);
    assert.equal(text(h, accepted.display_name).props.children, accepted.display_name, 'card follows server response');
    assert.equal(h.sessionWrites.length, 1, 'session greeting storage follows accepted server name');
    assert.equal(h.sessionWrites[0].displayName, accepted.display_name);
    ownership(h, 1); actual(h);
  });
});

test('PH03 edit action invalidates measured width from effective metrics despite stale window font scale', async () => {
  await withHost({}, async h => {
    const counts = h.one('RNView', props => props.testID === 'profile-counts');
    const passport = nearest(counts, 'RNView', props => typeof props.onLayout === 'function');
    await act(async () => passport.props.onLayout({ nativeEvent: { layout: { width: 288 } } }));
    const visible = text(h, 'Chỉnh hồ sơ'), oldMirror = mirror(h).props.onTextLayout;
    await act(async () => oldMirror({ nativeEvent: { lines: [{ width: 101.1 }] } }));
    assert.equal(flatten(text(h, 'Chỉnh hồ sơ').props.style).width, 101.5);
    await h.nativeFont(snapshot(2));
    assert.equal(h.state.window.fontScale, 1, 'window scale deliberately stale');
    assert.equal(text(h, 'Chỉnh hồ sơ') === visible, true, 'visible label retains host');
    assert.equal(flatten(visible.props.style).fontSize, 23.25);
    assert.equal(flatten(visible.props.style).lineHeight, 31.75);
    assert.equal(flatten(visible.props.style).width, undefined, 'old width invalidated by effective label metrics');
    await act(async () => oldMirror({ nativeEvent: { lines: [{ width: 3 }] } }));
    assert.equal(flatten(visible.props.style).width, undefined, 'retained old font callback rejected');
    const currentMirror = mirror(h).props.onTextLayout;
    await act(async () => currentMirror({ nativeEvent: { lines: [{ width: 160.1 }] } }));
    assert.equal(flatten(visible.props.style).width, 160.5);
    await act(async () => passport.props.onLayout({ nativeEvent: { layout: { width: 200 } } }));
    assert.equal(flatten(visible.props.style).width, undefined, 'width revision invalidates cached label width');
    await act(async () => currentMirror({ nativeEvent: { lines: [{ width: 1 }] } }));
    assert.equal(flatten(visible.props.style).width, undefined, 'retained prior column callback rejected');
    await act(async () => mirror(h).props.onTextLayout({ nativeEvent: { lines: [{ width: 180 }] } }));
    assert.equal(flatten(visible.props.style).width, 143, 'label width capped by finite column space');
    await h.nativeFont(unavailable());
    assert.equal(flatten(visible.props.style).width, undefined, 'fallback has a distinct measurement key');
    assert.equal(visible.props.allowFontScaling, undefined);
    ownership(h, 1); actual(h);
  });
});

test('PH04 actual profile hierarchy keeps server facts routes null-zero semantics and tab identity', async () => {
  await withHost({ deferSaved: true }, async h => {
    const original = h.owner('HoSoSong');
    const title = text(h, 'Cá nhân');
    assert.equal(flatten(title.props.style).fontSize, 17); assert.equal(flatten(title.props.style).lineHeight, 23);
    const counts = h.one('RNView', props => props.testID === 'profile-counts');
    const expected = [`${h.state.profile.counts.friends} bạn bè`, `${h.state.profile.counts.contexts} nhóm`, `${h.state.profile.counts.outings} kèo`, `${h.state.profile.counts.places_checked_in} nơi đã tới`, `${h.state.profile.counts.memories} khoảnh khắc`];
    const facts = counts.findAllByType('RNText');
    const factGroup = fact => nearest(fact, 'RNView', () => true);
    const groups = [...new Set(facts.map(factGroup))];
    assert.equal(groups.length, 2, 'five server facts belong to two actual native group parents');
    assert.deepEqual(groups.map(group => facts.filter(fact => factGroup(fact) === group).length), [3, 2], 'social activity and experiences own separate columns');
    const checkCounts = stacked => {
      assert.equal(h.one('RNView', props => props.testID === 'profile-counts') === counts, true, 'same count container through hot metrics');
      assert.equal(flatten(counts.props.style).width, '100%');
      assert.equal(flatten(counts.props.style).gap, 4);
      assert.equal(flatten(counts.props.style).flexWrap, undefined, 'facts do not depend on intrinsic wrapping');
      const currentFacts = counts.findAllByType('RNText');
      const currentGroups = [...new Set(currentFacts.map(factGroup))];
      assert.equal(currentGroups.length, 2, 'two actual group parents remain through hot metrics');
      for (let index = 0; index < groups.length; index += 1) {
        assert.equal(currentGroups[index] === groups[index], true, 'same two group parents through hot metrics');
        const style = flatten(groups[index].props.style);
        assert.equal(style.flexDirection, stacked ? 'column' : 'row');
        assert.equal(style.width, '100%'); assert.equal(style.minWidth, 0);
        assert.equal(style.columnGap, 12); assert.equal(style.rowGap, 4);
      }
      assert.deepEqual(currentFacts.map(node => node.props.children), expected);
      assert.equal(currentFacts.every(node => flatten(node.props.style).maxWidth === '100%'), true);
      for (let index = 0; index < facts.length; index += 1) {
        assert.equal(currentFacts[index] === facts[index], true, 'same five native Text hosts through hot metrics');
        assert.equal(factGroup(currentFacts[index]) === groups[index < 3 ? 0 : 1], true, 'same fact group ancestor');
        const style = flatten(currentFacts[index].props.style);
        assert.equal(style.minWidth, 0);
        assert.equal(style.flexBasis, stacked ? undefined : 0, 'zero row basis must be absent from a vertical group');
        assert.equal(style.flexGrow, stacked ? undefined : 1);
        assert.equal(style.flexShrink, stacked ? undefined : 1);
        assert.equal(style.width, stacked ? '100%' : undefined);
        assert.equal(style.alignSelf, stacked ? 'stretch' : undefined);
      }
      assert.equal(h.owner('HoSoSong') === original, true, 'same profile owner through count layout changes');
      ownership(h, 1);
    };
    checkCounts(false);
    for (const [value, stacked] of [[snapshot(2), true], [unavailable(), true], [snapshot(), false]]) {
      await h.nativeFont(value);
      assert.equal(h.state.window.fontScale, 1, 'native snapshot drives layout despite stale window scale');
      checkCounts(stacked);
    }
    assert.equal(h.profileWrites.length, 0, 'count layout does not save profile');
    assert.equal(h.sessionWrites.length, 0, 'count layout does not mutate session');
    assert.equal(h.root.root.findAll(node => typeof node.type === 'function' && node.type.name === 'AssetIllustration' && node.props.name === 'decor-cover').length, 0);
    for (const group of ['Kết nối & gu', 'Sổ của bạn', 'Quản lý']) {
      const label = text(h, group); assert.equal(label.props.accessibilityRole, 'header'); assert.equal(flatten(label.props.style).fontSize, 14);
    }
    const tabs = h.hosts('RNPressable', props => props.accessibilityRole === 'tab');
    assert.equal(tabs.length, 2); assert.equal(tabs.every(node => flatten(node.props.style).minHeight === 48), true);
    const active = tabs.find(node => node.props.accessibilityState.selected);
    assert.equal(active.findAllByType('RNView').some(node => flatten(node.props.style).width === 32 && flatten(node.props.style).height === 2), true);
    assert.equal(h.text('0 địa điểm').length, 0, 'unknown saved count is not zero');
    await h.finish(h.savedReads[0], { saved: [] });
    assert.equal(h.text('0 địa điểm').length, 1, 'server empty saved count is printed');
    for (const [label, route] of [['Bạn bè', '/friends'], ['Sở thích', '/personalization'], ['Tường của tôi', '/people/synthetic-A'], ['Tài chính của tôi', '/finance'], ['Hành trình', '/achievements'], ['Cài đặt', '/settings']]) {
      await h.pressText(label); assert.equal(h.routes.at(-1), route);
    }
    await h.pressText('Trang đã giữ'); assert.equal(h.owner('HoSoSong') === original, true);
    await h.pressText('Hồ sơ'); assert.equal(h.owner('HoSoSong') === original, true);
    await h.pressText('Đã lưu'); assert.equal(h.text('0 địa điểm').length > 0, true);
    await h.pressAccessibility('Quay lại');
    assert.equal(h.owner('HoSoSong') === original, false, 'saved panel unmount remains the documented baseline');
    await h.pressText('Tài khoản'); assert.equal(h.text('Đăng xuất').length > 0, true);
    await h.pressAccessibility('Quay lại'); ownership(h, 1); actual(h);
  });
  await withHost({ profile: { display_name: 'Synthetic long name for profile layout audit', bio: 'Synthetic long bio for layout audit', city: null, counts: { friends: 0, contexts: 123, outings: 456, places_checked_in: 789, memories: 0 } }, invitations: [{ state: 'pending' }, { state: 'accepted' }, { state: 'pending' }] }, async h => {
    assert.deepEqual(h.one('RNView', props => props.testID === 'profile-counts').findAllByType('RNText').map(node => node.props.children), ['0 bạn bè', '123 nhóm', '456 kèo', '789 nơi đã tới', '0 khoảnh khắc']);
    assert.equal(h.text('Synthetic long name for profile layout audit').length, 1);
    assert.equal(h.text('Synthetic long bio for layout audit').length, 1);
    assert.equal(h.textContent().includes('null'), false, 'missing city is not printed as text');
    assert.equal(h.text('2 lời mời đang chờ').length, 1, 'pending invitation count is server-derived');
    await h.pressText('Bạn bè'); assert.equal(h.routes.at(-1), '/friends?muc=da-nhan');
    actual(h);
  });
});
