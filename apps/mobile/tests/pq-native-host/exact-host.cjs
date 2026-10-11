// Load application source directly and exercise its observable behavior.
// This substitutes native packages, bundled assets, timers, sockets and I/O entrypoints.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');

const BASE_SHA = 'edccaf00bf9af5f0ed1f553f27244745d49418c3';
const { repo, sourceRoot } = require('./repository.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const React = dependency('react');
const Renderer = dependency('react-test-renderer');
const ts = dependency('typescript');
const { act } = React;

function flatten(value) {
  if (value === null || typeof value !== 'object') return undefined;
  if (!Array.isArray(value)) return value;
  const output = {};
  for (const child of value) { const next = flatten(child); if (next) Object.assign(output, next); }
  return output;
}
function person(id = 'synthetic-A') {
  return { person_id: id, token: `synthetic-token-${id}`, profile: { display_name: `Synthetic ${id}` }, context_id: null, contexts: [] };
}
function commentRow() {
  return { id: 'synthetic-comment', author_id: 'synthetic-B', author: 'Synthetic author', body: 'Synthetic server comment', parent_id: null, media_id: null, mentions: [], created_at: '2026-10-01T00:00:00Z', status: 'approved', can_delete: false };
}
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }

// Independent nonproportional fixture, complete for the actual native parser.
// No 26sp row exists: a Composer retaining its old 26sp lineHeight must fail.
const BASES = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6, 11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34];
const DOUBLE_DP = [-1.1, -0.49, -0.24, 0.62, 1.2, 2.3, 18.5, 20.25, 21.5, 23.25, 25, 28.75, 30.25, 31.75, 34.25, 36, 37.5, 39.25, 43, 44.25, 52];
function snapshot(scale = 1, lineHeight) {
  return { status: 'available', fontScale: scale, fontMetrics: { density: 2, spToDp: BASES.map((base, i) => ({ base, dp: base === 24 && lineHeight !== undefined ? lineHeight : scale === 1 ? base : DOUBLE_DP[i] })) } };
}

function createHost({ surface = 'field', initialSnapshot = snapshot(), fieldProps = {}, commentsProps = {}, pickerProps = {}, provideMetrics = true } = {}) {
  const state = { actor: person(), params: {}, post: { id: 'synthetic-post', can_comment: true }, nativeFont: initialSnapshot, fieldProps, commentsProps, pickerProps, provideMetrics, token: null, tick: 0, show: true };
  const cache = new Map(), consumed = new Map(), listeners = new Set(), timers = new Map();
  const calls = [], drafts = [], refs = [], routes = [], sockets = [], picks = [];
  let root, session, nodeId = 0, timerId = 0, attempt = 0, subscriptions = 0, removals = 0, focused = null;
  const visual = name => React.forwardRef(function NativeBoundary(props, ref) { return React.createElement(name, { ...props, ref }); });
  const Native = {};
  for (const name of ['View', 'Text', 'TextInput', 'Pressable', 'ScrollView', 'KeyboardAvoidingView', 'ActivityIndicator', 'RefreshControl', 'Image']) Native[name] = visual(`RN${name}`);
  Native.TextInput.State = { currentlyFocusedInput: () => focused };
  Object.assign(Native, {
    Platform: { OS: 'android', select: choices => choices.android ?? choices.default },
    Appearance: { getColorScheme: () => 'light' },
    StyleSheet: { create: value => value, flatten, hairlineWidth: 0.5, absoluteFill: { position: 'absolute', left: 0, right: 0, top: 0, bottom: 0 } },
    useWindowDimensions: () => ({ width: 390, height: 844, fontScale: 1, scale: 2 }),
    PixelRatio: { get: () => 2, getFontScale: () => 1, roundToNearestPixel: value => Math.round(value * 2) / 2 },
    Keyboard: { dismiss() {}, addListener: () => ({ remove() {} }) },
    AccessibilityInfo: { isReduceMotionEnabled: () => Promise.resolve(true), addEventListener: () => ({ remove() {} }), setAccessibilityFocus() {}, announceForAccessibility() {} },
    AppState: { currentState: 'active', addEventListener: () => ({ remove() {} }) },
    findNodeHandle: value => value?.id ?? null,
  });
  const Animated = { View: visual('RNAnimatedView'), ScrollView: Native.ScrollView, createAnimatedComponent: component => component };
  const animation = { duration() { return this; }, reduceMotion() { return this; }, delay() { return this; }, springify() { return this; } };
  const reanimated = {
    __esModule: true, default: Animated, FadeIn: animation,
    ReduceMotion: { Always: 'always', Never: 'never' }, Easing: { bezier: (...values) => values, inOut: value => value, ease: 'ease' },
    useReducedMotion: () => true,
    useSharedValue(value) { const ref = React.useRef(null); if (ref.current === null) ref.current = { value }; return ref.current; },
    useAnimatedStyle: callback => callback(), useAnimatedScrollHandler: callback => callback,
    withTiming: value => value, withSpring: value => value, withRepeat: value => value,
    cancelAnimation() {}, runOnJS: callback => callback,
  };
  const nativeFontModule = {
    getFontScale: () => state.nativeFont,
    addListener(name, callback) {
      assert.equal(name, 'onFontScaleChanged'); subscriptions += 1; listeners.add(callback);
      return { remove() { removals += 1; listeners.delete(callback); } };
    },
  };
  const api = {
    BASE_URL: 'https://synthetic.invalid', LOI_KHONG_NOI_DUOC: 'Synthetic transport refusal',
    newAttempt: () => ({ key: `synthetic-attempt-${++attempt}` }),
    datTokenPhien: token => { state.token = token; }, tokenPhienHienTai: () => state.token,
    duongDanAnhDaiDien: id => `/people/${id}/avatar`,
    translatedAsActor: async (_errors, route, options = {}) => {
      calls.push({ route, options });
      if (route.startsWith('/people/avatars?')) return { avatars: {} };
      if (route.startsWith('/friends')) return [];
      if (route.includes('/comments') && (options.method ?? 'GET') === 'GET') return { comments: [commentRow()], pending: [], next_cursor: null };
      if (route.includes('/comments')) throw new Error('Unexpected comment write in metric-only test');
      if (route === '/v2/community/posts' && options.method === 'POST') throw new Error('Unexpected post write in metric-only test');
      throw new Error(`Unexpected synthetic transport route ${route}`);
    },
  };
  const adapters = {
    'apps/mobile/src/api': api,
    'apps/mobile/src/phien': { khoiPhucPhien: () => Promise.resolve(state.actor), dangXuat: async () => {}, quenPhienDaThuHoi: async () => {}, docNhomCuaToi: async () => [] },
    'apps/mobile/src/danh-tinh': { ngheThuHoiPhien: callback => { state.revoke = callback; return () => { state.revoke = null; }; }, tokenPhienHienTai: () => state.token, headerNguoiGoi: () => ({ Authorization: 'Bearer synthetic-only' }) },
    // Legacy-session storage is an I/O boundary; actual session and both vaults execute.
    'apps/mobile/src/rudi/kho': { xoaPhienAsync: async () => {} },
  };
  const externals = {
    react: React, 'react/jsx-runtime': dependency('react/jsx-runtime'), 'react-native': Native,
    'react-native-reanimated': reanimated,
    'expo-modules-core': { requireOptionalNativeModule: name => { assert.equal(name, 'RudiFontScale'); return nativeFontModule; } },
    'expo-image': { Image: visual('RNExpoImage') },
    'expo-blur': { BlurView: visual('RNBlurView') },
    'expo-linear-gradient': { LinearGradient: visual('RNLinearGradient') },
    'expo-status-bar': { StatusBar: visual('RNStatusBar') },
    'expo-router': { useLocalSearchParams: () => state.params, usePathname: () => '/community/new', useRouter: () => ({ canGoBack: () => false, back() {}, push: value => routes.push(value), replace: value => routes.push(value) }) },
    'expo-router/build/react-navigation/core/NavigationContext': { NavigationContext: React.createContext(null) },
    'react-native-safe-area-context': { SafeAreaView: visual('RNSafeAreaView'), useSafeAreaInsets: () => ({ top: 24, bottom: 12, left: 0, right: 0 }) },
    '@expo/vector-icons': { Ionicons: visual('RNIonicons') },
    'react-native-svg': { __esModule: true, default: visual('RNSvg'), Path: visual('RNPath'), Circle: visual('RNCircle') },
    'expo-haptics': { ImpactFeedbackStyle: { Medium: 'medium' }, NotificationFeedbackType: { Success: 'success' }, selectionAsync() {}, impactAsync() {}, notificationAsync() {} },
    'expo-image-picker': { launchImageLibraryAsync: (...args) => { const pending = deferred(); picks.push({ ...pending, args }); return pending.promise; } },
    'expo-image-manipulator': { SaveFormat: { JPEG: 'jpeg' }, manipulateAsync: async () => { throw new Error('Unexpected image transform in metric-only test'); } },
  };
  class SyntheticSocket {
    constructor(url) { this.url = url; sockets.push(this); }
    send(value) { calls.push({ socket: value }); }
    close() { this.closed = true; }
  }
  const globals = {
    console, process: { env: {} }, URL, URLSearchParams, WebSocket: SyntheticSocket,
    setTimeout: callback => { const id = ++timerId; timers.set(id, callback); return id; }, clearTimeout: id => timers.delete(id),
    requestAnimationFrame: callback => { const id = ++timerId; timers.set(id, callback); return id; }, cancelAnimationFrame: id => timers.delete(id),
  };
  function load(relative) {
    relative = relative.replace(/\\/g, '/').replace(/\.(tsx?|js)$/, '');
    if (adapters[relative]) return adapters[relative];
    if (cache.has(relative)) return cache.get(relative).exports;
    if (/\.(png|jpg|webp)$/.test(relative)) return 1;
    const variants = relative.endsWith('.json') ? [''] : ['.ts', '.tsx', '/index.ts', '/index.tsx'];
    const file = variants.map(ext => path.join(sourceRoot, relative + ext)).find(fs.existsSync);
    assert.ok(file, `Exact source must exist: ${relative}`);
    consumed.set(relative, file);
    const bytes = fs.readFileSync(file, 'utf8');
    const module = { exports: {} }; cache.set(relative, module);
    if (file.endsWith('.json')) { module.exports = JSON.parse(bytes); return module.exports; }
    const output = ts.transpileModule(bytes, { fileName: file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true }, reportDiagnostics: true });
    // Never retain TS diagnostic source objects: their graphs can print source/trees.
    const errors = (output.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error).map(item => ({ code: item.code, start: item.start, message: ts.flattenDiagnosticMessageText(item.messageText, '\n') }));
    assert.deepEqual(errors, [], `Syntax diagnostics: ${relative}`);
    function req(specifier) {
      if (Object.hasOwn(externals, specifier)) return externals[specifier];
      assert.ok(specifier.startsWith('.'), `Unmapped native boundary ${specifier} from ${relative}`);
      return load(path.posix.normalize(path.posix.join(path.posix.dirname(relative), specifier)));
    }
    vm.runInNewContext(`(function(require,module,exports){${output.outputText}\n})`, globals, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  const Session = load('apps/mobile/src/rudi/session');
  const metrics = load('apps/mobile/src/rudi/ui/NativeFontMetrics');
  const Field = load('apps/mobile/src/rudi/ui/Field').Field;
  const Component = surface === 'comments' ? load('apps/mobile/src/rudi/community/Comments').Comments : surface === 'composer' ? load('apps/mobile/src/rudi/community/Composer').Composer : surface === 'picker' ? load('apps/mobile/src/rudi/community/MediaPicker').MediaPicker : Field;
  const fieldRef = value => refs.push(value);
  function FieldDraft() {
    const [value, setValue] = React.useState(state.fieldProps.value ?? 'Synthetic draft');
    return React.createElement(Field, { testID: 'field', multiline: true, numberOfLines: 3, oRef: fieldRef, ...state.fieldProps, value, onChangeText: next => { setValue(next); state.fieldProps.onChangeText?.(next); } });
  }
  function Probe() { session = Session.useRudiSession(); return null; }
  function App() {
    let element;
    if (surface === 'field') element = React.createElement(metrics.NativeFontMetricsProvider, { snapshot: state.nativeFont }, React.createElement(FieldDraft));
    if (surface === 'comments') element = React.createElement(Component, { person: state.actor.person_id, post: state.post, onDraftChange: draft => drafts.push(draft), ...state.commentsProps });
    if (surface === 'composer') element = React.createElement(Component);
    if (surface === 'picker') {
      element = React.createElement(Component, { person: state.actor.person_id, media: [], onChange() {}, ...state.pickerProps });
      if (state.provideMetrics) element = React.createElement(metrics.NativeFontMetricsProvider, { snapshot: state.nativeFont }, element);
    }
    return React.createElement(Session.RudiSessionProvider, null, React.createElement(Probe), state.show ? element : null);
  }
  const h = {
    state, consumed, calls, drafts, refs, load, routes, metrics, picks,
    get root() { return root; },
    observation() { return { live: listeners.size, subscriptions, removals }; },
    hosts(kind, match = () => true) { return root.root.findAll(node => node.type === kind && match(node.props)); },
    one(kind, match) { const matches = h.hosts(kind, match); assert.equal(matches.length, 1, `One ${kind} host`); return matches[0]; },
    input() { return h.one('RNTextInput', props => props.testID === (surface === 'comments' ? 'community-comment-input' : surface === 'composer' ? 'community-body' : 'field')); },
    async mount() {
      await act(async () => { root = Renderer.create(React.createElement(App), { createNodeMock: element => ({ id: ++nodeId, kind: element.type, focus() { focused = this; }, blur() { if (focused === this) focused = null; }, scrollTo() {}, getNativeScrollRef() { return this; }, measureInWindow(callback) { callback(0, 0, 390, 400); } }) }); });
    },
    async update(callback = () => {}) { await act(async () => { callback(); root.update(React.createElement(App, { tick: ++state.tick })); }); },
    async nativeFont(value) { await act(async () => { state.nativeFont = value; [...listeners].forEach(callback => callback()); if (surface === 'field' || surface === 'picker') root.update(React.createElement(App, { tick: ++state.tick })); }); },
    async remountSurface() { await h.update(() => { state.show = false; }); await h.update(() => { state.show = true; }); },
    async pressAccessibility(label) { await act(async () => h.one('RNPressable', props => props.accessibilityLabel === label).props.onPress()); },
    async pressText(label) { const nodes = h.hosts('RNPressable').filter(node => node.findAllByType('RNText').some(text => text.props.children === label)); assert.equal(nodes.length, 1, `One pressable for ${label}`); await act(async () => nodes[0].props.onPress()); },
    async rejectPick(error = new Error('Synthetic private native diagnostic')) { assert.ok(picks.length, 'a native pick is pending'); await act(async () => picks.at(-1).reject(error)); },
    async change(value) { await act(async () => h.input().props.onChangeText(value)); },
    async focus() { await act(async () => h.input().props.onFocus?.({ nativeEvent: { synthetic: true } })); },
    async actorTo(actor) { await act(async () => { state.actor = person(actor); session.datPhien(state.actor); root.update(React.createElement(App, { tick: ++state.tick })); }); },
    owner(name) { const found = root.root.findAll(node => typeof node.type === 'function' && node.type.name === name); assert.equal(found.length, 1, `One ${name} owner`); return found[0]; },
    async dispose() { if (root) await act(async () => root.unmount()); timers.clear(); },
  };
  return h;
}

module.exports = { BASE_SHA, repo, sourceRoot, createHost, snapshot, flatten, React, act };
