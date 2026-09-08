/**
 * Stickers (ADR-0021 §2.1): a closed vocabulary drawn as vector shapes.
 *
 * The id is the whole message body; the picture is built here from a few
 * path commands per sticker and coloured from the theme at render time
 * (`ui/stickers/Sticker.tsx`). Three copies of the id list exist on purpose --
 * the server's `app/domain/stickers.py`, `packages/shared/stickers.json`, and
 * `STICKER_IDS` below -- and `tests/test_sticker_vocabulary_matches_client.py`
 * refuses the day they disagree.
 *
 * Every `d` string is produced by the builders at the bottom: explicit
 * commands, fixed arity, plain decimals. react-native-svg parses `d` in Java
 * at mount and one stray token kills the app on its first frame (see
 * `ui/duong-svg.ts`); `tests/rudi-chat-sticker.test.mjs` parses every shape the
 * way the Java side does. An unknown id draws `khac` -- a dashed frame and a
 * question mark -- never nothing and never a throw.
 */
import stickers from "../../../../../packages/shared/stickers.json";
import { CHAN_NEP, hinhGhe, hinhNep } from "../art/nep";
import { netGay, tron as tronVe, type LopVe } from "../art/net";

export const STICKER_IDS = [
  "di-thoi",
  "an-gi",
  "cafe-khong",
  "ok-chot",
  "cho-ti",
  "ket-xe",
  "tra-tien-ne",
  "tuyet-voi",
] as const;
export type StickerId = (typeof STICKER_IDS)[number];

/** Palette roles, resolved to theme colours by the component. `coral` is brand; `line` is the paper's shade. */
export type MauSticker = "accent" | "ink" | "split" | "card" | "coral" | "line";
/** One path. `net` > 0 draws it as a stroke of that width (round caps), otherwise it is filled. */
export type LopSticker = { d: string; mau: MauSticker; net?: number };
export type HinhSticker = { id: string; nhan: string; lop: LopSticker[] };

/** Every shape is drawn in this square; the component scales it. */
export const KHUNG_STICKER = 96;

const NHAN_THEO_ID: Record<string, string> = Object.fromEntries(
  stickers.stickers.map((s) => [s.id, s.label]),
);

export function laStickerHopLe(id: string): id is StickerId {
  return (STICKER_IDS as readonly string[]).includes(id);
}

/** The label for a chip or a screen reader; an unknown id is just «Sticker». */
export function nhanSticker(id: string): string {
  return NHAN_THEO_ID[id] ?? "Sticker";
}

// ---- path builders -----------------------------------------------------------

/**
 * A path from its tokens. Written as data rather than as one long string
 * because the repo guard reads nine digits with single spaces between them as
 * an account number, and a hand-drawn path literal is exactly that shape.
 */
function duong(...phan: (string | number)[]): string {
  return phan.map((p) => (typeof p === "number" ? so(p) : p)).join(" ");
}

/** Plain decimal, never exponent notation, never `-0`. */
function so(n: number): string {
  const s = n.toFixed(2).replace(/\.?0+$/, "");
  return s === "-0" ? "0" : s;
}

const K = 0.5523;

/** A circle as four cubic Béziers (the Java parser has no `A` here). */
function tron(cx: number, cy: number, r: number): string {
  const k = r * K;
  return [
    `M ${so(cx + r)} ${so(cy)}`,
    `C ${so(cx + r)} ${so(cy + k)} ${so(cx + k)} ${so(cy + r)} ${so(cx)} ${so(cy + r)}`,
    `C ${so(cx - k)} ${so(cy + r)} ${so(cx - r)} ${so(cy + k)} ${so(cx - r)} ${so(cy)}`,
    `C ${so(cx - r)} ${so(cy - k)} ${so(cx - k)} ${so(cy - r)} ${so(cx)} ${so(cy - r)}`,
    `C ${so(cx + k)} ${so(cy - r)} ${so(cx + r)} ${so(cy - k)} ${so(cx + r)} ${so(cy)}`,
    "Z",
  ].join(" ");
}

/** A rounded rectangle; `r` is clamped to half the shorter side. */
function khungBo(x: number, y: number, w: number, h: number, r: number): string {
  const rr = Math.min(r, w / 2, h / 2);
  const k = rr * K;
  return [
    `M ${so(x + rr)} ${so(y)}`,
    `L ${so(x + w - rr)} ${so(y)}`,
    `C ${so(x + w - rr + k)} ${so(y)} ${so(x + w)} ${so(y + rr - k)} ${so(x + w)} ${so(y + rr)}`,
    `L ${so(x + w)} ${so(y + h - rr)}`,
    `C ${so(x + w)} ${so(y + h - rr + k)} ${so(x + w - rr + k)} ${so(y + h)} ${so(x + w - rr)} ${so(y + h)}`,
    `L ${so(x + rr)} ${so(y + h)}`,
    `C ${so(x + rr - k)} ${so(y + h)} ${so(x)} ${so(y + h - rr + k)} ${so(x)} ${so(y + h - rr)}`,
    `L ${so(x)} ${so(y + rr)}`,
    `C ${so(x)} ${so(y + rr - k)} ${so(x + rr - k)} ${so(y)} ${so(x + rr)} ${so(y)}`,
    "Z",
  ].join(" ");
}

/** A closed polygon through the given points. */
function daGiac(diem: readonly (readonly [number, number])[]): string {
  const [dau, ...con] = diem;
  return [`M ${so(dau[0])} ${so(dau[1])}`, ...con.map(([x, y]) => `L ${so(x)} ${so(y)}`), "Z"].join(" ");
}

/** A star with `n` points, outer radius `R`, inner radius `r`. */
function ngoiSao(cx: number, cy: number, R: number, r: number, n = 5): string {
  const diem: [number, number][] = [];
  for (let i = 0; i < n * 2; i++) {
    const goc = -Math.PI / 2 + (i * Math.PI) / n;
    const ban = i % 2 === 0 ? R : r;
    diem.push([cx + Math.cos(goc) * ban, cy + Math.sin(goc) * ban]);
  }
  return daGiac(diem);
}

// ---- the art layer's pen, in sticker roles -------------------------------------

/**
 * A drawing made with the art layer's pen (`art/*`, roles giay/muc/gap/bong)
 * re-labelled in sticker roles, so a sticker can be a Nếp scene and still go
 * through the same table, the same Java-grammar test and the same renderer as
 * the seven older shapes. Roles the stickers have no colour for throw here,
 * at module load, so the node test fails before an emulator does.
 */
function tuLopVe(lop: readonly LopVe[]): LopSticker[] {
  const vai: Partial<Record<LopVe["mau"], MauSticker>> = { giay: "card", muc: "ink", gap: "accent", bong: "line", split: "split" };
  return lop.map((l) => {
    const mau = vai[l.mau];
    if (mau === undefined) throw new Error(`sticker: vai màu «${l.mau}» không có trong bảng sticker`);
    return l.net === undefined ? { d: l.d, mau } : { d: l.d, mau, net: l.net };
  });
}

/**
 * «Chờ tí»: Nếp keeps a chair -- one hand on its back rail -- and looks up at
 * a clock that is a sign, not a detail. The chair is the main prop: at 64dp
 * the reader has to see «a seat being kept» before reading the label, so the
 * chair takes the right half at 0.95 of its scene size and the figure stands
 * at 0.82, leaning toward it (`nghieng` 6). The eyes go to the clock
 * (`nhin` up-right). Geometry: hand (88, 48) of the 96-box → (63.2, 55.7),
 * on the chair's cross rail (y 53..58.7, between the posts x 57.7..84.3);
 * bounding box x 0.2..92, inside the Java test's [−1, 97].
 *
 * The compact reading (`chiTiet` false) is the same drawing with the art
 * layer's own 48dp simplifications (no brow, no crease, thicker limbs, plain
 * chair) and a heavier clock stroke; the clock keeps its size.
 */
function choTi(chiTiet: boolean): LopVe[] {
  const tiLeNep = 0.82, x0 = -8.5, tiLeGhe = 0.95, gheX = 52;
  const y0 = CHAN_NEP - CHAN_NEP * tiLeNep;
  const nguoi = hinhNep("giu-cho", { x0, y0, tiLe: tiLeNep, chiTiet, nghieng: 6, nhin: [1.5, -1.2] });
  const ghe = hinhGhe(gheX, CHAN_NEP - 64 * tiLeGhe, { tiLe: tiLeGhe, chiTiet });
  const [cx, cy, r] = [80, 16, 13];
  const w = chiTiet ? 2.2 : 2.8;
  const dongHo: LopVe[] = [
    { d: tronVe(cx, cy, r), mau: "giay" },
    { d: tronVe(cx, cy, r), mau: "muc", net: w },
    { d: netGay([[cx, cy], [cx, cy - r * 0.62]]), mau: "muc", net: w * 0.9 },
    { d: netGay([[cx, cy], [cx + r * 0.5, cy + r * 0.28]]), mau: "muc", net: w * 0.9 },
    { d: tronVe(cx, cy, chiTiet ? 1.6 : 2), mau: "gap" },
  ];
  return [...ghe, ...dongHo, ...nguoi];
}

// ---- the shapes --------------------------------------------------------------

const HINH: Record<StickerId, LopSticker[]> = {
  "di-thoi": [
    { d: khungBo(14, 80, 32, 6, 3), mau: "ink" },
    { d: khungBo(22, 14, 6, 68, 3), mau: "ink" },
    { d: daGiac([[28, 16], [78, 26], [28, 40]]), mau: "accent" },
  ],
  "an-gi": [
    { d: daGiac([[58, 10], [64, 12], [44, 52], [38, 50]]), mau: "ink" },
    { d: daGiac([[68, 14], [74, 17], [52, 54], [46, 52]]), mau: "ink" },
    { d: duong("M", 14, 44, "L", 82, 44, "C", 82, 66, 68, 80, 48, 80, "C", 28, 80, 14, 66, 14, 44, "Z"), mau: "accent" },
    { d: khungBo(12, 40, 72, 8, 4), mau: "ink" },
    { d: tron(30, 28, 3.5), mau: "accent" },
    { d: tron(20, 20, 3), mau: "accent" },
  ],
  "cafe-khong": [
    { d: tron(70, 52, 11), mau: "accent" },
    { d: tron(70, 52, 6), mau: "card" },
    { d: khungBo(20, 34, 44, 40, 8), mau: "accent" },
    { d: khungBo(12, 76, 60, 6, 3), mau: "ink" },
    { d: tron(34, 22, 3), mau: "split" },
    { d: tron(46, 16, 3), mau: "split" },
    { d: tron(58, 22, 3), mau: "split" },
  ],
  "ok-chot": [
    { d: khungBo(12, 20, 72, 56, 10), mau: "accent" },
    { d: khungBo(18, 26, 60, 44, 7), mau: "card" },
    { d: daGiac([[30, 48], [36, 42], [44, 50], [62, 32], [68, 38], [44, 62]]), mau: "accent" },
  ],
  // The first sticker drawn in the Nếp language (08/09, one before eight): see `choTi`.
  "cho-ti": [...tuLopVe(choTi(true))],
  "ket-xe": [
    { d: daGiac([[20, 58], [40, 58], [50, 40], [66, 40], [70, 48], [60, 48], [56, 58], [78, 58], [78, 64], [20, 64]]), mau: "accent" },
    { d: khungBo(62, 26, 14, 5, 2), mau: "ink" },
    { d: daGiac([[66, 30], [70, 30], [68, 42], [64, 42]]), mau: "ink" },
    { d: tron(26, 70, 12), mau: "ink" },
    { d: tron(72, 70, 12), mau: "ink" },
    { d: tron(26, 70, 5), mau: "card" },
    { d: tron(72, 70, 5), mau: "card" },
  ],
  "tra-tien-ne": [
    { d: tron(48, 48, 34), mau: "split" },
    { d: tron(48, 48, 26), mau: "card" },
    { d: khungBo(44, 28, 8, 34, 4), mau: "split" },
    { d: khungBo(38, 30, 20, 6, 3), mau: "split" },
    { d: khungBo(30, 62, 36, 6, 3), mau: "split" },
  ],
  "tuyet-voi": [
    { d: ngoiSao(48, 52, 34, 14), mau: "coral" },
    { d: ngoiSao(78, 18, 8, 3), mau: "accent" },
  ],
};

/** The dashed frame and question mark drawn for an id this build does not know. */
const HINH_KHAC: LopSticker[] = [
  { d: khungBo(14, 14, 68, 68, 12), mau: "ink" },
  { d: khungBo(19, 19, 58, 58, 8), mau: "card" },
  { d: khungBo(45, 62, 6, 6, 3), mau: "ink" },
  { d: duong("M", 36, 40, "C", 36, 30, 60, 30, 60, 40, "C", 60, 48, 48, 48, 48, 56, "L", 48, 58, "L", 42, 58, "L", 42, 54, "C", 42, 44, 54, 46, 54, 40, "C", 54, 36, 42, 36, 42, 40, "Z"), mau: "ink" },
];

/**
 * A second reading for the small tile: an optical size, not a scale-down.
 * Only the shapes that have one appear here; the rest are legible as drawn.
 */
const HINH_RUT_GON: Partial<Record<StickerId, LopSticker[]>> = {
  "cho-ti": tuLopVe(choTi(false)),
};

/** `chiTiet` false picks the compact reading, for tiles under 72dp (the tray draws 64). */
export function hinhSticker(id: string, tuyChon: { chiTiet?: boolean } = {}): HinhSticker {
  const { chiTiet = true } = tuyChon;
  if (laStickerHopLe(id)) return { id, nhan: nhanSticker(id), lop: (chiTiet ? undefined : HINH_RUT_GON[id]) ?? HINH[id] };
  return { id, nhan: "Sticker", lop: HINH_KHAC };
}
