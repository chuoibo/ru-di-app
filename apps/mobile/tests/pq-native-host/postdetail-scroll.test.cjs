// Execute actual PostDetail scroll callbacks; synthetic geometry never proves Android restoration.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, act, sourceRoot } = require('./owner-host.cjs');
const previousAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const previousError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) previousError(...args); };
after(() => { console.error = previousError; if (previousAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = previousAct; });
const plain = value => JSON.parse(JSON.stringify(value));
const page = () => ({ comments: [{ id: 'synthetic-restored-row', author_id: 'synthetic-B', author: 'Synthetic restored author', body: 'Synthetic restored thread', parent_id: null, media_id: null, mentions: [], created_at: '2026-10-01T00:00:00Z', status: 'approved', can_delete: false }], pending: [], next_cursor: null });
async function withHost(run) {
  const h = createHost({ surface: 'detail' });
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
function body(h) { return h.one('RNScrollView', props => props.testID === 'community-post-body'); }
function capture(h) {
  const props = body(h).props;
  for (const name of ['onScroll', 'onLayout', 'onContentSizeChange', 'onScrollBeginDrag']) assert.equal(typeof props[name], 'function', `actual body callback ${name}`);
  return { scroll: props.onScroll, layout: props.onLayout, content: props.onContentSizeChange, drag: props.onScrollBeginDrag };
}
function event(y, content, viewport) {
  return { nativeEvent: { contentOffset: { x: 0, y }, ...(content === undefined ? {} : { contentSize: { width: 390, height: content } }), ...(viewport === undefined ? {} : { layoutMeasurement: { width: 390, height: viewport } }) } };
}
const layoutEvent = height => ({ nativeEvent: { layout: { x: 0, y: 0, width: 390, height } } });
async function scroll(h, y, content, viewport) { await act(async () => capture(h).scroll(event(y, content, viewport))); }
async function layout(h, height) { await act(async () => capture(h).layout(layoutEvent(height))); }
async function content(h, height) { await act(async () => capture(h).content(390, height)); }
async function journal(h, y) {
  // The source journal is mutable. Read its prop after an explicit same-route rerender.
  await h.update();
  assert.deepEqual(plain(body(h).props.contentOffset), { x: 0, y }, 'actual current-context scroll journal');
}
function commands(h) { return h.nativeHostCalls.filter(call => call.kind === 'scrollTo'); }
function ownership(h) {
  const observed = h.observation();
  assert.equal(observed.live, 1); assert.equal(observed.subscriptions, 1); assert.equal(observed.removals, 0); assert.equal(observed.maxLive, 1);
}
function actual(h) {
  for (const relative of ['apps/mobile/src/rudi/community/PostDetail', 'apps/mobile/src/rudi/community/Comments', 'apps/mobile/src/rudi/ui/Sheet', 'apps/mobile/src/rudi/ui', 'apps/mobile/src/rudi/community/api']) {
    const file = h.consumed.get(relative); assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true);
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
async function pendingInline(h, target) {
  await scroll(h, target); await journal(h, target);
  await h.openComments(); await h.closeComments();
  const queued = h.deferCommentsRead();
  await h.completeClose();
  assert.equal(queued.started, true, 'returned inline really starts the parked actual GET');
  assert.equal(h.reads.at(-1).resolve === queued.resolve, true, 'parked transport belongs to actual readComments');
  assert.equal(h.text('Synthetic restored thread').length, 0, 'returned inline thread is still loading');
  assert.equal(h.editors().length, 1); ownership(h);
  return queued;
}

test('SP01 phase and writer leases reject sheet clamps and retired inline callbacks through ACK and reopen', async () => {
  await withHost(async h => {
    ownership(h); const firstInline = capture(h);
    await h.openComments();
    const sheet = capture(h);
    await act(async () => { sheet.scroll(event(60)); firstInline.scroll(event(17)); });
    await journal(h, 0); // Zero target isolates ownership from the pending-target guard.
    await h.closeComments(); const closingBody = capture(h), firstCompletion = h.closeAnimation();
    await act(async () => closingBody.scroll(event(20)));
    await journal(h, 0); await h.completeClose(firstCompletion);
    await act(async () => { firstInline.scroll(event(17)); sheet.scroll(event(18)); closingBody.scroll(event(19)); });
    await journal(h, 0);
    await scroll(h, 461); await journal(h, 461);
    await h.change('Synthetic scroll closing ACK'); await h.openComments(); await h.send();
    await h.closeComments(); const closing = h.closeAnimation();
    await h.openComments(); assert.equal(h.commentSheet().props.open, false, 'rapid reopen waits for actual close completion');
    assert.equal(h.editors().length, 1);
    await h.resolveWrite(0); await h.completeClose(closing);
    assert.equal(h.input().props.value, '', 'ACK-cleared body stays clear after inline handoff');
    await journal(h, 461);
    await layout(h, 400); await content(h, 900); await scroll(h, 461, 900, 400);
    await scroll(h, 320, 900, 400); await journal(h, 320);
    await h.openComments(); const newWriter = h.writer();
    await h.completeClose(closing);
    assert.equal(h.commentSheet().props.open, true, 'old completion cannot hide a later sheet');
    assert.equal(h.writer() === newWriter, true); assert.equal(h.writes.length, 1); ownership(h); actual(h);
  });
});

test('SP02 delayed inline content rejects early ACK and clamps then restores only a fitting native target', async () => {
  await withHost(async h => {
    const queued = await pendingInline(h, 461);
    await scroll(h, 461, 500, 400); await scroll(h, 100, 500, 400); await journal(h, 461);
    await layout(h, 400); await content(h, 500);
    assert.equal(commands(h).length, 0, 'short loading content cannot receive target461');
    await h.finish(queued, page()); assert.equal(h.text('Synthetic restored thread').length, 1);
    await content(h, 900);
    assert.equal(commands(h).length, 1); assert.deepEqual(plain(commands(h)[0].options), { x: 0, y: 461, animated: false });
    await content(h, 900); await layout(h, 400);
    assert.equal(commands(h).length, 1, 'same target and geometry do not repeat commands');
    // A native reflow event must supersede the older fitting layout cache.
    await scroll(h, 461, 500, 400); await scroll(h, 100, 500, 400); await journal(h, 461);
    for (const [nativeContent, nativeViewport] of [[NaN, 400], [900, undefined], [undefined, 400]]) {
      await scroll(h, 461, nativeContent, nativeViewport); await scroll(h, 100, 500, 400); await journal(h, 461);
    }
    await scroll(h, 100, 900, 400); await journal(h, 461);
    await scroll(h, 461, 900, 400); await scroll(h, 320, 900, 400); await journal(h, 320);
    assert.equal(commands(h).length, 1); assert.equal(h.writes.length, 0); ownership(h); actual(h);
  });
  await withHost(async h => {
    const queued = await pendingInline(h, 320);
    // Native geometry can confirm initial contentOffset before layout callbacks/noop scrollTo.
    await scroll(h, 320, 800, 400); await scroll(h, 75, 800, 400); await journal(h, 75);
    await h.finish(queued, page()); await layout(h, 400); await content(h, 900);
    assert.equal(commands(h).length, 0, 'a confirmed native position does not wait for a noop command ACK'); ownership(h);
  });
  await withHost(async h => {
    const queued = await pendingInline(h, 320);
    await layout(h, 400); await content(h, 719.5);
    assert.equal(commands(h).length, 1); assert.deepEqual(plain(commands(h)[0].options), { x: 0, y: 319.5, animated: false }, 'fit and ACK use the same one-dp tolerance');
    await scroll(h, 319.5, 719.5, 400); await scroll(h, 250, 719.5, 400); await journal(h, 250);
    await h.finish(queued, page()); ownership(h);
  });
});

test('SP03 invalid drags preserve pending restore and valid user drag escapes failed or permanently short reads', async () => {
  for (const outcome of ['error', 'empty']) {
    await withHost(async h => {
      const queued = await pendingInline(h, 461);
      await layout(h, 400); await content(h, 500);
      for (const invalid of [-1, NaN, Infinity, -Infinity]) {
        await act(async () => capture(h).drag(event(invalid, 500, 400)));
        await scroll(h, 100, 500, 400); await journal(h, 461);
      }
      if (outcome === 'error') {
        await h.fail(queued, new Error('Synthetic delayed comments read failed'));
        assert.equal(h.hosts('RNAnimatedView', props => props.testID === 'cong-dong-loi-binh-luan').length, 1, 'actual read failure stays visible in the thread');
      } else {
        await h.finish(queued, { comments: [], pending: [], next_cursor: null });
        assert.equal(h.text(props => [props.children].flat().some(value => typeof value === 'string' && value.includes('Chưa có lời nào ở đây.'))).length, 1, 'actual returned thread is permanently empty');
      }
      assert.equal(commands(h).length, 0); await journal(h, 461);
      await act(async () => capture(h).drag(event(75, 500, 400)));
      await journal(h, 75); const reads = h.reads.length, gets = h.gets.length;
      await content(h, 900); await layout(h, 400);
      assert.equal(commands(h).length, 0, 'valid user drag cancels pending target before later content growth');
      await scroll(h, 100, 900, 400); await journal(h, 100);
      assert.equal(h.reads.length, reads); assert.equal(h.gets.length, gets); assert.equal(h.writes.length, 0); ownership(h); actual(h);
    });
  }
});

test('SP04 retired context callbacks cannot scroll a new body or mutate its journal after route change or teardown', async () => {
  await withHost(async h => {
    const queued = await pendingInline(h, 461), old = capture(h);
    await h.update(state => { state.params = { id: 'synthetic-post-2' }; });
    assert.equal(h.editors()[0].props.post.id, 'synthetic-post-2'); assert.equal(h.input().props.value, '');
    assert.equal(h.gets.at(-1).route.endsWith('/synthetic-post-2'), true, 'new post is verified by actual transport');
    const beforeCommands = commands(h).length;
    await act(async () => { old.layout(layoutEvent(400)); old.content(390, 900); old.scroll(event(461, 900, 400)); old.drag(event(17, 900, 400)); });
    assert.equal(commands(h).length, beforeCommands, 'retired callbacks cannot command the shared ref of the new native body');
    await journal(h, 0); await scroll(h, 80); await journal(h, 80);
    await h.change('Synthetic new-context draft'); await h.finish(queued, page());
    assert.equal(h.input().props.value, 'Synthetic new-context draft'); await journal(h, 80);
    const current = capture(h); await h.dispose();
    const calls = h.calls.length, native = h.nativeHostCalls.length;
    await act(async () => { current.layout(layoutEvent(400)); current.content(390, 900); current.scroll(event(12, 900, 400)); current.drag(event(13, 900, 400)); old.content(390, 1000); });
    assert.equal(h.calls.length, calls); assert.equal(h.nativeHostCalls.length, native); assert.equal(h.writes.length, 0);
    assert.equal(h.observation().live, 0); assert.equal(h.observation().subscriptions, 1); assert.equal(h.observation().removals, 1); actual(h);
  });
});
