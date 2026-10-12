// Settings-only package, session and I/O boundaries; no product component substitution.
const assert = require('node:assert/strict');
const BASES = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6, 11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34];
const CHOICES = ['sang', 'toi', 'he-thong'];
const REASONS = ['unsupported-platform', 'module-unavailable', 'module-inactive', 'context-unavailable', 'observer-unavailable', 'invalid-native-value', 'read-failed', 'subscription-failed'];
const ROUTES = ['/personalization', '/settings/account', '/settings/phien', '/settings/thiet-bi', '/settings/da-chan', '/settings/ve-rudi', '/settings/xoa-tai-khoan'];

function createSettingsBoundary({ state, api, profileRead, routes, initialTheme, systemScheme }) {
  assert.equal(CHOICES.includes(initialTheme), true, 'explicit Settings theme fixture');
  assert.equal(['light', 'dark'].includes(systemScheme), true, 'explicit system appearance fixture');
  state.profile = { wall_comment_policy: 'readers', ...state.profile };
  const ioState = { theme: initialTheme, systemScheme };
  const trace = [], nativeTrace = [], violations = [], retiredAppearance = [];
  const listeners = { font: new Set(), appearance: new Set(), accessibility: new Set(), session: new Set() };
  let appearanceSubscriptions = 0, appearanceRemovals = 0, eventId = 0;
  function check(kind, callback) {
    try { return callback(); } catch (error) {
      violations.push({ kind, message: String(error.message ?? error) });
      throw error;
    }
  }
  function forbidden(kind, value) { return check(kind, () => { throw new Error(`Unexpected Settings ${kind}: ${String(value)}`); }); }
  function closed(name, values) {
    return new Proxy({ __esModule: true, ...values }, {
      get(target, key) {
        if (Object.hasOwn(target, key)) return target[key];
        return forbidden(`${name}.method`, typeof key === 'string' ? key : 'symbol');
      },
    });
  }
  function actor(actorId) {
    assert.equal(actorId, state.actor.person_id);
    assert.equal(actorId, 'synthetic-A', 'only the declared Settings actor');
    assert.equal(state.actor.token, `synthetic-token-${actorId}`);
    assert.equal(state.token, state.actor.token, 'actual Session published the active bearer');
  }
  function subscribe(kind, name, callback, delegate) {
    return check(`${kind}.subscribe`, () => {
      assert.equal(name, { font: 'onFontScaleChanged', appearance: 'change', accessibility: 'reduceMotionChanged', session: 'revoked' }[kind]);
      assert.equal(typeof callback, 'function');
      const id = ++eventId;
      const subscription = delegate?.();
      if (subscription) assert.equal(typeof subscription.remove, 'function');
      listeners[kind].add(callback);
      nativeTrace.push({ kind, method: 'subscribe', name, id });
      if (kind === 'appearance') appearanceSubscriptions += 1;
      let live = true;
      return { remove() { return check(`${kind}.remove`, () => {
        assert.equal(live, true, 'each boundary subscription is removed once'); live = false;
        subscription?.remove(); listeners[kind].delete(callback);
        nativeTrace.push({ kind, method: 'remove', name, id });
        if (kind === 'appearance') { appearanceRemovals += 1; retiredAppearance.push(callback); }
      }); } };
    });
  }
  function fontSnapshot(value) { return check('font.snapshot', () => {
    assert.ok(value && typeof value === 'object');
    if (value.status === 'unavailable') {
      assert.deepEqual(Object.keys(value).sort(), ['reason', 'status']);
      assert.equal(REASONS.includes(value.reason), true); return;
    }
    assert.equal(value.status, 'available');
    assert.deepEqual(Object.keys(value).sort(), value.fontMetrics === undefined ? ['fontScale', 'status'] : ['fontMetrics', 'fontScale', 'status']);
    assert.equal(Number.isFinite(value.fontScale) && value.fontScale > 0, true);
    if (value.fontMetrics === undefined) return;
    const metrics = value.fontMetrics;
    assert.deepEqual(Object.keys(metrics).sort(), ['density', 'spToDp']);
    assert.equal(Number.isFinite(metrics.density) && metrics.density > 0, true);
    assert.ok(Array.isArray(metrics.spToDp)); assert.equal(metrics.spToDp.length, BASES.length);
    for (const base of BASES) {
      const rows = metrics.spToDp.filter(row => row.base === base); assert.equal(rows.length, 1);
      const row = rows[0]; assert.deepEqual(Object.keys(row).sort(), ['base', 'dp']);
      assert.equal(Number.isFinite(row.dp) && (base > 0 ? row.dp > 0 : row.dp < 0), true);
    }
  }); }
  function nativeFont(module) { return closed('font', {
    getFontScale(...args) { return check('font.get', () => {
      assert.equal(args.length, 0); fontSnapshot(state.nativeFont);
      nativeTrace.push({ kind: 'font', method: 'get', status: state.nativeFont.status, fontScale: state.nativeFont.fontScale });
      return module.getFontScale();
    }); },
    addListener(name, callback, ...rest) { return check('font.listen', () => {
      assert.equal(rest.length, 0);
      return subscribe('font', name, callback, () => module.addListener(name, callback));
    }); },
  }); }
  const appearance = closed('Appearance', {
    getColorScheme(...args) { return check('appearance.get', () => { assert.equal(args.length, 0); nativeTrace.push({ kind: 'appearance', method: 'get' }); return ioState.systemScheme; }); },
    addChangeListener(callback, ...rest) { return check('appearance.listen', () => { assert.equal(rest.length, 0); return subscribe('appearance', 'change', callback); }); },
  });
  const session = closed('phien', {
    async khoiPhucPhien(...args) { return check('session.restore', () => {
      assert.equal(args.length, 0); assert.equal(state.actor.person_id, 'synthetic-A');
      assert.equal(state.actor.token, 'synthetic-token-synthetic-A');
      trace.push({ kind: 'session-restore', actorId: state.actor.person_id }); return state.actor;
    }); },
    async docHoSoToi(actorId, ...rest) {
      check('profile.read', () => { assert.equal(rest.length, 0); actor(actorId); trace.push({ kind: 'profile-read', actorId }); });
      const value = await profileRead(actorId);
      return check('profile.response', () => {
        assert.equal(value.id, actorId); assert.equal(typeof value.display_name, 'string');
        assert.equal(['readers', 'friends', 'nobody'].includes(value.wall_comment_policy), true); return value;
      });
    },
    async dangXuat() { return forbidden('session', 'logout'); },
    async quenPhienDaThuHoi() { return forbidden('session', 'revoke'); },
    async docNhomCuaToi() { return forbidden('session', 'groups'); },
    async suaHoSoToi() { return forbidden('profile-write', 'suaHoSoToi'); },
    async doiTenTrongPhien() { return forbidden('session-write', 'doiTenTrongPhien'); },
  });
  const kho = closed('kho', {
    async xoaPhienAsync(...args) { return check('storage.delete', () => { assert.equal(args.length, 0); trace.push({ kind: 'session-draft-delete', key: 'rudi.phien.v1' }); }); },
    async docGiaoDienAsync(key, ...rest) { return check('theme.read', () => {
      assert.equal(rest.length, 0); assert.equal(key, 'rudi.giao-dien.v1'); trace.push({ kind: 'theme-read', key }); return ioState.theme;
    }); },
    async ghiGiaoDienAsync(key, value, ...rest) { return check('theme.write', () => {
      assert.equal(rest.length, 0); assert.equal(key, 'rudi.giao-dien.v1'); assert.equal(CHOICES.includes(value), true);
      ioState.theme = value; trace.push({ kind: 'theme-write', key, value });
    }); },
  });
  const settingsApi = closed('api', {
    BASE_URL: api.BASE_URL, LOI_KHONG_NOI_DUOC: api.LOI_KHONG_NOI_DUOC, ApiError: api.ApiError,
    thongDiepNguoiDoc: api.thongDiepNguoiDoc,
    newAttempt() { return forbidden('api-write', 'newAttempt'); },
    datTokenPhien(value, ...rest) { return check('session.token', () => {
      assert.equal(rest.length, 0); assert.equal(value, state.actor.token); api.datTokenPhien(value);
      trace.push({ kind: 'session-token', actorId: state.actor.person_id });
    }); },
    tokenPhienHienTai: () => state.token,
    duongDanAnhDaiDien(id) { return check('avatar.path', () => { actor(id); return api.duongDanAnhDaiDien(id); }); },
    async translatedAsActor(_errors, route, options) { return forbidden('transport', `${options?.method ?? 'GET'} ${route}`); },
    async translatedAnonymous(_errors, route, options) { return forbidden('anonymous-transport', `${options?.method ?? 'GET'} ${route}`); },
    async taiAnhDaiDien() { return forbidden('avatar-write', 'taiAnhDaiDien'); },
  });
  const danhTinh = closed('danh-tinh', {
    ngheThuHoiPhien(callback, ...rest) { return check('session.listen', () => {
      assert.equal(rest.length, 0); state.revoke = callback;
      const subscription = subscribe('session', 'revoked', callback);
      return () => { subscription.remove(); state.revoke = null; };
    }); },
    tokenPhienHienTai: () => state.token,
    headerNguoiGoi(actorId) { return check('actor.header', () => { actor(actorId); return { Authorization: `Bearer ${state.token}` }; }); },
  });
  function navigation(method, value) { return check('navigation', () => {
    assert.equal(method === 'push' && ROUTES.includes(value) || method === 'replace' && value === '/profile', true, 'only exact declared Settings routes and cold Back fallback');
    trace.push({ kind: 'navigation', method, value }); routes.push(value);
  }); }
  const router = closed('router', {
    canGoBack(...args) { return check('navigation.history', () => { assert.equal(args.length, 0); return false; }); },
    back() { return forbidden('navigation', 'back-without-history'); },
    push: value => navigation('push', value), replace: value => navigation('replace', value),
  });
  const asyncStorage = { __esModule: true, default: closed('AsyncStorage', Object.fromEntries(['getItem', 'setItem', 'removeItem'].map(method => [method, async (...args) => forbidden(`AsyncStorage.${method}`, args[0])]))) };
  const File = new Proxy(class UnexpectedSettingsFile {
    constructor(...args) { return forbidden('File.construct', args[0]); }
  }, { get(_target, key) { return forbidden('File.static', String(key)); } });
  const imagePicker = closed('ImagePicker', { async launchImageLibraryAsync() { return forbidden('photo-picker', 'launchImageLibraryAsync'); } });
  const imageManipulator = closed('ImageManipulatorModule', {
    SaveFormat: closed('SaveFormat', { JPEG: 'jpeg', PNG: 'png' }),
    ImageManipulator: closed('ImageManipulator', { manipulate() { return forbidden('photo-manipulator', 'manipulate'); } }),
    async manipulateAsync() { return forbidden('photo-manipulator', 'manipulateAsync'); },
  });
  function optionalModule(name) { return check('native-module', () => { assert.equal(name, 'RudiFontScale'); nativeTrace.push({ kind: 'module', name }); }); }
  function assertClean({ disposed = false } = {}) {
    assert.deepEqual(violations, [], 'caught unexpected Settings boundaries cannot pass unnoticed');
    assert.equal(state.token, state.actor.token, 'actual Session published the declared actor token');
    if (disposed) for (const [kind, active] of Object.entries(listeners)) assert.equal(active.size, 0, `Settings ${kind} listeners removed`);
  }
  return { state: ioState, trace, nativeTrace, violations, listeners, retiredAppearance, check, forbidden, closed, session, kho, api: settingsApi, danhTinh, router, appearance, asyncStorage,
    fileSystem: closed('FileSystem', { File }), imagePicker, imageManipulator, optionalModule, nativeFont, fontSnapshot, assertClean,
    appearanceObservation() { return { live: listeners.appearance.size, subscriptions: appearanceSubscriptions, removals: appearanceRemovals }; },
    setSystemScheme(value) { check('appearance.fixture', () => { assert.equal(['light', 'dark'].includes(value), true); ioState.systemScheme = value; [...listeners.appearance].forEach(callback => callback({ colorScheme: value })); }); },
    invokeRetiredAppearance(index = retiredAppearance.length - 1) { assert.ok(retiredAppearance[index]); retiredAppearance[index]({ colorScheme: ioState.systemScheme }); },
    accessibility: closed('AccessibilityInfo', {
      async isReduceMotionEnabled(...args) { return check('accessibility.read', () => { assert.equal(args.length, 0); nativeTrace.push({ kind: 'accessibility', method: 'read' }); return true; }); },
      addEventListener(name, callback, ...rest) { return check('accessibility.listen', () => { assert.equal(rest.length, 0); return subscribe('accessibility', name, callback); }); },
    }),
  };
}
module.exports = { createSettingsBoundary };
