// Actual PostDetail/Sheet/Field ownership and I/O; no Android Yoga/caret simulation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, act, sourceRoot } = require('./owner-host.cjs');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const font = scale => snapshot(scale, scale === 2 ? 36 : 24);
const unavailable = () => ({ status: 'unavailable', reason: 'module-inactive' });
const requestLabel = 'Bạn muốn Nếp giúp gì?';
const draftLabel = 'Sửa lại theo ý bạn';
const title = 'Nhờ Nếp giữ một điều';
const disclosure = 'Nếp chỉ nhận đoạn dưới đây và yêu cầu của bạn. Bạn xem lại bản nháp trước khi lưu thành ghi chép riêng.';
const requestAction = 'Đồng ý chia sẻ và gọi Nếp';
const draftAction = 'Lưu ghi chép riêng';
const requestRefusal = 'Nếp chưa trả lời được. Nội dung của bạn vẫn ở đây.';
const plain = value => JSON.parse(JSON.stringify(value));
function inside(node, parent) { for (let at = node.parent; at; at = at.parent) if (at === parent) return true; return false; }
function ancestor(node, predicate, message) { for (let at = node.parent; at; at = at.parent) if (predicate(at)) return at; assert.fail(message); }
function exactly(values, message) { assert.equal(values.length, 1, message); return values[0]; }
function input(h, id = 'community-nep-request') { return h.one('RNTextInput', props => props.testID === id); }
function field(h, id = 'community-nep-request') { return exactly(h.root.root.findAllByType(h.load('apps/mobile/src/rudi/ui/Field').Field).filter(node => node.props.testID === id), 'one actual Nếp Field'); }
function panel(h) { return h.one('RNAnimatedView', props => props.accessibilityLabel === 'Gọi Nếp'); }
function textWithin(node, value) { return exactly(node.findAllByType('RNText').filter(leaf => leaf.props.children === value), `one exact text: ${value}`); }
function button(h, label) { return exactly(panel(h).findAllByType('RNPressable').filter(node => node.findAllByType('RNText').some(leaf => leaf.props.children === label)), `one actual Nếp action: ${label}`); }
async function change(h, value, id = 'community-nep-request') { await act(async () => input(h, id).props.onChangeText(value)); }
async function press(node) { await act(async () => node.props.onPress()); }
function ownership(h, observers) {
  const { live, subscriptions, removals, maxLive } = h.observation();
  assert.deepEqual({ live, subscriptions, removals, maxLive }, { live: observers, subscriptions: observers, removals: 0, maxLive: observers }, 'one surface observer or an inherited/absent module boundary; never child observers');
  assert.equal(h.root.root.findAllByType(h.metrics.NativeFontMetricsProvider).length, 1, 'one actual provider owns the post and both Nếp slots');
}
function actual(h) {
  for (const name of ['community/PostDetail', 'community/Comments', 'community/api', 'ui/Sheet', 'ui/Field', 'ui/CauTaiCho', 'ui/NativeFontMetrics', 'chia-bill/useBillFontScale']) {
    const relative = `apps/mobile/src/rudi/${name}`, file = h.consumed.get(relative);
    assert.ok(file, `actual source consumed: ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true, 'strict complete source graph');
    assert.equal(h.consumedHashes.get(relative), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
}
function metrics(h, id, value, label) {
  const leaf = input(h, id), host = field(h, id), copy = textWithin(host, label), inputStyle = flatten(leaf.props.style), labelStyle = flatten(copy.props.style);
  const usable = value.status === 'available' && value.fontMetrics !== undefined;
  // The fixture uses native line pitch36 at 200%, not window.fontScale1 * base24.
  const expected = usable && value.fontScale === 2
    ? { size: 28.75, pitch: 36, min: 108, max: 288, labelSize: 23.25, labelPitch: 31.75 }
    : { size: 17, pitch: 24, min: 72, max: 192, labelSize: 14, labelPitch: 19 };
  assert.equal(inputStyle.fontSize, expected.size, 'actual textarea font metrics');
  assert.equal(inputStyle.lineHeight, expected.pitch, 'actual textarea native line pitch');
  assert.equal(inputStyle.minHeight, expected.min, 'three native input rows');
  assert.equal(inputStyle.maxHeight, expected.max, 'eight native input rows');
  assert.equal(leaf.props.allowFontScaling, usable ? false : undefined, 'manual conversion only when usable; RN owns fallback');
  assert.equal(labelStyle.fontSize, expected.labelSize, 'actual Field label font metrics');
  assert.equal(labelStyle.lineHeight, expected.labelPitch, 'actual Field label allocated line metrics');
  assert.equal(copy.props.allowFontScaling, usable ? false : undefined);
  assert.equal(host.props.nativeMetrics, true, 'constant opt-in preserves Field/Input wrapper types');
  assert.equal(leaf.props.maxFontSizeMultiplier, undefined, 'no font cap');
  if (usable) {
    const resolver = h.load('apps/mobile/src/rudi/native-font-metrics').resolveTextMetrics;
    const typography = h.load('apps/mobile/src/rudi/theme').typography;
    const resolved = resolver(value.fontMetrics, typography.body, {}, 'input');
    assert.equal(inputStyle.fontSize, resolved.style.fontSize);
    assert.equal(inputStyle.lineHeight, resolved.style.lineHeight, 'actual resolver consumed with the current native table');
  }
}
function document(h, id = 'community-nep-request') {
  const scope = panel(h), heading = textWithin(scope, title);
  const scroll = ancestor(heading, node => node.type === 'RNScrollView', 'title in the actual Sheet scroll');
  // These semantic ancestry assertions precede any diagnostic editor marker.
  assert.equal(inside(input(h, id), scroll), true, 'active editor shares the title/reference document scroll, outside fixed footer');
  assert.equal(scope.findAllByType('RNScrollView').length, 1, 'one document scroll; no nested fixed editor scroll');
  assert.equal(inside(textWithin(scope, disclosure), scroll), true);
  textWithin(scroll, h.state.post.body);
  const label = id === 'community-nep-draft' ? draftAction : input(h, id).props.editable === false ? 'Nếp đang viết…' : requestAction;
  const action = button(h, label);
  const footer = ancestor(action, node => node.type === 'RNView' && typeof node.props.onLayout === 'function', 'actual Sheet footer measurement host');
  assert.equal(inside(footer, scope), true);
  assert.equal(inside(action, scroll), false, 'primary decision stays outside document overflow');
  assert.equal(flatten(footer.props.style).flexShrink, 0, 'Sheet reserves the fixed action');
  assert.equal(footer.findAllByType('RNTextInput').length, 0, 'fixed footer cannot contain an editor');
  assert.equal(footer.findAllByType('RNScrollView').length, 0, 'fixed footer cannot contain a second document');
  assert.equal(footer.findAll(node => node.props.testID === 'cong-dong-loi-nep').length, 0, 'refusal stays with the document');
  const editor = ancestor(input(h, id), node => node.props.testID === 'community-nep-editor', 'diagnostic editor boundary exists after semantic ancestry passed');
  assert.equal(editor.type, 'RNView');
  assert.equal(inside(editor, scroll), true);
  const sheet = exactly(h.root.root.findAllByType(h.Sheet).filter(node => node.props.accessibilityLabel === 'Gọi Nếp'), 'one actual Nếp Sheet');
  assert.equal(sheet.props.avoidKeyboard, true); assert.equal(sheet.props.prioritizeFooter, true);
  assert.equal(sheet.props.showsVerticalScrollIndicator, true); assert.equal(sheet.props.dauTrang, undefined, 'no fixed title fragment');
  return { scroll, footer, editor, scope, action };
}
function retained(h, id, value, parts) {
  const before = { input: input(h, id), field: field(h, id), owner: h.owner('PostDetailContent'), ...parts };
  return () => {
    assert.equal(input(h, id) === before.input, true, 'same native textarea within its request/draft state');
    assert.equal(field(h, id) === before.field, true, 'same actual Field owner');
    assert.equal(h.owner('PostDetailContent') === before.owner, true, 'same post controller');
    assert.equal(input(h, id).props.value, value);
    const current = document(h, id);
    for (const name of ['scroll', 'footer', 'editor', 'scope']) assert.equal(current[name] === before[name], true, `same ${name} host across native font callbacks`);
  };
}
async function withNep(options, run) {
  const h = createHost({ surface: 'detail', nepTransport: true, initialSnapshot: font(1), ...options });
  h.state.window = { width: 320, height: 640, fontScale: 1, scale: 2 };
  try {
    await h.mount();
    await h.pressText('@Nếp · Giúp giữ khoảnh khắc');
    await run(h); actual(h);
  } finally { await h.dispose(); }
}
function payload(h, call, request) {
  assert.equal(call.route, '/v2/community/posts/synthetic-post-1/nep');
  assert.equal(call.options.method, 'POST'); assert.equal(call.options.actorId, 'synthetic-A');
  assert.deepEqual(plain(call.options.body), { confirmed: true, excerpt: h.state.post.body, request });
  assert.match(call.options.attempt.key, /^synthetic-attempt-/);
}
function refusal(h, parts, value) {
  const error = h.one('RNAnimatedView', props => props.testID === 'cong-dong-loi-nep');
  const copy = textWithin(error, value);
  const alert = ancestor(copy, node => node.type === 'RNView' && node.props.accessibilityRole === 'alert', 'actual refusal is a polite alert');
  assert.equal(alert.props.accessibilityLiveRegion, 'polite');
  assert.equal(inside(error, parts.scroll), true, 'full refusal can scroll with the editor');
  assert.equal(inside(error, parts.footer), false);
  const retry = button(h, 'Thử lại');
  assert.equal(inside(retry, error), false, 'retry has its own full-width row after the complete refusal');
  assert.equal(inside(retry, parts.scroll), true); assert.equal(flatten(retry.props.style).minHeight, 52, 'actual RudiButton retry keeps the full target contract');
  return { error, retry, copy };
}

test('QV01 request input label owner and one document survive stale-window 100→200→100', async () => {
  await withNep({}, async h => {
    const body = 'QC_QV01_A\n\nQC_QV01_C';
    await change(h, body);
    metrics(h, 'community-nep-request', font(1), requestLabel);
    const parts = document(h), keep = retained(h, 'community-nep-request', body, parts);
    const reads = h.callsFor('readComments').length, gets = h.callsFor('getPost').length;
    ownership(h, 1);
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value);
      metrics(h, 'community-nep-request', value, requestLabel); keep(); ownership(h, 1);
      assert.equal(h.state.window.fontScale, 1, 'window scale deliberately stale; no artificial route rerender');
      assert.equal(h.callsFor('readComments').length, reads); assert.equal(h.callsFor('getPost').length, gets);
    }
    assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0);
  });
});

test('QV02 long request pending refusal and actual retry stay in the document with CTA separate', async () => {
  await withNep({ initialSnapshot: font(2) }, async h => {
    const body = 'QC_QV02_' + 'A'.repeat(64);
    assert.equal(body.length, 72); await change(h, body);
    const parts = document(h), first = button(h, requestAction);
    assert.equal(first.props.disabled, false); await press(first);
    assert.equal(h.nepRequests.length, 1); payload(h, h.nepRequests[0], body);
    assert.equal(input(h).props.editable, false); assert.equal(button(h, 'Nếp đang viết…').props.disabled, true);
    await press(first); assert.equal(h.nepRequests.length, 1, 'working ref blocks a retained duplicate call');
    // Rejecting an unknown value exercises the actual public fallback sentence.
    await h.fail(h.nepRequests[0], 'Synthetic response refusal');
    const current = document(h), failure = refusal(h, current, requestRefusal);
    assert.equal(current.scroll === parts.scroll, true); assert.equal(current.footer === parts.footer, true);
    assert.equal(input(h).props.value, body); assert.equal(input(h).props.editable, true);
    assert.equal(flatten(failure.copy.props.style).fontSize, 28.75); assert.equal(flatten(failure.copy.props.style).lineHeight, 36);
    assert.equal(failure.copy.props.allowFontScaling, false, 'actual CauTaiCho consumes the same metrics owner');
    await press(failure.retry); assert.equal(h.nepRequests.length, 2); payload(h, h.nepRequests[1], body);
    assert.notEqual(h.nepRequests[1].options.attempt.key, h.nepRequests[0].options.attempt.key);
    assert.equal(input(h).props.value, body); assert.equal(input(h).props.editable, false);
    await h.fail(h.nepRequests[1], 'Synthetic second refusal'); refusal(h, document(h), requestRefusal);
    assert.equal(h.keepWrites.length, 0); assert.deepEqual(h.routes, []); ownership(h, 1);
  });
});

test('QV03 actual generated draft hot metrics editing save refusal and ACK preserve request semantics', async () => {
  await withNep({}, async h => {
    const request = 'QC_QV03 request unchanged', generated = 'QC_QV03 generated\n\nSynthetic draft';
    await change(h, request); await press(button(h, requestAction));
    await h.finish(h.nepRequests[0], { draft: generated });
    const id = 'community-nep-draft';
    metrics(h, id, font(1), draftLabel);
    const parts = document(h, id), keep = retained(h, id, generated, parts);
    assert.equal(input(h).props.value, request, 'original request remains editable in the reference');
    assert.equal(inside(input(h), parts.scroll), true); assert.equal(inside(button(h, requestAction), parts.scroll), true);
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value); metrics(h, id, value, draftLabel); keep(); ownership(h, 1);
      assert.equal(input(h).props.value, request);
    }
    const edited = 'QC_QV03 edited Aà😀\n\nC';
    await change(h, edited, id); const save = button(h, draftAction), retainedSave = save.props.onPress; assert.equal(save.props.disabled, false); await press(save);
    assert.equal(h.keepWrites.length, 1); assert.deepEqual(h.routes, []);
    const first = h.keepWrites[0];
    assert.equal(first.route, '/v2/community/keeps'); assert.equal(first.options.method, 'POST'); assert.equal(first.options.actorId, 'synthetic-A');
    assert.deepEqual(plain(first.options.body), { logical_id: first.options.body.logical_id, post_id: 'synthetic-post-1', body: edited, ai_generated: true });
    assert.match(first.options.body.logical_id, /^synthetic-attempt-/);
    assert.equal(input(h, id).props.editable, false); assert.equal(button(h, draftAction).props.disabled, true);
    await act(async () => retainedSave()); assert.equal(h.keepWrites.length, 1, 'working ref blocks a retained duplicate save');
    await h.fail(first, 'Synthetic save refusal');
    refusal(h, document(h, id), 'Chưa thực hiện được.');
    assert.equal(input(h, id).props.value, edited); assert.equal(input(h).props.value, request); assert.deepEqual(h.routes, []);
    const revised = edited + ' revised'; await change(h, revised, id); await press(button(h, draftAction));
    assert.equal(h.keepWrites.length, 2); const next = h.keepWrites[1];
    assert.notEqual(next.options.body.logical_id, first.options.body.logical_id, 'editing rotates the existing logical save attempt');
    assert.deepEqual(plain(next.options.body), { logical_id: next.options.body.logical_id, post_id: 'synthetic-post-1', body: revised, ai_generated: true });
    await h.finish(next, { id: 'synthetic-keep-1' });
    assert.deepEqual(h.routes, ['/community/keeps'], 'only an accepted save navigates to private keeps');
    const sheet = exactly(h.root.root.findAllByType(h.Sheet).filter(node => node.props.accessibilityLabel === 'Gọi Nếp'), 'Nếp Sheet component persists while closing');
    assert.equal(sheet.props.open, false); assert.equal(h.nepRequests.length, 1); ownership(h, 1);
  });
});

test('QV04 inherited unavailable and absent native boundaries retain one document and actual empty guards', async () => {
  for (const options of [
    { explicitProvider: true, initialSnapshot: font(2) },
    { initialSnapshot: unavailable() },
    { nativeFontModulePresent: false, initialSnapshot: font(2) },
  ]) {
    await withNep(options, async h => {
      document(h);
      const observers = options.explicitProvider || options.nativeFontModulePresent === false ? 0 : 1;
      ownership(h, observers);
      if (options.explicitProvider || options.nativeFontModulePresent === false) assert.equal(h.observation().reads, 0, 'no child or absent-module native reads');
      await change(h, ' \n '); const empty = button(h, requestAction); assert.equal(empty.props.disabled, true);
      await press(empty); assert.equal(h.nepRequests.length, 0, 'actual goiNep rejects empty input even through its retained handler');
      const body = 'QC_QV04 fallback request'; await change(h, body);
      const initial = options.nativeFontModulePresent === false ? unavailable() : options.initialSnapshot;
      metrics(h, 'community-nep-request', initial, requestLabel);
      const keep = retained(h, 'community-nep-request', body, document(h));
      if (options.nativeFontModulePresent !== false) {
        for (const value of [unavailable(), { status: 'available', fontScale: 2 }, font(2), font(1)]) {
          await h.nativeFont(value); metrics(h, 'community-nep-request', value, requestLabel); keep(); ownership(h, observers);
        }
      } else {
        await h.nativeFont(font(2));
        metrics(h, 'community-nep-request', unavailable(), requestLabel); keep(); ownership(h, 0);
      }
      if (options.explicitProvider) {
        await press(button(h, requestAction)); payload(h, h.nepRequests[0], body);
        await h.finish(h.nepRequests[0], { draft: '   ' });
        document(h, 'community-nep-draft');
        assert.equal(button(h, draftAction).props.disabled, true, 'whitespace draft keeps the existing save disabled state');
        assert.equal(input(h).props.value, body); assert.equal(h.keepWrites.length, 0);
        // No synthetic press of a disabled save: source has no empty-draft handler guard.
      }
      assert.deepEqual(h.routes, []);
    });
  }
});


test('QV05 full refusal and retry use separate document rows under the existing metrics owner', async () => {
  await withNep({}, async h => {
    const body = 'QC_QV05_A\n\nQC_QV05_C';
    await change(h, body); await press(button(h, requestAction));
    await h.fail(h.nepRequests[0], 'Synthetic request refused');
    const parts = document(h), failure = refusal(h, parts, requestRefusal);
    const stack = ancestor(failure.error, node => node.type === 'RNView' && inside(failure.retry, node), 'actual common document parent for refusal and retry');
    assert.equal(flatten(stack.props.style).flexDirection, 'column');
    assert.equal(flatten(stack.props.style).alignItems, 'stretch');
    assert.equal(flatten(stack.props.style).gap, 8);
    assert.equal(stack.props.testID, 'community-nep-refusal', 'marker follows the semantic row ancestry witness');
    assert.equal(failure.error.findAllByType('RNPressable').length, 0, 'sentence width is not divided with its retry');
    assert.equal(flatten(failure.retry.props.style).width, '100%', 'retry receives its own full-width row');
    assert.equal(failure.copy.props.numberOfLines, undefined, 'no important text line cap');
    assert.equal(failure.copy.props.ellipsizeMode, undefined, 'no important text truncation');
    const native = input(h), controller = h.owner('PostDetailContent');
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value);
      const current = refusal(h, document(h), requestRefusal);
      assert.equal(current.error === failure.error, true); assert.equal(current.retry === failure.retry, true);
      assert.equal(current.copy === failure.copy, true); assert.equal(input(h) === native, true);
      assert.equal(h.owner('PostDetailContent') === controller, true);
      assert.equal(flatten(current.copy.props.style).fontSize, value.fontScale === 2 ? 28.75 : 17);
      assert.equal(flatten(current.copy.props.style).lineHeight, value.fontScale === 2 ? 36 : 24);
      assert.equal(current.copy.props.allowFontScaling, false);
      assert.equal(input(h).props.value, body); ownership(h, 1);
    }
    assert.equal(h.nepRequests.length, 1); assert.equal(h.keepWrites.length, 0);
    assert.deepEqual(h.routes, []);
  });
});

test('QV06 clearing a generated draft preserves its actual editor and disabled Save through hot metrics', async () => {
  await withNep({}, async h => {
    const request = 'QC_QV06 request retained', generated = 'QC_QV06 generated';
    await change(h, request); await press(button(h, requestAction));
    await h.finish(h.nepRequests[0], { draft: generated });
    const id = 'community-nep-draft', native = input(h, id), coreField = field(h, id);
    const requestNative = input(h), controller = h.owner('PostDetailContent'), parts = document(h, id);
    await act(async () => native.props.onFocus({}));
    await change(h, '', id);
    assert.equal(h.root.root.findAllByType('RNTextInput').includes(native), true, 'empty content must retain the already mounted native draft editor');
    assert.equal(input(h, id) === native, true); assert.equal(field(h, id) === coreField, true);
    assert.equal(input(h, id).props.value, ''); assert.equal(input(h) === requestNative, true);
    assert.equal(input(h).props.value, request); assert.equal(h.owner('PostDetailContent') === controller, true);
    assert.equal(button(h, draftAction).props.disabled, true);
    textWithin(parts.footer, 'Viết nội dung ghi chép trước khi lưu.');
    assert.equal(inside(button(h, requestAction), parts.scroll), true, 'regeneration stays with the original request');
    assert.equal(inside(button(h, draftAction), parts.scroll), false, 'empty draft retains the Save decision in the fixed footer');
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value); metrics(h, id, value, draftLabel);
      assert.equal(input(h, id) === native, true); assert.equal(field(h, id) === coreField, true);
      assert.equal(input(h, id).props.value, ''); assert.equal(input(h).props.value, request);
      assert.equal(document(h, id).scroll === parts.scroll, true); assert.equal(button(h, draftAction).props.disabled, true);
      ownership(h, 1);
    }
    await change(h, 'QC_QV06 revised Aà😀\n\nC', id);
    assert.equal(input(h, id) === native, true); assert.equal(button(h, draftAction).props.disabled, false);
    assert.equal(h.nepRequests.length, 1); assert.equal(h.keepWrites.length, 0); assert.deepEqual(h.routes, []);
    await press(button(h, draftAction));
    assert.equal(h.keepWrites.length, 1); await h.fail(h.keepWrites[0], 'Synthetic save refused before clearing');
    await change(h, '', id);
    assert.equal(input(h, id) === native, true); assert.equal(input(h, id).props.value, '');
    assert.equal(button(h, draftAction).props.disabled, true);
    assert.equal(button(h, 'Thử lại').props.disabled, true, 'operation-specific retry cannot offer an empty native save');
    textWithin(document(h, id).scroll, 'Viết nội dung ghi chép trước khi thử lại.');
    assert.equal(h.keepWrites.length, 1); assert.equal(h.nepRequests.length, 1);
    await change(h, 'QC_QV06 refilled', id);
    assert.equal(input(h, id) === native, true); assert.equal(button(h, 'Thử lại').props.disabled, false);
    // Disabled Save/Retry handlers are not invoked: the existing handler contract remains unchanged.
  });
});

test('QV07 retry follows the recorded failed operation and preserves keep identity and consent payloads', async () => {
  await withNep({ initialSnapshot: font(2) }, async h => {
    const request = 'QC_QV07 original request', generated = 'QC_QV07 generated', edited = 'QC_QV07 edited Aà😀\n\nC';
    await change(h, request); await press(button(h, requestAction));
    await h.finish(h.nepRequests[0], { draft: generated });
    const id = 'community-nep-draft', native = input(h, id), parts = document(h, id);
    await change(h, edited, id); await press(button(h, draftAction));
    const first = h.keepWrites[0];
    assert.equal(h.keepWrites.length, 1); await h.fail(first, 'Synthetic keep refusal');
    const failedSave = refusal(h, document(h, id), 'Chưa thực hiện được.'), retainedRetry = failedSave.retry.props.onPress;
    await press(failedSave.retry);
    assert.equal(h.nepRequests.length, 1, 'save retry must not send a fresh inference request');
    assert.equal(h.keepWrites.length, 2, 'save retry repeats the actual keep transport');
    assert.equal(button(h, 'Thử lại').props.disabled, true, 'retry cannot offer another action while the recorded save is pending');
    const second = h.keepWrites[1];
    assert.equal(second.route, '/v2/community/keeps'); assert.equal(second.options.method, 'POST');
    assert.equal(second.options.actorId, first.options.actorId);
    assert.deepEqual(plain(second.options.body), plain(first.options.body), 'failed save retains logical_id, body, post and generated provenance');
    assert.notEqual(second.options.attempt.key, first.options.attempt.key, 'transport retry still uses the existing newAttempt policy');
    await act(async () => retainedRetry());
    assert.equal(h.keepWrites.length, 2, 'working guard blocks a retained duplicate save retry');
    assert.equal(h.nepRequests.length, 1);
    await h.fail(second, 'Synthetic keep refused again');
    assert.equal(input(h, id) === native, true); assert.equal(input(h, id).props.value, edited);
    await press(button(h, requestAction));
    assert.equal(h.nepRequests.length, 2); payload(h, h.nepRequests[1], request);
    await h.fail(h.nepRequests[1], 'Synthetic regeneration refusal');
    const failedRequest = refusal(h, document(h, id), requestRefusal);
    await press(failedRequest.retry);
    assert.equal(h.keepWrites.length, 2, 'request failure retry must not infer its operation from an existing draft');
    assert.equal(h.nepRequests.length, 3); payload(h, h.nepRequests[2], request);
    assert.notEqual(h.nepRequests[2].options.attempt.key, h.nepRequests[1].options.attempt.key);
    await h.finish(h.nepRequests[2], { draft: 'QC_QV07 regenerated' });
    assert.equal(input(h, id) === native, true); assert.equal(input(h).props.value, request);
    assert.equal(input(h, id).props.value, 'QC_QV07 regenerated');
    assert.equal(document(h, id).scroll === parts.scroll, true);
    assert.deepEqual(h.routes, []); ownership(h, 1);
  });
});

test('QV08 generated review keeps consent secondary and Save primary without automatic regeneration', async () => {
  await withNep({}, async h => {
    const request = 'QC_QV08 request Aà😀\n\nC', generated = 'QC_QV08 generated Aà😀\n\nC';
    const Button = h.load('apps/mobile/src/rudi/ui').RudiButton;
    const core = node => ancestor(node, at => at.type === Button, 'actual RudiButton owns the native action');
    const gradients = node => node.findAllByType('RNLinearGradient').length;
    await change(h, request);
    const firstParts = document(h), first = button(h, requestAction), firstCore = core(first);
    assert.equal(inside(first, firstParts.footer), true);
    assert.equal(inside(first, firstParts.scroll), false);
    assert.equal(gradients(first), 1, 'first explicit consent remains the primary gradient action');
    assert.equal(flatten(first.props.style).width, '100%');
    assert.equal(flatten(first.props.style).minHeight, 52);
    assert.equal(flatten(first.props.style).minWidth, 48);
    assert.equal(first.props.accessibilityRole, 'button'); assert.equal(first.props.disabled, false);
    assert.equal(h.nepRequests.length, 0); assert.equal(h.keepWrites.length, 0);
    const firstInput = input(h), retainedConsent = first.props.onPress;
    await press(first); assert.equal(h.nepRequests.length, 1); payload(h, h.nepRequests[0], request);
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value);
      const pending = button(h, 'Nếp đang viết…');
      assert.equal(core(pending) === firstCore, true); assert.equal(pending === first, true);
      assert.equal(flatten(pending.props.style).width, '100%');
      assert.equal(flatten(pending.props.style).minHeight, 52);
      assert.equal(pending.props.disabled, true); assert.equal(input(h) === firstInput, true);
      assert.equal(input(h).props.value, request); assert.equal(input(h).props.editable, false);
      metrics(h, 'community-nep-request', value, requestLabel); ownership(h, 1);
    }
    await act(async () => retainedConsent()); assert.equal(h.nepRequests.length, 1, 'pending initial consent cannot duplicate the request');
    await h.finish(h.nepRequests[0], { draft: generated });
    const id = 'community-nep-draft', parts = document(h, id);
    const native = input(h, id), nativeField = field(h, id), requestNative = input(h), requestField = field(h);
    const controller = h.owner('PostDetailContent');
    const consent = button(h, requestAction), consentCore = core(consent), saveCore = core(button(h, draftAction));
    function review(value, fontValue, busy = false) {
      const current = document(h, id), call = button(h, busy ? 'Nếp đang viết…' : requestAction), save = button(h, draftAction);
      assert.equal(gradients(call), 0, 'generated review must not paint regeneration as the primary action');
      assert.equal(core(call).props.variant, 'ghost'); assert.equal(core(call).props.compact, true);
      assert.equal(core(call).props.full, false);
      const callStyle = flatten(call.props.style);
      if (busy) {
        assert.equal(callStyle.borderStyle, 'dashed'); assert.equal(callStyle.borderWidth, 1.5, 'pending consent retains the existing readable disabled treatment');
      } else {
        assert.equal(callStyle.backgroundColor, 'transparent'); assert.equal(callStyle.borderColor, 'transparent');
      }
      assert.equal(callStyle.minHeight, 48); assert.equal(callStyle.minWidth, 48);
      assert.equal(callStyle.width, undefined); assert.equal(callStyle.alignSelf, 'flex-start');
      assert.equal(callStyle.maxWidth, '100%', 'secondary consent is constrained by the same document width');
      assert.equal(inside(call, current.scroll), true); assert.equal(inside(call, current.footer), false);
      assert.equal(core(save).props.variant, 'solid', 'Save owns the primary treatment after a draft exists');
      assert.equal(inside(save, current.footer), true); assert.equal(inside(save, current.scroll), false);
      assert.equal(flatten(save.props.style).width, '100%'); assert.equal(flatten(save.props.style).minHeight, 52);
      const disabled = busy || !value.trim();
      assert.equal(save.props.disabled, disabled); assert.equal(gradients(save), disabled ? 0 : 1, 'enabled fixed Save paints the primary gradient');
      assert.equal(call.props.disabled, busy); assert.equal(call.props.accessibilityRole, 'button');
      for (const [action, label] of [[call, busy ? 'Nếp đang viết…' : requestAction], [save, draftAction]]) {
        const copy = textWithin(action, label), style = flatten(copy.props.style);
        assert.equal(copy.props.allowFontScaling, false);
        assert.equal(style.fontSize, fontValue.fontScale === 2 ? 23.25 : 14);
        assert.equal(style.lineHeight, fontValue.fontScale === 2 ? 31.75 : 19);
        assert.equal(copy.props.numberOfLines, undefined); assert.equal(copy.props.ellipsizeMode, undefined);
      }
      assert.equal(input(h, id) === native, true); assert.equal(field(h, id) === nativeField, true);
      assert.equal(input(h) === requestNative, true); assert.equal(field(h) === requestField, true);
      assert.equal(input(h, id).props.value, value); assert.equal(input(h).props.value, request);
      assert.equal(input(h, id).props.editable, !busy); assert.equal(input(h).props.editable, !busy);
      assert.equal(core(call) === consentCore, true); assert.equal(core(save) === saveCore, true);
      assert.equal(h.owner('PostDetailContent') === controller, true);
      for (const name of ['scroll', 'footer', 'editor', 'scope']) assert.equal(current[name] === parts[name], true, `same review ${name} host`);
      metrics(h, id, fontValue, draftLabel); ownership(h, 1);
      assert.equal(h.state.window.fontScale, 1, 'native metrics change without a window-scale rerender');
    }
    review(generated, font(1));
    for (const value of [font(2), font(1)]) {
      await h.nativeFont(value); review(generated, value);
      assert.equal(button(h, requestAction) === consent, true, 'hot metrics retain the secondary native action');
      assert.equal(h.nepRequests.length, 1); assert.equal(h.keepWrites.length, 0);
    }
    await act(async () => native.props.onFocus({}));
    await change(h, '', id); review('', font(1));
    textWithin(parts.footer, 'Viết nội dung ghi chép trước khi lưu.');
    for (const value of [font(2), font(1)]) { await h.nativeFont(value); review('', value); }
    assert.equal(h.nepRequests.length, 1); assert.equal(h.keepWrites.length, 0, 'clearing or metrics cannot submit either operation');
    const edited = 'QC_QV08 edited Aà😀\n\nC'; await change(h, edited, id); review(edited, font(1));
    const regenerate = button(h, requestAction), retainedRegenerate = regenerate.props.onPress;
    await press(regenerate);
    assert.equal(h.nepRequests.length, 2, 'only the explicit secondary consent starts regeneration');
    payload(h, h.nepRequests[1], request);
    assert.notEqual(h.nepRequests[1].options.attempt.key, h.nepRequests[0].options.attempt.key);
    review(edited, font(1), true); assert.equal(h.keepWrites.length, 0);
    await h.nativeFont(font(2)); review(edited, font(2), true);
    await act(async () => retainedRegenerate()); assert.equal(h.nepRequests.length, 2, 'the unchanged working guard blocks retained regeneration');
    const regenerated = 'QC_QV08 regenerated Aà😀\n\nC';
    await h.finish(h.nepRequests[1], { draft: regenerated }); review(regenerated, font(2));
    await h.nativeFont(font(1)); review(regenerated, font(1));
    const save = button(h, draftAction), retainedSave = save.props.onPress;
    await press(save); review(regenerated, font(1), true);
    assert.equal(h.nepRequests.length, 2); assert.equal(h.keepWrites.length, 1);
    const write = h.keepWrites[0];
    assert.equal(write.route, '/v2/community/keeps'); assert.equal(write.options.method, 'POST');
    assert.equal(write.options.actorId, 'synthetic-A');
    assert.deepEqual(plain(write.options.body), { logical_id: write.options.body.logical_id, post_id: 'synthetic-post-1', body: regenerated, ai_generated: true });
    assert.match(write.options.body.logical_id, /^synthetic-attempt-/); assert.match(write.options.attempt.key, /^synthetic-attempt-/);
    await act(async () => retainedSave()); assert.equal(h.keepWrites.length, 1, 'the primary treatment preserves the retained-save lock');
    assert.deepEqual(h.routes, []);
    await h.finish(write, { id: 'synthetic-keep-1' });
    assert.deepEqual(h.routes, ['/community/keeps']); assert.equal(h.nepRequests.length, 2);
  });
});
