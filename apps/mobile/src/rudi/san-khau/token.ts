/**
 * The paper-stage tokens (ADR-0037 D2, D13), read from `tokens.json`.
 *
 * Paper height replaces the old «printed into the page / pasted onto it» pair:
 *
 *   0  printed on the page -- ink, stamps, rows, keylines. No shadow.
 *   1  pasted on the page  -- prints, tickets, receipts, stubs. Today's card shadow.
 *   2  standing up         -- pop-up layers and person standees. The shadow grows
 *                             with the fold angle and is zero when the layer lies flat.
 *   3  lifted              -- the object under a finger, a sheet. Larger and softer.
 *
 * One light for the whole app, from the top left; in the dark scheme it is a desk
 * lamp. These values are never mirrored into `guest.css`: the guest page draws no
 * stage, and the Python legacy tree does not change a byte for this campaign.
 *
 * Pure: no React Native, so node tests read exactly what ships.
 */
import tokens from "../../../../../packages/shared/tokens.json";

export type CaoGiay = 0 | 1 | 2 | 3;

export interface MauSanKhau {
  /** Shadow colour; the alpha comes from the paper height. */
  bong: string;
  /** Scene light (day) or desk lamp (dark), always used at a low alpha. */
  anhSang: string;
  /** The visible thickness of a cut sheet standing up. */
  mepGiay: string;
}

export interface BongCao {
  mau: string;
  dy: number;
  blur: number;
  alpha: number;
}

export function mauSanKhau(dark: boolean): MauSanKhau {
  return dark ? tokens.sanKhau.dark : tokens.sanKhau.light;
}

/**
 * The shadow of a sheet at paper height `cao`, or `null` at height 0: what is
 * printed into the page never casts one. `goc` is the fold angle in degrees for a
 * standing layer (0 = upright, 90 = lying flat); it scales height-2 shadows so a
 * layer rising out of the page grows its shadow with it.
 */
export function bongCao(cao: CaoGiay, dark: boolean, goc = 0): BongCao | null {
  if (cao === 0) return null;
  const muc = tokens.sanKhau.cao[String(cao) as "1" | "2" | "3"];
  const mau = mauSanKhau(dark).bong;
  const dung = cao === 2 ? Math.max(0, Math.min(1, Math.cos((Math.max(0, Math.min(90, goc)) * Math.PI) / 180))) : 1;
  // A shadow on the dark cloth needs more alpha to be seen at all. 1.6 is a
  // starting value, checked on the dark captures of each slice, not a measurement.
  const nhan = dark ? 1.6 : 1;
  return { mau, dy: muc.dy * dung, blur: muc.blur, alpha: Math.min(0.5, muc.alpha * nhan * dung) };
}
