// Real copied Comments + MediaPicker execute together. Native picker/manipulator,
// transport promises, stream delivery and visual hosts are the only adapters.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const { test, after } = require('node:test');
const { repo, sourceRoot } = require('./repository.cjs');
const { nativeMetricsBoundary } = require('./native-metrics-boundary.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const React = dependency('react');
const Renderer = dependency('react-test-renderer');
const ts = dependency('typescript');
const { act } = React;
const commentsPath = process.env.PQ_MEDIA_COMMENTS_SOURCE || path.join(sourceRoot, 'apps/mobile/src/rudi/community/Comments.tsx');
const mediaPath = process.env.PQ_MEDIA_PICKER_SOURCE || path.join(sourceRoot, 'apps/mobile/src/rudi/community/MediaPicker.tsx');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const oldError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) oldError(...args); };
after(() => { console.error = oldError; if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };
const commentRow = { id: 'synthetic-root', author_id: 'synthetic-B', author: 'Synthetic B', body: 'Synthetic root', parent_id: null, media_id: null, mentions: [], created_at: '2026-10-01T00:00:00Z', status: 'approved', can_delete: false };
const page = (comments = []) => ({ comments, pending: [], next_cursor: null });
const selectedFile = () => ({ asset: { uri: 'file:///synthetic-only-pending.jpg', width: 32, height: 32, type: 'image', mimeType: 'image/jpeg' } });
const draft = extra => ({ body: 'Synthetic pending body', parent: null, media: [], mentions: [], logical_id: 'synthetic-draft-id', pendingFile: null, person_id: 'synthetic-A', post_id: 'synthetic-post', ...extra });
async function withCase(run, initialDraft = null) {
  const reads = [], writes = [], picks = [], uploads = [], manipulations = [], streams = [], drafts = [];
  let root, attempt = 0;
  let props;
  const park = (store, args) => { const d = deferred(); store.push({ ...d, args }); return d.promise; };
  const community = {
    COMMUNITY_ERRORS: {}, readComments: (...args) => park(reads, args), comment: (...args) => park(writes, args),
    uploadMedia: (...args) => park(uploads, args), imageSource: (_person, uri) => ({ uri }), relativeTime: () => 'synthetic time',
  };
  const native = { View: 'View', Text: 'Text', Pressable: 'Pressable', ScrollView: 'ScrollView' };
  const adapters = {
    '../../api': { newAttempt: () => ({ key: `synthetic-attempt-${++attempt}` }), translatedAsActor: () => { throw new Error('Unexpected transport'); } },
    './api': community, react: React, 'react/jsx-runtime': dependency('react/jsx-runtime'), 'react-native': native,
    'expo-image': { Image: 'Image' },
    'expo-image-picker': { launchImageLibraryAsync: (...args) => park(picks, args) },
    'expo-image-manipulator': { SaveFormat: { JPEG: 'jpeg' }, manipulateAsync: (...args) => { manipulations.push(args); return Promise.resolve({ uri: 'file:///synthetic-only-clean.jpg', width: 32, height: 32 }); } },
    'expo-router': { useRouter: () => ({ push() {} }) }, '../theme': { typography: { body: { fontSize: 17, lineHeight: 24 } }, useRudiTheme: () => ({ colors: {} }) },
    ...nativeMetricsBoundary({ dependency, React, native, repo, sourceRoot }),
    '../ui/RudiIcon': { RudiIcon: 'RudiIcon' }, '../ui/PressScale': { PressScale: 'PressScale' },
    '../ui': { Field: 'Field', IconButton: 'IconButton', RudiButton: 'RudiButton' }, '../ui/AvatarNguoi': { AvatarNguoi: 'AvatarNguoi' },
    '../ui/Skeleton': { SkeletonLines: 'SkeletonLines' }, '../ui/HoiTaiHang': { HoiTaiHang: 'HoiTaiHang' },
    '../ui/CauTaiCho': { CauTaiCho: 'CauTaiCho' }, './MentionPicker': { MentionPicker: 'MentionPicker' },
    './useCommunityStream': { useCommunityStream: (person, ids, callback) => { streams.push({ person, ids, callback }); } },
  };
  const actual = new Map();
  function load(file) {
    if (actual.has(file)) return actual.get(file).exports;
    const source = fs.readFileSync(file, 'utf8');
    const output = ts.transpileModule(source, { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX }, reportDiagnostics: true });
    assert.deepEqual((output.diagnostics || []).filter(d => d.category === ts.DiagnosticCategory.Error).map(d => ({ code: d.code, start: d.start, message: ts.flattenDiagnosticMessageText(d.messageText, '\n') })), [], `actual copied TSX transpiles: ${file}`);
    const module = { exports: {} }; actual.set(file, module);
    const req = specifier => { if (specifier === './MediaPicker') return load(mediaPath); if (specifier === './loi-tep') return load(path.join(sourceRoot, 'apps/mobile/src/rudi/community/loi-tep.ts')); assert.ok(Object.hasOwn(adapters, specifier), `declared boundary ${specifier}`); return adapters[specifier]; };
    vm.runInThisContext(`(function(require,module,exports){${output.outputText}\n})`, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  const Comments = load(commentsPath).Comments;
  const h = {
    reads, writes, picks, uploads, manipulations, drafts,
    async post(id) { props = { ...props, post: { id, can_comment: true }, initialDraft: null }; await act(async () => root.update(React.createElement(Comments, props))); },
    async permission(can_comment) { props = { ...props, post: { ...props.post, can_comment } }; await act(async () => root.update(React.createElement(Comments, props))); },
    async unmount() { const old = root; root = null; await act(async () => old.unmount()); },
    hosts(type) { return root.root.findAllByType(type); },
    one(type) { const nodes = this.hosts(type); assert.equal(nodes.length, 1, `one ${type}`); return nodes[0]; },
    button(label) { const nodes = [...this.hosts('RudiButton'), ...this.hosts('IconButton')].filter(n => n.props.label === label || n.props.accessibilityLabel === label); assert.equal(nodes.length, 1, `one button ${label}`); return nodes[0]; },
    send() { return [...this.hosts('RudiButton'), ...this.hosts('IconButton')].find(n => /^(Gửi bình luận|Đang gửi…)$/.test(n.props.accessibilityLabel ?? n.props.label)); },
    draft() { return drafts.at(-1); },
    async body(value) { await act(async () => h.one('Field').props.onChangeText(value)); },
    async openTools() { const disclosure = h.hosts('PressScale').find(node => /^(Thêm vào bình luận|Đóng công cụ bình luận)$/.test(node.props.accessibilityLabel)); if (!disclosure) return true; if (disclosure.props.disabled) return false; if (!disclosure.props.accessibilityState?.expanded) await act(async () => disclosure.props.onPress()); return true; },
    async press(label) { if (label === 'Thêm ảnh' && ![...h.hosts('RudiButton'), ...h.hosts('IconButton')].some(node => node.props.accessibilityLabel === label)) { if (!await h.openTools()) return false; } const button = h.button(label); if (button.props.disabled) return false; await act(async () => button.props.onPress()); return true; },
    async sendNow() { const button = h.send(); if (button.props.disabled) return false; await act(async () => button.props.onPress()); return true; },
    async finish(call, value) { await act(async () => call.resolve(value)); },
    async fail(call, error) { await act(async () => call.reject(error)); },
    async reply() { const row = h.hosts('Pressable').find(n => n.findAllByType('Text').some(t => t.props.children === 'Trả lời')); assert.ok(row); await act(async () => row.props.onPress()); },
    async mentions(value) { await act(async () => h.one('MentionPicker').props.onChange(value)); },
  };
  props = { person: 'synthetic-A', post: { id: 'synthetic-post', can_comment: true }, initialDraft, expanded: true, onDraftChange: value => drafts.push(value) };
  await act(async () => { root = Renderer.create(React.createElement(Comments, props)); });
  try { await run(h); }
  finally { if (root) await act(async () => root.unmount()); }
}

test('real picker cannot start during send; ACK then old picker cancellation leaves next draft sendable', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic first send'); await h.sendNow();
    // r1 admits this tap and sets uploading=true while logical_id is unchanged.
    await h.press('Thêm ảnh');
    await h.finish(h.writes[0], { id: 'synthetic-comment' }); await h.finish(h.reads[1], page());
    // r1 ACK remounts the picker; its invalidated finally suppresses idle.
    if (h.picks[0]) await h.finish(h.picks[0], { canceled: true, assets: [] });
    await h.body('Synthetic next draft');
    await h.openTools(); assert.equal(h.button('Thêm ảnh').props.disabled, false, 'new real picker is idle');
    assert.equal(h.send().props.disabled, false, `next send must be enabled; reason=${h.send().props.lyDo}`);
    assert.equal(h.picks.length, 0, 'native picker never launched during pending send');
  });
});

test('real picker requires explicit pending discard before send and retains newer reply and mentions on old ACK', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page([commentRow]));
    assert.equal(h.send().props.disabled, true); await h.press('Bỏ tệp đang chờ'); const before = h.draft(); await h.sendNow();
    await h.reply(); await h.mentions(['synthetic-B']);
    await h.finish(h.writes[0], { id: 'synthetic-old-comment' }); await h.finish(h.reads[1], page([commentRow]));
    assert.equal(h.draft().body, before.body); assert.equal(h.draft().parent.id, commentRow.id); assert.deepEqual(h.draft().mentions, ['synthetic-B']); assert.equal(h.draft().pendingFile, null);
    await h.press('Thêm ảnh'); await h.finish(h.picks[0], { canceled: false, assets: [selectedFile().asset] }); assert.equal(h.uploads.length, 1);
    await h.finish(h.uploads[0], { id: 'synthetic-ready-image', type: 'image/jpeg', url: '/synthetic-image', state: 'ready' });
    assert.equal(h.draft().media[0].id, 'synthetic-ready-image'); assert.equal(h.draft().pendingFile, null); assert.equal(h.send().props.disabled, false);
    await h.sendNow(); const input = h.writes[1].args;
    assert.equal(input[4], commentRow.id); assert.equal(input[5], 'synthetic-ready-image'); assert.deepEqual(input[6], ['synthetic-B']);
    await h.fail(h.writes[1], new Error('Synthetic offline'));
  }, draft({ pendingFile: selectedFile() }));
});

test('synchronous picker busy signal prevents stale send handler before React state rerender', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic picker-first draft');
    await h.openTools(); const pick = h.button('Thêm ảnh').props.onPress, send = h.send().props.onPress;
    await act(async () => { pick(); send(); });
    assert.equal(h.picks.length, 1); assert.equal(h.writes.length, 0, 'send observes immediate onBusy signal');
    await h.finish(h.picks[0], { canceled: true, assets: [] }); assert.equal(h.send().props.disabled, false);
  });
});

test('real picker retained handlers cannot add/remove existing or pending files once enclosing send disables it', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page());
    const oldRemove = h.button('Bỏ tệp đính kèm 1').props.onPress, oldDrop = h.button('Bỏ tệp đang chờ').props.onPress, oldRetry = h.button('Thử lại tệp đã chọn').props.onPress;
    await h.press('Bỏ tệp đang chờ'); await h.openTools(); const oldPick = h.button('Thêm ảnh').props.onPress; await h.sendNow(); const snapshot = h.draft();
    assert.equal(h.button('Bỏ tệp đính kèm 1').props.disabled, true);
    await act(async () => { oldPick(); oldRemove(); oldDrop(); oldRetry(); });
    assert.equal(h.picks.length, 0); assert.equal(h.uploads.length, 0); assert.deepEqual(h.draft().media, snapshot.media); assert.deepEqual(h.draft().pendingFile, snapshot.pendingFile); assert.equal(h.draft().logical_id, snapshot.logical_id);
    await h.fail(h.writes[0], new Error('Synthetic response lost'));
    assert.equal(h.button('Bỏ tệp đính kèm 1').props.disabled, false); assert.equal(await h.press('Bỏ tệp đính kèm 1'), true); assert.deepEqual(h.draft().media, []);
  }, draft({ media: [{ id: 'synthetic-existing', type: 'image/jpeg', url: '/synthetic-existing' }], pendingFile: selectedFile() }));
});


test('QF01 same-turn send refuses retained picker before disabled rerender and leaves the next draft usable', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic send-first race');
    await h.openTools(); const send = h.send().props.onPress, pick = h.button('Thêm ảnh').props.onPress;
    await act(async () => { send(); pick(); }); assert.equal(h.writes.length, 1);
    assert.equal(h.picks.length, 0, 'immediate parent submission guard prevents native picker launch');
    await h.finish(h.writes[0], { id: 'synthetic-race-comment' }); await h.finish(h.reads[1], page());
    await h.body('Synthetic next after ACK'); assert.equal(h.send().props.disabled, false);
  });
});

test('a picker admitted by a newer post editor survives an older retired editor ACK', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic old send before new image'); await h.sendNow();
    await h.post('synthetic-next-post'); await h.finish(h.reads[1], page()); await h.body('Synthetic next image draft');
    await h.press('Thêm ảnh'); await h.finish(h.picks[0], { canceled: false, assets: [selectedFile().asset] });
    assert.equal(h.uploads.length, 1); const before = h.draft();
    await h.finish(h.writes[0], { id: 'synthetic-old-before-image' });
    assert.equal(h.draft().body, before.body); assert.deepEqual(h.draft().pendingFile, before.pendingFile); assert.equal(h.draft().post_id, 'synthetic-next-post');
    await h.finish(h.uploads[0], { id: 'synthetic-new-image', type: 'image/jpeg', url: '/synthetic-new-image', state: 'ready' });
    assert.equal(h.send().props.disabled, false); assert.equal(h.draft().media[0].id, 'synthetic-new-image'); assert.equal(h.draft().pendingFile, null);
  });
});

test('QR01 failed comment upload cannot be silently omitted and retry restores the selected image', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic comment with image');
    await h.press('Thêm ảnh'); await h.finish(h.picks[0], { canceled: false, assets: [selectedFile().asset] });
    await h.fail(h.uploads[0], new TypeError('java.io.FileNotFoundException /private/synthetic-native.jpg'));
    assert.equal(h.send().props.disabled, true); assert.equal(h.hosts('Text').filter(n => n.props.testID === 'community-comment-send-reason').length, 0, 'the readable upload error replaces a repeated pending helper');
    const rendered = h.hosts('Text').map(n => n.props.children).filter(v => typeof v === 'string').join(' ');
    assert.match(rendered, /Chưa chuẩn bị được tệp/); assert.doesNotMatch(rendered, /java\.|private|native.jpg/);
    const before = h.draft(); await act(async () => h.send().props.onPress()); assert.equal(h.writes.length, 0); assert.deepEqual(h.draft(), before);
    await h.press('Thử lại tệp đã chọn'); await h.finish(h.uploads[1], { id: 'synthetic-ready-after-failure', type: 'image/jpeg', url: '/synthetic-ready', state: 'ready' });
    assert.equal(h.draft().pendingFile, null); assert.equal(h.send().props.disabled, false);
    await h.sendNow(); assert.equal(h.writes[0].args[5], 'synthetic-ready-after-failure'); await h.fail(h.writes[0], new Error('Synthetic offline'));
  });
});

test('QR02 explicit pending discard enables an intentional text-only comment', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); assert.equal(h.send().props.disabled, true);
    await act(async () => h.send().props.onPress()); assert.equal(h.writes.length, 0);
    await h.press('Bỏ tệp đang chờ'); assert.equal(h.draft().pendingFile, null); assert.equal(h.send().props.disabled, false);
    await h.sendNow(); assert.equal(h.writes[0].args[5], null); await h.fail(h.writes[0], new Error('Synthetic offline'));
  }, draft({ pendingFile: selectedFile() }));
});


test('QF02 same-turn send refuses retired pending retry/drop callbacks before props rerender', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); const retry = h.button('Thử lại tệp đã chọn').props.onPress, drop = h.button('Bỏ tệp đang chờ').props.onPress;
    await h.press('Bỏ tệp đang chờ'); const before = h.draft(), send = h.send().props.onPress;
    await act(async () => { send(); retry(); drop(); }); assert.equal(h.writes.length, 1);
    assert.equal(h.manipulations.length, 0, 'no file preparation after submission starts'); assert.equal(h.uploads.length, 0); assert.deepEqual(h.draft(), before);
    await h.fail(h.writes[0], new Error('Synthetic offline')); assert.equal(h.send().props.disabled, false);
  }, draft({ pendingFile: selectedFile() }));
});

test('QF03 retired comment editor cannot launch a native picker from a retained handler', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.openTools(); const pick = h.button('Thêm ảnh').props.onPress;
    await h.unmount(); await act(async () => pick()); assert.equal(h.picks.length, 0, 'retired parent rejects the retained picker launch');
  });
});


test('QF04 comment permission changes retain admitted upload ownership and restore send without a stranded busy flag', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic permission-cycle draft');
    await h.openTools(); const retainedSend = h.send().props.onPress, retainedPick = h.button('Thêm ảnh').props.onPress;
    await h.press('Thêm ảnh'); await h.finish(h.picks[0], { canceled: false, assets: [selectedFile().asset] });
    assert.equal(h.uploads.length, 1);
    await h.permission(false);
    await act(async () => { retainedSend(); retainedPick(); });
    assert.equal(h.writes.length, 0, 'retired permission cannot submit through a retained handler');
    assert.equal(h.picks.length, 1, 'retired permission cannot open another native picker');
    await h.finish(h.uploads[0], { id: 'synthetic-permission-image', type: 'image/jpeg', url: '/synthetic-permission-image', state: 'ready' });
    await h.permission(true);
    assert.equal(h.draft().body, 'Synthetic permission-cycle draft');
    assert.equal(h.draft().media[0]?.id, 'synthetic-permission-image', 'admitted picker retains its completion owner');
    assert.equal(h.draft().pendingFile, null);
    assert.equal(h.send().props.disabled, false, 'admitted completion emitted idle to its original editor');
    await h.sendNow(); assert.equal(h.writes[0].args[5], 'synthetic-permission-image');
    await h.fail(h.writes[0], new Error('Synthetic response lost'));
  });
});
