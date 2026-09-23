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
 * settle: the rail stops a whole slip plus a margin above the tab bar; a window
 * too short for a rail collapses to a single legal point instead of inverting
 * (`duoi < tren` would make the clamp pick the wrong end and park Nếp under the
 * bar); and the stored position is a FRACTION, so rotating the phone or moving
 * to a tablet puts Nếp back where the person left it rather than at a pixel
 * offset that now means something else.
 *
 * Pure: no React, no Reanimated, no window. `NepDock.tsx` feeds it the measured
 * window and insets.
 */

/** How much of the slip shows when Nếp is out. 56dp matches the tab bar's own stamp. */
export const NEP_DIA = 56;
/** The slip is a little taller than it is wide: a slip, not a coin. */
export const NEP_TO_CAO = 64;

/**
 * The page's right margin, and the one number everything tucked must fit in.
 *
 * `space.md`, the gutter every RuDi screen keeps. It is not a comfortable
 * guess: on the running app (23/09) the smallest right margin of any TEXT was
 * exactly this -- message times in a conversation end 16dp from the edge --
 * while Explore's cards stop their text near 25dp. So a resting Nếp may use
 * the margin and nothing more, and neither may the area that answers a tap:
 * a hit area 24dp into the page is what caught the «Đồng ý» of an invitation
 * sitting against the right gutter (docs/claude/2026-09-23, flow 25).
 */
export const LE_TRANG = 16;
/** The slip's edge when Nếp is tucked away. */
export const NEP_MEP_HEP = 10;
/** A second slip, tucked behind, shows this much more of itself. */
export const TO_SAU_LO = 4;
/** The same edge with the second slip behind it: there is something waiting. */
export const NEP_MEP_DAY = NEP_MEP_HEP + TO_SAU_LO;

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
  const duoi = cao - day - LE_DUOI - NEP_TO_CAO;
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

/**
 * How far the tap area reaches LEFT of the slip, into the page.
 *
 * Tucked, the slip is a 10dp target, so it borrows the rest of the margin and
 * stops there: the tap area ends where the page's own content may begin. Out,
 * it borrows nothing -- the slip is already 56dp wide, and every dp of slop
 * past it lands on somebody's button.
 */
export function slopTrai(dangAn: boolean): number {
  return dangAn ? LE_TRANG - NEP_MEP_HEP : 0;
}
