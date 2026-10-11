/** Real source/React host; only native, routing, API and storage edges are replaced. */
import assert from "node:assert/strict";
import { existsSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { createRequire, registerHooks } from "node:module";
import { dirname, resolve, extname } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const repo = process.env.RUDI_TEST_REPO ?? resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const overlay = process.env.RUDI_TEST_SOURCE_ROOT;
const require = createRequire(resolve(repo, "apps/mobile/package.json"));
export const React = require("react");
const TestRenderer = require("react-test-renderer");
const ts = require("typescript");
export const { act } = React;
const key = Symbol.for("rudi.n.behavior.host");
const parameter = "rudi-n-behavior-case";
const cases = new Map();
const oldRegistry = globalThis[key];
const oldActEnvironment = globalThis.IS_REACT_ACT_ENVIRONMENT;
globalThis[key] = cases;
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
let serial = 0;
const receipts = [];
const emptySubscription = () => ({ remove() {} });

function flatten(styles) {
  if (!styles || typeof styles !== "object") return undefined;
  if (!Array.isArray(styles)) return styles;
  return Object.assign({}, ...styles.map(flatten).filter(Boolean));
}
function host(name, observe) {
  return React.forwardRef(function Host(props, ref) {
    observe?.(name, props);
    return React.createElement(name, { ...props, ref });
  });
}
function nativeBoundary(observe) {
  const rn = Object.fromEntries(["Text", "TextInput", "View", "Pressable", "ScrollView", "Image", "ActivityIndicator", "KeyboardAvoidingView", "RefreshControl", "FlatList", "Modal"].map(name => [name, host(`RN${name}`, observe)]));
  Object.assign(rn, {
    StyleSheet: { create: value => value, flatten, hairlineWidth: 0.5, absoluteFill: { position: "absolute", top: 0, left: 0, right: 0, bottom: 0 } },
    Platform: { OS: "android", select: choices => choices.android ?? choices.native ?? choices.default },
    Appearance: { getColorScheme: () => "light", addChangeListener: emptySubscription },
    useWindowDimensions: () => ({ width: 360, height: 800, scale: 2, fontScale: 1 }),
    Dimensions: { get: () => ({ width: 360, height: 800, scale: 2, fontScale: 1 }), addEventListener: emptySubscription },
    PixelRatio: { get: () => 2, getFontScale: () => 1, roundToNearestPixel: value => Math.round(value * 2) / 2 },
    AccessibilityInfo: { isReduceMotionEnabled: async () => true, addEventListener: emptySubscription },
    Keyboard: { addListener: emptySubscription, dismiss() {}, isVisible: () => false },
    BackHandler: { addEventListener: emptySubscription },
    Linking: { canOpenURL: async () => true, openURL: async () => undefined, addEventListener: emptySubscription },
    AppState: { currentState: "active", addEventListener: emptySubscription },
    NativeModules: {}, NativeEventEmitter: class { addListener() { return emptySubscription(); } },
    findNodeHandle: () => null,
    Share: { sharedAction: "sharedAction", dismissedAction: "dismissedAction", share: async () => ({ action: "dismissedAction" }) },
  });
  rn.TextInput.State = { currentlyFocusedInput: () => null };
  return rn;
}
function animationBoundary(rn) {
  const animation = new Proxy({}, { get: () => () => animation });
  const component = { View: rn.View, Text: rn.Text, ScrollView: rn.ScrollView, Image: rn.Image, createAnimatedComponent: value => value };
  return {
    default: component,
    useSharedValue: value => React.useRef({ value }).current,
    useDerivedValue: get => ({ value: get() }),
    useAnimatedStyle: get => get(), useAnimatedScrollHandler: fn => fn,
    useAnimatedReaction: () => undefined, useReducedMotion: () => true,
    withSpring: value => value, withTiming: value => value, withDelay: (_delay, value) => value, withSequence: (...values) => values.at(-1), withRepeat: value => value,
    runOnJS: fn => fn, cancelAnimation() {}, interpolate: value => value,
    Easing: { bezier: () => value => value, linear: value => value },
    ReduceMotion: { Always: "always", Never: "never", System: "system" },
    Extrapolation: { CLAMP: "clamp" }, FadeIn: animation, FadeOut: animation, LinearTransition: animation, Layout: animation,
  };
}
function identity(url) {
  if (!url?.startsWith("file:")) return null;
  const parsed = new URL(url);
  const id = parsed.searchParams.get(parameter);
  if (id === null) return null;
  parsed.search = "";
  return { id, path: fileURLToPath(parsed), url: parsed.href };
}
function version(path, id) {
  const url = pathToFileURL(path);
  url.searchParams.set(parameter, id);
  return url.href;
}
function effective(path) {
  if (!overlay) return path;
  const copy = resolve(overlay, path.slice(repo.length + 1));
  return existsSync(copy) ? copy : path;
}
const apiPath = resolve(repo, "apps/mobile/src/api.ts");
const identityPath = resolve(repo, "apps/mobile/src/danh-tinh.ts");
const sessionStoragePath = resolve(repo, "apps/mobile/src/phien.ts");
const localStorePath = resolve(repo, "apps/mobile/src/rudi/kho.ts");

function externalSource(id, specifier, names) {
  const lookup = `globalThis[Symbol.for(${JSON.stringify(Symbol.keyFor(key))})].get(${JSON.stringify(id)}).external(${JSON.stringify(specifier)})`;
  return `const b=${lookup};\n` + [...names].map(name => name === "default" ? "export default b.default;" : `export const ${name}=b[${JSON.stringify(name)}];`).join("\n");
}
function requestNames(source, state) {
  const sf = ts.createSourceFile("source.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  for (const node of sf.statements) {
    if (!ts.isImportDeclaration(node) && !ts.isExportDeclaration(node)) continue;
    if (!node.moduleSpecifier || !ts.isStringLiteral(node.moduleSpecifier)) continue;
    const specifier = node.moduleSpecifier.text;
    if (specifier.startsWith(".") || specifier === "react" || specifier.startsWith("react/")) continue;
    const names = state.names.get(specifier) ?? new Set();
    if (ts.isImportDeclaration(node)) {
      const clause = node.importClause;
      if (!clause || clause.isTypeOnly) continue;
      if (clause.name) names.add("default");
      if (clause.namedBindings && ts.isNamedImports(clause.namedBindings)) {
        for (const el of clause.namedBindings.elements) if (!el.isTypeOnly) names.add((el.propertyName ?? el.name).text);
      }
      if (clause.namedBindings && ts.isNamespaceImport(clause.namedBindings)) names.add("default");
    } else if (node.exportClause && ts.isNamedExports(node.exportClause)) {
      for (const el of node.exportClause.elements) if (!el.isTypeOnly) names.add((el.propertyName ?? el.name).text);
    }
    state.names.set(specifier, names);
  }
}
const loader = registerHooks({
  resolve(specifier, context, next) {
    const parent = identity(context.parentURL);
    if (!parent) return next(specifier, context);
    if (specifier === "react" || specifier.startsWith("react/")) return { shortCircuit: true, url: pathToFileURL(require.resolve(specifier)).href };
    if (specifier.startsWith(".")) {
      const target = resolve(dirname(parent.path), specifier);
      const path = [target, `${target}.ts`, `${target}.tsx`, `${target}.mjs`, `${target}/index.ts`, `${target}/index.tsx`].find(item => existsSync(item) && statSync(item).isFile());
      assert.ok(path, `unresolved actual source: ${specifier} from ${parent.path}`);
      if ([apiPath, identityPath, sessionStoragePath, localStorePath].includes(path)) {
        return { shortCircuit: true, url: `rudi-n-boundary:${parent.id}/${encodeURIComponent(path)}` };
      }
      return { shortCircuit: true, url: version(path, parent.id) };
    }
    // Each importer gets its requested export set; later graph discovery cannot
    // invalidate an already instantiated ESM namespace.
    const exports = [...(cases.get(parent.id).names.get(specifier) ?? [])].join(",");
    return { shortCircuit: true, url: `rudi-n-native:${parent.id}/${encodeURIComponent(specifier)}?exports=${encodeURIComponent(exports)}` };
  },
  load(url, context, next) {
    const boundary = /^rudi-n-(?:native|boundary):(\d+)\/(.+)$/.exec(url);
    if (boundary) {
      const state = cases.get(boundary[1]);
      const [encoded, exportQuery] = boundary[2].split("?exports=");
      const specifier = decodeURIComponent(encoded);
      const names = specifier.startsWith("/") ? state.internalNames(specifier) : new Set(decodeURIComponent(exportQuery ?? "").split(",").filter(Boolean));
      for (const name of Object.keys(state.external(specifier))) names.add(name);
      return { shortCircuit: true, format: "module", source: externalSource(boundary[1], specifier, names) };
    }
    const target = identity(url);
    if (!target) return next(url, context);
    const state = cases.get(target.id);
    const path = effective(target.path);
    let source = readFileSync(path, "utf8");
    state.consumed.set(target.path.slice(repo.length + 1), path);
    if (extname(path) === ".json") return { shortCircuit: true, format: "module", source: `export default ${source};` };
    if (extname(path) === ".mjs" || extname(path) === ".js") return { shortCircuit: true, format: "module", source };
    const result = ts.transpileModule(source, { fileName: path, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX }, reportDiagnostics: true });
    assert.deepEqual((result.diagnostics ?? []).filter(item => item.category === ts.DiagnosticCategory.Error).map(d => ({ code: d.code, start: d.start, message: ts.flattenDiagnosticMessageText(d.messageText, '\n') })), [], `syntax diagnostics: ${path}`);
    source = result.outputText;
    requestNames(source, state);
    // Metro's static bundled asset require is an RN asset boundary; no code require is accepted.
    source = source.replace(/\brequire\((['"])([^'"]+)\1\)/g, (match, _quote, asset) => {
      assert.match(asset, /\.(?:png|jpg|jpeg|webp|ttf|otf|svg)$/i, `unexpected code require in ${path}`);
      return JSON.stringify(`native-asset:${asset}`);
    });
    return { shortCircuit: true, format: "module", source };
  },
});
const oldError = console.error;
console.error = (...args) => {
  if (String(args[0]).includes("react-test-renderer is deprecated")) return;
  oldError(...args);
};

export function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
export async function withHost(run) {
  const id = String(++serial);
  const nativeRenders = [];
  const rn = nativeBoundary((type, props) => { if (type === "RNText") nativeRenders.push({ type, label: props.accessibilityLabel, children: props.children }); });
  const storage = new Map();
  const events = [];
  const consumed = new Map();
  const names = new Map();
  const pendingFocus = new Set();
  const nativeFontListeners = new Set();
  let focused = true;
  let root, sessionApi;
  const state = {
    names, consumed,
    nativeSnapshot: null,
    api: async (path, options) => { throw new Error(`unexpected synthetic API call: ${options?.method ?? "POST"} ${path}`); },
    finance: async path => { throw new Error(`unexpected finance read: ${path}`); },
    restoredSession: { person_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", context_id: null, membership_state: null, token: "synthetic-host-only" },
    internalNames(path) {
      const source = readFileSync(path, "utf8");
      return new Set([...source.matchAll(/export\s+(?:async\s+)?(?:function|class|const|let|var)\s+(\w+)/g)].map(match => match[1]).concat(path === apiPath ? ["datTokenPhien", "tokenPhienHienTai"] : []));
    },
    external(specifier) {
      if (specifier === "react-native") return rn;
      if (specifier === "react-native-reanimated") return animationBoundary(rn);
      if (specifier === "expo-router") return {
        useRouter: () => ({ push: path => events.push(["push", path]), back() {}, canGoBack: () => true }),
        usePathname: () => "/finance", useNavigation: () => ({ addListener: emptySubscription }),
        useLocalSearchParams: () => ({}),
        useFocusEffect(callback) {
          React.useEffect(() => {
            const entry = { callback, cleanup: focused ? callback() : undefined };
            pendingFocus.add(entry);
            return () => { entry.cleanup?.(); pendingFocus.delete(entry); };
          }, [callback]);
        },
      };
      if (specifier.includes("NavigationContext")) return { NavigationContext: React.createContext(null) };
      if (specifier === "react-native-safe-area-context") return { SafeAreaView: rn.View, useSafeAreaInsets: () => ({ top: 0, left: 0, right: 0, bottom: 0 }) };
      if (specifier === "@react-native-async-storage/async-storage") return { default: {
        async getItem(key) { events.push(["storage-read", key]); return storage.get(key) ?? null; },
        async setItem(key, value) { events.push(["storage-write", key, JSON.parse(value)]); storage.set(key, value); },
        async removeItem(key) { storage.delete(key); },
      } };
      if (specifier === apiPath) {
        class ApiError extends Error { constructor(status, code, detail) { super(detail); this.status = status; this.code = code; } }
        return new Proxy({ BASE_URL: "https://synthetic.invalid", ApiError,
          translatedAsActor: (_messages, path, options) => state.api(path, options),
          attemptFor: (attempts, id) => attempts[id] ??= { key: `synthetic-${id}`, at: 0 },
          thongDiepNguoiDoc: () => "Không đọc được dữ liệu thử.",
          datTokenPhien() {}, tokenPhienHienTai: () => "synthetic-host-only",
        }, { get: (target, name) => name in target ? target[name] : (..._args) => { throw new Error(`unexpected API boundary function: ${String(name)}`); } });
      }
      if (specifier === identityPath) return {
        headerNguoiGoi: actorId => ({ "X-Actor-ID": actorId }),
        actorHeaders: actorId => ({ "X-Actor-ID": actorId }),
        ngheThuHoiPhien: () => () => undefined,
        datTokenPhien() {}, tokenPhienHienTai: () => "synthetic-host-only",
      };
      if (specifier === sessionStoragePath) return { khoiPhucPhien: async () => state.restoredSession, dangXuat: async () => undefined, quenPhienDaThuHoi: async () => undefined };
      if (specifier === localStorePath) return new Proxy({}, { get: (_target, name) => name === "xoaPhienAsync" ? async () => undefined : () => { throw new Error(`unexpected storage function: ${String(name)}`); } });
      if (specifier === "expo-haptics") return { selectionAsync: async () => {}, impactAsync: async () => {}, notificationAsync: async () => {}, ImpactFeedbackStyle: { Medium: "medium" }, NotificationFeedbackType: { Success: "success" } };
      if (specifier === "expo-constants") return { default: { expoConfig: { extra: {} }, executionEnvironment: "host-test" } };
      if (specifier === "expo-modules-core") return { requireOptionalNativeModule: name => {
        assert.equal(name, "RudiFontScale", "only the explicitly exercised native module may be requested");
        if (state.nativeSnapshot === null) return null;
        return {
          getFontScale: () => state.nativeSnapshot,
          addListener(event, callback) {
            assert.equal(event, "onFontScaleChanged");
            nativeFontListeners.add(callback);
            return { remove() { nativeFontListeners.delete(callback); } };
          },
        };
      } };
      if (specifier === "expo-image") return { Image: host("ExpoImage") };
      if (specifier === "expo-blur") return { BlurView: host("BlurView") };
      if (specifier === "expo-linear-gradient") return { LinearGradient: host("LinearGradient") };
      if (specifier === "expo-status-bar") return { StatusBar: host("StatusBar") };
      if (specifier === "@expo/vector-icons") return { Ionicons: host("Ionicons"), MaterialCommunityIcons: host("MaterialCommunityIcons") };
      if (specifier === "react-native-svg") return new Proxy({ default: host("Svg") }, { get: (target, name) => target[name] ??= host(`Svg${String(name)}`) });
      if (specifier === "react-native-gesture-handler") return { GestureHandlerRootView: rn.View };
      if (specifier === "expo-secure-store") return { getItemAsync: async () => null, setItemAsync: async () => undefined, deleteItemAsync: async () => undefined };
      throw new Error(`unsupported native boundary: ${specifier}`);
    },
  };
  cases.set(id, state);
  const oldFetch = globalThis.fetch;
  globalThis.fetch = (...args) => state.finance(...args);
  const h = {
    state, storage, events, consumed, rn, nativeRenders,
    async load(path) { return import(version(resolve(repo, path), id)); },
    async render(element) {
      await act(async () => { if (root) root.update(element); else root = TestRenderer.create(element); });
    },
    get root() { return root.root; },
    all(type) { return root.root.findAllByType(type); },
    label(node) { return node.props.accessibilityLabel ?? node.findAllByType("RNText").map(text => text.children.filter(child => typeof child === "string").join("")).join(""); },
    texts() { return root.root.findAllByType("RNText").map(node => node.children.filter(child => typeof child === "string").join("")); },
    byId(testID) { return root.root.findAll(node => typeof node.type === "string" && node.props.testID === testID); },
    button(label) {
      const found = root.root.findAllByType("RNPressable").filter(node => h.label(node) === label);
      assert.equal(found.length, 1, `one rendered production button: ${label}`);
      return found[0];
    },
    async press(label) { await act(async () => { h.button(label).props.onPress({}); }); },
    async flush() { await act(async () => { await Promise.resolve(); }); },
    get nativeFontObservers() { return nativeFontListeners.size; },
    async font(snapshot) {
      await act(async () => { state.nativeSnapshot = snapshot; for (const callback of nativeFontListeners) callback(); });
    },
    async focus(value) {
      await act(async () => {
        if (value === focused) return;
        focused = value;
        for (const entry of pendingFocus) {
          if (value) entry.cleanup = entry.callback();
          else { entry.cleanup?.(); entry.cleanup = undefined; }
        }
      });
    },
    async financeElement() {
      const { FinanceScreen } = await h.load("apps/mobile/src/rudi/screens/Profile.tsx");
      const { RudiSessionProvider, useRudiSession } = await h.load("apps/mobile/src/rudi/session.tsx");
      function Probe() { sessionApi = useRudiSession(); return React.createElement(FinanceScreen); }
      return React.createElement(RudiSessionProvider, null, React.createElement(Probe));
    },
    async actor(person_id) { await act(async () => { sessionApi.datPhien({ ...state.restoredSession, person_id }); }); },
  };
  try { await run(h); }
  finally {
    if (root) await act(async () => { root.unmount(); });
    globalThis.fetch = oldFetch;
    receipts.push({ case: id, sources: [...consumed].sort().map(([logical, path]) => ({ logical, path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") })), external_boundaries: [...names.keys()].sort() });
    cases.delete(id);
  }
}
export function closeHost() {
  loader.deregister();
  if (oldRegistry === undefined) delete globalThis[key]; else globalThis[key] = oldRegistry;
  console.error = oldError;
  if (oldActEnvironment === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT; else globalThis.IS_REACT_ACT_ENVIRONMENT = oldActEnvironment;
  if (process.env.RUDI_TEST_RECEIPTS) writeFileSync(process.env.RUDI_TEST_RECEIPTS, JSON.stringify({ repo, overlay: overlay ?? null, cases: receipts }, null, 2) + "\n", { mode: 0o600 });
}
