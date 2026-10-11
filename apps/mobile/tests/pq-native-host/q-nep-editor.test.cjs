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
  const retry = exactly(error.findAllByType('RNPressable'), 'one actual retry handler');
  assert.equal(inside(retry, parts.scroll), true); assert.equal(flatten(retry.props.style).minHeight, 48, 'full retry target contract');
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
