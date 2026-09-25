/* `/chia-bill` đi cùng một đường với `/plan` (ADR-0036 §2.9).
 *
 * Đo: lệnh gõ trong khung chat ra đúng lệnh và đúng lời nhờ; thân gửi lên mang
 * `command: "chia_bill"` cùng gói bối cảnh y như plan; máy chủ cũ không khai
 * chia_bill thì client coi là chưa sẵn sàng; hàng lời gọi hỏng nói đúng việc và
 * không mời thử lại khi thử lại cũng ra đúng câu trả lời cũ; chip xem trước
 * trên nút gửi (thay khối «Mình đang thấy» của khay, ADR-0039) giữ «Chỉ gửi
 * lời nhờ» cho cả hai lệnh.
 *
 * KHÔNG đo: màn thật render ra sao (ảnh chụp là cổng riêng), hay mô hình đọc
 * đúng số tiền (máy chủ gọi skill chat-expense có sẵn; tầng này không gọi mô
 * hình nào).
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
 *     node --test tests/goi-ai-chia-bill.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
  chuHangLoiGoi,
  goiAi,
  lenhSanSang,
  LOI_KET_QUA_AI,
  LOI_NHO_CHIA_BILL,
  thuLaiDuoc,
} from "../dist-test/rudi/chat/ai-invocations.js";
import { timNhacAi } from "../dist-test/rudi/chat/nhac-ai.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const GOC_APP = join(HERE, "..");
const GOC_REPO = join(GOC_APP, "..", "..");
const context = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const actor = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";

test("lệnh gõ trong khung chat ra đúng lệnh và đúng lời nhờ", () => {
  assert.deepEqual(timNhacAi("/chia-bill"), { lenh: "chia_bill", loiNho: LOI_NHO_CHIA_BILL });
  assert.deepEqual(timNhacAi("/chiabill  "), { lenh: "chia_bill", loiNho: LOI_NHO_CHIA_BILL });
  assert.deepEqual(timNhacAi("/Chia-Bill mình trả 300k tiền nước"), { lenh: "chia_bill", loiNho: "mình trả 300k tiền nước" });
  assert.deepEqual(timNhacAi("/plan tối nay đi đâu"), { lenh: "plan", loiNho: "tối nay đi đâu" });
  assert.deepEqual(timNhacAi("@Rủ Đi gợi ý quán"), { lenh: "plan", loiNho: "gợi ý quán" });
  // The server refuses an empty prompt; a bare command must still be sendable.
  assert.ok(LOI_NHO_CHIA_BILL.trim().length > 0);
});

test("chia_bill gửi cùng thân với plan, chỉ khác lệnh", async () => {
  const original = globalThis.fetch;
  const calls = [];
  datTokenPhien("synthetic-test-token");
  globalThis.fetch = async (url, options) => { calls.push({ url, ...options }); return { ok: true, status: 202, json: async () => ({ id: "job" }), text: async () => "{}" }; };
  const boiCanh = { ban: 1, nguon: "chat-nhom", luot: [{ id: actor, vai: "ban", biDanh: "Lan", loai: "chu", luc: "2030-09-22T10:00:00Z", chu: "Tao trả 300k" }], tongLuot: 1, daCat: false };
  try {
    await goiAi(context, actor, "Gom giúp", actor, boiCanh, "chia_bill");
    await goiAi(context, actor, "Gom giúp", actor, undefined, "chia_bill");
    await goiAi(context, actor, "Đi đâu", actor);
    assert.match(calls[0].url, /\/ai-invocations$/);
    assert.deepEqual(JSON.parse(calls[0].body), { logical_id: actor, command: "chia_bill", prompt: "Gom giúp", boi_canh: boiCanh });
    // «Chỉ gửi lời nhờ»: no bundle key at all, exactly as for plan.
    assert.deepEqual(JSON.parse(calls[1].body), { logical_id: actor, command: "chia_bill", prompt: "Gom giúp" });
    assert.equal(JSON.parse(calls[2].body).command, "plan");
  } finally { globalThis.fetch = original; datTokenPhien(null); }
});

test("máy chủ không khai chia_bill thì coi là chưa sẵn sàng", () => {
  const cu = { protocol: "legacy", realtime: { available: true }, ai: { plan: { available: true, reason: null }, share_scope: "caller_attached" }, media: {} };
  assert.equal(lenhSanSang(cu, "plan"), true);
  assert.equal(lenhSanSang(cu, "chia_bill"), false);
  const moi = { ...cu, ai: { ...cu.ai, chia_bill: { available: true, reason: null } } };
  assert.equal(lenhSanSang(moi, "chia_bill"), true);
  assert.equal(lenhSanSang(null, "plan"), false);
});

test("hàng lời gọi chia_bill nói đúng việc, và không mời thử lại khi vô ích", () => {
  const job = (extra) => ({ id: "j", status: "failed", code: null, message_id: null, created_at: "", updated_at: "", ...extra });
  const khongKhoan = job({ command: "chia_bill", code: "chia_bill_no_expenses" });
  assert.equal(chuHangLoiGoi(khongKhoan).tieuDe, "Chưa gom được khoản chi");
  assert.equal(chuHangLoiGoi(khongKhoan).cau, LOI_KET_QUA_AI.chia_bill_no_expenses);
  assert.equal(thuLaiDuoc(khongKhoan), false);
  assert.equal(thuLaiDuoc(job({ command: "chia_bill", code: "provider_unavailable" })), true);
  // An older server echoes no command: the row is a plan row, as before.
  assert.equal(chuHangLoiGoi(job({})).tieuDe, "Chưa phác được tờ hẹn");
  assert.equal(chuHangLoiGoi(job({ status: "running", command: "chia_bill" })).tieuDe, "Đang gom khoản chi…");
});

test("mỗi câu kết quả là một mã worker Go thật sự ghi, và viết bằng giọng người", () => {
  // Every non-test Go file of the engine: `trigger_deleted` is written by the
  // publish step, not by the bill reader.
  const goc = join(GOC_REPO, "services", "core", "internal", "chatassist");
  const go = readdirSync(goc).filter((ten) => ten.endsWith(".go") && !ten.endsWith("_test.go")).map((ten) => readFileSync(join(goc, ten), "utf8")).join("\n");
  for (const [ma, cau] of Object.entries(LOI_KET_QUA_AI)) {
    assert.ok(go.includes(`"${ma}"`), `câu cho ${ma} nhưng worker không ghi mã đó`);
    assert.doesNotMatch(cau, /[a-z]+_[a-z_]+/, `câu chữ chứa một mã máy: ${cau}`);
    assert.doesNotMatch(cau, /lỗi|HTTP|[—–]/i, `câu viết như báo lỗi: ${cau}`);
    assert.ok(cau.length > 20);
  }
});

test("chip xem trước giữ «Chỉ gửi lời nhờ» cho cả chia_bill, và lời gọi mang đúng lệnh đã gõ", () => {
  // The preview is the chip above the send button now (ADR-0039 §2.3). It
  // takes no command at all: what protects the person cannot differ by
  // command, so it cannot be switched off by one.
  const chip = readFileSync(join(GOC_APP, "src", "rudi", "screens", "chat", "ChipBoiCanh.tsx"), "utf8");
  const props = chip.slice(chip.indexOf("export function ChipBoiCanh("), chip.indexOf(") {", chip.indexOf("export function ChipBoiCanh(")));
  assert.doesNotMatch(props, /lenh|chiaBill/, "chip không được phụ thuộc lệnh");
  const chu = readFileSync(join(GOC_APP, "src", "rudi", "chat", "chip-boi-canh.ts"), "utf8");
  assert.match(chu, /"Chỉ gửi lời nhờ"/);
  const live = readFileSync(join(GOC_APP, "src", "rudi", "screens", "chat", "GroupChatLive.tsx"), "utf8");
  assert.doesNotMatch(live, /chưa được bật/, "câu cũ «chưa được bật» còn chặn /chia-bill");
  assert.match(live, /lenh: nhac\.lenh, loiNho: nhac\.loiNho/, "lời gọi phải mang đúng lệnh của tin vừa gõ");
});
