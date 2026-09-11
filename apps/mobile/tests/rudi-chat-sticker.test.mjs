/* Stickers (L1, ADR-0021 §2.1): the vocabulary and every shape in it.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-chat-sticker.test.mjs
 *
 * Two claims only node can settle: every `d` parses the way react-native-svg's
 * Java `PathParser` parses it (one stray token kills the app on its first
 * frame, and tsc and the web export are both blind to it), and an id this
 * build does not know draws the «khac» tile rather than throwing or drawing
 * nothing. The id list is also compared to `packages/shared/stickers.json`
 * here; the server copy is compared at the repo root.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  KHUNG_STICKER,
  STICKER_IDS,
  hinhSticker,
  laStickerHopLe,
  nhanSticker,
} from "../dist-test/rudi/chat/sticker.js";

const ARITY = { M: 2, L: 2, C: 6, Z: 0 };
const SO = /^-?\d+(\.\d+)?$/;

/** Parse like the Java side: throw on anything it would refuse. */
function phanTich(d) {
  const tokens = d.trim().split(/[\s,]+/);
  const cmds = [];
  let i = 0;
  while (i < tokens.length) {
    const c = tokens[i++];
    assert.ok(c in ARITY, `lệnh lạ «${c}» trong: ${d}`);
    const args = [];
    for (let k = 0; k < ARITY[c]; k++) {
      const t = tokens[i++];
      assert.ok(t !== undefined && SO.test(t), `lệnh ${c} thiếu/sai số «${t}» trong: ${d}`);
      args.push(Number(t));
    }
    cmds.push({ c, args });
  }
  return cmds;
}

const shared = JSON.parse(readFileSync(new URL("../../../packages/shared/stickers.json", import.meta.url), "utf8"));

test("danh sách id khớp packages/shared/stickers.json, đúng thứ tự", () => {
  assert.deepEqual([...STICKER_IDS], shared.stickers.map((s) => s.id));
  for (const s of shared.stickers) assert.equal(nhanSticker(s.id), s.label);
});

test("mọi sticker có hình ở cả hai cỡ đọc, mọi đường qua được ngữ pháp Java, mọi điểm nằm trong khung", () => {
  for (const id of STICKER_IDS) {
    for (const chiTiet of [true, false]) {
      const hinh = hinhSticker(id, { chiTiet });
      assert.equal(hinh.id, id);
      assert.ok(hinh.lop.length >= 2, `${id}: cần ít nhất hai lớp`);
      for (const lop of hinh.lop) {
        const cmds = phanTich(lop.d);
        assert.equal(cmds[0].c, "M", `${id}: đường phải mở bằng M`);
        // A filled layer is a closed shape; a stroke (the art layer's pen) may
        // stay open, but it is a line, so it has at least two points.
        if (lop.net === undefined) assert.equal(cmds.at(-1).c, "Z", `${id}: lớp tô phải kín`);
        else {
          assert.ok(lop.net > 0 && Number.isFinite(lop.net), `${id}: nét phải dương`);
          assert.ok(cmds.length >= 2, `${id}: nét phải có ít nhất hai điểm`);
        }
        assert.ok(!/L\s+Z/.test(lop.d) && !/-0(?![.\d])/.test(lop.d) && !/e/.test(lop.d), `${id}: ${lop.d.slice(0, 40)}`);
        for (const { args } of cmds) {
          for (let k = 0; k < args.length; k += 2) {
            assert.ok(args[k] >= -1 && args[k] <= KHUNG_STICKER + 1, `${id}: x ngoài khung ${args[k]}`);
            assert.ok(args[k + 1] >= -1 && args[k + 1] <= KHUNG_STICKER + 1, `${id}: y ngoài khung ${args[k + 1]}`);
          }
        }
        assert.ok(["accent", "ink", "split", "card", "coral", "line"].includes(lop.mau), `${id}: màu lạ ${lop.mau}`);
      }
    }
  }
  // Since 09/09 all eight are drawn in the Nếp language, so EVERY id has a
  // second, simpler drawing for the tray rather than the 120dp one shrunk.
  // The loop, not a named id, is the point: the day someone adds a ninth
  // shape without a compact reading, this is what says so.
  for (const id of STICKER_IDS) {
    assert.notDeepEqual(
      hinhSticker(id, { chiTiet: false }).lop,
      hinhSticker(id).lop,
      `${id}: bản khay phải là hình vẽ thứ hai, không phải bản 120 thu lại`,
    );
  }
});

test("id lạ vẽ ô «khac» có dấu hỏi, nhãn «Sticker», không ném", () => {
  assert.equal(laStickerHopLe("khong-co-trong-bo"), false);
  assert.equal(laStickerHopLe("DI-THOI"), false);
  assert.equal(nhanSticker("khong-co-trong-bo"), "Sticker");
  const khac = hinhSticker("khong-co-trong-bo");
  assert.equal(khac.nhan, "Sticker");
  assert.ok(khac.lop.length >= 3);
  for (const lop of khac.lop) phanTich(lop.d);
  // The same tile for an empty id and for garbage: nothing here depends on the input.
  assert.deepEqual(hinhSticker("").lop, khac.lop);
});

test("nhãn không có gạch dài và id là slug ASCII không có dãy số dài", () => {
  for (const id of STICKER_IDS) {
    assert.match(id, /^[a-z0-9-]{1,32}$/);
    assert.ok(!/\d{4,}/.test(id), id);
    assert.ok(!nhanSticker(id).includes("—"), id);
  }
});

// Tái audit 10/09 (F45 còn mở): hai hình phải mang dấu hiệu đọc được ở khay 64.
// Đây là pin cơ chế — «có bánh», «có ô bầu dục» — không phải chứng minh người xem
// hiểu nghĩa; việc ấy là bảng không nhãn với người chưa đọc brief.
const tronKin = (lop) => lop.filter((l) => l.net === undefined && (l.d.match(/ C /g) ?? []).length === 4 && /Z$/.test(l.d));
test("ket-xe: vật chặn là phương tiện — ít nhất bốn hình tròn tô (hai bánh xe của Nếp, hai bánh của xe phía trước)", () => {
  for (const chiTiet of [true, false]) assert.ok(tronKin(hinhSticker("ket-xe", { chiTiet }).lop).length >= 4, `chiTiet=${chiTiet}`);
});
test("tra-tien-ne: tờ trên có ô bầu dục tô màu giấy — dấu tờ bạc, không ký hiệu tiền", () => {
  for (const chiTiet of [true, false]) {
    const lop = hinhSticker("tra-tien-ne", { chiTiet }).lop;
    assert.ok(tronKin(lop).some((l) => l.mau === "line"), `chiTiet=${chiTiet}: thiếu ô bầu dục`);
    assert.ok(!lop.some((l) => l.mau === "split"), "sticker tiền không dùng màu teal của sổ");
  }
});
