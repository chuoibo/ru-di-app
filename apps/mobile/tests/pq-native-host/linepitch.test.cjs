// Verify native input metrics, geometry and owner lifecycles through actual app source.
// Host checks props, React ownership and ref lifecycles, never Android rendering.
const assert = require('node:assert/strict');
const path = require('node:path');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, sourceRoot, React } = require('./exact-host.cjs');
const previousAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const previousError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) previousError(...args); };
after(() => { console.error = previousError; if (previousAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = previousAct; });
async function withHost(options, run) {
  const h = createHost(options);
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
function inputStyle(h) { return flatten(h.input().props.style); }
function assertActual(h, names) {
  for (const name of names) {
    const relative = `apps/mobile/src/rudi/${name}`;
    const used = h.consumed.get(relative);
    assert.ok(used, `Actual source consumed: ${relative}`);
    const expected = path.join(sourceRoot, `${relative}${name === 'native-font-metrics' ? '.ts' : '.tsx'}`);
    assert.equal(used, expected, `Exact candidate/base source: ${relative}`);
  }
}
function sameHost(h, input, body) {
  assert.equal(h.input() === input, true, 'native host identity is retained');
  assert.equal(h.input().props.value, body, 'raw controlled body is retained');
}

test('LP01 default Field remains native; opt-in converts actual input and multiline geometry', async () => {
  await withHost({}, async h => {
    assert.equal(inputStyle(h).fontSize, 17, 'actual shared body typography');
    assert.equal(inputStyle(h).lineHeight, 24);
    assert.equal(inputStyle(h).minHeight, 72);
    assert.equal(inputStyle(h).maxHeight, 192);
    const input = h.input(); await h.nativeFont(snapshot(2));
    sameHost(h, input, 'Synthetic draft');
    assert.equal(inputStyle(h).fontSize, 17, 'nativeMetrics defaults false');
    assert.equal(inputStyle(h).lineHeight, 24);
    assert.equal(inputStyle(h).minHeight, 72); assert.equal(inputStyle(h).maxHeight, 192);
    assert.equal(h.input().props.allowFontScaling, undefined);
    assert.equal(Object.hasOwn(h.input().props, 'nativeMetrics'), false, 'opt-in is not a native prop');
  });
  await withHost({ fieldProps: { nativeMetrics: true } }, async h => {
    const input = h.input(); const ref = h.refs[0];
    await h.nativeFont(snapshot(2));
    sameHost(h, input, 'Synthetic draft');
    assert.equal(h.input().props.allowFontScaling, false, 'converted DP is not scaled twice');
    assert.equal(inputStyle(h).fontSize, 28.75); assert.equal(inputStyle(h).lineHeight, 39.25);
    assert.equal(inputStyle(h).minHeight, 118.5, '3 × ceil(39.25 × 2) / 2');
    assert.equal(inputStyle(h).maxHeight, 316, '8 × the same 39.5dp line pitch');
    assert.equal(h.refs.length, 1); assert.equal(h.refs[0] === ref, true, 'forwarded native ref never detached');
    assertActual(h, ['ui/Field', 'ui/NativeFontMetrics', 'native-font-metrics']);
  });
});

test('LP02 compact actual Field hot 100→200→100 retains focus callback, ref, host and body', async () => {
  let focusCount = 0, blurCount = 0;
  await withHost({ fieldProps: { nativeMetrics: true, numberOfLines: 1, style: { minHeight: 24, maxHeight: 72 }, onFocus: () => { focusCount += 1; }, onBlur: () => { blurCount += 1; } } }, async h => {
    await h.focus(); await h.change('Synthetic focused body');
    const input = h.input(), ref = h.refs[0];
    for (const [value, size, height, min, max] of [
      [snapshot(2), 28.75, 39.25, 39.5, 118.5],
      [{ status: 'unavailable', reason: 'module-unavailable' }, 17, 24, 24, 72],
      [snapshot(1), 17, 24, 24, 72],
      [{ status: 'available', fontScale: 2 }, 17, 24, 24, 72],
      [snapshot(2), 28.75, 39.25, 39.5, 118.5],
      [snapshot(1), 17, 24, 24, 72],
    ]) {
      await h.nativeFont(value); sameHost(h, input, 'Synthetic focused body');
      assert.equal(inputStyle(h).fontSize, size); assert.equal(inputStyle(h).lineHeight, height);
      assert.equal(inputStyle(h).minHeight, min); assert.equal(inputStyle(h).maxHeight, max);
      assert.equal(focusCount, 1, 'metrics do not replay focus'); assert.equal(blurCount, 0, 'metrics do not emit blur');
      assert.equal(h.refs.length, 1, 'no callback ref detach or reattach'); assert.equal(h.refs[0] === ref, true);
    }
  });
});

test('LP03 unavailable, absent table, unknown style and capped input retain native fallback', async () => {
  const cases = [
    { native: { status: 'unavailable', reason: 'module-unavailable' }, props: {}, size: 17, line: 24 },
    { native: { status: 'available', fontScale: 2 }, props: {}, size: 17, line: 24 },
    { native: snapshot(2), props: { style: { minHeight: 24, maxHeight: 72, fontSize: 16 } }, size: 16, line: 24 },
    { native: snapshot(2), props: { style: { minHeight: 24, maxHeight: 72, lineHeight: undefined } }, size: 17, line: undefined },
    { native: snapshot(2), props: { maxFontSizeMultiplier: 1.25 }, size: 17, line: 24 },
    { native: snapshot(2), props: { maxFontSizeMultiplier: 1 }, size: 17, line: 24 },
    { native: snapshot(2), props: { maxFontSizeMultiplier: 0.5 }, size: 17, line: 24 },
    { native: snapshot(2), props: { style: { minHeight: 24, maxHeight: 72, lineHeight: 0 } }, size: 17, line: 0 },
  ];
  for (const entry of cases) {
    await withHost({ initialSnapshot: entry.native, fieldProps: { nativeMetrics: true, style: { minHeight: 24, maxHeight: 72 }, ...entry.props } }, async h => {
      assert.equal(inputStyle(h).fontSize, entry.size); assert.equal(inputStyle(h).lineHeight, entry.line);
      assert.equal(inputStyle(h).minHeight, 24); assert.equal(inputStyle(h).maxHeight, 72);
      assert.equal(h.input().props.allowFontScaling, undefined, 'fallback must leave RN in charge');
      assert.equal(h.input().props.maxFontSizeMultiplier, entry.props.maxFontSizeMultiplier);
    });
  }
});

test('LP04 scaling opt-out and explicit uncapped zero preserve their different intended paths', async () => {
  for (const [props, size, line, min, max] of [
    [{ allowFontScaling: false }, 17, 24, 24, 72],
    [{ maxFontSizeMultiplier: 0 }, 28.75, 39.25, 39.5, 118.5],
  ]) {
    await withHost({ initialSnapshot: snapshot(2), fieldProps: { nativeMetrics: true, style: { minHeight: 24, maxHeight: 72 }, ...props } }, async h => {
      assert.equal(inputStyle(h).fontSize, size); assert.equal(inputStyle(h).lineHeight, line);
      assert.equal(inputStyle(h).minHeight, min); assert.equal(inputStyle(h).maxHeight, max);
      assert.equal(h.input().props.allowFontScaling, false); assert.equal(h.input().props.maxFontSizeMultiplier, props.maxFontSizeMultiplier);
    });
  }
});

test('LP05 actual Comments observer/provider covers presentation footer without replacing context owner', async () => {
  const presentation = (thread, editor) => React.createElement('SyntheticSurface', null, React.createElement('SyntheticReference', null, thread), React.createElement('SyntheticFooter', null, editor));
  await withHost({ surface: 'comments', commentsProps: { presentation } }, async h => {
    await h.change('Synthetic comment draft');
    const input = h.input(), context = h.owner('CommentsForContext'), saved = h.drafts.at(-1), readsBefore = h.calls.filter(call => call.route?.includes('/comments')).length;
    assert.equal(h.observation().live, 1, 'one observer for thread and footer');
    assert.equal(h.root.root.findByType('SyntheticFooter').findAllByType('RNTextInput').length, 1, 'presentation footer contains the actual native input');
    for (const [value, min, max] of [[snapshot(2), 39.5, 118.5], [snapshot(1), 24, 72]]) {
      await h.nativeFont(value); sameHost(h, input, saved.body);
      assert.equal(h.owner('CommentsForContext') === context, true, 'font metrics are not part of context key');
      assert.equal(inputStyle(h).minHeight, min); assert.equal(inputStyle(h).maxHeight, max);
      assert.equal(h.input().props.allowFontScaling, false);
      assert.equal(h.drafts.at(-1).logical_id, saved.logical_id);
      assert.deepEqual(h.drafts.at(-1), saved, 'draft ownership and metadata remain exact');
      assert.equal(h.calls.filter(call => call.route?.includes('/comments')).length, readsBefore, 'font event does not reload the context');
      assert.deepEqual(h.observation(), { live: 1, subscriptions: 1, removals: 0 });
    }
    assertActual(h, ['community/Comments', 'ui/Field', 'ui/NativeFontMetrics', 'native-font-metrics']);
    // Confirm the keyed context boundary still exists independently of metrics.
    await h.update(() => { h.state.post = { id: 'synthetic-other-post', can_comment: true }; });
    assert.equal(h.owner('CommentsForContext') === context, false, 'real post context change retires prior owner');
    assert.equal(h.input().props.value, ''); assert.equal(h.drafts.at(-1).post_id, 'synthetic-other-post');
    assert.equal(h.observation().live, 1, 'surface observer remains above the keyed owner');
  });
});

test('LP06 actual Composer converts supported 24sp body and retains session owner, lease and vault on hot metrics', async () => {
  await withHost({ surface: 'composer' }, async h => {
    await h.change('Synthetic Composer draft');
    const input = h.input(), owner = h.owner('ComposerForm'), vault = h.load('apps/mobile/src/rudi/community/ban-nhap');
    const saved = vault.docNhapBai('synthetic-A', 'new:community'), lease = vault.phienNhapBai('synthetic-A');
    const observed = h.observation(); assert.equal(observed.live, 1, 'one current form observer after actual session restoration');
    for (const [value, size, line] of [[snapshot(2), 28.75, 39.25], [snapshot(1), 17, 24]]) {
      await h.nativeFont(value); sameHost(h, input, saved.body);
      assert.equal(h.owner('ComposerForm') === owner, true, 'metrics do not replace the keyed form owner');
      assert.equal(inputStyle(h).fontSize, size); assert.equal(inputStyle(h).lineHeight, line);
      assert.equal(h.input().props.allowFontScaling, false, 'supported table reaches actual native input');
      assert.equal(vault.phienNhapBai('synthetic-A'), lease); assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), saved);
      assert.deepEqual(h.observation(), observed, 'metric changes do not resubscribe or retire the form observer');
    }
    assertActual(h, ['community/Composer', 'ui/NativeFontMetrics', 'native-font-metrics']);
    await h.actorTo('synthetic-C');
    assert.equal(h.owner('ComposerForm') === owner, false, 'actual session change still replaces form owner');
    assert.equal(h.input().props.value, ''); assert.equal(vault.docNhapBai('synthetic-A', 'new:community'), null);
  });
});

test('LP07 actual native parser drops malformed tables and recovers without replacing P/Q drafts', async () => {
  for (const surface of ['comments', 'composer']) {
    await withHost({ surface }, async h => {
      await h.change(`Synthetic ${surface} fallback`); const input = h.input();
      const vault = surface === 'composer' ? h.load('apps/mobile/src/rudi/community/ban-nhap') : null;
      const currentDraft = () => vault ? vault.docNhapBai('synthetic-A', 'new:community') : h.drafts.at(-1);
      const saved = currentDraft();
      const missingRow = snapshot(2); missingRow.fontMetrics.spToDp.pop();
      const badDensity = snapshot(2); badDensity.fontMetrics.density = 0;
      const badSample = snapshot(2); badSample.fontMetrics.spToDp[0].dp = NaN;
      for (const malformed of [missingRow, badDensity, badSample, { status: 'available', fontScale: NaN }]) {
        await h.nativeFont(malformed);
        sameHost(h, input, `Synthetic ${surface} fallback`);
        assert.equal(inputStyle(h).fontSize, 17); assert.equal(inputStyle(h).lineHeight, 24); assert.equal(h.input().props.allowFontScaling, undefined);
        assert.deepEqual(currentDraft(), saved, 'invalid native data cannot change draft or logical id');
      }
      await h.nativeFont({ status: 'unavailable', reason: 'observer-unavailable' }); sameHost(h, input, `Synthetic ${surface} fallback`);
      assert.equal(inputStyle(h).fontSize, 17); assert.equal(inputStyle(h).lineHeight, 24); assert.equal(h.input().props.allowFontScaling, undefined);
      await h.nativeFont(snapshot(2)); sameHost(h, input, `Synthetic ${surface} fallback`);
      assert.equal(inputStyle(h).fontSize, 28.75); assert.equal(inputStyle(h).lineHeight, 39.25); assert.equal(h.input().props.allowFontScaling, false);
    });
  }
});

test('LP08 same-scale metrics are observed and Composer always reserves a complete line plus padding', async () => {
  await withHost({ surface: 'composer' }, async h => {
    await h.change('Synthetic tall line'); const input = h.input();
    await h.nativeFont(snapshot(2, 201.25));
    sameHost(h, input, 'Synthetic tall line');
    assert.equal(inputStyle(h).lineHeight, 201.25); assert.equal(inputStyle(h).minHeight, 209.5, '201.5dp pixel pitch + 8dp vertical padding');
    assert.equal(inputStyle(h).maxHeight, 420, 'responsive physical ceiling remains above the minimum');
    await h.nativeFont(snapshot(2, 500.25));
    sameHost(h, input, 'Synthetic tall line');
    assert.equal(inputStyle(h).lineHeight, 500.25); assert.equal(inputStyle(h).minHeight, 508.5);
    assert.equal(inputStyle(h).maxHeight, 508.5, 'physical ceiling may not contradict one complete line');
  });
});

test('LP10 actual Comments metadata stays beside avatar and own text converts without replacing hosts', async () => {
  await withHost({ surface: 'comments' }, async h => {
    const nearestView = node => { for (let parent = node.parent; parent; parent = parent.parent) if (parent.type === 'RNView') return parent; return null; };
    const author = () => h.one('RNText', props => props.children === 'Synthetic author');
    const column = nearestView(author()); assert.ok(column, 'author has a real native metadata column');
    const main = nearestView(column), row = nearestView(main); assert.ok(row, 'metadata sits in the main column beside avatar');
    const authorHost = author(), timeHost = column.findAllByType('RNText')[1];
    const bodyHost = textHost(h, 'Synthetic server comment'), headerHost = textHost(h, 'Bình luận');
    assert.equal(h.hosts('RNText', props => props.testID === 'community-comment-send-reason').length, 0, 'idle placeholder and disabled send need no duplicated visual helper');
    for (const value of [snapshot(1), snapshot(2), snapshot(1)]) {
      await h.nativeFont(value);
      const large = value.fontScale === 2;
      assert.equal(author() === authorHost, true, 'metadata is not replaced by font changes');
      const currentColumn = nearestView(author()); assert.equal(currentColumn === column, true);
      const columnStyle = flatten(currentColumn.props.style);
      assert.equal(flatten(main.props.style).flex, 1); assert.equal(flatten(main.props.style).minWidth, 0);
      assert.equal(columnStyle.gap, large ? 2 : 8);
      assert.equal(columnStyle.flexDirection, large ? 'column' : 'row');
      assert.equal(columnStyle.alignItems, large ? 'flex-start' : 'baseline');
      const labels = currentColumn.findAllByType('RNText');
      assert.equal(labels.length, 2, 'metadata contains exactly author then time');
      assert.equal(labels[0] === authorHost, true); assert.equal(typeof labels[1].props.children, 'string');
      assert.equal(labels[1] === timeHost, true, 'timestamp host stays mounted');
      assert.equal(labels[1].props.children.length > 0, true, 'actual relativeTime produces a visible timestamp');
      convertedText(h, 'Synthetic author', large ? 23.25 : 14, large ? 31.75 : 19);
      convertedText(h, labels[1].props.children, large ? 21.5 : 13, large ? 30.25 : 18);
      assert.equal(convertedText(h, 'Bình luận', large ? 34.25 : 21, large ? 43 : 27) === headerHost, true, 'heading host stays mounted');
      assert.equal(h.hosts('RNText', props => props.testID === 'community-comment-send-reason').length, 0, 'hot metrics do not add an idle helper');
      assert.equal(nearestView(labels[1]) === currentColumn, true, 'timestamp shares metadata column with author');
      assert.equal(nearestView(currentColumn) === main, true); assert.equal(nearestView(main) === row, true);
      assert.equal(flatten(row.props.style).flexDirection, 'row'); assert.equal(flatten(row.props.style).alignItems, 'flex-start');
      const avatar = row.findAllByType('RNView').filter(node => node.props.accessibilityLabel === 'Synthetic author');
      assert.equal(avatar.length, 1, 'one actual avatar is beside metadata');
      assert.equal(nearestView(avatar[0]) === row, true, 'avatar remains a separate sibling of metadata');
      const body = h.one('RNText', props => props.children === 'Synthetic server comment');
      assert.equal(convertedText(h, 'Synthetic server comment', large ? 28.75 : 17, large ? 39.25 : 24) === bodyHost, true, 'body host stays mounted');
      assert.equal(nearestView(body) === column, false, 'comment body remains outside metadata');
      const reply = h.hosts('RNPressable').filter(node => node.findAllByType('RNText').some(text => text.props.children === 'Trả lời'));
      assert.equal(reply.length, 1, 'reply action scope is retained');
      assert.equal(nearestView(reply[0]) === column, false, 'reply stays outside metadata');
    }
  });
});

function textHost(h, label) {
  const text = value => Array.isArray(value) ? value.flat(Infinity).map(child => typeof child === 'string' || typeof child === 'number' ? String(child) : '').join('') : typeof value === 'string' || typeof value === 'number' ? String(value) : '';
  return h.one('RNText', props => text(props.children) === label);
}
function convertedText(h, label, size, lineHeight) {
  const host = textHost(h, label), style = flatten(host.props.style);
  assert.equal(host.props.allowFontScaling, false, `DP scaling ownership: ${label}`);
  assert.equal(style.fontSize, size, `converted font size: ${label}`);
  assert.equal(style.lineHeight, lineHeight, `converted line height: ${label}`);
  return host;
}
async function seedPendingComposer(h, pendingFile) {
  const vault = h.load('apps/mobile/src/rudi/community/ban-nhap');
  const lease = vault.phienNhapBai('synthetic-A');
  vault.ghiNhapBai('synthetic-A', 'new:community', { body: 'Synthetic pending media draft', audience: 'public', topics: '', mentions: [], media: [], group: null, revision: 0, logical_id: 'synthetic-media-logical', pendingFile }, lease);
  await h.remountSurface();
  return { vault, lease, draft: () => vault.docNhapBai('synthetic-A', 'new:community') };
}

test('LP12 actual Composer and MediaPicker text convert through native pick failure without losing pending draft', async () => {
  await withHost({ surface: 'composer' }, async h => {
    const pending = { asset: { uri: 'file:///synthetic-pending.jpg', type: 'image', mimeType: 'image/jpeg', width: 32, height: 32 } };
    const journal = await seedPendingComposer(h, pending);
    const saved = journal.draft(), input = h.input(), owner = h.owner('ComposerForm');
    const assertComposerCopy = scale => {
      const large = scale === 2;
      convertedText(h, 'Synthetic synthetic-A', large ? 28.75 : 17, large ? 37.5 : 23);
      convertedText(h, 'Cộng đồng', large ? 21.5 : 13, large ? 30.25 : 18);
      convertedText(h, `${saved.body.length}/5000`, large ? 21.5 : 13, large ? 30.25 : 18);
      convertedText(h, 'Chủ đề', large ? 21.5 : 13, large ? 30.25 : 18);
      convertedText(h, 'Bài được duyệt trước khi xuất hiện trên cộng đồng.', large ? 21.5 : 13, large ? 30.25 : 18);
      convertedText(h, 'Gửi lên cộng đồng', large ? 23.25 : 14, large ? 31.75 : 19);
    };
    assertComposerCopy(1);
    await h.pressAccessibility('Thêm ảnh hoặc video'); assert.equal(h.picks.length, 1, 'actual MediaPicker requested native pick once');
    const helper = 'Lời viết vẫn ở đây trong lúc tệp được chuẩn bị.';
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale));
      convertedText(h, helper, scale === 2 ? 21.5 : 13, scale === 2 ? 30.25 : 18);
      assertComposerCopy(scale); sameHost(h, input, saved.body); assert.equal(h.owner('ComposerForm') === owner, true);
      assert.deepEqual(journal.draft(), saved, 'pending-file journal and logical id survive busy font changes');
    }
    await h.rejectPick(new Error('Synthetic private native picker diagnostic'));
    const error = 'Chưa chuẩn bị được tệp. Thử lại hoặc chọn tệp khác nhé.';
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale));
      const alert = convertedText(h, error, scale === 2 ? 21.5 : 13, scale === 2 ? 30.25 : 18);
      assert.equal(alert.props.accessibilityRole, 'alert', 'actual sanitized recovery error retains alert semantics');
      convertedText(h, 'Thử lại', scale === 2 ? 23.25 : 14, scale === 2 ? 31.75 : 19);
      convertedText(h, 'Bỏ chọn', scale === 2 ? 23.25 : 14, scale === 2 ? 31.75 : 19);
      assertComposerCopy(scale); sameHost(h, input, saved.body); assert.equal(h.owner('ComposerForm') === owner, true);
      assert.deepEqual(journal.draft(), saved); assert.equal(journal.vault.phienNhapBai('synthetic-A'), journal.lease);
    }
    assertActual(h, ['community/Composer', 'community/MediaPicker', 'ui/NativeFontMetrics']);
  });
});

test('LP13 actual processing recovery helper and stop message convert under the retained Composer provider', async () => {
  await withHost({ surface: 'composer' }, async h => {
    const pending = { asset: { uri: 'file:///synthetic-processing.mp4', type: 'video', mimeType: 'video/mp4', width: 32, height: 32, duration: 1000 }, uploaded: { id: 'synthetic-processing', type: 'video/mp4', url: '/synthetic-processing', width: 32, height: 32, duration_ms: 1000, state: 'processing' } };
    const journal = await seedPendingComposer(h, pending);
    const saved = journal.draft(), input = h.input(), owner = h.owner('ComposerForm');
    await h.pressAccessibility('Thử lại tệp đã chọn');
    assert.equal(h.picks.length, 0, 'processing retry reuses the admitted file without another native pick');
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale));
      convertedText(h, 'Lời viết vẫn ở đây trong lúc tệp được chuẩn bị.', scale === 2 ? 21.5 : 13, scale === 2 ? 30.25 : 18);
      convertedText(h, 'Dừng chờ tệp', scale === 2 ? 23.25 : 14, scale === 2 ? 31.75 : 19);
      sameHost(h, input, saved.body); assert.equal(h.owner('ComposerForm') === owner, true); assert.deepEqual(journal.draft(), saved);
    }
    await h.pressText('Dừng chờ tệp');
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale));
      convertedText(h, 'Đã dừng chờ. Thử lại để kiểm tra tệp đang xử lý.', scale === 2 ? 21.5 : 13, scale === 2 ? 30.25 : 18);
      convertedText(h, 'Thử lại', scale === 2 ? 23.25 : 14, scale === 2 ? 31.75 : 19);
      sameHost(h, input, saved.body); assert.equal(h.owner('ComposerForm') === owner, true); assert.deepEqual(journal.draft(), saved);
    }
    // The controlled native timer is never advanced; actual polling performs no I/O.
    assert.equal(h.calls.some(call => call.route?.includes('/media/')), false);
  });
});

test('LP14 actual MediaPicker outside any metrics provider retains its original native text path', async () => {
  const video = { id: 'synthetic-ready-video', type: 'video/mp4', url: '/synthetic-ready', width: 32, height: 32, duration_ms: 1000 };
  await withHost({ surface: 'picker', provideMetrics: false, pickerProps: { media: [video] } }, async h => {
    const label = textHost(h, 'Video đã sẵn sàng');
    for (const scale of [2, 1]) {
      await h.nativeFont(snapshot(scale));
      const current = textHost(h, 'Video đã sẵn sàng');
      assert.equal(current === label, true, 'fallback label host stays mounted');
      assert.equal(current.props.allowFontScaling, undefined, 'outside-provider callers keep RN scaling');
      assert.equal(flatten(current.props.style).fontSize, 14); assert.equal(flatten(current.props.style).lineHeight, 19);
      assert.equal(h.observation().live, 0, 'MediaPicker adds no independent observer');
    }
    assertActual(h, ['community/MediaPicker', 'ui/NativeFontMetrics']);
  });
});
