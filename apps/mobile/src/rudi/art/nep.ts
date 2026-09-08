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
import { type Diem, type LopVe, bau, bienDoi, cong, cungTron, daGiac, netCong, netGay, qCong, tron, thon, vien } from "./net";

/** The figure is authored in this square; a scene places it with `x0`, `y0`, `tiLe`. */
export const KHUNG_NEP = 96;

export const POSE_NEP = [
  "moi",
  "keo-ghe",
  "giu-cho",
  "gop-y",
  "cam-ban-do",
  "giu-khung",
  "doi",
  "ghi-lai",
  "vui",
  // Added for the eight stickers (09/09): each is a different thing to DO,
  // not the same body holding a different object.
  "buoc-di",
  "nang-to",
  "moi-ly",
  "dat-tay",
  "ngoi-xe",
  "dua-hai-tay",
  "nhay",
] as const;
export type PoseNep = (typeof POSE_NEP)[number];

/**
 * The face, as a small closed set.
 *
 * Until 09/09 every pose wore the SAME face: two eyes that followed the gaze,
 * one level brow, one raised brow, one closed sidelong smile. That is why the
 * review could say the figure had a range of gestures but no range of feeling,
 * and why «OK, chốt!» and «Tuyệt vời» could not read differently. The brows
 * and the mouth carry it; the eyes stay dots and keep following `nhin`,
 * because big eyes and blushing cheeks are exactly the register the concept
 * note ruled out.
 */
export const BIEU_CAM = ["binh-than", "hao-hung", "hoi", "quyet", "met", "nhuong"] as const;
export type BieuCamNep = (typeof BIEU_CAM)[number];

/**
 * The legs, as a small closed set.
 *
 * They used to be four fixed layers, so nobody could walk, sit or jump: «Đi
 * thôi!» had no direction of travel and «Kẹt xe» had nothing to be stuck on.
 * `dung` is the old drawing, unchanged, so every existing pose and scene keeps
 * the figure it had.
 */
export const DANG = ["dung", "buoc", "nhun", "ngoi", "chong"] as const;
export type DangNep = (typeof DANG)[number];

/**
 * What each pose does with the whole body, not only the arms (review 08/09,
 * package 1: a figure that acts out the scene leans into the thing it does
 * and looks at it). `nghieng` is how far the top of the sheet moves sideways
 * while the feet stay on the ground line, in 96-box units; `nhin` shifts the
 * eyes toward what the pose is about, a couple of units at most; `bieuCam` and
 * `dang` are the face and the legs, so a pose is a whole figure.
 */
const TU_THE: Record<PoseNep, { nghieng: number; nhin: readonly [number, number]; bieuCam: BieuCamNep; dang: DangNep }> = {
  moi: { nghieng: 0, nhin: [0.4, 0.2], bieuCam: "nhuong", dang: "dung" },
  "keo-ghe": { nghieng: -6, nhin: [1.3, 0.8], bieuCam: "binh-than", dang: "dung" },
  "giu-cho": { nghieng: 0, nhin: [1.4, 0.3], bieuCam: "binh-than", dang: "dung" },
  "gop-y": { nghieng: 2, nhin: [0.9, 1], bieuCam: "binh-than", dang: "dung" },
  "cam-ban-do": { nghieng: 5, nhin: [1, 1.2], bieuCam: "binh-than", dang: "dung" },
  "giu-khung": { nghieng: 4, nhin: [1.3, 0], bieuCam: "binh-than", dang: "dung" },
  doi: { nghieng: 0, nhin: [1, -0.6], bieuCam: "binh-than", dang: "dung" },
  "ghi-lai": { nghieng: 0, nhin: [1, -0.5], bieuCam: "binh-than", dang: "dung" },
  vui: { nghieng: 0, nhin: [0, -0.5], bieuCam: "hao-hung", dang: "dung" },
  // Mid-stride, leaning into the direction of travel, waving the group on.
  "buoc-di": { nghieng: 9, nhin: [1.4, -0.4], bieuCam: "hao-hung", dang: "buoc" },
  // Holding an empty bowl up and asking what goes in it.
  "nang-to": { nghieng: -2, nhin: [0.5, 0.7], bieuCam: "hoi", dang: "dung" },
  // Pushing a cup across to whoever is being asked.
  "moi-ly": { nghieng: 7, nhin: [1.5, 0.6], bieuCam: "nhuong", dang: "dung" },
  // Pressing a mark down on the plan: weight over the hand, eyes on it.
  "dat-tay": { nghieng: 6, nhin: [1.2, 1.4], bieuCam: "quyet", dang: "chong" },
  // Sitting on something that is not moving, one hand still on the bar.
  "ngoi-xe": { nghieng: 1, nhin: [1.1, 0.3], bieuCam: "met", dang: "ngoi" },
  // Both hands out, offering something small, with a small bow.
  "dua-hai-tay": { nghieng: 3, nhin: [1.2, 0.8], bieuCam: "nhuong", dang: "dung" },
  // Both feet off the ground.
  nhay: { nghieng: 0, nhin: [0, -0.9], bieuCam: "hao-hung", dang: "nhun" },
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
  /** Override the face. A pose brings its own; a scene may want a quieter one. */
  bieuCam?: BieuCamNep;
  /** Override the legs, e.g. to stand a walking pose still inside a scene. */
  dang?: DangNep;
  /**
   * Ink weight only, as a multiplier. Geometry, placement and every contact
   * point stay exactly where they are; strokes and limbs get thicker.
   *
   * It exists because a drawing that has to be SMALL inside its frame (the
   * `cho-ti` sticker gives half its box to a chair, so the figure stands at
   * 0.726 where the rest of the set stands near 0.84) comes out lighter than
   * the others and the tray stops reading as one set. The answer is not to
   * enlarge it -- the chair would leave the box -- but to draw it with a
   * heavier pen (lead decision 09/09: match the weight, keep the composition).
   */
  dam?: number;
}

export function laPoseNep(pose: string): pose is PoseNep {
  return (POSE_NEP as readonly string[]).includes(pose);
}

/** The layers of one pose, back to front. An unknown pose draws «moi». */
export function hinhNep(pose: string, tuyChon: TuyChonNep = {}): LopVe[] {
  const { x0 = 0, y0 = 0, tiLe = 1, chiTiet = true } = tuyChon;
  const P = bienDoi(x0, y0, tiLe);
  const dam = tuyChon.dam ?? 1;
  const net = (w: number) => w * tiLe * dam;
  const p: PoseNep = laPoseNep(pose) ? pose : "moi";
  const nghieng = tuyChon.nghieng ?? TU_THE[p].nghieng;
  const [nhinX, nhinY] = (tuyChon.nhin ?? TU_THE[p].nhin).map((v) => Math.max(-1.6, Math.min(1.6, v)));
  const bieuCam = tuyChon.bieuCam ?? TU_THE[p].bieuCam;
  const dang = tuyChon.dang ?? TU_THE[p].dang;
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

  // The eyes sit where the pose looks; the brows and the mouth carry the
  // feeling. The compact reading drops the brows and thickens what is left,
  // which is what keeps it a second drawing rather than a shrunk first one.
  const mieng = (rong: boolean): LopVe => {
    const w = net(rong ? 2.4 : 2);
    switch (bieuCam) {
      case "hao-hung":
        // Wider and deeper than the sidelong smile, still a closed curve.
        return { d: cong(S(44, 52), S(47, 58.5), S(53, 58.5), S(56.5, 52)), mau: "muc", net: w };
      case "hoi":
        // A small round mouth: the shape of a question, not a smile.
        return { d: bau(...S(50.5, 54.6), (rong ? 2.6 : 2.4) * tiLe, (rong ? 3.2 : 3) * tiLe), mau: "muc" };
      case "quyet":
        // A short straight line: decided, and not pleased about anything.
        return { d: netGay([S(46, 55), S(55, 54)]), mau: "muc", net: net(rong ? 2.6 : 2.2) };
      case "met":
        // A shallow S: worn down, without a frown.
        return {
          d: netCong(S(44.5, 54.6), [
            [S(47, 51.4), S(49, 58.2), S(51, 55.4)],
            [S(53, 52.6), S(55, 58.4), S(57, 55.2)],
          ]),
          mau: "muc",
          net: net(rong ? 2.2 : 1.9),
        };
      case "nhuong":
        // The offering smile: the sidelong one, a shade longer and softer.
        return { d: cong(S(45, 53), S(47.5, 56.8), S(52.5, 56.8), S(56, 52.6)), mau: "muc", net: w };
      case "binh-than":
      default:
        return { d: cong(S(46, rong ? 54 : 53), S(48, rong ? 56.5 : 56), S(52, rong ? 56.5 : 56), S(55, rong ? 54 : 53)), mau: "muc", net: w };
    }
  };

  const may = (): LopVe[] => {
    const w = net(1.9);
    switch (bieuCam) {
      case "hao-hung":
        // Both up, both arched.
        return [
          { d: qCong(S(34, 38), S(38.5, 35.5), S(43, 37.5)), mau: "muc", net: w },
          { d: qCong(S(48, 35.5), S(52, 33), S(56, 35)), mau: "muc", net: w },
        ];
      case "hoi":
        // One far up, one level: the shape of «ơ?».
        return [
          { d: netGay([S(34, 38.5), S(42, 37.5)]), mau: "muc", net: w },
          { d: netGay([S(47.5, 32), S(56, 34)]), mau: "muc", net: w },
        ];
      case "quyet":
        // Both lowered toward the middle: concentration, not anger.
        return [
          { d: netGay([S(34, 36.5), S(42, 39)]), mau: "muc", net: w },
          { d: netGay([S(48, 38), S(56, 35)]), mau: "muc", net: w },
        ];
      case "met":
        // Outer ends up, inner ends down: resigned.
        return [
          { d: netGay([S(34, 36), S(42, 39.5)]), mau: "muc", net: w },
          { d: netGay([S(48, 39), S(56, 35.5)]), mau: "muc", net: w },
        ];
      case "nhuong":
        // Level and soft, so nothing competes with the offered hand.
        return [
          { d: netGay([S(34.5, 37.5), S(42, 37)]), mau: "muc", net: w },
          { d: netGay([S(48, 36), S(55.5, 36.5)]), mau: "muc", net: w },
        ];
      case "binh-than":
      default:
        return [
          { d: netGay([S(35, 38), S(42, 37)]), mau: "muc", net: w },
          { d: netGay([S(48, 34), S(55, 36.5)]), mau: "muc", net: w },
        ];
    }
  };

  const mat: LopVe[] = chiTiet
    ? [
        { d: bau(...S(39 + nhinX, 45 + nhinY), 2.1 * tiLe, 3 * tiLe), mau: "muc" },
        { d: bau(...S(52 + nhinX, 43 + nhinY), 2.1 * tiLe, 3 * tiLe), mau: "muc" },
        ...may(),
        mieng(false),
      ]
    : [
        { d: tron(...S(39 + nhinX, 45 + nhinY), 2.6 * tiLe), mau: "muc" },
        { d: tron(...S(52 + nhinX, 43 + nhinY), 2.6 * tiLe), mau: "muc" },
        mieng(true),
      ];

  // Limbs are filled capsules, so they do not depend on the renderer's caps.
  const wTay = (chiTiet ? 4.2 : 5) * tiLe * dam;
  const rBan = (chiTiet ? 3.4 : 3.8) * tiLe * dam;
  const tay = (a: Diem, b: Diem): LopVe[] => [
    { d: vien(a, b, wTay), mau: "muc" },
    { d: tron(b[0], b[1], rBan), mau: "muc" },
  ];
  const L = S(25, 54), R = S(69, 50);

  // Legs from the sheared hip down. `dung` keeps both feet on the ground line;
  // the other stances are the reason a figure can now walk, sit or leave the
  // ground at all, which is what «Đi thôi!» and «Tuyệt vời» needed.
  const wDui = 5.5 * tiLe * dam, wCo = 3.6 * tiLe * dam, rBanChan = 3.6 * tiLe * dam;
  const chanTheo = (): LopVe[] => {
    switch (dang) {
      case "buoc":
        // Mid-stride: the near leg reaches forward and lands flat, the far leg
        // trails with the heel already lifted. The gap between the feet is the
        // whole read at 64dp, so it is wide on purpose.
        return [
          { d: thon(S(56, 75), P(74, 84), wDui, wCo), mau: "muc" },
          { d: vien(P(70, 87), P(80, 84), rBanChan), mau: "muc" },
          { d: thon(S(39, 76), P(23, 87), wDui, wCo), mau: "muc" },
          { d: vien(P(16, CHAN_NEP), P(26, CHAN_NEP), rBanChan), mau: "muc" },
        ];
      case "nhun":
        // Off the ground, knees tucked, with two short marks where the feet
        // just left. The marks are the only reason it reads as a jump and not
        // as a figure floating.
        return [
          { d: thon(S(39, 76), P(30, 84), wDui, wCo), mau: "muc" },
          { d: vien(P(26, 82), P(34, 84), rBanChan), mau: "muc" },
          { d: thon(S(56, 75), P(66, 83), wDui, wCo), mau: "muc" },
          { d: vien(P(62, 81), P(70, 83), rBanChan), mau: "muc" },
          ...(chiTiet
            ? [
                { d: netGay([P(30, CHAN_NEP), P(38, CHAN_NEP)]), mau: "bong" as const, net: net(2.4) },
                { d: netGay([P(58, CHAN_NEP), P(66, CHAN_NEP)]), mau: "bong" as const, net: net(2.4) },
              ]
            : []),
        ];
      case "ngoi":
        // Thighs forward and level, shins down: sitting on something the scene
        // supplies, with the feet ahead of the body rather than under it.
        return [
          { d: thon(S(39, 76), P(56, 78), wDui, wCo), mau: "muc" },
          { d: thon(P(56, 78), P(58, CHAN_NEP), wCo, wCo), mau: "muc" },
          { d: vien(P(54, CHAN_NEP), P(63, CHAN_NEP), rBanChan), mau: "muc" },
          { d: thon(S(50, 77), P(66, 80), wDui, wCo), mau: "muc" },
          { d: vien(P(64, 82), P(72, 80), rBanChan), mau: "muc" },
        ];
      case "chong":
        // Planted wide with the weight low: the stance of pressing down on
        // something rather than standing beside it.
        return [
          { d: thon(S(39, 74), P(28, 88), wDui, wCo), mau: "muc" },
          { d: thon(S(56, 73), P(70, 88), wDui, wCo), mau: "muc" },
          { d: vien(P(22, CHAN_NEP), P(32, CHAN_NEP), rBanChan), mau: "muc" },
          { d: vien(P(66, CHAN_NEP), P(76, CHAN_NEP), rBanChan), mau: "muc" },
        ];
      case "dung":
      default:
        return [
          { d: thon(S(39, 76), P(36, 90), wDui, wCo), mau: "muc" },
          { d: thon(S(56, 75), P(60, 90), wDui, wCo), mau: "muc" },
          { d: vien(P(31, CHAN_NEP), P(39, CHAN_NEP), rBanChan), mau: "muc" },
          { d: vien(P(58, CHAN_NEP), P(67, CHAN_NEP), rBanChan), mau: "muc" },
        ];
    }
  };
  const chan: LopVe[] = chanTheo();

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
    case "buoc-di":
      // Walking: the far arm swings forward and up, where the sticker hangs a
      // small flag at box (90, 26); the near arm swings back behind the body.
      tuThe = [...tay(R, P(90, 26)), ...tay(L, P(6, 62))];
      break;
    case "nang-to":
      // Both hands cradle something at chest height in front, box (64, 62):
      // an empty bowl, held up to ask what goes in it.
      tuThe = [...tay(R, P(88, 56)), ...tay(L, P(66, 60))];
      break;
    case "moi-ly":
      // Offering: the far arm straightens out and DOWN to the near edge of the
      // table at box (92, 68), where the sticker stands the cup, and the near
      // arm stays back. The reach is what says «this one is for you».
      tuThe = [...tay(R, P(88, 60)), ...tay(L, P(10, 62))];
      break;
    case "dat-tay":
      // Concluding: the far arm goes straight down onto the sheet at box
      // (80, 74) with the weight over it; the near arm hangs.
      tuThe = [...tay(R, P(80, 74)), ...tay(L, P(12, 66))];
      break;
    case "ngoi-xe":
      // Stuck: one hand still on the bar at box (86, 54), the other dropped.
      tuThe = [...tay(R, P(86, 54)), ...tay(L, P(22, 72))];
      break;
    case "dua-hai-tay": {
      // Handing something over with both hands: the near arm bends at the
      // elbow so the two hands meet in front, around box (76, 70), instead of
      // one arm crossing the body.
      const khuyu = P(40, 70);
      tuThe = [...tay(R, P(82, 66)), { d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(70, 72))];
      break;
    }
    case "nhay":
      // Off the ground, arms wide and up: wider than `vui`, which keeps its
      // quieter raise for the scenes.
      tuThe = [...tay(L, P(4, 22)), ...tay(R, P(93, 18))];
      break;
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
