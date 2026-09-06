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

test("mọi sticker có hình, mọi đường qua được ngữ pháp Java, mọi điểm nằm trong khung", () => {
  for (const id of STICKER_IDS) {
    const hinh = hinhSticker(id);
    assert.equal(hinh.id, id);
    assert.ok(hinh.lop.length >= 2, `${id}: cần ít nhất hai lớp`);
    for (const lop of hinh.lop) {
      const cmds = phanTich(lop.d);
      assert.equal(cmds[0].c, "M", `${id}: đường phải mở bằng M`);
      assert.equal(cmds.at(-1).c, "Z", `${id}: đường phải kín`);
      assert.ok(!/L\s+Z/.test(lop.d) && !/-0\b/.test(lop.d) && !/e/.test(lop.d), `${id}: ${lop.d.slice(0, 40)}`);
      for (const { args } of cmds) {
        for (let k = 0; k < args.length; k += 2) {
          assert.ok(args[k] >= -1 && args[k] <= KHUNG_STICKER + 1, `${id}: x ngoài khung ${args[k]}`);
          assert.ok(args[k + 1] >= -1 && args[k + 1] <= KHUNG_STICKER + 1, `${id}: y ngoài khung ${args[k + 1]}`);
        }
      }
      assert.ok(["accent", "ink", "split", "card", "coral"].includes(lop.mau), `${id}: màu lạ ${lop.mau}`);
    }
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
