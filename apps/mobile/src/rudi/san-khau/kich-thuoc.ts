/**
 * How big a paper stage stands at the head of a screen (ADR-0037 D1, plan S0.3).
 *
 * Pure: the window and the room the column has go in, a size comes out, so
 * the node tests measure the boundaries instead of eyeballing them. The stage
 * keeps its frame's proportions -- the art is never stretched -- and takes
 * the smaller of "the column's width" and "the height this window can spare".
 *
 *   - compact (phone): about a quarter of the window's height, 160-220 dp;
 *   - medium (small tablet, foldable): 220-260 dp;
 *   - expanded: 240-300 dp, the left page of a spread when the screen has one;
 *   - short (a phone on its side, an IME fighting a sheet): no stage at all --
 *     the room is the content's, and the screen's words carry it alone;
 *   - large system text: a quarter smaller, and the art's `gon` reading, so
 *     the words the person enlarged are not pushed below the fold by a drawing.
 */
import { chuLon, type HeightClass, type SizeClass } from "../adaptive";

export interface KichThuocSanKhau {
  w: number;
  h: number;
  /** Draw the art's compact reading (fewer props, same stage). */
  gon: boolean;
}

export interface DauVaoKichThuoc {
  /** The width the stage may take (the content column, after gutters). */
  rongCho: number;
  /** The window's height (dp). */
  caoCuaSo: number;
  /** The stage frame's width over its height. */
  tiLe: number;
  sizeClass: SizeClass;
  heightClass: HeightClass;
  fontScale: number;
}

const DAI_CAO: Record<SizeClass, { phan: number; min: number; max: number }> = {
  compact: { phan: 0.26, min: 160, max: 220 },
  medium: { phan: 0.26, min: 220, max: 260 },
  expanded: { phan: 0.3, min: 240, max: 300 },
};

/** Big text takes this much off the stage's height. */
const CHU_LON_BOT = 0.75;

/** The stage's size for this window, or `null` when there is no room for one. */
export function kichThuocSanKhau(vao: DauVaoKichThuoc): KichThuocSanKhau | null {
  const { rongCho, caoCuaSo, tiLe, sizeClass, heightClass, fontScale } = vao;
  if (heightClass === "short") return null;
  if (!Number.isFinite(rongCho) || rongCho <= 0 || !Number.isFinite(tiLe) || tiLe <= 0) return null;
  const dai = DAI_CAO[sizeClass];
  const lon = chuLon(fontScale);
  const theoCuaSo = Number.isFinite(caoCuaSo) ? caoCuaSo * dai.phan : dai.max;
  let caoToiDa = Math.min(dai.max, Math.max(dai.min, theoCuaSo));
  if (lon) caoToiDa *= CHU_LON_BOT;
  // Whole dp, and one rounding only on the side that is not the binding one,
  // so a height-capped stage is exactly at its cap rather than a dp under it.
  if (rongCho / tiLe <= caoToiDa) {
    const w = Math.floor(rongCho);
    return { w, h: Math.round(w / tiLe), gon: lon };
  }
  const h = Math.floor(caoToiDa);
  return { w: Math.min(Math.floor(rongCho), Math.round(h * tiLe)), h, gon: lon };
}

/**
 * The room Nếp takes in a layout when it performs (ADR-0037 D5): its own slot,
 * never laid over content. 128 dp on a tablet, 112 on a phone, 88 with large
 * system text (the words come first), and none on a window too short to spare
 * it -- a phone on its side keeps its room for what the person is doing.
 */
export function kichThuocNepDien(vao: { sizeClass: SizeClass; heightClass: HeightClass; fontScale: number }): number {
  if (vao.heightClass === "short") return 0;
  if (chuLon(vao.fontScale)) return 88;
  return vao.sizeClass === "compact" ? 112 : 128;
}
