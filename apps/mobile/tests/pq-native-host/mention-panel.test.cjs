// Actual Comments, MentionPicker, MediaPicker and Field execute together.
// Boundary traces and native props do not simulate Android IME, layout or clipping.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./owner-host.cjs');
const previousAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const previousError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) previousError(...args); };
after(() => { console.error = previousError; if (previousAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = previousAct; });
const plain = value => JSON.parse(JSON.stringify(value));
const text = value => Array.isArray(value) ? value.map(text).join('') : typeof value === 'string' || typeof value === 'number' ? String(value) : '';
const inside = (node, ancestor) => { for (let parent = node.parent; parent; parent = parent.parent) if (parent === ancestor) return true; return false; };
const nearestView = node => { for (let parent = node.parent; parent; parent = parent.parent) if (parent.type === 'RNView') return parent; return null; };
const view = (h, id) => h.one('RNView', props => props.testID === id);
const scroll = (h, id) => h.one('RNScrollView', props => props.testID === id);
const button = (h, label) => h.one('RNPressable', props => props.accessibilityLabel === label);
const disclosure = h => h.one('RNPressable', props => /^(Thêm vào bình luận|Đóng công cụ bình luận)$/.test(props.accessibilityLabel));
const send = h => h.one('RNPressable', props => /^(Gửi bình luận|Đang gửi…)$/.test(props.accessibilityLabel));
const boxes = h => h.hosts('RNPressable', props => props.accessibilityRole === 'checkbox');
const dismisses = h => h.nativeHostCalls.filter(call => call.kind === 'keyboard.dismiss').length;
const friend = (id, display_name = `Synthetic Ánh ${id}`) => ({ person_id: id, display_name, friends_since: '2026-10-01T00:00:00Z' });
const asset = { uri: 'file:///synthetic-mention.jpg', width: 32, height: 32, type: 'image', mimeType: 'image/jpeg' };
async function click(h, label) { const control = button(h, label); assert.equal(Boolean(control.props.disabled), false, 'actual target is enabled'); await act(async () => control.props.onPress()); }
async function toggle(h) { const control = disclosure(h); assert.equal(Boolean(control.props.disabled), false); await act(async () => control.props.onPress()); }
async function open(h, label = 'Gắn bạn bè') { await toggle(h); await click(h, label); }
async function withHost(options, run, prepare = () => {}) {
  const h = createHost(options); h.state.window = { width: 320, height: 640, scale: 2, fontScale: 1 }; prepare(h);
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
function identities(h) { return { input: h.input(), field: h.root.root.findByType(h.load('apps/mobile/src/rudi/ui/Field').Field), comments: h.owner('CommentsForContext'), mentions: h.owner('MentionPicker'), media: h.owner('MediaPicker') }; }
function same(h, previous, body) {
  const current = identities(h);
  for (const name of Object.keys(previous)) assert.equal(current[name] === previous[name], true, `same ${name} host or controller within this writer`);
  assert.equal(h.input().props.value, body, 'exact controlled synthetic body');
}
function actual(h) {
  for (const name of ['community/Comments', 'community/MentionPicker', 'community/MediaPicker', 'ui/Field']) {
    const relative = `apps/mobile/src/rudi/${name}`, file = path.join(sourceRoot, relative + '.tsx');
    assert.equal(h.consumed.get(relative), file, 'actual source-root binding: ' + name);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'), 'exact consumed source bytes: ' + name);
  }
}
function panelVisible(node, visible) {
  const style = flatten(node.props.style);
  assert.equal(style.display, undefined, 'native persistent panel never enters Yoga display:none');
  assert.equal(node.props.pointerEvents, visible ? 'auto' : 'none', 'hidden descendants cannot receive native touches');
  assert.equal(node.props.accessibilityElementsHidden, !visible);
  assert.equal(node.props.importantForAccessibility, visible ? 'auto' : 'no-hide-descendants');
  for (const [key, hidden] of Object.entries({ position: 'absolute', width: 0, height: 0, overflow: 'hidden', opacity: 0 })) {
    assert.equal(style[key], visible ? undefined : hidden, 'hidden native panel has no in-flow gap, visible footprint or paint: ' + key);
  }
}
function noWrite(h) { assert.equal(h.writes.length, 0); assert.equal(h.deletes.length, 0); }

test('QM01 explicit friend action dismisses once while a pinned close and independent list preserve the Unicode draft owner', async () => {
  await withHost({ friends: [friend('synthetic-friend-A'), friend('synthetic-friend-B')] }, async h => {
    const body = 'A\n\nC · Ánh 😀'; await h.change(body);
    const previous = identities(h), draft = plain(h.drafts.at(-1)), publications = h.drafts.length;
    const panel = view(h, 'community-comment-mentions'), list = scroll(h, 'community-comment-mentions-list');
    panelVisible(panel, false); assert.equal(h.friendReads.length, 0); assert.equal(dismisses(h), 0);
    await toggle(h); assert.equal(dismisses(h), 0, 'plus alone does not dismiss the keyboard'); assert.equal(h.friendReads.length, 0);
    await click(h, 'Gắn bạn bè'); assert.equal(dismisses(h), 1, 'one explicit friend action invokes one native boundary call');
    assert.equal(h.friendReads.length, 1); assert.equal(boxes(h).length, 2);
    panelVisible(panel, true); assert.equal(flatten(list.props.style).maxHeight, 216);
    assert.equal(flatten(list.props.style).flexGrow, 0); assert.equal(list.props.keyboardShouldPersistTaps, 'handled');
    const header = view(h, 'community-comment-mentions-header'), close = button(h, 'Đóng danh sách bạn'), details = scroll(h, 'community-comment-details');
    assert.equal(flatten(header.props.style).minHeight, 48); assert.equal(flatten(close.props.style).width, 48); assert.equal(flatten(close.props.style).height, 48);
    assert.equal(inside(close, header), true); assert.equal(inside(close, list), false); assert.equal(inside(close, details), false);
    assert.equal(inside(previous.mentions, list), true); assert.equal(inside(h.input(), list), false); assert.equal(inside(send(h), list), false);
    panelVisible(details, false); assert.equal(h.text(props => props.testID === 'community-comment-send-reason').length, 0);
    same(h, previous, body); assert.deepEqual(plain(h.drafts.at(-1)), draft); assert.equal(h.drafts.length, publications);
    await h.pressAccessibility('Đóng danh sách bạn'); panelVisible(panel, false); same(h, previous, body);
    assert.equal(dismisses(h), 1, 'closing the list does not dismiss again');
    await toggle(h); assert.equal(dismisses(h), 1); await click(h, 'Gắn bạn bè'); assert.equal(dismisses(h), 2); assert.equal(h.friendReads.length, 2);
    await h.focus(); panelVisible(panel, false);
    same(h, previous, body); assert.deepEqual(plain(h.drafts.at(-1)), draft); assert.equal(h.drafts.length, publications);
    assert.equal(h.input().props.autoFocus, undefined); assert.equal(h.picks.length + h.uploads.length, 0); noWrite(h);
    assert.equal(h.observation().subscriptions, 1); actual(h);
  });
});

test('QM02 pending media keeps selected friends and independent recovery while the open list expands after upload completes', async () => {
  await withHost({ friends: [friend('synthetic-selected')] }, async h => {
    const body = 'Synthetic pending Ánh\n\ncaption'; await h.change(body); const previous = identities(h);
    await toggle(h); const retainedImage = button(h, 'Thêm ảnh').props.onPress; await click(h, 'Thêm ảnh');
    await h.finish(h.picks[0], { canceled: false, assets: [asset] }); await h.fail(h.uploads[0], new Error('Synthetic upload refusal'));
    await h.nativeFont(snapshot(2)); await open(h);
    const panel = view(h, 'community-comment-mentions'), list = scroll(h, 'community-comment-mentions-list'), details = scroll(h, 'community-comment-details');
    panelVisible(panel, true); assert.equal(flatten(list.props.style).maxHeight, 160, 'pending recovery reserves a smaller bounded friend viewport');
    panelVisible(details, false); assert.equal(h.text(props => props.testID === 'community-comment-send-reason').length, 0);
    assert.equal(boxes(h).length, 1); await act(async () => boxes(h)[0].props.onPress());
    assert.deepEqual(plain(h.drafts.at(-1).mentions), ['synthetic-selected']); assert.equal(boxes(h)[0].props['aria-checked'], true);
    const retry = button(h, 'Thử lại tệp đã chọn'), drop = button(h, 'Bỏ tệp đang chờ');
    for (const control of [retry, drop, h.input(), send(h)]) { assert.equal(inside(control, list), false); assert.equal(inside(control, details), false); }
    assert.equal(Boolean(retry.props.disabled), false); assert.equal(Boolean(drop.props.disabled), false); assert.equal(send(h).props.disabled, true);
    const pendingDraft = plain(h.drafts.at(-1)); assert.equal(Boolean(pendingDraft.pendingFile), true);
    await act(async () => retainedImage()); assert.equal(h.picks.length, 1, 'retained image action rechecks the current pending file');
    assert.deepEqual(plain(h.drafts.at(-1)), pendingDraft); same(h, previous, body);
    await click(h, 'Thử lại tệp đã chọn'); assert.equal(h.picks.length, 1); assert.equal(h.uploads.length, 2);
    assert.equal(flatten(list.props.style).maxHeight, 160); panelVisible(panel, true);
    await h.finish(h.uploads[1], { id: 'synthetic-mention-ready', type: 'image/jpeg', url: '/synthetic-mention-ready', state: 'ready' });
    panelVisible(panel, true);
    assert.equal(flatten(list.props.style).maxHeight, 216, 'completed upload releases the recovery viewport budget without reopening');
    assert.equal(h.drafts.at(-1).pendingFile, null); assert.equal(h.drafts.at(-1).media[0].id, 'synthetic-mention-ready');
    assert.deepEqual(plain(h.drafts.at(-1).mentions), ['synthetic-selected']); assert.equal(boxes(h)[0].props['aria-checked'], true);
    assert.equal(send(h).props.disabled, false); assert.equal(h.hosts('RNView', props => props.testID === 'community-comment-recovery').length, 0);
    same(h, previous, body); await h.pressAccessibility('Đóng danh sách bạn');
    panelVisible(details, true);
    const preview = h.one('RNExpoImage', props => props.accessibilityLabel === 'Ảnh đã đính kèm');
    assert.equal(flatten(preview.props.style).width, 64); assert.equal(flatten(preview.props.style).height, 64);
    same(h, previous, body); noWrite(h); actual(h);
  });
});

test('QM03 actual friends reads recover from failure and empty results while retired callbacks and hot metrics preserve the draft', async () => {
  let first;
  await withHost({}, async h => {
    const body = 'Synthetic friends Ánh\n\nC'; await h.change(body); const previous = identities(h), draft = plain(h.drafts.at(-1));
    await open(h); assert.equal(first.started, true); assert.equal(h.friendReads.length, 1);
    const panel = view(h, 'community-comment-mentions'), list = scroll(h, 'community-comment-mentions-list');
    assert.equal(inside(previous.mentions, list), true); panelVisible(panel, true); assert.equal(boxes(h).length, 0);
    await h.nativeFont(snapshot(2)); same(h, previous, body);
    const permission = h.one('RNText', props => text(props.children) === 'Tag gửi lời nhắc, không mở quyền xem bài.');
    assert.equal(permission.props.allowFontScaling, false); assert.equal(flatten(permission.props.style).fontSize, 21.5);
    await h.nativeFont(snapshot(1)); same(h, previous, body); assert.equal(flatten(permission.props.style).fontSize, 13);
    await h.fail(first, new Error('Synthetic friends read unavailable'));
    assert.equal(h.text('Chưa đọc được danh sách bạn bè.').length, 1); assert.equal(boxes(h).length, 0);
    const second = h.deferFriendsRead(); await h.pressText('Thử lại danh sách bạn bè'); assert.equal(second.started, true); assert.equal(h.friendReads.length, 2);
    await h.finish(second, { friends: [] }); assert.equal(h.text('Chưa có bạn bè để tag.').length, 1); assert.equal(h.text('Chưa đọc được danh sách bạn bè.').length, 0);
    assert.equal(boxes(h).length, 0); await h.pressAccessibility('Đóng danh sách bạn');
    const retired = h.deferFriendsRead(); await open(h); assert.equal(retired.started, true); await h.pressAccessibility('Đóng danh sách bạn');
    const current = h.deferFriendsRead(); await open(h); assert.equal(current.started, true); assert.equal(h.friendReads.length, 4);
    await h.finish(retired, { friends: [friend('synthetic-retired')] }); assert.equal(boxes(h).length, 0, 'a retired open interval cannot supply the newer pending list');
    const currentFriends = [friend('synthetic-current', 'Synthetic người bạn có tên dài Ánh 😀')];
    await h.finish(current, { friends: currentFriends }); assert.equal(boxes(h).length, 1);
    const friendText = boxes(h)[0].findAllByType('RNText')[0]; assert.equal(text(friendText.props.children), currentFriends[0].display_name);
    await h.nativeFont(snapshot(2)); assert.equal(friendText.props.allowFontScaling, false); assert.equal(flatten(friendText.props.style).fontSize, 28.75); assert.equal(flatten(friendText.props.style).lineHeight, 39.25);
    same(h, previous, body); await h.nativeFont(snapshot(1)); assert.equal(flatten(friendText.props.style).fontSize, 17); same(h, previous, body);
    for (const read of h.friendReads) { assert.equal(read.route, '/people/synthetic-A/friends'); assert.equal(read.options.actorId, 'synthetic-A'); }
    assert.deepEqual(plain(h.drafts.at(-1)), draft); assert.equal(h.picks.length + h.uploads.length, 0); noWrite(h); assert.equal(h.observation().subscriptions, 1); actual(h);
  }, h => { first = h.deferFriendsRead(); });
});

test('QM04 actual relative timestamps use the metadata column width at large text without replacing author time or body hosts', async () => {
  let pending;
  const now = Date.now();
  const rows = [7, 4, 1].map((hours, index) => ({ id: `synthetic-time-${index}`, author_id: `synthetic-author-${index}`, author: `Synthetic author ${index}`, body: `Synthetic timestamp body ${index}`, parent_id: null, media_id: null, mentions: [], created_at: new Date(now - (hours * 60 + 5) * 60000).toISOString(), status: 'approved', can_delete: false }));
  await withHost({}, async h => {
    await h.finish(pending, { comments: rows, pending: [], next_cursor: null });
    const previousInput = h.input(), previousOwner = h.owner('CommentsForContext'), draft = plain(h.drafts.at(-1));
    const witnesses = rows.map((row, index) => {
      const item = view(h, `community-comment-${row.id}`);
      const metadata = item.findAll(node => node.type === 'RNView' && node.props.testID === 'community-comment-metadata'); assert.equal(metadata.length, 1);
      const texts = metadata[0].findAllByType('RNText'); assert.equal(texts.length, 2);
      const content = item.findAll(node => node.type === 'RNView' && node.props.testID === 'community-comment-content'); assert.equal(content.length, 1);
      const bodies = content[0].findAll(node => node.type === 'RNText' && text(node.props.children) === row.body); assert.equal(bodies.length, 1);
      assert.equal(text(texts[0].props.children), row.author); assert.equal(text(texts[1].props.children), `${[7, 4, 1][index]} giờ`);
      return { item, metadata: metadata[0], author: texts[0], time: texts[1], content: content[0], body: bodies[0] };
    });
    for (const scale of [1, 2, 1]) {
      await h.nativeFont(snapshot(scale));
      for (const witness of witnesses) {
        const item = view(h, witness.item.props.testID); assert.equal(item === witness.item, true);
        const metadata = item.findAll(node => node.type === 'RNView' && node.props.testID === 'community-comment-metadata')[0]; assert.equal(metadata === witness.metadata, true);
        const texts = metadata.findAllByType('RNText'); assert.equal(texts[0] === witness.author, true); assert.equal(texts[1] === witness.time, true);
        const body = witness.content.findAll(node => node.type === 'RNText' && text(node.props.children) === witness.body.props.children)[0]; assert.equal(body === witness.body, true);
        const timeStyle = flatten(witness.time.props.style), metadataStyle = flatten(metadata.props.style);
        assert.equal(metadataStyle.flexDirection, scale === 2 ? 'column' : 'row'); assert.equal(metadataStyle.alignItems, scale === 2 ? 'flex-start' : 'baseline');
        assert.equal(timeStyle.alignSelf, scale === 2 ? 'stretch' : undefined, 'large-text timestamp receives available column width rather than intrinsic narrow width');
        const column = nearestView(metadata); assert.ok(column, 'actual metadata has a native column ancestor');
        assert.equal(timeStyle.flexShrink, 0); assert.equal(flatten(column.props.style).flex, 1); assert.equal(flatten(column.props.style).minWidth, 0);
        assert.equal(flatten(column.props.style).alignItems, undefined, 'the flex column retains default cross-axis stretch');
        assert.equal(witness.time.props.allowFontScaling, false); assert.equal(timeStyle.fontSize, scale === 2 ? 21.5 : 13); assert.equal(timeStyle.lineHeight, scale === 2 ? 30.25 : 18);
        assert.equal(witness.author.props.allowFontScaling, false); assert.equal(witness.body.props.allowFontScaling, false);
        assert.equal(flatten(witness.body.props.style).fontSize, scale === 2 ? 28.75 : 17); assert.equal(flatten(witness.content.props.style).marginLeft, scale === 2 ? -40 : 0);
      }
      assert.equal(h.input() === previousInput, true); assert.equal(h.owner('CommentsForContext') === previousOwner, true); assert.deepEqual(plain(h.drafts.at(-1)), draft);
    }
    assert.equal(h.friendReads.length + h.picks.length + h.uploads.length, 0); noWrite(h); actual(h);
  }, h => { pending = h.deferCommentsRead(); });
});
