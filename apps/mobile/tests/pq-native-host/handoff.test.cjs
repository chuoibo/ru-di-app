// Execute actual PostDetail, Comments, MediaPicker, Sheet and useMotion source.
// Reanimated completion is controlled at the native animation boundary; Sheet is never replaced.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const { test, after } = require('node:test');
const { repo, sourceRoot } = require('./repository.cjs');
const { flatten } = require('./native-metrics-boundary.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const React = dependency('react'), Renderer = dependency('react-test-renderer'), ts = dependency('typescript');
const { act } = React;
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT; globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const oldError = console.error; console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) oldError(...args); };
after(() => { console.error = oldError; if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };
const row = { id: 'synthetic-parent', body: 'Synthetic parent', author: 'Synthetic B', author_id: 'synthetic-B', parent_id: null, status: 'approved', can_delete: false, created_at: '2026-10-01T00:00:00Z' };
const page = () => ({ comments: [row], pending: [], next_cursor: null });
const taggedDraft = (body, extra = {}) => ({ body, parent: null, media: [], mentions: [], pendingFile: null, logical_id: 'synthetic-retired-logical', person_id: 'synthetic-A', post_id: 'synthetic-post-1', ...extra });
const serverPost = (actor, id) => ({ id, author_id: actor, author: actor, body: 'Synthetic post body', revision: 1, audience: 'public', status: 'published', topics: [], mentions: [], media: [], context_id: null, can_comment: true, saved: false, diary: false });
async function withCase(run, { reduced = false, strict = false, actualField = false } = {}) {
  const state = { actor: 'synthetic-A', id: 'synthetic-post-1', nativeFont: { status: 'available', fontScale: 1 } };
  const fontListeners = new Set();
  const cache = new Map(), consumed = new Map(), gets = [], reads = [], writes = [], uploads = [], picks = [], timings = [], back = [], queues = new Map();
  let root, attempt = 0, sharedId = 0, collecting = null;
  const park = (store, args) => { const d = deferred(); store.push({ ...d, args }); return d.promise; };
  const invokeGet = (...args) => { const queued = queues.get('get')?.shift(); gets.push({ args, deferred: queued }); return queued ? queued.promise : Promise.resolve(serverPost(...args)); };
  const colors = new Proxy({}, { get: () => '#123456' });
  const theme = { colors, radius: { base: 12, control: 16 }, space: { md: 16 } };
  const Native = {
    View: 'View', Text: 'Text', TextInput: 'TextInput', Pressable: 'Pressable', ScrollView: 'ScrollView', KeyboardAvoidingView: 'KeyboardAvoidingView',
    Platform: { OS: 'android' }, StyleSheet: { create: x => x, flatten, absoluteFill: {} }, useWindowDimensions: () => ({ width: 390, height: 844, fontScale: 1, scale: 1 }),
    BackHandler: { addEventListener: (_name, callback) => { const entry = { callback, live: true }; back.push(entry); return { remove: () => { entry.live = false; } }; } },
    AccessibilityInfo: { isReduceMotionEnabled: () => Promise.resolve(reduced), addEventListener: () => ({ remove() {} }) },
  };
  const reanimated = {
    __esModule: true,
    default: { View: 'AnimatedView', createAnimatedComponent: component => component }, ReduceMotion: { Always: 'always', Never: 'never' }, Easing: { bezier: (...x) => x }, useReducedMotion: () => reduced,
    useSharedValue(initial) {
      const holder = React.useRef(null);
      if (!holder.current) { let value = initial; const id = ++sharedId; const shared = {};
        Object.defineProperty(shared, 'value', { get() { if (collecting) collecting.add(id); return value; }, set(next) { if (next && next.nativeAnimation) { value = next.target; if (next.kind === 'timing') timings.push({ ...next, shared: id }); } else value = next; } }); holder.current = shared;
      }
      return holder.current;
    },
    useAnimatedStyle(fn) { const used = new Set(); collecting = used; const style = fn(); collecting = null; return { ...style, syntheticSharedIds: [...used] }; },
    withSpring: (target, config) => ({ nativeAnimation: true, kind: 'spring', target, config }),
    withTiming: (target, config, callback) => ({ nativeAnimation: true, kind: 'timing', target, config, callback }),
    runOnJS: fn => fn, interpolate: value => value,
  };
  const gesture = () => { const g = {}; for (const name of ['activeOffsetY', 'onUpdate', 'onEnd']) g[name] = value => { g[name + 'Value'] = value; return g; }; return g; };
  const adapters = {
    'react': React, 'react/jsx-runtime': dependency('react/jsx-runtime'), 'react-native': Native,
    'expo-modules-core': { requireOptionalNativeModule: name => { assert.equal(name, 'RudiFontScale'); return { getFontScale: () => state.nativeFont, addListener: (event, callback) => { assert.equal(event, 'onFontScaleChanged'); fontListeners.add(callback); return { remove: () => fontListeners.delete(callback) }; } }; } },
    'react-native-reanimated': reanimated, 'react-native-gesture-handler': { Gesture: { Pan: gesture }, GestureDetector: 'GestureDetector' },
    'react-native-safe-area-context': { useSafeAreaInsets: () => ({ top: 24, bottom: 12, left: 0, right: 0 }) },
    '@expo/vector-icons': { Ionicons: 'Ionicons' }, 'react-native-svg': { __esModule: true, default: 'Svg', Circle: 'Circle', Path: 'Path' }, 'expo-router/build/react-navigation/core/NavigationContext': { NavigationContext: React.createContext(null) },
    'expo-router': { useLocalSearchParams: () => ({ id: state.id }), useRouter: () => ({ push() {}, replace() {} }) },
    'expo-image': { Image: 'Image' }, 'expo-image-picker': { launchImageLibraryAsync: (...args) => park(picks, args) },
    'expo-image-manipulator': { SaveFormat: { JPEG: 'jpeg' }, manipulateAsync: () => Promise.resolve({ uri: 'file:///synthetic-clean.jpg', width: 32, height: 32 }) },
    'expo-haptics': { ImpactFeedbackStyle: { Medium: 'medium' }, NotificationFeedbackType: { Success: 'success' }, selectionAsync() {}, impactAsync() {}, notificationAsync() {} },
    'apps/mobile/src/api': { ApiError: class ApiError extends Error {}, newAttempt: () => ({ key: `synthetic-attempt-${++attempt}` }), translatedAsActor: () => { throw new Error('Unexpected mutation transport'); } },
    'apps/mobile/src/rudi/session': { useRudiSession: () => ({ phien: state.actor ? { person_id: state.actor } : null }) },
    'apps/mobile/src/rudi/theme': { typography: { body: { fontSize: 17, lineHeight: 24 } }, lopPhu: { toi: () => '#00000066' }, useRudiTheme: () => theme },
    'apps/mobile/src/rudi/ui': { Field: 'Field', IconButton: 'IconButton', RudiButton: 'RudiButton', TopBar: 'TopBar', RudiScreen: props => React.createElement('RudiScreen', props, props.overlay, props.header, props.children, props.footer) },
    'apps/mobile/src/rudi/nep/NepProvider': { useNepGui: () => null },
    'apps/mobile/src/rudi/ui/AvatarNguoi': { AvatarNguoi: 'AvatarNguoi' }, 'apps/mobile/src/rudi/ui/Skeleton': { SkeletonLines: 'SkeletonLines' },
    'apps/mobile/src/rudi/ui/HoiTaiHang': { HoiTaiHang: 'HoiTaiHang' }, 'apps/mobile/src/rudi/ui/CauTaiCho': { CauTaiCho: 'CauTaiCho' },
    'apps/mobile/src/rudi/screens/nguoi/NoiDungBaoCao': { NoiDungBaoCao: 'NoiDungBaoCao' },
    'apps/mobile/src/rudi/community/MentionPicker': { MentionPicker: 'MentionPicker' }, 'apps/mobile/src/rudi/community/PostCard': { PostCard: 'PostCard' },
    'apps/mobile/src/rudi/community/useCommunityStream': { useCommunityStream() {} },
    'apps/mobile/src/rudi/community/api': { COMMUNITY_ERRORS: {}, getPost: invokeGet, readComments: (...args) => { reads.push(args); return Promise.resolve(page()); }, comment: (...args) => park(writes, args), uploadMedia: (...args) => park(uploads, args), imageSource: (_person, uri) => ({ uri }), relativeTime: () => 'synthetic time', feedback: () => { throw new Error('Unexpected feedback'); }, follow: () => { throw new Error('Unexpected follow'); }, likePost: () => { throw new Error('Unexpected like'); } },
  };
  const special = {
    'apps/mobile/src/rudi/community/PostDetail': process.env.PQ_HANDOFF_POSTDETAIL_SOURCE || path.join(sourceRoot, 'apps/mobile/src/rudi/community/PostDetail.tsx'),
    'apps/mobile/src/rudi/community/Comments': process.env.PQ_HANDOFF_COMMENTS_SOURCE || path.join(sourceRoot, 'apps/mobile/src/rudi/community/Comments.tsx'),
    'apps/mobile/src/rudi/community/MediaPicker': path.join(sourceRoot, 'apps/mobile/src/rudi/community/MediaPicker.tsx'),
  };
  function load(relative) {
    relative = relative.replace(/\\/g, '/').replace(/\.(tsx?|js)$/, '');
    if (adapters[relative]) return adapters[relative];
    if (cache.has(relative)) return cache.get(relative).exports;
    let file = special[relative];
    if (!file) file = [sourceRoot, repo].flatMap(base => ['.ts', '.tsx', '/index.ts', ''].map(ext => path.join(base, relative + ext))).find(fs.existsSync);
    assert.ok(file, `actual source or explicit boundary ${relative}`);
    const bytes = fs.readFileSync(file, 'utf8'); consumed.set(relative, file);
    if (file.endsWith('.json')) { const module = { exports: JSON.parse(bytes) }; cache.set(relative, module); return module.exports; }
    const output = ts.transpileModule(bytes, { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true }, reportDiagnostics: true });
    assert.deepEqual((output.diagnostics || []).filter(d => d.category === ts.DiagnosticCategory.Error).map(d => ({ code: d.code, start: d.start, message: ts.flattenDiagnosticMessageText(d.messageText, '\n') })), [], `actual TSX syntax: ${relative}`);
    const module = { exports: {} }; cache.set(relative, module);
    const req = specifier => { if (adapters[specifier]) return adapters[specifier]; assert.ok(specifier.startsWith('.'), `explicit external boundary ${specifier}`); return load(path.posix.normalize(path.posix.join(path.posix.dirname(relative), specifier))); };
    vm.runInThisContext(`(function(require,module,exports){${output.outputText}\n})`, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  if (actualField) adapters['apps/mobile/src/rudi/ui'].Field = load('apps/mobile/src/rudi/ui/Field').Field;
  const PostDetail = load('apps/mobile/src/rudi/community/PostDetail').PostDetail, Comments = load('apps/mobile/src/rudi/community/Comments').Comments, Sheet = load('apps/mobile/src/rudi/ui/Sheet').Sheet;
  const app = () => strict ? React.createElement(React.StrictMode, null, React.createElement(PostDetail)) : React.createElement(PostDetail);
  const h = {
    state, gets, reads, writes, uploads, picks, timings, consumed,
    async nativeFont(snapshot) { await act(async () => { state.nativeFont = snapshot; [...fontListeners].forEach(callback => callback()); }); },
    nativeFontObservers() { return fontListeners.size; },
    get root() { return root; },
    panel() { return root.root.findAllByType('AnimatedView').find(n => n.props.accessibilityLabel === 'Bình luận'); },
    nativeKeyboard() { return h.panel().parent; },
    referenceScroll() { const nodes = h.panel().findAllByType('ScrollView').filter(n => typeof n.props.onContentSizeChange === 'function'); assert.equal(nodes.length, 1); return nodes[0]; },
    detailsScroll() { return root.root.findAllByType('ScrollView').find(n => n.props.testID === 'community-comment-details'); },
    async resize(height) { await act(async () => h.nativeKeyboard().props.onLayout({ nativeEvent: { layout: { height } } })); },
    editors() { return root.root.findAllByType(Comments); },
    fields() { return root.root.findAllByType(actualField ? 'TextInput' : 'Field').filter(n => (n.props.accessibilityLabel ?? n.props.label) === 'Bình luận'); },
    field() { const nodes = h.fields(); assert.equal(nodes.length, 1, 'exactly one comment field'); return nodes[0]; },
    body() { return h.field().props.value; },
    card() { return root.root.findByType('PostCard'); },
    sheets() { return root.root.findAllByType(Sheet).filter(n => n.props.accessibilityLabel === 'Bình luận'); },
    sheet() { const nodes = h.sheets(); assert.equal(nodes.length, 1, 'one real comment Sheet'); return nodes[0]; },
    writer() { const nodes = h.editors(); assert.equal(nodes.length, 1, 'one writer'); return nodes[0].props.onDraftChange; },
    buttons(label) { return [...root.root.findAllByType('RudiButton'), ...root.root.findAllByType('IconButton')].filter(n => n.props.label === label || n.props.accessibilityLabel === label); },
    button(label) { const nodes = h.buttons(label); assert.equal(nodes.length, 1, `one ${label}`); return nodes[0]; },
    sendButton() { const nodes = [...root.root.findAllByType('RudiButton'), ...root.root.findAllByType('IconButton')].filter(n => /^(Gửi bình luận|Đang gửi…)$/.test(n.props.accessibilityLabel ?? n.props.label)); assert.equal(nodes.length, 1, 'one send'); return nodes[0]; },
    async bodyTo(value) { await act(async () => h.field().props.onChangeText(value)); },
    async open() { await act(async () => h.card().props.onComment()); },
    async close() { await act(async () => { const node = root.root.findAllByType('AnimatedView').find(n => n.props.accessibilityLabel === 'Bình luận'); assert.ok(node, 'actual sheet panel'); node.props.onAccessibilityEscape(); }); },
    closeAnimation() { const node = root.root.findAllByType('AnimatedView').find(n => n.props.accessibilityLabel === 'Bình luận'); assert.ok(node); const ids = node.props.style.flat().flatMap(s => s?.syntheticSharedIds || []); const event = timings.filter(t => ids.includes(t.shared) && t.target === 0).at(-1); assert.ok(event?.callback, 'actual Sheet withTiming completion'); return event; },
    async complete(event, finished = true) { await act(async () => event.callback(finished)); },
    async send() { const b = h.sendButton(); assert.equal(b.props.disabled, false, 'sendable'); await act(async () => b.props.onPress()); },
    async finish(call, value) { await act(async () => call.resolve(value)); }, async fail(call, error) { await act(async () => call.reject(error)); },
    deferGet() { const d = deferred(); if (!queues.has('get')) queues.set('get', []); queues.get('get').push(d); return d; },
    async actorTo(actor) { await act(async () => { state.actor = actor; root.update(app()); }); },
    async postTo(id) { await act(async () => { state.id = id; root.update(app()); }); },
    async reply() { const n = root.root.findAllByType('Pressable').find(p => p.findAllByType('Text').some(t => t.props.children === 'Trả lời')); assert.ok(n); await act(async () => n.props.onPress()); },
    async mentions(value) { await act(async () => root.root.findByType('MentionPicker').props.onChange(value)); },
    async openTools() { const disclosure = root.root.findAllByType('Pressable').find(node => /^(Thêm vào bình luận|Đóng công cụ bình luận)$/.test(node.props.accessibilityLabel)); if (disclosure && !disclosure.props.accessibilityState?.expanded) await act(async () => disclosure.props.onPress()); },
    async pendingFile() { await h.openTools(); await act(async () => h.button('Thêm ảnh').props.onPress()); await h.finish(picks.at(-1), { canceled: false, assets: [{ uri: 'file:///synthetic-pending.jpg', width: 32, height: 32, type: 'image', mimeType: 'image/jpeg' }] }); await h.fail(uploads.at(-1), new Error('Synthetic upload offline')); },
    async unmount() { if (root) { const current = root; root = null; await act(async () => current.unmount()); } },
  };
  await act(async () => { root = Renderer.create(app()); });
  try { await run(h); } finally { await h.unmount(); }
}

test('single editor owns closing sheet until actual completion; reopen and stale completion are serialized', async () => {
  await withCase(async h => {
    await h.bodyTo('Synthetic inline draft'); const inlineWriter = h.writer(); await h.actorTo('synthetic-A'); assert.equal(h.writer(), inlineWriter, 'journal callback stays stable on parent rerender'); await h.open(); assert.equal(h.editors().length, 1); assert.equal(h.sheet().props.open, true);
    await h.bodyTo('Synthetic sheet draft'); const writer = h.writer(); await h.close(); const closing = h.closeAnimation();
    assert.equal(h.editors().length, 1, 'closing keeps exactly one live editor'); assert.equal(h.editors()[0].props.expanded, undefined); assert.equal(h.writer() === writer, true);
    await h.open(); assert.equal(h.sheet().props.open, false, 'reopen is ignored while closing'); assert.equal(h.editors().length, 1);
    await h.complete(closing); assert.equal(h.sheets().length, 0); assert.equal(h.editors().length, 1); assert.equal(h.editors()[0].props.expanded, true); assert.equal(h.body(), 'Synthetic sheet draft');
    await act(async () => inlineWriter(taggedDraft('Synthetic retired inline overwrite')));
    await h.open(); assert.equal(h.body(), 'Synthetic sheet draft'); await h.complete(closing); assert.equal(h.sheet().props.open, true, 'old callback cannot hide reopened sheet'); assert.equal(h.editors().length, 1);
  });
});

test('ACK while sheet is closing publishes cleared draft before inline handoff', async () => {
  await withCase(async h => {
    await h.open(); await h.bodyTo('Synthetic accepted draft'); await h.reply(); await h.mentions(['synthetic-B']); await h.pendingFile(); await act(async () => h.button('Bỏ tệp đang chờ').props.onPress()); await h.send();
    await h.close(); const closing = h.closeAnimation(); await h.finish(h.writes[0], { id: 'synthetic-accepted' }); assert.equal(h.body(), '');
    await h.complete(closing); assert.equal(h.body(), ''); assert.equal(h.button('Gửi bình luận').props.disabled, true); assert.equal(h.buttons('Thử lại tệp đã chọn').length, 0);
    await h.open(); assert.equal(h.body(), ''); assert.equal(h.buttons('Hủy trả lời').length, 0);
  });
});

test('ACK after inline handoff cannot overwrite newer body, reply, mentions or selected-file journal', async () => {
  await withCase(async h => {
    await h.open(); await h.bodyTo('Synthetic old in-flight'); await h.send(); await h.close(); const closing = h.closeAnimation(); const oldWriter = h.writer(); await h.complete(closing);
    await h.bodyTo('Synthetic newer inline'); await h.reply(); await h.mentions(['synthetic-B']); await h.pendingFile();
    await h.finish(h.writes[0], { id: 'synthetic-late-ACK' }); await act(async () => oldWriter(taggedDraft('Synthetic old writer overwrite')));
    await h.open(); assert.equal(h.body(), 'Synthetic newer inline'); assert.equal(h.button('Hủy trả lời').props.accessibilityLabel ?? h.button('Hủy trả lời').props.label, 'Hủy trả lời'); assert.deepEqual(h.editors()[0].props.initialDraft.mentions, ['synthetic-B']); assert.equal(h.editors()[0].props.initialDraft.pendingFile.asset.uri, 'file:///synthetic-pending.jpg');
  });
});

test('same batch accepted ACK and close completion cannot resurrect the submitted draft', async () => {
  await withCase(async h => {
    await h.open(); await h.bodyTo('Synthetic same-batch submitted'); await h.send(); await h.close(); const closing = h.closeAnimation();
    await act(async () => { h.writes[0].resolve({ id: 'synthetic-same-batch-ACK' }); await Promise.resolve(); closing.callback(true); });
    assert.equal(h.editors().length, 1); assert.equal(h.body(), '', 'accepted body must reach journal before retiring writer'); await h.open(); assert.equal(h.body(), '');
  });
});

test('same batch edits and handoff preserve the latest synchronous snapshot and logical ID', async () => {
  await withCase(async h => {
    const edit = h.field().props.onChangeText, open = h.card().props.onComment;
    await act(async () => { edit('Synthetic inline same-batch latest'); open(); }); assert.equal(h.body(), 'Synthetic inline same-batch latest');
    await h.close(); const closing = h.closeAnimation(), sheetEdit = h.field().props.onChangeText;
    await act(async () => { sheetEdit('Synthetic closing same-batch latest'); closing.callback(true); }); assert.equal(h.body(), 'Synthetic closing same-batch latest');
    const logical = h.editors()[0].props.initialDraft.logical_id; await h.open(); assert.equal(h.editors()[0].props.initialDraft.logical_id, logical); assert.equal(h.body(), 'Synthetic closing same-batch latest');
  });
});

test('actor change during closing retires old animation, writer and pending ACK before a verified new editor mounts', async () => {
  await withCase(async h => {
    await h.open(); await h.bodyTo('Synthetic A private draft'); await h.send(); await h.close(); const animation = h.closeAnimation(), writer = h.writer(), close = h.sheet().props.onClose;
    const read = h.deferGet(); await h.actorTo('synthetic-B'); assert.equal(h.editors().length, 0, 'no editor using prior actor post read');
    await h.finish(h.writes[0], { id: 'synthetic-A-old' }); await h.complete(animation); await act(async () => { close(); writer(taggedDraft('Synthetic stale A')); });
    await h.finish(read, serverPost('synthetic-B', 'synthetic-post-1')); assert.equal(h.body(), ''); await h.bodyTo('Synthetic B draft');
    await h.complete(animation); await h.open(); await h.complete(animation); await act(async () => close()); assert.equal(h.sheet().props.open, true); assert.equal(h.body(), 'Synthetic B draft'); assert.equal(h.editors()[0].props.initialDraft.person_id, 'synthetic-B');
  });
});

test('post change during closing rejects old writer/completion and late post read', async () => {
  await withCase(async h => {
    await h.open(); await h.bodyTo('Synthetic old post draft'); await h.close(); const animation = h.closeAnimation(), writer = h.writer();
    const second = h.deferGet(); await h.postTo('synthetic-post-2'); assert.equal(h.editors().length, 0);
    await h.postTo('synthetic-post-3'); await h.bodyTo('Synthetic third post draft');
    await h.finish(second, serverPost('synthetic-A', 'synthetic-post-2')); await h.complete(animation); await act(async () => writer(taggedDraft('Synthetic old post overwrite')));
    assert.equal(h.body(), 'Synthetic third post draft'); await h.open(); await h.complete(animation); assert.equal(h.sheet().props.open, true); assert.equal(h.body(), 'Synthetic third post draft'); assert.equal(h.editors()[0].props.initialDraft.post_id, 'synthetic-post-3');
  });
});

test('reduced motion uses actual zero-duration completion and cancelled completion keeps closing owner', async () => {
  await withCase(async h => {
    await h.bodyTo('Synthetic reduce-motion draft'); await h.open(); await h.close(); const animation = h.closeAnimation(); assert.equal(animation.config.duration, 0);
    assert.equal(h.sheets().length, 1); assert.equal(h.editors().length, 1); await h.complete(animation, false); assert.equal(h.sheets().length, 1, 'finished=false does not hand off');
    await h.complete(animation, true); assert.equal(h.sheets().length, 0); assert.equal(h.body(), 'Synthetic reduce-motion draft');
  }, { reduced: true });
});

test('StrictMode lifecycle and route unmount make retained handoff/ACK callbacks harmless', async () => {
  await withCase(async h => {
    await h.bodyTo('Synthetic strict draft'); await h.open(); await h.send(); await h.close(); const animation = h.closeAnimation(), writer = h.writer(), open = h.card().props.onComment;
    const readsBefore = h.reads.length; await h.unmount(); await act(async () => { animation.callback(true); open(); writer(taggedDraft('Synthetic after unmount')); h.writes[0].resolve({ id: 'synthetic-after-unmount' }); });
    assert.equal(h.reads.length, readsBefore, 'retired ACK does not read into unmounted context');
  }, { strict: true });
});

const isInside = (node, ancestor) => { for (let parent = node.parent; parent; parent = parent.parent) if (parent === ancestor) return true; return false; };
const flatStyle = value => Object.assign({}, ...(Array.isArray(value) ? value.flat(Infinity) : [value]).filter(Boolean));

test('QL01 comment input and send stay outside thread overflow in a keyboard-avoiding measured Sheet', async () => {
  await withCase(async h => {
    await h.open();
    assert.equal(h.nativeKeyboard().type, 'KeyboardAvoidingView');
    assert.equal(h.nativeKeyboard().props.enabled, true, 'comment sheet opts into native keyboard avoidance');
    assert.equal(h.nativeKeyboard().props.behavior, 'height');
    assert.equal(isInside(h.field(), h.referenceScroll()), false, 'active input belongs to the fixed footer');
    assert.equal(isInside(h.sendButton(), h.referenceScroll()), false, 'send does not scroll away with the thread');
    assert.equal(h.field().props.multiline, true); assert.equal(h.field().props.accessibilityLabel, 'Bình luận');
    assert.equal(flatStyle(h.field().props.style).minHeight, 24, 'one line is enough to begin a comment');
    assert.equal(flatStyle(h.field().props.style).maxHeight, 72, 'long text scrolls within the editor');
    const text = h.panel().findAllByType('Text').map(n => n.props.children).filter(v => typeof v === 'string');
    assert.equal(text.filter(v => v === 'Bình luận').length, 1, 'one visible section title, accessible field label is not repeated');
    assert.equal(text.includes('Thêm một lời'), false); assert.equal(h.buttons('Mở rộng').length, 0);
    await h.resize(310);
    const measures = h.panel().findAllByType('View').filter(n => n.props.onLayout);
    assert.equal(measures.length, 2, 'head and fixed footer have independent native measurements');
    await act(async () => {
      measures[0].props.onLayout({ nativeEvent: { layout: { height: 48 } } });
      measures[1].props.onLayout({ nativeEvent: { layout: { height: 200 } } });
      h.referenceScroll().props.onContentSizeChange(320, 400);
    });
    assert.equal(flatStyle(h.panel().props.style).maxHeight, 286, 'available height after IME minus top safe area');
    assert.equal(flatStyle(h.panel().props.style).height, 286, 'reference gives up height before the footer');
    assert.equal(isInside(h.field(), h.referenceScroll()), false);
  }, { actualField: true });
});

test('QL02 attachment recovery and tags may scroll while native input and send remain fixed through handoff', async () => {
  await withCase(async h => {
    await h.open(); await h.bodyTo('Synthetic layout recovery'); await h.pendingFile();
    const details = h.detailsScroll(); assert.ok(details, 'attachment details have a separate bounded viewport');
    assert.equal(flatStyle(details.props.style).maxHeight, 72);
    assert.equal(details.props.keyboardShouldPersistTaps, 'handled'); assert.equal(details.props.showsVerticalScrollIndicator, true);
    assert.equal(isInside(h.button('Thử lại tệp đã chọn'), details), false, 'retry stays outside clipped attachment details');
    assert.equal(isInside(h.button('Bỏ tệp đang chờ'), details), false, 'discard stays outside clipped attachment details');
    assert.equal(isInside(h.field(), details), false); assert.equal(isInside(h.sendButton(), details), false);
    assert.equal(isInside(h.sendButton(), h.referenceScroll()), false, 'thread cannot absorb send when attachments are present');
    await h.openTools(); await act(async () => h.button('Gắn bạn bè').props.onPress()); await h.mentions(['synthetic-B']);
    const mention = h.root.root.findByType('MentionPicker'); assert.equal(isInside(mention, h.detailsScroll()), true); assert.equal(mention.props.expanded, true);
    const input = h.field(), writer = h.writer(); await h.resize(310); await h.resize(844);
    assert.equal(h.field() === input, true, 'native IME layout does not replace EditText'); assert.equal(h.writer() === writer, true);
    await h.close(); await h.complete(h.closeAnimation()); await h.open();
    assert.equal(h.body(), 'Synthetic layout recovery'); assert.deepEqual(h.editors()[0].props.initialDraft.mentions, ['synthetic-B']);
    assert.equal(h.editors()[0].props.initialDraft.pendingFile.asset.uri, 'file:///synthetic-pending.jpg');
  }, { actualField: true });
});

test('QL03 typing, tool expansion, IME resize and failed retry keep the same native input and exact submission', async () => {
  await withCase(async h => {
    await h.open(); const input = h.field(), writer = h.writer();
    await h.bodyTo('Synthetic stable native input'); assert.equal(h.field() === input, true, 'typing must preserve native focus and selection owner');
    await act(async () => h.root.root.findByType('MentionPicker').props.onExpandedChange?.(true)); assert.equal(h.field() === input, true);
    await h.resize(310); assert.equal(h.field() === input, true); assert.equal(h.writer() === writer, true);
    await h.send(); const original = h.writes[0].args; await h.fail(h.writes[0], new Error('Synthetic response lost'));
    assert.equal(h.field() === input, true); assert.equal(h.body(), original[2]); assert.equal(h.sendButton().props.disabled, false);
    await h.send(); assert.deepEqual(h.writes[1].args, original, 'retry keeps logical ID, body and audience metadata');
    await h.fail(h.writes[1], new Error('Synthetic retry offline'));
  }, { actualField: true });
});
// Exercise inline footer placement and hot-metrics ownership with the actual controllers.
// Android keyboard rendering and scroll restoration require native checks.

function linePitchSnapshot(scale = 1) {
  const bases = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6, 11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34];
  const dp = [-1.1, -0.49, -0.24, 0.62, 1.2, 2.3, 18.5, 20.25, 21.5, 23.25, 25, 28.75, 30.25, 31.75, 34.25, 36, 37.5, 39.25, 43, 44.25, 52];
  return { status: 'available', fontScale: scale, fontMetrics: { density: 2, spToDp: bases.map((base, i) => ({ base, dp: scale === 1 ? base : dp[i] })) } };
}
test('LP09 inline Comments live in keyboard-avoiding RudiScreen footer through hot metrics and exact retry', async () => {
  await withCase(async h => {
    const screen = h.root.root.findByType('RudiScreen');
    assert.equal(screen.props.avoidKeyboard, true, 'inline post screen opts into native keyboard avoidance');
    assert.equal(screen.props.scroll, false, 'local reference owns overflow while screen footer stays fixed');
    assert.ok(screen.props.footer, 'inline editor is reserved in fixed screen footer');
    const footerEditorID = 'community-comment-editor';
    assert.equal(screen.props.footer.type === undefined, false, 'footer is an actual React element');
    const footerHost = h.root.root.findAllByType('View').find(node => node.props.testID === footerEditorID);
    assert.ok(footerHost); assert.equal(isInside(h.field(), footerHost), true);
    assert.equal(isInside(h.sendButton(), footerHost), true);
    const reference = h.root.root.findAllByType('ScrollView').filter(node => isInside(h.card(), node));
    assert.equal(reference.length, 1, 'post and thread share one local reference scroll');
    assert.equal(reference[0].props.testID, 'community-post-body');
    assert.equal(isInside(h.field(), reference[0]), false, 'active editor is outside reference overflow');
    assert.equal(isInside(h.sendButton(), reference[0]), false, 'send is outside reference overflow');
    // Check placement at the actual RudiScreen call, not merely a rendered shell.
    const screenCall = h.root.root.findAll(node => typeof node.type === 'function' && node.type.name === 'RudiScreen');
    // The old host intentionally adapts RudiScreen to a native shell; the prop
    // identity below ties that shell's footer to the same production editor.
    assert.equal(screenCall.length, 1);
    assert.equal(screenCall[0].props.footer === screen.props.footer, true);
    assert.equal(h.editors().length, 1); assert.equal(h.nativeFontObservers(), 1);
    await h.nativeFont(linePitchSnapshot(1)); await h.bodyTo('Synthetic inline fixed footer draft');
    await act(async () => reference[0].props.onScroll({ nativeEvent: { contentOffset: { y: 135 } } }));
    const input = h.field(), writer = h.writer();
    await h.send(); const sent = h.writes[0].args;
    await h.fail(h.writes[0], new Error('Synthetic response lost'));
    for (const [value, min, max] of [[linePitchSnapshot(2), 39.5, 118.5], [linePitchSnapshot(1), 24, 72]]) {
      await h.nativeFont(value);
      assert.equal(h.field() === input, true, 'native editor is not replaced on metrics');
      assert.equal(h.writer() === writer, true, 'writer lease is independent of metrics');
      assert.equal(h.body(), sent[2]); assert.equal(h.field().props.allowFontScaling, false);
      assert.equal(flatStyle(h.field().props.style).minHeight, min); assert.equal(flatStyle(h.field().props.style).maxHeight, max);
      assert.equal(h.nativeFontObservers(), 1); assert.equal(h.writes.length, 1);
      const body = h.root.root.findAllByType('ScrollView').find(node => node.props.testID === 'community-post-body');
      assert.equal(body === reference[0], true, 'metrics retain the same local native scroll host');
      assert.deepEqual(body.props.contentOffset, { x: 0, y: 135 }, 'font changes do not change recorded body scroll');
    }
    await h.send(); assert.deepEqual(h.writes[1].args, sent, 'failed-send retry retains logical id and exact payload after metrics');
    await h.fail(h.writes[1], new Error('Synthetic second response lost'));
    await h.open();
    assert.deepEqual(h.root.root.findAllByType('ScrollView').find(node => node.props.testID === 'community-post-body').props.contentOffset, { x: 0, y: 135 }, 'explicit handoff restores local scroll offset');
    assert.equal(h.editors().length, 1); assert.equal(h.sheet().props.avoidKeyboard, true);
    assert.equal(h.editors()[0].props.initialDraft.logical_id, sent[3]);
    assert.equal(h.body(), sent[2]);
    await h.send(); await h.close(); const closing = h.closeAnimation();
    await h.finish(h.writes[2], { id: 'synthetic-accepted', status: 'pending' });
    await h.complete(closing);
    assert.equal(h.body(), '', 'accepted closing-sheet draft is not resurrected inline');
    assert.equal(h.sendButton().props.disabled, true);
    assert.equal(h.root.root.findByType('RudiScreen').props.avoidKeyboard, true);
    assert.ok(h.root.root.findByType('RudiScreen').props.footer);
    assert.deepEqual(h.root.root.findAllByType('ScrollView').find(node => node.props.testID === 'community-post-body').props.contentOffset, { x: 0, y: 135 }, 'close completion restores inline offset');
    assert.equal(h.consumed.get('apps/mobile/src/rudi/community/PostDetail'), path.join(sourceRoot, 'apps/mobile/src/rudi/community/PostDetail.tsx'));
    assert.equal(h.consumed.get('apps/mobile/src/rudi/community/Comments'), path.join(sourceRoot, 'apps/mobile/src/rudi/community/Comments.tsx'));
    assert.equal(h.consumed.get('apps/mobile/src/rudi/ui/Field'), path.join(sourceRoot, 'apps/mobile/src/rudi/ui/Field.tsx'));
  }, { actualField: true });
});

test('LP11 local body offset refuses nonfinite or negative values and resets on a real post context change', async () => {
  await withCase(async h => {
    const body = () => h.root.root.findAllByType('ScrollView').find(node => node.props.testID === 'community-post-body');
    const scroll = async y => act(async () => body().props.onScroll({ nativeEvent: { contentOffset: { y } } }));
    await scroll(87.5);
    for (const invalid of [-1, NaN, Infinity, -Infinity]) await scroll(invalid);
    await h.nativeFont(linePitchSnapshot(2));
    // Inspect the mutable scroll journal after an explicit same-actor route rerender.
    await h.actorTo(h.state.actor);
    assert.deepEqual(body().props.contentOffset, { x: 0, y: 87.5 }, 'only finite nonnegative offsets may be journaled');
    await h.open(); assert.deepEqual(body().props.contentOffset, { x: 0, y: 87.5 });
    await h.close(); await h.complete(h.closeAnimation()); assert.deepEqual(body().props.contentOffset, { x: 0, y: 87.5 });
    await h.postTo('synthetic-post-2');
    assert.deepEqual(body().props.contentOffset, { x: 0, y: 0 }, 'new context cannot inherit prior post scroll');
    assert.equal(h.body(), '', 'new context cannot inherit prior draft');
  }, { actualField: true });
});
