/* Hàng ghim dưới tiêu đề chat hai người: ai thấy gì (chủ sản phẩm 2026-09-28).
 *
 * Hồi quy của lát P2: gác hàng «Tờ giấy» theo `capDoi` làm lời đề nghị lập sổ
 * / bật «Một đôi» của người kia biến mất khỏi chat đám bạn — người nhận chỉ
 * biết khi tự vào «Cài đặt nhóm» → «Tờ giấy của hai mình».
 *
 * Đo: `hangGhimChat` — cặp đôi → hàng đầy đủ; cặp đám bạn + đề nghị của
 * người kia mà tôi chưa trả lời → hàng mời; lời đề nghị của chính tôi, đã trả
 * lời, hoặc không có gì → không hàng nào; nhóm → không hàng nào (và không đọc
 * sổ — gác nguồn ở tests/ai-chat-hai-nguoi.test.mjs).
 *
 * KHÔNG đo: hàng vẽ ra sao trên máy thật (Maestro 47).
 */
import assert from "node:assert/strict";
import test from "node:test";

import { hangGhimChat } from "../dist-test/rudi/to-giay/so-doi-map.js";

const TOI = "toi", KIA = "nguoi-ay";
const deNghi = (over = {}) => ({ id: "dn-1", purpose: "lap_so", expires_at: "2026-10-05T00:00:00Z", proposed_by_id: KIA, my_granted: false, ...over });
const so = (pending = []) => ({
  context_id: "cap-1",
  cycle_state: null,
  participants: [TOI, KIA],
  my_consents: [],
  their_consents_granted: {},
  pending_proposals: pending,
  constraints: [],
  nep_gui_ho: false,
  open_paper_id: null,
});
const ban = (s) => hangGhimChat({ haiNguoi: true, capDoi: false, so: s, toiId: TOI });

test("cặp đôi giữ hàng «Tờ giấy» đầy đủ, có hay không có đề nghị", () => {
  assert.deepEqual(hangGhimChat({ haiNguoi: true, capDoi: true, so: so(), toiId: TOI }), { loai: "to-giay", deNghi: undefined });
  assert.deepEqual(hangGhimChat({ haiNguoi: true, capDoi: true, so: null, toiId: TOI }), { loai: "to-giay", deNghi: undefined });
  const cua = deNghi({ purpose: "bat_doi" });
  assert.deepEqual(hangGhimChat({ haiNguoi: true, capDoi: true, so: so([cua]), toiId: TOI }), { loai: "to-giay", deNghi: cua });
});

test("cặp đám bạn: người kia đề nghị lập sổ hay bật «Một đôi» → hàng mời", () => {
  const lapSo = deNghi();
  assert.deepEqual(ban(so([lapSo])), { loai: "loi-moi", deNghi: lapSo });
  const batDoi = deNghi({ id: "dn-2", purpose: "bat_doi" });
  assert.deepEqual(ban(so([batDoi])), { loai: "loi-moi", deNghi: batDoi });
  // Mine first in the list does not hide theirs.
  assert.deepEqual(ban(so([deNghi({ id: "cua-toi", proposed_by_id: TOI, my_granted: true }), batDoi])), { loai: "loi-moi", deNghi: batDoi });
});

test("cặp đám bạn: đề nghị của chính tôi, đã trả lời, hay không có gì → không hàng nào", () => {
  assert.equal(ban(so()), null);
  assert.equal(ban(null), null, "sổ chưa đọc xong: im lặng, không đoán");
  assert.equal(ban(so([deNghi({ proposed_by_id: TOI, my_granted: true })])), null, "lời đề nghị của tôi không phải câu hỏi cho tôi");
  // Even if a server ever sent my own proposal without my grant on it.
  assert.equal(ban(so([deNghi({ proposed_by_id: TOI, my_granted: false })])), null);
  assert.equal(ban(so([deNghi({ my_granted: true })])), null, "tôi đã đồng ý thì hàng tự tắt");
  // `doc_chat` is retired; a friends' pair is never invited into it.
  assert.equal(ban(so([deNghi({ purpose: "doc_chat" })])), null);
});

test("nhóm: không hàng nào, kể cả khi lỡ có dữ liệu sổ", () => {
  assert.equal(hangGhimChat({ haiNguoi: false, capDoi: false, so: so([deNghi()]), toiId: TOI }), null);
  assert.equal(hangGhimChat({ haiNguoi: false, capDoi: true, so: so([deNghi()]), toiId: TOI }), null);
});
