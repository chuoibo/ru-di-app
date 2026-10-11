const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const { randomUUID } = require('node:crypto');
const { repo, sourceRoot } = require('./repository.cjs');
const { flatten } = require('./native-metrics-boundary.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const React = dependency('react');
const Renderer = dependency('react-test-renderer');
const ts = dependency('typescript');
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const { act } = React;
const oldError = console.error;
console.error = (...args) => { if (!String(args[0]).includes('react-test-renderer is deprecated')) oldError(...args); };
function person(id = 'synthetic-A') { return { person_id: id, token: `synthetic-token-${id}`, profile: { display_name: id }, contexts: [], context_id: null }; }
function post(id = 'synthetic-post', revision = 3) { return { id, author_id: 'synthetic-A', body: 'Synthetic server body', audience: 'public', context_id: null, revision, topics: ['Server'], mentions: [], media: [] }; }
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
function createHost({ actor = person(), params = {}, candidateRoot = sourceRoot, mode = 'candidate', realMedia = false } = {}) {
  const state = { actor, params, token: null, revoke: null, show: true, tick: 0, nativeFont: { status: "available", fontScale: 1 } };
  const fontListeners = new Set();
  const cache = new Map(), consumed = new Map(), queue = new Map(), calls = [], routes = [];
  let root, session;
  const invoke = (kind, args, fallback) => { const call = { kind, args }; calls.push(call); const parked = queue.get(kind)?.shift(); if (parked) { parked.call = call; return parked.promise; } return Promise.resolve(typeof fallback === 'function' ? fallback() : fallback); };
  const visual = name => props => React.createElement(name, props, props.children);
  const Button = visual('RudiButton');
  const Screen = props => React.createElement('RudiScreen', props, props.header, props.children, props.footer);
  const colors = new Proxy({}, { get: () => '#123456' });
  const native = { View: 'RNView', Text: 'RNText', Pressable: 'RNPressable', TextInput: 'RNTextInput', ScrollView: 'RNScrollView', Keyboard: { dismiss() {} }, Platform: { OS: 'android', select: v => v.android ?? v.default }, StyleSheet: { create: v => v, flatten, hairlineWidth: 1 }, useWindowDimensions: () => ({ width: 390, height: 844, fontScale: 1, scale: 1 }) };
  const adapters = {
    'apps/mobile/src/api': { newAttempt: () => ({ key: randomUUID() }), translatedAsActor: (...args) => invoke('translatedAsActor', args, { id: 'synthetic-post' }), datTokenPhien: token => { state.token = token; }, tokenPhienHienTai: () => state.token },
    'apps/mobile/src/phien': { khoiPhucPhien: () => Promise.resolve(state.actor), dangXuat: (...args) => invoke('dangXuat', args), quenPhienDaThuHoi: (...args) => invoke('quenPhienDaThuHoi', args), docNhomCuaToi: (...args) => invoke('docNhomCuaToi', args, [{ id: 'synthetic-group', display_name: 'Synthetic group', kind: 'group', my_state: 'active' }]) },
    'apps/mobile/src/danh-tinh': { ngheThuHoiPhien: fn => { state.revoke = fn; return () => { state.revoke = null; }; } },
    'apps/mobile/src/rudi/kho': { xoaPhienAsync: async () => {} },
    'apps/mobile/src/rudi/theme': { typography: new Proxy({ body: { fontSize: 17, lineHeight: 24 } }, { get: (target, key) => target[key] ?? {} }), useRudiTheme: () => ({ colors }) },
    'apps/mobile/src/rudi/ui': { Field: visual('Field'), RudiButton: Button, RudiScreen: Screen, TopBar: visual('TopBar') },
    'apps/mobile/src/rudi/ui/CauTaiCho': { CauTaiCho: props => React.createElement('CauTaiCho', props, props.cau, props.cau && props.hanhDong ? React.createElement(Button, { label: props.hanhDong.label, onPress: props.hanhDong.onPress }) : null) },
    'apps/mobile/src/rudi/ui/Skeleton': { SkeletonLines: visual('SkeletonLines') },
    'apps/mobile/src/rudi/ui/AvatarNguoi': { AvatarNguoi: visual('AvatarNguoi') },
    'apps/mobile/src/rudi/ui/RudiIcon': { RudiIcon: visual('RudiIcon') },
    'apps/mobile/src/rudi/community/api': { COMMUNITY_ERRORS: { revision_conflict: 'Revision conflict' }, getPost: (...args) => invoke('getPost', args, () => post(args[1])), createPost: (...args) => invoke('createPost', args, { id: 'synthetic-post' }), uploadMedia: (...args) => invoke('uploadMedia', args, { id: 'synthetic-upload', type: 'image/jpeg', url: '/synthetic-upload', state: 'ready' }), imageSource: (_person, uri) => ({ uri }) },
    'apps/mobile/src/rudi/community/MediaPicker': { MediaPicker: props => React.createElement('MediaPicker', props, props.renderToolbar?.(() => {}, false)) },
    'apps/mobile/src/rudi/community/MentionPicker': { MentionPicker: visual('MentionPicker') },
  };
  function load(relative) {
    relative = relative.replace(/\\/g, '/').replace(/\.(tsx?|js)$/, '');
    if (adapters[relative] && !(realMedia && relative === 'apps/mobile/src/rudi/community/MediaPicker')) return adapters[relative];
    if (cache.has(relative)) return cache.get(relative).exports;
    const variants = ['.ts', '.tsx', '/index.ts', '/index.tsx'];
    const bases = [candidateRoot, repo];
    const file = bases.flatMap(base => variants.map(ext => path.join(base, relative + ext))).find(fs.existsSync);
    assert.ok(file, `No actual source ${relative}`);
    const bytes = fs.readFileSync(file, 'utf8');
    consumed.set(relative, file);
    const result = ts.transpileModule(bytes, { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true }, reportDiagnostics: true });
    assert.deepEqual((result.diagnostics ?? []).filter(d => d.category === ts.DiagnosticCategory.Error).map(d => ({ code: d.code, start: d.start, message: ts.flattenDiagnosticMessageText(d.messageText, '\n') })), [], `Syntax ${relative}`);
    const module = { exports: {} }; cache.set(relative, module);
    const req = specifier => {
      if (specifier === 'react') return React;
      if (specifier === 'react/jsx-runtime') return dependency(specifier);
      if (specifier === 'react-native') return native;
      if (specifier === 'expo-modules-core') return { requireOptionalNativeModule: name => { assert.equal(name, 'RudiFontScale'); return { getFontScale: () => state.nativeFont, addListener: (event, fn) => { assert.equal(event, 'onFontScaleChanged'); fontListeners.add(fn); return { remove: () => fontListeners.delete(fn) }; } }; } };
      if (specifier === 'expo-image') return { Image: visual('Image') };
      if (specifier === 'expo-image-picker') return { launchImageLibraryAsync: (...args) => invoke('launchImageLibraryAsync', args, { canceled: true, assets: [] }) };
      if (specifier === 'expo-image-manipulator') return { SaveFormat: { JPEG: 'jpeg' }, manipulateAsync: (...args) => invoke('manipulateAsync', args, { uri: 'file:///synthetic-clean.jpg', width: 32, height: 32 }) };
      if (specifier === 'expo-router') return { useLocalSearchParams: () => state.params, useRouter: () => ({ replace: value => routes.push(['replace', value]), push: value => routes.push(['push', value]) }) };
      if (specifier === 'react-native-safe-area-context') return { useSafeAreaInsets: () => ({ bottom: 12, top: 24, left: 0, right: 0 }) };
      assert.ok(specifier.startsWith('.'), `Unmapped external ${specifier} from ${relative}`);
      return load(path.posix.normalize(path.posix.join(path.posix.dirname(relative), specifier)));
    };
    new vm.Script(`(function(require,module,exports){${result.outputText}\n})`, { filename: file }).runInThisContext()(req, module, module.exports);
    return module.exports;
  }
  const Session = load('apps/mobile/src/rudi/session');
  const Composer = load('apps/mobile/src/rudi/community/Composer').Composer;
  function Probe() { session = Session.useRudiSession(); return null; }
  function App() { React.useLayoutEffect(() => { const callback = state.afterCommit; state.afterCommit = null; callback?.(); }); return React.createElement(Session.RudiSessionProvider, null, React.createElement(Probe), state.show ? React.createElement(Composer) : null); }
  const update = async fn => { await act(async () => { fn(); root.update(React.createElement(App, { tick: ++state.tick })); }); };
  const hosts = (type, match = () => true) => root.root.findAll(node => node.type === type && match(node.props));
  const one = (type, match) => { const nodes = hosts(type, match); assert.equal(nodes.length, 1, `${type} count`); return nodes[0]; };
  const h = {
    calls, routes, consumed, load, state,
    async mount() { await act(async () => { root = Renderer.create(React.createElement(App)); }); },
    afterNextCommit(callback) { state.afterCommit = callback; },
    hideNow() { state.show = false; root.update(React.createElement(App, { tick: ++state.tick })); },
    async unmount() { await update(() => { state.show = false; }); },
    async remount() { await update(() => { state.show = true; }); },
    async dispose() { await act(async () => { root.unmount(); }); },
    async setActor(value) { state.actor = value; await act(async () => { value ? session.datPhien(value) : session.quenTaiCho(); }); },
    async setParams(value) { await update(() => { state.params = value; }); },
    audienceOpen() { return hosts('RNPressable', p => p.accessibilityRole === 'radio').length > 0; },
    body() { return one('RNTextInput', p => p.testID === 'community-body'); },
    field(label) { return one('Field', p => p.label === label || p.accessibilityLabel === label); },
    button(label) { return one('RudiButton', p => p.label === label || p.accessibilityLabel === label); },
    media() { return one('MediaPicker'); },
    text() { return hosts('RNText').map(node => node.props.children).filter(value => typeof value === 'string').join(' '); },
    tool(label) { return one('RNPressable', p => p.accessibilityLabel === label); },
    mentions() { return one('MentionPicker'); },
    error() { return one('CauTaiCho', p => p.testID === 'cong-dong-loi-gui').props; },
    send() { return one('RudiButton', p => /^(Gửi lên cộng đồng|Gửi bản sửa|Đăng lên tường|Đang gửi…)$/.test(p.label)); },
    async nativeFont(snapshot) { await act(async () => { state.nativeFont = snapshot; [...fontListeners].forEach(fn => fn()); }); },
    async changeBody(value) { await act(async () => { h.body().props.onChangeText(value); }); },
    async changeTopics(value) { let nodes = hosts('Field', p => p.label === 'Chủ đề'); if (!nodes.length) { await act(async () => { one('RNPressable', p => p.accessibilityLabel === 'Chủ đề').props.onPress(); }); } await act(async () => { h.field('Chủ đề').props.onChangeText(value); }); },
    async changeMedia(value) { await act(async () => { h.media().props.onChange(value); }); },
    async changeMentions(value) { await act(async () => { h.mentions().props.onChange(value); }); },
    async changeFile(value) { await act(async () => { h.media().props.onFileChange(value); }); },
    async chooseAudience(title) { await act(async () => { one('RNPressable', p => String(p.accessibilityLabel).startsWith('Người đọc:')).props.onPress(); }); const radio = hosts('RNPressable', p => p.accessibilityRole === 'radio').find(node => node.findAll(n => n.type === 'RNText' && n.props.children === title).length); assert.ok(radio, `Radio ${title}`); await act(async () => { radio.props.onPress(); }); },
    async press(label) { await act(async () => { h.button(label).props.onPress(); }); },
    async pressSend() { await act(async () => { h.send().props.onPress(); }); },
    deferNext(kind) { const d = deferred(); if (!queue.has(kind)) queue.set(kind, []); queue.get(kind).push(d); d.finish = async value => { await act(async () => { d.resolve(value); }); }; d.fail = async error => { await act(async () => { d.reject(error); }); }; return d; },
    callsFor(kind) { return calls.filter(c => c.kind === kind); },
    session() { return session; },
    async revoke() { await act(async () => { state.revoke(state.token); }); },
  };
  return h;
}
module.exports = { createHost, person, post, act };
