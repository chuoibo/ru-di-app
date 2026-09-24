/* The coral fold gate, shared: `art-duong.test.mjs` runs it over every still
 * drawing of Nếp, the stickers and the scenes; `nep-roi.test.mjs` runs it over
 * every frame of every performance of the paper puppet. Moved here verbatim
 * from art-duong.test.mjs (S0.4), so the two sweeps cannot drift apart.
 */
import assert from "node:assert/strict";

import { phanTich } from "./_kiem-lop.mjs";

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
export const MAU_CUBIC = 24;

/*
 * Two colour vocabularies reach this gate. `art/*.ts` names roles
 * `giay·bong·muc·gap`; `chat/sticker.ts` re-labels the same drawing through
 * `tuLopVe` into `card·line·ink·accent`. Looking only for «gap» found the fold
 * in the nine scenes and in NONE of the sixteen sticker readings, and the count
 * assertion below is what said so out loud instead of reporting a clean scan.
 */
export const VAI_ART = { coral: "gap", muc: "muc" };
export const VAI_STICKER = { coral: "accent", muc: "ink" };

/**
 * Points to test on one path, each with the error bound that applies to it.
 *
 * Straight commands are exact. A cubic contributes `MAU_CUBIC` samples plus the
 * chord error max|B''|/(8N²), computed from its own control points.
 */
export function diemCua(d) {
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
export const ben = (a, b, p) => (b[0] - a[0]) * (p[1] - a[1]) - (b[1] - a[1]) * (p[0] - a[0]);

/** How far inside the triangle p lies, in grid units. Negative means outside. */
export function sauTrong(tam, p) {
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
export const LAM_TRON = 0.02;

/** The three points of a triangle path, or null if it is not a triangle. */
export function tamGiac(d) {
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
export function laNepGap(dinh) {
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
export function khongCatNepGap(ten, lop, vai = VAI_ART, nhanDien = laNepGap) {
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


/**
 * Is this triangle the pocket sheet's coral SLIVER, i.e. S(50,20)·S(69,38)·S(57.6,31.4)?
 *
 * Same idea as `laNepGap`: identify by shape so placement and lean cannot blind
 * the gate. The sliver has three DISTINCT y levels (the page's fold has two on
 * one horizontal, so the two never match each other), and it is thin: the
 * third vertex sits within a third of the long edge's length from that edge
 * (the 48dp reading's sliver is 6 units thick on a 26-unit edge, and a lean of
 * 13 shears that to 0.28). `ghi-lai`'s pencil nib is fat (ratio ≈ 0.5) and
 * fails; `dua-giay`'s small corner has two vertices on one horizontal and
 * fails the first clause.
 */
/** Thickness of the sliver: the third vertex's distance from its long edge. */
export function beDayDai(dinh) {
  const [tren, giua, duoi] = [...dinh].sort((a, b) => a[1] - b[1]);
  const dx = duoi[0] - tren[0], dy = duoi[1] - tren[1];
  const L = Math.hypot(dx, dy);
  return L === 0 ? 0 : Math.abs(dx * (tren[1] - giua[1]) - dy * (tren[0] - giua[0])) / L;
}

export function laDaiGap(dinh) {
  const y = dinh.map((p) => p[1]);
  const [tren, giua, duoi] = [...dinh].sort((a, b) => a[1] - b[1]);
  if (new Set(y.map((v) => Math.round(v / LAM_TRON))).size !== 3) return false;
  const dx = duoi[0] - tren[0], dy = duoi[1] - tren[1];
  const L = Math.hypot(dx, dy);
  if (L === 0) return false;
  const d = Math.abs(dx * (tren[1] - giua[1]) - dy * (tren[0] - giua[0])) / L;
  const r = d / L;
  return r > 0.05 && r < 0.32;
}

