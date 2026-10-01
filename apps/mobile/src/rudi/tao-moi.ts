/**
 * What the «Tạo mới» desk offers first, from the tab it was opened on.
 *
 * The desk had one order for everyone (a pair saw the pair's entry first), and
 * it opened only from Lên plan. With the stamp on every tab, the desk knows
 * where the person was standing: on Cộng đồng (Khám phá's second section, a
 * tab route with no column, still `community` here) the thing to make is a
 * post, on Lên plan an outing, on Cá nhân a memory. The first item lies across the desk;
 * the rest keep one stable order beneath it, so nothing is hidden and nothing
 * moves except the one card that fits the place.
 */
import type { VatBan } from "./art/vat-ban";

export const VIEC_HEN = "/outings/new";
export const VIEC_BILL = "/smart-split/xom-leo/review";
export const VIEC_KY_NIEM = "/moments/new";
export const VIEC_STORY = "/stories/new";
export const VIEC_CAP = "/hai-nguoi/chon-nguoi";
export const VIEC_BAI = "/community/new";

/** One line per action; a second line only where two of them could be confused. */
export type ViecTao = { vat: VatBan; title: string; detail?: string; href: string };

export type TabTao = "community" | "explore" | "plan" | "messages" | "profile";

const TAB_TAO: readonly string[] = ["community", "explore", "plan", "messages", "profile"];

/** `?tu=` as the desk reads it: a known tab, or nothing. */
export function tabTu(raw: unknown): TabTao | null {
  return typeof raw === "string" && TAB_TAO.includes(raw) ? (raw as TabTao) : null;
}

/** The action that fits the tab; `null` when the desk keeps its usual order. */
function viecHop(tu: TabTao | null, coCap: boolean): string | null {
  switch (tu) {
    case "community":
      return VIEC_BAI;
    case "explore":
    case "plan":
      return VIEC_HEN;
    case "messages":
      return coCap ? VIEC_CAP : VIEC_STORY;
    case "profile":
      return VIEC_KY_NIEM;
    default:
      return coCap ? VIEC_CAP : null;
  }
}

/**
 * The desk's order for `viec` (the desk's own list, in its stable order).
 * `coCap`: the person has an active one-to-one chat, so the pair's entry
 * exists for them as a first choice. `coCongDong`: Cộng đồng is on, so
 * «Viết bài» has somewhere to go.
 */
export function thuTuViec<T extends { href: string }>({ viec, tu, coCap, coCongDong }: { viec: readonly T[]; tu: TabTao | null; coCap: boolean; coCongDong: boolean }): { viec: T[]; hopCho: boolean } {
  const co = viec.filter((v) => v.href !== VIEC_BAI || coCongDong);
  const dau = viecHop(tu, coCap);
  const truoc = co.find((v) => v.href === dau);
  if (!truoc) return { viec: [...co], hopCho: false };
  return { viec: [truoc, ...co.filter((v) => v !== truoc)], hopCho: tu !== null };
}
