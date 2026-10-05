/* Chat v2 engine (src/rudi/chat/e2ee/may-ma-hoa.ts) with REAL MLS: every
 * device is the Rust crate through its C ABI (the drill binary,
 * packages/chat-crypto-ffi/examples/drill.rs), against an in-memory lane that
 * keeps the Go lane's rules (services/core/internal/chatv2: epoch
 * compare-and-swap, the server-attested roster, one-time key packages,
 * Welcomes, first_sequence). The Go lane itself is proven by its own tests and
 * by the Go drill; this proves the engine drives the protocol right.
 *
 * Run by scripts/chat_drill.sh, or from apps/mobile:
 *     CHAT_DRILL_BIN=… npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/drill/*.drill.mjs
 * Synthetic data only.
 */
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createInterface } from "node:readline";
import test from "node:test";

import { ApiError } from "../../dist-test/api.js";
import { MayMaHoa } from "../../dist-test/rudi/chat/e2ee/may-ma-hoa.js";
import { dungPhong } from "../../dist-test/rudi/chat/e2ee/so-phong.js";
import { danhSach } from "../../dist-test/rudi/chat/e2ee/so-tin.js";

function drill() {
  const bin = process.env.CHAT_DRILL_BIN;
  assert.ok(bin, "CHAT_DRILL_BIN is not set; run scripts/chat_drill.sh");
  const child = spawn(bin, [], { stdio: ["pipe", "pipe", "inherit"] });
  const lines = createInterface({ input: child.stdout });
  const waiting = [];
  lines.on("line", (l) => waiting.shift()(JSON.parse(l)));
  const ask = (req) => new Promise((resolve) => { waiting.push(resolve); child.stdin.write(JSON.stringify(req) + "\n"); });
  return { ask, opened: new Set(), close: () => { child.stdin.end(); child.kill(); } };
}

/** CryptoPort over the drill: one named client per device, errors as the native module throws them. */
function crypto(d, name) {
  const ok = (a) => { if (a.error) throw new Error(`ERR_CHAT_CRYPTO_${String(a.error).toUpperCase()}`); return JSON.stringify(a); };
  return {
    // Opening a device already open is a process restart: sealed, freed and
    // resumed through the C ABI, as the phone does from its vault.
    open: async (actor, device) => {
      if (d.opened.has(name)) { ok(await d.ask({ client: name, fn: "restart" })); return true; }
      ok(await d.ask({ client: name, fn: "new", actor, device }));
      d.opened.add(name);
      return false;
    },
    createGroup: async (c) => ok(await d.ask({ client: name, fn: "create_group", conversation_id: c })),
    encrypt: async (c, l, op) => ok(await d.ask({ client: name, fn: "encrypt", conversation_id: c, logical_send_id: l, operation: JSON.parse(op) })),
    receive: async (env, roster) => ok(await d.ask({ client: name, fn: "receive", envelope: JSON.parse(env), roster: roster === null ? null : JSON.parse(roster) })),
    call: async (m, args) => ok(await d.ask({ client: name, fn: "call", method: m, args: JSON.parse(args) })),
  };
}

const b64ToBytes = (s) => [...Buffer.from(s, "base64")];
const refuse = (code) => { throw new ApiError(409, code, code); };

/** The Go lane's rules, in memory. */
class Lane {
  devices = new Map(); // device -> card
  kps = new Map(); // device -> [kp]
  welcomes = new Map(); // device -> [welcome]
  rooms = new Map(); // room -> {epoch, members: Map(device -> first), persons:Set, events:[], ready}
  expected(room) {
    const r = this.rooms.get(room);
    return [...this.devices.values()].filter((c) => r.persons.has(c.actor_id)).sort((a, b) => a.device_id.localeCompare(b.device_id));
  }
  sameSet(room) {
    const r = this.rooms.get(room);
    const e = this.expected(room).map((c) => c.device_id);
    return e.length === r.members.size && e.every((d) => r.members.has(d));
  }
  view(room) {
    const r = this.rooms.get(room);
    return { exists: true, epoch: r.epoch, last_sequence: r.events.length, ready: r.ready, members: [...r.members.keys()].map((d) => this.devices.get(d)), expected: this.expected(room) };
  }
  api() {
    return {
      enroll: async (actor, b) => { const c = { actor_id: actor, device_id: b.device_id, mls_signature_key: b64ToBytes(b.mls_signature_key), transport_signature_key: b64ToBytes(b.transport_signature_key) }; this.devices.set(b.device_id, c); return c; },
      publishKeyPackages: async (_a, device, packages) => { const l = [...(this.kps.get(device) ?? []), ...packages]; this.kps.set(device, l); return { available: l.length, max: 20 }; },
      welcomes: async (_a, device) => this.welcomes.get(device) ?? [],
      ackWelcome: async (_a, device, id) => { this.welcomes.set(device, (this.welcomes.get(device) ?? []).filter((w) => w.id !== id)); },
      roster: async (actor, room) => {
        const r = this.rooms.get(room);
        if (r === undefined) return { exists: false, epoch: 0, last_sequence: 0, ready: false, members: [], expected: [] };
        if (!r.persons.has(actor)) refuse("chat_v2_forbidden");
        return this.view(room);
      },
      bootstrap: async (_a, room, device) => {
        const r = { epoch: 1, members: new Map([[device, 1]]), persons: this.persons.get(room), events: [], ready: false };
        this.rooms.set(room, r);
        r.ready = this.sameSet(room);
        return this.view(room);
      },
      claim: async (_a, room, device, targets) => {
        const r = this.rooms.get(room);
        if (!r.members.has(device)) refuse("chat_v2_forbidden");
        return targets.map((t) => { const l = this.kps.get(t) ?? []; if (l.length === 0) refuse("chat_v2_key_package_unavailable"); return { card: this.devices.get(t), key_package: l.shift() }; });
      },
      commit: async (actor, room, bundle, added, removed) => {
        const r = this.rooms.get(room);
        if (bundle.envelope.epoch !== r.epoch) refuse("chat_v2_stale_epoch");
        const expected = new Set(this.expected(room).map((c) => c.device_id));
        for (const d of removed) if (!r.members.has(d) || expected.has(d)) refuse("chat_v2_roster_mismatch");
        for (const d of added) if (r.members.has(d) || !expected.has(d)) refuse("chat_v2_roster_mismatch");
        for (const d of removed) r.members.delete(d);
        const seq = r.events.length + 1;
        for (const d of added) r.members.set(d, seq + 1);
        const roster = [...r.members.keys()].sort().map((d) => this.devices.get(d));
        r.events.push({ sequence: seq, kind: "commit", actor_id: actor, commit: { envelope: bundle.envelope, roster }, created_at: "" });
        r.epoch += 1;
        r.ready = this.sameSet(room);
        for (const d of added) this.welcomes.set(d, [...(this.welcomes.get(d) ?? []), { id: randomUUID(), conversation_id: room, sequence: seq, welcome: bundle.welcome, roster }]);
        return { event: r.events[seq - 1], ready: r.ready };
      },
      send: async (actor, room, envelope) => {
        const r = this.rooms.get(room);
        if (!r.members.has(envelope.device_id) || !r.persons.has(actor)) refuse("chat_v2_forbidden");
        if (!r.ready) refuse("chat_v2_not_ready");
        if (envelope.epoch !== r.epoch) refuse("chat_v2_stale_epoch");
        const e = { sequence: r.events.length + 1, kind: "envelope", actor_id: actor, envelope, created_at: "" };
        r.events.push(e);
        return { event: e, replayed: false };
      },
      events: async (actor, room, device, after) => {
        const r = this.rooms.get(room);
        if (!r.persons.has(actor) || !r.members.has(device)) refuse("chat_v2_forbidden");
        const from = Math.max(after, r.members.get(device) - 1);
        return { events: r.events.filter((e) => e.sequence > from), next_sequence: r.events.length, has_more: false };
      },
    };
  }
  persons = new Map(); // room -> Set(person), the server's memberships
}

function kho() {
  const m = new Map();
  return { doc: async (k) => m.get(k) ?? null, ghi: async (k, v) => { m.set(k, v); } };
}

/** The sealed room store, in memory: one append = one atomic write. */
function so() {
  const rooms = new Map();
  return {
    rooms,
    doc: async (room) => (rooms.has(room) ? structuredClone(rooms.get(room)) : null),
    noi: async (room, cursor, ban) => {
      const p = rooms.get(room) ?? { cursor: 0, ban: [] };
      rooms.set(room, { cursor: cursor ?? p.cursor, ban: [...p.ban, ...structuredClone(ban)] });
    },
  };
}

/** The bodies a device's record shows, in lane order. */
const bodies = async (m, room) => danhSach(dungPhong((await m.dongBo(room)).ban).so).map((t) => t.body);

test("ba thiết bị MLS thật qua engine: mở phòng, Welcome, nhắn hai chiều, thêm người, đua epoch, gỡ người", async (t) => {
  const d = drill();
  t.after(() => d.close());
  const lane = new Lane();
  const api = lane.api();
  const room = randomUUID();
  const [an, binh, chi] = [randomUUID(), randomUUID(), randomUUID()];
  lane.persons.set(room, new Set([an, binh]));
  const moi = { an: [], binh: [], chi: [] };
  const may = (actor, name) => new MayMaHoa({ actorId: actor, crypto: crypto(d, name), api, kho: kho(), so: so(), uuid: randomUUID, label: name,
    onThietBiMoi: (m) => moi[name].push(m.card.actor_id) });
  const [mA, mB, mC] = [may(an, "an"), may(binh, "binh"), may(chi, "chi")];
  for (const m of [mA, mB, mC]) await m.moThietBi();

  assert.equal(await mA.chuanBi(room), true, "An opens the room and adds Bình");
  assert.deepEqual(await mB.nhanWelcome(), [room]);
  assert.equal(await mB.chuanBi(room), true);

  await mA.gui(room, { type: "text", body: "Tối nay đi ăn không?" });
  assert.deepEqual(await bodies(mB, room), ["Tối nay đi ăn không?"]);

  // Chi joins the group on the server; Bình's next send meets a room that is
  // not ready, brings it in line (adds Chi), and goes through.
  lane.persons.get(room).add(chi);
  lane.rooms.get(room).ready = lane.sameSet(room);
  await mB.gui(room, { type: "text", body: "Có Chi đi cùng" });
  assert.deepEqual(await mC.nhanWelcome(), [room]);
  // Every device that already knew the room is told Chi's device joined
  // (security review 05/10: no unseen ghost devices); Chi had no "before".
  await mA.dongBo(room);
  assert.deepEqual([moi.an, moi.binh, moi.chi], [[binh, chi], [chi], []]);
  assert.deepEqual(await bodies(mC, room), ["Có Chi đi cùng"], "Chi reads what was sent after the commit that added her, not before");
  // An's own record holds both: its own send and Bình's, in lane order.
  assert.deepEqual(await bodies(mA, room), ["Tối nay đi ăn không?", "Có Chi đi cùng"]);

  // An has not read Bình's commit: An's send meets a moved epoch, reads the
  // commit, re-encrypts and goes through; Chi reads it.
  await mA.gui(room, { type: "text", body: "Vậy 7 giờ nhé" });
  assert.deepEqual(await bodies(mC, room), ["Có Chi đi cùng", "Vậy 7 giờ nhé"]);

  // Bình leaves on the server: An removes Bình's device; Chi follows; Bình can no longer read.
  lane.persons.get(room).delete(binh);
  lane.rooms.get(room).ready = lane.sameSet(room);
  assert.equal(await mA.chuanBi(room), true);
  await mA.gui(room, { type: "text", body: "Hai đứa mình đi" });
  assert.deepEqual(await bodies(mC, room), ["Có Chi đi cùng", "Vậy 7 giờ nhé", "Hai đứa mình đi"]);
  await assert.rejects(() => mB.dongBo(room), (e) => e instanceof ApiError && e.code === "chat_v2_forbidden");
  // What Bình read while a member stays readable on Bình's own device.
  assert.deepEqual(danhSach(dungPhong((await mB.soPhong(room)).ban).so).map((t) => t.body), ["Tối nay đi ăn không?", "Có Chi đi cùng"]);
  d.close();
});

test("app chết giữa chừng không mất tin, một envelope rác không làm kẹt phòng, tin chưa gửi được gửi lại", async (t) => {
  const d = drill();
  t.after(() => d.close());
  const lane = new Lane();
  const api = lane.api();
  const room = randomUUID();
  const [an, binh] = [randomUUID(), randomUUID()];
  lane.persons.set(room, new Set([an, binh]));
  const khoA = kho();
  const khoB = kho();
  const soA = so();
  const soB = so();
  const may = (actor, name, k, s, apiOf = api) => new MayMaHoa({ actorId: actor, crypto: crypto(d, name), api: apiOf, kho: k, so: s, uuid: randomUUID, label: name });
  const mA = may(an, "an", khoA, soA);
  let mB = may(binh, "binh", khoB, soB);
  await mA.moThietBi();
  await mB.moThietBi();
  assert.equal(await mA.chuanBi(room), true);
  await mB.nhanWelcome();

  // 1. Bình's phone dies after the crypto opened a message and before the
  // record was written: nothing of that page reaches the store.
  await mA.gui(room, { type: "text", body: "tin trước lúc sập" });
  const that = soB.noi;
  soB.noi = async () => { throw new Error("app died"); };
  await assert.rejects(() => mB.dongBo(room), /app died/);
  soB.noi = that;
  mB = may(binh, "binh", khoB, soB); // a new process, the same device and stores
  await mB.moThietBi();
  assert.deepEqual(await bodies(mB, room), ["tin trước lúc sập"], "the replayed envelope answers from the crypto's journal");

  // 2. A member's device posts an envelope that will never open: it is
  // skipped and counted; the next real message still arrives.
  const real = lane.rooms.get(room).events.at(-1).envelope;
  await api.send(an, room, { ...real, logical_send_id: randomUUID(), ciphertext: Buffer.from("rác").toString("base64") });
  await mA.gui(room, { type: "text", body: "sau tin rác" });
  const p = await mB.dongBo(room);
  assert.deepEqual(danhSach(dungPhong(p.ban).so).map((x) => x.body), ["tin trước lúc sập", "sau tin rác"]);
  assert.equal(dungPhong(p.ban).khongMo, 1);

  // 3. A send that failed on the network stays as a failed row with the same
  // logical id; «Thử lại» lands it once.
  const down = { ...api, send: async () => { throw new ApiError(0, "network", "Mất mạng"); } };
  const mBdown = may(binh, "binh", khoB, soB, down);
  await mBdown.moThietBi();
  await assert.rejects(() => mBdown.gui(room, { type: "text", body: "gửi lúc mất mạng" }));
  let r = dungPhong((await mBdown.soPhong(room)).ban);
  assert.deepEqual(r.cho.map((c) => [c.r.operation.body, c.hong, c.thuLai]), [["gửi lúc mất mạng", true, true]]);
  const id = r.cho[0].id;
  mB = may(binh, "binh", khoB, soB);
  await mB.moThietBi();
  await mB.thuLai(room, id);
  r = dungPhong((await mB.dongBo(room)).ban);
  assert.deepEqual(r.cho, []);
  assert.deepEqual(danhSach(r.so).map((x) => x.body), ["tin trước lúc sập", "sau tin rác", "gửi lúc mất mạng"]);
  assert.deepEqual(await bodies(mA, room), ["tin trước lúc sập", "sau tin rác", "gửi lúc mất mạng"]);

  // 4. The phone dies right after writing a send down, before it left:
  // reopening sends it, once.
  const logical = randomUUID();
  await soB.noi(room, null, [{ t: "cho", r: { kind: "application", actor_id: binh, device_id: await mB.thietBi(), logical_send_id: logical, operation: { type: "text", body: "viết xong thì sập" } }, luc: Date.now() }]);
  mB = may(binh, "binh", khoB, soB);
  await mB.moThietBi();
  await mB.guiLai(room);
  await mB.guiLai(room);
  assert.deepEqual((await bodies(mA, room)).at(-1), "viết xong thì sập");
  assert.equal((await bodies(mA, room)).filter((b) => b === "viết xong thì sập").length, 1);
  assert.deepEqual(dungPhong((await mB.dongBo(room)).ban).cho, []);
  d.close();
});
