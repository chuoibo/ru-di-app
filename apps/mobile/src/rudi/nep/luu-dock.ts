/**
 * What survives a restart about where Nếp stands, and nothing more.
 *
 * Same split as `luu-tru.ts` beside `kho.ts`: every decision that can be WRONG
 * lives here, with no native import, so it runs under bare node against
 * deliberately corrupt input. `NepProvider.tsx` holds the AsyncStorage calls.
 *
 * Only two facts belong on disk, and the exclusions are the point:
 *   - `tyLe`, the position along the rail as 0..1, so the same spot survives a
 *     rotation or a different phone;
 *   - `an`, whether the person tucked Nếp away. That is a choice, and a choice
 *     the app forgets is a choice the app overrules.
 * `coViec` is the server's truth, `luiLai` belongs to whichever screen is open,
 * and `mo` is a panel nobody asked to have reopened three days later. A restart
 * that reopened the assistant over the screen the person launched into would be
 * the app talking first.
 *
 * Reads are total: a corrupt blob means «nothing chosen yet», never a throw.
 * This is `rudi.phien.v1`'s lesson, where one bad index crashed every launch.
 */

export const KHOA_DOCK = "rudi.nep.dock.v1";

export interface DockDaLuu {
  /** 0 at the top of the rail, 1 at the bottom. */
  tyLe: number;
  an: boolean;
}

export function maHoaDock(dock: { tyLe: number; an: boolean }): string {
  const tyLe = Number.isFinite(dock.tyLe) ? Math.min(1, Math.max(0, dock.tyLe)) : 0;
  // Written field by field rather than by spreading the caller's object: a
  // spread would quietly carry `coViec`, `luiLai` and `trangThai` to disk the
  // moment somebody passes the whole dock state, which is exactly the call
  // shape that is convenient to write.
  return JSON.stringify({ tyLe, an: dock.an === true });
}

export function giaiMaDock(raw: string | null): DockDaLuu | null {
  if (!raw) return null;
  let doc: unknown;
  try {
    doc = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!doc || typeof doc !== "object" || Array.isArray(doc)) return null;
  const o = doc as Record<string, unknown>;
  // A record missing either field is a record from a format we do not know.
  // Guessing the other half is how a half-written blob becomes a wrong answer
  // that looks deliberate.
  if (typeof o.tyLe !== "number" || !Number.isFinite(o.tyLe)) return null;
  if (typeof o.an !== "boolean") return null;
  return { tyLe: Math.min(1, Math.max(0, o.tyLe)), an: o.an };
}
