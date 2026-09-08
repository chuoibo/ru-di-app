/**
 * Nếp: the folded invitation that keeps a seat for you.
 *
 * Drawn after the 2026-09-08 concept sheet (docs/codex/2026-09-08/nep-concept),
 * not after the rectangular sketch of 07/09: a short, slightly wide sheet with
 * a diagonal fold across the lower body like two lapels, ONE coral corner
 * folded down over the top right, small ink eyes under an uneven brow, a
 * sidelong smile, short tapered ink legs and mitten hands that do something.
 * The concept's paper-plane seal and the two dashes on the body are left off
 * on purpose (they read as a mail app), and the body is a little shorter than
 * the sheet's so it stops looking like an envelope.
 *
 * Nếp is a removable layer. Every scene in `canh.ts` is complete without it,
 * and it never stands next to money, an error or a conflict (report 07/09 §6.4).
 *
 * Every path goes through `net.ts`: absolute M/L/C/Z only, parsed by
 * `tests/art-duong.test.mjs` the way the Java side parses it at mount.
 */
import { type Diem, type LopVe, bau, bienDoi, cong, cungTron, daGiac, netGay, qCong, tron, thon, vien } from "./net";

/** The figure is authored in this square; a scene places it with `x0`, `y0`, `tiLe`. */
export const KHUNG_NEP = 96;

export const POSE_NEP = ["moi", "keo-ghe", "giu-cho", "gop-y", "cam-ban-do", "giu-khung", "doi", "ghi-lai", "vui"] as const;
export type PoseNep = (typeof POSE_NEP)[number];

/**
 * What each pose does with the whole body, not only the arms (review 08/09,
 * package 1: a figure that acts out the scene leans into the thing it does
 * and looks at it). `nghieng` is how far the top of the sheet moves sideways
 * while the feet stay on the ground line, in 96-box units; `nhin` shifts the
 * eyes toward what the pose is about, a couple of units at most.
 */
const TU_THE: Record<PoseNep, { nghieng: number; nhin: readonly [number, number] }> = {
  moi: { nghieng: 0, nhin: [0.4, 0.2] },
  "keo-ghe": { nghieng: -6, nhin: [1.3, 0.8] },
  "giu-cho": { nghieng: 0, nhin: [1.4, 0.3] },
  "gop-y": { nghieng: 2, nhin: [0.9, 1] },
  "cam-ban-do": { nghieng: 5, nhin: [1, 1.2] },
  "giu-khung": { nghieng: 4, nhin: [1.3, 0] },
  doi: { nghieng: 0, nhin: [1, -0.6] },
  "ghi-lai": { nghieng: 0, nhin: [1, -0.5] },
  vui: { nghieng: 0, nhin: [0, -0.5] },
};

/** The feet stand on this line of the 96-box; a scene puts its floor here. */
export const CHAN_NEP = 91;

export interface TuyChonNep {
  /** Where the 96-box's origin lands in the caller's frame. */
  x0?: number;
  y0?: number;
  /** Scale applied to the 96-box. */
  tiLe?: number;
  /**
   * The 96dp reading has brows, a crease and props; the 48dp reading drops
   * them and thickens the limbs. A real second drawing, not a scaled one.
   */
  chiTiet?: boolean;
  /** Override the pose's lean: sideways travel of the top of the sheet, feet fixed. */
  nghieng?: number;
  /** Override where the pose looks: an eye offset, clamped to a couple of units. */
  nhin?: readonly [number, number];
}

export function laPoseNep(pose: string): pose is PoseNep {
  return (POSE_NEP as readonly string[]).includes(pose);
}

/** The layers of one pose, back to front. An unknown pose draws «moi». */
export function hinhNep(pose: string, tuyChon: TuyChonNep = {}): LopVe[] {
  const { x0 = 0, y0 = 0, tiLe = 1, chiTiet = true } = tuyChon;
  const P = bienDoi(x0, y0, tiLe);
  const net = (w: number) => w * tiLe;
  const p: PoseNep = laPoseNep(pose) ? pose : "moi";
  const nghieng = tuyChon.nghieng ?? TU_THE[p].nghieng;
  const [nhinX, nhinY] = (tuyChon.nhin ?? TU_THE[p].nhin).map((v) => Math.max(-1.6, Math.min(1.6, v)));
  // The lean: a shear of everything above the ground line, so the feet keep
  // their place on the floor and the head travels the whole `nghieng`. Hands
  // are placed through `P`, unsheared, because they land on things.
  const S = (x: number, y: number): Diem => P(x + (nghieng * (CHAN_NEP - y)) / (CHAN_NEP - 20), y);

  // The sheet: nearly a rectangle, a shade wider at the foot, its top edge
  // climbing to the right so the figure leans toward whoever it is talking
  // to. The corner at the top right is cut along H..G, where it folds down.
  const A = S(26, 22), H = S(50, 20), G = S(69, 38), C = S(70, 66), D = S(62, 74), E = S(30, 76), F = S(24, 50);
  // The folded-down corner: the mirror image of the cut-off corner across
  // H..G, a flap that lands over the body with its right angle inside.
  const Bp = S(50, 38);
  // The lapel: one crease from high on the left edge across the body to the
  // lower right, the face of the sheet below it overlapping in shade. It is
  // the concept's identity mark, not a cut corner (finish review 08/09).
  const V1 = S(26, 40), V2 = S(66, 74);
  const than: LopVe[] = [
    { d: daGiac([A, H, G, C, D, E, F]), mau: "giay" },
    { d: daGiac([V1, F, E, D, V2]), mau: "bong" },
    ...(chiTiet ? [{ d: netGay([V1, V2]), mau: "muc" as const, net: net(1.8) }] : []),
    { d: daGiac([H, G, Bp]), mau: "gap" },
    { d: daGiac([A, H, G, C, D, E, F]), mau: "muc", net: net(chiTiet ? 2.4 : 3) },
    { d: daGiac([H, G, Bp]), mau: "muc", net: net(chiTiet ? 1.8 : 2.4) },
  ];

  // The eyes sit where the pose looks; brows and smile stay with the face.
  const mat: LopVe[] = chiTiet
    ? [
        { d: bau(...S(39 + nhinX, 45 + nhinY), 2.1 * tiLe, 3 * tiLe), mau: "muc" },
        { d: bau(...S(52 + nhinX, 43 + nhinY), 2.1 * tiLe, 3 * tiLe), mau: "muc" },
        // One brow level, one raised: the expression lives in the brows, not in the mouth.
        { d: netGay([S(35, 38), S(42, 37)]), mau: "muc", net: net(1.9) },
        { d: netGay([S(48, 34), S(55, 36.5)]), mau: "muc", net: net(1.9) },
        // A short, closed, sidelong smile; never an open U.
        { d: cong(S(46, 53), S(48, 56), S(52, 56), S(55, 53)), mau: "muc", net: net(2) },
      ]
    : [
        { d: tron(...S(39 + nhinX, 45 + nhinY), 2.6 * tiLe), mau: "muc" },
        { d: tron(...S(52 + nhinX, 43 + nhinY), 2.6 * tiLe), mau: "muc" },
        { d: cong(S(46, 54), S(48, 56.5), S(52, 56.5), S(55, 54)), mau: "muc", net: net(2.4) },
      ];

  // Limbs are filled capsules, so they do not depend on the renderer's caps.
  const wTay = (chiTiet ? 4.2 : 5) * tiLe;
  const rBan = (chiTiet ? 3.4 : 3.8) * tiLe;
  const tay = (a: Diem, b: Diem): LopVe[] => [
    { d: vien(a, b, wTay), mau: "muc" },
    { d: tron(b[0], b[1], rBan), mau: "muc" },
  ];
  const L = S(25, 54), R = S(69, 50);

  // Legs from the sheared hip to feet that stay on the ground line.
  const chan: LopVe[] = [
    { d: thon(S(39, 76), P(36, 90), 5.5 * tiLe, 3.6 * tiLe), mau: "muc" },
    { d: thon(S(56, 75), P(60, 90), 5.5 * tiLe, 3.6 * tiLe), mau: "muc" },
    { d: vien(P(31, CHAN_NEP), P(39, CHAN_NEP), 3.6 * tiLe), mau: "muc" },
    { d: vien(P(58, CHAN_NEP), P(67, CHAN_NEP), 3.6 * tiLe), mau: "muc" },
  ];

  let tuThe: LopVe[];
  switch (p) {
    case "moi":
      // An open hand toward the empty seat; the other arm at rest.
      tuThe = [...tay(R, P(90, 52)), ...tay(L, P(14, 66))];
      break;
    case "keo-ghe":
      // Pulling a chair out: the far hand grips the top of a chair back the
      // scene stands at x 90 with its feet on the ground line; the body
      // leans back from the pull; the near hand opens to whoever the seat
      // is for; the eyes are on the seat, down and to the right.
      tuThe = [...tay(R, P(90, 34)), ...tay(L, P(8, 66))];
      break;
    case "giu-cho":
      // Keeping a seat: a hand rests on the rail of the chair beside it
      // (x 88, at chest height), the other arm at rest, looking across to
      // the seat still free.
      tuThe = [...tay(R, P(88, 48)), ...tay(L, P(14, 68))];
      break;
    case "gop-y": {
      // Reading a small folded route map held out to the right.
      const banDo: LopVe[] = [
        { d: daGiac([P(58, 52), P(92, 46), P(94, 68), P(60, 74)]), mau: "giay" },
        { d: daGiac([P(58, 52), P(92, 46), P(94, 68), P(60, 74)]), mau: "muc", net: net(1.8) },
        ...(chiTiet
          ? [
              { d: netGay([P(76, 49), P(77, 71)]), mau: "muc" as const, net: net(1.4) },
              { d: netGay([P(64, 66), P(70, 61)]), mau: "muc" as const, net: net(1.6) },
              { d: netGay([P(73, 59), P(80, 56)]), mau: "muc" as const, net: net(1.6) },
              { d: tron(...P(87, 53), 2.2 * tiLe), mau: "gap" as const },
            ]
          : []),
      ];
      tuThe = [...tay(R, P(82, 60)), ...banDo, ...tay(L, P(56, 78))];
      break;
    }
    case "cam-ban-do": {
      // Holding a map open on the right: its near edge runs just past the
      // body's right side. The far hand grips the top of that edge; the near
      // arm hangs to an elbow and the forearm comes forward to the edge at
      // waist height; the body leans in; the eyes are down on the route.
      const khuyu = P(38, 72);
      tuThe = [...tay(R, P(75, 26)), { d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(78, 66))];
      break;
    }
    case "giu-khung": {
      // Holding an empty frame up beside it: the far hand at the frame's
      // top corner, above the head; the near arm bent, its hand on the same
      // edge low; leaning toward it, looking level into the blank.
      const khuyu = P(38, 72);
      tuThe = [...tay(R, P(74, 18)), { d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(76, 70))];
      break;
    }
    case "doi": {
      // A hand at the chin, the other at rest, and a small clock to look at.
      const dongHo: LopVe[] = chiTiet
        ? [
            { d: tron(...P(86, 22), 7 * tiLe), mau: "giay" },
            { d: tron(...P(86, 22), 7 * tiLe), mau: "muc", net: net(1.8) },
            { d: netGay([P(86, 22), P(86, 17)]), mau: "muc", net: net(1.6) },
            { d: netGay([P(86, 22), P(90, 24)]), mau: "muc", net: net(1.6) },
          ]
        : [];
      tuThe = [
        { d: vien(L, P(30, 60), wTay), mau: "muc" },
        ...tay(P(30, 60), P(38, 52)),
        ...tay(R, P(82, 64)),
        ...dongHo,
      ];
      break;
    }
    case "ghi-lai": {
      // Writing it down: a pencil in the raised hand.
      const but: LopVe[] = [
        { d: vien(P(80, 40), P(93, 23), 3.6 * tiLe), mau: "muc" },
        { d: daGiac([P(92, 21), P(96, 18), P(95, 25)]), mau: "gap" },
      ];
      tuThe = [...tay(R, P(84, 36)), ...but, ...tay(L, P(14, 66))];
      break;
    }
    case "vui":
    default: {
      // Both arms up, quietly. No motion marks: the concept forbids an
      // excited face on every pose, and the raised arms already say it.
      tuThe = [...tay(L, P(10, 28)), ...tay(R, P(88, 24))];
      break;
    }
  }

  return [...than, ...mat, ...chan, ...tuThe];
}

/** A seat for the scenes: a simple bentwood café chair. Back to the left, or to the right with `lat`. */
export function hinhGhe(x0: number, y0: number, tuyChon: { tiLe?: number; chiTiet?: boolean; lat?: boolean } = {}): LopVe[] {
  const { tiLe = 1, chiTiet = true, lat = false } = tuyChon;
  const P = (x: number, y: number): Diem => [x0 + (lat ? 44 - x : x) * tiLe, y0 + y * tiLe];
  const w = (chiTiet ? 3 : 3.6) * tiLe;
  // Back: an open arc standing on two posts; seat: a flat ellipse; four legs.
  return [
    { d: cungTron(...P(20, 18), 14 * tiLe, Math.PI, 2 * Math.PI), mau: "muc", net: w },
    { d: netGay([P(6, 18), P(6, 40)]), mau: "muc", net: w },
    { d: netGay([P(34, 18), P(34, 40)]), mau: "muc", net: w },
    ...(chiTiet ? [{ d: qCong(P(9, 24), P(20, 30), P(31, 24)), mau: "muc" as const, net: w * 0.7 }] : []),
    { d: bau(...P(22, 42), 20 * tiLe, 6 * tiLe), mau: "giay" },
    { d: bau(...P(22, 42), 20 * tiLe, 6 * tiLe), mau: "muc", net: w },
    { d: netGay([P(6, 46), P(4, 64)]), mau: "muc", net: w },
    { d: netGay([P(38, 46), P(40, 64)]), mau: "muc", net: w },
    { d: netGay([P(16, 47), P(18, 62)]), mau: "muc", net: w * 0.8 },
    { d: netGay([P(30, 47), P(29, 62)]), mau: "muc", net: w * 0.8 },
  ];
}
