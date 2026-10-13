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

test('TR08 actual counter layout reserves the body tail without restarting settled reveal or accepting retired geometry', async () => {
  await withComposer(async (h, vault, lease) => {
    const keep = retained(h), saved = vault.docNhapBai('synthetic-A', 'new:community');
    const body = h.input(), input = h.native.node(body), container = scroll(h), target = h.native.node(container);
    const beforeIO = { calls: h.calls.length, picks: h.picks.length, uploads: h.uploads.length, routes: h.routes.length, attempts: h.attemptCount() };
    const counter = () => {
      const values = h.text(props => h.React.Children.toArray(props.children).join('') === `${bodyValue.length}/5000`);
      assert.equal(values.length, 1, 'one complete actual counter');
      return values[0];
    };
    async function counterLayout(height, callback = counter().props.onLayout, width = 288) {
      assert.equal(typeof callback, 'function', 'actual counter forwards its passive native layout callback');
      await act(async () => callback({ nativeEvent: { layout: { x: 16, y: 305, width, height } } }));
    }
    async function measureBody(top, height, viewportHeight) {
      await h.native.resolve(await viewport(h), [0, 77, 320, viewportHeight]);
      await h.native.resolve(latest(h, 'measureInWindow', input), [16, top, 288, height]);
    }
    async function scrolled(y) { await act(async () => container.props.onScroll({ nativeEvent: { contentOffset: { y } } })); }
    await h.nativeFont(font(1));
    assert.equal(h.state.window.fontScale, 2, 'native100 is observed while the window font remains stale at200');
    const counterHost = counter(), at100 = counterHost.props.onLayout;
    await h.native.focus(body); await h.native.emitKeyboard('keyboardDidShow');
    // These are explicit rectangles from the clipped native scene, not a Yoga model.
    await measureBody(77, 224, 240);
    assert.equal(commands(h).length, 0, 'the original body rectangle alone already fits baseline clearance');
    await counterLayout(18, at100);
    assert.ok(h.native.pendingFrames() > 0, 'counter layout reruns the actual screen reveal through its inset effect');
    await measureBody(77, 224, 240);
    assert.equal(commands(h).length, 1);
    assert.equal(commands(h)[0].target, target.id, 'counter tail uses the actual body scroll ancestor');
    assert.deepEqual({ ...commands(h)[0].options }, { y: 22, animated: false }, 'the complete counter and existing breathing room move above the viewport edge');
    await scrolled(22); await measureBody(55, 224, 240);
    assert.equal(commands(h).length, 1, 'the oversized input stays settled after counter-aware feedback');
    const settled = h.nativeHostCalls.length;
    await counterLayout(18);
    for (const value of [0, -1, NaN, Infinity]) await counterLayout(value);
    await counterLayout(18, counter().props.onLayout, 0);
    await counterLayout(18, counter().props.onLayout, NaN);
    assert.equal(h.native.pendingFrames(), 0, 'duplicate or invalid counter boxes cannot start a measurement loop');
    assert.equal(h.nativeHostCalls.length, settled);

    await h.nativeFont(font(2));
    assert.equal(h.state.window.fontScale, 2, 'native font events do not rewrite the window fixture');
    await counterLayout(18, at100);
    assert.equal(h.native.pendingFrames(), 0, 'retired100 counter callback cannot restart screen reveal');
    assert.equal(h.nativeHostCalls.length, settled);
    const at200 = counter().props.onLayout;
    await counterLayout(36, at200);
    await measureBody(55, 218, 234);
    assert.equal(commands(h).length, 2);
    assert.deepEqual({ ...commands(h)[1].options }, { y: 40, animated: false }, 'the larger actual counter is reserved without a scalar font guess');
    await scrolled(40); await measureBody(37, 218, 234);
    assert.equal(commands(h).length, 2);
    const settled200 = h.nativeHostCalls.length;
    await counterLayout(18, at100);
    assert.equal(h.native.pendingFrames(), 0, 'stale100 counter cannot replace the committed200 tail');
    assert.equal(h.nativeHostCalls.length, settled200);

    await h.nativeFont(font(1));
    await counterLayout(36, at200);
    assert.equal(h.native.pendingFrames(), 0, 'retired200 geometry cannot cross the100→200→100 epoch');
    const returned = counter().props.onLayout;
    await counterLayout(18, returned); await measureBody(37, 218, 234);
    assert.equal(commands(h).length, 2, 'a shorter tail does not reverse an already revealed oversized input');
    await counterLayout(36, at200);
    assert.equal(h.native.pendingFrames(), 0, 'retired200 geometry cannot replace the returned100 tail');
    await counterLayout(18, at100);
    assert.equal(h.native.pendingFrames(), 0, 'the first100 callback cannot restart the settled100 screen');

    await h.nativeFont({ status: 'unavailable', reason: 'module-inactive' });
    assert.notEqual(counter().props.allowFontScaling, false, 'unavailable metrics retain the original native scaling path');
    await counterLayout(18, returned);
    assert.equal(h.native.pendingFrames(), 0);
    // A fractional fallback box proves that actual layout, not18×fontScale, owns the tail.
    await counterLayout(42.5); await measureBody(37, 218, 234);
    assert.equal(commands(h).length, 3);
    assert.deepEqual({ ...commands(h)[2].options }, { y: 46.5, animated: false });
    await scrolled(46.5); await measureBody(30.5, 218, 234);
    assert.equal(commands(h).length, 3);
    await counterLayout(18, returned);
    assert.equal(h.native.pendingFrames(), 0, 'available-font geometry cannot replace the measured fallback tail');
    assert.equal(counter() === counterHost, true, 'the same native counter stays mounted through all metric paths');
    assert.equal(h.native.node(h.input()) === input, true);
    assert.equal(input.isFocused(), true);

    await h.native.focus(topic(h));
    assert.equal(clearance(h), 16, 'a pending topic still uses its own baseline, not the remembered body tail');
    await counterLayout(44);
    assert.equal(clearance(h), 16);
    await tail(h); assert.equal(clearance(h), 85, 'the actual helper/error tail still owns topic clearance');
    await finishReveal(h, await viewport(h));
    const topicSettled = h.nativeHostCalls.length;
    await counterLayout(45);
    assert.equal(clearance(h), 85, 'counter growth cannot replace a committed topic helper tail');
    assert.equal(h.native.pendingFrames(), 0); assert.equal(h.nativeHostCalls.length, topicSettled);
    keep();
    assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), saved);
    assert.equal(vault.phienNhapBai('synthetic-A'), lease);
    assert.deepEqual({ calls: h.calls.length, picks: h.picks.length, uploads: h.uploads.length, routes: h.routes.length, attempts: h.attemptCount() }, beforeIO, 'layout and fonts do not perform product I/O or replace the attempt');

    await h.native.blur(topic(h)); await h.native.emitKeyboard('keyboardDidHide'); await frames(h);
    await h.native.focus(body);
    const retired = counter().props.onLayout, beforeRetirement = h.nativeHostCalls.length, bodyTail = clearance(h);
    assert.ok(bodyTail > 16, 'the remembered actual counter owns body clearance again');
    vault.datChuSoHuuNhapBai('synthetic-retired'); vault.datChuSoHuuNhapBai('synthetic-A');
    await counterLayout(100, retired);
    assert.equal(clearance(h), bodyTail, 'retired lease geometry cannot change the body tail'); assert.equal(h.native.pendingFrames(), 0);
    assert.equal(h.nativeHostCalls.length, beforeRetirement, 'a retired draft lease cannot admit counter geometry');
    await h.update(state => { state.show = false; });
    await counterLayout(101, retired);
    assert.equal(h.nativeHostCalls.length, beforeRetirement, 'the retained callback cannot revive an unmounted screen');
    assert.equal(h.native.pendingFrames(), 0); assert.equal(h.observation().live, 0);
    assert.equal(h.observation().removals, h.observation().subscriptions);
    assert.equal(vault.docNhapBai('synthetic-A', 'new:community'), null, 'a retired counter callback cannot recreate the old draft');
  });
});

test('TR09 actual IME viewport bounds the retained body for first middle and end edits while full native lines and closed canvas remain safe', async () => {
  await withComposer(async (h, vault, lease) => {
    const long = 'QC native long draft. The group walks around the lake and keeps a memory. '.repeat(24);
    assert.equal(long.length, 1776);
    await h.nativeFont(font(1)); await h.change(long);
    const body = h.input(), input = h.native.node(body), container = scroll(h), target = h.native.node(container);
    const owners = ['ComposerForm', 'MediaPicker', 'MentionPicker'].map(name => [name, h.owner(name)]);
    const fieldHost = field(h), topicHost = topic(h), toolsHost = tools(h), observed = ownership(h);
    const style = () => h.Native.StyleSheet.flatten(h.input().props.style);
    const counter = () => {
      const values = h.text(props => h.React.Children.toArray(props.children).join('') === `${h.input().props.value.length}/5000`);
      assert.equal(values.length, 1, 'one actual body counter'); return values[0];
    };
    const counterLayout = async height => { await act(async () => counter().props.onLayout({ nativeEvent: { layout: { x: 16, y: 0, width: 288, height } } })); };
    const viewportLayout = async (height, callback = container.props.onLayout, width = h.state.window.width) => { await act(async () => callback({ nativeEvent: { layout: { x: 0, y: 77, width, height } } })); };
    const scrolled = async y => { await act(async () => container.props.onScroll({ nativeEvent: { contentOffset: { y } } })); };
    async function measureBody(top, inputHeight, viewportHeight) {
      await h.native.resolve(await viewport(h), [0, 77, h.state.window.width, viewportHeight]);
      await h.native.resolve(latest(h, 'measureInWindow', input), [16, top, 288, inputHeight]);
    }
    function keep(expected) {
      assert.equal(h.input() === body, true, 'viewport updates retain the actual native editor');
      assert.equal(h.native.node(h.input()) === input, true); assert.equal(input.isFocused(), true);
      assert.equal(h.input().props.value, expected); assert.equal(vault.docNhapBai('synthetic-A', 'new:community').body, expected);
      assert.equal(vault.phienNhapBai('synthetic-A'), lease);
      for (const [name, owner] of owners) assert.equal(h.owner(name) === owner, true, `same ${name}`);
      assert.equal(field(h) === fieldHost, true); assert.equal(topic(h) === topicHost, true); assert.equal(tools(h) === toolsHost, true);
      assert.deepEqual(ownership(h), observed, 'no new font subscription or owner churn');
      assert.equal(Object.hasOwn(h.input().props, 'bottomClearance'), false, 'scoped clearance is not a native input prop');
      assert.equal(h.input().props.maxLength, 5000); assert.equal(h.input().props.multiline, true);
    }
    await counterLayout(18);
    const noIme = { minHeight: style().minHeight, maxHeight: style().maxHeight };
    assert.equal(noIme.maxHeight, 640, 'closed phone keeps the incumbent full-window canvas ceiling');
    // Android may resize this actual scroll viewport before keyboardDidShow.
    await viewportLayout(240);
    assert.equal(style().maxHeight, noIme.maxHeight, 'a viewport layout alone cannot compact the closed canvas');
    await h.native.focus(body); await h.native.emitKeyboard('keyboardDidShow');
    assert.equal(style().maxHeight, 186, 'actual pre-show viewport reserves top16 and measured counter tail38');
    assert.equal(style().minHeight, noIme.minHeight, 'a fitting design minimum remains unchanged');
    assert.equal(style().maxHeight + 16 + clearance(h) <= 240, true, 'the whole declared body box now fits its real screen viewport');
    assert.equal(h.state.window.fontScale, 2, 'native100 is independent of stale window200');
    const at100 = container.props.onLayout;
    await scrolled(100); await measureBody(40, 186, 240);
    assert.equal(commands(h).length, 1, 'the actual fitting editor receives upward reveal instead of the oversized reverse guard');
    assert.equal(commands(h)[0].target, target.id); assert.deepEqual({ ...commands(h)[0].options }, { y: 47, animated: false });
    await scrolled(47); await measureBody(93, 186, 240);
    assert.equal(commands(h).length, 1, 'the top-revealed fitting editor settles without feedback oscillation');

    // These are controlled-text edits at the real user's three closed positions.
    // No caret rectangle, text-internal scrolling or native selection is simulated.
    for (const offset of [0, 888, 1776]) {
      const edited = long.slice(0, offset) + 'Z' + long.slice(offset);
      await h.change(edited); keep(edited);
      assert.equal(style().maxHeight + 16 + clearance(h) <= 240, true, `offset${offset} retains a fully revealable editor box`);
      const attempt = h.attemptCount();
      await act(async () => container.props.onContentSizeChange(288, 1777));
      await measureBody(93, 186, 240);
      assert.equal(commands(h).length, 1, 'a text edit does not reintroduce whole-input clipping');
      assert.equal(h.attemptCount(), attempt, 'layout feedback does not replace the outgoing attempt');
      await h.change(long); keep(long);
    }
    const productIO = { calls: h.calls.length, picks: h.picks.length, uploads: h.uploads.length, routes: h.routes.length, attempts: h.attemptCount() };
    const saved = vault.docNhapBai('synthetic-A', 'new:community');
    await h.nativeFont(font(2)); await counterLayout(36); await viewportLayout(234);
    assert.equal(style().maxHeight, 162, 'native200 uses the current viewport and actual36dp counter');
    const at200 = container.props.onLayout;
    await viewportLayout(600, at100);
    assert.equal(style().maxHeight, 162, 'retired100 viewport callback cannot replace committed200 geometry');
    await measureBody(93, 162, 234); assert.equal(commands(h).length, 1);
    await viewportLayout(100);
    assert.equal(style().minHeight, 44, 'a tight IME viewport clamps the old140.8dp design minimum down to one complete36dp native line plus8 padding');
    assert.equal(style().maxHeight, 44, 'the complete native line wins when the viewport cannot fit it with both clearances');
    await viewportLayout(234); await h.nativeFont(snapshot(2, 201.25));
    assert.equal(style().minHeight, 209.5); assert.equal(style().maxHeight, 209.5, 'extreme native line pitch is never clipped to an impossible viewport budget');
    await h.nativeFont(font(1)); await counterLayout(18); await viewportLayout(240);
    assert.equal(style().maxHeight, 186);
    await viewportLayout(700, at200); assert.equal(style().maxHeight, 186, 'old200 cannot cross the native100→200→100 epoch');
    const shortWindow = container.props.onLayout;
    await h.update(state => { state.window = { ...state.window, height: 800 }; });
    const tallWindow = container.props.onLayout;
    await viewportLayout(900, shortWindow); assert.equal(style().maxHeight, 186, 'old window-height callback cannot enlarge the actual IME budget');
    await viewportLayout(250); assert.equal(style().maxHeight, 196);
    await h.update(state => { state.window = { ...state.window, height: 640 }; });
    await viewportLayout(900, tallWindow); assert.equal(style().maxHeight, 196, 'window-height ABA rejects the retired middle epoch');
    await viewportLayout(240); assert.equal(style().maxHeight, 186);
    const beforeHide = container.props.onLayout;
    await h.native.emitKeyboard('keyboardDidHide');
    assert.equal(style().maxHeight, noIme.maxHeight); assert.equal(style().minHeight, noIme.minHeight, 'IME close restores original canvas constraints');
    await viewportLayout(600, beforeHide); assert.equal(style().maxHeight, noIme.maxHeight, 'old keyboard layouts cannot compact a closed editor');
    await viewportLayout(560); await h.native.emitKeyboard('keyboardDidShow');
    assert.equal(style().maxHeight, 506, 'pre-show actual layout is retained across the keyboard epoch');
    await viewportLayout(240); await viewportLayout(600, beforeHide);
    assert.equal(style().maxHeight, 186, 'old keyboard session cannot replace the new viewport');

    await h.native.emitKeyboard('keyboardDidHide'); const phoneLayout = container.props.onLayout;
    await h.update(state => { state.window = { width: 768, height: 1024, fontScale: 1, scale: 1 }; });
    assert.equal(style().maxHeight, undefined, 'closed wide canvas remains unbounded as before');
    const wideMinimum = style().minHeight;
    await viewportLayout(240, phoneLayout, 320); await h.native.emitKeyboard('keyboardDidShow');
    assert.equal(style().maxHeight, undefined, 'retired phone width cannot invent a tablet viewport');
    await viewportLayout(320); assert.equal(style().maxHeight, 266, 'the actual wide IME viewport also bounds its editor');
    for (const [height, width] of [[0, 768], [-1, 768], [NaN, 768], [Infinity, 768], [320, 0]]) {
      await viewportLayout(height, container.props.onLayout, width);
      assert.equal(style().maxHeight, undefined, 'invalid actual viewport returns the incumbent wide policy');
      assert.equal(Number.isFinite(style().minHeight), true);
    }
    await viewportLayout(320); await h.nativeFont({ status: 'unavailable', reason: 'module-inactive' });
    assert.notEqual(h.input().props.allowFontScaling, false, 'unavailable font metrics preserve native scaling');
    assert.equal(style().maxHeight, 266); assert.equal(style().minHeight, wideMinimum, 'unknown native pitch retains the incumbent safe minimum');
    await viewportLayout(100); assert.equal(style().maxHeight, wideMinimum, 'fallback never guesses a smaller complete-line minimum');
    await viewportLayout(320);
    const bodyCap = style().maxHeight;
    await h.native.focus(topic(h)); await tail(h);
    assert.equal(clearance(h), 85); assert.equal(style().maxHeight, bodyCap, 'topic helper clearance does not replace the measured body tail');
    await h.native.blur(topic(h)); await h.native.focus(body); keep(long);
    assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), saved, 'font and viewport changes retain the exact RAM draft');
    assert.deepEqual({ calls: h.calls.length, picks: h.picks.length, uploads: h.uploads.length, routes: h.routes.length, attempts: h.attemptCount() }, productIO, 'viewport and font updates admit no product I/O or logical attempts');
    const retiredLayout = container.props.onLayout;
    await h.update(state => { state.show = false; });
    const afterUnmount = h.nativeHostCalls.length;
    await viewportLayout(900, retiredLayout, 768); await frames(h);
    assert.equal(h.nativeHostCalls.length, afterUnmount, 'detached viewport cannot revive native measurements or scroll');
    assert.equal(h.native.pendingFrames(), 0); assert.equal(h.observation().live, 0);
    assert.equal(h.observation().removals, h.observation().subscriptions);
  });
});
