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

import { MAU_VE, bienDoi, cungTron, daGiac, giot, khungBo, netGay, qCong, tron, vien } from "../dist-test/rudi/art/net.js";
import { BIEU_CAM, GAP_NEP, KHUNG_NEP, POSE_NEP, hinhGhe, hinhNep, laPoseNep } from "../dist-test/rudi/art/nep.js";
import { STICKER_IDS, hinhSticker } from "../dist-test/rudi/chat/sticker.js";
import { duongChuyen, gocGap, thuGapBa, vongHo } from "../dist-test/rudi/art/motif.js";
import { GU_IDS, KHUNG_GU, hinhGu, laGuId } from "../dist-test/rudi/art/gu.js";
import { CANH_IDS, CANH_KHONG_NEP, KHUNG_CANH, POSE_CANH, hinhCanh, hopNgang, laCanhId, moTaCanh } from "../dist-test/rudi/art/canh.js";
import { kiemLop, phanTich } from "./_kiem-lop.mjs";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

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

test("Nếp: mọi pose trong POSE_NEP, hai cách đọc, có và không có phép đặt, đều qua ngữ pháp Java và nằm trong khung", () => {
  for (const pose of POSE_NEP) {
    for (const chiTiet of [true, false]) {
      for (const gap of GAP_NEP) {
        kiemLop(`nep ${pose} chiTiet=${chiTiet} gap=${gap}`, hinhNep(pose, { chiTiet, gap }), KHUNG_NEP, KHUNG_NEP);
        const dat = hinhNep(pose, { chiTiet, gap, x0: 10, y0: 5, tiLe: 0.8 });
        kiemLop(`nep ${pose} đặt gap=${gap}`, dat, 10 + KHUNG_NEP * 0.8, 5 + KHUNG_NEP * 0.8);
      }
    }
    // The pocket-folded sheet is a smaller drawing at 48 too, and deterministic.
    assert.ok(hinhNep(pose, { chiTiet: false, gap: "manh" }).length < hinhNep(pose, { gap: "manh" }).length, `${pose} manh`);
    assert.deepEqual(hinhNep(pose, { gap: "manh" }), hinhNep(pose, { gap: "manh" }));
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
  // The letter folded in thirds: a sheet with a cut corner, two creases at
  // h/3 and 2h/3, no coral. The cut corner is what says «paper»: a rounded
  // rectangle with two lines read as a lined index card (finish review 12/09).
  const thu = thuGapBa(4, 4, 60, 44);
  kiemLop("thư gấp ba", thu, 68, 52);
  assert.equal(thu.filter((l) => l.mau === "gap").length, 0, "thư gấp ba không mang coral: coral thuộc tờ dẫn, do màn hình đặt");
  const toThu = phanTich(thu.find((l) => l.mau === "giay").d).filter((x) => x.c !== "Z");
  assert.equal(toThu.length, 5, "tờ thư có góc cắt: năm đỉnh, không phải khung bo bốn góc");
  assert.equal(thu.filter((l) => l.mau === "bong" && l.net === undefined).length, 1, "một mảng gấp `bong` ở góc, không coral");
  const vet = thu.filter((l) => l.mau === "bong" && l.net !== undefined);
  assert.equal(vet.length, 2, "đúng hai vết gấp");
  const yVet = vet.map((l) => phanTich(l.d)[0].args[1]).sort((a, b) => a - b);
  assert.ok(Math.abs(yVet[0] - (4 + 44 / 3)) < 0.05 && Math.abs(yVet[1] - (4 + (2 * 44) / 3)) < 0.05, "vết gấp ở một phần ba và hai phần ba");
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

test("mọi cảnh hợp lệ có và không có Nếp; cảnh không Nếp là phần đầu của cảnh có Nếp", () => {
  for (const id of CANH_IDS) {
    const co = hinhCanh(id);
    const khong = hinhCanh(id, { nep: false });
    kiemLop(`cảnh ${id} có Nếp`, co, KHUNG_CANH.w, KHUNG_CANH.h);
    kiemLop(`cảnh ${id} không Nếp`, khong, KHUNG_CANH.w, KHUNG_CANH.h);
    assert.ok(khong.length >= 3, `${id}: cảnh phải đứng được một mình`);
    if (CANH_KHONG_NEP.has(id)) {
      // A failure or a money screen never gets the character, and asking for
      // it must not change the drawing: the rule lives in `hinhCanh`, not in
      // twenty callers remembering to pass `nep={false}` (DESIGN.md, «Nếp
      // đứng xa tiền»).
      assert.deepEqual(co, khong, `${id}: cảnh này không bao giờ có Nếp, kể cả khi được yêu cầu`);
    } else {
      assert.ok(khong.length < co.length, `${id}: phải có lớp Nếp thêm vào`);
      assert.deepEqual(co.slice(0, khong.length), khong, `${id}: Nếp là lớp trên cùng, không xáo trộn cảnh nền`);
    }
    assert.match(moTaCanh(id), /^[^—]{8,80}$/, `${id}: mô tả ngắn, không gạch dài`);
  }
  // Named, not merely non-empty: emptying the set of the FAILURE scene and
  // adding some decorative id would keep the branch above green while «Nếp
  // đứng xa tiền» silently lapsed on about twenty error and ledger screens.
  assert.ok(CANH_KHONG_NEP.has("chua-doc-duoc"), "cảnh lỗi phải nằm trong tập không-Nếp");
  assert.equal(laCanhId("chua-co-anh"), true);
  assert.deepEqual(hinhCanh("khong-co"), hinhCanh("chua-co-hoi"));
  assert.equal(moTaCanh("khong-co"), moTaCanh("chua-co-hoi"));
});

// Audit native 09/09, F45a. Two scenes drawn with the same body beside a
// different prop is the one thing the brief forbade, and `chua-co-keo` /
// `chua-co-tin-nhan` were both `ghi-lai`. The pose a scene stands in is data
// (`POSE_CANH`, read from the same entry `hinhCanh` draws from), so this can be
// checked instead of remembered.
test("mười cảnh: mỗi cảnh có Nếp đứng bằng một pose riêng, cảnh lỗi không có pose", () => {
  assert.deepEqual(Object.keys(POSE_CANH).sort(), [...CANH_IDS].sort(), "POSE_CANH phải nói về đúng mười cảnh");
  for (const id of CANH_KHONG_NEP) assert.equal(POSE_CANH[id], null, `${id}: cảnh không Nếp thì không có pose`);
  const dung = Object.entries(POSE_CANH).filter(([id]) => !CANH_KHONG_NEP.has(id));
  for (const [id, pose] of dung) assert.ok(pose !== null && laPoseNep(pose), `${id}: pose «${pose}» không có trong POSE_NEP`);
  const theoPose = new Map();
  for (const [id, pose] of dung) theoPose.set(pose, [...(theoPose.get(pose) ?? []), id]);
  const lap = [...theoPose.entries()].filter(([, ids]) => ids.length > 1);
  assert.deepEqual(lap, [], `cùng một dáng chỉ đổi đạo cụ: ${JSON.stringify(lap)}`);
});

// Audit native 09/09, F45d. The open ring under the torn sheet read as a
// loading spinner standing still; the coral in the failure scene has to sit ON
// the tear -- a jagged line -- and there must be no coral arc anywhere in it.
// Tái audit 10/09, R4 mở rộng cùng luật sang `bo-loc-che-het`: vòng hở coral trên
// lưới lọc cũng đọc như spinner. Ở hai cảnh này coral chỉ được là nét thẳng (vết
// rách; khung ô hở), không có cung tròn coral nào.
for (const id of ["chua-doc-duoc", "bo-loc-che-het"]) {
  test(`${id}: coral là nét thẳng, không có cung tròn coral nào`, () => {
    const lop = hinhCanh(id);
    const cung = lop.filter((l) => l.mau === "gap" && l.net !== undefined && /C/.test(l.d) && !/L/.test(l.d));
    assert.deepEqual(cung, [], `${id} còn một cung tròn coral (vòng hở đọc như spinner)`);
    assert.ok(lop.some((l) => l.mau === "gap" && l.net !== undefined && /L/.test(l.d)), `${id}: coral phải là nét thẳng`);
  });
}

// Tái audit 10/09 (F45a còn mở): đuôi bong bóng phải NHÌN THẤY — tờ giấy Nếp
// (`goi-loi` x0 -4, tiLe 0.78) chiếm x 16…52, nên không đỉnh nào của bong bóng
// được nằm trái x 52. Bản trước để đuôi chạy dưới tờ giấy tới miệng và bị che trọn.
test("chua-co-tin-nhan: cả bong bóng, kể cả đuôi, nằm bên phải tờ giấy Nếp", () => {
  const { x0 } = hopNgang(hinhCanh("chua-co-tin-nhan", { nep: false }));
  assert.ok(x0 >= 52, `bong bóng lấn dưới tờ giấy: x nhỏ nhất ${x0}`);
  // và đuôi thật sự trỏ về phía Nếp: đỉnh trái nhất ở độ cao miệng (56.6–61.6)
  const lop = hinhCanh("chua-co-tin-nhan", { nep: false });
  const diem = lop.flatMap((l) => [...l.d.matchAll(/([0-9.]+) ([0-9.]+)/g)].map((m) => [Number(m[1]), Number(m[2])]));
  const traiNhat = diem.reduce((a, b) => (b[0] < a[0] ? b : a));
  assert.ok(traiNhat[1] >= 56 && traiNhat[1] <= 62, `mũi đuôi ở y ${traiNhat[1]}, không ở độ cao miệng`);
});


/* ---------------------------------------------------------------------------
 * The folded coral corner is the identity mark. No ink may be painted on it.
 *
 * Until 09/09 five of the six expressions ended their right brow inside the
 * triangle, `nang-bong`'s raised hand planted a filled disc 2.8 units into it,
 * and a limb had done the same earlier and been re-routed by hand. Every one of
 * those was found by looking at a render, which is the kind of check that
 * lapses. The rule lives here now.
 *
 * Three things this gate has to get right, each of which it got WRONG in its
 * first draft (finish review, 09/09):
 *
 *  1. It measures PAINT, not centre lines. A 2.4-wide stroke whose centre is
 *     0.4 units inside puts 1.6 units of ink on the coral -- worse than the old
 *     brow this was written to stop -- and the first draft accepted it, because
 *     it compared the centre against a half-unit tolerance instead of adding
 *     the stroke's own half-width. The rule is `depth + net/2 <= 0`.
 *  2. It BOUNDS each curve instead of spot-checking it. Sampling a cubic at a
 *     few interior points is not a bound: a curve can bulge between samples.
 *     Sampling N+1 points leaves a chord error of at most max|B''|/(8N²), which
 *     is computable from the control points, so the bound is the worst sample
 *     PLUS that error. It is tight (order 1e-3 here) and it is sound.
 *     A convex hull of the control points is also sound but far too loose: the
 *     hull of a circle's cubics overshoots the circle by ~14% of its radius,
 *     which falsely accuses every hand and both eyes.
 *  3. It IDENTIFIES the fold instead of guessing it. «the first filled coral
 *     layer» is not an identity check -- `ghi-lai`'s pencil nib is also a
 *     three-point coral triangle, and ink legitimately overlaps it. The fold is
 *     the triangle S(50,20)·S(69,38)·S(50,38) for some lean, which is a shape
 *     test that survives any reordering.
 * ------------------------------------------------------------------------ */

/** Samples per cubic. The error term below is what makes this a bound. */
const MAU_CUBIC = 24;

/*
 * Two colour vocabularies reach this gate. `art/*.ts` names roles
 * `giay·bong·muc·gap`; `chat/sticker.ts` re-labels the same drawing through
 * `tuLopVe` into `card·line·ink·accent`. Looking only for «gap» found the fold
 * in the nine scenes and in NONE of the sixteen sticker readings, and the count
 * assertion below is what said so out loud instead of reporting a clean scan.
 */
const VAI_ART = { coral: "gap", muc: "muc" };
const VAI_STICKER = { coral: "accent", muc: "ink" };

/**
 * Points to test on one path, each with the error bound that applies to it.
 *
 * Straight commands are exact. A cubic contributes `MAU_CUBIC` samples plus the
 * chord error max|B''|/(8N²), computed from its own control points.
 */
function diemCua(d) {
  const ra = [];
  let cur = [0, 0];
  for (const { c, args } of phanTich(d)) {
    if (c === "Z") continue;
    if (c === "M" || c === "L") {
      cur = [args[0], args[1]];
      ra.push({ p: cur, saiSo: 0 });
      continue;
    }
    const p0 = cur;
    const c1 = [args[0], args[1]];
    const c2 = [args[2], args[3]];
    const p3 = [args[4], args[5]];
    const nhi = (a, b, e) => Math.hypot(a[0] - 2 * b[0] + e[0], a[1] - 2 * b[1] + e[1]);
    const saiSo = (6 * Math.max(nhi(p0, c1, c2), nhi(c1, c2, p3))) / (8 * MAU_CUBIC * MAU_CUBIC);
    for (let k = 0; k <= MAU_CUBIC; k++) {
      const t = k / MAU_CUBIC;
      const u = 1 - t;
      ra.push({
        saiSo,
        p: [
          u * u * u * p0[0] + 3 * u * u * t * c1[0] + 3 * u * t * t * c2[0] + t * t * t * p3[0],
          u * u * u * p0[1] + 3 * u * u * t * c1[1] + 3 * u * t * t * c2[1] + t * t * t * p3[1],
        ],
      });
    }
    cur = p3;
  }
  return ra;
}

/** Signed twice-area; the sign says which side of a→b the point p is on. */
const ben = (a, b, p) => (b[0] - a[0]) * (p[1] - a[1]) - (b[1] - a[1]) * (p[0] - a[0]);

/** How far inside the triangle p lies, in grid units. Negative means outside. */
function sauTrong(tam, p) {
  const [A, B, C] = tam;
  const huong = Math.sign(ben(A, B, C)) || 1;
  let it = Infinity;
  for (const [u, v] of [[A, B], [B, C], [C, A]]) {
    const canh = Math.hypot(v[0] - u[0], v[1] - u[1]);
    it = Math.min(it, (huong * ben(u, v, p)) / canh);
  }
  return it;
}

/** Coordinates are emitted rounded to two decimals, so shapes match to ~0.01. */
const LAM_TRON = 0.02;

/** The three points of a triangle path, or null if it is not a triangle. */
function tamGiac(d) {
  const diem = phanTich(d).filter((x) => x.c !== "Z").map((x) => [x.args[0], x.args[1]]);
  return diem.length === 3 ? diem : null;
}

/**
 * Is this triangle the fold, i.e. S(50,20)·S(69,38)·S(50,38)?
 *
 * It has to hold wherever the figure is PLACED, not just at the bare 96 grid:
 * scenes and stickers pass `x0`, `y0` and `tiLe`, so the literal 20 and 38 are
 * gone by the time a screen draws it. Checking the placed coordinates was the
 * first draft's bug -- it recognised nothing in any real composition and
 * reported «no fold here» for all 25 of them.
 *
 * What survives placement is the SHAPE. The shear moves x by `k·(91−y)/71` and
 * leaves y alone, so the fold is always: two vertices on one horizontal, one
 * above them, and a base-to-height ratio of exactly 19:18. `ghi-lai`'s pencil
 * nib (y = 21, 18, 25) fails the first clause; the ratio and the sane-lean
 * bound rule out an accidental match.
 */
function laNepGap(dinh) {
  const y = dinh.map((p) => p[1]);
  const tren = Math.min(...y);
  const duoi = Math.max(...y);
  const cao = duoi - tren;
  const dinhTren = dinh.filter((p) => Math.abs(p[1] - tren) < LAM_TRON);
  const dinhDuoi = dinh.filter((p) => Math.abs(p[1] - duoi) < LAM_TRON);
  if (dinhTren.length !== 1 || dinhDuoi.length !== 2) return false;
  const [trai, phai] = [...dinhDuoi].sort((a, b) => a[0] - b[0]);
  const day = phai[0] - trai[0];
  // Base : height is 19 : 18 at every scale. Tolerance scales with the drawing.
  if (Math.abs(day / cao - 19 / 18) > LAM_TRON / cao + 1e-9) return false;
  // And the lean the top vertex implies has to be one a body could have.
  const tiLe = cao / 18;
  const k = ((dinhTren[0][0] - trai[0]) * 71) / (18 * tiLe);
  return Number.isFinite(k) && Math.abs(k) <= 30;
}

/**
 * The rule `nep.ts` cites by name: no ink is painted inside the coral fold.
 *
 * Returns the number of points examined and the fold's vertices, or null when
 * the drawing has no fold at all (a scene with the figure turned off). Throws
 * on any violation.
 */
function khongCatNepGap(ten, lop, vai = VAI_ART, nhanDien = laNepGap) {
  let nepGap = null;
  for (const [i, l] of lop.entries()) {
    if (l.mau !== vai.coral || l.net !== undefined) continue;
    const t = tamGiac(l.d);
    if (t && nhanDien(t)) {
      nepGap = { dinh: t, d: l.d, i };
      break;
    }
  }
  if (nepGap === null) return null;
  // Two exemptions, both by identity rather than by position.
  //
  // Anything drawn BEFORE the fold is painted over by the fold's own coral, so
  // it cannot end up on the identity mark however close it passes. In a scene
  // that is most of the picture -- props are composed before the figure.
  //
  // And a stroke whose path is exactly a filled shape in the same drawing is
  // that shape's OUTLINE, not a mark upon it: the sheet's outline runs along
  // the fold's hypotenuse by construction, and the fold's own outline traces
  // the fold. Looking up «the first paper fill» instead was the first draft's
  // second bug -- in a scene it found the CHAIR's paper and then accused the
  // body outline of painting 0.96 units onto the coral.
  // Built from NON-ink fills only, and applied only to STROKES. Written as
  // «every fill» and applied to every layer, it silently exempted every ink
  // FILL -- both eyes, every hand -- because each one trivially matches itself.
  // The fill canary below is what caught that; the point floors did not.
  const dMang = new Set(
    lop.filter((l) => l.net === undefined && l.mau !== vai.muc).map((l) => l.d),
  );
  let daXet = 0;
  for (const [i, l] of lop.entries()) {
    if (i < nepGap.i) continue;
    if (l.mau !== vai.muc) continue;
    if (l.net !== undefined && dMang.has(l.d)) continue;
    const nua = (l.net ?? 0) / 2;
    for (const { p, saiSo } of diemCua(l.d)) {
      daXet += 1;
      const son = sauTrong(nepGap.dinh, p) + nua + saiSo;
      assert.ok(
        son <= 0,
        `${ten}: lớp mực ${i} sơn ${son.toFixed(2)} đơn vị lên nếp gấp tại ${p.map((n) => n.toFixed(1)).join(",")}`,
      );
    }
  }
  return { daXet, dinh: nepGap.dinh };
}

/*
 * `nghieng` and `dam` are public overrides on `TuyChonNep`, and a shipped
 * sticker uses both at once (`sticker.ts`, «Chờ tí»: nghieng 5, dam 1.16). A
 * scan over pose defaults alone would never have looked at the drawing that
 * actually ships. The contract is therefore the whole usable band: every
 * declared lean falls inside −8…13, and `dam` is scanned at both ends because
 * it scales stroke width and limb thickness linearly.
 */
const NGHIENG_QUET = Array.from({ length: 22 }, (_, i) => i - 8);
const DAM_QUET = [1, 1.3];

test("không nét mực nào sơn lên nếp gấp coral, qua mọi pose · biểu cảm · độ nghiêng · độ đậm", () => {
  let daXet = 0;
  let itNhat = Infinity;
  let banVe = 0;
  for (const pose of POSE_NEP) {
    for (const bieuCam of BIEU_CAM) {
      for (const chiTiet of [true, false]) {
        for (const nghieng of NGHIENG_QUET) {
          for (const dam of DAM_QUET) {
            const ten = `${pose}/${bieuCam}/${chiTiet ? "chi tiết" : "rút gọn"}/ng${nghieng}/dam${dam}`;
            const ra = khongCatNepGap(ten, hinhNep(pose, { chiTiet, bieuCam, nghieng, dam }));
            assert.ok(ra !== null, `${ten}: không nhận ra nếp gấp trong bản vẽ`);
            daXet += ra.daXet;
            itNhat = Math.min(itNhat, ra.daXet);
            banVe += 1;
          }
        }
      }
    }
  }
  assert.equal(banVe, POSE_NEP.length * BIEU_CAM.length * 2 * NGHIENG_QUET.length * DAM_QUET.length);
  // Floors read off the MEASURED count, not picked round: 8_577_360 ink points
  // over 9_504 drawings on 09/09, the thinnest single drawing giving 826. The
  // first version of this guard asserted «> 4000» against a true figure ten
  // times larger, and would have stayed green after the scan dropped nine
  // layers in ten. A floor whose denominator nobody re-derives is decoration.
  assert.ok(daXet > 7_000_000, `chỉ xét ${daXet} điểm mực trên toàn bộ — máy quét đã ngừng nhìn`);
  assert.ok(itNhat > 700, `có bản vẽ chỉ được xét ${itNhat} điểm mực`);
});

test("bound của cubic: sai số lấy từ chính điểm điều khiển, và đủ nhỏ để là biên", () => {
  // Straight commands are exact; a cubic carries max|B''|/(8N²), which is what
  // turns N samples into a bound instead of a spot check. Pinned here because
  // no drawing today passes close enough to the fold to exercise it, so
  // deleting the term would otherwise leave every test green.
  // Paths are built with the real builders, not written out as literals: a
  // literal like «M 0 0 C 0 10 …» is a long run of digits and the repo guard
  // reads those as possible account numbers. The guard is right to; the test
  // is what gives way.
  const thang = netGay([[0, 0], [10, 0]]);
  for (const x of diemCua(thang)) assert.equal(x.saiSo, 0, "đoạn thẳng không có sai số");
  // qCong lifts a quadratic to a cubic: c1 = p0 + ⅔(q−p0), c2 = p1 + ⅔(q−p1),
  // so with q at (5,15) both second differences come to (0,−10).
  const cong = diemCua(qCong([0, 0], [5, 15], [10, 0]));
  assert.equal(cong.length, 1 + (MAU_CUBIC + 1), "một điểm đầu cộng N+1 mẫu");
  const mau = cong[cong.length - 1];
  const cho = (6 * 10) / (8 * MAU_CUBIC * MAU_CUBIC);
  // Coordinates are emitted at two decimals, so the control points the parser
  // reads back differ from the exact ⅔ in the last digits.
  assert.ok(Math.abs(mau.saiSo - cho) < 1e-6, `sai số ${mau.saiSo} khác công thức ${cho}`);
  assert.ok(mau.saiSo > 0 && mau.saiSo < 0.02, "sai số phải dương và nhỏ hơn bề dày một nét");
});

test("cổng nếp gấp cũng chạy trên thứ thật sự lên màn: tám sticker và mười cảnh", () => {
  // `hinhNep` in isolation is not what a screen draws. Scenes and stickers
  // compose props with the figure, and the pencil nib in `ghi-lai` is a second
  // coral triangle that ink legitimately touches -- the reason the fold is
  // identified by shape rather than by «the first coral layer».
  let coNep = 0;
  for (const id of STICKER_IDS) {
    for (const chiTiet of [true, false]) {
      const ra = khongCatNepGap(`sticker ${id}/${chiTiet}`, hinhSticker(id, { chiTiet }).lop, VAI_STICKER);
      if (ra !== null) coNep += 1;
    }
  }
  for (const id of CANH_IDS) {
    for (const nep of [true, false]) {
      const ra = khongCatNepGap(`cảnh ${id}/nep=${nep}`, hinhCanh(id, { nep }));
      if (ra !== null) coNep += 1;
    }
  }
  // Every sticker carries the figure; among the scenes only the ones that are
  // allowed the figure, and only in their `nep: true` reading, do.
  assert.equal(coNep, STICKER_IDS.length * 2 + (CANH_IDS.length - CANH_KHONG_NEP.size));
});

test("cổng nếp gấp thật sự đỏ: nét, mảng, và bản vẽ không có nếp gấp", () => {
  const than = hinhNep("moi", { chiTiet: true });
  const ra = khongCatNepGap("moi", than);
  assert.ok(ra !== null);
  const [H, G, Bp] = ra.dinh;
  const giua = [(H[0] + G[0] + Bp[0]) / 3, (H[1] + G[1] + Bp[1]) / 3];
  assert.ok(sauTrong(ra.dinh, giua) > 2, "trọng tâm phải nằm sâu trong tam giác");
  for (const d of ra.dinh) {
    assert.ok(Math.abs(sauTrong(ra.dinh, d)) < 0.001, "đỉnh nằm ĐÚNG trên biên");
  }

  // A stroke through the middle of the fold.
  assert.throws(
    () => khongCatNepGap("canary nét", [...than, { d: netGay([giua, [giua[0] + 4, giua[1] + 2]]), mau: "muc", net: 1.9 }]),
    /sơn .* lên nếp gấp/,
    "một nét mực giữa nếp gấp phải làm cổng đỏ",
  );
  // A filled blob in the middle of the fold: strokes and fills are different
  // code paths (`net` present or absent) and both have to be watched.
  assert.throws(
    () => khongCatNepGap("canary mảng", [...than, { d: tron(giua[0], giua[1], 1.4), mau: "muc" }]),
    /sơn .* lên nếp gấp/,
    "một mảng mực giữa nếp gấp phải làm cổng đỏ",
  );
  // Paint, not centre lines. The centre of this hairline is OUTSIDE the fold --
  // asserted, not assumed -- and only its width carries ink onto the coral. It
  // is the case the first draft accepted, and the premise has to be a real
  // negative depth or the canary passes for the wrong reason and stops biting.
  const giuaCanh = [H[0] + (Bp[0] - H[0]) * 0.6, H[1] + (Bp[1] - H[1]) * 0.6];
  const nganh = [giuaCanh[0] - 0.5, giuaCanh[1]];
  const sauNganh = sauTrong(ra.dinh, nganh);
  assert.ok(sauNganh < 0, `tiền đề: tâm nét phải nằm NGOÀI nếp gấp, đo được ${sauNganh.toFixed(2)}`);
  assert.ok(sauNganh > -1.2, "tiền đề: và đủ gần để bề dày 2.4 với tới");
  assert.throws(
    () => khongCatNepGap("canary bề dày", [...than, { d: netGay([nganh, [nganh[0], nganh[1] - 3]]), mau: "muc", net: 2.4 }]),
    /sơn .* lên nếp gấp/,
    "bề dày của nét phải được tính, không chỉ tâm nét",
  );
  // The outline exemption is for STROKES only. An ink FILL that happens to
  // trace the same path as some other filled shape is still a blob of ink, not
  // an outline of one -- and written without that distinction the exemption
  // swallowed every eye and every hand.
  const trung = tron(giua[0], giua[1], 1.4);
  assert.throws(
    () => khongCatNepGap("canary mảng trùng đường", [
      ...than,
      { d: trung, mau: "gap" },
      { d: trung, mau: "muc" },
    ]),
    /sơn .* lên nếp gấp/,
    "mảng mực trùng đường với một mảng khác vẫn là mực trên coral",
  );
  // A triangle with the fold's exact 19:18 proportions but an apex no lean
  // could put there is not the fold either. Without this the sanity bound on
  // the lean is dead code that no drawing exercises.
  assert.equal(
    khongCatNepGap("nghiêng vô lý", [
      { d: daGiac([[200, 20], [69, 38], [50, 38]]), mau: "gap" },
      { d: netGay([[106, 32], [110, 33]]), mau: "muc", net: 2 },
    ]),
    null,
    "đúng tỉ lệ nhưng nghiêng vô lý thì không phải nếp gấp",
  );
  // And a drawing whose only coral triangle is NOT the fold must read as «no
  // fold here», never as «scanned and clean».
  assert.equal(
    khongCatNepGap("không có nếp gấp", [
      { d: daGiac([[92, 21], [96, 18], [95, 25]]), mau: "gap" },
      { d: netGay([[92, 21], [95, 25]]), mau: "muc", net: 2 },
    ]),
    null,
    "tam giác coral khác nếp gấp không được nhận nhầm là nếp gấp",
  );
});

test("bảy biểu cảm là bảy cái mày khác nhau, và met ngược chiều quyet", () => {
  // The brow is the only ink stroke that lives entirely above the eyes, once
  // the fold's own outline (a stroke retracing a filled shape) is set aside.
  const may = (bieuCam) => {
    const lop = hinhNep("moi", { chiTiet: true, bieuCam, nghieng: 0 });
    const dMang = new Set(lop.filter((l) => l.net === undefined).map((l) => l.d));
    const ung = lop.filter(
      (l) => l.mau === "muc" && l.net !== undefined && !dMang.has(l.d) &&
        phanTich(l.d).filter((x) => x.c !== "Z").every((x) => x.args[1] < 41 && (x.args.length < 4 || x.args[3] < 41)),
    );
    assert.equal(ung.length, 1, `${bieuCam}: phải nhận ra đúng MỘT cái mày, thấy ${ung.length}`);
    return ung[0];
  };

  // Seven drawings, seven paths. Two expressions sharing a brow path is the bug
  // that made `binh-than` and `nhuong` the same face until 09/09.
  const duong = BIEU_CAM.map((bc) => may(bc).d);
  assert.equal(new Set(duong).size, BIEU_CAM.length, "hai biểu cảm đang dùng chung một cái mày");

  // And the two that were confusable are now mirror images. `met` is resigned:
  // the end toward the nose lifts. `quyet` bears down: that end drops. Drawn
  // with the same slope they are one face, whatever the mouth does.
  const doc = (bc) => {
    const d = phanTich(may(bc).d).filter((x) => x.c !== "Z");
    const dau = d[0].args;
    const cuoi = d[d.length - 1].args;
    return (cuoi[cuoi.length - 1] - dau[1]) / (cuoi[cuoi.length - 2] - dau[0]);
  };
  const met = doc("met");
  const quyet = doc("quyet");
  assert.ok(quyet > 0.2, `quyet: đầu trong phải chúc XUỐNG, dốc đo được ${quyet.toFixed(2)}`);
  assert.ok(met < -0.2, `met: đầu trong phải hếch LÊN, dốc đo được ${met.toFixed(2)}`);
});

/**
 * Is this triangle the pocket sheet's coral SLIVER, i.e. S(50,20)·S(69,38)·S(57.6,31.4)?
 *
 * Same idea as `laNepGap`: identify by shape so placement and lean cannot blind
 * the gate. The sliver has three DISTINCT y levels (the page's fold has two on
 * one horizontal, so the two never match each other), and it is thin: the
 * third vertex sits within a quarter of the long edge's length from that edge.
 * `ghi-lai`'s pencil nib is fat (ratio ≈ 0.5) and fails; `dua-giay`'s small
 * corner has two vertices on one horizontal and fails the first clause.
 */
function laDaiGap(dinh) {
  const y = dinh.map((p) => p[1]);
  const [tren, giua, duoi] = [...dinh].sort((a, b) => a[1] - b[1]);
  if (new Set(y.map((v) => Math.round(v / LAM_TRON))).size !== 3) return false;
  const dx = duoi[0] - tren[0], dy = duoi[1] - tren[1];
  const L = Math.hypot(dx, dy);
  if (L === 0) return false;
  const d = Math.abs(dx * (tren[1] - giua[1]) - dy * (tren[0] - giua[0])) / L;
  const r = d / L;
  return r > 0.05 && r < 0.25;
}

test("gap trang: bản vẽ trùng từng toạ độ với baseline lấy từ main trước khi có biến thể", () => {
  // The «không đụng hội bạn» gate of the art layer. Every scene and sticker
  // composes `hinhNep` with the default `gap`, so the default output is pinned
  // to a sha256 taken from `main` (tests/fixtures/nep-trang-baseline.json).
  // Stored as a hash, not as paths: a coordinate list is nine digits with
  // single spaces between them, which the repo guard reads as an account number.
  const goc = JSON.parse(readFileSync(new URL("./fixtures/nep-trang-baseline.json", import.meta.url), "utf8"));
  const bo = {};
  let tong = 0;
  for (const pose of goc.poses) {
    assert.ok(laPoseNep(pose), `baseline có pose ${pose} mà nep.ts không còn`);
    for (const chiTiet of [true, false]) {
      const lop = hinhNep(pose, { chiTiet });
      bo[`${pose}|${chiTiet}`] = lop.map((l) => `${l.mau}|${l.net ?? ""}|${l.d}`);
      tong += lop.length;
    }
  }
  const sha256 = createHash("sha256").update(JSON.stringify(bo)).digest("hex");
  assert.equal(tong, goc.soLop, "số lớp của bản trang đổi so với main");
  assert.equal(sha256, goc.sha256, "một toạ độ của bản trang đã đổi: mọi cảnh và sticker của hội bạn đổi theo");
});

test("gap manh: đúng một dải coral hé ra dọc đường cắt, và không nét mực nào sơn lên nó", () => {
  // The same rule `khongCatNepGap` keeps for the page's corner, over a smaller
  // sweep of leans: the shape identity here is `laDaiGap`, not `laNepGap`.
  let banVe = 0;
  for (const pose of POSE_NEP) {
    for (const bieuCam of BIEU_CAM) {
      for (const chiTiet of [true, false]) {
        for (const nghieng of [-8, 0, 6, 13]) {
          const ten = `manh ${pose}/${bieuCam}/${chiTiet ? "chi tiết" : "rút gọn"}/ng${nghieng}`;
          const lop = hinhNep(pose, { chiTiet, bieuCam, nghieng, gap: "manh" });
          const dai = lop.filter((l) => l.mau === "gap" && l.net === undefined).map((l) => tamGiac(l.d)).filter((t) => t && laDaiGap(t));
          assert.equal(dai.length, 1, `${ten}: phải nhận ra đúng MỘT dải gấp, thấy ${dai.length}`);
          assert.equal(lop.filter((l) => l.mau === "gap" && l.net === undefined).map((l) => tamGiac(l.d)).filter((t) => t && laNepGap(t)).length, 0, `${ten}: bản mảnh không được mang nếp gấp của bản trang`);
          const ra = khongCatNepGap(ten, lop, VAI_ART, laDaiGap);
          assert.ok(ra !== null && ra.daXet > 300, `${ten}: máy quét không nhìn dải gấp`);
          banVe += 1;
        }
      }
    }
  }
  assert.equal(banVe, POSE_NEP.length * BIEU_CAM.length * 2 * 4);
  // And the two creases are there, as `bong` strokes, in both readings.
  for (const chiTiet of [true, false]) {
    const vet = hinhNep("moi", { chiTiet, gap: "manh" }).filter((l) => l.mau === "bong" && l.net !== undefined);
    assert.equal(vet.length, 2, `manh chiTiet=${chiTiet}: đúng hai vết gấp`);
  }
});

test("giu-kin: miệng là một nét thẳng khép, ngắn hơn và phẳng hơn quyet", () => {
  const mieng = (bieuCam) => {
    const lop = hinhNep("moi", { chiTiet: true, bieuCam, nghieng: 0 });
    // The mouth is the ink stroke or fill whose every point sits between y 50 and 60.
    return lop.filter((l) => l.mau === "muc" && phanTich(l.d).filter((x) => x.c !== "Z").every((x) => x.args.every((v, i) => i % 2 === 0 || (v > 50 && v < 60))));
  };
  const gk = mieng("giu-kin"), q = mieng("quyet");
  assert.equal(gk.length, 1, "giu-kin: đúng một cái miệng");
  const doan = (l) => phanTich(l.d).filter((x) => x.c !== "Z").map((x) => x.args);
  const [a, b] = doan(gk[0]);
  assert.equal(a[1], b[1], "giu-kin: miệng phẳng tuyệt đối");
  const [qa, qb] = doan(q[0]);
  assert.ok(Math.abs(b[0] - a[0]) < Math.abs(qb[0] - qa[0]), "giu-kin ngắn hơn quyet");
  assert.notEqual(qa[1], qb[1], "quyet nghiêng, để hai mặt không thành một");
});

test("coral: ba tư thế truyền giấy có đúng MỘT lớp gap, và đổi biến thể không thêm hay bớt coral ở pose nào", () => {
  // The finish review of 12/09 counted two coral marks on `dua-giay`: the
  // sliver on the body and a coral corner on the small sheet in the hand. The
  // shape gates (`laNepGap`, `laDaiGap`) look for the fold's SHAPE and would
  // never notice a second mark of a different shape, so this counts LAYERS.
  //
  // Two invariants, not a hand-written list of exceptions. Some poses on main
  // already carry a second coral as a prop (`ghi-lai` pencil nib, `gop-y` map
  // pin); a list naming them was the first draft here and it was wrong on the
  // second pose it met. The page's counts are pinned by the sha256 baseline
  // above; what this test adds is that the pocket variant has the SAME count
  // as the page for every pose, and that the three new poses have exactly one.
  const MOI = ["dua-giay", "up-xuong", "gap-lai"];
  const dem = (pose, gap, chiTiet) => hinhNep(pose, { chiTiet, gap }).filter((l) => l.mau === "gap").length;
  for (const pose of MOI) for (const gap of GAP_NEP) for (const chiTiet of [true, false]) {
    assert.equal(dem(pose, gap, chiTiet), 1, `${pose}/${gap}/${chiTiet}: đúng một lớp coral`);
  }
  for (const pose of POSE_NEP) for (const chiTiet of [true, false]) {
    assert.equal(dem(pose, "manh", chiTiet), dem(pose, "trang", chiTiet), `${pose}/${chiTiet}: mảnh và trang phải cùng số lớp coral`);
  }
});

test("mảnh vuông hơn trang: tờ thân lấp đầy hộp bao nhiều hơn, và không hẹp hơn", () => {
  // Spec §17.2 #3. Measured on the first `giay` layer, which is the sheet.
  // Squareness is a property of the OUTLINE, not of the bounding box: the
  // first cut had a wider box and still read as a hexagon, because its foot
  // and hip were bevelled (finish review 12/09, round 2). So the gate measures
  // how much of its own box the polygon fills -- bevels cost area -- and only
  // then that the box is not narrower than the page's.
  const doTho = (gap) => {
    const than = hinhNep("moi", { nghieng: 0, gap }).find((l) => l.mau === "giay");
    const pts = phanTich(than.d).filter((x) => x.c !== "Z").map((x) => [x.args[0], x.args[1]]);
    const xs = pts.map((p) => p[0]), ys = pts.map((p) => p[1]);
    let dt = 0;
    for (let i = 0; i < pts.length; i++) {
      const [x1, y1] = pts[i], [x2, y2] = pts[(i + 1) % pts.length];
      dt += x1 * y2 - x2 * y1;
    }
    const rong = Math.max(...xs) - Math.min(...xs), cao = Math.max(...ys) - Math.min(...ys);
    return { lapDay: Math.abs(dt) / 2 / (rong * cao), tiLe: rong / cao };
  };
  const trang = doTho("trang"), manh = doTho("manh");
  assert.ok(manh.lapDay > trang.lapDay + 0.03, `mảnh lấp ${manh.lapDay.toFixed(3)} hộp bao, phải hơn trang ${trang.lapDay.toFixed(3)} rõ ràng`);
  // The ceiling is about 0.92, not 1: the cut corner H..G is the character's
  // own fold and costs the box some 7% on both variants. Measured 0.876 with
  // straight edges against 0.838 for the page; below 0.87 a bevel is back.
  assert.ok(manh.lapDay >= 0.87, `mảnh lấp ${manh.lapDay.toFixed(3)}: dưới 0.87 là còn vát góc, đọc thành lục giác`);
  assert.ok(manh.tiLe > trang.tiLe + 0.02, `mảnh ${manh.tiLe.toFixed(3)} không được hẹp hơn trang ${trang.tiLe.toFixed(3)}`);
});
