/**
 * The three marks that tie the screens into one story (report 07/09 §6.3):
 *
 * - `vongHo`, the open ring: a table with one side left open. Decoration only;
 *   it is never a progress ring, so it is never animated, never paired with a
 *   percentage, and the gap stays wide.
 * - `duongChuyen`, the passed line: one ink stroke with a turn in it, joining
 *   things that belong together. When it means progress it carries a label.
 * - `gocGap`, the kept corner: a sheet with its top-right corner folded down,
 *   the same fold Nếp wears. Same corner, same angle, every time.
 *
 * All three are paths from `net.ts`: absolute M/L/C/Z only.
 */
import { duongCongS } from "../ui/duong-svg";
import { type LopVe, cungTron, daGiac, tron } from "./net";

export interface TuyChonVongHo {
  /** Centre of the gap, radians, y down; default top-right. */
  moTai?: number;
  /** Width of the gap, radians; default a little over 60 degrees. */
  moRong?: number;
  net?: number;
}

/** An open ring around (cx, cy). One coral stroke, nothing else. */
export function vongHo(cx: number, cy: number, r: number, tuyChon: TuyChonVongHo = {}): LopVe[] {
  const { moTai = -0.6, moRong = 1.15, net = 3.5 } = tuyChon;
  const tu = moTai + moRong / 2;
  const den = moTai - moRong / 2 + 2 * Math.PI;
  return [{ d: cungTron(cx, cy, r, tu, den), mau: "gap", net }];
}

/**
 * A passed line across a `w`×`h` box at (x0, y0): the kit's S-curve with the
 * pen's first touch marked in coral. Reads left to right, top to bottom.
 */
export function duongChuyen(x0: number, y0: number, w: number, h: number, huong: "down" | "up" = "down", net = 3): LopVe[] {
  const { d, diem } = duongCongS(w, h, huong);
  const dau = diem(0);
  // `duongCongS` draws in its own frame; the box is translated by prefixing
  // nothing and instead re-emitting the curve with the offset applied.
  const dich = d.replace(/(-?\d+(?:\.\d+)?) (-?\d+(?:\.\d+)?)/g, (_m, x: string, y: string) => `${so(Number(x) + x0)} ${so(Number(y) + y0)}`);
  return [
    { d: dich, mau: "muc", net },
    { d: tron(x0 + dau.x, y0 + dau.y, net * 1.1), mau: "gap" },
  ];
}

/** Plain decimal, the same rule as `net.ts`. */
function so(n: number): string {
  const s = n.toFixed(2).replace(/\.?0+$/, "");
  return s === "-0" ? "0" : s;
}

/** A sheet at (x, y), `w`×`h`, with its top-right corner folded down. */
export function gocGap(x: number, y: number, w: number, h: number, net = 2, gocTiLe = 0.28): LopVe[] {
  const c = Math.min(w, h) * gocTiLe;
  const than = daGiac([
    [x, y],
    [x + w - c, y],
    [x + w, y + c],
    [x + w, y + h],
    [x, y + h],
  ]);
  const gap = daGiac([
    [x + w - c, y],
    [x + w - c, y + c],
    [x + w, y + c],
  ]);
  return [
    { d: than, mau: "giay" },
    { d: gap, mau: "gap" },
    { d: than, mau: "muc", net },
    { d: gap, mau: "muc", net: net * 0.8 },
  ];
}
