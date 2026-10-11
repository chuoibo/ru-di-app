// Actual community media transport, with only fetch/blob/native identity edges.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const { repo, sourceRoot } = require('./repository.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const ts = dependency('typescript');
const network = 'Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.';
function harness(steps) {
  const calls = [], closed = [], body = { close: () => closed.push('body') };
  const blob = { size: 12, slice: (...args) => { calls.push(['slice', ...args]); return body; }, close: () => closed.push('blob') };
  const file = { blob: async () => blob };
  const adapters = { '../../api': { BASE_URL: 'http://synthetic.invalid', LOI_KHONG_NOI_DUOC: network, newAttempt: () => ({ key: 'synthetic' }), translatedAsActor() { throw new Error('Unexpected translated call'); } }, '../../danh-tinh': { headerNguoiGoi: () => ({ Authorization: 'synthetic' }) }, '../ngay-viet': { ngayVN: () => 'synthetic' } };
  const input = path.join(sourceRoot, 'apps/mobile/src/rudi/community/api.ts');
  const result = ts.transpileModule(fs.readFileSync(input, 'utf8'), { fileName: input, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS }, reportDiagnostics: true });
  assert.deepEqual((result.diagnostics ?? []).filter(d => d.category === ts.DiagnosticCategory.Error).map(d => ({ code: d.code, start: d.start, message: ts.flattenDiagnosticMessageText(d.messageText, '\n') })), []);
  const module = { exports: {} };
  const helperPath = path.join(sourceRoot, 'apps/mobile/src/rudi/community/loi-tep.ts');
  if (fs.existsSync(helperPath)) { const helper = { exports: {} }; const output = ts.transpileModule(fs.readFileSync(helperPath, 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }); new vm.Script(`(function(exports){${output.outputText}\n})`, { filename: helperPath }).runInThisContext()(helper.exports); adapters['./loi-tep'] = helper.exports; }
  const fetch = async (...args) => { calls.push(['fetch', ...args]); assert.ok(steps.length, 'every fetch is declared'); const next = steps.shift(); if (next instanceof Error) throw next; return next === 'file' ? file : next; };
  new vm.Script(`(function(require,module,exports,fetch){${result.outputText}\n})`, { filename: input }).runInThisContext()(name => { assert.ok(Object.hasOwn(adapters, name), name); return adapters[name]; }, module, module.exports, fetch);
  return { api: module.exports, calls, closed, blob, file, body, readable: adapters['./loi-tep'] };
}
test('MT01 native network rejection becomes a human sentence and releases both blob references', async () => {
  const h = harness(['file', new TypeError('fetch failed: java.net.ConnectException: Failed to connect to localhost/127.0.0.1:53559')]);
  await assert.rejects(h.api.uploadMedia('synthetic-A', 'file:///synthetic.jpg', 'image/jpeg'), error => { assert.equal(error.message, network); assert.equal(error instanceof h.readable.LoiTep, true, 'UI can distinguish app-authored copy from native diagnostics'); return true; });
  assert.deepEqual(h.closed, ['body', 'blob']);
});
test('MT02 local-file diagnostic is preserved at transport boundary rather than mislabeled as network', async () => {
  const h = harness([new Error('java.io.FileNotFoundException: /private/synthetic.jpg')]);
  await assert.rejects(h.api.uploadMedia('synthetic-A', 'file:///synthetic.jpg', 'image/jpeg'), error => { assert.equal(error.message, 'java.io.FileNotFoundException: /private/synthetic.jpg'); return true; });
  assert.equal(h.calls.length, 1);
});
test('MT03 malformed success response is explained and keeps cleanup balanced', async () => {
  const h = harness(['file', { ok: true, json: async () => { throw new SyntaxError('Unexpected token internal hostname'); } }]);
  await assert.rejects(h.api.uploadMedia('synthetic-A', 'file:///synthetic.jpg', 'image/jpeg'), error => { assert.equal(error.message, 'Chưa đọc được phản hồi tải tệp. Thử lại nhé.'); return true; });
  assert.deepEqual(h.closed, ['body', 'blob']);
});
test('MT04 known server refusal retains its specific actionable sentence', async () => {
  const h = harness(['file', { ok: false, json: async () => ({ code: 'media_too_large' }) }]);
  await assert.rejects(h.api.uploadMedia('synthetic-A', 'file:///synthetic.jpg', 'image/jpeg'), /Ảnh tối đa 12 MB, video tối đa 64 MB/);
  assert.deepEqual(h.closed, ['body', 'blob']);
});
test('MT05 ready media retains native body MIME and success metadata', async () => {
  const ready = { id: 'synthetic-ready', type: 'image/jpeg', url: '/synthetic-ready', state: 'ready' };
  const h = harness(['file', { ok: true, json: async () => ready }]);
  assert.deepEqual(await h.api.uploadMedia('synthetic-A', 'file:///synthetic.jpg', 'image/jpeg'), ready);
  assert.deepEqual(h.calls[1], ['slice', 0, 12, 'image/jpeg']);
  assert.equal(h.calls[2][2].body, h.body); assert.equal(h.calls[2][2].headers['Content-Type'], 'image/jpeg');
  assert.deepEqual(h.closed, ['body', 'blob']);
});
