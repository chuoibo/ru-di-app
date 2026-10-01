/**
 * The tab strip's columns, worked out away from React so a test can hold them.
 *
 * Cộng đồng is a route of the tab navigator — it keeps the strip on screen,
 * its own URL and its deep links — but it has no column: it is the second
 * section of Khám phá, and while it is open the Khám phá column is the lit
 * one (owner's mockup, 01/10). `app/(tabs)/_layout.tsx` marks it
 * `href: null`; this map names the column that hosts it, and a test holds the
 * two together. The guide's extractor (`tools/rut-huong-dan.mjs`) reads this
 * map too, so Nếp knows the route is two taps from the other tabs.
 */
export const MUC_TRONG_TAB: Readonly<Record<string, string | undefined>> = { community: "explore" };

export type ThanhTab = {
  /** Route names that get a column, in layout order. */
  cot: string[];
  /** Slot of the «Tạo» stamp among the strip's slots; 0 on the rail. */
  viTriDau: number;
  /** Slots on the strip: the columns plus the stamp's. */
  soCot: number;
  /** Index into `cot` of the lit column; -1 when the open route has none. */
  cotChon: number;
};

export function xepThanh(tenRoute: readonly string[], dangMo: string, rail: boolean): ThanhTab {
  const cot = tenRoute.filter((ten) => MUC_TRONG_TAB[ten] === undefined);
  return {
    cot,
    // An odd slot count has a middle: four columns put the stamp third of five.
    viTriDau: rail ? 0 : Math.floor(cot.length / 2),
    soCot: cot.length + 1,
    cotChon: cot.indexOf(MUC_TRONG_TAB[dangMo] ?? dangMo),
  };
}

/** The strip slot of column `i`, counting the stamp's slot. */
export function oCuaCot(thanh: ThanhTab, i: number): number {
  return i >= thanh.viTriDau ? i + 1 : i;
}

/** Khám phá's two sections, by route name. */
export type MucKhamPha = "explore" | "community";

// Memory only: a fresh start of the app opens Địa điểm (owner's call, 01/10).
let mucCuoi: MucKhamPha = "explore";

/** Remember the section of Khám phá last in view. */
export function ghiMucKhamPha(muc: MucKhamPha): void {
  mucCuoi = muc;
}

/** The route a column opens: Khám phá reopens the section last in view. */
export function dichCuaCot(cot: string): string {
  return cot === "explore" ? mucCuoi : cot;
}
