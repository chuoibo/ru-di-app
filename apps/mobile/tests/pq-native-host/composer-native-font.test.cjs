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
    const audience = h.tool('Người đọc: Cộng đồng');
    const scope = audience.findAllByType('RNText').find(node => node.props.children === 'Cộng đồng');
    assert.ok(scope);
    let scopeRow = scope.parent;
    while (scopeRow && scopeRow.type !== 'RNView') scopeRow = scopeRow.parent;
    assert.ok(scopeRow);
    const toolLabels = ['Thêm ảnh hoặc video', 'Chủ đề', 'Tag bạn bè'];
    const tools = toolLabels.map(label => h.tool(label));
    const captions = tools.map(tool => tool.findAllByType('RNText')[0]);
    const checkLayout = stacked => {
      assert.equal(h.tool('Người đọc: Cộng đồng') === audience, true, 'same audience target through hot font');
      assert.equal(audience.findAllByType('RNText').find(node => node.props.children === 'Cộng đồng') === scope, true);
      let currentRow = scope.parent;
      while (currentRow && currentRow.type !== 'RNView') currentRow = currentRow.parent;
      assert.equal(currentRow === scopeRow, true, 'same audience row');
      assert.equal(flatten(scope.props.style).flexShrink, 1); assert.equal(flatten(scope.props.style).minWidth, 0);
      assert.deepEqual(scopeRow.findAllByType('RudiIcon').map(node => node.props.name), ['globe-outline', 'chevron-down'], 'scope and its disclosure icon belong together');
      assert.deepEqual(captions.map(node => node.props.children), ['Ảnh/video', 'Chủ đề', 'Gắn bạn'], 'full toolbar captions');
      for (let index = 0; index < tools.length; index += 1) {
        const tool = h.tool(toolLabels[index]);
        assert.equal(tool === tools[index], true, 'same native tool target through hot font');
        assert.equal(tool.findAllByType('RNText')[0] === captions[index], true, 'same caption host');
        const style = flatten(typeof tool.props.style === 'function' ? tool.props.style({ pressed: false }) : tool.props.style);
        assert.equal(style.flexDirection, 'row'); assert.equal(style.minHeight, 48); assert.equal(style.minWidth, 48);
        assert.equal(style.flex, stacked ? 0 : 1); assert.equal(style.gap, stacked ? 12 : 6);
        assert.equal(style.paddingHorizontal, stacked ? 4 : undefined, 'normal row does not spend width on extra padding');
        assert.equal(tool.findAllByType('RudiIcon')[0].props.size, stacked ? 21 : 18);
        assert.equal(flatten(captions[index].props.style).flexShrink, 1); assert.equal(flatten(captions[index].props.style).minWidth, 0);
        assert.equal(captions[index].props.numberOfLines, undefined, 'long labels may wrap rather than lose meaning');
        assert.equal(captions[index].props.ellipsizeMode, undefined);
      }
    };
    checkLayout(false);
    await h.nativeFont({ status: 'available', fontScale: 2 });
    assert.equal(toolbar(h).flexDirection, 'column', 'native event must overcome stale window fontScale=1');
    checkLayout(true);
    assert.equal(h.body() === input, true, 'TextInput remains mounted'); assert.equal(h.body().props.value, before.body);
    assert.deepEqual(vault.docNhapBai('synthetic-A', 'new:community'), before);
    await h.nativeFont({ status: 'available', fontScale: 1 }); assert.equal(toolbar(h).flexDirection, 'row'); assert.equal(h.body() === input, true);
    checkLayout(false);
  } finally { await h.dispose(); }
});
test('PF02 unavailable native observer falls back to window geometry without inventing a scale', async () => {
  const h = createHost({ realMedia: true }); h.state.nativeFont = { status: 'unavailable', reason: 'observer-unavailable' }; await h.mount();
  try { await h.changeBody('Synthetic fallback'); assert.equal(toolbar(h).flexDirection, 'row'); assert.equal(h.body().props.value, 'Synthetic fallback'); }
  finally { await h.dispose(); }
});
