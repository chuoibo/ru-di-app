/* Chip xem trước trên nút gửi (ADR-0046 §2.3, đề xuất; giữ ADR-0036 §2.5).
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

import { CHI_GUI_LOI_NHO, cauXem, chuChip, goiSeGui, gomTheoNguoi } from "../dist-test/rudi/chat/chip-boi-canh.js";
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
  // 60 long messages on screen, and a caller asking for the ceiling: the
  // bundle keeps at most 40 turns and then drops whole turns from the old end
  // until it fits the byte ceiling. Three-byte letters at the per-turn cap: 40
  // of them are well over the bundle's byte ceiling.
  const tinHien = Array.from({ length: 60 }, (_, i) => tin(60 - i, "ờ".repeat(400)));
  const goi = gomBoiCanhChat({ tin: tinHien, personId: toi, soLuot: 40 });
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
  // The line fits 320dp; the spoken name keeps the whole sentence (QA UI-167).
  assert.deepEqual(chuChip(goi, true, false), { cau: "AI chưa sẵn sàng · gửi như tin thường", nhanDoc: "Rủ Đi AI chưa sẵn sàng · Gửi như tin thường", xem: false, doi: null });
  assert.deepEqual(chuChip(gomBoiCanhChat({ tin: [], personId: toi }), true, true), { cau: "Nhóm chưa có tin nào, chỉ gửi lời nhờ", xem: false, doi: null });
  assert.deepEqual(chuChip(null, true, true), { cau: "Chỉ gửi lời nhờ, không kèm tin nào", xem: false, doi: null });
  assert.equal(goiSeGui(null, true), undefined);
});

test("màn đưa cho chip đúng gói sẽ gửi, và «Xem» liệt kê đúng gói đó", () => {
  const chip = readFileSync(join(SRC, "screens", "chat", "ChipBoiCanh.tsx"), "utf8");
  // The pair flag only picks words for two people; the count still comes
  // from the bundle (design 2026-09-28).
  assert.match(chip, /chuChip\(goi, kemTin, sanSang(, haiNguoi)?\)/, "câu của chip phải đọc từ gói");
  assert.match(chip, /gomTheoNguoi\(goi\.luot\)/, "«Xem» phải liệt kê đúng các lượt của gói");
  assert.match(chip, /accessibilityLabel=\{chu\.nhanDoc\}/, "câu ngắn trên mắt, câu đủ cho tai");
  const live = readFileSync(join(SRC, "screens", "chat", "GroupChatLive.tsx"), "utf8");
  assert.match(live, /const goiChip = [^;]*boiCanhAi/, "gói trên chip phải là gói gomBoiCanhChat dựng");
  assert.match(live, /<ChipBoiCanh goi=\{goiChip\}/);
  // The bundle frozen at the press is the one the chip showed.
  assert.match(live, /goi: goiSeGui\(goiChip, kemTin\)/);
});

// Bug 2026-09-28 (lab screenshot 08): the «Xem» sheet was a child of the chip,
// and a Sheet fills its nearest parent, so it filled the 356x40 chip instead
// of the screen. The sheet is now its own component the screen mounts at its
// root, beside KhaySticker and MenuTin. No render harness here, so this reads
// the source: the chip draws no Sheet, and both hosts mount the sheet at the
// same depth as KhaySticker, never inside the chip's wrapper.
test("tấm «Xem» không nằm trong chip: màn gắn nó ở gốc, cạnh các tấm khác", () => {
  const chip = readFileSync(join(SRC, "screens", "chat", "ChipBoiCanh.tsx"), "utf8");
  const thanChip = chip.slice(chip.indexOf("export function ChipBoiCanh("), chip.indexOf("export function TamXemBoiCanh("));
  assert.ok(thanChip.length > 0, "không tìm thấy thân ChipBoiCanh");
  assert.doesNotMatch(thanChip, /<Sheet\b/, "chip không được tự vẽ Sheet");
  assert.match(thanChip, /onPress=\{onXem\}/, "«Xem» của chip chỉ báo cho màn");
  assert.match(chip.slice(chip.indexOf("export function TamXemBoiCanh(")), /<Sheet accessibilityLabel="Những tin sẽ gửi kèm lời nhờ"/);

  const thut = (d) => d.length - d.trimStart().length;
  const live = readFileSync(join(SRC, "screens", "chat", "GroupChatLive.tsx"), "utf8");
  assert.match(live, /<ChipBoiCanh [^\n]*onXem=\{\(\) => setXemBoiCanh\(true\)\}/);
  const dong = live.split("\n");
  const tam = dong.findIndex((d) => d.includes("<TamXemBoiCanh "));
  const sticker = dong.findIndex((d) => d.includes("<KhaySticker "));
  assert.ok(tam > 0 && sticker > 0, "thiếu TamXemBoiCanh hoặc KhaySticker");
  assert.equal(thut(dong[tam]), thut(dong[sticker]), "TamXemBoiCanh phải cùng tầng với KhaySticker (gốc màn)");
  assert.match(dong[tam], /goi=\{goiChip\}/, "tấm phải liệt kê đúng gói chip đếm");

});

test("UI-163: mặc định kèm 20 tin như ADR-0046 §2, và không ai nới được quá trần 40", () => {
  const tinHien = Array.from({ length: 60 }, (_, i) => tin(60 - i, "ừ"));
  assert.equal(gomBoiCanhChat({ tin: tinHien, personId: toi }).luot.length, 20, "mặc định là 20, không phải trần");
  assert.equal(chuChip(gomBoiCanhChat({ tin: tinHien, personId: toi }), true, true).cau, "Kèm 20 tin gần đây");
  assert.equal(gomBoiCanhChat({ tin: tinHien, personId: toi, soLuot: 60 }).luot.length, 40, "số xin thêm vẫn bị giữ ở trần máy chủ nhận");
  assert.equal(gomBoiCanhChat({ tin: tinHien.slice(0, 7), personId: toi }).luot.length, 7, "ít tin hơn thì kèm đúng số đang có");
});

// The «Xem» sheet as a transcript: one name over a person's consecutive turns,
// nothing merged away -- the sheet still lists exactly the bundle's turns.
test("«Xem» gom lượt liền nhau của một người, giữ đủ và đúng thứ tự", () => {
  const tinHien = [tin(5, "e"), tin(3, "d"), tin(2, "b"), tin(1, "a")];
  const goi = gomBoiCanhChat({ tin: tinHien, personId: toi });
  const doan = gomTheoNguoi(goi.luot);
  assert.deepEqual(doan.flatMap((d) => d.luot.map((l) => l.id)), goi.luot.map((l) => l.id), "đủ lượt, đúng thứ tự");
  for (let i = 1; i < doan.length; i += 1) assert.notEqual(doan[i].nguoi, doan[i - 1].nguoi, "hai đoạn liền nhau là hai người");
  assert.ok(doan.some((d) => d.luot.length > 1), "lượt liền nhau của một người chung một đoạn");
  assert.ok(doan.every((d) => d.cuaToi === d.luot.every((l) => l.vai === "toi")));
});
