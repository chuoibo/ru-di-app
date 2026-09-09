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
import { type Diem, type LopVe, bau, daGiac, khungBo, netGay, qCong, tron } from "./net";
import { CHAN_NEP, hinhGhe, hinhNep, type PoseNep, type TuyChonNep } from "./nep";
import { duongChuyen, gocGap, vongHo } from "./motif";

/** Landscape, so the figure and the thing it points at can stand side by side. */
export const KHUNG_CANH = { w: 144, h: 112 } as const;

/** One floor for every scene: chair legs, the foot of a frame or a map, and Nếp's feet all end here. */
const SAN = 102;

export const CANH_IDS = [
  "chua-co-hoi",
  "chua-co-keo",
  "chua-co-anh",
  "chua-co-ban",
  "tim-khong-ra",
  // 09/09: every silence in the shell gets a scene of its own, so «nothing
  // here» stops being one blank shape repeated on forty screens.
  "chua-doc-duoc",
  "chua-co-tin-nhan",
  "chua-co-loi-moi",
  "chua-co-ky-niem",
  "bo-loc-che-het",
] as const;
export type CanhId = (typeof CANH_IDS)[number];

/**
 * Scenes drawn WITHOUT the figure, whatever the caller asks for.
 *
 * «Luật Nếp Đứng Xa Tiền» (DESIGN.md): the character never stands beside an
 * error, a conflict or money. A failure state is exactly that, and it reaches
 * roughly twenty screens including the ledger ones, so the rule is enforced
 * here rather than left to every caller to remember `nep={false}`.
 */
export const CANH_KHONG_NEP: ReadonlySet<CanhId> = new Set(["chua-doc-duoc"]);

/** What a screen reader hears for the scene; one sentence, no story. */
const MO_TA: Record<CanhId, string> = {
  "chua-co-hoi": "Một chiếc ghế được kéo ra, chừa sẵn chỗ",
  "chua-co-keo": "Một tờ hẹn trống, nét mực bắt đầu từ đó",
  "chua-co-anh": "Một khung ảnh còn trống, góc giấy gấp",
  "chua-co-ban": "Hai chiếc ghế, một chỗ còn trống",
  "tim-khong-ra": "Một tấm bản đồ gấp, đường đi chưa tới nơi",
  "chua-doc-duoc": "Một tờ giấy rách, mảnh rách trượt sang bên",
  "chua-co-tin-nhan": "Một bong bóng thoại còn trống, vừa được mở lời",
  "chua-co-loi-moi": "Một phong thư còn nguyên, chưa có ai gửi đi",
  "chua-co-ky-niem": "Một sợi dây phơi ảnh, hai chiếc kẹp còn trống",
  "bo-loc-che-het": "Một tấm lưới che gần kín, còn một ô để nhìn qua",
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
  // The server did not answer: a sheet torn, the torn-off corner slid a little
  // to the side, and the coral ON the tear. No ring: the open ring under the
  // sheet read as a loading spinner standing still (audit 09/09, F45d), and a
  // failure has to look finished, not pending. No figure here, ever
  // (`CANH_KHONG_NEP`).
  "chua-doc-duoc": () => {
    const to: readonly Diem[] = [[34, 24], [104, 20], [106, 46], [90, 48], [84, 56], [92, 62], [88, 72], [36, 76]];
    const manh: readonly Diem[] = [[96, 50], [112, 48], [114, 78], [98, 80], [94, 66], [102, 60], [96, 56]];
    const rach: readonly Diem[] = [[90, 48], [84, 56], [92, 62], [88, 72]];
    const rachManh: readonly Diem[] = [[94, 66], [102, 60], [96, 56]];
    return [
      { d: daGiac(to), mau: "giay" },
      { d: daGiac(to), mau: "muc", net: 2.2 },
      { d: netGay([[46, 36], [86, 34]]), mau: "bong", net: 3 },
      { d: netGay([[46, 48], [72, 47]]), mau: "bong", net: 3 },
      { d: netGay([[46, 60], [76, 59]]), mau: "bong", net: 3 },
      { d: netGay(rach), mau: "gap", net: 3 },
      { d: daGiac(manh), mau: "giay" },
      { d: daGiac(manh), mau: "muc", net: 2.2 },
      { d: netGay(rachManh), mau: "gap", net: 3 },
    ];
  },
  // No message yet: an empty speech bubble whose tail lands ON THE MOUTH of the
  // figure calling out (`goi-loi` at x0 -4, y0 16, scale 0.78: the mouth is at
  // scene (30..41, 57..61), the tail tip at (41, 55)). Drawn before the figure,
  // so the face covers the tip and the bubble is seen to come out of it. It
  // used to sit beside a writing body -- the same body `chua-co-keo` writes
  // with -- and read as a second plan sheet (audit 09/09, F45a).
  "chua-co-tin-nhan": () => {
    const than: readonly Diem[] = [[51, 45], [57, 11], [125, 15], [123, 57], [61, 59], [41, 55]];
    return [
      { d: daGiac(than), mau: "giay" },
      { d: daGiac(than), mau: "muc", net: 2.2 },
      { d: tron(91, 35, 3.4), mau: "gap" },
    ];
  },
  // No invitation: an open envelope with nothing inside it yet.
  "chua-co-loi-moi": () => {
    const bao: readonly Diem[] = [[58, 46], [126, 42], [128, 96], [60, SAN]];
    return [
      { d: daGiac(bao), mau: "giay" },
      { d: daGiac(bao), mau: "muc", net: 2.2 },
      { d: daGiac([[58, 46], [93, 70], [126, 42]]), mau: "bong" },
      { d: netGay([[58, 46], [93, 70], [126, 42]]), mau: "muc", net: 2 },
      { d: daGiac([[112, 43], [126, 42], [127, 55]]), mau: "gap" },
    ];
  },
  // No memory on the wall: a line strung overhead with ONE EMPTY clip. The
  // print is not on the line -- it is still in a hand below, which is what the
  // screen reader is told too.
  "chua-co-ky-niem": () => [
    { d: qCong([40, 40], [88, 54], [136, 36]), mau: "muc", net: 2.4 },
    // Two clips, both empty: one clip alone reads as a peg, two read as a line
    // that photographs belong on and none of them are here yet.
    // The coral clip sits where the raised hand reaches (box (86, 16) of a
    // figure at x0 8, scale 0.74 → scene (71.6, 46.5)); the second one is
    // further along the line, so the line reads as a line and not a peg.
    { d: khungBo(67, 47, 9, 13, 3), mau: "gap" },
    { d: khungBo(67, 47, 9, 13, 3), mau: "muc", net: 1.8 },
    { d: khungBo(110, 42, 9, 13, 3), mau: "gap" },
    { d: khungBo(110, 42, 9, 13, 3), mau: "muc", net: 1.8 },
  ],
  // The filter hides everything: a mesh drawn across, with one gap left.
  "bo-loc-che-het": () => {
    const khung: readonly Diem[] = [[52, 24], [126, 24], [126, 88], [52, 88]];
    return [
      { d: daGiac(khung), mau: "giay" },
      { d: daGiac(khung), mau: "muc", net: 2.2 },
      ...[38, 54, 70].map((y) => ({ d: netGay([[54, y], [124, y]] as const), mau: "bong" as const, net: 3 })),
      ...[70, 88, 106].map((x) => ({ d: netGay([[x, 26], [x, 86]] as const), mau: "bong" as const, net: 3 })),
      // The one square left open: the reason to lift a filter rather than give up.
      // The open square sits at the NEAR edge, level with the figure's eyes,
      // and it is the SAME place the hand braces: box (72, 26) of a figure at
      // x0 -1, scale 0.76 → scene (53.7, 43.3). Out at x 97 it was across the
      // panel from the head and the pose read as standing beside a screen
      // (finish review 09/09).
      { d: khungBo(50, 58, 18, 18, 2), mau: "giay" },
      ...vongHo(59, 67, 12, { moTai: Math.PI * 0.7, net: 2.6 }),
    ];
  },
};

/**
 * Where the figure stands in each scene, as DATA: which pose, at what scale,
 * where the 96-box lands (`y0` defaults to feet on the floor), the face and
 * legs that override the pose's own, and any prop the figure has to be drawn
 * OVER. `hinhCanh` draws from this entry and `POSE_CANH` is read from the same
 * entry, so the pose a test sees is the pose the screen gets -- a separate list
 * of poses would be a second place to be wrong (audit native 09/09, F45a).
 *
 * Each pose's contact points land on the prop: `x0 + 96-box x * tiLe` equals
 * the prop's edge. Feet on the floor unless `y0` says otherwise.
 */
type ChoNep = {
  pose: PoseNep;
  x0: number;
  y0?: number;
  tiLe: number;
  them?: Omit<TuyChonNep, "x0" | "y0" | "tiLe">;
  /** Layers the figure is drawn over: the print in its hand, not the wall. */
  truoc?: () => LopVe[];
} | null;

const NEP: Record<CanhId, ChoNep> = {
  // Hand at box (90, 34) → scene (90, 56.4): the top of the chair's left post.
  // Offering, not merely standing there: the face is the one the pose is for.
  "chua-co-hoi": { pose: "keo-ghe", x0: 18, tiLe: 0.8, them: { bieuCam: "nhuong" } },
  // The sheet floats; the figure writes beside it, off the floor line on purpose.
  // Concentrating on the first line rather than smiling at it.
  "chua-co-keo": { pose: "ghi-lai", x0: -2, y0: 14, tiLe: 0.8, them: { bieuCam: "quyet" } },
  // Hands at box (74, 18) and (76, 70) → scene x 58.2 and 59.8: the frame's near edge,
  // a sliver of which stays visible between the body and the frame. Looking
  // THROUGH the empty frame, which is what makes it a viewfinder and not a board.
  "chua-co-anh": { pose: "giu-khung", x0: -1, tiLe: 0.8, them: { bieuCam: "hoi" } },
  // SEATED on the near chair, hand open across the table to the seat still
  // free: a different situation from the pulled-out chair above, not the same
  // figure beside a different prop. Placed by the SEAT, not by the floor: the
  // hips (box y 76) have to meet `hinhGhe(54, …, 0.7)`'s seat at scene y 86.6,
  // so the floor default is wrong here and the feet land in front of the chair.
  "chua-co-ban": { pose: "giu-cho", x0: 30, y0: 86.6 - 76 * 0.7, tiLe: 0.7, them: { bieuCam: "nhuong", dang: "ngoi" } },
  // Hands at box (75, 26) and (78, 66) → scene x 61 and 63.4: the map's near edge.
  "tim-khong-ra": { pose: "cam-ban-do", x0: 1, tiLe: 0.8, them: { bieuCam: "hoi" } },
  // The one scene the figure may never enter: a failure (`CANH_KHONG_NEP`).
  "chua-doc-duoc": null,
  // Calling out, off the floor line like `chua-co-keo` so the two silences of
  // the chat and the plan sit at the same height -- but a different act: the
  // hand is at the mouth, not on a pencil, and the bubble starts at the mouth.
  "chua-co-tin-nhan": { pose: "goi-loi", x0: -4, y0: 16, tiLe: 0.78 },
  // Holding the empty envelope out, offering it to whoever is not here yet.
  // Hands at box (82, 66) and (70, 72) → scene x 61.9 and 52.6: the near edge
  // of the envelope, so it is being HELD OUT rather than standing beside one.
  "chua-co-loi-moi": { pose: "dua-hai-tay", x0: -2, tiLe: 0.78, them: { bieuCam: "nhuong" } },
  // One arm straight up to the empty clip on the line, the other holding the
  // print that has not gone up yet. Reaching, not framing: `chua-co-anh`
  // already owns the two-hands-on-a-rectangle body.
  "chua-co-ky-niem": {
    pose: "voi-len",
    x0: 8,
    tiLe: 0.74,
    // The print hanging from the low hand, drawn first so the fingers stay in
    // front of it. Read off the pose's own low hand (box (8, 62) of `voi-len`),
    // not a point near it: a prop anchored to a coordinate no pose owns is how
    // a hand ends up holding nothing. Kept above the floor and clear of both
    // legs: at the old hand it read as a white square behind a shin.
    truoc: () => {
      const P = (x: number, y: number): Diem => [8 + x * 0.74, SAN - CHAN_NEP * 0.74 + y * 0.74];
      const [gx, gy] = P(8, 62);
      return [
        { d: khungBo(gx - 9, gy, 22, 20, 3), mau: "giay" },
        { d: khungBo(gx - 9, gy, 22, 20, 3), mau: "muc", net: 2.2 },
      ];
    },
  },
  // Leaning in at the one open square: the far hand at box (78, 44) → scene
  // (58.3, 66.3), which IS the square, so the brace point and the hole are the
  // same place -- and it sits at eye height rather than above the head, so the
  // figure is looking through it. A body bent sideways with the other arm
  // down, not the two-handed grip `tim-khong-ra` already uses.
  "bo-loc-che-het": { pose: "ghe-nhin", x0: -1, tiLe: 0.76 },
};

/** The pose each scene stands in; `null` for a scene that never has the figure. */
export const POSE_CANH: Readonly<Record<CanhId, PoseNep | null>> = Object.fromEntries(
  CANH_IDS.map((id) => [id, NEP[id]?.pose ?? null]),
) as Record<CanhId, PoseNep | null>;

function veNep(cho: ChoNep): LopVe[] {
  if (cho === null) return [];
  const y0 = cho.y0 ?? SAN - CHAN_NEP * cho.tiLe;
  return [...(cho.truoc?.() ?? []), ...hinhNep(cho.pose, { x0: cho.x0, y0, tiLe: cho.tiLe, ...cho.them })];
}

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
  // A scene in `CANH_KHONG_NEP` never draws the figure, whatever the caller
  // passes: the rule that keeps the character away from errors and money is
  // carried here, not by twenty screens each remembering `nep={false}`.
  const veNepDuoc = nep && !CANH_KHONG_NEP.has(canh);
  return veNepDuoc ? [...NEN[canh](), ...veNep(NEP[canh])] : NEN[canh]();
}
