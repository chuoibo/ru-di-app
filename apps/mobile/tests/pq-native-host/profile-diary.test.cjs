// Actual Profile/Diary source; synthetic transport is the only diary boundary.
// These cases prove ownership and async state, not Yoga, image decode or beauty.
const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { createProfileHost, snapshot, act, sourceRoot } = require('./profile-host.cjs');
global.IS_REACT_ACT_ENVIRONMENT = true;

function diary(id, overrides = {}) {
  return {
    id, owner_id: 'synthetic-A', outing_id: `synthetic-outing-${id}`, kind: 'moment', audience: 'private', revision: 1,
    created_at: '2026-10-03T10:00:00Z', updated_at: '2026-10-03T11:00:00Z',
    ...overrides,
    document: { title: `Synthetic saved title ${id}`, subtitle: `Synthetic saved subtitle ${id}`, cover_id: '', pages: [{ layout: 'note', heading: '', text: `Synthetic saved excerpt ${id}`, photo_ids: [] }], ai_generated: false, ...overrides.document },
  };
}
const page = (diaries, next_cursor = null) => ({ diaries, next_cursor });
const unavailable = () => ({ status: 'unavailable', reason: 'module-unavailable' });
function homeReader(h) {
  assert.equal(h.diaryReads.length, 1, 'profile home owns one initial diary read before any kept navigation');
}
function actual(h, wall = false) {
  const modules = ['apps/mobile/src/rudi/screens/Profile', 'apps/mobile/src/rudi/screens/profile/HoSoSong', 'apps/mobile/src/rudi/screens/profile/DiaryLead', 'apps/mobile/src/rudi/diary/useDiaryCollection', 'apps/mobile/src/rudi/diary/api', 'apps/mobile/src/rudi/ui/NativeFontMetrics'];
  if (wall) modules.push('apps/mobile/src/rudi/diary/Wall');
  for (const relative of modules) {
    const file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(sourceRoot + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
  assert.equal(h.boundaryViolations.length, 0);
  assert.equal(h.observation().maxLive, 1);
}
async function withHost(options, callback) {
  const h = createProfileHost(options);
  try { await h.mount(); await callback(h); } finally { if (h.root) await h.dispose(); }
}
function saved(h, book) {
  assert.equal(h.text(book.document.title).length, 1, 'accepted current diary title remains readable');
  if (book.document.subtitle) assert.equal(h.text(book.document.subtitle).length, 1);
  const excerpt = book.document.pages.find(item => item.text.trim() !== '')?.text;
  if (excerpt) assert.equal(h.text(excerpt).length, 1);
  const title = h.one('RNText', props => props.children === book.document.title);
  assert.equal(title.props.numberOfLines, undefined, 'saved title has no line cap');
  assert.equal(title.props.adjustsFontSizeToFit, undefined);
  assert.equal(h.text(`${book.kind === 'trip' ? 'Sổ chuyến đi' : 'Khoảnh khắc'} · ${book.audience === 'private' ? 'Chỉ mình tôi' : 'Công khai'}`).length, 1);
}
function transport(h) { return { calls: h.calls.length, profiles: h.profileReads.length, writes: h.profileWrites.length, sessions: h.sessionWrites.length, storage: h.storageCalls.length, native: h.nativeHostCalls.length, picks: h.picks.length, uploads: h.uploads.length, sockets: h.sockets.length }; }
function button(h, label) {
  const controls = h.hosts('RNPressable').filter(node => node.findAllByType('RNText').some(text => text.props.children === label));
  assert.equal(controls.length, 1, `One actual labeled button: ${label}`);
  return controls[0];
}

test('PD01 preview and full shelf share the exact actor-scoped server page through tabs and hot metrics', async () => {
  const first = diary('first'), older = diary('older', { kind: 'trip', audience: 'public' });
  await withHost({ diaries: [first, older] }, async h => {
    homeReader(h);
    const owner = h.owner('ProfileContent'), profile = h.owner('HoSoSong');
    assert.equal(h.diaryReads.length, 1);
    assert.equal(h.diaryReads[0].route, '/people/synthetic-A/diaries?before=');
    assert.equal(h.diaryReads[0].options.actorId, 'synthetic-A');
    saved(h, first);
    assert.equal(h.text(older.document.title).length, 0, 'preview chooses the first server-ordered page only');
    assert.equal(h.text('Những ngày muốn giữ').length, 0, 'profile lead does not add a static wrapper before the real title');
    assert.equal(h.hosts('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${first.document.title}`).length, 0, 'no cover means no substitute art');
    await h.pressText('Mở trang'); assert.equal(h.routes.at(-1), '/diaries/first');
    const before = transport(h);
    for (const value of [snapshot(2), unavailable(), snapshot()]) {
      await h.nativeFont(value);
      await h.update(state => { state.window = { ...state.window, width: value.fontScale === 2 ? 320 : 768 }; });
      await h.pressText('Trang đã giữ');
      assert.equal(h.text(first.document.title).length, 1); assert.equal(h.text(older.document.title).length, 1);
      await h.pressAccessibility(`Mở ${older.document.title}`); assert.equal(h.routes.at(-1), '/diaries/older');
      await h.pressText('Hồ sơ'); saved(h, first);
      assert.ok(h.owner('ProfileContent') === owner); assert.ok(h.owner('HoSoSong') === profile);
      assert.deepEqual(transport(h), before, 'font/window/tab changes do not read, write or touch native I/O');
      assert.equal(h.diaryReads.length, 1);
    }
    actual(h, true);
  });
});

test('PD02 loading failure retry and empty stay distinct without fabricated content or duplicate retry reads', async () => {
  await withHost({ deferDiary: true }, async h => {
    homeReader(h);
    assert.equal(h.hosts('RNView', props => props.accessibilityLabel === 'Đang đọc trang đã giữ').length, 1);
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-diary-empty').length, 0);
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-diary-error').length, 0);
    await h.fail(h.diaryReads[0], h.apiError('Synthetic diary read refusal'));
    assert.equal(h.text('Synthetic diary read refusal').length, 1);
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-diary-empty').length, 0, 'a refusal is not an empty shelf');
    const retry = button(h, 'Thử lại').props.onPress;
    h.deferDiaryRead();
    await act(async () => { retry(); retry(); });
    assert.equal(h.diaryReads.length, 2, 'retained rapid retry cannot launch a second in-flight read');
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-diary-empty').length, 0);
    await h.finish(h.diaryReads[1], page([]));
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-diary-error').length, 0);
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-diary-empty').length, 1);
    assert.equal(h.text('Chưa có trang đã giữ').length, 1);
    assert.equal(h.hosts('RNExpoImage', props => /^Ảnh bìa /.test(props.accessibilityLabel ?? '')).length, 0);
    await h.pressText('Xem kèo của nhóm'); assert.equal(h.routes.at(-1), '/(tabs)/plan');
    actual(h);
  });
});

test('PD03 shelf pagination preserves the latest preview and retries the failed cursor without replacing accepted pages', async () => {
  const first = diary('first'), second = diary('second'), third = diary('third');
  await withHost({ diaries: [first], diaryNext: 'synthetic-cursor-1' }, async h => {
    homeReader(h);
    await h.pressText('Trang đã giữ');
    const more = button(h, 'Mở những trang trước').props.onPress;
    h.deferDiaryRead();
    await act(async () => { more(); more(); });
    assert.equal(h.diaryReads.length, 2, 'pagination shares the home reader and starts only one next-page read');
    assert.equal(h.diaryReads[1].route, '/people/synthetic-A/diaries?before=synthetic-cursor-1');
    await h.finish(h.diaryReads[1], page([second], 'synthetic-cursor-2'));
    assert.equal(h.text(first.document.title).length, 1); assert.equal(h.text(second.document.title).length, 1);
    h.deferDiaryRead(); await h.pressText('Mở những trang trước');
    await h.fail(h.diaryReads[2], h.apiError('Synthetic earlier-page refusal'));
    assert.equal(h.text(first.document.title).length, 1); assert.equal(h.text(second.document.title).length, 1);
    assert.equal(h.text('Synthetic earlier-page refusal').length, 1);
    h.deferDiaryRead(); await h.pressText('Thử lại');
    assert.equal(h.diaryReads[3].route, '/people/synthetic-A/diaries?before=synthetic-cursor-2', 'retry keeps the failed pagination cursor');
    assert.equal(h.diaryReads[3].options.actorId, 'synthetic-A');
    await h.finish(h.diaryReads[3], page([third]));
    for (const book of [first, second, third]) assert.equal(h.text(book.document.title).length, 1);
    await h.pressText('Hồ sơ'); saved(h, first);
    assert.equal(h.text(second.document.title).length, 0); assert.equal(h.text(third.document.title).length, 0);
    assert.equal(h.diaryReads.length, 4); actual(h, true);
  });
});

test('PD04 replacement actor ignores late prior-actor success and failure while keeping the accepted new page', async () => {
  for (const late of ['success', 'failure']) {
    await withHost({ deferDiary: true }, async h => {
      homeReader(h);
      const old = h.diaryReads[0];
      h.deferDiaryRead();
      await h.actorTo('synthetic-B');
      assert.equal(h.diaryReads.length, 2);
      const current = h.diaryReads[1], book = diary('actor-B', { owner_id: 'synthetic-B', audience: 'public' });
      assert.equal(current.route, '/people/synthetic-B/diaries?before=');
      assert.equal(current.options.actorId, 'synthetic-B');
      assert.equal(h.hosts('RNView', props => props.testID === 'profile-saved-diary').length, 0, 'actor replacement hides old pages before its read answers');
      await h.finish(current, page([book])); saved(h, book);
      if (late === 'success') await h.finish(old, page([diary('actor-A-private')]));
      else await h.fail(old, h.apiError('Synthetic stale-actor refusal'));
      saved(h, book);
      assert.equal(h.text(diary('actor-A-private').document.title).length, 0);
      assert.equal(h.text('Synthetic stale-actor refusal').length, 0);
      await h.pressText('Mở trang'); assert.equal(h.routes.at(-1), '/diaries/actor-B');
      assert.equal(h.diaryReads.length, 2); actual(h);
    });
  }
});

test('PD05 retained retry and late response cannot revive a disposed reader; remount owns a fresh read', async () => {
  await withHost({ deferDiary: true }, async h => {
    homeReader(h);
    await h.fail(h.diaryReads[0], h.apiError('Synthetic refusal before disposal'));
    const retry = button(h, 'Thử lại').props.onPress;
    h.deferDiaryRead(); await act(async () => retry());
    const parked = h.diaryReads[1];
    await h.update(state => { state.show = false; });
    const before = transport(h);
    await act(async () => retry());
    await h.finish(parked, page([diary('disposed-private')]));
    assert.deepEqual(transport(h), before, 'disposed callbacks and responses produce no product I/O');
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-saved-diary').length, 0);
    const current = diary('fresh-mount');
    await h.update(state => { state.diaryPage = page([current]); state.show = true; });
    assert.equal(h.diaryReads.length, 3); saved(h, current);
    assert.equal(h.text(diary('disposed-private').document.title).length, 0);
    assert.equal(h.observation().live, 1);
    assert.equal(h.boundaryViolations.length, 0);
  });
});

test('PD06 actual cover auth source stays stable through metrics and a failed image leaves only saved words', async () => {
  const book = diary('cover', { document: { cover_id: 'synthetic-photo', subtitle: '', pages: [{ layout: 'note', heading: '', text: '', photo_ids: [] }] } });
  await withHost({ diaries: [book] }, async h => {
    homeReader(h);
    const image = h.one('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${book.document.title}`);
    const source = image.props.source, retiredAError = image.props.onError, before = transport(h);
    assert.equal(source.uri, 'https://synthetic.invalid/diaries/cover/photos/synthetic-photo');
    assert.equal(source.headers.Authorization, 'Bearer synthetic-only');
    assert.equal(image.props.cachePolicy, 'none');
    for (const value of [snapshot(2), unavailable(), snapshot()]) {
      await h.nativeFont(value);
      await h.update(state => { state.window = { ...state.window, width: 320 }; });
      assert.ok(h.one('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${book.document.title}`) === image);
      assert.ok(image.props.source === source, 'font and width do not replace the auth image source');
      assert.deepEqual(transport(h), before);
    }
    await act(async () => image.props.onError({ error: 'Synthetic image refusal' }));
    assert.equal(h.hosts('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${book.document.title}`).length, 0);
    saved(h, book);
    assert.equal(h.text('Ảnh chưa mở được. Nội dung vẫn còn đây.').length, 1);
    await h.nativeFont(snapshot(2));
    assert.equal(h.hosts('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${book.document.title}`).length, 0, 'font change does not retry a failed image');
    assert.equal(h.text('Synthetic saved excerpt cover').length, 0, 'no saved text means no invented excerpt');
    assert.deepEqual(transport(h), before);
    const next = diary('cover-B', { document: { cover_id: 'synthetic-photo-B' } });
    const savedOwner = h.owner('SavedDiary');
    await h.profileFocus(false);
    await h.update(state => { state.diaryPage = page([next]); });
    await h.profileFocus(true);
    assert.equal(h.diaryReads.length, 2);
    assert.ok(h.owner('SavedDiary') === savedOwner, 'new accepted diary changes the source in the same mounted owner');
    saved(h, next);
    const imageB = h.one('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${next.document.title}`);
    assert.equal(imageB.props.source.uri, 'https://synthetic.invalid/diaries/cover-B/photos/synthetic-photo-B');
    assert.notEqual(imageB.props.source, source);
    const retiredBError = imageB.props.onError;
    await act(async () => retiredBError({ error: 'Synthetic B image refusal' }));
    assert.equal(h.hosts('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${next.document.title}`).length, 0);
    const accepted = transport(h);
    await act(async () => retiredAError({ error: 'Synthetic late A image refusal' }));
    assert.equal(h.hosts('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${next.document.title}`).length, 0, 'retired A error must not overwrite B failure and remount/retry B');
    saved(h, next);
    await h.nativeFont(snapshot());
    assert.equal(h.hosts('RNExpoImage', props => props.accessibilityLabel === `Ảnh bìa ${next.document.title}`).length, 0);
    assert.deepEqual(transport(h), accepted); actual(h);
    await h.update(state => { state.show = false; });
    const disposed = transport(h);
    await act(async () => { retiredAError({ error: 'Synthetic disposed A' }); retiredBError({ error: 'Synthetic disposed B' }); });
    assert.equal(h.hosts('RNView', props => props.testID === 'profile-saved-diary').length, 0);
    assert.deepEqual(transport(h), disposed, 'disposed image callbacks cannot revive the surface or issue I/O');
    assert.equal(h.boundaryViolations.length, 0);
  });
});

test('PD07 navigation refocus revalidates the one cached page without remount and blurred callbacks stay inert', async () => {
  const first = diary('before-focus'), latest = diary('after-focus');
  await withHost({ diaries: [first] }, async h => {
    homeReader(h);
    const owner = h.owner('ProfileContent'), profile = h.owner('HoSoSong');
    const reader = h.owner('DiaryLead').props.collection;
    await h.profileFocus(false);
    const before = transport(h);
    await act(async () => { reader.reload(); reader.retry(); reader.more(); });
    assert.deepEqual(transport(h), before, 'blurred controls do not read');
    saved(h, first);
    h.deferDiaryRead();
    await h.profileFocus(true);
    assert.equal(h.diaryReads.length, 2, 'one new focus revalidates exactly one diary page');
    assert.equal(h.diaryReads[1].route, '/people/synthetic-A/diaries?before=');
    saved(h, first);
    assert.equal(h.text('Đang cập nhật trang đã giữ…').length, 1, 'accepted cache remains readable while focus refresh is pending');
    assert.ok(h.owner('ProfileContent') === owner); assert.ok(h.owner('HoSoSong') === profile);
    await h.finish(h.diaryReads[1], page([latest])); saved(h, latest);
    assert.equal(h.text(first.document.title).length, 0);
    const accepted = transport(h);
    await h.profileFocus(true);
    await h.nativeFont(snapshot(2)); await h.pressText('Trang đã giữ'); await h.pressText('Hồ sơ');
    assert.deepEqual(transport(h), accepted, 'already-focused state, font and tabs do not refetch');
    actual(h, true);
  });
});
