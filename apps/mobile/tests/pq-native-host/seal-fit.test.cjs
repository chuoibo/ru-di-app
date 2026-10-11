// Native line/layout callbacks are synthetic inputs; successful widths below are not device findings.
const assert = require('node:assert/strict');
const { test, after } = require('node:test');
const oldAct = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const { withSealHost, closeSealHost, snapshot, flatten } = require('./seal-fit-host.cjs');
after(async () => { await closeSealHost(); if (oldAct === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldAct; });
const statusAdvance = Math.fround(30.827345);
const amendmentAdvance = Math.fround(96.95235);
function request(h) { const fit = h.fit(); assert.ok(fit, 'a native fitted request is pending'); assert.equal(fit.props.numberOfLines, undefined, 'probe never truncates to manufacture one line'); return flatten(fit.props.style).width; }
async function upper(h, advance, cap) {
  assert.equal(request(h), cap, 'verify full finite upper first');
  await h.lines(h.fit(), [advance]);
  assert.equal(h.width(), undefined, 'verified upper is internal until the final width settles');
}
async function completeByCallbacks(h, threshold, advance) {
  let callbacks = 0;
  while (h.fit()) {
    assert.ok(callbacks < 12, 'bounded native fit requests');
    const width = request(h);
    await h.lines(h.fit(), width >= threshold ? [advance] : [advance / 2, advance / 2]);
    callbacks += 1;
    if (h.fit()) assert.equal(h.width(), undefined, 'no tentative candidate or midpoint is published');
  }
  return callbacks;
}

test('SF01 actual status rejects captured31 wrap, accepts explicit next-pixel feedback and ignores ABA/retired attempts', async () => {
  await withSealHost({}, async h => {
    await h.layout(230); await h.change('Synthetic adjacent draft A');
    const before = { text: h.visible(), owner: h.root.findByType(h.Component), seal: h.seal(), input: h.input(), parent: h.raw.byId('sf-parent')[0] };
    const old100Wide = h.wide().props.onTextLayout;
    await h.lines(h.wide(), [statusAdvance]);
    assert.equal(h.width(), undefined, 'wide advance alone cannot certify explicit31 width');
    const old100Upper = h.fit().props.onTextLayout;
    await upper(h, statusAdvance, 210);
    assert.equal(request(h), 31, 'captured100 rounded candidate');
    const old100Candidate = h.fit().props.onTextLayout;
    await h.lines(h.fit(), [Math.fround(17.279688), Math.fround(12.747658)]);
    assert.equal(h.width(), undefined, 'actual captured two-line input cannot publish failed31');
    assert.equal(request(h), 32, 'request the next physical pixel, not guessed padding');
    // One-line success at32 is supplied by this host fixture, not observed on Android.
    await h.lines(h.fit(), [statusAdvance]);
    assert.equal(h.width(), 32); assert.equal(h.fit(), null);
    await h.lines(old100Upper, [statusAdvance]);
    assert.equal(h.width(), 32, 'retired upper attempt cannot reopen a settled search');
    await h.lines(old100Candidate, [statusAdvance]);
    assert.equal(h.width(), 32, 'retired failed candidate cannot shrink the certified width');
    await h.lines(old100Wide, [1000]); assert.equal(h.width(), 32, 'duplicate wide callback is ignored');
    for (const [next, advance, rounded] of [[snapshot(2), Math.fround(61.4), 62], [snapshot(1), statusAdvance, 31]]) {
      await h.update(next); assert.equal(h.width(), undefined, 'new committed metrics retire the previous certificate');
      await h.lines(old100Wide, [statusAdvance]); assert.equal(h.width(), undefined, 'old100 callback cannot revive after100→200→100');
      await h.lines(h.wide(), [advance]); await upper(h, advance, 210); assert.equal(request(h), rounded);
      if (rounded === 31) {
        await h.lines(old100Candidate, [statusAdvance]); assert.equal(h.width(), undefined); assert.equal(request(h), 31, 'ABA callback cannot accept the fresh31 attempt');
        await h.lines(h.fit(), [Math.fround(17.279688), Math.fround(12.747658)]); assert.equal(request(h), 32);
      }
      await h.lines(h.fit(), [advance]); assert.equal(h.width(), rounded === 31 ? 32 : 62);
    }
    await h.lines(old100Wide, [statusAdvance]); assert.equal(h.width(), 32, 'retired same-key wide callback cannot erase fresh returned100 geometry');
    assert.equal(h.visible() === before.text, true, 'status native label stays mounted across fit and hot metrics');
    assert.equal(h.root.findByType(h.Component) === before.owner, true); assert.equal(h.seal() === before.seal, true);
    assert.equal(h.input() === before.input, true); assert.equal(h.raw.byId('sf-parent')[0] === before.parent, true);
    assert.equal(h.input().props.value, 'Synthetic adjacent draft A'); assert.deepEqual(h.raw.events, []);
  });
});

test('SF02 actual amendment verifies97/next-pixel, handles malformed/capped events and keeps button/draft ownership', async () => {
  await withSealHost({ kind: 'button' }, async h => {
    await h.layout(253); await h.change('Synthetic adjacent draft B');
    const before = { text: h.visible(), owner: h.root.findByType(h.Component), press: h.root.findByType(h.PressScale), button: h.seal(), input: h.input() };
    await h.lines(h.wide(), [amendmentAdvance]); assert.equal(h.width(), undefined, 'wide advance alone cannot certify explicit97');
    const retiredUpper = h.fit().props.onTextLayout;
    await upper(h, amendmentAdvance, 205); assert.equal(request(h), 97);
    await h.lines(h.fit(), [Math.fround(58.708588), Math.fround(37.843746)]); assert.equal(request(h), 98); assert.equal(h.width(), undefined);
    // Success at98 is an explicit synthetic native callback, not a native width claim.
    await h.lines(h.fit(), [amendmentAdvance]); assert.equal(h.width(), 98);
    await h.lines(retiredUpper, [amendmentAdvance]); assert.equal(h.width(), 98, 'retired upper cannot restart a completed button fit');
    assert.equal(h.visible() === before.text, true, 'same-metrics width feedback keeps the native button label');
    assert.equal(h.root.findByType(h.Component) === before.owner, true); assert.equal(h.root.findByType(h.PressScale) === before.press, true);
    assert.equal(h.seal() === before.button, true); assert.equal(h.input() === before.input, true); assert.equal(h.input().props.value, 'Synthetic adjacent draft B');
    // A large but bounded native-column fixture forces the12-callback ceiling.
    await h.layout(65584); await h.lines(h.wide(), [amendmentAdvance]);
    await upper(h, amendmentAdvance, 65536);
    const count = await completeByCallbacks(h, 65536, amendmentAdvance);
    assert.equal(count, 11, 'upper plus11 subsequent callbacks reaches the hard12 bound');
    assert.equal(h.width(), 65536, 'cap retains an explicitly verified finite upper, not the last failed midpoint');
    // A larger search range is terminal after verifying its finite upper.
    await h.layout(70048); await h.lines(h.wide(), [amendmentAdvance]); assert.equal(request(h), 70000);
    await h.lines(h.fit(), [amendmentAdvance]); assert.equal(h.width(), 70000); assert.equal(h.fit(), null);
    const malformed = [[], [null], [undefined], [{ width: 0 }], [{ width: NaN }], [{ width: Infinity }], [{ width: -1 }], [{ width: 206 }]];
    for (let i = 0; i < malformed.length; i++) {
      await h.layout(253 + i); await h.lines(h.wide(), [amendmentAdvance]); const cap = 205 + i;
      await upper(h, amendmentAdvance, cap); assert.equal(request(h), 97);
      await h.lines(h.fit(), malformed[i]); assert.equal(h.fit(), null, 'malformed/overflow fit terminates');
      assert.equal(h.width(), cap, 'malformed candidate retains verified full-column upper');
    }
    await h.layout(280); await h.lines(h.wide(), [amendmentAdvance]); const callback = h.fit().props.onTextLayout;
    await h.unmountSurface(); await h.lines(callback, [amendmentAdvance]); assert.equal(h.raw.all('RNText').length, 0);
    assert.deepEqual(h.raw.events, [], 'fit probes perform no transport/storage/navigation actions');
  });
});
