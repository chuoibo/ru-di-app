/**
 * Five silences, each a small scene (report 07/09 §12.2): no group yet, no
 * outing yet, no photo yet, no friend yet, nothing found. They go into
 * `EmptyState`'s `illustration` slot, which has waited for artwork since the
 * slot was drawn.
 *
 * Every scene is complete without Nếp: a chair pulled out, a sheet the ink
 * starts on, an empty frame, a second seat, a map whose route has not landed.
 * Nếp is one layer on top, switched off with `nep: false`, so the same scene
 * can be compared with and without the figure on the same device before the
 * figure is used anywhere else (concept note 08/09, §hợp đồng 6).
 */
import { type LopVe, bau, daGiac, netGay, qCong } from "./net";
import { hinhGhe, hinhNep } from "./nep";
import { duongChuyen, gocGap, vongHo } from "./motif";

/** Landscape, so the figure and the thing it points at can stand side by side. */
export const KHUNG_CANH = { w: 144, h: 112 } as const;

export const CANH_IDS = ["chua-co-hoi", "chua-co-keo", "chua-co-anh", "chua-co-ban", "tim-khong-ra"] as const;
export type CanhId = (typeof CANH_IDS)[number];

/** What a screen reader hears for the scene; one sentence, no story. */
const MO_TA: Record<CanhId, string> = {
  "chua-co-hoi": "Một chiếc ghế được kéo ra, chừa sẵn chỗ",
  "chua-co-keo": "Một tờ hẹn trống, nét mực bắt đầu từ đó",
  "chua-co-anh": "Một khung ảnh còn trống, góc giấy gấp",
  "chua-co-ban": "Hai chiếc ghế, một chỗ còn trống",
  "tim-khong-ra": "Một tấm bản đồ gấp, đường đi chưa tới nơi",
};

export function laCanhId(id: string): id is CanhId {
  return (CANH_IDS as readonly string[]).includes(id);
}

/** The first scene stands in for an id this build has no scene for. */
const CANH_MAC_DINH: CanhId = "chua-co-hoi";

function canhHopLe(id: string): CanhId {
  if (laCanhId(id)) return id;
  return CANH_MAC_DINH;
}

export function moTaCanh(id: string): string {
  return MO_TA[canhHopLe(id)];
}

const NEN: Record<CanhId, () => LopVe[]> = {
  // A seat pulled out, and the open ring on the floor around it.
  "chua-co-hoi": () => [...vongHo(108, 84, 24, { moTai: Math.PI, net: 3 }), ...hinhGhe(86, 38)],
  // The sheet the plan starts on: a folded corner, and the pen leaving the page.
  "chua-co-keo": () => [...gocGap(58, 30, 62, 46), ...duongChuyen(68, 30, 72, 40)],
  // An instant-film frame with nothing in it yet, leaning a little.
  "chua-co-anh": () => {
    const khung: readonly (readonly [number, number])[] = [[58, 24], [122, 20], [124, 92], [60, 96]];
    return [
      { d: daGiac(khung), mau: "giay" },
      { d: daGiac([[66, 32], [114, 29], [115, 74], [68, 77]]), mau: "bong" },
      { d: daGiac([[108, 21], [122, 20], [123, 33]]), mau: "gap" },
      { d: daGiac(khung), mau: "muc", net: 2.2 },
      { d: daGiac([[66, 32], [114, 29], [115, 74], [68, 77]]), mau: "muc", net: 1.6 },
    ];
  },
  // Two chairs; the ring marks the one still free.
  "chua-co-ban": () => [
    ...vongHo(118, 86, 22, { moTai: Math.PI, net: 3 }),
    ...hinhGhe(6, 44, { tiLe: 0.9 }),
    { d: bau(72, 78, 15, 5), mau: "giay" },
    { d: bau(72, 78, 15, 5), mau: "muc", net: 2.2 },
    { d: netGay([[72, 83], [72, 98]]), mau: "muc", net: 2.4 },
    { d: bau(72, 99, 8, 2.6), mau: "muc", net: 2.2 },
    ...hinhGhe(98, 44, { tiLe: 0.9, lat: true }),
  ],
  // A folded map whose route ends in an open ring, not at a pin.
  "tim-khong-ra": () => [
    { d: daGiac([[52, 30], [126, 24], [130, 84], [56, 90]]), mau: "giay" },
    { d: daGiac([[52, 30], [126, 24], [130, 84], [56, 90]]), mau: "muc", net: 2.2 },
    { d: netGay([[78, 28], [80, 88]]), mau: "muc", net: 1.4 },
    { d: netGay([[104, 26], [106, 86]]), mau: "muc", net: 1.4 },
    { d: qCong([62, 80], [72, 40], [100, 44]), mau: "muc", net: 2.4 },
    ...vongHo(110, 44, 8, { moTai: Math.PI * 0.75, moRong: 1.3, net: 2.4 }),
  ],
};

const NEP: Record<CanhId, () => LopVe[]> = {
  "chua-co-hoi": () => hinhNep("moi", { x0: 0, y0: 6, tiLe: 0.8 }),
  "chua-co-keo": () => hinhNep("ghi-lai", { x0: -2, y0: 14, tiLe: 0.8 }),
  "chua-co-anh": () => hinhNep("moi", { x0: 0, y0: 16, tiLe: 0.8 }),
  "chua-co-ban": () => hinhNep("moi", { x0: -6, y0: 0, tiLe: 0.7 }),
  "tim-khong-ra": () => hinhNep("moi", { x0: -4, y0: 18, tiLe: 0.75 }),
};

/**
 * The horizontal extent of a layer list, read from every x in its paths, so a
 * scene without the figure can be framed to its props instead of leaving the
 * figure's empty slot as an indent (finish review 08/09).
 */
export function hopNgang(lop: readonly LopVe[]): { x0: number; x1: number } {
  let x0 = Infinity, x1 = -Infinity;
  for (const l of lop) {
    let i = 0;
    for (const t of l.d.split(/\s+/)) {
      if (/^[A-Za-z]$/.test(t)) { i = 0; continue; }
      const n = Number(t);
      if (Number.isNaN(n)) continue;
      if (i % 2 === 0) { if (n < x0) x0 = n; if (n > x1) x1 = n; }
      i += 1;
    }
  }
  return Number.isFinite(x0) ? { x0, x1 } : { x0: 0, x1: KHUNG_CANH.w };
}

export interface TuyChonCanh {
  /** Draw the figure; off, the scene is the props alone. */
  nep?: boolean;
}

/** The layers of one scene, back to front. An unknown id draws the first scene. */
export function hinhCanh(id: string, tuyChon: TuyChonCanh = {}): LopVe[] {
  const { nep = true } = tuyChon;
  const canh = canhHopLe(id);
  return nep ? [...NEN[canh](), ...NEP[canh]()] : NEN[canh]();
}
