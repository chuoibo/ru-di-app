// Real React executes the copied Comments TSX. Only its imported boundaries are
// replaced: UI/native hosts, theme/router, media/mention children, stream events,
// transport promises and unique attempt minting. No Comments logic is copied here.
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { createRequire } = require('node:module');
const { join } = require('node:path');
const { test, after } = require('node:test');
const vm = require('node:vm');
const { repo, sourceRoot } = require('./repository.cjs');
const { nativeMetricsBoundary } = require('./native-metrics-boundary.cjs');
const appRequire = createRequire(join(repo, 'apps/mobile/package.json'));
const React = appRequire('react');
const Renderer = appRequire('react-test-renderer');
const ts = appRequire('typescript');
const { act } = React;
const sourcePath = process.env.COMMENTS_SOURCE || join(sourceRoot, 'apps/mobile/src/rudi/community/Comments.tsx');
const source = readFileSync(sourcePath, 'utf8');
const result = ts.transpileModule(source, {
  fileName: sourcePath,
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  reportDiagnostics: true,
});
assert.deepEqual((result.diagnostics || []).filter(d => d.category === ts.DiagnosticCategory.Error).map(d => ({ code: d.code, start: d.start, message: ts.flattenDiagnosticMessageText(d.messageText, '\n') })), [], 'actual copied TSX transpiles');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const oldError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) oldError(...args); };
after(() => { console.error = oldError; if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });

function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
const row = (id, extra = {}) => ({ id, author_id: 'reader', author: 'Synthetic reader', body: `body-${id}`, parent_id: null, media_id: null, mentions: [], created_at: '2026-10-01T00:00:00Z', status: 'approved', can_delete: true, ...extra });
const page = (comments = [], next_cursor = null, pending = []) => ({ comments, next_cursor, pending });
const post = id => ({ id, can_comment: true });
async function withCase(run, overrides = {}) {
  const reads = [], writes = [], deletes = [], streams = [], drafts = [];
  let nextAttempt = 0, root;
  const makeCall = (store, args) => { const d = deferred(); store.push({ ...d, args }); return d.promise; };
  const community = {
    COMMUNITY_ERRORS: {},
    readComments: (...args) => makeCall(reads, args),
    comment: (...args) => makeCall(writes, args),
    imageSource: (_person, path) => ({ uri: path }),
    relativeTime: () => 'synthetic time',
  };
  const native = { Pressable: 'Pressable', ScrollView: 'ScrollView', Text: 'Text', View: 'View' };
  const mocks = {
    '../../api': { translatedAsActor: (...args) => makeCall(deletes, args), newAttempt: () => ({ key: `attempt-${++nextAttempt}`, at: nextAttempt }) },
    './api': community,
    react: React,
    'react/jsx-runtime': appRequire('react/jsx-runtime'),
    'react-native': native,
    'expo-image': { Image: 'Image' },
    'expo-router': { useRouter: () => ({ push() {} }) },
    '../theme': { typography: { body: { fontSize: 17, lineHeight: 24 } }, useRudiTheme: () => ({ colors: {} }) },
    ...nativeMetricsBoundary({ dependency: appRequire, React, native, repo, sourceRoot }),
    '../ui': { Field: 'Field', IconButton: 'IconButton', RudiButton: 'RudiButton' },
    '../ui/RudiIcon': { RudiIcon: 'RudiIcon' }, '../ui/PressScale': { PressScale: 'PressScale' },
    '../ui/AvatarNguoi': { AvatarNguoi: 'AvatarNguoi' },
    '../ui/Skeleton': { SkeletonLines: 'SkeletonLines' },
    '../ui/HoiTaiHang': { HoiTaiHang: 'HoiTaiHang' },
    '../ui/CauTaiCho': { CauTaiCho: 'CauTaiCho' },
    './MediaPicker': { MediaPicker: props => React.createElement('MediaPicker', props, props.renderLayout?.(null, null, { preview: null, notice: null, busy: false, pending: props.initialFile != null })) },
    './MentionPicker': { MentionPicker: 'MentionPicker' },
    './useCommunityStream': { useCommunityStream: (person, posts, callback) => { streams.push({ person, posts, callback }); } },
  };
  const module = { exports: {} };
  const execute = vm.runInThisContext(`(function(require,module,exports){${result.outputText}\n})`, { filename: sourcePath });
  execute(specifier => { assert.ok(Object.hasOwn(mocks, specifier), `declared boundary ${specifier}`); return mocks[specifier]; }, module, module.exports);
  const Comments = module.exports.Comments;
  let props = { person: 'actor-a', post: post('post-a'), onDraftChange: draft => drafts.push(draft), ...overrides };
  const h = {
    reads, writes, deletes, streams, drafts,
    get root() { return root; },
    get props() { return props; },
    hosts(type) { return root.root.findAllByType(type); },
    one(type) { const rows = this.hosts(type); assert.equal(rows.length, 1, `one ${type}`); return rows[0]; },
    button(label) { const rows = [...this.hosts('RudiButton'), ...this.hosts('IconButton')].filter(node => node.props.label === label || node.props.accessibilityLabel === label); assert.equal(rows.length, 1, `one button ${label}`); return rows[0]; },
    text() { return this.hosts('Text').map(node => node.children.filter(c => typeof c === 'string').join('')).join('\n'); },
    readError() { return this.hosts('CauTaiCho').find(node => node.props.testID === 'cong-dong-loi-binh-luan').props.cau; },
    draft() { return drafts.at(-1); },
    async update(next) { props = { ...props, ...next }; await act(async () => root.update(React.createElement(Comments, props))); },
    async unmount() { const old = root; root = null; await act(async () => old.unmount()); },
    async press(label) {
      if (label === 'Xóa' && !this.hosts('RudiButton').some(node => node.props.label === label)) {
        const choices = this.hosts('PressScale').filter(node => node.props.accessibilityLabel === 'Thêm lựa chọn cho bình luận');
        assert.equal(choices.length, 1, 'one actual comment choices control');
        await act(async () => choices[0].props.onPress());
      }
      await act(async () => this.button(label).props.onPress());
    },
    async body(value) { await act(async () => this.one('Field').props.onChangeText(value)); },
    async sync() { const callback = streams.at(-1).callback; await act(async () => callback({ kind: 'sync', cursor: 1 })); },
    async changed() { const callback = streams.at(-1).callback; await act(async () => callback({ kind: 'post.changed', cursor: 2 })); },
    async resolve(call, value) { await act(async () => { call.resolve(value); await Promise.resolve(); }); },
    async reject(call, value = new Error('synthetic transport failure')) { await act(async () => { call.reject(value); await Promise.resolve(); }); },
  };
  try { await act(async () => { root = Renderer.create(React.createElement(Comments, props)); }); await run(h); }
  finally { if (root) await act(async () => root.unmount()); }
}

test('refresh supersedes append; duplicate and stale cursor appends are refused', async () => withCase(async h => {
  await h.resolve(h.reads[0], page([row('first')], 'cursor-1'));
  const oldAppend = h.button('Đọc tiếp bình luận').props.onPress;
  await act(async () => { oldAppend(); oldAppend(); });
  assert.equal(h.reads.length, 2, 'only one append per live cursor');
  await h.sync();
  assert.equal(h.reads.length, 3, 'refresh starts while append is in flight');
  await h.resolve(h.reads[2], page([row('fresh')], 'cursor-2', [row('pending-new', { status: 'pending' })]));
  await h.resolve(h.reads[1], page([row('stale')], 'cursor-old', [row('pending-old', { status: 'pending' })]));
  assert.match(h.text(), /body-fresh/);
  assert.match(h.text(), /body-pending-new/);
  assert.doesNotMatch(h.text(), /body-stale|body-pending-old|body-first/);
  await act(async () => oldAppend());
  assert.equal(h.reads.length, 3, 'old captured cursor cannot append into new page');
  await h.press('Đọc tiếp bình luận');
  assert.deepEqual(h.reads[3].args, ['actor-a', 'post-a', 'cursor-2']);
}));

test('newest refresh wins both successful and failed older reads', async () => withCase(async h => {
  await h.sync();
  await h.resolve(h.reads[1], page([row('newest')]));
  await h.resolve(h.reads[0], page([row('old-first')]));
  assert.match(h.text(), /body-newest/);
  assert.doesNotMatch(h.text(), /body-old-first/);
  await h.sync(); await h.sync();
  await h.resolve(h.reads[3], page([row('latest')]));
  await h.reject(h.reads[2], new Error('stale-error'));
  assert.equal(h.readError(), null, 'an obsolete failure cannot paint current read error');
  assert.match(h.text(), /body-latest/);
}));

test('send refresh supersedes append and reconnect retains input', async () => withCase(async h => {
  await h.resolve(h.reads[0], page([row('first')], 'cursor-1'));
  await h.body('synthetic draft');
  const logical = h.draft().logical_id;
  await h.press('Đọc tiếp bình luận');
  await h.sync();
  assert.equal(h.one('Field').props.value, 'synthetic draft');
  assert.equal(h.draft().logical_id, logical);
  await h.resolve(h.reads[2], page([row('reconnected')], 'cursor-2'));
  await h.press('Gửi bình luận');
  await h.resolve(h.writes[0], { id: 'sent', status: 'pending' });
  assert.equal(h.reads.length, 4, 'send completion starts a fresh first page');
  await h.resolve(h.reads[3], page([row('after-send')], null, [row('sent', { status: 'pending' })]));
  await h.resolve(h.reads[1], page([row('stale-append')], 'obsolete'));
  assert.match(h.text(), /body-after-send|body-sent/);
  assert.doesNotMatch(h.text(), /body-stale-append|body-reconnected/);
  assert.equal(h.one('Field').props.value, '');
}));

test('reply failure retains all input; cancel reply rotates logical id before resend', async () => withCase(async h => {
  await h.resolve(h.reads[0], page([row('parent')]));
  await h.body('same body');
  await act(async () => h.one('Pressable').props.onPress());
  const media = [{ id: 'media-a' }], mentions = ['friend-a'], file = { asset: { uri: 'file:///synthetic' } };
  await act(async () => { h.one('MediaPicker').props.onChange(media); h.one('MediaPicker').props.onFileChange(file); h.one('MentionPicker').props.onChange(mentions); });
  assert.equal(h.button('Gửi bình luận').props.disabled, true, 'pending selection waits for retry or explicit discard');
  await act(async () => h.one('MediaPicker').props.onFileChange(null));
  const originalKey = h.draft().logical_id;
  await h.press('Gửi bình luận');
  assert.deepEqual(h.writes[0].args, ['actor-a', 'post-a', 'same body', originalKey, 'parent', 'media-a', mentions]);
  await h.reject(h.writes[0]);
  assert.equal(h.one('Field').props.value, 'same body');
  assert.deepEqual(h.draft().media, media); assert.deepEqual(h.draft().mentions, mentions);
  assert.equal(h.draft().pendingFile, null); assert.equal(h.draft().parent.id, 'parent');
  assert.equal(h.draft().logical_id, originalKey, 'unchanged failed payload keeps identity');
  await h.press('Hủy trả lời');
  assert.equal(h.one('Field').props.value, 'same body');
  assert.notEqual(h.draft().logical_id, originalKey, 'parent edit must mint a new payload identity');
  await h.press('Gửi bình luận');
  assert.equal(h.writes[1].args[4], null); assert.notEqual(h.writes[1].args[3], originalKey);
}));

test('successful older send does not erase changed reply, mentions or pending attachment', async () => withCase(async h => {
  await h.resolve(h.reads[0], page([row('parent')])); await h.body('keep this');
  await act(async () => h.one('Pressable').props.onPress());
  await h.press('Gửi bình luận');
  await h.press('Hủy trả lời');
  const file = { asset: { uri: 'file:///new-pending' } };
  await act(async () => { h.one('MentionPicker').props.onChange(['new-friend']); h.one('MediaPicker').props.onFileChange(file); });
  const newKey = h.draft().logical_id;
  await h.resolve(h.writes[0], { id: 'older-send', status: 'pending' });
  assert.equal(h.one('Field').props.value, 'keep this');
  assert.equal(h.draft().parent, null); assert.deepEqual(h.draft().mentions, ['new-friend']);
  assert.deepEqual(h.draft().pendingFile, file); assert.equal(h.draft().logical_id, newKey);
  assert.equal(h.reads.length, 2);
}));

for (const switchBy of ['actor', 'post']) {
  test(`${switchBy} switch resets draft and isolates old reads/send/delete/child callbacks`, async () => withCase(async h => {
    await h.resolve(h.reads[0], page([row('old-row')], 'old-cursor'));
    await h.body('old draft');
    await act(async () => h.one('MediaPicker').props.onChange([{ id: 'old-media' }]));
    await h.press('Gửi bình luận');
    await h.press('Xóa');
    await act(async () => h.one('HoiTaiHang').props.onDongY());
    await h.sync();
    const oldRead = h.reads[1], oldStream = h.streams.at(-1).callback;
    const oldMedia = h.one('MediaPicker').props, oldMention = h.one('MentionPicker').props, oldBody = h.one('Field').props.onChangeText;
    const oldDraft = h.draft();
    await h.update({ ...(switchBy === 'actor' ? { person: 'actor-b' } : { post: post('post-b') }), initialDraft: oldDraft });
    assert.equal(h.one('Field').props.value, '', 'old context draft is not adopted');
    assert.deepEqual(h.draft().media, []); assert.deepEqual(h.draft().mentions, []); assert.equal(h.draft().parent, null);
    assert.equal(h.draft().pendingFile, null); assert.equal(h.button('Gửi bình luận').props.disabled, true);
    assert.doesNotMatch(h.text(), /body-old-row/);
    assert.equal(h.hosts('HoiTaiHang').length, 0);
    await h.body('new draft');
    const currentKey = h.draft().logical_id;
    const currentReads = h.reads.length, currentWrites = h.writes.length;
    await act(async () => { oldStream({ kind: 'sync', cursor: 1 }); oldStream({ kind: 'post.changed', cursor: 2 }); oldMedia.onChange([{ id: 'late-media' }]); oldMedia.onFileChange({ asset: {} }); oldMedia.onBusy(true); oldMention.onChange(['late-friend']); oldBody('late body'); });
    await h.resolve(oldRead, page([row('late-old-read')], 'late-cursor'));
    await h.resolve(h.writes[0], { id: 'old-send', status: 'pending' });
    await h.resolve(h.deletes[0], undefined);
    assert.equal(h.reads.length, currentReads, 'old completions cannot refresh the previous context');
    assert.equal(h.writes.length, currentWrites);
    assert.equal(h.one('Field').props.value, 'new draft'); assert.equal(h.draft().logical_id, currentKey);
    assert.deepEqual(h.draft().media, []); assert.deepEqual(h.draft().mentions, []); assert.equal(h.draft().pendingFile, null);
    assert.equal(h.button('Gửi bình luận').props.disabled, false, 'old child busy callback cannot disable current send');
    assert.equal(h.hosts('RudiButton').some(n => n.props.label === 'Có bình luận mới'), false);
    assert.doesNotMatch(h.text(), /late-old-read|late body/); assert.equal(h.readError(), null);
  }));
}

test('untagged and mismatched drafts are refused; only current tagged draft restores', async () => {
  const draft = { body: 'restored', parent: null, media: [], mentions: [], logical_id: 'restored-key', pendingFile: null };
  for (const initialDraft of [draft, { ...draft, person_id: 'actor-b', post_id: 'post-a' }, { ...draft, person_id: 'actor-a', post_id: 'post-b' }]) {
    await withCase(async h => { assert.equal(h.one('Field').props.value, '', 'unowned or wrong-context input cannot reveal another draft'); }, { initialDraft });
  }
  await withCase(async h => { assert.equal(h.one('Field').props.value, 'restored'); assert.equal(h.draft().logical_id, 'restored-key'); }, { initialDraft: { ...draft, person_id: 'actor-a', post_id: 'post-a' } });
});

test('current read failure keeps visible comments and retry repairs it', async () => withCase(async h => {
  await h.resolve(h.reads[0], page([row('visible')])); await h.body('keep draft');
  await h.sync(); await h.reject(h.reads[1], new Error('current-read-error'));
  assert.match(h.text(), /body-visible/); assert.equal(h.readError(), 'current-read-error');
  assert.equal(h.one('Field').props.value, 'keep draft');
  await act(async () => h.hosts('CauTaiCho').find(n => n.props.testID === 'cong-dong-loi-binh-luan').props.hanhDong.onPress());
  await h.resolve(h.reads[2], page([row('repaired')]));
  assert.equal(h.readError(), null); assert.match(h.text(), /body-repaired/);
}));


test('unmount makes retained handlers and delayed read/send/delete harmless', async () => withCase(async h => {
  await h.resolve(h.reads[0], page([row('old-row')], 'old-cursor'));
  await h.body('old body');
  await h.press('Gửi bình luận'); await h.press('Xóa');
  const oldDelete = h.one('HoiTaiHang').props.onDongY;
  await act(async () => oldDelete());
  await h.sync();
  const oldSend = h.button('Đang gửi…').props.onPress;
  const oldAppend = h.button('Đọc tiếp bình luận').props.onPress;
  const oldStream = h.streams.at(-1).callback;
  const oldMedia = h.one('MediaPicker').props;
  const count = { reads: h.reads.length, writes: h.writes.length, deletes: h.deletes.length, drafts: h.drafts.length };
  await h.unmount();
  await act(async () => { oldSend(); oldAppend(); oldDelete(); oldStream({ kind: 'sync', cursor: 3 }); oldMedia.onChange([{ id: 'late' }]); });
  await h.resolve(h.reads[1], page([row('late')]));
  await h.reject(h.writes[0]); await h.reject(h.deletes[0]);
  assert.deepEqual({ reads: h.reads.length, writes: h.writes.length, deletes: h.deletes.length, drafts: h.drafts.length }, count);
}));
