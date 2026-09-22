/**
 * Where on the edge Nếp may stand, in dp.
 *
 * The bottom of the RuDi shell is already taken: `ui/RudiTabBar.tsx` puts the
 * create stamp in the middle of the bar as a real column (`FAB = 56`), so a
 * floating assistant parked bottom-right in the usual web-chat position would
 * sit on the one control the tab bar cannot afford to lose. Nếp therefore rides
 * a VERTICAL rail on the right edge, and this module is the only place the rail
 * knows where it ends.
 *
 * Coordinates are the dock's TOP edge, not its centre, because that is what a
 * `translateY` on the container animates and what the clamp has to bound.
 *
 * Three rules worth pinning, all of them arithmetic that a screenshot cannot
 * settle: the rail stops a whole disc plus a margin above the tab bar; a window
 * too short for a rail collapses to a single legal point instead of inverting
 * (`duoi < tren` would make the clamp pick the wrong end and park Nếp under the
 * bar); and the stored position is a FRACTION, so rotating the phone or moving
 * to a tablet puts Nếp back where the person left it rather than at a pixel
 * offset that now means something else.
 *
 * Pure: no React, no Reanimated, no window. `NepDock.tsx` feeds it the measured
 * window and insets.
 */

/** The resting disc. 56dp matches the tab bar's own stamp, so they read as kin. */
export const NEP_DIA = 56;
/** The paper edge left showing when Nếp is tucked away. */
export const NEP_MEP_HEP = 6;
/** The same edge with a second sheet behind it: there is something waiting. */
export const NEP_MEP_DAY = 10;
/** How tall that edge is, so it stays a grabbable target at 6dp wide. */
export const NEP_MEP_CAO = 44;

export const LE_TREN = 16;
/** Bigger than the top margin: the tab bar is a control, the header is not. */
export const LE_DUOI = 24;

export interface KhungDock {
  /** Window height (dp). */
  cao: number;
  /** Everything occupied at the top: safe area plus any header. */
  dinh: number;
  /** Everything occupied at the bottom: tab bar plus safe area. */
  day: number;
}

export interface RayDoc {
  tren: number;
  duoi: number;
}

function so(n: number, mac: number): number {
  return Number.isFinite(n) ? n : mac;
}

export function rayDoc(khung: KhungDock): RayDoc {
  const cao = Math.max(0, so(khung.cao, 0));
  const dinh = Math.max(0, so(khung.dinh, 0));
  const day = Math.max(0, so(khung.day, 0));
  const tren = dinh + LE_TREN;
  const duoi = cao - day - LE_DUOI - NEP_DIA;
  // A short window (a phone on its side, a sheet fighting the IME) can leave no
  // rail at all. Pin to the top rather than hand back an inverted range.
  return duoi < tren ? { tren, duoi: tren } : { tren, duoi };
}

export function ghimVaoRay(y: number, ray: RayDoc): number {
  return Math.min(ray.duoi, Math.max(ray.tren, so(y, ray.tren)));
}

/** Position as 0..1 along the rail, which is what gets written to disk. */
export function tyLeTuY(y: number, ray: RayDoc): number {
  const doDai = ray.duoi - ray.tren;
  if (doDai <= 0) return 0;
  return (ghimVaoRay(y, ray) - ray.tren) / doDai;
}

/** The reverse, tolerant of whatever the disk hands back. */
export function yTuTyLe(tyLe: number, ray: RayDoc): number {
  const t = Math.min(1, Math.max(0, so(tyLe, 0)));
  return ray.tren + (ray.duoi - ray.tren) * t;
}

/** The whole notification vocabulary while Nếp is hidden: one sheet, or two. */
export function beRongMep(coViec: boolean): number {
  return coViec ? NEP_MEP_DAY : NEP_MEP_HEP;
}
