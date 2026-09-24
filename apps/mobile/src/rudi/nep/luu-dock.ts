/**
 * What survives a restart about where Nếp stands, and nothing more.
 *
 * Same split as `luu-tru.ts` beside `kho.ts`: every decision that can be WRONG
 * lives here, with no native import, so it runs under bare node against
 * deliberately corrupt input. `NepProvider.tsx` holds the AsyncStorage calls.
 *
 * Only one fact belongs on disk: `tyLe`, the position along the rail as 0..1,
 * so the same spot survives a rotation or a different phone.
 *
 * Whether Nếp is pulled out is NOT stored. v2 stored it (`ra`), on the
 * reasoning that a choice the app forgets is a choice it overrules. But a
 * pulled-out Nếp is 56dp on a page whose text runs to a 16dp margin, and on
 * every launch after one tap it was the resting state: measured 24/09 on the
 * web build, it lay over «200.000đ» and «22:30» on Explore. Within a session
 * the choice holds, across screens and money screens alike (ADR-0033 §2.4);
 * a launch starts tucked, as `trang-thai.ts` promises. v1 (`an`) and v2
 * (`ra`) are not read, so neither can put Nếp back over the page.
 * `coViec` is the server's truth, `luiLai` belongs to whichever screen is open,
 * and `mo` is a panel nobody asked to have reopened three days later. A restart
 * that reopened the assistant over the screen the person launched into would be
 * the app talking first.
 *
 * Reads are total: a corrupt blob means «nothing chosen yet», never a throw.
 * This is `rudi.phien.v1`'s lesson, where one bad index crashed every launch.
 */

export const KHOA_DOCK = "rudi.nep.dock.v3";

export interface DockDaLuu {
  /** 0 at the top of the rail, 1 at the bottom. */
  tyLe: number;
}

export function maHoaDock(dock: { tyLe: number }): string {
  const tyLe = Number.isFinite(dock.tyLe) ? Math.min(1, Math.max(0, dock.tyLe)) : 0;
  // Written field by field rather than by spreading the caller's object: a
  // spread would quietly carry `coViec`, `luiLai` and `trangThai` to disk the
  // moment somebody passes the whole dock state, which is exactly the call
  // shape that is convenient to write.
  return JSON.stringify({ tyLe });
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
  if (typeof o.tyLe !== "number" || !Number.isFinite(o.tyLe)) return null;
  return { tyLe: Math.min(1, Math.max(0, o.tyLe)) };
}
