// Inbox-only session/native/I/O seams. No application component is substituted.
// Actual phien, catalogue, invitation, summary and story code run above these boundaries.
const assert = require('node:assert/strict');
const plain = value => JSON.parse(JSON.stringify(value));
const keys = value => Object.keys(value).sort().join(',');
function createInboxBoundary({ React, state, parked, io }) {
  assert.ok(io && Array.isArray(io.contexts) && io.contexts.length > 0, 'Explicit Inbox context/membership allowlist');
  const contexts = new Map(io.contexts.map(row => [row.id, row.membership]));
  assert.equal(contexts.size, io.contexts.length);
  for (const [id, membership] of contexts) {
    assert.match(id, /^synthetic-(group|pair|invite)-[1-9][0-9]*$/);
    assert.match(membership, /^synthetic-membership-[1-9][0-9]*$/);
  }
  assert.ok(Array.isArray(io.inviteIds) && io.inviteIds.every(id => contexts.has(id)));
  const invitationIds = new Set(io.inviteIds);
  const people = new Set(io.people);
  assert.ok(people.size > 0 && [...people].every(id => /^synthetic-[A-Z]$/.test(id)));
  state.actor = { ...state.actor, expires_at: '2099-10-01T00:00:00Z', membership_state: null, membership_id: null };
  state.token = state.actor.token; // The prop-session seam starts with a supplied authenticated actor.
  const unexpected = [], catalogueReads = [], summaryReads = [], storyReads = [], invitationReads = [], accepts = [], declines = [], avatarReads = [], storage = [], pendingStores = [], sessionUpdates = [], modules = [], appState = new Set(), actorListeners = new Set();
  let deferStore = 0, savedSession = null;
  function invalid(kind, error) {
    unexpected.push({ kind, message: String(error.message ?? error) });
    throw error;
  }
  function check(kind, callback) { try { return callback(); } catch (error) { return invalid(kind, error); } }
  function forbidden(kind, value) { return invalid(kind, new Error(`Unexpected Inbox ${kind}: ${String(value)}`)); }
  function actor(value) {
    assert.equal(people.has(value.person_id), true, 'Only explicit synthetic actors');
    assert.equal(value.token, `synthetic-token-${value.person_id}`);
    assert.equal(value.expires_at, '2099-10-01T00:00:00Z');
    assert.ok(value.context_id === null || contexts.has(value.context_id));
    if (value.context_id !== null) assert.equal(value.membership_id, contexts.get(value.context_id));
  }
  function catalogue(value) {
    assert.equal(keys(value), 'contexts'); assert.ok(Array.isArray(value.contexts));
    const ids = new Set();
    for (const row of value.contexts) {
      assert.equal(contexts.has(row.id), true); assert.equal(ids.has(row.id), false); ids.add(row.id);
      assert.equal(row.membership_id, contexts.get(row.id));
      assert.ok(['active', 'invited'].includes(row.my_state));
      assert.equal(typeof row.display_name, 'string'); assert.ok(row.display_name.length > 0);
      assert.equal(Number.isSafeInteger(row.member_count) && row.member_count >= 0, true);
      assert.equal(Number.isSafeInteger(row.unread_count) && row.unread_count >= 0, true);
      if (row.counterpart != null) assert.equal(people.has(row.counterpart.id), true);
      if (row.last_message) assert.ok(row.last_message.author_id === null || people.has(row.last_message.author_id));
    }
  }
  function validated(store, call, validate) {
    const promise = parked(store, call), resolve = call.resolve;
    call.resolve = value => { check('response', () => validate(value)); resolve(value); };
    return promise;
  }
  function transport(route, options, call) {
    return check('transport', () => {
      assert.equal(options.actorId, state.actor.person_id, 'Exact supplied actor');
      assert.equal(state.token, state.actor.token, 'Active token matches the published actor');
      assert.equal(state.token, `synthetic-token-${options.actorId}`, 'Exact active token for the requested synthetic actor');
      call.activeToken = state.token;
      const method = options.method;
      if (route === '/people/me/contexts' && method === 'GET') {
        assert.equal(keys(options), 'actorId,method'); call.kind = 'inboxCatalogue';
        return validated(catalogueReads, call, catalogue);
      }
      if (route === '/v2/chat/summaries' && method === 'GET') {
        assert.equal(keys(options), 'actorId,method'); call.kind = 'inboxSummary';
        return validated(summaryReads, call, value => {
          assert.equal(keys(value), 'rooms'); assert.ok(Array.isArray(value.rooms));
          for (const row of value.rooms) { assert.equal(contexts.has(row.conversation_id), true); assert.ok(Number.isSafeInteger(row.unread) && row.unread >= 0); }
        });
      }
      if (route === '/stories' && method === 'GET') {
        assert.equal(keys(options), 'actorId,method'); call.kind = 'inboxStory';
        return validated(storyReads, call, value => {
          assert.equal(keys(value), 'authors'); assert.ok(Array.isArray(value.authors));
          const seen = new Set();
          for (const row of value.authors) {
            assert.equal(people.has(row.author.id), true); assert.equal(seen.has(row.author.id), false); seen.add(row.author.id);
            assert.equal(typeof row.author.display_name, 'string'); assert.equal(typeof row.all_seen, 'boolean'); assert.ok(Array.isArray(row.stories));
          }
        });
      }
      for (const [id, membership] of contexts) {
        if (invitationIds.has(id) && route === `/contexts/${id}/invitation` && method === 'GET') {
          assert.equal(keys(options), 'actorId,method'); call.kind = 'inboxInvitation';
          return validated(invitationReads, call, value => {
            assert.equal(value.context_id, id); assert.equal(typeof value.display_name, 'string');
            assert.ok(value.invited_by === null || people.has(value.invited_by.id));
            assert.ok(Number.isSafeInteger(value.member_count) && value.member_count >= 0);
          });
        }
        if (invitationIds.has(id) && route === `/memberships/${membership}/accept` && method === 'POST') {
          assert.equal(keys(options), 'actorId,method'); call.kind = 'inboxAccept';
          return validated(accepts, call, value => { assert.equal(keys(value), 'state'); assert.ok(['invited', 'active', 'left'].includes(value.state)); });
        }
        if (invitationIds.has(id) && route === `/contexts/${id}/invitation` && method === 'DELETE') {
          assert.equal(keys(options), 'actorId,attempt,method'); assert.equal(keys(options.attempt), 'at,key');
          assert.match(options.attempt.key, /^synthetic-attempt-[1-9][0-9]*$/);
          assert.equal(options.attempt.at, Date.parse('2026-10-01T00:00:00Z') + Number(options.attempt.key.slice('synthetic-attempt-'.length)));
          call.kind = 'inboxDecline'; return validated(declines, call, value => assert.equal(value, undefined));
        }
      }
      if (method === 'GET' && route.startsWith('/people/avatars?ids=')) {
        assert.equal(keys(options), 'actorId,method,timeoutMs'); assert.equal(options.timeoutMs, 10_000);
        const ids = route.slice('/people/avatars?ids='.length).split(',').map(decodeURIComponent);
        assert.equal(new Set(ids).size, ids.length); assert.ok(ids.length > 0);
        assert.ok(ids.every(id => people.has(id))); assert.equal(route, '/people/avatars?ids=' + ids.map(encodeURIComponent).join(','));
        call.kind = 'inboxAvatar'; avatarReads.push(call);
        return Promise.resolve({ avatars: Object.fromEntries(ids.map(id => [id, null])) });
      }
      throw new Error(`Unplanned ${method} ${route}`);
    });
  }
  const secureStore = {
    async getItemAsync(key) { return check('secureStore.get', () => { assert.equal(key, 'rudi.phien'); storage.push({ method: 'get', key }); return savedSession; }); },
    async setItemAsync(key, value) {
      const pending = check('secureStore.set', () => {
        assert.equal(key, 'rudi.phien'); const parsed = JSON.parse(value); actor(parsed);
        assert.equal(parsed.person_id, state.actor.person_id); assert.equal(parsed.token, state.actor.token);
        const call = { method: 'set', key, value: parsed }; storage.push(call);
        if (deferStore > 0) { deferStore -= 1; return { call, promise: parked(pendingStores, call) }; }
        savedSession = value; return null;
      });
      if (pending) { await pending.promise; savedSession = value; }
    },
    async deleteItemAsync(key) { return forbidden('secureStore.delete', key); },
  };
  const localStorage = {
    async docGiaoDienAsync(key) { return check('local.get', () => { assert.equal(key, 'rudi.nep.dock.v3'); storage.push({ method: 'local.get', key }); return null; }); },
    async ghiGiaoDienAsync(key, value) { return check('local.set', () => { assert.equal(key, 'rudi.nep.dock.v3'); assert.deepEqual(plain(JSON.parse(value)), { tyLe: 0.62 }); storage.push({ method: 'local.set', key, value }); }); },
    async xoaPhienAsync() { return forbidden('local.delete', 'session'); },
  };
  function replaceActor(value) { check('session', () => actor(value)); state.token = value.token; state.actor = value; [...actorListeners].forEach(callback => callback()); }
  function useActor() { return React.useSyncExternalStore(callback => { actorListeners.add(callback); return () => actorListeners.delete(callback); }, () => state.actor, () => state.actor); }
  const sessionBoundary = {
    useRudiSession() { const phien = useActor(); return { phien, phienDaDoc: true, datPhien(value) { check('datPhien', () => actor(value)); sessionUpdates.push(plain(value)); replaceActor(value); } }; },
  };
  const asyncStorage = { __esModule: true, default: Object.fromEntries(['getItem', 'setItem', 'removeItem', 'multiGet', 'multiSet'].map(method => [method, async (...args) => forbidden(`asyncStorage.${method}`, args[0])])) };
  return {
    navigation(method, value) { return check('navigation', () => {
      assert.equal(method, 'push');
      const allowed = new Set(['/groups/new', '/friends/add', '/stories/new', ...[...contexts.keys()].map(id => `/groups/${id}/chat`), ...[...people].map(id => `/stories/${id}`)]);
      assert.equal(allowed.has(value), true, 'Only explicit fixture navigation routes');
    }); },
    transport, forbidden, check, secureStore, localStorage, sessionBoundary, useActor, replaceActor,
    unexpected, catalogueReads, summaryReads, storyReads, invitationReads, accepts, declines, avatarReads, storage, pendingStores, sessionUpdates, modules, appState,
    deferSessionWrite() { deferStore += 1; },
    optionalModule(name) { modules.push(name); if (name === 'RudiChatCrypto') return null; return forbidden('optionalModule', name); },
    fileSystem: { File: class UnexpectedInboxFile { constructor(...args) { return forbidden('file', args[0]); } } },
    asyncStorage,
    assertClean() { assert.deepEqual(unexpected, [], 'Caller-caught unexpected I/O cannot pass unnoticed'); },
  };
}
module.exports = { createInboxBoundary };
