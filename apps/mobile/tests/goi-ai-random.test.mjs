import assert from "node:assert/strict";
import test from "node:test";

import { chuHangLoiGoi, goiAi, lenhSanSang, LOI_GOI_AI } from "../dist-test/rudi/chat/ai-invocations.js";
import { tachLoiNho, timNhacAi } from "../dist-test/rudi/chat/nhac-ai.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";
import { docDiaDiem } from "../dist-test/screens/chat/ke-hoach.js";

test("lựa chọn giữ tên, id và nguồn của hàng danh mục", () => {
  const place = docDiaDiem({ id: "synthetic-place", name: "Quán Thử", source: "curated" });
  assert.deepEqual(place, { id: "synthetic-place", ten: "Quán Thử", nguon: "curated" });
  assert.equal(docDiaDiem({ name: "Món bịa", source: "ai" }), null);
});

test("parser giữ nguyên khóa text NFD/khoảng trắng nhưng vẫn trim tên và bỏ khóa trắng", () => {
  for (const id of ["q-eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", "p-tiem-nuong-xom-lao", "  Cafe\u0301 / Đà Lạt  ", "a%2Fb", "%2e%2e", "%252e"]) {
    const place = docDiaDiem({ id, name: "  Quán Thử  ", source: "curated" });
    assert.deepEqual(place, { id, ten: "Quán Thử", nguon: "curated" });
  }
  for (const id of [null, undefined, "", "   ", 7]) {
    assert.equal(docDiaDiem({ id, name: "Quán Thử" }), null);
    assert.deepEqual(docDiaDiem({ id, name: "  Tự nhập  " }, true), { id: "", ten: "Tự nhập" });
  }
});

test("/random chỉ nhận token đầu và giữ đúng lời nhờ", () => {
  assert.deepEqual(timNhacAi(" /RaNdOm quán ăn ở Đà Lạt"), { lenh: "random", loiNho: "quán ăn ở Đà Lạt" });
  assert.deepEqual(timNhacAi("/random @Rủ Đi tìm quán".normalize("NFD")), { lenh: "random", loiNho: "tìm quán" });
  for (const s of ["/randomize quán", "hôm nay /random quán", "/random-quán"]) assert.equal(timNhacAi(s), null);
  // Bare random opens the editor; it never invents a request or candidates.
  assert.deepEqual(timNhacAi("/random"), { lenh: "random", loiNho: "" });
  assert.equal(tachLoiNho("/random @Rủ Đi, "), "");
});

test("random cần capability và tin nhờ; máy chủ cũ đóng mặc định", () => {
  const old = { ai: { plan: { available: true, reason: null }, mention: true } };
  assert.equal(lenhSanSang(old, "random"), false);
  assert.equal(lenhSanSang(null, "random"), false);
  assert.equal(lenhSanSang({ ai: { ...old.ai, random: { available: true, reason: null } } }, "random"), true);
  assert.equal(lenhSanSang({ ai: { random: { available: true, reason: null } } }, "random"), false);
  assert.equal(lenhSanSang({ ai: { ...old.ai, random: { available: false, reason: "random_source_unavailable" } } }, "random"), false);
  assert.match(LOI_GOI_AI.random_source_unavailable, /tự thêm lựa chọn/);
});

test("random giữ cùng lời gọi, gói chia sẻ và trigger như hàng đợi sẵn có", async () => {
  const original = globalThis.fetch;
  const calls = [];
  const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const trigger = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const bundle = { ban: 1, nguon: "chat-nhom", luot: [], tongLuot: 0, daCat: false };
  datTokenPhien("synthetic-test-token");
  globalThis.fetch = async (url, options) => { calls.push({ url, ...options }); return { ok: true, status: 202, json: async () => ({ id: "job", command: "random" }), text: async () => "{}" }; };
  try {
    await goiAi(id, id, "quán ăn ở Đà Lạt", id, bundle, "random", trigger);
    await goiAi(id, id, "quán ăn ở Đà Lạt", id, undefined, "random", trigger);
    assert.match(calls[0].url, /\/ai-invocations$/);
    assert.deepEqual(JSON.parse(calls[0].body), { logical_id: id, command: "random", prompt: "quán ăn ở Đà Lạt", boi_canh: bundle, trigger_message_id: trigger });
    assert.deepEqual(JSON.parse(calls[1].body), { logical_id: id, command: "random", prompt: "quán ăn ở Đà Lạt", trigger_message_id: trigger });
  } finally { globalThis.fetch = original; datTokenPhien(null); }
});

test("hàng random nói tìm lựa chọn và cho tự thêm khi chưa tìm được", () => {
  const job = { id: "j", command: "random", status: "running", code: null, message_id: null, created_at: "", updated_at: "" };
  assert.equal(chuHangLoiGoi(job).tieuDe, "Đang tìm lựa chọn…");
  const failed = chuHangLoiGoi({ ...job, status: "failed" });
  assert.equal(failed.tieuDe, "Chưa tìm được lựa chọn");
  assert.match(failed.cau, /tự thêm lựa chọn/);
  assert.doesNotMatch(failed.cau, /tạo kèo|phác|Chia bill/);
});
