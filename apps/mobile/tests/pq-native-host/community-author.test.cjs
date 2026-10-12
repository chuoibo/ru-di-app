// Actual feed/card sources and inherited metrics; host styles do not prove Android pixels.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, post, act, sourceRoot } = require('./owner-host.cjs');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const window100 = { width: 320, height: 640, fontScale: 1, scale: 2 };
const unavailable = () => ({ status: 'unavailable', reason: 'module-inactive' });
function one(nodes, message) { assert.equal(nodes.length, 1, message); return nodes[0]; }
function style(node) { return flatten(typeof node.props.style === 'function' ? node.props.style({ pressed: false }) : node.props.style); }
function action(parent, label) { return one(parent.findAllByType('RNPressable').filter(node => node.props.accessibilityLabel === label), 'one actual action: ' + label); }
function text(parent, value) { return one(parent.findAllByType('RNText').filter(node => node.props.children === value), 'one complete actual text'); }
function card(h, id = 'synthetic-post-1') { return h.one('RNView', props => props.testID === 'community-post-' + id); }
function metadata(root) { return one(root.findAllByType('RNText').filter(node => Array.isArray(node.props.children) && node.props.children.includes('Cộng đồng')), 'one complete date/audience paragraph'); }
function identity(root) {
  return one(root.findAllByType('RNView').filter(node => node !== root && node.findAllByType('RNPressable').some(child => /^Hồ sơ /.test(child.props.accessibilityLabel)) && node.findAllByType('RNPressable').some(child => child.props.accessibilityLabel === 'Thêm lựa chọn cho bài')), 'one actual author/control group');
}
function authorMain(root) { return one(identity(root).findAllByType('RNView').filter(node => node !== identity(root) && node.findAllByType('RNPressable').some(child => /^Hồ sơ /.test(child.props.accessibilityLabel)) && !node.findAllByType('RNPressable').some(child => child.props.accessibilityLabel === 'Thêm lựa chọn cho bài')), 'actual avatar/copy group'); }
function controls(root) { return one(identity(root).findAllByType('RNView').filter(node => node !== identity(root) && node.findAllByType('RNPressable').some(child => child.props.accessibilityLabel === 'Thêm lựa chọn cho bài') && !node.findAllByType('RNPressable').some(child => /^Hồ sơ /.test(child.props.accessibilityLabel))), 'actual secondary controls group'); }
async function press(node) { assert.notEqual(node.props.disabled, true); await act(async () => { node.props.onPress(); }); }
async function closeSheet(h, sheet) {
  const label = sheet.props.accessibilityLabel;
  await act(async () => sheet.props.onClose());
  const panel = h.one('RNAnimatedView', props => props.accessibilityLabel === label);
  const ids = [panel.props.style].flat(Infinity).flatMap(style => style?.syntheticSharedIds ?? []);
  const event = h.timings.filter(item => ids.includes(item.shared) && item.target === 0).at(-1);
  assert.equal(typeof event?.callback, 'function', 'actual Sheet close completion');
  await act(async () => event.callback(true));
  assert.equal(h.hosts('RNAnimatedView', props => props.accessibilityLabel === label).length, 0);
}
function observation(h, removed = false, absent = false) {
  const value = h.observation();
  assert.deepEqual({ live: value.live, subscriptions: value.subscriptions, removals: value.removals, maxLive: value.maxLive }, { live: removed || absent ? 0 : 1, subscriptions: absent ? 0 : 1, removals: removed && !absent ? 1 : 0, maxLive: absent ? 0 : 1 });
  if (!removed) assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1, 'only the existing surface provider');
}
function metrics(h, node, base) {
  const effective = style(node);
  assert.equal(node.props.allowFontScaling, false);
  for (const [key, value] of Object.entries(base)) assert.equal(effective[key], one(h.state.nativeFont.fontMetrics.spToDp.filter(row => row.base === value), 'effective metric row').dp);
  assert.equal(node.props.maxFontSizeMultiplier, undefined); assert.notEqual(node.props.adjustsFontSizeToFit, true);
}
function counters(h) { return { calls: h.calls.length, attempts: h.attemptCount(), storage: h.community.storageCalls.length, routes: h.routes.length, sockets: h.sockets.length, native: h.nativeHostCalls.length, haptics: h.community.haptics.length }; }
function actual(h) {
  for (const relative of ['apps/mobile/app/(tabs)/community', 'apps/mobile/src/rudi/community/CommunityScreen', 'apps/mobile/src/rudi/community/PostCard', 'apps/mobile/src/rudi/community/Comments', 'apps/mobile/src/rudi/community/useCommunityStream', 'apps/mobile/src/rudi/community/api', 'apps/mobile/src/rudi/community/album', 'apps/mobile/src/rudi/community/bang-tin', 'apps/mobile/src/rudi/ui/NativeFontMetrics', 'apps/mobile/src/rudi/ui/AvatarNguoi', 'apps/mobile/src/rudi/nguoi/anh-dai-dien', 'apps/mobile/src/rudi/ui/PressScale', 'apps/mobile/src/rudi/ui/useMotion', 'apps/mobile/src/rudi/theme']) {
    const file = h.consumed.get(relative); assert.ok(file, 'actual source consumed: ' + relative);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function fixture(extra = {}) { return { ...post(), author: 'fixture_trang', ...extra }; }
async function feed(posts, options, run) {
  const absent = options.nativeFontModulePresent === false;
  const h = createHost({ surface: 'community', communityView: 'tab', nativeGeometry: true, initialWindow: window100, ...options, communityIO: { notifications: [], preferences: { asked: true, personalized: false }, seen: null, posts } });
  // Real avatar cache seeded with a known absence; no avatar-fetch claim is made.
  const avatars = h.load('apps/mobile/src/rudi/nguoi/anh-dai-dien');
  for (const author of new Set(posts.map(row => row.author_id))) avatars.baoDaDoiAnh(author, 'synthetic-A', null);
  try {
    await h.mount(); assert.equal(h.community.feedReads.length, 1);
    await h.finish(h.community.feedReads[0], { posts, next_cursor: null, personalized: false });
    assert.equal(h.sockets.length, 1); await act(async () => h.sockets[0].onopen());
    await run(h); actual(h); h.community.assertClean();
  } finally {
    await h.dispose(); h.community.assertClean(); observation(h, true, absent);
    assert.equal(Object.values(h.community.listeners).every(entries => entries.size === 0), true, 'actual native listeners removed');
    assert.equal(h.sockets.every(socket => socket.closed), true, 'actual stream closes');
  }
}

test('CH04 actual author reflows on inherited native metrics and retains body, album and semantic controls', async () => {
  const body = 'Synthetic long story retained through a native font change. '.repeat(40);
  const row = fixture({ body, media: [1, 2].map(id => ({ id: 'synthetic-media-' + id, url: '/v2/community/media/synthetic-media-' + id, type: 'image/jpeg', width: 480, height: 320, duration_ms: 0 })) });
  await feed([row], {}, async h => {
    observation(h); const root = card(h), group = identity(root), cardOwner = one(h.cards(), 'one actual memo card');
    assert.equal(style(group).flexDirection, 'row', 'ordinary type keeps a compact author row');
    const name = text(root, row.author), meta = metadata(root), profile = action(root, 'Hồ sơ ' + row.author), follow = action(root, 'Theo dõi tác giả'), more = action(root, 'Thêm lựa chọn cho bài');
    const followRef = h.native.node(follow), moreRef = h.native.node(more), profileRef = h.native.node(profile);
    assert.equal(h.load('apps/mobile/src/rudi/community/bang-tin').thanBiCat(body), true);
    await press(action(root, 'Đọc toàn bộ câu chuyện')); assert.equal(text(root, body).props.numberOfLines, undefined);
    const bodyLeaf = text(root, body);
    const frame = one(root.findAllByType('RNView').filter(node => typeof node.props.onLayout === 'function' && style(node).aspectRatio === 4 / 3), 'actual album allocation');
    await act(async () => frame.props.onLayout({ nativeEvent: { layout: { width: 288 } } }));
    const album = one(root.findAllByType('RNScrollView'), 'actual album controller'), albumRef = h.native.node(album);
    await act(async () => album.props.onMomentumScrollEnd({ nativeEvent: { contentOffset: { x: 288 } } }));
    assert.ok(root.findAllByType('RNText').some(node => node.props.accessibilityLabel === 'Ảnh 2 trên 2'));
    const images = root.findAllByType('RNExpoImage').filter(node => node.props.accessibilityLabel === 'Ảnh trong bài đăng'); assert.equal(images.length, 2);
    const imageRefs = images.map(node => h.native.node(node)), before = counters(h);
    for (const value of [snapshot(2), snapshot(), snapshot(2), snapshot()]) {
      await h.nativeFont(value); observation(h);
      assert.equal(h.state.window.fontScale, 1, 'window font scale stays stale');
      assert.equal(card(h) === root, true); assert.equal(one(h.cards(), 'retained actual memo card') === cardOwner, true);
      assert.equal(identity(root) === group, true); assert.equal(text(root, row.author) === name, true); assert.equal(metadata(root) === meta, true);
      assert.equal(style(group).flexDirection, value.fontScale === 2 ? 'column' : 'row', 'CH04 effective native text controls the author axis');
      const main = authorMain(root), actions = controls(root);
      assert.equal(style(main).alignItems, 'flex-start'); assert.equal(style(main).flexBasis, 'auto');
      assert.equal(style(main).width, value.fontScale === 2 ? '100%' : 'auto'); assert.equal(style(main).flexGrow, value.fontScale === 2 ? 0 : 1); assert.equal(style(main).flexShrink, value.fontScale === 2 ? 0 : 1);
      assert.equal(style(actions).flexDirection, 'row'); assert.equal(style(actions).alignSelf, value.fontScale === 2 ? 'flex-end' : 'flex-start');
      assert.equal(h.native.node(action(root, 'Theo dõi tác giả')) === followRef, true); assert.equal(h.native.node(action(root, 'Thêm lựa chọn cho bài')) === moreRef, true); assert.equal(h.native.node(action(root, 'Hồ sơ ' + row.author)) === profileRef, true);
      for (const target of [profile, follow, more]) { assert.equal(style(target).minWidth ?? style(target).width, 48); assert.equal(style(target).minHeight ?? style(target).height, 48); }
      metrics(h, name, { fontSize: 17, lineHeight: 23, letterSpacing: -0.15 }); metrics(h, meta, { fontSize: 13, lineHeight: 18 });
      assert.equal(text(root, body) === bodyLeaf, true); assert.equal(bodyLeaf.props.numberOfLines, undefined);
      assert.equal(one(root.findAllByType('RNScrollView'), 'retained album') === album, true); assert.equal(h.native.node(album) === albumRef, true);
      assert.ok(root.findAllByType('RNText').some(node => node.props.accessibilityLabel === 'Ảnh 2 trên 2'));
      assert.equal(root.findAllByType('RNExpoImage').filter(node => node.props.accessibilityLabel === 'Ảnh trong bài đăng').every((node, i) => h.native.node(node) === imageRefs[i]), true);
      assert.deepEqual(counters(h), before, 'font-only events cannot dispatch I/O, attempts or navigation');
    }
  });
});

test('CH05 full long-name metadata and own-author controls survive unavailable, partial and missing metrics', async () => {
  const rows = [fixture({ author: 'Synthetic long author name with every word retained' }), { ...post('synthetic-post-2', 'synthetic-A'), author: 'Synthetic account owner with a long display name' }];
  await feed(rows, {}, async h => {
    const roots = rows.map(row => card(h, row.id));
    for (const [i, root] of roots.entries()) { assert.equal(text(root, rows[i].author).props.numberOfLines, undefined); assert.equal(metadata(root).props.numberOfLines, undefined, 'CH05 the full date/audience paragraph has no line clamp'); assert.equal(metadata(root).props.children.join('').endsWith(' · Cộng đồng'), true); }
    assert.equal(roots[1].findAllByType('RNPressable').some(node => /theo dõi tác giả/i.test(node.props.accessibilityLabel)), false, 'own-author card has no follow target');
    const before = counters(h), partial = snapshot(); partial.fontMetrics.spToDp = partial.fontMetrics.spToDp.filter(row => row.base !== 17);
    for (const value of [unavailable(), partial, snapshot()]) {
      await h.nativeFont(value); observation(h);
      const fallback = value.status === 'unavailable' || value === partial;
      for (const [i, root] of roots.entries()) {
        assert.equal(card(h, rows[i].id) === root, true); assert.equal(style(identity(root)).flexDirection, fallback ? 'column' : 'row', 'CH05 unusable inherited author metrics choose readable full-width layout');
        assert.equal(text(root, rows[i].author).props.allowFontScaling, fallback ? undefined : false);
        assert.equal(text(root, rows[i].author).props.numberOfLines, undefined); assert.equal(metadata(root).props.numberOfLines, undefined);
        assert.equal(style(authorMain(root)).minWidth, 0);
      }
      assert.equal(controls(roots[1]).findAllByType('RNPressable').length, 1); assert.deepEqual(counters(h), before);
    }
  });
  await feed([rows[0]], { nativeFontModulePresent: false }, async h => {
    observation(h, false, true); assert.equal(style(identity(card(h))).flexDirection, 'column', 'missing module keeps the owned fallback readable');
    assert.equal(text(card(h), rows[0].author).props.allowFontScaling, undefined);
  });
});

test('CH06 actual author controls keep routes, follow and pending like/save behavior through font reflow', async () => {
  const row = fixture({ body: 'Synthetic short story for actual navigation.' });
  await feed([row], {}, async h => {
    const root = card(h), more = action(root, 'Thêm lựa chọn cho bài'), follow = action(root, 'Theo dõi tác giả');
    await press(action(root, 'Thích bài, 2 lượt thích')); assert.equal(h.community.likeWrites.length, 1);
    assert.equal(action(root, 'Theo dõi tác giả').props.disabled, true); assert.equal(action(root, 'Lưu để đọc lại').props.disabled, true);
    const before = counters(h); await h.nativeFont(snapshot(2)); observation(h); assert.deepEqual(counters(h), before);
    assert.equal(action(root, 'Thêm lựa chọn cho bài') === more, true); assert.equal(action(root, 'Theo dõi tác giả') === follow, true);
    await press(more);
    const sheets = h.root.root.findAllByType(h.load('apps/mobile/src/rudi/ui/Sheet').Sheet);
    const options = one(sheets.filter(node => node.props.accessibilityLabel === 'Lựa chọn cho bài đăng'), 'actual post actions Sheet'); assert.equal(options.props.open, true);
    await closeSheet(h, options);
    await press(action(root, 'Hồ sơ ' + row.author)); assert.equal(h.routes.at(-1), '/people/synthetic-B');
    await h.finish(h.community.likeWrites[0], { ...row, liked: true, likes: 3 });
    assert.equal(action(root, 'Theo dõi tác giả').props.disabled, false);
    await press(action(root, 'Theo dõi tác giả')); assert.equal(h.community.followWrites.length, 1);
    const followBody = h.community.followWrites[0].options.body;
    assert.deepEqual(Object.keys(followBody).sort(), ['kind', 'target']); assert.equal(followBody.kind, 'person'); assert.equal(followBody.target, 'synthetic-B');
    const following = counters(h); await h.nativeFont(snapshot()); assert.deepEqual(counters(h), following);
    await h.finish(h.community.followWrites[0], {}); assert.equal(action(root, 'Bỏ theo dõi tác giả').props.disabled, false);
    await press(action(root, 'Lưu để đọc lại')); assert.equal(h.community.feedbackWrites.length, 1); assert.equal(action(root, 'Bỏ lưu bài').props.disabled, true);
    await h.fail(h.community.feedbackWrites[0], new Error('Synthetic save refusal')); assert.equal(action(root, 'Lưu để đọc lại').props.disabled, false);
    await press(action(root, 'Mở bình luận, 3 bình luận')); assert.equal(h.community.commentReads.length, 1);
    assert.equal(h.sockets.length, 2); await act(async () => h.sockets[1].onopen());
    const auth = JSON.parse(h.nativeHostCalls.filter(call => typeof call.socket === 'string').at(-1).socket);
    assert.deepEqual(auth.posts, ['synthetic-post-1']); assert.equal(auth.token, h.state.actor.token); assert.equal(auth.after, 0);
    const comments = one(h.root.root.findAllByType(h.load('apps/mobile/src/rudi/ui/Sheet').Sheet).filter(node => node.props.accessibilityLabel === 'Bình luận'), 'actual Comments Sheet'); assert.equal(comments.props.open, true);
    await closeSheet(h, comments); assert.equal(h.sockets[1].closed, true);
    await press(action(root, 'Đọc toàn bộ câu chuyện')); assert.equal(h.routes.at(-1), '/community/posts/synthetic-post-1');
    assert.equal(card(h) === root, true); observation(h);
  });
});
