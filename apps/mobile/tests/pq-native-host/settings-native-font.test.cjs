// Actual Settings/appearance/session graph; host props do not establish Android geometry.
const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { createSettingsHost, snapshot, flatten, sourceRoot } = require('./profile-host.cjs');
global.IS_REACT_ACT_ENVIRONMENT = true;

const unavailable = () => ({ status: 'unavailable', reason: 'module-inactive' });
const themes = ['Sáng', 'Tối', 'Theo hệ thống'];
const settingsProfile = { display_name: 'Synthetic Settings name', wall_comment_policy: 'friends' };
const localHelper = 'Lựa chọn này chỉ ở trên máy này, không gửi đi đâu.';
const routes = [
  ['Sở thích', '/personalization'],
  ['Tài khoản & bảo mật', '/settings/account'],
  ['Phiên đăng nhập', '/settings/phien'],
  ['Thiết bị nhắn tin mã hoá', '/settings/thiet-bi'],
  ['Người đã chặn', '/settings/da-chan'],
  ['Điều khoản và dữ liệu', '/settings/ve-rudi'],
  ['Xoá tài khoản', '/settings/xoa-tai-khoan'],
];
async function withHost(options, callback) {
  const h = createSettingsHost({ profile: settingsProfile, ...options });
  h.state.window = { width: 320, height: 640, fontScale: 1, scale: 2 };
  try { await h.mount(); await callback(h); } finally { if (h.root) await h.dispose(); }
}
function actual(h) {
  for (const relative of [
    'apps/mobile/src/rudi/screens/cai-dat/CaiDatScreen',
    'apps/mobile/src/rudi/ui/GiaoDienProvider',
    'apps/mobile/src/rudi/ui/NativeFontMetrics',
    'apps/mobile/src/rudi/chia-bill/useBillFontScale',
    'apps/mobile/src/rudi/session',
    'apps/mobile/src/rudi/ui',
    'apps/mobile/src/rudi/nguoi/chinh-sach-tuong',
    'apps/mobile/src/camera/native',
  ]) {
    const file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(sourceRoot + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function hidden(node) {
  for (let current = node; current; current = current.parent) {
    const p = current.props;
    if (p?.importantForAccessibility === 'no-hide-descendants' || p?.['aria-hidden'] === true || p?.['aria-hidden'] === '') return true;
    if (flatten(p?.style)?.display === 'none') return true;
  }
  return false;
}
function text(h, copy) {
  const found = h.text(props => props.children === copy).filter(node => !hidden(node));
  assert.equal(found.length, 1, `one visible actual text: ${copy}`);
  return found[0];
}
function radio(h, label) {
  return h.one('RNPressable', props => props.accessibilityRole === 'radio' && props.accessibilityLabel === label);
}
function ownerCount(h, count, removals = 0) {
  const observed = h.observation();
  assert.equal(observed.subscriptions, count, 'Settings owns exactly the required observer count');
  assert.equal(observed.live, count - removals, 'native observer is live until cleanup');
  assert.equal(observed.removals, removals);
  assert.equal(observed.maxLive, count === 0 ? 0 : 1, 'no nested native metrics owner');
}
function noWrites(h) {
  assert.equal(h.profileWrites.length, 0, 'reading Settings never PATCHes profile/policy');
  assert.equal(h.sessionWrites.length, 0, 'font changes never persist session names');
  assert.equal(h.picks.length + h.uploads.length + h.writes.length + h.deletes.length, 0, 'no photo or community mutations');
  assert.equal(h.settingsIo.trace.filter(row => row.kind === 'theme-write').length, 0, 'font changes never set local theme');
  h.settingsIo.assertClean();
}
function copiedMetrics(h, scale, manual) {
  for (const label of themes) {
    const leaf = text(h, label);
    assert.equal(leaf.props.allowFontScaling, manual ? false : undefined, 'local radio copy follows actual owner or RN fallback');
    assert.equal(flatten(leaf.props.style).fontSize, manual && scale === 2 ? 28.75 : 17);
    assert.equal(flatten(leaf.props.style).lineHeight, manual && scale === 2 ? 39.25 : 24);
    assert.equal(leaf.props.numberOfLines, undefined, 'radio label is full text');
  }
  const helper = text(h, localHelper);
  assert.equal(helper.props.allowFontScaling, manual ? false : undefined);
  assert.equal(flatten(helper.props.style).fontSize, manual && scale === 2 ? 21.5 : 13);
  assert.equal(flatten(helper.props.style).lineHeight, manual && scale === 2 ? 30.25 : 18);
  assert.equal(helper.props.numberOfLines, undefined, 'local explanatory copy is not clamped');
  const sharedTitle = text(h, 'Giao diện');
  assert.equal(sharedTitle.props.allowFontScaling, manual ? false : undefined, 'actual shared SectionHeader inherits owner');
  assert.equal(flatten(sharedTitle.props.style).fontSize, manual && scale === 2 ? 34.25 : 21);
  assert.equal(flatten(sharedTitle.props.style).lineHeight, manual && scale === 2 ? 43 : 27);
  const row = text(h, 'Thiết bị nhắn tin mã hoá');
  assert.equal(row.props.allowFontScaling, manual ? false : undefined, 'actual shared ListRow inherits owner');
  assert.equal(flatten(row.props.style).fontSize, manual && scale === 2 ? 23.25 : 14);
  assert.equal(flatten(row.props.style).lineHeight, manual && scale === 2 ? 31.75 : 19);
}

test('ST01 standalone Settings owns one observer, retains actual copy/radios through hot and fallback without I/O', async () => {
  await withHost({}, async h => {
    ownerCount(h, 1); copiedMetrics(h, 1, true);
    assert.equal(h.profileReads.length, 1, 'one declared real profile read');
    const content = h.owner('CaiDatContent');
    const radios = themes.map(label => radio(h, label));
    const copies = [...themes, localHelper, 'Giao diện', 'Thiết bị nhắn tin mã hoá'].map(copy => [copy, text(h, copy)]);
    const ioBefore = JSON.stringify(h.settingsIo.trace);
    for (const value of [snapshot(2), snapshot(), unavailable(), snapshot(2), snapshot()]) {
      await h.nativeFont(value);
      ownerCount(h, 1); copiedMetrics(h, value.fontScale ?? 1, Boolean(value.fontMetrics));
      assert.equal(h.state.window.fontScale, 1, 'window font scale deliberately stale');
      assert.equal(h.owner('CaiDatContent') === content, true, 'same Settings state owner');
      for (let index = 0; index < themes.length; index += 1) {
        const current = radio(h, themes[index]);
        assert.equal(current === radios[index], true, 'same native radio host');
        assert.equal(current.props['aria-checked'], themes[index] === 'Theo hệ thống', 'actual theme state retained');
      }
      for (const [copy, leaf] of copies) assert.equal(text(h, copy) === leaf, true, 'same visible native text host');
      assert.equal(JSON.stringify(h.settingsIo.trace), ioBefore, 'font notifications cause no application/storage/navigation I/O');
      noWrites(h);
    }
    actual(h);
    const reads = h.observation().reads;
    await h.dispose(); ownerCount(h, 1, 1);
    await h.invokeRetired();
    assert.equal(h.observation().reads, reads, 'retired native event cannot reread after unmount');
    assert.equal(h.settingsIo.appearanceObservation().live, 0, 'actual appearance provider cleans up');
    h.settingsIo.assertClean();
  });
});

test('ST02 inherited available and unavailable owners suppress Settings native observation and preserve fallback/content', async () => {
  for (const initialSnapshot of [snapshot(2), unavailable()]) {
    await withHost({ explicitProvider: true, initialSnapshot }, async h => {
      ownerCount(h, 0);
      copiedMetrics(h, initialSnapshot.fontScale ?? 1, Boolean(initialSnapshot.fontMetrics));
      const content = h.owner('CaiDatContent'), selected = radio(h, 'Theo hệ thống');
      const ioBefore = JSON.stringify(h.settingsIo.trace);
      for (const value of [unavailable(), snapshot(2), snapshot()]) {
        await h.nativeFont(value);
        ownerCount(h, 0);
        assert.equal(h.observation().reads, 0, 'inherited unavailable owner never falls through to native getter');
        copiedMetrics(h, value.fontScale ?? 1, Boolean(value.fontMetrics));
        assert.equal(h.owner('CaiDatContent') === content, true);
        assert.equal(radio(h, 'Theo hệ thống') === selected, true);
        assert.equal(JSON.stringify(h.settingsIo.trace), ioBefore, 'inherited font changes cause no application I/O');
        noWrites(h);
      }
      actual(h);
    });
  }
});

test('ST03 actual header/local appearance setter and read-only route actions keep selection/owner through hot metrics', async () => {
  await withHost({}, async h => {
    const ui = h.load('apps/mobile/src/rudi/ui');
    const top = h.root.root.findByType(ui.TopBar);
    assert.equal(top.props.title, 'Cài đặt', 'one named shared TopBar replaces the duplicate body heading');
    const title = text(h, 'Cài đặt');
    assert.equal(title.props.accessibilityRole, 'header');
    const back = h.one('RNPressable', props => props.accessibilityLabel === 'Quay lại');
    assert.equal(flatten(back.props.style).width, 48);
    assert.equal(flatten(back.props.style).height, 48);
    const content = h.owner('CaiDatContent'), radios = themes.map(label => radio(h, label));
    const baselineReads = h.profileReads.length;
    await h.pressAccessibility('Tối');
    for (const option of themes) assert.equal(radio(h, option).props['aria-checked'], option === 'Tối');
    assert.equal(h.settingsIo.state.theme, 'toi');
    assert.equal(h.settingsIo.trace.filter(row => row.kind === 'theme-write').length, 1);
    const before = JSON.stringify(h.settingsIo.trace);
    for (const value of [snapshot(2), snapshot()]) {
      await h.nativeFont(value); copiedMetrics(h, value.fontScale, true);
      assert.equal(h.owner('CaiDatContent') === content, true);
      for (let index = 0; index < themes.length; index += 1) assert.equal(radio(h, themes[index]) === radios[index], true);
      assert.equal(text(h, 'Cài đặt') === title, true);
      assert.equal(radio(h, 'Tối').props['aria-checked'], true, 'explicit nondefault theme survives native font changes');
      assert.equal(h.settingsIo.state.theme, 'toi');
      assert.equal(JSON.stringify(h.settingsIo.trace), before);
    }
    for (const label of ['Sáng', 'Theo hệ thống']) {
      await h.pressAccessibility(label);
      for (const option of themes) assert.equal(radio(h, option).props['aria-checked'], option === label);
      assert.equal(h.settingsIo.state.theme, { Sáng: 'sang', 'Theo hệ thống': 'he-thong' }[label]);
    }
    assert.equal(h.settingsIo.trace.filter(row => row.kind === 'theme-write').length, 3, 'only explicit local radio presses persist appearance');
    for (const [label, target] of routes) {
      await h.pressText(label);
      const entry = h.settingsIo.trace.filter(row => row.kind === 'navigation').at(-1);
      assert.equal(entry.method, 'push'); assert.equal(entry.value, target);
    }
    await h.pressAccessibility('Quay lại');
    const fallback = h.settingsIo.trace.filter(row => row.kind === 'navigation').at(-1);
    assert.equal(fallback.method, 'replace'); assert.equal(fallback.value, '/profile');
    assert.equal(radio(h, 'Chỉ bạn bè').props['aria-checked'], true, 'read-only fixture policy remains server-selected');
    assert.equal(h.profileReads.length, baselineReads, 'local theme and navigation never refetch the profile');
    assert.equal(h.profileWrites.length + h.picks.length + h.uploads.length, 0, 'no policy or avatar operation exercised');
    actual(h); h.settingsIo.assertClean();
  });
});
