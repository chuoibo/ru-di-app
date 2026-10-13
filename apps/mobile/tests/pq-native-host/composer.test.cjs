const test = require('node:test');
const assert = require('node:assert/strict');
const { createHost, person, post, act } = require('./host.cjs');
const vaultPath = 'apps/mobile/src/rudi/community/ban-nhap';
const host = options => createHost({ candidateRoot: process.env.PQ_COMPOSER_CANDIDATE_ROOT, ...options, mode: process.env.PQ_COMPOSER_VARIANT ?? 'candidate' });
const draft = (body = 'Synthetic unsent', logical_id = 'synthetic-logical') => ({ body, audience: 'public', topics: 'PQ445', mentions: ['synthetic-B'], media: [{ id: 'synthetic-media', url: '/synthetic', type: 'image/jpeg', width: 32, height: 32, duration_ms: 0 }], group: null, revision: 3, logical_id, pendingFile: { asset: { uri: 'file:///synthetic-only', width: 32, height: 32 }, uploaded: { id: 'synthetic-upload', type: 'image/jpeg' } } });

test('body-only draft restores after form remount', async () => {
  const h = host(); await h.mount();
  try { await h.changeBody('Synthetic remount canary'); await h.unmount(); await h.remount(); assert.equal(h.body().props.value, 'Synthetic remount canary'); }
  finally { await h.dispose(); }
});

test('group audience and chosen context survive remount and remain in the submitted payload', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic group draft'); await h.chooseAudience('Một nhóm'); await h.press('Synthetic group');
    await h.unmount(); await h.remount(); const send = h.deferNext('createPost'); await h.pressSend();
    assert.equal(send.call.args[1].audience, 'group'); assert.equal(send.call.args[1].context_id, 'synthetic-group');
    await send.fail(new Error('Synthetic offline'));
  } finally { await h.dispose(); }
});

test('new draft survives form remount with body, tools, selected file and the same retry identity', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic new draft'); await h.changeTopics('PQ445'); await h.changeMentions(['synthetic-B']); await h.changeMedia(draft().media); await h.changeFile(draft().pendingFile);
    const v = h.load(vaultPath), before = v.docNhapBai('synthetic-A', 'new:community');
    await h.unmount(); await h.remount();
    assert.equal(h.body().props.value, 'Synthetic new draft');
    assert.equal(h.field('Chủ đề').props.value, 'PQ445');
    assert.deepEqual(h.media().props.media, draft().media);
    assert.deepEqual(h.media().props.initialFile, draft().pendingFile);
    assert.deepEqual(h.mentions().props.selected, ['synthetic-B']);
    assert.equal(v.docNhapBai('synthetic-A', 'new:community').logical_id, before.logical_id);
  } finally { await h.dispose(); }
});

test('new community, new wall and each edit post have isolated draft scopes', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic community');
    await h.setParams({ wall: '1' }); assert.equal(h.body().props.value, ''); assert.equal(h.send().props.label, 'Đăng lên tường');
    await h.changeBody('Synthetic wall');
    await h.setParams({ edit: 'synthetic-post-one' }); assert.equal(h.body().props.value, 'Synthetic server body');
    await h.changeBody('Synthetic edit one');
    await h.setParams({ edit: 'synthetic-post-two' }); assert.equal(h.body().props.value, 'Synthetic server body');
    await h.changeBody('Synthetic edit two');
    await h.setParams({ edit: 'synthetic-post-one' }); assert.equal(h.body().props.value, 'Synthetic edit one');
    await h.setParams({}); assert.equal(h.body().props.value, 'Synthetic community');
    await h.setParams({ wall: '1' }); assert.equal(h.body().props.value, 'Synthetic wall');
  } finally { await h.dispose(); }
});

test('actual session actor change, logout and current revoke destroy prior draft vault', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic actor A'); await h.setActor(person('synthetic-B'));
    assert.equal(h.body().props.value, '');
    await h.changeBody('Synthetic actor B'); await h.setActor(person()); assert.equal(h.body().props.value, '');
    await h.changeBody('Synthetic before logout'); await h.setActor(null); await h.setActor(person()); assert.equal(h.body().props.value, '');
    await h.changeBody('Synthetic before revoke'); await h.revoke(); await h.setActor(person()); assert.equal(h.body().props.value, '');
  } finally { await h.dispose(); }
});

test('edit GET verifies access before exposing saved content, then hydrates at its base revision', async () => {
  const h = host(); await h.mount();
  try {
    const v = h.load(vaultPath), lease = v.phienNhapBai('synthetic-A');
    v.ghiNhapBai('synthetic-A', 'edit:synthetic-post', { ...draft(), pendingFile: null }, lease);
    const read = h.deferNext('getPost'); await h.setParams({ edit: 'synthetic-post' });
    assert.throws(() => h.body(), /RNTextInput count/);
    await read.fail(new Error('Synthetic forbidden')); assert.throws(() => h.body(), /RNTextInput count/);
    const read2 = h.deferNext('getPost'); await h.press('Thử lại');
    await read2.finish(post()); assert.equal(h.body().props.value, 'Synthetic unsent'); assert.equal(h.send().props.disabled, false);
    const send = h.deferNext('translatedAsActor'); await h.pressSend();
    assert.equal(send.call.args[2].body.revision, 3); assert.equal(send.call.args[2].body.logical_id, 'synthetic-logical');
    await send.fail(new Error('Synthetic offline'));
  } finally { await h.dispose(); }
});

test('changed edit revision keeps local input locked until explicit discard and reread', async () => {
  const h = host(); await h.mount();
  try {
    const v = h.load(vaultPath); v.ghiNhapBai('synthetic-A', 'edit:synthetic-post', draft(), v.phienNhapBai('synthetic-A'));
    const read = h.deferNext('getPost'); await h.setParams({ edit: 'synthetic-post' }); await read.finish(post('synthetic-post', 4));
    assert.equal(h.body().props.value, 'Synthetic unsent'); assert.equal(h.send().props.disabled, true);
    await h.pressSend(); assert.equal(h.callsFor('translatedAsActor').length, 0);
    assert.equal(v.docNhapBai('synthetic-A', 'edit:synthetic-post').body, 'Synthetic unsent');
    const reread = h.deferNext('getPost'); await h.press('Bỏ nháp và mở bản mới'); await reread.finish(post('synthetic-post', 4));
    assert.equal(h.body().props.value, 'Synthetic server body'); assert.equal(h.send().props.disabled, false);
    assert.equal(v.docNhapBai('synthetic-A', 'edit:synthetic-post'), null, 'pristine server body must not recreate discarded local draft');
  } finally { await h.dispose(); }
});

test('late edit read cannot hydrate another post or actor', async () => {
  const h = host(); await h.mount();
  try {
    const old = h.deferNext('getPost'); await h.setParams({ edit: 'synthetic-old' });
    await h.setParams({ edit: 'synthetic-new' }); await h.changeBody('Synthetic current edit');
    await old.finish({ ...post('synthetic-old'), body: 'Synthetic OLD reply' });
    assert.equal(h.body().props.value, 'Synthetic current edit');
    const a = h.deferNext('getPost'); await h.setParams({ edit: 'synthetic-A-late' });
    await h.setActor(person('synthetic-B')); await h.changeBody('Synthetic B edit');
    await a.finish({ ...post('synthetic-A-late'), body: 'Synthetic A private draft' }); assert.equal(h.body().props.value, 'Synthetic B edit');
  } finally { await h.dispose(); }
});

test('failed create retry after remount preserves logical_id and exact payload; changed body rekeys', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic retry'); const first = h.deferNext('createPost'); await h.pressSend(); await first.fail(new Error('Synthetic response lost'));
    await h.unmount(); await h.remount(); const retry = h.deferNext('createPost'); await h.pressSend();
    assert.deepEqual(retry.call.args[1], first.call.args[1]); await retry.fail(new Error('Synthetic still offline'));
    await h.changeBody('Synthetic correction'); const changed = h.deferNext('createPost'); await h.pressSend();
    assert.notEqual(changed.call.args[1].logical_id, first.call.args[1].logical_id); await changed.fail(new Error('Synthetic offline'));
  } finally { await h.dispose(); }
});

test('late successful create clears only its own matching snapshot and never navigates remounted input', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic first send'); const send = h.deferNext('createPost'); await h.pressSend();
    await h.unmount(); await h.remount(); await h.changeBody('Synthetic newer draft');
    await send.finish({ id: 'synthetic-created' });
    assert.equal(h.body().props.value, 'Synthetic newer draft'); assert.equal(h.routes.length, 0);
    const v = h.load(vaultPath); assert.equal(v.docNhapBai('synthetic-A', 'new:community')?.body, 'Synthetic newer draft');
  } finally { await h.dispose(); }
});

test('A logout then same A login rejects old completion even when route and actor match', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic old session'); const send = h.deferNext('createPost'); await h.pressSend();
    await h.setActor(null); await h.setActor(person()); await h.changeBody('Synthetic new session');
    await send.finish({ id: 'synthetic-old-resource' });
    assert.equal(h.body().props.value, 'Synthetic new session'); assert.equal(h.routes.length, 0);
    assert.equal(h.load(vaultPath).docNhapBai('synthetic-A', 'new:community').body, 'Synthetic new session');
  } finally { await h.dispose(); }
});

test('successful create clears only current scope and does not resurrect on remount', async () => {
  const h = host(); await h.mount();
  try {
    await h.changeBody('Synthetic community retained'); await h.setParams({ wall: '1' }); await h.changeBody('Synthetic submitted wall');
    await h.pressSend(); assert.deepEqual(h.routes, [['replace', '/community/posts/synthetic-post']]);
    await h.unmount(); await h.remount(); assert.equal(h.body().props.value, '');
    await h.setParams({}); assert.equal(h.body().props.value, 'Synthetic community retained');
  } finally { await h.dispose(); }
});

test('vault snapshots clone mutable inputs/outputs and reject stale same-actor leases', async () => {
  const h = host(); await h.mount();
  try {
    const v = h.load(vaultPath), lease = v.phienNhapBai('synthetic-A'); const input = draft();
    v.ghiNhapBai('synthetic-A', 'new:community', input, lease); input.mentions.push('mutated'); input.media[0].id = 'mutated'; input.pendingFile.asset.uri = 'mutated';
    const output = v.docNhapBai('synthetic-A', 'new:community'); assert.equal(output.media[0].id, 'synthetic-media'); assert.equal(output.pendingFile.asset.uri, 'file:///synthetic-only'); assert.deepEqual(output.mentions, ['synthetic-B']);
    output.media[0].id = 'mutated read'; assert.equal(v.docNhapBai('synthetic-A', 'new:community').media[0].id, 'synthetic-media');
    v.boNhapBai('synthetic-A', 'new:community', lease, 'wrong-logical'); assert.ok(v.docNhapBai('synthetic-A', 'new:community'));
    await h.setActor(null); await h.setActor(person());
    v.ghiNhapBai('synthetic-A', 'new:community', draft('Synthetic valid new lease'), v.phienNhapBai('synthetic-A'));
    v.ghiNhapBai('synthetic-A', 'new:community', draft('Synthetic stale write'), lease); v.boNhapBai('synthetic-A', 'new:community', lease);
    assert.equal(v.docNhapBai('synthetic-A', 'new:community').body, 'Synthetic valid new lease');
  } finally { await h.dispose(); }
});

test('PV01 legacy writer host delegates the actual viewport hook and preserves caller styles with a provided owner', async () => {
  const { flatten } = require('./native-metrics-boundary.cjs');
  const h = host({ inputViewportHeight: 240 }); await h.mount();
  try {
    await h.changeBody('Synthetic provided viewport draft');
    const input = h.body(), saved = h.load(vaultPath).docNhapBai('synthetic-A', 'new:community'), beforeCalls = h.calls.length;
    const module = h.load('apps/mobile/src/rudi/ui/InputViewport');
    assert.equal(h.load('apps/mobile/src/rudi/ui').useRudiInputViewportHeight === module.useRudiInputViewportHeight, true, 'adapter exposes the actual source hook');
    assert.match(h.consumed.get('apps/mobile/src/rudi/ui/InputViewport'), /\/apps\/mobile\/src\/rudi\/ui\/InputViewport\.tsx$/, 'actual leaf source consumed');
    for (const [height, maximum] of [[240, 208], [320, 288], [null, 844], [240, 208]]) {
      await h.setInputViewportHeight(height);
      const style = flatten(h.body().props.style);
      assert.equal(style.maxHeight, maximum, 'the actual hook reads its provided owner, including explicit null');
      assert.equal(style.fontSize, 17); assert.equal(style.lineHeight, 24);
      assert.equal(style.paddingVertical, 4); assert.equal(style.textAlignVertical, 'top');
      assert.equal(style.color, '#123456', 'geometry forwarding retains incoming caller ink');
      assert.equal(h.body() === input, true, 'provider updates retain the native input');
      assert.equal(h.body().props.value, 'Synthetic provided viewport draft');
      assert.equal(Object.hasOwn(h.body().props, 'bottomClearance'), false, 'custom clearance never reaches the native host');
      assert.deepEqual(h.load(vaultPath).docNhapBai('synthetic-A', 'new:community'), saved, 'viewport updates keep the RAM draft and logical attempt');
    }
    assert.equal(h.calls.length, beforeCalls); assert.deepEqual(h.routes, [], 'context reads have no transport or navigation side effects');
  } finally { await h.dispose(); }
  const absent = host(); await absent.mount();
  try { assert.equal(flatten(absent.body().props.style).maxHeight, 844, 'absent owner uses the actual context default'); }
  finally { await absent.dispose(); }
});
