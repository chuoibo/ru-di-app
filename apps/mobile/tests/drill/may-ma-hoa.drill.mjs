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
import { SO_TRONG, apDung, danhSach } from "../../dist-test/rudi/chat/e2ee/so-tin.js";

function drill() {
  const bin = process.env.CHAT_DRILL_BIN;
  assert.ok(bin, "CHAT_DRILL_BIN is not set; run scripts/chat_drill.sh");
  const child = spawn(bin, [], { stdio: ["pipe", "pipe", "inherit"] });
  const lines = createInterface({ input: child.stdout });
  const waiting = [];
  lines.on("line", (l) => waiting.shift()(JSON.parse(l)));
  const ask = (req) => new Promise((resolve) => { waiting.push(resolve); child.stdin.write(JSON.stringify(req) + "\n"); });
  return { ask, close: () => { child.stdin.end(); child.kill(); } };
}

/** CryptoPort over the drill: one named client per device, errors as the native module throws them. */
function crypto(d, name) {
  const ok = (a) => { if (a.error) throw new Error(`ERR_CHAT_CRYPTO_${String(a.error).toUpperCase()}`); return JSON.stringify(a); };
  return {
    open: async (actor, device) => { ok(await d.ask({ client: name, fn: "new", actor, device })); return false; },
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

test("ba thiết bị MLS thật qua engine: mở phòng, Welcome, nhắn hai chiều, thêm người, đua epoch, gỡ người", async (t) => {
  const d = drill();
  t.after(() => d.close());
  const lane = new Lane();
  const api = lane.api();
  const room = randomUUID();
  const [an, binh, chi] = [randomUUID(), randomUUID(), randomUUID()];
  lane.persons.set(room, new Set([an, binh]));
  const may = (actor, name) => new MayMaHoa({ actorId: actor, crypto: crypto(d, name), api, kho: kho(), uuid: randomUUID, label: name });
  const [mA, mB, mC] = [may(an, "an"), may(binh, "binh"), may(chi, "chi")];
  for (const m of [mA, mB, mC]) await m.moThietBi();

  assert.equal(await mA.chuanBi(room), true, "An opens the room and adds Bình");
  assert.deepEqual(await mB.nhanWelcome(), [room]);
  assert.equal(await mB.chuanBi(room), true);

  let soB = SO_TRONG;
  await mA.gui(room, { type: "text", body: "Tối nay đi ăn không?" });
  await mB.dongBo(room, (t) => { soB = apDung(soB, t.received, t.sequence); });
  assert.deepEqual(danhSach(soB).map((t) => t.body), ["Tối nay đi ăn không?"]);

  // Chi joins the group on the server; Bình's next send meets a room that is
  // not ready, brings it in line (adds Chi), and goes through.
  lane.persons.get(room).add(chi);
  lane.rooms.get(room).ready = lane.sameSet(room);
  await mB.gui(room, { type: "text", body: "Có Chi đi cùng" });
  assert.deepEqual(await mC.nhanWelcome(), [room]);
  let soC = SO_TRONG;
  await mC.dongBo(room, (t) => { soC = apDung(soC, t.received, t.sequence); });
  assert.deepEqual(danhSach(soC).map((t) => t.body), ["Có Chi đi cùng"], "Chi reads what was sent after the commit that added her, not before");

  // An has not read Bình's commit: An's send meets a moved epoch, reads the
  // commit, re-encrypts and goes through; Chi reads it.
  await mA.gui(room, { type: "text", body: "Vậy 7 giờ nhé" });
  await mC.dongBo(room, (t) => { soC = apDung(soC, t.received, t.sequence); });
  assert.deepEqual(danhSach(soC).map((t) => t.body), ["Có Chi đi cùng", "Vậy 7 giờ nhé"]);

  // Bình leaves on the server: An removes Bình's device; Chi follows; Bình can no longer read.
  lane.persons.get(room).delete(binh);
  lane.rooms.get(room).ready = lane.sameSet(room);
  assert.equal(await mA.chuanBi(room), true);
  await mA.gui(room, { type: "text", body: "Hai đứa mình đi" });
  await mC.dongBo(room, (t) => { soC = apDung(soC, t.received, t.sequence); });
  assert.deepEqual(danhSach(soC).map((t) => t.body), ["Có Chi đi cùng", "Vậy 7 giờ nhé", "Hai đứa mình đi"]);
  await assert.rejects(() => mB.dongBo(room, () => undefined), (e) => e instanceof ApiError && e.code === "chat_v2_forbidden");
  d.close();
});
