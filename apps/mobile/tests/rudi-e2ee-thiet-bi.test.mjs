/* Chat v2: a device's key mark (src/rudi/chat/e2ee/thiet-bi.ts, ADR-0057 §1.3).
 *
 * The same card gives the same mark on every phone; a different transport or
 * MLS key gives a different one -- the point of comparing it out of band.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { cungKhoa, dauKhoa } from "../dist-test/rudi/chat/e2ee/thiet-bi.js";

const card = (t, m) => ({ actor_id: "a", device_id: "d", transport_signature_key: t, mls_signature_key: m });
const bytes = (start) => Array.from({ length: 32 }, (_, i) => (start + i) & 0xff);

test("dấu khoá: 10 byte khoá transport rồi 10 byte khoá MLS, nhóm bốn chữ hex", () => {
  assert.equal(dauKhoa(card(bytes(0xaa), bytes(0xca))), "aaab acad aeaf b0b1 b2b3 cacb cccd cecf d0d1 d2d3");
});

test("đổi một byte của khoá nào cũng đổi dấu khoá", () => {
  const goc = dauKhoa(card(bytes(1), bytes(2)));
  const t = bytes(1); t[9] ^= 1;
  const m = bytes(2); m[0] ^= 1;
  assert.notEqual(dauKhoa(card(t, bytes(2))), goc);
  assert.notEqual(dauKhoa(card(bytes(1), m)), goc);
});

test("thẻ của máy này so với thẻ máy chủ giữ: cùng khoá mới là một", () => {
  const that = card(bytes(1), bytes(2));
  assert.equal(cungKhoa(that, card(bytes(1), bytes(2))), true);
  // The server swapped this phone's transport key for its own.
  assert.equal(cungKhoa(that, card(bytes(9), bytes(2))), false);
  assert.equal(cungKhoa(that, { ...card(bytes(1), bytes(2)), device_id: "khac" }), false);
});
