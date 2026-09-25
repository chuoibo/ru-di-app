/**
 * The paper objects of the stage (ADR-0037 D1): the outlines that make a card
 * read as a receipt, a ticket, a stamp, an envelope, a torn stub -- drawn
 * behind React Native text, never carrying text themselves.
 *
 * Every shape is a pure function of its size (dp) and returns path strings in
 * the grammar the art gate accepts (absolute M/L/C/Z, plain decimals). Edges
 * that look hand-made (a tear) are deterministic: a fixed wobble by index, so
 * the same width always prints the same edge and nothing shimmers on
 * re-render. Shapes stay inside their box, so a component can lay them out
 * with the size it measured and nothing spills past its edge.
 */
import { type Diem, daGiac, doanCungTron, duong, netGay } from "./net";

const PI = Math.PI;

/** A deterministic wobble in -1..1 for index `i`: two octaves of sines, never random. */
function lech(i: number): number {
  return 0.6 * Math.sin(i * 2.399) + 0.4 * Math.sin(i * 5.07 + 1.3);
}

/**
 * The points of a zigzag edge along y from x0 to x1: teeth `buoc` wide,
 * `sau` deep, pointing into the paper (`huong` +1 down, -1 up). The teeth are
 * fitted to the length so both ends land on the edge.
 */
export function diemRangCua(x0: number, x1: number, y: number, buoc: number, sau: number, huong: 1 | -1): Diem[] {
  const dai = Math.abs(x1 - x0);
  const so = Math.max(1, Math.round(dai / Math.max(2, buoc)));
  const b = (x1 - x0) / so;
  const diem: Diem[] = [[x0, y]];
  for (let i = 0; i < so; i += 1) {
    diem.push([x0 + b * (i + 0.5), y + huong * sau]);
    diem.push([x0 + b * (i + 1), y]);
  }
  return diem;
}

export interface TuyChonHoaDon {
  rangTren?: boolean;
  rangDuoi?: boolean;
  buoc?: number;
  sau?: number;
}

/**
 * A thermal receipt: a sheet whose top and/or bottom edge is torn to teeth.
 * `nen` is the fill (paper), `vien` the same outline as a stroke.
 */
export function hinhHoaDon(w: number, h: number, tuyChon: TuyChonHoaDon = {}): { nen: string; vien: string } {
  const { rangTren = false, rangDuoi = true, buoc = 10, sau = 4 } = tuyChon;
  const tren = rangTren ? diemRangCua(0, w, sau, buoc, sau, -1) : ([[0, 0], [w, 0]] as Diem[]);
  const duoi = rangDuoi ? diemRangCua(w, 0, h - sau, buoc, sau, 1) : ([[w, h], [0, h]] as Diem[]);
  const vong: Diem[] = [...tren, ...duoi];
  return { nen: daGiac(vong), vien: netGay([...vong, vong[0]]) };
}

export interface TuyChonVe {
  /** Corner radius. */
  r?: number;
  /** Where the tear line runs (x); the notches sit on it. Default: 72% of the width. */
  xCat?: number;
  /** Notch radius. */
  rKhuyet?: number;
}

/**
 * A ticket: a rounded card with a semicircle bitten out of the top and bottom
 * edge where the stub tears off, and the perforation between the two notches.
 */
export function hinhVe(w: number, h: number, tuyChon: TuyChonVe = {}): { nen: string; vien: string; duc: string; xCat: number } {
  const r = Math.min(tuyChon.r ?? 12, w / 4, h / 4);
  const rk = Math.min(tuyChon.rKhuyet ?? 8, h / 4);
  const xCat = Math.min(w - r - rk - 1, Math.max(r + rk + 1, tuyChon.xCat ?? w * 0.72));
  const K = 0.5523 * r;
  const phan = [
    duong("M", r, 0),
    duong("L", xCat - rk, 0),
    // the notch in the top edge, bitten downward (clockwise from 180 to 0 degrees)
    ...doanCungTron(xCat, 0, rk, PI, 0),
    duong("L", w - r, 0),
    duong("C", w - r + K, 0, w, r - K, w, r),
    duong("L", w, h - r),
    duong("C", w, h - r + K, w - r + K, h, w - r, h),
    duong("L", xCat + rk, h),
    // the notch in the bottom edge, bitten upward
    ...doanCungTron(xCat, h, rk, 0, -PI),
    duong("L", r, h),
    duong("C", r - K, h, 0, h - r + K, 0, h - r),
    duong("L", 0, r),
    duong("C", 0, r - K, r - K, 0, r, 0),
  ];
  const nen = [...phan, "Z"].join(" ");
  const duc = netGay([[xCat, rk + 3], [xCat, h - rk - 3]]);
  return { nen, vien: phan.join(" "), duc, xCat };
}

/**
 * A postage stamp: a rectangle whose four edges are bitten by semicircles
 * every `buoc`, the perforation of a sheet of stamps.
 */
export function hinhTem(w: number, h: number, buoc = 9, r = 2.6): string {
  const soNgang = Math.max(2, Math.round(w / buoc));
  const soDoc = Math.max(2, Math.round(h / buoc));
  const bx = w / soNgang;
  const by = h / soDoc;
  const phan: string[] = [duong("M", 0, 0)];
  // top edge, left to right: bites point down
  for (let i = 0; i < soNgang; i += 1) {
    const cx = bx * (i + 0.5);
    phan.push(duong("L", cx - r, 0), ...doanCungTron(cx, 0, r, PI, 0));
  }
  phan.push(duong("L", w, 0));
  // right edge, top to bottom: bites point left
  for (let i = 0; i < soDoc; i += 1) {
    const cy = by * (i + 0.5);
    phan.push(duong("L", w, cy - r), ...doanCungTron(w, cy, r, -PI / 2, -1.5 * PI));
  }
  phan.push(duong("L", w, h));
  // bottom edge, right to left: bites point up
  for (let i = soNgang - 1; i >= 0; i -= 1) {
    const cx = bx * (i + 0.5);
    phan.push(duong("L", cx + r, h), ...doanCungTron(cx, h, r, 0, -PI));
  }
  phan.push(duong("L", 0, h));
  // left edge, bottom to top: bites point right
  for (let i = soDoc - 1; i >= 0; i -= 1) {
    const cy = by * (i + 0.5);
    phan.push(duong("L", 0, cy + r), ...doanCungTron(0, cy, r, PI / 2, -PI / 2));
  }
  phan.push("Z");
  return phan.join(" ");
}

/**
 * A torn edge from x0 to x1 around y: short irregular steps within
 * `bienDo`, deterministic by index -- the edge a stub keeps after it is torn.
 */
export function diemXe(x0: number, x1: number, y: number, bienDo = 3, buoc = 6): Diem[] {
  const so = Math.max(2, Math.round(Math.abs(x1 - x0) / buoc));
  const diem: Diem[] = [];
  for (let i = 0; i <= so; i += 1) {
    const x = x0 + ((x1 - x0) * i) / so;
    const dy = i === 0 || i === so ? 0 : lech(i) * bienDo;
    diem.push([x, y + dy]);
  }
  return diem;
}

/** A card torn along the bottom: the stub of a receipt, the half of a ticket that stays. */
export function hinhCuong(w: number, h: number, bienDo = 3): { nen: string; vien: string } {
  const day = diemXe(w, 0, h - bienDo, bienDo);
  const vong: Diem[] = [[0, 0], [w, 0], ...day];
  return { nen: daGiac(vong), vien: netGay([...vong, vong[0]]) };
}

/**
 * An envelope seen from the back: the body, the flap (closed = a triangle
 * down to 55% of the height; open = the same triangle flipped up above the
 * body, `mo` 0..1 between the two), and the two diagonal folds of the pocket.
 * The flap of an open envelope reaches above y = 0, so the drawing box is
 * `h + h * 0.55` tall with the body starting at `yThan`.
 */
export function hinhPhongBi(w: number, h: number, mo = 0): { than: string; nap: string; nep: string; yThan: number; cao: number } {
  const sauNap = h * 0.55;
  const yThan = sauNap;
  const m = Math.max(0, Math.min(1, mo));
  const dinhY = yThan + sauNap * (1 - 2 * m);
  const than = daGiac([[0, yThan], [w, yThan], [w, yThan + h], [0, yThan + h]]);
  const nap = daGiac([[0, yThan], [w, yThan], [w / 2, dinhY]]);
  const nep = netGay([[0, yThan + h], [w / 2, yThan + h * 0.45], [w, yThan + h]]);
  return { than, nap, nep, yThan, cao: yThan + h };
}

/**
 * A dashed line as one path of short strokes (`M ... L ...` per dash): the
 * perforation of a ticket, the dotted cut of a receipt. The dashes are fitted
 * so both ends of the line are ink, never a gap.
 */
export function duongDut(a: Diem, b: Diem, gach = 4, ho = 3): string {
  const dx = b[0] - a[0];
  const dy = b[1] - a[1];
  const dai = Math.hypot(dx, dy);
  if (dai === 0) return duong("M", a[0], a[1], "L", b[0], b[1]);
  const so = Math.max(1, Math.round((dai + ho) / (gach + ho)));
  const buoc = (dai + ho) / so;
  // The dash stretches or shrinks a little so the last one ends ON the far
  // end: with `so` dashes and `so - 1` gaps, dash = step - gap exactly.
  const g = buoc - ho > 0 ? buoc - ho : dai;
  const ux = dx / dai;
  const uy = dy / dai;
  const phan: string[] = [];
  for (let i = 0; i < so; i += 1) {
    const t0 = i * buoc;
    const t1 = Math.min(dai, t0 + g);
    phan.push(duong("M", a[0] + ux * t0, a[1] + uy * t0, "L", a[0] + ux * t1, a[1] + uy * t1));
  }
  return phan.join(" ");
}

/**
 * The flourish under a signature (the pact of the two-person notebook, plan
 * S2): a small loop where the pen lands, then one long easing stroke that
 * rises at its end, `w` wide and `h` tall. One open path of cubics -- it is
 * drawn on as a stroke, so it has a length and a start. Deterministic.
 */
export function netChuKy(w: number, h = 14): string {
  const y = h * 0.62;
  const loop = Math.min(18, w * 0.12);
  return duong(
    "M", 2, y,
    "C", 2 + loop * 0.35, y - h * 0.55, 2 + loop, y - h * 0.5, 2 + loop * 0.8, y,
    "C", 2 + loop * 0.62, y + h * 0.32, 2 + loop * 0.18, y + h * 0.18, 2 + loop * 0.55, y,
    "C", w * 0.38, y - h * 0.18, w * 0.7, y + h * 0.28, w - 2, y - h * 0.46,
  );
}
