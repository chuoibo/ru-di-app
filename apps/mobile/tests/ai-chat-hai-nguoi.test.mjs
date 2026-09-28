/* Rủ Đi AI trong chat hai người (thiết kế 2026-09-28, lát M1).
 *
 * Đo: ở cặp, «@Rủ Đi» là lời nhờ `hoi` (lệnh duy nhất máy chủ mở cho cặp),
 * lời nhờ trống có câu mặc định viết cho hai người; «/plan», «/chia-bill»
 * không được gợi ý và không thành lời gọi; chip đọc độ sẵn sàng của `hoi` từ
 * chat-capabilities (cần cả `mention`, vì `hoi` phải nêu tin); lời gọi lên
 * máy chủ mang `command: "hoi"` và tin tag; chữ ở cặp không nói «nhóm» hay
 * «cả nhóm»; màn không còn chặn cặp trước timNhacAi. Nhóm giữ nguyên.
 *
 * KHÔNG đo: máy chủ thật nhận `hoi` ở cặp (việc của lát S1, test Postgres
 * phía Go), màn vẽ ra sao trên máy thật (Maestro 41, chưa chạy được ở cloud).
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { LOI_NHO_HOI_HAI_NGUOI, LOI_NHO_PLAN, timNhacAi } from "../dist-test/rudi/chat/nhac-ai.js";
import { CAU_CHO_TRA_LOI, LOI_GOI_AI, chuHangLoiGoi, goiAi, lenhSanSang } from "../dist-test/rudi/chat/ai-invocations.js";
import { chuKhay, lenhGoiY } from "../dist-test/rudi/chat/khay-cong-cu.js";
import { cauXem, cauXemCach, chuChip } from "../dist-test/rudi/chat/chip-boi-canh.js";
import { gomBoiCanhChat } from "../dist-test/rudi/chat/boi-canh-chat.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = join(HERE, "..", "src", "rudi");
const toi = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const ban = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const phong = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const NHOM = /nhóm|hội/i;

function tin(i, chu) {
  const id = `aaaaaaaa-bbbb-4ccc-8ddd-${String(i).padStart(12, "e")}`;
  return { id, context_id: phong, author_id: i % 2 ? toi : ban, kind: "text", body: chu, image_url: null, card: null, created_at: `2030-09-22T10:${String(i % 60).padStart(2, "0")}:00Z`, cursor: id };
}

/** chat-capabilities as the server reports them for a pair (design §2.1). */
const CAP_CAP = {
  protocol: "legacy",
  realtime: { available: true },
  ai: {
    plan: { available: false, reason: "group_plan_only" },
    chia_bill: { available: false, reason: "group_plan_only" },
    hoi: { available: true, reason: null },
    share_scope: "caller_attached",
    mention: true,
  },
  media: { image: true, sticker: true, voice: false },
};

test("cặp: @Rủ Đi là lời nhờ hoi; lệnh gạch chéo không thành lời gọi; nhóm giữ nguyên", () => {
  assert.deepEqual(timNhacAi("@Rủ Đi gợi ý quán", true), { lenh: "hoi", loiNho: "gợi ý quán" });
  assert.deepEqual(timNhacAi("tối nay @rudi đi đâu", true), { lenh: "hoi", loiNho: "tối nay đi đâu" });
  assert.deepEqual(timNhacAi("/chia-bill @Rủ Đi gom giúp", true), { lenh: "hoi", loiNho: "gom giúp" });
  for (const thuong of ["/plan tối nay đi đâu", "/PLAN", "/chia-bill", "/chiabill mình trả 300k", "tối nay ăn gì", "hỏi rudi thử xem"]) {
    assert.equal(timNhacAi(thuong, true), null, `«${thuong}» ở cặp không được thành lời gọi AI`);
  }
  // Group behaviour is untouched.
  assert.deepEqual(timNhacAi("@Rủ Đi gợi ý quán"), { lenh: "plan", loiNho: "gợi ý quán" });
  assert.equal(timNhacAi("/chia-bill")?.lenh, "chia_bill");
});

test("cặp: @Rủ Đi trống mang câu mặc định cho hai người, không phải câu phác kèo của nhóm", () => {
  for (const tran of ["@Rủ Đi", "@rudi ", "@Rủ Đi, @ru di", "/plan @Rủ Đi"]) {
    const nhac = timNhacAi(tran, true);
    assert.deepEqual(nhac, { lenh: "hoi", loiNho: LOI_NHO_HOI_HAI_NGUOI }, tran);
  }
  assert.notEqual(LOI_NHO_HOI_HAI_NGUOI, LOI_NHO_PLAN);
  assert.doesNotMatch(LOI_NHO_HOI_HAI_NGUOI, NHOM);
  assert.match(LOI_NHO_HOI_HAI_NGUOI, /hai/);
  assert.equal(timNhacAi("@Rủ Đi")?.loiNho, LOI_NHO_PLAN, "nhóm vẫn giữ câu mặc định cũ");
});

test("độ sẵn sàng của hoi đọc từ chat-capabilities, cần cả mention, và đóng khi thiếu", () => {
  assert.equal(lenhSanSang(CAP_CAP, "hoi"), true);
  // What the server refuses in a pair is not offered.
  assert.equal(lenhSanSang(CAP_CAP, "plan"), false);
  assert.equal(lenhSanSang(CAP_CAP, "chia_bill"), false);
  // `hoi` must name its trigger: a server without `mention` cannot take it.
  assert.equal(lenhSanSang({ ...CAP_CAP, ai: { ...CAP_CAP.ai, mention: undefined } }, "hoi"), false);
  assert.equal(lenhSanSang({ ...CAP_CAP, ai: { ...CAP_CAP.ai, hoi: undefined } }, "hoi"), false);
  assert.equal(lenhSanSang({ ...CAP_CAP, ai: { ...CAP_CAP.ai, hoi: { available: false, reason: "provider_unavailable" } } }, "hoi"), false);
  assert.equal(lenhSanSang(null, "hoi"), false);
});

test("cặp: lời gọi lên máy chủ mang command hoi và tin tag, kèm gói người gọi đã xem", async () => {
  const goi = gomBoiCanhChat({ tin: [tin(2, "cuối tuần rảnh không"), tin(1, "rảnh nè")], personId: toi });
  const nhac = timNhacAi("@Rủ Đi đi đâu giờ", true);
  const original = globalThis.fetch;
  const calls = [];
  datTokenPhien("synthetic-test-token");
  globalThis.fetch = async (url, options) => { calls.push({ url, ...options }); return { ok: true, status: 202, json: async () => ({ id: "job" }), text: async () => "{}" }; };
  try {
    const trigger = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
    await goiAi(phong, toi, nhac.loiNho, toi, goi, nhac.lenh, trigger);
    assert.equal(calls.length, 1);
    assert.match(String(calls[0].url), new RegExp(`/contexts/${phong}/ai-invocations$`));
    const body = JSON.parse(calls[0].body);
    assert.equal(body.command, "hoi");
    assert.equal(body.prompt, "đi đâu giờ");
    assert.equal(body.trigger_message_id, trigger);
    assert.equal(body.boi_canh.luot.length, 2);
  } finally { globalThis.fetch = original; datTokenPhien(null); }
});

test("cặp: ô soạn không gợi ý /plan hay /chia-bill; khay có lối vào Rủ Đi AI bằng @Rủ Đi", () => {
  const cap = lenhGoiY(true);
  assert.deepEqual(cap.map((l) => l.nhan), ["/vote", "@Rủ Đi"]);
  for (const l of cap) assert.doesNotMatch(l.moTa, NHOM, l.moTa);
  assert.deepEqual(lenhGoiY(false).map((l) => l.nhan), ["/plan", "/vote", "/chia-bill", "@Rủ Đi"]);

  const k = chuKhay(true);
  assert.equal(k.lenhAi, "hoi");
  assert.equal(k.moDauHoiAi, "@Rủ Đi ");
  assert.equal(timNhacAi(k.moDauHoiAi, true)?.lenh, "hoi", "chữ khay điền sẵn phải là một lời nhờ ở cặp");
  assert.ok(k.hoiAiTrenKhay, "khay của cặp phải có lối vào Rủ Đi AI");
  assert.equal(k.hoiAiTrenKhay.label, "Hỏi Rủ Đi AI");
  for (const cau of Object.values(k.hoiAiTrenKhay)) assert.doesNotMatch(cau, NHOM, cau);
  const g = chuKhay(false);
  assert.equal(g.lenhAi, "plan");
  assert.equal(g.hoiAiTrenKhay, null);
});

test("cặp: chữ của chip, tấm «Xem», câu từ chối và hàng hỏng không nói «nhóm»", () => {
  const trong = gomBoiCanhChat({ tin: [], personId: toi });
  const goi = gomBoiCanhChat({ tin: [tin(1, "chào")], personId: toi });
  const cauCap = [
    chuChip(trong, true, true, true).cau,
    chuChip(goi, true, true, true).cau,
    chuChip(goi, false, true, true).cau,
    chuChip(goi, true, false, true).cau,
    cauXem(goi, true),
    cauXemCach(true),
    ...Object.values(LOI_GOI_AI),
    ...Object.values(CAU_CHO_TRA_LOI),
  ];
  for (const cau of cauCap) assert.doesNotMatch(cau, /nhóm|thành viên/i, cau);
  assert.match(cauXem(goi, true), /cả hai bạn/);
  // The group keeps its words.
  assert.equal(chuChip(trong, true, true).cau, "Nhóm chưa có tin nào, chỉ gửi lời nhờ");
  assert.match(cauXem(goi), /cả nhóm/);

  const hong = chuHangLoiGoi({ id: "x", command: "hoi", status: "failed", code: null, message_id: null, created_at: "", updated_at: "" });
  for (const cau of Object.values(hong)) assert.doesNotMatch(cau, /nhóm|tờ hẹn|kèo/i, cau);
});

test("màn chat: cặp không còn bị chặn trước timNhacAi, và chữ cặp đi qua cờ, không tách màn", () => {
  const live = readFileSync(join(SRC, "screens", "chat", "GroupChatLive.tsx"), "utf8");
  assert.doesNotMatch(live, /nhanRieng \? null : timNhacAi/, "cổng cũ chặn cặp ở chữ đang gõ");
  assert.doesNotMatch(live, /!nhanRieng \? timNhacAi/, "cổng cũ chặn cặp lúc gửi");
  assert.match(live, /const nhacDangGo = timNhacAi\(nhap, nhanRieng\)/);
  assert.match(live, /const nhac = command === undefined \? timNhacAi\(body, nhanRieng\) : null/);
  assert.match(live, /<ChipBoiCanh goi=\{goiChip\} haiNguoi=\{nhanRieng\}/);
  assert.match(live, /lenhGoiY\(nhanRieng\)/);
  assert.match(live, /chuKhay\(nhanRieng\)\.moDauHoiAi/);
  // «Tự tạo kèo» under a failed request belongs to a plan, never to `hoi`.
  assert.match(live, /\(request\.command \?\? "plan"\) === "plan" \? <RudiButton label="Tự tạo kèo"/);
  // No plan cards in a pair.
  assert.match(live, /if \(nhanRieng\) return null;/);
  const soHen = readFileSync(join(SRC, "screens", "chat", "SoHen.tsx"), "utf8");
  assert.match(soHen, /lenhSanSang\(capabilities, chu\.lenhAi\)/, "khay phải đọc độ sẵn sàng của đúng lệnh nó mời");
  assert.match(soHen, /chu\.hoiAiTrenKhay/);
});
