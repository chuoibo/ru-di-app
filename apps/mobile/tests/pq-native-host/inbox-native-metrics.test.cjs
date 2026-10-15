// Actual Inbox, StoryRail, Field, metrics and phien/wire helpers; native/I/O/session-hook seams only.
// Expo Messages route wiring is inspected, not executed. No native pixels/caret/crypto proof.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./owner-host.cjs');
const previousAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (previousAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = previousAct; });
const plain = value => JSON.parse(JSON.stringify(value));
const groupName = 'Mùa thu ở góc phố · Hội bạn đi bộ và những cuộc hẹn nhỏ';
const pairName = 'Bình kiểm thử với một tên đầy đủ thật dài';
const authorName = 'Cẩm Tú kiểm thử và những câu chuyện ở góc phố';
const contextIds = ['synthetic-group-1', 'synthetic-group-2', 'synthetic-pair-1', 'synthetic-invite-1'];
const io = { contexts: contextIds.map((id, index) => ({ id, membership: `synthetic-membership-${index + 1}` })), people: ['synthetic-A', 'synthetic-B', 'synthetic-C'], inviteIds: ['synthetic-invite-1'] };
const narrow = { width: 320, height: 640, fontScale: 1, scale: 2 };
function catalogue({ invited = true } = {}) {
  const rows = [
    { id: contextIds[0], display_name: groupName, my_state: 'active', my_role: 'admin', membership_id: io.contexts[0].membership, member_count: 12, unread_count: 100, kind: 'group', theme: 'sunset', last_message: { id: 'synthetic-message-1', kind: 'text', preview: 'Lời cũ trên máy chủ', author_id: 'synthetic-B', author_display_name: pairName, created_at: '2026-10-01T11:00:00Z' } },
    { id: contextIds[1], display_name: 'Nhóm kiểm thử yên tĩnh', my_state: 'active', my_role: 'member', membership_id: io.contexts[1].membership, member_count: 3, unread_count: 0, kind: 'group', last_message: { id: 'synthetic-message-2', kind: 'text', preview: 'Hẹn nhau ở góc phố', author_id: 'synthetic-A', author_display_name: 'Tôi kiểm thử', created_at: '2026-10-01T10:00:00Z' } },
    { id: contextIds[2], display_name: 'Tên nhóm không được dùng cho pair', my_state: 'active', my_role: 'member', membership_id: io.contexts[2].membership, member_count: 2, unread_count: 2, kind: 'pair', unavailable: true, counterpart: { id: 'synthetic-B', display_name: pairName }, last_message: { id: 'synthetic-message-3', kind: 'text', preview: 'Một lời nhắn dài để kiểm tra cách đọc đủ nội dung', author_id: 'synthetic-B', author_display_name: pairName, created_at: 'invalid-date' } },
  ];
  if (invited) rows.push({ id: contextIds[3], display_name: 'Lời mời kiểm thử cuối tuần', my_state: 'invited', my_role: 'member', membership_id: io.contexts[3].membership, member_count: 4, unread_count: 0, kind: 'group', last_message: null });
  return { contexts: rows };
}
function summaries() { return { rooms: [{ conversation_id: contextIds[0], last_sequence: 8, last_at: '2026-10-01T12:30:00Z', last_actor_id: 'synthetic-B', unread: 100, read_sequence: 3 }] }; }
function stories() { return { authors: [{ author: { id: 'synthetic-C', display_name: authorName }, all_seen: false, stories: [] }, { author: { id: 'synthetic-A', display_name: 'Tên tôi từ wire' }, all_seen: true, stories: [] }] }; }
function invitation() { return { context_id: contextIds[3], display_name: 'Lời mời kiểm thử cuối tuần', invited_by: { id: 'synthetic-C', display_name: 'Cẩm Tú kiểm thử' }, member_count: 4, invited_at: '2026-10-01T09:00:00Z' }; }
function text(value) {
  if (value == null || typeof value === 'boolean') return '';
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  if (Array.isArray(value)) return value.map(text).join('');
  return value.props ? text(value.props.children) : '';
}
const semantic = value => text(value).replace(/\u00a0/g, ' ').replace(/\u2060/g, '');
function one(values, message) { assert.equal(values.length, 1, message); return values[0]; }
function leaf(owner, copy) { return one(owner.findAllByType('RNText').filter(node => semantic(node.props.children) === copy), `one complete copy: ${copy}`); }
function ancestor(node, type) { for (let at = node.parent; at; at = at.parent) if (at.type === type) return at; assert.fail('Missing actual ancestor ' + String(type)); }
function component(h, module, name) { return one(h.root.root.findAllByType(h.load(module)[name]), 'one actual ' + name); }
function screen(h) { return component(h, 'apps/mobile/src/rudi/screens/groups/Conversations', 'ConversationsScreen'); }
function rail(h) { return component(h, 'apps/mobile/src/rudi/screens/story/StoryRail', 'StoryRail'); }
function row(h, name, pair = false) { return h.one('RNPressable', props => props.accessibilityLabel === (pair ? `Mở cuộc trò chuyện với ${name}` : `Mở nhóm ${name}`)); }
function group(h) { return row(h, groupName); }
function converted(h, node, bases, input = false) {
  assert.equal(node.props.allowFontScaling, false, 'actual native-table conversion');
  ancestor(node, input ? h.metrics.TextInput : h.metrics.Text);
  const provider = ancestor(node, h.metrics.NativeFontMetricsProvider);
  assert.equal(provider.props.snapshot.status, 'available');
  const style = flatten(node.props.style), table = h.state.nativeFont.fontMetrics.spToDp;
  for (const [key, base] of Object.entries(bases)) assert.equal(style[key], one(table.filter(item => item.base === base), `native table ${base}`).dp, `exact nonlinear ${key}`);
  assert.equal(node.props.maxFontSizeMultiplier, undefined); assert.notEqual(node.props.adjustsFontSizeToFit, true);
}
function raw(node, size, height) { assert.notEqual(node.props.allowFontScaling, false); const style = flatten(node.props.style); assert.equal(style.fontSize, size); assert.equal(style.lineHeight, height); }
function counters(h) { return { api: h.calls.length, store: h.inbox.storage.length, updates: h.inbox.sessionUpdates.length, attempts: h.attemptCount(), routes: h.routes.length, native: h.nativeHostCalls.length, picks: h.picks.length, uploads: h.uploads.length, sockets: h.sockets.length, modules: h.inbox.modules.length, actor: h.state.actor.person_id, token: h.state.token, context: h.state.actor.context_id, membership: h.state.actor.membership_id }; }
// Font getter/subscription counts live in observation(); nativeHostCalls records nonfont native side effects.
function noFontIO(h, held) { assert.deepEqual(counters(h), held, 'font event cannot create API/storage/session/attempt/navigation/nonfont native I/O'); h.inbox.assertClean(); }
async function press(node) { assert.equal(Boolean(node.props.disabled), false); await act(async () => node.props.onPress()); }
async function beginRefresh(h) {
  const actual = component(h, 'apps/mobile/src/rudi/ui', 'RudiScreen');
  const before = { catalogue: h.inbox.catalogueReads.length, summary: h.inbox.summaryReads.length, story: h.inbox.storyReads.length };
  let promise, status = 'pending';
  // Capture the real nap Promise without returning it to act while its I/O is parked.
  await act(async () => {
    promise = actual.props.onRefresh();
    assert.equal(typeof promise?.then, 'function', 'actual refresh returns its completion Promise');
    // Observe settlement only; finishRefresh still awaits this original Promise and propagates rejection.
    promise.then(() => { status = 'fulfilled'; }, () => { status = 'rejected'; });
  });
  assert.equal(h.inbox.catalogueReads.length, before.catalogue + 1, 'refresh admits one actual catalogue read');
  assert.equal(h.inbox.summaryReads.length, before.summary + 1, 'refresh admits one actual summary read');
  assert.equal(h.inbox.storyReads.length, before.story, 'catalogue refresh does not read stories');
  assert.equal(status, 'pending', 'refresh remains pending before explicit transport completion');
  return { promise, status: () => status, catalogue: h.inbox.catalogueReads[before.catalogue], summary: h.inbox.summaryReads[before.summary] };
}
async function finishRefresh(refresh) {
  await act(async () => { await refresh.promise; });
  assert.equal(refresh.status(), 'fulfilled', 'the same actual refresh Promise completed');
}
async function finishReads(h, data = catalogue(), story = stories(), summary = summaries()) {
  const startInvites = h.inbox.invitationReads.length;
  await h.finish(h.inbox.catalogueReads.at(-1), data);
  await h.finish(h.inbox.summaryReads.at(-1), summary);
  for (const call of h.inbox.invitationReads.slice(startInvites)) await h.finish(call, invitation());
  if (story !== null) await h.finish(h.inbox.storyReads.at(-1), story);
  await h.flushInboxTimers(); h.inbox.assertClean();
}
async function inbox(options, run) {
  const h = createHost({ surface: 'inbox', inboxIO: io, nativeGeometry: true, initialWindow: narrow, ...options });
  // Load closure before mount: a missing dependency cannot be hidden by a React retry.
  h.load('apps/mobile/src/rudi/screens/groups/Conversations');
  try { await h.mount(); await run(h); h.inbox.assertClean(); } finally {
    await h.dispose();
    const disposed = plain(h.observation());
    assert.equal(disposed.live, 0, 'Inbox observers are removed on dispose');
    assert.equal(disposed.removals, disposed.subscriptions, 'Every actual Inbox subscription is removed once');
    assert.equal(h.inbox.appState.size, 0, 'Inbox AppState observers are removed on dispose');
    h.inbox.assertClean();
    const held = counters(h);
    for (let index = 0; index < h.retiredCallbacks.length; index += 1) await h.invokeRetired(index);
    noFontIO(h, held);
    assert.deepEqual(plain(h.observation()), disposed, 'Queued retired notifications cannot even reread native font metrics');
  }
}
function sourceProof(h) {
  for (const module of ['apps/mobile/src/rudi/screens/groups/Conversations', 'apps/mobile/src/rudi/screens/story/StoryRail', 'apps/mobile/src/phien', 'apps/mobile/src/phien-web', 'apps/mobile/src/rudi/chat/e2ee/api-v2', 'apps/mobile/src/rudi/chat/e2ee/tom-tat', 'apps/mobile/src/rudi/chat/e2ee/useTinNhanV2', 'apps/mobile/modules/rudi-chat-crypto', 'apps/mobile/src/rudi/nhom/loi-moi-nhom', 'apps/mobile/src/rudi/nhom/tim-cuoc-tro-chuyen', 'apps/mobile/src/rudi/nhan-rieng/nhan-rieng', 'apps/mobile/src/rudi/story/story', 'apps/mobile/src/rudi/chat/tin-song', 'apps/mobile/src/rudi/ui/NativeFontMetrics', 'apps/mobile/src/rudi/ui/Field', 'apps/mobile/src/rudi/ui/AvatarNguoi', 'apps/mobile/src/rudi/nguoi/anh-dai-dien', 'apps/mobile/src/rudi/nep/NepProvider']) {
    const file = h.consumed.get(module); assert.ok(file, `actual source consumed: ${module}`);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true);
    assert.equal(h.consumedHashes.get(module), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
  assert.equal(h.consumed.has('apps/mobile/src/rudi/session'), false, 'explicit session-hook boundary; session boot is not claimed');
  assert.deepEqual(plain(h.inbox.modules), ['RudiChatCrypto']);
  assert.equal(h.sockets.length, 0); assert.equal(h.uploads.length, 0); assert.equal(h.picks.length, 0);
}
function largeRow(h, expectedLarge) {
  const item = group(h), name = leaf(item, groupName), copy = ancestor(name, 'RNView'), body = ancestor(copy, 'RNView');
  assert.equal(flatten(body.props.style).flexDirection, expectedLarge ? 'column' : 'row');
  assert.equal(name.props.numberOfLines, expectedLarge ? undefined : 2);
  const preview = leaf(item, 'Thành viên: Tin nhắn mã hoá đầu cuối');
  const members = leaf(item, '12 thành viên · bạn quản trị');
  assert.equal(preview.props.numberOfLines, expectedLarge ? undefined : 1); assert.equal(members.props.numberOfLines, expectedLarge ? undefined : 1);
  assert.equal(ancestor(preview, 'RNView') === copy, true); assert.equal(ancestor(members, 'RNView') === copy, true, 'ordered sibling paragraphs');
  const directCopy = copy.children.filter(child => typeof child !== 'string').map(child => semantic(child.props.children));
  assert.deepEqual(directCopy, [groupName, 'Thành viên: Tin nhắn mã hoá đầu cuối', '12 thành viên · bạn quản trị']);
  assert.equal(flatten(preview.props.style).fontWeight, '700');
  const meta = ancestor(leaf(item, '100'), 'RNView');
  assert.equal(flatten(meta.props.style).height, undefined); assert.equal(flatten(meta.props.style).minHeight, 26);
  return { item, name, preview, members, copy, body };
}
function glyph(h) {
  const frame = one(group(h).findAllByType('RNView').filter(node => node.props.accessibilityElementsHidden === true && node.props.importantForAccessibility === 'no-hide-descendants'), 'one decorative hidden notebook');
  const face = one(frame.findAllByType('RNView').filter(node => flatten(node.props.style)?.width === 34), 'fixed notebook face');
  assert.equal(flatten(face.props.style).height, 48); assert.equal(flatten(face.props.style).overflow, 'hidden');
  const node = leaf(face, 'M'); assert.equal(node.props.allowFontScaling, false, 'only hidden pictogram has an explicit scaling opt-out');
  assert.equal(flatten(node.props.style).fontSize, 17); assert.equal(flatten(node.props.style).lineHeight, 23); assert.equal(flatten(node.props.style).letterSpacing, -0.15);
  return node;
}

test('IB01 actual Inbox reflows ordered long copy and stories on native 100→200→100 while preserving query input refs and decorative glyph', async () => {
  // Source wiring only: no Expo route or session boot executes in this host.
  const route = fs.readFileSync(path.join(sourceRoot, 'apps/mobile/app/(tabs)/messages.tsx'), 'utf8');
  assert.match(route, /<ConversationsScreen\s+phien=\{phien\}/); assert.match(route, /if \(!phienDaDoc\) return null/);
  await inbox({}, async h => {
    assert.equal(h.inbox.catalogueReads.length, 1); assert.equal(h.inbox.summaryReads.length, 1); assert.equal(h.inbox.storyReads.length, 1);
    await finishReads(h);
    const content = screen(h), controller = h.owner('ConversationsContent'), input = h.input(), ref = h.native.node(input), initialRail = rail(h);
    const heldRow = largeRow(h, false), heldGlyph = glyph(h), storyLabel = leaf(initialRail, authorName);
    assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1);
    converted(h, leaf(content, 'Tin nhắn'), { fontSize: 28, lineHeight: 34, letterSpacing: -0.65 });
    converted(h, input, { fontSize: 17, lineHeight: 24 }, true);
    const placeholder = leaf(content, 'Tìm người hoặc nhóm'); assert.equal(placeholder.props.allowFontScaling, false); assert.equal(flatten(placeholder.props.style).fontSize, 17);
    await h.native.focus(input); await h.change('mua thu');
    assert.equal(group(h) === heldRow.item, true, 'matching keyed row survives local query');
    const before = counters(h);
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale)); noFontIO(h, before);
      assert.equal(h.state.window.fontScale, 1); assert.equal(screen(h) === content, true); assert.equal(h.owner('ConversationsContent') === controller, true); assert.equal(rail(h) === initialRail, true);
      assert.equal(h.input() === input, true); assert.equal(h.native.node(input) === ref, true); assert.equal(ref.isFocused(), true); assert.equal(input.props.value, 'mua thu');
      const now = largeRow(h, scale === 2); assert.equal(now.item === heldRow.item, true); assert.equal(now.name === heldRow.name, true); assert.equal(now.preview === heldRow.preview, true);
      assert.equal(glyph(h) === heldGlyph, true); converted(h, now.name, { fontSize: 17, lineHeight: 23, letterSpacing: -0.15 });
      converted(h, now.preview, { fontSize: 13, lineHeight: 18 }); converted(h, now.members, { fontSize: 13, lineHeight: 18 }); converted(h, input, { fontSize: 17, lineHeight: 24 }, true);
      assert.equal(leaf(rail(h), authorName) === storyLabel, true); assert.equal(storyLabel.props.numberOfLines, undefined); converted(h, storyLabel, { fontSize: 13, lineHeight: 18 });
      const tile = ancestor(storyLabel, 'RNPressable'); assert.equal(flatten(tile.props.style).width, scale === 2 ? 112 : 72);
      assert.equal(h.observation().live, 1); assert.equal(h.observation().subscriptions, 1); assert.equal(h.observation().removals, 0); assert.equal(h.observation().maxLive, 1); assert.ok(h.observation().reads >= 3);
    }
    await h.change(''); await h.flushInboxTimers(); const cleared = counters(h); await h.nativeFont(snapshot(2)); noFontIO(h, cleared);
    const hint = leaf(content, 'Tìm người hoặc nhóm'); assert.equal(hint.props.allowFontScaling, false); assert.equal(flatten(hint.props.style).fontSize, 28.75); assert.equal(flatten(hint.props.style).lineHeight, 39.25);
    await h.nativeFont(snapshot()); noFontIO(h, cleared); assert.equal(h.input() === input, true); assert.equal(ref.isFocused(), true);
    sourceProof(h);
  });
});

test('IB02 inherited unavailable missing and partial native tables keep one owner and the same search/name through fallback recovery', async () => {
  const partial = snapshot(2); partial.fontMetrics.spToDp = partial.fontMetrics.spToDp.filter(row => row.base !== 24);
  for (const unavailable of [{ status: 'unavailable', reason: 'module-inactive' }, { status: 'available', fontScale: 2 }, { status: 'available', fontScale: 2, fontMetrics: { density: 2, spToDp: [] } }, partial]) {
    await inbox({ explicitProvider: true, inheritedInboxMetrics: true, initialSnapshot: unavailable }, async h => {
      await finishReads(h); const input = h.input(), content = screen(h), name = largeRow(h, true).name;
      raw(input, 17, 24); assert.deepEqual(plain(h.observation()), { live: 0, subscriptions: 0, removals: 0, reads: 0, maxLive: 0 });
      assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1);
      assert.equal(leaf(rail(h), authorName).props.numberOfLines, undefined); glyph(h);
      await h.change('mua thu'); const held = counters(h);
      await h.nativeFont(snapshot(2)); noFontIO(h, held); assert.equal(screen(h) === content, true); assert.equal(h.input() === input, true); assert.equal(largeRow(h, true).name === name, true);
      converted(h, input, { fontSize: 17, lineHeight: 24 }, true); converted(h, name, { fontSize: 17, lineHeight: 23, letterSpacing: -0.15 });
      await h.nativeFont(unavailable); noFontIO(h, held); assert.equal(h.input() === input, true); assert.equal(input.props.value, 'mua thu'); raw(input, 17, 24);
      assert.deepEqual(plain(h.observation()), { live: 0, subscriptions: 0, removals: 0, reads: 0, maxLive: 0 }); h.inbox.assertClean();
    });
  }
  await inbox({ nativeFontModulePresent: false, initialSnapshot: snapshot(2) }, async h => {
    await finishReads(h); raw(h.input(), 17, 24); raw(largeRow(h, true).name, 17, 23); glyph(h);
    assert.deepEqual(plain(h.observation()), { live: 0, subscriptions: 0, removals: 0, reads: 0, maxLive: 0 }); h.inbox.assertClean();
  });
});

test('IB03 actual shared StoryRail preserves default RN compatibility and opt-in full labels with no child observer or lost routes on error', async () => {
  for (const optin of [false, true]) {
    await inbox({ inboxView: 'story', explicitProvider: true, inheritedInboxMetrics: true, initialSnapshot: snapshot(2), inboxStoryProps: optin ? { nativeMetrics: true } : {} }, async h => {
      assert.equal(h.inbox.catalogueReads.length, 0); assert.equal(h.inbox.summaryReads.length, 0); assert.equal(h.inbox.storyReads.length, 1);
      const compose = h.one('RNPressable', props => props.accessibilityLabel === 'Đăng story mới');
      assert.equal(leaf(compose, 'Đăng story').props.numberOfLines, optin ? undefined : 1);
      await h.finish(h.inbox.storyReads[0], stories()); await h.flushInboxTimers();
      const label = leaf(rail(h), authorName), tile = ancestor(label, 'RNPressable');
      assert.equal(label.props.numberOfLines, optin ? undefined : 1); assert.equal(flatten(tile.props.style).width, optin ? 112 : 72);
      if (optin) converted(h, label, { fontSize: 13, lineHeight: 18 }); else raw(label, 13, 18);
      assert.equal(tile.props.accessibilityLabel, `Story của ${authorName}, chưa xem`); assert.equal(leaf(rail(h), 'Bạn').props.numberOfLines, optin ? undefined : 1);
      await press(compose); await press(tile); assert.deepEqual(plain(h.routes), ['/stories/new', '/stories/synthetic-C']);
      // A fresh rail failure keeps compose reachable; retry loads authors. Injected refusal is expected.
      await act(async () => [...h.inbox.appState].forEach(callback => callback('active'))); assert.equal(h.inbox.storyReads.length, 1, 'Story focus does not invent an AppState listener');
      const railInstance = rail(h); await h.update(state => { state.show = false; }); await h.update(state => { state.show = true; });
      assert.equal(rail(h) === railInstance, false, 'explicit hide/show is a fresh rail, not a font transition');
      await h.fail(h.inbox.storyReads[1], new Error('Synthetic story offline'));
      const failureCompose = h.one('RNPressable', props => props.accessibilityLabel === 'Đăng story mới'); assert.equal(Boolean(failureCompose.props.disabled), false);
      await h.pressText('Đọc lại story'); assert.equal(h.inbox.storyReads.length, 3); await h.finish(h.inbox.storyReads[2], stories()); await h.flushInboxTimers();
      assert.equal(leaf(rail(h), authorName).props.numberOfLines, optin ? undefined : 1);
      assert.deepEqual(plain(h.observation()), { live: 0, subscriptions: 0, removals: 0, reads: 0, maxLive: 0 }); h.inbox.assertClean();
    });
  }
});

test('IB04 strict caught-error ledgers and actual catalogue/v2 merge preserve unread invitation order dates author copy and preview hierarchy', async () => {
  await inbox({}, async h => {
    const api = h.load('apps/mobile/src/api');
    await assert.rejects(() => api.translatedAsActor({}, '/stories', { method: 'POST', actorId: 'synthetic-A' }), /Unplanned/);
    assert.equal(h.inbox.unexpected.length, 1); assert.equal(h.inbox.unexpected[0].kind, 'transport');
    h.inbox.unexpected.splice(0); // The sentinel refusal has been explicitly observed; scenario ledger starts clean.
    await assert.rejects(() => h.inbox.secureStore.setItemAsync('wrong-key', '{}'));
    assert.equal(h.inbox.unexpected.length, 1); assert.equal(h.inbox.unexpected[0].kind, 'secureStore.set'); h.inbox.unexpected.splice(0);
    await finishReads(h);
    const rows = screen(h).findAllByType('RNPressable').filter(node => /^Mở (nhóm|cuộc trò chuyện với) /.test(node.props.accessibilityLabel ?? ''));
    assert.equal(rows.length, 4); assert.equal(rows[0].props.accessibilityLabel, 'Mở nhóm Lời mời kiểm thử cuối tuần'); assert.equal(rows[0].props.disabled, true);
    assert.equal(leaf(rows[0], 'Cẩm Tú kiểm thử mời bạn · 4 người trong nhóm').props.children.length > 0, true);
    const unread = group(h); leaf(unread, 'Thành viên: Tin nhắn mã hoá đầu cuối'); assert.equal(unread.findAllByType('RNText').some(node => semantic(node.props.children).includes('Lời cũ trên máy chủ')), false);
    assert.equal(one(unread.findAllByType('RNView').filter(node => node.props.accessibilityLabel === '100 tin chưa đọc'), 'actual unread100').props.accessibilityLabel, '100 tin chưa đọc');
    const time = h.load('apps/mobile/src/rudi/chat/tin-song').gioPhut('2026-10-01T12:30:00Z'); leaf(unread, time);
    const read = row(h, 'Nhóm kiểm thử yên tĩnh'); const preview = leaf(read, 'Bạn: Hẹn nhau ở góc phố'); assert.equal(flatten(preview.props.style).fontWeight, '400');
    const pair = row(h, pairName, true); leaf(pair, 'Không còn nhận tin'); assert.equal(pair.findAllByType('RNText').some(node => semantic(node.props.children) === 'Invalid Date'), false);
    assert.equal(pair.findAllByType('RNText').some(node => semantic(node.props.children) === 'Nhắn riêng'), false, 'unavailable message takes precedence over pair metadata');
    assert.equal(h.inbox.accepts.length, 0); assert.equal(h.inbox.declines.length, 0); assert.equal(h.attemptCount(), 0);
    assert.equal(h.inbox.storage.filter(call => call.method === 'set').length, 1, 'actual catalogue merge writes the session once'); sourceProof(h);
  });
});

test('IB05 actual group selection keeps synchronous locks across pending native200 and pair navigation preserves the current group', async () => {
  await inbox({}, async h => {
    await finishReads(h); await h.change('mua thu');
    const input = h.input(), ref = h.native.node(input), content = screen(h), retained = group(h).props.onPress;
    h.inbox.deferSessionWrite(); await act(async () => { retained(); retained(); }); assert.equal(h.inbox.pendingStores.length, 1); assert.equal(h.routes.length, 0);
    const pending = h.inbox.pendingStores[0]; assert.equal(pending.value.context_id, contextIds[0]);
    const held = counters(h); await h.nativeFont(snapshot(2)); noFontIO(h, held); converted(h, input, { fontSize: 17, lineHeight: 24 }, true);
    assert.equal(h.input() === input, true); assert.equal(h.native.node(input) === ref, true); assert.equal(screen(h) === content, true); assert.equal(input.props.value, 'mua thu'); assert.equal(group(h).props.disabled, true);
    await act(async () => retained()); assert.equal(h.inbox.pendingStores.length, 1);
    await h.finish(pending, undefined); assert.deepEqual(plain(h.routes), [`/groups/${contextIds[0]}/chat`]);
    assert.equal(h.inbox.sessionUpdates.at(-1).context_id, contextIds[0]); assert.equal(h.inbox.sessionUpdates.at(-1).membership_id, io.contexts[0].membership);
    await h.change('binh'); const stores = h.inbox.storage.length, updates = h.inbox.sessionUpdates.length;
    await press(row(h, pairName, true)); assert.equal(h.inbox.storage.length, stores); assert.equal(h.inbox.sessionUpdates.length, updates); assert.equal(h.state.actor.context_id, contextIds[0]);
    assert.deepEqual(plain(h.routes), [`/groups/${contextIds[0]}/chat`, `/groups/${contextIds[2]}/chat`]);
    const settled = counters(h); await h.nativeFont(snapshot()); noFontIO(h, settled); assert.equal(input.props.value, 'binh'); assert.equal(h.attemptCount(), 0); h.inbox.assertClean();
  });
});

test('IB06 actual invitation confirmation and accept/decline writers retain exact payload attempts and synchronous locks across native font changes', async () => {
  await inbox({}, async h => {
    await finishReads(h);
    const name = 'Lời mời kiểm thử cuối tuần';
    await h.pressAccessibility(`Từ chối lời mời vào nhóm ${name}`);
    const confirm = component(h, 'apps/mobile/src/rudi/ui/HoiTaiHang', 'HoiTaiHang');
    const question = `Từ chối lời mời vào ${name}? Nếu đổi ý, bạn cần được mời lại.`;
    converted(h, leaf(confirm, question), { fontSize: 13, lineHeight: 18 }); assert.equal(h.attemptCount(), 0); assert.equal(h.inbox.declines.length, 0);
    await h.pressText('Thôi'); assert.equal(h.inbox.declines.length, 0);
    await h.pressAccessibility(`Từ chối lời mời vào nhóm ${name}`);
    const current = component(h, 'apps/mobile/src/rudi/ui/HoiTaiHang', 'HoiTaiHang'), retained = current.props.onDongY;
    await act(async () => { retained(); retained(); }); assert.equal(h.inbox.declines.length, 1); assert.equal(h.attemptCount(), 1);
    const pending = h.inbox.declines[0]; assert.deepEqual(plain(pending.options), { method: 'DELETE', actorId: 'synthetic-A', attempt: { key: 'synthetic-attempt-1', at: Date.parse('2026-10-01T00:00:00Z') + 1 } });
    const before = counters(h); await h.nativeFont(snapshot(2)); noFontIO(h, before); assert.equal(current.props.dangLam, true); converted(h, leaf(current, question), { fontSize: 13, lineHeight: 18 });
    await act(async () => retained()); assert.equal(h.inbox.declines.length, 1); assert.equal(h.attemptCount(), 1);
    await h.fail(pending, apiError(h, 'Lời mời kiểm thử tạm thời chưa đổi')); leaf(screen(h), 'Lời mời kiểm thử tạm thời chưa đổi');
    await h.pressText('Thôi');
    const accept = h.one('RNPressable', props => props.accessibilityLabel === `Đồng ý vào nhóm ${name}`).props.onPress;
    await act(async () => { accept(); accept(); }); assert.equal(h.inbox.accepts.length, 1); assert.equal(h.attemptCount(), 1);
    assert.deepEqual(plain(h.inbox.accepts[0].options), { method: 'POST', actorId: 'synthetic-A' });
    const held = counters(h); await h.nativeFont(snapshot()); noFontIO(h, held); await act(async () => accept()); assert.equal(h.inbox.accepts.length, 1);
    await h.finish(h.inbox.accepts[0], { state: 'active' });
    assert.equal(h.inbox.sessionUpdates.at(-1).membership_state, 'active'); assert.equal(h.inbox.sessionUpdates.at(-1).membership_id, io.contexts[3].membership);
    assert.equal(h.inbox.catalogueReads.length, 2); assert.equal(h.inbox.summaryReads.length, 2);
    const refreshed = catalogue({ invited: false }); await finishReads(h, refreshed, null); assert.equal(h.routes.length, 0); h.inbox.assertClean();
  });
});
function apiError(h, message) { return new (h.load('apps/mobile/src/api').ApiError)(503, message); }

test('IB07 actual loading error ready-empty and no-match states keep recovery actions and native copy without changing their source contracts', async () => {
  await inbox({}, async h => {
    assert.equal(screen(h).findAllByType(h.load('apps/mobile/src/rudi/ui/Skeleton').SkeletonGroup).length, 1);
    await h.fail(h.inbox.catalogueReads[0], apiError(h, 'Chưa đọc được nhóm kiểm thử'));
    await h.finish(h.inbox.summaryReads[0], { rooms: [] }); await h.finish(h.inbox.storyReads[0], { authors: [] });
    const failed = component(h, 'apps/mobile/src/rudi/ui/ErrorState', 'ErrorState'); raw(leaf(failed, 'Chưa đọc được danh sách nhóm'), 21, 27); leaf(failed, 'Chưa đọc được nhóm kiểm thử');
    await h.pressText('Thử lại'); assert.equal(h.inbox.catalogueReads.length, 2);
    await finishReads(h, { contexts: [] }, null, { rooms: [] });
    const empty = component(h, 'apps/mobile/src/rudi/ui/EmptyState', 'EmptyState');
    converted(h, leaf(empty, 'Chưa có nhóm nào'), { fontSize: 21, lineHeight: 27, letterSpacing: -0.3 });
    const create = one(empty.findAllByType(h.load('apps/mobile/src/rudi/ui').RudiButton).filter(node => node.props.label === 'Tạo nhóm'), 'one actual empty create-group action');
    const createPressable = one(create.findAllByType('RNPressable'), 'one actual empty create-group pressable');
    leaf(createPressable, 'Tạo nhóm'); assert.equal(createPressable.props.accessibilityRole, 'button');
    await press(createPressable); assert.deepEqual(plain(h.routes), ['/groups/new']);
    await h.pressText('Thêm bạn bằng tên tài khoản'); assert.deepEqual(plain(h.routes), ['/groups/new', '/friends/add']);
    const refresh = await beginRefresh(h);
    await finishReads(h, catalogue(), null); await finishRefresh(refresh); await h.change('khong co ket qua kiem thu');
    leaf(screen(h), 'Chưa tìm thấy cuộc trò chuyện'); await h.pressText('Xoá tìm kiếm'); assert.equal(h.input().props.value, ''); assert.equal(group(h).props.disabled, false);
    const held = counters(h); await h.nativeFont(snapshot(2)); noFontIO(h, held); converted(h, leaf(screen(h), 'Tin nhắn'), { fontSize: 28, lineHeight: 34, letterSpacing: -0.65 });
    h.inbox.assertClean();
  });
});

test('IB08 actual catalogue token and unmount guards reject stale reads without session writes and ready refresh failures retain rows', async () => {
  await inbox({}, async h => {
    const oldCatalogue = h.inbox.catalogueReads[0], oldSummary = h.inbox.summaryReads[0];
    const next = { ...h.state.actor, person_id: 'synthetic-B', token: 'synthetic-token-synthetic-B', profile: { display_name: 'Tài khoản kiểm thử B' } };
    await h.inboxActorTo(next);
    assert.equal(h.state.token, next.token, 'Prop-session seam publishes token B before actor B');
    assert.equal(h.state.actor.token, next.token);
    assert.equal(h.inbox.catalogueReads.length, 2); assert.equal(h.inbox.summaryReads.length, 2); assert.equal(h.inbox.storyReads.length, 2);
    for (const queued of [h.inbox.catalogueReads[1], h.inbox.summaryReads[1], h.inbox.storyReads[1]]) {
      assert.equal(queued.options.actorId, 'synthetic-B'); assert.equal(queued.activeToken, next.token, 'Each queued B read captured the active B token');
    }
    await h.finish(oldCatalogue, catalogue()); await h.finish(oldSummary, summaries());
    assert.equal(h.inbox.storage.filter(call => call.method === 'set').length, 0); assert.equal(h.inbox.sessionUpdates.length, 0);
    await finishReads(h); assert.equal(h.inbox.catalogueReads[1].options.actorId, 'synthetic-B'); assert.equal(h.inbox.sessionUpdates.at(-1).person_id, 'synthetic-B');
    const heldRow = group(h), refusedRefresh = await beginRefresh(h);
    await h.fail(refusedRefresh.catalogue, apiError(h, 'Danh sách kiểm thử đang offline')); await h.finish(refusedRefresh.summary, { rooms: [] });
    await finishRefresh(refusedRefresh);
    assert.equal(group(h) === heldRow, true); leaf(screen(h), 'Chưa cập nhật được danh sách. Danh sách kiểm thử đang offline');
    const lateRefresh = await beginRefresh(h), writes = h.inbox.storage.length, updates = h.inbox.sessionUpdates.length;
    await h.update(state => { state.show = false; }); await h.finish(lateRefresh.catalogue, catalogue()); await h.finish(lateRefresh.summary, summaries());
    await finishRefresh(lateRefresh);
    assert.equal(h.inbox.storage.length, writes); assert.equal(h.inbox.sessionUpdates.length, updates); h.inbox.assertClean();
    // The first Story promise is intentionally left parked. Its actor-race semantics are outside this catalogue guard claim.
  });
});
