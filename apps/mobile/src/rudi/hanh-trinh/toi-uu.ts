/** Nearest-neighbour path: keep the first stop, greedily pick the closest remaining. */

import { haversineMet, type ToaDo } from "./khoang";

export type MocToiUu = ToaDo & { id: string };

export function toiUuGanNhat(mocs: readonly MocToiUu[]): string[] {
  if (mocs.length === 0) return [];
  const con = mocs.slice(1);
  const ra = [mocs[0].id];
  let tai = mocs[0];
  while (con.length > 0) {
    let tot = 0;
    let metTot = haversineMet(tai, con[0]);
    for (let i = 1; i < con.length; i++) {
      const met = haversineMet(tai, con[i]);
      if (met < metTot) {
        tot = i;
        metTot = met;
      }
    }
    tai = con[tot];
    ra.push(tai.id);
    con.splice(tot, 1);
  }
  return ra;
}
