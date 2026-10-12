// Closed synthetic I/O for the actual Community route and standalone screen.
const assert = require('node:assert/strict');

function createCommunityBoundary({ state, parked, io }) {
  assert.deepEqual(Object.keys(io).sort(), io.posts === undefined ? ['notifications', 'preferences', 'seen'] : ['notifications', 'posts', 'preferences', 'seen']);
  const cardPosts = io.posts === undefined ? null : JSON.parse(JSON.stringify(io.posts));
  if (cardPosts !== null) {
    assert.ok(Array.isArray(cardPosts) && cardPosts.length > 0);
    for (const row of cardPosts) {
      assert.match(row.id, /^synthetic-post-[12]$/); assert.ok(['synthetic-A', 'synthetic-B'].includes(row.author_id));
      assert.equal(typeof row.author, 'string'); assert.equal(typeof row.body, 'string');
    }
    assert.equal(new Set(cardPosts.map(row => row.id)).size, cardPosts.length);
  }
  const postIds = new Set((cardPosts ?? []).map(row => row.id));
  const authorIds = new Set((cardPosts ?? []).map(row => row.author_id));
  assert.deepEqual(Object.keys(io.preferences).sort(), ['asked', 'personalized']);
  assert.equal(typeof io.preferences.asked, 'boolean'); assert.equal(typeof io.preferences.personalized, 'boolean');
  assert.equal(io.seen === null || typeof io.seen === 'string', true);
  assert.ok(Array.isArray(io.notifications));
  for (const row of io.notifications) {
    assert.deepEqual(Object.keys(row).sort(), ['created_at', 'id', 'kind', 'post_id']);
    assert.equal(row.id, 'synthetic-notification-1'); assert.equal(row.post_id, 'synthetic-post-1');
    assert.equal(row.kind, 'mention'); assert.equal(row.created_at, '2026-10-02T00:00:00Z');
  }
  const unexpected = [], navigationCalls = [], storageCalls = [], sessionCalls = [], haptics = [], modules = [];
  const feedReads = [], preferenceWrites = [], historyWrites = [], notificationReads = [], preferenceReads = [];
  const followWrites = [], likeWrites = [], feedbackWrites = [], commentReads = [];
  const listeners = { app: new Set(), back: new Set(), accessibility: new Set() }, sockets = [];
  const attempts = new Set();
  function checked(kind, callback) {
    try { return callback(); } catch (error) { unexpected.push({ kind, message: String(error) }); throw error; }
  }
  function forbidden(kind, detail) { return checked(kind, () => assert.fail(`Unexpected Community boundary ${kind}: ${String(detail)}`)); }
  function attempt(value) {
    assert.deepEqual(Object.keys(value ?? {}).sort(), ['at', 'key']);
    assert.match(value.key, /^synthetic-attempt-[1-9][0-9]*$/);
    assert.equal(value.at, Date.parse('2026-10-01T00:00:00Z') + Number(value.key.slice('synthetic-attempt-'.length)));
    assert.equal(attempts.has(value.key), false, 'each admitted mutation has a fresh attempt'); attempts.add(value.key);
  }
  function transport(route, options, call) { return checked('transport', () => {
    assert.equal(state.actor.person_id, 'synthetic-A'); assert.equal(options.actorId, state.actor.person_id); assert.equal(state.token, state.actor.token);
    const method = options.method;
    if (/^\/v2\/community\/feed\?mode=(for_you|following|trending|saved|mine|hidden)$/.test(route) && method === 'GET') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'method']);
      call.kind = 'communityFeedRead'; return parked(feedReads, call).then(page => checked('feed-response', () => {
        assert.deepEqual(Object.keys(page).sort(), ['next_cursor', 'personalized', 'posts']);
        assert.ok(Array.isArray(page.posts));
        if (cardPosts === null) assert.equal(page.posts.length, 0, 'this header fixture admits no post-body substitutions');
        else assert.deepEqual(page.posts, cardPosts, 'only the explicit actual-card fixture enters the feed');
        assert.equal(page.next_cursor, null); assert.equal(page.personalized, false); return page;
      }));
    }
    if (route === '/v2/community/preferences' && method === 'GET') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'method']); preferenceReads.push(call);
      return { ...io.preferences };
    }
    if (route === '/v2/community/notifications' && method === 'GET') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'method']); notificationReads.push(call);
      return { notifications: io.notifications.map(row => ({ ...row })) };
    }
    if (route === '/v2/community/preferences' && method === 'PUT') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'attempt', 'body', 'method']);
      assert.deepEqual(Object.keys(options.body).sort(), ['personalized']); assert.equal(typeof options.body.personalized, 'boolean');
      attempt(options.attempt); call.kind = 'communityPreferenceWrite'; return parked(preferenceWrites, call).then(value => checked('preference-response', () => {
        assert.deepEqual(Object.keys(value).sort(), ['asked', 'personalized']); assert.equal(value.asked, true);
        assert.equal(value.personalized, options.body.personalized); io.preferences = { ...value }; return value;
      }));
    }
    if (route === '/v2/community/history' && method === 'DELETE') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'attempt', 'method']);
      attempt(options.attempt); call.kind = 'communityHistoryWrite'; return parked(historyWrites, call).then(value => checked('history-response', () => { assert.deepEqual(Object.keys(value), []); return value; }));
    }
    if (cardPosts !== null && route === '/v2/community/follows' && ['PUT', 'DELETE'].includes(method)) {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'attempt', 'body', 'method']);
      assert.deepEqual(Object.keys(options.body).sort(), ['kind', 'target']); assert.equal(options.body.kind, 'person');
      assert.equal(authorIds.has(options.body.target), true); assert.notEqual(options.body.target, state.actor.person_id);
      attempt(options.attempt); call.kind = 'communityFollowWrite'; return parked(followWrites, call).then(value => checked('follow-response', () => { assert.deepEqual(Object.keys(value), []); return value; }));
    }
    const cardId = [...postIds].find(id => route === `/v2/community/posts/${id}/like` || route === `/v2/community/posts/${id}/feedback` || route === `/v2/community/posts/${id}/comments?after=`);
    if (cardId !== undefined && route.endsWith('/like') && method === 'PUT') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'attempt', 'method']); attempt(options.attempt);
      call.kind = 'communityLikeWrite'; return parked(likeWrites, call).then(value => checked('like-response', () => {
        const original = cardPosts.find(row => row.id === cardId);
        assert.deepEqual(value, { ...original, liked: true, likes: original.likes + 1 }); return value;
      }));
    }
    if (cardId !== undefined && route.endsWith('/feedback') && method === 'PUT') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'attempt', 'body', 'method']);
      assert.deepEqual(Object.keys(options.body).sort(), ['enabled', 'kind']); assert.equal(options.body.kind, 'saved'); assert.equal(typeof options.body.enabled, 'boolean');
      attempt(options.attempt); call.kind = 'communityFeedbackWrite'; return parked(feedbackWrites, call).then(value => checked('feedback-response', () => { assert.deepEqual(Object.keys(value), []); return value; }));
    }
    if (cardId !== undefined && route.endsWith('/comments?after=') && method === 'GET') {
      assert.deepEqual(Object.keys(options).sort(), ['actorId', 'method']); commentReads.push(call);
      return { comments: [], pending: [], next_cursor: null };
    }
    return forbidden('transport', `${method} ${route}`);
  }); }
  const storage = {
    async xoaPhienAsync(...args) { return checked('storage', () => { assert.equal(args.length, 0); storageCalls.push({ method: 'xoaPhienAsync' }); }); },
    async docGiaoDienAsync(key) { return checked('storage', () => {
      assert.equal(key, 'cong-dong-thong-bao-da-xem:synthetic-A'); storageCalls.push({ method: 'docGiaoDienAsync', key }); return io.seen;
    }); },
    async ghiGiaoDienAsync(...args) { return forbidden('storage-write', args[0]); },
  };
  const session = {
    async khoiPhucPhien(...args) { return checked('session', () => { assert.equal(args.length, 0); sessionCalls.push('restore'); return state.actor; }); },
    async dangXuat() { return forbidden('session', 'logout'); },
    async quenPhienDaThuHoi() { return forbidden('session', 'revoke'); },
    async docNhomCuaToi() { return forbidden('session', 'groups'); },
  };
  function navigation(method, value) { return checked('navigation', () => {
    const allowed = method === 'navigate' && value === '/explore'
      || method === 'push' && ['/community/notifications', '/community/new', '/community/keeps'].includes(value)
      || method === 'push' && value?.pathname === '/community/search' && Object.keys(value).sort().join(',') === 'params,pathname'
        && Object.keys(value.params).join(',') === 'q' && value.params.q === 'QC community query'
      || cardPosts !== null && method === 'push' && (typeof value === 'string')
        && ([...authorIds].some(id => value === `/people/${id}`) || [...postIds].some(id => value === `/community/posts/${id}`));
    assert.equal(Boolean(allowed), true, 'navigation uses an explicitly admitted route and payload'); navigationCalls.push({ method, value });
  }); }
  function listen(kind, name, callback) { return checked('event', () => {
    assert.equal(name, { app: 'change', back: 'hardwareBackPress', accessibility: 'reduceMotionChanged' }[kind]);
    assert.equal(typeof callback, 'function'); listeners[kind].add(callback); let live = true;
    return { remove() { assert.equal(live, true); live = false; listeners[kind].delete(callback); } };
  }); }
  function socket(connection, url) { return checked('socket', () => {
    assert.equal(url, 'wss://synthetic.invalid/v2/community/stream'); sockets.push(connection);
  }); }
  function send(connection, value) { return checked('socket-send', () => {
    assert.equal(sockets.includes(connection), true); const frame = JSON.parse(value);
    assert.deepEqual(Object.keys(frame).sort(), ['after', 'posts', 'token']);
    assert.equal(frame.token, state.actor.token); assert.equal(frame.after, 0);
    if (cardPosts === null || sockets.indexOf(connection) === 0) assert.deepEqual(frame.posts, []);
    else { assert.equal(frame.posts.length, 1); assert.equal(postIds.has(frame.posts[0]), true, 'Comments can subscribe only to an admitted card'); }
  }); }
  function haptic(method, args) { return checked('haptic', () => {
    assert.equal(method, 'selectionAsync'); assert.equal(args.length, 0); haptics.push(method);
  }); }
  function optionalModule(name) { return checked('native-module', () => { assert.equal(name, 'RudiFontScale'); modules.push(name); }); }
  function fontSnapshot(value) { return checked('native-font', () => {
    if (value.status === 'unavailable') { assert.deepEqual(Object.keys(value).sort(), ['reason', 'status']); assert.equal(value.reason, 'module-inactive'); return; }
    assert.deepEqual(Object.keys(value).sort(), ['fontMetrics', 'fontScale', 'status']); assert.equal(value.status, 'available');
    assert.equal([1, 2].includes(value.fontScale), true); assert.equal(value.fontMetrics.density, 2);
    assert.ok(Array.isArray(value.fontMetrics.spToDp));
    for (const row of value.fontMetrics.spToDp) { assert.deepEqual(Object.keys(row).sort(), ['base', 'dp']); assert.equal(Number.isFinite(row.base), true); assert.equal(Number.isFinite(row.dp), true); }
  }); }
  function assertClean() {
    assert.deepEqual(unexpected, [], 'caught application errors cannot hide unexpected I/O/events');
    assert.equal(state.token, state.actor.token, 'actual session provider published the synthetic bearer');
  }
  return { forbidden, transport, storage, session, navigation, listen, socket, send, haptic, optionalModule, fontSnapshot, assertClean,
    unexpected, navigationCalls, storageCalls, sessionCalls, haptics, modules, listeners, sockets,
    feedReads, preferenceWrites, historyWrites, notificationReads, preferenceReads,
    followWrites, likeWrites, feedbackWrites, commentReads };
}
module.exports = { createCommunityBoundary };
