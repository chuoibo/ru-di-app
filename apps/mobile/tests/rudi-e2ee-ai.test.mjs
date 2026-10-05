/* Chat v2: Rủ Đi AI's answer in an end-to-end room (ADR-0057 §6) as the
 * device draws it (src/rudi/chat/e2ee/so-tin.ts, ve-v2.ts).
 *
 * An `ai_card` is drawn as the assistant's only once checked against the
 * server's receipt; unchecked it is not drawn; a card that failed the check is
 * the sender's own words, said so. Nobody edits a card.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { SO_TRONG, apDung, danhSach } from "../dist-test/rudi/chat/e2ee/so-tin.js";
import { khoaXacMinh, sangTin } from "../dist-test/rudi/chat/e2ee/ve-v2.js";

const AN = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BINH = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const ROOM = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const INV = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const TAG = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";
const nhan = (actor, operation, logical) => ({ kind: "application", actor_id: actor, device_id: actor, logical_send_id: logical, operation });
const CARD = '{"kind":"tra_loi","payload":{"invocation_id":"' + INV + '","phan":[]}}';

function phong() {
  let so = apDung(SO_TRONG, nhan(AN, { type: "text", body: "@Rủ Đi tối nay ăn gì?" }, TAG), 1);
  so = apDung(so, nhan(AN, { type: "ai_card", invocation_id: INV, reply_to: TAG, card: CARD }, INV), 2, "2026-10-06T00:10:00Z");
  return so;
}

test("thẻ AI chưa kiểm thì không vẽ; kiểm đúng thì là của Rủ Đi AI, không mang tác giả", () => {
  const the = danhSach(phong()).find((t) => t.ai !== null);
  assert.equal(sangTin(the, ROOM, BINH, {}, {}), null);
  const ve = sangTin(the, ROOM, BINH, {}, { [khoaXacMinh(the)]: true });
  assert.equal(ve.kind, "ai_card");
  assert.equal(ve.author_id, null);
  assert.deepEqual(ve.card, JSON.parse(CARD));
  assert.equal(ve.reply_to.id, TAG);
  assert.equal(ve.created_at, "2026-10-06T00:10:00Z");
});

test("thẻ AI không khớp biên nhận là lời của chính người gửi, nói rõ", () => {
  const the = danhSach(phong()).find((t) => t.ai !== null);
  const ve = sangTin(the, ROOM, BINH, {}, { [khoaXacMinh(the)]: false });
  assert.equal(ve.kind, "text");
  assert.equal(ve.author_id, AN);
  assert.equal(ve.card, null);
  assert.match(ve.body, /không khớp/);
});

test("kiểm theo cả lời gọi lẫn người gửi: cùng id lời gọi mà người khác gửi là thẻ khác", () => {
  let so = phong();
  so = apDung(so, nhan(BINH, { type: "ai_card", invocation_id: INV, reply_to: TAG, card: CARD }, "ffffffff-ffff-4fff-8fff-ffffffffffff"), 3);
  const [cuaAn, cuaBinh] = danhSach(so).filter((t) => t.ai !== null);
  assert.notEqual(khoaXacMinh(cuaAn), khoaXacMinh(cuaBinh));
  assert.equal(sangTin(cuaBinh, ROOM, AN, {}, { [khoaXacMinh(cuaAn)]: true }), null, "the check of An's card does not vouch for Bình's");
});

test("không ai sửa được thẻ AI, kể cả người đã gửi nó; xoá thì xoá cả thẻ", () => {
  let so = phong();
  so = apDung(so, nhan(AN, { type: "edit", message_id: INV, body: "Rủ Đi nói: đi bar" }, "abababab-abab-4bab-8bab-abababababab"), 3);
  const the = danhSach(so).find((t) => t.id === INV);
  assert.equal(the.body, null);
  assert.equal(the.edited, false);
  so = apDung(so, nhan(AN, { type: "delete", message_id: INV }, "cdcdcdcd-cdcd-4dcd-8dcd-cdcdcdcdcdcd"), 4);
  const xoa = danhSach(so).find((t) => t.id === INV);
  assert.equal(xoa.ai, null);
  assert.equal(sangTin(xoa, ROOM, BINH, {}, {}).kind, "deleted");
});
