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
 *   - `ra`, whether the person pulled Nếp out of the edge. That is a choice,
 *     and a choice the app forgets is a choice the app overrules.
 *
 * v1 stored `an` («tucked away») because Nếp used to start out. It now starts
 * tucked (`trang-thai.ts`), and under that default a v1 `an: false` no longer
 * means anything the person chose: it is what every install that never
 * touched Nếp wrote. Reading it as «pulled out» would put the old 57dp disc
 * back over the text of every such install, so v1 is not read at all.
 * `coViec` is the server's truth, `luiLai` belongs to whichever screen is open,
 * and `mo` is a panel nobody asked to have reopened three days later. A restart
 * that reopened the assistant over the screen the person launched into would be
 * the app talking first.
 *
 * Reads are total: a corrupt blob means «nothing chosen yet», never a throw.
 * This is `rudi.phien.v1`'s lesson, where one bad index crashed every launch.
 */

export const KHOA_DOCK = "rudi.nep.dock.v2";

export interface DockDaLuu {
  /** 0 at the top of the rail, 1 at the bottom. */
  tyLe: number;
  ra: boolean;
}

export function maHoaDock(dock: { tyLe: number; ra: boolean }): string {
  const tyLe = Number.isFinite(dock.tyLe) ? Math.min(1, Math.max(0, dock.tyLe)) : 0;
  // Written field by field rather than by spreading the caller's object: a
  // spread would quietly carry `coViec`, `luiLai` and `trangThai` to disk the
  // moment somebody passes the whole dock state, which is exactly the call
  // shape that is convenient to write.
  return JSON.stringify({ tyLe, ra: dock.ra === true });
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
  if (typeof o.ra !== "boolean") return null;
  return { tyLe: Math.min(1, Math.max(0, o.tyLe)), ra: o.ra };
}
