/**
 * The dial's arithmetic (plan S0.5, `BanXoay`): a turn of the hand to a time
 * of day in quarter hours, or to one of a few levels. Pure, so the node gate
 * holds the snapping, the wrap and the text a screen reader reads.
 *
 * The dial is a clock face turned by a finger: 0 degrees at the top, growing
 * clockwise; one full turn is `tronVong` steps (a day of quarter hours is 96,
 * but a 24-hour face is hard to read, so time uses a 12-hour face turned twice:
 * the value keeps counting past a turn and the text says which half).
 */
export const PHUT_MOI_NAC = 15;
export const NAC_MOT_NGAY = (24 * 60) / PHUT_MOI_NAC;

/** Degrees (0 at the top, clockwise) of the pointer from the dial's centre to a touch. */
export function gocTuCham(tam: { x: number; y: number }, cham: { x: number; y: number }): number {
  const g = (Math.atan2(cham.x - tam.x, -(cham.y - tam.y)) * 180) / Math.PI;
  return (g + 360) % 360;
}

/** The step a pointer angle rests on, for a face of `soNac` steps per turn. */
export function nacTuGoc(goc: number, soNac: number): number {
  const buoc = 360 / soNac;
  return Math.round((((goc % 360) + 360) % 360) / buoc) % soNac;
}

export function gocTuNac(nac: number, soNac: number): number {
  return ((((nac % soNac) + soNac) % soNac) * 360) / soNac;
}

/**
 * Moving the hand from one step to the next on a face that is turned more than
 * once: the value follows the SHORTEST way round, so passing 12 goes on to 13
 * instead of jumping back to 0.
 */
export function nacTiep(hienTai: number, nacMoi: number, soNac: number): number {
  const cu = ((hienTai % soNac) + soNac) % soNac;
  let d = nacMoi - cu;
  if (d > soNac / 2) d -= soNac;
  if (d < -soNac / 2) d += soNac;
  return hienTai + d;
}

/** A time of day as quarter-hour steps from midnight, wrapped into one day. */
export function nacGio(phut: number): number {
  const n = Math.round(phut / PHUT_MOI_NAC);
  return ((n % NAC_MOT_NGAY) + NAC_MOT_NGAY) % NAC_MOT_NGAY;
}

const hai = (n: number) => (n < 10 ? `0${n}` : String(n));

/** «18:30» for a step count. */
export function gioTuNac(nac: number): string {
  const n = ((nac % NAC_MOT_NGAY) + NAC_MOT_NGAY) % NAC_MOT_NGAY;
  const phut = n * PHUT_MOI_NAC;
  return `${hai(Math.floor(phut / 60))}:${hai(phut % 60)}`;
}

/** «18:30» or «6:05» as minutes from midnight, or null. */
export function docGio(chu: string): number | null {
  const m = /^\s*(\d{1,2})[:h.](\d{2})\s*$/i.exec(chu);
  if (!m) return null;
  const gio = Number(m[1]);
  const phut = Number(m[2]);
  if (gio > 23 || phut > 59) return null;
  return gio * 60 + phut;
}

/** What a screen reader says for a time: «18 giờ 30». */
export function docGioThanhLoi(nac: number): string {
  const [g, p] = gioTuNac(nac).split(":").map(Number);
  return p === 0 ? `${g} giờ` : `${g} giờ ${p}`;
}

/** One step up or down, held inside [min, max] (the levels dial does not wrap). */
export function buocGioiHan(gia: number, huong: 1 | -1, min: number, max: number): number {
  return Math.max(min, Math.min(max, gia + huong));
}
