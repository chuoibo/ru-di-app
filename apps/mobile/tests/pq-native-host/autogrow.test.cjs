// Exercise actual Field/Comments callbacks with independent native-line fixtures.
// These checks cover props and ownership, not Android Text/EditText wrap parity.
const assert = require('node:assert/strict');
const { test, after } = require('node:test');
const { createHost, snapshot, flatten, React, act } = require('./exact-host.cjs');
const previousAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const previousError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) previousError(...args); };
after(() => { console.error = previousError; if (previousAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = previousAct; });
const compact = { nativeMetrics: true, autoGrowLines: 3, numberOfLines: 1, style: { minHeight: 24, maxHeight: 72 } };
async function withHost(options, run) {
  const h = createHost(options);
  try { await h.mount(); await run(h); } finally { await h.dispose(); }
}
function style(h) { return flatten(h.input().props.style); }
function mirror(h) { return h.one('RNText', props => props.testID === `${h.input().props.testID}-measurement`); }
function lines(count, pitch = 24) { return Array.from({ length: count }, (_, i) => ({ x: 0, y: i * pitch, width: 101 + i, height: pitch })); }
async function width(h, value) { await act(async () => h.input().props.onLayout({ nativeEvent: { layout: { x: 0, y: 0, width: value, height: style(h).height } } })); }
async function measure(h, count, pitch = 24) { await act(async () => mirror(h).props.onTextLayout({ nativeEvent: { lines: lines(count, pitch) } })); }
function geometry(h, rows, pitch = 24) {
  assert.equal(style(h).height, rows * pitch, 'height follows accepted native rows and current pixel pitch');
  assert.equal(h.input().props.numberOfLines, rows, 'EditText setLines follows the same bounded row count');
  assert.equal(style(h).flex, 0, 'explicit height is not expanded by multiline flex');
}

test('AG01 native measured rows grow one two three, clamp four, and empty clears immediately', async () => {
  await withHost({ fieldProps: compact }, async h => {
    geometry(h, 1); await width(h, 258);
    await measure(h, 1); geometry(h, 1);
    await measure(h, 2); geometry(h, 2);
    await measure(h, 3); geometry(h, 3);
    await measure(h, 4); geometry(h, 3);
    await measure(h, 1); geometry(h, 1);
    // Adjacent pixel boxes [0,73], [73,145], [145,217] at density3,
    // serialized as RN's independent Float32 y/height divisions.
    await act(async () => mirror(h).props.onTextLayout({ nativeEvent: { lines: [
      { x: 0, y: 0, width: 101, height: Math.fround(73 / 3) },
      { x: 0, y: Math.fround(73 / 3), width: 102, height: 24 },
      { x: 0, y: Math.fround(145 / 3), width: 103, height: 24 },
    ] } })); geometry(h, 3);
    await h.change('Synthetic first\nSynthetic second\n');
    geometry(h, 3); // Last accepted count is provisional until this native revision arrives.
    await measure(h, 3); geometry(h, 3);
    await h.change(''); geometry(h, 1);
    assert.equal(h.hosts('RNText', props => String(props.testID).endsWith('-measurement')).length, 0);
    await h.change('Synthetic fresh'); geometry(h, 1, 24);
  });
});

test('AG02 hot metrics preserve input ref body focus handlers while measured rows use current pitch', async () => {
  let focus = 0, blur = 0;
  await withHost({ fieldProps: { ...compact, onFocus: () => { focus += 1; }, onBlur: () => { blur += 1; } } }, async h => {
    await h.focus(); await h.change('Synthetic focused multiline'); await width(h, 258); await measure(h, 2);
    geometry(h, 2); const input = h.input(), ref = h.refs[0], oldMirror = mirror(h);
    await h.nativeFont(snapshot(2)); geometry(h, 2, 39.5);
    assert.equal(mirror(h) === oldMirror, false, 'only the measuring leaf is replaced for fresh native metrics');
    await measure(h, 3, 39.5); geometry(h, 3, 39.5);
    await h.nativeFont(snapshot(1)); geometry(h, 3); await measure(h, 1); geometry(h, 1);
    assert.equal(h.input() === input, true); assert.equal(h.refs.length, 1); assert.equal(h.refs[0] === ref, true);
    assert.equal(h.input().props.value, 'Synthetic focused multiline'); assert.equal(focus, 1); assert.equal(blur, 0);
    assert.equal(Object.hasOwn(h.input().props, 'autoGrowLines'), false, 'kit opt-in never reaches the native input');
  });
});

test('AG03 committed body width and font revisions reject stale native callbacks', async () => {
  let layouts = 0;
  await withHost({ fieldProps: { ...compact, onLayout: () => { layouts += 1; } } }, async h => {
    await width(h, 258); await measure(h, 1);
    const bodyCallback = mirror(h).props.onTextLayout;
    await h.change('Synthetic next body');
    await act(async () => bodyCallback({ nativeEvent: { lines: lines(3) } })); geometry(h, 1);
    const widthCallback = mirror(h).props.onTextLayout, priorLayout = h.input().props.onLayout;
    await width(h, 199);
    await act(async () => widthCallback({ nativeEvent: { lines: lines(3) } })); geometry(h, 1);
    await act(async () => priorLayout({ nativeEvent: { layout: { width: 75 } } }));
    assert.equal(flatten(mirror(h).props.style).width, 199, 'stale layout cannot change committed content width');
    assert.equal(layouts, 3, 'caller receives layout notifications even when internal measurement is stale');
    const fontCallback = mirror(h).props.onTextLayout;
    await h.nativeFont(snapshot(2));
    await act(async () => fontCallback({ nativeEvent: { lines: lines(3) } })); geometry(h, 1, 39.5);
    await measure(h, 1, 39.5); geometry(h, 1, 39.5);
    const retired = mirror(h).props.onTextLayout;
    await h.update(() => { h.state.show = false; });
    const drafts = h.drafts.length, calls = h.calls.length, refs = h.refs.length;
    await act(async () => retired({ nativeEvent: { lines: lines(3) } }));
    assert.equal(h.drafts.length, drafts); assert.equal(h.calls.length, calls); assert.equal(h.refs.length, refs);
    assert.equal(h.hosts('RNTextInput').length, 0);
  });
});

test('AG04 invalid width and native line geometry cannot replace accepted rows', async () => {
  await withHost({ fieldProps: compact }, async h => {
    for (const bad of [NaN, Infinity, -1, 0]) await width(h, bad);
    assert.equal(h.hosts('RNText', props => String(props.testID).endsWith('-measurement')).length, 0);
    await width(h, 258); const leaf = mirror(h); await measure(h, 1);
    const invalid = [null, [], [null], [{ x: 0, y: 0, width: 10, height: 0 }],
      [{ x: NaN, y: 0, width: 10, height: 24 }], [{ x: 0, y: -1, width: 10, height: 24 }],
      [{ x: 0, y: 0, width: -1, height: 24 }], [{ x: 0, y: 0, width: 10, height: Infinity }],
      [{ x: 0, y: 0, width: 10, height: 24 }, { x: 0, y: 12, width: 10, height: 24 }]];
    for (const value of invalid) {
      await act(async () => mirror(h).props.onTextLayout({ nativeEvent: { lines: value } })); geometry(h, 1);
      assert.equal(mirror(h) === leaf, true, 'invalid geometry causes no new measuring revision');
    }
    for (const bad of [NaN, Infinity, -1, 0]) await width(h, bad);
    assert.equal(flatten(mirror(h).props.style).width, 258); geometry(h, 1);
  });
});

test('AG05 mirror uses effective input content box and inaccessible noninteractive native text', async () => {
  await withHost({ initialSnapshot: snapshot(2), fieldProps: { ...compact, style: { minHeight: 24, maxHeight: 96, paddingHorizontal: 4, paddingVertical: 2, borderWidth: 1, includeFontPadding: false, textAlign: 'right', writingDirection: 'rtl', fontWeight: '600' } } }, async h => {
    await width(h, 200); const props = mirror(h).props, measured = flatten(props.style), input = style(h);
    assert.equal(measured.width, 190); assert.equal(measured.left, 5); assert.equal(measured.padding, 0);
    for (const name of ['fontSize', 'lineHeight', 'letterSpacing', 'fontWeight', 'includeFontPadding', 'textAlign', 'writingDirection']) assert.equal(measured[name], input[name], `mirror matches input ${name}`);
    assert.equal(props.accessible, false); assert.equal(props.importantForAccessibility, 'no-hide-descendants'); assert.equal(props.accessibilityElementsHidden, true);
    assert.equal(props.pointerEvents, 'none'); assert.equal(props.selectable, false); assert.equal(props.allowFontScaling, false);
    assert.equal(props.accessibilityLabel, undefined); assert.equal(props.onPress, undefined); assert.equal(measured.opacity, 0); assert.equal(measured.position, 'absolute');
    assert.equal(props.textBreakStrategy, 'simple'); assert.equal(h.input().props.textBreakStrategy, 'simple');
    assert.equal(props.android_hyphenationFrequency, 'none'); assert.equal(props.numberOfLines, 3); assert.equal(props.ellipsizeMode, 'clip');
    assert.equal(props.children, h.input().props.value, 'only the current controlled body is measured in memory');
  });
});

test('AG06 default fallback caps and unsupported geometry retain existing Field path', async () => {
  const cases = [
    { props: { ...compact, nativeMetrics: false } }, { props: { ...compact, autoGrowLines: undefined } },
    { props: { ...compact, autoGrowLines: NaN } }, { props: { ...compact, autoGrowLines: 2.5 } },
    { props: { ...compact, autoGrowLines: 0 } }, { props: { ...compact, autoGrowLines: 9 } },
    { props: { ...compact, multiline: false } }, { props: { ...compact, maxFontSizeMultiplier: 1.25 } },
    { props: compact, native: { status: 'unavailable', reason: 'module-unavailable' } },
    { props: compact, native: { status: 'available', fontScale: 2 } },
    { props: { ...compact, style: { minHeight: 24, maxHeight: 72, height: 41 } }, height: 41 },
    { props: { ...compact, style: { minHeight: 24, maxHeight: 72, paddingStart: 4 } } },
    { props: { ...compact, style: { minHeight: 24, maxHeight: 72, paddingLeft: '5%' } } },
    { props: { ...compact, style: { minHeight: 24, maxHeight: 72, borderWidth: NaN } } },
  ];
  for (const entry of cases) await withHost({ initialSnapshot: entry.native ?? snapshot(2), fieldProps: entry.props }, async h => {
    assert.equal(style(h).height, entry.height); assert.equal(h.hosts('RNText', props => String(props.testID).endsWith('-measurement')).length, 0);
    assert.equal(h.input().props.numberOfLines, entry.props.numberOfLines);
  });
  await withHost({ initialSnapshot: snapshot(2), fieldProps: { ...compact, allowFontScaling: false } }, async h => {
    geometry(h, 1, 24); await width(h, 258); assert.equal(flatten(mirror(h).props.style).lineHeight, 24);
  });
  await withHost({ fieldProps: compact }, async h => {
    await width(h, 258); await measure(h, 1);
    await h.nativeFont({ status: 'unavailable', reason: 'module-unavailable' }); assert.equal(style(h).height, undefined);
    await width(h, 180);
    await h.nativeFont(snapshot(2)); geometry(h, 1, 39.5);
    assert.equal(flatten(mirror(h).props.style).width, 180, 'width updates during metrics fallback are retained for recovery');
  });
});

test('AG07 actual Comments footer grows without changing context journal or transport', async () => {
  const presentation = (thread, editor) => React.createElement('SyntheticSurface', null, thread, React.createElement('SyntheticFooter', null, editor));
  await withHost({ surface: 'comments', commentsProps: { presentation } }, async h => {
    await h.change('Synthetic comment multiline'); await h.focus(); await width(h, 258);
    const input = h.input(), owner = h.owner('CommentsForContext'), draft = h.drafts.at(-1), calls = h.calls.length;
    assert.equal(h.root.root.findByType('SyntheticFooter').findAllByType('RNTextInput').length, 1);
    await measure(h, 3); geometry(h, 3);
    await h.nativeFont(snapshot(2)); geometry(h, 3, 39.5); await measure(h, 3, 39.5); geometry(h, 3, 39.5);
    assert.equal(h.input() === input, true); assert.equal(h.owner('CommentsForContext') === owner, true);
    assert.deepEqual(h.drafts.at(-1), draft); assert.equal(h.calls.length, calls); assert.deepEqual(h.observation(), { live: 1, subscriptions: 1, removals: 0 });
  });
});

test('AG08 same width duplicate rows and same-scale changed pitch keep handlers and input host stable', async () => {
  const contentSize = () => {}, selection = () => {}, keyPress = () => {};
  await withHost({ fieldProps: { ...compact, onContentSizeChange: contentSize, onSelectionChange: selection, onKeyPress: keyPress } }, async h => {
    await width(h, 258); await measure(h, 1); const input = h.input(), leaf = mirror(h), ref = h.refs[0];
    await width(h, 258); await measure(h, 1); assert.equal(mirror(h) === leaf, true, 'same width and rows do not make a measurement feedback loop');
    await h.nativeFont(snapshot(1, 30.25)); geometry(h, 1, 30.5);
    assert.equal(mirror(h) === leaf, false, 'same-scale line metrics create a fresh measuring leaf');
    await measure(h, 1, 30.5); geometry(h, 1, 30.5);
    assert.equal(h.input() === input, true); assert.equal(h.refs.length, 1); assert.equal(h.refs[0] === ref, true);
    for (const [name, callback] of [['onContentSizeChange', contentSize], ['onSelectionChange', selection], ['onKeyPress', keyPress]]) assert.equal(h.input().props[name] === callback, true, `${name} remains caller-owned`);
  });
});
