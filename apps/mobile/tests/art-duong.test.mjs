/* The art layer's paths, parsed the way react-native-svg's Java `PathParser`
 * parses them: explicit commands, fixed arity, plain decimals.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/art-duong.test.mjs
 *
 * Why a node test and not the emulator: a malformed `d` throws at mount inside
 * Fabric and kills the app on its first frame; the board then goes red on every
 * flow at once and says nothing about which shape did it (2026-09-05, a tape
 * edge ending in `L Z`). tsc sees a string; the web export draws a truncated
 * path and carries on. So every pose of Nếp, every motif, every taste glyph and
 * every empty scene is run through this parser, in both readings and under a
 * placement transform, before any screen may use it.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { MAU_VE, bienDoi, cungTron, giot, khungBo, qCong, tron, vien } from "../dist-test/rudi/art/net.js";
import { KHUNG_NEP, POSE_NEP, hinhGhe, hinhNep, laPoseNep } from "../dist-test/rudi/art/nep.js";
import { duongChuyen, gocGap, vongHo } from "../dist-test/rudi/art/motif.js";
import { GU_IDS, KHUNG_GU, hinhGu, laGuId } from "../dist-test/rudi/art/gu.js";
import { CANH_IDS, KHUNG_CANH, hinhCanh, laCanhId, moTaCanh } from "../dist-test/rudi/art/canh.js";

const ARITY = { M: 2, L: 2, C: 6, Z: 0 };
const SO = /^-?\d+(\.\d+)?$/;

/** Parse like the Java side: throw on anything it would refuse. */
function phanTich(d) {
  const tokens = d.trim().split(/[\s,]+/);
  const cmds = [];
  let i = 0;
  while (i < tokens.length) {
    const c = tokens[i++];
    assert.ok(c in ARITY, `lệnh lạ «${c}» trong: ${d.slice(0, 60)}`);
    const args = [];
    for (let k = 0; k < ARITY[c]; k++) {
      const t = tokens[i++];
      assert.ok(t !== undefined && SO.test(t), `lệnh ${c} thiếu/sai số «${t}» trong: ${d.slice(0, 60)}`);
      args.push(Number(t));
    }
    cmds.push({ c, args });
  }
  return cmds;
}

/** Every layer of a drawing: grammar, closure, box, colour role, stroke width. */
function kiemLop(ten, lop, w, h, biên = 1) {
  assert.ok(Array.isArray(lop) && lop.length > 0, `${ten}: không có lớp nào`);
  for (const [i, l] of lop.entries()) {
    const nhan = `${ten} lớp ${i}`;
    const cmds = phanTich(l.d);
    assert.equal(cmds[0].c, "M", `${nhan}: phải mở bằng M`);
    assert.ok(!/L\s+Z/.test(l.d) && !/(^|\s)-0(?=\s|$)/.test(l.d) && !/e/.test(l.d), `${nhan}: ${l.d.slice(0, 40)}`);
    if (l.net === undefined) {
      assert.equal(cmds.at(-1).c, "Z", `${nhan}: một lớp tô phải kín`);
      assert.notEqual(cmds.at(-2).c, "M", `${nhan}: Z phải đứng sau một điểm thật`);
    } else {
      assert.ok(Number.isFinite(l.net) && l.net > 0, `${nhan}: nét ${l.net}`);
      assert.ok(cmds.length >= 2, `${nhan}: một nét phải có ít nhất một đoạn`);
    }
    assert.ok(MAU_VE.includes(l.mau), `${nhan}: vai màu lạ ${l.mau}`);
    for (const { args } of cmds) {
      for (let k = 0; k < args.length; k += 2) {
        assert.ok(args[k] >= -biên && args[k] <= w + biên, `${nhan}: x ngoài hộp ${args[k]}`);
        assert.ok(args[k + 1] >= -biên && args[k + 1] <= h + biên, `${nhan}: y ngoài hộp ${args[k + 1]}`);
      }
    }
  }
}

test("bộ đọc còn sống: nó từ chối Q, lệnh cụt, số mũ và -0", () => {
  assert.throws(() => phanTich("M 1 2 Q 3 4 5 6"));
  assert.throws(() => phanTich("M 1 2 L Z"));
  assert.throws(() => phanTich("M 1e2 2 L 3 4"));
  assert.doesNotThrow(() => phanTich("M 1 2 L 3 4 C 1 2 3 4 5 6 Z"));
  assert.ok(/(^|\s)-0(?=\s|$)/.test("M -0 3"), "mẫu -0 phải bị bắt");
});

test("net.ts: từng builder ra đường hợp lệ và số thập phân thường", () => {
  for (const d of [tron(10, 10, 5), khungBo(2, 2, 20, 10, 4), vien([2, 2], [30, 14], 4), giot(20, 20, 3), qCong([0, 0], [10, 20], [20, 0]), cungTron(24, 24, 10, 0, Math.PI * 1.8)]) {
    const cmds = phanTich(d);
    assert.equal(cmds[0].c, "M");
    assert.ok(!/e/.test(d) && !/(^|\s)-0(?=\s|$)/.test(d), d.slice(0, 40));
  }
  // A full-circle arc must split into quarter turns, never one 360° cubic.
  assert.ok(phanTich(cungTron(0, 0, 10, 0, 2 * Math.PI)).filter((c) => c.c === "C").length >= 4);
  // The transform is an offset then a scale, in that order.
  assert.deepEqual(bienDoi(10, 20, 0.5)(4, 6), [12, 23]);
});

test("Nếp: sáu pose, hai cách đọc, có và không có phép đặt, đều qua ngữ pháp Java và nằm trong khung", () => {
  for (const pose of POSE_NEP) {
    for (const chiTiet of [true, false]) {
      kiemLop(`nep ${pose} chiTiet=${chiTiet}`, hinhNep(pose, { chiTiet }), KHUNG_NEP, KHUNG_NEP);
      const dat = hinhNep(pose, { chiTiet, x0: 10, y0: 5, tiLe: 0.8 });
      kiemLop(`nep ${pose} đặt`, dat, 10 + KHUNG_NEP * 0.8, 5 + KHUNG_NEP * 0.8);
    }
    // The 48dp reading is a smaller drawing, not the same one: fewer layers.
    assert.ok(hinhNep(pose, { chiTiet: false }).length < hinhNep(pose).length, pose);
    // Same call, same string: nothing here rolls a die per render.
    assert.deepEqual(hinhNep(pose), hinhNep(pose));
  }
  assert.equal(laPoseNep("moi"), true);
  assert.equal(laPoseNep("MOI"), false);
  assert.deepEqual(hinhNep("khong-co-pose-nay"), hinhNep("moi"));
  kiemLop("ghế", hinhGhe(0, 0), 44, 64);
  kiemLop("ghế lật", hinhGhe(0, 0, { lat: true }), 44, 64);
});

test("motif: vòng hở là một nét, có khoảng hở thật; đường chuyền và góc gấp hợp lệ", () => {
  const vong = vongHo(48, 48, 40);
  kiemLop("vòng hở", vong, 96, 96);
  assert.equal(vong.length, 1);
  const cmds = phanTich(vong[0].d);
  const dau = cmds[0].args, cuoi = cmds.at(-1).args.slice(-2);
  assert.ok(Math.hypot(dau[0] - cuoi[0], dau[1] - cuoi[1]) > 20, "hai đầu nét phải cách nhau: không phải vòng kín");
  kiemLop("đường chuyền", duongChuyen(4, 4, 88, 60), 96, 72);
  kiemLop("góc gấp", gocGap(4, 4, 60, 44), 68, 52);
});

test("gu: tám glyph trong lưới 48, mỗi glyph có nét mực và tối đa một mảng coral", () => {
  for (const id of GU_IDS) {
    const lop = hinhGu(id);
    kiemLop(`gu ${id}`, lop, KHUNG_GU, KHUNG_GU);
    assert.ok(lop.some((l) => l.mau === "muc" && l.net !== undefined), `${id}: cần nét mực`);
    assert.ok(lop.filter((l) => l.mau === "gap").length <= 1, `${id}: tối đa một chi tiết coral`);
    assert.deepEqual(hinhGu(id), hinhGu(id));
  }
  assert.equal(laGuId("mon-local"), true);
  assert.equal(laGuId("burger"), false);
  const la = hinhGu("khong-co-trong-tu-vung");
  kiemLop("gu lạ", la, KHUNG_GU, KHUNG_GU);
  assert.deepEqual(hinhGu(""), la);
});

test("cảnh: năm cảnh hợp lệ có và không có Nếp; cảnh không Nếp là phần đầu của cảnh có Nếp", () => {
  for (const id of CANH_IDS) {
    const co = hinhCanh(id);
    const khong = hinhCanh(id, { nep: false });
    kiemLop(`cảnh ${id} có Nếp`, co, KHUNG_CANH.w, KHUNG_CANH.h);
    kiemLop(`cảnh ${id} không Nếp`, khong, KHUNG_CANH.w, KHUNG_CANH.h);
    assert.ok(khong.length >= 3 && khong.length < co.length, `${id}: cảnh phải đứng được một mình`);
    assert.deepEqual(co.slice(0, khong.length), khong, `${id}: Nếp là lớp trên cùng, không xáo trộn cảnh nền`);
    assert.match(moTaCanh(id), /^[^—]{8,80}$/, `${id}: mô tả ngắn, không gạch dài`);
  }
  assert.equal(laCanhId("chua-co-anh"), true);
  assert.deepEqual(hinhCanh("khong-co"), hinhCanh("chua-co-hoi"));
  assert.equal(moTaCanh("khong-co"), moTaCanh("chua-co-hoi"));
});
