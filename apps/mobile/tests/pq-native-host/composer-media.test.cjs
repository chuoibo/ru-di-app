// Actual Composer, MediaPicker, Session and RAM vault execute together. Adapters
// cover native picker/manipulator, transport promises and visual host elements.
const test = require('node:test');
const assert = require('node:assert/strict');
const { createHost, person, act } = require('./host.cjs');
const vaultPath = 'apps/mobile/src/rudi/community/ban-nhap';
const file = () => ({ asset: { uri: 'file:///synthetic-pending.jpg', width: 32, height: 32, type: 'image', mimeType: 'image/jpeg' } });
const media = () => [{ id: 'synthetic-existing', type: 'image/jpeg', url: '/synthetic-existing' }];
const upload = () => ({ id: 'synthetic-ready', type: 'image/jpeg', url: '/synthetic-ready', state: 'ready' });
const saved = extra => ({ body: 'Synthetic saved body', audience: 'public', topics: '', mentions: [], media: [], group: null, revision: 0, logical_id: 'synthetic-saved-id', pendingFile: null, ...extra });
async function withHost(run, initial) {
  const h = createHost({ realMedia: true }); await h.mount();
  try {
    if (initial) { const v = h.load(vaultPath); v.ghiNhapBai('synthetic-A', 'new:community', saved(initial), v.phienNhapBai('synthetic-A')); await h.unmount(); await h.remount(); }
    await run(h);
  } finally { await h.dispose(); }
}
const draft = h => h.load(vaultPath).docNhapBai('synthetic-A', 'new:community');

test('PM01 real Composer picker-first blocks a retained send callback in the same React batch', async () => {
  await withHost(async h => {
    await h.changeBody('Synthetic picker-first body'); const before = draft(h);
    const pick = h.tool('Thêm ảnh hoặc video').props.onPress, send = h.send().props.onPress;
    const native = h.deferNext('launchImageLibraryAsync');
    await act(async () => { pick(); send(); });
    assert.equal(h.callsFor('launchImageLibraryAsync').length, 1);
    assert.equal(h.callsFor('createPost').length, 0, 'immediate upload latch blocks stale send before rerender');
    assert.equal(h.send().props.disabled, true); assert.match(h.send().props.lyDo, /tệp/);
    await native.finish({ canceled: true, assets: [] });
    assert.equal(h.send().props.disabled, false); assert.equal(draft(h).logical_id, before.logical_id);
  });
});

test('PM02 actual attachment controls explain their disabled state throughout send and recover after failure', async () => {
  await withHost(async h => {
    await h.press('Bỏ tệp đang chờ'); // Explicitly discard the pending choice before submitting ready media.
    const request = h.deferNext('createPost'); await h.pressSend(); const before = draft(h);
    assert.equal(h.button('Bỏ tệp').props.disabled, true);
    assert.equal(h.button('Bỏ tệp').props.lyDo, 'Đợi gửi xong rồi sửa tệp.');
    assert.equal(h.tool('Thêm ảnh hoặc video').props.disabled, true);
    await request.fail(new Error('Synthetic response lost'));
    assert.equal(h.button('Bỏ tệp').props.disabled, false);
    assert.deepEqual(draft(h), before); await h.press('Bỏ tệp');
    assert.deepEqual(draft(h).media, []); assert.notEqual(draft(h).logical_id, before.logical_id);
  }, { media: media(), pendingFile: file() });
});

test('PM03 retained actual child remove/drop/retry handlers cannot alter a sending Composer', async () => {
  await withHost(async h => {
    const callbacks = ['Bỏ tệp', 'Bỏ tệp đang chờ', 'Thử lại tệp đã chọn'].map(label => h.button(label).props.onPress);
    await h.press('Bỏ tệp đang chờ');
    const request = h.deferNext('createPost'); await h.pressSend(); const before = draft(h);
    await act(async () => callbacks.forEach(fn => fn()));
    assert.equal(h.callsFor('uploadMedia').length, 0, 'retained retry cannot upload while sending');
    assert.deepEqual(draft(h), before);
    assert.throws(() => h.button('Bỏ tệp đang chờ'), /count/, 'retired pending choice is not revived');
    await request.fail(new Error('Synthetic response lost'));
  }, { media: media(), pendingFile: file() });
});

test('PM04 send-first blocks the retained real picker toolbar callback before disabled props rerender', async () => {
  await withHost(async h => {
    await h.changeBody('Synthetic send-first body');
    const pick = h.tool('Thêm ảnh hoặc video').props.onPress, send = h.send().props.onPress;
    const request = h.deferNext('createPost');
    await act(async () => { send(); pick(); });
    assert.equal(h.callsFor('createPost').length, 1);
    assert.equal(h.callsFor('launchImageLibraryAsync').length, 0, 'same-turn submission blocks native picker launch');
    await request.fail(new Error('Synthetic response lost')); assert.equal(h.send().props.disabled, false);
  });
});

test('PM05 send-first blocks retired pending retry and drop before child disabled props rerender', async () => {
  await withHost(async h => {
    const retry = h.button('Thử lại tệp đã chọn').props.onPress, drop = h.button('Bỏ tệp đang chờ').props.onPress;
    await h.press('Bỏ tệp đang chờ');
    const send = h.send().props.onPress, before = draft(h), request = h.deferNext('createPost');
    await act(async () => { send(); retry(); drop(); });
    assert.equal(h.callsFor('uploadMedia').length, 0, 'synchronous parent guard blocks retired retry');
    assert.equal(h.callsFor('manipulateAsync').length, 0, 'no preparation starts after send begins');
    assert.deepEqual(draft(h), before);
    await request.fail(new Error('Synthetic response lost'));
  }, { pendingFile: file() });
});

test('PM06 real upload joins the post payload and identical retries retain its attempt identity', async () => {
  await withHost(async h => {
    await h.changeBody('Synthetic uploaded body'); const before = draft(h).logical_id;
    const native = h.deferNext('launchImageLibraryAsync'), transfer = h.deferNext('uploadMedia');
    await act(async () => h.tool('Thêm ảnh hoặc video').props.onPress());
    await native.finish({ canceled: false, assets: [file().asset] });
    assert.equal(h.send().props.disabled, true); assert.equal(h.callsFor('uploadMedia').length, 1);
    await transfer.finish(upload()); assert.equal(h.send().props.disabled, false);
    const ready = draft(h); assert.deepEqual(ready.media.map(m => m.id), ['synthetic-ready']); assert.equal(ready.pendingFile, null); assert.notEqual(ready.logical_id, before);
    const first = h.deferNext('createPost'); await h.pressSend(); assert.deepEqual(first.call.args[1].media_ids, ['synthetic-ready']);
    await first.fail(new Error('Synthetic response lost'));
    const second = h.deferNext('createPost'); await h.pressSend(); assert.equal(second.call.args[1].logical_id, first.call.args[1].logical_id); assert.deepEqual(second.call.args[1].media_ids, ['synthetic-ready']);
    await second.fail(new Error('Synthetic response lost again'));
  });
});

test('PM07 an actual upload completing after actor retirement cannot change the new actor or route', async () => {
  await withHost(async h => {
    await h.changeBody('Synthetic actor A upload');
    const native = h.deferNext('launchImageLibraryAsync'), transfer = h.deferNext('uploadMedia');
    await act(async () => h.tool('Thêm ảnh hoặc video').props.onPress());
    await native.finish({ canceled: false, assets: [file().asset] });
    await h.setActor(person('synthetic-B')); await h.changeBody('Synthetic actor B body');
    const v = h.load(vaultPath), before = v.docNhapBai('synthetic-B', 'new:community');
    await transfer.finish(upload());
    assert.equal(h.body().props.value, 'Synthetic actor B body'); assert.deepEqual(v.docNhapBai('synthetic-B', 'new:community'), before); assert.equal(h.send().props.disabled, false); assert.deepEqual(h.routes, []);
  });
});


test('PR01 failed actual picker upload retains selection, blocks direct send and recovers through retry', async () => {
  await withHost(async h => {
    await h.changeBody('Synthetic failed upload body');
    const native = h.deferNext('launchImageLibraryAsync'), transfer = h.deferNext('uploadMedia');
    await act(async () => h.tool('Thêm ảnh hoặc video').props.onPress());
    await native.finish({ canceled: false, assets: [file().asset] });
    const { LoiTep } = h.load('apps/mobile/src/rudi/community/loi-tep');
    await transfer.fail(new LoiTep('Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.'));
    assert.match(h.text(), /Không kết nối được Rủ Đi/); assert.doesNotMatch(h.text(), /localhost|java\.|53559/);
    assert.equal(h.send().props.disabled, true); assert.equal(h.send().props.lyDo, 'Thử lại hoặc bỏ tệp đang chờ.');
    const text = h.consumed; assert.ok(text.has('apps/mobile/src/rudi/community/MediaPicker'));
    assert.ok(h.button('Thử lại tệp đã chọn')); assert.equal(h.button('Thử lại tệp đã chọn').props.label, 'Thử lại'); assert.equal(h.button('Bỏ tệp đang chờ').props.label, 'Bỏ chọn');
    const before = draft(h); await h.pressSend(); assert.equal(h.callsFor('createPost').length, 0, 'direct send cannot omit the selected failed file');
    await h.unmount(); await h.remount(); assert.deepEqual(draft(h), before); assert.equal(h.send().props.disabled, true);
    const retry = h.deferNext('uploadMedia'); await h.press('Thử lại tệp đã chọn'); await retry.finish(upload());
    assert.equal(draft(h).pendingFile, null); assert.equal(h.send().props.disabled, false);
    const request = h.deferNext('createPost'); await h.pressSend(); assert.deepEqual(request.call.args[1].media_ids, ['synthetic-ready']); await request.fail(new Error('Synthetic response lost'));
  });
});

test('PR02 explicit pending discard is the only path to a text-only post after attachment failure', async () => {
  await withHost(async h => {
    const before = draft(h); assert.equal(h.send().props.disabled, true); await h.pressSend(); assert.equal(h.callsFor('createPost').length, 0);
    await h.press('Bỏ tệp đang chờ'); assert.equal(draft(h).pendingFile, null); assert.equal(h.send().props.disabled, false); assert.equal(draft(h).body, before.body);
    const request = h.deferNext('createPost'); await h.pressSend(); assert.deepEqual(request.call.args[1].media_ids, []); await request.fail(new Error('Synthetic response lost'));
  }, { pendingFile: file() });
});
