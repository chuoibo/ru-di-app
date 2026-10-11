// Real copied Comments + MediaPicker execute together. Native picker/manipulator,
// transport promises, stream delivery and visual hosts are the only adapters.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const { test, after } = require('node:test');
const { repo, sourceRoot } = require('./repository.cjs');
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
    'expo-router': { useRouter: () => ({ push() {} }) }, '../theme': { typography: {}, useRudiTheme: () => ({ colors: {} }) },
    '../ui': { Field: 'Field', RudiButton: 'RudiButton' }, '../ui/AvatarNguoi': { AvatarNguoi: 'AvatarNguoi' },
    '../ui/Skeleton': { SkeletonLines: 'SkeletonLines' }, '../ui/HoiTaiHang': { HoiTaiHang: 'HoiTaiHang' },
    '../ui/CauTaiCho': { CauTaiCho: 'CauTaiCho' }, './MentionPicker': { MentionPicker: 'MentionPicker' },
    './useCommunityStream': { useCommunityStream: (person, ids, callback) => { streams.push({ person, ids, callback }); } },
  };
  const actual = new Map();
  function load(file) {
    if (actual.has(file)) return actual.get(file).exports;
    const source = fs.readFileSync(file, 'utf8');
    const output = ts.transpileModule(source, { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX }, reportDiagnostics: true });
    assert.deepEqual((output.diagnostics || []).filter(d => d.category === ts.DiagnosticCategory.Error), [], `actual copied TSX transpiles: ${file}`);
    const module = { exports: {} }; actual.set(file, module);
    const req = specifier => { if (specifier === './MediaPicker') return load(mediaPath); assert.ok(Object.hasOwn(adapters, specifier), `declared boundary ${specifier}`); return adapters[specifier]; };
    vm.runInThisContext(`(function(require,module,exports){${output.outputText}\n})`, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  const Comments = load(commentsPath).Comments;
  const h = {
    reads, writes, picks, uploads, manipulations, drafts,
    hosts(type) { return root.root.findAllByType(type); },
    one(type) { const nodes = this.hosts(type); assert.equal(nodes.length, 1, `one ${type}`); return nodes[0]; },
    button(label) { const nodes = this.hosts('RudiButton').filter(n => n.props.label === label); assert.equal(nodes.length, 1, `one button ${label}`); return nodes[0]; },
    send() { return this.hosts('RudiButton').find(n => /^(Gửi bình luận|Đang gửi…)$/.test(n.props.label)); },
    draft() { return drafts.at(-1); },
    async body(value) { await act(async () => h.one('Field').props.onChangeText(value)); },
    async press(label) { const button = h.button(label); if (button.props.disabled) return false; await act(async () => button.props.onPress()); return true; },
    async sendNow() { const button = h.send(); if (button.props.disabled) return false; await act(async () => button.props.onPress()); return true; },
    async finish(call, value) { await act(async () => call.resolve(value)); },
    async fail(call, error) { await act(async () => call.reject(error)); },
    async reply() { const row = h.hosts('Pressable').find(n => n.findAllByType('Text').some(t => t.props.children === 'Trả lời')); assert.ok(row); await act(async () => row.props.onPress()); },
    async mentions(value) { await act(async () => h.one('MentionPicker').props.onChange(value)); },
  };
  await act(async () => { root = Renderer.create(React.createElement(Comments, { person: 'synthetic-A', post: { id: 'synthetic-post', can_comment: true }, initialDraft, expanded: true, onDraftChange: value => drafts.push(value) })); });
  try { await run(h); }
  finally { await act(async () => root.unmount()); }
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
    assert.equal(h.button('Thêm ảnh').props.disabled, false, 'new real picker is idle');
    assert.equal(h.send().props.disabled, false, `next send must be enabled; reason=${h.send().props.lyDo}`);
    assert.equal(h.picks.length, 0, 'native picker never launched during pending send');
  });
});

test('real picker locks retry/drop controls during send and retains newer reply, mentions and pending file on old ACK', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page([commentRow])); const before = h.draft(); await h.sendNow();
    assert.equal(h.button('Thử lại tệp đã chọn').props.disabled, true);
    assert.equal(h.button('Bỏ tệp đang chờ').props.disabled, true);
    assert.equal(await h.press('Thử lại tệp đã chọn'), false); assert.equal(await h.press('Bỏ tệp đang chờ'), false);
    await h.reply(); await h.mentions(['synthetic-B']);
    await h.finish(h.writes[0], { id: 'synthetic-old-comment' }); await h.finish(h.reads[1], page([commentRow]));
    assert.equal(h.draft().body, before.body); assert.equal(h.draft().parent.id, commentRow.id); assert.deepEqual(h.draft().mentions, ['synthetic-B']);
    assert.deepEqual(h.draft().pendingFile, before.pendingFile);
    assert.equal(await h.press('Thử lại tệp đã chọn'), true); assert.equal(h.uploads.length, 1);
    await h.finish(h.uploads[0], { id: 'synthetic-ready-image', type: 'image/jpeg', url: '/synthetic-image', state: 'ready' });
    assert.equal(h.draft().media[0].id, 'synthetic-ready-image'); assert.equal(h.draft().pendingFile, null);
    assert.equal(h.send().props.disabled, false, 'real upload completion clears busy');
    await h.sendNow(); const input = h.writes[1].args;
    assert.equal(input[4], commentRow.id); assert.equal(input[5], 'synthetic-ready-image'); assert.deepEqual(input[6], ['synthetic-B']);
    await h.fail(h.writes[1], new Error('Synthetic offline'));
  }, draft({ pendingFile: selectedFile() }));
});

test('synchronous picker busy signal prevents stale send handler before React state rerender', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic picker-first draft');
    const pick = h.button('Thêm ảnh').props.onPress, send = h.send().props.onPress;
    await act(async () => { pick(); send(); });
    assert.equal(h.picks.length, 1); assert.equal(h.writes.length, 0, 'send observes immediate onBusy signal');
    await h.finish(h.picks[0], { canceled: true, assets: [] }); assert.equal(h.send().props.disabled, false);
  });
});

test('real picker retained handlers cannot add/remove existing or pending files once enclosing send disables it', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page());
    const oldPick = h.button('Thêm ảnh').props.onPress, oldRemove = h.button('Bỏ tệp').props.onPress, oldDrop = h.button('Bỏ tệp đang chờ').props.onPress, oldRetry = h.button('Thử lại tệp đã chọn').props.onPress;
    await h.sendNow(); const snapshot = h.draft();
    assert.equal(h.button('Bỏ tệp').props.disabled, true);
    await act(async () => { oldPick(); oldRemove(); oldDrop(); oldRetry(); });
    assert.equal(h.picks.length, 0); assert.equal(h.uploads.length, 0); assert.deepEqual(h.draft().media, snapshot.media); assert.deepEqual(h.draft().pendingFile, snapshot.pendingFile); assert.equal(h.draft().logical_id, snapshot.logical_id);
    await h.fail(h.writes[0], new Error('Synthetic response lost'));
    assert.equal(h.button('Bỏ tệp').props.disabled, false); assert.equal(await h.press('Bỏ tệp'), true); assert.deepEqual(h.draft().media, []);
  }, draft({ media: [{ id: 'synthetic-existing', type: 'image/jpeg', url: '/synthetic-existing' }], pendingFile: selectedFile() }));
});


test('same-turn send then retained picker cancellation cannot leave uploading latched after ACK', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic send-first race');
    const send = h.send().props.onPress, pick = h.button('Thêm ảnh').props.onPress;
    await act(async () => { send(); pick(); }); assert.equal(h.writes.length, 1); assert.equal(h.picks.length, 1);
    await h.finish(h.writes[0], { id: 'synthetic-race-comment' }); await h.finish(h.reads[1], page());
    await h.finish(h.picks[0], { canceled: true, assets: [] }); await h.body('Synthetic next after cancellation');
    assert.equal(h.send().props.disabled, false, `real finally must emit idle; reason=${h.send().props.lyDo}`);
  });
});

test('same-turn picker admitted before disabled rerender retains its new file through older ACK', async () => {
  await withCase(async h => {
    await h.finish(h.reads[0], page()); await h.body('Synthetic old send before new image');
    const send = h.send().props.onPress, pick = h.button('Thêm ảnh').props.onPress;
    await act(async () => { send(); pick(); });
    await h.finish(h.writes[0], { id: 'synthetic-old-before-image' }); await h.finish(h.reads[1], page());
    await h.finish(h.picks[0], { canceled: false, assets: [selectedFile().asset] });
    assert.equal(h.uploads.length, 1, 'ACK must retain the active picker generation');
    await h.finish(h.uploads[0], { id: 'synthetic-new-image', type: 'image/jpeg', url: '/synthetic-new-image', state: 'ready' });
    await h.body('Synthetic next image draft');
    assert.equal(h.send().props.disabled, false); assert.equal(h.draft().media[0].id, 'synthetic-new-image'); assert.equal(h.draft().pendingFile, null);
  });
});
