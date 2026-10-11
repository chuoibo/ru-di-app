// Actual application graph with native, package, and I/O boundaries only.
// Native host props establish React ownership; Android layout still needs device evidence.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const crypto = require('node:crypto');
const { createRequire } = require('node:module');
const { repo, sourceRoot } = require('./repository.cjs');
const dependency = createRequire(path.join(repo, 'apps/mobile/package.json'));
const React = dependency('react');
const Renderer = dependency('react-test-renderer');
const ts = dependency('typescript');
const { act } = React;

function flatten(value) {
  if (value === null || typeof value !== 'object') return undefined;
  if (!Array.isArray(value)) return value;
  return Object.assign({}, ...value.map(flatten).filter(Boolean));
}
function person(id = 'synthetic-A') {
  return { person_id: id, token: `synthetic-token-${id}`, profile: { display_name: `Synthetic ${id}` }, context_id: null, contexts: [] };
}
function post(id = 'synthetic-post-1', author = 'synthetic-B') {
  return { id, author_id: author, author: `Synthetic ${author}`, body: `Synthetic body ${id}`, revision: 1, audience: 'public', status: 'published', topics: ['Synthetic topic'], mentions: [], media: [], context_id: null, can_comment: true, saved: false, diary: false, created_at: '2026-10-01T00:00:00Z', liked: false, following: false, likes: 2, comments: 3, why: 'Synthetic feed explanation' };
}
function commentRow() {
  return { id: 'synthetic-parent', author_id: 'synthetic-B', author: 'Synthetic comment author', body: 'Synthetic server comment', parent_id: null, media_id: null, mentions: [], created_at: '2026-10-01T00:00:00Z', status: 'approved', can_delete: false };
}
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
const BASES = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6, 11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34];
const DOUBLE_DP = [-1.1, -0.49, -0.24, 0.62, 1.2, 2.3, 18.5, 20.25, 21.5, 23.25, 25, 28.75, 30.25, 31.75, 34.25, 36, 37.5, 39.25, 43, 44.25, 52];
function snapshot(scale = 1, lineHeight) {
  return { status: 'available', fontScale: scale, fontMetrics: { density: 2, spToDp: BASES.map((base, i) => ({ base, dp: base === 24 && lineHeight !== undefined ? lineHeight : scale === 1 ? base : DOUBLE_DP[i] })) } };
}

function profileFixture(overrides = {}) {
  return { id: 'synthetic-A', display_name: 'Synthetic profile', bio: 'Synthetic profile bio', city: 'Synthetic city', created_at: '2026-10-01T00:00:00Z', login_methods: [], interests: [], budget_band: null, ...overrides, counts: { friends: 3, contexts: 2, outings: 5, places_checked_in: 7, memories: 11, ...overrides.counts } };
}
function journeyFixture() {
  return { routes: [], active_run: null, earned_badges: [], candidates: [], chapters: [], mp4_credits: { granted: 0, used: 0, available: 0 }, suggestion_preview: { checkins: 0, distinct_destinations: 0, photo_days: 0, story_days: 0, shared_outings: 0 } };
}
function createHost({ surface = 'profile', initialSnapshot = snapshot(), explicitProvider = false, deferPost = false, strict = false, commentsProps = {}, feedPosts, boundaryChildren, friends = [], profile = {}, savedPlaces = [], invitations = [], deferProfile = false, deferSaved = false, placeNames = {} } = {}) {
  assert.ok(['comments', 'detail', 'feed', 'boundary', 'profile'].includes(surface));
  const state = { profile: profileFixture(profile), savedPlaces: [...savedPlaces], invitations: [...invitations], placeNames: { ...placeNames }, journey: journeyFixture(), actor: person(), params: { id: 'synthetic-post-1' }, post: post(), feedPosts: feedPosts ?? [post('synthetic-feed-1'), post('synthetic-feed-2'), post('synthetic-feed-3')], nativeFont: initialSnapshot, explicitProvider, commentsProps, boundaryChildren, friends, show: true, tick: 0, token: null, window: { width: 390, height: 844, fontScale: 1, scale: 2 } };
  const cache = new Map(), consumed = new Map(), consumedHashes = new Map();
  const listeners = new Set(), callbacks = [], retiredCallbacks = [], timers = new Map();
  const commentsReadQueue = [], friendsReadQueue = [], profileReadQueue = [], savedReadQueue = [];
  if (deferProfile) profileReadQueue.push({ ...deferred(), started: false });
  if (deferSaved) savedReadQueue.push({ ...deferred(), started: false });
  const profileReads = [], profileWrites = [], sessionWrites = [], savedReads = [];
  // Profile reads are a named phien I/O boundary. Taste also reads /people/me,
  // so parking by route alone would consume the wrong request.
  function profileRead(actorId) {
    const call = { kind: 'readProfile', boundary: 'phien.docHoSoToi', route: '/people/me', options: { method: 'GET', actorId } };
    calls.push(call); profileReads.push(call);
    const queued = profileReadQueue.shift();
    if (queued) { Object.assign(call, queued); queued.started = true; return queued.promise; }
    return Promise.resolve(state.profile);
  }
  const calls = [], drafts = [], refs = [], routes = [], sockets = [], timings = [], picks = [], uploads = [], writes = [], gets = [], reads = [], friendReads = [], deletes = [], nativeHostCalls = [];
  let root, session, focused = null, timerId = 0, nodeId = 0, attempt = 0, sharedId = 0, collecting = null;
  let getterReads = 0, subscriptions = 0, removals = 0, maxLive = 0, postDeferredRemaining = deferPost;
  // Destination browsing storage is a native package boundary, scoped to this host.
  // Reject unknown API/keys even if product fallback code catches the thrown error.
  const storageValues = new Map(), storageCalls = [], boundaryViolations = [];
  function refuseStorage(kind, method) {
    boundaryViolations.push({ boundary: 'AsyncStorage', kind, method });
    throw new Error('Unmapped synthetic AsyncStorage boundary request');
  }
  function storageKey(method, key) {
    if (key !== 'rudi.diem-den.v1') refuseStorage('key', method);
    storageCalls.push({ method, key });
  }
  const storageMethods = Object.freeze({
    async getItem(key) { storageKey('getItem', key); return storageValues.get(key) ?? null; },
    async setItem(key, value) {
      storageKey('setItem', key);
      if (typeof value !== 'string') refuseStorage('value-type', 'setItem');
      storageValues.set(key, value);
    },
    async removeItem(key) { storageKey('removeItem', key); storageValues.delete(key); },
  });
  const syntheticStorage = new Proxy(storageMethods, {
    get(target, property) {
      if (typeof property === 'string' && Object.hasOwn(target, property)) return target[property];
      return refuseStorage('method', typeof property === 'string' ? property : 'symbol');
    },
  });
  const visual = name => React.forwardRef(function NativeBoundary(props, ref) { return React.createElement(name, { ...props, ref }); });
  const Native = {};
  for (const name of ['View', 'Text', 'TextInput', 'Pressable', 'ScrollView', 'KeyboardAvoidingView', 'ActivityIndicator', 'RefreshControl', 'Image', 'Switch', 'Modal']) Native[name] = visual(`RN${name}`);
  Native.TextInput.State = { currentlyFocusedInput: () => focused };
  class AnimatedValue { constructor(value) { this.value = value; } setValue(value) { this.value = value; } interpolate() { return this; } }
  Object.assign(Native, {
    Platform: { OS: 'android', select: choices => choices.android ?? choices.default },
    Appearance: { getColorScheme: () => 'light' },
    StyleSheet: { create: value => value, flatten, hairlineWidth: 0.5, absoluteFill: { position: 'absolute', left: 0, right: 0, top: 0, bottom: 0 } },
    useWindowDimensions: () => state.window,
    PixelRatio: { get: () => state.window.scale, getFontScale: () => state.window.fontScale, roundToNearestPixel: value => Math.round(value * state.window.scale) / state.window.scale },
    Keyboard: { dismiss() {}, addListener: () => ({ remove() {} }) },
    BackHandler: { addEventListener: () => ({ remove() {} }) },
    AccessibilityInfo: { isReduceMotionEnabled: () => Promise.resolve(true), addEventListener: () => ({ remove() {} }), setAccessibilityFocus() {}, announceForAccessibility() {} },
    AppState: { currentState: 'active', addEventListener: () => ({ remove() {} }) },
    findNodeHandle: value => value?.id ?? null,
    Share: { dismissedAction: 'dismissedAction', share: async () => ({ action: 'dismissedAction' }) },
    Easing: { bezier: (...values) => values },
    Animated: { Value: AnimatedValue, View: visual('RNLegacyAnimatedView'), timing: (value, options) => ({ start(callback) { value.setValue(options.toValue); callback?.({ finished: true }); }, stop() {} }), parallel: animations => ({ start(callback) { animations.forEach(animation => animation.start()); callback?.({ finished: true }); }, stop() {} }) },
  });
  // A real renderItem tree for every row, with stable keys and a forwarded native ref.
  // No native windowing, clipping, layout or pixel scroll behavior is simulated.
  Native.FlatList = React.forwardRef(function NativeFlatList(props, ref) {
    const element = value => typeof value === 'function' ? React.createElement(value) : value;
    return React.createElement('RNFlatList', { ...props, ref }, element(props.ListHeaderComponent), props.data.length ? props.data.map((item, index) => React.createElement(React.Fragment, { key: props.keyExtractor?.(item, index) ?? String(index) }, props.renderItem({ item, index }))) : element(props.ListEmptyComponent), element(props.ListFooterComponent));
  });
  const Animated = { View: visual('RNAnimatedView'), ScrollView: Native.ScrollView, createAnimatedComponent: component => component };
  const animation = { duration() { return this; }, reduceMotion() { return this; }, delay() { return this; }, springify() { return this; } };
  const reanimated = {
    __esModule: true, default: Animated, FadeIn: animation, FadeOut: animation,
    ReduceMotion: { Always: 'always', Never: 'never' }, Easing: { bezier: (...values) => values, inOut: value => value, ease: 'ease' }, useReducedMotion: () => true,
    useSharedValue(initial) {
      const ref = React.useRef(null);
      if (ref.current === null) {
        let value = initial; const id = ++sharedId; const shared = {};
        Object.defineProperty(shared, 'value', { get() { if (collecting) collecting.add(id); return value; }, set(next) { if (next && next.nativeAnimation) { value = next.target; if (next.kind === 'timing') timings.push({ ...next, shared: id }); } else value = next; } });
        ref.current = shared;
      }
      return ref.current;
    },
    useAnimatedStyle(callback) { const used = new Set(); collecting = used; const value = callback(); collecting = null; return { ...value, syntheticSharedIds: [...used] }; },
    useAnimatedScrollHandler: callback => callback,
    withTiming: (target, config, callback) => ({ nativeAnimation: true, kind: 'timing', target, config, callback }),
    withSpring: (target, config) => ({ nativeAnimation: true, kind: 'spring', target, config }),
    withRepeat: value => value, cancelAnimation() {}, runOnJS: callback => callback, interpolate: value => value,
  };
  function gestureChain() {
    const values = {};
    const chain = new Proxy(values, { get(target, key) { if (key in target) return target[key]; return value => { target[String(key) + 'Value'] = value; return chain; }; } });
    return chain;
  }
  const nativeFontModule = {
    getFontScale() { getterReads += 1; return state.nativeFont; },
    addListener(name, callback) {
      assert.equal(name, 'onFontScaleChanged'); subscriptions += 1; listeners.add(callback); callbacks.push(callback); maxLive = Math.max(maxLive, listeners.size);
      let live = true;
      return { remove() { assert.equal(live, true, 'native subscription is removed once'); live = false; removals += 1; listeners.delete(callback); retiredCallbacks.push(callback); } };
    },
  };
  function parked(store, call) { const pending = deferred(); Object.assign(call, pending); store.push(call); return pending.promise; }
  const api = {
    BASE_URL: 'https://synthetic.invalid', LOI_KHONG_NOI_DUOC: 'Synthetic transport refusal',
    ApiError: class ApiError extends Error { constructor(status, message) { super(message); this.status = status; } },
    thongDiepNguoiDoc: () => 'Synthetic refusal', newAttempt: () => ({ key: `synthetic-attempt-${++attempt}` }),
    datTokenPhien: token => { state.token = token; }, tokenPhienHienTai: () => state.token, duongDanAnhDaiDien: id => `/people/${id}/avatar`,
    translatedAsActor: async (_errors, route, options = {}) => {
      const method = options.method ?? 'GET'; const call = { route, options, kind: 'transport' }; calls.push(call);
      if (route === '/people/me' && method === 'GET') { call.kind = 'readTaste'; return state.profile; }
      if (route === '/people/me' && method === 'PATCH') {
        call.kind = 'writeProfile'; call.body = options.body;
        return parked(profileWrites, call).then(value => { state.profile = value; return value; });
      }
      if (route === '/people/me/saved-places' && method === 'GET') {
        call.kind = 'readSavedPlaces'; savedReads.push(call);
        const queued = savedReadQueue.shift();
        if (queued) { Object.assign(call, queued); queued.started = true; return queued.promise; }
        return { saved: state.savedPlaces.map(place_id => ({ place_id })) };
      }
      if (/^\/people\/[^/]+\/friend-requests\?direction=incoming$/.test(route) && method === 'GET') { call.kind = 'readInvitations'; return { requests: state.invitations }; }
      if (/^\/people\/[^/]+\/diaries\?before=$/.test(route) && method === 'GET') { call.kind = 'readDiaries'; return { diaries: [], next_cursor: null }; }
      if (route === '/me/achievement-routes' && method === 'GET') { call.kind = 'readJourney'; return state.journey; }
      if (route.startsWith('/people/avatars?')) return { avatars: {} };
      if (route.startsWith('/friends')) return [];
      if (/^\/people\/[^/]+\/friends$/.test(route) && method === 'GET') {
        call.kind = 'readFriends'; friendReads.push(call);
        const queued = friendsReadQueue.shift();
        if (queued) { Object.assign(call, queued); queued.started = true; return queued.promise; }
        return { friends: state.friends };
      }
      if (route.startsWith('/v2/community/feed?')) { call.kind = 'readFeed'; return { posts: state.feedPosts, next_cursor: null, personalized: false }; }
      if (route === '/v2/community/preferences' && method === 'GET') return { asked: true, personalized: false };
      if (route === '/v2/community/notifications') return { notifications: [] };
      if (route === '/v2/community/follows' && method === 'GET') return { follows: [] };
      if (route.includes('/comments') && method === 'GET') {
        call.kind = 'readComments'; reads.push(call);
        const queued = commentsReadQueue.shift();
        if (queued) { Object.assign(call, queued); queued.started = true; return queued.promise; }
        return { comments: [commentRow()], pending: [], next_cursor: null };
      }
      if (route.endsWith('/comments') && method === 'POST') { call.kind = 'comment'; return parked(writes, call); }
      if (/^\/v2\/community\/posts\/[^/]+\/comments\/[^/]+$/.test(route) && method === 'DELETE') {
        call.kind = 'deleteComment'; return parked(deletes, call);
      }
      if (/^\/v2\/community\/posts\/[^/]+$/.test(route) && method === 'GET') {
        call.kind = 'getPost'; gets.push(call);
        if (postDeferredRemaining) { postDeferredRemaining = false; const pending = deferred(); Object.assign(call, pending); return pending.promise; }
        return { ...state.post, id: route.split('/').at(-1) };
      }
      if (route.endsWith('/status') && method === 'GET') return { state: 'ready' };
      throw new Error(`Unexpected synthetic transport ${method} ${route}`);
    },
  };
  api.translatedAnonymous = async (_errors, route, options = {}) => {
    const call = { route, options, kind: 'anonymousTransport' }; calls.push(call);
    assert.equal(options.method ?? 'GET', 'GET', 'anonymous profile boundary is read only');
    if (/^\/places\/[^/]+$/.test(route)) {
      const id = decodeURIComponent(route.slice('/places/'.length));
      call.kind = 'readSavedPlace';
      return { id, name: state.placeNames[id] ?? `Synthetic place ${id}`, category: 'synthetic', kinds: [], photo_count: 0, traits: [], description: null, reviews: [], photos_available: false, activities: [] };
    }
    throw new Error(`Unexpected synthetic anonymous transport ${options.method ?? 'GET'} ${route}`);
  };
  const adapters = {
    'apps/mobile/src/api': api,
    'apps/mobile/src/phien': {
      khoiPhucPhien: () => Promise.resolve(state.actor), dangXuat: async () => {}, quenPhienDaThuHoi: async () => {}, docNhomCuaToi: async () => [],
      docHoSoToi: actorId => profileRead(actorId),
      suaHoSoToi: (actorId, body) => api.translatedAsActor({}, '/people/me', { method: 'PATCH', body, actorId, attempt: api.newAttempt() }),
      doiTenTrongPhien: async (input, displayName) => {
        const next = { ...input, profile: { ...input.profile, display_name: displayName } };
        sessionWrites.push({ kind: 'persistSessionName', displayName, session: next });
        api.datTokenPhien(next.token);
        return next;
      },
    },
    'apps/mobile/src/danh-tinh': { ngheThuHoiPhien: callback => { state.revoke = callback; return () => { state.revoke = null; }; }, tokenPhienHienTai: () => state.token, headerNguoiGoi: () => ({ Authorization: 'Bearer synthetic-only' }) },
    'apps/mobile/src/rudi/kho': { xoaPhienAsync: async () => {}, docGiaoDienAsync: async () => null, ghiGiaoDienAsync: async () => {} },
  };
  const router = { canGoBack: () => false, back() {}, push: value => routes.push(value), replace: value => routes.push(value) };
  const svg = new Proxy({ __esModule: true, default: visual('RNSvg') }, { get(target, key) { if (key in target) return target[key]; target[key] = visual(`RNSvg${String(key)}`); return target[key]; } });
  const externals = {
    react: React, 'react/jsx-runtime': dependency('react/jsx-runtime'), 'react-native': Native, 'react-native-reanimated': reanimated,
    '@react-native-async-storage/async-storage': { __esModule: true, default: syntheticStorage },
    'expo-modules-core': { requireOptionalNativeModule: name => { assert.equal(name, 'RudiFontScale'); return nativeFontModule; } },
    'expo-image': { Image: visual('RNExpoImage') }, 'expo-video': { VideoView: visual('RNVideoView'), useVideoPlayer: (_source, setup) => { const ref = React.useRef(null); if (ref.current === null) { ref.current = { pause() {}, loop: false, muted: false }; setup?.(ref.current); } return ref.current; } },
    'expo-blur': { BlurView: visual('RNBlurView') }, 'expo-linear-gradient': { LinearGradient: visual('RNLinearGradient') }, 'expo-status-bar': { StatusBar: visual('RNStatusBar') },
    'expo-router': { Redirect: visual('RNRouterRedirect'), useLocalSearchParams: () => state.params, usePathname: () => surface === 'profile' ? '/profile' : surface === 'detail' ? `/community/posts/${state.params.id}` : '/community', useRouter: () => router, useFocusEffect: callback => React.useEffect(callback, [callback]) },
    'expo-router/build/react-navigation/core/NavigationContext': { NavigationContext: React.createContext(null) },
    'react-native-safe-area-context': { SafeAreaView: visual('RNSafeAreaView'), useSafeAreaInsets: () => ({ top: 24, bottom: 12, left: 0, right: 0 }) },
    '@expo/vector-icons': { Ionicons: visual('RNIonicons') }, 'react-native-svg': svg,
    'react-native-gesture-handler': { GestureHandlerRootView: visual('RNGestureHandlerRootView'), GestureDetector: visual('RNGestureDetector'), Gesture: { Pan: gestureChain, Pinch: gestureChain, Tap: gestureChain, Race: gestureChain, Simultaneous: gestureChain, Exclusive: gestureChain } },
    'expo-haptics': { ImpactFeedbackStyle: { Medium: 'medium' }, NotificationFeedbackType: { Success: 'success' }, selectionAsync() {}, impactAsync() {}, notificationAsync() {} },
    'expo-image-picker': { launchImageLibraryAsync: (...args) => { const pending = deferred(); picks.push({ ...pending, args }); return pending.promise; } },
    'expo-image-manipulator': { SaveFormat: { JPEG: 'jpeg' }, manipulateAsync: async (...args) => { nativeHostCalls.push({ kind: 'manipulateAsync', args }); return { uri: 'file:///synthetic-clean.jpg', width: 32, height: 32 }; } },
  };
  class SyntheticSocket { constructor(url) { this.url = url; sockets.push(this); } send(value) { nativeHostCalls.push({ socket: value }); } close() { this.closed = true; } }
  const globals = {
    console, process: { env: {} }, URL, URLSearchParams, AbortController, WebSocket: SyntheticSocket,
    setTimeout: callback => { const id = ++timerId; timers.set(id, callback); return id; }, clearTimeout: id => timers.delete(id),
    requestAnimationFrame: callback => { const id = ++timerId; timers.set(id, callback); return id; }, cancelAnimationFrame: id => timers.delete(id),
    fetch: async (uri, options) => {
      if (String(uri).startsWith('file:///')) return { blob: async () => ({ size: 8, type: 'image/jpeg', slice() { return { size: 8, type: 'image/jpeg', close() {} }; }, close() {} }) };
      if (uri === api.BASE_URL + '/v2/community/media') { const response = await parked(uploads, { uri, options }); return { ok: true, json: async () => response }; }
      throw new Error(`Unexpected synthetic fetch ${uri}`);
    },
  };
  function load(relative) {
    relative = relative.replace(/\\/g, '/').replace(/\.(tsx?|js)$/, '');
    if (Object.hasOwn(adapters, relative)) return adapters[relative];
    if (cache.has(relative)) return cache.get(relative).exports;
    const variants = /\.(json|mjs|png|jpe?g|webp)$/.test(relative) ? [''] : ['.ts', '.tsx', '/index.ts', '/index.tsx'];
    const file = variants.map(ext => path.resolve(sourceRoot, relative + ext)).find(fs.existsSync);
    assert.ok(file && (file === path.resolve(sourceRoot) || file.startsWith(path.resolve(sourceRoot) + path.sep)), `Strict frozen source must exist: ${relative}`);
    const canonicalRoot = fs.realpathSync(sourceRoot), canonicalFile = fs.realpathSync(file);
    assert.ok(canonicalFile.startsWith(canonicalRoot + path.sep), `Frozen source cannot escape through a symlink: ${relative}`);
    const bytes = fs.readFileSync(file); consumed.set(relative, file); consumedHashes.set(relative, crypto.createHash('sha256').update(bytes).digest('hex'));
    const module = { exports: {} }; cache.set(relative, module);
    if (/\.(png|jpe?g|webp)$/.test(file)) { module.exports = 1; return module.exports; }
    if (file.endsWith('.json')) { module.exports = JSON.parse(bytes.toString('utf8')); return module.exports; }
    // Shared .mjs bytes remain exact source; a .js compiler filename requests
    // the CommonJS VM adapter rather than extension-implied ESM output.
    const output = ts.transpileModule(bytes.toString('utf8'), { fileName: file.endsWith('.mjs') ? file.slice(0, -4) + '.js' : file, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true, allowJs: true }, reportDiagnostics: true });
    assert.deepEqual((output.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error).map(item => ({ code: item.code, start: item.start, message: ts.flattenDiagnosticMessageText(item.messageText, '\n') })), [], `Syntax diagnostics: ${relative}`);
    function req(specifier) {
      if (Object.hasOwn(externals, specifier)) return externals[specifier];
      assert.ok(specifier.startsWith('.'), `Unmapped external boundary ${specifier} from ${relative}`);
      return load(path.posix.normalize(path.posix.join(path.posix.dirname(relative), specifier)));
    }
    vm.runInNewContext(`(function(require,module,exports){${output.outputText}\n})`, globals, { filename: file })(req, module, module.exports);
    return module.exports;
  }
  const Session = load('apps/mobile/src/rudi/session');
  const metrics = load('apps/mobile/src/rudi/ui/NativeFontMetrics');
  const comments = load('apps/mobile/src/rudi/community/Comments');
  const Field = load('apps/mobile/src/rudi/ui/Field').Field;
  const NepProvider = surface === 'profile' ? load('apps/mobile/src/rudi/nep/NepProvider').NepProvider : null;
  const Component = surface === 'profile' ? load('apps/mobile/src/rudi/screens/Profile').ProfileScreen : surface === 'detail' ? load('apps/mobile/src/rudi/community/PostDetail').PostDetail : surface === 'feed' ? load('apps/mobile/src/rudi/community/CommunityScreen').CommunityScreen : comments.Comments;
  const PostCard = surface === 'detail' || surface === 'feed' ? load('apps/mobile/src/rudi/community/PostCard').PostCard : null;
  const Sheet = surface === 'detail' || surface === 'feed' ? load('apps/mobile/src/rudi/ui/Sheet').Sheet : null;
  function BoundaryProbe() {
    const [value, setValue] = React.useState('Synthetic boundary draft');
    const owned = metrics.useNativeFontMetricsOwner();
    return React.createElement('OwnerProbe', { owned }, React.createElement(metrics.Text, { testID: 'owner-probe-text', style: { fontSize: 17, lineHeight: 24 } }, 'Synthetic boundary text'), React.createElement(Field, { testID: 'owner-probe-input', nativeMetrics: true, multiline: true, numberOfLines: 3, value, onChangeText: setValue, oRef: value => refs.push(value) }), state.boundaryChildren?.({ React, load, metrics }));
  }
  function SessionProbe() { session = Session.useRudiSession(); return null; }
  function App() {
    let element;
    if (surface === 'comments') element = React.createElement(Component, { person: state.actor.person_id, post: state.post, onDraftChange: draft => drafts.push(draft), ...state.commentsProps });
    else if (surface === 'boundary') element = React.createElement(comments.CommunityFontMetrics, null, React.createElement(BoundaryProbe));
    else element = React.createElement(Component);
    if (state.explicitProvider) element = React.createElement(metrics.NativeFontMetricsProvider, { snapshot: state.nativeFont }, element);
    const application = surface === 'profile' ? React.createElement(NepProvider, null, state.show ? element : null) : state.show ? element : null;
    const content = React.createElement(Session.RudiSessionProvider, null, React.createElement(SessionProbe), application);
    return strict ? React.createElement(React.StrictMode, null, content) : content;
  }
  function hosts(kind, match = () => true) { assert.ok(root, 'mounted host'); return root.root.findAll(node => node.type === kind && match(node.props)); }
  function one(kind, match) { const values = hosts(kind, match); assert.equal(values.length, 1, `One ${kind} host`); return values[0]; }
  const h = {
    state, consumed, consumedHashes, profileReads, profileWrites, sessionWrites, savedReads, storageCalls, boundaryViolations, calls, drafts, refs, routes, sockets, picks, uploads, writes, gets, reads, friendReads, deletes, nativeHostCalls, callbacks, retiredCallbacks, timings, load, metrics, React, Native, PostCard, Sheet,
    get root() { return root; },
    get session() { return session; },
    apiError(message, status = 422) { return new api.ApiError(status, message); },
    observation() { return { live: listeners.size, subscriptions, removals, reads: getterReads, maxLive }; },
    hosts, one,
    input() { return one('RNTextInput', props => props.testID === (surface === 'boundary' ? 'owner-probe-input' : 'community-comment-input')); },
    profileInputs() { return hosts('RNTextInput', props => ['Ô tên hiển thị', 'Ô giới thiệu', 'Ô thành phố'].includes(props.accessibilityLabel)); },
    profileInput(label) { assert.ok(['Ô tên hiển thị', 'Ô giới thiệu', 'Ô thành phố'].includes(label), 'explicit profile input label'); return one('RNTextInput', props => props.accessibilityLabel === label); },
    async changeProfile(label, value) { await act(async () => h.profileInput(label).props.onChangeText(value)); },
    text(match = () => true) { return hosts('RNText', props => typeof match === 'string' ? props.children === match : match(props)); },
    textContent() { return hosts('RNText').map(node => node.props.children).filter(value => typeof value === 'string').join('\n'); },
    owner(name) { const values = root.root.findAll(node => typeof node.type === 'function' && node.type.name === name); assert.equal(values.length, 1, `One ${name} owner`); return values[0]; },
    editors() { return root.root.findAllByType(comments.Comments); },
    writer() { const values = h.editors(); assert.equal(values.length, 1, 'one Comments writer'); return values[0].props.onDraftChange; },
    cards() { return root.root.findAll(node => typeof node.type === 'function' && node.type.name === 'PostCard'); },
    commentSheet() { const values = root.root.findAllByType(Sheet).filter(node => node.props.accessibilityLabel === 'Bình luận'); assert.equal(values.length, 1, 'one actual comment Sheet'); return values[0]; },
    panel() { return one('RNAnimatedView', props => props.accessibilityLabel === 'Bình luận'); },
    callsFor(kind) { return calls.filter(call => call.kind === kind); },
    async mount() { await act(async () => { root = Renderer.create(React.createElement(App), { createNodeMock: element => ({ id: ++nodeId, kind: element.type, focus() { focused = this; }, blur() { if (focused === this) focused = null; }, scrollTo(options) { nativeHostCalls.push({ kind: 'scrollTo', options }); }, scrollToOffset(options) { nativeHostCalls.push({ kind: 'scrollToOffset', options }); }, getNativeScrollRef() { return this; }, measureInWindow(callback) { callback(0, 0, 390, 400); } }) }); }); },
    async update(callback = () => {}) { await act(async () => { callback(state); root.update(React.createElement(App, { tick: ++state.tick })); }); },
    async nativeFont(value) { await act(async () => { state.nativeFont = value; [...listeners].forEach(callback => callback()); if (state.explicitProvider) root.update(React.createElement(App, { tick: ++state.tick })); }); },
    async invokeRetired(index = retiredCallbacks.length - 1) { assert.ok(retiredCallbacks[index]); await act(async () => retiredCallbacks[index]()); },
    async change(value) { await act(async () => h.input().props.onChangeText(value)); },
    async focus() { await act(async () => h.input().props.onFocus?.({ nativeEvent: { synthetic: true } })); },
    async pressAccessibility(label) {
      // A legacy semantic tool step now follows the actual disclosure UI first.
      const commentTool = ['comments', 'detail'].includes(surface) && (label === 'Thêm ảnh' || label.startsWith('Gắn bạn bè'));
      if (commentTool && hosts('RNPressable', props => props.accessibilityLabel === label).length === 0) {
        const disclosure = one('RNPressable', props => /^(Thêm vào bình luận|Đóng công cụ bình luận)$/.test(props.accessibilityLabel));
        if (disclosure.props.disabled) return false;
        if (!disclosure.props.accessibilityState?.expanded) await act(async () => disclosure.props.onPress());
      }
      await act(async () => one('RNPressable', props => props.accessibilityLabel === label).props.onPress());
      return true;
    },
    async pressText(label) { const values = hosts('RNPressable').filter(node => node.findAllByType('RNText').some(text => text.props.children === label)); assert.equal(values.length, 1, `One pressable ${label}`); await act(async () => values[0].props.onPress()); },
    async openComments(index = 0) { const values = h.cards(); assert.ok(values[index], 'actual PostCard exists'); await act(async () => values[index].props.onComment()); },
    async closeComments() { await act(async () => h.commentSheet().props.onClose()); },
    closeAnimation() { const panel = h.panel(); const ids = [panel.props.style].flat(Infinity).flatMap(style => style?.syntheticSharedIds ?? []); const event = timings.filter(item => ids.includes(item.shared) && item.target === 0).at(-1); assert.ok(event?.callback, 'actual Sheet native completion'); return event; },
    async completeClose(event = h.closeAnimation(), finished = true) { await act(async () => event.callback(finished)); },
    deferProfileRead() { const queued = { ...deferred(), started: false }; profileReadQueue.push(queued); return queued; },
    deferSavedRead() { const queued = { ...deferred(), started: false }; savedReadQueue.push(queued); return queued; },
    deferCommentsRead() { const queued = { ...deferred(), started: false }; commentsReadQueue.push(queued); return queued; },
    deferFriendsRead() { const queued = { ...deferred(), started: false }; friendsReadQueue.push(queued); return queued; },
    async resolvePost(value = state.post, index = gets.findIndex(call => typeof call.resolve === 'function')) { assert.ok(gets[index]?.resolve, 'deferred actual getPost transport'); await act(async () => gets[index].resolve(value)); },
    async rejectPost(error, index = gets.findIndex(call => typeof call.reject === 'function')) { assert.ok(gets[index]?.reject); await act(async () => gets[index].reject(error)); },
    async send() { const button = one('RNPressable', props => /^(Gửi bình luận|Đang gửi…)$/.test(props.accessibilityLabel)); assert.equal(Boolean(button.props.disabled), false, 'send is enabled'); await act(async () => button.props.onPress()); },
    async resolveWrite(index, value = { id: 'synthetic-accepted', status: 'pending' }) { assert.ok(writes[index]); await act(async () => writes[index].resolve(value)); },
    async rejectWrite(index, error = new Error('Synthetic response lost')) { assert.ok(writes[index]); await act(async () => writes[index].reject(error)); },
    async finish(call, value) { assert.ok(call?.resolve); await act(async () => call.resolve(value)); },
    async fail(call, error) { assert.ok(call?.reject); await act(async () => call.reject(error)); },
    async actorTo(id) { await act(async () => { state.actor = person(id); session.datPhien(state.actor); root.update(React.createElement(App, { tick: ++state.tick })); }); },
    async dispose() { if (root) { const current = root; root = null; await act(async () => current.unmount()); } timers.clear(); assert.deepEqual(boundaryViolations, [], 'No unknown synthetic native/storage boundary requests'); },
  };
  return h;
}

function createProfileHost(options = {}) { return createHost({ ...options, surface: 'profile' }); }

module.exports = { createProfileHost, snapshot, flatten, act, sourceRoot };
