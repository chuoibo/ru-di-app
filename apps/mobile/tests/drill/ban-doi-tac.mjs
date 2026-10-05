/* A synthetic peer device for the native E2EE check (ADR-0057): the app's own
 * engine (dist-test/rudi/chat/e2ee/may-ma-hoa.js) driving real MLS through the
 * drill binary, against a real disposable stack over HTTP. It joins the room
 * when a Welcome arrives, follows the roster, and answers every message it
 * decrypts with «Bình đã nhận: <text>» -- so a phone on the other end proves
 * both directions. Synthetic accounts only; never a real host.
 *
 *   EXPO_PUBLIC_API_URL=http://127.0.0.1:<port> CHAT_DRILL_BIN=… \
 *   node tests/drill/ban-doi-tac.mjs <bearer-token> <person-id> <room-id> [seconds]
 */
import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createInterface } from "node:readline";

import { datTokenPhien } from "../../dist-test/api.js";
import { apiV2 } from "../../dist-test/rudi/chat/e2ee/api-v2.js";
import { MayMaHoa } from "../../dist-test/rudi/chat/e2ee/may-ma-hoa.js";

const [token, person, room, giay = "600"] = process.argv.slice(2);
if (!token || !person || !room || !process.env.CHAT_DRILL_BIN || !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.EXPO_PUBLIC_API_URL ?? "")) {
  console.error("cần token, person, room, CHAT_DRILL_BIN và EXPO_PUBLIC_API_URL loopback");
  process.exit(2);
}
datTokenPhien(token);

const child = spawn(process.env.CHAT_DRILL_BIN, [], { stdio: ["pipe", "pipe", "inherit"] });
const lines = createInterface({ input: child.stdout });
const waiting = [];
lines.on("line", (l) => waiting.shift()(JSON.parse(l)));
const ask = (req) => new Promise((resolve) => { waiting.push(resolve); child.stdin.write(JSON.stringify(req) + "\n"); });
const ok = (a) => { if (a.error) throw new Error(`ERR_CHAT_CRYPTO_${String(a.error).toUpperCase()}`); return JSON.stringify(a); };
const crypto = {
  open: async (actor, device) => { ok(await ask({ client: "peer", fn: "new", actor, device })); return false; },
  createGroup: async (c) => ok(await ask({ client: "peer", fn: "create_group", conversation_id: c })),
  encrypt: async (c, l, op) => ok(await ask({ client: "peer", fn: "encrypt", conversation_id: c, logical_send_id: l, operation: JSON.parse(op) })),
  receive: async (env, roster) => ok(await ask({ client: "peer", fn: "receive", envelope: JSON.parse(env), roster: roster === null ? null : JSON.parse(roster) })),
  call: async (m, args) => ok(await ask({ client: "peer", fn: "call", method: m, args: JSON.parse(args) })),
};
const store = new Map();
const kho = { doc: async (k) => store.get(k) ?? null, ghi: async (k, v) => { store.set(k, v); } };
const may = new MayMaHoa({ actorId: person, crypto, api: apiV2, kho, uuid: randomUUID, label: "Bình (máy giả lập)",
  onThietBiMoi: (m) => console.log(`thiết bị mới trong phòng: ${m.card.device_id}`) });

await may.moThietBi();
console.log("đã ghi danh thiết bị");
const het = Date.now() + Number(giay) * 1000;
let trongPhong = false;
while (Date.now() < het) {
  try {
    if ((await may.nhanWelcome()).includes(room)) {
      trongPhong = true;
      console.log("đã vào phòng qua Welcome");
    }
    if (!trongPhong) {
      const r = await apiV2.roster(person, room, await may.thietBi());
      trongPhong = r.exists && r.members.some((c) => c.device_id === (store.get(`rudi.chat-v2.device.${person}`) ?? ""));
    }
    if (trongPhong) {
      await may.chuanBi(room);
      const nhan = [];
      await may.dongBo(room, (t) => { if (t.received.operation.type === "text" || t.received.operation.type === "reply") nhan.push(t.received.operation.body); });
      for (const body of nhan) {
        console.log(`đã giải mã: ${body}`);
        await may.gui(room, { type: "text", body: `Bình đã nhận: ${body}` });
      }
    }
  } catch (error) {
    console.log(`lỗi: ${error.code ?? ""} ${error.message}`);
  }
  await new Promise((r) => setTimeout(r, 1000));
}
child.stdin.end();
child.kill();
