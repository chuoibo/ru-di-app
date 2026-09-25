/**
 * Avatars that failed to load this session (ADR-0037 D6, plan S0.5).
 *
 * Every avatar in a roster asks the server for a picture; a person who never
 * uploaded one answers 404, and a list of forty people would ask forty times
 * on every render and draw forty broken frames before the initials. The first
 * failure is remembered here, so the rest of the session draws that person's
 * ink and initial straight away. Bounded, oldest forgotten first; a new upload
 * by the viewer clears their own entry (`quenAnhHong(id)`).
 */
export const TRAN_ANH_HONG = 500;

const anhHong = new Set<string>();

export function danhDauAnhHong(personId: string): void {
  anhHong.delete(personId);
  anhHong.add(personId);
  while (anhHong.size > TRAN_ANH_HONG) {
    const cu = anhHong.values().next().value;
    if (cu === undefined) break;
    anhHong.delete(cu);
  }
}

export function anhDaHong(personId: string): boolean {
  return anhHong.has(personId);
}

/** Forget one person's failure (they just uploaded), or all of them. */
export function quenAnhHong(personId?: string): void {
  if (personId === undefined) anhHong.clear();
  else anhHong.delete(personId);
}
