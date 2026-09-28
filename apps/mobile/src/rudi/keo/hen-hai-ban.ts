/**
 * «Hẹn của hai bạn»: the outings two-person sheets became, for the plan tab.
 *
 * A pair is never the current group (ADR-0021 §2.5), so the plan tab, which
 * lists the current group's outings, never showed the plan a couple had just
 * agreed on (QA 23/09: «Tờ lời rủ 26/09» existed only in the database). This
 * picks, across every pair the person is active in, the outings still ahead
 * -- soonest first, each with the other person's name -- and nothing else:
 * what already happened stays in that pair's own notebook.
 */
import { chiaKeo, type KeoCoNgay } from "./nhip-keo";

export interface DoiCuaToi {
  id: string;
  tenNguoiKia: string;
}

export interface HenHaiBan<T> {
  contextId: string;
  tenNguoiKia: string;
  keo: T;
}

export function henHaiBan<T extends KeoCoNgay>(
  doi: readonly DoiCuaToi[],
  keoTheoDoi: ReadonlyMap<string, readonly T[]>,
  today: string,
): HenHaiBan<T>[] {
  const tatCa: (T & { __doi: DoiCuaToi })[] = [];
  for (const d of doi) {
    for (const k of keoTheoDoi.get(d.id) ?? []) tatCa.push({ ...k, __doi: d });
  }
  return chiaKeo(tatCa, today).sapToi.map(({ __doi, ...keo }) => ({
    contextId: __doi.id,
    tenNguoiKia: __doi.tenNguoiKia,
    keo: keo as unknown as T,
  }));
}
