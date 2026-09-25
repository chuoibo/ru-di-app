/**
 * The motion maths of the paper stage (ADR-0037 D3), as plain functions.
 *
 * Every function here is a worklet: it runs on the UI thread inside
 * `useDerivedValue` / `useAnimatedStyle`, and the same code runs under node for
 * the tests (the `"worklet"` directive is an ordinary string statement there).
 * No React, no Reanimated import: the numbers are the contract, the hooks only
 * feed them.
 *
 * Angles are in degrees at this boundary; renderers convert.
 *   0  = a layer standing upright out of the page
 *   90 = the same layer lying flat in the page
 */
import { MOTION_MS, NGAN_SACH_SAN_KHAU } from "../motion";

const GOC_NAM = 90;

function kep(x: number, a: number, b: number): number {
  "worklet";
  return x < a ? a : x > b ? b : x;
}

/** cubic-bezier(0, 0, 0.2, 1) -- the `decelerate` easing -- solved for x = t. */
export function giamToc(t: number): number {
  "worklet";
  const x = kep(t, 0, 1);
  // The ends are exact, so a stage at rest is exactly flat or exactly upright.
  if (x === 0 || x === 1) return x;
  // Bisection on x(s) of the bezier with P1 = (0, 0), P2 = (0.2, 1). Newton
  // from s = x overshoots near 0, where x(s) is flat (about 0.6 s^2); halving
  // an interval cannot, and 24 halvings are far below a pixel.
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 24; i += 1) {
    const s = (lo + hi) / 2;
    const u = 1 - s;
    const xs = 3 * u * s * s * 0.2 + s * s * s;
    if (xs < x) lo = s;
    else hi = s;
  }
  const s = (lo + hi) / 2;
  const u = 1 - s;
  return 3 * u * s * s + s * s * s;
}

/**
 * The fold angle of layer `i` of `n` while the whole stage opens, `mo` in
 * 0..1 of the pop-up's total time. Far layers rise first; each layer rises over
 * `shared`, starting `batTang` after the one behind it, and the fifth layer on
 * starts with the fourth (so the total never passes `batToiDa`).
 */
export function gocBatTang(mo: number, i: number, n: number): number {
  "worklet";
  const so = Math.max(1, n);
  const tong = Math.min(NGAN_SACH_SAN_KHAU.batToiDa, MOTION_MS.shared + NGAN_SACH_SAN_KHAU.batTang * Math.min(3, so - 1));
  const t = kep(mo, 0, 1) * tong;
  const tre = NGAN_SACH_SAN_KHAU.batTang * Math.min(3, Math.max(0, i));
  const p = kep((t - tre) / MOTION_MS.shared, 0, 1);
  return GOC_NAM * (1 - giamToc(p));
}

/**
 * A stage used as a screen header folds flat as the list scrolls: 0 degrees at
 * the top, `gocToiDa` once the list has scrolled `h` (the stage's own height),
 * hinged on the stage's bottom edge like a pop-up page closing.
 */
export function gocGapCuon(y: number, h: number, gocToiDa = 80): number {
  "worklet";
  if (h <= 0) return 0;
  return gocToiDa * kep(y / h, 0, 1);
}

/** How far a layer of depth `sau` (0 far .. 3 near) slides for a parallax input `v` in -1..1. */
export function lechThiSai(v: number, sau: number, bienDo = 6): number {
  "worklet";
  // Far layers barely move, near layers move most: the eye reads depth from
  // the difference, not from any single layer's travel.
  return kep(v, -1, 1) * bienDo * ((kep(sau, 0, 3) + 1) / 4);
}

/**
 * The shadow strength of a standing layer at fold angle `goc`: 1 upright,
 * 0 lying flat -- a layer printed into the page casts nothing.
 */
export function bongTheoGoc(goc: number): number {
  "worklet";
  return Math.cos((kep(goc, 0, GOC_NAM) * Math.PI) / 180);
}

/** Which way a page turns between steps: forward turns from the right edge. */
export function huongLat(truoc: number, sau: number): 1 | -1 {
  "worklet";
  return sau >= truoc ? 1 : -1;
}

/** The three beats of a stamp landing (the `Stamp` contract): drop, ink, sink. */
export const NHIP_DAU = Object.freeze({ lao: 130, cham: 60 });

/**
 * Where a stamp is at `t` ms after it starts: `roi` 0..1 over the drop
 * (accelerating), `muc` 0..1 over the contact after it, `xong` once both are done.
 */
export function giaiDoanDau(t: number): { roi: number; muc: number; xong: boolean } {
  "worklet";
  const roiTho = kep(t / NHIP_DAU.lao, 0, 1);
  const roi = roiTho * roiTho;
  const muc = kep((t - NHIP_DAU.lao) / NHIP_DAU.cham, 0, 1);
  return { roi, muc, xong: t >= NHIP_DAU.lao + NHIP_DAU.cham };
}

/**
 * How far a notebook cover swings open (M6, «bìa sổ mở»): past upright and
 * short of flat, so the inside of the cover stays in view as a page and the
 * cover never ends edge-on. `phoiCanh` is the perspective distance in dp.
 */
export const BIA_MO = Object.freeze({ gocToiDa: 108, phoiCanh: 800 });

/**
 * The room a cover of `rong` x `cao` needs round its book while it swings
 * from shut (0) to `goc` degrees about the spine: `trai` to the left of the
 * spine, where the inside of the cover ends up, and `doc` above and below,
 * where its free edge grows as it comes toward the eye (a point `z` nearer is
 * drawn `p / (p - z)` times larger, about the hinge's centre). A book that
 * keeps this room never paints over its neighbours mid-swing.
 */
export function choLatBia(rong: number, cao: number, goc: number = BIA_MO.gocToiDa, p: number = BIA_MO.phoiCanh): { trai: number; doc: number } {
  let trai = 0;
  let doc = 0;
  for (let g = 0; g <= goc; g += 1) {
    const r = (g * Math.PI) / 180;
    const phong = p / (p - rong * Math.sin(r));
    trai = Math.max(trai, -rong * Math.cos(r) * phong);
    doc = Math.max(doc, ((phong - 1) * cao) / 2);
  }
  return { trai: Math.ceil(trai), doc: Math.ceil(doc) };
}
