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
import { CHAN_NEP, hinhGhe, hinhNep } from "./nep";
import { duongChuyen, gocGap, vongHo } from "./motif";

/** Landscape, so the figure and the thing it points at can stand side by side. */
export const KHUNG_CANH = { w: 144, h: 112 } as const;

/** One floor for every scene: chair legs, the foot of a frame or a map, and Nếp's feet all end here. */
const SAN = 102;

/** Place the figure so its feet stand on the floor at `x0`. */
function nepTrenSan(pose: string, x0: number, tiLe: number): LopVe[] {
  return hinhNep(pose, { x0, y0: SAN - CHAN_NEP * tiLe, tiLe });
}

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
  // A seat pulled out, and the open ring on the floor around it. The chair's
  // left post stands at x 90, where the figure's hand lands.
  "chua-co-hoi": () => [...vongHo(106, 84, 24, { moTai: Math.PI, net: 3 }), ...hinhGhe(84, SAN - 64)],
  // The sheet the plan starts on: a folded corner, and the pen leaving the page.
  "chua-co-keo": () => [...gocGap(58, 30, 62, 46), ...duongChuyen(68, 30, 72, 40)],
  // An instant-film frame with nothing in it yet, leaning a little.
  // Its foot rests on the floor; its near edge runs down x 58..60.
  "chua-co-anh": () => {
    const khung: readonly (readonly [number, number])[] = [[58, 30], [122, 26], [124, 98], [60, SAN]];
    return [
      { d: daGiac(khung), mau: "giay" },
      { d: daGiac([[66, 38], [114, 35], [115, 80], [68, 83]]), mau: "bong" },
      { d: daGiac([[108, 27], [122, 26], [123, 39]]), mau: "gap" },
      { d: daGiac(khung), mau: "muc", net: 2.2 },
      { d: daGiac([[66, 38], [114, 35], [115, 80], [68, 83]]), mau: "muc", net: 1.6 },
    ];
  },
  // Two chairs at a small table, all on the floor; the ring marks the one
  // still free. The near chair's left post stands at x 58 for the figure's hand.
  "chua-co-ban": () => [
    ...vongHo(122, 88, 18, { moTai: Math.PI, net: 3 }),
    ...hinhGhe(54, SAN - 64 * 0.7, { tiLe: 0.7 }),
    { d: bau(93, 82, 11, 4), mau: "giay" },
    { d: bau(93, 82, 11, 4), mau: "muc", net: 2 },
    { d: netGay([[93, 86], [93, SAN - 2]]), mau: "muc", net: 2.2 },
    { d: bau(93, SAN - 1, 6, 2.2), mau: "muc", net: 2 },
    ...hinhGhe(108, SAN - 64 * 0.7, { tiLe: 0.7, lat: true }),
  ],
  // A folded map standing on the floor, its route ending in an open ring,
  // not at a pin. Its near edge runs down x 60..64.
  "tim-khong-ra": () => [
    { d: daGiac([[60, 36], [128, 30], [132, 96], [64, SAN]]), mau: "giay" },
    { d: daGiac([[60, 36], [128, 30], [132, 96], [64, SAN]]), mau: "muc", net: 2.2 },
    { d: netGay([[83, 34], [85, 98]]), mau: "muc", net: 1.4 },
    { d: netGay([[106, 32], [108, 96]]), mau: "muc", net: 1.4 },
    { d: qCong([70, 90], [80, 48], [106, 54]), mau: "muc", net: 2.4 },
    ...vongHo(116, 54, 8, { moTai: Math.PI * 0.75, moRong: 1.3, net: 2.4 }),
  ],
};

/**
 * The figure, placed so that each pose's contact points land on the prop:
 * `x0 + 96-box x * tiLe` equals the prop's edge. Feet on the floor, always.
 */
const NEP: Record<CanhId, () => LopVe[]> = {
  // Hand at box (90, 34) → scene (90, 56.4): the top of the chair's left post.
  "chua-co-hoi": () => nepTrenSan("keo-ghe", 18, 0.8),
  // The sheet floats; the figure writes beside it, off the floor line on purpose.
  "chua-co-keo": () => hinhNep("ghi-lai", { x0: -2, y0: 14, tiLe: 0.8 }),
  // Hands at box (74, 18) and (76, 70) → scene x 58.2 and 59.8: the frame's near edge,
  // a sliver of which stays visible between the body and the frame.
  "chua-co-anh": () => nepTrenSan("giu-khung", -1, 0.8),
  // Hand at box (88, 48) → scene (58, 70): the top of the near chair's left post.
  "chua-co-ban": () => nepTrenSan("giu-cho", -8, 0.75),
  // Hands at box (75, 26) and (78, 66) → scene x 61 and 63.4: the map's near edge.
  "tim-khong-ra": () => nepTrenSan("cam-ban-do", 1, 0.8),
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
