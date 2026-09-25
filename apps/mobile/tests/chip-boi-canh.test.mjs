/* Chip xem trước trên nút gửi (ADR-0039 §2.3, đề xuất; giữ ADR-0036 §2.5).
 *
 * Đo: số tin trên chip đọc từ CHÍNH gói sẽ gửi, không từ danh sách trên màn
 * (hai con số lệch nhau ngay khi gói bị cắt theo byte hay theo trần lượt);
 * «Xem» chỉ có khi có tin đi kèm; «Chỉ gửi lời nhờ» là một chạm và sau chạm
 * đó KHÔNG lượt nào đi — thân gửi lên không có khoá `boi_canh`, nhưng vẫn
 * mang tin tag; chip nói thật khi AI chưa sẵn sàng và khi nhóm chưa có tin.
 *
 * KHÔNG đo: chip vẽ ra sao trên máy thật (cổng ảnh chụp là việc riêng, chưa
 * chạy ở lát này), hay người thật hiểu câu chữ.
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
 *     node --test tests/chip-boi-canh.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { CHI_GUI_LOI_NHO, cauXem, chuChip, goiSeGui } from "../dist-test/rudi/chat/chip-boi-canh.js";
import { gomBoiCanhChat } from "../dist-test/rudi/chat/boi-canh-chat.js";
import { goiAi } from "../dist-test/rudi/chat/ai-invocations.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = join(HERE, "..", "src", "rudi");
const toi = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const ban = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const phong = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";

function tin(i, chu) {
  const id = `aaaaaaaa-bbbb-4ccc-8ddd-${String(i).padStart(12, "e")}`;
  return { id, context_id: phong, author_id: i % 2 ? toi : ban, kind: "text", body: chu, image_url: null, card: null, created_at: `2030-09-22T10:${String(i % 60).padStart(2, "0")}:00Z`, cursor: id };
}

test("số tin trên chip là số lượt của gói, không phải số tin trên màn", () => {
  // 60 long messages on screen: the bundle keeps at most 40 turns and then
  // drops whole turns from the old end until it fits the byte ceiling.
  // Three-byte letters at the per-turn cap: 40 of them are well over the
  // bundle's byte ceiling.
  const tinHien = Array.from({ length: 60 }, (_, i) => tin(60 - i, "ờ".repeat(400)));
  const goi = gomBoiCanhChat({ tin: tinHien, personId: toi });
  assert.ok(goi.luot.length < 40 && goi.luot.length > 0, `gói phải bị cắt theo byte, có ${goi.luot.length} lượt`);
  const chu = chuChip(goi, true, true);
  assert.equal(chu.cau, `Kèm ${goi.luot.length} tin gần đây`);
  assert.notEqual(chu.cau, `Kèm ${tinHien.length} tin gần đây`);
  assert.equal(chu.xem, true);
  assert.equal(chu.doi, CHI_GUI_LOI_NHO);
  assert.match(cauXem(goi), new RegExp(`đúng ${goi.luot.length} tin`));
});

test("«Chỉ gửi lời nhờ» là một chạm, và sau chạm đó không lượt nào đi", async () => {
  const goi = gomBoiCanhChat({ tin: [tin(2, "Q1 nha"), tin(1, "Ăn lẩu đi")], personId: toi });
  assert.equal(goiSeGui(goi, true), goi);
  assert.equal(goiSeGui(goi, false), undefined);
  const chu = chuChip(goi, false, true);
  assert.equal(chu.cau, "Chỉ gửi lời nhờ, không kèm tin nào");
  assert.equal(chu.xem, false);
  assert.equal(chu.doi, "Kèm lại 2 tin");

  const original = globalThis.fetch;
  const calls = [];
  datTokenPhien("synthetic-test-token");
  globalThis.fetch = async (url, options) => { calls.push({ url, ...options }); return { ok: true, status: 202, json: async () => ({ id: "job" }), text: async () => "{}" }; };
  try {
    const trigger = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
    await goiAi(phong, toi, "đi đâu", toi, goiSeGui(goi, false), "plan", trigger);
    await goiAi(phong, toi, "đi đâu", toi, goiSeGui(goi, true), "plan", trigger);
    const chiLoiNho = JSON.parse(calls[0].body);
    assert.deepEqual(Object.keys(chiLoiNho).sort(), ["command", "logical_id", "prompt", "trigger_message_id"]);
    assert.equal(chiLoiNho.trigger_message_id, trigger);
    assert.equal(JSON.parse(calls[1].body).boi_canh.luot.length, 2);
    // A server that does not declare `mention` gets no trigger key at all.
    await goiAi(phong, toi, "đi đâu", toi, undefined, "plan", undefined);
    assert.equal("trigger_message_id" in JSON.parse(calls[2].body), false);
  } finally { globalThis.fetch = original; datTokenPhien(null); }
});

test("chip nói thật khi AI chưa sẵn sàng, khi nhóm chưa có tin, và khi máy chủ không nhận gói", () => {
  const goi = gomBoiCanhChat({ tin: [tin(1, "chào")], personId: toi });
  assert.deepEqual(chuChip(goi, true, false), { cau: "Rủ Đi AI chưa sẵn sàng · Gửi như tin thường", xem: false, doi: null });
  assert.deepEqual(chuChip(gomBoiCanhChat({ tin: [], personId: toi }), true, true), { cau: "Nhóm chưa có tin nào, chỉ gửi lời nhờ", xem: false, doi: null });
  assert.deepEqual(chuChip(null, true, true), { cau: "Chỉ gửi lời nhờ, không kèm tin nào", xem: false, doi: null });
  assert.equal(goiSeGui(null, true), undefined);
});

test("màn đưa cho chip đúng gói sẽ gửi, và «Xem» liệt kê đúng gói đó", () => {
  const chip = readFileSync(join(SRC, "screens", "chat", "ChipBoiCanh.tsx"), "utf8");
  assert.match(chip, /chuChip\(goi, kemTin, sanSang\)/, "câu của chip phải đọc từ gói");
  assert.match(chip, /goi\.luot\.map\(/, "«Xem» phải liệt kê đúng các lượt của gói");
  const live = readFileSync(join(SRC, "screens", "chat", "GroupChatLive.tsx"), "utf8");
  assert.match(live, /const goiChip = [^;]*boiCanhAi/, "gói trên chip phải là gói gomBoiCanhChat dựng");
  assert.match(live, /<ChipBoiCanh goi=\{goiChip\}/);
  // The bundle frozen at the press is the one the chip showed.
  assert.match(live, /goi: goiSeGui\(goiChip, kemTin\)/);
});
