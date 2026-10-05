/* Sửa khoản chi sau khi phát đợt thu (ADR-0056): the bridge module.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-dieu-chinh.test.mjs
 *
 * Who must agree, the new transfers and whether an amendment applies are the
 * server's; the phone only refuses a share that is not whole đồng before it
 * leaves, and brings its stored links up to date once the server applied.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { ApiError, BASE_URL, datTokenPhien } from "../dist-test/api.js";
import {
  MAX_AMOUNT_VND,
  cauTrangThaiDieuChinh,
  coTheDeXuat,
  deXuatDieuChinh,
  dieuChinhDangMo,
  docHoSoDieuChinh,
  linkSauDieuChinh,
  loiNhanLinkDuyet,
  phanBoBanDau,
  toiCanTraLoi,
  tongPhanBo,
  traLoiDieuChinh,
} from "../dist-test/rudi/dot-thu/dieu-chinh.js";

const AN = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BINH = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const CHI = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const ATTEMPT = { key: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", at: Date.UTC(2026, 9, 5) };

function json(status, body) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const HO_SO = {
  batch_owner_id: AN,
  batch_status: "published",
  expenses: [
    { expense_id: "e-1", expense_version_id: "v-1", description: "Lẩu (dữ liệu mẫu)", paid_by_id: AN, recorded_by_id: AN, total_amount_vnd: 300000,
      allocations: [{ person_id: AN, amount_vnd: 100000 }, { person_id: BINH, amount_vnd: 100000 }, { person_id: CHI, amount_vnd: 100000 }] },
  ],
  amendments: [
    { id: "a-1", status: "proposed", expense_id: "e-1", proposed_by_id: AN, reason: "Chi không ăn", created_at: "2026-10-05T01:00:00Z",
      expires_at: "2026-10-12T01:00:00Z", resolved_at: null, applied_batch_version_id: null,
      lines: [{ sender_id: BINH, recipient_id: AN, old_amount_vnd: 100000, new_amount_vnd: 200000 }, { sender_id: CHI, recipient_id: AN, old_amount_vnd: 100000, new_amount_vnd: 0 }],
      parties: [{ person_id: AN, decision: "accept", via: "proposer" }, { person_id: BINH, decision: null, via: null }, { person_id: CHI, decision: null, via: null }],
      pending_person_ids: [BINH, CHI] },
  ],
};

test("docHoSoDieuChinh đọc khoản của đợt và đề xuất theo hình dạng máy chủ", async () => {
  datTokenPhien("token-thu");
  const goi = [];
  globalThis.fetch = async (url, init = {}) => {
    goi.push({ url: String(url), method: init.method });
    return json(200, HO_SO);
  };
  const hs = await docHoSoDieuChinh("c-1", "b-1", BINH);
  assert.match(goi[0].url, /\/batches\/b-1\/amendments$/);
  assert.equal(goi[0].method, "GET");
  assert.equal(hs.chuDot, AN);
  assert.deepEqual(hs.khoan[0].phanBo, [{ personId: AN, vnd: 100000 }, { personId: BINH, vnd: 100000 }, { personId: CHI, vnd: 100000 }]);
  const dc = dieuChinhDangMo(hs);
  assert.equal(dc.id, "a-1");
  assert.deepEqual(dc.dong[1], { senderId: CHI, recipientId: AN, cuVnd: 100000, moiVnd: 0 });
  assert.equal(dc.ben[0].traLoi, "dong_y");
  assert.equal(toiCanTraLoi(dc, BINH), true);
  assert.equal(toiCanTraLoi(dc, AN), false);
  assert.equal(cauTrangThaiDieuChinh(dc), "Chờ 2 người trả lời");
  assert.equal(coTheDeXuat(hs, hs.khoan[0], AN), true);
  assert.equal(coTheDeXuat(hs, hs.khoan[0], BINH), false);
  assert.deepEqual(phanBoBanDau(hs.khoan[0], [AN, BINH, CHI, "x"]), { [AN]: 100000, [BINH]: 100000, [CHI]: 100000, x: 0 });
  datTokenPhien(null);
});

test("deXuatDieuChinh chặn phần không phải đồng nguyên trước khi gửi, gửi đúng thân, trả link duyệt tuyệt đối", async () => {
  datTokenPhien("token-thu");
  const goi = [];
  globalThis.fetch = async (url, init = {}) => {
    goi.push({ url: String(url), body: init.body ? JSON.parse(init.body) : null });
    if (goi.length === 1) return json(201, { amendment_id: "a-2", status: "proposed", review_links: [{ sender_id: CHI, path: "/g/tok/dieu-chinh" }] });
    return json(409, { code: "amendment_already_open", detail: "open" });
  };
  const base = { contextId: "c-1", batchId: "b-1", actorId: AN, expenseId: "e-1", lyDo: "Chi không ăn", attempt: ATTEMPT };
  await assert.rejects(() => deXuatDieuChinh({ ...base, phanBo: { [AN]: 100000.5 } }), RangeError);
  await assert.rejects(() => deXuatDieuChinh({ ...base, phanBo: { [AN]: MAX_AMOUNT_VND, [BINH]: 1 } }), RangeError);
  assert.equal(goi.length, 0, "a share that is not whole đồng never left the phone");
  const kq = await deXuatDieuChinh({ ...base, phanBo: { [AN]: 100000, [BINH]: 200000, [CHI]: 0 } });
  assert.deepEqual(goi[0].body, { expense_id: "e-1", reason: "Chi không ăn", allocations: { [AN]: 100000, [BINH]: 200000, [CHI]: 0 } });
  assert.deepEqual(kq, { id: "a-2", trangThai: "proposed", links: [{ senderId: CHI, url: BASE_URL + "/g/tok/dieu-chinh" }] });
  await assert.rejects(
    () => deXuatDieuChinh({ ...base, phanBo: { [AN]: 1 } }),
    (e) => e instanceof ApiError && /đang có một đề xuất sửa/.test(e.message),
  );
  datTokenPhien(null);
});

test("traLoiDieuChinh gửi accept đúng kiểu boolean", async () => {
  datTokenPhien("token-thu");
  const goi = [];
  globalThis.fetch = async (url, init = {}) => {
    goi.push({ url: String(url), body: JSON.parse(init.body) });
    return json(200, { amendment_id: "a-1", status: "applied" });
  };
  const tt = await traLoiDieuChinh({ contextId: "c-1", batchId: "b-1", amendmentId: "a-1", actorId: BINH, dongY: false, attempt: ATTEMPT });
  assert.match(goi[0].url, /\/batches\/b-1\/amendments\/a-1\/decision$/);
  assert.deepEqual(goi[0].body, { accept: false });
  assert.equal(tt, "applied");
  datTokenPhien(null);
});

test("tongPhanBo: đồng nguyên, không âm, tổng trong trần", () => {
  assert.equal(tongPhanBo({ a: 1, b: 2 }), 3);
  assert.equal(tongPhanBo({ a: -1 }), null);
  assert.equal(tongPhanBo({ a: 0.5 }), null);
  assert.equal(tongPhanBo({ a: MAX_AMOUNT_VND }), MAX_AMOUNT_VND);
  assert.equal(tongPhanBo({ a: MAX_AMOUNT_VND, b: 1 }), null);
});

test("linkSauDieuChinh: khách được sửa giữ link duyệt (bỏ /dieu-chinh), số lấy từ bảng mới; khách khác giữ nguyên", () => {
  const envelopes = [
    { senderId: BINH, senderName: "Bình", amountVnd: 100000, url: "http://x/g/cu-binh", opened: false, obligations: [{ obligationId: "o-1", amountVnd: 100000 }] },
    { senderId: CHI, senderName: "Chi", amountVnd: 100000, url: "http://x/g/cu-chi", opened: false, obligations: [{ obligationId: "o-2", amountVnd: 100000 }] },
  ];
  const moi = linkSauDieuChinh(envelopes, [{ senderId: BINH, url: "http://x/g/moi-binh/dieu-chinh" }], [
    { id: "o-9", senderId: BINH, recipientId: AN, amountVnd: 200000, trangThai: "outstanding", tranhCai: false },
  ]);
  assert.deepEqual(moi[0], { ...envelopes[0], url: "http://x/g/moi-binh", amountVnd: 200000, obligations: [{ obligationId: "o-9", amountVnd: 200000 }] });
  assert.deepEqual(moi[1], envelopes[1]);
  assert.match(loiNhanLinkDuyet("Chi", "http://x/g/t/dieu-chinh"), /phần của Chi[\s\S]*http:\/\/x\/g\/t\/dieu-chinh/);
  assert.doesNotMatch(loiNhanLinkDuyet("Chi", "u"), /\d{3}\.\d{3}/, "the share message carries no amount");
});
