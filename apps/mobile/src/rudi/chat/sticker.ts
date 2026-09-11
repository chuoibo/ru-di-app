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
import { bau as bauVe, cong as congVe, daGiac as daGiacVe, netGay, quat as quatVe, tron as tronVe, vien as vienVe, type LopVe } from "../art/net";

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
 * «Chờ tí»: Nếp keeps a chair -- one hand gripping the top of its back post --
 * and looks up at a clock that is a sign, not a detail. The chair is the main
 * prop: at 64dp the reader has to see «a seat being kept» before reading the
 * label, so the chair takes the right half at 0.9 of its scene size and the
 * figure stands at 0.726, leaning toward it (`nghieng` 5; the scene's
 * `keo-ghe` leans back, a pull, and here nobody pulls). The pose is `keo-ghe`
 * because its far hand is the one that grips a chair back's top (P(90, 34) of
 * the 96-box) and its near hand opens to whoever the seat is for; `giu-cho`
 * rests the hand mid-back, which at sticker size read as an arm through the
 * frame (finish review 08/09). Geometry: hand → (62.4, 49.6) = top of the near
 * post of `hinhGhe(57, …, 0.9)`, present in both readings (the compact chair
 * drops its cross rail, not its posts). Eyes go to the clock (`nhin` up-right
 * at the clamp). Clock at (72, 17) r 12 above the chair back with an 8-unit
 * gap; hands at 12 and about 1: a small angle is the one-glance sign of «một
 * tí», where a wide one would say «a time». Bounding box x 0.4..94.8.
 *
 * The compact reading (`chiTiet` false) is the same drawing with the art
 * layer's own 48dp simplifications (no brow, no crease, thicker limbs, plain
 * chair) and a heavier clock stroke; the clock keeps its size.
 */
function choTi(chiTiet: boolean): LopVe[] {
  const tiLeNep = 0.726, x0 = -2.9, tiLeGhe = 0.9, gheX = 57;
  const y0 = CHAN_NEP - CHAN_NEP * tiLeNep;
  // Same composition as the approved pilot, one heavier pen: at 0.726 the
  // figure came out lighter than the seven drawn beside it and the tray read
  // as two sets (lead decision 09/09).
  const nguoi = hinhNep("keo-ghe", { x0, y0, tiLe: tiLeNep, chiTiet, nghieng: 5, nhin: [1.4, -1.6], dam: 1.16 });
  const ghe = hinhGhe(gheX, CHAN_NEP - 64 * tiLeGhe, { tiLe: tiLeGhe, chiTiet });
  const [cx, cy, r] = [72, 17, 12];
  const w = chiTiet ? 2.2 : 2.8;
  const dongHo: LopVe[] = [
    { d: tronVe(cx, cy, r), mau: "giay" },
    { d: tronVe(cx, cy, r), mau: "muc", net: w },
    { d: netGay([[cx, cy], [cx, cy - r * 0.62]]), mau: "muc", net: w * 0.9 },
    { d: netGay([[cx, cy], [cx + r * 0.25, cy - r * 0.43]]), mau: "muc", net: w * 0.9 },
    { d: tronVe(cx, cy, chiTiet ? 1.6 : 2), mau: "gap" },
  ];
  return [...ghe, ...dongHo, ...nguoi];
}

/**
 * The other seven, in the same language (09/09, after the review approved the
 * pilot). The brief the review wrote is the one followed here: multiply the
 * VISUAL GRAMMAR, not one pose with a different object beside it. So each one
 * is a different thing to do, with its own stance and its own face, and the
 * prop budget is deliberately one object where the story allows it -- three
 * objects read slower than one mass, which is the thing `cho-ti` pays for.
 *
 * The figure is placed by `dat`: `x0`, the scale, and the floor at `CHAN_NEP`,
 * so every one of them stands on the same ground line as the scenes do. Where
 * a hand has to land on something, the prop is positioned FROM the pose's
 * documented contact point rather than by eye (DESIGN.md: change a scene and
 * you change both ends).
 */
function dat(pose: string, tiLe: number, x0: number, chiTiet: boolean): { nguoi: LopVe[]; P: (x: number, y: number) => [number, number] } {
  const y0 = CHAN_NEP - CHAN_NEP * tiLe;
  return {
    nguoi: hinhNep(pose, { x0, y0, tiLe, chiTiet }),
    P: (x, y) => [x0 + x * tiLe, y0 + y * tiLe],
  };
}

/** «Đi thôi!»: already walking, leaning into it, waving the group on with a small flag. */
function diThoi(chiTiet: boolean): LopVe[] {
  const { nguoi, P } = dat("buoc-di", 0.86, 1, chiTiet);
  const [hx, hy] = P(90, 26);
  const w = chiTiet ? 2.4 : 3;
  const dinh: [number, number] = [hx + 6.5, hy - 15];
  const co: LopVe[] = [
    { d: vienVe([hx, hy], dinh, w), mau: "muc" },
    { d: daGiacVe([dinh, [dinh[0] + 12, dinh[1] + 5], [dinh[0], dinh[1] + 10]]), mau: "gap" },
  ];
  // Two marks behind the trailing foot: the direction of travel, said once.
  const vet: LopVe[] = chiTiet
    ? [
        { d: netGay([[3, 52], [12, 52]]), mau: "bong", net: 2.6 },
        { d: netGay([[1, 62], [8, 62]]), mau: "bong", net: 2.6 },
      ]
    : [{ d: netGay([[2, 56], [12, 56]]), mau: "bong", net: 3.2 }];
  return [...vet, ...nguoi, ...co];
}

/** «Ăn gì?»: an empty bowl held up in both hands, and a face that is asking. */
function anGi(chiTiet: boolean): LopVe[] {
  const { nguoi, P } = dat("nang-to", 0.84, -2, chiTiet);
  const [tx, ty] = P(84, 58);
  const [nx] = P(62, 58);
  // The rim sits AT hand height and no wider than the two hands, so both of
  // them are on it. It used to hang 4 units below and 8 units wider, which is
  // «a bowl near a figure» rather than «a bowl being held».
  const cx = (tx + nx) / 2, cy = ty, r = (tx - nx) / 2 + 3;
  const w = chiTiet ? 2.4 : 3;
  const to: LopVe[] = [
    { d: quatVe(cx, cy, r, 0, Math.PI), mau: "gap" },
    { d: quatVe(cx, cy, r, 0, Math.PI), mau: "muc", net: w },
    { d: netGay([[cx - r - 2, cy], [cx + r + 2, cy]]), mau: "muc", net: w },
  ];
  return [...nguoi, ...to];
}

/** «Cà phê không?»: the cup pushed across to whoever is being asked. */
function caPheKhong(chiTiet: boolean): LopVe[] {
  const { nguoi, P } = dat("moi-ly", 0.78, -2, chiTiet);
  const [hx, hy] = P(88, 60);
  const w = chiTiet ? 2.4 : 3;
  const x = hx - 5, y = hy - 4, bw = 20, bh = 20;
  const ly: LopVe[] = [
    { d: tronVe(x + bw + 3.4, y + 9, 5.2), mau: "muc", net: w },
    { d: daGiacVe([[x, y], [x + bw, y], [x + bw - 3, y + bh], [x + 3, y + bh]]), mau: "gap" },
    { d: daGiacVe([[x, y], [x + bw, y], [x + bw - 3, y + bh], [x + 3, y + bh]]), mau: "muc", net: w },
    { d: netGay([[x - 5, y + bh + 4], [x + bw + 4, y + bh + 4]]), mau: "muc", net: w },
  ];
  return [...nguoi, ...ly];
}

/** «OK, chốt!»: the mark being pressed down on the plan. Weight over the hand. */
function okChot(chiTiet: boolean): LopVe[] {
  const { nguoi, P } = dat("dat-tay", 0.74, -3, chiTiet);
  const [hx, hy] = P(78, 84);
  const w = chiTiet ? 2.2 : 2.8;
  const to: [number, number][] = [[hx - 16, hy - 10], [hx + 22, hy - 14], [hx + 25, hy + 2], [hx - 13, hy + 6]];
  const giay: LopVe[] = [
    { d: daGiacVe(to), mau: "giay" },
    { d: daGiacVe(to), mau: "muc", net: w },
    // The tick is being MADE, not printed: two strokes of the same pen.
    { d: netGay([[hx + 3, hy - 5], [hx + 9, hy], [hx + 20, hy - 10]]), mau: "gap", net: w * 1.7 },
  ];
  // Sheet first, THEN the figure: the hand that presses it has to be visible,
  // or the gesture that carries «chốt» is behind the paper (finish review).
  return [...giay, ...nguoi];
}

/**
 * «Kẹt xe»: sitting on something that is not moving, because the road ahead is
 * full -- the back of a bus stands right at the front wheel.
 *
 * The first version was the bike and the tired face alone, and it read as «đi
 * xe» (audit 09/09, F45b). The second put a tall box in front, and at tray size
 * that box read as a phone or a kiosk (re-audit 10/09): nothing said VEHICLE.
 * What says it now is the thing every child draws first -- a body lifted on
 * two wheels -- plus the rear-window band and ONE tail-light bar. Two coral
 * dots were tried and rejected (two dots over a line is a face); one bar is
 * the back of a vehicle and nothing else. The rider sits a touch smaller and
 * further left so the bus is wider than before and tops the rider's head: the
 * thing in the way is bigger than you. A blind read of the first wheeled
 * version still said «van / food cart» (finish review 11/09): the body was a
 * portrait box with one window. Now it is wider than tall with two panes.
 */
function ketXe(chiTiet: boolean): LopVe[] {
  // The rider sits at 0.66 so the bus can be wider than it is tall; the bike's
  // fixed offsets scale with the rider (`k`) so the frame keeps its proportions.
  const tiLe = 0.66, k = tiLe / 0.76;
  const { nguoi, P } = dat("ngoi-xe", tiLe, -9, chiTiet);
  const [hx, hy] = P(86, 54);
  const [mx, my] = P(46, 78);
  const w = chiTiet ? 2.4 : 3;
  const rBanh = 8.5 * k, ySan = CHAN_NEP - rBanh;
  // The bus: its near edge is the front wheel's far edge plus two units; body
  // wider than tall, two rear-window panes (one pane reads as a screen), its
  // own wheels on the same ground line under a body that stops above them, and
  // ONE coral tail-light bar.
  const bx = hx - 4 + rBanh + 2, bTop = 42, bBot = 80, rB = 5.5, cyB = CHAN_NEP - rB;
  const giua = (bx + 95) / 2;
  const xeTruoc: LopVe[] = [
    { d: tronVe(bx + 8, cyB, rB), mau: "muc" },
    { d: tronVe(95 - 8, cyB, rB), mau: "muc" },
    { d: khungBo(bx, bTop, 95 - bx, bBot - bTop, 3), mau: "giay" },
    { d: khungBo(bx + 3, bTop + 5, giua - 1 - (bx + 3), 15, 2), mau: "bong" },
    { d: khungBo(giua + 1, bTop + 5, 95 - 3 - (giua + 1), 15, 2), mau: "bong" },
    { d: khungBo(bx, bTop, 95 - bx, bBot - bTop, 3), mau: "muc", net: w },
    { d: khungBo(bx + 3, bBot - 9, 95 - bx - 6, 3.5, 1.5), mau: "gap" },
  ];
  const xe: LopVe[] = [
    { d: netGay([[mx - 10 * k, my + 2 * k], [mx + 12 * k, my + 2 * k]]), mau: "muc", net: w * 1.8 },
    { d: netGay([[mx + 10 * k, my + 3 * k], [hx - 2 * k, hy + 4 * k]]), mau: "muc", net: w },
    { d: netGay([[hx - 8 * k, hy + 1 * k], [hx + 5 * k, hy - 2 * k]]), mau: "muc", net: w },
    { d: netGay([[mx - 8 * k, my + 4 * k], [mx - 14 * k, ySan - 2]]), mau: "muc", net: w },
    { d: netGay([[hx - 2 * k, hy + 6 * k], [hx - 4, ySan - 2]]), mau: "muc", net: w },
    { d: tronVe(mx - 16 * k, ySan, rBanh), mau: "gap" },
    { d: tronVe(mx - 16 * k, ySan, rBanh), mau: "muc", net: w },
    { d: tronVe(hx - 4, ySan, rBanh), mau: "gap" },
    { d: tronVe(hx - 4, ySan, rBanh), mau: "muc", net: w },
  ];
  // No exhaust puff: it was the one motion cue left in a picture whose whole
  // point is «not moving» (finish review 10/09). Bus first, then the bike, then
  // the rider: the hand that props the chin has to be seen.
  return [...xeTruoc, ...xe, ...nguoi];
}

/**
 * «Trả tiền nè»: three notes fanned out of both hands, with a small bow.
 *
 * Deliberately NOT a coin, a tick, a currency mark, a QR or a bank: this is a
 * sentence the sender says, and it must never be mistaken for the system
 * confirming that money moved (ADR-0021 boundary, DESIGN.md «ngoại lệ
 * sticker»). What carries the meaning is the gesture of handing over one's
 * share. Two stacked notes with a coral corner read as a ticket or a card
 * (audit 09/09, F45c); a fan of three read as cash at 120 but still as
 * «vé/giấy» at tray size (re-audit 10/09). The top note now carries the two
 * marks every banknote has and no ticket does -- an oval (the portrait) and an
 * inner frame (the double border) -- and nothing that names a currency. A torn
 * half of a bill was tried and rejected: handing over the bill reads as asking
 * for money, the opposite sentence.
 */
function traTienNe(chiTiet: boolean): LopVe[] {
  const { nguoi, P } = dat("dua-hai-tay", 0.82, -8, chiTiet);
  const [ax, ay] = P(82, 66);
  const w = chiTiet ? 2.2 : 2.8;
  // The fan pivots just past the far hand, so the hands stay outside the notes
  // and in front of them: an object handed by nobody is the transaction
  // artifact the ADR boundary rules out.
  const goc: [number, number] = [ax + 2, ay + 2];
  const xoay = (x: number, y: number, deg: number): [number, number] => {
    const a = (deg * Math.PI) / 180, dx = x - goc[0], dy = y - goc[1];
    return [goc[0] + dx * Math.cos(a) - dy * Math.sin(a), goc[1] + dx * Math.sin(a) + dy * Math.cos(a)];
  };
  const to = (deg: number): [number, number][] =>
    [[goc[0], goc[1] - 6], [goc[0] + 27, goc[1] - 6], [goc[0] + 27, goc[1] + 6], [goc[0], goc[1] + 6]].map(([x, y]) => xoay(x, y, deg));
  const tien: LopVe[] = [];
  for (const deg of [14, 0, -14]) {
    const pts = to(deg);
    tien.push({ d: daGiacVe(pts), mau: "giay" }, { d: daGiacVe(pts), mau: "muc", net: w });
    if (deg === -14) {
      // The banknote marks, on the note that is seen whole: the inner frame
      // only at the detailed size (a hairline inside a 64dp note is noise), the
      // oval at both, filled paper-shade so it reads as a shape, not a hole.
      if (chiTiet) {
        const trong = [[goc[0] + 2.5, goc[1] - 3.8], [goc[0] + 24.5, goc[1] - 3.8], [goc[0] + 24.5, goc[1] + 3.8], [goc[0] + 2.5, goc[1] + 3.8]]
          .map(([x, y]) => xoay(x, y, deg)) as [number, number][];
        tien.push({ d: daGiacVe(trong), mau: "muc", net: w * 0.5 });
      }
      const [cx, cy] = xoay(goc[0] + 13.5, goc[1], deg);
      tien.push({ d: bauVe(cx, cy, 3.8, 2.7), mau: "bong" }, { d: bauVe(cx, cy, 3.8, 2.7), mau: "muc", net: w * 0.6 });
      // The top note keeps the folded coral corner: the same fold Nếp wears.
      const c: [number, number][] = [pts[1], [pts[1][0] - 7, pts[1][1] + 1], [pts[1][0] - 1, pts[1][1] + 6]];
      tien.push({ d: daGiacVe(c), mau: "gap" }, { d: daGiacVe(c), mau: "muc", net: w });
    }
  }
  // Notes first, figure second: the hands must be seen giving them.
  return [...tien, ...nguoi];
}

/** «Tuyệt vời»: both feet off the ground, and one burst. */
function tuyetVoi(chiTiet: boolean): LopVe[] {
  const { nguoi, P } = dat("nhay", 0.86, 2, chiTiet);
  const [hx, hy] = P(93, 18);
  const w = chiTiet ? 2.4 : 3;
  const tia: LopVe[] = [
    { d: netGay([[hx + 2, hy - 4], [hx + 8, hy - 12]]), mau: "gap", net: w },
    { d: netGay([[hx + 6, hy + 1], [hx + 15, hy - 3]]), mau: "gap", net: w },
    ...(chiTiet ? [{ d: netGay([[hx - 3, hy - 7], [hx - 1, hy - 15]]), mau: "gap" as const, net: w }] : []),
  ];
  return [...nguoi, ...tia];
}

// ---- the shapes --------------------------------------------------------------

const HINH: Record<StickerId, LopSticker[]> = {
  "di-thoi": [...tuLopVe(diThoi(true))],
  "an-gi": [...tuLopVe(anGi(true))],
  "cafe-khong": [...tuLopVe(caPheKhong(true))],
  "ok-chot": [...tuLopVe(okChot(true))],
  "cho-ti": [...tuLopVe(choTi(true))],
  "ket-xe": [...tuLopVe(ketXe(true))],
  "tra-tien-ne": [...tuLopVe(traTienNe(true))],
  "tuyet-voi": [...tuLopVe(tuyetVoi(true))],
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
 * Since 09/09 ALL eight have one: the tray draws a second, simpler drawing
 * rather than the 120dp one shrunk, so there is no fallback left to describe.
 */
const HINH_RUT_GON: Record<StickerId, LopSticker[]> = {
  "di-thoi": tuLopVe(diThoi(false)),
  "an-gi": tuLopVe(anGi(false)),
  "cafe-khong": tuLopVe(caPheKhong(false)),
  "ok-chot": tuLopVe(okChot(false)),
  "cho-ti": tuLopVe(choTi(false)),
  "ket-xe": tuLopVe(ketXe(false)),
  "tra-tien-ne": tuLopVe(traTienNe(false)),
  "tuyet-voi": tuLopVe(tuyetVoi(false)),
};

/** `chiTiet` false picks the compact reading, for tiles under 72dp (the tray draws 64). */
export function hinhSticker(id: string, tuyChon: { chiTiet?: boolean } = {}): HinhSticker {
  const { chiTiet = true } = tuyChon;
  if (laStickerHopLe(id)) return { id, nhan: nhanSticker(id), lop: chiTiet ? HINH[id] : HINH_RUT_GON[id] };
  return { id, nhan: "Sticker", lop: HINH_KHAC };
}
