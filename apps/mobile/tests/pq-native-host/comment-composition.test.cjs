// Actual Comments, Field, MediaPicker, MentionPicker and PostDetail execute together.
// Native layout callbacks are independent fixtures; Android layout and IME still need captures.
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
const inside = (node, ancestor) => { for (let p = node.parent; p; p = p.parent) if (p === ancestor) return true; return false; };
const nearestView = node => { for (let p = node.parent; p; p = p.parent) if (p.type === 'RNView') return p; return null; };
const view = (h, id) => h.one('RNView', props => props.testID === id);
const scroll = (h, id) => h.one('RNScrollView', props => props.testID === id);
const button = (h, label) => h.one('RNPressable', props => props.accessibilityLabel === label);
const send = h => h.one('RNPressable', props => /^(Gửi bình luận|Đang gửi…)$/.test(props.accessibilityLabel));
const disclosure = h => h.one('RNPressable', props => /^(Thêm vào bình luận|Đóng công cụ bình luận)$/.test(props.accessibilityLabel));
async function click(h, label) { const control = button(h, label); assert.equal(Boolean(control.props.disabled), false, 'actual target is enabled'); await act(async () => control.props.onPress()); }
async function toggle(h) { const control = disclosure(h); assert.equal(Boolean(control.props.disabled), false); await act(async () => control.props.onPress()); }
async function closeMentions(h) {
  const label = 'Đóng danh sách bạn';
  const controls = h.hosts('RNPressable').filter(node => node.props.accessibilityLabel === label || node.findAllByType('RNText').some(leaf => text(leaf.props.children) === label));
  assert.equal(controls.length, 1, 'one actual close control by its semantic label');
  assert.equal(Boolean(controls[0].props.disabled), false, 'actual close control is enabled');
  await act(async () => controls[0].props.onPress());
}
async function withHost(options, run, prepare = () => {}) {
  const h = createHost(options); h.state.window = { width: 320, height: 640, scale: 2, fontScale: 1 }; prepare(h);
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
function same(h, input, owner, body) {
  assert.equal(h.input() === input, true, 'same native input host within this writer');
  assert.equal(h.owner('CommentsForContext') === owner, true, 'same draft owner');
  assert.equal(h.input().props.value, body, 'exact controlled synthetic body');
}
function actual(h, names) {
  for (const name of names) {
    const relative = `apps/mobile/src/rudi/${name}`;
    const file = path.join(sourceRoot, relative + '.tsx');
    assert.equal(h.consumed.get(relative), file, 'actual source root binding: ' + name);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'), 'exact consumed bytes: ' + name);
  }
}
function mirror(h) { return h.one('RNText', props => props.testID === 'community-comment-input-measurement'); }
function nativeLines(count, pitch) { return Array.from({ length: count }, (_, i) => ({ x: 0, y: i * pitch, width: 90 + i, height: pitch })); }
async function nativeWidth(h, width) { await act(async () => h.input().props.onLayout({ nativeEvent: { layout: { x: 0, y: 0, width, height: 24 } } })); }
async function nativeRows(h, count, pitch) { await act(async () => mirror(h).props.onTextLayout({ nativeEvent: { lines: nativeLines(count, pitch) } })); }
function row(id, extra = {}) { return { id, author_id: 'synthetic-B', author: 'Synthetic comment author', body: 'Synthetic server comment', parent_id: null, media_id: null, mentions: [], created_at: '2026-10-01T00:00:00Z', status: 'approved', can_delete: false, ...extra }; }
const page = (comments, pending = []) => ({ comments, pending, next_cursor: null });
const asset = { uri: 'file:///synthetic-composition.jpg', width: 32, height: 32, type: 'image', mimeType: 'image/jpeg' };
async function refresh(h, result, cursor) {
  const pending = h.deferCommentsRead(), socket = h.sockets.find(value => !value.closed && typeof value.onmessage === 'function');
  assert.ok(socket, 'actual community stream has a synthetic native socket');
  await act(async () => socket.onmessage({ data: JSON.stringify({ kind: 'sync', cursor }) }));
  assert.equal(pending.started, true, 'actual stream invokes actual comment read');
  await h.finish(pending, result);
}

test('QC01 one quiet disclosure entry row removes idle duplication and retains native owner through tool toggles', async () => {
  await withHost({}, async h => {
    const entry = view(h, 'community-comment-entry'), slot = view(h, 'community-comment-field-slot');
    const plus = disclosure(h), submit = send(h), entryStyle = flatten(entry.props.style);
    assert.equal(entry.children.length, 3, 'only plus, Field slot and send share the entry');
    assert.equal(entryStyle.flexDirection, 'row'); assert.equal(entryStyle.alignItems, 'flex-end'); assert.equal(entryStyle.gap, 8);
    assert.equal(nearestView(plus) === entry, true); assert.equal(nearestView(slot) === entry, true); assert.equal(nearestView(submit) === entry, true);
    assert.equal(flatten(plus.props.style).width, 48); assert.equal(flatten(plus.props.style).height, 48);
    assert.equal(flatten(submit.props.style).width, 48); assert.equal(flatten(submit.props.style).height, 48);
    assert.equal(flatten(slot.props.style).flex, 1); assert.equal(flatten(slot.props.style).minWidth, 0);
    assert.equal(320 - 2 * 16 - flatten(plus.props.style).width - flatten(submit.props.style).width - 2 * entryStyle.gap - 30, 146, 'source budget, not a Yoga measurement');
    assert.equal(h.hosts('RNView', props => props.testID === 'community-comment-tools').length, 0, 'no idle secondary bar');
    assert.equal(h.text(props => props.testID === 'community-comment-send-reason').length, 0);
    assert.equal(flatten(scroll(h, 'community-comment-details').props.style).display, 'none');
    assert.equal(submit.props.disabled, true); assert.equal(h.input().props.placeholder, 'Viết…'); assert.equal(h.input().props.accessibilityLabel, 'Bình luận');
    await h.change('Synthetic Ánh 😀 draft'); await h.focus();
    const input = h.input(), owner = h.owner('CommentsForContext'), picker = h.owner('MediaPicker'), saved = plain(h.drafts.at(-1)), calls = h.calls.length;
    for (const scale of [1, 2, 1]) {
      await h.nativeFont(snapshot(scale)); await toggle(h);
      assert.equal(disclosure(h).props.accessibilityState.expanded, true);
      assert.equal(flatten(view(h, 'community-comment-tools').props.style).flexDirection, scale === 2 ? 'column' : 'row');
      assert.equal(button(h, 'Thêm ảnh').props.disabled, false); assert.ok(button(h, 'Gắn bạn bè'));
      same(h, input, owner, saved.body); assert.equal(h.owner('MediaPicker') === picker, true);
      await toggle(h); same(h, input, owner, saved.body);
      assert.equal(h.hosts('RNView', props => props.testID === 'community-comment-tools').length, 0);
      assert.deepEqual(plain(h.drafts.at(-1)), saved, 'disclosure does not rotate logical draft or edit its metadata');
    }
    assert.equal(h.calls.length, calls); assert.equal(h.friendReads.length, 0); assert.equal(h.picks.length, 0); assert.equal(h.uploads.length, 0); assert.equal(h.writes.length, 0);
    assert.equal(h.observation().live, 1); assert.equal(h.observation().subscriptions, 1);
    actual(h, ['community/Comments', 'community/MediaPicker', 'community/MentionPicker', 'ui/Field', 'ui/PressScale', 'ui/RudiIcon']);
  });
});

test('QC02 narrow native content width and hot metrics keep independent measured three-line geometry without remount', async () => {
  await withHost({}, async h => {
    assert.ok(view(h, 'community-comment-entry'));
    await h.change('Synthetic first\nÁnh 😀\n'); await h.focus(); await nativeWidth(h, 146); await nativeRows(h, 2, 24);
    const input = h.input(), owner = h.owner('CommentsForContext'), saved = plain(h.drafts.at(-1)), oldMeasure = mirror(h).props.onTextLayout;
    assert.equal(flatten(mirror(h).props.style).width, 146, 'native callback is already content width, no second chrome subtraction');
    assert.equal(flatten(h.input().props.style).height, 48); assert.equal(h.input().props.numberOfLines, 2);
    await h.nativeFont(snapshot(2));
    await act(async () => oldMeasure({ nativeEvent: { lines: nativeLines(1, 24) } }));
    assert.equal(flatten(h.input().props.style).height, 79, 'stale prior-font rows cannot collapse pending current geometry');
    await nativeRows(h, 3, 39.5); assert.equal(flatten(h.input().props.style).height, 118.5); assert.equal(h.input().props.numberOfLines, 3);
    assert.equal(mirror(h).props.importantForAccessibility, 'no-hide-descendants'); assert.equal(mirror(h).props.accessibilityElementsHidden, true); assert.equal(mirror(h).props.pointerEvents, 'none');
    await h.nativeFont(snapshot(1)); await nativeRows(h, 3, 24); assert.equal(flatten(h.input().props.style).height, 72);
    same(h, input, owner, saved.body); assert.deepEqual(plain(h.drafts.at(-1)), saved);
    assert.equal(h.observation().subscriptions, 1); assert.equal(h.picks.length + h.uploads.length + h.writes.length, 0);
  });
});

test('QC03 pending recovery stays outside details and mentions remain reachable while retained image replacement is refused', async () => {
  await withHost({}, async h => {
    await h.change('Synthetic pending with caption'); const input = h.input(), owner = h.owner('CommentsForContext');
    await toggle(h); const retainedImage = button(h, 'Thêm ảnh').props.onPress; await click(h, 'Thêm ảnh');
    await h.finish(h.picks[0], { canceled: false, assets: [asset] }); await h.fail(h.uploads[0], new Error('Synthetic upload unavailable'));
    same(h, input, owner, 'Synthetic pending with caption');
    const details = scroll(h, 'community-comment-details');
    assert.equal(flatten(details.props.style).maxHeight, 72);
    assert.equal(inside(button(h, 'Thử lại tệp đã chọn'), details), false); assert.equal(inside(button(h, 'Bỏ tệp đang chờ'), details), false);
    assert.equal(inside(input, details), false); assert.equal(inside(send(h), details), false);
    assert.equal(send(h).props.disabled, true); assert.equal(disclosure(h).props.disabled, false);
    assert.equal(h.text(props => props.testID === 'community-comment-send-reason').length, 0, 'upload error is the one visible explanation');
    assert.equal(h.hosts('RNText', props => props.accessibilityRole === 'alert').length, 1);
    await toggle(h); assert.equal(h.hosts('RNPressable', props => props.accessibilityLabel === 'Thêm ảnh').length, 0, 'pending file is resolved before replacing it');
    assert.ok(button(h, 'Gắn bạn bè')); assert.ok(button(h, 'Thử lại tệp đã chọn'));
    await act(async () => retainedImage()); assert.equal(h.picks.length, 1, 'current pending guard also protects retained image action');
    await click(h, 'Gắn bạn bè'); assert.equal(h.friendReads.length, 1); same(h, input, owner, 'Synthetic pending with caption');
    await closeMentions(h);
    await click(h, 'Thử lại tệp đã chọn'); assert.equal(h.picks.length, 1, 'retry reuses selected native file');
    await h.finish(h.uploads[1], { id: 'synthetic-ready', type: 'image/jpeg', url: '/synthetic-ready', state: 'ready' });
    assert.equal(h.drafts.at(-1).pendingFile, null); assert.equal(h.drafts.at(-1).media[0].id, 'synthetic-ready'); assert.equal(send(h).props.disabled, false);
    const preview = h.one('RNExpoImage', props => props.accessibilityLabel === 'Ảnh đã đính kèm'); assert.equal(flatten(preview.props.style).width, 64); assert.equal(flatten(preview.props.style).height, 64);
    assert.equal(flatten(button(h, 'Bỏ tệp đính kèm 1').props.style).width, 48); same(h, input, owner, 'Synthetic pending with caption');
    assert.equal(h.writes.length, 0);
  });
});

test('QC04 actual friends disclosure preserves controlled draft and checkbox selection limit through hot metrics', async () => {
  const friends = Array.from({ length: 21 }, (_, i) => ({ person_id: `synthetic-friend-${i}`, display_name: `Synthetic Ánh ${i}`, friends_since: '2026-10-01T00:00:00Z' }));
  let pending;
  await withHost({ friends }, async h => {
    await h.change('Synthetic mentions draft'); const input = h.input(), owner = h.owner('CommentsForContext');
    assert.equal(h.friendReads.length, 0); await toggle(h); assert.equal(h.friendReads.length, 0, 'plus alone performs no friends read');
    await click(h, 'Gắn bạn bè'); assert.equal(pending.started, true); assert.equal(h.friendReads.length, 1);
    assert.equal(h.friendReads[0].route, '/people/synthetic-A/friends'); assert.equal(h.friendReads[0].options.actorId, 'synthetic-A');
    await h.finish(pending, { friends });
    const boxes = () => h.hosts('RNPressable', props => props.accessibilityRole === 'checkbox'); assert.equal(boxes().length, 21);
    for (let i = 0; i < 21; i++) { const box = boxes()[i]; assert.equal(flatten(box.props.style).minHeight, 48); await act(async () => box.props.onPress()); }
    assert.equal(h.drafts.at(-1).mentions.length, 20); assert.equal(boxes()[20].props['aria-checked'], false);
    await act(async () => boxes()[0].props.onPress()); assert.equal(h.drafts.at(-1).mentions.length, 19);
    await act(async () => boxes()[20].props.onPress()); assert.equal(h.drafts.at(-1).mentions.length, 20); assert.equal(boxes()[20].props['aria-checked'], true);
    await h.nativeFont(snapshot(2)); same(h, input, owner, 'Synthetic mentions draft');
    const friendText = boxes()[1].findAllByType('RNText')[0]; assert.equal(friendText.props.allowFontScaling, false); assert.equal(flatten(friendText.props.style).fontSize, 28.75);
    const permission = h.one('RNText', props => text(props.children) === 'Tag gửi lời nhắc, không mở quyền xem bài.'); assert.equal(permission.props.allowFontScaling, false); assert.equal(flatten(permission.props.style).fontSize, 21.5); assert.equal(flatten(permission.props.style).fontWeight, '400');
    await closeMentions(h); await toggle(h); await click(h, 'Gắn bạn bè, đã chọn 20 người'); assert.equal(h.friendReads.length, 2); assert.equal(boxes().filter(box => box.props['aria-checked']).length, 20);
    same(h, input, owner, 'Synthetic mentions draft'); assert.equal(h.writes.length + h.picks.length + h.uploads.length, 0); assert.equal(h.observation().subscriptions, 1);
  }, h => { pending = h.deferFriendsRead(); });
});

test('QC05 inline gutter and fixed entry preserve retry payload and handoff journal while tool state resets per writer', async () => {
  await withHost({ surface: 'detail' }, async h => {
    const thread = view(h, 'community-comment-list');
    const wrapper = h.hosts('RNView', props => flatten(props.style)?.paddingHorizontal === 16 && flatten(props.style)?.paddingVertical === 20).find(node => inside(thread, node));
    assert.ok(wrapper, 'PostDetail gives actual thread the intended 16dp gutters');
    const input = h.input(), owner = h.owner('CommentsForContext'); await h.change('Synthetic reply and retry'); await h.pressText('Trả lời');
    await toggle(h); same(h, input, owner, 'Synthetic reply and retry'); await toggle(h);
    const details = scroll(h, 'community-comment-details'); assert.equal(inside(button(h, 'Hủy trả lời'), details), true); assert.equal(inside(send(h), details), false);
    await h.send(); const sent = plain(h.writes[0].options.body); await h.rejectWrite(0);
    await h.nativeFont(snapshot(2)); same(h, input, owner, sent.body); assert.equal(send(h).props.disabled, false);
    assert.equal(inside(h.input(), h.one('RNScrollView', props => props.testID === 'community-post-body')), false, 'native input is in the fixed footer');
    await h.send(); assert.deepEqual(plain(h.writes[1].options.body), sent, 'disclosure and metrics never rotate a failed submission');
    await h.openComments(); assert.equal(h.input() === input, false, 'explicit writer handoff still remounts by design'); assert.equal(h.input().props.value, sent.body);
    assert.equal(disclosure(h).props.accessibilityState.expanded, false, 'local tool presentation is not draft data');
    await h.resolveWrite(1); assert.equal(h.input().props.value, sent.body, 'retired inline ACK cannot clear current sheet draft');
    await h.send(); await h.closeComments(); const closing = h.closeAnimation(); await h.resolveWrite(2); await h.completeClose(closing);
    assert.equal(h.input().props.value, ''); assert.equal(send(h).props.disabled, true); assert.equal(h.editors().length, 1); assert.equal(h.observation().live, 1);
    actual(h, ['community/PostDetail', 'community/Comments', 'community/MediaPicker', 'community/MentionPicker']);
  });
});

test('QC06 adaptive thread retains text hosts and revoked delete permission hides stale confirmation and retry', async () => {
  const mine = row('synthetic-mine', { author_id: 'synthetic-A', author: 'Synthetic long author', body: 'Synthetic own comment', can_delete: true });
  const reply = row('synthetic-reply', { body: 'Synthetic reply', parent_id: mine.id });
  const waiting = row('synthetic-waiting', { body: 'Synthetic pending comment', status: 'pending' });
  let initial;
  await withHost({}, async h => {
    await h.finish(initial, page([mine, reply], [waiting]));
    const comment = view(h, 'community-comment-synthetic-mine');
    const metadata = comment.findAllByType('RNView').find(node => node.props.testID === 'community-comment-metadata');
    const content = comment.findAllByType('RNView').find(node => node.props.testID === 'community-comment-content');
    const author = h.one('RNText', props => props.children === mine.author), body = h.one('RNText', props => props.children === mine.body), timestamp = metadata.findAllByType('RNText')[1];
    assert.equal(flatten(metadata.props.style).flexDirection, 'row'); assert.equal(flatten(metadata.props.style).gap, 8); assert.equal(flatten(content.props.style).marginLeft, 0);
    assert.equal(flatten(view(h, 'community-comment-synthetic-reply').props.style).marginLeft, 24);
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale)); assert.equal(h.one('RNText', props => props.children === mine.author) === author, true); assert.equal(h.one('RNText', props => props.children === mine.body) === body, true); assert.equal(metadata.findAllByType('RNText')[1] === timestamp, true);
      assert.equal(flatten(metadata.props.style).flexDirection, scale === 2 ? 'column' : 'row'); assert.equal(flatten(content.props.style).marginLeft, scale === 2 ? -40 : 0); assert.equal(body.props.allowFontScaling, false);
    }
    await click(h, 'Thêm lựa chọn cho bình luận'); await h.pressText('Xóa');
    const Confirm = h.load('apps/mobile/src/rudi/ui/HoiTaiHang').HoiTaiHang, confirmation = h.root.root.findByType(Confirm), retainedDelete = confirmation.props.onDongY;
    await refresh(h, page([{ ...mine, can_delete: false }, reply], [waiting]), 1);
    assert.equal(h.root.root.findAllByType(Confirm).length, 0, 'revoked current permission removes obsolete confirmation'); await act(async () => retainedDelete()); assert.equal(h.deletes.length, 0);
    await refresh(h, page([mine, reply], [waiting]), 2); assert.equal(h.root.root.findAllByType(Confirm).length, 1, 'the same local confirmation can render when permission returns'); await h.pressText('Xóa'); assert.equal(h.deletes.length, 1);
    const deleteRefusal = new Error('Synthetic delete refused');
    await h.fail(h.deletes[0], deleteRefusal);
    const ErrorRow = h.load('apps/mobile/src/rudi/ui/CauTaiCho').CauTaiCho, failure = h.root.root.findAllByType(ErrorRow).find(node => node.props.cau === String(deleteRefusal)); assert.ok(failure);
    const retainedRetry = failure.findAllByType('RNPressable')[0].props.onPress;
    await refresh(h, page([{ ...mine, can_delete: false }, reply], [waiting]), 3);
    const currentFailure = h.root.root.findAllByType(ErrorRow).find(node => node.props.cau === String(deleteRefusal)); assert.ok(currentFailure, 'error remains readable after permission changes'); assert.equal(currentFailure.findAllByType('RNPressable').length, 0, 'no dead retry target');
    await act(async () => { retainedDelete(); retainedRetry(); }); assert.equal(h.deletes.length, 1, 'retained callbacks obey current permission too');
    assert.equal(h.root.root.findAllByType(Confirm).length, 0); assert.equal(h.writes.length, 0);
  }, h => { initial = h.deferCommentsRead(); h.state.token = 'synthetic-stream-token'; });
});
