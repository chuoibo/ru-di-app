// The native event source changes while useWindowDimensions stays at 1.
const test = require('node:test');
const assert = require('node:assert/strict');
const { createHost } = require('./host.cjs');
function flatten(value) { return Array.isArray(value) ? Object.assign({}, ...value.filter(Boolean).map(flatten)) : value ?? {}; }
function toolbar(h) { let node = h.tool('Thêm ảnh hoặc video'); while (node && node.type !== 'RNView') node = node.parent; assert.ok(node); return flatten(node.props.style); }
test('PF01 real native font event stacks Composer tools without replacing input or draft', async () => {
  const h = createHost({ realMedia: true }); await h.mount();
  try {
    await h.changeBody('Synthetic font event draft'); const input = h.body(), vault = h.load('apps/mobile/src/rudi/community/ban-nhap');
    const before = vault.docNhapBai('synthetic-A', 'new:community'); assert.equal(toolbar(h).flexDirection, 'row');
    await h.nativeFont({ status: 'available', fontScale: 2 });
    assert.equal(toolbar(h).flexDirection, 'column', 'native event must overcome stale window fontScale=1');
    assert.equal(h.body() === input, true, 'TextInput remains mounted'); assert.equal(h.body().props.value, before.body);
    assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), before);
    await h.nativeFont({ status: 'available', fontScale: 1 }); assert.equal(toolbar(h).flexDirection, 'row'); assert.equal(h.body() === input, true);
  } finally { await h.dispose(); }
});
test('PF02 unavailable native observer falls back to window geometry without inventing a scale', async () => {
  const h = createHost({ realMedia: true }); h.state.nativeFont = { status: 'unavailable', reason: 'observer-unavailable' }; await h.mount();
  try { await h.changeBody('Synthetic fallback'); assert.equal(toolbar(h).flexDirection, 'row'); assert.equal(h.body().props.value, 'Synthetic fallback'); }
  finally { await h.dispose(); }
});
