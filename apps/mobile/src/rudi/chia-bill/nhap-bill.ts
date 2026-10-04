/**
 * The bill being typed, kept per group until it is recorded or the person
 * drops it (QA UI-052).
 *
 * The flow's steps live in the screen's state, so the browser's Back left the
 * screen and Forward came back to an empty step 1; a reload did the same; and
 * «Nhập tay» from step 1 replaced a three-dish bill with a blank one. Every
 * one of those lost dishes without asking. The draft is kept here on every
 * change: on the web in `sessionStorage` (it survives Back, Forward and a
 * reload of the tab, and dies with the tab), on a phone in memory for the
 * app's life. Nothing in it is sent anywhere; the server only ever sees the
 * bill the person moves on with.
 */
const boNho = new Map<string, unknown>();
const khoa = (contextId: string) => `rudi.chia-bill.nhap.${contextId}`;

// Only a browser has `sessionStorage`; a phone, and the test runner, do not,
// and keep the memory copy alone. Reading it can throw where site data is blocked.
function kho(): Storage | null {
  try {
    return (globalThis as { sessionStorage?: Storage }).sessionStorage ?? null;
  } catch {
    return null;
  }
}

/**
 * Where a bill draft is kept: one per group, and one per group and trip when
 * the bill is written from a trip (ADR-0054), so a bill begun from one trip
 * never surfaces in another and lands in its ledger.
 */
export function khoaNhapBill(contextId: string, outingId?: string): string {
  if (outingId === undefined) return contextId;
  return `${contextId}:${outingId}`;
}

export function luuNhapBill(contextId: string, nhap: unknown): void {
  boNho.set(contextId, nhap);
  try {
    kho()?.setItem(khoa(contextId), JSON.stringify(nhap));
  } catch {
    /* A full or blocked store keeps the in-memory copy. */
  }
}

export function docNhapBill<T>(contextId: string): T | null {
  if (boNho.has(contextId)) return boNho.get(contextId) as T;
  try {
    const raw = kho()?.getItem(khoa(contextId));
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

export function boNhapBill(contextId: string): void {
  boNho.delete(contextId);
  try {
    kho()?.removeItem(khoa(contextId));
  } catch {
    /* Nothing to clear. */
  }
}

/** A bill worth keeping: at least one dish with a name or an amount. */
export function coMonDangGo(lines: readonly { name: string; lineTotalVnd: number }[]): boolean {
  return lines.some((l) => l.name.trim() !== "" || l.lineTotalVnd > 0);
}

/** How long a kept bill reopens where it was, rather than being offered from step 1. */
export const MO_LAI_TRONG_MS = 15 * 60_000;

/**
 * Where a kept bill reopens. A recent one -- a reload, Back then Forward --
 * at the step the person was on, nothing to tap; an older one from step 1,
 * where it is offered («Tiếp tục bill này») beside a fresh bill, so a split
 * left half-done hours ago does not open by itself on the next «Chia hoá đơn».
 */
export function buocMoLai<B>(nhap: { luc?: number; buoc?: B } | null, bayGio: number): B | null {
  if (!nhap?.buoc || nhap.luc === undefined) return null;
  return bayGio - nhap.luc <= MO_LAI_TRONG_MS ? nhap.buoc : null;
}
