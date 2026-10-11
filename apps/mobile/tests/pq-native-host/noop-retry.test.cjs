const test = require('node:test');
const assert = require('node:assert/strict');
const { createHost, act } = require('./host.cjs');
const host = options => createHost({ candidateRoot: process.env.PQ_COMPOSER_CANDIDATE_ROOT, ...options });
const vault = 'apps/mobile/src/rudi/community/ban-nhap';
const media = (id = 'synthetic-media', url = '/synthetic-before') => [{ id, url, type: 'image/jpeg', width: 32, height: 32, duration_ms: 0 }];
async function uncertain(h) {
  const send = h.deferNext('createPost'); await h.pressSend();
  await send.fail(new Error('Synthetic response lost; commit status unknown'));
  return send.call.args[1];
}
async function retry(h) {
  const send = h.deferNext('createPost'); await h.pressSend();
  const value = send.call.args[1]; await send.fail(new Error('Synthetic still offline')); return value;
}
async function withHost(run, options) { const h = host(options); await h.mount(); try { await run(h); } finally { await h.dispose(); } }

test('reselecting the selected public audience closes its menu and retries the exact uncertain payload', async () => withHost(async h => {
  await h.changeBody('Synthetic uncertain audience'); const first = await uncertain(h);
  await h.chooseAudience('Công khai · Cộng đồng');
  assert.equal(h.audienceOpen(), false, 'no-op selection still closes the audience menu');
  assert.equal(h.body().props.value, first.body);
  assert.deepEqual(await retry(h), first, 'same selected audience must not mint a second write identity');
  await h.unmount(); await h.remount(); assert.deepEqual(await retry(h), first);
}));

test('reselecting the current group keeps context and uncertain identity', async () => withHost(async h => {
  await h.changeBody('Synthetic uncertain group'); await h.chooseAudience('Một nhóm'); await h.press('Synthetic group');
  const first = await uncertain(h); await h.press('Synthetic group');
  assert.deepEqual(await retry(h), first, 'same selected group must remain an exact retry');
}));

test('identical body, topic and ordered mentions callbacks retain uncertain identity', async () => withHost(async h => {
  await h.changeBody('Synthetic unchanged callback'); await h.changeTopics('Alpha, Beta'); await h.changeMentions(['synthetic-B', 'synthetic-C']);
  const first = await uncertain(h);
  await h.changeBody(first.body); await h.changeTopics('Alpha, Beta'); await h.changeMentions([...first.mentions]);
  assert.deepEqual(await retry(h), first);
}));

test('topic formatting that emits the same ordered strings retains identity and saves the explicit UI edit', async () => withHost(async h => {
  await h.changeBody('Synthetic topic formatting'); await h.changeTopics('Alpha, Beta'); const first = await uncertain(h);
  await h.changeTopics('  Alpha  , , Beta, ');
  assert.equal(h.field('Chủ đề').props.value, '  Alpha  , , Beta, ');
  assert.deepEqual(await retry(h), first);
  await h.unmount(); await h.remount();
  assert.equal(h.field('Chủ đề').props.value, '  Alpha  , , Beta, ');
  assert.deepEqual(await retry(h), first);
}));

test('same media IDs preserve retry while updated metadata and selected file survive remount', async () => withHost(async h => {
  await h.changeBody('Synthetic media metadata'); await h.changeMedia(media()); const first = await uncertain(h);
  const next = media('synthetic-media', '/synthetic-after'); next[0].width = 64;
  const file = { asset: { uri: 'file:///synthetic-next', width: 64, height: 32 }, uploaded: { id: 'synthetic-media', type: 'image/jpeg' } };
  await h.changeMedia(next); await h.changeFile(file);
  assert.deepEqual(await retry(h), first);
  await h.unmount(); await h.remount();
  assert.deepEqual(h.media().props.media, next); assert.deepEqual(h.media().props.initialFile, file);
  assert.deepEqual(await retry(h), first);
}));

test('raw body whitespace and backend-normalized topic spelling are still distinct outgoing payloads', async () => withHost(async h => {
  await h.changeBody('Synthetic raw digest'); await h.changeTopics('Alpha'); const first = await uncertain(h);
  await h.changeBody(first.body + ' '); const whitespace = await retry(h);
  assert.notEqual(whitespace.logical_id, first.logical_id, 'server hashes body before trimming');
  assert.equal(whitespace.body, first.body + ' ');
  await h.changeTopics('alpha'); const lower = await retry(h);
  assert.notEqual(lower.logical_id, whitespace.logical_id, 'server hashes topics before case normalization');
  assert.deepEqual(lower.topics, ['alpha']);
  await h.changeTopics('#alpha'); const hash = await retry(h);
  assert.notEqual(hash.logical_id, lower.logical_id, 'server hashes topics before stripping hashtag');
}));

test('ordered topics, ordered mentions, media IDs and audience changes each rotate identity', async () => withHost(async h => {
  await h.changeBody('Synthetic substantive edits'); await h.changeTopics('Alpha, Beta'); await h.changeMentions(['synthetic-B', 'synthetic-C']); await h.changeMedia(media());
  let previous = await uncertain(h);
  for (const edit of [() => h.changeTopics('Beta, Alpha'), () => h.changeMentions(['synthetic-C', 'synthetic-B']), () => h.changeMedia(media('synthetic-other')), () => h.chooseAudience('Chỉ mình tôi')]) {
    await edit(); const next = await retry(h); assert.notEqual(next.logical_id, previous.logical_id); previous = next;
  }
}));

test('same-batch edit away then back keeps new intent even when the final payload equals the uncertain one', async () => withHost(async h => {
  await h.changeBody('Synthetic uncertain before edit'); const first = await uncertain(h);
  const change = h.body().props.onChangeText;
  await act(async () => { change('Synthetic temporary explicit edit'); change(first.body); });
  const next = await retry(h); assert.equal(next.body, first.body);
  assert.notEqual(next.logical_id, first.logical_id, 'returning to the old text must not resurrect its retired identity');
  assert.equal(h.load(vault).docNhapBai('synthetic-A', 'new:community').logical_id, next.logical_id);
}));

test('same-batch no-op then partial edits submit current combined fields using their current identity', async () => withHost(async h => {
  await h.changeBody('Synthetic before batch'); await h.changeTopics('Alpha'); const first = await uncertain(h);
  const changeBody = h.body().props.onChangeText, changeTopics = h.field('Chủ đề').props.onChangeText, send = h.send().props.onPress;
  const deferred = h.deferNext('createPost');
  await act(async () => { changeBody(first.body); changeTopics('Beta'); changeBody('Synthetic current batch'); send(); });
  const input = deferred.call.args[1];
  assert.equal(input.body, 'Synthetic current batch'); assert.deepEqual(input.topics, ['Beta']);
  assert.notEqual(input.logical_id, first.logical_id);
  await deferred.fail(new Error('Synthetic response lost'));
}));

test('edit mode preserves logical ID for exact payload-equivalent retries without replacing its base revision', async () => withHost(async h => {
  await h.changeBody('Synthetic edit retry'); await h.changeTopics('Alpha');
  const first = h.deferNext('translatedAsActor'); await h.pressSend(); await first.fail(new Error('Synthetic edit response lost'));
  await h.changeTopics(' Alpha, '); await h.changeBody('Synthetic edit retry');
  const second = h.deferNext('translatedAsActor'); await h.pressSend();
  assert.deepEqual(second.call.args[2].body, first.call.args[2].body); assert.equal(second.call.args[2].body.revision, 3);
  await second.fail(new Error('Synthetic still offline'));
}, { params: { edit: 'synthetic-post' } }));


test('same-batch edit then immediate ACK retires the sent snapshot before remount', async () => withHost(async h => {
  await h.changeBody('Synthetic previous uncertain'); await uncertain(h);
  const edit = h.body().props.onChangeText, send = h.send().props.onPress;
  await act(async () => { edit('Synthetic new submitted'); send(); });
  assert.equal(h.callsFor('createPost').at(-1).args[1].body, 'Synthetic new submitted');
  assert.equal(h.routes.length, 1);
  assert.equal(h.load(vault).docNhapBai('synthetic-A', 'new:community'), null, 'accepted current snapshot must retire its matching journal');
  await h.unmount(); await h.remount(); assert.equal(h.body().props.value, '');
}));


test('body edit followed by route unmount in the same batch publishes the new draft before passive effects', async () => withHost(async h => {
  await h.changeBody('Synthetic prior saved draft'); const first = await uncertain(h);
  const edit = h.body().props.onChangeText;
  await act(async () => { edit('Synthetic last edit before back'); h.hideNow(); });
  const saved = h.load(vault).docNhapBai('synthetic-A', 'new:community');
  assert.equal(saved.body, 'Synthetic last edit before back'); assert.notEqual(saved.logical_id, first.logical_id);
  await h.remount(); assert.equal(h.body().props.value, saved.body);
  assert.equal((await retry(h)).logical_id, saved.logical_id);
}));

test('selected file metadata followed by route unmount in the same batch remains in the actor draft', async () => withHost(async h => {
  await h.changeBody('Synthetic selected-file back'); const first = await uncertain(h);
  const file = { asset: { uri: 'file:///synthetic-before-back.jpg', width: 64, height: 32 }, uploaded: undefined };
  const selected = h.media().props.onFileChange;
  await act(async () => { selected(file); h.hideNow(); });
  const saved = h.load(vault).docNhapBai('synthetic-A', 'new:community');
  assert.deepEqual(saved.pendingFile, file); assert.equal(saved.logical_id, first.logical_id);
  await h.remount(); assert.deepEqual(h.media().props.initialFile, file);
}));

test('old ACK cannot clear a remounted newer logical snapshot edited and closed before passive save', async () => withHost(async h => {
  await h.changeBody('Synthetic old in-flight body'); const old = h.deferNext('createPost'); await h.pressSend();
  await h.unmount(); await h.remount(); const edit = h.body().props.onChangeText;
  await act(async () => { edit('Synthetic newer body before back'); old.resolve({ id: 'synthetic-old-post' }); h.hideNow(); });
  const saved = h.load(vault).docNhapBai('synthetic-A', 'new:community');
  assert.ok(saved); assert.equal(saved.body, 'Synthetic newer body before back');
  assert.notEqual(saved.logical_id, old.call.args[1].logical_id); assert.equal(h.routes.length, 0);
  await h.remount(); assert.equal(h.body().props.value, saved.body);
}));

test('old ACK with the same payload ID cannot drop a newer pending-file draft after remount', async () => withHost(async h => {
  await h.changeBody('Synthetic same payload newer attachment'); const old = h.deferNext('createPost'); await h.pressSend();
  await h.unmount(); await h.remount();
  const file = { asset: { uri: 'file:///synthetic-newer-pending.jpg', width: 64, height: 32 }, uploaded: undefined };
  const selected = h.media().props.onFileChange;
  await act(async () => { selected(file); old.resolve({ id: 'synthetic-old-post' }); h.hideNow(); });
  const saved = h.load(vault).docNhapBai('synthetic-A', 'new:community');
  assert.ok(saved, 'same outgoing ID is insufficient to identify a newer selected-file journal');
  assert.deepEqual(saved.pendingFile, file); assert.equal(saved.logical_id, old.call.args[1].logical_id);
  assert.equal(h.routes.length, 0); await h.remount(); assert.deepEqual(h.media().props.initialFile, file);
}));


test('a pristine edit attempt saved before transport retries the same snapshot after response loss and remount', async () => withHost(async h => {
  const first = h.deferNext('translatedAsActor'); await h.pressSend(); await first.fail(new Error('Synthetic pristine edit response lost'));
  const input = first.call.args[2].body;
  const saved = h.load(vault).docNhapBai('synthetic-A', 'edit:synthetic-post');
  assert.ok(saved, 'an attempted pristine edit is now an uncertain draft'); assert.equal(saved.logical_id, input.logical_id);
  await h.unmount(); await h.remount(); const second = h.deferNext('translatedAsActor'); await h.pressSend();
  assert.deepEqual(second.call.args[2].body, input); await second.fail(new Error('Synthetic still offline'));
}, { params: { edit: 'synthetic-post' } }));


test('retained handlers cannot publish after unmount commit before passive cleanup', async () => withHost(async h => {
  await h.changeBody('Synthetic saved before retirement'); const first = await uncertain(h);
  const edit = h.body().props.onChangeText, selected = h.media().props.onFileChange;
  h.afterNextCommit(() => { edit('Synthetic retired handler body'); selected({ asset: { uri: 'file:///synthetic-retired-file.jpg', width: 32, height: 32 } }); });
  await act(async () => { h.hideNow(); });
  const saved = h.load(vault).docNhapBai('synthetic-A', 'new:community');
  assert.equal(saved.body, first.body, 'layout retirement must precede callbacks from an unmounted editor');
  assert.equal(saved.pendingFile, null); assert.equal(saved.logical_id, first.logical_id);
  await h.remount(); assert.equal(h.body().props.value, first.body);
}));
