/* Nhắn riêng 1:1 on the wire and on screen (L2, ADR-0021 §2.5).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-nhan-rieng.test.mjs
 *
 * What matters: the door is one POST with the bearer and an attempt key and no
 * body of its own; a pair is named after the other person and never after an
 * id or an empty string; the money screens never pick a pair as the default
 * group; the row a fresh pair returns goes to the front of the session's list;
 * and every refusal reads as the one sentence the server sends.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien, newAttempt } from "../dist-test/api.js";
import { chonNhomMacDinh } from "../dist-test/phien.js";
import {
  LOI_NHAN_RIENG,
  TEN_NGUOI_KHUYET,
  ghepVaoDanhSach,
  laPair,
  moNhanRieng,
  tenCuocTroChuyen,
} from "../dist-test/rudi/nhan-rieng/nhan-rieng.js";

const ME = "4dd00000-dddd-4ddd-8ddd-0000d0000009";
const BAN = "5ee00000-eeee-4eee-8eee-0000e0000009";
const PAIR = "6ff00000-ffff-4fff-8fff-0000f0000009";
const NHOM = "1aa00000-aaaa-4aaa-8aaa-0000a0000009";

function hang(extra = {}) {
  return {
    id: PAIR,
    display_name: "Bạn Thân",
    my_state: "active",
    my_role: "member",
    membership_id: "7aa00000-aaaa-4aaa-8aaa-0000a0000010",
    member_count: 2,
    unread_count: 0,
    last_message: null,
    theme: "mac-dinh",
    kind: "pair",
    counterpart: { id: BAN, display_name: "Bạn Thân" },
    ...extra,
  };
}

function traLoi(body, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}

test.afterEach(() => datTokenPhien(null));

test("moNhanRieng là một POST tới /people/{id}/dm với bearer và khoá attempt, không thân", async () => {
  datTokenPhien("tok");
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url, init });
    return traLoi(hang(), 201);
  };
  try {
    const ra = await moNhanRieng(BAN, ME, newAttempt());
    assert.equal(ra.id, PAIR);
    assert.equal(ra.kind, "pair");
  } finally {
    globalThis.fetch = truoc;
  }
  assert.equal(daGoi.length, 1);
  assert.match(String(daGoi[0].url), new RegExp(`/people/${BAN}/dm$`));
  assert.equal(daGoi[0].init.method, "POST");
  assert.equal(daGoi[0].init.headers.Authorization, "Bearer tok");
  assert.ok(daGoi[0].init.headers["Idempotency-Key"], "một cú bấm hai lần chỉ mở một cặp");
  assert.equal(daGoi[0].init.body, undefined, "cửa này không nhận thân");
});

test("mọi từ chối đọc thành đúng câu của máy chủ, không thêm lý do", async () => {
  datTokenPhien("tok");
  const truoc = globalThis.fetch;
  globalThis.fetch = async () => traLoi({ code: "person_not_found", detail: "Chưa thể nhắn riêng với người này." }, 404);
  try {
    await assert.rejects(
      moNhanRieng(BAN, ME, newAttempt()),
      (loi) => loi.code === "person_not_found" && loi.message === "Chưa thể nhắn riêng với người này.",
    );
  } finally {
    globalThis.fetch = truoc;
  }
  assert.equal(LOI_NHAN_RIENG.person_not_found, LOI_NHAN_RIENG.permission_denied);
  for (const cau of Object.values(LOI_NHAN_RIENG)) {
    assert.ok(!cau.includes("—"), cau);
    assert.ok(!/chặn|xoá|bạn bè/i.test(cau), `câu không được nói lý do: ${cau}`);
  }
});

test("một cặp gọi bằng tên người kia; nhóm gọi bằng tên nhóm; thiếu tên thì là một từ, không phải id", () => {
  assert.equal(tenCuocTroChuyen(hang()), "Bạn Thân");
  assert.equal(tenCuocTroChuyen({ display_name: "Hội đi Đà Lạt", kind: "group" }), "Hội đi Đà Lạt");
  assert.equal(tenCuocTroChuyen({ display_name: "Hội cũ" }), "Hội cũ", "máy chủ cũ không gửi kind");
  assert.equal(tenCuocTroChuyen(hang({ display_name: "", counterpart: { id: BAN, display_name: "" } })), TEN_NGUOI_KHUYET);
  assert.equal(tenCuocTroChuyen(hang({ display_name: "", counterpart: null })), TEN_NGUOI_KHUYET);
  assert.equal(tenCuocTroChuyen(hang({ display_name: "Người kia", counterpart: undefined })), "Người kia");
  assert.equal(tenCuocTroChuyen(undefined), "Nhóm");
  const thieu = tenCuocTroChuyen(hang({ display_name: "", counterpart: { id: BAN, display_name: "  " } }));
  assert.ok(!thieu.includes(BAN) && !thieu.includes("-"), "không bao giờ in id");
});

test("laPair chỉ đúng với kind=pair", () => {
  assert.equal(laPair(hang()), true);
  assert.equal(laPair({ kind: "group" }), false);
  assert.equal(laPair({}), false);
  assert.equal(laPair(undefined), false);
});

test("chonNhomMacDinh không bao giờ chọn một cặp làm nhóm mặc định của màn tiền", () => {
  const nhom = { ...hang({ id: NHOM, display_name: "Hội", kind: "group", counterpart: null }) };
  const phien = { token: "t", person_id: ME, context_id: null, membership_state: null, membership_id: null, expires_at: "2031-01-01T00:00:00Z" };
  assert.equal(chonNhomMacDinh({ ...phien, contexts: [hang(), nhom] }).context_id, NHOM, "bỏ qua cặp, lấy nhóm");
  assert.equal(chonNhomMacDinh({ ...phien, contexts: [hang()] }).context_id, null, "chỉ có cặp thì không có nhóm mặc định");
  assert.equal(chonNhomMacDinh({ ...phien, contexts: [nhom, hang()] }).context_id, NHOM);
});

test("ghepVaoDanhSach đặt cặp mới lên đầu và thay hàng cũ cùng id", () => {
  const nhom = hang({ id: NHOM, display_name: "Hội", kind: "group", counterpart: null });
  const cu = hang({ display_name: "Tên cũ" });
  const moi = hang({ display_name: "Bạn Thân" });
  const ra = ghepVaoDanhSach([nhom, cu], moi);
  assert.deepEqual(ra.map((n) => n.id), [PAIR, NHOM]);
  assert.equal(ra[0].display_name, "Bạn Thân");
  assert.deepEqual(ghepVaoDanhSach(undefined, moi).map((n) => n.id), [PAIR]);
});
