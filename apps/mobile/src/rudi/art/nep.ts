/**
 * Nếp: the folded invitation that keeps a seat for you.
 *
 * Drawn after the 2026-09-08 concept sheet (docs/archive/codex/2026-09-08/nep-concept),
 * not after the rectangular sketch of 07/09: a short, slightly wide sheet with
 * a diagonal fold across the lower body like two lapels, ONE coral corner
 * folded down over the top right, small ink eyes under ONE uneven brow (the
 * folded corner covers where the other would go), a sidelong smile, short
 * tapered ink legs and mitten hands that do something.
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
  // 09/09 finish review: three scenes had reused an existing pose beside a
  // different rectangle, which is the one thing the brief forbade. These are
  // the bodies that make them different situations.
  "voi-len",
  "ghe-nhin",
  // 10/09 (audit F45a): «no message yet» used the writing body beside a
  // speech bubble, the same body `chua-co-keo` writes with. Opening a
  // conversation is a different act from starting a plan: this one calls out.
  "goi-loi",
  // Spec «Nếp truyền giấy» §17.3: the two-person notebook. Each is a thing
  // done to a sheet of paper -- handing one over, pressing one face down,
  // folding one small -- because in that notebook Nếp is the note passed
  // between two people, not the page everybody writes on.
  "dua-giay",
  "up-xuong",
  "gap-lai",
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
export const BIEU_CAM = ["binh-than", "hao-hung", "hoi", "quyet", "met", "nhuong", "giu-kin"] as const;
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
 * Which sheet the figure is (spec «Nếp truyền giấy» §17).
 *
 * `trang` is the page of the group notebook: the drawing that shipped and
 * that every scene and sticker composes. It does not move by one coordinate;
 * `tests/art-duong.test.mjs` pins its output against a sha256 taken from
 * `main` before this option existed. `manh` is the same figure folded in
 * four to fit a pocket, for the two-person notebook: a squarer sheet, two
 * crossing creases where the lapel was, and the coral corner folded INWARD
 * so only a sliver shows along the cut. Same eyes, same one brow, same mouth,
 * same hands. It is Nếp, not a second character.
 */
export const GAP_NEP = ["trang", "manh"] as const;
export type GapNep = (typeof GAP_NEP)[number];

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
  // Sitting on something that is not moving: one hand still on the bar, the
  // other propping the chin, eyes down. Bored, not riding (audit 10/09, F45b).
  "ngoi-xe": { nghieng: 1, nhin: [1.2, 1.4], bieuCam: "met", dang: "ngoi" },
  // Both hands out, offering something small, with a small bow.
  "dua-hai-tay": { nghieng: 3, nhin: [1.2, 0.8], bieuCam: "nhuong", dang: "dung" },
  // Both feet off the ground.
  nhay: { nghieng: 0, nhin: [0, -0.9], bieuCam: "hao-hung", dang: "nhun" },
  // Holding something light up in front of the face and looking into it.
  // Reaching up for a line overhead, weight on the front foot.
  // Reaching up for a line overhead. `dung`, not `buoc`: the stride threw a
  // foot out to the left, straight under whatever the low hand is holding.
  "voi-len": { nghieng: 4, nhin: [0.8, -1.4], bieuCam: "binh-than", dang: "dung" },
  // Bending sideways to look through something narrow.
  // Bending sideways to look through something narrow. `dung`, not `chong`:
  // planted legs hold the body upright and the shear stops reading, which is
  // how the first version came out as a shrug (finish review 09/09).
  "ghe-nhin": { nghieng: 13, nhin: [1.6, 0.3], bieuCam: "hoi", dang: "dung" },
  // Calling out: the near hand cupped beside the mouth, the far hand open toward
  // whoever it is for. The speech bubble a scene draws starts at the mouth.
  "goi-loi": { nghieng: 3, nhin: [1.4, 0.2], bieuCam: "hao-hung", dang: "dung" },
  // Offering a small folded sheet to the right, leaning into the offer and
  // looking at the hand that holds it.
  "dua-giay": { nghieng: 6, nhin: [1.4, 0.4], bieuCam: "nhuong", dang: "dung" },
  // One hand pressing a sheet face down on the floor beside the feet: not
  // yet, and no comment about it.
  "up-xuong": { nghieng: 5, nhin: [1.6, 3.0], bieuCam: "giu-kin", dang: "dung" },
  // Sitting, folding a small sheet in both hands in front of the chest, eyes
  // down on the fold.
  "gap-lai": { nghieng: 0, nhin: [0.6, 1.4], bieuCam: "giu-kin", dang: "dung" },
};

/** A pose's lean, gaze, face and stance, as the still figure draws it (unknown poses read as «moi»). */
export function tuTheCuaPose(pose: string): { nghieng: number; nhin: readonly [number, number]; bieuCam: BieuCamNep; dang: DangNep } {
  return TU_THE[laPoseNep(pose) ? pose : "moi"];
}

/** The feet stand on this line of the 96-box; a scene puts its floor here. */
export const CHAN_NEP = 91;

export interface TuyChonNep {
  /** Which sheet: the group page (`"trang"`, default, unchanged) or the pocket-folded one (`"manh"`). */
  gap?: GapNep;
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

/**
 * Everything one drawing of the figure is built from, resolved once: the
 * placement, the lean as a point map, the widths, the face and the legs. The
 * pieces below (`thanNep`, `matNep`, `chanNep`, `tayNep`) each read it, so the
 * paper puppet (`nep-roi.ts`) takes the sheet and the face exactly as the
 * still figure draws them and hinges its own limbs on the same points.
 * `hinhNep` is those four pieces in the order it always drew them: the sha256
 * lock of the `trang` sheet (tests/art-duong.test.mjs) holds across the split.
 */
export interface NguCanhNep {
  /** The pose after the unknown-pose fallback. */
  p: PoseNep;
  gap: GapNep;
  chiTiet: boolean;
  tiLe: number;
  /** Ink weight multiplier (`TuyChonNep.dam`). */
  dam: number;
  /** Placement only: offset, then scale. Hands and props land through this. */
  P: (x: number, y: number) => Diem;
  /** Placement plus the lean: a shear of everything above the ground line. */
  S: (x: number, y: number) => Diem;
  /** A stroke width at this scale and ink weight. */
  net: (w: number) => number;
  nhinX: number;
  nhinY: number;
  bieuCam: BieuCamNep;
  dang: DangNep;
  /** Arm capsule width and mitten radius. */
  wTay: number;
  rBan: number;
  /** The near and far shoulders, sheared with the body. */
  L: Diem;
  R: Diem;
}

/**
 * Where the limbs are hinged on the upright sheet, in the 96-box before any
 * lean: the two shoulders (the pocket sheet's near shoulder sits out at its
 * wider edge) and the two hips. The paper puppet pins its limbs here.
 */
export const VAI_GAN: Diem = [25, 54];
export const VAI_GAN_MANH: Diem = [20, 54];
export const VAI_XA: Diem = [69, 50];
export const HONG_GAN: Diem = [39, 76];
export const HONG_XA: Diem = [56, 75];

/** Resolve one drawing's context. An unknown pose draws «moi». */
export function nguCanhNep(pose: string, tuyChon: TuyChonNep = {}): NguCanhNep {
  const { x0 = 0, y0 = 0, tiLe = 1, chiTiet = true, gap = "trang" } = tuyChon;
  const P = bienDoi(x0, y0, tiLe);
  const dam = tuyChon.dam ?? 1;
  const net = (w: number) => w * tiLe * dam;
  const p: PoseNep = laPoseNep(pose) ? pose : "moi";
  const nghieng = tuyChon.nghieng ?? TU_THE[p].nghieng;
  // Gaze is clamped so a pupil cannot leave the face. ±1.6 was the ceiling
  // until `up-xuong`: a shift that small never read as «looking down at the
  // sheet» (blind reads, rounds 3 and 4), and no existing pose or scene passes
  // more than 1.6, so widening the clamp moves nothing that is already drawn.
  const [nhinX, nhinY] = (tuyChon.nhin ?? TU_THE[p].nhin).map((v) => Math.max(-3, Math.min(3, v)));
  const bieuCam = tuyChon.bieuCam ?? TU_THE[p].bieuCam;
  const dang = tuyChon.dang ?? TU_THE[p].dang;
  // The lean: a shear of everything above the ground line, so the feet keep
  // their place on the floor and the head travels the whole `nghieng`. Hands
  // are placed through `P`, unsheared, because they land on things.
  const S = (x: number, y: number): Diem => P(x + (nghieng * (CHAN_NEP - y)) / (CHAN_NEP - 20), y);
  // Limbs are filled capsules, so they do not depend on the renderer's caps.
  const wTay = (chiTiet ? 4.2 : 5) * tiLe * dam;
  const rBan = (chiTiet ? 3.4 : 3.8) * tiLe * dam;
  // The pocket-fold sheet is wider on the left, so its near shoulder moves
  // out with the edge; the far shoulder sits where both edges nearly agree.
  const L = S(gap === "manh" ? VAI_GAN_MANH[0] : VAI_GAN[0], VAI_GAN[1]), R = S(VAI_XA[0], VAI_XA[1]);
  return { p, gap, chiTiet, tiLe, dam, P, S, net, nhinX, nhinY, bieuCam, dang, wTay, rBan, L, R };
}

/** The layers of one pose, back to front. An unknown pose draws «moi». */
export function hinhNep(pose: string, tuyChon: TuyChonNep = {}): LopVe[] {
  const ctx = nguCanhNep(pose, tuyChon);
  return [...thanNep(ctx), ...matNep(ctx), ...chanNep(ctx), ...tayNep(ctx)];
}

/** The sheet: its outline, the lapel (or the pocket sheet's creases) and the coral corner. */
export function thanNep(ctx: NguCanhNep): LopVe[] {
  const { S, net, chiTiet, gap } = ctx;
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
  // The pocket-folded sheet (`gap: "manh"`). Same cut edge H..G, so the one
  // brow keeps its reason and nothing above the eyes moves; the outline is a
  // shade narrower so «folded in four» reads at the silhouette. Two creases
  // cross where the lapel was -- drawn BEFORE the face, so the eyes and mouth
  // sit on top of them the way ink sits on a fold. And the corner folds
  // INWARD: the flap goes under, and what shows is a sliver of its coral back
  // along the cut. What matters is folded in; a sliver says it is there.
  // `laDaiGap` in tests/art-duong.test.mjs identifies this sliver by shape and
  // keeps ink out of it, the way `laNepGap` does for the page's corner.
  const thanManh = (): LopVe[] => {
    // Squarer by being WIDER, not shorter: the legs are anchored at y 76, so a
    // shorter sheet would float above them. Squarer at the OUTLINE, not just
    // at the bounding box: the first cut kept the page's bevels at the foot
    // and its bulge at the hip, and a blind read saw a hexagon (finish review
    // 12/09, round 2), so the left edge runs straight down and the foot
    // straight across, with only the one-unit wobble of a hand-cut sheet. And
    // wider than the eye's threshold: 48 over 56 still read «hẹp hơn» in
    // round 3, so the sheet grows to the LEFT (the cut edge H..G stays where
    // the page has it) to 53 over 56. The two creases quarter the sheet: the
    // horizontal one crosses the face between the eyes and the mouth, drawn
    // before them, because a crease at the hem (y 62) made the top cell twice
    // the bottom one and the figure read tall. `art-duong` measures how much
    // of its box the sheet fills and that the box is not narrower than 0.94.
    const Am = S(19, 22), Cm = S(72, 74), Dm = S(70, 76), Em = S(20, 76);
    // The sliver's inner vertex: 4.8 units off the cut at the 96dp reading, 6
    // at the 48dp one. Three units was about one dp at 48 and a blind read
    // called the coral «gần như mất» (round 3); at 96 it still read as «vệt hở
    // phải nhìn kỹ» (round 4). `laDaiGap` accepts both.
    const M = chiTiet ? S(56.2, 32.5) : S(55.4, 33.4);
    const vien = [Am, H, G, Cm, Dm, Em];
    return [
      { d: daGiac(vien), mau: "giay" },
      { d: netGay([S(47, 23), S(47, 75)]), mau: "bong", net: net(chiTiet ? 1.8 : 2.4) },
      { d: netGay([S(20, 50.5), S(69.6, 50.5)]), mau: "bong", net: net(chiTiet ? 1.8 : 2.4) },
      { d: daGiac([H, G, M]), mau: "gap" },
      { d: daGiac(vien), mau: "muc", net: net(chiTiet ? 2.4 : 3) },
      ...(chiTiet ? [{ d: daGiac([H, G, M]), mau: "muc" as const, net: net(1.6) }] : []),
    ];
  };
  const than: LopVe[] =
    gap === "trang"
      ? [
          { d: daGiac([A, H, G, C, D, E, F]), mau: "giay" },
          { d: daGiac([V1, F, E, D, V2]), mau: "bong" },
          ...(chiTiet ? [{ d: netGay([V1, V2]), mau: "muc" as const, net: net(1.8) }] : []),
          { d: daGiac([H, G, Bp]), mau: "gap" },
          { d: daGiac([A, H, G, C, D, E, F]), mau: "muc", net: net(chiTiet ? 2.4 : 3) },
          { d: daGiac([H, G, Bp]), mau: "muc", net: net(chiTiet ? 1.8 : 2.4) },
        ]
      : thanManh();
  return than;
}

/**
 * The face in its three parts: the two eyes (where the pose looks), the one
 * brow and the mouth (what it feels). The compact reading has no brow.
 */
export function matNepPhan(ctx: NguCanhNep): { mat: LopVe[]; may: LopVe[]; mieng: LopVe } {
  const { S, net, tiLe, chiTiet, bieuCam, nhinX, nhinY } = ctx;
  // The eyes sit where the pose looks; the brow and the mouth carry the
  // feeling. The compact reading drops the brow and thickens what is left,
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
      case "giu-kin":
        // Keeping it: a short level line, closed. Shorter and dead level where
        // `quyet` is longer and tilted -- knowing, not deciding.
        return { d: netGay([S(47, 54.5), S(53, 54.5)]), mau: "muc", net: net(rong ? 2.2 : 1.9) };
      case "nhuong":
        // The offering smile: the sidelong one, a shade longer and softer.
        return { d: cong(S(45, 53), S(47.5, 56.8), S(52.5, 56.8), S(56, 52.6)), mau: "muc", net: w };
      case "binh-than":
      default:
        return { d: cong(S(46, rong ? 54 : 53), S(48, rong ? 56.5 : 56), S(52, rong ? 56.5 : 56), S(55, rong ? 54 : 53)), mau: "muc", net: w };
    }
  };

  // ONE brow, over the left eye, and never a second one.
  //
  // The folded corner (H 50,20 · G 69,38 · Bp 50,38) owns everything above the
  // eyes from x 50 rightwards. A right brow has nowhere to live: drawn where a
  // brow belongs it puts a black bar across the coral -- the identity mark --
  // and pushed below the fold's own outline it fuses with the right eye into a
  // single dark blob. Both readings were rendered side by side on 09/09 before
  // this was decided. So the flap covers that brow, which is what a folded
  // sheet does, and the header's "uneven brow" becomes literal: the left brow
  // carries the whole amplitude, the mouth carries the rest.
  //
  // `khongCatNepGap` in tests/art-duong.test.mjs holds the corner clear from
  // now on -- a mechanism, not a promise. It measures PAINT (centre plus half
  // the stroke), bounds each curve rather than spot-sampling it, identifies
  // the fold by shape so the lean cannot blind it, and runs over every pose ×
  // expression × lean × ink weight, and over the composed stickers and scenes
  // that actually reach a screen.
  const may = (): LopVe[] => {
    const w = net(1.9);
    switch (bieuCam) {
      case "hao-hung":
        // Arched high: the brow of somebody already halfway out the door.
        return [{ d: qCong(S(33.5, 38), S(38.5, 34.6), S(43.5, 37.2)), mau: "muc", net: w }];
      case "hoi":
        // Far up and tilted in: the shape of «ơ?».
        return [{ d: netGay([S(33, 34.6), S(42.5, 31.8)]), mau: "muc", net: w }];
      case "quyet":
        // Inner end DOWN toward the eyes: decided, bearing down on the thing.
        return [{ d: netGay([S(33.5, 35.4), S(42.5, 39.2)]), mau: "muc", net: net(2.1) }];
      case "met":
        // Inner end UP, outer end down -- the opposite of `quyet`, and the
        // reason the two used to be indistinguishable: this one was drawn with
        // the determined slope by mistake until 09/09. Resigned, not angry.
        return [{ d: netGay([S(33.5, 39.4), S(42.5, 35.8)]), mau: "muc", net: w }];
      case "giu-kin":
        // Lowered two units and dead flat: the brow of somebody who knows and
        // is not going to say. Lower than `binh-than`, level unlike `quyet`.
        return [{ d: netGay([S(34, 39.6), S(42.5, 39.6)]), mau: "muc", net: w }];
      case "nhuong":
        // A shallow arc bowed DOWNWARD, the mirror of `hao-hung`: warm and
        // asking rather than announcing, so the face yields to the hand doing
        // the offering. Straight and merely tilted, it was within one unit of
        // `binh-than` and the two were the same face (caught 09/09).
        return [{ d: qCong(S(33.8, 36.4), S(38.5, 39), S(43, 36.8)), mau: "muc", net: w }];
      case "binh-than":
      default:
        // Nearly level: no feeling claimed, which is the point of the default.
        return [{ d: netGay([S(34.5, 37.5), S(42.5, 37.1)]), mau: "muc", net: w }];
    }
  };

  const mat: LopVe[] = chiTiet
    ? [
        { d: bau(...S(39 + nhinX, 45 + nhinY), 2.1 * tiLe, 3 * tiLe), mau: "muc" },
        { d: bau(...S(52 + nhinX, 43 + nhinY), 2.1 * tiLe, 3 * tiLe), mau: "muc" },
      ]
    : [
        { d: tron(...S(39 + nhinX, 45 + nhinY), 2.6 * tiLe), mau: "muc" },
        { d: tron(...S(52 + nhinX, 43 + nhinY), 2.6 * tiLe), mau: "muc" },
      ];
  return { mat, may: chiTiet ? may() : [], mieng: mieng(!chiTiet) };
}

/** The face as the still figure draws it: eyes, brow, mouth. */
export function matNep(ctx: NguCanhNep): LopVe[] {
  const { mat, may, mieng } = matNepPhan(ctx);
  return [...mat, ...may, mieng];
}

/** The legs of the pose's stance. */
export function chanNep(ctx: NguCanhNep): LopVe[] {
  const { S, P, net, tiLe, dam, chiTiet, dang } = ctx;
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
  return chan;
}

/** The arms of the pose and whatever it holds. */
export function tayNep(ctx: NguCanhNep): LopVe[] {
  const { p, P, net, tiLe, chiTiet, wTay, rBan, L, R } = ctx;
  const tay = (a: Diem, b: Diem): LopVe[] => [
    { d: vien(a, b, wTay), mau: "muc" },
    { d: tron(b[0], b[1], rBan), mau: "muc" },
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
      // Keeping a seat: the far hand reaches ACROSS and DOWN, to box (90, 66),
      // where a table top or a rail is; the near arm rests. It used to stop at
      // chest height, which left the gesture ending in mid-air once the figure
      // sat down at a table (finish review 09/09).
      tuThe = [...tay(R, P(90, 66)), ...tay(L, P(14, 68))];
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
    case "nang-to": {
      // Both hands cradle an empty bowl held out in front, at box (62, 58) and
      // (84, 58) -- LEVEL, so a rim can rest on both. The near arm bends at an
      // elbow for the same reason `dua-hai-tay` does: run it straight from the
      // far shoulder and one capsule crosses the whole torso and the mouth,
      // which is what the finish review saw at 64dp.
      const khuyu = P(34, 70);
      tuThe = [...tay(R, P(84, 58)), { d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(62, 58))];
      break;
    }
    case "moi-ly":
      // Offering: the far arm straightens out to box (88, 60), where the
      // sticker stands the cup, and the near arm stays back. The reach is what
      // says «this one is for you».
      tuThe = [...tay(R, P(88, 60)), ...tay(L, P(10, 62))];
      break;
    case "dat-tay":
      // Concluding: the far arm comes DOWN onto the sheet at box (78, 84),
      // near the floor, and the near arm braces back. Pressing something down
      // low is a different silhouette from holding something out at the waist;
      // at 64dp the two used to be the same shape with a different mark in it.
      tuThe = [...tay(R, P(78, 84)), ...tay(L, P(10, 54))];
      break;
    case "ngoi-xe": {
      // Stuck: one hand still on the bar at box (86, 54); the near arm bends at
      // an elbow and the hand comes up to the chin at (34, 58) -- the one
      // gesture that says waiting rather than riding (audit 10/09, F45b). The
      // hand sits left of the mouth (x 44+) and well below the fold (y 38-).
      const khuyu = P(20, 72);
      tuThe = [...tay(R, P(86, 54)), { d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(34, 58))];
      break;
    }
    case "dua-hai-tay": {
      // Handing something over with both hands: the far hand at box (82, 66),
      // the near one at (70, 72) after an elbow, so the two meet in front
      // instead of one arm crossing the body.
      const khuyu = P(40, 70);
      tuThe = [...tay(R, P(82, 66)), { d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(70, 72))];
      break;
    }
    case "nhay":
      // Off the ground, arms wide and up: wider than `vui`, which keeps its
      // quieter raise for the scenes.
      tuThe = [...tay(L, P(4, 22)), ...tay(R, P(93, 18))];
      break;
    case "voi-len":
      // One arm up and OUT to a line overhead at box (86, 16); the other holds
      // what is about to go on it, out to the side at box (8, 62), which is
      // the coordinate the scene hangs the print from. Out, not straight up:
      // a vertical reach from that shoulder runs along the fold triangle's
      // right edge and darkens the coral corner.
      tuThe = [...tay(R, P(86, 16)), ...tay(L, P(8, 62))];
      break;
    case "ghe-nhin":
      // Leaning in to look through a gap: the far hand braces at box (78, 44),
      // where the scene puts the one open square, and the near arm hangs DOWN.
      // Held out straight it made a T, and a T reads as a shrug however far
      // the body leans. The brace point is BELOW y 38 on purpose: a limb run
      // up to the shoulder's own height crosses the sheared fold triangle and
      // puts a dark bar across the coral corner, which is the identity mark.
      tuThe = [...tay(R, P(78, 44)), ...tay(L, P(10, 72))];
      break;
    case "goi-loi": {
      // Calling out: the near arm bends at an elbow (16, 70) and the hand cups
      // beside the mouth at (37, 58) -- left of the mouth, which starts at x 44,
      // and twenty units under the fold; the far arm opens out and down to
      // (92, 66), toward whoever the word is for. The bubble is the scene's:
      // it starts where this mouth is (`canh.ts`, `chua-co-tin-nhan`).
      const khuyu = P(16, 70);
      tuThe = [{ d: vien(L, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(37, 58)), ...tay(R, P(92, 66))];
      break;
    }
    case "dua-giay": {
      // Handing a small folded sheet to the right: the far hand carries it
      // out past the body, the near arm rests. The sheet is drawn after the
      // hand so it sits in the palm. Its corner is a plain fold, NOT coral:
      // the figure has exactly one coral mark, the sliver on its own body, and
      // a blind read of the first cut counted two (finish review 12/09). The
      // flap is `giay` with an ink edge, not `bong`: on the dark scheme
      // `paperShade` is darker than `paper`, and a shaded flap read as a
      // notch cut out of the sheet (round 3).
      const vienTo = [P(79, 39), P(88, 39), P(91, 42), P(91, 55), P(79, 55)];
      const vatTo = [P(88, 39), P(88, 42), P(91, 42)];
      const to: LopVe[] = [
        { d: daGiac(vienTo), mau: "giay" },
        { d: daGiac(vienTo), mau: "muc", net: net(1.6) },
        { d: daGiac(vatTo), mau: "muc", net: net(1.2) },
      ];
      tuThe = [...tay(R, P(85, 47)), ...to, ...tay(L, P(14, 66))];
      break;
    }
    case "up-xuong": {
      // A sheet lying face down on the floor to the right, one hand flat on
      // it. Two cuts read as «kéo vật bằng que» and «kéo va li» (finish
      // reviews 12/09): a slab-shaped sheet, an arm reaching sideways to its
      // edge, a figure standing straight and looking ahead. So: the sheet
      // lies FLAT, four times wider than tall, on the ground line; the arm
      // comes nearly straight down with the mitten in the MIDDLE of the
      // sheet, not at its edge; and the figure leans toward it and looks
      // down at it (`TU_THE`). Its corner fold is `giay` with an ink edge,
      // like the sheet in `dua-giay`.
      const vienTo = [P(64, 83), P(92, 83), P(96, 87), P(96, 91), P(64, 91)];
      const vatTo = [P(92, 83), P(92, 87), P(96, 87)];
      const to: LopVe[] = [
        { d: daGiac(vienTo), mau: "giay" },
        { d: daGiac(vienTo), mau: "muc", net: net(1.6) },
        { d: daGiac(vatTo), mau: "muc", net: net(1.2) },
      ];
      const khuyu = P(74, 70);
      tuThe = [...to, { d: vien(R, khuyu, wTay), mau: "muc" }, ...tay(khuyu, P(80, 87)), ...tay(L, P(12, 64))];
      break;
    }
    case "gap-lai": {
      // Folding a small sheet in both hands at the chest. Standing: the
      // shared seated legs read as «một chân đá ra sau» under a figure whose
      // hands are busy (round 3), and they belong to `ngoi-xe`, so they are not
      // this pose's to change. The fold itself has to be visible: a flat
      // sheet with a faint line down its middle read as «cầm thẻ». So the
      // sheet is two panels, the right one already turning toward the viewer
      // (a narrower parallelogram), with the fold as an ink edge between them.
      // Drawn over the body -- it is held in front -- and below the sliver.
      const trai = [P(45, 58), P(53, 58), P(53, 70), P(45, 70)];
      const phai = [P(53, 58), P(60, 61), P(60, 67), P(53, 70)];
      const to: LopVe[] = [
        { d: daGiac(trai), mau: "giay" },
        { d: daGiac(phai), mau: "giay" },
        { d: daGiac(trai), mau: "muc", net: net(1.6) },
        { d: daGiac(phai), mau: "muc", net: net(1.6) },
      ];
      tuThe = [...to, ...tay(L, P(44, 66)), ...tay(R, P(61, 63))];
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
  return tuThe;
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
