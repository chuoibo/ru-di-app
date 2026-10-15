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

test('PH03 identity edit keeps its semantic action without a measured text mirror through stale-window metrics', async () => {
  await withHost({}, async h => {
    const owner = h.owner('HoSoSong'), content = h.owner('ProfileContent');
    const edit = h.one('RNPressable', props => props.accessibilityLabel === 'Chỉnh hồ sơ');
    const before = { transport: h.calls.length, profile: h.profileReads.length, storage: h.storageCalls.length, native: h.nativeHostCalls.length };
    for (const value of [snapshot(2), unavailable(), snapshot(), snapshot(2)]) {
      await h.nativeFont(value);
      assert.equal(h.state.window.fontScale, 1, 'effective native metrics change while window scale stays stale');
      assert.ok(h.one('RNPressable', props => props.accessibilityLabel === 'Chỉnh hồ sơ') === edit);
      assert.equal(edit.props.accessibilityRole, 'button');
      assert.equal(edit.props.accessibilityHint, 'Sửa tên, giới thiệu và thành phố');
      assert.equal(flatten(edit.props.style).minWidth >= 48, true);
      assert.equal(flatten(edit.props.style).minHeight >= 48, true);
      assert.equal(h.hosts('RNText', props => props.pointerEvents === 'none' && typeof props.onTextLayout === 'function').length, 0, 'identity action has no stale glyph measurement callback');
      assert.equal(h.owner('HoSoSong') === owner, true); assert.equal(h.owner('ProfileContent') === content, true);
      assert.deepEqual({ transport: h.calls.length, profile: h.profileReads.length, storage: h.storageCalls.length, native: h.nativeHostCalls.length }, before);
      assert.equal(h.profileWrites.length, 0); assert.equal(h.sessionWrites.length, 0);
      ownership(h, 1); actual(h);
    }
    await h.pressAccessibility('Chỉnh hồ sơ');
    assert.equal(h.profileInputs().length, 3, 'actual identity action opens the existing form');
    await h.pressText('Huỷ');
  });
});

test('PH04 actual profile hierarchy keeps server facts routes null-zero semantics and tab identity', async () => {
  // Actual source and native host props only: no Yoga, glyph fit or TalkBack simulation.
  const fields = [
    ['friends', 'friends', 'bạn bè'], ['contexts', 'contexts', 'nhóm'],
    ['outings', 'outings', 'kèo'], ['places', 'places_checked_in', 'điểm check-in'],
    ['memories', 'memories', 'khoảnh khắc'],
  ];
  const io = h => ({ transport: h.calls.length, profileReads: h.profileReads.length, profileWrites: h.profileWrites.length, sessionWrites: h.sessionWrites.length, storage: h.storageCalls.length, nonfontNativeIO: h.nativeHostCalls.length, sockets: h.sockets.length, picks: h.picks.length, uploads: h.uploads.length });
  const exerciseFacts = async h => {
    await h.update(state => { state.window = { ...state.window, width: 320 }; });
    const original = h.owner('HoSoSong'), content = h.owner('ProfileContent');
    const counts = h.one('RNView', props => props.testID === 'profile-counts');
    const friend = h.one('RNPressable', props => props.testID === 'profile-fact-friends');
    const hosts = fields.map(([id]) => id === 'friends' ? friend.findAllByType('RNText')[0] : h.one('RNText', props => props.testID === `profile-fact-${id}`));
    const before = io(h);
    for (const [value, manual] of [[snapshot(), true], [snapshot(2), true], [snapshot(), true], [snapshot(2), true], [unavailable(), false], [{ status: 'available', fontScale: 2 }, false], [snapshot(), true]]) {
      await h.nativeFont(value);
      assert.ok(h.one('RNView', props => props.testID === 'profile-counts') === counts);
      assert.equal(counts.findAllByType('RNPressable').length, 1, 'only the real friends door is actionable');
      assert.equal(friend.props.accessibilityLabel, `${h.state.profile.counts.friends} bạn bè`);
      assert.equal(friend.props.accessibilityRole, 'button');
      assert.equal(flatten(friend.props.style).minHeight >= 48, true);
      for (let index = 0; index < fields.length; index += 1) {
        const [id, field, noun] = fields[index];
        const host = id === 'friends' ? friend.findAllByType('RNText')[0] : h.one('RNText', props => props.testID === `profile-fact-${id}`);
        assert.ok(host === hosts[index], 'same count-and-noun reading host through metrics changes');
        assert.equal(host.props.children, `${h.state.profile.counts[field]} ${noun}`, 'full noun stays paired with the exact distinct server field');
        assert.equal(host.props.allowFontScaling, manual ? false : undefined);
        for (const prop of ['numberOfLines', 'adjustsFontSizeToFit', 'maxFontSizeMultiplier', 'onPress']) assert.equal(host.props[prop], undefined);
        const style = flatten(host.props.style);
        assert.equal(style.height, undefined); assert.equal(style.maxHeight, undefined);
      }
      assert.equal(h.owner('HoSoSong') === original, true); assert.equal(h.owner('ProfileContent') === content, true);
      assert.equal(h.state.window.width, 320); assert.equal(h.state.window.fontScale, 1);
      assert.deepEqual(io(h), before, 'font changes issue no product transport, writes, storage or nonfont native I/O');
      assert.equal(h.boundaryViolations.length, 0); ownership(h, 1); actual(h);
    }
    return original;
  };
  await withHost({ deferSaved: true }, async h => {
    const title = text(h, 'Cá nhân');
    assert.equal(flatten(title.props.style).fontSize, 17); assert.equal(flatten(title.props.style).lineHeight, 23);
    const original = await exerciseFacts(h);
    assert.equal(h.root.root.findAll(node => typeof node.type === 'function' && node.type.name === 'AssetIllustration' && node.props.name === 'decor-cover').length, 0);
    for (const group of ['Sổ của bạn', 'Quản lý']) assert.equal(text(h, group).props.accessibilityRole, 'header');
    const tabs = h.hosts('RNPressable', props => props.accessibilityRole === 'tab');
    assert.equal(tabs.length, 2);
    assert.equal(tabs.every(node => flatten(node.props.style).minHeight >= 48), true);
    assert.equal(tabs.filter(node => node.props.accessibilityState.selected).length, 1);
    for (const tab of tabs) for (const label of tab.findAllByType('RNText')) assert.equal(label.props.numberOfLines, undefined);
    assert.equal(h.text('0 địa điểm').length, 0, 'unknown saved count is not zero');
    await h.finish(h.savedReads[0], { saved: [] });
    assert.equal(h.text('0 địa điểm').length, 1, 'server empty saved count is printed');
    await h.pressAccessibility(`${h.state.profile.counts.friends} bạn bè`); assert.equal(h.routes.at(-1), '/friends');
    for (const [label, route] of [['Sở thích', '/personalization'], ['Tường của tôi', '/people/synthetic-A'], ['Tài chính của tôi', '/finance'], ['Hành trình', '/achievements'], ['Cài đặt', '/settings']]) {
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
  await withHost({ profile: { display_name: 'Synthetic long name for profile layout audit', bio: 'Synthetic long bio for layout audit', city: null, counts: { friends: 0, contexts: 1234567, outings: 456789, places_checked_in: 7890123, memories: 0 } }, invitations: [{ state: 'pending' }, { state: 'accepted' }, { state: 'pending' }] }, async h => {
    await exerciseFacts(h);
    assert.equal(h.text('Synthetic long name for profile layout audit').length, 1);
    assert.equal(h.text('Synthetic long bio for layout audit').length, 1);
    assert.equal(h.textContent().includes('null'), false, 'missing city is not printed as text');
    assert.equal(h.text('2 lời mời đang chờ').length, 1, 'pending invitation count is server-derived');
    await h.pressAccessibility('0 bạn bè'); assert.equal(h.routes.at(-1), '/friends?muc=da-nhan');
    actual(h);
  });
});

test('PH05 profile keeps identity edit and retained draft together through hot metrics', async () => {
  await withHost({}, async h => {
    await h.update(state => { state.window = { ...state.window, width: 320 }; });
    const original = h.owner('HoSoSong'), content = h.owner('ProfileContent');
    const identity = h.one('RNView', props => props.testID === 'profile-identity');
    const edit = h.one('RNPressable', props => props.accessibilityLabel === 'Chỉnh hồ sơ');
    const avatar = h.one('RNPressable', props => props.accessibilityLabel === 'Đổi ảnh đại diện');
    const ioCalls = h.calls.length, countReads = h.profileReads.length;
    assert.equal(identity.findAllByType('RNPressable').includes(edit), true);
    assert.equal(identity.findAllByType('RNPressable').includes(avatar), true);
    assert.equal(identity.findAllByType('RNView').some(node => node.props.testID === 'profile-counts'), false, 'profile facts are outside identity');
    for (const value of [snapshot(2), unavailable(), snapshot()]) {
      await h.nativeFont(value);
      assert.ok(h.one('RNView', props => props.testID === 'profile-identity') === identity);
      assert.ok(h.one('RNPressable', props => props.accessibilityLabel === 'Chỉnh hồ sơ') === edit);
      assert.ok(h.one('RNPressable', props => props.accessibilityLabel === 'Đổi ảnh đại diện') === avatar);
      assert.equal(h.owner('HoSoSong') === original, true); assert.equal(h.owner('ProfileContent') === content, true);
      assert.equal(h.state.window.fontScale, 1);
      assert.equal(h.profileReads.length, countReads); assert.equal(h.calls.length, ioCalls);
      assert.equal(h.profileWrites.length, 0); assert.equal(h.sessionWrites.length, 0); ownership(h, 1);
    }
    await h.pressAccessibility('Chỉnh hồ sơ');
    assert.equal(h.profileInputs().length, 3);
    await h.changeProfile(labels[0], syntheticDraft[0]);
    await h.pressAccessibility('Quay lại');
    const resume = h.one('RNPressable', props => props.accessibilityLabel === 'Sửa tiếp hồ sơ');
    const retainedNote = text(h, 'Bản sửa chưa lưu vẫn còn đây.');
    assert.equal(retainedNote.props.accessibilityLiveRegion, 'polite');
    const resumedIdentity = h.one('RNView', props => props.testID === 'profile-identity');
    assert.equal(resumedIdentity.findAllByType('RNPressable').includes(resume), true);
    assert.equal(resumedIdentity.findAllByType('RNText').includes(retainedNote), true, 'resume and retained draft hint belong to identity');
    assert.equal(flatten(resume.props.style).minWidth >= 48, true); assert.equal(flatten(resume.props.style).minHeight >= 48, true);
    await h.pressAccessibility('Sửa tiếp hồ sơ');
    assert.equal(h.profileInput(labels[0]).props.value, syntheticDraft[0]);
    await h.pressText('Huỷ');
    assert.equal(h.profileWrites.length, 0); assert.equal(h.sessionWrites.length, 0);
    assert.equal(h.owner('HoSoSong') === original, true); assert.equal(h.owner('ProfileContent') === content, true);
    ownership(h, 1); actual(h);
  });
});
