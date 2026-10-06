/* Chat v2: the message list the decrypted operations make (ADR-0057 §5).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-e2ee-so-tin.test.mjs
 *
 * Pure and order-faithful: only an author edits or deletes their message, a
 * reaction toggles per person, a delete clears content and reactions, a
 * replayed operation changes nothing.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { SO_TRONG, apDung, danhSach } from "../dist-test/rudi/chat/e2ee/so-tin.js";

const AN = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BINH = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
let n = 0;
function nhan(actor, operation, logical = `cccccccc-cccc-4ccc-8ccc-${String(++n).padStart(12, "0")}`) {
  return { kind: "application", actor_id: actor, device_id: actor, logical_send_id: logical, operation };
}

test("tin, trả lời, sửa của chính tác giả, xoá, reaction bật/tắt", () => {
  const goc = nhan(AN, { type: "text", body: "Tối nay đi ăn không?" });
  let so = apDung(SO_TRONG, goc, 1);
  so = apDung(so, nhan(BINH, { type: "reply", reply_to: goc.logical_send_id, body: "Đi!" }), 2);
  so = apDung(so, nhan(BINH, { type: "edit", message_id: goc.logical_send_id, body: "giả mạo" }), 3);
  assert.equal(danhSach(so)[0].body, "Tối nay đi ăn không?", "Bình sửa tin của An");
  so = apDung(so, nhan(AN, { type: "edit", message_id: goc.logical_send_id, body: "Tối nay 7 giờ đi ăn không?" }), 4);
  assert.deepEqual([danhSach(so)[0].body, danhSach(so)[0].edited], ["Tối nay 7 giờ đi ăn không?", true]);
  so = apDung(so, nhan(BINH, { type: "reaction", message_id: goc.logical_send_id, emoji: "❤️" }), 5);
  so = apDung(so, nhan(AN, { type: "reaction", message_id: goc.logical_send_id, emoji: "❤️" }), 6);
  assert.deepEqual({ ...danhSach(so)[0].reactions }, { "❤️": [BINH, AN] });
  so = apDung(so, nhan(BINH, { type: "reaction", message_id: goc.logical_send_id, emoji: "❤️" }), 7);
  assert.deepEqual({ ...danhSach(so)[0].reactions }, { "❤️": [AN] });
  so = apDung(so, nhan(BINH, { type: "delete", message_id: goc.logical_send_id }), 8);
  assert.equal(danhSach(so)[0].deleted, false, "Bình xoá tin của An");
  so = apDung(so, nhan(AN, { type: "delete", message_id: goc.logical_send_id }), 9);
  assert.deepEqual([danhSach(so)[0].deleted, danhSach(so)[0].body, { ...danhSach(so)[0].reactions }], [true, null, {}]);
  assert.equal(danhSach(so)[1].replyTo, goc.logical_send_id, "a reply keeps its deleted target");
  const lai = apDung(so, goc, 10);
  assert.equal(lai, so, "a replayed operation changes nothing");
});

test("emoji là chuỗi của người gửi: '__proto__' hay 'constructor' chỉ là một emoji (rà soát bảo mật 05/10)", () => {
  const goc = nhan(AN, { type: "text", body: "Ảnh đẹp" });
  let so = apDung(SO_TRONG, goc, 1);
  for (const emoji of ["__proto__", "constructor", "toString"]) {
    so = apDung(so, nhan(BINH, { type: "reaction", message_id: goc.logical_send_id, emoji }), 2);
  }
  assert.deepEqual(Object.keys(danhSach(so)[0].reactions).sort(), ["__proto__", "constructor", "toString"]);
  assert.deepEqual(danhSach(so)[0].reactions["__proto__"], [BINH]);
  so = apDung(so, nhan(BINH, { type: "reaction", message_id: goc.logical_send_id, emoji: "__proto__" }), 3);
  assert.equal(Object.hasOwn(danhSach(so)[0].reactions, "__proto__"), false);
  assert.equal(Object.getPrototypeOf(danhSach(so)[0].reactions), null);
});
