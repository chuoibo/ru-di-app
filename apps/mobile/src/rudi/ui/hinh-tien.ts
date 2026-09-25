/**
 * The geometry of the money screens' paper objects (ADR-0037, plan S1), pure
 * so node tests hold it: where everyone sits at the bill table, how long the
 * collection's strip of tape is, and where the settlement's ink arrows run.
 *
 * None of it knows an amount. The table seats people, the tape is a count of
 * arrivals the server made, the arrows say who pays whom; every number a
 * person reads is the server's, printed as text beside these drawings.
 */
import { netGay, qCong, type Diem } from "../art/net";

// ---- The bill table (`ui/BanGanMon`) ------------------------------------------

/** A seat's standee: head and body, in dp. */
export const GHE = 40;
/** The name line over a seat behind the table. */
export const TEN_TREN = 18;
/** A seat's box is this wide (the standee and a name under or over it). */
export const RONG_GHE = GHE * 1.8;
/** The dish card in the middle of the table: 128 wide, narrower on a small table so the plates stay in view. */
export function theMon(rx: number): { w: number; h: number } {
  return { w: Math.min(128, Math.round(rx * 1.1)), h: 52 };
}

export interface NguoiQuanhBan {
  id: string;
  name: string;
}

export interface ViTriGhe {
  id: string;
  name: string;
  /** Centre of the standee. */
  x: number;
  y: number;
  /** In front of the table (drawn after it, name below) or behind it (name above). */
  truoc: boolean;
  /** The plate in front of this seat, on the table. */
  dia: { x: number; y: number };
}

export interface HinhBan {
  cx: number;
  cy: number;
  rx: number;
  ry: number;
  /** The frame's height. */
  cao: number;
  ghe: ViTriGhe[];
}

/**
 * Where everyone sits. Two people face each other from the table's two ends,
 * so a couple's table stays short and the page keeps its room (the list under
 * it must stay on the first screen: `.maestro/28` taps a row there without
 * scrolling); three or more sit round it, starting at the near side. A seat
 * behind the table carries its name above the head, where the table cannot
 * cover it, and the side seats step a little further out so a long name
 * clears the rim.
 */
export function viTriGhe(nguoi: readonly NguoiQuanhBan[], w: number): HinhBan {
  const n = nguoi.length;
  const haiDau = n === 2;
  // A table the size of a table: on a tablet the frame widens, the table does not.
  const rx = haiDau ? Math.min(130, Math.max(60, w / 2 - GHE * 1.4 - 10)) : Math.min(170, Math.max(60, w / 2 - GHE - 22));
  const ry = rx * 0.42;
  const cx = w / 2;
  // The ring the seats stand on. From five people on, seats stand on the
  // diagonals too, and a side seat's name hangs right where the diagonal
  // seat below it has its head: the ring opens up to 100 tall so they clear.
  const vongY = n >= 5 ? Math.max(ry + GHE * 0.5, 100) : ry + GHE * 0.5;
  const cy = haiDau ? ry + 22 : vongY + GHE * 1.2;
  const the = theMon(rx);
  const ghe = nguoi.map((p, i) => {
    const t = haiDau ? (i === 0 ? Math.PI : 0) : Math.PI / 2 + (i * 2 * Math.PI) / Math.max(1, n);
    // A plate at the rim in front of its seat, clear of the dish card in the
    // middle; a couple's plates sit just past the card's two ends.
    const dia = haiDau ? Math.min(rx - 8, Math.max(rx * 0.72, the.w / 2 + 4)) / rx : 0.84;
    return {
      id: p.id,
      name: p.name,
      x: cx + (rx + GHE * 0.55 + (haiDau ? 0 : 4)) * Math.cos(t),
      y: cy + vongY * Math.sin(t),
      truoc: Math.sin(t) > -1e-9,
      dia: { x: cx + rx * dia * Math.cos(t), y: cy + ry * dia * Math.sin(t) },
    };
  });
  const cao = haiDau ? cy + ry + 24 : cy + vongY + GHE + 2;
  return { cx, cy, rx, ry, cao, ghe };
}

/** Where a seat's standee is drawn (head to base): what a name must never cover. */
export function hinhCuaGhe(g: ViTriGhe): { trai: number; tren: number; phai: number; duoi: number } {
  return { trai: g.x - 17, tren: g.y - GHE * 0.75, phai: g.x + 17, duoi: g.y + 19 };
}

/** Where a seat's name line is: under the standee in front of the table, over it behind. */
export function tenCuaGhe(g: ViTriGhe): { trai: number; tren: number; phai: number; duoi: number } {
  const tren = g.truoc ? g.y + 21 : g.y - GHE * 0.75 - TEN_TREN;
  return { trai: g.x - RONG_GHE / 2, tren, phai: g.x + RONG_GHE / 2, duoi: tren + TEN_TREN };
}

/** The box a seat takes in the frame: the standee, and its name above or below. */
export function hopGhe(g: ViTriGhe): { trai: number; tren: number; phai: number; duoi: number } {
  const tren = g.y - GHE * 0.75 - (g.truoc ? 0 : TEN_TREN);
  return { trai: g.x - RONG_GHE / 2, tren, phai: g.x + RONG_GHE / 2, duoi: tren + GHE * 1.25 + TEN_TREN };
}

// ---- The collection's tape (`ui/DaiTienDo`) -----------------------------------

/** The shortest tape that still reads as tape (its two torn ends). */
export const BANG_NGAN_NHAT = 14;

/**
 * The tape's length on a track `w` wide: none at zero, the whole track when
 * all arrived, and never so short that one arrival out of many vanishes.
 */
export function dayBang(da: number, tong: number, w: number): number {
  if (!(tong > 0) || !(da > 0) || w <= 0) return 0;
  return Math.min(w, Math.max(BANG_NGAN_NHAT, Math.round(w * Math.min(1, da / tong))));
}

// ---- The settlement's arrows (`ui/SoDoChuyen`) --------------------------------

/** How far an arrow keeps from a figure's centre: its head above, its sides. */
export const R_NHAN = 30;
/** The arrowhead's arms. */
const MUI = 8;

export interface ChuyenSoDo {
  tu: string;
  toi: string;
}

export interface MuiTen {
  tu: string;
  toi: string;
  /** The shaft: one cubic, drawn as a stroke. */
  d: string;
  /** The head: two arms meeting at the tip. */
  dau: string;
  /** The shaft's length (dp), for drawing it on. */
  dai: number;
  /** Where the shaft ends. */
  mui: Diem;
  /** The shaft's control point (the quadratic's), for tests that walk it. */
  dieuKhien: Diem;
}

export interface HinhNhanSoDo {
  id: string;
  /** Centre of the standee. */
  x: number;
  y: number;
  /** Where the name hangs: under the standee, or over it (the top row, so arrows leave downwards clear of it). */
  ten: "duoi" | "tren";
}

export interface SoDo {
  w: number;
  h: number;
  nguoi: HinhNhanSoDo[];
  muiTen: MuiTen[];
}

/** Points along the shaft (the quadratic the cubic equals), ends included. */
export function diemTrenMui(p0: Diem, q: Diem, p1: Diem, soDoan = 24): Diem[] {
  const ra: Diem[] = [];
  for (let i = 0; i <= soDoan; i += 1) {
    const t = i / soDoan;
    const a = (1 - t) * (1 - t);
    const b = 2 * (1 - t) * t;
    const c = t * t;
    ra.push([a * p0[0] + b * q[0] + c * p1[0], a * p0[1] + b * q[1] + c * p1[1]]);
  }
  return ra;
}

function doDai(diem: readonly Diem[]): number {
  let dai = 0;
  for (let i = 1; i < diem.length; i += 1) dai += Math.hypot(diem[i][0] - diem[i - 1][0], diem[i][1] - diem[i - 1][1]);
  return dai;
}

/** One row of figures spread evenly across the width. */
function hang(ids: readonly string[], w: number, y: number, ten: HinhNhanSoDo["ten"]): HinhNhanSoDo[] {
  return ids.map((id, i) => ({ id, x: (w * (i + 0.5)) / ids.length, y, ten }));
}

/** A figure's box in the diagram: the standee, and its name over or under it. */
export function hopNhan(p: HinhNhanSoDo): { trai: number; tren: number; phai: number; duoi: number } {
  return p.ten === "tren" ? { trai: p.x - 44, tren: p.y - 46, phai: p.x + 44, duoi: p.y + 22 } : { trai: p.x - 44, tren: p.y - 26, phai: p.x + 44, duoi: p.y + 40 };
}

/**
 * The transfer diagram. A settlement's transfers run from the people who owe
 * to the people who are owed, and in the server's minimal set nobody is both
 * -- so the drawing is two sides facing each other and no arrow ever has to
 * pass a third figure:
 *   - a few people (at most two each side): payers in a column on the left,
 *     the paid in a column on the right, arrows across the middle;
 *   - more: the paid in a row through the middle, the payers in a row above
 *     and a row below them, every arrow a short run between two rows.
 * Only the people a transfer names are drawn; a set that is not minimal
 * still draws, a person placed on the side they first appear on.
 * Deterministic in the order the transfers come.
 */
export function soDoChuyen(chuyen: readonly ChuyenSoDo[], w: number): SoDo {
  const tra: string[] = [];
  const nhan: string[] = [];
  for (const c of chuyen) {
    if (!tra.includes(c.tu) && !nhan.includes(c.tu)) tra.push(c.tu);
    if (!tra.includes(c.toi) && !nhan.includes(c.toi)) nhan.push(c.toi);
  }
  const DONG = 96;
  let h: number;
  let nguoi: HinhNhanSoDo[];
  const cot = tra.length <= 2 && nhan.length <= 2;
  if (cot) {
    const so = Math.max(tra.length, nhan.length, 1);
    h = so * DONG + 16;
    const doc = (ids: readonly string[], x: number) => ids.map((id, i) => ({ id, x, y: h / 2 - 7 + (i - (ids.length - 1) / 2) * DONG, ten: "duoi" as const }));
    nguoi = [...doc(tra, 56), ...doc(nhan, w - 56)];
  } else {
    // The top row wears its names above, so its arrows leave downwards clear
    // of them; the paid and the bottom row wear theirs below, and the bottom
    // row stands a little further off so an arrow rising into the middle row
    // still has a shaft once it has cleared the name there.
    const tren = tra.filter((_, i) => i % 2 === 0);
    const duoi = tra.filter((_, i) => i % 2 === 1);
    const yTren = 50;
    const yGiua = yTren + DONG;
    const yDuoi = yGiua + DONG + 16;
    h = (duoi.length > 0 ? yDuoi : yGiua) + 46;
    nguoi = [...hang(tren, w, yTren, "tren"), ...hang(nhan, w, yGiua, "duoi"), ...hang(duoi, w, yDuoi, "duoi")];
  }
  const cho = new Map(nguoi.map((p) => [p.id, p]));
  const muiTen: MuiTen[] = [];
  for (const c of chuyen) {
    const a = cho.get(c.tu);
    const b = cho.get(c.toi);
    if (!a || !b || a === b) continue;
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const d = Math.hypot(dx, dy);
    const ux = dx / d;
    const uy = dy / d;
    // Across the columns, a slight bow to the left of travel, so A→B and B→A
    // never share a line; between rows, straight, so an arrow rising into a
    // name-wearing row arrives under the name, not through it.
    const cong = cot ? 0.12 : 0;
    const giua: Diem = [(a.x + b.x) / 2 - uy * d * cong, (a.y + b.y) / 2 + ux * d * cong];
    const huong = (p: HinhNhanSoDo, them: number): Diem => {
      const vx = giua[0] - p.x;
      const vy = giua[1] - p.y;
      const l = Math.hypot(vx, vy);
      // Through the side the name hangs on, the arrow starts or stops where
      // its ray leaves the name's band (72 wide, reaching 40 below the centre
      // or 46 above it), never inside it.
      const cx = vx / l;
      const cy = vy / l;
      const quaTen = p.ten === "duoi" ? cy > 0.35 : cy < -0.35;
      const raKhoiTen = Math.min((p.ten === "duoi" ? 42 : 48) / Math.abs(cy), Math.abs(cx) > 1e-6 ? 38 / Math.abs(cx) : Infinity);
      const r = (quaTen ? Math.max(R_NHAN, raKhoiTen) : R_NHAN) + them;
      return [p.x + cx * r, p.y + cy * r];
    };
    const p0 = huong(a, 0);
    const p1 = huong(b, 4);
    const tx = p1[0] - giua[0];
    const ty = p1[1] - giua[1];
    const tl = Math.hypot(tx, ty);
    const [ex, ey] = [tx / tl, ty / tl];
    const canh = (goc: number): Diem => [p1[0] - MUI * (ex * Math.cos(goc) - ey * Math.sin(goc)), p1[1] - MUI * (ex * Math.sin(goc) + ey * Math.cos(goc))];
    muiTen.push({ tu: c.tu, toi: c.toi, d: qCong(p0, giua, p1), dau: netGay([canh(0.5), p1, canh(-0.5)]), dai: doDai(diemTrenMui(p0, giua, p1)), mui: p1, dieuKhien: giua });
  }
  return { w, h, nguoi, muiTen };
}
