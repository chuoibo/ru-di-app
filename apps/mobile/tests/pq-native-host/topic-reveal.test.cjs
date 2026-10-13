// Actual Composer, Field and RudiScreen behavior with explicit native callback boundaries.
// Rectangles are host fixtures; Android must separately prove paint, IME and caret visibility.
const { test, after } = require('node:test');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
after(() => { if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { createHost, snapshot, act, sourceRoot } = require('./owner-host.cjs');

const validTopics = 'Alpha, Beta';
const invalidTopics = 'Alpha, Beta, Gamma, Delta, Epsilon, Zeta';
const bodyValue = 'Synthetic topic draft Aà😀\nC';
const errorText = 'Tối đa 5 chủ đề. Bỏ bớt 1 chủ đề nhé.';
const helperText = 'Tối đa 5 chủ đề, ngăn cách bằng dấu phẩy.';
const moderation = 'Bài được duyệt trước khi xuất hiện trên cộng đồng.';
const font = scale => snapshot(scale, scale === 2 ? 36 : 24);
function ancestor(node, type) { let value = node.parent; while (value && value.type !== type) value = value.parent; assert.ok(value, `actual ${type} ancestor`); return value; }
function inside(node, parent) { for (let value = node.parent; value; value = value.parent) if (value === parent) return true; return false; }
function topic(h) { return h.one('RNTextInput', props => props.accessibilityLabel === 'Chủ đề'); }
function field(h) { return h.root.root.findByType(h.load('apps/mobile/src/rudi/ui/Field').Field); }
function tools(h) { return ancestor(field(h), 'RNView'); }
function scroll(h) { return ancestor(field(h), 'RNScrollView'); }
function screen(h) { return h.root.root.findByType(h.load('apps/mobile/src/rudi/ui').RudiScreen); }
function clearance(h) { return screen(h).props.focusBottomInset ?? 16; }
function commands(h) { return h.nativeHostCalls.filter(call => call.kind === 'scrollTo'); }
function latest(h, kind, node) { const pending = h.native.pending(kind, node); assert.ok(pending.length > 0, `actual ${kind} requested for the focused field`); return pending.at(-1); }
async function frames(h) { let count = 0; while (h.native.pendingFrames() && count++ < 12) await h.native.frame(); assert.equal(h.native.pendingFrames(), 0, 'bounded native RAF queue'); }
async function requestTail(h) { await frames(h); return latest(h, 'measure', h.native.node(tools(h))); }
async function tail(h, measurement = null, value = 69, inputHeight = 72) {
  measurement ??= await requestTail(h);
  const input = h.native.node(topic(h)), container = h.native.node(tools(h));
  await h.native.resolve(measurement, [0, 0, 288, 250 + inputHeight + value, 16, 0]);
  const layout = latest(h, 'measureLayout', input);
  assert.equal(layout.ancestor === container, true, 'native input measured relative to its actual tools ancestor');
  await h.native.resolve(layout, [0, 250, 258, inputHeight]);
  return layout;
}
async function viewport(h) { await frames(h); return latest(h, 'measureInWindow', h.native.node(scroll(h))); }
async function finishReveal(h, measure, inputTop = 150) {
  await h.native.resolve(measure, [0, 77, 320, 194]);
  const input = latest(h, 'measureInWindow', h.native.node(topic(h)));
  await h.native.resolve(input, [31, inputTop, 258, 72]);
  return input;
}
async function layout(h) { await act(async () => tools(h).props.onLayout?.({ nativeEvent: { layout: { x: 0, y: 0, width: 288, height: 391 } } })); }
function actual(h) {
  for (const relative of ['community/Composer', 'community/MediaPicker', 'community/MentionPicker', 'ui/Field', 'ui/NativeFontMetrics']) {
    const key = `apps/mobile/src/rudi/${relative}`, file = h.consumed.get(key);
    assert.ok(file, `actual source consumed ${relative}`);
    assert.equal(fs.realpathSync(file).startsWith(fs.realpathSync(sourceRoot) + path.sep), true);
    assert.equal(h.consumedHashes.get(key), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
  }
  const file = h.consumed.get('apps/mobile/src/rudi/ui'); assert.ok(file);
  assert.equal(h.consumedHashes.get('apps/mobile/src/rudi/ui'), crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'));
}
async function withComposer(run, topics = invalidTopics) {
  const h = createHost({ surface: 'composer', initialSnapshot: font(2), nativeGeometry: true });
  h.state.window = { width: 320, height: 640, fontScale: 2, scale: 1 };
  try {
    await h.mount();
    const vault = h.load('apps/mobile/src/rudi/community/ban-nhap'), lease = vault.phienNhapBai('synthetic-A');
    assert.notEqual(lease, null);
    vault.ghiNhapBai('synthetic-A', 'new:community', { body: bodyValue, audience: 'public', topics, mentions: ['synthetic-B'], media: [], group: null, revision: 0, logical_id: 'synthetic-reveal-attempt', pendingFile: null }, lease);
    await h.update(state => { state.show = false; }); await h.update(state => { state.show = true; });
    await run(h, vault, lease);
    actual(h);
  } finally { await h.dispose(); }
}
function ownership(h) { const { reads: _reads, ...value } = h.observation(); return value; }
function retained(h) {
  const values = { input: topic(h), body: h.input(), field: field(h), tools: tools(h), form: h.owner('ComposerForm'), picker: h.owner('MediaPicker'), mention: h.owner('MentionPicker'), observation: ownership(h) };
  return () => {
    assert.equal(topic(h) === values.input, true, 'same native topic input'); assert.equal(h.input() === values.body, true);
    assert.equal(field(h) === values.field, true); assert.equal(tools(h) === values.tools, true, 'same native tools parent');
    for (const [name, node] of [['ComposerForm', values.form], ['MediaPicker', values.picker], ['MentionPicker', values.mention]]) assert.equal(h.owner(name) === node, true, `same ${name} owner`);
    assert.equal(h.input().props.value, bodyValue); assert.deepEqual(ownership(h), values.observation, 'no font-owner subscription churn');
  };
}

test('TR01 actual topics measure the full validation tail while exact disclosure belongs to the scroll body', async () => {
  await withComposer(async (h, vault, lease) => {
    const keep = retained(h), saved = vault.docNhapBai('synthetic-A', 'new:community');
    const notice = h.text(moderation); assert.equal(notice.length, 1, 'one unchanged moderation disclosure');
    assert.equal(inside(notice[0], scroll(h)), true, 'full disclosure in the scroll body, outside the fixed footer');
    const author = h.one('RNPressable', props => props.accessibilityLabel === 'Người đọc: Cộng đồng');
    assert.equal(inside(notice[0], author), false, 'disclosure remains a separately accessible text');
    const error = h.text(errorText); assert.equal(error.length, 1); assert.equal(error[0].props.accessibilityRole, 'alert'); assert.equal(error[0].props.accessibilityLiveRegion, 'polite');
    await h.native.focus(topic(h)); await tail(h);
    assert.equal(clearance(h), 85, 'whole native tail69 plus16 breathing room');
    keep(); assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), saved); assert.equal(vault.phienNhapBai('synthetic-A'), lease);
    const submit = h.hosts('RNPressable').filter(node => node.findAllByType('RNText').some(text => text.props.children === 'Gửi lên cộng đồng'));
    assert.equal(submit.length, 1); assert.equal(submit[0].props.disabled, true);
    await act(async () => submit[0].props.onPress());
    assert.equal(h.calls.some(call => call.options.method === 'POST'), false, 'invalid topics cannot send even through a retained press handler');
    assert.equal(h.picks.length, 0); assert.equal(h.uploads.length, 0);
  });
});

test('TR02 helper error body and hot-font ABA revisions reject retired native tail callbacks', async () => {
  await withComposer(async (h, vault) => {
    const keep = retained(h); await h.native.focus(topic(h));
    const errorA = await requestTail(h);
    await act(async () => topic(h).props.onChangeText(validTopics));
    const helper = h.text(helperText); assert.equal(helper.length, 1); assert.equal(h.text(errorText).length, 0);
    const helperA = await requestTail(h);
    await act(async () => topic(h).props.onChangeText(invalidTopics));
    const errorB = await requestTail(h);
    const pending = h.native.pending('measureLayout').length;
    await h.native.resolve(errorA, [0, 0, 288, 900, 16, 0]);
    await h.native.resolve(helperA, [0, 0, 288, 850, 16, 0]);
    assert.equal(h.native.pending('measureLayout').length, pending, 'old error/helper callbacks do not start input measurement');
    assert.equal(clearance(h), 16, 'pending current revision uses baseline clearance');
    await tail(h, errorB); assert.equal(clearance(h), 85);
    await layout(h); const fontA = await requestTail(h);
    await h.nativeFont(font(1)); const fontB = await requestTail(h);
    await h.nativeFont(font(2)); const fontC = await requestTail(h);
    await h.native.resolve(fontA, [0, 0, 288, 800, 16, 0]);
    await h.native.resolve(fontB, [0, 0, 288, 750, 16, 0]);
    assert.equal(clearance(h), 16, '100→200 ABA cannot donate stale geometry');
    await tail(h, fontC); assert.equal(clearance(h), 85); keep();
    assert.equal(vault.docNhapBai('synthetic-A', 'new:community').topics, invalidTopics);
    assert.equal(h.text(errorText).length, 1); assert.equal(h.text(helperText).length, 0);
    assert.equal(h.calls.some(call => call.options.method === 'POST'), false);
  });
});

test('TR03 replacement native refs blur retired lease and unmount reject pending field geometry', async () => {
  await withComposer(async (h, vault) => {
    await h.native.focus(topic(h));
    const first = await requestTail(h); await h.native.resolve(first, [0, 0, 288, 391, 16, 0]);
    const oldToolsLayout = latest(h, 'measureLayout', h.native.node(topic(h)));
    const toolsSwap = h.native.replace(tools(h));
    await h.native.resolve(oldToolsLayout, [0, 250, 258, 72]); assert.equal(clearance(h), 16, 'stale ancestor ref rejected'); toolsSwap.restore();
    await layout(h); const second = await requestTail(h); await h.native.resolve(second, [0, 0, 288, 391, 16, 0]);
    const oldInputLayout = latest(h, 'measureLayout', h.native.node(topic(h)));
    const inputSwap = h.native.replace(topic(h));
    await h.native.resolve(oldInputLayout, [0, 250, 258, 72]); assert.equal(clearance(h), 16, 'stale input ref rejected'); inputSwap.restore();
    await layout(h); const blurPending = await requestTail(h);
    await h.native.blur(topic(h)); await h.native.resolve(blurPending, [0, 0, 288, 391, 16, 0]);
    assert.equal(clearance(h), 16, 'blur retires pending measurement');
    await h.native.focus(topic(h)); const leasePending = await requestTail(h);
    vault.datChuSoHuuNhapBai('synthetic-retired'); vault.datChuSoHuuNhapBai('synthetic-A');
    await h.native.resolve(leasePending, [0, 0, 288, 391, 16, 0]);
    assert.equal(clearance(h), 16, 'same actor after a new lease cannot accept old editor geometry');
    assert.equal(vault.docNhapBai('synthetic-A', 'new:community'), null);
    await h.update(state => { state.show = false; });
    assert.equal(h.observation().live, 0);
    assert.equal(h.native.pendingFrames(), 0, 'owner cleanup cancels its RAF');
    assert.equal(commands(h).length, 0);
  });
  await withComposer(async h => {
    await h.native.focus(topic(h)); const pending = await requestTail(h);
    await layout(h); assert.ok(h.native.pendingFrames() > 0, 'field has queued RAF before unmount');
    await h.update(state => { state.show = false; });
    await h.native.resolve(pending, [0, 0, 288, 391, 16, 0]);
    assert.equal(h.native.pending('measureLayout').length, 0, 'unmounted owner cannot start a later native measure');
    assert.equal(h.native.pendingFrames(), 0);
  });
});

test('TR04 actual screen reruns reveal for a measured inset and user drag cancels every async stage', async () => {
  await withComposer(async h => {
    await h.native.focus(topic(h)); const pendingTail = await requestTail(h);
    await h.native.emitKeyboard('keyboardDidShow');
    await finishReveal(h, await viewport(h)); assert.equal(commands(h).length, 0, 'input itself fits with baseline16');
    await tail(h, pendingTail);
    assert.equal(clearance(h), 85);
    assert.ok(h.native.pendingFrames() > 0, 'inset update itself schedules real screen reveal without a viewport event');
    await finishReveal(h, await viewport(h));
    assert.equal(commands(h).length, 1); assert.deepEqual({ ...commands(h)[0].options }, { y: 36, animated: false });
    assert.equal(commands(h)[0].target, h.native.node(scroll(h)).id);
    const count = commands(h).length;
    await act(async () => scroll(h).props.onLayout());
    assert.ok(h.native.pendingFrames() > 0);
    await act(async () => scroll(h).props.onScrollBeginDrag()); await frames(h);
    assert.equal(commands(h).length, count, 'drag after RAF scheduling cancels reveal');
    assert.equal(h.native.pending('measureInWindow').length, 0);
    await h.native.emitKeyboard('keyboardDidHide'); await h.native.emitKeyboard('keyboardDidShow');
    const duringViewport = await viewport(h);
    await act(async () => scroll(h).props.onScrollBeginDrag());
    await h.native.resolve(duringViewport, [0, 77, 320, 194]);
    assert.equal(h.native.pending('measureInWindow', h.native.node(topic(h))).length, 0, 'drag during viewport callback cannot measure the input');
    await h.native.emitKeyboard('keyboardDidHide'); await h.native.emitKeyboard('keyboardDidShow');
    const duringInput = await viewport(h); await h.native.resolve(duringInput, [0, 77, 320, 194]);
    const input = latest(h, 'measureInWindow', h.native.node(topic(h)));
    await act(async () => scroll(h).props.onScrollBeginDrag()); await h.native.resolve(input, [31, 150, 258, 72]);
    assert.equal(commands(h).length, count, 'drag during input callback cannot steal the scroll');
    await layout(h); await tail(h); await frames(h);
    assert.equal(h.native.pending('measureInWindow').length, 0, 'new field layout does not reset the manual-drag escape');
  });
});

test('TR05 keyboard hide changed focused handle and unmount retire queued screen measurements', async () => {
  await withComposer(async (h, vault) => {
    const saved = vault.docNhapBai('synthetic-A', 'new:community');
    await h.native.focus(topic(h)); await tail(h); await h.native.emitKeyboard('keyboardDidShow');
    const hidden = await viewport(h); await h.native.emitKeyboard('keyboardDidHide');
    await h.native.resolve(hidden, [0, 77, 320, 194]);
    assert.equal(h.native.pending('measureInWindow', h.native.node(topic(h))).length, 0, 'hide rejects queued viewport callback');
    await h.native.emitKeyboard('keyboardDidShow'); const changed = await viewport(h);
    await h.native.resolve(changed, [0, 77, 320, 194]); const oldInput = latest(h, 'measureInWindow', h.native.node(topic(h)));
    await h.native.focus(h.input()); await h.native.resolve(oldInput, [31, 150, 258, 72]);
    assert.equal(commands(h).length, 0, 'a different focused native input rejects the old measurement');
    await h.native.focus(topic(h)); await h.native.emitKeyboard('keyboardDidHide'); await h.native.emitKeyboard('keyboardDidShow');
    const retired = await viewport(h); await h.native.resolve(retired, [0, 77, 320, 194]); const retiredInput = latest(h, 'measureInWindow', h.native.node(topic(h)));
    await h.update(state => { state.show = false; }); await h.native.resolve(retiredInput, [31, 150, 258, 72]);
    assert.equal(commands(h).length, 0, 'unmounted screen cannot scroll from a retained callback');
    assert.equal(h.native.pendingFrames(), 0); assert.equal(h.observation().live, 0);
    assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), saved, 'measurement and screen retirement do not modify the draft');
  });
  await withComposer(async h => {
    await h.native.focus(topic(h)); await tail(h); await h.native.emitKeyboard('keyboardDidShow');
    assert.ok(h.native.pendingFrames() > 0, 'screen has queued reveal RAF before unmount');
    await h.update(state => { state.show = false; });
    assert.equal(h.native.pendingFrames(), 0, 'screen cleanup cancels queued reveal frames');
    assert.equal(h.native.pending('measureInWindow').length, 0, 'retired RAF never reaches native measurement');
    assert.equal(commands(h).length, 0);
  });
});

test('TR06 invalid native geometry fails to baseline and explicit refocus starts a fresh request', async () => {
  await withComposer(async h => {
    const keep = retained(h); await h.native.focus(topic(h)); await tail(h); assert.equal(clearance(h), 85);
    await layout(h); const invalidTools = await requestTail(h); await h.native.resolve(invalidTools, [0, 0, 288, NaN, 16, 0]);
    assert.equal(clearance(h), 16, 'invalid tools box drops accepted geometry');
    for (const dimensions of [[0, -1, 258, 72], [0, 250, Infinity, 72], [0, 250, 258, 0], [0, 400, 258, 72]]) {
      await layout(h); const pending = await requestTail(h); await h.native.resolve(pending, [0, 0, 288, 391, 16, 0]);
      const input = latest(h, 'measureLayout', h.native.node(topic(h))); await h.native.resolve(input, dimensions);
      assert.equal(clearance(h), 16, 'invalid coordinates or negative tail never become an inset');
    }
    await layout(h); const failure = await requestTail(h); await h.native.resolve(failure, [0, 0, 288, 391, 16, 0]);
    await h.native.fail(latest(h, 'measureLayout', h.native.node(topic(h)))); assert.equal(clearance(h), 16);
    await h.native.focus(topic(h));
    await tail(h); assert.equal(clearance(h), 85, 'same-focus event starts measurement even without state change'); keep();
    await h.native.emitKeyboard('keyboardDidShow');
    const badViewport = await viewport(h); await h.native.resolve(badViewport, [0, NaN, 320, 194]);
    assert.equal(h.native.pending('measureInWindow', h.native.node(topic(h))).length, 0, 'invalid viewport cannot start an input measurement');
    await act(async () => scroll(h).props.onLayout());
    const zeroViewport = await viewport(h); await h.native.resolve(zeroViewport, [0, 77, 320, 0]);
    assert.equal(h.native.pending('measureInWindow', h.native.node(topic(h))).length, 0, 'zero viewport cannot reveal');
    await act(async () => scroll(h).props.onLayout());
    const badInput = await viewport(h); await h.native.resolve(badInput, [0, 77, 320, 194]);
    await h.native.resolve(latest(h, 'measureInWindow', h.native.node(topic(h))), [31, Infinity, 258, 72]);
    assert.equal(commands(h).length, 0, 'nonfinite native input coordinates cannot become a scroll command');
    await h.native.blur(topic(h)); assert.equal(clearance(h), 16, 'another input gets the default clearance');
    assert.equal(h.calls.some(call => call.options.method === 'POST'), false);
  });
});

test('TR07 oversized focused body converges after scroll feedback while fitting inputs retain upward reveal', async () => {
  await withComposer(async (h, vault, lease) => {
    const keep = retained(h), saved = vault.docNhapBai('synthetic-A', 'new:community');
    const body = h.input(), input = h.native.node(body), container = scroll(h), target = h.native.node(container);
    // Native rectangles are explicit fixtures. The second measurement translates
    // the same input by the exact scroll delta; no Yoga or caret layout is simulated.
    async function measureBody(inputTop, inputHeight) {
      await h.native.resolve(await viewport(h), [0, 77, 320, 194]);
      await h.native.resolve(latest(h, 'measureInWindow', input), [16, inputTop, 288, inputHeight]);
    }
    async function scrolled(y) {
      await act(async () => container.props.onScroll({ nativeEvent: { contentOffset: { y } } }));
    }
    await h.native.focus(body); await h.native.emitKeyboard('keyboardDidShow');
    assert.equal(clearance(h), 16, 'body uses the existing baseline clearance');
    await measureBody(93, 420);
    assert.equal(commands(h).length, 1);
    assert.equal(commands(h)[0].target, target.id, 'actual body scroll ancestor');
    assert.deepEqual({ ...commands(h)[0].options }, { y: 258, animated: false });
    await scrolled(258); await measureBody(-165, 420);
    assert.equal(commands(h).length, 1, 'bottom-aligned oversized input cannot trigger the opposite top correction');
    await scrolled(258); await measureBody(-165, 420);
    assert.equal(commands(h).length, 1, 'repeated native scroll feedback stays at the same settled offset');

    await h.native.emitKeyboard('keyboardDidHide');
    await scrolled(100); await h.native.emitKeyboard('keyboardDidShow');
    await measureBody(40, 72);
    assert.equal(commands(h).length, 2, 'a fitting input still receives the original upward correction');
    assert.equal(commands(h)[1].target, target.id);
    assert.deepEqual({ ...commands(h)[1].options }, { y: 47, animated: false });
    await scrolled(47); await measureBody(93, 72);
    assert.equal(commands(h).length, 2, 'fitting input settles at the original top clearance');

    // H163 fits inside raw V194 but cannot fit H + top16 + bottom16.
    await act(async () => container.props.onLayout());
    await measureBody(93, 163);
    assert.equal(commands(h).length, 3);
    assert.deepEqual({ ...commands(h)[2].options }, { y: 48, animated: false });
    await scrolled(48); await measureBody(92, 163);
    assert.equal(commands(h).length, 3, 'the clearance boundary also converges without a one-point reverse scroll');
    keep(); assert.equal(h.native.node(h.input()) === input, true, 'same focused native body handle');
    assert.equal(input.isFocused(), true, 'screen reveal does not replace native focus');
    assert.equal(vault.phienNhapBai('synthetic-A'), lease);
    assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), saved, 'reveal never edits the RAM draft');
    assert.equal(h.calls.some(call => call.options.method === 'POST'), false);
    assert.equal(h.picks.length, 0); assert.equal(h.uploads.length, 0);
  });
});
